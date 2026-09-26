# New PC Setup / Disaster Recovery

This is the reproducible Windows setup for the PXLBLZ → Art-Net project.

## Prerequisites

```text
Git
Node.js 22.x
npm
PowerShell
Chrome/Chromium
Go 1.23+ only when rebuilding/testing native Go tools
```

## Recommended layout

```text
C:\Projects\PXLBLZ-ArtNet\
  pxlblz-artnet-bridge\
  PXLBLZ-IDE-main\
  PXLBLZ-IDE\
```

## 1. Clone the bridge

```bat
cd /d C:\Projects\PXLBLZ-ArtNet
git clone https://github.com/JimmyBlunt/pxlblz-artnet-bridge.git
```

## 2. Restore the pinned PXLBLZ upstream

```bat
git clone https://github.com/jon-whiteroomsoftware/PXLBLZ-IDE.git PXLBLZ-IDE-main
cd PXLBLZ-IDE-main
git switch main
git reset --hard d685125b34c694f311972e258efb48d12cf05cd8

git worktree add "..\PXLBLZ-IDE" -b pxlblz-artnet-output d685125b34c694f311972e258efb48d12cf05cd8
git worktree list
```

## 3. Install PXLBLZ dependencies

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\PXLBLZ-IDE
npm ci
```

Do not run `npm audit fix --force`.

## 4. Apply the bridge integration

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\pxlblz-artnet-bridge

powershell -ExecutionPolicy Bypass -File ".\pxlblz-integration\install-pxlblz-output.ps1" -PxlblzPath "..\PXLBLZ-IDE"
```

Verify:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\PXLBLZ-IDE
npm run build
```

## 5. Prepare native-Windows local Studio

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\pxlblz-artnet-bridge

powershell -ExecutionPolicy Bypass -File ".\pxlblz-integration\prepare-windows-local-studio.ps1" -PxlblzPath "..\PXLBLZ-IDE" -MainPxlblzPath "..\PXLBLZ-IDE-main"
```

Then:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\PXLBLZ-IDE
npm run dev
```

Set the cookie command printed by the helper in the browser console and verify:

```js
fetch('/api/me').then(r => r.json()).then(console.log)
```

Expected: `authenticated: true`, `user.id: github:local-dev`.

Open:

```text
http://localhost:5174/PXLBLZ-IDE/studio?pxout=1
```

## 6. Run the native router

BACK_PANEL only, already hardware verified:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\pxlblz-artnet-bridge
bin\windows-x64\pxlblz-router.exe --config router\config\routes.backpanel-all.json --input ws
```

All currently known controllers:

```bat
bin\windows-x64\pxlblz-router.exe --config router\config\routes.installation-known.json --input ws
```

The known-installation config includes:

```text
10.0.0.244  WS2812_NODE  GRB
10.0.0.253  BACK_PANEL_249 RGB
10.0.0.251  PANEL8_251   BGR
44 Art-Net universes/frame
8186 logical pixels
```

APA102 is intentionally excluded until its exact route is confirmed.

## 7. Safe tests

No hardware packets:

```bat
router\run-installation-known-preflight.bat
router\run-installation-known-dry-run.bat
```

Small physical route-ID markers:

```bat
router\run-installation-known-port-id-test.bat
```

Virtual exact-adapter regression:

```bat
node pxlblz-integration\virtual-test\run-virtual-e2e.mjs
```

Performance smoke:

```bat
perf-test\run-performance.bat smoke
```

## Recovery after a damaged npm install

```bat
git restore package.json package-lock.json
rmdir /s /q node_modules
npm cache verify
npm ci
```

If Windows reports `EBUSY`, terminate only the Node processes belonging to
that PXLBLZ worktree before removing `node_modules`.

## Read before further changes

```text
README.md
docs/CURRENT_STATUS_HANDOFF.md
docs/PROJECT_PLAN.md
docs/WINDOWS_PXLBLZ_LOCAL_STUDIO.md
router/TEST_RESULTS.md
controller-reference/BACK_PANEL_249_RECEIVER.md
pxlblz-integration/README.md
```
