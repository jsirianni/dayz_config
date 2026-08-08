package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMainFunctionSuccessPath(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })

	os.Args = []string{"dzcli", "validate", "economy", fixturePath(t, "mission", "cfgeconomycore.xml")}
	main()
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	args := append([]string{filepath.Dir(file), "testdata"}, parts...)
	return filepath.Join(args...)
}
