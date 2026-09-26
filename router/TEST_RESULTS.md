# PXLBLZ Router v0.2.1 - Test Results

Updated: 2026-09-26

## Unit tests

`go test ./...` passes for:

- ArtDmx packet construction/parsing
- even-payload receiver compatibility
- route/universe planning
- test patterns
- LatestFrame replacement semantics
- minimal RFC6455 WebSocket binary round-trip

## Windows build

Cross-built successfully for Windows x64:

- `pxlblz-router.exe`
- `artnet-listener.exe`
- `pxlblz-frame-sender.exe`

## Live-input stress test

```text
pxlblz-frame-sender 120 FPS
        ↓
ws://127.0.0.1:9980/pixels
        ↓
LatestFrame
        ↓
pxlblz-router 60 FPS
        ↓
48 Art-Net universes
```

Observed:

```text
600 WebSocket frames / 5 s
~120 input FPS
~60 output FPS
~60 frames/s replaced, not queued
2880 Art-Net packets/s
0 invalid frames
0 send errors
```

## Real hardware result

BACK_PANEL_249 external WebSocket input test:

```text
8186 logical pixels
60 FPS external input
30 FPS Art-Net output
29 universes/frame
870 packets/s nominal
0 invalid input frames
0 send errors
all 6 physical panels visibly active
```

BACK_PANEL has 7 electrical outputs; P6 and P7 are two electrical lanes of the same sixth physical panel.

PASS.

## Direct PXLBLZ IDE integration

First direct browser integration test from the real PXLBLZ IDE succeeded.

Source state:

```text
PXLBLZ IDE upstream commit d685125b
experimental ?pxout=1 adapter
ws://127.0.0.1:9980/pixels
8186 logical pixels / 24558 bytes per frame
```

Observed router telemetry:

```text
clients 1
RX 60.0 fps
replaced ~30/s
invalid 0/s
TX 30.0 fps
870 pkt/s
0 send errors
```

This proves the direct transport path:

```text
PXLBLZ render loop
→ browser WebSocket
→ router LatestFrame
→ Art-Net sender
```

PASS for direct PXLBLZ-to-router transport and frame-size agreement.

Physical visual correctness of the selected live PXLBLZ pattern is tracked separately from transport acceptance.

## Virtual exact-adapter integration environment

The real `pxlblz-integration/src/externalPixelOutput.ts` was compiled and run
against the real Go router without physical LED hardware.

Adapter self-test:

```text
ADAPTER_SELFTEST_PASS
Float [0,1] -> RGB888 conversion/clamping: PASS
wrong frame size -> drop: PASS
browser websocket backpressure -> drop, not queue: PASS
?pxout absent -> no-op: PASS
```

Virtual E2E A:

```text
8000 pixels
adapter input ~89-91 FPS
router output 60 FPS
48 universes U0-U47
2880 Art-Net packets/s
listener invalid 0
router invalid 0
send errors 0
VIRTUAL_E2E_A_PASS
```

Virtual E2E B using the production BACK_PANEL route table:

```text
8186 pixels / 24558 bytes
adapter ~60 FPS
router 30 FPS
29 universes/frame
870 packets/s
--dry-run
invalid 0
send errors 0
VIRTUAL_E2E_B_PASS
ALL_VIRTUAL_TESTS_PASS
```

The test environment lives under `pxlblz-integration/virtual-test/`.


## Known-installation virtual UDP gate

The combined known-controller configuration was tested with all physical IPs
temporarily redirected to localhost while preserving logical pixel ranges,
universes and color orders.

```text
input: 8186 pixels / 24558 bytes
controllers represented: 3
routes: 12
Art-Net universes/frame: 44
router target: 30 FPS
expected packet rate: 1320 packets/s
orders exercised: GRB + RGB + BGR
actual UDP loopback: yes
invalid packets: 0
websocket invalid frames: 0
send errors: 0
```

The listener explicitly observed boundary universes across all three controller
groups, including U0, U3, U6, U7, U12, U14, U120, U149, U156 and U161.

PASS on Windows and Linux virtual CI.

## Current software regression status

As of 2026-09-26:

- Go unit tests: PASS
- exact TypeScript adapter self-test: PASS
- exact adapter → WS → router → UDP loopback: PASS
- BACK_PANEL production route dry-run: PASS
- known 3-controller route over real loopback UDP: PASS
- performance smoke lab: PASS on Windows and Linux
- Windows PXLBLZ installer against pinned upstream: PASS
- modified PXLBLZ production build: PASS
- patch whitespace check: PASS
- Windows local D1 + synthetic Studio session: PASS
- local Worker `/api/me` with `github:local-dev`: PASS

The remaining acceptance items require the real installation: visual validation
of WS2812_NODE/PANEL8_251 and the missing exact APA102 routing data.
