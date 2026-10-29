# scripts/reset-sandbox.ps1
# Completely removes the sandbox folder and re-runs setup-sandbox.ps1

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$workspaceDir = Split-Path -Parent $scriptDir
$sandboxDir = Join-Path $workspaceDir "sandbox"

if (Test-Path $sandboxDir) {
    Write-Host "Removing existing sandbox..."
    Remove-Item -Recurse -Force $sandboxDir
}

Write-Host "Re-creating fresh sandbox..."
& (Join-Path $scriptDir "setup-sandbox.ps1")
