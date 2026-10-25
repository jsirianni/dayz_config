package main

import "testing"

func TestClientDistributionPlatforms(t *testing.T) {
	for _, platform := range []string{"windows", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			if !supportedClient(platform, arch) {
				t.Fatalf("supported client rejected: %s/%s", platform, arch)
			}
		}
	}
	for _, platform := range []string{"darwin", "freebsd", "unsupported"} {
		if supportedClient(platform, "amd64") {
			t.Fatalf("unsupported client accepted: %s", platform)
		}
	}
	if supportedClient("windows", "386") {
		t.Fatal("unsupported architecture accepted")
	}
}
