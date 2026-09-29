param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('Start', 'Stop', 'Backup')]
    [string]$Action,
    [string]$DataDirectory = (Join-Path $env:LOCALAPPDATA 'PersonalKnowledgeBase\data'),
    [int]$Port = 8081,
    [switch]$NoBrowser,
    [switch]$SmokeTest
)

$ErrorActionPreference = 'Stop'
$exe = Join-Path $PSScriptRoot 'Connla.exe'
$data = [IO.Path]::GetFullPath($DataDirectory)
$pidFile = Join-Path $data 'server.pid'
$url = "http://127.0.0.1:$Port/"

if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) {
    throw "Missing Connla.exe next to this launcher: $exe"
}

function Get-RecordedProcess {
    if (-not (Test-Path -LiteralPath $pidFile -PathType Leaf)) { return $null }
    $recordedId = 0
    if (-not [int]::TryParse((Get-Content -LiteralPath $pidFile -Raw).Trim(), [ref]$recordedId)) {
        throw "Invalid process ID file: $pidFile"
    }
    $processInfo = Get-CimInstance Win32_Process -Filter "ProcessId = $recordedId"
    if ($null -eq $processInfo) {
        Remove-Item -LiteralPath $pidFile
        return $null
    }
    if (-not [string]::Equals([IO.Path]::GetFullPath($processInfo.ExecutablePath), [IO.Path]::GetFullPath($exe), [StringComparison]::OrdinalIgnoreCase)) {
        throw "Process ID $recordedId belongs to another program; not touching it."
    }
    $expectedDataArgument = "--data `"$data`""
    if ($processInfo.CommandLine.IndexOf($expectedDataArgument, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
        throw "Process ID $recordedId uses another data directory; not touching it."
    }
    return $processInfo
}

function Test-LocalPort {
    $client = [Net.Sockets.TcpClient]::new()
    try {
        $result = $client.BeginConnect('127.0.0.1', $Port, $null, $null)
        if (-not $result.AsyncWaitHandle.WaitOne(500)) { return $false }
        try { $client.EndConnect($result); return $true } catch { return $false }
    } finally {
        $client.Close()
    }
}

function Wait-Ready([Diagnostics.Process]$Process) {
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        if ($Process.HasExited) { throw "Connla exited during startup. See $data\server.err.log" }
        try {
            $response = Invoke-WebRequest "${url}healthz" -UseBasicParsing -TimeoutSec 1
            if ($response.StatusCode -eq 200) { return }
        } catch { }
        Start-Sleep -Milliseconds 500
    }
    throw "Connla did not become ready. See $data\server.err.log"
}

switch ($Action) {
    'Start' {
        $running = Get-RecordedProcess
        if ($null -ne $running) {
            if (-not $NoBrowser) { Start-Process $url }
            Write-Output "Connla is already running at $url"
            break
        }
        if (Test-LocalPort) { throw "Port $Port is already in use; no new instance was started." }
        New-Item -ItemType Directory -Path $data -Force | Out-Null
        $process = Start-Process -FilePath $exe -ArgumentList @('--addr', '127.0.0.1', '--port', "$Port", '--data', "`"$data`"") -PassThru -WindowStyle Hidden -RedirectStandardOutput (Join-Path $data 'server.out.log') -RedirectStandardError (Join-Path $data 'server.err.log')
        try {
            Wait-Ready $process
            Set-Content -LiteralPath $pidFile -Value $process.Id -Encoding Ascii
            Write-Output "Connla ready: $url"
            Write-Output "Data directory: $data"
            if (-not $NoBrowser) { Start-Process $url }
        } catch {
            if (-not $process.HasExited) { Stop-Process -Id $process.Id }
            throw
        } finally {
            if ($SmokeTest -and -not $process.HasExited) {
                Stop-Process -Id $process.Id
                Remove-Item -LiteralPath $pidFile -ErrorAction SilentlyContinue
            }
        }
    }
    'Stop' {
        $running = Get-RecordedProcess
        if ($null -eq $running) { Write-Output 'Connla is not running.'; break }
        Stop-Process -Id $running.ProcessId
        Remove-Item -LiteralPath $pidFile
        Write-Output 'Connla stopped. Personal data was not deleted.'
    }
    'Backup' {
        if ($null -ne (Get-RecordedProcess)) { throw 'Stop the knowledge base before backing it up.' }
        if (Test-LocalPort) { throw "Port $Port is in use. Confirm the knowledge base is stopped before backing up." }
        if (-not (Test-Path -LiteralPath (Join-Path $data 'memos_prod.db') -PathType Leaf)) {
            throw "No knowledge base database found in $data"
        }
        $backups = Join-Path (Split-Path $data -Parent) 'backups'
        New-Item -ItemType Directory -Path $backups -Force | Out-Null
        $destination = Join-Path $backups (Get-Date -Format 'yyyyMMdd-HHmmss-fff')
        & $exe backup create --data $data --output $destination
        if ($LASTEXITCODE -ne 0) { throw 'Backup failed; no verified backup was published.' }
        Write-Output "Backup directory: $destination"
    }
}
