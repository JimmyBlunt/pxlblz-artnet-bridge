# v0.2 Live RGB Input

Default endpoint:

```text
ws://127.0.0.1:9980/pixels
```

One WebSocket **binary message** equals one complete canonical RGB frame:

```text
R G B  R G B  R G B ...
```

Message length must equal:

```text
config.input.pixel_count * 3
```

There is deliberately no JSON and no frame header in the hot path.

## Real-time semantics

The receiver is a one-slot `LatestFrame` handoff. If PXLBLZ generates frames faster than a controller's Art-Net rate, superseded frames are replaced instead of queued.

Example:

```text
PXLBLZ 60 FPS -> LatestFrame -> BACK_PANEL output 30 FPS
                 ^ no FIFO queue
```

Hardware test:

```bat
run-backpanel-ws-port-id-test.bat
```

This exact external-frame path has been verified on all 7 BACK_PANEL electrical outputs / 6 physical panels.
