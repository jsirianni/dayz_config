package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
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
