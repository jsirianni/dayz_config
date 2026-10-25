package deploy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Remote struct {
	SSH      *ssh.Client
	SFTP     *sftp.Client
	Endpoint Endpoint
	agent    net.Conn
	cancel   func() bool
}

func Connect(ctx context.Context, e Endpoint) (*Remote, error) {
	return ConnectPinned(ctx, e, "", false)
}

// ConnectPinned permits initial enrollment only when the supplied fingerprint
// matches. Existing known-host entries always take precedence over a new pin.
// A dry-run can verify a pin without writing known_hosts.
func ConnectPinned(ctx context.Context, e Endpoint, fingerprint string, persist bool) (*Remote, error) {
	if e.User == "" || e.KnownHosts == "" || !strings.HasPrefix(e.RemoteDir, "/") || strings.Contains(e.RemoteDir, "..") {
		return nil, errors.New("endpoint requires user, knownHosts and absolute SFTP remoteDir")
	}
	if _, _, err := net.SplitHostPort(e.Address); err != nil {
		return nil, fmt.Errorf("address must include SSH port: %w", err)
	}
	verify, err := hostVerifier(e.KnownHosts, fingerprint, persist)
	if err != nil {
		return nil, fmt.Errorf("load pinned host keys: %w", err)
	}
	r := &Remote{Endpoint: e}
	var auth []ssh.AuthMethod
	if e.KeyFile != "" {
		raw, err := os.ReadFile(e.KeyFile)
		if err != nil {
			return nil, err
		}
		var signer ssh.Signer
		if e.KeyPassphraseEnv != "" {
			value, ok := os.LookupEnv(e.KeyPassphraseEnv)
			if !ok {
				return nil, errors.New("key passphrase environment variable is unset")
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase(raw, []byte(value))
		} else {
			signer, err = ssh.ParsePrivateKey(raw)
		}
		if err != nil {
			return nil, fmt.Errorf("parse SSH key (encrypted keys require keyPassphraseEnv or an agent): %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if e.KeyFile == "" {
		r.agent, err = dialAgent(ctx)
		if err != nil {
			return nil, fmt.Errorf("SSH agent unavailable; configure a keyFile or start an agent: %w", err)
		}
		auth = append(auth, ssh.PublicKeysCallback(agent.NewClient(r.agent).Signers))
	}
	conn, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", e.Address)
	if err != nil {
		r.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	clientConfig := &ssh.ClientConfig{User: e.User, Auth: auth, HostKeyCallback: verify}
	if fingerprint != "" {
		clientConfig.HostKeyAlgorithms = []string{ssh.KeyAlgoED25519}
	}
	cc, ch, req, err := ssh.NewClientConn(conn, e.Address, clientConfig)
	if err != nil {
		conn.Close()
		r.Close()
		return nil, fmt.Errorf("SSH handshake failed: %w", err)
	}
	_ = conn.SetDeadline(time.Time{})
	r.SSH = ssh.NewClient(cc, ch, req)
	r.cancel = context.AfterFunc(ctx, func() { r.SSH.Close() })
	r.SFTP, err = sftp.NewClient(r.SSH)
	if err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}
func (r *Remote) Close() {
	if r.cancel != nil {
		r.cancel()
	}
	if r.SFTP != nil {
		r.SFTP.Close()
	}
	if r.SSH != nil {
		r.SSH.Close()
	}
	if r.agent != nil {
		r.agent.Close()
	}
}
func (r *Remote) remote(parts ...string) string {
	return path.Join(append([]string{r.Endpoint.RemoteDir}, parts...)...)
}
func (r *Remote) ReadJSON(rel string, v any) error {
	f, err := r.SFTP.Open(r.remote(rel))
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > MaxFileBytes {
		return errors.New("remote JSON too large")
	}
	return json.Unmarshal(raw, v)
}
func (r *Remote) State(alias string) (State, error) {
	var s State
	err := r.ReadJSON("state.json", &s)
	if err == nil && s.Version != Protocol {
		err = VersionError(s.Version)
	}
	if err == nil && s.Target != alias {
		err = errors.New("endpoint points at a different server")
	}
	return s, err
}
func (r *Remote) Ready(alias string) (State, error) {
	s, err := r.State(alias)
	if err != nil {
		return s, err
	}
	var heartbeat struct {
		Version int       `json:"version"`
		Updated time.Time `json:"updated"`
	}
	if err = r.ReadJSON("heartbeat.json", &heartbeat); err != nil {
		return s, err
	}
	if heartbeat.Version != Protocol {
		return s, VersionError(heartbeat.Version)
	}
	if time.Since(heartbeat.Updated) > 15*time.Second || time.Until(heartbeat.Updated) > 15*time.Second {
		return s, errors.New("agent heartbeat is stale or clocks differ; check the Windows service and time synchronization")
	}
	return s, nil
}
func (r *Remote) put(rel string, raw []byte, verify bool) error {
	dest := r.remote(rel)
	tmp := dest + ".upload-" + NewID()
	if err := r.rejectRemoteLinks(path.Dir(dest)); err != nil {
		return err
	}
	if err := r.SFTP.MkdirAll(path.Dir(dest)); err != nil {
		return err
	}
	f, err := r.SFTP.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer r.SFTP.Remove(tmp)
	_, writeErr := io.Copy(f, bytes.NewReader(raw))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if verify {
		f, err := r.SFTP.Open(tmp)
		if err != nil {
			return err
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(f, int64(len(raw))+1))
		f.Close()
		if err != nil {
			return err
		}
		if n != int64(len(raw)) || hex.EncodeToString(h.Sum(nil)) != Digest(raw) {
			return errors.New("remote upload hash mismatch")
		}
	}
	return r.SFTP.Rename(tmp, dest)
}

func (r *Remote) rejectRemoteLinks(root string) error {
	prefix := "/"
	for _, part := range strings.Split(path.Clean(root), "/") {
		if part == "" {
			continue
		}
		if len(part) == 2 && part[1] == ':' {
			prefix = "/" + part + "/"
		} else {
			prefix = path.Join(prefix, part)
		}
		fi, err := r.SFTP.Lstat(prefix)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			return fmt.Errorf("remote upload ancestor is a link or not a directory: %s", prefix)
		}
	}
	return nil
}
func (r *Remote) Submit(req Request, raw []byte) error {
	if req.Version != Protocol || !ValidID(req.ID) {
		return errors.New("invalid request identity")
	}
	if _, err := r.Ready(req.Target); err != nil {
		return err
	}
	if req.Action == "deploy" {
		if Digest(raw) != req.PackageSHA256 {
			return errors.New("request package hash mismatch")
		}
		if err := r.put("inbox/"+req.ID+".zip", raw, true); err != nil {
			return err
		}
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return r.put("inbox/"+req.ID+".json", encoded, true)
}
func (r *Remote) Wait(ctx context.Context, id string, progress func(Result)) (Result, error) {
	if !ValidID(id) {
		return Result{}, errors.New("invalid operation ID")
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	last := ""
	for {
		var result Result
		err := r.ReadJSON("results/"+id+".json", &result)
		if err == nil {
			if result.Version != Protocol {
				return result, VersionError(result.Version)
			}
			if result.ID != id {
				return result, errors.New("result ID mismatch")
			}
			if result.Phase != last {
				last = result.Phase
				if progress != nil {
					progress(result)
				}
			}
			if result.Status == "succeeded" {
				return result, nil
			}
			if result.Status == "failed" {
				return result, errors.New(result.Error)
			}
		} else if !os.IsNotExist(err) {
			return result, fmt.Errorf("operation %s may still be running; reconnect with status --operation: %w", id, err)
		}
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("operation %s may still be running; reconnect with status --operation: %w", id, ctx.Err())
		case <-ticker.C:
		}
	}
}

type Preview struct {
	Provision bool     `json:"provision,omitempty"`
	Target    string   `json:"target"`
	Commit    string   `json:"commit"`
	Add       []string `json:"add"`
	Replace   []string `json:"replace"`
	Delete    []string `json:"delete"`
	Restart   bool     `json:"restart"`
	Warnings  int      `json:"warnings"`
}

func (r *Remote) Preview(b *Bundle, restart bool) (Preview, error) {
	s, err := r.State(b.Manifest.Target)
	p := Preview{Target: b.Manifest.Target, Commit: b.Manifest.Commit, Restart: restart}
	if err != nil {
		return p, err
	}
	var host HostConfig
	if err = r.ReadJSON("host.json", &host); err != nil {
		return p, err
	}
	current := map[string]bool{}
	for _, f := range b.Manifest.Files {
		current[f.Destination] = true
		// Remote installation paths use an explicit drive prefix on Windows SFTP.
		root := "/" + strings.TrimPrefix(strings.ReplaceAll(host.InstallDir, "\\", "/"), "/")
		dest := path.Join(root, f.Destination)
		fi, err := r.SFTP.Lstat(dest)
		if os.IsNotExist(err) {
			p.Add = append(p.Add, f.Destination)
			continue
		}
		if err != nil {
			return p, err
		}
		if !fi.Mode().IsRegular() {
			return p, fmt.Errorf("remote managed path is not a regular file: %s", f.Destination)
		}
		fRemote, err := r.SFTP.Open(dest)
		if err != nil {
			return p, err
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(fRemote, MaxFileBytes+1))
		fRemote.Close()
		if err != nil {
			return p, err
		}
		if n > MaxFileBytes {
			return p, errors.New("remote managed file too large")
		}
		if hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
			p.Replace = append(p.Replace, f.Destination)
		}
	}
	if s.Installed != nil {
		for _, f := range s.Installed.Files {
			if !current[f.Destination] {
				p.Delete = append(p.Delete, f.Destination)
			}
		}
	}
	return p, nil
}
