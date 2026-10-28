//go:build linux

package deploy

import (
	"context"
	"errors"
	"io"
)

func SubmitAgentUpgrade(context.Context, string, io.Writer) error {
	return errors.New("agent upgrade submission runs only on the Windows host")
}
func RunUpgradeCoordinator(string) error {
	return errors.New("agent upgrade coordinator runs only on the Windows host")
}
func RunUpgradeRetirement(string) error {
	return errors.New("legacy retirement guardian runs only on the Windows host")
}
