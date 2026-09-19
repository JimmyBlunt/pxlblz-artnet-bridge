# PXLBLZ Art-Net Bridge — Project Plan

## Status

**2026-09-20:** standalone router and binary WebSocket live-input path are hardware verified.  
**Active milestone:** connect the PXLBLZ IDE render loop directly to `ws://127.0.0.1:9980/pixels`.

## Core architecture

```text
Pixelblaze pattern + shared 3D map
             ↓
         PXLBLZ IDE
             ↓ canonical RGB frame
  ws://127.0.0.1:9980/pixels
             ↓
       LatestFrame slot
             ↓
     pxlblz-router.exe
             ↓ Art-Net ArtDmx / UDP unicast
        LED controllers
             ↓
        physical LEDs
```

The key design split is:

- **PXLBLZ decides what each logical pixel looks like.**
- **The router decides where each logical pixel physically goes.**

The 3D JSON map therefore stays in PXLBLZ. Art-Net routing is a separate route table.

## Verified BACK_PANEL_249 topology

Target: `10.0.0.253:6454`

There are **7 electrical outputs but 6 physical panels**. P6 and P7 are two electrical lanes of the same sixth panel.

| Port | Logical pixels | LEDs | Art-Net universes |
|---|---:|---:|---|
| P1 | 1440–1642 | 203 | U120–U121 |
| P2 | 3744–4481 | 738 | U122–U126 |
| P3 | 4482–5361 | 880 | U127–U132 |
| P4 | 5362–6171 | 810 | U133–U137 |
| P5 | 6172–6523 | 352 | U139–U141 |
| P6 | 6524–7133 | 610 | U142–U145 |
| P7 | 7134–7645 | 512 | U146–U149 |

Expected controller-wide frame set:

```text
U120-U137 and U139-U149
29 universes total
```

U138 and U150-U152 are not expected.

## Receiver requirements discovered during bring-up

The Teensy receiver assembles frames **controller-wide**, not per port.

Important wire rules:

- all expected universes must arrive for one frame candidate;
- one common non-zero Art-Net sequence is used for the entire logical frame;
- ArtDmx payload length must be even;
- short final universes are accepted if they contain all required RGB bytes;
- partial-frame timeout is >100 ms from the first accepted packet;
- ArtSync is not used by this receiver.

Critical compatibility fix:

```text
P1 = 203 LEDs = 609 RGB bytes
U120 = 510 bytes
U121 = 99 useful bytes
wire payload must be 100 bytes
```

Router v0.1.1+ pads odd final payloads with one zero byte.

Current OUT6 is **610 LEDs**, so U145 requires **300 bytes**.

## Hardware test history

### H0 — local loopback

```text
8000 pixels
48 universes
60 FPS
2880 ArtDmx packets/s
0 invalid
0 send errors
```

PASS.

### H1 — one real universe

U120 reached the real Teensy at ~30 packets/s. No DMA because the controller-wide frame was intentionally incomplete.

PASS for network reachability.

### H2 — complete P1 only

U120-U121 reached the controller, but no DMA because all active controller universes were required.

This confirmed controller-wide frame assembly.

### H3 — all 29 universes, router v0.1.0

~870–890 packets/s arrived but DMA stayed at zero.

Receiver-source inspection found the odd ArtDmx payload issue.

### H4 — router v0.1.1

Even-length final payload padding enabled.

Observed:

```text
Art-Net ≈861 pkt/s
DMA ≈30/s
incomplete counter stable
moving physical LED visible
```

First full end-to-end success.

### H5 — seven electrical-port identification

All 7 electrical outputs were correctly addressed and visible across all 6 physical panels.

PASS.

### H6 — external binary WebSocket input

```text
external sender 60 FPS
↓ WebSocket
LatestFrame
↓ router 30 FPS
29 universes/frame
870 pkt/s nominal
↓
all 6 physical panels visible
```

Router counters:

```text
RX 60 FPS
replaced ~30 frames/s
invalid 0
TX 30 FPS
870 pkt/s
0 send errors
```

This proves LatestFrame semantics and fully separates input/render FPS from controller TX FPS.

## Hard real-time rule

The pixel path is lossy/live:

> **Drop superseded frames; never accumulate latency.**

If PXLBLZ renders at 60 FPS and a controller outputs at 30 FPS, the router samples the newest complete frame at 30 Hz.

## Performance target

The PC software path should comfortably support at least ~8k pixels at 60 FPS.

Physical LED output rate is a separate constraint and depends mainly on the longest LED lane and controller output implementation.

## Deferred tuning backlog

These are important but intentionally not blockers for PXLBLZ integration:

1. per-controller Art-Net output FPS;
2. DMA-completion vs complete-frame vs physical-show telemetry;
3. ObjectFLED wire-guard/latch timing;
4. splitting long Teensy LED lanes further;
5. brightness ownership;
6. color-order ownership;
7. per-controller scheduling from one latest logical frame;
8. packet burst pacing if future hardware needs it;
9. blackout / hold-last-frame policy when PXLBLZ stops;
10. sequence/restart/network-loss recovery tests;
11. ArtPoll/node discovery;
12. DDP and E1.31/sACN output;
13. physical-panel abstraction separate from electrical-port abstraction;
14. route reversal/direction metadata;
15. PXLBLZ Float→UInt8 conversion profiling;
16. full render→WebSocket→router→controller timing telemetry.

## Next gate: PXLBLZ direct output

The next change must preserve the already verified router.

Preferred PXLBLZ integration:

```text
createRenderLoop()
       ↓ packed rendered Float64 RGB
normal Preview paint
       +
ExternalPixelOutput
       ↓ clamp / Float→UInt8 once
browser WebSocket binary
       ↓
router
```

Constraints:

- no Canvas/WebGL readback;
- no second pattern render;
- no duplicate mapping engine;
- existing preview must remain unchanged;
- integration initially opt-in and easy to disable;
- full-resolution preview only should own the external output.

