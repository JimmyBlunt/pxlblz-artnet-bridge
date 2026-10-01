# Konfigurationshilfe: PXLBLZ → Art-Net-Router → Controller

Diese Anleitung erklärt, **wo welche Adresse, welcher Port und welches Routing-Ziel
eingestellt wird** und welche Schritte bei typischen Änderungen nötig sind
(neue Controller-IP, andere LED-Anzahl, weiterer Controller, andere Config).

Stand: v0.3 (`feature/v0.3-multi-controller`), Windows-Starter `windows-launcher-artnet`.

---

## 0. Der einfache Weg: Einstellungsseite des Routers

Solange der Router läuft, gibt es eine zentrale Einstellungsseite im Browser:

```text
http://127.0.0.1:9988/        (Tab-Titel „PXLBLZ-IDE~ArtNet · Router“)
```

Dort lässt sich alles aus diesem Dokument ohne Texteditor und **ohne Neustart** erledigen:

| Aufgabe | Auf der Seite |
| --- | --- |
| Controller-IP geändert | Abschnitt **Controller** → IP-Feld ändern (alle Routen ziehen mit) – oder **Controller im Netz suchen** → beim gefundenen Gerät „IP übernehmen für …“ |
| Neuer Controller | **Controller im Netz suchen** → „Als Controller übernehmen“ (Ausgänge, LED-Anzahl und Universes kommen direkt vom Controller) |
| LED-Anzahl am Controller geändert | beim Controller „Ausgänge vom Controller übernehmen“ |
| Routen, Universes, Farben, FPS, Eingang | direkt in den Tabellen/Feldern |
| Prüfen | Knopf **Prüfen** (zeigt Fehler und die geplante Ausgabe) |
| Aktivieren | **Übernehmen & speichern** – wirkt sofort, die vorige Datei wird in `configackups\` gesichert (die letzten 20) |
| Verdrahtung/Ausgabe testen | Abschnitt **Testmuster** (Lauflicht, Port-ID, Regenbogen, Weiß, Schwarz) – PXLBLZ übernimmt danach automatisch wieder |
| Kontrolle | jede Controller-Karte zeigt live „Router sendet … fps“ und „Controller meldet … vollständig/s · … unvollständig/s“ |

Die Seite ist nur vom eigenen PC erreichbar (127.0.0.1). Die gespeicherte Datei ist die,
mit der der Router gestartet wurde (Pfad unten auf der Seite).

**Port 9988, nicht 9981:** TouchDesigner belegt 9981. Belegt ein anderes Programm den Port,
meldet der Router beim Start `WARNING: port … answers for ANOTHER program`; dann mit
`--status-listen 127.0.0.1:<freier Port>` starten.

Die folgenden Abschnitte erklären, was die Seite im Hintergrund tut, und wie es ohne die
Seite geht.

---

## 1. Der Datenweg auf einen Blick

```text
 PXLBLZ-Studio (Chrome, eigenes Profil)          Pattern + Map -> RGB-Frame (n Pixel)
   http://localhost:5175/PXLBLZ-IDE/studio?pxout=1&pxoutUrl=ws://127.0.0.1:9980/pixels
        │  WebSocket, 1 Nachricht = 1 kompletter RGB-Frame
        ▼
 pxlblz-router.exe  (E:\PXLBLZ-ArtNet\artnet\)
   liest:  config\<deine-config>.json
   lauscht: ws://127.0.0.1:9980/pixels   Status: http://127.0.0.1:9988/status
        │  Art-Net (ArtDmx, UDP-Port 6454, Unicast)
        ▼  an target_ip jedes Controllers
 Art-Net-Controller (ESP32 / Teensy), z. B. 10.0.0.248
   Weboberfläche / API: http://<controller-ip>/   (/api/config, /api/status)
   Ausgang 6: APA102, 136 LEDs, Universe 149
        │
        ▼
 LEDs
```

**Wichtig:** PXLBLZ kennt die Controller-IP **nicht**. PXLBLZ schickt immer nur an
den lokalen Router (`127.0.0.1:9980`). **Die Controller-IP steht ausschließlich in der
Router-Config.** Ändert sich die IP eines Controllers, wird nur die Router-Config
angepasst – an PXLBLZ und an der Map ändert sich nichts.

---

## 2. Welche Einstellung steht wo?

| Was | Wo | Beispiel |
| --- | --- | --- |
| **Controller-IP** (Ziel der Art-Net-Pakete) | Router-Config: `controllers[].target_ip` **und** `routes[].target_ip` | `10.0.0.248` |
| Universe-Nummern | Router-Config: `routes[].universe_start` **und** Controller: Ausgang → Start-Universe | `149` |
| LED-Anzahl pro Ausgang | Router-Config: `routes[].pixel_count` **und** Controller: Ausgang → LED-Anzahl | `136` |
| Welche Pixel aus dem PXLBLZ-Frame wohin gehen | Router-Config: `routes[].pixel_start` / `pixel_count` | `0` / `136` |
| Reihenfolge und Position der Pixel | PXLBLZ-**Map** (und Verkabelung) | `maps/esp-test-8x8-12x6.js` |
| Bildrate an den Controller | Router-Config: `controllers[].fps_target` | `60` |
| Bildrate an die LEDs | Controller: Ziel-FPS (`targetFps`) | `60` |
| Farbreihenfolge (RGB/GRB/BGR …) | **Controller** (Ausgang → Farbreihenfolge); Router dann `RGB` | Controller `BGR`, Router `RGB` |
| Welche Router-Config der Starter benutzt | `%LOCALAPPDATA%\PXLBLZ-IDE-ArtNet\launcher-config.json` → `routerConfig` | `config\routes.esp-test-172-8x8-12x6.json` |
| Router-Eingang (WebSocket) | fest im Starter: `127.0.0.1:9980` | – |
| Wohin PXLBLZ sendet | Studio-URL `pxoutUrl=…` (setzt der Starter automatisch) | `ws://127.0.0.1:9980/pixels` |

### Ports auf dem PC

| Port | Dienst |
| --- | --- |
| 5174 | PXLBLZ-API + lokale Datenbank (`PXLBLZ-IDE-main`) |
| 5175 | PXLBLZ-Oberfläche mit Art-Net-Ausgabe (`PXLBLZ-IDE`) |
| 9980 | Router-Eingang (WebSocket, Pixel von PXLBLZ) |
| 9988 | Router-Status und **Einstellungsseite** (`http://127.0.0.1:9988/`) – nicht 9981: dort lauscht TouchDesigner |
| 6454 (UDP) | Art-Net zum Controller (ausgehend) |

---

## 3. Die Router-Config im Detail

Die Datei, die der Starter benutzt, liegt im Workspace:

```text
E:\PXLBLZ-ArtNet\artnet\config\routes.esp-test-172-8x8-12x6.json
```

Beispiel (aktuelle Testumgebung: eine APA102-Kette 8×8 + FeatherWing 12×6):

```json
{
  "version": 2,
  "input": {
    "pixel_count": 136,          // Pixel pro Frame, die der Router verarbeitet
    "fps_target": 60,            // Standard-FPS, falls ein Controller keine eigene hat
    "stale_timeout_ms": 1000,    // so lange ohne neuen Frame von PXLBLZ ...
    "on_stale": "blackout",      // ... dann: hold | blackout | stop
    "variable_size": true        // PXLBLZ-Frames jeder Pixelzahl annehmen
  },
  "artnet": { "udp_port": 6454, "unicast": true },
  "controllers": [
    { "name": "ESP_TEST", "target_ip": "10.0.0.248", "fps_target": 60 }
  ],
  "routes": [
    { "name": "ESP_OUT6_APA102_CHAIN", "enabled": true,
      "target_ip": "10.0.0.248",            // MUSS einem controllers[].target_ip entsprechen
      "physical_port": 7,                   // nur Beschriftung/Doku
      "pixel_start": 0, "pixel_count": 136, // Pixel 0..135 aus dem PXLBLZ-Frame
      "universe_start": 149,                // erstes Universe; je 170 RGB-Pixel ein Universe
      "color_order": "RGB" }
  ]
}
```

(JSON erlaubt keine Kommentare – die `//` oben dienen nur der Erklärung.)

### Felder

**`input`**
- `pixel_count` – Größe des Frames, mit dem der Router rechnet. Alle Routen müssen
  innerhalb `0 … pixel_count-1` liegen.
- `variable_size: true` – PXLBLZ darf beliebig viele Pixel schicken: Die ersten
  `pixel_count` werden verwendet, fehlende sind schwarz. Ohne diese Option muss PXLBLZ
  **genau** `pixel_count` Pixel schicken, sonst wird jeder Frame verworfen.
- `stale_timeout_ms` / `on_stale` – was passiert, wenn PXLBLZ aufhört zu senden
  (Tab im Hintergrund, Studio geschlossen): `hold` letztes Bild halten,
  `blackout` schwarz senden, `stop` nichts mehr senden.

**`controllers[]`** – ein Eintrag pro Art-Net-Gerät
- `name` – frei wählbar, eindeutig.
- `target_ip` – **IP-Adresse des Controllers.** Eindeutig pro Controller.
- `fps_target` – mit dieser Rate sendet der Router an diesen Controller (1–240).
- `enabled: false` – Controller (und alle seine Routen) vorübergehend abschalten.

**`routes[]`** – ein Eintrag pro Controller-Ausgang (bzw. Universe-Bereich)
- `target_ip` – muss **exakt** einer `controllers[].target_ip` entsprechen.
- `pixel_start`, `pixel_count` – welcher Ausschnitt des PXLBLZ-Frames an diesen Ausgang geht.
- `universe_start` – erstes Art-Net-Universe. Der Router belegt
  `aufrunden(pixel_count / 170)` Universes ab dort. Routen auf derselben IP dürfen sich
  **nicht** überschneiden.
- `color_order` – `RGB`, `RBG`, `GRB`, `GBR`, `BRG`, `BGR`. Wenn der Controller die
  Farben selbst umsortiert (ESP-Firmware: ja), hier `RGB` lassen.

---

## 4. Was am Controller passen muss

Der Controller setzt einen Frame nur dann an die LEDs ab, wenn **alle** Universes, die
er für seine aktiven Ausgänge erwartet, vollständig angekommen sind. Deshalb müssen
Router-Config und Controller-Config zusammenpassen:

| Controller (Weboberfläche `http://<ip>/`) | muss passen zu (Router-Config) |
| --- | --- |
| aktive Ausgänge | je Ausgang eine Route auf derselben IP |
| Ausgang → Start-Universe | `routes[].universe_start` |
| Ausgang → LED-Anzahl | `routes[].pixel_count` |
| nicht benutzte Ausgänge | **deaktivieren** – sonst wartet der Controller auf deren Universes und verwirft alles |
| Ziel-FPS | ≤ `controllers[].fps_target` sinnvoll (sonst gibt er Frames doppelt aus) |

Änderungen in der Controller-Weboberfläche immer **„Übernehmen“** und danach
**„Speichern“**, sonst sind sie nach einem Neustart weg.

Prüfen: `http://<controller-ip>/api/status` – die Zähler `framesComplete` müssen
steigen, `framesIncomplete` darf nicht mitsteigen.

---

## 5. Schritt für Schritt

### A) Die IP eines Controllers hat sich geändert

1. Neue IP herausfinden: Geräteliste des WLAN-/LAN-Routers, oder im Browser
   `http://<vermutete-ip>/` öffnen (die Controller-Weboberfläche muss erscheinen).
2. Die Router-Config des Starters öffnen:
   `E:\PXLBLZ-ArtNet\artnet\config\<deine-config>.json`
3. Die alte IP an **allen** Stellen ersetzen:
   - in `controllers[]` → `target_ip`
   - in **jeder** Route dieses Controllers → `target_ip`
4. Prüfen (PowerShell):
   ```powershell
   E:\PXLBLZ-ArtNet\artnet\pxlblz-router.exe --config E:\PXLBLZ-ArtNet\artnet\config\<deine-config>.json --list-routes
   ```
   Die Ausgabe zeigt pro Controller die neue IP und die Universes.
5. Den laufenden Router beenden: Task-Manager → `pxlblz-router.exe` → Task beenden.
   (Der Starter startet einen bereits laufenden Router nicht neu.)
6. Doppelklick auf **PXLBLZ-IDE - ArtNet** auf dem Desktop.
7. Kontrolle: `http://127.0.0.1:9988/status` zeigt bei `controllers` die neue
   `target_ip`, und am Controller steigt `framesComplete`.
8. Damit die IP sich nicht mehr ändert: im WLAN-/LAN-Router eine **feste IP
   (DHCP-Reservierung)** für den Controller anlegen.

**Wichtig – Repo-Kopie mitziehen:** Die Configs im Workspace sind Kopien aus dem Repo
(`router\config\`). `Install-DesktopShortcut.ps1` kopiert beim nächsten Aufruf **alle**
Repo-Configs erneut in den Workspace und überschreibt dabei Änderungen dort. Deshalb
die Änderung auch in `pxlblz-artnet-bridge\router\config\<deine-config>.json` machen
(und committen) – oder nur im Repo ändern und den Installer neu ausführen (siehe B).

### B) Eine andere Router-Config verwenden (anderer Aufbau, andere Controller)

1. Config im Repo unter `router\config\` anlegen oder anpassen.
2. Prüfen: `pxlblz-router.exe --config <datei> --list-routes`.
3. Installer mit der gewünschten Config ausführen (kopiert Router + alle Configs in den
   Workspace und trägt die Config im Starter ein, erstellt keine neue Verknüpfung):
   ```powershell
   .\windows-launcher-artnet\Install-DesktopShortcut.ps1 -WorkspaceRoot 'E:\PXLBLZ-ArtNet' `
       -RouterBinDirectory '<Ordner mit pxlblz-router.exe>' `
       -RouterConfig 'config\<datei>.json' -NoShortcut
   ```
4. Laufenden Router beenden (Task-Manager → `pxlblz-router.exe`), dann Desktop-Verknüpfung
   starten.

Welche Config gerade aktiv ist: `%LOCALAPPDATA%\PXLBLZ-IDE-ArtNet\launcher-config.json`
(Feld `routerConfig`) und `artnet-start-status.txt` im selben Ordner.

### C) Die LED-Anzahl oder Verkabelung hat sich geändert

Drei Stellen gehören zusammen:

1. **Controller:** Ausgang → LED-Anzahl (und ggf. Start-Universe) setzen,
   **Übernehmen + Speichern**.
2. **Router-Config:** `routes[].pixel_count` (und `universe_start`) gleich setzen;
   `input.pixel_count` ≥ Summe aller Routen-Pixel.
3. **PXLBLZ-Map:** gleiche Pixelzahl und Reihenfolge wie die Verkabelung.
   Bei Unsicherheit über die Reihenfolge ein Lauflicht senden (siehe Abschnitt 6).

Danach Router neu starten (Task-Manager → `pxlblz-router.exe`, Desktop-Verknüpfung).

### D) Einen weiteren Controller hinzufügen

1. Neuen Eintrag in `controllers[]` mit eigener `target_ip`, `name`, `fps_target`.
2. Pro Ausgang eine Route mit derselben `target_ip`, eigenem `pixel_start`-Bereich im
   PXLBLZ-Frame und den Universes, die dieser Controller erwartet.
3. `input.pixel_count` so erhöhen, dass alle Routen hineinpassen.
4. Die PXLBLZ-Map um die neuen Pixel erweitern (Reihenfolge = `pixel_start` der Routen).
5. `--list-routes` prüfen, Router neu starten.

Jeder Controller bekommt im Router seinen **eigenen Takt und seine eigene
Sequenznummer** – unterschiedliche FPS pro Controller sind möglich.

### E) Den PC an ein anderes Netz anschließen

- Der PC muss im **selben Netz** wie die Controller sein (z. B. `10.0.0.x`).
  Prüfen: `ping <controller-ip>`.
- **VPN (z. B. NordVPN) ausschalten** – es kann lokale Pakete umleiten oder verzögern.
- Kabel ist besser als WLAN; ein Handy-Hotspot verliert bei 60 FPS spürbar Pakete.

---

## 6. Prüfen und Testen

| Prüfung | Wie |
| --- | --- |
| Config gültig, IPs/Universes richtig | `pxlblz-router.exe --config <datei> --list-routes` |
| Router läuft, PXLBLZ verbunden | `http://127.0.0.1:9988/status` → `ws_clients` = 1, `rx_frames` steigt |
| Controller empfängt vollständig | `http://<controller-ip>/api/status` → `framesComplete` steigt, `framesIncomplete` nicht |
| Netz erreichbar | `ping <controller-ip>` |
| Starter-Status | `%LOCALAPPDATA%\PXLBLZ-IDE-ArtNet\artnet-start-status.txt` |

**Testmuster ohne PXLBLZ** (laufenden Router vorher beenden, sonst senden zwei):

```powershell
# Lauflicht in Verkabelungsreihenfolge, 4 LEDs pro Sekunde
E:\PXLBLZ-ArtNet\artnet\pxlblz-router.exe --config <datei> --input pattern --pattern chase --fps 4 --duration 60s --status-listen 127.0.0.1:9982
# Regenbogen, 60 FPS, 10 s
E:\PXLBLZ-ArtNet\artnet\pxlblz-router.exe --config <datei> --input pattern --pattern rainbow --duration 10s --status-listen 127.0.0.1:9982
```

---

## 7. Fehlerbilder

| Symptom | Ursache | Abhilfe |
| --- | --- | --- |
| LEDs komplett dunkel, Controller zählt `framesIncomplete` hoch | Controller erwartet ein Universe, das der Router nicht sendet (z. B. unbenutzter Ausgang noch aktiv, falsche Universe-Nummer) | Unbenutzte Ausgänge deaktivieren; `universe_start` angleichen |
| LEDs dunkel, Controller zählt gar nichts | falsche `target_ip`, anderes Netz, VPN | IP prüfen (Abschnitt 5A), `ping`, VPN aus |
| Die letzten LEDs einer Kette bleiben dunkel | Controller-Ausgang hat zu wenige LEDs eingestellt | LED-Anzahl am Controller erhöhen (+ Route) |
| Router `rx_frames` bleibt 0, `ws_clients` = 1 | PXLBLZ-Tab rendert nicht (Hintergrund, minimiert) | Chrome-Fenster des Starters nach vorne holen bzw. F5 |
| Router `ws_clients` = 0 | Studio nicht offen oder mit falscher `pxoutUrl` | Desktop-Verknüpfung erneut starten |
| Bild gespiegelt / zeilenweise versetzt | Map passt nicht zur Verkabelung | `flipX` / `flipY` / `serpentine` in der Map; Lauflicht-Test |
| Falsche Farben | Farbreihenfolge doppelt umsortiert | Am Controller die Reihenfolge des LED-Typs, im Router `RGB` |
| Starter meldet „Port 9980 gehört nicht zum erwarteten Router“ | ein anderer Router-Prozess läuft (z. B. ein Testmuster) | diesen Prozess beenden, Starter erneut |
| Änderung an der Config wirkt nicht | Router läuft noch mit der alten Config | Router im Task-Manager beenden, Starter erneut |

---

## 8. Aktuelle Testumgebung (Stand 2026-10-01)

| | |
| --- | --- |
| Controller | ESP32 `esp32-wroom-flex-8ws-2apa`, **10.0.0.248** (vorher 172.20.10.2 am Hotspot) |
| Aktiver Ausgang | Ausgang 6, APA102, **136 LEDs**, Universe **149**, Ziel 60 FPS; Ausgang 0 deaktiviert |
| Kette | 8×8-Matrix (Pixel 0–63) → DotStar FeatherWing 12×6 (Pixel 64–135), beide von unten links, zeilenweise ohne Zickzack; 8×8 rechtsbündig unter der FeatherWing |
| Router-Config | `router/config/routes.esp-test-172-8x8-12x6.json` |
| PXLBLZ-Map | `pxlblz-integration/maps/esp-test-8x8-12x6.js` (auch in der lokalen PXLBLZ-Datenbank) |
| Pattern | `pxlblz-integration/patterns/snowflake-icesparkle-carpet-v06-esp-test.js` |
| Sicherung der früheren Controller-Config | `PXLBLZ_ArtNet_Handoff_2026-09-26\esp-config-backup\` |
