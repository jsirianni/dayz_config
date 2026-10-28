package deploy

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestAgentBuildIdentityAndUpgradeProvenance(t *testing.T) {
	revision := strings.Repeat("a", 40)
	info := &debug.BuildInfo{Path: agentModule + "/cmd/dzdeploy", Main: debug.Module{Path: agentModule}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: revision}, {Key: "vcs.modified", Value: "false"}, {Key: "GOOS", Value: "windows"}, {Key: "GOARCH", Value: "amd64"}}}
	id, err := identityFromBuildInfo(info)
	if err != nil {
		t.Fatal(err)
	}
	id.SHA256 = strings.Repeat("b", 64)
	if err = ValidateUpgradeCandidate(id, revision); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*AgentIdentity){func(i *AgentIdentity) { i.Dirty = true }, func(i *AgentIdentity) { i.Revision = "" }, func(i *AgentIdentity) { i.Revision = strings.Repeat("c", 40) }, func(i *AgentIdentity) { i.GOOS = "linux" }, func(i *AgentIdentity) { i.GOARCH = "arm64" }, func(i *AgentIdentity) { i.Protocol++ }, func(i *AgentIdentity) { i.SHA256 = "" }} {
		bad := id
		change(&bad)
		if ValidateUpgradeCandidate(bad, revision) == nil {
			t.Fatal("accepted incompatible or unproven artifact", bad)
		}
	}
	info.Settings = info.Settings[:1]
	unknown, err := identityFromBuildInfo(info)
	if err != nil || !unknown.Dirty {
		t.Fatal("missing cleanliness must remain unverified", unknown, err)
	}
	info.Main.Path = "other/module"
	if _, err = identityFromBuildInfo(info); err == nil {
		t.Fatal("accepted unrelated module")
	}
}

func TestReadAgentIdentityRejectsNonExecutable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "agent.exe")
	if err := os.WriteFile(file, []byte("not an agent executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAgentIdentity(file); err == nil {
		t.Fatal("accepted unstamped non-executable")
	}
	if _, err := ReadAgentIdentity(filepath.Dir(file)); err == nil {
		t.Fatal("accepted directory")
	}
}
