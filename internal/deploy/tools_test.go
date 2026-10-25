package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestToolDownloadVerifiesBeforeReplacing(t *testing.T) {
	for _, good := range []bool{true, false} {
		t.Run(fmt.Sprint(good), func(t *testing.T) {
			root := t.TempDir()
			binary := []byte("a release binary")
			name := "dzcli_9.0.0_" + runtime.GOOS + "_" + runtime.GOARCH
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			digest := Digest(binary)
			if !good {
				digest = strings.Repeat("0", 64)
			}
			release := map[string]any{"tag_name": "v9.0.0", "assets": []map[string]string{{"name": name, "browser_download_url": "https://github.com/jsirianni/dzcli/releases/download/v9.0.0/" + name, "digest": "sha256:" + digest}}}
			raw, _ := json.Marshal(release)
			client := &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				body := raw
				if strings.HasPrefix(req.URL.String(), "https://github.com/") {
					body = binary
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
			})}
			p, err := ensureDz(context.Background(), root, false, client)
			if good {
				if err != nil {
					t.Fatal(err)
				}
				if readTest(t, filepath.Dir(p), filepath.Base(p)) != string(binary) {
					t.Fatal("wrong binary")
				}
			} else {
				if err == nil {
					t.Fatal("bad digest accepted")
				}
				if _, err = os.Stat(DzPath(root)); !os.IsNotExist(err) {
					t.Fatal("unverified binary installed")
				}
			}
		})
	}
}
