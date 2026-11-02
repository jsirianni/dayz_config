package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

type processLogsKey struct{}

// A session owns one Lumberjack instance and one cross-process lease per file.
// Command captures only flush their framers; they do not reopen loggers or leave
// a new Lumberjack maintenance goroutine behind after every command.
type processLogs struct {
	mu     sync.Mutex
	config HostConfig
	files  map[string]*processCapture
	closed bool
}

type borrowedLog struct{ io.Writer }

func (borrowedLog) Close() error { return nil }

// Lumberjack permits Write after Close to reopen its file. Prevent that after
// the session relinquishes its lease, even if an interrupted child drains late.
type sessionLog struct {
	mu     sync.Mutex
	output io.WriteCloser
	closed bool
}

func (s *sessionLog) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, os.ErrClosed
	}
	return s.output.Write(p)
}
func (s *sessionLog) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.output.Close()
}

func processLogSession(ctx context.Context, config HostConfig) (context.Context, *processLogs, bool) {
	if logs, ok := ctx.Value(processLogsKey{}).(*processLogs); ok {
		return ctx, logs, false
	}
	logs := &processLogs{config: config, files: map[string]*processCapture{}}
	return context.WithValue(ctx, processLogsKey{}, logs), logs, true
}

func captureFromContext(ctx context.Context, config HostConfig, kind string, onError func(error)) (*processCapture, error) {
	if logs, ok := ctx.Value(processLogsKey{}).(*processLogs); ok {
		return logs.capture(ctx, kind, onError)
	}
	return openProcessCapture(ctx, config, kind, onError)
}

func (m *processLogs) capture(ctx context.Context, kind string, onError func(error)) (*processCapture, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("process log session closed")
	}
	owner := m.files[kind]
	if owner == nil {
		var err error
		owner, err = openProcessCapture(ctx, m.config, kind, nil)
		if err != nil {
			return nil, err
		}
		owner.output = &sessionLog{output: owner.output}
		m.files[kind] = owner
	}
	c := newProcessCapture(borrowedLog{owner.output}, onError)
	c.stdout.limit, c.stderr.limit = owner.stdout.limit, owner.stderr.limit
	return c, nil
}

func (m *processLogs) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	var result error
	for kind, owner := range m.files {
		if err := owner.Close(); err != nil {
			captureErrorFile(m.config, kind, err)
			result = errors.Join(result, fmt.Errorf("close %s log: %w", kind, err))
		}
	}
	return result
}
