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


## Observed hardware results

### 2026-09-20 - L3D physical boundary test

Observed on the modified L3D cube:

- physical pixel 63: first/back 8x8 plane, right/top end
- physical pixel 64: second plane from the back, left/bottom start
- physical pixel 127: second plane from the back, right/top end
- physical pixel 128: third plane from the back, left/bottom start

This strongly matches the historical L3D physical indexing contract:

```text
physical = z*64 + x*8 + y
```

Interpretation so far:

- each 64-pixel Fadecandy block is one complete 8x8 cube plane;
- z advances one plane every 64 pixels;
- plane order advances from back toward front;
- each new plane begins at its left/bottom corner and ends at its right/top corner.

Next verification: test indices 0, 7, 8, 56 and 63 inside one plane to confirm the exact x/y traversal.


### L3D mapper verification

- physical plane/index orientation: PASS
- logical XYZ axis mapping through `pxlblz-fadecandy`: PASS
- observed axes:
  - +X left -> right
  - +Y bottom -> top
  - +Z back -> front
  - common origin at left/bottom/back

The configured historical L3D mapping is therefore confirmed on the real modified cube.


### Corner and layer verification

- eight-corner logical pattern: PASS
- all eight logical cube corners appeared at the expected physical cube corners
- moving 8x8 layer pattern: PASS
- layer order and direction matched the expected back-to-front Z progression

At this point the historical L3D coordinate mapping, cube dimensions, axis orientation, and plane progression are confirmed on the real hardware.


### Full logical voxel walk

- logical voxel walk through all 512 positions: PASS
- exactly one voxel was active at a time
- all physical LEDs were reached
- no incorrect plane jumps, duplicate positions, or missing positions were observed

Result: the complete L3D logical-to-physical mapping is verified on the real modified cube.

Next gate: validate the binary WebSocket input path independently of PXLBLZ.


### fcserver / USB / OPC connection verification

Observed on Windows with the real hardware:

- fcserver listening on 127.0.0.1:7890: PASS
- Fadecandy USB device attached: PASS
- reported firmware version: 1.07
- incoming Open Pixel Control client connection accepted: PASS

This confirms the server, USB device enumeration, and OPC TCP connection layer on the real machine.
