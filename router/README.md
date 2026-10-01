# PXLBLZ Art-Net Router v0.3.0-dev

> **v0.3 (branch `feature/v0.3-multi-controller`, software-verified, installation verified on hardware 2026-10-02):** per-controller scheduling (own FPS + own Art-Net sequence per controller IP), config v2 with `controllers[]`, stale-input `hold|blackout|stop`, `input.variable_size`, `panel-walk` visual test pattern, full installation config. v1 configs behave exactly as before. See `../docs/V0.3_MULTI_CONTROLLER.md`.
>
> New quick tests: `run-multi-loopback-test.bat`, `run-backpanel-panel-walk-test.bat`, `run-installation-ws.bat`.

## Settings page and API (v0.3, `127.0.0.1:9988`)

`--status-listen` (default `127.0.0.1:9988`; not 9981, TouchDesigner uses it) serves:

| Endpoint | Purpose |
| --- | --- |
| `GET /` | settings page (German): controllers, routes, input, live status |
| `GET /status` | JSON: RX counters, last frame age, WS clients, per-controller frames/packets/errors/stale |
| `GET /api/config`, `PUT /api/config` | read / validate + apply live (no restart) + save; backup to `config/backups` (newest 20); unknown JSON fields are rejected |
| `POST /api/config/check` | validate only |
| `GET /api/subnets`, `GET /api/discover` | find controllers on the local networks (firmware `/api/config` + `/api/status`) |
| `GET /api/controller?ip=` | read one controller's outputs and status |
| `POST /api/import` | take over a controller's outputs as routes |
| `POST /api/test?pattern=&seconds=`, `POST /api/test/stop` | test patterns (rainbow, port-id, ...) instead of PXLBLZ |

Only loopback hosts and same-origin requests are accepted; responses carry
`X-PXLBLZ-Router`, and the router checks at startup that the port really answers as itself.
Applying a config keeps the latest frame when the size is unchanged and continues the Art-Net
sequence numbers (`internal/engine`). Configs:

| Config | Use |
| --- | --- |
| `config/routes.installation-live.json` | running installation: .248 test rig, .253 Teensy, .251 APA102, 60 fps, RGB |
| `config/routes.esp-test-172-8x8-12x6.json` | ESP test rig 10.0.0.248 alone (136 px, U149) |
| `config/routes.installation-full.json` | earlier 8186-pixel logical frame incl. .244 (reference) |
| `config/routes.backpanel-*.json` | BACK_PANEL_249 hardware tests |

Measuring through the running router: `node ../perf-test/installation-fps-test.mjs --fps 60 --seconds 30`.

## Merged from main (integration 2026-10)

- Optional per-route `"brightness": 0..1` (default 1). Applied in the router through a 256-entry
  lookup table together with the colour reorder; shown as `level` in the route summary.
- Virtual Teensy controller / receiver / visualizer (`cmd/pxlblz-virtual-controller`,
  `cmd/pxlblz-receiver-probe`, `internal/virtualcontroller`) - see `VIRTUAL_CONTROLLER.md`.


> **v0.2.1 hardware-routing test build:** keeps the v0.1.1 even-length ArtDmx compatibility fix and adds the `port-id` diagnostic pattern plus a ready-to-run 7-port BACK_PANEL_249 configuration.

Current verified data path:

```text
external RGB source
    ↓ binary WebSocket
ws://127.0.0.1:9980/pixels
    ↓ latest-frame handoff
pxlblz-router.exe
    ↓ Art-Net ArtDmx / UDP unicast
BACK_PANEL_249
    ↓
7 electrical outputs / 6 physical panels
```

## Included tools

- `pxlblz-router.exe`
- `pxlblz-frame-sender.exe`
- `artnet-listener.exe`

## Quick tests

Loopback:

```bat
run-loopback-test.bat
```

WebSocket live-input loopback:

```bat
run-ws-loopback-test.bat
```

Hardware-verified BACK_PANEL external-frame test:

```bat
run-backpanel-ws-port-id-test.bat
```

See `LIVE_INPUT_v0.2.md`, `TEST_RESULTS.md` and `../docs/PROJECT_PLAN.md` for details.


## Known installation configuration

`config/routes.installation-known.json` combines all controllers whose exact
routing is currently known:

```text
10.0.0.244  WS2812_NODE    GRB
10.0.0.253  BACK_PANEL_249 RGB
10.0.0.251  PANEL8_251     BGR
```

It uses 8186 logical input pixels and emits 44 Art-Net universes per frame,
which is 1320 packets/s at the current common 30 FPS test rate.

The config is virtually verified over real loopback UDP. APA102 is deliberately
not included because its exact route is still unknown. (Historical: the running
installation is `config/routes.installation-live.json`, where .251 is the APA102 controller.)

Fresh clones can use the committed binaries automatically through the
`run-*.bat` launchers under this directory.
