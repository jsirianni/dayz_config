//go:build linux

package deploy

import (
	"context"
	"errors"
	"io"
)

func RunEnrollment(context.Context, string, io.Writer) error {
	return errors.New("host enrollment must execute on Windows")
}
func RelaySteamAuth(context.Context, HostConfig, io.Reader, io.Writer) error {
	return errors.New("Steam authentication relay must execute on Windows")
}
func startAuthBroker(context.Context, HostConfig) (func(), error) { return func() {}, nil }
