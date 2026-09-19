# BACK_PANEL_249 receiver findings

Source inspection date: 2026-09-19.

Active PlatformIO environment:

```text
teensy41_octo_web_rx32
```

Active receive path:

```text
include/runtime_receiver.h
include/artnet_run_policy.h
src/web_main.cpp
```

## Expected universes

The running configuration expects exactly 29 active Art-Net universes:

```text
U120-U137
U139-U149
```

U138 is not assigned. U150-U152 belong to disabled OUT8 and are not expected.

## Last-universe minimum payloads

| Output | LEDs | Last universe | Minimum ArtDmx payload |
|---|---:|---|---:|
| OUT1 | 203 | U121 | 100 bytes on wire, 99 used |
| OUT2 | 738 | U126 | 174 |
| OUT3 | 880 | U132 | 90 |
| OUT4 | 810 | U137 | 390 |
| OUT5 | 352 | U141 | 36 |
| OUT6 | 610 | U145 | 300 |
| OUT7 | 512 | U149 | 6 |

The declared ArtDmx payload must be even, 2..512 bytes, and the UDP packet length must equal `18 + payload`.

## Sequence

The receiver has one sequence state for the whole controller.

For sequence 1..255 every expected universe in one candidate must carry the same sequence. A newer sequence abandons an incomplete candidate.

Sequence 0 is supported as a special unordered collection mode, but the bridge uses normal non-zero sequencing.

## Completion and DMA

Frame completeness is controller-wide.

Only after all expected route bits are present does the receiver publish a complete frame. The run policy then schedules FastLED/ObjectFLED output when timing and DMA guards allow it.

Partial candidates expire after >100 ms measured from their first accepted packet.

## Diagnostic lesson

Packet count alone does not prove frame completeness because received packet statistics also include ignored/rejected traffic.

During bring-up, the decisive diagnostics were:

```text
accepted/complete frames
rejected
ignored
incomplete
stale
duplicates
frames submitted
DMA completed
```

