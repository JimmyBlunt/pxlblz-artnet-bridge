# PXLBLZ Router - Test Results (v0.2.x main line + v0.3 line, merged 2026-10)

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

## TCP dump wire-level verification

A production-route packet capture now verifies what actually leaves the router's
UDP socket. The test assigns the real controller IPs to Linux loopback aliases,
runs the exact production `routes.installation-known.json`, captures
`udp port 6454` with `tcpdump`, and validates the resulting PCAP independently.

Captured result:

```text
6512 ArtDmx packets captured
44 packets expected per complete logical frame
148 complete sequence groups
destination 10.0.0.244 present
destination 10.0.0.253 present
destination 10.0.0.251 present
all ArtDmx payload lengths even
BACK_PANEL U138 absent
unexpected packets 0
status PASS
TCPDUMP_ARTNET_PASS
```

Receiver-sensitive BACK_PANEL tail lengths were confirmed directly from PCAP:

```text
U121 100 bytes
U126 174 bytes
U132  90 bytes
U137 390 bytes
U141  36 bytes
U145 300 bytes
U149   6 bytes
```

The capture also uses a deterministic logical RGB test value
`(100, 200, 50)` and verifies physical color order plus route brightness from
the actual captured ArtDmx payload bytes:

```text
10.0.0.244 U0    GRB × 0.70 -> captured 140, 70, 35
10.0.0.253 U120  RGB × 0.30 -> captured  30, 60, 15
10.0.0.251 U156  BGR × 0.70 -> captured  35,140, 70
```

PASS.

The CI workflow is `.github/workflows/tcpdump-wire.yml`. It uploads the raw
PCAP, decoded tcpdump text, verifier JSON and router log as an artifact.

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

## P2 — v0.3 60 FPS headroom (10 min) and installation soak (60 min), 2026-09-26

Same machine/toolchain as P1, commit ec37979, one virtual receiver per controller.

```text
headroom: --controller-fps 60, 600 s, expected 2640 pkt/s                 PASS, no warnings
  LOOP_A  59.995 fps   540 pkt/s  35997 frames  incompl 0 gaps 0  interval p99 18.43 ms  max 60.87 ms
  LOOP_B  59.997 fps   360 pkt/s  35999 frames  incompl 0 gaps 0  interval p99 19.00 ms  max 48.87 ms
  LOOP_C  59.998 fps  1740 pkt/s  36000 frames  incompl 0 gaps 0  interval p99 18.86 ms  max 86.26 ms
  router send avg p50/p99 0.40/0.95 ms, heap 0.6 -> 1.9 MB, goroutines <= 11

soak: installation 60/30/30 FPS, 3600 s                                   PASS, no warnings
  LOOP_A  60.000 fps   540 pkt/s  216001 frames  incompl 0 gaps 0  interval p99 18.72 ms  max 39.31 ms
  LOOP_B  30.000 fps   180 pkt/s  108001 frames  incompl 0 gaps 0  interval p99 35.42 ms  max 53.09 ms
  LOOP_C  30.000 fps   870 pkt/s  108001 frames  incompl 0 gaps 0  interval p99 35.34 ms  max 49.28 ms
  router send avg p50/p99 0.33/0.78 ms, max 8.98 ms; heap 0.6 -> 0.8 MB (max 2.4), 23 GCs, goroutines <= 11
  adapter 216120 produced / 215730 sent (0.18 % backpressure skips), wrong size 0
```

Reading: 432 003 controller frames in one hour without a single incomplete frame,
sequence gap, duplicate or late packet; no heap growth, no goroutine leak, no FPS drift.
Isolated frame-interval maxima (40–86 ms, i.e. a few frames late, never lost) are OS
scheduling hiccups on a non-realtime laptop; p99 stays within 12 % of the period.
Software side of v0.3 is done; remaining gate is hardware V1–V4.

## H-ESP1 — first real-hardware throughput ramp, ESP32 test controller, 2026-10-01

Controller `esp32-wroom-flex-8ws-2apa` at 172.20.10.2, **Wi-Fi via phone hotspot**
(PC also on the hotspot, ping 9–34 ms). Running config: out0 WS2812B 203 px U120–U121,
out6 APA102 127 px U149 (4 MHz SPI), controller `targetFps` 30, output 8.3 ms/frame.
Router v0.3 (ec37979), `config/routes.esp-test-172.json`, 3 universes/frame.
Counters from the controller's `GET /api/status`, diffed around each step
(`perf-test/hardware-ramp.mjs`, 20 s per step, pattern rainbow).

T2 port-id 30 fps, 10 s: sent 299 frames / 897 packets → received 299 complete / 897 packets.

```text
fps  sent  rx complete        rx packets   incompl  dropped  seqErr  LED out frames
 30   600   600 (100.00 %)    1800/1800         0        0       0   590
 45   900   898  (99.78 %)    2696/2700         1        0       0   594
 60  1199  1193  (99.50 %)    3580/3597         1        0       0   597
 90  1799  1752  (97.39 %)    5291/5397        20        0       0   597
120  2399  2276  (94.87 %)    6896/7197        45        0       0   597
router: exact FPS every step, send avg 0.20–0.30 ms, 0 send errors
```

Reading:
- Losses are packets that never reached the controller (Wi-Fi/hotspot), not router or
  firmware drops (controller droppedPackets/sequenceErrors stay 0).
- LED output stays at ~30 FPS in every step: the firmware's `targetFps` 30 caps physical
  output; faster complete frames are accepted and superseded.
- Wi-Fi hotspot is not representative of the wired installation controllers.

### H-ESP2 — same controller with firmware `targetFps` 60 (changed by user in the controller UI, saved)

Router config `routes.esp-test-172.json` now `fps_target` 60.

```text
fps  sent   rx complete        rx packets     incompl  timeouts  LED out frames (fps)
 30    600   596  (99.33 %)     1790/1800          1         0     587  (29.4)
 45    900   898  (99.78 %)     2696/2700          1         0     862  (43.1)
 60   1200  1192  (99.33 %)     3584/3600          4         1    1130  (56.5)
 75   1500  1484  (98.93 %)     4457/4500          3         0    1170  (58.5, firmware cap)
 90   1800  1757  (97.61 %)     5292/5400         14         0    1163  (58.2)

60 fps for 300 s:
 60  17998 17793  (98.86 %)    53540/53994        95         9   16807  (56.0)
router: exact 60.0 fps, send avg 0.26 ms, 0 send errors
```

Reading: over the phone hotspot ~0.8 % of packets are lost in transit; complete frames
that arrive bunched inside one output period supersede each other, so LEDs show ~56 fps
for 60 sent. Router-side output is exact; the remaining gap is the Wi-Fi link.
Next: repeat on a proper AP / wired link for a representative number.

### H-ESP3 — PXLBLZ → router → ESP end to end, real test rig, 2026-10-01

Rig: ONE APA102 chain on ESP out6 = 8x8 matrix + DotStar FeatherWing 12x6 = 136 px (U149).
ESP set to out6 = 136 px, out0 disabled (applied + saved via its /api/config; backup of
the previous config in the handoff folder `esp-config-backup/`). Router config
`routes.esp-test-172-8x8-12x6.json`, PXLBLZ IDE (local, d685125b + output adapter,
desktop starter `windows-launcher-artnet`) with map `maps/esp-test-8x8-12x6.js`.

```text
PXLBLZ -> router: 1 WebSocket client, ~60 fps, 0 invalid frames (variable_size)
router -> ESP, 5 s: 302 packets -> 302 complete frames, 0 incomplete; LED output ~56 fps
```

Root cause of the "last LEDs dark" symptom: out6 was configured for 127 px while the
chain has 136 (the last 9 FeatherWing LEDs were never driven).

### H-ESP4 — ported pattern on the test rig, 2026-10-01

Map (flipY on both boards) and `patterns/snowflake-icesparkle-carpet-v06-esp-test.js`
imported into the local PXLBLZ D1 (backup of `.wrangler/state` taken first). User
confirmation on the LEDs: all 136 LEDs lit, blue variant marker bottom-left on the 8x8.
Open: 8x8 serpentine of the upper rows (assumed).
Update: router chase test (`--pattern chase --fps 4`) on the rig — 8x8 rows run left->right
without serpentine (user observation "B"). Map changed to `MATRIX.serpentine: false`; the
local PXLBLZ map row was re-baked accordingly. Rig wiring is now fully verified.
User confirmation after reload: the diagonal wave runs cleanly across both boards. Test rig done.

## W1 — router configuration page + launcher step 3, 2026-10-01

Router web page `http://127.0.0.1:9988/` (moved from 9981: TouchDesigner also binds
0.0.0.0:9981 and Windows then routes requests to either program; startup self-check now warns).

```text
go vet + all unit tests                                  PASS (engine, controllerapi, web, ...)
live on ESP rig 10.0.0.248 via the page:
  discovery 10.0.0.0/24                                  found, recognised as configured
  FPS 60 -> 50 -> 60 applied live (no restart)           router 50.3 / 60.3 fps, ESP 0 incomplete
  backup on save                                         config/backups/<name>.<timestamp>.json
  test pattern "white" 3 s                               PXLBLZ input paused, resumed automatically
perf lab after the engine refactor: smoke PASS, installation PASS (0 incomplete, 0 gaps)
launcher installer, temp workspace:
  first install                                          11 configs added, choice stored
  re-install, workspace config edited, no -RouterConfig  config kept, choice kept
  -OverwriteConfigs                                      replaced, old file in config/backups
launcher start check                                     all OK incl. "Einstellungsseite"
PXLBLZ output tab                                        title "PXLBLZ-IDE~ArtNet", badge shows router state
```

## H-INST1 — installation controllers at 30 / 60 FPS, 2026-10-01

Through the running router (config API + test pattern, no second sender),
`perf-test/installation-fps-test.mjs`, rainbow, 20-30 s per step. Details and options:
`docs/FPS60_AUSWERTUNG.md`.

```text
controller                         30 fps      60 fps: complete rx      LEDs at 60      LED output/frame
ESP rig 10.0.0.248 (Wi-Fi)          100 %      60.0 fps (0-0.1 % loss)   59.3 fps        2.2 ms
Teensy 10.0.0.253 (wired, 32 U)     100 %      60.0 fps (0 % loss)       29.9 fps        29.3 ms (880-LED lane)
APA102 10.0.0.251 (Wi-Fi, 6 U)      100 %      57.5-58.1 fps (3-4 %)     29.5 (target 30) / 56.1-56.8 (target 60, not saved)  12.4 ms
router: exact 30.0 / 60.0 fps per controller, 0 send errors
```

.251 packet analysis at 60 fps: 7482 sent / 7186 received (96 %), firmware dropped 0,
sequence errors 0 -> loss is the Wi-Fi link. Router-side mitigations tried and removed:
send twice (5.1 % loss), packet gap 0.5 ms (4.0 %), 1.5 ms (6.0 %).
Controller settings were restored after every run (.251 targetFps 30, saved=matches).

### H-INST2 — port-id on the installation, 2026-10-02

Router test pattern `port-id` (page /api/test) on .253 (8 outputs), .251 (2 outputs) and .248:
user confirmation "colors are good" - every output shows its port color (1 red, 2 green,
3 blue, 4 cyan, 5 magenta, 6 yellow, 7 white, 8 red). Universes and color order confirmed;
router sends RGB to all three (both firmwares reorder themselves).

### H-INST3 — ESP32 firmware with receive task (rxQueueDrops), 2026-10-02

.251 and .248 flashed with the firmware from `ESP_FIRMWARE_RX_FIX_PROMPT.md` (own Art-Net
receive task + queue). `installation-fps-test.mjs --fps 60,30 --seconds 30`, rainbow,
TouchDesigner output stopped (a first run with TouchDesigner also sending Art-Net gave
controller counts above the router rate - invalid). .244 not reachable (timeout).

```text
fps  controller          router  complete  loss    incompl/s  LED out  output
 60  APA102 .251          60.0     59.5     0.88 %   0.08      58.2     12.2 ms
 60  Teensy .253          60.0     60.0     0 %      0         34.0     29.3 ms
 30  APA102 .251          30.0     30.0     0 %      0         30.0     12.2 ms
 30  Teensy .253          30.0     30.0     0 %      0         30.0     29.3 ms
```

.251 before the fix (H-INST1): 3-4 % loss, ~1 incomplete frame/s, 56-57 fps at the LEDs.
After: 0.9 % loss, 0.08 incomplete/s, 58.2 fps at the LEDs; `rxQueueDrops` 0,
`rxSocketErrors` 0, `droppedPackets` 0, `sequenceErrors` 0 - the firmware queue no longer
loses anything, the remaining loss happens below the socket (Wi-Fi / lwIP).
Acceptance (>= 99.9 % packets, >= 59.5 complete frames/s): just missed (99.1 %, 59.48).

### H-INST4 — router config 60 fps for all controllers, 3 x 60 s, 2026-10-02

`routes.installation-live.json` (SchrankAll, 8683 px): `input.fps_target` and every
`controllers[].fps_target` set to 60 and saved (backup `config/backups/routes.installation-live.20261002-055801.json`);
controllers already at `targetFps` 60. `installation-fps-test.mjs --fps 60,60,60 --seconds 60`, rainbow,
no other Art-Net sender. .244 not reachable.

```text
run  controller     router  complete  loss          incompl/s  LED out  output
1-3  APA102 .251     60.0   59.4-59.5  0.83-1.00 %   0.13-0.21  58.0-58.5  12.2 ms
1-3  Teensy .253     60.0   60.0       0-0.02 %      0          34.0       29.3 ms
```

Router and wired network: loss-free at 60 fps. .251: remaining ~0.9 % lost below the firmware
queue (`rxQueueDrops` 0) - Wi-Fi / lwIP. .253: 34 fps at the LEDs, limited by the 880-LED lane.
Diagnostics differ between firmwares (Teensy ~90 status fields, ESP32 21) - alignment task:
`docs/FIRMWARE_DIAGNOSE_ANGLEICH_PROMPT.md`.
