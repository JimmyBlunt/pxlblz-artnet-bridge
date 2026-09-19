# Preview.tsx integration

The prototype intentionally hooks the existing packed render path.

## 1. Add import

Near the other engine imports in `src/components/Preview.tsx`:

```ts
import { createExternalPixelOutput } from '@/engine/externalPixelOutput'
```

## 2. Create the output after layout resolution

Immediately after:

```ts
const { mapPoints, pixelCount, draw } = layout
```

add:

```ts
const externalPixelOutput =
  pixelCountCap === null ? createExternalPixelOutput(pixelCount) : null
```

The `pixelCountCap === null` guard prevents secondary/capped preview surfaces
from accidentally becoming hardware senders.

## 3. Add a packed paint function

Keep the existing `paint` function for the normal non-output path.

After it, add:

```ts
const paintPacked = externalPixelOutput?.enabled
  ? (frame: Float64Array, brightness: number, dimmed: boolean) => {
      if (positions3D) {
        const view = useCameraStore.getState()
        renderer.setCamera(captureCameraRef.current ?? view.camera)
        renderer.setZoom(view.zoom)
      }

      renderer.paint(frame, brightness, dimmed)
      captureRef.current.afterPaint(canvasRef.current)

      externalPixelOutput.sendPacked(frame)
    }
  : undefined
```

## 4. Pass it into the render loop

Add:

```ts
paintPacked,
```

beside the existing `paint,`.

When `paintPacked` is defined, PXLBLZ's existing renderLoop allocates one
reusable `Float64Array(pixelCount * 3)`. It renders each logical pixel once
into that array and calls only `paintPacked`, so there is no second render.

## 5. Cleanup

Change:

```ts
return () => loop.stop()
```

to:

```ts
return () => {
  loop.stop()
  externalPixelOutput?.close()
}
```

When `?pxout=1` is absent, `paintPacked` stays undefined and the existing
array-of-tuples Preview path remains unchanged.
