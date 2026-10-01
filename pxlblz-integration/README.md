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

## Router test command

Normally the desktop starter (`windows-launcher-artnet/`) starts router, local IDE and the
Studio with `?pxout=1&pxoutUrl=ws://127.0.0.1:9980/pixels`. By hand:

```bat
pxlblz-router.exe --config config\routes.installation-live.json --input ws
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
