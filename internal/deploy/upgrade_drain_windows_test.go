//go:build windows

package deploy

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc/mgr"
)

func TestUpgradeRecoveryDelayUsesLongestPendingRestart(t *testing.T) {
	policy := upgradeServicePolicy{Actions: []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: time.Second},
		{Type: mgr.NoAction, Delay: time.Hour},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
	}}
	if got, err := upgradeRecoveryDelay(policy); err != nil || got != 30*time.Second {
		t.Fatalf("restart drain delay = %v, %v", got, err)
	}
	for _, delay := range []time.Duration{-time.Millisecond, maxUpgradeRecoveryDelay + time.Millisecond} {
		policy.Actions = []mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: delay}}
		if _, err := upgradeRecoveryDelay(policy); err == nil {
			t.Fatalf("unsupported restart delay %v was accepted", delay)
		}
	}
	if got, err := upgradeRecoveryDelay(upgradeServicePolicy{}); err != nil || got != 0 {
		t.Fatalf("empty recovery policy needs no drain: %v, %v", got, err)
	}
	policy.Actions = []mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: maxUpgradeRecoveryDelay}}
	if got, err := upgradeRecoveryDelay(policy); err != nil || got != maxUpgradeRecoveryDelay {
		t.Fatalf("supported maximum restart delay = %v, %v", got, err)
	}
}

func TestUpgradeRecoveryDrainUsesConfirmedDisabledTime(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		disabledAt time.Time
		want       time.Duration
	}{
		{"missing-confirmation", time.Time{}, 30 * time.Second},
		{"clock-moved-back", now.Add(time.Minute), 30 * time.Second},
		{"just-disabled", now, 30 * time.Second},
		{"partially-drained", now.Add(-10 * time.Second), 20 * time.Second},
		{"exactly-drained", now.Add(-30 * time.Second), 0},
		{"previous-boot-confirmation-elapsed", now.Add(-time.Hour), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := upgradeRecoveryDrainRemaining(30*time.Second, tc.disabledAt, now)
			if err != nil || got != tc.want {
				t.Fatalf("remaining disabled drain = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	if _, err := upgradeRecoveryDrainRemaining(-time.Second, now, now); err == nil {
		t.Fatal("negative drain interval was accepted")
	}
}

func TestUpgradeRecoveryDrainWaitCanBeCancelled(t *testing.T) {
	policy := upgradeServicePolicy{Actions: []mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 30 * time.Second}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitUpgradeRecoveryDrain(ctx, policy, time.Now().UTC()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled drain wait = %v", err)
	}
	if err := waitUpgradeRecoveryDrain(context.Background(), policy, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("elapsed drain wait failed: %v", err)
	}
}
