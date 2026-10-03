# PXLBLZ-IDE~FastLED - Desktop-Starter

Startet die PXLBLZ-IDE mit FastLED-Unterstuetzung samt allen Diensten und oeffnet das Studio
mit dem **lokalen Login** (`github:local-dev`, kein Google/GitHub). Aufbau wie der
Fadecandy-Starter: `Start-PXLBLZ-FastLED.ps1` oeffnet ein Starterfenster
(`Show-PXLBLZ-Launcher.ps1`), jede Komponente laeuft einzeln ueber
`Invoke-LauncherComponent.ps1`.

| Komponente | Was | Port |
|---|---|---|
| Git | Workspace vorhanden, IDE-Checkout mit FastLED (`feature/fastled`) | - |
| Api | API + lokale D1-Daten aus `PXLBLZ-IDE-main` | 5174 |
| Ide | FastLED-IDE aus `PXLBLZ-IDE-fastled` (API-Proxy auf 5174) | 5175 |
| Compiler | FastLED-Compiler (`fastled-integration/service/server.mjs`) | 9996 |
| Router | Art-Net-Router aus `<workspace>\artnet` (Config aus der Art-Net-Installation) | 9980 / 9988 |
| Browser | Chrome/Edge mit eigenem Profil, lokaler Login, `?pxout=1` | - |

Laufende, passende Dienste werden wiederverwendet. Fremde Prozesse werden nie beendet. Einzige
Ausnahme ist der Knopf **"5175 freigeben"**: Er beendet nach Rueckfrage nur einen Vite-Server einer
anderen PXLBLZ-IDE (z. B. vom Art-Net-Starter). Art-Net-Starter und FastLED-Starter nutzen beide
Port 5175 und laufen daher nicht gleichzeitig. Die FastLED-IDE kann aber auch alle
Pixelblaze-Patterns.

## Installation

```powershell
.\Install-DesktopShortcut.ps1 -WorkspaceRoot E:\PXLBLZ-ArtNet
```

- Installiert nach `<workspace>\launcher-fastled` (bewusst nicht auf C:, das Browserprofil waechst).
- Legt "PXLBLZ-IDE - FastLED" auf dem Desktop und im Startmenue an.
- Optionen: `-IdeFolder`, `-FastLedIntegrationPath` (Standard `<workspace>\fastled-work`),
  `-RouterConfig` (Standard: wie der Art-Net-Starter), `-OutputUrl` (Standard
  `ws://127.0.0.1:9980/pixels`, fuer Fadecandy die Fadecandy-Bridge-URL eintragen).
- Eine erneute Installation behaelt Einstellungen und Browserprofil.

## Ohne Fenster (Tests)

```powershell
.\Start-PXLBLZ-FastLED.ps1 -CheckOnly   # nur pruefen
.\Start-PXLBLZ-FastLED.ps1 -NoOpen      # Dienste starten, Login pruefen, kein Browser
```

Logs liegen im Installationsordner: `server*.log`, `ide-fastled*.log`, `fastled-service*.log`,
`artnet-router*.log`, `local-login*.log`, `launch-history.log`, `launcher-fatal.log`.
