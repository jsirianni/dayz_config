// Package pve implements the small API surface needed by pvecli. It never
// retries mutations: HTTP method names alone do not establish idempotence.
package pve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	Base, TokenID, Secret string
	HTTP                  *http.Client
	Poll                  time.Duration
}
type Resource struct {
	VMID     int    `json:"vmid"`
	Node     string `json:"node"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Template int    `json:"template"`
}
type Config map[string]any

func (c Config) String(k string) string {
	if c[k] == nil {
		return ""
	}
	return fmt.Sprint(c[k])
}
func (c Config) Int(k string) int   { n, _ := strconv.Atoi(c.String(k)); return n }
func (c Config) Bool(k string) bool { return c.String(k) == "1" || c.String(k) == "true" }

type Pending struct {
	Key     string `json:"key"`
	Value   any    `json:"value,omitempty"`
	Pending any    `json:"pending,omitempty"`
	Delete  any    `json:"delete,omitempty"`
}
type Task struct {
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus"`
	UPID       string `json:"upid"`
}
type TaskFailure struct {
	ID, ExitStatus string
	Logs           any
}

func (e *TaskFailure) Error() string {
	return fmt.Sprintf("task %s failed (%s): %v", e.ID, e.ExitStatus, e.Logs)
}

type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("Proxmox HTTP %d: %s", e.Status, e.Message) }
func New(endpoint, token, secret string) *Client {
	return &Client{Base: strings.TrimSuffix(strings.TrimRight(endpoint, "/"), "/api2/json") + "/api2/json", TokenID: token, Secret: secret, HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, Poll: time.Second}
}
func (c *Client) Do(ctx context.Context, method, path string, values url.Values, out any) error {
	var body io.Reader
	u := c.Base + path
	if method == http.MethodGet {
		if len(values) > 0 {
			u += "?" + values.Encode()
		}
	} else if values != nil {
		body = strings.NewReader(values.Encode())
	}
	req, e := http.NewRequestWithContext(ctx, method, u, body)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "PVEAPIToken="+c.TokenID+"="+c.Secret)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return fmt.Errorf("Proxmox request: %w", e)
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if e != nil {
		return e
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(b))
		if len(msg) > 2048 {
			msg = msg[:2048]
		}
		if c.Secret != "" {
			msg = strings.ReplaceAll(msg, c.Secret, "[redacted]")
		}
		return &APIError{resp.StatusCode, msg}
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if e = json.Unmarshal(b, &env); e != nil {
		return fmt.Errorf("invalid Proxmox response: %w", e)
	}
	if out == nil {
		return nil
	}
	if len(env.Data) == 0 {
		return errors.New("Proxmox response missing data")
	}
	d := json.NewDecoder(strings.NewReader(string(env.Data)))
	d.UseNumber()
	return d.Decode(out)
}
func Path(node string, id int) string {
	return fmt.Sprintf("/nodes/%s/qemu/%d", url.PathEscape(node), id)
}
func (c *Client) CheckVersion(ctx context.Context) (string, error) {
	var v struct {
		Version string `json:"version"`
	}
	e := c.Do(ctx, "GET", "/version", nil, &v)
	if e != nil {
		return "", e
	}
	if !strings.HasPrefix(v.Version, "9.") {
		return v.Version, fmt.Errorf("unsupported Proxmox version %q; require 9.x", v.Version)
	}
	return v.Version, nil
}
func (c *Client) Resources(ctx context.Context) ([]Resource, error) {
	var v []Resource
	e := c.Do(ctx, "GET", "/cluster/resources", url.Values{"type": {"vm"}}, &v)
	return v, e
}
func (c *Client) Find(ctx context.Context, id int) (Resource, bool, error) {
	rs, e := c.Resources(ctx)
	if e != nil {
		return Resource{}, false, e
	}
	for _, r := range rs {
		if r.VMID == id {
			if r.Type != "qemu" {
				return r, false, errors.New("VMID belongs to a container, not a QEMU VM")
			}
			return r, true, nil
		}
	}
	return Resource{}, false, nil
}
func (c *Client) VMConfig(ctx context.Context, node string, id int) (Config, error) {
	v := Config{}
	e := c.Do(ctx, "GET", Path(node, id)+"/config", nil, &v)
	return v, e
}
func (c *Client) Pending(ctx context.Context, node string, id int) ([]Pending, error) {
	var v []Pending
	e := c.Do(ctx, "GET", Path(node, id)+"/pending", nil, &v)
	changes := []Pending{}
	for _, row := range v {
		if row.Pending != nil || fmt.Sprint(row.Delete) == "1" || fmt.Sprint(row.Delete) == "true" {
			changes = append(changes, row)
		}
	}
	return changes, e
}
func (c *Client) Submit(ctx context.Context, method, path string, values url.Values) (string, error) {
	var task string
	e := c.Do(ctx, method, path, values, &task)
	if e == nil && !strings.HasPrefix(task, "UPID:") {
		e = errors.New("mutation returned no task ID; inspect remote state before retrying")
	}
	return task, e
}
func (c *Client) TaskStatus(ctx context.Context, node, task string) (Task, error) {
	var v Task
	e := c.Do(ctx, "GET", "/nodes/"+url.PathEscape(node)+"/tasks/"+url.PathEscape(task)+"/status", nil, &v)
	return v, e
}
func (c *Client) TaskLogs(ctx context.Context, node, task string) (any, error) {
	var v any
	e := c.Do(ctx, "GET", "/nodes/"+url.PathEscape(node)+"/tasks/"+url.PathEscape(task)+"/log", url.Values{"limit": {"30"}}, &v)
	return v, e
}
func (c *Client) Wait(ctx context.Context, node, task string) error {
	for {
		v, e := c.TaskStatus(ctx, node, task)
		if e != nil {
			return e
		}
		switch v.Status {
		case "stopped":
			if v.ExitStatus != "OK" {
				logs, _ := c.TaskLogs(ctx, node, task)
				return &TaskFailure{ID: task, ExitStatus: v.ExitStatus, Logs: logs}
			}
			return nil
		case "running":
		default:
			return fmt.Errorf("unexpected task status %q", v.Status)
		}
		if e := Delay(ctx, c.Poll); e != nil {
			return e
		}
	}
}
func Delay(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (c *Client) Rows(ctx context.Context, path string) ([]Config, error) {
	var r []Config
	e := c.Do(ctx, "GET", path, nil, &r)
	return r, e
}
func (c *Client) AgentFile(ctx context.Context, node string, id int, file string) (string, error) {
	var r struct {
		Content string `json:"content"`
	}
	e := c.Do(ctx, "GET", Path(node, id)+"/agent/file-read", url.Values{"file": {file}, "count": {"16384"}, "decode": {"1"}}, &r)
	return r.Content, e
}
