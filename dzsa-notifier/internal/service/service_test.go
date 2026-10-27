package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/prometheus/client_golang/prometheus"
)

func testService(worker func(context.Context)) *Service {
	s := New("127.0.0.1:0", prometheus.NewRegistry(), worker, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	s.notify = func(string) error { return nil }
	return s
}

func awaitResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("service did not return and join its goroutines")
		return nil
	}
}

func TestCancellationStopsWorkerAndClosesMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	started := make(chan struct{})
	joined := make(chan struct{})
	var ready atomic.Bool
	s := testService(func(ctx context.Context) {
		if !ready.Load() {
			t.Error("worker started before readiness")
		}
		close(started)
		<-ctx.Done()
		close(joined)
	})
	bound := make(chan net.Listener, 1)
	s.listen = func(network, address string) (net.Listener, error) {
		listener, err := net.Listen(network, address)
		bound <- listener
		return listener, err
	}
	var states []string
	s.notify = func(state string) error {
		states = append(states, state)
		if state == daemon.SdNotifyReady {
			ready.Store(true)
		}
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	listener := <-bound
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start")
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + listener.Addr().String() + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.Status)
	}
	cancel()
	if err := awaitResult(t, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-joined:
	default:
		t.Fatal("worker not joined")
	}
	if strings.Join(states, ",") != daemon.SdNotifyReady+","+daemon.SdNotifyStopping {
		t.Fatal(states)
	}
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err == nil {
		_ = connection.Close()
		t.Fatal("metrics listener still open")
	}
}

func TestBindFailureDoesNotStartWorkerOrNotify(t *testing.T) {
	failure := errors.New("address in use")
	s := testService(func(context.Context) { t.Error("worker started after bind failure") })
	s.listen = func(string, string) (net.Listener, error) { return nil, failure }
	s.notify = func(string) error { t.Error("readiness after bind failure"); return nil }
	if err := s.Run(context.Background()); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}

func TestReadinessFailureClosesMetricsWithoutStartingWorker(t *testing.T) {
	failure := errors.New("notify socket unavailable")
	s := testService(func(context.Context) { t.Error("worker started after readiness failure") })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	s.listen = func(string, string) (net.Listener, error) { return listener, nil }
	s.notify = func(string) error { return failure }
	if err := s.Run(context.Background()); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		_ = connection.Close()
		t.Fatal("listener leaked on readiness failure")
	}
}

func TestMetricsFailureCancelsAndJoinsWorker(t *testing.T) {
	joined := make(chan struct{})
	started := make(chan struct{})
	s := testService(func(ctx context.Context) { close(started); <-ctx.Done(); close(joined) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	s.listen = func(string, string) (net.Listener, error) { return listener, nil }
	done := make(chan error, 1)
	go func() { done <- s.Run(context.Background()) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start")
	}
	_ = listener.Close()
	if err := awaitResult(t, done); err == nil || !strings.Contains(err.Error(), "metrics server") {
		t.Fatal(err)
	}
	select {
	case <-joined:
	default:
		t.Fatal("worker leaked on metrics failure")
	}
}

func TestUnexpectedWorkerExitStopsService(t *testing.T) {
	s := testService(func(context.Context) {})
	done := make(chan error, 1)
	go func() { done <- s.Run(context.Background()) }()
	if err := awaitResult(t, done); err == nil || !strings.Contains(err.Error(), "worker stopped unexpectedly") {
		t.Fatal(err)
	}
}

func TestAlreadyCanceledContextDoesNotStartResources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := testService(func(context.Context) { t.Error("worker started") })
	s.listen = func(string, string) (net.Listener, error) {
		t.Error("listener opened")
		return nil, errors.New("unexpected listen")
	}
	s.notify = func(string) error { t.Error("readiness notified"); return nil }
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSystemdNotificationWithoutSocket(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "")
	if err := notifySystemd(daemon.SdNotifyReady); err != nil {
		t.Fatal(err)
	}
}
