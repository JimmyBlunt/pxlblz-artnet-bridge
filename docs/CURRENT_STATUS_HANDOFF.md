# PXLBLZ → Art-Net Bridge — Current Status & Handoff

**Handoff date:** 2026-09-26  
**Bridge repository:** `JimmyBlunt/pxlblz-artnet-bridge`  
**PXLBLZ upstream:** `jon-whiteroomsoftware/PXLBLZ-IDE`  
**Pinned PXLBLZ commit used for the integration:** `d685125b34c694f311972e258efb48d12cf05cd8`  
**Current bridge generation:** Router v0.2.0 + experimental direct PXLBLZ browser output

---

## 1. Main mission

The project goal is to use **PXLBLZ IDE as the creative/rendering engine and 3D mapping engine** for a large LED installation, while a native Windows process performs all physical Art-Net routing.

Target architecture:

```text
Pixelblaze pattern
      +
shared 1D/2D/3D PXLBLZ map
              ↓
          PXLBLZ IDE
              ↓
canonical logical RGB frame
              ↓
binary WebSocket localhost
ws://127.0.0.1:9980/pixels
              ↓
        LatestFrame slot
              ↓
      pxlblz-router.exe
              ↓
Art-Net ArtDmx / UDP unicast
              ↓
multiple physical LED controllers
              ↓
parallel LED output lanes
              ↓
physical installation
```

The governing design rule is:

> **PXLBLZ decides what every logical pixel looks like. The router decides where every logical pixel physically goes.**

This keeps creative mapping and physical routing independent.

---

## 2. What is already working

### Router core

The native Windows router is implemented in Go and currently supports:

- Windows x64 standalone executable
- JSON route configuration
- multiple target controller IPs
- arbitrary logical pixel ranges
- arbitrary Art-Net universe starts
- multiple physical output routes per controller
- standard RGB universe splitting at 510 channels / 170 RGB pixels
- short final universes
- receiver-compatible even-length ArtDmx padding
- configurable RGB channel order
- one common Art-Net sequence for every universe belonging to one logical frame
- UDP unicast
- built-in diagnostic patterns
- binary WebSocket live-frame input
- LatestFrame semantics with no accumulating video queue
- live TX/RX telemetry
- loopback Art-Net listener
- standalone external frame generator

### Real hardware

The router has driven the actual BACK_PANEL controller and LEDs.

Verified end-to-end:

```text
external RGB generator
→ WebSocket
→ pxlblz-router
→ 29 Art-Net universes
→ real Teensy receiver
→ complete controller frame
→ DMA
→ visible LEDs on all 6 physical panels
```

### Direct PXLBLZ transport

The experimental PXLBLZ integration is also working at transport level:

```text
PXLBLZ browser render loop
→ packed Float64 RGB frame
→ Float→RGB888
→ browser WebSocket
→ pxlblz-router
```

Observed during the direct PXLBLZ test:

```text
8186 logical pixels
24558 RGB bytes/frame
clients 1
RX 60.0 fps
replaced ~30 frames/s
invalid 0/s
TX 30.0 fps
870 Art-Net packets/s
0 send errors
```

This proves the browser/render-to-router transport and exact frame-size agreement.

**Still pending:** deliberate visual confirmation that a recognizable live PXLBLZ pattern is mapped correctly across the real physical panels.

---

## 3. BACK_PANEL_249 verified topology

Target:

```text
10.0.0.253:6454
```

Important terminology:

```text
7 active electrical output ports
6 physical panels
4105 configured LEDs on this controller
```

P6 and P7 are two independent electrical lanes of the **same sixth physical panel**.

| Port | Physical panel | Logical pixels | LEDs | Universes | Color |
|---|---|---:|---:|---|---|
| P1 | Panel 1 | 1440–1642 | 203 | U120–U121 | RGB |
| P2 | Panel 2 | 3744–4481 | 738 | U122–U126 | RGB |
| P3 | Panel 3 | 4482–5361 | 880 | U127–U132 | RGB |
| P4 | Panel 4 | 5362–6171 | 810 | U133–U137 | RGB |
| P5 | Panel 5 | 6172–6523 | 352 | U139–U141 | RGB |
| P6 | Panel 6A | 6524–7133 | 610 | U142–U145 | RGB |
| P7 | Panel 6B | 7134–7645 | 512 | U146–U149 | RGB |

Controller-wide expected universe set:

```text
U120-U137
U139-U149
= 29 universes
```

Not expected:

```text
U138
U150-U152
```

OUT8 is currently disabled.

---

## 4. Receiver findings that are hard requirements

The actual Teensy receiver source was inspected during bring-up.

Important behavior:

### Controller-wide completeness

Frames are assembled **across the whole controller**, not per individual output.

A candidate frame is complete only when all expected universes for all enabled routes have been accepted.

### ArtDmx payload rules

The receiver requires:

```text
payload length 2..512 bytes
payload length must be EVEN
UDP datagram length = 18 + declared ArtDmx payload length
payload must contain all RGB bytes required by that route
```

Short tail packets are valid.

Current final-universe requirements:

| Output | Last universe | Minimum wire payload |
|---|---|---:|
| P1 | U121 | 100 bytes, 99 RGB bytes used |
| P2 | U126 | 174 |
| P3 | U132 | 90 |
| P4 | U137 | 390 |
| P5 | U141 | 36 |
| P6 | U145 | 300 |
| P7 | U149 | 6 |

### Critical compatibility bug already fixed

For P1:

```text
203 LEDs × 3 = 609 RGB bytes
U120 = 510 bytes
tail = 99 RGB bytes
```

Router v0.1.0 declared 99 bytes and the receiver rejected it because the ArtDmx length was odd.

Router v0.1.1+ sends:

```text
99 real bytes + 1 zero padding byte
declared length = 100
```

This fix is hardware verified.

### Sequence

One non-zero sequence number is used for **every universe in one logical frame**.

Only after all packets have been sent does the router increment the sequence.

Do not increment sequence per universe.

### Timeout

Partial candidate timeout is roughly:

```text
>100 ms from the FIRST accepted packet
```

Sequence state resets after roughly 1000 ms without accepted traffic.

ArtSync is not required and the current receiver does not use it.

---

## 5. Test history

### H0 — 8000-pixel local loopback

```text
8000 pixels
U0-U47
60 FPS
600 frames / 10 s
28,800 packets
59.99 average FPS
~2879.55 packets/s
0 send errors
0 invalid listener packets
```

PASS.

### H1 — one universe to real controller

```text
U120 only
~30 packets/s
DMA 0
incomplete increased
```

Confirmed real LAN/IP/UDP/Art-Net reachability.

### H2 — complete P1 route only

```text
U120-U121
~60 packets/s
DMA 0
incomplete increased
```

This exposed controller-wide completeness.

### H3 — all expected universes with router v0.1.0

```text
~870-890 packets/s
DMA 0
incomplete increased
```

Packet count was correct but frame acceptance still failed.

Receiver source inspection exposed the odd-tail-payload rule.

### H4 — v0.1.1 compatibility fix

After even-length tail padding:

```text
~861 packets/s
DMA ~30/s
incomplete stopped increasing
moving physical LED visible
```

First full end-to-end hardware PASS.

### H5 — seven electrical output identification test

Port-ID diagnostic pattern:

```text
P1 red
P2 green
P3 blue
P4 cyan
P5 magenta
P6 yellow
P7 white
```

All 7 electrical routes received data. All 6 physical panels lit correctly.

PASS.

### H6 — external WebSocket sender → real hardware

External sender at 60 FPS, router at 30 FPS:

```text
RX 60 FPS
replaced ~30/s
invalid 0
TX 30 FPS
870 packets/s
0 send errors
all 6 physical panels visibly active
```

This proved the live-input architecture and FPS decoupling.

### H7 — direct PXLBLZ IDE browser output

Using the real PXLBLZ IDE render loop:

```text
clients 1
RX 60 FPS
replaced ~30/s
invalid 0
TX 30 FPS
870 packets/s
0 send errors
```

Frame size exactly matched 8186 pixels / 24558 RGB bytes.

PASS for direct PXLBLZ transport.

Still pending: deliberate visual content/mapping validation using a recognizable PXLBLZ pattern.

---

## 6. PXLBLZ integration design

Integration lives under:

```text
pxlblz-integration/
```

Main adapter:

```text
src/externalPixelOutput.ts
```

Activation is currently deliberately experimental:

```text
?pxout=1
```

Default endpoint:

```text
ws://127.0.0.1:9980/pixels
```

### Important implementation choices

- Uses PXLBLZ's existing packed render path.
- No Canvas/WebGL readback.
- No second pattern render.
- No duplicate mapping engine.
- One `Float64Array(pixelCount * 3)` is used by the render loop.
- Preview still receives the rendered frame.
- The same logical RGB frame is converted to a reusable `Uint8Array`.
- One WebSocket binary message equals one complete RGB888 frame.
- The full-resolution Preview only owns the hardware output.
- Secondary/capped previews do not send hardware frames.
- Browser WebSocket backpressure is deliberately lossy.
- If a full frame is already buffered, a newer render may be skipped.
- The native router adds a second LatestFrame buffer.

Hard real-time principle:

> **Drop superseded frames. Never accumulate animation latency.**

### Brightness

The current prototype sends canonical/raw pattern RGB.

PXLBLZ preview brightness/dimmed presentation is **not yet defined as hardware brightness ownership**.

This is deliberately deferred.

---

## 7. Current software repositories

### Bridge project

```text
https://github.com/JimmyBlunt/pxlblz-artnet-bridge.git
```

Contains:

- complete router source
- Windows x64 EXEs
- Art-Net listener
- frame sender
- configs
- tests
- receiver notes
- project plan
- PXLBLZ integration adapter
- GitHub Actions build
- Windows-specific local Studio workaround

### PXLBLZ upstream

```text
https://github.com/jon-whiteroomsoftware/PXLBLZ-IDE.git
```

Integration was built and verified against:

```text
d685125b34c694f311972e258efb48d12cf05cd8
Keep shared Show fixture Node-safe (#1029)
```

Local development branch used:

```text
pxlblz-artnet-output
```

---

## 8. Current blocking point

The next functional goal is to enter authenticated **PXLBLZ Studio** locally and perform the real visual pattern test.

Real GitHub/Google login does not work in a default local checkout because local OAuth credentials are intentionally absent.

PXLBLZ provides synthetic local identities for development.

However, its managed command:

```text
npm run dev:main
```

currently assumes Unix process tools such as:

```text
ps -axo ...
lsof ...
```

On native Windows the coordinator fails with errors such as:

```text
ps: illegal option -- x
Timed out waiting for http://localhost:5174/api/me
```

This is a PXLBLZ local-runtime coordinator portability issue, not an Art-Net problem.

### Current planned Windows workaround

Use the normal Vite + Cloudflare Worker path directly:

1. Keep a clean local `main` worktree so PXLBLZ's session helper can identify it.
2. Keep `.dev.vars` in the main worktree with a local `SESSION_SECRET`.
3. Copy/link that file into the Art-Net worktree for direct Vite use.
4. Run local D1 migrations manually.
5. Seed `github:local-dev` manually in local D1.
6. Run ordinary `npm run dev`.
7. Run `npm run dev:session -- --developer`.
8. Set the returned `pxlblz_session` cookie in the browser.
9. Open `/PXLBLZ-IDE/studio?pxout=1`.

See `docs/WINDOWS_PXLBLZ_LOCAL_STUDIO.md`.

### Important dependency lesson

Do **not** run:

```text
npm audit fix --force
```

against the pinned upstream checkout.

It changed PXLBLZ's tested dependency graph, including Miniflare, and partially broke the local install. The project was recovered by:

```text
git restore package.json package-lock.json
remove node_modules
npm cache verify
npm ci
```

---

## 9. Known additional controllers still to integrate

The current router hardware verification concentrated on BACK_PANEL_249.

Known installation routing from the earlier TouchDesigner configuration:

### WS2812_NODE

```text
IP 10.0.0.244
color order GRB
historically targeted ~60 FPS
```

Known routes:

```text
P1 logical 0..516      517 LEDs   U0..U3
P2 logical 517..785    269 LEDs   U6..U7
P3 logical 786..1183   398 LEDs   U12..U14
```

Universe gaps are intentional/reserved in the existing installation layout.

### PANEL8_251

```text
IP 10.0.0.251
color order BGR
historically targeted ~30 FPS
```

Known routes:

```text
P1 logical 2720..2975  256 LEDs   U156..U157
P2 logical 7646..8185  540 LEDs   U158..U161
```

### APA102 controller

A separate APA102/ESP controller exists in the installation, but its final routing still needs to be gathered/confirmed before it is added to the router config.

The current maximum known logical index is 8185, therefore the current full logical frame is:

```text
8186 pixels
24558 RGB bytes
```

---

## 10. Deferred fine-tuning backlog

These items are important and intentionally preserved for later:

1. Per-controller Art-Net TX FPS.
2. Separate PXLBLZ render FPS from controller physical FPS.
3. Complete-frame vs DMA-start vs DMA-complete telemetry.
4. ObjectFLED/FastLED wire-time guard and latch timing.
5. Split long physical LED lanes further, ideally toward ~400–600 LEDs/lane where useful.
6. Hardware brightness ownership.
7. Color-order ownership.
8. Per-controller scheduler sampling one shared LatestFrame.
9. UDP packet-burst pacing only if measurements show a need.
10. Blackout / hold-last-frame / fade behavior when PXLBLZ stops.
11. Restart and sequence-wrap behavior.
12. Network interruption recovery.
13. ArtPoll / node discovery.
14. DDP output.
15. E1.31/sACN output.
16. Panel abstraction separate from electrical output-port abstraction.
17. Route-level reverse/forward direction metadata.
18. Float→UInt8 conversion profiling at 8k+ pixels.
19. Full end-to-end render→WS→router→controller latency telemetry.
20. PXLBLZ UI for external output instead of the temporary `?pxout=1` flag.
21. Persisted output configuration/profile.
22. Controller health/status panel.

---

## 11. Next milestones in order

### G8 — Local Studio access on Windows

Complete the manual synthetic-session workaround and reach Studio with:

```text
?pxout=1
```

### G9 — Visual PXLBLZ hardware verification

Use an unmistakable test pattern and verify:

- all 6 BACK_PANEL physical panels
- correct logical ordering
- correct panel direction
- correct RGB color
- correct P6/P7 continuity across physical Panel 6
- animation movement matches PXLBLZ preview

### G10 — Add remaining controllers

Add and hardware-test:

- WS2812_NODE / 10.0.0.244
- PANEL8_251 / 10.0.0.251
- APA102 controller once confirmed

### G11 — Proper PXLBLZ output UI

Replace the URL query flag with a proper output/control surface.

### G12 — Multi-controller scheduling and tuning

Apply controller-specific FPS, brightness, telemetry and later output protocols.

---

## 12. Definition of success

The full project is considered functionally complete when:

```text
one PXLBLZ pattern + one shared logical map
→ one logical RGB frame
→ independently scheduled controller outputs
→ all physical LED hardware
```

runs continuously with:

- stable animation
- no accumulating latency
- correct topology
- recoverable controller/network interruptions
- explicit controller health
- reproducible configuration
- no TouchDesigner required in the main path
