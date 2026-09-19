# Fadecandy/L3D hardware test protocol

This protocol deliberately validates the stack from the physical output upward.

## Test order

1. fcserver sees the Fadecandy USB device.
2. TCP connection to fcserver:7890 succeeds.
3. A hand-built OPC Set Pixel Colors packet changes physical pixel 0.
4. Physical pixels 0, 63, 64, 127, ... identify the eight 64-pixel Fadecandy outputs.
5. A full 64-pixel strand command lights exactly one expected electrical output.
6. A physical 0..511 chase proves continuous physical addressing and reveals wiring discontinuities.
7. Only after those pass, insert the L3D logical-to-physical mapper.
8. Run axes/corners/layers/voxel logical patterns.
9. Only after mapping is verified, connect the PXLBLZ WebSocket producer.

This isolates transport, physical addressing, mapping, and rendering into separate gates.

## Probe commands

Run from the `fadecandy` directory.

### H0 - TCP/OPC + physical pixel 0

```powershell
go run ./cmd/fadecandy-probe --mode pixel --pixel 0 --color red --hex
```

Expected OPC header for a 512-pixel RGB frame:

```text
00 00 06 00
```

Meaning:

- channel 0
- command 0x00 Set Pixel Colors
- payload length 0x0600 = 1536 bytes
- 512 RGB pixels

Expected hardware result: only physical LED 0 is red.

### H1 - boundary pixels

Run these separately:

```powershell
go run ./cmd/fadecandy-probe --mode pixel --pixel 63  --color red
go run ./cmd/fadecandy-probe --mode pixel --pixel 64  --color green
go run ./cmd/fadecandy-probe --mode pixel --pixel 127 --color blue
go run ./cmd/fadecandy-probe --mode pixel --pixel 128 --color yellow
go run ./cmd/fadecandy-probe --mode pixel --pixel 511 --color white
```

These distinguish 64-pixel Fadecandy output boundaries without any L3D mapping.

### H2 - one complete Fadecandy output

```powershell
go run ./cmd/fadecandy-probe --mode strand --strand 0 --color red
go run ./cmd/fadecandy-probe --mode strand --strand 1 --color green
...
go run ./cmd/fadecandy-probe --mode strand --strand 7 --color white
```

Exactly 64 physical pixel slots are populated for each command.

### H3 - physical chase

```powershell
go run ./cmd/fadecandy-probe --mode chase --color white --fps 10 --duration 60s
```

This walks raw physical indices 0..511. No cube-coordinate mapping is involved.

Record any point where the visual path jumps to another physical location. That information tells us whether the modified L3D still follows the historical wiring or needs a custom 512-entry LUT.

### H4 - full-frame checks

```powershell
go run ./cmd/fadecandy-probe --mode all --color red
go run ./cmd/fadecandy-probe --mode all --color green
go run ./cmd/fadecandy-probe --mode all --color blue
go run ./cmd/fadecandy-probe --mode black
```

These verify RGB channel order, global color behavior, and blackout.

## Pass/fail recording

For each test record:

- command
- fcserver console output
- whether the Fadecandy device/status LED reacted
- physical LEDs observed
- expected vs actual color
- unexpected additional LEDs
- whether output was stable or flickering

Do not adjust L3D mapping to compensate for failures in H0-H4. These tests operate below the mapping layer.
