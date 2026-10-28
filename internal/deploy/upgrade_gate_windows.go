//go:build windows

package deploy

// Generic write includes READ_CONTROL/SYNCHRONIZE, which directory readers also
// request. Deny only mutation bits so inspection continues while uploads stop.
func gateUpgradeInbox(path, sid string) error {
	return upgradeSetDACL(path, "D:P(D;OICI;0x00010156;;;BA)(D;OICI;0x00010156;;;"+sid+")(A;OICI;FA;;;SY)(A;OICI;FRFX;;;BA)(A;OICI;FRFX;;;"+sid+")")
}
