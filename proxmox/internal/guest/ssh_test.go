package guest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestMACSelectionAndStaticAddress(t *testing.T) {
	ns := []Interface{{MAC: "02:00:00:00:00:01", Addresses: []Address{{IP: "192.0.2.10"}}}, {MAC: "02:00:00:00:00:02", Addresses: []Address{{IP: "169.254.1.1"}, {IP: "198.51.100.20"}}}}
	got, e := SelectAddress(ns, "02:00:00:00:00:02", "dhcp")
	if e != nil || got != "198.51.100.20" {
		t.Fatal(got, e)
	}
	if _, e = SelectAddress(ns, "02:00:00:00:00:02", "198.51.100.21/24"); e == nil {
		t.Fatal("wrong static address accepted")
	}
	ns[1].Addresses = append(ns[1].Addresses, Address{IP: "198.51.100.22"})
	if _, e = SelectAddress(ns, "02:00:00:00:00:02", "dhcp"); e == nil {
		t.Fatal("ambiguous addresses accepted")
	}
}
func signer(t *testing.T) ssh.Signer {
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	s, e := ssh.NewSignerFromKey(key)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestPinnedSSHAuthenticationAndCommand(t *testing.T) {
	host, user := signer(t), signer(t)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
		if string(k.Marshal()) != string(user.PublicKey().Marshal()) {
			return nil, ssh.ErrNoAuth
		}
		return nil, nil
	}}
	cfg.AddHostKey(host)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, e := l.Accept()
			if e != nil {
				return
			}
			go func() {
				defer conn.Close()
				sc, channels, requests, e := ssh.NewServerConn(conn, cfg)
				if e != nil {
					return
				}
				defer sc.Close()
				go ssh.DiscardRequests(requests)
				for ch := range channels {
					c, reqs, e := ch.Accept()
					if e != nil {
						continue
					}
					go func() {
						defer c.Close()
						for r := range reqs {
							if r.Type == "exec" {
								_ = r.Reply(true, nil)
								_, _ = c.Write([]byte("pvecli-ready\n"))
								_, _ = c.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
								return
							}
							_ = r.Reply(false, nil)
						}
					}()
				}
			}()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client, e := Dial(ctx, l.Addr().String(), "test", user, host.PublicKey())
	if e != nil {
		t.Fatal(e)
	}
	out, e := Command(ctx, client, "echo ready")
	client.Close()
	if e != nil || out != "pvecli-ready\n" {
		t.Fatal(out, e)
	}
	if bad, e := Dial(ctx, l.Addr().String(), "test", user, signer(t).PublicKey()); e == nil {
		bad.Close()
		t.Fatal("host-key mismatch accepted")
	}
	l.Close()
	<-done
}
