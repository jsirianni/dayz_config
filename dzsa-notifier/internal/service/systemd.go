package service

import "github.com/coreos/go-systemd/v22/daemon"

func notifySystemd(state string) error {
	// A missing NOTIFY_SOCKET is normal when running in a container or terminal.
	_, err := daemon.SdNotify(false, state)
	return err
}
