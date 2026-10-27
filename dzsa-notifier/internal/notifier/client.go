package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strconv"
	"time"
)

const queryURL = "https://dayzsalauncher.com/api/v1/query/"
const responseLimit = 1 << 20

type Server struct {
	Endpoint struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	} `json:"endpoint"`
	Game string            `json:"game"`
	Name string            `json:"name"`
	Mods []json.RawMessage `json:"mods"`
}

type Client struct {
	http    *http.Client
	baseURL string
	version string
}

func NewClient(timeout time.Duration, version string) *Client {
	return &Client{http: &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, baseURL: queryURL, version: version}
}

// Check returns a service-wide deferral even when the response is unsuccessful.
func (c *Client) Check(ctx context.Context, endpoint string) (*Server, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return nil, time.Time{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dzsa-notifier/"+c.version)
	r, err := c.http.Do(req)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("DZSA request: %w", err)
	}
	defer r.Body.Close()
	// Delta-seconds start when the response arrives, not when its request began.
	deferUntil := retryAfter(r.Header.Get("Retry-After"), time.Now())
	if r.StatusCode != http.StatusOK {
		return nil, deferUntil, fmt.Errorf("DZSA HTTP %d", r.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, responseLimit+1))
	if err != nil {
		return nil, deferUntil, fmt.Errorf("read DZSA response: %w", err)
	}
	if len(raw) > responseLimit {
		return nil, deferUntil, errors.New("DZSA response exceeds 1 MiB")
	}
	var envelope struct {
		Status *int    `json:"status"`
		Error  string  `json:"error"`
		Result *Server `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, deferUntil, fmt.Errorf("invalid DZSA JSON: %w", err)
	}
	if envelope.Status == nil || *envelope.Status != 0 || envelope.Result == nil {
		return nil, deferUntil, fmt.Errorf("DZSA did not confirm server: %q", envelope.Error)
	}
	want, _ := netip.ParseAddrPort(endpoint) // Validated before any requests.
	actual, err := netip.ParseAddr(envelope.Result.Endpoint.IP)
	if err != nil || actual != want.Addr() || envelope.Result.Endpoint.Port != int(want.Port()) || envelope.Result.Game != "dayz" {
		return nil, deferUntil, errors.New("DZSA returned a different endpoint or game")
	}
	return envelope.Result, deferUntil, nil
}

func retryAfter(value string, now time.Time) time.Time {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		// Avoid duration overflow from malformed or hostile response headers.
		if seconds > int64((1<<63-1)/int64(time.Second)) {
			seconds = int64((1<<63 - 1) / int64(time.Second))
		}
		return now.Add(time.Duration(seconds) * time.Second)
	}
	if t, err := http.ParseTime(value); err == nil && t.After(now) {
		return t
	}
	return time.Time{}
}
