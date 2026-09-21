# Fadecandy v0.1.1 — Windows desktop snapshot

Saved on 2026-09-21, before any Pattern preset work.

## Included changes

- Variable RGB input sizes: keep the first 512 logical pixels, black-pad short
  frames, reject empty/incomplete triplets and limit WebSocket input to 16 MiB.
  This allows preview pixel counts to change without leaving stale cube pixels.
- Tests for 1, 128, 512, 2048 and 2197 pixel input, including a real masked
  WebSocket path through normalization and L3D mapping into OPC.
- Versioned `pxlblz-fadecandy-0.1.1.exe` matching the executable used locally.
  Its existing banner remains `0.1.1-dev`; this tag captures the working state.
  The generic `pxlblz-fadecandy.exe` is the existing CI-managed binary and is not
  the executable chosen by the desktop starter.
- Windows desktop installer, explicit Fadecandy naming, private browser profile,
  local signed login, identity/source/device/version checks and background startup.
- Corrected repeated-start behavior: reopen the connected browser profile through a temporary self-closing helper tab instead of silently returning. No second Pattern is opened.
- Exact modified IDE files, pinned to upstream commit
  `d685125b34c694f311972e258efb48d12cf05cd8`, plus SHA-256 manifest.
- Previous local login proxy, measurement tool, first bridge starter and live notes.

## Validation

- `go test ./...` passed for the Fadecandy module on 2026-09-21.
- PowerShell parser and Node syntax checks passed for launcher/helper sources.
- The installer was applied to the existing workspace and its desktop shortcut.
- Local session checks passed against both ports 5174 and 5175.
- The live port-5175 Preview contains the external RGB output adapter.
- Fadecandy USB detection returned firmware 1.07.
- Earlier live capture on the same day showed about 60 received frames/s and
  58–60 OPC frames/s, invalid 0/s, without output errors. This is software-path
  evidence; it does not by itself verify physical LED appearance.
- Repeated launch is checked without closing user tabs or restarting the stable API.

## Restore

Follow [windows-launcher/README.md](../../windows-launcher/README.md).
The personal D1 database, session secret and browser cookies are intentionally
not published. Preserve those separately to retain personal Patterns and Shows.
The current preview integration is saved byte-for-byte, including pre-existing
comment encoding artifacts; no source cleanup or preset implementation is included.

- After the browser-based repeated-start correction, the original input stream resumed at about 60 fps while the number of bridge clients stayed unchanged. User confirmation of window visibility remains pending.
