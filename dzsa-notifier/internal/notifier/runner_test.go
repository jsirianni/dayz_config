package notifier

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type checkFunc func(context.Context, string, time.Time) (*Server, time.Time, error)

func (f checkFunc) Check(c context.Context, s string, t time.Time) (*Server, time.Time, error) {
	return f(c, s, t)
}
func testRunner(c Checker) *Runner {
	return NewRunner(Config{Interval: 20 * time.Millisecond, Endpoints: []string{endpoint, "203.0.113.10:27019"}}, c, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

func TestCycleFailureDoesNotBlockOtherEndpointsAndMetrics(t *testing.T) {
	calls := 0
	r := testRunner(checkFunc(func(_ context.Context, e string, _ time.Time) (*Server, time.Time, error) {
		calls++
		if e == endpoint {
			return nil, time.Time{}, errors.New("offline")
		}
		return &Server{Name: "up"}, time.Time{}, nil
	}))
	if r.Cycle(context.Background()) == nil || calls != 2 {
		t.Fatal("cycle did not finish all endpoints")
	}
	if testutil.ToFloat64(r.attempts.WithLabelValues(endpoint)) != 1 || testutil.ToFloat64(r.failures.WithLabelValues(endpoint)) != 1 || testutil.ToFloat64(r.success.WithLabelValues("203.0.113.10:27019")) != 1 || testutil.ToFloat64(r.lastSuccess.WithLabelValues("203.0.113.10:27019")) == 0 {
		t.Fatal("incorrect metrics")
	}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		promhttp.HandlerFor(r.Registry, promhttp.HandlerOpts{}).ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "dzsa_notifier_request_duration_seconds") {
			t.Fatal(w.Body.String())
		}
	}
	if calls != 2 {
		t.Fatal("metrics scrape performed notifications")
	}
}

func TestGlobalDeferralAndFairness(t *testing.T) {
	var calls []string
	r := testRunner(checkFunc(func(_ context.Context, e string, _ time.Time) (*Server, time.Time, error) {
		calls = append(calls, e)
		return nil, time.Now().Add(time.Hour), errors.New("429")
	}))
	_ = r.Cycle(context.Background())
	_ = r.Cycle(context.Background())
	if len(calls) != 1 {
		t.Fatal("ignored global cooldown", calls)
	}
	r.deferUntil = time.Time{}
	_ = r.Cycle(context.Background())
	if len(calls) != 2 || calls[1] != r.Config.Endpoints[1] {
		t.Fatal("starved remaining endpoint", calls)
	}
}

func TestRunImmediateSequentialCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var active, maximum, calls atomic.Int32
	r := testRunner(checkFunc(func(ctx context.Context, _ string, _ time.Time) (*Server, time.Time, error) {
		n := active.Add(1)
		if n > maximum.Load() {
			maximum.Store(n)
		}
		defer active.Add(-1)
		if calls.Add(1) == 3 {
			cancel()
			return nil, time.Time{}, ctx.Err()
		}
		select {
		case <-time.After(35 * time.Millisecond):
			return &Server{}, time.Time{}, nil
		case <-ctx.Done():
			return nil, time.Time{}, ctx.Err()
		}
	}))
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runner did not cancel")
	}
	if calls.Load() != 3 || maximum.Load() != 1 {
		t.Fatal("concurrent or missing requests")
	}
}

func TestMissedTicksAreSkipped(t *testing.T) {
	start := time.Now()
	if got := nextSlot(start, start.Add(350*time.Millisecond), 100*time.Millisecond); !got.Equal(start.Add(400 * time.Millisecond)) {
		t.Fatal(got)
	}
	if got := nextSlot(start, start, 100*time.Millisecond); !got.Equal(start.Add(100 * time.Millisecond)) {
		t.Fatal(got)
	}
}
