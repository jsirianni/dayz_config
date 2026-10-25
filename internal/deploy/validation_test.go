package deploy

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("DZDEPLOY_ARGS_TEST_HELPER") == "1" {
		fmt.Print(strings.Join(os.Args[1:], "|"))
		os.Exit(0)
	}
	if os.Getenv("DZDEPLOY_STEAM_TEST_HELPER") == "1" {
		input := bufio.NewReader(os.Stdin)
		fmt.Fprint(os.Stdout, "Please enter your pass")
		time.Sleep(10 * time.Millisecond)
		fmt.Fprint(os.Stdout, "word: ")
		pw, err := input.ReadString('\n')
		if err != nil || pw != "private-password\n" {
			os.Exit(4)
		}
		// Deliberately emulate unsafe vendor output: the broker must redact it.
		fmt.Fprintln(os.Stdout, "echoed private-password")
		fmt.Fprint(os.Stdout, "Enter the Steam Guard code from your email: ")
		code, err := input.ReadString('\n')
		if err != nil || code != "private-guard-code\n" {
			os.Exit(5)
		}
		fmt.Fprintln(os.Stdout, "Logged in OK")
		os.Exit(0)
	}
	if mode := os.Getenv("DZDEPLOY_TEST_VALIDATOR"); mode != "" {
		switch mode {
		case "fail":
			fmt.Println("error: invalid configuration")
			os.Exit(1)
		case "help":
			fmt.Println("Usage:\n  dzcli [command]")
		case "warn":
			fmt.Println("warning: known condition")
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestValidationFailsClosed(t *testing.T) {
	root := t.TempDir()
	writeTest(t, root, "serverDZ.cfg", "hostname = \"test\";")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"fail", "help", "warn"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DZDEPLOY_TEST_VALIDATOR", mode)
			r, err := Validate(context.Background(), self, root)
			if mode == "warn" {
				if err != nil || r.Failures != 0 || r.Warnings != 1 {
					t.Fatal(r, err)
				}
			} else if err == nil || r.Failures != 1 {
				t.Fatal("validator failure not propagated", r, err)
			}
		})
	}
	if _, err := Validate(context.Background(), filepath.Join(root, "missing.exe"), root); err == nil {
		t.Fatal("missing executable accepted")
	}
}
