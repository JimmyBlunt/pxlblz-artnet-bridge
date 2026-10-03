# FastLED-Beispielkatalog für PXLBLZ-IDE (`catalog/`)

Alle offiziellen FastLED-Beispiele (`examples/` am gepinnten Commit) als importierbare Daten für den
Beispielkatalog der IDE: unveränderte Quelltexte + Metadaten + Ergebnis eines echten Probelaufs
durch dieselbe Toolchain, die die IDE benutzt.

```
catalog/
  fastled-examples.json     Katalog (Schema unten), stabil sortiert nach upstreamPath
  examples/<id>/...         Quelltexte, byte-genaue Kopie des Upstream-Ordners (inkl. Unterordner)
  screenmaps/<id>.json      Screen-Map-Koordinaten, wenn der Sketch setScreenMap() aufruft
  LICENSE.FastLED           MIT-Lizenz von FastLED (Upstream-Datei LICENSE am gepinnten Commit)
  CATALOG_RESULTS.md        Ergebnistabelle (Zählungen, jeder Fehlschlag mit Grund)
  build-catalog.mjs         Generator;  catalog-meta.mjs  Kategorien + manuelle Overrides
  run-worker.mjs, results-md.mjs   Hilfsdateien des Generators
```

## Neu erzeugen (z. B. nach einem FastLED-Pin-Update)

```
# 1. FastLED-Checkout auf den neuen Commit (sparse: /src/, /examples/, /library.properties)
#    und Bibliotheken neu bauen
npm run build
# 2. Katalog (startet einen eigenen Compile-Dienst auf 127.0.0.1:9997 und stoppt ihn danach)
node catalog/build-catalog.mjs                 # alle Beispiele, 600 Frames, mit nativer Parität
node catalog/build-catalog.mjs --only Blink,Fx/FxCylon   # Auswahl (id oder upstreamPath)
node catalog/build-catalog.mjs --bundle-only   # nur Quelltexte/Hashes; Ergebnisse unveränderter Beispiele bleiben
#    Optionen: --frames N, --port P, --service http://127.0.0.1:9996 (laufenden Dienst nutzen),
#              --no-parity, --jobs N (Standard 1; große Sketches brauchen 1-2 GB RAM pro Compiler)
# 3. neue/umbenannte Beispiele in catalog-meta.mjs einer Kategorie zuordnen, platform/note prüfen
# 4. npm run verify (Golden-Frames) muss grün bleiben
```

`id`s sind stabil: Ordnername des Sketches (`Blink`, `FxCylon`, `ArrayOfLedArrays`); nur wenn ein Name
upstream doppelt vorkommt, wird der Pfad mit `-` verbunden (`Fx-Foo`). `upstreamPath` ist immer eindeutig.
Ein Beispiel mit unveränderten Dateien behält dieselben `sha256`/`filesHash`; die IDE kann daran
erkennen, ob ein gespeicherter Nutzer-Sketch noch dem Original entspricht.

## Schema (`schemaVersion` 1)

```ts
interface FastLedExampleCatalog {
  schemaVersion: 1;
  generatedAt: string;                       // ISO-Zeit
  generator: string;
  fastled: { version: string; commit: string; repository: string; localPatch: string | null;
             license: 'MIT'; licenseFile: 'LICENSE.FastLED' };
  toolchain: { zig: string; abiVersion: 1; frames: number; seed: number; schedule: string };
  categories: { id: CategoryId; title: string; titleDe: string }[];   // Anzeige-Reihenfolge
  summary: { total; compileOk; compileError; run: Record<RunStatus | 'not-run', number>;
             platform: { pc; hardware }; parity: { identical; diff; other }; byCategory: Record<CategoryId, number> };
  examples: FastLedExample[];
}

type CategoryId = 'basics' | 'classic' | 'color' | 'noise' | 'fire' | 'matrix' | 'fx' | 'multi'
                | 'audio' | 'hardware' | 'wasm' | 'advanced';
type RunStatus = 'ok' | 'budget' | 'crash' | 'no-leds' | 'timeout';

interface FastLedExample {
  id: string;                    // stabil, eindeutig, = Ordnername unter catalog/examples/
  title: string;                 // Sketch-Name
  category: CategoryId;
  upstreamPath: string;          // z. B. "Fx/FxCylon"
  mainFile: string;              // "<title>.ino", relativ zum Beispielordner
  description: string | null;    // @brief aus dem Sketch (englisch, upstream)
  upstreamFilter: string | null; // FastLEDs "@filter:"-Zeile (Zielplattformen upstream), z. B. "(platform is esp32)"
  files: {
    path: string;                // relativ zum Beispielordner, '/'-getrennt, kann Unterordner enthalten
    role: 'main' | 'sketch' | 'source' | 'header' | 'data' | 'doc' | 'tool';
    bytes: number; sha256: string;
    compile: boolean;            // true = gehört in POST /compile (main als `source`, Rest in `files`)
  }[];
  omittedFiles: { path: string; bytes: number; sha256: string; reason: string }[];  // > 256 KB, binär, kein Quelltext
  filesHash: string;             // sha256 über (path, sha256) aller files
  fastled: { version: string; commit: string };
  license: 'MIT (FastLED)';
  upstreamUrl: string;           // GitHub-Ordner am gepinnten Commit

  compile: { status: 'ok'; wasmBytes: number; key: string; warnings: number }
         | { status: 'error'; errors: { file?: string; line?: number; message: string }[] };
  run: null | { status: RunStatus | 'not-run'; frames: number; ledCount: number; showCount: number;
                budgetFrames: number;    // Frames mit FRAME_STATUS.BUDGET
                litFrames: number;       // Frames mit mindestens einer nicht-schwarzen LED (L1 oder L2)
                changingFrames: number;  // Frames, deren leds[] sich vom Vorframe unterscheiden
                virtualMs: number; frameMsAvg: number; frameMsMax: number;
                console: { lines: number; sample: string[] };   // Serial-Ausgabe (erste Zeilen)
                error?: string; stage?: string; framesRun?: number };
  ledCount: number | null;
  strips: { ledOffset; ledCount; pin; colorOrder: string; rgbw; isSpi: boolean; enabled: boolean; wireBytes }[];
  uiElements: { name: string; type: string; kind: 'slider'|'checkbox'|'number'|'button'|'dropdown'|'text'|'other';
                group?; min?; max?; step?; options?: string[]; defaultValue? }[];
  screenMap: null | { file: string;     // "screenmaps/<id>.json"
                      maps: { strip; ledOffset; length; diameter; xyWidth?; xyHeight?; xyType?;
                              bounds?: { minX; minY; maxX; maxY }; uniqueX; uniqueY }[] };
  geometry: { dim: 1 | 2 | 3 | null; width?: number; height?: number;
              source: 'xymap' | 'screenmap' | 'source' | 'strip' | 'manual' | 'none';
              shape?: 'line' | 'grid' | 'ring' | 'free'; bounds?; maps?; approxWidth?; approxHeight? };
  platform: 'pc' | 'hardware';  // pc = zeigt im PC-Modus etwas Sinnvolles
  platformAuto: 'pc' | 'hardware';
  note: string;                  // kurzer deutscher Hinweis für die Katalog-UI
  reason?: string;               // englisch, warum hardware/fehlerhaft (Doku)
  parity: null | { status: 'identical' | 'diff' | 'native-compile-error' | 'native-crash' | 'native-timeout' | 'trace-error';
                   frames?; l1DiffBytes?; l2DiffBytes?; rawDiffBytes?; metaDiffs?; l1Bytes?; firstDiffFrame?; diffFrames?; error? };
}
```

`screenmaps/<id>.json`: Array wie `FastLedHost.getScreenMaps()` —
`{strip, ledOffset, length, diameter, xyWidth?, xyHeight?, xyType?, x: number[], y: number[]}` je Controller.

### So lädt die IDE ein Beispiel

```ts
const ex = catalog.examples.find((e) => e.id === id)!;
const read = (p: string) => fetchText(`catalog/examples/${ex.id}/${p}`);
const source = await read(ex.mainFile);
const files = Object.fromEntries(await Promise.all(
  ex.files.filter((f) => f.compile && f.role !== 'main').map(async (f) => [f.path, await read(f.path)])));
await fetch('http://127.0.0.1:9996/compile', { method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ source, files, options: { fileName: ex.mainFile } }) });
```

## Mehrdatei-Modell des Compile-Dienstes

`POST /compile {source, files}`: `source` ist die Haupt-`.ino`, `files` alle weiteren Dateien des
Sketch-Ordners mit Pfad relativ zum Ordner (Unterordner erlaubt, `/` als Trenner; kein `..`, keine
absoluten Pfade, keine versteckten Dateien; Endungen `.h .hh .hpp .hxx .inc .ipp .tpp .c .cpp .cc .cxx
.ino .pde .json .txt .csv`).

- Haupt-Sketch = eine Übersetzungseinheit (Arduino-Vorverarbeitung: implizites `Arduino.h`,
  automatische Prototypen). Weitere `.ino`/`.pde` werden alphabetisch angehängt (Arduino-IDE-Tabs), mit
  `#line`, sodass Diagnosen auf der richtigen Datei/Zeile landen.
- **Jede** `.c/.cpp/.cc/.cxx` im Ordner, auch in Unterordnern, ist eine eigene Übersetzungseinheit und
  wird gelinkt (PlatformIO-Semantik; die Arduino-IDE übersetzt nur Wurzel + `src/`, alle FastLED-Beispiele
  funktionieren mit beiden Modellen, `LuminescentGrand` braucht `arduino/` und `shared/`).
  `.c` wird als C++ übersetzt.
- Header/Daten liegen in derselben Ordnerstruktur, `#include "…"` löst relativ zur einbindenden Datei
  auf; zusätzlich ist der Sketch-Ordner im Include-Pfad.
- Jede Einheit nutzt den vorkompilierten FastLED-Header, außer sie definiert vor ihrem ersten
  `#include <FastLED.h>` ein Makro (dann ohne PCH, damit das Makro wirkt).
- Höchstens `jobs` (config.json, Standard 3) Compiler-Prozesse gleichzeitig.
- Diagnosen: `file` ist der relative Pfad (`src/wave.cpp`, `Tab2.ino`), `inSketch: true`.
- Test: `npm run test:multifile`.
