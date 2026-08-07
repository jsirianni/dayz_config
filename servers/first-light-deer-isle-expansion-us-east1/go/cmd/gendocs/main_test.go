package main

import (
	"os"
	"testing"
)

func TestMainDelegatesToCommandDocs(t *testing.T) {
	originalExit := exit
	originalArgs := os.Args
	defer func() {
		exit = originalExit
		os.Args = originalArgs
	}()

	var exitCode int
	exit = func(code int) {
		exitCode = code
	}
	os.Args = []string{"gendocs", "--bad"}

	main()

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
}
