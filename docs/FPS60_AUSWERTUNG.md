# 60 FPS in der Installation – Messung und Optimierungsoptionen

Stand 2026-10-01, Router v0.3 (`feature/v0.3-multi-controller`), gemessen über den laufenden
Router (Einstellungs-API + Testmuster, kein zweiter Sender) mit
`perf-test/installation-fps-test.mjs`. Muster: Regenbogen (alle Universes ändern sich).

## Ergebnis

| Controller | Anbindung | LEDs / Universes | 30 FPS | 60 FPS vom Router: vollständig empfangen | 60 FPS an den LEDs | LED-Ausgabe pro Frame |
| --- | --- | --- | --- | --- | --- | --- |
| ESP32 Testrig **10.0.0.248** (APA102) | WLAN | 136 / 1 | 100 % | 60,0 fps, 0–0,1 % Verlust | **59,3 fps** ✓ | 2,2 ms |
| Teensy 4.1 Octo **10.0.0.253** (WS2812B, 8 Ausgänge) | Kabel (Ping 1 ms) | 4593 / 32 | 100 % | **60,0 fps, 0 % Verlust** | 29,9 fps | **29,3 ms** |
| ESP32 flex8 **10.0.0.251** (APA102, 2 Ausgänge) | WLAN (Ping 4–6 ms) | 805 / 6 | 100 % | 57,5–58,1 fps, **3–4 % Verlust** | 29,5 fps (Ziel 30) · **56,1–56,8 fps** mit Ziel 60 | 12,4 ms |

Router: in jedem Lauf exakt 30,0 bzw. 60,0 fps pro Controller, 0 Sendefehler.
**Der Router und das Kabelnetz sind kein Engpass.**

## Engpass 1: Teensy 10.0.0.253 – LED-Ausgabe (Physik)

WS2812B werden mit 800 kHz beschrieben: 30 µs pro LED, plus ~300 µs Latch. Die acht Ausgänge
laufen parallel, die Ausgabezeit bestimmt der **längste Strang**:

| Ausgang | OUT1 | OUT2 | **OUT3** | OUT4 | OUT5 | OUT6 | OUT7 | OUT8 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| LEDs | 203 | 738 | **880** | 810 | 352 | 610 | 512 | 488 |

880 × 30 µs + 300 µs = 26,7 ms (gemessen 29,3 ms DMA) → **höchstens ~34 fps**, egal welche
FPS eingestellt sind. Für 60 fps (16,7 ms) darf der längste Strang **höchstens ~540 LEDs** haben.
Auch perfekt gleichmäßig auf 8 Ausgänge verteilt (4593 / 8 = 574) reicht es knapp nicht (~57 fps).

| Option | Wirkung | Aufwand |
| --- | --- | --- |
| A. Ziel-FPS des Teensy von 30 auf 60 (die Firmware gibt dann aus, so schnell die DMA erlaubt) | 30 → **~34 fps** | Einstellung; geht nur im gestoppten Zustand (`config_locked` während Art-Net läuft) |
| B. Stränge innerhalb der 8 Ausgänge umverteilen (OUT3/OUT4/OUT2 kürzen, OUT1/OUT5 verlängern) | max. ~574 LEDs → **~57 fps** | Verkabelung (Einspeisepunkte ändern) + Controller-/Router-Routen |
| C. **9 oder mehr parallele Ausgänge** (Teensy 4.1 kann über ObjectFLED mehr Pins als die 8 des Octo-Adapters treiben; zusätzliche Pegelwandler) | Stränge ≤ ~510 LEDs → **60 fps** | Hardware + Firmware-Pinbelegung |
| D. Zweiter Controller für einen Teil der Backpanels | Stränge halbiert → **60 fps** | Hardware |
| WS2812-Timing übertakten | riskant (lange Leitungen, Fehlfarben) | nicht empfohlen |

## Engpass 2: APA102 10.0.0.251 – WLAN-Paketverlust

- Die LED-Ausgabe ist kein Problem: 12,4 ms bei 4 MHz SPI (APA102 vertragen deutlich mehr;
  8 MHz würden die Zeit etwa halbieren – für 60 fps nicht nötig).
- Mit Ziel-FPS 60 am Controller kommen **56–57 fps** an den LEDs an. Es fehlen genau die Frames,
  bei denen eines der 6 Pakete im WLAN verloren geht: gesendet 7482, angekommen 7186 Pakete
  (**96 %**); die Firmware selbst verwirft nichts (0 dropped, 0 Sequenzfehler).
- Der ESP-Testrig im selben WLAN (1 Paket pro Frame) verliert nichts – .251 hat 6 Pakete pro
  Frame und einen schlechteren Funkweg.

Router-seitig ausprobiert und **ohne Wirkung** (wieder entfernt):

| Versuch | Verlust |
| --- | --- |
| normal | 3,2–4,0 % |
| jeden Frame doppelt senden | 5,1 % (Duplikate gehen genauso verloren, Firmware zählt sie als Fehler) |
| Pakete mit 0,5 ms Abstand | 4,0 % |
| Pakete mit 1,5 ms Abstand | 6,0 % |

| Option | Wirkung | Aufwand |
| --- | --- | --- |
| E. Ziel-FPS von .251 auf 60 (Übernehmen **und Speichern**) | 30 → **~56 fps** sofort | Einstellung |
| F. .251 per Kabel (Ethernet-fähiges Board; die ESP32-Firmware kennt RMII/W5500-Profile) | Verlust → 0 → **60 fps** | Hardware |
| G. Besserer WLAN-Empfang (Access Point näher, eigener 2,4-GHz-Kanal) | Verlust ↓ | Netz |

## Empfehlung

1. **Router: alle Controller auf `fps_target` 60** – kostet nichts (Kabel verlustfrei), die
   Controller bekommen immer das neueste Bild (geringere Latenz), auch wenn sie selbst langsamer
   ausgeben.
2. **.251 Ziel-FPS 60** am Controller (Option E) → ~56 fps; danach Kabel/WLAN (F/G) für saubere 60.
3. **Teensy .253:** kurzfristig Option A (~34 fps). Für echte 60 fps ist Hardware nötig:
   Option C (mehr parallele Ausgänge) oder D (zweiter Controller); B bringt ~57 fps ohne neue
   Hardware, nur mit Umverkabelung.
4. PXLBLZ muss selbst 60 fps rendern: das Studio-Fenster im Vordergrund lassen (die Statusleiste
   „ArtNet · … Bilder/s“ zeigt es).

## Umgesetzt (2026-10-01, Sofortmaßnahmen)

- Router: alle drei Controller `fps_target` 60 (gespeichert, Config im Repo nachgezogen).
- .251: Ziel-FPS 60 übernommen und gespeichert (vorherige Config gesichert in
  `PXLBLZ_ArtNet_Handoff_2026-09-26\esp-config-backup\esp-10.0.0.251-config-before-fps60-2026-10-01.json`).

Kontrollmessung danach (Regenbogen, 20 s): ESP .248 **59,5 fps** an den LEDs, Teensy .253 60,0 fps
vollständig empfangen / 30,0 fps an den LEDs (bis Option A–D), APA .251 **56,2 fps** an den LEDs
(4,3 % WLAN-Verlust).

### Nachtrag: Teensy auf 60, .251 umgestellt (2026-10-01)

- **Teensy .253:** Ziel-FPS 60 (Option A) über seine API: `/api/stop` → `/api/config` →
  `/api/save` → `/api/start` (ein paar Sekunden dunkel; vorherige Config gesichert in
  `esp-config-backup\teensy-10.0.0.253-config-before-fps60-2026-10-01.json`).
  Ergebnis: **34,1 fps an den LEDs** (vorher 30), DMA 29,3 ms, empfangen weiter 60/60 vollständig.
- **.251 an einen anderen Platz gestellt:** Verlust unverändert (3,3–3,9 %, ~1 unvollständiger
  Frame/s), Ping 0 % / 4–8 ms. Der Verlust hängt an der **Paketrate**, nicht am Funkweg:

  | Pakete/s zum ESP | Verlust |
  | --- | --- |
  | .248 bei 60 fps (60/s) | 0 % |
  | .251 bei 30 fps (180/s) | 0,2 % |
  | .251 bei 60 fps (360/s) | ~4 % |

  Erste Vermutung war der WLAN-Stromsparmodus – `WiFi.setSleep(false)` ist in der Firmware aber
  bereits gesetzt.

Stand an den LEDs bei 60 fps: .248 **59,3**, .253 **34,1**, .251 **57,3** fps.

### Nachtrag 2026-10-02: Ursache .251 in der Firmware gefunden

Firmware-Quellen: https://github.com/JimmyBlunt/ArtNet-Controller-ESP-Teensy41
(`firmware/esp32_artnet`, Env `esp32-wifi-esp251`). In `loop()` laufen Web-API, Art-Net-Empfang
(`WiFiUDP::parsePacket`, max. 32 pro Durchlauf) und `outputs.show()` nacheinander. `show()`
blockiert 12,4 ms (APA102 fest auf `DATA_RATE_MHZ(4)`); in der Zeit läuft die kleine
lwIP-Empfangswarteschlange bei 6 Paketen/Frame über – der Stack verwirft, die Firmware zählt es
nicht. Abhilfe: eigener Empfangs-Task auf Core 0 mit Queue an `loop()`, optional höhere SPI-Rate.
Fertiger Auftrag für den Firmware-PC: [`ESP_FIRMWARE_RX_FIX_PROMPT.md`](ESP_FIRMWARE_RX_FIX_PROMPT.md).
Abnahme nach dem Flashen mit `perf-test/installation-fps-test.mjs --fps 60`: .251 ≥ 99,9 %
Pakete, ≥ 59,5 vollständige Frames/s.

Port-ID-Test (jeder Ausgang in seiner Portfarbe) auf .253 und .251: Universes und Farbreihenfolge
korrekt. Aktive Router-Config der Installation: `router/config/routes.installation-live.json`
(Controller-Übersicht: `controller-reference/INSTALLATION_CONTROLLERS.md`).
