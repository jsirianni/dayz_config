package deploy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func sshFixture(t *testing.T) (Endpoint, string) {
	t.Helper()
	root := t.TempDir()
	_, clientKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "id_ed25519")
	if err = os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if string(key.Marshal()) != string(signer.PublicKey().Marshal()) {
			return nil, fmt.Errorf("unexpected key")
		}
		return nil, nil
	}}
	config.AddHostKey(hostSigner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	connections := []net.Conn{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			connections = append(connections, conn)
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				_, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(requests)
				for ch := range channels {
					if ch.ChannelType() != "session" {
						_ = ch.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, requests, err := ch.Accept()
					if err != nil {
						return
					}
					wg.Add(1)
					go func() {
						defer wg.Done()
						defer channel.Close()
						for req := range requests {
							if req.Type != "subsystem" {
								_ = req.Reply(false, nil)
								continue
							}
							var name struct{ Value string }
							_ = ssh.Unmarshal(req.Payload, &name)
							if name.Value != "sftp" {
								_ = req.Reply(false, nil)
								continue
							}
							_ = req.Reply(true, nil)
							server, err := sftp.NewServer(channel)
							if err == nil {
								_ = server.Serve()
								server.Close()
							}
							return
						}
					}()
				}
			}()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		mu.Lock()
		for _, c := range connections {
			c.Close()
		}
		mu.Unlock()
		wg.Wait()
	})
	known := filepath.Join(t.TempDir(), "known_hosts")
	if err = os.WriteFile(known, []byte(knownhosts.Line([]string{listener.Addr().String()}, hostSigner.PublicKey())+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	remoteRoot := filepath.ToSlash(root)
	if !strings.HasPrefix(remoteRoot, "/") {
		remoteRoot = "/" + remoteRoot
	}
	if err = WriteJSON(filepath.Join(root, "state.json"), State{Version: Protocol, Target: "nasdara-staging"}); err != nil {
		t.Fatal(err)
	}
	if err = WriteJSON(filepath.Join(root, "heartbeat.json"), map[string]any{"version": Protocol, "updated": time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	return Endpoint{Address: listener.Addr().String(), User: "deploy", KeyFile: keyFile, KnownHosts: known, RemoteDir: remoteRoot}, root
}
func TestSFTPSubmissionIsDurableAfterDisconnect(t *testing.T) {
	endpoint, root := sshFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	remote, err := Connect(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	b := testBundle(t, nil)
	raw, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Version: Protocol, ID: b.Manifest.ID, Target: b.Manifest.Target, Action: "deploy", PackageSHA256: Digest(raw)}
	if err = remote.Submit(req, raw); err != nil {
		remote.Close()
		t.Fatal(err)
	}
	remote.Close()
	actual, err := os.ReadFile(filepath.Join(root, "inbox", req.ID+".zip"))
	if err != nil || Digest(actual) != req.PackageSHA256 {
		t.Fatal("package lost after disconnect", err)
	}
	var saved Request
	if err = ReadJSON(filepath.Join(root, "inbox", req.ID+".json"), &saved); err != nil || saved.ID != req.ID {
		t.Fatal("durable request missing", err)
	}
	r := Result{Version: Protocol, ID: req.ID, Target: req.Target, Status: "succeeded", Phase: "complete"}
	if err = WriteJSON(filepath.Join(root, "results", req.ID+".json"), r); err != nil {
		t.Fatal(err)
	}
	remote, err = Connect(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	got, err := remote.Wait(ctx, req.ID, nil)
	if err != nil || got.Status != "succeeded" {
		t.Fatal(got, err)
	}
}
func TestSSHRejectsUnpinnedHost(t *testing.T) {
	endpoint, _ := sshFixture(t)
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := ssh.NewSignerFromKey(key)
	if err = os.WriteFile(endpoint.KnownHosts, []byte(knownhosts.Line([]string{endpoint.Address}, signer.PublicKey())+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if remote, err := Connect(context.Background(), endpoint); err == nil {
		remote.Close()
		t.Fatal("mismatched host key accepted")
	}
}
func TestRequestNotPublishedWhenUploadFails(t *testing.T) {
	endpoint, root := sshFixture(t)
	if err := os.MkdirAll(filepath.Join(root, "inbox"), 0700); err != nil {
		t.Fatal(err)
	}
	b := testBundle(t, nil)
	raw, _ := b.Encode()
	if err := os.Mkdir(filepath.Join(root, "inbox", b.Manifest.ID+".zip"), 0700); err != nil {
		t.Fatal(err)
	}
	remote, err := Connect(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	req := Request{Version: Protocol, ID: b.Manifest.ID, Target: b.Manifest.Target, Action: "deploy", PackageSHA256: Digest(raw)}
	if err = remote.Submit(req, raw); err == nil {
		t.Fatal("upload collision accepted")
	}
	if _, err = os.Stat(filepath.Join(root, "inbox", req.ID+".json")); !os.IsNotExist(err) {
		t.Fatal("request published without package")
	}
}
func FuzzDecode(f *testing.F) {
	b := []byte("not a zip")
	f.Add(b)
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		_, _ = Decode(data, "nasdara-staging")
	})
}
