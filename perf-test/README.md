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

## Custom overrides

Direct Node invocation supports:

```text
--profile smoke|perf|soak|stress
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
