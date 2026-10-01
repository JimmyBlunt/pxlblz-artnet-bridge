# PXLBLZ IDE integration

This directory contains the shared experimental external pixel output adapter for PXLBLZ IDE.

The same PXLBLZ-side code is used for both verified native output daemons:

```text
PXLBLZ render loop
      -> one packed Float64 RGB frame
normal preview + ExternalPixelOutput
      -> one reusable RGB888 conversion
browser WebSocket
      -> ws://127.0.0.1:9980/pixels -> Art-Net router (pxlblz-router, LED installation)
      -> ws://127.0.0.1:9981/pixels -> Fadecandy/L3D output (pxlblz-fadecandy)
```

There is no canvas readback, no second pattern render and no duplicate logical mapping stage.

## Install

From the bridge repository:

```powershell
powershell -ExecutionPolicy Bypass -File .\pxlblz-integration\install-pxlblz-output.ps1 -PxlblzPath "C:\path\to\PXLBLZ-IDE"
```

`-Target` only selects the URL the installer prints; the installed code is the same:

- `artnet` (default) -> `ws://127.0.0.1:9980/pixels`
- `fadecandy` -> `ws://127.0.0.1:9981/pixels`
- `custom` -> pass `-CustomUrl "ws://host:port/path"`

The installer checks the expected PXLBLZ source anchors, creates timestamped backups of
`Preview.tsx` and `PreviewDeck.tsx`, copies `externalPixelOutput.ts` into `src/engine` and
`ExternalPixelOutputToggle.tsx` into `src/components`, wires the existing `paintPacked` render
path into the preview, adds the OUT button to the Preview header, applies
`patches/custom-map-fixed-count.patch`, keeps hardware output opt-in, and prints the exact
query string for the selected target. It reads and writes the PXLBLZ sources as UTF-8 without
BOM (also under Windows PowerShell 5.1) and can be re-run on an already patched checkout.

The installer was originally reviewed against upstream PXLBLZ
`d685125b34c694f311972e258efb48d12cf05cd8`; the integration-merge version was verified
(install twice + `tsc --noEmit`) against `21b764ab`, the revision of the local IDE in use.

## Enabling output

Output is deliberately opt-in: `?pxout=1` (plus `pxoutUrl=` for a non-default target).

Once enabled, the output flag and optional `pxoutUrl` are kept in `sessionStorage`, so
Gallery -> Studio navigation or an auth redirect in the **same browser tab** does not silently
disable hardware output. `?pxout=0` explicitly disables it again for that tab. Closing the
tab clears the state.

## Output control UI

The integration adds a compact **OUT** button to the PXLBLZ Preview header.

- gray `OUT`: external output disabled;
- green `OUT`: external output enabled;
- clicking the button stores the tab-local preference and reloads the current
  PXLBLZ route so the render loop is rebuilt deterministically;
- Gallery -> Studio navigation keeps the output preference in the same tab;
- closing the tab clears the session preference;
- `?pxout=1` and `?pxout=0` remain supported for automation and recovery;
- `pxoutUrl` remains the endpoint override.

The button reports **configured enabled/disabled state**, not controller health.
The native router telemetry remains the authority for live client connection,
RX/TX FPS, replaced frames, packet rate and send errors.

## Fixed pixel count for literal custom maps (`patches/`)

PXLBLZ stores a pixel count **per pattern**; upstream, a user's custom map does not change it.
A pattern last saved with 170 pixels therefore renders 170 pixels even on the 8683-point
installation map, and the router (exact size) drops every frame.
`patches/custom-map-fixed-count.patch` (applied by `install-pxlblz-output.ps1`) treats a
custom map whose source is a **literal coordinate array** (`[[x,y,z], ...]`) like PXLBLZ's
stock literal maps and its Show editor already do: the pixel count is fixed to the number of
points (lock symbol in the Preview deck), for every pattern. Function-source maps keep the
editable count.

## Tab title and status badge

With output enabled the Studio tab is titled **PXLBLZ-IDE~ArtNet** and a small badge at the
bottom left polls the router (`http://127.0.0.1:9988/status`, override with
`?pxoutStatus=<url>`): green = frames flowing, amber = no frames / test pattern running,
red = router not reachable. It links to the router settings page `http://127.0.0.1:9988/`.
Title and badge are only shown for Art-Net targets: with the Fadecandy URL (port 9981) they
stay off unless `?pxoutStatus=` is given.

## Art-Net router test command

Normally the desktop starter (`windows-launcher-artnet/`) starts router, local IDE and the
Studio with `?pxout=1&pxoutUrl=ws://127.0.0.1:9980/pixels`. By hand:

```bat
pxlblz-router.exe --config config\routes.installation-live.json --input ws
```

Then open PXLBLZ with `?pxout=1`.

The router should report a client and valid RX frames before Art-Net starts.

## Maps and patterns (`maps/`, `patterns/`)

| File | Content |
| --- | --- |
| `maps/esp-test-8x8-12x6.js` | ESP test rig 10.0.0.248: one APA102 chain, 8x8 matrix (px 0-63) then DotStar FeatherWing 12x6 (px 64-135), both wired from the bottom left, rows without serpentine, 8x8 right-aligned under the wing. Pixel order = router pixel order (`routes.esp-test-172-8x8-12x6.json`, and pixels 0-135 of `routes.installation-live.json`). |
| `patterns/snowflake-icesparkle-carpet-v06-esp-test.js` | SnowFlake IceSparkle carpet v0.6b for that map (`render2D`, calibration from the map). Sliders: Helligkeit, Abdeckung, Wellenlaenge, Wellental Laenge, Wellental Schwingen, Density, Glimmen Ausklang, Glimmen Kontrast, Wabber Form, featherWing. |

Paste them into the Studio, or import them into the local database with the tool below.

## Local database tools (`tools/`)

Both run from the `PXLBLZ-IDE-main` folder (they use its `node_modules` and PXLBLZ's own
functions) and only **write SQL**; apply it with wrangler. Back up `.wrangler/state` first.

- `seed-local-identity.mts` - creates the local developer identities (`github:local-dev`
  and the local agent users) with PXLBLZ's `localIdentitySeedSql()`; needed once on a new PC
  after `npm run db:migrate:local`:

  ```bat
  npx tsx <bridge>\pxlblz-integration\tools\seed-local-identity.mts seed.sql
  npx wrangler d1 execute pxlblz-ide --local --file seed.sql
  ```

- `local-d1-import.mts` - inserts or updates a custom map and/or pattern for the local user,
  stored exactly as the Studio does (map baked with `bakeMapSource()`, pattern linked to the
  map). Options: `--map <file> --map-name <name> --pixels <n>`, `--pattern <file>
  --pattern-name <name>`, `--map-id` / `--pattern-id` to update existing entries (ids are
  printed on insert), `--user` (default `github:local-dev`), `--ide`, `--out`:

  ```bat
  npx tsx <bridge>\pxlblz-integration\tools\local-d1-import.mts --map <bridge>\pxlblz-integration\maps\esp-test-8x8-12x6.js --map-name "ESP Testrig" --pixels 136 --pattern <bridge>\pxlblz-integration\patterns\snowflake-icesparkle-carpet-v06-esp-test.js --pattern-name "SnowFlake IceSparkle Carpet v0.6b - ESP Testrig" --out import.sql
  npx wrangler d1 execute pxlblz-ide --local --file import.sql
  ```

  Reload the Studio tab (F5) afterwards.

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

Then run PXLBLZ with the stock 8x8x8 cube map and 512 pixels.

Append this query string to whatever local URL PXLBLZ prints:

```text
?pxout=1&pxoutUrl=ws%3A%2F%2F127.0.0.1%3A9981%2Fpixels
```

The Fadecandy router must report:

```text
clients 1
RX > 0 fps
invalid 0/s
```

and the physical cube should match the PXLBLZ preview.

## Art-Net target

The same patched PXLBLZ build can instead point at the verified Art-Net router with:

```text
?pxout=1&pxoutUrl=ws%3A%2F%2F127.0.0.1%3A9980%2Fpixels
```

No second PXLBLZ integration is required.

## Brightness

The first prototype deliberately sends canonical raw pattern RGB from the packed render frame.

PXLBLZ preview brightness and dimmed state are not currently applied to hardware output. Brightness ownership remains a separate design decision.

## Real-time behavior

Browser WebSocket backpressure is lossy by design. If one complete RGB frame is already buffered, the next render frame is skipped rather than queued.

The native router then applies a second LatestFrame layer. This keeps output live instead of accumulating animation latency.

## Production Fadecandy interpolation

The normal Fadecandy config keeps:

```json
"interpolate": true
```

The no-interpolation config remains only as a diagnostic option for exact per-frame identity tests.
