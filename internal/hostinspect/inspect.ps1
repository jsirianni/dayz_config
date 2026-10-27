$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$service = Get-CimInstance Win32_Service | Where-Object Name -eq $spec.Service | Select-Object -First 1
$processes = @(Get-CimInstance Win32_Process -Filter "Name='DayZServer_x64.exe'" | Where-Object { $_.ExecutablePath -ieq $spec.Executable } | ForEach-Object {
    $ft = $_.CreationDate.ToUniversalTime().ToFileTimeUtc()
    [pscustomobject]@{pid=[int]$_.ProcessId;parent=[int]$_.ParentProcessId;executable=[string]$_.ExecutablePath;token=('{0}:{1}' -f ([uint32]($ft -shr 32)),([uint32]($ft -band [long]4294967295)));started=$_.CreationDate.ToUniversalTime().ToString('o')}
})
$ids = @($processes | ForEach-Object {$_.pid})
$ports = @(Get-NetUDPEndpoint | Where-Object {$_.OwningProcess -in $ids} | Select-Object -ExpandProperty LocalPort -Unique | ForEach-Object {[int]$_})
$query = [ordered]@{name='';map='';error=''}
$udp = New-Object Net.Sockets.UdpClient
try {
    $udp.Client.ReceiveTimeout = 3000
    $udp.Connect('127.0.0.1',[int]$spec.QueryPort)
    [byte[]]$request = @(255,255,255,255,84) + [Text.Encoding]::ASCII.GetBytes("Source Engine Query`0")
    [void]$udp.Send($request,$request.Length)
    $peer = New-Object Net.IPEndPoint([Net.IPAddress]::Any,0)
    [byte[]]$reply = $udp.Receive([ref]$peer)
    if ($reply.Length -ge 9 -and $reply[4] -eq 65) {
        [byte[]]$challengeRequest = $request + $reply[5..8]
        [void]$udp.Send($challengeRequest,$challengeRequest.Length)
        $reply = $udp.Receive([ref]$peer)
    }
    if ($reply.Length -lt 7 -or $reply[4] -ne 73) {throw 'Unexpected A2S_INFO response'}
    $parts = [Text.Encoding]::UTF8.GetString($reply,6,$reply.Length-6).Split([char]0)
    if ($parts.Length -lt 2) {throw 'Incomplete A2S_INFO response'}
    $query.name = [string]$parts[0]
    $query.map = [string]$parts[1]
} catch {$query.error = [string]$_.Exception.Message} finally {$udp.Dispose()}
[pscustomobject]@{at=[DateTime]::UtcNow.ToString('o');service=[pscustomobject]@{state=[string]$service.State;startMode=[string]$service.StartMode;pid=[int]$service.ProcessId};processes=$processes;ports=$ports;query=$query} | ConvertTo-Json -Depth 5 -Compress

