package commanddocs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunGeneratesDocsWithDefaultOutput(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var stdout bytes.Buffer

	err := Run(nil, &stdout)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertContains(t, stdout.String(), filepath.Join("docs", "commands.md"))
	assertContains(t, readFile(t, filepath.Join(dir, "docs", "commands.md")), "# dzcli Command Reference")
}

func TestRunGeneratesDocsWithConfiguredOutput(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "reference")
	var stdout bytes.Buffer

	err := Run([]string{"--output", outputDir}, &stdout)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	assertContains(t, stdout.String(), filepath.Join(outputDir, "commands.md"))
	assertContains(t, readFile(t, filepath.Join(outputDir, "commands.md")), "dzcli update economy types")
}

func TestRunRejectsInvalidArgs(t *testing.T) {
	tests := [][]string{
		{"--bad", "docs"},
		{"--output", ""},
		{"--output"},
		{"--output", "docs", "extra"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			err := Run(args, io.Discard)
			if err == nil {
				t.Fatal("err = nil, want error")
			}
		})
	}
}

func TestRunReturnsGenerateError(t *testing.T) {
	originalMkdirAll := mkdirAll
	defer func() { mkdirAll = originalMkdirAll }()

	mkdirAll = func(string, os.FileMode) error {
		return errors.New("generate failed")
	}

	err := Run([]string{"--output", "docs"}, io.Discard)

	if err == nil {
		t.Fatal("err = nil, want generate error")
	}
	assertContains(t, err.Error(), "generate failed")
}

func TestMainReturnsSuccess(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dir := t.TempDir()

	code := Main([]string{"--output", dir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	assertContains(t, stdout.String(), filepath.Join(dir, "commands.md"))
	assertEqual(t, stderr.String(), "")
}

func TestMainWritesErrorToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"--bad"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	assertEqual(t, stdout.String(), "")
	assertContains(t, stderr.String(), "usage")
}

func TestGenerateReturnsMkdirError(t *testing.T) {
	originalMkdirAll := mkdirAll
	defer func() { mkdirAll = originalMkdirAll }()

	mkdirAll = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}

	_, err := Generate("docs")

	if err == nil {
		t.Fatal("err = nil, want mkdir error")
	}
	assertContains(t, err.Error(), "mkdir failed")
}

func TestGenerateReturnsWriteError(t *testing.T) {
	originalWriteFile := writeFile
	defer func() { writeFile = originalWriteFile }()

	writeFile = func(string, []byte, os.FileMode) error {
		return errors.New("write failed")
	}

	_, err := Generate(t.TempDir())

	if err == nil {
		t.Fatal("err = nil, want write error")
	}
	assertContains(t, err.Error(), "write failed")
}

func TestRenderIncludesCommandDetails(t *testing.T) {
	root := &cobra.Command{
		Use:     "sample",
		Short:   "Sample short",
		Long:    "Sample long\n\ntext",
		Example: "sample child --name value",
	}
	root.PersistentFlags().StringP("config", "c", "", "config | file")
	root.Flags().Bool("verbose", false, "show\nmore")
	hidden := &cobra.Command{Use: "hidden", Hidden: true}
	child := &cobra.Command{Use: "child", Short: "Child command"}
	child.Flags().String("name", "default", "name value")
	root.AddCommand(hidden)
	root.AddCommand(child)

	output := Render(root)

	assertContains(t, output, "# dzcli Command Reference")
	assertContains(t, output, "## sample")
	assertContains(t, output, "Sample short")
	assertContains(t, output, "Sample long")
	assertContains(t, output, "sample child --name value")
	assertContains(t, output, "| `child` | Child command |")
	assertContains(t, output, "| `--verbose` | `false` | show<br>more |")
	assertContains(t, output, "| `-c, --config` | `` | config \\| file |")
	assertContains(t, output, "## sample child")
	assertContains(t, output, "| `--name` | `default` | name value |")
	assertNotContains(t, output, "hidden")
}

func TestFlagRowsSkipHiddenFlags(t *testing.T) {
	command := &cobra.Command{Use: "sample"}
	command.Flags().Bool("visible", false, "visible flag")
	command.Flags().Bool("hidden", false, "hidden flag")
	flag := command.Flags().Lookup("hidden")
	flag.Hidden = true

	rows := flagRows(command.Flags())

	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	assertEqual(t, rows[0].name, "--visible")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(data)
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}

func assertNotContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("%q contains %q", haystack, needle)
	}
}

func assertEqual[T comparable](t *testing.T, got T, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
