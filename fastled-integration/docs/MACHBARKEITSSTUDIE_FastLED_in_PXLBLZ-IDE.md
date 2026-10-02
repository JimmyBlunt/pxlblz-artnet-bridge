# Machbarkeitsstudie: FastLED-Code in der PXLBLZ-IDE (1:1-Ausgabe)

Stand: 2026-10-02 · Bezug: FastLED `master` @ ec0a0f3 (2026-09-29, `library.properties` 3.10.5), PXLBLZ-IDE @ d685125b
Status: **nur Analyse + Mess-Spike**. Am IDE-Code, am Router und an den Launchern wurde nichts geändert. Alle Spike-Dateien liegen in `E:\PXLBLZ-ArtNet\fastled-study\`.

---

## 0. Kurzfassung

| Frage | Antwort |
|---|---|
| Kann die IDE FastLED-Sketches **100 % 1:1** ausgeben? | **Ja, aber nur auf einem Weg:** das echte FastLED plus Sketch nach WebAssembly kompilieren und in der IDE wie ein Pattern laufen lassen (Option A). Dann ist es *derselbe Code*, die Ausgabe ist per Konstruktion bitgleich. |
| Kann man FastLED-Code 1:1 in **Pixelblaze-Sprache** übersetzen (läuft dann auch auf Pixelblaze-Hardware und in Shows)? | **Nicht allgemein zu 100 %.** Für den FastLED-Kern (lib8tion, Zufall, Beat-Funktionen) und einfache Demos ist es **nachweislich bitgenau möglich** (Spike: 0 Abweichungen). C++ als Sprache, der 16.16-Zahlenraum, der Speicher (10.240-Wort-Array-Pool) und die Performance verhindern aber eine *garantierte* 100-%-Portierung beliebiger Sketches. |
| Empfehlung | **Option A (WASM-Engine) als Hauptweg** für Vorschau und Art-Net-Ausgabe (deine Installation). Optional später **Option C** (FastLED-kompatible Bibliothek in Pixelblaze-Sprache plus Hilfsportierung) für ausgewählte Patterns, die auf Pixelblaze-Hardware oder in kompilierte Shows sollen. Jedes portierte Pattern bekommt ein automatisch gemessenes Prädikat „bitgenau verifiziert“. |
| Aufwand (grob) | A: ca. 22–34 Personentage · C: ca. 20–30 PT Bibliothek, dazu 0,5–2 PT je Demo · B (vollautomatischer Transpiler): ca. 45–70 PT bei voraussichtlich 60–80 % Abdeckung |

**Wichtigste Messergebnisse aus dem Spike (Abschnitt 5):**

- Echtes FastLED nativ (Stub-Plattform), JS-Referenz und Pixelblaze-Portierung in der Precise-Engine (16.16) der IDE liefern für `scale8`, `qadd8`, `qsub8`, `sin8`, `sin16`, `random8`/`random16`, `beat8` und `beatsin8` **identische Ergebnisse** (über 470.000 Fälle, inklusive millis-Überlauf nach 49,7 Tagen).
- **Fire2012** (unverändertes FastLED-Beispiel): Pixelblaze-Portierung gegen natives FastLED **0 von 40.770 Bytes abweichend** über 453 Frames (Precise-Modus).
- Derselbe Code im **Fast-Modus** der IDE (float64): 100 % der Frames falsch. Also nie im Fast-Modus verifizieren.
- Ein einziger Reihenfolgefehler (`seed = seed + random16()` statt C++-Semantik) macht **jeden Frame** falsch (20.020 Byte-Abweichungen).
- Die übliche „Forum-Portierung“ von `beatsin8` über `wave(time())` weicht um bis zu **209 von 255 Stufen** ab.

---

## 1. Ziel und Definition von „100 % 1:1“

Du möchtest in der PXLBLZ-IDE neben Pixelblaze-Patterns auch FastLED-Code (aktuelle Version) verwenden. Die Ausgabe soll **exakt** der Ausgabe entsprechen, die der Sketch beim direkten Lauf erzeugt, und sich wie ein Pixelblaze-Pattern verhalten: Vorschau, Maps, Art-Net-Ausgabe, idealerweise Shows.

„1:1“ muss auf einer Ebene festgelegt werden. Die Studie unterscheidet drei Ebenen:

| Ebene | Was verglichen wird | Bemerkung |
|---|---|---|
| **L1 Logik** | Inhalt von `leds[]` (CRGB) bei jedem `FastLED.show()` | Das ist „das Pattern“. Die IDE gibt heute genau diese Ebene per Art-Net aus. |
| **L2 Leitung** | Bytes, die der Controller nach `setBrightness`, `setCorrection`/`setTemperature`, Temporal Dithering und Power-Limit sendet | Entspricht dem, was physisch an der LED ankommt |
| **L3 Timing** | Zeitpunkt und Abstand der Frames | Hängt bei FastLED von der echten Laufzeit ab (`millis()`, `delay()`, Busy-Loops) |

**Wichtige Erkenntnis:** „Direkt laufen“ ist selbst nicht eindeutig.

- **Plattformvarianten in FastLED:** `sin8`/`sin16` haben eine AVR-Assembler-Variante, eine C-Variante und mit `USE_SIN_32` eine LUT-Variante mit *anderen* Werten (`src/platforms/trig8.h`). `scale8` ist über `FASTLED_SCALE8_FIXED` konfigurierbar. Die Referenz muss also festgelegt werden, zum Beispiel „ESP32/Teensy, C-Pfad, Standard-Defines“.
- **Zeitabhängige Demos** (Pacifica, Pride2015, TwinkleFox, DemoReel100) rechnen mit `millis()`-Differenzen. Auf echter Hardware ist der Frame-Abstand nicht deterministisch, also ist die Ausgabe *auch auf der Hardware selbst* nicht reproduzierbar. Ein 1:1-Vergleich ist nur mit **gleicher virtueller Uhr** möglich. FastLED unterstützt das (`fl::inject_time_provider`, `setDelayFunction`), der Spike nutzt es.
- **`random()` von Arduino** kommt aus der C-Bibliothek der jeweiligen Plattform (avr-libc, newlib, msvcrt) und ist damit plattformabhängig. `random8`/`random16` von FastLED sind dagegen ein festes LCG und portabel.

**Vorschlag für die Definition:** 1:1 heißt bitgleich auf L1 (bei Bedarf auch L2) bei gleicher virtueller Zeitbasis, gleichem Zufalls-Seed und festgelegter Referenzplattform.

---

## 2. Ausgangslage in der IDE (gelesen, nicht geändert)

- **Pattern-Pipeline:** `bundle()` parst die Pixelblaze-Sprache mit Acorn und erzeugt `code` (float64, „Fast“) und `fxCode` (16.16-Fixed-Point, „Precise“). `loadPattern()` liefert ein `PatternHandle` mit `beforeRender(delta)`, `render/render2D/render3D(index, …)`, `getExports`, `setPatternVar`, `controls` (`src/engine/loadPattern.ts:54`). **Das ist die natürliche Andockstelle für eine FastLED-Engine.**
- **Frame-Loop:** `renderLoop.ts` ruft je Frame einmal `beforeRender` auf, dann `render*` je Pixel. Mit `paintPacked` landet jeder Frame als `Float64Array` (RGB 0..1) im Ausgabe-Adapter.
- **Art-Net-Pfad (deine Installation):** `PXLBLZ-IDE/src/engine/externalPixelOutput.ts` sendet RGB888 per WebSocket an `ws://127.0.0.1:9980/pixels`, der Router gibt per Art-Net aus. Die Byte-Umrechnung ist `Math.round(v*255)`. Damit wird ein FastLED-Byte `b`, ausgegeben als `rgb(b/255)`, sowohl in float64 als auch in 16.16 **verlustfrei** wieder zu `b` (im Spike bestätigt).
- **Precise-Engine:** `src/engine/fixedpoint.ts` bildet die 16.16-Arithmetik von Pixelblaze nach: int32-Überlauf, Multiplikation über Limbs, Bitwise-Operationen nur auf dem Ganzzahlteil (laut Code-Kommentar am Gerät mit Firmware 3.67 gemessen).
- **Ressourcengrenzen Pixelblaze-VM:** Array-Pool mit 10.240 Wörtern (`showVmResourceLedger.ts`), Zahlenbereich −32768..+32767,99998.
- **Shows** werden zu normalen Pixelblaze-Artefakten kompiliert. Alles, was in Shows *auf Hardware* laufen soll, muss in Pixelblaze-Sprache vorliegen.

---

## 3. Woraus „FastLED direkt“ besteht

| Baustein | Inhalt | Schwierigkeit beim Portieren nach Pixelblaze |
|---|---|---|
| Sprache | C++17: Klassen, Templates, Referenzen, Zeiger, Structs, Funktionszeiger-Arrays (DemoReel100), `static`-Locals, Integer-Typen mit definierter Überlauf-Semantik, Sequenzierungsregeln | **hoch.** Pixelblaze kennt kein `let`/`const`, keine Klassen, kein `switch`, kein `new`, nur Zahlen und Arrays |
| lib8tion | `scale8`, `qadd8`, `sin8/16`, `beat*`, `random8/16`, `sqrt16`, `ease8`, … | **mittel.** Bitgenau machbar (Spike), braucht aber sorgfältige Formulierung |
| Farbe | `CRGB`/`CHSV`, `hsv2rgb_rainbow`, Paletten (`CRGBPalette16/256`, `ColorFromPalette`, `nblend`), `HeatColor` | mittel. Als Bibliothek machbar, Packen von 3 Bytes je Pixel nötig wegen Speicher |
| Noise | `inoise8/16`, `fill_noise*` (Perlin, Integer) | mittel bis hoch (32-Bit-Zwischenwerte) |
| Puffer-Modell | `leds[NUM_LEDS]`, freie Indexschreibzugriffe, `fadeToBlackBy`, `blur1d/2d`, `fill_*` | mittel. Pixelblaze rendert pro Pixel, braucht also einen Puffer in `beforeRender` und `render(i)` liest ihn |
| Zeit/Loop | `setup()` und `loop()` in Endlosschleife, `FastLED.show()` (auch mehrfach je Loop), `delay()`, `FastLED.delay()` (zeigt intern mehrfach an), `EVERY_N_*`, `millis()` als u32 | **hoch.** Pixelblaze ruft `beforeRender` einmal pro Frame auf. `delay()` mitten in Schleifen (Cylon, Blink) braucht Koroutinen oder Zerlegung in Zustandsautomaten |
| Pixel-Pipeline (L2) | Helligkeit, Farbkorrektur, Farbtemperatur, Temporal Dithering, Power-Limit, Farbreihenfolge | mittel. Muss bei Bedarf explizit nachgebaut werden |
| Plattform | Varianten für AVR, ARM, ESP32 und WASM, Arduino-API (`random`, `analogRead`) | Referenzplattform festlegen |

**Feature-Erhebung der Standard-Demos** (Zählung der Fundstellen):

| Demo | Zeilen | Paletten | 16-Bit-Mathe | `EVERY_N` | Funktionszeiger | Zufall | Besonderheit |
|---|---:|---:|---:|---:|---:|---:|---|
| Blink | 97 | 0 | 0 | 0 | 0 | 0 | `delay` in loop |
| Cylon | 59 | 0 | 0 | 0 | 0 | 0 | `show` + `delay` in for-Schleifen, also Koroutine nötig |
| Fire2012 | 113 | 0 | 0 | 0 | 0 | 6 | im Spike **bitgenau portiert** |
| ColorPalette | 206 | 47 | 0 | 0 | 0 | 2 | Paletten-Wechsel per `millis` |
| DemoReel100 | 136 | 2 | 2 | 2 | 3 | 4 | Array aus Funktionszeigern |
| Pride2015 | 84 | 0 | 20 | 0 | 0 | 0 | `beatsin88`, u32-Zeitdeltas |
| Pacifica | 170 | 6 | 30 | 1 | 0 | 0 | `fl::`-Typen, u32-Deltas, `EVERY_N_MILLISECONDS(20)` |
| TwinkleFox | 392 | 41 | 6 | 2 | 0 | 0 | eigener PRNG, viele Paletten |
| NoisePlusPalette | 414 | 42 | 8 | 0 | 0 | 7 | `inoise8`, XY-Mapping, 2D |

---

## 4. Lösungsoptionen

### Option A: Echtes FastLED als WebAssembly-Engine in der IDE (empfohlen)

**Idee:** Sketch und unverändertes FastLED werden mit Emscripten zu WASM kompiliert. Ein kleiner Adapter implementiert dasselbe `PatternHandle` wie ein Pixelblaze-Pattern:

```
beforeRender(delta) -> virtuelle Uhr += delta; loop() laufen lassen bis zum nächsten FastLED.show()
render(index)       -> rgb(leds[index].r/255, .g/255, .b/255)   (oder L2-Bytes)
controls            -> FastLED-UI-Elemente (UISlider, UICheckbox ...) als IDE-Controls
```

- **Treue:** Es läuft derselbe Code, daher bitgleich auf L1 und auch auf L2, weil Helligkeit, Korrektur und Dithering von FastLED selbst gerechnet werden. Bedingungen: gleiche Defines (C-Pfad wie ESP32/Teensy, nicht AVR-Assembler) und dieselbe virtuelle Uhr.
- **Wiederverwendung:** FastLED hat eine offizielle WASM-Plattform (`src/platforms/wasm/`, `getFrameData`, JS-Bindings, UI-Elemente, Screen-Map) und den offiziellen Compiler `pip install fastled` (zackees/fastled-wasm, MIT). Beides ist direkt nutzbar.
- **Zu bauen:**
  1. Kompilierdienst: lokal im Launcher (Emscripten oder FastLED-Compiler, ca. 1–2 GB auf E:). Cloudflare Pages und Workers können kein Emscripten ausführen. Alternativ clang-in-WASM im Browser (sehr groß, nicht empfohlen).
  2. Eigener Einstiegspunkt statt `PROXY_TO_PTHREAD`/`requestAnimationFrame` der Standard-Plattform: deterministisches `step(ms)` mit `show()` als Frame-Grenze (Asyncify oder FastLED-Koroutinen). Die Standard-WASM-Plattform nutzt Web Worker mit `SharedArrayBuffer`, das braucht COOP/COEP-Header und kann mit OAuth-Popups bzw. der Extension kollidieren.
  3. IDE: neuer Pattern-Typ bzw. Sprachmodus „FastLED (C++)“ (Monaco-C++, Fehlerausgabe des Compilers), Adapter auf `PatternHandle`, Zuordnung `leds[i]` → Map-Index (identisch zur Pixelblaze-Indexlogik), Controls, Var-Watcher (optional).
  4. Testharness mit Golden Frames (siehe Abschnitt 9).
- **Grenzen:** Läuft im Browser, also Vorschau und Art-Net (deine Installation, siehe Router). **Nicht** auf Pixelblaze-Hardware, nicht in hardware-kompilierten Shows. In Browser-Shows ist es als Clip denkbar, mit eigenem Aufwand.
- **Performance:** Nativer Code mit ca. 1,2–2× WASM-Overhead. 4.593 px bei 60 FPS sind für FastLED-Demos unkritisch, die Router-Kette schafft 60 FPS bereits.
- **Aufwand:** ca. 22–34 PT (Dienst 5–8, Engine-Einstieg 5–8, IDE-Integration 8–12, Tests 4–6).

### Option B: Vollautomatischer Transpiler C++ → Pixelblaze-Sprache

- **Idee:** C++-Frontend (libclang-AST) erzeugt Pixelblaze-Code, dazu eine bitgenaue FastLED-Laufzeitbibliothek in Pixelblaze-Sprache.
- **Treue:** Im Prinzip erreichbar für die genutzte Teilmenge (Spike beweist es für lib8tion und Fire2012), **aber nicht garantiert**:
  - **Zahlenraum 16.16:** u16-Werte ≥ 32768 und alle u32-Werte (`millis()`) brauchen Darstellungstricks (int16-Zweierkomplement, Halbworte, Limb-Multiplikation, siehe `beat88` im Spike). Literale wie `65536` werden still zu `0`. Der Spike ist selbst in diese Falle gelaufen.
  - **Speicher:** `leds[]` mit 4.593 px × 3 = 13.779 Wörter passt nicht in den Pool von 10.240 Wörtern. Packen auf 1 Wort je Pixel ist Pflicht, Zusatzpuffer (Heat, Noise) verschärfen das.
  - **Sprache:** Klassen, Templates, Zeiger und Referenzen, Funktionszeiger, `static`-Locals und Sequenzierungsregeln lassen sich nur teilweise mechanisch übersetzen.
  - **Kontrollfluss:** `delay()`/`show()` mitten in Schleifen erfordern eine Transformation in Zustandsautomaten (machbar, aber komplex).
  - **Performance auf Pixelblaze-HW:** u16/u32-Emulation kostet ein Vielfaches (z. B. `beat88` ca. 30 VM-Operationen statt einer). Ob 60 FPS erreicht werden, ist pro Pattern offen und muss auf Hardware gemessen werden.
- **Aufwand:** ca. 45–70 PT, voraussichtliche Abdeckung der Standard-Demos 60–80 %, Rest manuell.

### Option C: FastLED-kompatible Bibliothek in Pixelblaze-Sprache plus Hilfsportierung

- Stock-Library `FastLED`/`FL8` (lib8tion, CRGB-Packing, Paletten, `HeatColor`, `hsv2rgb_rainbow`, `fill_*`, `fadeToBlackBy`, `blur`, `EVERY_N`, `beat*`, Noise) mit **bitgenauem Nachweis gegen natives FastLED** (Harness aus dem Spike). Demos werden halbautomatisch oder manuell portiert, jede Portierung wird automatisch Frame für Frame verifiziert.
- **Vorteil:** Ergebnis ist echte Pixelblaze-Sprache, läuft auf Pixelblaze-Hardware und in Shows. **Nachteil:** Pro Sketch Handarbeit, keine Garantie für beliebigen Code.
- **Aufwand:** Bibliothek ca. 20–30 PT, je Demo 0,5–2 PT.

### Option D: CPU-Emulation (avr8js / Wokwi)

- Echte Firmware wird emuliert, also bitgleich für genau diese Plattform. avr8js (MIT) kann nur AVR: 2 KB RAM beim ATmega328, damit maximal ca. 600 LEDs, und die AVR-Assembler-Mathevarianten passen nicht zur ESP32/Teensy-Referenz. Wokwi (ESP32) ist proprietär und nicht einbettbar.
- **Nur als unabhängige Test-Referenz** sinnvoll, nicht als IDE-Engine.

### Option E: Eigener C++-Interpreter bzw. Transpiler nach JavaScript (Ansatz von ArduSim)

- Hat dieselben Sprachprobleme wie B, ohne den Hardware-Vorteil. Nicht empfohlen, weil A bei gleichem Ziel (Browser) exakt ist.

### Bewertungsmatrix (1 = schlecht, 5 = sehr gut)

| Kriterium | A WASM | B Transpiler | C Bibliothek + Port | D Emulation | E C++→JS |
|---|:-:|:-:|:-:|:-:|:-:|
| 1:1-Treue L1 | **5** | 3 | 4 (je Pattern verifiziert) | 5 (nur AVR) | 2 |
| 1:1-Treue L2 (Helligkeit, Dithering) | **5** | 2 | 3 | 5 | 2 |
| Beliebige Sketches | **5** | 3 | 2 | 3 | 3 |
| Läuft auf Pixelblaze-HW / in Shows | 1 | **4** | **4** | 1 | 1 |
| Art-Net-Installation (dein Betrieb) | **5** | 4 | 4 | 2 | 4 |
| Performance | **5** | 2 (HW) / 4 (Browser) | 3 | 1 | 4 |
| Aufwand (5 = gering) | 3 | 1 | 3 | 2 | 2 |
| Wartung bei FastLED-Updates | **5** (neu kompilieren) | 2 | 2 | 3 | 2 |

---

## 5. Mess-Spike (durchgeführt, reproduzierbar)

**Aufbau:**
- `fastled/`: FastLED master @ ec0a0f3 (sparse clone, nur Lesezugriff, bis auf eine dokumentierte Ein-Zeilen-Korrektur, siehe `LOCAL_PATCHES.txt`)
- `build-fastled.ps1`: kompiliert FastLED komplett mit **zig c++ 0.16** (in `.venv`, über PyPI) auf der **Stub-Plattform** (`STUB_PLATFORM`, `FASTLED_STUB_IMPL`, `FASTLED_TESTING`)
- `spike/ref-native-math.cpp`: ruft die echten FastLED-Funktionen auf und bildet FNV-1a-Hashes, mit injizierter virtueller Zeit
- `spike/harness-native.cpp` + `build-demo.ps1`: lässt ein **unverändertes** FastLED-Beispiel (`#include` der `.ino`) mit virtueller Uhr laufen und schreibt `leds[]` und die Stub-Leitungsdaten je `loop()`
- `spike/fl8-pixelblaze.js`, `spike/fire2012-pixelblaze.js`: Portierungen in Pixelblaze-Sprache
- `spike/run-spike.ts`, `spike/compare-demo.ts`: führen die Portierungen mit der **originalen IDE-Engine** aus (`bundle` → `loadPattern` → `createShim`/`createFxShim`, nur lesender Import) und vergleichen
- Wiederholen: `tsx --tsconfig ..\..\PXLBLZ-IDE\tsconfig.json run-spike.ts` bzw. `compare-demo.ts fire2012-pixelblaze.js ..\build\fire2012.leds.bin 30 453`

### 5.1 lib8tion-Kern: Pixelblaze-Port gegen C-Semantik

| Test | Fälle | Abweichungen Fast (float64) | Abweichungen Precise (16.16) |
|---|---:|---:|---:|
| scale8 (vollständig 256×256) | 65.536 | 0 | **0** |
| qadd8 | 65.536 | 0 | **0** |
| qsub8 | 65.536 | 0 | **0** |
| sin8 (vollständig) | 256 | 0 | **0** |
| sin16 (vollständig) | 65.536 | 0 | **0** |
| random16 (100.000 Züge, Seed 1337) | 100.000 | 99.995 | **0** |
| random8 (100.000 Züge) | 100.000 | 99.604 | **0** |
| beat8 (9 BPM × 0–10 min, nach 1 Tag, millis-Überlauf 2³²) | 8.154 | 0 | **0** |
| beatsin8 (dieselben Zeitpunkte) | 8.154 | 0 | **0** |
| beatsin8 als „Forum-Idiom“ `wave(time())` | 5.418 | 5.319 (max. 209 Stufen) | – |

### 5.2 Referenz gegen echtes, natives FastLED

Die FNV-1a-Hashes aller neun Funktionen sind zwischen JS-Referenz und nativ kompiliertem FastLED **identisch** (`spike/spike-results.md`). Daraus folgt die Kette: **natives FastLED = Referenz = Pixelblaze-Port (Precise)**.

### 5.3 Komplette Demo Fire2012 (unverändertes Beispiel)

| Variante | Frames | abweichende Frames | abweichende Bytes |
|---|---:|---:|---:|
| Pixelblaze-Port, Precise (16.16) | 453 | **0** | **0 / 40.770** |
| Pixelblaze-Port, Fast (float64) | 453 | 453 | 22.391 / 40.770 |
| Port mit naiver Reihenfolge `seed = seed + random16()` | 453 | 453 | 20.020 / 40.770 |
| Port mit naivem Überlauf `(i*sc)>>8` | 453 | 0* | 0* |

\* Bestanden nur **zufällig**: Der Überlauf wickelt in 16.16 modulo 2¹⁶, und Fire2012 nutzt danach nur die unteren Bits (`& 63`, `& 128`). In anderem Code (Vergleiche, Ausgabe) wäre derselbe Fehler sichtbar. Das zeigt, warum jede Portierung **gemessen** und nicht nur begutachtet werden muss.

### 5.4 Leitungsebene (L2) im Stub

Die Stub-Leitungsdaten weichen in 14,2 % der Bytes um +1 von `leds[]` ab (Temporal Dithering). Helligkeit 200 und `TypicalLEDStrip`-Korrektur sind in der Stub-Aufzeichnung **nicht** sichtbar. Für L2-Treue ist der Stub daher keine ausreichende Referenz. In Phase 2 wäre die WASM-Plattform bzw. eine ESP32-Messung (Wokwi oder Logic Analyzer) nötig. Bei Option A ist das unkritisch, weil FastLED die Pipeline selbst rechnet.

---

## 6. Harte Grenzen einer Pixelblaze-Portierung

1. **Zahlenraum:** 16.16 mit Vorzeichen. u16 nur als Zweierkomplement, u32 nur in Halbworten. Multiplikationen mit Zwischenwerten über 32767 müssen umgestellt werden (`i/256*(sc+1)` statt `i*(sc+1)>>8`). Float-Code (`sin()`, `sqrt()` in Sketches) ist auf Pixelblaze nur 16.16-genau, also **nicht** bitgleich zu IEEE-float. Nur Integer-FastLED ist voll portierbar.
2. **Speicher:** Array-Pool 10.240 Wörter. Die Installation mit 4.593 px braucht gepackte Puffer (1 Wort = R, G, B).
3. **Bitwise-Semantik offen:** Die IDE-Engine führt Bitwise-Operationen nur auf dem Ganzzahlteil aus (laut Code am Gerät mit FW 3.67 gemessen). Die offizielle ElectroMage-Referenz sagt dagegen, sie arbeiten auf allen 32 Bit. Der Spike ist gegen die *IDE-Engine* verifiziert. **Vor einem Hardware-Einsatz muss die Portierung auf echter Pixelblaze-Hardware gegengemessen werden.**
4. **Zeitmodell:** `beforeRender(delta)` ist kein `loop()`. `delay()` und mehrfache `show()` je Loop brauchen eine Transformation.
5. **Sprachumfang:** Nur eine C-nahe Teilmenge von C++ ist mechanisch übersetzbar.
6. **Performance:** u16/u32-Emulation ist teuer, 60 FPS auf Pixelblaze-HW sind je Pattern ungewiss.

---

## 7. Bestehende Ansätze und ihr Nutzen

| Projekt / Quelle | Was es ist | Nutzbarkeit für uns |
|---|---|---|
| **FastLED WASM-Plattform** (`src/platforms/wasm`) + **zackees/fastled-wasm** (`pip install fastled`, MIT) | Offizieller Compiler FastLED → WASM/JS/HTML inklusive UI-Elementen und Frame-Export | **Sehr hoch.** Basis für Option A (Compiler, Bindings, UI → Controls) |
| **FastLED Stub-Plattform** + `inject_time_provider` / `setDelayFunction` | Host-Build ohne Hardware für Unit-Tests | **Sehr hoch.** Im Spike bereits als Golden-Frame-Referenz im Einsatz |
| jandelgado/fastled-wasm | Frühes Emscripten-Experiment | gering (überholt durch das offizielle Projekt) |
| ElectroMage-Forum „WLED pattern porting?“ (pixie, zranger1, scruffynerf), „Translating from FastLED to PixelBlaze“ (Pacifica), „Translating FastLED pixel maps“ | Manuelle Idiome: Puffer-Architektur, `EVERY_N`-Emulation über Delta-Akkumulatoren, Paletten in Map-Daten | **mittel** für Ideen. Keine veröffentlichte Bibliothek, nicht bitgenau (gemessen: bis zu 209 Stufen Fehler beim `wave()`-Idiom) |
| atuline/PixelBlaze | Portierung in Gegenrichtung (Pixelblaze → FastLED) mit Helper-Header | gering bis mittel (Zuordnungstabelle der Built-ins) |
| jasoncoon LED Mapper | Konvertiert FastLED-XY-Maps und Pixelblaze-Maps | **mittel** (Import von XY-Mapping/Screen-Map als IDE-Map) |
| Pixelique (12/2025) | Browser-FastLED-Editor auf avr8js-Basis, kann zu WLED streamen | gering. Zeigt den Weg über Emulation, aber nur AVR und Hobbyprojekt |
| ArduSim (MIT) | C++→JS-Transpiler für Arduino inklusive FastLED-Unterstützung | gering. Proof of Concept für Option E, FastLED-Treue unbekannt, kaum verbreitet |
| Wokwi / avr8js | ESP32-Emulation (proprietär) bzw. AVR-Emulation (MIT) | mittel als **unabhängige Test-Referenz** für ESP32 in Phase 2 |
| esp-idf-lib `lib8tion` | C-Portierung von lib8tion für ESP-IDF | keine (wieder C) |

Fazit: Eine fertige, bitgenaue FastLED→Pixelblaze-Lösung gibt es nicht. Für den Browser-Weg (A) ist dagegen fast alles vorhanden.

---

## 8. Integrationsskizze Option A (Entwurf, noch nicht umgesetzt)

```
Studio: Pattern-Typ „FastLED (C++)“ ──► Monaco (cpp) ──► lokaler Compile-Dienst (Launcher, E:)
                                                         │  emcc + FastLED (gepinnte Version)
                                                         ▼
                                    sketch.wasm + Metadaten (NUM_LEDS, UI-Elemente, Screen-Map)
                                                         │
Preview.tsx ─► createRenderLoop ─► FastLedHandle implements PatternHandle
                                    beforeRender(Δ): vclock += Δ; run until show()
                                    render(i): rgb(leds[i]/255)
                                                         │
                         Renderer (WebGL)   +   externalPixelOutput ─► Router ─► Art-Net
```

- Map: `leds[i]` wird über den Index zugeordnet, genau wie bei Pixelblaze. Eine FastLED-Screen-Map lässt sich optional als IDE-Map importieren.
- Controls: `UISlider`/`UICheckbox`/`UINumberField` von FastLED werden auf IDE-Controls abgebildet.
- Pinning: FastLED-Version und Defines werden im Pattern gespeichert, damit die Ausgabe reproduzierbar bleibt.
- Send to Controller: für FastLED-Patterns deaktiviert (oder später über Option C).

---

## 9. Testkonzept (für die Umsetzungsphase)

1. **Golden Frames:** Jedes Standard-Beispiel läuft nativ auf der Stub-Plattform mit virtueller Uhr (Schritt 1000/60 ms bzw. `delay`-gesteuert), Seed 1337 und N = 1.200 Frames. `leds[]` wird je `show()` gespeichert (Harness existiert bereits).
2. **Kandidat:** derselbe Sketch über Option A (WASM im IDE-Adapter, headless in Vitest/Playwright) bzw. über die Portierung (Option C) in Precise.
3. **Kriterium:** 0 abweichende Bytes auf L1. Für L2 zusätzlich Vergleich gegen die FastLED-WASM-Leitungsdaten.
4. **Umfang:** Blink, Cylon, Fire2012, Fire2012WithPalette, ColorPalette, DemoReel100, Pride2015, Pacifica, TwinkleFox, NoisePlusPalette, XYMatrix, Noise, RGBCalibrate. Dazu Langzeittests über den millis-Überlauf.
5. **Hardware-Gegenprobe (nur für Option C):** dieselben Frames auf echter Pixelblaze-Hardware auslesen (Bitwise-Frage aus Abschnitt 6.3).

---

## 10. Phasenplan und Risiken

| Phase | Inhalt | Ergebnis |
|---|---|---|
| 0 ✅ | Analyse + Spike (dieses Dokument) | Machbarkeit belegt, Risiken quantifiziert |
| 1 | Entscheidungen (Abschnitt 11) | Zieldefinition 1:1, Referenzplattform |
| 2 | Prototyp A: Emscripten lokal, deterministischer Einstieg, 13 Demos gegen Golden Frames | Nachweis „0 Bytes Abweichung“ für A |
| 3 | IDE-Integration A (Pattern-Typ, Compile-Dienst im Launcher, Controls, Art-Net) | FastLED-Patterns im Studio, Ausgabe auf deine Installation |
| 4 (optional) | Option C: FL-Bibliothek + Hilfsportierungen + HW-Gegenprobe | Ausgewählte FastLED-Patterns auf Pixelblaze-HW und in Shows |

| Risiko | Bewertung | Gegenmaßnahme |
|---|---|---|
| Emscripten bzw. Compile-Dienst auf Windows/Launcher | mittel | FastLED-Compiler (`fastled`) bzw. emsdk auf E:, Version pinnen |
| `delay()`-/Busy-Loop-Sketches im deterministischen Modus | mittel | Asyncify bzw. FastLED-Koroutinen, `show()` als Frame-Grenze, Watchdog |
| COOP/COEP bei der Standard-WASM-Plattform | mittel | eigener Einstieg ohne pthreads |
| Bitwise-Semantik IDE gegen Firmware | hoch (nur Option C) | Hardware-Gegenmessung |
| FastLED-API-Änderungen (schneller Release-Takt, `fl::`-Umbau) | mittel | Version je Pattern pinnen, Golden Frames in CI |

---

## 11. Offene Entscheidungen (bitte beantworten)

1. **Ausgabeziel:** Reicht Vorschau + Art-Net (deine Installation mit Teensy/ESP32), oder müssen FastLED-Patterns auch auf **Pixelblaze-Controllern** bzw. in hardware-kompilierten **Shows** laufen?
2. **Referenzplattform für „direkt“:** ESP32, Teensy 4.1 (deine Controller) oder AVR? Davon hängen z. B. die `sin8`-Varianten ab.
3. **Ebene:** Reicht L1 (`leds[]`), oder brauchst du L2 (Helligkeit, Korrektur, Dithering exakt wie am Controller)?
4. **Compile-Dienst:** Ist ein lokaler Dienst im Launcher in Ordnung (ca. 1–2 GB auf E:)?

---

## 12. Nebenbefunde (nicht Teil der Aufgabe, nichts geändert)

1. **Fast-Modus der IDE** ist für überlaufabhängige Integer-Mathematik (LCG, u16) nicht treu (Spike: 99,6 % Fehler). FastLED-artiger Code muss in Precise laufen bzw. verifiziert werden.
2. **Bitwise-Diskrepanz** zwischen IDE-Engine (Ganzzahlteil) und offizieller Pixelblaze-Doku (32 Bit), siehe Abschnitt 6.3.
3. **`PXLBLZ-IDE/src/components/Preview.tsx`** (Art-Net-Worktree, uncommitted): Die Ausgabe-Adapter-Änderung hat ein BOM eingefügt und UTF-8-Zeichen in Kommentaren zerstört (z. B. `—` → `â€”`). Funktional egal, aber der Diff wird unnötig groß.
4. **Upstream-Fehler FastLED:** `rmt5_peripheral_mock.cpp.hpp:351` (`stub::fl::micros()`) kompiliert im Windows-Host-Build nicht. Lokal in der Referenzkopie korrigiert (`LOCAL_PATCHES.txt`), Kandidat für einen Upstream-Report.
5. **Literal-Falle 16.16:** `65536` (und alles ≥ 32768) wird in Pixelblaze still zu `0` bzw. wickelt um. Ein Lint in der IDE, der solche Literale meldet, wäre auch für normale Patterns nützlich.

---

### Quellen

- FastLED: [Repository](https://github.com/FastLED/FastLED), [fastled4.md](https://github.com/FastLED/FastLED/blob/master/fastled4.md), [WASM-Plattform](https://github.com/FastLED/FastLED/tree/master/src/platforms/wasm), [Stub-Plattform / Plattform-Readme](https://github.com/FastLED/FastLED/blob/master/src/platforms/readme.md), [Releases](https://github.com/FastLED/FastLED/releases)
- [zackees/fastled-wasm](https://github.com/zackees/fastled-wasm), [PyPI fastled](https://pypi.org/project/fastled/), [jandelgado/fastled-wasm](https://github.com/jandelgado/fastled-wasm)
- ElectroMage-Forum: [WLED pattern porting?](https://forum.electromage.com/t/wled-pattern-porting/1295) ([Seite 2](https://forum.electromage.com/t/wled-pattern-porting/1295?page=2)), [Translating from FastLED to PixelBlaze](https://forum.electromage.com/t/translating-from-fastled-to-pixelblaze/572), [Translating FastLED pixel maps to Pixelblaze](https://forum.electromage.com/t/translating-fastled-pixel-maps-to-pixelblaze/3195), [Converted a number of patterns for use with FastLED](https://forum.electromage.com/t/converted-a-number-of-patterns-for-use-with-fastled/1815)
- [LED Mapper](https://jasoncoon.github.io/led-mapper/), [Pixelique-Vorstellung](https://forum.arduino.cc/t/another-online-fastled-simulator/1417505), [ArduSim](https://github.com/smandal439/ardusim), [AVR8js](https://blog.wokwi.com/avr8js-simulate-arduino-in-javascript/), [Wokwi ESP32](https://docs.wokwi.com/guides/esp32)
