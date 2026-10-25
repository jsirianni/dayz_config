package deploy

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var reserved = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])(?:\.|$)`)

func SafeRelative(p string) error {
	if p == "" || strings.ContainsAny(p, "\\:\x00\r\n") || strings.HasPrefix(p, "/") || path.Clean(p) != p {
		return fmt.Errorf("unsafe relative path %q", p)
	}
	for _, s := range strings.Split(p, "/") {
		if s == "." || s == ".." || strings.TrimRight(s, " .") != s || reserved.MatchString(s) || strings.ContainsAny(s, `<>"|?*`) {
			return fmt.Errorf("unsafe path component %q", s)
		}
	}
	return nil
}
func SafeJoin(root, rel string) (string, error) {
	if err := SafeRelative(rel); err != nil {
		return "", err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	// Check the root and all existing parents too: a junction above the leaf is unsafe.
	for current := root; ; current = filepath.Dir(current) {
		if err := noLink(current); err != nil {
			return "", err
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	current := root
	for _, part := range strings.Split(rel, "/") {
		current = filepath.Join(current, part)
		if err := noLink(current); err != nil {
			return "", err
		}
	}
	if !within(root, current) {
		return "", errors.New("path escaped root")
	}
	return current, nil
}
func noLink(p string) error {
	fi, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 || isReparse(p) {
		return fmt.Errorf("links/reparse points are not allowed: %s", p)
	}
	return nil
}

// Destination is the single ownership policy shared by package builder and host.
// A new runtime directory must be deliberately added here before it can be owned.
func Destination(t Target, source string) (string, bool) {
	if SafeRelative(source) != nil {
		return "", false
	}
	lower := strings.ToLower(source)
	ext := strings.ToLower(path.Ext(source))
	for _, part := range strings.Split(lower, "/") {
		if strings.HasPrefix(part, "storage_") || part == "logs" || part == "playerdata" || part == "atm" || part == "validation" || part == "scripts" || part == "docs" || part == "backups" {
			return "", false
		}
	}
	if strings.HasSuffix(lower, "/credentials.txt") || strings.HasSuffix(lower, "/persistentserverdata.json") {
		return "", false
	}
	switch source {
	case "serverDZ.cfg", "dayz-service.bat", "dayz-service-manager.bat", "profiles/3PPVehicleWhitelist.json", "profiles/VPPAdminTools/Permissions/SuperAdmins/SuperAdmins.txt":
		return source, true
	}
	if ext != ".xml" && ext != ".json" && ext != ".cfg" && ext != ".c" {
		return "", false
	}
	prefix := "mpmissions/" + t.Mission + "/"
	if strings.HasPrefix(source, prefix) {
		return source, true
	}
	if t.Mission == "empty.deerisle" && strings.HasPrefix(source, "empty.deerisle/") {
		return "mpmissions/" + source, true
	}
	for _, p := range []string{"profiles/ExpansionMod/Settings/", "profiles/ExpansionMod/Loadouts/", "profiles/ExpansionMod/Market/", "profiles/ExpansionMod/Traders/", "profiles/ExpansionMod/AI/FSM/", "profiles/ExpansionMod/AI/LootDrops/", "profiles/ExpansionMod/Quests/NPCs/", "profiles/ExpansionMod/Quests/Objectives/", "profiles/ExpansionMod/Quests/Quests/", "profiles/TerjeSettings/"} {
		if strings.HasPrefix(source, p) {
			return source, true
		}
	}
	return "", false
}
