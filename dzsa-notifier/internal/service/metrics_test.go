package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestShutdownAllowsInflightScrapeToFinish(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	releaseRequest := func() { once.Do(func() { close(release) }) }
	t.Cleanup(releaseRequest)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "scrape completed")
	})}
	t.Cleanup(func() { _ = server.Close() })
	serving := serveMetrics(server, listener)
	requestDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			_, err = io.ReadAll(response.Body)
			_ = response.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("scrape did not start")
	}
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- shutdownMetrics(server, time.Second) }()
	// Serve returning proves Shutdown closed the listener before we release the
	// in-flight request; Shutdown must still wait for that request to complete.
	if err := <-serving; err != http.ErrServerClosed {
		t.Fatal(err)
	}
	select {
	case err := <-shutdownDone:
		t.Fatalf("shutdown did not wait for scrape: %v", err)
	default:
	}
	releaseRequest()
	if err := awaitResult(t, requestDone); err != nil {
		t.Fatal(err)
	}
	if err := awaitResult(t, shutdownDone); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownDeadlineForcesConnectionClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finished := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(finished)
	})}
	t.Cleanup(func() { _ = server.Close() })
	serving := serveMetrics(server, listener)
	requestDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			_ = response.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("scrape did not start")
	}
	if err := shutdownMetrics(server, 20*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	<-serving
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("handler not canceled by forced close")
	}
	if err := awaitResult(t, requestDone); err == nil {
		t.Fatal("overdue request was not closed")
	}
}
