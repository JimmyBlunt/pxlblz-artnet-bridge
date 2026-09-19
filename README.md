# pxlblz-artnet-bridge

Native Art-Net output bridge for [PXLBLZ IDE](https://github.com/jon-whiteroomsoftware/PXLBLZ-IDE).

Current verified pipeline:

```text
PXLBLZ / external RGB source
        ↓ binary WebSocket
pxlblz-router.exe
        ↓ Art-Net ArtDmx / UDP unicast
LED controllers
        ↓
physical LEDs
```

## Current status

- Windows router v0.2 live-input path verified
- Binary WebSocket input on `ws://127.0.0.1:9980/pixels`
- Latest-frame semantics, no accumulating video queue
- Art-Net universe routing and even-length final-payload compatibility
- BACK_PANEL_249 real-hardware test passed across all 7 electrical outputs / 6 physical panels
- Next milestone: direct PXLBLZ IDE render-frame integration

See `docs/PROJECT_PLAN.md` for architecture, hardware findings, test history and deferred tuning work.
