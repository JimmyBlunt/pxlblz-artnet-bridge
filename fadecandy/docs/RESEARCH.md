# Research notes

## PXLBLZ-IDE seams

The current PXLBLZ render loop already exposes a packed frame path
(`paintPacked`) backed by a `Float64Array`. This is the correct source for
external pixel output: no Canvas/WebGL readback and no second Pattern render.

The current Art-Net project plan independently reaches the same conclusion and
defines the next PXLBLZ integration as:

```text
createRenderLoop()
  -> packed rendered Float64 RGB
  -> Preview paint + ExternalPixelOutput
  -> clamp/Float->UInt8 once
  -> binary WebSocket
```

Fadecandy should therefore consume the same canonical output contract rather
than inventing another Pattern/render path.

## PXLBLZ cube order

PXLBLZ's stock cube mapper generates coordinates in x-fastest, then y, then z
order. With 512 pixels, side=8 and the logical index is:

```text
x = i % 8
y = floor(i/8) % 8
z = floor(i/64)
```

This is exactly the logical coordinate convention used in this workspace.

## Fadecandy / fcserver

fcserver accepts Open Pixel Control on TCP port 7890. A Set Pixel Colors packet is:

```text
byte 0     OPC channel
byte 1     command 0x00
bytes 2-3  payload length, big endian
bytes 4..  RGB bytes
```

A complete Set Pixel Colors message causes fcserver to broadcast the new frame
to attached Fadecandy devices. Fadecandy's server configuration maps OPC pixel
ranges to physical device pixels.

For a single controller with all 512 pixels:

```json
"map": [[0, 0, 0, 512]]
```

Fadecandy output pixels 0..511 correspond to eight 64-pixel outputs.

fcserver also supports a WebSocket interface on port 7890. Binary WebSocket
messages use the OPC channel/command header, but the two OPC length bytes are
reserved/zero because WebSocket already carries frame length. That is useful for
a future browser-direct implementation.

## Fadecandy image quality features worth preserving

The original server/device stack exposes:

- gamma / whitepoint color correction;
- temporal dithering;
- inter-frame interpolation.

Those features are a large part of why Fadecandy output feels unusually smooth.
The bridge should therefore send ordinary RGB values and leave those operations
to fcserver/Fadecandy instead of trying to reproduce them in PXLBLZ.

## L3D reference

The original Looking Glass L3D library uses an 8x8x8 cube and maps coordinates
with:

```text
index = (z*64) + (x*8) + y
```

This workspace represents the same mapping with axis-order metadata instead of
embedding the formula in the output code.

## Useful external projects / references

- Fadecandy source/docs and forks: OPC protocol, WebSocket protocol, fcserver config.
- zestyping/openpixelcontrol: protocol reference plus 3D layout simulator.
- Looking-Glass/L3D: original cube coordinate/index implementation.
- enjrolas/L3D-Hardware: original L3D hardware reference.
- TheMariday/marimapper: modern mapper with tested Fadecandy backend; potentially useful if the physical cube's real wiring/orientation differs from assumed mapping.

## Architectural conclusion

The most useful split is not `Pixelblaze -> Fadecandy hardware directly`.
It is:

```text
PXLBLZ canonical frame
    -> logical/physical mapping adapter
    -> controller-specific transport
```

Art-Net and Fadecandy then become peer outputs from the same rendered frame.
