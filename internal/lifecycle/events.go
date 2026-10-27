// Package lifecycle stores bounded, durable host lifecycle observations.
package lifecycle

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Event struct {
	At        time.Time `json:"at"`
	Target    string    `json:"target"`
	Kind      string    `json:"kind"`
	Cause     string    `json:"cause,omitempty"`
	Operation string    `json:"operation,omitempty"`
	PID       int       `json:"pid,omitempty"`
	Token     string    `json:"token,omitempty"`
	Commit    string    `json:"commit,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	Source    string    `json:"source,omitempty"`
}

// Write uses immutable individual files, avoiding torn concurrent JSONL appends.
func Write(root string, e Event) error {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	dir := filepath.Join(root, "events")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".event-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(e)
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, e.At.Format("20060102T150405.000000000Z")+"-"+filepath.Base(f.Name())[7:]+".json"))
}

func Decode(r io.Reader) (Event, error) {
	var e Event
	b, err := io.ReadAll(io.LimitReader(r, 65537))
	if err != nil {
		return e, err
	}
	if len(b) > 65536 {
		return e, errors.New("lifecycle event exceeds limit")
	}
	err = json.Unmarshal(b, &e)
	if err == nil && (e.At.IsZero() || e.Target == "" || e.Kind == "") {
		err = errors.New("invalid lifecycle event")
	}
	return e, err
}
