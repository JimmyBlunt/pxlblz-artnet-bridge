# FastLED-Beispielkatalog: Ergebnisse (generiert von `node catalog/build-catalog.mjs`)

Stand: 2026-10-03T11:15:42.799Z | FastLED 3.10.5 @ ec0a0f35ef | zig 0.16.0 | 600 Frames, Seed 1, Zeitplan wie `npm run verify`

Pro Beispiel: Kompilieren über den Compile-Dienst (`POST /compile`, wie die IDE), dann 600 Frames in Node über `runtime/fastledWasmHost.ts`; wo es läuft zusätzlich native Referenz (x86_64, Win32-Fiber, FastLEDs eager Encode-Pfad) und Byte-Vergleich L1 (`leds[]`), L2 (Leitungsbytes RGB) und L2 roh pro Frame.

## Übersicht

| | Anzahl |
|---|---:|
| Beispiele gesamt | 114 |
| Kompiliert | 106 |
| Kompilierfehler | 8 |
| Lauf: ok | 86 |
| Lauf: no-leds | 19 |
| Lauf: timeout | 1 |
| Plattform pc (zeigt im PC-Modus etwas Sinnvolles) | 73 |
| Plattform hardware | 41 |
| Parität nativ = wasm (0 abweichende Bytes) | 103 |
| Parität mit Abweichung | 0 |
| Parität nicht messbar (Referenz-Build/-Lauf) | 2 |

## Nach Kategorie

| Kategorie | gesamt | kompiliert | läuft (ok/budget) | pc | hardware | Parität 0 |
|---|---:|---:|---:|---:|---:|---:|
| Grundlagen | 9 | 9 | 6 | 6 | 3 | 9 |
| Klassische Effekte | 6 | 6 | 6 | 6 | 0 | 6 |
| Farbe & Paletten | 9 | 9 | 9 | 9 | 0 | 9 |
| Noise | 4 | 4 | 4 | 4 | 0 | 4 |
| Feuer | 5 | 5 | 5 | 5 | 0 | 5 |
| Matrix / 2D | 13 | 13 | 12 | 12 | 1 | 13 |
| FX-Bibliothek | 15 | 15 | 13 | 13 | 2 | 15 |
| Mehrere Strips | 5 | 5 | 5 | 5 | 0 | 5 |
| Audio | 12 | 11 | 8 | 3 | 9 | 10 |
| Hardware / Treiber | 19 | 15 | 8 | 6 | 13 | 15 |
| WASM / UI | 4 | 4 | 3 | 3 | 1 | 4 |
| Fortgeschritten | 13 | 10 | 7 | 1 | 12 | 8 |

## Fehlschläge und Abweichungen

| Beispiel | Kategorie | Status | Grund |
|---|---|---|---|
| Asio/ClientValidation | Fortgeschritten | Parität: native-compile-error | network; sketch includes stub_main.hpp (own main()), so no native reference — native-compile-error |
| Asio/RpcBidirectional | Fortgeschritten | Kompilierfehler | needs FastLED networking (HttpStreamServer) and std::thread — RpcBidirectional.ino:46: no member named 'HttpStreamServer' in namespace 'fl::net::http' / RpcBidirectional.ino:50: no member named 'HttpStreamClient' in namespace 'fl::net::http' |
| Asio/RpcClient | Fortgeschritten | Kompilierfehler | needs FastLED networking (HttpStreamClient) — RpcClient.ino:42: no member named 'HttpStreamClient' in namespace 'fl::net::http' / RpcClient.ino:86: no member named 'HttpStreamClient' in namespace 'fl::net::http' |
| Asio/RpcServer | Fortgeschritten | Kompilierfehler | needs FastLED networking (HttpStreamServer) — RpcServer.ino:53: no member named 'HttpStreamServer' in namespace 'fl::net::http' / RpcServer.ino:69: no member named 'HttpStreamServer' in namespace 'fl::net::http' |
| Audio | Audio | Parität: native-crash | audio input (Teensy/ESP32) — native-crash exit 3221225477  |
| AudioFftParity | Audio | Lauf: no-leds | ESP32 DSP self-test — keine LEDs registriert / kein show() |
| AudioInput | Audio | Lauf: no-leds | audio input driver only — keine LEDs registriert / kein show() |
| AutoResearch | Fortgeschritten | Lauf: timeout | hardware test harness; no frame within 90 s — no result after 90 s |
| BlurBenchmark | Fortgeschritten | Lauf: no-leds | benchmark, console only — keine LEDs registriert / kein show() |
| ElPanelReactive | Audio | Lauf: no-leds | EL panels via PWM; LED view only under __EMSCRIPTEN__ — keine LEDs registriert / kein show() |
| Esp8266Uart | Hardware / Treiber | Kompilierfehler | ESP8266-only driver class — Esp8266Uart.ino:19: no member named 'UARTController_ESP8266' in namespace 'fl' |
| Fx/FxLedmapper32x32 | FX-Bibliothek | Lauf: no-leds | video asset / filesystem — keine LEDs registriert / kein show() |
| Fx/FxSdCard | FX-Bibliothek | Lauf: no-leds | SD card / filesystem — keine LEDs registriert / kein show() |
| Multiple/ParallelOutputDemo | Hardware / Treiber | Lauf: no-leds | Teensy parallel output — keine LEDs registriert / kein show() |
| MultipleEsp32SpiBuses | Hardware / Treiber | Lauf: no-leds | ESP32 SPI buses — keine LEDs registriert / kein show() |
| ParallelSPI | Hardware / Treiber | Lauf: no-leds | hardware parallel SPI — keine LEDs registriert / kein show() |
| PerfDisc | Matrix / 2D | Lauf: no-leds | benchmark, console only — keine LEDs registriert / kein show() |
| PinMode | Grundlagen | Lauf: no-leds | GPIO test, console only — keine LEDs registriert / kein show() |
| Pintest | Hardware / Treiber | Lauf: no-leds | pin test, console only — keine LEDs registriert / kein show() |
| Ports/PJRCSpectrumAnalyzer | Audio | Kompilierfehler | needs OctoWS2811.h / Teensy Audio library — PJRCSpectrumAnalyzer.h:25: 'OctoWS2811.h' file not found |
| RGBCalibrate | Grundlagen | Lauf: no-leds | all addLeds() commented out upstream — keine LEDs registriert / kein show() |
| RX | Hardware / Treiber | Lauf: no-leds | ESP32 RMT RX test — keine LEDs registriert / kein show() |
| SIMD | Fortgeschritten | Lauf: no-leds | self-test, console only — keine LEDs registriert / kein show() |
| SmartMatrix | Hardware / Treiber | Lauf: no-leds | SmartMatrix library / Teensy — keine LEDs registriert / kein show() |
| SpecialDrivers/RP/Parallel_IO | Hardware / Treiber | Kompilierfehler | #error outside RP2040/RP2350 — Parallel_IO.ino:52: "This sketch requires RP2040 or RP2350 platform (Raspberry Pi Pico). Use bash compile rp2040 SpecialDrivers/RP instead." |
| SpecialDrivers/Teensy/OctoWS2811/OctoWS2811 | Hardware / Treiber | Kompilierfehler | needs OctoWS2811.h — OctoWS2811_impl.h:11: 'OctoWS2811.h' file not found |
| SpecialDrivers/Teensy/OctoWS2811/OctoWS2811Demo | Hardware / Treiber | Kompilierfehler | needs OctoWS2811.h — OctoWS2811Demo.h:12: 'OctoWS2811.h' file not found |
| Spi | Hardware / Treiber | Lauf: no-leds | hardware SPI bus — keine LEDs registriert / kein show() |
| Test | Grundlagen | Lauf: no-leds | self-test, console only — keine LEDs registriert / kein show() |
| WasmScreenCoords | WASM / UI | Lauf: no-leds | code only under __EMSCRIPTEN__ — keine LEDs registriert / kein show() |

## Alle Beispiele

| Beispiel | Kategorie | Kompilieren | Lauf | LEDs | Geometrie | UI | Plattform | Parität (Bytes) | Hinweis |
|---|---|---|---|---:|---|---:|---|---|---|
| AnalogOutput | Grundlagen | ok | ok | 1 | 1D | 0 | pc | 0 | Eine analoge RGB-LED (PWM-Pins); im PC-Modus als einzelner Farbpunkt mit Farbwechsel. |
| Animartrix | Matrix / 2D | ok | ok | 4096 | 2D 64×64 | 10 | pc | 0 | Läuft im PC-Modus (64×64-Matrix). |
| AnimartrixRing | Matrix / 2D | ok | ok | 244 | 2D frei (ring) | 9 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| Apa102 | Hardware / Treiber | ok | ok | 20 | 1D | 0 | pc | 0 | APA102-Strip (SPI-Chipsatz), statisches Bild; L2-Leitungsbytes nur eingeschränkt. |
| Apa102HD | Hardware / Treiber | ok | ok | 40 | 1D | 0 | pc | 0 | APA102HD-Gamma-Demo (SPI-Chipsatz), statisches Bild; L2-Leitungsbytes nur eingeschränkt. |
| Asio/Client | Fortgeschritten | ok | ok | 10 | 1D | 0 | hardware | 0 | HTTP-Client – kein Netzwerk im PC-Modus; LEDs zeigen nur einen Status. |
| Asio/ClientValidation | Fortgeschritten | ok | ok | 10 | 1D | 0 | hardware | native-compile-error | HTTP-Client-Selbsttest – kein Netzwerk im PC-Modus. |
| Asio/Loopback | Fortgeschritten | ok | ok | 10 | 1D | 0 | hardware | 0 | HTTP-Server-Loopback-Test – kein Netzwerk im PC-Modus. |
| Asio/RpcBidirectional | Fortgeschritten | error | - | - | - | 0 | hardware | - | HTTP-Streaming-RPC (Host-Netzwerk + Threads); nicht verfügbar. |
| Asio/RpcClient | Fortgeschritten | error | - | - | - | 0 | hardware | - | HTTP-Streaming-RPC-Client; nicht verfügbar (kein Netzwerk). |
| Asio/RpcServer | Fortgeschritten | error | - | - | - | 0 | hardware | - | HTTP-Streaming-RPC-Server; nicht verfügbar (kein Netzwerk). |
| Asio/Server | Fortgeschritten | ok | ok | 10 | 1D | 0 | hardware | 0 | HTTP-Server – kein Netzwerk im PC-Modus. |
| Async | Fortgeschritten | ok | ok | 60 | 1D | 0 | pc | 0 | Async-Task-Demo; läuft im PC-Modus (60 LEDs). |
| Audio | Audio | ok | ok | 180 | 1D | 1 | hardware | native-crash | Braucht Audio-Eingang (Mikrofon); im PC-Modus ohne Reaktion. |
| AudioFftParity | Audio | ok | no-leds | 0 | - | 0 | hardware | 0 | Selbsttest des ESP-DSP-FFT-Backends; keine LEDs. |
| AudioInput | Audio | ok | no-leds | 0 | - | 0 | hardware | 0 | I2S-/Teensy-Audio-Eingang; keine LEDs. |
| AudioReactive | Audio | ok | ok | 60 | 1D | 0 | hardware | 0 | Braucht Audio-Eingang (Teensy/ESP32); im PC-Modus dunkel. |
| AudioUrl | Audio | ok | ok | 64 | 1D | 1 | hardware | 0 | Spielt Audio von einer URL ab – kein Netzwerk/Audio im PC-Modus, dunkel. |
| AutoResearch | Fortgeschritten | ok | timeout | - | - | 0 | hardware | - | FastLED-interne Test-/Benchmark-Suite für Hardware; blockiert im PC-Modus. |
| BeatDetection | Audio | ok | ok | 60 | 2D frei (free) | 1 | hardware | 0 | Beat-Erkennung braucht Audio-Eingang; im PC-Modus dunkel. |
| Blink | Grundlagen | ok | ok | 1 | 1D | 0 | pc | 0 | Läuft im PC-Modus (1 LEDs). |
| BlinkParallel | Mehrere Strips | ok | ok | 1024 | 1D | 0 | pc | 0 | Läuft im PC-Modus (1024 LEDs). |
| Blur | Matrix / 2D | ok | ok | 64 | 1D | 0 | pc | 0 | Läuft im PC-Modus (64 LEDs). |
| Blur2d | Matrix / 2D | ok | ok | 484 | 2D 22×22 | 0 | pc | 0 | Läuft im PC-Modus (22×22-Matrix). |
| BlurBenchmark | Fortgeschritten | ok | no-leds | 0 | 2D 8×8 | 0 | hardware | 0 | AVR-Benchmark (Gauß-Blur); nur Konsolenausgabe. |
| Chromancer | Matrix / 2D | ok | ok | 560 | 2D frei (free) | 11 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| Codec | Fortgeschritten | ok | ok | 1024 | 2D 32×32 | 3 | hardware | 0 | Decodiert JPEG/WebP/GIF/MPEG1 per Button; im PC-Modus bleibt die Matrix dunkel. |
| ColorBoost | Farbe & Paletten | ok | ok | 484 | 2D 22×22 | 5 | pc | 0 | Läuft im PC-Modus (22×22-Matrix). |
| ColorPalette | Farbe & Paletten | ok | ok | 50 | 1D | 0 | pc | 0 | Läuft im PC-Modus (50 LEDs). |
| ColorProfile | Farbe & Paletten | ok | ok | 60 | 1D | 0 | pc | 0 | Läuft im PC-Modus (60 LEDs). |
| ColorTemperature | Farbe & Paletten | ok | ok | 60 | 1D | 0 | pc | 0 | Läuft im PC-Modus (60 LEDs). |
| Corkscrew | Matrix / 2D | ok | ok | 288 | 2D frei (free) | 6 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| Cylon | Grundlagen | ok | ok | 64 | 1D | 0 | pc | 0 | Läuft im PC-Modus (64 LEDs). |
| DemoReel100 | Klassische Effekte | ok | ok | 64 | 1D | 0 | pc | 0 | Läuft im PC-Modus (64 LEDs). |
| Downscale | Matrix / 2D | ok | ok | 5120 | 2D frei (free) | 11 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| EaseInOut | Klassische Effekte | ok | ok | 10000 | 2D 100×100 | 5 | pc | 0 | Zeigt die Easing-Kurve als statisches Bild auf 100×100; Kurventyp per UI. |
| ElPanelReactive | Audio | ok | no-leds | 0 | - | 10 | hardware | 0 | Steuert EL-Panels per PWM (keine LEDs); Visualisierung nur im FastLED-Web-Compiler. |
| Esp8266Uart | Hardware / Treiber | error | - | - | - | 0 | hardware | - | ESP8266-UART-Treiber; nur für ESP8266. |
| FestivalStick | Klassische Effekte | ok | ok | 288 | 2D frei (free) | 34 | pc | 0 | Läuft im PC-Modus (288 LEDs, viele UI-Regler). |
| Fire2012 | Feuer | ok | ok | 30 | 1D | 0 | pc | 0 | Läuft im PC-Modus (30 LEDs). |
| Fire2012WithPalette | Feuer | ok | ok | 30 | 1D | 0 | pc | 0 | Läuft im PC-Modus (30 LEDs). |
| Fire2023 | Feuer | ok | ok | 120 | 2D 28×4 | 0 | pc | 0 | Läuft im PC-Modus (28×4-Matrix). |
| FireCylinder | Feuer | ok | ok | 10000 | 2D 100×100 | 8 | pc | 0 | Läuft im PC-Modus (100×100-Matrix). |
| FireMatrix | Feuer | ok | ok | 10000 | 2D 100×100 | 5 | pc | 0 | Läuft im PC-Modus (100×100-Matrix). |
| FirstLight | Grundlagen | ok | ok | 60 | 1D | 0 | pc | 0 | Läuft im PC-Modus (60 LEDs). |
| FlowField | Noise | ok | ok | 4096 | 2D 64×64 | 17 | pc | 0 | Läuft im PC-Modus (64×64-Matrix). |
| Fx/FxCylon | FX-Bibliothek | ok | ok | 64 | 2D frei (free) | 0 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| Fx/FxDemoReel100 | FX-Bibliothek | ok | ok | 64 | 2D frei (free) | 0 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| Fx/FxEngine | FX-Bibliothek | ok | ok | 484 | 2D 22×22 | 3 | pc | 0 | Läuft im PC-Modus (22×22-Matrix). |
| Fx/FxFire2012 | FX-Bibliothek | ok | ok | 92 | 2D frei (free) | 0 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| Fx/FxGfx2Video | FX-Bibliothek | ok | ok | 484 | 2D 22×22 | 0 | pc | 0 | Läuft im PC-Modus (22×22-Matrix). |
| Fx/FxLedmapper32x32 | FX-Bibliothek | ok | no-leds | 0 | 2D 32×32 | 4 | hardware | 0 | Spielt ein Video aus einer Datei (nicht im Bundle, kein Dateisystem); keine LEDs. |
| Fx/FxNoisePlusPalette | FX-Bibliothek | ok | ok | 256 | 2D 16×16 | 2 | pc | 0 | Läuft im PC-Modus (16×16-Matrix). |
| Fx/FxNoiseRing | FX-Bibliothek | ok | ok | 250 | 2D frei (ring) | 6 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| Fx/FxPacifica | FX-Bibliothek | ok | ok | 60 | 2D frei (free) | 0 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| Fx/FxPride2015 | FX-Bibliothek | ok | ok | 200 | 2D frei (free) | 0 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| Fx/FxSdCard | FX-Bibliothek | ok | no-leds | 0 | 2D 32×32 | 4 | hardware | 0 | Spielt ein Video von SD-Karte; kein Dateisystem im PC-Modus. |
| Fx/FxTwinkleFox | FX-Bibliothek | ok | ok | 100 | 1D | 0 | pc | 0 | Läuft im PC-Modus (100 LEDs). |
| Fx/FxWater | FX-Bibliothek | ok | ok | 1024 | 2D 32×32 | 0 | pc | 0 | Läuft im PC-Modus (32×32-Matrix). |
| Fx/FxWave2d | FX-Bibliothek | ok | ok | 4096 | 2D 64×64 | 28 | pc | 0 | Läuft im PC-Modus (64×64-Matrix). |
| Fx/Particles1d | FX-Bibliothek | ok | ok | 210 | 2D frei (ring) | 9 | pc | 0 | Läuft im PC-Modus (2D-Layout). |
| HD107 | Hardware / Treiber | ok | ok | 40 | 1D | 0 | pc | 0 | HD107S-Strip (SPI-Chipsatz), statisches Bild; L2-Leitungsbytes nur eingeschränkt. |
| HSVTest | Farbe & Paletten | ok | ok | 5 | 1D | 4 | pc | 0 | Läuft im PC-Modus (5 LEDs). |
| hydropack | Audio | ok | ok | 18 | 2D frei (free) | 5 | hardware | 0 | Musikreaktiv (ESP32 + Audio); im PC-Modus dunkel. |
| Json | WASM / UI | ok | ok | 100 | 1D | 0 | pc | 0 | JSON-API-Demo; läuft im PC-Modus (100 LEDs), Details in der Konsole. |
| LuminescentGrand | Audio | ok | ok | 1760 | 2D frei (free) | 5 | pc | 0 | Klavier-Visualisierung (1.760 LEDs); ohne MIDI-Eingang nur Demo-Animation. |
| Luminova | Matrix / 2D | ok | ok | 1024 | 2D 32×32 | 0 | pc | 0 | Läuft im PC-Modus (32×32-Matrix). |
| MoodRing | Audio | ok | ok | 244 | 2D frei (ring) | 22 | pc | 0 | Ring mit 244 LEDs; ohne Mikrofon im Fallback-Modus (ruhige Animation). |
| Multiple/ArrayOfLedArrays | Mehrere Strips | ok | ok | 180 | 1D | 0 | pc | 0 | Läuft im PC-Modus (180 LEDs). |
| Multiple/MirroringSample | Mehrere Strips | ok | ok | 240 | 1D | 0 | pc | 0 | Läuft im PC-Modus (240 LEDs). |
| Multiple/MultiArrays | Mehrere Strips | ok | ok | 96 | 1D | 0 | pc | 0 | Läuft im PC-Modus (96 LEDs). |
| Multiple/MultipleStripsInOneArray | Mehrere Strips | ok | ok | 180 | 1D | 0 | pc | 0 | Läuft im PC-Modus (180 LEDs). |
| Multiple/ParallelOutputDemo | Hardware / Treiber | ok | no-leds | 0 | - | 0 | hardware | 0 | Teensy-Parallelausgabe; im PC-Modus keine LEDs. |
| MultipleEsp32SpiBuses | Hardware / Treiber | ok | no-leds | 0 | - | 0 | hardware | 0 | Zwei ESP32-SPI-Busse; keine LEDs im PC-Modus. |
| Noise | Noise | ok | ok | 256 | 2D 16×16 | 0 | pc | 0 | Läuft im PC-Modus (8×8-Matrix). |
| NoisePlayground | Noise | ok | ok | 256 | 2D 16×16 | 0 | pc | 0 | Läuft im PC-Modus (16×16-Matrix). |
| NoisePlusPalette | Noise | ok | ok | 256 | 2D 16×16 | 0 | pc | 0 | Läuft im PC-Modus (16×16-Matrix). |
| OTA | Hardware / Treiber | ok | ok | 60 | 1D | 0 | hardware | 0 | Over-the-Air-Update für ESP32 (WLAN); im PC-Modus nur statisches Bild. |
| Overclock | Hardware / Treiber | ok | ok | 484 | 2D 22×22 | 0 | pc | 0 | Läuft im PC-Modus (22×22); das eigentliche Thema (Übertakten des Treibers) ist Hardware. |
| Pacifica | Klassische Effekte | ok | ok | 60 | 1D | 0 | pc | 0 | Läuft im PC-Modus (60 LEDs). |
| ParallelSPI | Hardware / Treiber | ok | no-leds | 0 | - | 0 | hardware | 0 | Paralleler SPI-Bus (Hardware); keine LEDs im PC-Modus. |
| PerfDisc | Matrix / 2D | ok | no-leds | 1 | 1D | 0 | hardware | 0 | AVR-Benchmark (drawDisc); nur Konsolenausgabe. |
| PinMode | Grundlagen | ok | no-leds | 0 | - | 0 | hardware | 0 | Prüft pinMode/digitalWrite/digitalRead; nur Konsolenausgabe. |
| Pintest | Hardware / Treiber | ok | no-leds | 0 | - | 0 | hardware | 0 | Pin-Test für AVR/Teensy; nur Konsolenausgabe. |
| Ports/PJRCSpectrumAnalyzer | Audio | error | - | - | - | 0 | hardware | - | Teensy-only (OctoWS2811 + Audio-Bibliothek). |
| Pride2015 | Klassische Effekte | ok | ok | 200 | 1D | 0 | pc | 0 | Läuft im PC-Modus (200 LEDs). |
| Remote | Fortgeschritten | ok | ok | 10 | 1D | 0 | hardware | 0 | RPC-Demo über serielle Schnittstelle; ohne Gegenstelle dunkel. |
| RGBCalibrate | Grundlagen | ok | no-leds | 0 | - | 0 | hardware | 0 | Alle addLeds()-Zeilen sind im Original auskommentiert – erst einen Chipsatz einkommentieren. |
| RGBSetDemo | Grundlagen | ok | ok | 40 | 1D | 0 | pc | 0 | Läuft im PC-Modus (40 LEDs). |
| RGBW | Farbe & Paletten | ok | ok | 10 | 1D | 0 | pc | 0 | Läuft im PC-Modus (10 LEDs). |
| RGBWColorimetric | Farbe & Paletten | ok | ok | 60 | 1D | 0 | pc | 0 | Läuft im PC-Modus (60 LEDs). |
| RGBWEmulated | Farbe & Paletten | ok | ok | 25 | 1D | 0 | pc | 0 | Läuft im PC-Modus (25 LEDs). |
| RGBWW | Farbe & Paletten | ok | ok | 60 | 1D | 0 | pc | 0 | Läuft im PC-Modus (60 LEDs). |
| RX | Hardware / Treiber | ok | no-leds | 0 | - | 0 | hardware | 0 | ESP32-RX-Kanaltest (Draht zwischen TX und RX nötig); keine LEDs. |
| Sailboat | Audio | ok | ok | 200 | 1D | 14 | pc | 0 | Läuft im PC-Modus (200 LEDs); ohne Audio-Eingang nur Grundanimation. |
| SIMD | Fortgeschritten | ok | no-leds | 0 | - | 0 | hardware | 0 | Selbsttest der SIMD-Funktionen; nur Konsolenausgabe. |
| SmartMatrix | Hardware / Treiber | ok | no-leds | 0 | - | 0 | hardware | 0 | SmartMatrix-HUB75-Panel (Teensy); keine LEDs im PC-Modus. |
| SpecialDrivers/Adafruit/AdafruitBridge | Hardware / Treiber | ok | ok | 10 | 1D | 0 | pc | 0 | Adafruit-NeoPixel-Bridge; läuft im PC-Modus (10 LEDs). |
| SpecialDrivers/ESP/DriverTest | Hardware / Treiber | ok | ok | 60 | 1D | 0 | hardware | 0 | Testet die ESP32-LED-Treiber; im PC-Modus dunkel. |
| SpecialDrivers/RP/Parallel_IO | Hardware / Treiber | error | - | - | - | 0 | hardware | - | Nur RP2040/RP2350 (Pico). |
| SpecialDrivers/Teensy/ObjectFLED/TeensyMassiveParallel | Hardware / Treiber | ok | ok | 60 | 1D | 0 | pc | 0 | Teensy-Beispiel im Fallback-Modus: läuft im PC-Modus (60 LEDs). |
| SpecialDrivers/Teensy/OctoWS2811/OctoWS2811 | Hardware / Treiber | error | - | - | - | 0 | hardware | - | Teensy-only (OctoWS2811-Bibliothek). |
| SpecialDrivers/Teensy/OctoWS2811/OctoWS2811Demo | Hardware / Treiber | error | - | - | - | 0 | hardware | - | Teensy-only (OctoWS2811-Bibliothek). |
| Spi | Hardware / Treiber | ok | no-leds | 0 | - | 0 | hardware | 0 | SPI-Treiber-Demo ohne LED-Controller im PC-Modus. |
| Test | Grundlagen | ok | no-leds | 0 | - | 0 | hardware | 0 | Selbsttest der 8-Bit-Mathematik; nur Konsolenausgabe. |
| TwinkleFox | Klassische Effekte | ok | ok | 100 | 1D | 0 | pc | 0 | Läuft im PC-Modus (100 LEDs). |
| UITest | WASM / UI | ok | ok | 16 | 1D | 2 | pc | 0 | Läuft im PC-Modus (16 LEDs). |
| wasm | WASM / UI | ok | ok | 1024 | 2D 32×32 | 12 | pc | 0 | Läuft im PC-Modus (32×32-Matrix). |
| WasmScreenCoords | WASM / UI | ok | no-leds | 0 | - | 0 | hardware | 0 | Nur im FastLED-Web-Compiler (Code steht hinter #ifdef __EMSCRIPTEN__); hier leer. |
| Wave | Matrix / 2D | ok | ok | 100 | 1D | 8 | pc | 0 | Läuft im PC-Modus (100 LEDs); Wellen per UI-Button „Trigger“ auslösen. |
| Wave2d | Matrix / 2D | ok | ok | 10000 | 2D 100×100 | 11 | pc | 0 | Läuft im PC-Modus (100×100-Matrix). |
| WS2816 | Grundlagen | ok | ok | 3 | 1D | 0 | pc | 0 | Läuft im PC-Modus (3 LEDs). |
| XYMatrix | Matrix / 2D | ok | ok | 256 | 2D 16×16 | 0 | pc | 0 | Läuft im PC-Modus (16×16-Matrix). |
| XYPath | Matrix / 2D | ok | ok | 4096 | 2D 64×64 | 5 | pc | 0 | Pfad-Rendering auf 64×64; Bild ändert sich über die UI-Regler. |
