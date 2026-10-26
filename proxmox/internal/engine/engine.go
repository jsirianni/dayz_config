// Package engine coordinates API mutations and their durable local intent.
package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jsirianni/dayz_config/proxmox/internal/guest"
	"github.com/jsirianni/dayz_config/proxmox/internal/operation"
	"github.com/jsirianni/dayz_config/proxmox/internal/pve"
	"github.com/jsirianni/dayz_config/proxmox/internal/spec"
	"github.com/jsirianni/dayz_config/proxmox/internal/vm"
)

type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }
func Input(e error) error {
	if e == nil {
		return nil
	}
	return &Error{2, e}
}
func Unknown(e error) error {
	if e == nil {
		return nil
	}
	return &Error{3, e}
}

type Engine struct {
	Client            *pve.Client
	Store             operation.Store
	Context, Identity string
	Guest             guest.Checker
	Progress          io.Writer
}

func (e *Engine) newResult(kind string, id int, node string) *operation.Result {
	return &operation.Result{APIVersion: spec.Version, Status: "planned", Kind: kind, Context: e.Context, ContextIdentity: e.Identity, VMID: id, Node: node}
}
func (e *Engine) Existing(ctx context.Context, id int) (pve.Resource, pve.Config, error) {
	r, ok, err := e.Client.Find(ctx, id)
	if err != nil {
		return r, nil, err
	}
	if !ok {
		return r, nil, Input(fmt.Errorf("VM %d not found or not visible", id))
	}
	c, err := e.Client.VMConfig(ctx, r.Node, id)
	return r, c, err
}
func (e *Engine) Show(ctx context.Context, id int) (*operation.Result, error) {
	r, c, err := e.Existing(ctx, id)
	if err != nil {
		return nil, err
	}
	pending, err := e.Client.Pending(ctx, r.Node, id)
	result := e.newResult("inspect", id, r.Node)
	result.Status = "succeeded"
	result.Data = map[string]any{"resource": r, "configuration": c}
	result.Pending = pending
	return result, err
}
func (e *Engine) Save(r *operation.Result) error { return e.Store.Save(r) }
func (e *Engine) Fail(r *operation.Result, err error, unknown bool) (*operation.Result, error) {
	r.Status = "failed"
	r.Error = err.Error()
	if unknown {
		r.Status = "unknown"
	}
	if se := e.Save(r); se != nil {
		err = fmt.Errorf("%w; save journal: %v", err, se)
	}
	if unknown {
		return r, Unknown(err)
	}
	return r, err
}
func (e *Engine) Task(ctx context.Context, r *operation.Result, phase, method, path string, v url.Values) error {
	r.Phase = phase
	r.Status = "running"
	r.ActiveTask = ""
	r.ActiveTaskPhase = phase
	r.SubmissionPending = true
	if err := e.Save(r); err != nil {
		return err
	}
	task, err := e.Client.Submit(ctx, method, path, v)
	if err != nil {
		var api *pve.APIError
		if errors.As(err, &api) && (api.Status == 400 || api.Status == 401 || api.Status == 403 || api.Status == 404 || api.Status == 409) {
			r.SubmissionPending = false
			return Input(err)
		}
		return Unknown(err)
	}
	r.Tasks = append(r.Tasks, task)
	r.ActiveTask = task
	r.SubmissionPending = false
	if err = e.Save(r); err != nil {
		return Unknown(err)
	}
	if e.Progress != nil {
		fmt.Fprintf(e.Progress, "%s: %s (%s)\n", r.OperationID, phase, task)
	}
	if err = e.Client.Wait(ctx, r.Node, task); err != nil {
		if taskUncertain(err) {
			return Unknown(err)
		}
		return err
	}
	operation.Complete(r, phase)
	return e.Save(r)
}
func IsUnknown(err error) bool     { var code *Error; return errors.As(err, &code) && code.Code == 3 }
func taskUncertain(err error) bool { var failed *pve.TaskFailure; return !errors.As(err, &failed) }
func (e *Engine) Mutate(ctx context.Context, id int, kind string, changes url.Values, action, disk, size string, dry bool) (*operation.Result, error) {
	rsrc, cfg, err := e.Existing(ctx, id)
	if err != nil {
		return nil, err
	}
	if cfg.Bool("template") {
		return nil, Input(errors.New("use templates only as deployment sources"))
	}
	r := e.newResult(kind, id, rsrc.Node)
	r.Changes = map[string]string{}
	for k := range changes {
		r.Changes[k] = changes.Get(k)
	}
	method, path := "POST", pve.Path(r.Node, id)+"/config"
	if action != "" {
		path = pve.Path(r.Node, id) + "/status/" + action
		r.Changes["power_action"] = action
		changes = url.Values{}
	}
	if disk != "" {
		old, er := vm.DiskSize(cfg, disk)
		if er != nil {
			return nil, Input(er)
		}
		target, er := vm.SizeBytes(size)
		if er != nil {
			return nil, Input(er)
		}
		if target < old {
			return nil, Input(errors.New("disk shrinking is unsupported"))
		}
		if target == old {
			changes = url.Values{}
		} else {
			changes = url.Values{"disk": {disk}, "size": {strconv.FormatInt(target, 10)}}
		}
		method = "PUT"
		path = pve.Path(r.Node, id) + "/resize"
		r.Changes = map[string]string{"disk": disk, "size": size}
	}
	rights := map[string]bool{}
	if action != "" {
		rights["VM.PowerMgmt"] = true
	} else if disk != "" && len(changes) > 0 {
		rights["VM.Config.Disk"] = true
	} else {
		for k := range changes {
			switch {
			case k == "cpu" || k == "cores" || k == "sockets":
				rights["VM.Config.CPU"] = true
			case k == "memory":
				rights["VM.Config.Memory"] = true
			case strings.HasPrefix(k, "net"):
				rights["VM.Config.Network"] = true
			default:
				rights["VM.Config.Options"] = true
			}
		}
	}
	for right := range rights {
		if err = e.Require(ctx, fmt.Sprintf("/vms/%d", id), right); err != nil {
			return nil, err
		}
	}
	if dry {
		r.Pending, err = e.Client.Pending(ctx, r.Node, id)
		return r, err
	}
	if action == "" && len(changes) == 0 {
		r.Status = "succeeded"
		r.Phase = "unchanged"
		r.Data = cfg
		r.Pending, err = e.Client.Pending(ctx, r.Node, id)
		return r, err
	}
	unlock, err := e.Store.Lock(e.Context, id)
	if err != nil {
		return nil, Input(err)
	}
	defer unlock()
	r.OperationID = operation.NewID()
	if action == "" {
		if cfg.String("digest") == "" {
			return nil, Input(errors.New("configuration digest missing"))
		}
		changes.Set("digest", cfg.String("digest"))
	}
	if err = e.Task(ctx, r, kind, method, path, changes); err != nil {
		return e.Fail(r, err, IsUnknown(err))
	}
	if action != "" {
		r.Phase = "verify-power"
		if err = e.Save(r); err != nil {
			return r, err
		}
		want := "stopped"
		if action == "start" || action == "reboot" {
			want = "running"
		}
		for {
			current, found, er := e.Client.Find(ctx, id)
			if er != nil {
				return e.Fail(r, er, true)
			}
			if !found {
				return e.Fail(r, errors.New("VM disappeared while verifying power state"), true)
			}
			if current.Status == want {
				break
			}
			if er = pve.Delay(ctx, e.Client.Poll); er != nil {
				return e.Fail(r, er, true)
			}
		}
	}
	actual, err := e.Client.VMConfig(ctx, r.Node, id)
	if err != nil {
		return e.Fail(r, err, true)
	}
	r.Data = actual
	pending, err := e.Client.Pending(ctx, r.Node, id)
	if err != nil {
		return e.Fail(r, err, true)
	}
	r.Pending = pending
	r.Status = "succeeded"
	r.Phase = "complete"
	return r, e.Save(r)
}
func (e *Engine) Preflight(ctx context.Context, node, storage, pool, bridge string, vlan int, id int) error {
	if _, ok, err := e.Client.Find(ctx, id); err != nil {
		return err
	} else if ok {
		return Input(fmt.Errorf("destination VMID %d already exists", id))
	}
	nodes, err := e.Client.Rows(ctx, "/nodes")
	if err != nil {
		return err
	}
	online := false
	for _, n := range nodes {
		if n.String("node") == node && n.String("status") == "online" {
			online = true
		}
	}
	if !online {
		return Input(errors.New("target node is not online/visible"))
	}
	ss, err := e.Client.Rows(ctx, "/nodes/"+url.PathEscape(node)+"/storage")
	if err != nil {
		return err
	}
	found := false
	for _, s := range ss {
		if s.String("storage") == storage && s.Bool("active") && s.Bool("enabled") && containsCSV(s.String("content"), "images") {
			found = true
		}
	}
	if !found {
		return Input(errors.New("target storage is unavailable or does not support VM images"))
	}
	ns, err := e.Client.Rows(ctx, "/nodes/"+url.PathEscape(node)+"/network")
	if err != nil {
		return err
	}
	found = false
	for _, n := range ns {
		if n.String("iface") == bridge && (n.String("type") == "bridge" || n.String("type") == "OVSBridge") {
			if vlan > 0 && n.String("type") == "bridge" && !n.Bool("bridge_vlan_aware") {
				return Input(errors.New("VLAN requires a VLAN-aware bridge"))
			}
			found = true
		}
	}
	if !found {
		return Input(errors.New("bridge is not visible on target node"))
	}
	var p any
	if err = e.Client.Do(ctx, "GET", "/pools/"+url.PathEscape(pool), nil, &p); err != nil {
		return err
	}
	return e.Require(ctx, "/pool/"+pool, "VM.Allocate")
}
func containsCSV(s, w string) bool {
	for _, v := range strings.Split(s, ",") {
		if v == w {
			return true
		}
	}
	return false
}
func (e *Engine) Require(ctx context.Context, path string, privileges ...string) error {
	var p map[string]pve.Config
	if err := e.Client.Do(ctx, "GET", "/access/permissions", url.Values{"path": {path}}, &p); err != nil {
		return err
	}
	for _, name := range privileges {
		if !p[path].Bool(name) {
			return Input(fmt.Errorf("missing %s on %s", name, path))
		}
	}
	return nil
}
func (e *Engine) RequireGuest(ctx context.Context, path string) error {
	var perms map[string]pve.Config
	if err := e.Client.Do(ctx, "GET", "/access/permissions", url.Values{"path": {path}}, &perms); err != nil {
		return err
	}
	p := perms[path]
	if p.Bool("VM.Monitor") || p.Bool("VM.GuestAgent.Unrestricted") || (p.Bool("VM.GuestAgent.Audit") && p.Bool("VM.GuestAgent.FileRead")) {
		return nil
	}
	return Input(errors.New("readiness requires guest-agent audit and public host-key file-read permissions on " + path))
}
func (e *Engine) Deploy(ctx context.Context, s spec.Deploy, dry bool) (*operation.Result, error) {
	if err := s.Validate(); err != nil {
		return nil, Input(err)
	}
	if s.Context != e.Context {
		return nil, Input(errors.New("spec context does not match selected context"))
	}
	publicKey, keyErr := guest.PublicKey(s.Guest)
	if keyErr != nil {
		return nil, Input(keyErr)
	}
	if err := e.Preflight(ctx, s.Node, s.Storage, s.Pool, s.Network.Bridge, s.Network.VLAN, s.VMID); err != nil {
		return nil, err
	}
	_, cfg, err := e.source(ctx, s)
	if err != nil {
		return nil, err
	}
	if _, err = vm.HardwareChanges(cfg, s.Hardware); err != nil {
		return nil, Input(err)
	}
	if s.Hardware.DiskGiB > 0 {
		old, er := vm.DiskSize(cfg, s.Hardware.Disk)
		if er != nil {
			return nil, Input(er)
		}
		if int64(s.Hardware.DiskGiB)<<30 < old {
			return nil, Input(errors.New("requested disk would shrink template disk"))
		}
	}
	if err = e.Require(ctx, fmt.Sprintf("/vms/%d", s.TemplateVMID), "VM.Clone"); err != nil {
		return nil, err
	}
	if err = e.Require(ctx, "/storage/"+s.Storage, "Datastore.AllocateSpace"); err != nil {
		return nil, err
	}
	poolPath := "/pool/" + s.Pool
	rights := []string{"VM.Audit", "VM.Config.Network", "VM.Config.Cloudinit", "VM.PowerMgmt"}
	if s.Hardware.CPU != "" || s.Hardware.Cores > 0 || s.Hardware.Sockets > 0 {
		rights = append(rights, "VM.Config.CPU")
	}
	if s.Hardware.MemoryMiB > 0 {
		rights = append(rights, "VM.Config.Memory")
	}
	if s.Hardware.DiskGiB > 0 {
		rights = append(rights, "VM.Config.Disk")
	}
	if err = e.Require(ctx, poolPath, rights...); err != nil {
		return nil, err
	}
	if err = e.RequireGuest(ctx, poolPath); err != nil {
		return nil, err
	}
	r := e.newResult("deploy", s.VMID, s.Node)
	r.Deploy = &s
	r.PublicKey = publicKey
	r.InputDigest = deploymentDigest(s, publicKey)
	r.Changes = map[string]string{"clone": strconv.Itoa(s.TemplateVMID), "name": s.Name, "readiness": "authenticated SSH"}
	if dry {
		return r, nil
	}
	unlock, err := e.Store.Lock(e.Context, s.VMID)
	if err != nil {
		return nil, Input(err)
	}
	defer unlock()
	r.OperationID = operation.NewID()
	return e.runDeploy(ctx, r)
}
func (e *Engine) source(ctx context.Context, s spec.Deploy) (pve.Resource, pve.Config, error) {
	r, c, err := e.Existing(ctx, s.TemplateVMID)
	if err != nil {
		return r, c, err
	}
	if !c.Bool("template") || r.Node != s.Node {
		return r, c, Input(errors.New("source must be a template on the destination node"))
	}
	if !c.Bool("agent") && !strings.HasPrefix(c.String("agent"), "1,") && !strings.HasPrefix(c.String("agent"), "enabled=1") {
		return r, c, Input(errors.New("template must enable QEMU guest agent"))
	}
	cloud := false
	for k, v := range c {
		if (strings.HasPrefix(k, "ide") || strings.HasPrefix(k, "sata") || strings.HasPrefix(k, "scsi")) && strings.Contains(fmt.Sprint(v), "cloudinit") {
			cloud = true
		}
	}
	if !cloud {
		return r, c, Input(errors.New("template needs a Cloud-Init drive"))
	}
	windows := strings.HasPrefix(s.Guest.OS, "windows")
	if strings.HasPrefix(c.String("ostype"), "win") != windows {
		return r, c, Input(errors.New("template OS type does not match guest OS"))
	}
	if strings.HasPrefix(c.String("description"), "pvecli recipe=") && (!strings.Contains(c.String("description"), "os="+s.Guest.OS+" ") || s.Guest.User != "pveadmin") {
		return r, c, Input(errors.New("managed template requires its recorded OS and pveadmin user"))
	}
	if _, _, err = vm.Model(c.String(s.Network.Interface)); err != nil {
		return r, c, Input(err)
	}
	return r, c, nil
}
func marker(r *operation.Result) string {
	return "pvecli operation=" + r.OperationID + " input=" + r.InputDigest
}
func deploymentDigest(s spec.Deploy, key string) string {
	return spec.Digest(struct {
		Spec      spec.Deploy
		PublicKey string
	}{s, key})
}
func (e *Engine) runDeploy(ctx context.Context, r *operation.Result) (*operation.Result, error) {
	s := *r.Deploy
	if !operation.Has(r, "clone") {
		_, found, err := e.Client.Find(ctx, s.VMID)
		if err != nil {
			return e.Fail(r, err, true)
		}
		if found {
			cfg, er := e.Client.VMConfig(ctx, s.Node, s.VMID)
			if er != nil {
				return e.Fail(r, er, true)
			}
			if cfg.String("description") != marker(r) {
				return e.Fail(r, errors.New("destination ownership cannot be proven; inspect VM before proceeding"), true)
			}
			// A response lost before the UPID was saved cannot be safely classified.
			if len(r.Tasks) == 0 {
				return e.Fail(r, errors.New("clone submission outcome uncertain: owned VM exists but no saved UPID; inspect its tasks manually"), true)
			}
			if er = e.Client.Wait(ctx, s.Node, r.Tasks[0]); er != nil {
				return e.Fail(r, er, taskUncertain(er))
			}
			if cfg.String("lock") != "" {
				return e.Fail(r, errors.New("destination remains locked"), true)
			}
			operation.Complete(r, "clone")
			if er = e.Save(r); er != nil {
				return r, er
			}
		} else {
			if r.Phase == "clone" {
				return e.Fail(r, errors.New("clone was submitted but destination is not visible; refusing to resubmit"), true)
			}
			v := url.Values{"newid": {strconv.Itoa(s.VMID)}, "name": {s.Name}, "full": {"1"}, "storage": {s.Storage}, "pool": {s.Pool}, "description": {marker(r)}}
			if err = e.Task(ctx, r, "clone", "POST", pve.Path(s.Node, s.TemplateVMID)+"/clone", v); err != nil {
				return e.Fail(r, err, IsUnknown(err))
			}
		}
	}
	if !operation.Has(r, "configure") {
		cfg, err := e.Client.VMConfig(ctx, s.Node, s.VMID)
		if err != nil {
			return e.Fail(r, err, true)
		}
		if cfg.String("description") != marker(r) {
			return e.Fail(r, errors.New("destination ownership changed"), true)
		}
		v, err := vm.HardwareChanges(cfg, s.Hardware)
		if err != nil {
			return e.Fail(r, err, false)
		}
		key := r.PublicKey
		netConfig, err := vm.MergeProperty(cfg.String(s.Network.Interface), map[string]string{"bridge": s.Network.Bridge, "tag": vlanString(s.Network.VLAN), "link_down": "0"}, "")
		if err != nil {
			return e.Fail(r, err, false)
		}
		v.Set(s.Network.Interface, netConfig)
		v.Set("ciuser", s.Guest.User)
		if strings.HasPrefix(s.Guest.OS, "windows") {
			v.Set("citype", "configdrive2")
		} else {
			v.Set("citype", "nocloud")
		}
		v.Set("ciupgrade", "0")
		v.Set("sshkeys", strings.ReplaceAll(url.QueryEscape(key), "+", "%20"))
		v.Set("ipconfig"+strings.TrimPrefix(s.Network.Interface, "net"), "ip="+s.Network.IPv4+gateway(s.Network.Gateway))
		if s.Network.DNS != "" {
			v.Set("nameserver", s.Network.DNS)
		}
		if cfg.String("digest") == "" {
			return e.Fail(r, errors.New("destination configuration has no digest"), true)
		}
		v.Set("digest", cfg.String("digest"))
		if err = e.Task(ctx, r, "configure", "POST", pve.Path(s.Node, s.VMID)+"/config", v); err != nil {
			return e.Fail(r, err, IsUnknown(err))
		}
	}
	if s.Hardware.DiskGiB > 0 && !operation.Has(r, "resize") {
		cfg, err := e.Client.VMConfig(ctx, s.Node, s.VMID)
		if err != nil {
			return e.Fail(r, err, true)
		}
		old, err := vm.DiskSize(cfg, s.Hardware.Disk)
		if err != nil {
			return e.Fail(r, err, false)
		}
		target := int64(s.Hardware.DiskGiB) << 30
		if old > target {
			return e.Fail(r, errors.New("destination disk exceeds requested size"), false)
		}
		if old < target {
			v := url.Values{"disk": {s.Hardware.Disk}, "size": {strconv.Itoa(s.Hardware.DiskGiB) + "G"}, "digest": {cfg.String("digest")}}
			if err = e.Task(ctx, r, "resize", "PUT", pve.Path(s.Node, s.VMID)+"/resize", v); err != nil {
				return e.Fail(r, err, IsUnknown(err))
			}
		} else {
			operation.Complete(r, "resize")
			if err = e.Save(r); err != nil {
				return r, err
			}
		}
	}
	if !operation.Has(r, "start") {
		rs, ok, err := e.Client.Find(ctx, s.VMID)
		if err != nil || !ok {
			if err == nil {
				err = errors.New("destination disappeared")
			}
			return e.Fail(r, err, true)
		}
		if rs.Status != "running" {
			if err = e.Task(ctx, r, "start", "POST", pve.Path(s.Node, s.VMID)+"/status/start", nil); err != nil {
				return e.Fail(r, err, IsUnknown(err))
			}
		} else {
			operation.Complete(r, "start")
			if err = e.Save(r); err != nil {
				return r, err
			}
		}
	}
	r.Phase = "readiness"
	r.Status = "running"
	if err := e.Save(r); err != nil {
		return r, err
	}
	a, err := e.Guest.Ready(ctx, e.Client, s.Node, s.VMID, s.Network, s.Guest)
	if err != nil {
		return e.Fail(r, err, ctx.Err() != nil)
	}
	r.SSHEndpoint = a.Endpoint
	r.HostKey = a.Fingerprint
	r.Pending, err = e.Client.Pending(ctx, s.Node, s.VMID)
	if err != nil {
		return e.Fail(r, err, true)
	}
	r.Status = "succeeded"
	r.Phase = "complete"
	r.Error = ""
	operation.Complete(r, "readiness")
	return r, e.Save(r)
}
func vlanString(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}
func gateway(s string) string {
	if s == "" {
		return ""
	}
	return ",gw=" + s
}
func (e *Engine) Resume(ctx context.Context, id string) (*operation.Result, error) {
	r, err := e.Store.Load(id)
	if err != nil {
		return nil, Input(err)
	}
	if r.Kind != "deploy" || r.Deploy == nil {
		return nil, Input(errors.New("only native deployments can be resumed"))
	}
	if r.Context != e.Context || r.ContextIdentity != e.Identity || r.PublicKey == "" || r.InputDigest != deploymentDigest(*r.Deploy, r.PublicKey) {
		return nil, Input(errors.New("journal input/context identity mismatch"))
	}
	if err = r.Deploy.Validate(); err != nil {
		return nil, Input(err)
	}
	key, keyErr := guest.PublicKey(r.Deploy.Guest)
	if keyErr != nil {
		return nil, Input(keyErr)
	}
	if key != r.PublicKey {
		return nil, Input(errors.New("SSH identity changed since submission; restore the original key before resuming"))
	}
	if r.Status == "succeeded" {
		return r, nil
	}
	unlock, err := e.Store.Lock(e.Context, r.VMID)
	if err != nil {
		return nil, Input(err)
	}
	defer unlock()
	cfg, err := e.Client.VMConfig(ctx, r.Node, r.VMID)
	if err != nil {
		return e.Fail(r, err, true)
	}
	if cfg.String("description") != marker(r) {
		return e.Fail(r, errors.New("remote operation ownership mismatch"), true)
	}
	// Finish any recorded task before making a subsequent write.
	if r.SubmissionPending {
		return e.Fail(r, errors.New("submission outcome is unknown; inspect remote tasks before any further mutation"), true)
	}
	if r.ActiveTask != "" && !operation.Has(r, r.ActiveTaskPhase) {
		if r.ActiveTaskPhase != r.Phase {
			return e.Fail(r, errors.New("journal active task phase mismatch"), true)
		}
		if err = e.Client.Wait(ctx, r.Node, r.ActiveTask); err != nil {
			return e.Fail(r, err, taskUncertain(err))
		}
		operation.Complete(r, r.ActiveTaskPhase)
	}
	return e.runDeploy(ctx, r)
}
func (e *Engine) Status(ctx context.Context, id string) (*operation.Result, error) {
	r, err := e.Store.Load(id)
	if err != nil {
		return nil, Input(err)
	}
	if r.Context != e.Context || r.ContextIdentity != e.Identity {
		return nil, Input(errors.New("operation belongs to a different context identity"))
	}
	tasks := map[string]any{}
	for _, t := range r.Tasks {
		v, er := e.Client.TaskStatus(ctx, r.Node, t)
		if er != nil {
			tasks[t] = er.Error()
		} else {
			tasks[t] = v
		}
	}
	r.Data = tasks
	if r.Kind == "template-build" {
		dir := filepath.Join(e.Store.Dir, "builds", r.OperationID)
		if _, er := os.Stat(dir); er == nil {
			found := false
			for _, p := range r.Residual {
				if p == dir {
					found = true
				}
			}
			if !found {
				r.Residual = append(r.Residual, dir)
			}
		}
	}
	return r, nil
}
