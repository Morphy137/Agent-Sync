<div align="center">
  <img src="frontend/src/assets/app-icon-rounded.png" alt="AgentSync logo" width="120">
  <h1>AgentSync</h1>
  <p><strong>Your agent configuration files, in sync.</strong></p>
  <p>Manage and synchronize instructions, skills, and other files across your tools and computers.</p>
  <p>
    <a href="https://github.com/Morphy137/Agent-Sync/releases/latest"><img src="https://img.shields.io/github/v/release/Morphy137/Agent-Sync?label=version" alt="Latest release"></a>
    <img src="https://img.shields.io/badge/platform-Windows-555" alt="Platform: Windows">
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="License: MIT"></a>
  </p>
  <p><strong>English</strong> · <a href="README.es.md">Español</a></p>
</div>

---

AgentSync copies files and folders from a repository, a local folder, or a subscribed repository to the locations your tools use. A source can have a different name at each destination, such as `AGENTS.md` → `CLAUDE.md`.

## Features

- **Sync rules:** Send one source to multiple destinations with one-way, two-way, or create-only modes.
- **Location aliases:** Map names such as `claude` or `codex` to folders on each computer.
- **File editor:** Browse your main folder and subscriptions, edit text, preview Markdown, and format JSON.
- **Subscriptions:** Connect shared repositories, review incoming changes, and use their files in rules. Local edits stay on your computer and block updates until resolved.
- **Safety:** Review conflicts and collisions, and create backups before managed files are overwritten.
- **Git, CLI, and MCP:** Connect a remote repository and manage AgentSync through the desktop app, CLI, or MCP server.
- **English and Spanish:** Use the application in either language.

## Get started

1. Download `agentsync-setup.exe` from the [latest release](https://github.com/Morphy137/Agent-Sync/releases/latest) and install it.
2. Open AgentSync and select an existing folder, create one, or clone a repository.
3. Add a sync rule, choose its source, and select one or more destination locations.
4. Review any conflicts and synchronize.

AgentSync runs on **Windows 10 or 11, 64-bit**, and requires [Git](https://git-scm.com/). The installer includes the Microsoft Edge WebView2 runtime when needed. Portable use and update options are described in the [installation guide](docs/INSTALL_AND_UPDATES.md).

## Documentation

| Guide | Contents |
| --- | --- |
| [Installation and updates](docs/INSTALL_AND_UPDATES.md) | Installer, portable use, automatic updates, and releases |
| [Sync rules](docs/SYNC_RULES.md) | Sources, destinations, locations, modes, and transforms |
| [Remotes and subscriptions](docs/REMOTES_AND_SUBSCRIPTIONS.md) | Git remotes, shared repositories, and incoming changes |
| [CLI and MCP](docs/CLI_AND_MCP.md) | Command-line use and the MCP server |
| [Agent skill](skills/agentsync/SKILL.md) | Instructions for agents using the CLI or MCP server |
| [Recognizers](docs/RECOGNIZERS.md) | Detection of tools, instructions, and other content |
| [Architecture](docs/ARCHITECTURE.md) | Components and synchronization behavior |
| [Changelog](CHANGELOG.md) | Changes in each release |

## Development

The project uses Go, Wails v2, Svelte, and TypeScript. Development requires Go **1.25 or later**, Node.js **20.19+ or 22.12+**, npm, Git, and the Wails v2 CLI. See [Wails installation](https://wails.io/docs/gettingstarted/installation/) for the Windows build prerequisites.

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
cd frontend
npm install
cd ..
wails dev
```

Run `wails build` to produce `build/bin/agentsync.exe`. To run the project checks:

```powershell
go vet ./...
go test ./...
cd frontend
npx eslint .
npx svelte-check --threshold warning
npx vitest run
```

For an isolated two-computer sandbox, run `powershell -ExecutionPolicy Bypass -File scripts/setup-sandbox.ps1`. The [CLI guide](docs/CLI_AND_MCP.md) explains how to set `AGENTSYNC_CONFIG_DIR` for an isolated configuration.

## License

[MIT License](LICENSE). Copyright © 2026 Mr Morphy.
