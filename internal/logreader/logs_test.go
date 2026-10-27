package logreader

import (
	"strings"
	"testing"
)

func TestSecretsAndOversizedLinesNeverAppear(t *testing.T) {
	text := "ordinary log\npassword = hunter2\nAuthorization: Bearer abc123\n+login account steam-pass\n-----BEGIN OPENSSH PRIVATE KEY-----\nshort-key-material\nYWJj\n-----END OPENSSH PRIVATE KEY-----\n" + strings.Repeat("x", 20000) + "\nfinal line\n"
	out := CleanText(text)
	for _, secret := range []string{"hunter2", "abc123", "steam-pass", "short-key-material", "YWJj", strings.Repeat("x", 20)} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret appeared: %s", secret)
		}
	}
	if !strings.Contains(out, "ordinary log") || !strings.Contains(out, "final line") {
		t.Fatal(out)
	}
}

func TestTailBoundsAndIncompleteCredentialLines(t *testing.T) {
	text := strings.Repeat("a", MaxBytes) + "\nfirst safe line\npassword=split-secret"
	excerpt, err := Tail(strings.NewReader(text), int64(len(text)), 200, true)
	if err != nil || !excerpt.Truncated || excerpt.BytesRead > MaxBytes || strings.Contains(excerpt.Text, "split-secret") || !strings.Contains(excerpt.Text, "first safe line") {
		t.Fatal(excerpt, err)
	}
	excerpt, err = Tail(strings.NewReader("one\ntwo\nthree\n"), 14, 2, false)
	if err != nil || excerpt.Text != "two\nthree\n" || !excerpt.Truncated {
		t.Fatal(excerpt, err)
	}
}

func TestIssueGroupingPreservesSessionAndSources(t *testing.T) {
	a := Summarize("12:00:00 ERROR: NULL pointer\n12:00:01 ERROR: NULL pointer\n", "a", "current")
	b := Summarize("12:00:02 ERROR: NULL pointer\n", "b", "current")
	old := Summarize("12:00:03 ERROR: NULL pointer\n", "old", "historical")
	merged := MergeIssues(append(append(a, b...), old...))
	if len(merged) != 2 || merged[0].Count != 3 || len(merged[0].Sources) != 2 || merged[1].Count != 1 {
		t.Fatal(merged)
	}
}
