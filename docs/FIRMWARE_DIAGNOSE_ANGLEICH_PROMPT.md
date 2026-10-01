# Auftrag: Diagnosewerte der ESP32-Firmware an die Teensy-Firmware angleichen

Repo: https://github.com/JimmyBlunt/ArtNet-Controller-ESP-Teensy41 – Projekt
`firmware/esp32_artnet` (PlatformIO). Neuer Branch `feat/esp32-status-parity` (auf dem Stand
von `fix/esp32-artnet-rx-task`). Nicht auf `main` pushen.

## Ausgangslage (gemessen 2026-10-02)

`GET /api/status` liefert sehr unterschiedliche Informationen:

- **Teensy 4.1** (`teensy41-octo-web-rx32`, 10.0.0.253): ~90 Felder – Firmware/Build,
  Zustand, Konfiguration pro Ausgang, Netz, UDP-Queue inkl. Spitzenwert, Bildraten
  (empfangen / ausgegeben), Ausgabezeit aktuell + Maximum, Art-Net-Zähler
  (complete, incomplete, ignored, duplicates, overwritten, stale, rejected), HTTP-Statistik.
- **ESP32** (10.0.0.251 / 10.0.0.248): 21 Felder – `pixelCount, universeCount, packets,
  rxPackets, rxQueueDrops, rxOversizedPackets, rxSocketErrors, framesComplete,
  framesIncomplete, fps, packetsPerSecond, heapFree, heapMinFree, frameTimeUs, outputTimeUs,
  lastFrameLatencyMs (immer null), startUniverse, outputFrames, timeouts, droppedPackets,
  sequenceErrors`. Keine Firmware-Version, kein Zustand, keine Netz-/WLAN-Daten, keine
  Maximalwerte, keine Angabe pro Ausgang.

Ziel: Beide Firmwares sollen für dieselben Fragen dieselben Felder (gleiche Namen, gleiche
Bedeutung) liefern, damit Router-Einstellungsseite und Messskript (`perf-test/
installation-fps-test.mjs`) alle Nodes gleich auswerten können.

## Umsetzung (ESP32)

`GET /api/status` **ergänzen** – alle bisherigen Felder bleiben unverändert erhalten
(Abwärtskompatibilität). Neue Felder mit den **Teensy-Namen**:

| Gruppe | Felder | Bemerkung |
| --- | --- | --- |
| Identität | `firmware`, `build_revision` (Git-Hash + Datum), `board_profile` (= hardwareProfile), `uptime_s`, `reset_reason` | Build-Infos per `build_flags` einbetten |
| Zustand | `state` (z. B. `ARTNET_RUNNING`, `TEST`, `IDLE`), `unsaved` (Config ≠ NVS), `last_error` | |
| Netz | `ip`, `mac`, `link`, `wifi_rssi`, `wifi_channel`, `wifi_bssid`, `wifi_reconnects`, `udp_listening` | WLAN-Felder nur bei WLAN-Profilen, bei Ethernet `link` aus dem PHY |
| Konfiguration | `led_type` (pro Ausgang als Liste), `lengths`, `start_universes`, `color_orders`, `pins`, `target_fps`, `expected_universes`, `total_pixels`, `brightness`, `spi_hz` (tatsächlich benutzte SPI-Rate je APA102-Ausgang) | Listen in Ausgangs-Reihenfolge, deaktivierte Ausgänge weglassen oder `enabled`-Liste |
| Empfang | `udp_received` (= rxPackets), `udp_queue_drops` (= rxQueueDrops), `queue_peak` (höchste Queue-Füllung seit Start), `artnet_packets` (= packets), `artnet_ignored` (fremde Universes/Opcodes), `artnet_duplicates`, `artnet_overwritten` (vollständiger Frame ersetzt, bevor er ausgegeben wurde), `artnet_incomplete`, `artnet_stale` (= timeouts) | Zähler monoton seit Start |
| Raten | `artnet_complete_fps`, `output_fps` (tatsächlich ausgegebene Frames/s), `packets_per_second` | über 1 s gemittelt |
| Zeiten | `output_us`, `max_output_us`, `loop_us_max`, `frame_latency_us` (Empfang letztes Paket → Ausgabe-Start; ersetzt das immer leere `lastFrameLatencyMs`) | Maxima mit `POST /api/status/reset` zurücksetzbar |
| Speicher | `heap_free`, `heap_min_free` | |
| HTTP | `http_requests`, `http_max_us` | |

Zusätzlich auf dem Teensy (kleiner Teil, gleicher Branch-Name im Teensy-Projekt):
`uptime_s`, `reset_reason`, `output_fps` (= `dma_fps`) und `queue_peak` sind schon bzw. fast
vorhanden – nur fehlende Namen ergänzen, nichts umbenennen.

Regeln:
- Kein Zähler darf den Art-Net-Pfad bremsen: nur atomare Inkremente im Empfangs-Task, die
  JSON-Erzeugung läuft im Web-Kontext.
- `WifiSecrets.h` niemals ausgeben (keine SSID/Passwörter in `/api/status`).
- Web-UI der Firmware: die neuen Werte in einer Diagnose-Ansicht anzeigen (optional).

## Tests und Build

- Native-Tests (`[env:native]`) für die JSON-Erzeugung (Feldnamen vorhanden, Typen).
- Bauen: `esp32-wifi-esp251` und das Env für 10.0.0.248 (`esp32-wroom-flex-8ws-2apa`).
- Flashen per OTA zuerst .248, dann .251 (wie beim RX-Fix); danach `/api/config` unverändert,
  `/api/storage` `saved`/`matches` true.

## Abnahme

`GET http://10.0.0.251/api/status` und `http://10.0.0.253/api/status` enthalten alle Felder
der Tabelle mit plausiblen Werten; bei 60 fps vom Router zeigt .251 `artnet_complete_fps`
≈ 60 und `output_fps` ≈ Bildrate an den LEDs. Danach passt der PXLBLZ-PC Messskript und
Einstellungsseite an die gemeinsamen Namen an.

## Abschluss

Commit(s) auf `feat/esp32-status-parity`, Branch pushen. Melde geänderte Dateien,
Build-Ergebnisse, Beispiel-JSON beider Controller nach dem Flashen.
