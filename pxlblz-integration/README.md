# PXLBLZ IDE integration

This directory contains the shared experimental external pixel output adapter for PXLBLZ IDE.

The same PXLBLZ-side code is used for both verified native output daemons:

```text
PXLBLZ render loop
      â†“ one packed Float64 RGB frame
normal preview + ExternalPixelOutput
      â†“ one reusable RGB888 conversion
browser WebSocket
      â”œâ”€ ws://127.0.0.1:9980/pixels -> Art-Net router
      â””â”€ ws://127.0.0.1:9981/pixels -> Fadecandy/L3D router
```

There is no canvas readback, no second pattern render and no duplicate logical mapping stage.

## Install

From the bridge repository:

```powershell
powershell -ExecutionPolicy Bypass -File .\pxlblz-integration\install-pxlblz-output.ps1 -PxlblzPath "C:\path\to\PXLBLZ-IDE" -Target fadecandy
```

Available targets:

- `fadecandy` -> `ws://127.0.0.1:9981/pixels`
- `artnet` -> `ws://127.0.0.0.1:9980/pixels`
- ``custom` -> pass `-CustomUrl "ws://host:port/path"`

The installer checks the expected PXLBLZ source anchors, creates a timestamped backup of `Preview.tsx`, copies `externalPixelOutput.ts` into `src/engine`, wires the existing `paintPacked` render path into the preview, keeps hardware output opt-in, and prints the exact URL/query parameters for the selected target.

The reviewed upstream PXLBLZ revision is:

```text
d685125b34c694f311972e258efb48d12cf05cd8
```

## Fadecandy/L3D first live test

Start the already hardware-verified lower stack:

```powershell
cd fadecandy
.\bin\fcserver.exe .\config\fcserver-l3d.json
```

Second terminal:

```powershell
cd fadecandy
..\bin\windows-x64\pxlblz-fadecandy.exe --config .\config\l3d-8x8x8.json --input ws
```

Then run PXLBLZ with the stock 8x8x8 cube map and 512 pixels and open:

```text
http://localhost:5173/?pxout=1&pxoutUrl=ws%3A%2F%2F127.0.0.1%3A9981%2Fpixels
```

The Fadecandy router must report:

```text
clients 1
RX > 0 fps
invalid 0/s
```

and the physical cube should match the PXLBLZ preview.

## Art-Net target

The same patched PXLBLZ build can instead point at the verified Art-Net router:

```text
http://localhost:5173/?pxout=1&pxoutUrl=ws%3A%2F%2F127.0.0.1%3A9980%2Fpixels
```

No second PXLBLZ integration is required.

## Brightness

The first prototype deliberately sends canonical raw pattern RGB from the packed render frame.

PXLBLZ preview brightness and dimmed state are not currently applied to hardware output. Brightness ownership remains a separate design decision.

## Real-time behavior

Browser WebSocket backpressure is lossy by design. If one complete RGB frame is already buffered, the next render frame is skipped rather than queued.

The native router then applies a second LatestFrame layer. ThhÈÙY\Èİ]]]™H[œİXYÙˆXØİ[][][™È[š[X][Ûˆ][˜ŞK‚‚ˆÈÈ›ÙXİ[Ûˆ˜YXØ[™H[\œÛ][Û‚‚•H›Ü›X[˜YXØ[™HÛÛ™šYÈÙY\Î‚‚˜œÛÛ‚ˆš[\œÛ]HˆYB˜‚•H›ËZ[\œÛ][ÛˆÛÛ™šYÈ™[XZ[œÈÛ›H\ÈHXYÛ›ÜİXÈÜ[Ûˆ›Üˆ^Xİ\‹Yœ˜[YHY[]H\İË‚