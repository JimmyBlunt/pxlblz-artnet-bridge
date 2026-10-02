# FastLED Patterns in PXLBLZ-IDE: Design

Status: design only, no product code changed. Written 2026-10-02.
Base: upstream `origin/main` = `21b764ab` (read-only worktree `E:\PXLBLZ-ArtNet\_wt\ide-upstream-ro`, removed after the analysis).
All `file:line` references point into that commit. Art-Net output references point to the old local base `E:\PXLBLZ-ArtNet\PXLBLZ-IDE` (branch with staged `src/engine/externalPixelOutput.ts` and a `src/components/Preview.tsx` hook).

Terminology follows `CONTEXT.md`. In this repo **Pattern** means an authored source file. "Sketch" is on its *Avoid* list, so the UI and docs say **FastLED Pattern**. **Controller** means a physical Pixelblaze; the LED hardware reached through the Art-Net/Fadecandy router is called the "LED output" (router) here, never "Controller". **Control** means a preview widget.

---

## 0. Kurzfassung (summary)

| Topic | Decision |
|---|---|
| Marking a Pattern as FastLED | A **source marker** on the first line: `// @pxlblz-language fastled`. No D1 column, no migration. The marker travels with `src` through every existing save, autosave, clone and navigation path unchanged. `#include <FastLED.h>` is used only as a hint ("Convert to FastLED Pattern?") when C++ is pasted or imported. |
| Editor | Same `Editor`/`PixelblazeCodeEditor`. `editorFlavor` stays `'pattern'`. The *language* is derived from the buffer (`patternLanguage(source)`). Monaco switches to its built-in `cpp` language. Async compiler diagnostics become Monaco markers (owner `fastled`). A static FastLED API catalogue drives completion and hover. |
| Preview | A new `FastLedPatternHandle` implements `PatternHandle`. `beforeRender(delta)` calls `host.frame(delta)`. `render*(index)` calls `rgb(byte/255)` for wire byte (L2) or `leds[]` (L1) at `index`. The renderer, var watcher, FPS readout and Art-Net `paintPacked` stay unchanged. Async compile goes through a new `fastledStore`. Only a successful compile *publishes* `previewSource`, so "last working preview" behaves exactly as for Pixelblaze. |
| Controls | FastLED `UISlider`/`UICheckbox`/`UINumberField` map to `PatternMetadata.controls` (`slider…`/`toggle…`). A new optional `range` field carries min/max/step. Values persist per Pattern in the existing, currently unused `PatternRecord.controls` / `controls_json` column. |
| Pixel count | `leds[i]` ↔ map index `i`. The compile service always passes `-DPXLBLZ_NUM_LEDS=<layout pixelCount>`. The new-Pattern template uses it, so template-based Patterns follow the map. Sources with a hard-coded `NUM_LEDS` keep it, and the preview warns about the mismatch and offers "Use N pixels". |
| Shows | **Real Show integration is not reasonable** (estimate 6–10 weeks). It would break the core invariant "a Show compiles into one portable Pixelblaze Pattern". Recommended instead: a browser-side **Playlist** that sequences Pixelblaze and FastLED Patterns (optionally Shows) with cuts and crossfades on one map, sent to the router. MVP about 5–7 dev-days. |
| Exclusions | Send to Controller (Run/Save), background saved-artifact reconciliation, Show pattern pickers and compile, agent pattern discovery, `.epe` download/copy, Precise fidelity. Exact places in §7. |
| Effort | FastLED Patterns in Studio: about 8–10 dev-days. Playlist MVP: 5–7 dev-days, plus seek/checkpoints about 3 days. |

---

## 1. Architektur (architecture)

```
                       ┌──────────────────────── Browser: PXLBLZ-IDE (Studio, /studio/patterns/<id>) ────────────────────────┐
 D1 personal_patterns  │                                                                                                   │
 (src incl. marker,    │  patternStore.userPatterns ──open──► openPatternRecord (branch on patternLanguage(src))           │
  controls_json)  ◄────┤        ▲  autosave (unchanged)            │                                                       │
                       │        │                                  ▼                                                       │
                       │  editorStore.source ◄── Monaco (language = cpp | pixelblaze) ◄── fastledProviders (catalog)       │
                       │        │                    ▲ markers owner 'fastled'                                             │
                       │        │ debounce 600–800ms │                                                                    │
                       │        ▼                    │                                                                    │
                       │  fastledStore.requestCompile(source, defines) ──fetch──► FastLED compile service                 │
                       │        │   status/diagnostics/artifactVersion             127.0.0.1:<port>  POST /compile          │
                       │        │   (on ok + still current) setPreviewSource(src)  ◄── {wasm, diagnostics, meta}           │
                       │        ▼                                                                                          │
                       │  fastledArtifactCache  (key = sha256(source + defines) → WebAssembly.Module + meta)               │
                       │        │                                                                                          │
                       │  Preview.tsx effect (previewSource is FastLED)                                                    │
                       │     resolveLayout(nativeDim from @pxlblz-dim) → pixelCount ─┐                                     │
                       │     await host.instantiate(module); host.init(seed)          │                                     │
                       │     createFastLedPatternHandle(host, {pixelCount, output})   │                                     │
                       │        │  PatternHandle: beforeRender(Δ)=frame(Δ); render(i)=rgb(L2[i]/255)                       │
                       │        ▼                                                                                          │
                       │  createRenderLoop (unchanged) ── fast shim capture ── paint / paintPacked                         │
                       │        │                                         │                                                │
                       │        ▼                                         ▼                                                │
                       │  WebGL renderer (preview brightness only)   externalPixelOutput.sendPacked(frame)  (byte = v*255) │
                       └──────────────────────────────────────────────────────┬────────────────────────────────────────────┘
                                                                              │ ws://127.0.0.1:9980/pixels (RGB888 frames)
                                                                              ▼
                                                                     pxlblz-router ──► Art-Net / Fadecandy
```

Invariants this design keeps:
- Hardware-bound Patterns stay plain Pixelblaze code. FastLED Patterns are never pushed to a Controller.
- `PatternHandle` (`src/engine/loadPattern.ts:54-72`) and `createRenderLoop` (`src/engine/renderLoop.ts:53`) need no interface change.
- Personal content stays in D1. The only localStorage use is session preferences such as the compile-service URL.
- Main-thread execution. Like `new Function()` Patterns, a runaway `loop()` could freeze the tab, so the wasm host must enforce an iteration/time budget (§9, R3).

---

## 2. Persistenz und Marker (Q1)

### 2.1 How a Pattern is stored today

- D1 table: `migrations/0001_personal_storage.sql:23-35`, which defines `personal_patterns(user_id, id, name, src, controls_json DEFAULT '{}', params_json, settings_json, created_at, updated_at)`. `migrations/0018_pattern_attribution.sql:1` adds `authors_json`.
- Record type: `src/engine/personalContentRecords.ts:4-17`. `PatternRecord { id, name, src, controls, authors?, updatedAt, params?, settings? }`.
- D1 codec: `src/cloudflare/patterns.ts:25-36` (row → record), `:54-89` (insert), `:91-116` (sparse update; `controls` is written to `controls_json` at `:101`).
- Worker routes: `src/worker/routes/patterns/index.ts`, plus `src/worker/routes/patterns/[id].ts:21-33`. PATCH passes `Partial<PatternRecord>` straight to `updateD1Pattern`.
- Client: `src/engine/remotePersonalContentProvider.ts:27-50` is plain JSON pass-through. The `PersonalContentProvider` interface is at `src/engine/personalContentProvider.ts:35-72`.
- Store: `src/store/patternStore.ts:113-155` (`addPattern`, `updatePatternSrc`, which also extracts authors from `src`).
- Records are created at:
  - `PatternList.tsx:639` (new, `NEW_PATTERN_SRC`)
  - `PatternList.tsx:264-272` (.epe import)
  - `App.tsx:732-759` (`handleForkDemo`, copies `src: source`)
  - `savedProgramImport.ts:108`
  - `workspaceStarters.ts`
- Every path either copies `src` verbatim or creates it from a template.
- **Finding:** `PatternRecord.controls` is always written as `{}` (`PatternList.tsx:269,639`, `App.tsx:744`). Nothing in the app persists live Control values (`grep controlValues` → only `ControlsPanel.tsx`, `Preview.tsx`, `controlStore.ts`). The column exists and round-trips but is unused. FastLED can use it (§4).

### 2.2 Options

| Option | Round-trip safety | Invasiveness | Problems |
|---|---|---|---|
| **A. First-line marker `// @pxlblz-language fastled`** | Rides in `src` through create, autosave (`autosaveSync.ts:249-258`), navigation flush (`navigationPreflightStore.ts:145`), fork (`App.tsx:741`) and D1. Nothing can drop it. | Zero schema/API change. | A user can delete it and convert the Pattern back. That is acceptable as an explicit act. |
| B. `#include <FastLED.h>` detection | Same as A. | Zero. | Implicit. Deleting or commenting the include while editing flips the language and Monaco mode mid-edit. Incomplete sources (no include yet) cannot be FastLED. |
| C. New column `language` (migration 0030) | Every record constructor and copy path must carry it, about 6 sites today and more later. A missed site silently turns C++ into a "Pixelblaze" Pattern. | Migration (local and remote), `cloudflare/patterns.ts`, routes, provider, record type, tests. | Highest risk for the least benefit. |

**Decision: A.** Detection rules for `patternLanguage(src)` in the new `src/engine/patternLanguage.ts`:
- Scan only the leading comment block (lines before the first non-comment, non-blank line, capped at 20 lines) for `^\s*//\s*@pxlblz-language\s*[:=]?\s*fastled\b` (case-insensitive). If found, the result is `'fastled'`, otherwise `'pixelblaze'`.
- Pixelblaze sources never start with `#include`. A Pixelblaze source that merely *mentions* FastLED in a comment does not match because the directive syntax is specific.
- Optional directives in the same block, all of which round-trip in `src`:
  - `// @pxlblz-dim 1|2|3`: native dimension for default layout, rail lens and dim pill. Default 1.
  - `// @pxlblz-output wire|leds`: L2 wire bytes (default, what the user asked for) or L1 logical `leds[]`.
- `looksLikeFastLedSource(src)` (`/^\s*#\s*include\s*[<"]FastLED\.h[>"]/m`) is only a hint. When the buffer of a Pixelblaze Pattern has it and no marker, the editor shows an inline action "Convert to FastLED Pattern" that prepends the marker. It is never applied silently.

---

## 3. Editor (Q2)

### 3.1 Today
- `src/components/Editor.tsx`
  - Autosave tick every 4 s via `flushPendingAutosave` (`:48-54`). It saves only when `compileStatus === 'good'` (`autosaveSync.ts:68`).
  - `handleChange` (`:61-83`) debounces 600 ms and publishes `setPreviewSource` only if flavor `pattern`, same Pattern, same source, `compileStatus === 'good'`.
  - The validation effect (`:89-135`) runs the synchronous validator per flavor (`validateSource` for Patterns, `validate.ts:3-8`, `ParseError {message, line 1-based, column 0-based}`). It sets `compileStatus` and writes Monaco markers with owner `'pixelblaze'` (`:119-134`).
- `src/components/PixelblazeCodeEditor.tsx`
  - `language={flavor === 'map' ? 'javascript' : PIXELBLAZE_LANG_ID}` (`:99`).
  - `beforeMount` registers the Pixelblaze language and providers (`:56-68`).
  - Monaco loads through `@monaco-editor/react`'s default CDN loader (`package.json:203`, no local `monaco-editor` import), so the **full Monaco build with the built-in `cpp` Monarch grammar is already available**.
- `src/components/monaco/pixelblazeLanguage.ts:21-119` (Monarch + theme + `registerProviders`) and `providers.ts:14-157` (completion/signature/hover from `BUILTIN_FUNCTIONS`, library doc index).
- Dirty/save state: `editorStore.ts:24-96` (`source`, `bufferEdited`, `compileStatus`, `previewSource`, `previewUnavailableReason`), `SaveStatusBadge`, `CompileStatusBadge.tsx` (binary tone).

### 3.2 Plan
- **Do not add an `EditorFlavor`.** A FastLED Pattern is a Pattern for persistence, autosave, navigation preflight and routes. All 12 `editorFlavor === 'pattern'` checks stay valid. The language is a derived value: `const language = editorFlavor === 'pattern' ? patternLanguage(source) : null`.
- `PixelblazeCodeEditor`
  - New prop `language?: 'pixelblaze' | 'fastled'`. Map it to Monaco `'cpp'` when FastLED (`:99`). `@monaco-editor/react` calls `setModelLanguage` on prop change, and the model, undo stack and value stay the same.
  - In `beforeMount`, also call `registerFastLedProviders(monaco)`, guarded so it registers once.
- `Editor.tsx` validation effect, FastLED branch:
  - Clear `'pixelblaze'` markers.
  - Call `useFastLedStore.getState().requestCompile(source)`, which is debounced in the store.
  - Do **not** set `compileStatus` synchronously. The store sets it when a compile for exactly this source finishes: `'good'` on success or warnings, `'broken'` on errors.
  - While compiling, the previous status stays and `CompileStatusBadge` shows `tone='working'` (the existing `StatusTone`, `StatusDot.tsx:17`).
  - Stale `'good'` during a compile only means autosave may save unverified source. That is harmless, and the navigation flush saves broken source anyway (#818).
- `Editor.tsx` `handleChange` debounce: add `patternLanguage(value) === 'pixelblaze'` to the publish condition (`:72-79`). For FastLED, *only the store publishes* after a successful compile of the still-current source.
- Markers: the store exposes `diagnostics: {line, column, endLine?, endColumn?, severity, message}[]` keyed to `forSource`. A small effect in `Editor.tsx` writes them with `monaco.editor.setModelMarkers(model, 'fastled', …)` when `forSource === source`, using the same column math as `:123-131` (service columns are 1-based, so do not add 1). Diagnostics located in FastLED headers are attached to the sketch line the service reports as the instantiation point, or to line 1 with the message prefixed by `file:line`.
- Read-only branch (`:101-105`): FastLED stock examples do not exist in v1, so nothing changes.
- **Completions and hover (cheap, yes):**
  - New `src/engine/fastled/fastledApiCatalog.ts` holds a static array `{name, kind: 'function'|'macro'|'type'|'constant'|'method', signature, doc}` of about 150–250 entries.
  - Contents: lib8tion (`sin8`, `cos8`, `beatsin8/16`, `beat8`, `scale8`, `qadd8`, `qsub8`, `random8/16`, `ease8InOutQuad`, `lerp8by8`…), colorutils (`fill_solid`, `fill_rainbow`, `fill_gradient_RGB`, `fadeToBlackBy`, `nscale8`, `blur1d/2d`, `nblend`, `ColorFromPalette`, `blend`), `CRGB`/`CHSV` members and named colors, `FastLED.*` (`addLeds`, `show`, `clear`, `setBrightness`, `setCorrection`, `setTemperature`, `setDither`, `setMaxPowerInVoltsAndMilliamps`, `delay`), macros (`EVERY_N_MILLISECONDS`, `EVERY_N_SECONDS`, `EVERY_N_MILLISECONDS_I`), palettes (`RainbowColors_p`, `PartyColors_p`, `HeatColors_p`, `CloudColors_p`, `LavaColors_p`, `OceanColors_p`, `ForestColors_p`), chipsets/orders (`WS2812B`, `GRB`…), UI (`UISlider`, `UICheckbox`, `UINumberField`, `UIButton`, `UIDropdown`).
  - Generate it once with `scripts/generate-fastled-api-catalog.ts`, which reads the pinned FastLED tree in `E:\PXLBLZ-ArtNet\fastled-work` (`///` Doxygen briefs), and commit the output. Hand-editing is fine too.
  - `src/components/monaco/fastledProviders.ts` registers completion and hover for `'cpp'`, mirroring `providers.ts:20-109`, plus member completion after `FastLED.` and `leds[i].`. Signature help is optional.
- Header pill: `App.tsx:1452` renders `<DimPills dims={exportedDims(source)} />`. For FastLED, render a `FastLED` pill plus the `@pxlblz-dim` value instead.

---

## 4. Preview-Pfad (Q3)

### 4.1 Today
`src/components/Preview.tsx`, main effect `:157-380`, keyed on `previewSource`, map/shape/surface/pixelCount, fidelity and libraries:
1. `bundle(previewSource, libraries)` (`:174-181`), then `nativeDimension(metadata.renderFns)` (`:183-184`).
2. `resolveLayout(...)` (`:200-215`) gives `mapPoints`, `pixelCount`, draw positions.
3. `selectRenderCompatibility(layout.mapDim, metadata.renderFns)` (`:231`).
4. Shim: `fidelity === 'fast' ? createShim : createFxShim` (`:248-259`). `rgb()` stores floats in the capture slot (`shim.ts:159-161`). `writeCapturedPixel` copies them into the packed frame (`shim.ts:136-141`).
5. `loadPattern(code, metadata, shim.builtins)` (`:261-264`). Then `setPatternVars`, `setControls`, and control defaults read from exports (`:265-300`).
6. `createRenderer`, then `createRenderLoop({ handle, shim, clock, mapPoints, pixelCount, renderCompatibility, …, paint, onFrame })` (`:345-372`). `onFrame` feeds elapsed time and the var watcher from `handle.getExports()` (`:355-371`).
7. Control changes are forwarded to `handle.controls[name](value)` (`:395-409`).
8. Overlays: runtime error, "Last working preview" (`:104-112`, `:632-641`), "Preview unavailable" (`:642-658`).

Render loop: `advanceBeforeRender` calls `handle.beforeRender(encode(speed*Δ))` once per frame (`renderLoop.ts:93-100`), then `render*` per pixel and capture (`:111-149`). `renderPreviewFrame()` is `doTick(0)`, i.e. `beforeRender(0)` (`:218-224`).

Art-Net hook (old base, `git diff --cached -- src/components/Preview.tsx`):
- `createExternalPixelOutput(pixelCount)` only when `pixelCountCap === null`.
- A `paintPacked` wrapper calls `renderer.paint(frame, …)` and then `externalPixelOutput.sendPacked(frame)`.
- `close()` runs on effect cleanup.
- `sendPacked` converts `Math.round(v*255)` (`externalPixelOutput.ts:91-95, 336-338`) **before** any preview brightness, because brightness is applied inside `renderer.paint`. So for `v = b/255` the router receives exactly `b`.

### 4.2 `FastLedPatternHandle`

New file `src/engine/fastled/fastledPatternHandle.ts` (pure TS, no React):

```ts
export interface FastLedHostLike {           // subset of fastledWasmHost.ts (contract, may change slightly)
  init(seed: number): void
  frame(dtMs: number): void                  // advance virtual clock, run loop() until next FastLED.show()
  getLeds(): Uint8Array                      // L1, RGB888, length 3*ledCount
  getWire(): Uint8Array                      // L2, MUST be RGB-ordered (see §9 R1)
  ledCount(): number
  getUi(): FastLedUiElement[]                // {id, name, type, min?, max?, step?, defaultValue, value}
  setUi(id: number, value: number): void
  millis?(): number; showCount?(): number    // optional, for the var watcher
}

export function createFastLedPatternHandle(opts: {
  host: FastLedHostLike
  rgb: (r: number, g: number, b: number) => void   // shim.builtins.rgb of a FAST shim
  output: 'wire' | 'leds'
  controls: FastLedControlBinding[]                // from fastledControls.ts
}): { handle: PatternHandle; metadata: PatternMetadata }
```

Semantics:
- `beforeRender(delta)`:
  - If `delta > 0`, call `host.frame(delta)`.
  - If no frame has been produced yet (first call, typically `renderPreviewFrame()` with 0), call `host.frame(0)` once.
  - `delta <= 0` after that means no new FastLED frame. A paused repaint re-emits the cached frame instead of running `loop()` again. Speed scaling already happened in the loop (`renderLoop.ts:94`), so FastLED sees the preview speed.
  - Then cache `buf = output === 'wire' ? host.getWire() : host.getLeds()` and `n = host.ledCount()`. The view may be re-created after `memory.grow`, so re-read it every frame.
- `render(index)`, `render2D(index, …)`, `render3D(index, …)`: all three ignore coordinates.
  - `i = index | 0`.
  - If `i < n`: `rgb(buf[3i]/255, buf[3i+1]/255, buf[3i+2]/255)`. Otherwise `rgb(0, 0, 0)`.
  - Division by 255 and `Math.round(v*255)` round-trip exactly in float64 (bytes 0..255).
- `metadata.renderFns = { hasBeforeRender: true, hasRender: true, hasRender2D: true, hasRender3D: true }`. `selectRenderCompatibility` then always picks the exact-dimension slot, so no adaptation note appears (`renderCompatibility.ts:63-77`). The native dimension for layout defaults comes from `@pxlblz-dim`, **not** from `nativeDimension(renderFns)`.
- `getExports()`: the var watcher surface is `{ ledCount, millis, frames, <uiName>: value … }`. `metadata.patternVars` lists the same keys. This is meaningful because the user sees UI values and the virtual clock. C++ globals are not introspectable in v1.
- `getRuntimeState`, `getPatternFunctions`, `setPatternFunction`, `setPatternVar`, `setRuntimeVar` return `{}`/`false`. They are only used by Show fast replay, which never sees FastLED (§6).
- `controls[exportName] = (v) => host.setUi(id, denormalize(v))`. The next frame uses the new value. When paused, `Preview.tsx:406-408` repaints, which re-emits the cached frame, so the value takes effect at the next running frame. That matches FastLED semantics.

Preview integration, the FastLED branch inside the same effect:
1. `if (patternLanguage(previewSource) === 'fastled')`:
   - `nativeDim = parseFastLedDirectives(previewSource).dim`, then `setNativeDim`.
   - `resolveLayout` runs exactly as today (`:200-215`). Map choice works unchanged.
2. `defines = { PXLBLZ_NUM_LEDS: layout.pixelCount, PXLBLZ_WIDTH?, PXLBLZ_HEIGHT? }`. Width and height come from `layout.layoutLabel` when it reports a regular grid.
3. `artifact = fastledArtifactCache.get(previewSource, defines)`:
   - Missing: call `useFastLedStore.getState().requestCompile(previewSource, defines, { forPreview: true })` and return. The overlay shows "Compiling FastLED Pattern…". Store completion bumps `artifactVersion`, which is a **new effect dependency**, and the effect re-runs.
   - Error: show the unavailable overlay (no last-good exists on open) and return.
4. **Always use the fast shim** (`createShim`), ignoring `fidelity`, because Precise 16.16 is meaningless for C++ output. Disable the fidelity selector in `PreviewDeck.tsx:441` with the hint "FastLED Patterns render natively".
5. `host = await instantiateFastLedHost(artifact.module)`. The effect becomes async-aware with a `cancelled` flag set in the cleanup. Instantiation from an already compiled `WebAssembly.Module` is fast. Use the async API: Chrome restricts synchronous compile/instantiate of large modules on the main thread.
6. `host.init(seed)` (seed fixed per Pattern, e.g. hash of the Pattern id, so reloads are reproducible). Apply persisted control values with `setUi` **before** the first frame.
7. `{ handle, metadata } = createFastLedPatternHandle(…)`. Then `setPatternVars`, `setControls`, and `resetControls(defaults ∪ persisted)`.
8. Renderer, `paint`/`paintPacked`, `createRenderLoop` and the external output run exactly as in the Pixelblaze path, so this code should be shared, not duplicated. **Refactor first:** extract steps 6–7 of §4.1 (renderer creation, paint wrappers, loop creation, external output) into a local function `startLoop(handle, shim, layout, renderCompatibility)` used by both branches.
9. LED count mismatch: if `host.ledCount() !== layout.pixelCount`, publish a notice through `editorStore.setRenderAdaptation(…)`, the existing one-line readout channel (`Preview.tsx:242`), e.g. "Pattern drives 144 LEDs; map has 256 — extra pixels stay black". See §5.

Async compile and last-good behaviour, mapped to the existing contract ("executable preview source remains the last valid publication", `CONTEXT.md` *Active document*):
- `previewSource` changes only through:
  - (a) `openPatternRecord`, i.e. the persisted text, compiled on demand by Preview;
  - (b) `fastledStore` publishing after a successful compile of the **still current** buffer (`source === compiled && activePatternId === requestedFor`).
  
  Broken edits therefore never replace the running preview, and `showsLastWorkingPreview` (`Preview.tsx:104-112`) works unchanged because its inputs (`compileStatus`, `editorSource !== previewSource`) behave the same.
- Superseded compiles: the store keeps `requestSeq`. Results whose sequence is not latest only fill the cache. An `AbortController` cancels the fetch. The service should cancel too.
- Service unreachable (`fetch` rejects, or `GET /health` fails):
  - The store sets `serviceStatus: 'offline'`.
  - The preview overlay reads "FastLED compile service not reachable on 127.0.0.1:<port>", with the service start hint.
  - `compileStatus` is left untouched so autosave keeps working.
  - The store retries with backoff.
- Opening a FastLED Pattern (`src/store/openPattern.ts:13-29`):
  - Skip `validateSource`.
  - `setCompileStatus('good')` (provisional).
  - `setPreviewSource(record.src)` (empty → `setPreviewUnavailable('empty-source')` as today).
  - Kick `requestCompile(record.src)`.
  - If that first compile fails and `previewSource === record.src`, call `setPreviewUnavailable('broken-source')` and set `compileStatus` to `'broken'`. This reproduces the "reopening saved unexecutable source makes the preview explicitly unavailable" rule.

Art-Net: output works automatically once the `pxlblz-artnet-output` changes are rebased onto upstream (`externalPixelOutput.ts` plus the `paintPacked` hook). FastLED adds nothing. Preview brightness affects only the WebGL canvas, so the router gets FastLED's exact bytes. **Verify in a test** that `renderer.paint` does not mutate the packed frame (§8).

---

## 5. Controls (Q4) and Maps / pixel count (Q5)

### 5.1 Controls today
- Derived in `bundle.ts:116ff` from exported `sliderX`/`toggleX`/`hsvPickerX`/`rgbPickerX` functions into `PatternMetadata.controls` (`loadPattern.ts:22-38`).
- Rendered by `ControlsPanel.tsx`: slider fixed `min 0 / max 1 / step 0.01` (`:101-111`), toggle `0|1` (`:114-127`), color pickers (`:129-147`).
- Values live in `controlStore` (`controlStore.ts:1-28`). Defaults come from the Pattern's initialised vars on each loop rebuild (`Preview.tsx:275-300`) and are forwarded through `handle.controls` (`:395-409`). They are **not persisted** (see §2.1).

### 5.2 FastLED mapping (`src/engine/fastled/fastledControls.ts`)

| FastLED UI | Control kind | exportName | Value domain in controlStore | Notes |
|---|---|---|---|---|
| `UISlider(name, value, min, max, step)` | `slider` | `slider` + PascalCase(sanitized name), deduped | raw value with `range: {min, max, step}` | `step <= 0` means continuous: use `(max-min)/1000` |
| `UINumberField(name, value, min, max)` | `slider` | as above | raw, `range` | v2: dedicated number field |
| `UICheckbox(name, value)` | `toggle` | `toggle…` | `0/1` | |
| `UIButton`, `UIDropdown`, `UITitle`, `UIDescription`, `UIHelp` | not mapped in v1 | — | — | v2: momentary button, `DeckSelect` (exists: `DeckSelect.tsx`), text in the Controls hint |

- `PatternMetadata.controls[n]` gains an optional `range?: { min: number; max: number; step?: number }` in `loadPattern.ts:22-38`. Pixelblaze controls never set it, so they behave exactly as before.
- `ControlsPanel.renderSlider` (`:100-111`): `min={c.range?.min ?? 0} max={c.range?.max ?? 1} step={c.range?.step ?? 0.01}`, and `presentation` becomes number instead of percentage when `range` is present. The default fallback `0.5` becomes `c.range ? c.range.min : 0.5`.
- `previewPanel.describeControlsReadout` (`src/engine/previewPanel.ts:23`) formats ranged values raw.
- The label is the original UI name (`label: el.name`). `description` stays empty.
- **Persistence:** a new `patternStore.updatePatternControls(id, controls)` sparse-merges into `PatternRecord.controls` and PATCHes `{ controls }`. The route already passes it through: `[id].ts:26-32` → `patterns.ts:101`.
  - Preview subscribes to `controlStore` and, for FastLED Patterns only, debounces (500 ms) persistence of `{ [uiName]: value }`. Key by UI *name*, not by id, so ids are free to change between compiles.
  - On load: `defaults = UI defaults`, overridden by `record.controls[uiName]` when present and inside the range. Then `setUi` before the first `frame` and `resetControls(merged)`.
  - Clamping and ignoring unknown keys keeps stale values harmless.
  - Pixelblaze Patterns keep today's non-persistent behaviour. Extending persistence to them is a separate product decision.
- Determinism: UI values are inputs. A replay or playlist must apply the same values at t=0. The Playlist stores per-entry overrides (§6.3).

### 5.3 Maps and pixel count
- The map decides pixel count and positions: `resolveLayout` → `layout.pixelCount`, with the per-Pattern override in `Settings.pixelCount` (`settings.ts:19-34`) via the cascade. Positions only affect where dots are drawn.
- **Policy:**
  1. Index identity: map index `i` ↔ `leds[i]`, where `leds` is all `addLeds` controllers concatenated in registration order (host contract).
  2. The compile service **always** receives `-DPXLBLZ_NUM_LEDS=<layout.pixelCount>` (plus `PXLBLZ_WIDTH`/`PXLBLZ_HEIGHT` for regular 2D grids). The template uses `#ifndef PXLBLZ_NUM_LEDS … #define NUM_LEDS PXLBLZ_NUM_LEDS`, so template-based Patterns follow the map. Changing pixel count means a recompile, cached per `(source, defines)`.
  3. Foreign sources with a literal `NUM_LEDS` keep it, and the mismatch notice appears (§4.2 step 9). Extra map pixels render black and extra LEDs are not shown. For a stock map with variable count, the notice offers **"Use N pixels"**, which calls `updatePatternSettings(id, { pixelCount: N })`. For fixed-count custom maps it only warns.
  4. The router expects a fixed frame size (`externalPixelOutput.ts:312-319`, router `pixel_count`). The frame size is the *map* pixel count, which is what the router config is based on. Keep it that way.
- New-Pattern template (`src/engine/fastled/newFastLedPattern.ts`):

```cpp
// @pxlblz-language fastled
// @pxlblz-dim 1
#include <FastLED.h>

#ifndef PXLBLZ_NUM_LEDS
#define PXLBLZ_NUM_LEDS 64
#endif
#define NUM_LEDS PXLBLZ_NUM_LEDS
#define DATA_PIN 2

CRGB leds[NUM_LEDS];
UISlider speed("Speed", 30, 1, 120, 1);

void setup() {
  FastLED.addLeds<WS2812B, DATA_PIN, GRB>(leds, NUM_LEDS);
  FastLED.setBrightness(255);
}

void loop() {
  fill_rainbow(leds, NUM_LEDS, beat8((uint8_t)speed.value()), 7);
  FastLED.show();
}
```

---

## 6. Shows (Q6): verdict and alternative

### 6.1 How Shows run today
- A Show record compiles through `compileShowForPreview` (`src/engine/showPreviewArtifact.ts:76-145`). Every Pattern instance's **Pixelblaze source** is resolved (`:94-102`, `sourceForShowPatternRef` `:198-201`). Missing refs fall back silently to `DEMOS.TestPattern1D`. Members are lowered *as Pixelblaze AST* (`showMemberLowering.ts`, `showCompiler.ts`) into **one generated Pixelblaze artifact** (`compileShowRecipeCached`, `:116`).
- Stage preview: `ShowStagePreview.tsx` creates `createFastReplayRuntime({ code: artifact.code, fxCode, metadata, … })` (`:337`, `:553`, `:772`). That is one `loadPattern` of the whole artifact (`fastReplay.ts:1-8`), driven by live `advanceLive` or deterministic 60 Hz `advanceTo`.
- Seek checkpoints snapshot the complete JS runtime state, keyed by artifact identity (`fastReplayCheckpoints.ts`, Tech Ref §24).
- Delivery (Controller Run/Save, `.epe`, `.pxlshow`) ships the same artifact. Portable compatibility, resource ledger and installation coverage all reason about Pixelblaze source (`showPreviewArtifact.ts:147-180`).

### 6.2 Verdict: real Show integration is **not reasonable**
A FastLED clip cannot be lowered into Pixelblaze code. The only route is a "foreign member" that the compiled artifact calls through a preview-only host builtin (e.g. `__ext(instanceSlot, index)` → rgb).

That would require:
1. A new member kind in the Show v2 record, codec (`showV2Codec.ts`), admission owners, history, validation and agent grammar.
2. Compiler emission plus scheduler lifecycle (start/restart/Freeze/Stutter/time-scale/Refresh) mapped to one wasm instance per Pattern instance.
3. `fastReplay` snapshot/restore extended with wasm linear-memory copies and host state. Technically doable: copy `memory.buffer`, but each snapshot is FastLED's heap.
4. Delivery and portable checks refusing such Shows everywhere.
5. Changes to the most heavily gated and mutation-qualified subsystem (`test:mutation:show-authoring`, Show command contracts).

It also contradicts the documented invariant "A Show saves choreography but compiles into one portable Pixelblaze Pattern" (`AGENTS.md`, Tech Ref Part 5) and the Show term in `CONTEXT.md`.

Estimate: **6–10 weeks**, high regression risk, constant rebase conflicts with an upstream that is very active in exactly these files. **Not recommended.**

(Option C from the feasibility study, a bit-exact FastLED-style library written in the Pixelblaze language, remains the only way to get FastLED-*like* content into real Shows and onto hardware Controllers. It is a separate project.)

### 6.3 Alternative: **Playlist** (browser-side sequencer), recommended
A new, explicitly browser-only concept. Proposed `CONTEXT.md` term:

> **Playlist**: an ordered, browser-only sequence of Patterns (Pixelblaze or FastLED; optionally compiled Shows) that PXLBLZ-IDE renders itself on one map and sends to the LED output (router). Entries play for a duration and join by Cut or Crossfade. Unlike a Show, a Playlist never compiles into a Pixelblaze artifact and is never sent to a Controller.
> _Avoid_: calling it a Show; "playlist" for Show choreography.

Model (`src/engine/playlist/playlistModel.ts`):
```ts
interface PlaylistRecord {
  id: string; name: string; updatedAt: number
  mapId: string | null; pixelCount: number | null      // one layout for the whole Playlist
  loop: boolean
  entries: PlaylistEntry[]
}
interface PlaylistEntry {
  id: string
  pattern: { kind: 'stock' | 'user'; id: string }      // same shape as ShowPatternRef
  durationMs: number
  transitionIn: { kind: 'cut' } | { kind: 'crossfade'; durationMs: number }
  controls?: Record<string, number | number[]>          // per-entry Control overrides
  speed?: number
}
```

Runtime (`src/engine/playlist/playlistRuntime.ts`):
- A shared **`createPatternRuntime(source, layout, deps)`** factory returns `{ handle, shim, loop }` for *either* language. It is the same code `Preview.tsx` uses after the refactor in §4.2 step 8, so the Playlist reuses `loadPattern`/`bundle` and `FastLedPatternHandle` without a second engine.
- Each runtime's `createRenderLoop` gets a `paintPacked` that copies into its own `Float64Array`. The Playlist ticks the active runtime(s) with `loop.tickFrame(dt)`.
- Mixer: outside transitions, output = the active buffer **unchanged**, so FastLED output stays bit-exact. During a crossfade, `out[k] = a[k]*(1-w) + b[k]*w` with linear `w` over the transition. Then `renderer.paint(out, previewBrightness)` and `externalPixelOutput.sendPacked(out)`.
- Pre-roll: the next entry's runtime is created and FastLED-compiled **before** its transition starts. All FastLED entries compile when the Playlist opens, and any missing artifact blocks Play with a clear message.
- Clocks: each entry starts at t=0 when it enters (Pixelblaze `createVirtualClock`, FastLED `init(seed)`), deterministic given the 60 Hz step.
- Seek (P2): rebuild the entry runtime at its start and fast-forward with `tickHeadless(1000/60)` to the offset. Optional checkpoints: Pixelblaze via the existing `fastReplay` snapshot pattern, FastLED via host `snapshot()`/`restore()` (linear-memory copy).
- Optional P3: a Show as an entry, running its compiled artifact through `createFastReplayRuntime(...).advanceLive(dt)` into the same mixer buffer, on the Playlist's map.

Persistence (least invasive, no migration):
- A settings-KV key `playlists`. Add it to `allowedSettingsKeys` (`src/cloudflare/resourceProtection.ts:78-88`).
- Add `getPlaylists()`/`setPlaylists()` to `PersonalContentProvider` (`personalContentProvider.ts:35-72`, demo-mode no-ops like `:65-66`). The remote implementation uses `GET/PUT /api/settings/playlists` (`src/worker/routes/settings/[key].ts`).
- Write limit 1.9 MB per request (`resourceProtection.ts:1`) is plenty.
- Promote to a `personal_playlists` table only if Playlists grow large or need per-row sync.

UI (MVP, no new Studio place):
- A **Playlist** `DeckSection` in the Patterns place's preview deck (`PreviewDeck.tsx`) with entry list (`PatternCombobox` reuse), duration and crossfade fields, Play/Stop/Next/Prev, Loop.
- While a Playlist plays, `Preview.tsx` swaps its single-Pattern loop for the Playlist runtime on the same canvas and external output. Add `playlistStore.playing` to the effect deps.
- A seventh Studio place (`StudioPlaceControl`, `routerStore`, rail section) is P3 and only worth it if Playlists become central.

Effort: MVP (model, KV persistence, runtime with cut/crossfade, deck UI, tests) **5–7 dev-days**, plus seek and checkpoints about 3 days, plus a Show-as-entry about 2 days.

---

## 7. Ausschlüsse (Q7): where FastLED must be refused

Use one helper, `isFastLedSource(src)` from `patternLanguage.ts`, everywhere.

| # | Place | File:line | Change |
|---|---|---|---|
| 1 | Send to Controller (Run/Save) gate | `src/components/ControllerActionRow.tsx:111` | Set the pattern subject's `deliveryBlocker` to `'FastLED Patterns run in PXLBLZ-IDE only; output goes through the LED router'` when the active source is FastLED. The existing view model (`src/engine/controllerActionRow.ts:10,107`) already renders blockers. |
| 2 | Push action (defence in depth) | `src/store/controllerStore.ts:1847-1855` (`pushActivePattern`) | Early return plus a `pushResult` error if `isFastLedSource(previewSource)`. |
| 3 | Background saved-artifact reconciliation | `src/store/controllerStore.ts:1244-1253` | `userPatterns.filter(p => !isFastLedSource(p.src))` before building `artifacts`. Otherwise a bound id could push C++ through `bundleWithPasses` (`:1260`). |
| 4 | Controller saved-program matching | `src/components/ControllerSavedProgramsPane.tsx:968-973, 1051` | Filter FastLED out of `studioPatterns` (they can never match a device program). |
| 5 | Show pattern pickers | `src/components/ShowEditor.tsx:2919-2924` (`editorPatternOptions`); `src/engine/showV2TimelineEditorModel.ts:25` (source choices) | Filter FastLED out of choice lists. Keep the full list in compile dependencies (`ShowEditor.tsx:1026, 2944`) so an existing reference fails loudly instead of falling back. |
| 6 | Show compile guard | `src/engine/showPreviewArtifact.ts:94-102` (`compileShowForPreview`) | Before the recipe: if any resolved instance source is FastLED, return `{ artifact: null, error: '"<name>" is a FastLED Pattern; Shows can only contain Pixelblaze Patterns. Use a Playlist.' }`. This covers Stage, delivery, export and agent paths through the shared function. |
| 7 | Agent pattern discovery | `src/agent/editorAdmission.ts:103-108` (`capturePatternMetadata`) | Filter FastLED out. Required: `projectAgentPatterns` (`src/engine/agentDiscovery.ts:62-80`) wraps *all* Patterns in one `try`, so a single C++ source makes `bundle` throw and discovery returns `undefined` for **every** Pattern. |
| 8 | Agent harness MCP (diagnostic, not product) | `src/agent-harness/mcp/showsServer.ts:193` | Same filter (optional, keeps the harness green). |
| 9 | Copy / Download .epe | `src/App.tsx:767-791, 1466-1467` | For FastLED: Copy copies the raw source (`stampedPatternArtifact` → `bundle` would throw). Download `.epe` is hidden (pass `undefined`). Optional "Download .ino" (raw text). |
| 10 | Precise fidelity | `src/components/PreviewDeck.tsx:441` | Disabled for FastLED (§4.2). |
| 11 | Rail dimension lens | `src/engine/dimLens.ts:16-19` (`nativeDim`) | Return `parseFastLedDirectives(src).dim` for FastLED (default would be 2). |
| 12 | Gallery / Pattern detail / stock catalogue | `galleryCatalog.ts`, `PatternDetailPage.tsx` | No change. The Gallery shows stock content only, and v1 ships no FastLED stock Patterns (stock edits fan out into census suites, `docs/agents/stock-content.md`). |
| 13 | .epe import | `PatternList.tsx:245-283` | No change (an .epe never contains FastLED). Optional `.ino` import adds the marker. |

---

## 8. Tests (Q8)

Existing conventions:
- Vitest projects in `vite.config.ts:48-82`: `node` (`**/*.test.ts`), `jsdom` (`**/*.test.tsx` plus a list), `chromium-layout`.
- Engine logic is covered heavily, components lightly, cross-layer flows with Playwright (`AGENTS.md`).
- Reference style: `src/engine/renderLoop.test.ts` (mock handle/shim/clock), `loadPattern.test.ts`, `Preview.test.tsx` (smoke), `Editor.recovery.test.tsx`, Playwright `e2e/pattern-panel.auth.spec.ts` (authenticated local runtime).

New tests:
1. `src/engine/patternLanguage.test.ts`: table tests for the marker (first line, after license comment, case, `:`/`=` forms, marker after code → not detected, Pixelblaze mentioning FastLED → pixelblaze), directive parsing, `looksLikeFastLedSource`. Round-trip: `PatternRecord` through `patternRecordFromRow`/`createD1Pattern` (`cloudflare/patterns.test.ts` style) keeps the marker byte-identical.
2. `src/engine/fastled/fastledPatternHandle.test.ts` (node, **fake host**, no wasm):
   - `beforeRender(0)` before any frame calls `frame(0)` once; later zeros do not.
   - `render(i)` emits `buf/255`.
   - `i >= ledCount` gives black.
   - `wire` vs `leds` selection.
   - Controls call `setUi` with denormalized values.
   - Exports contain UI values.
   - Run through the real `createRenderLoop` with the real fast shim and a `paintPacked` spy, then assert `Math.round(frame[k]*255) === bytes[k]` for all k.
3. `src/engine/fastled/fastledPatternHandle.wasm.test.ts` (node, **real wasm**):
   - Load a committed fixture `src/engine/fastled/__fixtures__/<demo>.wasm` (built by the engine agent's toolchain from the pinned FastLED; keep small demos) through the vendored `fastledWasmHost.ts` (`fs.readFile` → `WebAssembly.compile`).
   - Run N=600 frames at `1000/60` through `createRenderLoop` + `paintPacked`. Compare bytes to golden L1/L2 dumps produced by the native/wasm harness (`fastled-work/tests`). Criterion: 0 differing bytes.
   - Covers Fire2012, Pacifica, DemoReel100 (UI-free), and one UISlider demo.
4. `src/engine/fastled/fastledControls.test.ts`: UI → controls mapping, name sanitising/dedup, range/step, persisted-value merge with clamping and unknown keys.
5. `src/engine/fastled/fastledCompileClient.test.ts`: mocked `fetch`, covering request shape (`{source, defines}`), base64/binary wasm decode, diagnostic normalisation, abort on supersede, offline classification.
6. `src/store/fastledStore.test.ts`:
   - Only the latest request publishes `previewSource`.
   - No publication if the buffer or Pattern changed.
   - Error sets `compileStatus 'broken'` and keeps `previewSource`.
   - On-open failure sets `previewUnavailable('broken-source')`.
   - Cache hit needs no fetch.
7. Exclusion tests next to the existing suites: `controllerActionRow.test.ts` (blocker), `controllerStore.test.ts` (reconciliation filter and `pushActivePattern` guard), `agentDiscovery`/`editorAdmission.discovery.test.ts` (a FastLED Pattern no longer breaks discovery), `showPreviewArtifact.test.ts` (explicit error), `dimLens.test.ts`.
8. Art-Net bit-exactness: a renderer test asserting `paint()` does not mutate the packed frame, plus an `externalPixelOutput` unit test (fake WebSocket) that bytes equal `round(v*255)` (on the artnet branch).
9. Component (jsdom, light): `Editor` with FastLED source sets Monaco language `cpp` and writes `fastled` markers from store diagnostics; `ControlsPanel` honours `range`.
10. Playwright `e2e/fastled-pattern.auth.spec.ts`:
    - Create a FastLED Pattern from the rail menu.
    - The editor shows C++.
    - A deliberate error shows a marker and keeps "Last working preview".
    - Fixing it resumes.
    - Send to Controller is blocked.
    - The compile service is **mocked** with `page.route('http://127.0.0.1:*/compile', …)` returning the fixture wasm, so CI needs no zig.
    - The real service is checked by a manual local smoke step.
11. Playlist (later): pure runtime tests with fake handles (cut is bit-exact, crossfade math, pre-roll ordering, deterministic seek equals cold replay).

---

## 9. Risiken und Vertragspunkte für die Engine (risks)

| # | Risk | Mitigation / contract request to the engine agent |
|---|---|---|
| R1 | **Wire bytes in controller color order** (e.g. `GRB` from `addLeds<…, GRB>`) would show swapped colors in the preview and on the router, which expects RGB888. | `getWire()` must return **RGB-ordered** bytes after brightness/correction/temperature/dither/power-limit, before color-order reordering. Alternatively the host exposes the per-controller `EOrder` and the handle un-swizzles. Golden tests must check this. |
| R2 | `frame(dt)` semantics with sketches using `delay()`/`FastLED.delay()`/`EVERY_N_*` vs free-running loops. | Contract: "advance virtual time by dt, run `loop()` until the next `show()` (or until the virtual time target is reached), return". Define `dt = 0` as "no time advance" (the IDE calls it only once, for the first frame). Document how multiple `show()` per `loop()` behave. |
| R3 | Infinite `loop()` without `show()`, or long `while` loops, **freeze the tab** (main-thread wasm). | Host-side budget: max `loop()` iterations per `frame()` and/or an instrumented instruction counter in the zig build. Return with an error flag that the handle turns into a runtime error (`renderLoop` `onError`). |
| R4 | Compile latency (zig + FastLED) makes editing feel slower than Pixelblaze. | Service caches precompiled FastLED objects. IDE debounces about 800 ms, cancels superseded compiles, keeps the last good preview running, shows a `working` badge. Cache per `(source, defines)`. |
| R5 | CORS / Private Network Access: IDE origin `http://localhost:5174` (or the deployed Worker) calls `http://127.0.0.1:<port>`. | Service answers `OPTIONS` with `Access-Control-Allow-Origin` (echo the allowed origins), `Access-Control-Allow-Headers: content-type` and `Access-Control-Allow-Private-Network: true`. Add `GET /health` {version, fastledVersion} for offline detection. URL configurable like `?pxoutUrl=` (`?fastledService=` → sessionStorage). |
| R6 | ABI drift between `fastled-work` and the vendored `fastledWasmHost.ts`. | Export an `abiVersion()` from wasm. The host refuses mismatches with a clear message. Pin it in a test. Vendor the host file with a provenance header. |
| R7 | Pixel-count changes cause recompiles. | Cache. The pixel count popover commits on release. A literal `NUM_LEDS` avoids recompiles entirely. |
| R8 | Upstream churn (`Preview.tsx`, `ShowEditor.tsx` 10k+ lines, `controllerStore.ts`) means rebase conflicts. | Keep upstream-file edits small and branch-local. Most logic lives in new files (`src/engine/fastled/*`, `fastledStore.ts`). The `Preview.tsx` refactor (`startLoop` extraction) goes first as its own commit. |
| R9 | Marker deleted by accident flips the editor to Pixelblaze with a cascade of parse errors. | The detection window tolerates leading comments. Monaco undo restores it. Optional guard: if the persisted record is FastLED and the buffer loses the marker, show an inline "This Pattern is no longer marked as FastLED — restore marker?" action. |
| R10 | `controls_json` starts carrying data for FastLED Patterns, and a future upstream feature might start using it for Pixelblaze. | Keys are FastLED UI names, so any collision is harmless. Document it in the Tech Ref §3. |
| R11 | Show Stage output to Art-Net is not hooked on the old base either (only `Preview.tsx`). | Out of scope here. The Playlist uses the same `externalPixelOutput`. |
| R12 | Upstream process gates (wrsp review, UI proof, `X-Authored-Model` trailer) in `AGENTS.md`. | This is a private fork branch (`feature/fastled`, stacked on `pxlblz-artnet-output`). Follow the gates only if it is ever proposed upstream. |

---

## 10. Datei-für-Datei-Änderungsplan gegen 21b764ab

### New files
| File | Content |
|---|---|
| `src/engine/patternLanguage.ts` (+ `.test.ts`) | `PatternLanguage`, `FASTLED_MARKER`, `patternLanguage(src)`, `isFastLedSource`, `parseFastLedDirectives(src) → {dim, output}`, `looksLikeFastLedSource`, `withFastLedMarker(src)` |
| `src/engine/fastled/fastledWasmHost.ts` | Vendored from `E:\PXLBLZ-ArtNet\fastled-work\runtime` (provenance header, `ABI_VERSION`). Async `instantiateFastLedHost(module)` |
| `src/engine/fastled/fastledCompileClient.ts` (+ test) | `compileFastLed({source, defines}, signal)`, `checkFastLedService()`, URL resolution (`?fastledService=`, sessionStorage, default `http://127.0.0.1:<port>`), diagnostic normalisation |
| `src/engine/fastled/fastledArtifactCache.ts` | LRU (about 16 entries) keyed by `sha256(source + JSON(defines))` → `{ module: WebAssembly.Module, meta, diagnostics }` |
| `src/engine/fastled/fastledPatternHandle.ts` (+ `.test.ts`, `.wasm.test.ts`) | §4.2 |
| `src/engine/fastled/fastledControls.ts` (+ test) | §5.2 mapping, (de)normalisation, persisted-value merge |
| `src/engine/fastled/fastledApiCatalog.ts` | Static completion/hover catalogue (§3.2) |
| `src/engine/fastled/newFastLedPattern.ts` | `NEW_FASTLED_PATTERN_SRC` (§5.3) |
| `src/engine/fastled/__fixtures__/*.wasm`, `*.golden.json` | Golden fixtures from the engine harness |
| `src/components/monaco/fastledProviders.ts` | `registerFastLedProviders(monaco)` for `'cpp'` |
| `src/store/fastledStore.ts` (+ test) | `{ status: 'idle'\|'compiling'\|'ok'\|'error', serviceStatus, forSource, diagnostics, artifactVersion, lastDefines }`, `requestCompile(source, defines?, opts?)`, publication rule (§4.2), sets `compileStatus` |
| `scripts/generate-fastled-api-catalog.ts` (optional) | Catalogue generator from the pinned FastLED tree |
| `e2e/fastled-pattern.auth.spec.ts` | §8 item 10 |
| Later: `src/engine/playlist/playlistModel.ts`, `playlistRuntime.ts`, `src/store/playlistStore.ts`, `src/components/PlaylistDeckSection.tsx` | §6.3 |

### Modified files
| File | Change |
|---|---|
| `src/engine/loadPattern.ts:22-38` | Optional `range?: {min; max; step?}` on control metadata. `PatternHandle` unchanged |
| `src/components/Preview.tsx` | (1) Refactor: extract `startLoop(...)` from `:307-379`. (2) FastLED branch in the main effect (§4.2): directive dim, defines, cache lookup or compile request, async instantiate with `cancelled` guard, fast shim forced, handle, controls defaults plus persisted values, LED-count notice via `setRenderAdaptation`. (3) Deps add `fastledArtifactVersion`. (4) Overlays "Compiling FastLED Pattern…" / "FastLED compile service not reachable". (5) Debounced control persistence for FastLED. (6) After the artnet rebase: unchanged `paintPacked`/`externalPixelOutput` wiring |
| `src/components/Editor.tsx:61-135` | Language derivation; FastLED skips sync validation and calls `requestCompile`; debounce publish only for Pixelblaze; marker effect for owner `fastled`; clear the other owner on language switch |
| `src/components/PixelblazeCodeEditor.tsx:6-12, 56-68, 99` | `language` prop, `'cpp'` mapping, register FastLED providers |
| `src/components/CompileStatusBadge.tsx` | `tone='working'` while `fastledStore.status === 'compiling'` for a FastLED buffer |
| `src/store/openPattern.ts:13-29` | FastLED branch (§4.2 "Opening") |
| `src/store/patternStore.ts` | `updatePatternControls(id, controls)` (sparse merge, PATCH `{controls}`, no `updatedAt` bump, like `updatePatternSettings` `:157-171`) |
| `src/components/ControlsPanel.tsx:80-111` | Ranged sliders |
| `src/engine/previewPanel.ts:23` | Ranged readout |
| `src/components/PreviewDeck.tsx:441` | Fidelity disabled for FastLED |
| `src/components/rail/PatternsRailSection.tsx:95-98` | Menu item `{ label: 'New FastLED pattern', onSelect: onCreateFastLedPattern }` (new prop) |
| `src/components/PatternList.tsx:635-655, 1103` | `handleCreateFastLedPattern` (clone of `handleCreatePattern` with `NEW_FASTLED_PATTERN_SRC`, name `Untitled FastLED Pattern`), pass to rail section. Optional `.ino` import |
| `src/App.tsx:1452, 1466-1467, 767-775` | FastLED pill; Copy raw source; hide Download .epe |
| `src/engine/dimLens.ts:16-19` | FastLED dim |
| `src/components/ControllerActionRow.tsx:111` | `deliveryBlocker` |
| `src/store/controllerStore.ts:1244-1253, 1847-1855` | Reconciliation filter, push guard |
| `src/components/ControllerSavedProgramsPane.tsx:968, 1051` | Filter |
| `src/components/ShowEditor.tsx:2919-2924` | Filter options |
| `src/engine/showV2TimelineEditorModel.ts:25` | Filter choices |
| `src/engine/showPreviewArtifact.ts:94-102` | Explicit FastLED error |
| `src/agent/editorAdmission.ts:103-108` | Filter |
| `src/agent-harness/mcp/showsServer.ts:193` | Filter (optional) |
| `CONTEXT.md` | Terms **FastLED Pattern** (Pattern written in C++ against FastLED, marked by `// @pxlblz-language fastled`, previewed by real FastLED compiled to wasm, browser and LED-router only, never sent to a Controller), **Pattern language**, **FastLED compile service**, later **Playlist**. Extend *Pattern* ("…or, for a FastLED Pattern, C++…") and the *Send to Controller* exclusion |
| `docs/reference/PXLBLZ Technical Reference.md` Part 2 | New section "FastLED Patterns" (marker, compile service, handle, determinism, limits) |
| Later for Playlist: `src/cloudflare/resourceProtection.ts:78-88`, `src/engine/personalContentProvider.ts:35-72` (+ remote provider) | `playlists` settings key, provider get/set |

Prerequisite (separate branch, already exists on the old base): port `src/engine/externalPixelOutput.ts` plus the `Preview.tsx` `paintPacked` hook onto `21b764ab` (branch `pxlblz-artnet-output`). `feature/fastled` stacks on it.

---

## 11. Implementierungs-Checkliste (ordered; executable by another agent)

Work in your own worktree of the IDE fork (for example `E:\PXLBLZ-ArtNet\_wt\ide-fastled`, branch `feature/fastled` from `pxlblz-artnet-output` rebased on `21b764ab`). Never touch `E:\PXLBLZ-ArtNet\PXLBLZ-IDE(-main)` directly. Use npm cache `E:\PXLBLZ-ArtNet\npm-cache`.

1. **Contract sync with the engine agent** (`E:\PXLBLZ-ArtNet\fastled-work`): confirm items R1–R3, R5 and R6 in §9 (RGB-ordered `getWire`, `frame(0)`, loop budget, CORS/PNA, `/health`, `abiVersion`, `defines` in `POST /compile`, diagnostics shape `{line, column(1-based), endLine?, endColumn?, severity, message, file?}`). Get one small fixture `.wasm` plus golden L1/L2 dumps.
2. **Prerequisite:** the Art-Net output branch is rebased onto upstream and green (`npx tsc -b`, `npx vitest run src/engine/renderLoop.test.ts src/components/Preview.test.tsx`).
3. Add `src/engine/patternLanguage.ts` and its tests. Run `npx vitest run src/engine/patternLanguage.test.ts`.
4. Vendor `fastledWasmHost.ts`. Add `fastledPatternHandle.ts`, `fastledControls.ts` and node tests with the fake host, including the `createRenderLoop` + fast shim + `paintPacked` byte round-trip test.
5. Add the wasm golden test with the fixture (0 differing bytes on L1 and L2).
6. Add `fastledCompileClient.ts`, `fastledArtifactCache.ts`, `fastledStore.ts` and tests (publication rule, supersede, offline, on-open failure).
7. Commit the `Preview.tsx` refactor separately: extract `startLoop(...)` with no behaviour change. Run existing Preview and renderLoop tests.
8. Implement the `Preview.tsx` FastLED branch: async instantiate with `cancelled` guard, forced fast shim, overlays, `artifactVersion` dependency, LED-count notice, control defaults plus persisted values, debounced `updatePatternControls`.
9. Make `loadPattern.ts` `range`, `ControlsPanel.tsx`, `previewPanel.ts` and the `patternStore.updatePatternControls` changes, with tests.
10. Editor: `PixelblazeCodeEditor` `language` prop with `'cpp'`, `Editor.tsx` branch plus markers, `CompileStatusBadge` working tone, `openPattern.ts` branch.
11. Add `fastledApiCatalog.ts` (generated or curated) and `fastledProviders.ts`.
12. Rail and header: "New FastLED pattern", template, FastLED pill, `dimLens`, Copy/Download handling, fidelity disable.
13. Exclusions §7 #1–#9, with the tests listed in §8 item 7.
14. Playwright `e2e/fastled-pattern.auth.spec.ts` with the mocked service. Then a manual smoke against the real compile service and router: open the IDE with `?pxout=1`, check the router frame rate and confirm byte equality by capturing one frame at the router.
15. Docs: `CONTEXT.md` terms and Tech Ref section.
16. Full checks: `npm run lint`, `npx tsc -b --pretty false`, `npx vitest run`, focused e2e.
17. **Playlist (next milestone):** model, KV key and provider, `createPatternRuntime` factory (reuse of the step 7 extraction), runtime with cut/crossfade and pre-roll, deck section UI, tests (§8 item 11), docs and `CONTEXT.md` term. Then seek/checkpoints, then optional Show-as-entry.
