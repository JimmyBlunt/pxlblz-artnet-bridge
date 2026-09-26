# Known Installation Topology

This file separates **verified router routes** from other known installation routes that still need router hardware verification.

## Logical frame

Current full logical frame:

```text
pixels 0..8185
8186 pixels total
24558 RGB bytes
```

---

## Routing coverage

The logical frame is 8186 pixels, but the routes whose exact physical
configuration is currently known cover **6085 unique logical pixels**.

Known routed ranges:

```text
0..1183       1184 pixels  WS2812_NODE
1440..1642     203 pixels  BACK_PANEL P1
2720..2975     256 pixels  PANEL8 P1
3744..7645    3902 pixels  BACK_PANEL P2-P7
7646..8185     540 pixels  PANEL8 P2
-----------------------------------------
total         6085 pixels
```

Currently unrouted logical ranges:

```text
1184..1439     256 pixels
1643..2719    1077 pixels
2976..3743     768 pixels
-----------------------------------------
total         2101 pixels
```

These gaps must not be guessed. The APA102/ESP controller is known to exist, but
the available project history does not contain enough confirmed information to
assign its outputs to these ranges or to prove that it accounts for all gaps.

The combined known-controller config is:

```text
router/config/routes.installation-known.json
```

It has been virtually validated over real loopback UDP at:

```text
8186 input pixels
44 Art-Net universes/frame
30 FPS
1320 packets/s
GRB + RGB + BGR route orders
0 invalid packets
```

---

## BACK_PANEL_249 — VERIFIED

```text
IP: 10.0.0.253
Art-Net UDP: 6454
Color order: RGB
Target FPS used during successful tests: 30
7 electrical outputs
6 physical panels
```

```text
P1  1440..1642  203 LEDs  U120..U121
P2  3744..4481  738 LEDs  U122..U126
P3  4482..5361  880 LEDs  U127..U132
P4  5362..6171  810 LEDs  U133..U137
P5  6172..6523  352 LEDs  U139..U141
P6  6524..7133  610 LEDs  U142..U145
P7  7134..7645  512 LEDs  U146..U149
```

P6 and P7 together form one physical Panel 6.

---

## WS2812_NODE — KNOWN FROM PRIOR INSTALLATION CONFIG, ROUTER VERIFICATION PENDING

```text
IP: 10.0.0.244
Color order: GRB
Existing target rate: ~60 FPS
```

```text
P1  0..516      517 LEDs  U0..U3
P2  517..785    269 LEDs  U6..U7
P3  786..1183   398 LEDs  U12..U14
```

The universe gaps are intentional in the existing configuration.

---

## PANEL8_251 — KNOWN FROM PRIOR INSTALLATION CONFIG, ROUTER VERIFICATION PENDING

```text
IP: 10.0.0.251
Color order: BGR
Existing target rate: ~30 FPS
```

```text
P1  2720..2975  256 LEDs  U156..U157
P2  7646..8185  540 LEDs  U158..U161
```

---

## APA102 / ESP controller — TO BE CONFIRMED

A separate APA102 controller exists in the installation.

Before adding it to the production router config, confirm:

- IP address
- logical source pixel range(s)
- physical output/lane count
- universe or alternative protocol layout
- color order
- practical output FPS
- brightness policy
