# Architecture

## Goal

Add a Fadecandy/L3D output without changing the verified Art-Net transport.
PXLBLZ remains responsible for rendering logical pixels; this module is only
responsible for L3D physical ordering and Fadecandy transport.

## Data path

```text
Pixelblaze Pattern (render3D)
        |
PXLBLZ-IDE render loop
        |
packed logical RGB frame
512 pixels, x-fastest/y/z
        |
binary WebSocket
        |
pxlblz-fadecandy
        |
L3D mapping
logical xyz -> physical 0..511
        |
OPC RGB8 packet
        |
TCP 127.0.0.1:7890
        |
fcserver
        |
Fadecandy USB
        |
8 x 64 physical outputs
        |
L3D 8x8x8 cube
```

## Why mapping and transport are separate

The pattern must not know physical wiring. The mapper converts a canonical
logical cube into the physical LED index order. The OPC layer only sees an
already ordered RGB byte frame.

This gives three independent layers:

1. PXLBLZ geometry/rendering.
2. L3D physical mapping.
3. Fadecandy/OPC transport.

Changing a flipped axis, rotated cube, custom repair wiring, or a replacement
controller does not require changing Pattern code.

## Canonical logical cube

PXLBLZ's stock Cube map is x-fastest, then y, then z. At 512 pixels this is an
8x8x8 lattice:

```text
logical = x + y*8 + z*64
```

This workspace uses that as the canonical input contract.

The original L3D library uses:

```text
physical = z*64 + x*8 + y
```

That is represented declaratively as:

```json
"input_order":  ["x", "y", "z"],
"output_order": ["y", "x", "z"]
```

No hard-coded special-case formula is needed in the router.

## Mapping modes

### Parametric cube mapping

Implemented now:

- dimensions;
- arbitrary input axis order;
- arbitrary output axis order;
- flip X/Y/Z.

This already expresses the original L3D wiring and all pure axis rotations / reflections.

### Custom LUT

Implemented in the mapper API: a 512-entry logical-to-physical lookup table can
replace parametric mapping. This is the escape hatch for historical repairs,
non-standard rewiring, swapped strands, or partial rebuilds.

### Serpentine transforms

Deferred until hardware verification proves they are required. They can be
added as another mapper transform without touching OPC or PXLBLZ.

## Frame timing

Follow the Art-Net bridge's verified live-video rule:

> Drop superseded frames; never accumulate latency.

The input keeps only the newest complete frame. If PXLBLZ runs at 60 FPS and the
Fadecandy output is intentionally limited to 30 FPS, intermediate frames are
replaced rather than queued.

## Transport choice

The standalone bridge uses standard OPC over TCP because:

- fcserver natively supports it on port 7890;
- it is only a 4-byte header plus RGB payload;
- Go can send it with the standard library;
- TCP_NODELAY can be enabled;
- no extra WebSocket dependency is needed in the local bridge.

A future browser-direct path may instead use fcserver's WebSocket interface.
That path should reuse the existing PXLBLZ Chrome relay rather than introduce a
second extension architecture.
