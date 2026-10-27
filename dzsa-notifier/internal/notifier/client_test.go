package notifier

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const endpoint = "203.0.113.10:27016"
const validResponse = `{"status":0,"result":{"endpoint":{"ip":"203.0.113.10","port":27016},"game":"dayz","name":"Test DayZ","mods":[{"name":"CF"}]}}`

func TestDZSAResponses(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		body string
		good bool
	}{
		{"confirmed", 200, validResponse, true},
		{"http failure", 503, validResponse, false},
		{"rejected", 200, `{"status":1,"error":"Rejected"}`, false},
		{"missing status", 200, `{"result":{}}`, false},
		{"missing result", 200, `{"status":0}`, false},
		{"bad JSON", 200, "<html>", false},
		{"two documents", 200, validResponse + "{}", false},
		{"wrong game", 200, strings.ReplaceAll(validResponse, `"dayz"`, `"arma"`), false},
		{"wrong IP", 200, strings.ReplaceAll(validResponse, "203.0.113.10", "203.0.113.11"), false},
		{"wrong port", 200, strings.ReplaceAll(validResponse, "27016", "27017"), false},
		{"oversize", 200, strings.Repeat(" ", responseLimit+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/"+endpoint || r.Header.Get("Accept") != "application/json" || r.Header.Get("User-Agent") != "dzsa-notifier/test" {
					t.Errorf("wrong request: %+v", r)
				}
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			c := &Client{http: s.Client(), baseURL: s.URL + "/", version: "test"}
			server, _, err := c.Check(context.Background(), endpoint, time.Now())
			if tc.good {
				if err != nil || server.Name != "Test DayZ" || len(server.Mods) != 1 {
					t.Fatalf("%+v %v", server, err)
				}
			} else if err == nil {
				t.Fatal("accepted invalid response")
			}
		})
	}
}

func TestDZSATimeoutCancellationTLSAndRedirect(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	c := NewClient(20*time.Millisecond, "test")
	c.baseURL = s.URL + "/"
	if _, _, err := c.Check(context.Background(), endpoint, time.Now()); err == nil {
		t.Fatal("trusted unknown TLS certificate")
	}
	c.http = s.Client()
	c.http.Timeout = 20 * time.Millisecond
	if _, _, err := c.Check(context.Background(), endpoint, time.Now()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := c.Check(ctx, endpoint, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/redirect", http.StatusFound) }))
	defer s2.Close()
	c = NewClient(time.Second, "test")
	c.baseURL = s2.URL + "/"
	if _, _, err := c.Check(context.Background(), endpoint, time.Now()); err == nil || !strings.Contains(err.Error(), "302") {
		t.Fatalf("redirect followed: %v", err)
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, tc := range []struct {
		value string
		want  time.Time
	}{
		{"60", now.Add(time.Minute)}, {now.Add(time.Hour).Format(http.TimeFormat), now.Add(time.Hour)},
		{"invalid", time.Time{}}, {"-2", time.Time{}}, {now.Add(-time.Hour).Format(http.TimeFormat), time.Time{}},
	} {
		if got := retryAfter(tc.value, now); !got.Equal(tc.want) {
			t.Errorf("%q: %v != %v", tc.value, got, tc.want)
		}
	}
	if got := retryAfter("9223372036854775807", now); !got.After(now) {
		t.Fatal("overflowed Retry-After")
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "60"); w.WriteHeader(429) }))
	defer s.Close()
	c := NewClient(time.Second, "test")
	c.baseURL = s.URL + "/"
	if _, until, err := c.Check(context.Background(), endpoint, now); err == nil || !until.Equal(now.Add(time.Minute)) {
		t.Fatalf("deferral lost: %v %v", until, err)
	}
}
