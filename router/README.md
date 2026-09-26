# PXLBLZ Art-Net Router v0.2.1

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
not included because its exact route is still unknown.

Fresh clones can use the committed binaries automatically through the
`run-*.bat` launchers under this directory.
