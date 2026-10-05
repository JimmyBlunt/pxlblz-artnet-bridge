# FastLED-Patterns für PXLBLZ-IDE (Patch-Serie)

Diese Patches bringen **FastLED Patterns** in die PXLBLZ-IDE: echte FastLED-Sketches (C++) im
Studio-Editor, Vorschau über die wasm-Engine aus `fastled-integration/`, Ausgabe unverändert über
den Art-Net/Fadecandy-Router. Sie liegen auf dem Art-Net-Stand (`pxlblz-artnet-output`, externe
Pixel-Ausgabe) auf; ohne ihn bricht das Skript ab.

```
patches/0001-…0013-*.patch   git format-patch pxlblz-artnet-output..feature/fastled
install-fastled-ide.ps1        spielt sie per git am --3way ein (idempotent)
```

Stand der Serie: IDE `feature/fastled` @ c7fca3d0, Basis `pxlblz-artnet-output` @ 071002e9
(= upstream 21b764ab + 2 lokale Art-Net-Commits). Host-ABI der wasm-Engine: 1.

## Einspielen

```powershell
# Checkout mit Art-Net-Ausgabe, sauber committet (es wird nichts verworfen)
powershell -ExecutionPolicy Bypass -File .\install-fastled-ide.ps1 -IdePath E:\PXLBLZ-ArtNet\PXLBLZ-IDE -CheckOnly
powershell -ExecutionPolicy Bypass -File .\install-fastled-ide.ps1 -IdePath E:\PXLBLZ-ArtNet\PXLBLZ-IDE
```

- Bereits eingespielt → Meldung „nichts zu tun“, Exit 0.
- Konflikt → `git am --abort`, Checkout unverändert, Exit 1 (dann „Nachziehen“ unten).
- Danach IDE neu starten. Der FastLED-Compiler (`npm run service` in `fastled-integration`,
  Port 9996) wird vom Art-Net-Launcher gestartet; ohne ihn zeigt die Vorschau einen Hinweis.

## Benutzung (kurz)

- Patterns-Rail → „+“ → **New FastLED pattern**. Erste Zeile `// @pxlblz-language fastled`
  (optional `// @pxlblz-dim 1|2|3`, `// @pxlblz-output wire|leds`).
- `PXLBLZ_NUM_LEDS` = Pixelzahl der aktuellen Map (Compiler-Define).
- `UISlider` / `UICheckbox` / `UINumberField` erscheinen unter **Controls** und werden pro Pattern
  gespeichert. Compilerfehler erscheinen als Marker im Editor.
- Eingefügter FastLED-Code ohne Marker → Leiste „Convert to FastLED Pattern“.
- Mit `?pxout=1` gehen die Leitungsbytes (L2, nach Helligkeit/Korrektur/Dithering) exakt an den Router.
- Nie an einen Pixelblaze gesendet, nicht in Shows, nicht in der Agent-Discovery, kein .epe-Download.

## Nachziehen nach einem Upstream-Update der IDE

Im IDE-Repo (Worktree mit `feature/fastled`):

```bash
git fetch origin
# 1. Art-Net-Stand auf das neue upstream setzen (eigene Serie, siehe Art-Net-Doku)
git rebase origin/main pxlblz-artnet-output
# 2. FastLED-Serie darauf rebasen, Konflikte lösen (meist Preview.tsx, ShowEditor.tsx, controllerStore.ts)
git rebase pxlblz-artnet-output feature/fastled
# 3. Prüfen
npx vitest run src/engine/patternLanguage.test.ts src/engine/fastled src/store/fastledStore.test.ts
npx tsc -b tsconfig.json tsconfig.node.json --pretty false
# 4. Patches neu exportieren (alte vorher entfernen) und hier committen
git format-patch --binary -o <bridge>/fastled-integration/ide/patches pxlblz-artnet-output..feature/fastled
```

Ändert sich die Engine-ABI (`runtime/fastledWasmHost.ts`), die Datei unverändert nach
`src/engine/fastled/fastledWasmHost.ts` kopieren (Kopfkommentar behalten) und die Test-Fixtures mit
`node scripts/generate-fastled-fixtures.mjs` (`FASTLED_WORK=<fastled-integration>`) neu erzeugen.

`.gitattributes` hält die `.patch`-Dateien ohne Zeilenende-Umwandlung (sie enthalten binäre wasm-Fixtures).
