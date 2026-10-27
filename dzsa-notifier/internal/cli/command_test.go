package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jsirianni/dayz_config/dzsa-notifier/internal/notifier"
)

func validConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("endpoints: [203.0.113.10:27016]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ENDPOINTS", "INTERVAL", "REQUEST_TIMEOUT", "METRICS_LISTEN"} {
		key := "DZSA_NOTIFIER_" + name
		previous, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, previous)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func noExecution(t *testing.T) executeFunc {
	t.Helper()
	return func(context.Context, mode, notifier.Config, string, *slog.Logger) error {
		t.Fatal("unexpected execution or network access")
		return nil
	}
}

func TestHelpAndVersionDoNotLoadConfiguration(t *testing.T) {
	for _, flag := range []string{"--help", "-h", "--version"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := command{version: "1.2.3", stdout: &stdout, stderr: &stderr}
			args := []string{"--config", filepath.Join(t.TempDir(), "missing"), flag}
			if code := cmd.run(context.Background(), args, 0, noExecution(t)); code != 0 {
				t.Fatalf("exit %d: stdout=%s stderr=%s", code, &stdout, &stderr)
			}
			if flag == "--version" {
				if stdout.String() != "1.2.3\n" || stderr.Len() != 0 {
					t.Fatalf("wrong version streams: stdout=%q stderr=%q", &stdout, &stderr)
				}
			} else if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Usage of dzsa-notifier") {
				t.Fatalf("wrong help streams: stdout=%q stderr=%q", &stdout, &stderr)
			}
		})
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--unknown"}, {"--config"}, {"--once=not-a-bool"}, {"positional"},
		{"--once", "--check-config"}, {"--once", "--version"}, {"--version", "--check-config"},
		{"--version", "unexpected"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := command{version: "test", stdout: &stdout, stderr: &stderr}
			if code := cmd.run(context.Background(), args, 1000, noExecution(t)); code != 64 {
				t.Fatalf("exit %d; want 64", code)
			}
			if stderr.Len() == 0 || stdout.Len() != 0 {
				t.Fatalf("wrong usage streams: stdout=%q stderr=%q", &stdout, &stderr)
			}
		})
	}
}

func TestExplicitMissingConfigIsNotHiddenByEnvironment(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("DZSA_NOTIFIER_ENDPOINTS", "203.0.113.10:27016")
	var output bytes.Buffer
	cmd := command{stdout: &output, stderr: io.Discard}
	args := []string{"--config", filepath.Join(t.TempDir(), "missing"), "--once"}
	if code := cmd.run(context.Background(), args, 1000, noExecution(t)); code != 64 {
		t.Fatalf("exit %d; want 64", code)
	}
	if !strings.Contains(output.String(), "invalid configuration") {
		t.Fatal(output.String())
	}
}

func TestConfigurationValidationIsOfflineAndAllowedAsRoot(t *testing.T) {
	clearConfigEnvironment(t)
	var output bytes.Buffer
	cmd := command{stdout: &output, stderr: io.Discard}
	args := []string{"--check-config", "--config", validConfig(t)}
	if code := cmd.run(context.Background(), args, 0, noExecution(t)); code != 0 {
		t.Fatalf("exit %d: %s", code, &output)
	}
	if !strings.Contains(output.String(), "configuration valid") {
		t.Fatal(output.String())
	}
}

func TestRootExecutionRejectedForBothRuntimeModes(t *testing.T) {
	clearConfigEnvironment(t)
	path := validConfig(t)
	for _, extra := range [][]string{nil, {"--once"}} {
		cmd := command{stdout: io.Discard, stderr: io.Discard}
		args := append([]string{"--config", path}, extra...)
		if code := cmd.run(context.Background(), args, 0, noExecution(t)); code != 77 {
			t.Fatalf("args %v: exit %d; want 77", args, code)
		}
	}
}

func TestModeDispatchAndExitCodes(t *testing.T) {
	clearConfigEnvironment(t)
	path := validConfig(t)
	t.Setenv("DZSA_NOTIFIER_INTERVAL", "2m")
	for _, tc := range []struct {
		name string
		args []string
		want mode
		err  error
		code int
	}{
		{name: "daemon", want: modeDaemon},
		{name: "once", args: []string{"--once"}, want: modeOnce},
		{name: "daemon failure", want: modeDaemon, err: errors.New("listener failed"), code: 1},
		{name: "once failure", args: []string{"--once"}, want: modeOnce, err: errors.New("endpoint failed"), code: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			cmd := command{version: "test-version", stdout: &output, stderr: io.Discard}
			ctx := context.WithValue(context.Background(), struct{}{}, "invocation")
			calls := 0
			execute := func(gotCtx context.Context, gotMode mode, config notifier.Config, version string, log *slog.Logger) error {
				calls++
				if gotCtx != ctx || gotMode != tc.want || version != cmd.version || config.Interval != 2*time.Minute || len(config.Endpoints) != 1 || log == nil {
					t.Fatalf("wrong execution arguments: mode=%d config=%+v version=%s", gotMode, config, version)
				}
				return tc.err
			}
			args := append([]string{"--config", path}, tc.args...)
			if code := cmd.run(ctx, args, 1000, execute); code != tc.code || calls != 1 {
				t.Fatalf("exit %d, calls %d; want %d, 1", code, calls, tc.code)
			}
			if tc.err != nil && !strings.Contains(output.String(), tc.err.Error()) {
				t.Fatal("runtime failure was not logged")
			}
		})
	}
}

func TestRepeatedInvocationsHaveIndependentFlags(t *testing.T) {
	clearConfigEnvironment(t)
	path := validConfig(t)
	cmd := command{stdout: io.Discard, stderr: io.Discard}
	var modes []mode
	execute := func(_ context.Context, mode mode, _ notifier.Config, _ string, _ *slog.Logger) error {
		modes = append(modes, mode)
		return nil
	}
	for _, args := range [][]string{{"--config", path, "--once"}, {"--config", path}} {
		if code := cmd.run(context.Background(), args, 1000, execute); code != 0 {
			t.Fatalf("exit %d", code)
		}
	}
	if len(modes) != 2 || modes[0] != modeOnce || modes[1] != modeDaemon {
		t.Fatalf("flags leaked between invocations: %v", modes)
	}
}

func TestCancellationIsForwardedToExecution(t *testing.T) {
	clearConfigEnvironment(t)
	cmd := command{stdout: io.Discard, stderr: io.Discard}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	execute := func(ctx context.Context, _ mode, _ notifier.Config, _ string, _ *slog.Logger) error {
		return ctx.Err()
	}
	if code := cmd.run(ctx, []string{"--config", validConfig(t), "--once"}, 1000, execute); code != 1 {
		t.Fatalf("cancelled one-shot exit %d; want 1", code)
	}
}

func TestPublicRunCanBeCalledRepeatedly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for i := 0; i < 2; i++ {
		if code := Run(context.Background(), []string{"--version"}, "1.0.0", &stdout, &stderr); code != 0 {
			t.Fatal(code)
		}
	}
	if stdout.String() != "1.0.0\n1.0.0\n" || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
	}
}
