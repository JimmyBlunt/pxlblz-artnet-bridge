# New PC Setup / Disaster Recovery

This document restores the project on another Windows PC.

## 1. Prerequisites

Recommended:

```text
Git
Node.js 22.x
npm
PowerShell
Chrome/Chromium
Go 1.23+ only if rebuilding the native router
```

PXLBLZ currently declares Node:

```text
^22.13.0 || >=24
```

The project was developed with Node 22.

---

## 2. Workspace layout

Recommended:

```text
C:\Projects\PXLBLZ-ArtNet\
    pxlblz-artnet-bridge\
    PXLBLZ-IDE-main\
    PXLBLZ-IDE\
```

`PXLBLZ-IDE-main` is the clean `main` worktree.

`PXLBLZ-IDE` is the modified `pxlblz-artnet-output` worktree.

---

## 3. Clone the bridge project

```bat
cd /d C:\Projects\PXLBLZ-ArtNet
git clone https://github.com/JimmyBlunt/pxlblz-artnet-bridge.git
```

Verify:

```bat
cd pxlblz-artnet-bridge
git status
```

Windows binaries are already included under:

```text
bin\windows-x64\
```

---

## 4. Restore the exact PXLBLZ source version

Pin:

```text
d685125b34c694f311972e258efb48d12cf05cd8
```

Clone:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet
git clone https://github.com/jon-whiteroomsoftware/PXLBLZ-IDE.git PXLBLZ-IDE-main

cd PXLBLZ-IDE-main
git switch main
git reset --hard d685125b34c694f311972e258efb48d12cf05cd8
```

Create the Art-Net worktree from the same commit:

```bat
git worktree add "..\PXLBLZ-IDE" -b pxlblz-artnet-output d685125b34c694f311972e258efb48d12cf05cd8
```

Verify:

```bat
git worktree list
```

There should be a `main` worktree and a `pxlblz-artnet-output` worktree.

---

## 5. Install dependencies

Clean main worktree:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\PXLBLZ-IDE-main
npm ci
```

Modified worktree:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\PXLBLZ-IDE
npm ci
```

Do not run:

```text
npm audit fix --force
```

The pinned project contains alpha/dev dependencies and forced remediation can change the tested dependency graph.

---

## 6. Apply the PXLBLZ Art-Net integration

From PowerShell or CMD:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\pxlblz-artnet-bridge

powershell -ExecutionPolicy Bypass -File ".\pxlblz-integration\install-pxlblz-output.ps1" -PxlblzPath "..\PXLBLZ-IDE"
```

The installer:

- backs up `Preview.tsx`
- installs `src/engine/externalPixelOutput.ts`
- inserts the packed-frame hook
- adds cleanup

Verify the modified PXLBLZ worktree:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\PXLBLZ-IDE
git status
npm run build
```

Expected modified/untracked files include:

```text
src/components/Preview.tsx
src/engine/externalPixelOutput.ts
Preview.tsx.pxout-backup-...
```

---

## 7. Verify the native router before involving PXLBLZ

Bridge directory:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\pxlblz-artnet-bridge
```

Local loopback:

```bat
router\run-ws-loopback-test.bat
```

Expected steady state:

```text
RX ~60 FPS
TX ~60 FPS
0 invalid
0 send errors
```

Real BACK_PANEL external sender test:

```bat
router\run-backpanel-ws-port-id-test.bat
```

Expected:

```text
RX 60 FPS
TX 30 FPS
870 pkt/s
0 invalid
0 send errors
all 6 physical BACK_PANEL panels active
```

---

## 8. Start router for real PXLBLZ input

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\pxlblz-artnet-bridge

bin\windows-x64\pxlblz-router.exe --config router\config\routes.backpanel-all.json --input ws
```

Expected:

```text
Pixel input listening: ws://127.0.0.1:9980/pixels
Waiting for first valid input frame before Art-Net transmission starts...
```

---

## 9. Basic unauthenticated PXLBLZ test

In the modified PXLBLZ worktree:

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\PXLBLZ-IDE
npm run dev
```

Typical URL:

```text
http://localhost:5174/PXLBLZ-IDE/
```

Enable the experimental external output:

```text
http://localhost:5174/PXLBLZ-IDE/?pxout=1
```

The direct transport was previously verified with:

```text
clients 1
RX 60 FPS
invalid 0
TX 30 FPS
870 pkt/s
errors 0
```

---

## 10. Local Studio login on native Windows

Real GitHub/Google OAuth is optional and not required for this project test.

The upstream managed `npm run dev:main` coordinator is currently problematic on native Windows because it calls Unix `ps`/`lsof`.

Use the manual direct-worker workaround.

### Main worktree secret

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\PXLBLZ-IDE-main
copy .dev.vars.example .dev.vars
notepad .dev.vars
```

At minimum set a long local value:

```text
SESSION_SECRET=replace-with-a-long-local-development-secret
```

GitHub and Google client fields may remain empty for synthetic local login.

### Copy the dev vars to the modified worktree

```bat
cd /d C:\Projects\PXLBLZ-ArtNet\PXLBLZ-IDE
copy /Y "..\PXLBLZ-IDE-main\.dev.vars" ".dev.vars"
```

### Apply local D1 migrations

```bat
npm run db:migrate:local
```

### Seed the local developer identity

```bat
npx wrangler d1 execute pxlblz-ide --local --command "INSERT INTO users (id, github_user_id, github_login, display_name, avatar_url, created_at, updated_at) VALUES ('github:local-dev','local-dev','local-dev','Local Dev',NULL,unixepoch(),unixepoch()) ON CONFLICT(id) DO UPDATE SET display_name=excluded.display_name, updated_at=excluded.updated_at;"
```

### Start ordinary Vite/Cloudflare worker

```bat
npm run dev
```

### Mint a synthetic developer session

In a second terminal in the modified worktree:

```bat
npm run dev:session -- --developer
```

It should print:

```text
pxlblz_session=<long-token>
```

Open browser DevTools Console on localhost and set:

```js
document.cookie = "pxlblz_session=<TOKEN>; path=/; SameSite=Lax"
```

Verify:

```js
fetch('/api/me').then(r => r.json()).then(console.log)
```

Expected:

```text
authenticated: true
```

Open:

```text
http://localhost:5174/PXLBLZ-IDE/studio?pxout=1
```

This manual Windows path was the active continuation point at handoff and still needs final confirmation.

---

## 11. If npm dependencies get damaged

Known recovery:

```bat
git restore package.json package-lock.json
```

Find Node processes holding the worktree:

```bat
powershell -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object {$_.Name -eq 'node.exe' -and $_.CommandLine -like '*PXLBLZ-IDE*'} | Select-Object ProcessId,CommandLine"
```

Stop the relevant PID:

```bat
taskkill /PID <PID> /F
```

Then:

```bat
rmdir /s /q node_modules
npm cache verify
npm ci
```

---

## 12. Continue development

Before changing protocol code, read:

```text
README.md
docs/CURRENT_STATUS_HANDOFF.md
docs/PROJECT_PLAN.md
docs/WINDOWS_PXLBLZ_LOCAL_STUDIO.md
router/TEST_RESULTS.md
controller-reference/BACK_PANEL_249_RECEIVER.md
pxlblz-integration/README.md
```

Then use `docs/AI_HANDOFF_PROMPT.md` when starting a new AI/developer session.
