package vm

import (
	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"strings"
	"testing"
)

func TestPatchPreservesSubfields(t *testing.T) {
	v, e := MergeProperty("virtio=02:00:00:00:00:01,bridge=vmbr0,firewall=1,queues=4,tag=20", map[string]string{"bridge": "vmbr1", "tag": ""}, "e1000")
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"e1000=02:00:00:00:00:01", "firewall=1", "queues=4", "bridge=vmbr1"} {
		if !strings.Contains(v, s) {
			t.Fatal(v)
		}
	}
	if strings.Contains(v, "tag=") {
		t.Fatal(v)
	}
	cpu, e := HardwareChanges(pve.Config{"cpu": "host,flags=+aes,hidden=1"}, spec.Hardware{CPU: "x86-64-v2-AES"})
	if e != nil || cpu.Get("cpu") != "x86-64-v2-AES,flags=+aes,hidden=1" {
		t.Fatal(cpu, e)
	}
}
func TestAbsoluteSizes(t *testing.T) {
	for _, s := range []string{"100G", "100GiB", "100GB"} {
		n, e := SizeBytes(s)
		if e != nil || n != 100<<30 {
			t.Fatal(s, n, e)
		}
	}
	for _, s := range []string{"+20G", "-1G", "0G", "NaN", "999999999999999T"} {
		if _, e := SizeBytes(s); e == nil {
			t.Fatal(s)
		}
	}
}
