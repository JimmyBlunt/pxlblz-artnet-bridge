# FastLED-Engine für PXLBLZ-IDE (`fastled-integration`)

Echte, unveränderte FastLED-Sketches (`setup()`/`loop()`, Arduino-Stil) laufen bitgenau als
WebAssembly im Browser (und in Node für Tests). Deterministisch, single-threaded, ohne
SharedArrayBuffer/COOP/COEP, getaktet vom Frame-Loop der IDE.

Verifiziert: 15 Sketches (13 FastLED-Beispiele + 2 Tests, bis 4.633 LEDs), je 1.200 Frames,
**0 abweichende Bytes** zwischen nativer Referenz und wasm, auf L1 (`leds[]`) und L2
(Leitungsbytes). Siehe [RESULTS.md](RESULTS.md).

## Architektur

```
Sketch (.ino) ──POST /compile──► service/server.mjs (127.0.0.1:9996)
                                   │ zig c++ -target wasm32-wasi  (1 TU, PCH mit FastLED.h)
                                   │ + vorkompilierte libfastled.a + pxl_runtime.o (gecacht)
                                   │ + binaryen Asyncify (in-process, npm "binaryen")
                                   ▼
                              sketch.wasm  ──►  runtime/fastledWasmHost.ts (Browser/Node)
                                                 init / frame / getLeds / getWire / getUi / setUi
```

- **Toolchain:** zig 0.16 (clang 20, bundled wasi-libc + libc++). Kein emsdk nötig. FastLED läuft auf
  seiner **Stub-Plattform** (dieselbe wie FastLEDs Host-Unit-Tests), single-threaded (`FASTLED_MULTITHREADED=0`),
  Netzwerk aus. Alle FastLED-Unity-Units außer `fl.test` werden in eine Archiv-Lib kompiliert; der Linker
  nimmt nur, was der Sketch braucht (`--gc-sections`).
- **FastLED bleibt unverändert.** Anpassungen nur über Header, die per `-I toolchain/override` vor
  `fastled/src` gefunden werden, plus `toolchain/pxl_prelude.h` (Force-Include):
  - `platforms/stub/platform_time.cpp.hpp`: `millis/micros/delay/delayMicroseconds` → virtuelle Uhr.
  - `platforms/shared/bitbang/bitbang_channel_driver.cpp.hpp`: Treiber ohne Bit-Banging (Senke).
  - `platforms/stub/clockless_channel_stub.h`: meldet `COLOR_ORDER`, Lazy-Encode (s. u.).
  - Prelude: Workarounds für FastLEDs Single-Thread-Stub-Pfad (no-op Threads, `std::chrono`-Forward-Decl).
  - Eine Ein-Zeilen-Korrektur in FastLED selbst: `toolchain/fastled-local.patch` (ESP32-RMT-Mock, Windows-Build).
- **Fiber:** `loop()` läuft in einer Koroutine. wasm: binaryen **Asyncify** (Import `env.pxl_suspend`,
  Steuerung in `fastledWasmHost.ts`). Native Referenz: Win32-Fiber (`native/fiber_win.cpp`).
  Die Scheduling-Logik (`runtime/pxl_runtime.cpp`) ist in beiden Builds **derselbe Code**.
- **L2 (Leitungsbytes):** FastLEDs eigene Pipeline (`PixelController` + `PixelIterator::writeWS2812`):
  `setBrightness`, `setCorrection`, `setTemperature`, temporales Dithering, Power-Limit, `COLOR_ORDER`, RGBW.
  Warum die Studie keine Helligkeit/Korrektur sah: der Stub-Capture (`ActiveStripTracker`) ruft absichtlich
  `disableColorAdjustment()` auf und setzt Helligkeit 255; die echten Leitungsbytes liegen im `ChannelData`
  des `SlimBridgeController`. Genau diese werden jetzt geliefert.
- **Lazy-Encode (Standard):** `show()` sichert nur eine Kopie von `PixelController` (inkl. Dither-Zustand)
  und Pixeldaten; kodiert wird erst bei `getWire()`. `FastLED.delay()` ruft `show()` jede Millisekunde auf;
  bei 4.593 LEDs spart das ~18 Encodes pro Frame (20 ms → 1 ms). Bytegleichheit ist verifiziert:
  Referenz = nativer **eager** Pfad (unverändertes FastLED), Kandidat = wasm **lazy**.

## Was ist ein IDE-Frame (Zeitmodell)

Virtuelle Sketch-Uhr in µs (`millis()` = µs/1000, `micros()` läuft wie auf Hardware nach ~71 min über).
Rechnen und `show()` kosten keine virtuelle Zeit.

- `init(seed)`: führt `setup()` vollständig aus; `delay()` in `setup()` schiebt nur die Uhr vor.
  Danach steht die Frame-Uhr auf dem Ende von `setup()`.
- `frame(dtMs)`: Frame-Uhr += dt, dann läuft `loop()` weiter, bis der Sketch nicht ohne Fortschritt der
  Frame-Uhr weiterkommt:
  - `delay(ms)`/`delayMicroseconds()`/`FastLED.delay()`, deren Weckzeit **hinter** der Frame-Uhr liegt,
    pausieren bis ein späterer Frame sie erreicht. Die Sketch-Uhr läuft dann **exakt** ab der Weckzeit weiter
    (keine Drift, unabhängig von der IDE-Framerate). Delays innerhalb des Frames laufen sofort durch.
  - Ein `loop()`-Durchlauf ohne verbrauchte Zeit (kein delay, z. B. Pride2015, Pacifica mit `EVERY_N_*`)
    beendet den Frame; der nächste Durchlauf startet zur nächsten Frame-Zeit. Also: zeitfreie Sketches = 1 `loop()` pro Frame.
  - Busy-Wait auf `millis()/micros()` (> 200.000 Abfragen ohne Zeitfortschritt) beendet den Frame ebenso;
    die Zeit springt dann auf die Frame-Zeit (Busy-Waits funktionieren also in Echtzeit).
  - **Budget:** mehr als 100.000 `loop()`-Durchläufe oder 20.000 `show()` in einem Frame → Status
    `BUDGET` (1); der Sketch bleibt fortsetzbar, der Tab friert nicht ein (`setBudget()` ändert die Grenzen).
- **Angezeigt** wird der Zustand des letzten `show()` mit virtueller Zeit ≤ Frame-Zeit (wie Abtasten eines
  echten Strips). Mehrere `show()` pro Frame: nur das letzte ist sichtbar.
- `frame(0)`: der **erste** Aufruf nach `init()` rendert den ersten Frame (er folgt dem Sketch bis zum ersten
  `show()`, max. 60 s virtuell). Jeder spätere `frame(0)` tut nichts (Status `SKIPPED` = 3) und liefert den letzten Frame.
- UI-Werte (`setUi`) wirken sofort, d. h. für den nächsten ausgeführten Sketch-Code.

## Laufzeit-ABI (wasm-Exports, `PXL_ABI_VERSION` = 1)

| Export | Bedeutung |
|---|---|
| `pxl_abi_version() -> i32` | 1; der Host verweigert andere Versionen |
| `pxl_init(seed u32) -> i32` | `setup()`; seed für Arduino `random()` (0 → 1) |
| `pxl_frame_begin(dt_us u32) -> i32`, `pxl_fiber_entry()`, `pxl_frame_end() -> u32` | ein Frame (vom Host über Asyncify orchestriert); Status 0 OK, 1 BUDGET, 2 NOT_INIT, 3 SKIPPED |
| `pxl_led_count()`, `pxl_leds_ptr()` | L1: RGB888 aller `addLeds`-Controller in Registrierungsreihenfolge |
| `pxl_wire_rgb_ptr()` | L2 in **RGB-Reihenfolge**, 3 × ledCount |
| `pxl_wire_raw_ptr()`, `pxl_wire_raw_len()` | L2 exakt wie auf der Datenleitung (COLOR_ORDER, RGBW) |
| `pxl_strip_count()`, `pxl_strips_ptr()` | je Strip 10 × i32: ledOffset, ledCount, wireOffset, wireLen, pin, colorOrder (EOrder oktal), rgbw, isSpi, enabled, 0 |
| `pxl_ui_json() -> char*`, `pxl_ui_version()`, `pxl_ui_set(json) -> i32` | FastLED-JsonUi: Elementliste; Setzen per `{"<id oder name>": wert}` |
| `pxl_now_ms() f64`, `pxl_now_us_lo/hi()`, `pxl_target_ms()`, `pxl_show_count()`, `pxl_loop_count()`, `pxl_frame_show_count()`, `pxl_brightness()` | Status/Var-Watcher |
| `pxl_set_budget(loops, shows)`, `pxl_set_lazy(on)`, `pxl_alloc/pxl_free`, `pxl_asyncify_data()` | Hilfen |

Imports: `env.pxl_suspend` und ein paar WASI-Funktionen (`fd_write` → Konsole; `clock_time_get` = virtuelle Uhr;
`random_get` deterministisch; alles andere ENOSYS). Kein Dateisystem, kein Netz.

## JS-Host (`runtime/fastledWasmHost.ts`)

Reines TypeScript (nur löschbare Syntax, läuft direkt in Node ≥ 22.18 und über Vite im Browser).

```ts
const host = await instantiateFastLedHost(bytesOrModule, { onConsole: (text, stream) => {} });
host.init(seed);               // setup()
host.frame(dtMs);              // -> { status, showCount, framesShows, nowMs }; frameUs(dtUs) exakt
host.getLeds();                // Uint8Array L1 (View in wasm-Speicher, pro Frame neu holen)
host.getWire();                // Uint8Array L2, RGB-Reihenfolge -> Art-Net
host.getWireRaw(); host.getStrips(); host.ledCount(); host.millis(); host.showCount();
host.getUi();                  // [{id, name, type, kind, min, max, step, options, value, defaultValue, group}]
host.setUi('Speed', 80);       // per Name (für Persistenz) oder id; klemmt wie FastLED
host.setBudget(loops, shows); host.abiVersion();
```
Wirft beim Trap des Sketches (Instanz ist danach tot; neu instanziieren). Ein Neustart = neue Instanz
(Modul einmal mit `compileFastLedModule` kompilieren, beliebig oft instanziieren).

## Compile-Dienst (`service/server.mjs`)

```
npm run service            # http://127.0.0.1:9996  (Port: --port, PXL_FASTLED_PORT oder config.local.json)
```
- `GET /health` → `{ok, ready, busy, queued, version, abiVersion, fastledVersion}`
- `GET /version` → FastLED-Commit/Version, zig-Version, ABI, Node
- `POST /compile` `{source, defines?, files?, options?}` → `{ok, key, cached, wasm (base64), wasmUrl, size, diagnostics[], prototypes[], timings}`
  - `defines`: z. B. `{"PXLBLZ_NUM_LEDS": 4593}` → `-D`
  - `files`: weitere Dateien des Sketch-Ordners (`*.h`, `*.cpp`)
  - `options`: `optimize: "fast"|"size"|"full"`, `autoPrototypes` (Standard an), `fileName`, `inline:false`
  - `Accept: application/wasm` → bei Erfolg rohe Bytes
  - Diagnosen: `{file:"sketch.ino", line, col, severity, message}` auf Zeilen des Nutzer-Sketches
- `GET /artifact/<key>.wasm` → gecachtes Modul
- CORS für `http://localhost:5174/5175` (+127.0.0.1), inkl. `Access-Control-Allow-Private-Network: true`.
  Belegter Port → klare Fehlermeldung, Exit 1. Cache nach Hash(Quelltext, defines, Dateien, Lib-Version).

## Einrichten, bauen, prüfen

```
npm run setup      # FastLED @ ec0a0f3 + Patch, zig 0.16 (uv/pip-venv), npm-Pakete, schreibt config.local.json
npm run build      # vorkompilierte Libs wasm + native (kalt ca. 1,5 min je Ziel, danach Sekunden)
npm run verify     # oder .\verify.ps1  -> Tabelle + RESULTS.md (Exit 0 = 0 abweichende Bytes)
npm run test:service
npm run bench      # Compile-/Frame-Zeiten bei 4.593 LEDs
node toolchain/build.mjs sketch mein.ino [-DPXLBLZ_NUM_LEDS=136] [--native]
```
Pfade: `config.json` (relativ zum Ordner, Standard `vendor/fastled`, `.venv`), maschinenspezifisch in
`config.local.json` (nicht eingecheckt). Caches unter `.cache/` (inkl. zig-Cache), Ausgaben unter `build/`.

## Messwerte (dieser Rechner, Node 24)

| | Wert |
|---|---|
| Lib-Build kalt (wasm bzw. native, 3 parallel) | ~90 s; erster Link überhaupt zusätzlich ~80 s (zig baut libc++ für wasm32-wasi einmalig) |
| Sketch-Compile warm (neu, ungecacht) | 2,0–3 s im Dienst (Compile ~1–1,4 s mit PCH, Link ~0,35 s, Asyncify ~0,6–1 s); unter Last bis ~6 s |
| Sketch-Compile gecacht | 10–40 ms |
| wasm-Größe | ~480–630 KB (`optimize:"size"` ~360–430 KB, `"full"` ~280–330 KB, gleiche Frame-Zeit) |
| Frame-Zeit 4.593 LEDs inkl. `getWire()` | 0,8–1,5 ms Mittel (Pride2015 0,83, Pacifica 1,06, DemoReel100 1,09, Fire2012WithPalette 1,49, PipelineTest 0,92), p99 2–9 ms |

## Grenzen

- Arduino `random()`/`rand()`/`randomSeed()`: deterministisch nach avr-libc (Park-Miller, Startwert 1 bzw. `seed`)
  = AVR-Semantik; ESP32-Hardware-Zufall ist nicht nachgebildet. `rand()` liefert 31 Bit (AVR: 15 Bit).
  FastLEDs `random8/16` behalten ihren eigenen Startwert.
- Hardware-APIs sind Stubs: `analogRead` (Zufall), `digitalWrite/pinMode` (no-op), `Serial.print` → Konsole,
  keine Eingabe. Kein WiFi/Netz/Dateisystem/Audio-Input, keine Threads/FreeRTOS-Tasks.
- Ein `while(true){}` ganz **ohne** Aufrufe von `millis/micros/delay/show` lässt sich nicht unterbrechen
  (kein Instruktionszähler) und blockiert den Tab. Budget/Spin-Erkennung greifen nur über diese Aufrufe.
- `setup()` läuft ohne Unterbrechung (Animationen in `setup()` sind nicht sichtbar).
- `show()` kostet keine Zeit (auf Hardware ~30 µs/LED); Sketches ohne `delay`, die sich auf die Übertragungsdauer
  verlassen, laufen hier 1 `loop()` pro Frame.
- `#define FASTLED_*`-Optionen im Sketch wirken nur auf Header-Code; die Lib ist mit Standardwerten vorkompiliert.
  Steht vor `#include <FastLED.h>` ein `#define`, wird ohne PCH kompiliert (langsamer, aber korrekt).
- Automatische Prototypen (Arduino-IDE-Verhalten) per einfacher Erkennung; exotische Signaturen ggf. selbst deklarieren.
- Nur Controller über `addLeds<>()` (Clockless) liefern L2; SPI-Chipsätze (APA102 …) liefern L1, L2 nur eingeschränkt.
- Referenz ist FastLEDs Stub-Plattform nativ (x86_64). Gegen echte ESP32-Hardware ist nicht gemessen.
- RESULTS.md/README.md sind UTF-8 ohne BOM; in Windows PowerShell 5.1 mit `Get-Content -Encoding UTF8` lesen.

## Dateien

```
config.json, package.json, verify.ps1, .gitignore, README.md, RESULTS.md
runtime/pxl_runtime.cpp        Laufzeit/ABI (wasm + native)
runtime/fastledWasmHost.ts     JS-Host
toolchain/toolchain.mjs        Build-/Compile-Logik (zig, PCH, Asyncify, Diagnosen, Prototypen)
toolchain/build.mjs, setup.mjs, pins.json, fastled-local.patch, pxl_prelude.h
toolchain/include/             Arduino.h, pxl_sketch.h, pxl_sketch_fastled.h (PCH)
toolchain/override/platforms/  die drei Override-Header
native/harness.cpp, native/fiber_win.cpp   native Referenz
service/server.mjs             Compile-Dienst
tests/verify.mjs, trace.mjs, service-test.mjs, bench-all.mjs, bench.mjs, smoke.mjs, sketches/*.ino
```
