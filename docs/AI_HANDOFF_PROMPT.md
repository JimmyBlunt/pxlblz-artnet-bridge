# AI / Developer Handoff Prompt

Copy the block below into a new ChatGPT/Codex/developer session when continuing this project.

---

## READY-TO-PASTE PROMPT

You are continuing an existing project called **PXLBLZ → Art-Net Bridge**.

Do not redesign it from scratch. Read the repository and handoff documents first, preserve the verified behavior, and make incremental testable changes.

### Repositories

Bridge repository:

```text
https://github.com/JimmyBlunt/pxlblz-artnet-bridge.git
```

Upstream PXLBLZ repository:

```text
https://github.com/jon-whiteroomsoftware/PXLBLZ-IDE.git
```

PXLBLZ integration was built and tested against exact upstream commit:

```text
d685125b34c694f311972e258efb48d12cf05cd8
```

The modified PXLBLZ branch is called:

```text
pxlblz-artnet-output
```

### Main mission

PXLBLZ must remain the creative/rendering and logical mapping engine.

The native router must own physical routing.

Architecture:

```text
PXLBLZ Pattern + 3D Map
        ↓
logical RGB frame
        ↓
binary WebSocket localhost
ws://127.0.0.1:9980/pixels
        ↓
LatestFrame
        ↓
pxlblz-router.exe
        ↓
Art-Net ArtDmx UDP unicast
        ↓
LED controllers
        ↓
physical LEDs
```

The core rule is:

**PXLBLZ decides what every logical pixel looks like. The router decides where each logical pixel physically goes.**

Do not duplicate the PXLBLZ map in the router.

### Current frame format

Current full logical pixel count:

```text
8186 pixels
24558 RGB bytes/frame
```

One WebSocket binary message is exactly one complete RGB888 frame.

No JSON, Base64, or per-frame header is used in the hot path.

The path is deliberately real-time/lossy:

**Drop superseded frames; never queue animation latency.**

### Verified router behavior

The router is Go, Windows x64 and currently versioned v0.2.0.

It has already passed:

- 8000-pixel / 48-universe / 60-FPS loopback
- real BACK_PANEL controller network delivery
- controller-wide 29-universe completeness
- receiver-specific even ArtDmx tail padding
- seven electrical output route test
- six physical panel LED test
- external WebSocket input at 60 FPS feeding Art-Net at 30 FPS
- direct PXLBLZ browser WebSocket input with 0 invalid frames

Do not break these tests.

### Critical Art-Net receiver rules

BACK_PANEL_249 is at:

```text
10.0.0.253:6454
```

It has 7 electrical output lanes but 6 physical panels.

P6 and P7 are two lanes of the same Panel 6.

Expected universes are exactly:

```text
U120-U137
U139-U149
```

29 universes total.

U138 is intentionally absent.

All expected universes for one logical frame must carry the same non-zero Art-Net sequence.

Only increment sequence after the whole logical frame has been transmitted.

The receiver requires ArtDmx payload length to be even.

Example P1:

```text
203 LEDs = 609 RGB bytes
U120 = 510
remaining = 99
wire U121 = 100 bytes
last byte = zero padding
```

Current final tail lengths:

```text
P1 U121 100
P2 U126 174
P3 U132 90
P4 U137 390
P5 U141 36
P6 U145 300
P7 U149 6
```

Frame completion is controller-wide, not port-by-port.

### BACK_PANEL routing

```text
P1  pixels 1440..1642  203 LEDs  U120..U121  RGB
P2  pixels 3744..4481  738 LEDs  U122..U126  RGB
P3  pixels 4482..5361  880 LEDs  U127..U132  RGB
P4  pixels 5362..6171  810 LEDs  U133..U137  RGB
P5  pixels 6172..6523  352 LEDs  U139..U141  RGB
P6  pixels 6524..7133  610 LEDs  U142..U145  RGB
P7  pixels 7134..7645  512 LEDs  U146..U149  RGB
```

### Direct PXLBLZ adapter

The adapter is under:

```text
pxlblz-integration/
```

It hooks PXLBLZ's existing `paintPacked` render path.

Requirements:

- do not render the pattern twice
- do not read the WebGL canvas back
- do not duplicate mapping
- normal Preview must continue to work
- only the full-resolution Preview sends output
- output is currently enabled with `?pxout=1`
- default target is `ws://127.0.0.1:9980/pixels`
- browser backpressure must drop frames instead of building a FIFO
- native router LatestFrame is the second anti-latency layer

Direct PXLBLZ transport was observed at:

```text
clients 1
RX 60 FPS
replaced ~30/s
invalid 0
TX 30 FPS
870 pkt/s
errors 0
```

### Current blocker / exact continuation point

The next objective is **local PXLBLZ Studio access on native Windows**, then a visual hardware pattern test.

Real GitHub/Google OAuth is not configured locally.

PXLBLZ has a synthetic local developer session.

Its managed `npm run dev:main` coordinator currently assumes Unix commands such as `ps -axo` and `lsof`, so on Windows it fails with:

```text
ps: illegal option -- x
Timed out waiting for http://localhost:5174/api/me
```

Do not mistake this for an Art-Net problem.

The intended Windows workaround is documented in:

```text
docs/WINDOWS_PXLBLZ_LOCAL_STUDIO.md
```

The latest local state before handoff was:

1. Clean PXLBLZ `main` worktree exists.
2. Art-Net worktree exists on `pxlblz-artnet-output`.
3. PXLBLZ dependencies were restored to the pinned lockfile with `npm ci`.
4. A previous `npm audit fix --force` changed dependencies and was reverted.
5. `dev:main` is still unusable natively because of Unix process-tool assumptions.
6. Next step is the direct Vite/Cloudflare + manual D1 + synthetic-session workaround.

Do NOT run `npm audit fix --force` on the pinned checkout.

### Next sequence

1. Complete local Studio login workaround on Windows.
2. Open Studio with `?pxout=1`.
3. Run a deliberately simple, recognizable PXLBLZ test pattern.
4. Verify real panel order, colors, direction and Panel-6 P6/P7 continuity.
5. Record exact result in `router/TEST_RESULTS.md` and `docs/PROJECT_PLAN.md`.
6. Add remaining installation controllers:
   - WS2812_NODE at 10.0.0.244
   - PANEL8_251 at 10.0.0.251
   - APA102 controller after its route is confirmed
7. Replace `?pxout=1` with a real PXLBLZ output UI.
8. Add per-controller FPS scheduling and deferred tuning only after correctness is proven.

### Known remaining controller mapping

WS2812_NODE:

```text
10.0.0.244
GRB
P1 0..516      517 LEDs  U0..U3
P2 517..785    269 LEDs  U6..U7
P3 786..1183   398 LEDs  U12..U14
```

PANEL8_251:

```text
10.0.0.251
BGR
P1 2720..2975  256 LEDs  U156..U157
P2 7646..8185  540 LEDs  U158..U161
```

APA102 mapping still needs confirmation.

### Fine tuning to preserve, not solve prematurely

Keep these in the backlog:

- per-controller output FPS
- DMA/wire-time/latch tuning
- long lane splitting
- brightness ownership
- color-order ownership
- controller scheduling
- packet burst pacing
- blackout/hold behavior
- sequence/restart recovery
- ArtPoll
- DDP
- E1.31
- reverse routes
- panel-vs-port abstraction
- performance/latency telemetry

### Working style

Make small changes.
Keep verified router behavior untouched unless necessary.
Add tests for protocol changes.
Update project documentation whenever a test changes a hard fact.
Prefer measured hardware results over assumptions.

Start by reading:

```text
README.md
docs/CURRENT_STATUS_HANDOFF.md
docs/PROJECT_PLAN.md
docs/WINDOWS_PXLBLZ_LOCAL_STUDIO.md
router/TEST_RESULTS.md
controller-reference/BACK_PANEL_249_RECEIVER.md
pxlblz-integration/README.md
```

Then continue from the current Windows Studio-login workaround.
