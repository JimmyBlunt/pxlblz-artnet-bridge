# Implementation plan

## Phase 0 - isolated prototype

Status: in progress / locally implemented.

- [x] separate `fadecandy/` workspace;
- [x] standard OPC TCP packet encoder;
- [x] fcserver TCP client with TCP_NODELAY;
- [x] latest-frame WebSocket input;
- [x] original L3D 8x8x8 mapping preset;
- [x] X/Y/Z axis reorder and axis flips;
- [x] custom LUT support in mapper core;
- [x] dry-run mode;
- [x] unit tests for OPC and mapping;
- [x] built-in cube orientation patterns.

## Phase 1 - bench test without PXLBLZ

Run fcserver with `config/fcserver-l3d.json` and connect the real Fadecandy.
Then run the bridge using internal Patterns:

1. `corners` - identifies all eight cube corners;
2. `axes` - red X, green Y, blue Z from the origin;
3. `layers` - walks one Z layer at a time;
4. `voxel` - walks one logical voxel through all 512 indices;
5. `xyz` - full RGB coordinate gradient.

From these tests determine:

- which physical corner is logical (0,0,0);
- direction of +X/+Y/+Z;
- whether any axis is swapped;
- whether any axis is reversed;
- whether the modified cube contains strand-level rewiring not expressible by simple axes.

Only change the mapping preset. Do not change Pattern code or OPC code.

## Phase 2 - PXLBLZ external output target

Add a reusable ExternalPixelOutput layer at the packed render-frame seam.

Suggested target model:

```text
ExternalPixelOutput
  targets:
    - ArtNet localhost WS :9980
    - Fadecandy L3D localhost WS :9981
```

Initial UI can be deliberately small:

```text
External Output: Off | Art-Net | Fadecandy L3D
```

Later this can become multi-target fanout if simultaneous outputs are useful.

Rules:

- only full-resolution active preview owns output;
- one Float->UInt8 conversion per frame;
- no canvas readback;
- no second render;
- no physical routing inside PXLBLZ;
- disconnect cleanly when output is disabled;
- latest-frame semantics remain lossy/live.

## Phase 3 - direct PXLBLZ -> fcserver option

Optional optimization, not needed for first hardware success.

Instead of localhost bridge WebSocket:

```text
PXLBLZ browser
 -> existing Chrome relay
 -> ws://fcserver:7890
 -> OPC-over-WebSocket
```

Advantages:

- one less process;
- fcserver already supports browser-oriented WebSocket OPC.

Reasons to defer:

- the standalone bridge is easier to diagnose;
- the bridge gives us a clean mapping boundary;
- custom LUT/axis transforms remain outside PXLBLZ;
- no extension changes are needed for first light.

## Phase 4 - mapping UX

After real-cube verification:

- mapping preset selector;
- X/Y/Z flip toggles;
- axis-order dropdowns;
- optional generated 512-entry LUT export;
- mapping diagnostics view with logical and physical index;
- optional Marimapper-import adapter if measured coordinates are useful.

## Phase 5 - quality and telemetry

- output FPS override;
- fcserver reconnect counter;
- frame replacement counter;
- send duration / maximum send duration;
- black/hold-last-frame policy on PXLBLZ stop;
- optional brightness ownership decision;
- optional fcserver gamma/whitepoint controls;
- option to toggle Fadecandy dithering/interpolation for A/B testing.
