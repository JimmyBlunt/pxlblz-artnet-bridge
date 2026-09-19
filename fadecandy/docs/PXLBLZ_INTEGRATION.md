# PXLBLZ integration contract

## Input frame contract

The Fadecandy bridge expects exactly 1536 bytes per WebSocket binary message:

```text
512 pixels * RGB * uint8
```

Pixel `i` is the PXLBLZ stock-cube logical index, x-fastest/y/z.

## Intended PXLBLZ producer

Use the existing `createRenderLoop()` packed-frame path. The output adapter
should convert the packed float frame to RGB8 without touching the normal preview
renderer.

Pseudo-flow:

```text
paintPacked(Float64Array frame)
    previewRenderer.paint(frame)
    externalOutput.publish(frame)
```

The external output implementation owns:

- clamping 0..1;
- Float -> UInt8;
- WebSocket lifecycle;
- optional target fanout.

It must not own:

- physical L3D mapping;
- Fadecandy gamma;
- controller wiring;
- Pattern evaluation.

## Development ports

- Art-Net router: `ws://127.0.0.1:9980/pixels`
- Fadecandy router: `ws://127.0.0.1:9981/pixels`

Keeping them separate during bring-up prevents either prototype from disturbing
the already hardware-verified Art-Net path.
