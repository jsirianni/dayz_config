//go:build windows

// This executable is an isolated integration fixture, never a production agent.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsirianni/dayz_config/internal/deploy"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

type fixtureConfig struct {
	Host    deploy.HostConfig
	URL     string
	CA      string
	Service string
}
type fixtureGame struct {
	config fixtureConfig
	native deploy.NativeGame
}

func (g *fixtureGame) Alive(s deploy.State) bool { return g.native.Alive(s) }
func (g *fixtureGame) Stop(ctx context.Context, s deploy.State) error {
	return g.native.HardStop(ctx, s)
}
func (g *fixtureGame) HardStop(ctx context.Context, s deploy.State) error {
	return g.native.HardStop(ctx, s)
}
func (g *fixtureGame) Update(context.Context, deploy.Target) error { return nil }
func (g *fixtureGame) Launch(t deploy.Target) (int, string, error) {
	child := exec.Command(filepath.Join(g.config.Host.InstallDir, "DayZServer_x64.exe"), "game", g.config.Host.QueryAddress)
	if err := child.Start(); err != nil {
		return 0, "", err
	}
	go func() { _ = child.Wait() }()
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(child.Process.Pid))
	if err != nil {
		return child.Process.Pid, "", err
	}
	defer windows.CloseHandle(h)
	var created, exited, kernel, user windows.Filetime
	err = windows.GetProcessTimes(h, &created, &exited, &kernel, &user)
	return child.Process.Pid, fmt.Sprintf("%d:%d", created.HighDateTime, created.LowDateTime), err
}
func (g *fixtureGame) Healthy(ctx context.Context, t deploy.Target) (*deploy.DZSAReport, error) {
	if _, err := g.native.Healthy(ctx, t); err != nil {
		return nil, err
	}
	cert, err := os.ReadFile(g.config.CA)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(cert) {
		return nil, errors.New("invalid fixture CA")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.config.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var envelope struct {
		Status int               `json:"status"`
		Result deploy.DZSAServer `json:"result"`
	}
	if response.StatusCode != 200 {
		return nil, errors.New("fixture query failed")
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return nil, err
	}
	if envelope.Status != 0 || envelope.Result.Game != "dayz" || envelope.Result.GamePort != t.GamePort {
		return nil, errors.New("fixture endpoint identity mismatch")
	}
	return &deploy.DZSAReport{Address: "203.0.113.10:27018", CheckedAt: time.Now().UTC(), Server: envelope.Result}, nil
}

type handler struct{ config fixtureConfig }

func (h *handler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	game := &fixtureGame{config: h.config, native: deploy.NativeGame{Config: h.config.Host}}
	go func() { done <- deploy.RunAgent(ctx, h.config.Host, game) }()
	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	status <- current
	for {
		select {
		case err := <-done:
			if err != nil {
				_ = os.WriteFile(filepath.Join(h.config.Host.StateDir, "fixture-error.txt"), []byte(err.Error()), 0600)
				return true, 1
			}
			return false, 0
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				status <- current
			case svc.Stop, svc.Shutdown:
				cancel()
				status <- svc.Status{State: svc.StopPending, WaitHint: 30000}
				select {
				case <-done:
					return false, 0
				case <-time.After(30 * time.Second):
					return true, 1
				}
			}
		}
	}
}

func runGame(address string) error {
	udp, err := net.ListenPacket("udp", address)
	if err != nil {
		return err
	}
	defer udp.Close()
	var reply bytes.Buffer
	reply.Write([]byte{255, 255, 255, 255, 73, 17})
	for _, s := range []string{"Test", "nasdara", "dayz", "DayZ"} {
		reply.WriteString(s)
		reply.WriteByte(0)
	}
	_ = binary.Write(&reply, binary.LittleEndian, uint16(24492))
	reply.Write([]byte{0, 60, 0, 'd', 'w', 0, 1})
	reply.WriteString("fixture-1.0")
	reply.Write([]byte{0, 0})
	b := make([]byte, 4096)
	for {
		_, peer, e := udp.ReadFrom(b)
		if e != nil {
			return e
		}
		if _, e = udp.WriteTo(reply.Bytes(), peer); e != nil {
			return e
		}
	}
}

func main() {
	if len(os.Args) == 3 && os.Args[1] == "game" {
		if err := runGame(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 2 {
		panic("fixture requires a protected configuration")
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var config fixtureConfig
	if err = json.Unmarshal(b, &config); err != nil {
		panic(err)
	}
	root := filepath.Dir(config.Host.StateDir)
	expected := filepath.Clean(os.Getenv("ProgramData"))
	if !strings.EqualFold(filepath.Dir(root), expected) || !strings.HasPrefix(filepath.Base(root), "dzdeploy-operator-integration-") {
		panic("fixture refuses non-disposable layout")
	}
	if _, err = os.Stat(filepath.Join(root, "fixture.marker")); err != nil {
		panic(err)
	}
	if err = config.Host.Validate(); err != nil {
		panic(err)
	}
	if !strings.HasPrefix(config.URL, "https://127.0.0.1:") {
		panic("fixture HTTPS must be loopback")
	}
	if err = svc.Run(config.Service, &handler{config}); err != nil {
		panic(err)
	}
}
