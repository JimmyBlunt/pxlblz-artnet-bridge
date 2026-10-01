# pxlblz-artnet-bridge

Native Art-Net output bridge for [PXLBLZ IDE](https://github.com/jon-whiteroomsoftware/PXLBLZ-IDE).

## Verified pipeline

```text
PXLBLZ / external RGB source
        ↓ binary WebSocket
ws://127.0.0.1:9980/pixels
        ↓ latest-frame handoff
pxlblz-router.exe
        ↓ Art-Net ArtDmx / UDP unicast
LED controllers
        ↓
physical LEDs
```

## Current status (branch `feature/v0.3-multi-controller`, 2026-10-02)

- **Router v0.3**: several controllers in one config (`controllers[]`), own scheduler, FPS
  target and Art-Net sequence per controller, `on_stale` blackout, `input.variable_size`
  - see [docs/V0.3_MULTI_CONTROLLER.md](docs/V0.3_MULTI_CONTROLLER.md)
- **Settings page** at `http://127.0.0.1:9988/`: edit, check and apply the config live (with
  backups), discover controllers and import their outputs, test patterns - see
  [docs/KONFIGURATION_HILFE.md](docs/KONFIGURATION_HILFE.md)
- **PXLBLZ IDE runs locally** with the Art-Net output adapter (tab "PXLBLZ-IDE~ArtNet", status
  badge) and a desktop starter: [`windows-launcher-artnet/`](windows-launcher-artnet/README.md)
- **Whole installation verified on hardware** at 60 fps router target (port-id colors OK):
  Teensy .253 (4593 px, 32 universes), APA102 .251 (805 px), ESP test rig .248 (136 px).
  Running config: `router/config/routes.installation-live.json`; controllers:
  [controller-reference/INSTALLATION_CONTROLLERS.md](controller-reference/INSTALLATION_CONTROLLERS.md)
- Router and wired network deliver 60 fps loss-free. At the LEDs: .248 ~59 fps, .253 ~34 fps
  (WS2812 lane length), .251 ~57 fps (firmware receive loop; fix prepared in
  [docs/ESP_FIRMWARE_RX_FIX_PROMPT.md](docs/ESP_FIRMWARE_RX_FIX_PROMPT.md)) - details and
  options in [docs/FPS60_AUSWERTUNG.md](docs/FPS60_AUSWERTUNG.md)
- Unchanged from v0.2: binary WebSocket input `ws://127.0.0.1:9980/pixels`, latest-frame
  semantics (old frames are replaced, never queued), even-length final ArtDmx payloads

## Repository layout

```text
router/
  cmd/pxlblz-router/         router + settings page (web.go, ui/index.html)
  cmd/                       frame sender, Art-Net listener, perf tools
  internal/                  Art-Net, routing, scheduler, engine (live config swap),
                             controllerapi (firmware API discovery), WebSocket, LatestFrame
  config/                    routing configs (routes.installation-live.json = running installation)
  third_party/projectMM/     provenance + GPLv3 license
  *.bat                      Windows build/test helpers
  TEST_RESULTS.md            software, perf and hardware test log

bin/windows-x64/             v0.2 executables from main (not updated on the v0.3 branch)

docs/
  KONFIGURATION_HILFE.md     settings page, IPs, routing, troubleshooting (German)
  FPS60_AUSWERTUNG.md        60 fps measurements and optimization options
  KNOWN_INSTALLATION_TOPOLOGY.md, V0.3_MULTI_CONTROLLER.md, CURRENT_STATUS_HANDOFF.md,
  ESP_FIRMWARE_RX_FIX_PROMPT.md, PROJECT_PLAN.md, ...

controller-reference/
  INSTALLATION_CONTROLLERS.md  verified controllers, outputs, APIs
  snapshots-2026-10-02/        controller /api/config snapshots
  BACK_PANEL_249_RECEIVER.md

pxlblz-integration/
  src/externalPixelOutput.ts   PXLBLZ IDE output adapter
  maps/, patterns/             PXLBLZ map + pattern for the ESP test rig
  tools/                       local D1 import / identity seed for the local IDE
  install-pxlblz-output.ps1, virtual-test/

windows-launcher-artnet/     desktop starter (router + local PXLBLZ IDE)
perf-test/                   hardware ramp + installation FPS test through the router API
```

## Windows binaries

The v0.2 Windows x64 executables on `main` are committed under `bin/windows-x64/`, generated
by GitHub Actions after the test suite passes (checksums in `SHA256SUMS.txt`). The v0.3 branch
does not touch `bin/`; build the v0.3 router from source (below) - the desktop starter
installs the router it is given.

## Build from source

```bat
cd router
go test ./...
build-windows.bat
```

## Hardware routing currently verified

`routes.installation-live.json` (2026-10-02), all RGB, 60 fps router target:

```text
ESP test rig 10.0.0.248   pixels    0..135   U149            136 LEDs (APA102)
Teensy       10.0.0.253   pixels  136..4728  U120..U152*    4593 LEDs (WS2812B, 8 outputs)
APA102       10.0.0.251   pixels 4729..5533  U156..U161      805 LEDs (2 outputs)
* without U138
```

Per-output tables and the earlier 8186-pixel BACK_PANEL_249 frame:
[docs/KNOWN_INSTALLATION_TOPOLOGY.md](docs/KNOWN_INSTALLATION_TOPOLOGY.md).

See [docs/PROJECT_PLAN.md](docs/PROJECT_PLAN.md) for architecture, test history, receiver requirements, and the deferred fine-tuning backlog.

## Virtual end-to-end regression test

The hardware-free regression test uses the **real TypeScript PXLBLZ output adapter**
and the real Go router:

```bash
node pxlblz-integration/virtual-test/run-virtual-e2e.mjs
```

It validates Float-to-RGB888 conversion, websocket backpressure semantics, the
8000-pixel / 48-universe UDP loopback path, and the production 8186-pixel
BACK_PANEL routing dry-run. CI runs the same gate on Linux and Windows.

See `pxlblz-integration/virtual-test/README.md`.
