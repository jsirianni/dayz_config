// Package testutil provides a local, TLS-enabled Proxmox contract fixture.
package testutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"golang.org/x/crypto/ssh"
)

type Request struct {
	Method, Path string
	Values       url.Values
}
type API struct {
	Server              *httptest.Server
	mu                  sync.Mutex
	VMs                 map[int]pve.Config
	Requests            []Request
	TaskExit, TaskState string
	LoseCloneReply      bool
	Override            func(http.ResponseWriter, *http.Request) bool
}

func New(t *testing.T) *API {
	a := &API{VMs: map[int]pve.Config{9000: {"template": 1, "name": "ubuntu", "ostype": "l26", "agent": "1,fstrim_cloned_disks=1", "scsi0": "local-lvm:base-9000,size=32G", "sata0": "local-lvm:cloudinit,media=cdrom", "net0": "virtio=02:00:00:00:00:01,bridge=vmbr0,firewall=1,queues=4", "cpu": "host,flags=+aes", "cores": 2, "memory": 2048, "digest": "digest1"}}, TaskExit: "OK", TaskState: "stopped"}
	a.Server = httptest.NewTLSServer(http.HandlerFunc(a.handle))
	t.Cleanup(a.Server.Close)
	return a
}
func (a *API) Client() *pve.Client {
	c := pve.New(a.Server.URL, "ops@pve!cli", "top-secret-token")
	c.HTTP = a.Server.Client()
	c.Poll = time.Millisecond
	return c
}
func Data(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v})
}
func (a *API) handle(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	a.mu.Lock()
	a.Requests = append(a.Requests, Request{r.Method, r.URL.Path, r.Form})
	a.mu.Unlock()
	if r.Header.Get("Authorization") != "PVEAPIToken=ops@pve!cli=top-secret-token" {
		http.Error(w, "bad auth", 401)
		return
	}
	if a.Override != nil && a.Override(w, r) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p := strings.TrimPrefix(r.URL.Path, "/api2/json")
	switch p {
	case "/version":
		Data(w, map[string]string{"version": "9.1.2"})
		return
	case "/cluster/resources":
		rs := []pve.Resource{}
		for id, c := range a.VMs {
			status := c.String("status")
			if status == "" {
				status = "stopped"
			}
			rs = append(rs, pve.Resource{VMID: id, Node: "pve", Name: c.String("name"), Type: "qemu", Template: c.Int("template"), Status: status})
		}
		Data(w, rs)
		return
	case "/nodes":
		Data(w, []any{map[string]string{"node": "pve", "status": "online"}})
		return
	case "/nodes/pve/storage":
		Data(w, []any{map[string]any{"storage": "local-lvm", "active": 1, "enabled": 1, "content": "images"}, map[string]any{"storage": "local", "active": 1, "enabled": 1, "content": "iso"}})
		return
	case "/nodes/pve/network":
		Data(w, []any{map[string]any{"iface": "vmbr0", "type": "bridge", "bridge_vlan_aware": 1}})
		return
	case "/pools/lab":
		Data(w, map[string]any{"poolid": "lab"})
		return
	case "/access/permissions":
		path := r.URL.Query().Get("path")
		Data(w, map[string]any{path: map[string]int{"VM.Audit": 1, "VM.Allocate": 1, "VM.Clone": 1, "Datastore.AllocateSpace": 1, "Datastore.AllocateTemplate": 1, "VM.PowerMgmt": 1, "VM.Config.CPU": 1, "VM.Config.Memory": 1, "VM.Config.Options": 1, "VM.Config.Disk": 1, "VM.Config.Network": 1, "VM.Config.Cloudinit": 1, "VM.Config.CDROM": 1, "VM.GuestAgent.Audit": 1, "VM.GuestAgent.FileRead": 1}})
		return
	}
	if strings.Contains(p, "/tasks/") {
		if strings.HasSuffix(p, "/status") {
			Data(w, pve.Task{Status: a.TaskState, ExitStatus: a.TaskExit})
		} else {
			Data(w, []any{map[string]string{"t": "fixture task log"}})
		}
		return
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if len(parts) >= 5 && parts[0] == "nodes" && parts[2] == "qemu" {
		id, _ := strconv.Atoi(parts[3])
		cfg, exists := a.VMs[id]
		if !exists {
			http.Error(w, "missing VM", 404)
			return
		}
		action := parts[4]
		if r.Method == "GET" {
			switch action {
			case "config":
				Data(w, cfg)
			case "pending":
				Data(w, []pve.Pending{{Key: "memory", Value: 2048, Pending: 4096}})
			default:
				http.Error(w, "missing endpoint "+p, 404)
			}
			return
		}
		switch action {
		case "clone":
			newID, _ := strconv.Atoi(r.Form.Get("newid"))
			if a.VMs[newID] != nil {
				http.Error(w, "collision", 409)
				return
			}
			clone := pve.Config{}
			for k, v := range cfg {
				clone[k] = v
			}
			clone["template"] = 0
			clone["name"] = r.Form.Get("name")
			clone["description"] = r.Form.Get("description")
			clone["net0"] = "virtio=02:00:00:00:00:c9,bridge=vmbr0,firewall=1,queues=4"
			clone["status"] = "stopped"
			a.VMs[newID] = clone
			if a.LoseCloneReply {
				conn, _, _ := w.(http.Hijacker).Hijack()
				conn.Close()
				return
			}
		case "config":
			if d := r.Form.Get("digest"); d != "" && d != cfg.String("digest") {
				http.Error(w, "digest mismatch", 409)
				return
			}
			for k, v := range r.Form {
				if k != "digest" {
					cfg[k] = v[0]
				}
			}
			cfg["digest"] = "digest2"
		case "resize":
			cfg[r.Form.Get("disk")] = "local-lvm:vm-disk,size=" + r.Form.Get("size")
		case "status":
			if len(parts) > 5 && parts[5] == "start" {
				cfg["status"] = "running"
			} else {
				cfg["status"] = "stopped"
			}
		default:
			http.Error(w, "unexpected mutation "+p, 400)
			return
		}
		Data(w, fmt.Sprintf("UPID:pve:1:1:1:qm:%d:%d:", id, len(a.Requests)))
		return
	}
	http.Error(w, "unexpected endpoint "+p, 404)
}
func (a *API) Writes() []Request {
	a.mu.Lock()
	defer a.mu.Unlock()
	var rs []Request
	for _, r := range a.Requests {
		if r.Method != "GET" {
			rs = append(rs, r)
		}
	}
	return rs
}
func (a *API) SetVM(id int, c pve.Config) { a.mu.Lock(); defer a.mu.Unlock(); a.VMs[id] = c }
func (a *API) GetVM(id int) pve.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := pve.Config{}
	for k, v := range a.VMs[id] {
		out[k] = v
	}
	return out
}
func Key(t *testing.T) string {
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	b, e := ssh.MarshalPrivateKey(key, "")
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "id_ed25519")
	if e = os.WriteFile(p, pem.EncodeToMemory(b), 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func Deploy(t *testing.T) spec.Deploy {
	return spec.Deploy{APIVersion: spec.Version, Kind: "VMDeployment", Context: "lab", TemplateVMID: 9000, VMID: 201, Name: "new-vm", Node: "pve", Storage: "local-lvm", Pool: "lab", Hardware: spec.Hardware{Cores: 4, MemoryMiB: 4096, Disk: "scsi0"}, Network: spec.Network{Interface: "net0", Bridge: "vmbr0", IPv4: "dhcp"}, Guest: spec.Guest{OS: "ubuntu-24.04", User: "pveadmin", KeyFile: Key(t), Port: 22}}
}
