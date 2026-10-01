# Windows desktop starter: PXLBLZ → Art-Net (v0.3)

Art-Net counterpart of the Fadecandy starter (`windows-launcher/`, tag `fadecandy-v0.1.1`
on `feature/fadecandy-l3d-output`). Same workspace layout, same local login, same
dedicated browser profile — the Art-Net router v0.3 replaces fcserver + Fadecandy bridge.
Installed separately (`%LOCALAPPDATA%\PXLBLZ-IDE-ArtNet`, own mutex and browser
profile), so it does not interfere with an existing Fadecandy installation.

**Configuration page:** while the router runs, `http://127.0.0.1:9988/` edits controllers, IPs,
routes and input live (validated, applied without restart, saved with backup), finds controllers
on the network and runs test patterns.

**Configuration help (German, step by step: controller IPs, routing targets, universes,
LED counts, changing the active config, troubleshooting):
[`docs/KONFIGURATION_HILFE.md`](../docs/KONFIGURATION_HILFE.md).**

## Workspace layout

```text
E:\PXLBLZ-ArtNet\
  PXLBLZ-IDE-main\   upstream d685125b: local API + D1 (.wrangler/state) on :5174
  PXLBLZ-IDE\        same revision + output adapter (externalPixelOutput.ts), UI on :5175
  artnet\            pxlblz-router.exe + config\ (copied by the installer)
```

Patterns, maps and shows live in the local D1 of `PXLBLZ-IDE-main` on this PC only —
exactly as in the Fadecandy setup. Nothing is synchronised with GitHub or the online IDE.

## Setup

1. Node.js 24+, Git, Chrome or Edge.
2. Clone `https://github.com/jon-whiteroomsoftware/PXLBLZ-IDE.git` as `PXLBLZ-IDE-main`,
   reset to `d685125b34c694f311972e258efb48d12cf05cd8`, add the worktree `PXLBLZ-IDE`
   (branch `pxlblz-artnet-output`) at the same commit.
3. `pxlblz-integration\install-pxlblz-output.ps1 -PxlblzPath <workspace>\PXLBLZ-IDE`.
4. `npm ci` in both IDE folders (set `npm_config_cache` to a drive with space if C: is full).
5. In `PXLBLZ-IDE-main`: create `.dev.vars` from `.dev.vars.example` with a long random
   `SESSION_SECRET` (OAuth fields stay empty), copy it to `PXLBLZ-IDE`, run
   `npm run db:migrate:local`, then seed the upstream local identities
   (`localIdentitySeedSql(dev-runtime.json)` from `scripts/dev-runtime-auth.ts`) with
   `npx tsx <bridge>\pxlblz-integration\tools\seed-local-identity.mts seed.sql` and
   `npx wrangler d1 execute pxlblz-ide --local --file seed.sql`. Maps/patterns from the repo
   can be imported with `tools\local-d1-import.mts` (see `pxlblz-integration/README.md`).
6. Build the router (or use a v0.3 test build) and install:

```powershell
.\windows-launcher-artnet\Install-DesktopShortcut.ps1 -WorkspaceRoot 'E:\PXLBLZ-ArtNet' `
    -RouterBinDirectory '<folder with pxlblz-router.exe>' -RouterConfig 'config\routes.installation-live.json'
```

`routes.installation-live.json` is the whole verified installation (ESP test rig .248,
Teensy .253, APA102 .251); `routes.esp-test-172-8x8-12x6.json` drives the test rig alone.

Double-click **PXLBLZ-IDE - ArtNet** on the desktop (also in the Start menu folder
**PXLBLZ-IDE ArtNet**). A small window "PXLBLZ-IDE~ArtNet wird gestartet ..." shows the
checks; it closes by itself after success (10-30 s) and stays open with the error message
otherwise. Every run is logged first in `launch-history.log` - if a click shows no window
and no new line there, Windows never started the script. The installer also creates
**PXLBLZ-ArtNet Einstellungen** (opens `http://127.0.0.1:9988/`) on the desktop and in the
Start menu.

Re-running the installer is safe: it replaces the router exe (stopping this workspace's
running router first), **adds** new repository configs but **keeps** existing workspace
configs (reported as "Config behalten"; replace them only with `-OverwriteConfigs`, which
backs up the old file to `config\backups`), and keeps the previously chosen router config
when `-RouterConfig` is omitted. The workspace config is the one the router's web page edits.

In the output-enabled Studio tab (title **PXLBLZ-IDE~ArtNet**) a small badge at the bottom
left shows router state, frames per second arriving at the router, controllers and running
test patterns, with a link to the configuration page (green = frames flowing, amber = no
frames / test pattern, red = router not reachable; click "ArtNet" to collapse).

| Component | Address | Verified |
| --- | --- | --- |
| Local API and D1 | localhost:5174 | `__identity` = `PXLBLZ-IDE-main` |
| IDE with output adapter | localhost:5175 | `__identity` = `PXLBLZ-IDE`; served Preview contains `createExternalPixelOutput` |
| Art-Net router v0.3 | ws://127.0.0.1:9980/pixels, status http://127.0.0.1:9988/status | executable path, config in live process arguments, `--list-routes` valid |
| Router configuration page | http://127.0.0.1:9988/ | answered by this router (`X-PXLBLZ-Router` header) - warning only if another program holds the port |
| Local account | both IDE API routes | signed `github:local-dev` session accepted |

The login helper creates the signed session in the background, verifies it against both
API routes and hands it to the dedicated browser profile through a one-time loopback
redirect as an **HttpOnly** cookie. The token is never printed or stored in a file.
The Studio opens with `?pxout=1&pxoutUrl=ws://127.0.0.1:9980/pixels`.

`-CheckOnly` checks without starting anything; `-NoOpen` starts services but no browser.
Status and errors: `%LOCALAPPDATA%\PXLBLZ-IDE-ArtNet\artnet-start-status.txt` plus logs.

## Frame size

Router configs used with the IDE set `input.variable_size: true` (as the Fadecandy
bridge does): any whole-pixel PXLBLZ map size is accepted, the first `pixel_count`
pixels are routed, missing pixels are black. Configs without it keep the exact-size
check (e.g. the hardware-verified BACK_PANEL).

## Limits (same as Fadecandy v0.1.1)

Preview-driven: a paused/background browser tab stops sending, the router applies
`on_stale` after `stale_timeout_ms`. Preview brightness is not applied to the RGB output.
Use only one output-enabled tab at a time.
