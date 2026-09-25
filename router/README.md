# PXLBLZ Art-Net Router v0.3.0-dev

> **v0.3 (branch `feature/v0.3-multi-controller`, software-verified, hardware pending):** per-controller scheduling (own FPS + own Art-Net sequence per controller IP), config v2 with `controllers[]`, stale-input `hold|blackout|stop`, `panel-walk` visual test pattern, `GET /status` JSON on `127.0.0.1:9981`, full known installation config. v1 configs behave exactly as before. See `../docs/V0.3_MULTI_CONTROLLER.md`.
>
> New quick tests: `run-multi-loopback-test.bat`, `run-backpanel-panel-walk-test.bat`, `run-installation-ws.bat`.


> **v0.2.0 hardware-routing test build:** keeps the v0.1.1 even-length ArtDmx compatibility fix and adds the `port-id` diagnostic pattern plus a ready-to-run 7-port BACK_PANEL_249 configuration.

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
