// Package service owns the notifier's long-running worker and metrics lifecycle.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/prometheus/client_golang/prometheus"
)

// Service runs a worker alongside its metrics endpoint until cancellation or a
// runtime failure. It owns and joins both goroutines before returning.
type Service struct {
	address string
	metrics prometheus.Gatherer
	worker  func(context.Context)
	logger  *slog.Logger
	listen  func(string, string) (net.Listener, error)
	notify  func(string) error
}

func New(address string, metrics prometheus.Gatherer, worker func(context.Context), logger *slog.Logger) *Service {
	return &Service{
		address: address,
		metrics: metrics,
		worker:  worker,
		logger:  logger,
		listen:  net.Listen,
		notify:  notifySystemd,
	}
}

// Run treats caller cancellation as a normal stop. Runtime failures are returned
// to the command so the supervisor can restart the process.
func (s *Service) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	listener, err := s.listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("metrics listen: %w", err)
	}
	server := newMetricsServer(s.metrics)
	serving := serveMetrics(server, listener)
	defer func() {
		_ = server.Close()
		<-serving
	}()
	if err := s.notify(daemon.SdNotifyReady); err != nil {
		return fmt.Errorf("systemd readiness notification: %w", err)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		s.worker(workerCtx)
	}()
	s.logger.Info("notifier started", "metrics_listen", listener.Addr().String())
	err = wait(ctx, serving, workerDone)
	if notifyErr := s.notify(daemon.SdNotifyStopping); notifyErr != nil {
		s.logger.Warn("systemd stopping notification failed", "error", notifyErr)
	}
	cancel()
	<-workerDone
	err = errors.Join(err, shutdownMetrics(server, metricsShutdownTimeout))
	s.logger.Info("notifier stopped")
	return err
}

func wait(ctx context.Context, serving <-chan error, workerDone <-chan struct{}) error {
	select {
	case <-ctx.Done():
		return nil
	case err := <-serving:
		return fmt.Errorf("metrics server stopped: %w", err)
	case <-workerDone:
		if ctx.Err() != nil {
			return nil
		}
		return errors.New("notification worker stopped unexpectedly")
	}
}
