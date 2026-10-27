package snapshot

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func inspect(z *zip.Reader, rules Rules) (Manifest, map[string]*zip.File, error) {
	var manifest Manifest
	files := map[string]*zip.File{}
	folded := map[string]bool{}
	if len(z.File) > MaxEntries+1 {
		return manifest, nil, errors.New("archive entry limit exceeded")
	}
	for _, file := range z.File {
		name := strings.TrimSuffix(file.Name, "/")
		if file.Name == "manifest.json" {
			if files[name] != nil {
				return manifest, nil, errors.New("duplicate manifest")
			}
			files[name] = file
			continue
		}
		if name == "" || path.Clean(name) != name || strings.Contains(name, "\\") || !rules.Allow(name) {
			return manifest, nil, fmt.Errorf("unsafe archive path %q", file.Name)
		}
		if _, err := rules.Check(os.TempDir(), name); err != nil {
			return manifest, nil, err
		}
		if file.Mode()&os.ModeSymlink != 0 || (!file.Mode().IsRegular() && !file.FileInfo().IsDir()) {
			return manifest, nil, errors.New("archive links/special files are prohibited")
		}
		lower := strings.ToLower(name)
		if folded[lower] {
			return manifest, nil, errors.New("case-colliding archive entries")
		}
		folded[lower] = true
		files[name] = file
	}
	metadata := files["manifest.json"]
	if metadata == nil || metadata.UncompressedSize64 > 32<<20 {
		return manifest, nil, errors.New("missing or oversized backup manifest")
	}
	r, err := metadata.Open()
	if err != nil {
		return manifest, nil, err
	}
	data, err := io.ReadAll(io.LimitReader(r, 32<<20+1))
	r.Close()
	if err != nil || len(data) > 32<<20 {
		return manifest, nil, errors.New("cannot read bounded manifest")
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		return manifest, nil, err
	}
	if manifest.Version != 1 || len(manifest.Entries) > MaxEntries || len(manifest.Entries)+1 != len(z.File) {
		return manifest, nil, errors.New("invalid backup manifest version or entry inventory")
	}
	seen := map[string]bool{}
	var total int64
	for _, entry := range manifest.Entries {
		file := files[entry.Path]
		if file == nil || entry.Path == "manifest.json" || seen[entry.Path] || entry.Size < 0 || entry.Size > MaxDataBytes-total {
			return manifest, nil, errors.New("manifest inventory mismatch")
		}
		seen[entry.Path] = true
		total += entry.Size
		if entry.Directory != file.FileInfo().IsDir() || uint64(entry.Size) != file.UncompressedSize64 {
			return manifest, nil, errors.New("archive size/type mismatch")
		}
		if !entry.Directory {
			if raw, e := hex.DecodeString(entry.SHA256); e != nil || len(raw) != sha256.Size {
				return manifest, nil, errors.New("invalid content hash")
			}
		}
	}
	return manifest, files, nil
}

// Verify streams every declared byte and rejects undeclared entries and CRC/hash errors.
func Verify(ctx context.Context, file, target string, rules Rules) (Manifest, error) {
	return extract(ctx, file, target, "", rules, nil)
}

// Extract verifies all content, including skipped files. Skip keys and archive
// paths use the same case-insensitive identity as the Windows restore target.
func Extract(ctx context.Context, file, target, destination string, rules Rules, skip map[string]bool) (Manifest, error) {
	return extract(ctx, file, target, destination, rules, skip)
}
func extract(ctx context.Context, file, target, destination string, rules Rules, skip map[string]bool) (Manifest, error) {
	z, err := zip.OpenReader(file)
	if err != nil {
		return Manifest{}, err
	}
	defer z.Close()
	return readContents(ctx, &z.Reader, target, destination, rules, skip)
}

func VerifyReader(ctx context.Context, r io.ReaderAt, size int64, target string, rules Rules) (Manifest, error) {
	z, err := zip.NewReader(r, size)
	if err != nil {
		return Manifest{}, err
	}
	return readContents(ctx, z, target, "", rules, nil)
}

func readContents(ctx context.Context, z *zip.Reader, target, destination string, rules Rules, skip map[string]bool) (Manifest, error) {
	manifest, files, err := inspect(z, rules)
	if err != nil {
		return manifest, err
	}
	if manifest.Target != target {
		return manifest, errors.New("backup belongs to a different target")
	}
	excluded := map[string]bool{}
	for name, omit := range skip {
		if omit {
			excluded[strings.ToLower(name)] = true
		}
	}
	for _, entry := range manifest.Entries {
		if err = ctx.Err(); err != nil {
			return manifest, err
		}
		dest := ""
		if destination != "" && !excluded[strings.ToLower(entry.Path)] {
			dest, err = rules.Check(destination, entry.Path)
			if err != nil {
				return manifest, err
			}
		}
		if entry.Directory {
			if dest != "" {
				if err = os.MkdirAll(dest, 0700); err != nil {
					return manifest, err
				}
			}
			continue
		}
		input, e := files[entry.Path].Open()
		if e != nil {
			return manifest, e
		}
		hash := sha256.New()
		var output *os.File
		writer := io.Writer(hash)
		if dest != "" {
			if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
				input.Close()
				return manifest, err
			}
			output, err = os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				input.Close()
				return manifest, err
			}
			writer = io.MultiWriter(output, hash)
		}
		n, e := copyContext(ctx, writer, io.LimitReader(input, entry.Size+1))
		input.Close()
		if output != nil {
			e = errors.Join(e, output.Close())
		}
		if e != nil {
			return manifest, e
		}
		if n != entry.Size || hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
			return manifest, fmt.Errorf("backup content verification failed: %s", entry.Path)
		}
	}
	return manifest, nil
}
