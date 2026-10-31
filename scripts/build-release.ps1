# Builds the release assets into build/bin:
#   agentsync.exe, agentsync-cli.exe, agentsync-setup.exe (per-user installer) and SHA256SUMS.txt.
# Requires Go, Node, the Wails CLI and NSIS (makensis on PATH or in its default folder).
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$version = (Get-Content wails.json -Raw | ConvertFrom-Json).info.productVersion
$ldflags = "-s -w"
$bin = Join-Path $root "build\bin"

$nsisDir = "${env:ProgramFiles(x86)}\NSIS"
if (-not (Get-Command makensis -ErrorAction SilentlyContinue) -and (Test-Path $nsisDir)) {
    $env:PATH = "$nsisDir;$env:PATH"
}

Write-Host "Building AgentSync $version"
wails build -clean -trimpath -ldflags $ldflags
if ($LASTEXITCODE -ne 0) { throw "wails build failed" }

# The CLI must exist before the installer is built so it gets bundled.
go build -trimpath -ldflags "$ldflags -X agentsync/internal/cli.Version=$version" -o (Join-Path $bin "agentsync-cli.exe") ./cmd/agentsync-cli
if ($LASTEXITCODE -ne 0) { throw "CLI build failed" }

wails build -nsis -trimpath -ldflags $ldflags
if ($LASTEXITCODE -ne 0) { throw "installer build failed" }

# Wails rewrites go.mod line endings and the frontend hash; keep the tree clean.
git checkout -- go.mod frontend/package.json.md5 build/windows/installer/wails_tools.nsh 2>$null

$assets = "agentsync.exe", "agentsync-cli.exe", "agentsync-setup.exe"
$lines = foreach ($name in $assets) {
    $hash = (Get-FileHash (Join-Path $bin $name) -Algorithm SHA256).Hash.ToLower()
    "$hash *$name"
}
Set-Content -Path (Join-Path $bin "SHA256SUMS.txt") -Value $lines -Encoding ascii
Write-Host "Release assets ready in $bin"
Get-ChildItem $bin | Select-Object Name, Length
