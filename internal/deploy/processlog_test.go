package deploy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jsirianni/dayz_config/internal/logreader"
	"go.yaml.in/yaml/v3"
)

type captureBuffer struct{ bytes.Buffer }

func (*captureBuffer) Close() error { return nil }

func capturedRecords(t *testing.T, text string) []logreader.Record {
	t.Helper()
	var records []logreader.Record
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &fields); err != nil || len(fields) != 3 {
			t.Fatalf("invalid record: %.200s: %v", line, err)
		}
		r, ok := logreader.DecodeRecord(line)
		if !ok || !strings.HasSuffix(string(fields["timestamp"]), `Z"`) {
			t.Fatalf("not a UTC record: %.200s", line)
		}
		records = append(records, r)
	}
	return records
}

func TestCaptureFramesAndFlushes(t *testing.T) {
	out := &captureBuffer{}
	c := newProcessCapture(out, nil)
	before := time.Now().UTC()
	for _, chunk := range []string{"fir", "st\r", "\n\nsecond\nlast"} {
		_, _ = c.stdout.Write([]byte(chunk))
	}
	_, _ = c.stderr.Write([]byte("<warning>\nerror tail"))
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	records := capturedRecords(t, out.String())
	if len(records) != 6 {
		t.Fatal(records)
	}
	want := []string{"first", "", "second", "<warning>", "last", "error tail"}
	for i, r := range records {
		if r.Message != want[i] || r.Timestamp.Before(before) || r.Timestamp.After(time.Now().UTC()) {
			t.Fatalf("record %d: %+v", i, r)
		}
		level := "info"
		if i == 3 || i == 5 {
			level = "error"
		}
		if r.Level != level {
			t.Fatal(r)
		}
	}
}

func TestCaptureOversizePreservesTextAndUTF8(t *testing.T) {
	for _, message := range []string{strings.Repeat("x", maxPendingLine), strings.Repeat("x", maxPendingLine+31), strings.Repeat("x", maxPendingLine-1) + "€" + strings.Repeat("y", maxPendingLine)} {
		out := &captureBuffer{}
		c := newProcessCapture(out, nil)
		input := message + "\r\nnext\n"
		for len(input) > 0 {
			n := min(len(input), 8191)
			_, _ = c.stdout.Write([]byte(input[:n]))
			input = input[n:]
			if len(c.stdout.pending) > maxPendingLine {
				t.Fatal("unbounded buffer")
			}
		}
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
		records := capturedRecords(t, out.String())
		var joined strings.Builder
		for _, r := range records[:len(records)-1] {
			joined.WriteString(r.Message)
		}
		if joined.String() != message || records[len(records)-1].Message != "next" {
			t.Fatal("message lost or extra line")
		}
	}
}

func TestCaptureConcurrentStreams(t *testing.T) {
	out := &captureBuffer{}
	c := newProcessCapture(out, nil)
	var wg sync.WaitGroup
	for _, stream := range []*processStream{c.stdout, c.stderr} {
		wg.Add(1)
		go func(s *processStream) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				fmt.Fprintf(s, "line %d\n", i)
			}
		}(stream)
	}
	wg.Wait()
	_ = c.Close()
	next := map[string]int{}
	for _, r := range capturedRecords(t, out.String()) {
		if r.Message != fmt.Sprintf("line %d", next[r.Level]) {
			t.Fatal(r)
		}
		next[r.Level]++
	}
	if next["info"] != 100 || next["error"] != 100 {
		t.Fatal(next)
	}
}

type brokenCapture struct{}

func (brokenCapture) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func (brokenCapture) Close() error              { return io.ErrClosedPipe }

func TestCaptureFailureKeepsDraining(t *testing.T) {
	var calls int
	c := newProcessCapture(brokenCapture{}, func(error) { calls++ })
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := captureHelper(ctx, "large")
	c.attach(cmd)
	prepareCommand(cmd)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(c.Close(), io.ErrClosedPipe) || calls != 1 {
		t.Fatalf("failure not reported exactly once: %d", calls)
	}
}

func captureHelper(ctx context.Context, mode string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCaptureHelper$")
	cmd.Env = append(os.Environ(), "DZDEPLOY_CAPTURE_HELPER="+mode)
	return cmd
}

func TestCaptureHelper(t *testing.T) {
	mode := os.Getenv("DZDEPLOY_CAPTURE_HELPER")
	if mode == "" {
		return
	}
	if mode == "lease" {
		cfg := HostConfig{StateDir: os.Getenv("DZDEPLOY_CAPTURE_STATE")}
		c, err := openProcessCapture(context.Background(), cfg, "console", nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, "locked")
		_, _ = io.Copy(io.Discard, os.Stdin)
		_ = c.Close()
		os.Exit(0)
	}
	if mode == "large" {
		for i := 0; i < 1000; i++ {
			fmt.Fprintln(os.Stdout, strings.Repeat("a", 4096))
			fmt.Fprintln(os.Stderr, strings.Repeat("b", 4096))
		}
		os.Exit(0)
	}
	fmt.Fprint(os.Stdout, "ordinary\nfinal stdout")
	fmt.Fprint(os.Stderr, "error\nfinal stderr")
	if mode == "fail" {
		os.Exit(7)
	}
	os.Exit(0)
}

func TestLoggedCommandPreservesExitAndInitializesBeforeLaunch(t *testing.T) {
	cfg := HostConfig{StateDir: t.TempDir()}
	err := runLoggedCommand(context.Background(), cfg, "subprocess", captureHelper(context.Background(), "fail"))
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 || hasCaptureFailure(err) {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(cfg.StateDir, "logs", "subprocess.log"))
	if err != nil {
		t.Fatal(err)
	}
	r := capturedRecords(t, string(b))
	if len(r) != 4 {
		t.Fatal(r)
	}
	bad := HostConfig{StateDir: t.TempDir()}
	if err := os.WriteFile(filepath.Join(bad.StateDir, "logs"), []byte("file blocks directory"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := captureHelper(context.Background(), "ok")
	err = runLoggedCommand(context.Background(), bad, "console", cmd)
	if err == nil || cmd.Process != nil {
		t.Fatal("command started without logging", err)
	}
}

func TestProcessLogMigrationAndSeparatePolicies(t *testing.T) {
	compress := false
	cfg := HostConfig{StateDir: t.TempDir(), Logging: LoggingConfig{Console: RotationConfig{MaxSizeMB: 1, MaxBackups: 1, MaxAgeDays: 1, Compress: &compress}}}
	writeTest(t, cfg.StateDir, "logs/server-console.log", "old plain text\n")
	c, err := openProcessCapture(context.Background(), cfg, "console", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.stdout.Write([]byte("new\n"))
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(cfg.StateDir, "logs", "server-console-*.log"))
	if len(files) != 1 {
		t.Fatal(files)
	}
	old, _ := os.ReadFile(files[0])
	if string(old) != "old plain text\n" {
		t.Fatal(string(old))
	}
	c, err = openProcessCapture(context.Background(), cfg, "console", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.stderr.Write([]byte("appended\n"))
	_ = c.Close()
	b, _ := os.ReadFile(filepath.Join(cfg.StateDir, "logs", "server-console.log"))
	if len(capturedRecords(t, string(b))) != 2 {
		t.Fatal("JSON rotated on reopen")
	}
	updates, err := openProcessCapture(context.Background(), cfg, "updates", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = updates.stdout.Write([]byte("steam\n"))
	_ = updates.Close()
	entries, _ := os.ReadDir(filepath.Join(cfg.StateDir, "logs"))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".gz") {
			t.Fatal("console ignored compression=false")
		}
	}
}

func TestLogLeaseExcludesOtherCaptures(t *testing.T) {
	cfg := HostConfig{StateDir: t.TempDir()}
	first, err := openProcessCapture(context.Background(), cfg, "console", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if second, err := openProcessCapture(ctx, cfg, "console", nil); err == nil {
		second.Close()
		t.Fatal("simultaneous writers")
	}
	other, err := openProcessCapture(context.Background(), cfg, "updates", nil)
	if err != nil {
		t.Fatal(err)
	}
	other.Close()
	first.Close()
	last, err := openProcessCapture(context.Background(), cfg, "console", nil)
	if err != nil {
		t.Fatal("lease not released", err)
	}
	last.Close()
}

func TestLoggingConfigDefaultsAndValidation(t *testing.T) {
	c := testConfig(t)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []RotationConfig{c.Logging.Console, c.Logging.SteamCMD, c.Logging.Subprocess} {
		if p.MaxSizeMB != 50 || p.MaxBackups != 5 || p.MaxAgeDays != 30 || p.Compress == nil || !*p.Compress {
			t.Fatal(p)
		}
	}
	c.Logging.Console.MaxBackups = -1
	if c.Validate() == nil {
		t.Fatal("negative retention accepted")
	}
}

func TestLogConfigurationCompatibility(t *testing.T) {
	old := testConfig(t)
	raw, _ := json.Marshal(old)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	delete(fields, "logging")
	raw, _ = json.Marshal(fields)
	var restored HostConfig
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil {
		t.Fatal(err)
	}
	_ = old.Validate()
	if !reflect.DeepEqual(old, restored) {
		t.Fatal("legacy configuration not normalized equivalently")
	}
	var custom struct {
		Logging LoggingConfig `yaml:"logging"`
	}
	if err := yaml.Unmarshal([]byte("logging:\n  console:\n    max_size_mb: 7\n    compress: false\n  steamcmd:\n    max_backups: 2\n"), &custom); err != nil {
		t.Fatal(err)
	}
	old.Logging = custom.Logging
	if err := old.Validate(); err != nil {
		t.Fatal(err)
	}
	if old.Logging.Console.MaxSizeMB != 7 || *old.Logging.Console.Compress || old.Logging.SteamCMD.MaxBackups != 2 || !*old.Logging.SteamCMD.Compress || old.Logging.Subprocess.MaxBackups != 5 {
		t.Fatal(old.Logging)
	}
	old.Logging.Console.MaxAgeDays = int(^uint(0) >> 1)
	if old.Validate() == nil {
		t.Fatal("overflowing duration accepted")
	}
}

func TestProcessLogSessionReusesWriterAndReleasesLease(t *testing.T) {
	config := HostConfig{StateDir: t.TempDir()}
	ctx, manager, owner := processLogSession(context.Background(), config)
	if !owner {
		t.Fatal("missing owner")
	}
	_, nested, nestedOwner := processLogSession(ctx, config)
	if nested != manager || nestedOwner {
		t.Fatal("nested setup did not borrow session")
	}
	for i := 0; i < 2; i++ {
		c, err := captureFromContext(ctx, config, "subprocess", nil)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(c.stdout, "command %d\n", i)
		if err = c.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if len(manager.files) != 1 {
		t.Fatal("logger not reused")
	}
	blocked, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if second, err := openProcessCapture(blocked, config, "subprocess", nil); err == nil {
		second.Close()
		t.Fatal("session lease released between commands")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := openProcessCapture(context.Background(), config, "subprocess", nil)
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
	if _, err = manager.capture(context.Background(), "subprocess", nil); err == nil {
		t.Fatal("closed manager reused")
	}
}

func TestLogLeaseAcrossProcesses(t *testing.T) {
	config := HostConfig{StateDir: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := captureHelper(ctx, "lease")
	cmd.Env = append(cmd.Env, "DZDEPLOY_CAPTURE_STATE="+config.StateDir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	prepareCommand(cmd)
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "locked" {
		t.Fatal(line, err)
	}
	blocked, stop := context.WithTimeout(ctx, 80*time.Millisecond)
	defer stop()
	if other, err := openProcessCapture(blocked, config, "console", nil); err == nil {
		other.Close()
		t.Fatal("cross-process writer lease ignored")
	}
	stdin.Close()
	if err = cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	last, err := openProcessCapture(ctx, config, "console", nil)
	if err != nil {
		t.Fatal(err)
	}
	last.Close()
}

func TestClosedLogSessionCannotReopenWriter(t *testing.T) {
	config := HostConfig{StateDir: t.TempDir()}
	ctx, manager, _ := processLogSession(context.Background(), config)
	c, err := captureFromContext(ctx, config, "console", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Close(); err != nil {
		t.Fatal(err)
	}
	_, _ = c.stdout.Write([]byte("late output\n"))
	if !errors.Is(c.Close(), os.ErrClosed) {
		t.Fatal("late drain reopened released log")
	}
}

func TestTornRecordMigrationAndEscapedRecordSize(t *testing.T) {
	compress := false
	config := HostConfig{StateDir: t.TempDir(), Logging: LoggingConfig{Console: RotationConfig{MaxSizeMB: 1, Compress: &compress}}}
	first, _ := json.Marshal(logreader.Record{Level: "info", Message: "prior", Timestamp: time.Now().UTC()})
	legacy := string(first) + "\n" + `{"level":"info"`
	writeTest(t, config.StateDir, "logs/server-console.log", legacy)
	c, err := openProcessCapture(context.Background(), config, "console", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Control bytes expand six-fold in JSON, so smaller rotation sizes need
	// smaller raw chunks even when the original line is shorter than 1 MiB.
	_, _ = c.stdout.Write([]byte(strings.Repeat("\x01", 200000) + "\n"))
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(config.StateDir, "logs", "server-console*.log"))
	foundLegacy := false
	for _, name := range files {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) == legacy {
			foundLegacy = true
			continue
		}
		if len(b) > 1<<20 {
			t.Fatal("record exceeds rotation size", len(b))
		}
		_ = capturedRecords(t, string(b))
	}
	if !foundLegacy {
		t.Fatal("torn historical record rewritten or lost")
	}
}

func TestIndependentRotationCleanupAndCompression(t *testing.T) {
	cfg := HostConfig{StateDir: t.TempDir(), Logging: LoggingConfig{
		Console:  RotationConfig{MaxSizeMB: 1, MaxBackups: 2, MaxAgeDays: 1},
		SteamCMD: RotationConfig{MaxSizeMB: 10},
	}}
	writeTest(t, cfg.StateDir, "logs/server-console-2000-01-01T00-00-00.000.log", "expired\n")
	c, err := openProcessCapture(context.Background(), cfg, "console", nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := openProcessCapture(context.Background(), cfg, "updates", nil)
	if err != nil {
		t.Fatal(err)
	}
	message := strings.Repeat("x", 500000) + "\n"
	for i := 0; i < 9; i++ {
		_, _ = c.stdout.Write([]byte(message))
		_, _ = u.stdout.Write([]byte(message))
		time.Sleep(4 * time.Millisecond)
	}
	if err := errors.Join(c.Close(), u.Close()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		backups, _ := filepath.Glob(filepath.Join(cfg.StateDir, "logs", "server-console-*.log*"))
		ready := len(backups) == 2
		for _, p := range backups {
			ready = ready && strings.HasSuffix(p, ".gz") && !strings.Contains(p, "2000-")
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("rotation cleanup did not settle", backups)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if backups, _ := filepath.Glob(filepath.Join(cfg.StateDir, "logs", "updates-*.log*")); len(backups) != 0 {
		t.Fatal("console policy leaked into updates", backups)
	}
}

func init() {
	if os.Getenv("DZDEPLOY_NATIVE_CAPTURE_HELPER") == "1" {
		fmt.Fprint(os.Stdout, "native output\nfinal stdout")
		fmt.Fprint(os.Stderr, "native error\nfinal stderr")
		// Give Launch enough time to verify the child's process identity.
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}
}

func TestNativeGameLaunchDrainsFinalOutput(t *testing.T) {
	cfg := testConfig(t)
	target, _ := TargetFor(cfg.Target)
	self, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(cfg.InstallDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(cfg.InstallDir, "DayZServer_x64.exe"), self, 0700); err != nil {
		t.Fatal(err)
	}
	writeTest(t, cfg.InstallDir, "mpmissions/"+target.Mission+"/db/types.xml", "<types/>")
	t.Setenv("DZDEPLOY_NATIVE_CAPTURE_HELPER", "1")
	g := &NativeGame{Config: cfg}
	pid, _, err := g.Launch(target)
	if err != nil {
		t.Fatal(pid, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = g.waitCapture(ctx); err != nil {
		t.Fatal(err)
	}
	// Stop on an already-exited child still synchronizes the capture lifecycle.
	if err = g.Stop(ctx, State{}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(cfg.StateDir, "logs", "server-console.log"))
	if err != nil {
		t.Fatal(err)
	}
	records := capturedRecords(t, string(b))
	if len(records) != 4 {
		t.Fatal(records)
	}
	messages := map[string]string{}
	for _, r := range records {
		messages[r.Message] = r.Level
	}
	if messages["final stdout"] != "info" || messages["final stderr"] != "error" {
		t.Fatal(messages)
	}
}
