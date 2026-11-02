package logreader

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func recordLine(level, message string) string {
	b, _ := json.Marshal(Record{Level: level, Message: message, Timestamp: time.Now().UTC()})
	return string(b) + "\n"
}

func TestJSONRedactionAndGrouping(t *testing.T) {
	input := recordLine("info", "ordinary <text>") + recordLine("error", "password=hunter2") + recordLine("info", "-----BEGIN OPENSSH PRIVATE KEY-----") + recordLine("info", "short-key-material") + recordLine("info", strings.Repeat("a", 48)) + recordLine("info", "-----END OPENSSH PRIVATE KEY-----") + recordLine("info", "after key")
	out := CleanText(input)
	for _, secret := range []string{"hunter2", "short-key-material", strings.Repeat("a", 40)} {
		if strings.Contains(out, secret) {
			t.Fatal("secret leaked")
		}
	}
	for i, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		r, ok := DecodeRecord(line)
		if !ok {
			t.Fatal(line)
		}
		if i > 0 && i < 6 && r.Message != redactedLine {
			t.Fatal(r)
		}
	}
	// A bounded tail may begin inside a block whose END marker is JSON-escaped.
	text := recordLine("info", "short-private-data") + recordLine("info", "-----END OPENSSH PRIVATE KEY-----")
	text = strings.ReplaceAll(text, "-----", `\u002d\u002d\u002d\u002d\u002d`)
	if strings.Contains(CleanText(text), "short-private-data") {
		t.Fatal("escaped end marker concealed key context")
	}
	issues := Summarize(recordLine("error", "same failure")+recordLine("error", "same failure")+recordLine("info", "warning: low disk")+recordLine("info", "ordinary"), "console", "mixed")
	if len(issues) != 2 || issues[0].Message != "same failure" || issues[0].Count != 2 {
		t.Fatal(issues)
	}
}

func TestJSONFollowPartialPrivateKeyAndRotation(t *testing.T) {
	var f Follower
	now := time.Now()
	first := recordLine("info", "-----BEGIN OPENSSH PRIVATE KEY-----")
	partial := recordLine("info", "key-material")
	text := first + partial[:len(partial)/2]
	out, err := f.Read(strings.NewReader(text), "console", int64(len(text)), now, 20)
	if err != nil || strings.Contains(out.Text, "key-material") {
		t.Fatal(out, err)
	}
	text = first + partial + recordLine("info", "-----END OPENSSH PRIVATE KEY-----")
	out, err = f.Read(strings.NewReader(text), "console", int64(len(text)), now.Add(time.Second), 20)
	if err != nil || strings.Contains(out.Text, "key-material") {
		t.Fatal(out, err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(out.Text, "\n"), "\n") {
		if _, ok := DecodeRecord(line); !ok {
			t.Fatal("invalid redacted JSON", line)
		}
	}
	text = recordLine("info", "replacement")
	out, err = f.Read(strings.NewReader(text), "console", int64(len(text)), now.Add(2*time.Second), 20)
	if err != nil || !out.Rotated || !strings.Contains(out.Text, "replacement") {
		t.Fatal(out, err)
	}
}

func gzipText(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCompressedTailBoundsAndRedacts(t *testing.T) {
	text := strings.Repeat("old line\n", 300000) + recordLine("error", "password=hidden") + recordLine("info", "final")
	out, err := CompressedTail(bytes.NewReader(gzipText(t, text)), 2)
	if err != nil || !out.Truncated || strings.Contains(out.Text, "hidden") || !strings.Contains(out.Text, "final") || len(out.Text) > MaxBytes {
		t.Fatal(out, err)
	}
	if _, err := CompressedTail(strings.NewReader("not gzip"), 20); err == nil {
		t.Fatal("accepted corrupt archive")
	}
	compressed := gzipText(t, strings.Repeat("x", MaxArchiveBytes+1))
	out, err = CompressedTail(bytes.NewReader(compressed), 20)
	if err == nil || !out.Truncated || out.Text != "" {
		t.Fatal("decompression limit not enforced", out, err)
	}
}

func TestJSONRedactionKeepsExcerptBounded(t *testing.T) {
	text := strings.Repeat(recordLine("info", "password=x"), 40000)
	out, err := Tail(strings.NewReader(text), int64(len(text)), 10000, false)
	if err != nil || len(out.Text) > MaxBytes || strings.Contains(out.Text, "password=x") {
		t.Fatal("unbounded JSON redaction", err, len(out.Text))
	}
}
