<div align="center">
  <img src="frontend/src/assets/app-icon-rounded.png" alt="Logo de AgentSync" width="120">
  <h1>AgentSync</h1>
  <p><strong>Los archivos de configuración de tus agentes, sincronizados.</strong></p>
  <p>Administra y sincroniza instrucciones, habilidades y otros archivos entre tus herramientas y equipos.</p>
  <p>
    <a href="https://github.com/Morphy137/Agent-Sync/releases/latest"><img src="https://img.shields.io/github/v/release/Morphy137/Agent-Sync?label=versi%C3%B3n" alt="Última versión"></a>
    <img src="https://img.shields.io/badge/plataforma-Windows-555" alt="Plataforma: Windows">
    <a href="LICENSE"><img src="https://img.shields.io/badge/licencia-MIT-blue" alt="Licencia: MIT"></a>
  </p>
  <p><a href="README.md">English</a> · <strong>Español</strong></p>
</div>

---

AgentSync copia archivos y carpetas de un repositorio, una carpeta local o un repositorio suscrito a las ubicaciones que usan tus herramientas. Un mismo origen puede tener un nombre diferente en cada destino, por ejemplo `AGENTS.md` → `CLAUDE.md`.

## Funciones

- **Reglas de sincronización:** Envía un origen a varios destinos con los modos unidireccional, bidireccional o solo creación.
- **Alias de ubicaciones:** Asigna nombres como `claude` o `codex` a carpetas de cada equipo.
- **Editor de archivos:** Explora tu carpeta principal y suscripciones, edita texto, previsualiza Markdown y formatea JSON.
- **Suscripciones:** Conecta repositorios compartidos, revisa los cambios entrantes y usa sus archivos en reglas. Las ediciones locales permanecen en tu equipo y bloquean las actualizaciones hasta que se resuelvan.
- **Protección:** Revisa conflictos y colisiones; se crean copias de seguridad antes de sobrescribir archivos gestionados.
- **Git, CLI y MCP:** Conecta un repositorio remoto y administra AgentSync desde la aplicación, la CLI o el servidor MCP.
- **Inglés y español:** Usa la aplicación en cualquiera de los dos idiomas.

## Primeros pasos

1. Descarga `agentsync-setup.exe` desde la [última versión](https://github.com/Morphy137/Agent-Sync/releases/latest) e instálalo.
2. Abre AgentSync y selecciona una carpeta existente, crea una o clona un repositorio.
3. Agrega una regla de sincronización, elige su origen y selecciona una o varias ubicaciones de destino.
4. Revisa los conflictos que aparezcan y sincroniza.

AgentSync funciona en **Windows 10 u 11 de 64 bits** y requiere [Git](https://git-scm.com/). El instalador incluye el entorno de ejecución de Microsoft Edge WebView2 cuando es necesario. La [guía de instalación](docs/INSTALL_AND_UPDATES.md) explica el uso portátil y las opciones de actualización.

## Documentación

Las siguientes guías técnicas están disponibles en inglés:

| Guía | Contenido |
| --- | --- |
| [Instalación y actualizaciones](docs/INSTALL_AND_UPDATES.md) | Instalador, uso portátil, actualizaciones y versiones |
| [Reglas de sincronización](docs/SYNC_RULES.md) | Orígenes, destinos, ubicaciones, modos y transformaciones |
| [Remotos y suscripciones](docs/REMOTES_AND_SUBSCRIPTIONS.md) | Remotos Git, repositorios compartidos y cambios entrantes |
| [CLI y MCP](docs/CLI_AND_MCP.md) | Uso de la línea de comandos y del servidor MCP |
| [Skill para agentes](skills/agentsync/SKILL.md) | Instrucciones para agentes que usan la CLI o el servidor MCP |
| [Reconocedores](docs/RECOGNIZERS.md) | Detección de herramientas, instrucciones y otros contenidos |
| [Arquitectura](docs/ARCHITECTURE.md) | Componentes y funcionamiento de la sincronización |
| [Historial de cambios](CHANGELOG.md) | Cambios de cada versión |

## Desarrollo

El proyecto utiliza Go, Wails v2, Svelte y TypeScript. Para desarrollar se necesita Go **1.25 o superior**, Node.js **20.19+ o 22.12+**, npm, Git y la CLI de Wails v2. Consulta la [instalación de Wails](https://wails.io/docs/gettingstarted/installation/) para conocer los requisitos de compilación en Windows.

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
cd frontend
npm install
cd ..
wails dev
```

Ejecuta `wails build` para generar `build/bin/agentsync.exe`. Para ejecutar las comprobaciones del proyecto:

```powershell
go vet ./...
go test ./...
cd frontend
npx eslint .
npx svelte-check --threshold warning
npx vitest run
```

Para crear un entorno aislado con dos equipos, ejecuta `powershell -ExecutionPolicy Bypass -File scripts/setup-sandbox.ps1`. La [guía de la CLI](docs/CLI_AND_MCP.md) explica cómo usar `AGENTSYNC_CONFIG_DIR` para aislar la configuración.

## Licencia

[Licencia MIT](LICENSE). Copyright © 2026 Mr Morphy.
