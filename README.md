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

## Current status

- Router v0.2 live-input path verified on Windows
- Binary WebSocket input on `ws://127.0.0.1:9980/pixels`
- Latest-frame semantics: old video frames are replaced, never queued
- Art-Net universe routing with receiver-compatible even-length final payloads
- BACK_PANEL_249 hardware test passed across all **7 electrical outputs / 6 physical panels**
- External sender 60 FPS → router 30 FPS → 29 Art-Net universes → real LEDs: PASS
- Next milestone: direct PXLBLZ IDE render-frame integration

## Repository layout

```text
router/
  cmd/                       Go entry points
  internal/                  Art-Net, routing, WebSocket, LatestFrame
  config/                    verified/test routing configurations
  third_party/projectMM/     provenance + GPLv3 license
  *.bat                      Windows build/test helpers

bin/windows-x64/
  pxlblz-router.exe
  pxlblz-frame-sender.exe
  artnet-listener.exe
  SHA256SUMS.txt

docs/
  PROJECT_PLAN.md

controller-reference/
  BACK_PANEL_249_RECEIVER.md

.github/workflows/
  build-windows.yml
```

## Windows binaries

The current Windows x64 executables are committed under:

```text
bin/windows-x64/
```

They are generated from the committed Go source by GitHub Actions after the test suite passes. Checksums are in `bin/windows-x64/SHA256SUMS.txt`.

## Build from source

```bat
cd router
go test ./...
build-windows.bat
```

## Hardware routing currently verified

`BACK_PANEL_249` at `10.0.0.253:6454`:

```text
P1  pixels 1440..1642  U120..U121  203 LEDs
P2  pixels 3744..4481  U122..U126  738 LEDs
P3  pixels 4482..5361  U127..U132  880 LEDs
P4  pixels 5362..6171  U133..U137  810 LEDs
P5  pixels 6172..6523  U139..U141  352 LEDs
P6  pixels 6524..7133  U142..U145  610 LEDs
P7  pixels 7134..7645  U146..U149  512 LEDs
```

P6 + P7 are two electrical lanes belonging to the same sixth physical panel.

See [docs/PROJECT_PLAN.md](docs/PROJECT_PLAN.md) for architecture, test history, receiver requirements, and the deferred fine-tuning backlog.
