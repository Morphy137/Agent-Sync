# scripts/setup-sandbox.ps1
# Creates a fully isolated test sandbox simulating remote Git repository, PC-A, and PC-B.
# Idempotent.

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$workspaceDir = Split-Path -Parent $scriptDir
$sandboxDir = Join-Path $workspaceDir "sandbox"

Write-Host "Setting up AgentSync sandbox in: $sandboxDir"

if (!(Test-Path $sandboxDir)) {
    New-Item -ItemType Directory -Path $sandboxDir -Force | Out-Null
}

$remoteGit = Join-Path $sandboxDir "remote.git"
$pcADir = Join-Path $sandboxDir "pc-a"
$pcBDir = Join-Path $sandboxDir "pc-b"

# 1. Initialize bare remote repository if not exists
if (!(Test-Path $remoteGit)) {
    Write-Host "Creating bare remote git repository..."
    git init --bare $remoteGit
}

# 2. Setup PC-A
$pcAConfigs = Join-Path $pcADir "agent-configs"
$pcAHome = Join-Path $pcADir "home"
$pcAConfigDir = Join-Path $pcADir "config"

New-Item -ItemType Directory -Path $pcAConfigDir -Force | Out-Null
New-Item -ItemType Directory -Path $pcAHome -Force | Out-Null

if (!(Test-Path (Join-Path $pcAConfigs ".git"))) {
    Write-Host "Cloning/Initializing PC-A agent configs..."
    if (Test-Path $pcAConfigs) { Remove-Item -Recurse -Force $pcAConfigs }
    
    # Initialize repository and push to bare remote
    New-Item -ItemType Directory -Path $pcAConfigs -Force | Out-Null
    git -C $pcAConfigs init -b main
    git -C $pcAConfigs config user.name "PC-A"
    git -C $pcAConfigs config user.email "pca@test.local"
    git -C $pcAConfigs remote add origin $remoteGit

    # Create profile structure
    $claudeDir = Join-Path $pcAConfigs "CLAUDE"
    New-Item -ItemType Directory -Path (Join-Path $claudeDir "commands") -Force | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $claudeDir "agents") -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $claudeDir "CLAUDE.md"), "# CLAUDE global - source v1`n")
    [System.IO.File]::WriteAllText((Join-Path $claudeDir "commands\review.md"), "# Code review instructions v1`n")
    [System.IO.File]::WriteAllText((Join-Path $claudeDir "agents\planner.md"), "# Planner agent instructions v1`n")

    $geminiDir = Join-Path $pcAConfigs "GEMINI"
    New-Item -ItemType Directory -Path $geminiDir -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $geminiDir "GEMINI.md"), "# GEMINI global - source v1`n")

    $codexDir = Join-Path $pcAConfigs "CODEX"
    New-Item -ItemType Directory -Path $codexDir -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $codexDir "AGENTS.md"), "# CODEX global - source v1`n")

    $opencodeDir = Join-Path $pcAConfigs "OPENCODE"
    New-Item -ItemType Directory -Path $opencodeDir -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $opencodeDir "AGENTS.md"), "# OPENCODE global - source v1`n")

    [System.IO.File]::WriteAllText((Join-Path $pcAConfigs ".gitignore"), ".DS_Store`nThumbs.db`ndesktop.ini`n*.agentsync.tmp`n")

    git -C $pcAConfigs add -A
    git -C $pcAConfigs commit -m "chore: initialize agent configs repository with profiles"
    git -C $pcAConfigs push -u origin main
}

# Setup PC-A fake target homes
$claudeHomeA = Join-Path $pcAHome ".claude"
New-Item -ItemType Directory -Path $claudeHomeA -Force | Out-Null
[System.IO.File]::WriteAllText((Join-Path $claudeHomeA "CLAUDE.md"), "# CLAUDE local - PC-A pre-existing version`n")
[System.IO.File]::WriteAllText((Join-Path $claudeHomeA "settings.json"), '{"apiKey": "fake-key-123", "theme": "dark"}' + "`n")

$geminiHomeA = Join-Path $pcAHome ".gemini"
New-Item -ItemType Directory -Path $geminiHomeA -Force | Out-Null
[System.IO.File]::WriteAllText((Join-Path $geminiHomeA "GEMINI.md"), "# GEMINI global - source v1`n")

New-Item -ItemType Directory -Path (Join-Path $pcAHome ".codex") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $pcAHome ".config\opencode") -Force | Out-Null

# 3. Setup PC-B
$pcBConfigs = Join-Path $pcBDir "agent-configs"
$pcBHome = Join-Path $pcBDir "home"
$pcBConfigDir = Join-Path $pcBDir "config"

New-Item -ItemType Directory -Path $pcBConfigDir -Force | Out-Null
New-Item -ItemType Directory -Path $pcBHome -Force | Out-Null

if (!(Test-Path (Join-Path $pcBConfigs ".git"))) {
    Write-Host "Cloning PC-B agent configs from bare remote..."
    if (Test-Path $pcBConfigs) { Remove-Item -Recurse -Force $pcBConfigs }
    git clone $remoteGit $pcBConfigs
    git -C $pcBConfigs config user.name "PC-B"
    git -C $pcBConfigs config user.email "pcb@test.local"
}

# Setup PC-B fake target homes and clone parent folder
New-Item -ItemType Directory -Path (Join-Path $pcBHome ".claude") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $pcBHome ".gemini") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $pcBHome ".codex") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $pcBHome ".config\opencode") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $pcBDir "empty-parent") -Force | Out-Null

Write-Host "AgentSync sandbox setup completed successfully."
Write-Host "To run in sandbox mode with isolated home:"
Write-Host '  $env:AGENTSYNC_HOME_OVERRIDE = "' + (Join-Path $pcADir "home") + '"'
Write-Host '  $env:APPDATA = "' + (Join-Path $pcADir "appdata") + '"'
Write-Host '  wails dev'
