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

## L2 — v0.3.0-dev after merging `main` (v0.2.1 performance telemetry), 2026-09-26

Windows 11 x64, Go 1.23.12, EXEs built exactly like `.github/workflows/build-windows.yml`.
Send-time telemetry (`TotalFrameTime`) is now counted per controller; the aggregate
`Final TX:` and `RX ... | send avg ... | heap ...` lines keep the `main` format.

```text
go vet ./...                        clean
go test ./...                       all packages ok (new: TestSendTotalsSumControllers)
BenchmarkSendFrameDryRunBackPanel8186   0 allocs/op
three-controller loopback, 60 fps sender for 7 s:
  LOOP_A 60/60 fps, LOOP_B 30/30 fps, LOOP_C 30/30 fps, 1590 pkt/s, 0 send errors, listener invalid 0
  after 1000 ms without input: all controllers STALE/blackout
  /status JSON served on 127.0.0.1:9981
perf-test/run-performance.mjs regexes against router stdout: 11 RX lines + Final TX matched
```

Not run locally: `run-virtual-e2e.mjs` and `run-performance.mjs` (need `tsc`).
PASS (software). Hardware V1–V4 still open.

## P1 — Performance lab: virtual Art-Net controllers, `main` vs v0.3 (2026-09-26)

Windows 11, i5-1345U laptop (P+E cores, on battery profile unknown), Go 1.27.0, Node 24,
TypeScript 5.8.3. Every run: exact PXLBLZ adapter → router (WebSocket) → frame-aware
`artnet-probe` virtual receiver(s). `main` = `fix/perf-lab-script` (main + lab repairs).

Lab repairs needed first (both on `fix/perf-lab-script`, merged here):

- `perf-test/run-performance.mjs` was unparsable since 9390418 (`$'` in a `String.replace`
  replacement spliced the file tail in three times) — rebuilt.
- `artnet-probe` used the OS default UDP buffer (64 KiB on Windows); a 193-universe stress
  burst (~100 KB) overflowed it and produced false incomplete frames on **both** branches.
  Now `--rcvbuf` 8 MiB by default.

New profile `installation`: three virtual controllers, one probe each (127.0.0.1/.2/.3).

```text
profile       side  result  probe fps           pps     incompl gaps  send avg p50/p99 ms
smoke         main  PASS    30.00                 871   0       0     0.44 / 0.63
smoke         v0.3  PASS    30.00                 870   0       0     0.35 / 0.48
perf          main  PASS    60.00                1740   0       0     0.44 / 0.59
perf          v0.3  PASS    60.00                1740   0       0     0.44 / 0.57
stress #1     main  PASS   120.00               23161   0       0     2.60 / 3.23
stress #1     v0.3  PASS   120.00               23162   0       0     2.91 / 3.81
stress #2     main  PASS   120.00               23162   0       0     2.83 / 3.59
stress #2     v0.3  PASS   119.97               23155   0       0     2.93 / 4.06
installation  v0.3  PASS    60.00/30.00/30.00   540/180/870  0  0     0.43 / 0.64
installation  v0.3  PASS    (second run, identical per-controller figures)
```

Dry-run SendFrame microbenchmarks, 10 interleaved runs each, median (min):

```text
                     main              v0.3
BackPanel8186        385 ns (326)      469 ns (358)
RGB32768            2953 ns (2565)    3850 ns (2581)
GRB32768           48189 ns (41057)  45037 ns (40467)
```

Reading: v0.3 adds a fixed ~80 ns per controller frame (mutex + atomic counters), 0 allocs.
At 120 FPS that is 0.001 % of the 8.3 ms frame budget; real sends are dominated by the
UDP syscalls (2–3 ms for 193 universes). No correctness regressions in any profile.
The stress profile warns about ~49 % adapter backpressure skips on both branches — the
120 FPS / 32768 px WebSocket input side, not the Art-Net output; unchanged by v0.3.
