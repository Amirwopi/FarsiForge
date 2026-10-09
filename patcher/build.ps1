# build.ps1 — compiles FarsiForgePatcher.cs into GUI + CLI exes using the
# Windows built-in csc.exe (.NET Framework 4.x). No SDK required.
#
# Usage:
#   pwsh -File build.ps1            # build both, copy to Tools\patcher
#   pwsh -File build.ps1 -NoCopy    # build only, do not copy to Tools
#
# Output:
#   patcher\bin\FarsiForgePatcher.exe      (GUI, winexe — no console window)
#   patcher\bin\FarsiForgePatcherCli.exe   (CLI, console exe)

[CmdletBinding()]
param(
    [switch]$NoCopy
)

$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$src = Join-Path $scriptDir 'FarsiForgePatcher.cs'
$binDir = Join-Path $scriptDir 'bin'
$outDir = Join-Path $scriptDir '..\Tools\patcher' | Resolve-Path -ErrorAction SilentlyContinue

if (-not (Test-Path -LiteralPath $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
}

# ---- locate csc.exe ----
$csc = $null
$preferred = 'C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe'
if (Test-Path -LiteralPath $preferred) {
    $csc = $preferred
} else {
    $found = Get-ChildItem 'C:\Windows\Microsoft.NET\Framework64\v*\csc.exe' -ErrorAction SilentlyContinue |
        Sort-Object FullName -Descending | Select-Object -First 1
    if ($found) { $csc = $found.FullName }
}
if (-not $csc) {
    # fallback to 32-bit framework
    $found = Get-ChildItem 'C:\Windows\Microsoft.NET\Framework\v*\csc.exe' -ErrorAction SilentlyContinue |
        Sort-Object FullName -Descending | Select-Object -First 1
    if ($found) { $csc = $found.FullName }
}
if (-not $csc) {
    throw "csc.exe not found under C:\Windows\Microsoft.NET\Framework64 or Framework"
}
Write-Host "Using csc: $csc"

$refs = @('/r:System.Windows.Forms.dll', '/r:System.Drawing.dll')
$guiOut = Join-Path $binDir 'FarsiForgePatcher.exe'
$cliOut = Join-Path $binDir 'FarsiForgePatcherCli.exe'

function Invoke-Csc([string[]]$cscArgs) {
    # Use the call operator so PowerShell handles argument quoting correctly.
    $stdout = & $csc @cscArgs 2>&1
    $exit = $LASTEXITCODE
    if ($exit -ne 0) {
        $stdout | ForEach-Object { Write-Host $_ }
        throw "csc failed (exit $exit)"
    }
    $stdout | ForEach-Object { Write-Host $_ }
}

# ---- GUI build (winexe: no console window) ----
Write-Host "Building GUI (winexe) -> $guiOut"
Invoke-Csc -cscArgs @(
    '/nologo',
    '/target:winexe',
    '/platform:anycpu',
    "/out:$guiOut",
    '/r:System.Windows.Forms.dll',
    '/r:System.Drawing.dll',
    $src
)

# ---- CLI build (console exe, defines CLI) ----
Write-Host "Building CLI (exe) -> $cliOut"
Invoke-Csc -cscArgs @(
    '/nologo',
    '/target:exe',
    '/define:CLI',
    "/out:$cliOut",
    '/r:System.Windows.Forms.dll',
    '/r:System.Drawing.dll',
    $src
)

Write-Host "Build OK."

# ---- copy to Tools\patcher ----
if (-not $NoCopy) {
    $toolsDir = Join-Path $scriptDir '..\Tools\patcher'
    $toolsDir = (Resolve-Path $toolsDir -ErrorAction SilentlyContinue)
    if (-not $toolsDir) {
        $toolsDir = Join-Path $scriptDir '..\Tools\patcher'
        New-Item -ItemType Directory -Path $toolsDir -Force | Out-Null
        $toolsDir = (Resolve-Path $toolsDir).Path
    } else {
        $toolsDir = $toolsDir.Path
    }
    Copy-Item -LiteralPath $guiOut -Destination $toolsDir -Force
    Copy-Item -LiteralPath $cliOut -Destination $toolsDir -Force
    Write-Host "Copied exes to: $toolsDir"
}