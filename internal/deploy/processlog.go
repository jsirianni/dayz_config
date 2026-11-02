package deploy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jsirianni/dayz_config/internal/logreader"
	"gopkg.in/natefinch/lumberjack.v2"
)

type RotationConfig struct {
	MaxSizeMB  int   `json:"maxSizeMB" yaml:"max_size_mb,omitempty"`
	MaxBackups int   `json:"maxBackups" yaml:"max_backups,omitempty"`
	MaxAgeDays int   `json:"maxAgeDays" yaml:"max_age_days,omitempty"`
	Compress   *bool `json:"compress,omitempty" yaml:"compress,omitempty"`
}

type LoggingConfig struct {
	Console    RotationConfig `json:"console" yaml:"console,omitempty"`
	SteamCMD   RotationConfig `json:"steamcmd" yaml:"steamcmd,omitempty"`
	Subprocess RotationConfig `json:"subprocess" yaml:"subprocess,omitempty"`
}

func (c *RotationConfig) defaults() error {
	if c.MaxSizeMB == 0 {
		c.MaxSizeMB = 50
	}
	if c.MaxBackups == 0 {
		c.MaxBackups = 5
	}
	if c.MaxAgeDays == 0 {
		c.MaxAgeDays = 30
	}
	if c.Compress == nil {
		enabled := true
		c.Compress = &enabled
	}
	if c.MaxSizeMB < 1 || c.MaxBackups < 1 || c.MaxAgeDays < 1 {
		return errors.New("log size, backup count and age must be positive")
	}
	if int64(c.MaxSizeMB) > math.MaxInt64/(1<<20) || int64(c.MaxAgeDays) > math.MaxInt64/int64(24*time.Hour) {
		return errors.New("log size or age exceeds supported range")
	}
	return nil
}

const maxPendingLine = 1 << 20

// capture never returns a sink error from Write: exec must keep draining pipes
// even if a disk fills. Close reports the first failure after all output drains.
type processCapture struct {
	mu             sync.Mutex
	output         io.WriteCloser
	err            error
	onError        func(error)
	unlock         func()
	stdout, stderr *processStream
	once           sync.Once
	closeErr       error
}

type processStream struct {
	mu      sync.Mutex
	capture *processCapture
	level   string
	pending []byte
	split   bool
	limit   int
}

func newProcessCapture(out io.WriteCloser, onError func(error)) *processCapture {
	c := &processCapture{output: out, onError: onError}
	c.stdout = &processStream{capture: c, level: "info", limit: maxPendingLine}
	c.stderr = &processStream{capture: c, level: "error", limit: maxPendingLine}
	return c
}

func (c *processCapture) fail(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	first := c.err == nil
	if first {
		c.err = err
	}
	c.mu.Unlock()
	if first && c.onError != nil {
		c.onError(err)
	}
}

func (c *processCapture) record(level string, message []byte) {
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	b, err := json.Marshal(logreader.Record{Level: level, Message: string(message), Timestamp: time.Now().UTC()})
	if err == nil {
		b = append(b, '\n')
		var n int
		n, err = c.output.Write(b)
		if err == nil && n != len(b) {
			err = io.ErrShortWrite
		}
	}
	c.mu.Unlock()
	c.fail(err)
}

func (s *processStream) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(p)
	for len(p) > 0 {
		end := bytes.IndexByte(p, '\n')
		if end < 0 {
			end = len(p)
		}
		take := min(end, s.limit-len(s.pending))
		s.pending = append(s.pending, p[:take]...)
		p = p[take:]
		// Keep a full buffer until the next byte: a following LF (or CRLF)
		// must not produce an extra empty record at a chunk boundary.
		if len(p) > 0 && p[0] == '\n' {
			line := s.pending
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if len(line) > 0 || !s.split {
				s.capture.record(s.level, line)
			}
			s.pending = s.pending[:0]
			s.split = false
			p = p[1:]
		} else if len(s.pending) == s.limit && len(p) > 0 {
			cut := len(s.pending)
			// Avoid replacing a valid UTF-8 character split between records.
			start := cut - 1
			for start > 0 && start > cut-utf8.UTFMax && !utf8.RuneStart(s.pending[start]) {
				start--
			}
			if !utf8.FullRune(s.pending[start:]) {
				cut = start
			}
			if s.pending[len(s.pending)-1] == '\r' {
				cut = min(cut, len(s.pending)-1)
			}
			if cut < 1 {
				cut = len(s.pending)
			}
			s.capture.record(s.level, s.pending[:cut])
			s.pending = append(s.pending[:0], s.pending[cut:]...)
			s.split = true
		}
	}
	return n, nil
}

func (s *processStream) flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) > 0 {
		s.capture.record(s.level, s.pending)
		s.pending = nil
	}
}

func (c *processCapture) attach(cmd *exec.Cmd) {
	cmd.Stdout, cmd.Stderr = c.stdout, c.stderr
	cmd.WaitDelay = 5 * time.Second
}

func (c *processCapture) Close() error {
	c.once.Do(func() {
		c.stdout.flush()
		c.stderr.flush()
		c.fail(c.output.Close())
		if c.unlock != nil {
			c.unlock()
		}
		c.mu.Lock()
		c.closeErr = c.err
		c.mu.Unlock()
	})
	return c.closeErr
}

func openProcessCapture(ctx context.Context, config HostConfig, kind string, onError func(error)) (*processCapture, error) {
	var policy RotationConfig
	var name string
	switch kind {
	case "console":
		policy, name = config.Logging.Console, "server-console.log"
	case "updates":
		policy, name = config.Logging.SteamCMD, "updates.log"
	case "subprocess":
		policy, name = config.Logging.Subprocess, "subprocess.log"
	default:
		return nil, errors.New("invalid process log kind")
	}
	if err := policy.defaults(); err != nil {
		return nil, err
	}
	filename, err := SafeJoin(config.StateDir, "logs/"+name)
	if err != nil {
		return nil, err
	}
	lockDir, err := SafeJoin(config.StateDir, "logs/."+kind+"-writer")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(lockDir, 0700); err != nil {
		return nil, err
	}
	// Per-file leases cover commands from both the service and provisioning.
	deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var unlock func()
	for {
		unlock, err = lockProcessLog(lockDir)
		if err == nil {
			break
		}
		select {
		case <-deadline.Done():
			return nil, fmt.Errorf("process log %s unavailable: %w", kind, err)
		case <-time.After(25 * time.Millisecond):
		}
	}
	ok := false
	defer func() {
		if !ok {
			unlock()
		}
	}()
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	legacy := false
	if err == nil && stat.Size() > 0 {
		reader := bufio.NewReader(io.LimitReader(f, 8<<20))
		first, _ := reader.ReadString('\n')
		_, valid := logreader.DecodeRecord(first)
		last := []byte{0}
		_, lastErr := f.ReadAt(last, stat.Size()-1)
		legacy = !valid || lastErr != nil || last[0] != '\n'
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return nil, err
	}
	logger := &lumberjack.Logger{Filename: filename, MaxSize: policy.MaxSizeMB, MaxBackups: policy.MaxBackups, MaxAge: policy.MaxAgeDays, Compress: *policy.Compress}
	if legacy {
		err = logger.Rotate()
	} else {
		_, err = logger.Write(nil)
	}
	if err != nil {
		_ = logger.Close()
		return nil, err
	}
	c := newProcessCapture(logger, onError)
	// JSON can expand a byte into six escaped bytes. Leave room for metadata so
	// even a tiny configured file can accept each indivisible JSON record.
	c.stdout.limit = min(maxPendingLine, (policy.MaxSizeMB*(1<<20)-256)/6)
	c.stderr.limit = c.stdout.limit
	c.unlock = unlock
	ok = true
	return c, nil
}

// runLoggedCommand preserves exec.ExitError via errors.Join for callers which
// distinguish successful installer exit codes. Command arguments are not logged.
func runLoggedCommand(ctx context.Context, config HostConfig, kind string, cmd *exec.Cmd) error {
	c, err := captureFromContext(ctx, config, kind, nil)
	if err != nil {
		captureErrorFile(config, kind, err)
		return &captureFailure{fmt.Errorf("initialize %s log: %w", kind, err)}
	}
	stdout, stderr := cmd.Stdout, cmd.Stderr
	c.attach(cmd)
	if stdout != nil {
		cmd.Stdout = io.MultiWriter(c.stdout, stdout)
	}
	if stderr != nil {
		cmd.Stderr = io.MultiWriter(c.stderr, stderr)
	}
	prepareCommand(cmd)
	err = cmd.Run()
	if errors.Is(err, exec.ErrWaitDelay) {
		c.fail(err)
	}
	closeErr := c.Close()
	if closeErr != nil {
		closeErr = &captureFailure{fmt.Errorf("%s output capture failed: %w", kind, closeErr)}
		captureErrorFile(config, kind, closeErr)
	}
	return errors.Join(err, closeErr)
}

// commandTail retains bounded error context for administrative commands while
// their complete output is streamed to disk. Both pipes may write concurrently.
type commandTail struct {
	mu   sync.Mutex
	data []byte
}

func (b *commandTail) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	const limit = 64 << 10
	if len(p) >= limit {
		b.data = append(b.data[:0], p[len(p)-limit:]...)
	} else {
		if len(b.data)+len(p) > limit {
			b.data = append(b.data[:0], b.data[len(b.data)+len(p)-limit:]...)
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}
func (b *commandTail) String() string { b.mu.Lock(); defer b.mu.Unlock(); return string(b.data) }

type captureFailure struct{ error }

func (e *captureFailure) Unwrap() error { return e.error }
func hasCaptureFailure(err error) bool  { var capture *captureFailure; return errors.As(err, &capture) }

func captureErrorFile(config HostConfig, kind string, err error) {
	_ = AtomicWrite(filepath.Join(config.StateDir, "logs", "capture-error.txt"), []byte(kind+" output capture failed: "+err.Error()), 0600)
}
