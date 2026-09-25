# PXLBLZ Router v0.2.0 - Test Results

Date: 2026-09-20

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

---

## L1 — v0.3.0-dev three-controller loopback (branch `feature/v0.3-multi-controller`, 2026-09-26)

Not a hardware test. Linux build of the same Go source, `config/routes.multi-loopback.json`
(three controllers on 127.0.0.1 / .2 / .3 with the real .244 / .251 / .253 universe layout),
external WebSocket sender at 60 FPS for 4 s, then input stopped.

```text
LOOP_A_60FPS  TX 60.0/60 fps   540 pkt/s   9 universes  GRB
LOOP_B_30FPS  TX 30.0/30 fps   180 pkt/s   6 universes  BGR
LOOP_C_30FPS  TX 30.0/30 fps   870 pkt/s  29 universes  RGB (= BACK_PANEL layout)
total         1590 pkt/s, listener invalid 0, send errors 0
RX 60 fps, replaced 0, invalid 0
after 1000 ms without input: all controllers STALE/blackout, still transmitting black
```

Unit tests (`go test -race ./...`) additionally pin:

- BACK_PANEL frame unchanged: 29 universes, no U138, tails 100/174/90/390/36/300/6, one common sequence;
- independent per-controller sequence numbers; wrap 255 → 1 (never 0);
- no transmission before the first valid input frame;
- stale handling `hold` / `blackout` / `stop` and recovery when input resumes;
- `panel-walk` walks routes in config order and hands over P6 → P7.

PASS (software). Hardware re-verification on BACK_PANEL_249 with v0.3 is still required — see
`docs/V0.3_MULTI_CONTROLLER.md`, test V1.
