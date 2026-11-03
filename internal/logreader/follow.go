package logreader

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

// Follower tracks a raw byte cursor, not rendered text. Identity is the selected
// log path. Raw prefix/cursor anchors and modification metadata also detect
// same-path replacement or copy-truncation on SFTP, which has no portable inode.
// Each reader is borrowed for one poll; no remote handle stays open across polls.
type Follower struct {
	identity string
	modified time.Time
	offset   int64
	started  bool
	prefix   []byte
	anchor   []byte
	partial  []byte
	dropping bool
	oversize bool
	markers  string
	redactor redactor
}

type FollowExcerpt struct {
	Excerpt
	Rotated bool
}

const anchorBytes = 256

func readRange(r io.ReadSeeker, offset, count int64) ([]byte, error) {
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(r, count))
}

func (f *Follower) continuous(r io.ReadSeeker, size int64) (bool, error) {
	if size < f.offset {
		return false, nil
	}
	for _, sample := range []struct {
		offset int64
		data   []byte
	}{{0, f.prefix}, {f.offset - int64(len(f.anchor)), f.anchor}} {
		if len(sample.data) == 0 {
			continue
		}
		b, err := readRange(r, sample.offset, int64(len(sample.data)))
		if err != nil {
			return false, err
		}
		if !bytes.Equal(b, sample.data) {
			return false, nil
		}
	}
	return true, nil
}

// Read emits the initial bounded tail, then newly appended complete lines. Partial
// lines and private-key state survive polls; redaction happens only after framing.
// Payload reads and returned text are capped at MaxBytes per poll; small raw
// continuity anchors add at most 1 KiB of reads. Truncation is explicit.
func (f *Follower) Read(r io.ReadSeeker, identity string, size int64, modified time.Time, lines int) (FollowExcerpt, error) {
	var out FollowExcerpt
	if identity == "" || size < 0 || lines < 1 || lines > 10000 {
		return out, errors.New("follow requires a log identity, nonnegative size and 1..10000 initial lines")
	}
	reset := !f.started || identity != f.identity || size < f.offset || (size == f.offset && !modified.Equal(f.modified))
	if !reset {
		ok, err := f.continuous(r, size)
		if err != nil {
			return out, err
		}
		reset = !ok
	}
	initial := reset
	if reset {
		out.Rotated = f.started
		*f = Follower{identity: identity, modified: modified, started: true}
		f.offset = max(0, size-MaxBytes)
		f.dropping = f.offset > 0
		out.Truncated = f.dropping
	}
	data, err := readRange(r, f.offset, min(size-f.offset, MaxBytes))
	if err != nil {
		return out, err
	}
	out.BytesRead = len(data)
	f.offset += int64(len(data))
	f.modified = modified
	text, truncated := f.consume(data)
	out.Truncated = out.Truncated || truncated
	if initial {
		parts := strings.SplitAfter(text, "\n")
		if len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
		if len(parts) > lines {
			parts = parts[len(parts)-lines:]
			out.Truncated = true
		}
		text = strings.Join(parts, "")
	}
	out.Text = text
	if len(f.prefix) == 0 {
		f.prefix, err = readRange(r, 0, min(f.offset, anchorBytes))
		if err != nil {
			return out, err
		}
	}
	f.anchor, err = readRange(r, max(0, f.offset-anchorBytes), min(f.offset, anchorBytes))
	return out, err
}

var privateBoundary = regexp.MustCompile(`(?i)-----(BEGIN|END) (?:[A-Z0-9]+ )?PRIVATE KEY-----`)

// Oversized unterminated lines retain only a short marker suffix, never the line.
func (f *Follower) observe(fragment []byte) {
	text := f.markers + string(fragment)
	for _, match := range privateBoundary.FindAllStringSubmatch(text, -1) {
		f.redactor.private = strings.EqualFold(match[1], "BEGIN")
	}
	f.markers = strings.Clone(text[max(0, len(text)-128):])
}

func (f *Follower) consume(data []byte) (string, bool) {
	data = append(f.partial, data...)
	f.partial = nil
	f.redactor.prepareMessages(string(data))
	var out strings.Builder
	truncated := false
	for len(data) > 0 {
		at := bytes.IndexByte(data, '\n')
		if at < 0 {
			if f.dropping || f.oversize || len(data) > maxLineBytes {
				f.observe(data)
				f.oversize = !f.dropping
			} else {
				f.partial = bytes.Clone(data)
			}
			break
		}
		line := data[:at]
		data = data[at+1:]
		if f.dropping {
			f.observe(line)
			f.dropping, f.markers = false, ""
			continue
		}
		text := ""
		if f.oversize {
			f.observe(line)
			text = redactedLine
			f.oversize, f.markers = false, ""
		} else {
			text = f.redactor.line(string(line))
		}
		if out.Len()+len(text)+1 <= MaxBytes {
			out.WriteString(text)
			out.WriteByte('\n')
		} else {
			truncated = true
		}
	}
	return out.String(), truncated
}
