// Package hostinspect collects read-only Windows observations over SSH.
package hostinspect

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf16"
)

type Spec struct {
	Service, Executable string
	QueryPort           int
}
type Process struct {
	SessionStamp string    `json:"sessionStamp,omitempty"`
	PID          int       `json:"pid"`
	Parent       int       `json:"parent"`
	Executable   string    `json:"executable"`
	Token        string    `json:"token"`
	Started      time.Time `json:"started"`
}
type Service struct {
	State      string `json:"state"`
	StartMode  string `json:"startMode"`
	PID        int    `json:"pid"`
	Token      string `json:"token,omitempty"`
	Executable string `json:"executable,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}
type Query struct {
	Name  string `json:"name"`
	Map   string `json:"map"`
	Error string `json:"error,omitempty"`
}
type Snapshot struct {
	At        time.Time `json:"at"`
	Service   Service   `json:"service"`
	Processes []Process `json:"processes"`
	Ports     []int     `json:"ports"`
	Query     Query     `json:"query"`
	Error     string    `json:"error,omitempty"`
}

//go:embed inspect.ps1
var script string

func Command(s Spec) (string, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	// A Windows PowerShell child can inherit PowerShell 7's incompatible module
	// paths. Use the native process's protected built-in modules for inspection.
	source := fmt.Sprintf("$env:PSModulePath=[IO.Path]::Combine($PSHOME,'Modules');$spec = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')) | ConvertFrom-Json\n%s", base64.StdEncoding.EncodeToString(raw), script)
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	if _, err = z.Write([]byte(source)); err != nil {
		return "", err
	}
	if err = z.Close(); err != nil {
		return "", err
	}
	source = fmt.Sprintf("$b=[Convert]::FromBase64String('%s');$m=New-Object IO.MemoryStream(,$b);$z=New-Object IO.Compression.GzipStream($m,[IO.Compression.CompressionMode]::Decompress);$r=New-Object IO.StreamReader($z);try{ & ([scriptblock]::Create($r.ReadToEnd())) }finally{$r.Dispose();$z.Dispose();$m.Dispose()}", base64.StdEncoding.EncodeToString(compressed.Bytes()))
	words := utf16.Encode([]rune(source))
	b := make([]byte, len(words)*2)
	for i, w := range words {
		b[i*2] = byte(w)
		b[i*2+1] = byte(w >> 8)
	}
	return "powershell.exe -NoProfile -NonInteractive -EncodedCommand " + base64.StdEncoding.EncodeToString(b), nil
}
