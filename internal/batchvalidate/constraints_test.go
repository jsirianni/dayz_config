package batchvalidate

import (
	"go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestPackageUsesNoExecutionOrNativeBridge(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	directory := filepath.Dir(currentFile)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := map[string]bool{"C": true, "os/exec": true, "syscall": true, "plugin": true}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		file, err := parser.ParseFile(gotoken.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imported := range file.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", path, err)
			}
			if forbidden[name] {
				t.Errorf("%s imports forbidden package %q", entry.Name(), name)
			}
		}
	}
}
