# Exact live IDE source

These files were copied byte-for-byte from the working Fadecandy IDE on 2026-09-21.
Use the upstream commit and destination paths in `manifest.json`. Copy only into
`PXLBLZ-IDE`, not the stable API checkout `PXLBLZ-IDE-main`.

The upstream repository contains the remaining source, package lock and licenses.
These files retain their upstream provenance; local additions connect the existing
packed preview frame to the external WebSocket RGB output.

Note (integration merge 2026-10-02): `Preview.tsx` here is byte-exact (sha256 in
`manifest.json`) and therefore still contains the BOM and the mojibake in comments
(e.g. a dash stored as three Latin-1 characters) that the old installer
(`Get-Content -Raw` / `Set-Content -Encoding utf8`) introduced in the live Fadecandy IDE.
It is kept unchanged on purpose; the current `install-pxlblz-output.ps1` no longer does this.
