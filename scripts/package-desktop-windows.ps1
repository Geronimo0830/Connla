param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$')]
    [string]$ReleaseVersion
)

$ErrorActionPreference = 'Stop'
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$output = Join-Path $repo 'build\connla-desktop'
$localDotnet = Join-Path $repo 'build\tools\dotnet\dotnet.exe'
$dotnet = if (Test-Path -LiteralPath $localDotnet -PathType Leaf) { $localDotnet } else { (Get-Command dotnet -ErrorAction Stop).Source }

& (Join-Path $PSScriptRoot 'package-local-windows.ps1') -ReleaseVersion $ReleaseVersion
if ($LASTEXITCODE -ne 0) { throw 'Connla backend packaging failed.' }

New-Item -ItemType Directory -Path $output -Force | Out-Null
& $dotnet publish (Join-Path $repo 'desktop\Connla.Desktop.csproj') -c Release -r win-x64 --self-contained true -p:Version=$ReleaseVersion -o $output
if ($LASTEXITCODE -ne 0) { throw 'Connla desktop publish failed.' }

$binary = Join-Path $output 'Connla.exe'
if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) { throw 'Desktop executable is missing.' }
Copy-Item -LiteralPath (Join-Path $repo 'docs\LOCAL_DESKTOP.md') -Destination (Join-Path $output 'README.md') -Force
Copy-Item -LiteralPath (Join-Path $repo 'THIRD_PARTY_NOTICES.md') -Destination $output -Force
Copy-Item -LiteralPath (Join-Path $repo 'BRAND_ASSETS.md') -Destination $output -Force

$webViewLicense = Join-Path $env:USERPROFILE '.nuget\packages\microsoft.web.webview2\1.0.4191.47\LICENSE.txt'
if (-not (Test-Path -LiteralPath $webViewLicense -PathType Leaf)) { throw 'WebView2 license file is missing from the restored package.' }
Copy-Item -LiteralPath $webViewLicense -Destination (Join-Path $output 'WEBVIEW2_LICENSE.txt') -Force

Write-Output "Connla desktop package ready: $output"
