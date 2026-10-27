package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsShutdownTimeout = 5 * time.Second

func newMetricsServer(metrics prometheus.Gatherer) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metrics, promhttp.HandlerOpts{}))
	return &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}

func serveMetrics(server *http.Server, listener net.Listener) <-chan error {
	serving := make(chan error, 1)
	go func() {
		defer close(serving)
		serving <- server.Serve(listener)
	}()
	return serving
}

func shutdownMetrics(server *http.Server, timeout time.Duration) error {
	// The service context is canceled by this point, so give in-flight scrapes an
	// independent grace period, then force-close overdue connections.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return fmt.Errorf("metrics shutdown: %w", err)
	}
	return nil
}
