# Virtual Art-Net Controller / Teensy receiver emulator

## Purpose

`pxlblz-virtual-controller` is a hardware-free receiver emulator for the
PXLBLZ Art-Net project.

It is intentionally **not** another sender-side packet listener. It models the
controller-side protocol state machine and physical-output scheduling so sender
changes can be tested at the same logical boundary as the real Teensy.

Primary profile:

```text
BACK_PANEL_249
7 electrical outputs
4105 LEDs
29 expected universes
U120-U137 + U139-U149
30 FPS physical output
```

## Fidelity basis

The BACK_PANEL model follows the active receiver behavior documented from the
Teensy source snapshot corresponding to source commit `d5d828f` and firmware
identifier `orbital-port-tests-20260913`.

Modeled receiver rules include:

- packet counter advances before protocol acceptance;
- only valid ArtDmx is accepted;
- ArtSync is rejected;
- ArtDmx payload length must be 2..512, even, and exactly match UDP payload;
- expected tail universes may be short but must contain every configured RGB byte;
- valid universes outside the active profile are ignored;
- one controller-wide frame candidate covers all active ports;
- one common non-zero sequence is shared by all expected universes;
- sequence 255 -> 1 is valid;
- forward distance 1..127 is newer, >127 is stale;
- duplicate universe in the same non-zero sequence is ignored/counted;
- sequence zero collects without sequence sorting;
- a duplicate universe during a sequence-zero candidate abandons that partial
  candidate and restarts with the duplicate packet;
- switching between sequence-zero and non-zero modes abandons a partial candidate;
- partial-frame timeout is strictly greater than 100 ms from the first accepted
  packet and is not extended by later packets;
- sequence state resets after >1000 ms without an accepted packet;
- frame completion is controller-wide;
- complete frames are published to the simulated output scheduler;
- run policy outputs at the configured controller FPS;
- simulated ObjectFLED guard is longest-port LED count * 30 us + 300 us latch;
- after >1000 ms without a complete frame, a previously running controller
  submits a blackout.

For the current BACK_PANEL profile the longest 880-pixel lane therefore produces:

```text
880 * 30 us + 300 us = 26700 us
```

### What is not emulated byte-for-byte

The emulator does not pretend to reproduce:

- Teensy Ethernet/lwIP driver interrupt timing;
- actual ObjectFLED DMA engine implementation;
- cache/bus contention on the MCU;
- physical WS2812 electrical timing/jitter;
- controller web-server implementation;
- firmware behavior that was not present in the available source analysis.

It is a protocol/run-policy twin, not an instruction-level MCU emulator.

## Windows binary

GitHub Actions builds:

```text
bin/windows-x64/pxlblz-virtual-controller.exe
```

## Start the BACK_PANEL emulator

From `router/`:

```bat
run-virtual-backpanel-controller.bat
```

Or directly:

```bat
..\bin\windows-x64\pxlblz-virtual-controller.exe ^
  --config config\routes.backpanel-virtual.json ^
  --target-ip 127.0.0.1 ^
  --listen 127.0.0.1:6454 ^
  --web 127.0.0.1:9982
```

Open:

```text
http://127.0.0.1:9982/
```

## Live PXLBLZ test with no hardware

Run:

```bat
run-virtual-backpanel-live.bat
```

This creates:

```text
PXLBLZ
  -> WebSocket 127.0.0.1:9980
pxlblz-router
  -> Art-Net UDP 127.0.0.1:6454
pxlblz-virtual-controller
  -> receiver state machine
  -> simulated DMA/output
  -> browser visualization 127.0.0.1:9982
```

Then enable **OUT** in PXLBLZ.

No physical Art-Net controller receives those packets.

## Automated smoke test

```bat
run-virtual-backpanel-selftest.bat
```

The test runs the native router plus frame sender against the virtual receiver
and writes a JSON summary to the Windows temporary directory.

## Browser visualization

The visualizer shows:

- receiver/run-policy state;
- packet rate;
- complete-frame rate;
- simulated DMA completion rate;
- candidate completeness;
- sequence mode and current sequence;
- rejected / ignored / stale / duplicate / incomplete counters;
- complete frames versus physically submitted frames;
- receiver ingest p99;
- first-packet-to-complete assembly p99;
- simulated blackout state;
- one live RGB strip per electrical output;
- universe range, LED count, color order and brightness metadata per route.

Endpoints are intentionally simple:

```text
GET /api/status   JSON receiver state/counters/timing
GET /api/frame    raw RGB888 physical display buffer
GET /             visualizer
```

This makes the emulator usable by other tooling as well as the included UI.

## Fault injection

The companion executable:

```text
pxlblz-receiver-probe.exe
```

can reproduce receiver edge cases without changing the main router.

Examples while the virtual controller is running:

Normal complete frame:

```bat
pxlblz-receiver-probe.exe --scenario complete
```

U145 too short, reproducing the historical 78-byte failure:

```bat
pxlblz-receiver-probe.exe --scenario short-tail --fault-universe 145 --fault-bytes 78
```

Missing U149:

```bat
pxlblz-receiver-probe.exe --scenario missing --fault-universe 149
```

Sequence incremented per packet:

```bat
pxlblz-receiver-probe.exe --scenario packet-seq
```

Expected frame plus ignored U138/U150/U151/U152:

```bat
pxlblz-receiver-probe.exe --scenario extras
```

Sequence-zero frame:

```bat
pxlblz-receiver-probe.exe --scenario seq0
```

Duplicate universe in sequence-zero mode:

```bat
pxlblz-receiver-probe.exe --scenario duplicate0
```

Watch the receiver counters in the browser while running these tests.

## Automated compatibility vectors

`internal/virtualcontroller/receiver_test.go` encodes the receiver behaviors
that previously required firmware/hardware probing:

- exact 29-universe complete frame;
- U145 78-byte rejection then partial-frame timeout;
- missing U149 then partial-frame timeout;
- per-packet changing sequence never completing;
- extra ignored universes while expected frame still completes;
- sequence-zero completion;
- sequence-zero duplicate abandon/restart;
- 255 -> 1 wrap;
- stale sequence rejection;
- 26.7 ms wire guard;
- simulated DMA completion;
- >1 second data-loss blackout;
- >1 second sequence-state reset.

The normal virtual E2E suite also connects the **real sender router over UDP**
to this emulator and requires clean receiver counters.

## Performance work

Receiver-side benchmarks:

```bat
cd router
go test -run ^$ -bench BenchmarkBackPanel -benchmem ./internal/virtualcontroller
```

The emulator exposes ingest and assembly timings in the UI/API so optimizations
can be evaluated on both sender and receiver sides.

Useful experiments now possible without hardware:

- change input FPS and observe complete/replaced/submitted ratios;
- test packet burst pacing;
- compare different route layouts;
- change longest simulated LED lane and wire guard;
- test 30/40/60 FPS run policies;
- inject missing/short/duplicate/stale packets;
- test sequence wrap and sender restart;
- measure first-packet -> complete-frame latency;
- measure protocol ingest cost separately from UDP sender cost;
- test blackout/recovery behavior;
- visualize physical port data after color-order/brightness transformation.

## Profiles other than BACK_PANEL

The executable derives expected universes dynamically from the selected target
IP in a router config:

```text
--config <route-config>
--target-ip <profile-controller-ip>
```

This makes the state machine usable for WS2812_NODE and PANEL8_251 route-level
experiments too. However only BACK_PANEL_249 currently has source-derived
Teensy receiver semantics documented in detail. Other controller firmware
should not be called an exact firmware twin until its receiver code/behavior is
verified.
