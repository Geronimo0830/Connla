param(
    [switch]$SkipFrontendBuild,
    [ValidatePattern('^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$')]
    [string]$ReleaseVersion
)

$ErrorActionPreference = 'Stop'
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$output = Join-Path $repo 'build\connla-windows'
$frontend = Join-Path $repo 'web'
$buildFlags = @()
if ($ReleaseVersion) {
    $commit = (& git -C $repo rev-parse --short=12 HEAD).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Cannot identify the source revision for this release.' }
    $buildFlags = @('-ldflags', "-X github.com/usememos/memos/internal/version.Version=$ReleaseVersion -X github.com/usememos/memos/internal/version.Commit=$commit")
}

if (-not $IsWindows -and $PSVersionTable.PSEdition -eq 'Core') {
    throw 'This packaging script must run on Windows.'
}

New-Item -ItemType Directory -Path $output -Force | Out-Null

if (-not $SkipFrontendBuild) {
    if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) { throw 'pnpm is required to build the frontend.' }
    Push-Location $frontend
    try {
        & pnpm release
        if ($LASTEXITCODE -ne 0) { throw 'Frontend release build failed.' }
    } finally {
        Pop-Location
    }
}

$binary = Join-Path $output 'Connla.exe'
if (Get-Command go -ErrorAction SilentlyContinue) {
    Push-Location $repo
    try {
        & go build @buildFlags -o $binary ./cmd/memos
        if ($LASTEXITCODE -ne 0) { throw 'Go build failed.' }
    } finally {
        Pop-Location
    }
} else {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Install Go 1.27 or start Docker Desktop to build the Windows program.' }
    Push-Location $repo
    try {
        & docker run --rm -v "${repo}:/workspace" -v 'pkb-go-mod:/go/pkg/mod' -v 'pkb-go-build:/root/.cache/go-build' -w /workspace -e GOOS=windows -e GOARCH=amd64 -e CGO_ENABLED=0 golang:1.27.0-bookworm /usr/local/go/bin/go build @buildFlags -o /workspace/build/connla-windows/Connla.exe ./cmd/memos
        if ($LASTEXITCODE -ne 0) { throw 'Docker Go build failed.' }
    } finally {
        Pop-Location
    }
}

Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'local-windows\KnowledgeBase.ps1') -Destination $output -Force
foreach ($name in @('start.cmd', 'stop.cmd', 'backup.cmd')) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "local-windows\$name") -Destination $output -Force
}
Copy-Item -LiteralPath (Join-Path $repo 'docs\LOCAL_WINDOWS.md') -Destination (Join-Path $output 'README.md') -Force
Copy-Item -LiteralPath (Join-Path $repo 'docs\BACKUP_RESTORE.md') -Destination $output -Force

& $binary version
if ($LASTEXITCODE -ne 0) { throw 'Built program failed its version check.' }
if ($ReleaseVersion) {
    $actualVersion = (& $binary version).Trim()
    if ($actualVersion -ne $ReleaseVersion) { throw "Built program reports $actualVersion instead of $ReleaseVersion." }
}

Write-Output "Connla Windows package ready: $output"
