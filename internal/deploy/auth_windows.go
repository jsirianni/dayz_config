//go:build windows

package deploy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func authPipe(c HostConfig) string { return `\\.\pipe\dzdeploy-auth-` + c.Target }

func startAuthBroker(ctx context.Context, c HostConfig) (func(), error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	listener, err := winio.ListenPipe(authPipe(c), &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;" + user.User.Sid.String() + ")", InputBufferSize: 8192, OutputBufferSize: 8192})
	if err != nil {
		return nil, err
	}
	brokerCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	var active sync.Mutex
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func(conn net.Conn) {
				defer wg.Done()
				defer conn.Close()
				if !active.TryLock() {
					_ = sendAuth(conn, "error", "")
					return
				}
				defer active.Unlock()
				authCtx, stop := context.WithTimeout(brokerCtx, 10*time.Minute)
				defer stop()
				closeOnCancel := context.AfterFunc(authCtx, func() { conn.Close() })
				defer closeOnCancel()
				if deadline, ok := authCtx.Deadline(); ok {
					_ = conn.SetDeadline(deadline)
				}
				if err := steamAuthenticate(authCtx, c, conn); err != nil {
					_ = sendAuth(conn, "error", "")
				}
			}(conn)
		}
	}()
	return func() { cancel(); listener.Close(); wg.Wait() }, nil
}

func relayAuthPipe(ctx context.Context, c HostConfig, in io.Reader, out io.Writer) error {
	conn, err := winio.DialPipeContext(ctx, authPipe(c))
	if err != nil {
		return errors.New("service authentication pipe unavailable; check agent status")
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	go func() { _, _ = io.Copy(conn, in); conn.Close() }()
	_, err = io.Copy(out, conn)
	return err
}

func steamAuthenticate(ctx context.Context, c HostConfig, conn io.ReadWriter) error {
	unlock, err := acquireSteam(ctx, c)
	if err != nil {
		return err
	}
	defer unlock()
	cmd := exec.CommandContext(ctx, c.SteamCMD, "+login", c.SteamUser, "+quit")
	cmd.Dir = filepath.Dir(c.SteamCMD)
	prepareCommand(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer reader.Close()
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err = cmd.Start(); err != nil {
		writer.Close()
		return err
	}
	writer.Close()
	// Always terminate/reap an interrupted authentication process. No game process
	// is affected; credentials are never passed to the updater's log writer.
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if err = sendAuth(conn, "status", ""); err != nil {
		return err
	}
	input := bufio.NewReaderSize(conn, 8192)
	dialog := steamDialog{}
	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			kind := dialog.consume(string(buf[:n]))
			if kind != "" {
				if err = sendAuth(conn, kind, ""); err != nil {
					return err
				}
				if kind == "password" || kind == "guard" {
					answer, err := readAuthFrame(input)
					if err != nil {
						return err
					}
					if answer.Type != "answer" || answer.Value == "" || len(answer.Value) > 4096 || strings.ContainsAny(answer.Value, "\r\n\x00") {
						return errors.New("invalid authentication answer")
					}
					if _, err = io.WriteString(stdin, answer.Value+"\n"); err != nil {
						return err
					}
					answer.Value = ""
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err = cmd.Wait(); err != nil {
		return errors.New("SteamCMD login failed")
	}
	if !dialog.success {
		return errors.New("SteamCMD did not confirm a successful login")
	}
	return sendAuth(conn, "ready", "")
}
