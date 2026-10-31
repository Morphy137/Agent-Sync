# Prints the CHANGELOG.md section of a version (e.g. 0.0.3) as GitHub release notes.
param([Parameter(Mandatory = $true)][string]$Version)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$changelog = Get-Content (Join-Path $root "CHANGELOG.md") -Raw
$escaped = [regex]::Escape($Version)
$match = [regex]::Match($changelog, "(?ms)^## \[$escaped\][^\n]*\n(.*?)(?=^## \[|\z)")
if (-not $match.Success) { throw "CHANGELOG.md has no section for $Version" }
@"
## Downloads
- ``agentsync-setup.exe`` - installer (recommended; updates itself)
- ``agentsync.exe`` - the app without installing (add an empty ``portable`` file next to it to keep data beside the program)
- ``agentsync-cli.exe`` - command line and MCP server for AI agents
- ``SHA256SUMS.txt`` - checksums

$($match.Groups[1].Value.Trim())

## Notes
- Requires the WebView2 runtime (the installer adds it if missing) and Git.
- The executables are not code-signed; Windows SmartScreen may show a warning.
"@
