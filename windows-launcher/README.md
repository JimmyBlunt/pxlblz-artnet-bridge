# Windows desktop starter: Fadecandy v0.1.1

This versions the working Windows L3D setup, including the desktop shortcut,
a separate Chrome/Edge profile, the local authenticated Studio and startup checks.

## Install on the existing workspace

Run from the repository root in PowerShell:

```powershell
.\windows-launcher\Install-DesktopShortcut.ps1 -WorkspaceRoot 'C:\path\to\PXLBLZ_IDE--2--ArtNet'
```

The installer creates **PXLBLZ-IDE - Fadecandy** on the Windows desktop. It copies
the launcher into `%LOCALAPPDATA%\PXLBLZ-IDE` and writes the workspace location to
`launcher-config.json`. The path is machine-specific and is never committed.
Existing browser profile data is preserved. Installation does not start servers.
The installed files are identical to the versioned launcher files.

Double-click the shortcut to check and start the stack:

| Component | Address | What is verified |
| --- | --- | --- |
| Local API and D1 | localhost:5174 | `__identity` matches `PXLBLZ-IDE-main` |
| IDE with output adapter | localhost:5175 | `__identity` matches `PXLBLZ-IDE`; served Preview includes `createExternalPixelOutput` |
| Fadecandy USB server | 127.0.0.1:7890 | Fadecandy protocol replies and a USB controller is attached |
| Fadecandy bridge | 127.0.0.1:9981 | Versioned executable path, configuration and live process arguments |
| Local account | both IDE API routes | Signed `github:local-dev` session is accepted |

Missing services start in the background; occupied ports belonging to a different
installation are reported, not killed. `-CheckOnly` checks without starting services
or opening the browser; `-NoOpen` may start services but does not open a browser.
Failures appear in a message box and in `fadecandy-start-status.txt`.

The normal browser profile is not modified. A dedicated profile lives at
`%LOCALAPPDATA%\PXLBLZ-IDE\BrowserProfile`. A short-lived loopback server sets an
HttpOnly local session cookie and redirects to the output-enabled Studio.
A new start opens GyroidGlow3D with `pxout=1` and the port-9981 output target.

When that profile already has an output connection, the starter opens a temporary
helper tab through the browser's normal startup path. The browser reveals its own
profile and the helper tab closes itself, returning to the existing IDE. It never
loads a second output-enabled Pattern. If automatic tab closing is prevented, the
helper page explains that it can be closed manually.
This does not close unrelated IDE windows. An older output tab outside this profile
can still connect: use only one actively rendering output tab to avoid mixed frames.

## Restore the workspace on another computer

Use this layout (the folder names currently form part of the launcher contract):

```text
workspace/
  PXLBLZ-IDE-main/              upstream checkout, local API and .wrangler/state
  PXLBLZ-IDE/                   same upstream revision plus saved output source
  2-FadeCandy-/
    pxlblz-artnet-bridge/       this repository at tag fadecandy-v0.1.1
    pxlblz-fadecandy-0.1.1.exe  copied by the installer from the versioned binary
    fcserver.exe               pinned third-party executable
```

1. Install Node.js 24 or newer and Git. Use Chrome or Edge.
2. Restore or clone this repository into `2-FadeCandy-/pxlblz-artnet-bridge` and
   check out `fadecandy-v0.1.1`.
3. Obtain both IDE folders from
   `https://github.com/jon-whiteroomsoftware/PXLBLZ-IDE.git` at
   `d685125b34c694f311972e258efb48d12cf05cd8`. The second can be a Git worktree.
4. Copy `pxlblz-integration/snapshots/fadecandy-v0.1.1/Preview.tsx` to
   `PXLBLZ-IDE/src/components/Preview.tsx` and `externalPixelOutput.ts` to
   `PXLBLZ-IDE/src/engine/externalPixelOutput.ts`. Keep the API checkout unchanged.
   `manifest.json` records SHA-256 hashes. This reproduces the live source exactly;
   it does not require the patch installer to match a later upstream revision.
5. Run `npm ci` in both IDE folders. On an existing computer retain its canonical
   `PXLBLZ-IDE-main/.dev.vars` and `.wrangler/state`. On a new computer configure a
   new local SESSION_SECRET, run the upstream local database migrations, and provision
   the upstream `github:local-dev` identity using `scripts/dev-runtime-auth.ts`.
   No GitHub/Google OAuth credentials are needed for the local developer session.
   This repository does not contain your database or logins; a fresh setup has no
   personal Patterns/Shows until you restore them separately.
6. If fcserver is missing, run `fadecandy/install-fcserver.ps1`. The desktop installer
   copies that result into `2-FadeCandy-/fcserver.exe` and verifies its hash against
   the snapshot manifest. Connect the Fadecandy board over USB.
7. Run `Install-DesktopShortcut.ps1 -WorkspaceRoot <workspace>` and open the shortcut.

The exact previously used helper scripts are preserved in `fadecandy/local-tools`.
They are archival sources, not the new entry point. To use them in their original
form, copy them back into `2-FadeCandy-/` so their original relative paths resolve.

## Privacy and backup scope

Versioned: launcher source and installer, browser-profile creation rules, desktop
shortcut definition, bridge source/tests and binary, cube configuration, modified
IDE source, upstream revision, local helper tools and validation notes.

Kept local: BrowserProfile contents/cookies/history, SESSION_SECRET, `.dev.vars`,
`.wrangler/state`, local configuration, logs, dependencies and transient backups.
The `.lnk` is recreated by the installer rather than committing machine-specific
binary links. This tag is a source/runtime backup, not a backup of personal content.

## Validation and current limits

See [version notes](../docs/versions/fadecandy-v0.1.1.md). The bridge remains preview-
driven: a paused/background browser can stop sending frames, and the bridge holds
the last frame. Preview brightness is not applied to the hardware RGB adapter.
Per-pattern named presets are not implemented in this saved version.
