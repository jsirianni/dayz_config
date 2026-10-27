//go:build linux

package deploy

import "os"

func protectPrivateDirectory(file string) error { return os.Chmod(file, 0700) }
func protectPrivateFile(file string) error      { return os.Chmod(file, 0600) }
