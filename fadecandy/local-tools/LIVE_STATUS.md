# Fadecandy live connection — 2026-09-20

## Local authenticated workspace (current entry point)

Open http://127.0.0.1:5176/local-workspace-login and choose **Lokalen Workspace öffnen**.
This uses the repository's existing `github:local-dev` identity and normal signed session implementation against the local database in `PXLBLZ-IDE-main/.wrangler/state`.
No GitHub OAuth setup or cloud synchronization is involved. The entry point binds only to 127.0.0.1 and rejects foreign Host/Origin requests. It does not modify Chrome's cookie database or replace localhost sessions.

Verified in the in-app browser:

- Local Dev account menu appears.
- The top navigation opens Maps without redirecting to sign-in.
- Local API reads for me, patterns, maps and shows succeed.
- GyroidGlow3D Cube volume / 512 pixels settings survive a page reload.

Chrome automation reported `ERR_BLOCKED_BY_CLIENT` when opening this entry point. No extension or browser protection was disabled. User-side Chrome access still needs verification.

Start the helper from this `2-FadeCandy-` directory, after both existing Vite servers (5174 and 5175) are running:

```powershell
node ..\PXLBLZ-IDE-main\node_modules\tsx\dist\cli.mjs .\local-workspace.mts
```

The helper is a local reverse proxy: UI → 5175, API → local 5174. Keep its process running. This is a local development setup, not a deployed service or an installed auto-start service.

## Frame-rate measurement

2026-09-20 10:43:37–10:44:07 UTC: temporary transparent loopback measurement at port 9982 forwarded frames to the existing bridge at 9981.

- One browser source.
- 59.9–60.0 complete RGB frames per second, 1536 bytes each.
- 60 changed frames per second (not repeated copies of one frame).
- Around 51–158 illuminated pixels in sampled GyroidGlow3D frames; this pattern intentionally lights only part of the volume.
- After measurement, the preview was returned to direct port 9981 and the measurement process was stopped.
- No rendering scheduler code was changed. Earlier Chrome UI observations showed both 1 FPS and 60 FPS; browser throttling remains a limitation of the preview-driven output. A login page sends no frames and the bridge holds the last received frame.

The measured sender is verified at 60 FPS; the user must still confirm the physical animation looks smooth.

## Follow-up: stutter / flashes after reopening the workspace

The user reported persistent stutter and flashes. A fresh connection inventory showed **two simultaneous browser clients** connected to the same :9981 bridge. In-app tabs 5 and 6 were both running GyroidGlow3D with external output enabled, at different animation times. The bridge accepts all clients into one LatestFrame buffer, so frames from independent producers overwrite one another.

The agent-created test tab 5 was closed. User tab 6 remains open. A subsequent TCP inventory verified exactly **one** connected input client; its browser log confirms 512 pixels / 1536 bytes. Physical improvement still needs user confirmation.

Use only one output-enabled Studio tab at a time. The bridge currently has no exclusive-sender arbitration. No interpolation setting was changed for this follow-up, so the effect of removing the duplicate sender can be assessed on its own.

## Current verified state

- The server on port 5174 serves a Preview without the external output adapter.
- The existing `../PXLBLZ-IDE` checkout contains the adapter and is now served on port 5175.
- Open the full Studio preview, not the capped gallery detail preview:
  http://localhost:5175/PXLBLZ-IDE/studio/patterns/GyroidGlow3D?pxout=1&pxoutUrl=ws%3A%2F%2F127.0.0.1%3A9981%2Fpixels
- Selected map: Cube volume. Pixel count: 512 (8×8×8).
- Browser console verified: `[pxout] connected ws://127.0.0.1:9981/pixels (512 pixels / 1536 bytes)`.
- Established TCP connections verified for browser → bridge :9981 and bridge → fcserver :7890.
- After the user reconnected USB, the fcserver status page reports **Fadecandy LED Controller**, serial `LZUJQUQYZLZAQQZX`, firmware **1.07**.
- Browser → bridge and bridge → fcserver TCP connections remain established. Browser output logs also show frames sent.
- Current physical LED appearance still needs the user's visual confirmation; USB detection and software connectivity are verified.

## Restart the prepared IDE server

From the `PXLBLZ-IDE` directory, in PowerShell:

```powershell
$env:VITE_API_PROXY_TARGET = 'http://localhost:5174'
node node_modules/vite/bin/vite.js --host localhost --port 5175 --strictPort
```

Keep this process running. The existing 5174 server was not stopped or changed. No IDE source changes were made in this session.

## Next physical step

Observe the cube while the GyroidGlow3D Studio preview runs and confirm that the animated pattern is visible. USB reconnection has resolved the previous missing-device state.

Current output behavior holds the last valid frame on source loss. Preview brightness is not applied by the existing external RGB adapter; do not treat that slider as a hardware blackout control.

Physical confirmation: User confirms the cube now runs smoothly without hanging, stutter or flashing after closing the duplicate output tab. Keep exactly one output-enabled PXLBLZ tab. Local workspace remains at http://127.0.0.1:5176/PXLBLZ-IDE/ with direct ws://127.0.0.1:9981/pixels output. No interpolation change was needed.

## Variable pixel-count update (v0.1.1-dev)
Implemented and all Go package tests pass, including masked WebSocket -> normalization -> L3D mapping -> 512-pixel OPC checks for 2048, 128, 512, 2197 and 1 pixels. Oversized frames crop to the first 512 logical pixels; short frames black-pad before wiring mapping. Empty/incomplete RGB rejected; WS input cap 16 MiB. Map remains free.
New executable: 2-FadeCandy-/pxlblz-fadecandy-0.1.1.exe. Start with 2-FadeCandy-/Start-Fadecandy.ps1 after stopping the previous bridge. Source edits in pxlblz-artnet-bridge/fadecandy. Go toolchain downloaded from go.dev and SHA256 verified, stored under 2-FadeCandy-/tools/go.
Activation pending: Windows denied Stop-Process for old bridge PID 43668 on port 9981 (Access denied). Old bridge remains running. No fcserver or other process stopped.


Activation complete: User stopped old PID 43668. Started Start-Fadecandy.ps1 / v0.1.1-dev (exec session 86508), direct ws://127.0.0.1:9981/pixels to fcserver 7890. Live UI tested SceneSplice3D at 2197 pixels (6591 bytes), then 125 pixels (375 bytes), then restored Cube volume / 512. For both variable sizes: one sender, RX approximately 60 fps, invalid 0/s, OPC TX 60 fps, no output errors. Browser left running SceneSplice3D. No controller Chrome extension needed for this Fadecandy path.
