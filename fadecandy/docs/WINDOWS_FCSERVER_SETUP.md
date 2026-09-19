# Windows fcserver setup for the L3D bench

This is the recommended first-light path for the L3D/Fadecandy work.

The goal is deliberately narrow: prove Fadecandy USB + fcserver + OPC before enabling any L3D mapping or PXLBLZ output.

## 1. Files

Project branch:

- `feature/fadecandy-l3d-output`

Our Windows diagnostic executables are produced by GitHub Actions into:

- `bin/windows-x64/fadecandy-probe.exe`
- `bin/windows-x64/pxlblz-fadecandy.exe`

The L3D fcserver configuration is:

- `fadecandy/config/fcserver-l3d.json`

The original Fadecandy project includes a ready-to-run Windows `fcserver.exe`. For reproducibility, our installer pins this source revision:

- repository: `rewolff/fadecandy`
- revision: `de7f94a052579b3cea1dc81e17231be469f1603a`
- binary: `bin/fcserver.exe`
- expected size: 310286 bytes

## 2. Automatic install

Open PowerShell in the repository root:

```powershell
cd .\fadecandy
powershell -ExecutionPolicy Bypass -File .\install-fcserver.ps1
```

This downloads:

```text
fadecandy\bin\fcserver.exe
```

No compiler is needed.

## 3. Connect the Fadecandy board

Connect the Fadecandy controller directly over USB.

On normal Windows 10/11 installations the WinUSB driver may install automatically.

If Windows does not expose the board correctly:

1. Open Device Manager and check whether Fadecandy is present with a warning icon.
2. Use Zadig only if necessary.
3. In Zadig select the Fadecandy device.
4. Prefer the WinUSB driver first.
5. Reconnect the board after driver installation.

Do not change firmware for the first test unless the existing board is not recognized at all.

## 4. Start fcserver

From the `fadecandy` directory:

```powershell
.\bin\fcserver.exe .\config\fcserver-l3d.json
```

Expected server behavior includes a message that it is listening on:

```text
127.0.0.1:7890
```

and, when the board is recognized, a Fadecandy USB device attachment message with its serial/firmware version.

Keep this console window open.

The config maps OPC channel 0, pixels 0..511, directly to the 512 physical Fadecandy pixel slots:

```json
"map": [[0, 0, 0, 512]]
```

Dithering and inter-frame interpolation remain enabled.

## 5. Gate H0: direct physical pixel 0

Open a second PowerShell window in the repository root:

```powershell
.\bin\windows-x64\fadecandy-probe.exe --mode pixel --pixel 0 --color red --hex
```

Expected packet summary:

```text
OPC channel=0 command=0x00 payload=1536 bytes pixels=512 total=1540 bytes
```

The packet begins:

```text
00 00 06 00 FF 00 00 ...
```

Meaning:

- OPC channel 0
- Set Pixel Colors command 0x00
- 0x0600 = 1536 bytes
- physical pixel 0 = red
- all remaining pixels = black

Expected hardware result: one physical LED is red.

This test contains no L3D coordinate mapping.

## 6. Gate H1: Fadecandy 64-pixel boundaries

Run separately:

```powershell
.\bin\windows-x64\fadecandy-probe.exe --mode pixel --pixel 63 --color red
.\bin\windows-x64\fadecandy-probe.exe --mode pixel --pixel 64 --color green
.\bin\windows-x64\fadecandy-probe.exe --mode pixel --pixel 127 --color blue
.\bin\windows-x64\fadecandy-probe.exe --mode pixel --pixel 128 --color yellow
.\bin\windows-x64\fadecandy-probe.exe --mode pixel --pixel 511 --color white
```

These identify the boundaries between the eight 64-pixel Fadecandy outputs.

## 7. Gate H2: complete electrical outputs

Examples:

```powershell
.\bin\windows-x64\fadecandy-probe.exe --mode strand --strand 0 --color red
.\bin\windows-x64\fadecandy-probe.exe --mode strand --strand 1 --color green
.\bin\windows-x64\fadecandy-probe.exe --mode strand --strand 7 --color white
```

Exactly 64 physical pixel slots should be populated per test.

## 8. Gate H3: raw physical chase

```powershell
.\bin\windows-x64\fadecandy-probe.exe --mode chase --color white --fps 10 --duration 60s
```

This walks raw physical indices 0..511.

Record where the lit point physically moves. Any unexpected jump is useful evidence about the modified cube wiring.

## 9. Gate H4: RGB and blackout

```powershell
.\bin\windows-x64\fadecandy-probe.exe --mode all --color red
.\bin\windows-x64\fadecandy-probe.exe --mode all --color green
.\bin\windows-x64\fadecandy-probe.exe --mode all --color blue
.\bin\windows-x64\fadecandy-probe.exe --mode black
```

Only after H0-H4 pass do we insert the L3D logical-to-physical mapper.

## Troubleshooting

### Connection refused on 127.0.0.1:7890

`fcserver.exe` is not running, exited due to config/USB problems, or is listening on another address/port.

### fcserver runs but no Fadecandy attachment appears

Check Device Manager. If the board is not correctly bound to WinUSB, use Zadig to install WinUSB for the Fadecandy device.

### fcserver sees the board but no LEDs change

Stay below the L3D mapping layer. Use `fadecandy-probe.exe` with physical pixel/strand tests and verify power, ground, data direction, and the controller-to-LED wiring.

### Wrong colors

Use the H4 red/green/blue full-frame tests first. Do not compensate in the L3D coordinate mapper. Color order is a separate output-layer concern.
