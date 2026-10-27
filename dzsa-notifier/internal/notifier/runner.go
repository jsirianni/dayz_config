package notifier

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Checker interface {
	Check(context.Context, string) (*Server, time.Time, error)
}

type Runner struct {
	Config      Config
	Client      Checker
	Log         *slog.Logger
	Registry    *prometheus.Registry
	attempts    *prometheus.CounterVec
	failures    *prometheus.CounterVec
	success     *prometheus.GaugeVec
	lastSuccess *prometheus.GaugeVec
	duration    *prometheus.HistogramVec
	deferUntil  time.Time
	// The next cycle begins at the deferred endpoint, preventing rate-limit starvation.
	next int
}

func NewRunner(c Config, client Checker, log *slog.Logger) *Runner {
	r := &Runner{Config: c, Client: client, Log: log, Registry: prometheus.NewRegistry()}
	r.attempts = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "dzsa_notifier_attempts_total", Help: "DZSA notification attempts."}, []string{"endpoint"})
	r.failures = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "dzsa_notifier_failures_total", Help: "Failed DZSA notification attempts."}, []string{"endpoint"})
	r.success = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "dzsa_notifier_last_attempt_success", Help: "Whether the last attempt succeeded (0 before the first attempt)."}, []string{"endpoint"})
	r.lastSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "dzsa_notifier_last_success_timestamp_seconds", Help: "Unix timestamp of last success, or 0 if none."}, []string{"endpoint"})
	r.duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "dzsa_notifier_request_duration_seconds", Help: "DZSA notification request duration."}, []string{"endpoint"})
	r.Registry.MustRegister(r.attempts, r.failures, r.success, r.lastSuccess, r.duration, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	for _, endpoint := range c.Endpoints {
		r.attempts.WithLabelValues(endpoint)
		r.failures.WithLabelValues(endpoint)
		r.success.WithLabelValues(endpoint).Set(0)
		r.lastSuccess.WithLabelValues(endpoint).Set(0)
	}
	return r
}

func (r *Runner) Cycle(ctx context.Context) error {
	if time.Now().Before(r.deferUntil) {
		r.Log.Info("cycle deferred by DZSA", "retry_at", r.deferUntil)
		return errors.New("DZSA Retry-After is still active")
	}
	attempted, failed := 0, 0
	start := r.next
	for i := 0; i < len(r.Config.Endpoints); i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		index := (start + i) % len(r.Config.Endpoints)
		endpoint := r.Config.Endpoints[index]
		began := time.Now()
		server, until, err := r.Client.Check(ctx, endpoint)
		attempted++
		r.attempts.WithLabelValues(endpoint).Inc()
		r.duration.WithLabelValues(endpoint).Observe(time.Since(began).Seconds())
		if err != nil {
			failed++
			r.failures.WithLabelValues(endpoint).Inc()
			r.success.WithLabelValues(endpoint).Set(0)
			r.Log.Error("notification failed", "endpoint", endpoint, "error", err)
		} else {
			r.success.WithLabelValues(endpoint).Set(1)
			r.lastSuccess.WithLabelValues(endpoint).SetToCurrentTime()
			r.Log.Info("server confirmed by DZSA", "endpoint", endpoint, "server_name", server.Name, "mod_count", len(server.Mods))
		}
		r.next = (index + 1) % len(r.Config.Endpoints)
		if until.After(time.Now()) {
			r.deferUntil = until
			break
		}
	}
	r.Log.Info("cycle complete", "attempted", attempted, "failed", failed, "configured", len(r.Config.Endpoints))
	if failed > 0 || attempted != len(r.Config.Endpoints) {
		return errors.New("one or more endpoints failed or were deferred")
	}
	return nil
}

// Run performs an immediate cycle. Overruns skip scheduled slots, never queue work.
func (r *Runner) Run(ctx context.Context) {
	next := time.Now()
	for ctx.Err() == nil {
		_ = r.Cycle(ctx)
		next = nextSlot(next, time.Now(), r.Config.Interval)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func nextSlot(previous, now time.Time, interval time.Duration) time.Time {
	return previous.Add((now.Sub(previous)/interval + 1) * interval)
}
