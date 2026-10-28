package deploy

import (
	"context"
	"fmt"
)

// ResumeAgentUpgradeSubmission closes the submission/SCM-registration window.
// It reuses protected, already verified bytes and the original operation ID.
// Reading status alone never invokes this recovery action.
func (r *Remote) ResumeAgentUpgradeSubmission(ctx context.Context, alias, id string) error {
	result, err := r.AgentUpgradeStatus(ctx, alias, id)
	if err != nil {
		return err
	}
	if result.Status != "running" || result.Phase != "submitted" {
		return nil
	}
	source := `$base=Join-Path $env:ProgramData 'dzdeploy';$upgrades=Join-Path $base 'upgrades';$target=Join-Path $upgrades $spec.target;$operation=Join-Path $target $spec.id;$request=Join-Path $operation 'request.json';$exe=Join-Path $operation 'coordinator.exe';foreach($path in @($base,$upgrades,$target,$operation,$request,$exe)){$item=Get-Item -Force -LiteralPath $path;$cursor=$item;while($null -ne $cursor){if(($cursor.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw 'Upgrade recovery path contains a reparse point'};$cursor=if($cursor.PSIsContainer){$cursor.Parent}else{$cursor.Directory}};$acl=Get-Acl -LiteralPath $path;$owner=$acl.GetOwner([Security.Principal.SecurityIdentifier]).Value;if($owner -ne 'S-1-5-18' -and $owner -ne 'S-1-5-32-544'){throw 'Upgrade recovery path has an untrusted owner'};$unsafe=[Security.AccessControl.FileSystemRights]::Write -bor [Security.AccessControl.FileSystemRights]::Delete -bor [Security.AccessControl.FileSystemRights]::DeleteSubdirectoriesAndFiles -bor [Security.AccessControl.FileSystemRights]::ChangePermissions -bor [Security.AccessControl.FileSystemRights]::TakeOwnership;foreach($ace in $acl.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])){if($ace.AccessControlType -eq 'Allow' -and ($ace.FileSystemRights -band $unsafe) -ne 0 -and $ace.IdentityReference.Value -ne 'S-1-5-18' -and $ace.IdentityReference.Value -ne 'S-1-5-32-544'){throw 'Upgrade recovery path permits service writes'}}};$file=Get-Item -Force -LiteralPath $request;if($file.PSIsContainer -or $file.Length -gt 67108864){throw 'Invalid protected upgrade request'};$req=[IO.File]::ReadAllText($request)|ConvertFrom-Json;if($req.version -ne 1 -or $req.id -ne $spec.id -or $req.target -ne $spec.target){throw 'Protected upgrade request identity differs'};if((Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant() -ne $req.candidateSHA256){throw 'Protected coordinator digest differs'};$service=Get-CimInstance Win32_Service -Filter ("Name='dzdeploy-upgrade-"+$spec.target+'-'+$spec.id+"'");if($null -ne $service -and $service.State -ne 'Stopped'){Write-Output '{"status":"running"}';exit 0};& $exe agent-upgrade-submit --request $request;if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}`
	out := &boundedBuffer{limit: 64 << 10}
	stderr := &boundedBuffer{limit: 16 << 10}
	if err = r.exec(ctx, upgradeSpecScript(map[string]string{"target": alias, "id": id}, source), nil, out, stderr); err != nil {
		return fmt.Errorf("cannot resume prepared upgrade %s; inspect the same operation before retrying: %w", id, err)
	}
	return nil
}
