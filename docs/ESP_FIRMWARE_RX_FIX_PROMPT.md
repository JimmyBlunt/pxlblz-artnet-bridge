# Auftrag: ESP32-Firmware – Art-Net-Paketverlust bei 60 fps beheben (Empfangs-Task)

Repo: https://github.com/JimmyBlunt/ArtNet-Controller-ESP-Teensy41 (Stand `24e609b`)
Projekt: `firmware/esp32_artnet` (PlatformIO). Arbeite auf einem **neuen Branch**
`fix/esp32-artnet-rx-task`. Nicht auf `main` pushen.

## Problem (gemessen am 2026-10-01/02)

Controller **10.0.0.251** (`hardwareProfile` `flex8-ws2812-apa102`, Build-Env
`esp32-wifi-esp251`): 2 APA102-Ausgänge, 256 + 549 LEDs, Universes 156–161 (6 Pakete pro
Frame), `targetFps` 60, WLAN.

Bei 60 fps vom Art-Net-Router (360 Pakete/s) kommen nur **96 %** der Pakete an
(7482 gesendet / 7186 empfangen), ~1 unvollständiger Frame pro Sekunde, an den LEDs
~56 fps statt 60. Die Firmware selbst meldet 0 `droppedPackets` und 0 `sequenceErrors`.
Bei 30 fps (180 Pakete/s) nur 0,2 % Verlust. Ein zweiter ESP (10.0.0.248, 1 Paket/Frame,
2 ms Ausgabe) im selben WLAN verliert 0 %. Standortwechsel, Doppelt-Senden und
Paketabstand auf Senderseite haben **nichts** verbessert. Der Verlust hängt an der Paketrate.

## Ursache

`firmware/src/main.cpp` → `loop()` macht alles nacheinander auf **einem** Task:
`webApi.handleClient()`, Art-Net-Empfang (`artnet.poll()` = `WiFiUDP::parsePacket()`,
max. 32 pro Durchlauf) und `outputs.show()`. `show()` blockiert bei .251 **~12,4 ms**
(`outputTimeUs`; APA102 mit fest eingebautem `DATA_RATE_MHZ(4)` in `LedOutputs.cpp`).
In dieser Zeit holt niemand Pakete ab; die lwIP-UDP-Empfangswarteschlange (Arduino-Default
klein, ~6 Pakete) läuft bei 6 Paketen/Frame und 60 fps über → Pakete werden im Stack
verworfen, ohne dass die Firmware es zählt.

`WiFi.setSleep(false)` ist bereits gesetzt (`NetworkManager.cpp`) – nicht die Ursache.

## Umsetzung

1. **Eigener Empfangs-Task** (FreeRTOS, `xTaskCreatePinnedToCore` auf **Core 0**;
   `loop()` läuft auf Core 1), höhere Priorität als `loop`:
   - liest Art-Net-Pakete **sofort** ab, unabhängig von `outputs.show()`;
   - bevorzugt eigener lwIP/BSD-UDP-Socket (`lwip/sockets.h`, `bind` Port 6454,
     **blockierendes** `recvfrom` mit kurzem Timeout, `SO_RCVBUF` vergrößern falls verfügbar)
     statt Polling; wenn das mit dem Arduino-Core nicht sauber geht, `WiFiUDP` **nur noch**
     in diesem Task benutzen (Polling mit `vTaskDelay(1)` nur wenn nichts anliegt);
   - übergibt **rohe Pakete** (max. 530 Byte) über eine `xQueue` fester Größe
     (z. B. 64 Einträge, vorallokierte Puffer, keine Heap-Allokation pro Paket) an `loop()`;
   - zählt Überläufe der Queue (neuer Status-Zähler, z. B. `rxQueueDrops`) und die
     empfangenen Pakete.
2. `loop()` leert die Queue **vollständig** (nicht auf 32 begrenzt) und füttert wie bisher
   `UniverseAssembler` (`parseArtDmx` → `assembler->accept`). Der Assembler bleibt
   single-threaded in `loop()` – **kein** gemeinsamer Zugriff aus zwei Tasks.
   Verhalten bei `hardwareTest.active()` (verwerfen) und `assembler->expire()` unverändert.
3. Bestehendes Verhalten nicht ändern: Frame-Vollständigkeit, Sequenz-Logik, Blackout,
   Web-API, `/update` (OTA), Config-Speicherung (NVS), WLAN-Boot-Guard.
4. **Optional, nur falls ohne Bildfehler:** APA102-Datenrate für die tatsächlich benutzten
   Pins erhöhen (z. B. 8–12 MHz) bzw. `spiHz` aus der Config wirklich verwenden. Nur
   umsetzen, wenn sauber testbar; sonst in der Abschlussmeldung als Folgeschritt nennen.

## Tests und Build

- Vorhandene Native-Tests laufen lassen (`[env:native]`), neue Tests für die Queue-Übergabe
  (Reihenfolge, voller Queue → Zähler, keine Paketzerstückelung), soweit ohne Hardware möglich.
- Bauen: `esp32-wifi-esp251` (für 10.0.0.251) und das Env, dessen `hardwareProfile`
  `esp32-wroom-flex-8ws-2apa` ist (Test-ESP 10.0.0.248; siehe `WebApi.cpp`, Profil-Makros) –
  beide müssen fehlerfrei bauen.
- `firmware/include/WifiSecrets.h` (SSID/Passwort) liegt nur lokal und darf **nicht**
  committet werden; prüfe vor dem Build, dass sie existiert, und gib die Zugangsdaten
  nie aus.

## Flashen (OTA) – Reihenfolge

1. Zuerst **10.0.0.248** über `http://10.0.0.248/update` mit dem passenden `firmware.bin`.
   Nach dem Neustart prüfen: erreichbar, `GET /api/config` unverändert (Ausgang 6 APA102,
   136 LEDs, U149, `targetFps` 60), `GET /api/status` zählt Pakete.
2. Erst danach **10.0.0.251** (`.pio/build/esp32-wifi-esp251/firmware.bin`).
   Danach prüfen: `/api/config` unverändert (2× APA102, 256/549 LEDs, U156/U158,
   `targetFps` 60, `saved`/`matches` true).
3. Fällt ein Gerät aus dem WLAN: USB-Recovery laut `docs/WIFI_BOOT_RECOVERY.md` (COM6).

## Abnahme (misst der PXLBLZ-PC mit dem laufenden Art-Net-Router)

Router mit 60 fps, Testmuster Regenbogen, 30 s:
- 10.0.0.251: **≥ 99,9 %** der Pakete empfangen, **≥ 59,5** vollständige Frames/s,
  unvollständig ≈ 0, an den LEDs ≈ 60 fps.
- 10.0.0.248: unverändert ≥ 59 fps, 0 % Verlust.

## Abschluss

Commit(s) auf `fix/esp32-artnet-rx-task` mit Beschreibung, Branch pushen. Melde:
geänderte Dateien, Build-Ergebnisse beider Envs, Testergebnisse, welche Geräte geflasht
wurden, Status der beiden Controller nach dem Flashen, und ob Punkt 4 (SPI-Rate) umgesetzt
wurde.
