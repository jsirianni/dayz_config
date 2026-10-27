package logreader

import (
	"strings"
	"testing"
	"time"
)

func readFollow(t *testing.T, f *Follower, name, text string, modified time.Time, lines int) FollowExcerpt {
	t.Helper()
	out, err := f.Read(strings.NewReader(text), name, int64(len(text)), modified, lines)
	if err != nil {
		t.Fatal(err)
	}
	if out.BytesRead > MaxBytes || len(out.Text) > MaxBytes || len(f.partial) > maxLineBytes {
		t.Fatal("follow bounds exceeded")
	}
	return out
}

func TestFollowerPreservesRepeatedAndRedactionEquivalentAppends(t *testing.T) {
	for _, tt := range []struct{ first, appended, want string }{
		{"repeated error\n", "repeated error\nrepeated error\n", "repeated error\nrepeated error\n"},
		{"password=first\n", "password=second\npassword=third\n", redactedLine + "\n" + redactedLine + "\n"},
	} {
		var f Follower
		now := time.Now()
		readFollow(t, &f, "console", tt.first, now, 1)
		text := tt.first + tt.appended
		out := readFollow(t, &f, "console", text, now, 1)
		if out.Text != tt.want || out.Rotated {
			t.Fatalf("lost appended occurrences: %+v", out)
		}
		if got := readFollow(t, &f, "console", text, now, 1); got.Text != "" {
			t.Fatalf("replayed unchanged bytes: %+v", got)
		}
	}
}

func TestFollowerFramesCredentialsAndKeepsPrivateKeyStateBetweenReads(t *testing.T) {
	var f Follower
	now := time.Now()
	text := "safe initial\npass"
	if out := readFollow(t, &f, "a", text, now, 1); out.Text != "safe initial\n" {
		t.Fatal(out)
	}
	text += "word=split-credential"
	if out := readFollow(t, &f, "a", text, now, 1); out.Text != "" {
		t.Fatal("partial credential exposed", out)
	}
	text += "\n-----BEGIN OPENSSH PRIVATE KEY-----\n"
	if out := readFollow(t, &f, "a", text, now, 1); out.Text != redactedLine+"\n"+redactedLine+"\n" {
		t.Fatal(out)
	}
	text += "short material\n"
	if out := readFollow(t, &f, "a", text, now, 1); out.Text != redactedLine+"\n" {
		t.Fatal("private-key state lost", out)
	}
	text += "-----END OPENSSH PRIVATE KEY-----\nafter key\n"
	if out := readFollow(t, &f, "a", text, now, 1); out.Text != redactedLine+"\nafter key\n" {
		t.Fatal(out)
	}
}

func TestFollowerDetectsSourceChangesTruncationAndRewrites(t *testing.T) {
	now := time.Now()
	for _, tt := range []struct {
		name, text string
		modified   time.Time
	}{
		{"other", "old log\n", now},
		{"same", "new log\n", now},
		{"same", "x\n", now},
		{"same", "old log\n", now.Add(time.Second)},
		{"same", "replacement longer log\n", now},
	} {
		var f Follower
		readFollow(t, &f, "same", "old log\n", now, 10)
		out := readFollow(t, &f, tt.name, tt.text, tt.modified, 10)
		if !out.Rotated || out.Text != tt.text {
			t.Fatalf("rotation lost: %+v", out)
		}
	}
}

func TestFollowerBoundsOversizedPartialLinesAndReadsBacklogSequentially(t *testing.T) {
	var f Follower
	now := time.Now()
	text := strings.Repeat("x", MaxBytes+1000) + "\nafter oversized\n"
	first := readFollow(t, &f, "a", "", now, 1)
	if first.Text != "" {
		t.Fatal(first)
	}
	if out := readFollow(t, &f, "a", text, now, 1); out.Text != "" || out.BytesRead != MaxBytes {
		t.Fatal(out)
	}
	if out := readFollow(t, &f, "a", text, now, 1); out.Text != redactedLine+"\nafter oversized\n" {
		t.Fatal(out)
	}
	// A new source starts with a bounded tail and discards an incomplete first line.
	if out := readFollow(t, &f, "b", text, now, 1); out.Text != "after oversized\n" || !out.Truncated {
		t.Fatal(out)
	}
}

func TestFollowerInitialTailKeepsRedactionContext(t *testing.T) {
	var f Follower
	text := "-----BEGIN PRIVATE KEY-----\nshort material\n"
	out := readFollow(t, &f, "a", text, time.Time{}, 1)
	if out.Text != redactedLine+"\n" || !out.Truncated {
		t.Fatal(out)
	}
}
