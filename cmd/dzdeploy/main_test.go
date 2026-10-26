package main

import (
	"errors"
	"testing"

	"github.com/jsirianni/dayz_config/internal/deploy"
)

func TestEnrollmentAccountInsideDeploy(t *testing.T) {
	for _, initial := range []string{"", "REPLACE_STEAM_ACCOUNT", "configured"} {
		for _, dry := range []bool{false, true} {
			host := &deploy.HostConfig{SteamUser: initial}
			calls := 0
			err := enrollmentAccount(host, dry, func(string) (string, error) {
				calls++
				return " player ", nil
			})
			if err != nil {
				t.Fatal(err)
			}
			wantCalls := 0
			if !dry && initial != "configured" {
				wantCalls = 1
				if host.SteamUser != "player" {
					t.Fatal("prompt answer not used")
				}
			}
			if calls != wantCalls || host.SteamUser == "" {
				t.Fatal(initial, dry, calls, host.SteamUser)
			}
		}
	}
	for _, answer := range []string{"", "REPLACE_STEAM_ACCOUNT", "bad account", "bad\naccount"} {
		if err := enrollmentAccount(&deploy.HostConfig{}, false, func(string) (string, error) { return answer, nil }); err == nil {
			t.Fatal("invalid account accepted", answer)
		}
	}
	want := errors.New("terminal unavailable")
	if err := enrollmentAccount(&deploy.HostConfig{}, false, func(string) (string, error) { return "", want }); !errors.Is(err, want) {
		t.Fatal("prompt error lost", err)
	}
}

func TestRejectsAmbiguousCLIInput(t *testing.T) {
	for _, args := range [][]string{{"deploy"}, {"deploy", "unknown"}, {"deploy", "nasdara", "--offline"}, {"restart", "nasdara", "--dry-run"}, {"status", "nasdara", "--timeout", "0"}, {"agent"}, {"tools", "unknown"}, {"rollback", "nasdara", "--deployment", "../../bad"}} {
		if err := run(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
