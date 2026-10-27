package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImmutableEventFiles(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 2; i++ {
		if err := Write(root, Event{At: time.Now().UTC(), Target: "nasdara", Kind: "launch", PID: 42}); err != nil {
			t.Fatal(err)
		}
	}
	files, err := os.ReadDir(filepath.Join(root, "events"))
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
	for _, name := range files {
		f, err := os.Open(filepath.Join(root, "events", name.Name()))
		if err != nil {
			t.Fatal(err)
		}
		e, err := Decode(f)
		f.Close()
		if err != nil || e.PID != 42 {
			t.Fatal(e, err)
		}
	}
	if _, err = Decode(strings.NewReader(strings.Repeat("x", 65537))); err == nil {
		t.Fatal("unbounded event accepted")
	}
}
