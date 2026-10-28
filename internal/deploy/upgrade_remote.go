package deploy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf16"
)

type UpgradePreview struct {
	Target                string          `json:"target"`
	Preview               bool            `json:"preview"`
	Previous              *AgentIdentity  `json:"previous,omitempty"`
	Candidate             AgentIdentity   `json:"candidate"`
	Game                  ProcessIdentity `json:"game"`
	Handoff               string          `json:"handoff"`
	Blocked               []string        `json:"blocked,omitempty"`
	PreserveGame          bool            `json:"preserveGame"`
	PreserveConfiguration bool            `json:"preserveConfiguration"`
	UpdateSteam           bool            `json:"updateSteam"`
	Unchanged             bool            `json:"unchanged"`
	ServiceState          string          `json:"serviceState"`
}

func upgradePowerShell(source string) string {
	// Win32 OpenSSH commonly launches through cmd.exe, whose command line is
	// limited to 8191 characters. Compress the fixed collector/bridge source.
	var compressed bytes.Buffer
	z, _ := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	_, _ = z.Write([]byte(source))
	_ = z.Close()
	source = fmt.Sprintf("$b=[Convert]::FromBase64String('%s');$m=New-Object IO.MemoryStream(,$b);$z=New-Object IO.Compression.GzipStream($m,[IO.Compression.CompressionMode]::Decompress);$r=New-Object IO.StreamReader($z);try{ & ([scriptblock]::Create($r.ReadToEnd())) }finally{$r.Dispose();$z.Dispose();$m.Dispose()}", base64.StdEncoding.EncodeToString(compressed.Bytes()))
	words := utf16.Encode([]rune(source))
	raw := make([]byte, len(words)*2)
	for i, word := range words {
		raw[2*i], raw[2*i+1] = byte(word), byte(word>>8)
	}
	return "powershell.exe -NoProfile -NonInteractive -EncodedCommand " + base64.StdEncoding.EncodeToString(raw)
}

func upgradeSpecScript(spec any, source string) string {
	raw, _ := json.Marshal(spec)
	// A literal PowerShell string keeps data inert while allowing gzip to share
	// repeated path prefixes with the fixed script (important under cmd's limit).
	literal := strings.ReplaceAll(string(raw), "'", "''")
	source = strings.ReplaceAll(source, "Get-Item -LiteralPath", "Get-Item -Force -LiteralPath")
	return upgradePowerShell("$ErrorActionPreference='Stop';$spec='" + literal + "'|ConvertFrom-Json;" + source)
}

// coordinatorJSON reads only fixed, administrator-owned coordinator evidence.
// The service-writable state directory is never accepted as adoption authority.
func (r *Remote) coordinatorJSON(ctx context.Context, alias, id, kind string, destination any) error {
	if _, err := TargetFor(alias); err != nil {
		return err
	}
	if kind != "adoption" && (kind != "result" || !ValidID(id)) {
		return errors.New("invalid coordinator evidence selector")
	}
	spec := map[string]string{"target": alias, "id": id, "kind": kind}
	source := `function Assert-Protected($path){$acl=Get-Acl -LiteralPath $path;$owner=$acl.GetOwner([Security.Principal.SecurityIdentifier]).Value;if($owner -ne 'S-1-5-18' -and $owner -ne 'S-1-5-32-544'){throw 'Coordinator evidence owner is not Administrators or SYSTEM'};$unsafe=[Security.AccessControl.FileSystemRights]::Write -bor [Security.AccessControl.FileSystemRights]::Delete -bor [Security.AccessControl.FileSystemRights]::DeleteSubdirectoriesAndFiles -bor [Security.AccessControl.FileSystemRights]::ChangePermissions -bor [Security.AccessControl.FileSystemRights]::TakeOwnership;foreach($ace in $acl.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])){if($ace.AccessControlType -eq [Security.AccessControl.AccessControlType]::Allow -and ($ace.FileSystemRights -band $unsafe) -ne 0 -and $ace.IdentityReference.Value -ne 'S-1-5-18' -and $ace.IdentityReference.Value -ne 'S-1-5-32-544'){throw 'Coordinator evidence permits writes by a non-administrator identity'}}};$base=Join-Path $env:ProgramData 'dzdeploy';$upgrades=Join-Path $base 'upgrades';$root=Join-Path $upgrades $spec.target;$file=if($spec.kind -eq 'adoption'){Join-Path $root 'adoption.json'}else{Join-Path (Join-Path $root $spec.id) 'result.json'};if(-not(Test-Path -LiteralPath $file)){Write-Output '{"missing":true}';exit 0};$item=Get-Item -LiteralPath $file;if($item.PSIsContainer -or $item.Length -gt 67108864 -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw 'Invalid coordinator evidence file'};$cursor=$item.Directory;while($null -ne $cursor){if(($cursor.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw 'Coordinator evidence path contains a reparse point'};$cursor=$cursor.Parent};foreach($path in @($base,$upgrades,$root,$item.DirectoryName,$file)){Assert-Protected $path};[IO.File]::ReadAllText($file)`
	out := &boundedBuffer{limit: MaxFileBytes}
	stderr := &boundedBuffer{limit: 16 << 10}
	if err := r.exec(ctx, upgradeSpecScript(spec, source), nil, out, stderr); err != nil {
		return fmt.Errorf("read protected agent upgrade evidence (administrative SSH execution required): %w", err)
	}
	var marker struct {
		Missing bool `json:"missing"`
	}
	if json.Unmarshal(out.Bytes(), &marker) == nil && marker.Missing {
		return os.ErrNotExist
	}
	if err := json.Unmarshal(out.Bytes(), destination); err != nil {
		return fmt.Errorf("invalid agent upgrade evidence: %w", err)
	}
	return nil
}

func (r *Remote) AgentUpgradeStatus(ctx context.Context, alias, id string) (UpgradeResult, error) {
	var result UpgradeResult
	if err := r.coordinatorJSON(ctx, alias, id, "result", &result); err != nil {
		return result, err
	}
	return result, result.Validate(alias, id)
}

func (r *Remote) AgentAdoption(ctx context.Context, alias string) (AdoptionProof, error) {
	var proof AdoptionProof
	if err := r.coordinatorJSON(ctx, alias, "", "adoption", &proof); err != nil {
		return proof, err
	}
	if proof.Version != Protocol || proof.Target != alias || !ValidID(proof.ID) || proof.Updated.IsZero() || proof.Game.PID <= 0 || proof.Game.Token == "" || proof.Game.Executable == "" || proof.Agent.PID <= 0 || proof.Agent.Token == "" || !digestPattern.MatchString(proof.Agent.SHA256) {
		return proof, errors.New("invalid protected agent adoption proof")
	}
	return proof, nil
}

func (r *Remote) UpgradeHost(alias string) (HostConfig, error) {
	var host HostConfig
	if err := r.ReadJSON("host.json", &host); err != nil {
		return host, err
	}
	if err := host.Validate(); err != nil {
		return host, err
	}
	if host.Target != alias {
		return host, errors.New("agent upgrade host identity mismatch")
	}
	if err := validateEnrollment(Endpoint{RemoteDir: r.Endpoint.RemoteDir, Host: &host}, alias); err != nil {
		return host, err
	}
	return host, nil
}

func (r *Remote) PreviewAgentUpgrade(ctx context.Context, alias string, candidate AgentIdentity) (UpgradePreview, error) {
	preview := UpgradePreview{Target: alias, Preview: true, Candidate: candidate, PreserveGame: true, PreserveConfiguration: true, Handoff: "legacy debugger handoff"}
	host, err := r.UpgradeHost(alias)
	if err != nil {
		return preview, err
	}
	state, err := r.State(alias)
	if err != nil {
		return preview, err
	}
	target, _ := TargetFor(alias)
	if state.Installed != nil {
		target = state.Installed.Launcher
	}
	observed, err := r.observe(ctx, host, target)
	if err != nil {
		return preview, err
	}
	if len(observed.Processes) > 1 {
		preview.Blocked = append(preview.Blocked, "multiple game processes in the managed installation")
	}
	if len(observed.Processes) == 1 {
		game := observed.Processes[0]
		preview.Game = ProcessIdentity{PID: game.PID, Token: game.Token, Executable: game.Executable}
		if game.PID != state.PID || game.Token != state.ProcessToken {
			preview.Blocked = append(preview.Blocked, "observed game differs from saved process identity")
		}
	} else if state.PID != 0 {
		preview.Blocked = append(preview.Blocked, "saved game process is no longer present")
	}
	var heartbeat Heartbeat
	if err = r.ReadJSON("heartbeat.json", &heartbeat); err != nil && observed.Service.State == "Running" {
		return preview, err
	}
	for _, feature := range heartbeat.Capabilities {
		if feature == AgentHandoffCapability {
			preview.Handoff = "cooperative agent handoff"
		}
	}
	var inspected struct {
		Administrator bool          `json:"administrator"`
		Configured    bool          `json:"configured"`
		Busy          bool          `json:"busy"`
		Previous      AgentIdentity `json:"previous"`
	}
	source := `$name='dzdeploy-'+$spec.target;$service=Get-CimInstance Win32_Service -Filter ("Name='"+$name+"'");if($null -eq $service){throw 'Managed agent service is missing'};$binary=Join-Path $spec.state 'bin\dzdeploy.exe';$hostFile=Join-Path $spec.state 'host.json';$expected='"'+$binary+'" agent --host-config "'+$hostFile+'"';$token='';$bound=$true;if($service.ProcessId -gt 0){$process=Get-Process -Id $service.ProcessId;$bound=$process.Path -ieq $binary;$ft=$process.StartTime.ToUniversalTime().ToFileTimeUtc();$token=('{0}:{1}' -f ([uint32]($ft -shr 32)),([uint32]($ft -band [long]4294967295)))};$busy=(Test-Path -LiteralPath (Join-Path $spec.state 'journal.json')) -or (Test-Path -LiteralPath (Join-Path $spec.state 'restore.json')) -or (@(Get-ChildItem -LiteralPath (Join-Path $spec.state 'inbox') -Force).Count -ne 0);foreach($file in Get-ChildItem -LiteralPath (Join-Path $spec.state 'results') -Filter '*.json'){if(([IO.File]::ReadAllText($file.FullName)|ConvertFrom-Json).status -eq 'running'){$busy=$true}};$principal=New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent());[pscustomobject]@{administrator=$principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator);configured=($service.PathName -ieq $expected -and $bound);busy=$busy;previous=@{protocol=1;sha256=(Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash.ToLowerInvariant();goos='windows';goarch='amd64';pid=[int]$service.ProcessId;token=$token}}|ConvertTo-Json -Depth 5 -Compress`
	out := &boundedBuffer{limit: 64 << 10}
	stderr := &boundedBuffer{limit: 16 << 10}
	if err = r.exec(ctx, upgradeSpecScript(map[string]string{"target": alias, "state": host.StateDir}, source), nil, out, stderr); err != nil {
		return preview, fmt.Errorf("read agent upgrade preflight: %w", err)
	}
	if err = json.Unmarshal(out.Bytes(), &inspected); err != nil {
		return preview, err
	}
	preview.Previous = &inspected.Previous
	if heartbeat.Agent != nil {
		preview.Previous.Revision = heartbeat.Agent.Revision
		preview.Previous.Dirty = heartbeat.Agent.Dirty
	}
	if !inspected.Administrator {
		preview.Blocked = append(preview.Blocked, "administrative SSH execution is required for the upgrade coordinator")
	}
	if !inspected.Configured {
		preview.Blocked = append(preview.Blocked, "managed service or process executable differs from the expected binary")
	}
	if inspected.Busy {
		preview.Blocked = append(preview.Blocked, "pending operation or recovery journal blocks upgrade")
	}
	preview.ServiceState = observed.Service.State
	fresh := heartbeat.Version == Protocol && time.Since(heartbeat.Updated) <= 15*time.Second && time.Until(heartbeat.Updated) <= 15*time.Second
	if observed.Service.State == "Running" {
		if !fresh {
			preview.Blocked = append(preview.Blocked, "running agent heartbeat is stale or incompatible")
		}
		if observed.Service.PID != inspected.Previous.PID {
			preview.Blocked = append(preview.Blocked, "service process changed during inspection")
		}
	} else if observed.Service.State != "Stopped" {
		preview.Blocked = append(preview.Blocked, "agent service is transitioning; wait until running or stopped")
	}
	if heartbeat.Agent != nil && fresh && observed.Service.State == "Running" && len(preview.Blocked) == 0 {
		a := heartbeat.Agent
		preview.Unchanged = inspected.Previous.SHA256 == candidate.SHA256 && a.SHA256 == candidate.SHA256 && a.PID == inspected.Previous.PID && a.Token == inspected.Previous.Token && a.Revision == candidate.Revision && a.Protocol == Protocol && !a.Dirty
	}
	return preview, nil
}

func (r *Remote) SubmitAgentUpgrade(ctx context.Context, req UpgradeRequest, binary []byte) error {
	if err := req.Validate(); err != nil {
		return err
	}
	if Digest(binary) != req.CandidateSHA256 {
		return errors.New("agent upgrade candidate checksum mismatch")
	}
	host, err := r.UpgradeHost(req.Target)
	if err != nil {
		return err
	}
	if host != req.Host {
		return errors.New("host configuration changed during upgrade preparation")
	}
	dir := "setup/" + req.ID
	if err = r.put(dir+"/dzdeploy.exe", binary, true); err != nil {
		return err
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err = r.put(dir+"/request.json", raw, true); err != nil {
		return err
	}
	root, _ := WindowsPath(host.StateDir)
	exe := root + `\` + strings.ReplaceAll(dir, "/", `\`) + `\dzdeploy.exe`
	request := root + `\` + strings.ReplaceAll(dir, "/", `\`) + `\request.json`
	// Never execute the service-writable upload as administrator. This fixed
	// bridge captures the bytes, verifies client-bound digests, then executes only
	// an administrator-owned copy. The service cannot replace it between checks.
	spec := map[string]string{"target": req.Target, "id": req.ID, "binary": exe, "request": request, "binarySHA256": req.CandidateSHA256, "requestSHA256": Digest(raw)}
	source := `function Assert-Private($path){$item=Get-Item -LiteralPath $path;if(($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw 'Protected upgrade path contains a reparse point'};$acl=Get-Acl -LiteralPath $path;$owner=$acl.GetOwner([Security.Principal.SecurityIdentifier]).Value;if($owner -ne 'S-1-5-18' -and $owner -ne 'S-1-5-32-544'){throw 'Protected upgrade path has an untrusted owner'};$unsafe=[Security.AccessControl.FileSystemRights]::Write -bor [Security.AccessControl.FileSystemRights]::Delete -bor [Security.AccessControl.FileSystemRights]::DeleteSubdirectoriesAndFiles -bor [Security.AccessControl.FileSystemRights]::ChangePermissions -bor [Security.AccessControl.FileSystemRights]::TakeOwnership;foreach($ace in $acl.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])){if($ace.AccessControlType -eq [Security.AccessControl.AccessControlType]::Allow -and ($ace.FileSystemRights -band $unsafe) -ne 0 -and $ace.IdentityReference.Value -ne 'S-1-5-18' -and $ace.IdentityReference.Value -ne 'S-1-5-32-544'){throw 'Protected upgrade path permits service writes'}}};function Ensure-Private($path){if(-not(Test-Path -LiteralPath $path)){[IO.Directory]::CreateDirectory($path)|Out-Null;$acl=New-Object Security.AccessControl.DirectorySecurity;$acl.SetOwner((New-Object Security.Principal.SecurityIdentifier('S-1-5-32-544')));$acl.SetAccessRuleProtection($true,$false);foreach($sid in @('S-1-5-18','S-1-5-32-544')){$identity=New-Object Security.Principal.SecurityIdentifier($sid);$rule=New-Object Security.AccessControl.FileSystemAccessRule($identity,'FullControl','ContainerInherit,ObjectInherit','None','Allow');$acl.AddAccessRule($rule)};$authenticated=New-Object Security.Principal.SecurityIdentifier('S-1-5-11');$traverse=New-Object Security.AccessControl.FileSystemAccessRule($authenticated,'ReadAndExecute','None','None','Allow');$acl.AddAccessRule($traverse);Set-Acl -LiteralPath $path -AclObject $acl};Assert-Private $path};function Read-Captured($path,$expected){$stream=[IO.File]::Open($path,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read);try{if($stream.Length -gt 67108864){throw 'Upgrade upload exceeds size limit'};$bytes=New-Object byte[] ([int]$stream.Length);$offset=0;while($offset -lt $bytes.Length){$read=$stream.Read($bytes,$offset,$bytes.Length-$offset);if($read -eq 0){throw 'Upgrade upload ended early'};$offset+=$read};$sha=[Security.Cryptography.SHA256]::Create();try{$actual=([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','').ToLowerInvariant()}finally{$sha.Dispose()};if($actual -ne $expected){throw 'Upgrade upload does not match client checksum'};return ,$bytes}finally{$stream.Dispose()}};$binary=Read-Captured $spec.binary $spec.binarySHA256;$request=Read-Captured $spec.request $spec.requestSHA256;$base=Join-Path $env:ProgramData 'dzdeploy';Ensure-Private $base;$upgrades=Join-Path $base 'upgrades';Ensure-Private $upgrades;$target=Join-Path $upgrades $spec.target;Ensure-Private $target;$operation=Join-Path $target $spec.id;Ensure-Private $operation;$submission=Join-Path $operation 'submission';Ensure-Private $submission;$exe=Join-Path $submission 'dzdeploy.exe';$requestFile=Join-Path $submission 'request.json';[IO.File]::WriteAllBytes($exe,$binary);[IO.File]::WriteAllBytes($requestFile,$request);foreach($path in @($exe,$requestFile)){$acl=Get-Acl -LiteralPath $path;$acl.SetOwner((New-Object Security.Principal.SecurityIdentifier('S-1-5-32-544')));Set-Acl -LiteralPath $path -AclObject $acl;Assert-Private $path};& $exe agent-upgrade-submit --request $requestFile;if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}`
	out := &boundedBuffer{limit: 64 << 10}
	stderr := &boundedBuffer{limit: 16 << 10}
	// Captured request bytes are bound to the client hash above. Reject every
	// overlap before changing an ACL: hardening an ancestor of a runtime tree
	// would alter the installation, and a writable ancestor could forge receipts.
	overlapCheck := `function Assert-NoReparse($path){$cursor=[IO.Path]::GetFullPath($path);while($cursor){if(Test-Path -LiteralPath $cursor){$item=Get-Item -LiteralPath $cursor;if(($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw 'Upgrade path ancestry contains a reparse point'}};$next=Split-Path -Parent $cursor;if($next -eq $cursor){break};$cursor=$next}};function Test-Overlap($left,$right){$a=[IO.Path]::GetFullPath($left).TrimEnd([char]92);$b=[IO.Path]::GetFullPath($right).TrimEnd([char]92);$comparison=[StringComparison]::OrdinalIgnoreCase;return $a.Equals($b,$comparison) -or $a.StartsWith($b+'\',$comparison) -or $b.StartsWith($a+'\',$comparison)};$captured=[Text.Encoding]::UTF8.GetString($request)|ConvertFrom-Json;if($captured.target -ne $spec.target -or $captured.id -ne $spec.id -or $null -eq $captured.host){throw 'Captured upgrade request identity mismatch'};Assert-NoReparse $base;foreach($runtime in @($captured.host.stateDir,$captured.host.installDir,$captured.host.workshopDir,(Split-Path -Parent $captured.host.steamcmd))){if(Test-Overlap $base $runtime){throw 'Protected upgrade root overlaps a service-writable runtime directory'};Assert-NoReparse $runtime};`
	source = strings.Replace(source, ";Ensure-Private $base;", ";"+overlapCheck+"Ensure-Private $base;", 1)
	if err = r.exec(ctx, upgradeSpecScript(spec, source), nil, out, stderr); err != nil {
		return fmt.Errorf("agent upgrade submission may have started; inspect operation %s instead of resubmitting: %w", req.ID, err)
	}
	return nil
}

func (r *Remote) WaitAgentUpgrade(ctx context.Context, alias, id string, progress func(UpgradeResult)) (UpgradeResult, error) {
	if !ValidID(id) {
		return UpgradeResult{}, errors.New("invalid agent upgrade operation ID")
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var last UpgradeResult
	for {
		result, err := r.AgentUpgradeStatus(ctx, alias, id)
		if err == nil {
			if result.Phase != last.Phase && progress != nil {
				progress(result)
			}
			last = result
			switch result.Status {
			case "succeeded", "unchanged":
				return result, nil
			case "failed":
				return result, errors.New(result.Error)
			}
		} else if !os.IsNotExist(err) {
			return last, fmt.Errorf("operation %s may still be running; reconnect with agent upgrade %s --operation %s: %w", id, alias, id, err)
		}
		select {
		case <-ctx.Done():
			return last, fmt.Errorf("operation %s may still be running; reconnect with agent upgrade %s --operation %s: %w", id, alias, id, ctx.Err())
		case <-ticker.C:
		}
	}
}
