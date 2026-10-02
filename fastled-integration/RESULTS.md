# Golden-Frame-Verifikation (generiert von `npm run verify`)

Stand: 2026-10-02T13:59:20.200Z | FastLED ec0a0f35ef (3.10.5, lokaler Patch fa85aac74abe) | zig 0.16.0 | Node v24.19.0

Referenz: dasselbe unveränderte Beispiel nativ (x86_64-windows-gnu, Stub-Plattform, gleiche Defines, Win32-Fiber, FastLEDs eager Encode-Pfad). Kandidat: wasm32-wasi + Asyncify über runtime/fastledWasmHost.ts in Node (Lazy-Encode). 1200 Frames, identischer Frame-Zeitplan (ca. 60 fps mit Jitter, dt=0-Frames inkl. Frame 0, 250-ms-Hänger), Seed 1, UI-Ereignisse bei PipelineTest. Verglichen wird pro Frame: Status, show()-Zähler, virtuelle Uhr (µs), L1 (leds[] beim letzten show()), L2 RGB (getWire()) und L2 roh (Leitungsreihenfolge, getWireRaw()).

| Sketch | Frames | LEDs | show()-Aufrufe | virt. Zeit | L1 abweichend | L2 (RGB) abweichend | L2 roh abweichend | Meta abweichend | L2 != L1 | wasm | ms/Frame (Node, ohne getWire) |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| Blink | 1200 | 1 | 69 | 20.4 s | **0** / 3600 | **0** / 3600 | **0** / 3600 | 0 | 0.0 % | 478 KB | 0.005 |
| Cylon | 1200 | 64 | 2076 | 20.8 s | **0** / 230400 | **0** / 230400 | **0** / 230400 | 0 | 52.5 % | 478 KB | 0.008 |
| Fire2012 | 1200 | 30 | 22057 | 23.8 s | **0** / 108000 | **0** / 108000 | **0** / 108000 | 0 | 47.1 % | 477 KB | 0.022 |
| Fire2012WithPalette | 1200 | 30 | 22057 | 23.8 s | **0** / 108000 | **0** / 108000 | **0** / 108000 | 0 | 46.6 % | 481 KB | 0.026 |
| ColorPalette | 1200 | 50 | 22835 | 23.8 s | **0** / 180000 | **0** / 180000 | **0** / 180000 | 0 | 48.2 % | 481 KB | 0.029 |
| DemoReel100 | 1200 | 64 | 23354 | 23.8 s | **0** / 230400 | **0** / 230400 | **0** / 230400 | 0 | 87.2 % | 483 KB | 0.027 |
| Pride2015 | 1200 | 200 | 1188 | 23.8 s | **0** / 720000 | **0** / 720000 | **0** / 720000 | 0 | 66.6 % | 480 KB | 0.013 |
| Pacifica | 1200 | 60 | 650 | 23.8 s | **0** / 216000 | **0** / 216000 | **0** / 216000 | 0 | 66.7 % | 526 KB | 0.012 |
| TwinkleFox | 1200 | 100 | 1188 | 23.8 s | **0** / 360000 | **0** / 360000 | **0** / 360000 | 0 | 22.8 % | 522 KB | 0.012 |
| NoisePlusPalette | 1200 | 256 | 1188 | 23.8 s | **0** / 921600 | **0** / 921600 | **0** / 921600 | 0 | 61.0 % | 487 KB | 0.021 |
| XYMatrix | 1200 | 256 | 1188 | 20.8 s | **0** / 921600 | **0** / 921600 | **0** / 921600 | 0 | 65.9 % | 477 KB | 0.009 |
| Noise | 1200 | 256 | 1188 | 23.8 s | **0** / 921600 | **0** / 921600 | **0** / 921600 | 0 | 65.0 % | 481 KB | 0.037 |
| RGBCalibrate | 1200 | 0 | 21 | 22.0 s | **0** / 0 | **0** / 0 | **0** / 0 | 0 | 0.0 % | 393 KB | 0.001 |
| PipelineTest | 1200 | 100 | 2443 | 21.0 s | **0** / 360000 | **0** / 360000 | **0** / 360000 | 0 | 74.1 % | 611 KB | 0.034 |
| PipelineTest@4593 | 1200 | 4633 | 2443 | 21.0 s | **0** / 16678800 | **0** / 16678800 | **0** / 16678800 | 0 | 84.8 % | 611 KB | 0.266 |

**Ergebnis: PASS - 0 abweichende Bytes**

"L2 != L1" = Anteil der Bytes, in denen die Leitungsdaten (nach Helligkeit, Farbkorrektur/-temperatur, Dithering) von leds[] abweichen; zeigt, dass die Pipeline tatsächlich angewendet wird. RGBCalibrate hat im Original alle addLeds()-Zeilen auskommentiert (0 LEDs).
