# PXLBLZ IDE integration

This directory contains the first experimental direct output adapter from
PXLBLZ IDE to `pxlblz-router.exe`.

## Goal

```text
PXLBLZ render loop
      ↓ one rendered packed Float64 RGB frame
      ├─ normal PXLBLZ WebGL preview
      └─ Float RGB → reusable RGB888 buffer
                    ↓
             browser WebSocket
       ws://127.0.0.1:9980/pixels
                    ↓
             pxlblz-router.exe
                    ↓
                  Art-Net
```

There is **no canvas readback**, no second pattern render and no duplicate 3D
mapping stage.

## First prototype behaviour

Output is deliberately opt-in:

```text
?pxout=1
```

Once enabled, the experimental output flag and optional `pxoutUrl` are kept in
`sessionStorage`, so Gallery → Studio navigation or an auth redirect in the
**same browser tab** does not silently disable hardware output. Use:

```text
?pxout=0
```

to explicitly disable it again for that tab. Closing the tab clears the state.

Example when PXLBLZ is running on Vite's usual local URL:

```text
http://localhost:5173/?pxout=1
```

The default router endpoint is:

```text
ws://127.0.0.1:9980/pixels
```

An alternate endpoint can be supplied for development:

```text
?pxout=1&pxoutUrl=ws%3A%2F%2F127.0.0.1%3A9980%2Fpixels
```

## Important prototype decisions

- Output is enabled only for the full-resolution Preview instance.
- Normal PXLBLZ behaviour is unchanged when `pxout` is absent.
- The adapter sends canonical raw pattern RGB, clamped to 0..1 then converted
  to RGB888.
- The PXLBLZ preview brightness/dimmed renderer controls are **not applied to
  hardware output in this first prototype**. Brightness ownership remains a
  deferred design decision in the project plan.
- Browser WebSocket backpressure is lossy by design. If one whole RGB frame is
  already buffered, a new render frame is skipped instead of queued.
- The native router adds a second LatestFrame layer, so slow controllers never
  accumulate old animation frames.

## Installation

Use `install-pxlblz-output.ps1` from PowerShell, or apply
`Preview.integration.md` manually.

The installer expects the current upstream PXLBLZ Preview structure that was
reviewed around commit `d685125b34c694f311972e258efb48d12cf05cd8`.
It creates a backup of `Preview.tsx` before editing it.

## Router test command

Start the already hardware-verified BACK_PANEL router:

```bat
pxlblz-router.exe --config config\routes.backpanel-all.json --input ws
```

Then open PXLBLZ with `?pxout=1`.

The router should report a client and valid RX frames before Art-Net starts.

## Output control UI

The integration now adds a compact **OUT** button to the PXLBLZ Preview header.

- gray `OUT`: external output disabled;
- green `OUT`: external output enabled;
- clicking the button stores the tab-local preference and reloads the current
  PXLBLZ route so the render loop is rebuilt deterministically;
- Gallery → Studio navigation keeps the output preference in the same tab;
- closing the tab clears the session preference;
- `?pxout=1` and `?pxout=0` remain supported for automation and recovery;
- `pxoutUrl` remains the advanced endpoint override.

The button reports **configured enabled/disabled state**, not controller health.
The native router telemetry remains the authority for live client connection,
RX/TX FPS, replaced frames, packet rate and send errors.
