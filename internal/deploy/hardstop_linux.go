//go:build linux

package deploy

import (
	"context"
	"errors"
)

func (g *NativeGame) HardStop(context.Context, State) error {
	return errors.New("native host hard stops require Windows")
}
