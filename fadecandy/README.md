# PXLBLZ Fadecandy / L3D output

Independent experimental output path for driving an 8x8x8 L3D cube through
`fcserver`/Fadecandy without changing the existing Art-Net router.

Data path:

```text
PXLBLZ-IDE logical RGB frame (512 pixels)
  -> localhost binary WebSocket
  -> L3D logical-to-physical mapper
  -> OPC Set Pixel Colors over TCP
  -> fcserver :7890
  -> Fadecandy USB
  -> 8x8x8 L3D cube
```

The default mapping assumes canonical logical order x-fastest and reproduces the
original L3D physical formula `physical = z*64 + x*8 + y`.

## Local development

```bash
go test ./...
go run ./cmd/pxlblz-fadecandy --config config/l3d-8x8x8.json --dry-run
```

The live input endpoint is intentionally separate from the Art-Net router during
development (`ws://127.0.0.1:9981/pixels`). Later integration can either give
PXLBLZ multiple output targets or fan a single rendered frame to both routers.

## Hardware-orientation test patterns

No PXLBLZ connection is required for the first cube test:

```bash
go run ./cmd/pxlblz-fadecandy --config config/l3d-8x8x8.json --input pattern --pattern axes
go run ./cmd/pxlblz-fadecandy --config config/l3d-8x8x8.json --input pattern --pattern corners
go run ./cmd/pxlblz-fadecandy --config config/l3d-8x8x8.json --input pattern --pattern voxel
```

`axes` draws red +X, green +Y and blue +Z from a white origin. It is the fastest
way to identify the real cube orientation before touching any PXLBLZ code.
