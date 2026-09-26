# Reusable performance / throughput / stability lab

This environment is intended to stay with the project and be reused for every
future router or PXLBLZ-output optimization.

It runs the **real TypeScript externalPixelOutput adapter**, the **real Go
router**, and a **frame-aware virtual Art-Net receiver**.

## What it measures

### Correctness / stability

The Art-Net probe reconstructs complete logical frames and records:

- complete frames
- incomplete frames
- invalid ArtDmx packets
- unexpected universes
- wrong ArtDmx payload lengths
- duplicate universes
- late packets from an already completed sequence
- sequence-gap events
- per-universe packet counts

### Timing / throughput

It records:

- complete output FPS
- packets per second
- Mbit/s
- first-packet → complete-frame assembly time
- frame-to-frame interval
- p50 / p95 / p99 / max timing distributions

### Router cost

Router telemetry records once per second:

- RX FPS
- replaced input frames
- TX FPS
- packets/s
- average SendFrame cost
- last SendFrame cost
- maximum SendFrame cost
- Go heap
- GC count
- goroutine count
- send errors

### Microbenchmarks

Go benchmarks measure the packet/routing hot path without timers/network:

- production BACK_PANEL 8186-pixel route
- synthetic 32768-pixel RGB route
- synthetic 32768-pixel GRB reorder route
- ns/op
- throughput
- allocations/op

## Profiles

### smoke

Fast local regression:

```text
8186 pixels
input 60 FPS
router 30 FPS
29 universes/frame
10 seconds
```

Run:

```bat
run-performance.bat smoke
```

### perf

Performance comparison before/after an optimization:

```text
8186 pixels
input 120 FPS
router 60 FPS
29 universes/frame
60 seconds
```

Run:

```bat
run-performance.bat perf
```

### soak

Long stability/memory run:

```text
8186 pixels
input 60 FPS
router 30 FPS
default 15 minutes
```

Run default:

```bat
run-performance.bat soak
```

One hour:

```bat
run-performance.bat soak 3600
```

### stress

Scaling headroom:

```text
32768 pixels
input 120 FPS
router 120 FPS
~193 universes/frame
60 seconds
```

Run:

```bat
run-performance.bat stress
```

### installation (v0.3 multi-controller)

Simulates the whole installation output with **one virtual Art-Net receiver per
controller**. Uses `router/config/routes.multi-loopback.json` (real .244 / .251 /
.253 universe layout) on 127.0.0.1 / .2 / .3:

```text
LOOP_A_60FPS  127.0.0.1   9 universes  60 FPS  GRB
LOOP_B_30FPS  127.0.0.2   6 universes  30 FPS  BGR
LOOP_C_30FPS  127.0.0.3  29 universes  30 FPS  RGB (BACK_PANEL layout)
input 60 FPS, 20 seconds, expected 1590 packets/s
```

Each controller gets its own frame-aware `artnet-probe` bound to its IP, so
completeness, sequence gaps and FPS are checked **per controller** against that
controller's own `fps_target`. The router runs without `--fps` so the
per-controller scheduler is exercised; `--output-fps` is ignored for this profile.

Run:

```bat
run-performance.bat installation
```

Extra result files: `probe-<controller>.json` / `probe-<controller>.log`;
`summary.json` gains `probes` (per controller) and `expected.controllers`.
`summary.probe` holds the fastest controller so `compare-results.mjs` keeps working.

## Custom overrides

Direct Node invocation supports:

```text
--profile smoke|perf|soak|stress|installation
--seconds N
--pixels N
--input-fps N
--output-fps N
--out PATH
```

Example:

```bat
node perf-test\run-performance.mjs --profile stress --pixels 50000 --input-fps 120 --output-fps 90 --seconds 120
```

## Result files

Every run gets its own immutable result folder:

```text
perf-test/results/<timestamp>-<profile>-<platform>/
    summary.json
    machine.json
    probe.json
    router-samples.csv
    router.log
    probe.log
    driver.log
    microbench.txt
```

`summary.json` is the primary comparison artifact.

Keep important baseline/result folders outside Git or attach them to releases /
benchmark records. The generated result tree is gitignored.

## Current pass conditions

Hard failures currently include:

- any invalid packet
- any unexpected universe
- any payload mismatch
- any incomplete frame
- any duplicate universe
- any sequence gap
- any late packet from the completed sequence
- output FPS below 97% of target
- packet rate below 97% of expected
- router invalid WebSocket frames
- router send errors
- p99 SendFrame duty >= 75% of frame period
- Art-Net assembly p99 >= min(25 ms, 90% of frame period)

Heap growth is reported and currently warns above 32 MB rather than failing.
After we collect long-run baselines on the target Windows machine, that should
be tightened into a hard stability threshold.


## Load-generator design

Performance profiles use the exact PXLBLZ adapter with a deadline-anchored
scheduler. The scheduler does not use a drifting `setInterval`: every frame
deadline remains relative to the original start time.

The performance lab uses `--mode static` for the source frame so the benchmark
isolates:

```text
Float64 -> RGB888 conversion
WebSocket transport
LatestFrame
router packetization
UDP output
receiver reassembly
```

The static frame still changes one value each frame so consecutive messages are
not byte-identical.

The separate normal virtual E2E test keeps the dynamic frame generator for a
more browser-like functional exercise.

## Adapter microbenchmark

The exact production `externalPixelOutput.ts` is benchmarked with a null
WebSocket sink before the E2E run.

Reported metrics include:

```text
ms/frame
frames/s
RGB output MB/s
Float64 input MB/s
```

This isolates the Float64-to-RGB888 conversion cost from UDP/network timing.

## Baseline versus candidate

Keep the `summary.json` from a known-good run, make one optimization, run the
same profile again, then compare:

```bat
node perf-test\compare-results.mjs baseline\summary.json candidate\summary.json
```

The comparison prints percentage changes for:

- adapter conversion
- complete output FPS / packets per second
- Art-Net assembly p99
- frame interval p99
- router SendFrame p50 / p95 / p99 / max
- heap metrics
- Go microbenchmark ns/op and MB/s

It exits non-zero if a previously passing correctness gate becomes failing.

For meaningful performance comparisons use the **same machine, same profile,
same duration and minimal unrelated workload**.
