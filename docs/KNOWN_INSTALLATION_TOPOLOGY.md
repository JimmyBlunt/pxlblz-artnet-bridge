# Known Installation Topology

Current, verified state of every controller (IPs, outputs, universes, firmware, measured
FPS): **[`controller-reference/INSTALLATION_CONTROLLERS.md`](../controller-reference/INSTALLATION_CONTROLLERS.md)**.
Running router config: `router/config/routes.installation-live.json`.

## Verified on hardware (2026-10-02, port-id colors OK)

| Controller | IP | Universes | LEDs | Router color order |
| --- | --- | --- | --- | --- |
| Teensy41 Octo WS2812B back panels | 10.0.0.253 | U120-U152 without U138 (8 outputs) | 4593 | RGB |
| ESP32 flex8 APA102 back panels | 10.0.0.251 | U156-U161 (2 outputs) | 805 | RGB |
| ESP32 APA102 test rig | 10.0.0.248 | U149 | 136 | RGB |

Changes against the earlier notes below: the Teensy now has **8** outputs enabled
(OUT8 488 LEDs at U150-U152, 32 universes); OUT6 is 610 LEDs. .251's second output has
**549** LEDs (not 540) and the router sends **RGB** (not BGR) - the firmware reorders.

### Pixel layout

`routes.installation-live.json` places the controllers **sequentially** in the PXLBLZ frame:

```text
ESP test rig   pixels    0..135
Teensy .253    pixels  136..4728   (OUT1..OUT8 in order)
APA102 .251    pixels 4729..5533   (OUT0, OUT1)
5534 pixels total
```

This layout is what the router test patterns and the ESP test map use. A PXLBLZ map for
the real installation must produce its points in exactly this order (or the routes'
`pixel_start` values must be changed to match the map).

## Historical logical frame (earlier TouchDesigner/Pixelblaze mapping)

`router/config/routes.installation-full.json` and the 2026-09 BACK_PANEL_249 tests used an
8186-pixel logical frame with these source ranges:

```text
10.0.0.244  P1    0..516   517 LEDs  U0..U3      GRB   (not yet connected to the router)
            P2  517..785   269 LEDs  U6..U7
            P3  786..1183  398 LEDs  U12..U14
10.0.0.251  P1 2720..2975  256 LEDs  U156..U157
            P2 7646..8185  540 LEDs  U158..U161  (now 549 LEDs)
10.0.0.253  P1 1440..1642  203 LEDs  U120..U121
            P2 3744..4481  738 LEDs  U122..U126
            P3 4482..5361  880 LEDs  U127..U132
            P4 5362..6171  810 LEDs  U133..U137
            P5 6172..6523  352 LEDs  U139..U141
            P6 6524..7133  610 LEDs  U142..U145
            P7 7134..7645  512 LEDs  U146..U149
            (P6 + P7 = one physical panel; OUT8 was disabled then)
```

Keep it as reference for the original mapping; it is not the running config.

### Virtual known-controller config (from main)

`router/config/routes.installation-known.json` combines .244 (GRB), .253 (RGB) and .251 (BGR,
pre-verification assumption) on the same 8186-pixel frame and is used by the hardware-free
virtual gates: 44 Art-Net universes/frame, 1320 packets/s at 30 FPS, 0 invalid packets over
real loopback UDP. It covers 6085 of the 8186 logical pixels; the unrouted ranges
1184..1439, 1643..2719 and 2976..3743 (2101 pixels) were never assigned. It is a regression
fixture, not the running config (the running config sends RGB to .251).

## Still to confirm

- **10.0.0.244** WS2812 node: IP, universes and color order from the earlier installation
  config; router hardware verification pending.
