//go:build windows

package deploy

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sys/windows/svc/mgr"
)

const maxUpgradeRecoveryDelay = 10 * time.Minute

// upgradeRecoveryDelay bounds the time for which an already scheduled SCM
// restart could remain pending after the managed service is disabled. Resetting
// recovery actions alone does not cancel a previously queued restart.
func upgradeRecoveryDelay(policy upgradeServicePolicy) (time.Duration, error) {
	var delay time.Duration
	for _, action := range policy.Actions {
		if action.Type != mgr.ServiceRestart {
			continue
		}
		if action.Delay < 0 || action.Delay > maxUpgradeRecoveryDelay {
			return 0, errors.New("agent upgrade requires SCM restart delays between zero and ten minutes")
		}
		if action.Delay > delay {
			delay = action.Delay
		}
	}
	return delay, nil
}

// upgradeRecoveryDrainRemaining uses the confirmed disabled time, not the
// earlier suppression intent. Missing or future timestamps cannot prove elapsed
// disabled time, so replay waits the entire interval conservatively.
func upgradeRecoveryDrainRemaining(delay time.Duration, disabledAt, now time.Time) (time.Duration, error) {
	if delay < 0 || delay > maxUpgradeRecoveryDelay {
		return 0, errors.New("invalid agent upgrade recovery drain interval")
	}
	if delay == 0 {
		return 0, nil
	}
	if disabledAt.IsZero() || disabledAt.After(now) {
		return delay, nil
	}
	elapsed := now.Sub(disabledAt)
	if elapsed >= delay {
		return 0, nil
	}
	return delay - elapsed, nil
}

// waitUpgradeRecoveryDrain assumes the caller keeps the managed service
// disabled for this entire wait. The single relative timer uses monotonic time
// so later wall-clock adjustments do not shorten the verified disabled period.
func waitUpgradeRecoveryDrain(ctx context.Context, policy upgradeServicePolicy, disabledAt time.Time) error {
	delay, err := upgradeRecoveryDelay(policy)
	if err != nil {
		return err
	}
	remaining, err := upgradeRecoveryDrainRemaining(delay, disabledAt, time.Now())
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if remaining == 0 {
		return nil
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
