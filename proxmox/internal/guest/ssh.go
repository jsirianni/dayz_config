// Package guest verifies SSH against a key obtained from the authenticated
// hypervisor channel, never from an unauthenticated network key scan.
package guest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"github.com/jsirianni/dayz_config/proxmox/internal/vm"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Access struct{ Endpoint, Fingerprint string }

// Checker permits engine tests to replace guest I/O without bypassing API tests.
type Checker interface {
	Ready(context.Context, *pve.Client, string, int, spec.Network, spec.Guest) (Access, error)
}
type SSH struct{ Poll time.Duration }

type Interface struct {
	Name      string    `json:"name"`
	MAC       string    `json:"hardware-address"`
	Addresses []Address `json:"ip-addresses"`
}
type Address struct {
	IP   string `json:"ip-address"`
	Type string `json:"ip-address-type"`
}

func Signer(g spec.Guest) (ssh.Signer, error) {
	b, e := os.ReadFile(g.KeyFile)
	if e != nil {
		return nil, e
	}
	if g.PassphraseEnv != "" {
		p, ok := os.LookupEnv(g.PassphraseEnv)
		if !ok {
			return nil, errors.New("SSH passphrase environment variable is unset")
		}
		return ssh.ParsePrivateKeyWithPassphrase(b, []byte(p))
	}
	return ssh.ParsePrivateKey(b)
}
func PublicKey(g spec.Guest) (string, error) {
	s, e := Signer(g)
	if e != nil {
		return "", e
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(s.PublicKey()))), nil
}
func SelectAddress(interfaces []Interface, mac, expected string) (string, error) {
	want := ""
	if expected != "dhcp" {
		ip, _, e := net.ParseCIDR(expected)
		if e != nil {
			return "", e
		}
		want = ip.String()
	}
	set := map[string]bool{}
	for _, n := range interfaces {
		if !strings.EqualFold(n.MAC, mac) {
			continue
		}
		for _, a := range n.Addresses {
			ip := net.ParseIP(a.IP)
			if ip == nil || ip.To4() == nil || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() {
				continue
			}
			if want != "" && ip.String() != want {
				continue
			}
			set[ip.String()] = true
		}
	}
	if len(set) != 1 {
		return "", fmt.Errorf("expected one usable IPv4 address on MAC %s, found %d", mac, len(set))
	}
	for ip := range set {
		return ip, nil
	}
	panic("unreachable")
}
func (s SSH) Connect(ctx context.Context, c *pve.Client, node string, id int, n spec.Network, g spec.Guest) (*ssh.Client, Access, error) {
	signer, e := Signer(g)
	if e != nil {
		return nil, Access{}, e
	}
	cfg, e := c.VMConfig(ctx, node, id)
	if e != nil {
		return nil, Access{}, e
	}
	_, mac, e := vm.Model(cfg.String(n.Interface))
	if e != nil {
		return nil, Access{}, e
	}
	poll := s.Poll
	if poll <= 0 {
		poll = 2 * time.Second
	}
	last := errors.New("guest not yet ready")
	for {
		var r struct {
			Result []Interface `json:"result"`
		}
		e = c.Do(ctx, "GET", pve.Path(node, id)+"/agent/network-get-interfaces", nil, &r)
		if e == nil {
			var ip string
			ip, e = SelectAddress(r.Result, mac, n.IPv4)
			if e == nil {
				path := "/etc/ssh/ssh_host_ed25519_key.pub"
				if strings.HasPrefix(g.OS, "windows") {
					path = `C:\ProgramData\ssh\ssh_host_ed25519_key.pub`
				}
				var keyText string
				keyText, e = c.AgentFile(ctx, node, id, path)
				if e == nil {
					var key ssh.PublicKey
					key, _, _, _, e = ssh.ParseAuthorizedKey([]byte(keyText))
					if e == nil {
						addr := net.JoinHostPort(ip, strconv.Itoa(g.Port))
						var client *ssh.Client
						client, e = Dial(ctx, addr, g.User, signer, key)
						if e == nil {
							return client, Access{Endpoint: g.User + "@" + addr, Fingerprint: ssh.FingerprintSHA256(key)}, nil
						}
					}
				}
			}
		}
		last = e
		if e != nil && strings.Contains(e.Error(), "host key mismatch") {
			return nil, Access{}, e
		}
		var api *pve.APIError
		if errors.As(e, &api) && (api.Status == 401 || api.Status == 403) {
			return nil, Access{}, e
		}
		if e = pve.Delay(ctx, poll); e != nil {
			return nil, Access{}, fmt.Errorf("guest SSH readiness: %v: %w", last, e)
		}
	}
}
func Dial(ctx context.Context, addr, user string, signer ssh.Signer, host ssh.PublicKey) (*ssh.Client, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, e := d.DialContext(ctx, "tcp", addr)
	if e != nil {
		return nil, e
	}
	deadline := time.Now().Add(15 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	cc, ch, req, e := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.FixedHostKey(host)})
	if e != nil {
		conn.Close()
		return nil, e
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(cc, ch, req), nil
}
func Command(ctx context.Context, c *ssh.Client, command string) (string, error) {
	s, e := c.NewSession()
	if e != nil {
		return "", e
	}
	defer s.Close()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	var out cappedBuffer
	s.Stdout = &out
	s.Stderr = &out
	e = s.Run(command)
	if ctx.Err() != nil {
		return out.String(), ctx.Err()
	}
	return out.String(), e
}

type cappedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if b.buf.Len() < 65536 {
		limit := 65536 - b.buf.Len()
		if len(p) > limit {
			p = p[:limit]
		}
		_, _ = b.buf.Write(p)
	}
	return n, nil
}
func (b *cappedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.String() }
func (s SSH) Ready(ctx context.Context, c *pve.Client, node string, id int, n spec.Network, g spec.Guest) (Access, error) {
	client, access, e := s.Connect(ctx, c, node, id, n, g)
	if e != nil {
		return access, e
	}
	defer client.Close()
	command := "printf 'pvecli-ready\\n'"
	if strings.HasPrefix(g.OS, "windows") {
		command = `powershell.exe -NoLogo -NoProfile -NonInteractive -Command "Write-Output 'pvecli-ready'"`
	}
	out, e := Command(ctx, client, command)
	if e != nil {
		return access, fmt.Errorf("authenticated SSH command failed: %w", e)
	}
	if strings.TrimSpace(out) != "pvecli-ready" {
		return access, errors.New("unexpected SSH readiness response")
	}
	return access, nil
}
func Upload(ctx context.Context, c *ssh.Client, path string, content []byte) error {
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	s, e := sftp.NewClient(c)
	if e != nil {
		return e
	}
	defer s.Close()
	f, e := s.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if e != nil {
		return e
	}
	_, e = io.Copy(f, bytes.NewReader(content))
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
