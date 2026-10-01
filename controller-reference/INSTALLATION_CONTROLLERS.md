# Installation controllers (verified 2026-10-02)

Firmware sources: https://github.com/JimmyBlunt/ArtNet-Controller-ESP-Teensy41
(`firmware/esp32_artnet`, `firmware/teensy41_artnet`). Config snapshots read from the
controllers' `GET /api/config`: [`snapshots-2026-10-02/`](snapshots-2026-10-02/).
Router config: [`router/config/routes.installation-live.json`](../router/config/routes.installation-live.json).
Port-id test on 2026-10-02: every output shows its port color - universes and color order OK.
The router sends **RGB** to all three; both firmwares reorder colors themselves.

## Overview

| | Teensy back panels | APA102 back panels | ESP test rig |
| --- | --- | --- | --- |
| IP | **10.0.0.253** | **10.0.0.251** | **10.0.0.248** |
| Link | Ethernet (ping 1 ms) | Wi-Fi (ping 4-8 ms) | Wi-Fi |
| Firmware | `teensy41-octo-web-rx32`, build `orbital-port-tests-20260913` | ESP32, profile `flex8-ws2812-apa102` (env `esp32-wifi-esp251`) | ESP32, profile `esp32-wroom-flex-8ws-2apa` |
| LEDs | WS2812B, 8 outputs, **4593 px** | APA102, 2 outputs, **805 px** | APA102, 1 chain, **136 px** |
| Universes | **U120-U152 without U138** (32) | **U156-U161** (6) | **U149** (1) |
| Controller target FPS | 60 (max allowed 60) | 60 | 60 |
| LED output per frame | 29.3 ms | 12.4 ms (SPI fixed 4 MHz) | 2.2 ms |
| LEDs at 60 fps router | **34 fps** (lane length) | **~57 fps** (packet loss, see below) | **~59 fps** |

## Teensy 10.0.0.253 - outputs

| Out | Pin | LEDs | Universes |
| --- | --- | --- | --- |
| 1 | 2 | 203 | U120-U121 |
| 2 | 14 | 738 | U122-U126 |
| 3 | 7 | **880** | U127-U132 |
| 4 | 8 | 810 | U133-U137 |
| 5 | 6 | 352 | U139-U141 |
| 6 | 20 | 610 | U142-U145 |
| 7 | 21 | 512 | U146-U149 |
| 8 | 5 | 488 | U150-U152 |

OUT8 is enabled now (the 2026-09 BACK_PANEL_249 notes had it disabled and 29 universes).
WS2812B: 30 µs per LED -> the 880-LED lane limits the panel to ~34 fps; 60 fps needs every
lane <= ~540 LEDs (see `docs/FPS60_AUSWERTUNG.md`).

API (web firmware): `GET /api/config`, `GET /api/status` (`artnet_complete`, `artnet_incomplete`,
`artnet_rejected`, `dma_completed`, `dma_observed_us`, ...). Config changes only while stopped:
`POST /api/stop` -> `POST /api/config` (full config JSON, `targetFps` 1..60, per-output
`targetFps` must equal the global one) -> `POST /api/save` -> `POST /api/start`
(panels are dark for a few seconds). `POST /api/reboot` only when stopped and saved.
Receiver rules: `BACK_PANEL_249_RECEIVER.md`.

## APA102 10.0.0.251 - outputs

| Out | Data / clock | LEDs | Universes |
| --- | --- | --- | --- |
| 0 | 18 / 19 | 256 | U156-U157 |
| 1 | 25 / 26 | 549 | U158-U161 |

`colorOrder` RGB in the controller (older notes said BGR at the router - not needed).
API: `GET/POST /api/config` (apply), `POST /api/config/save`, `GET /api/storage`
(`saved`, `matches`), `GET /api/status` (`packets`, `framesComplete`, `framesIncomplete`,
`droppedPackets`, `sequenceErrors`, `outputFrames`, `outputTimeUs`), OTA at `/update`.

**Known issue:** ~4 % of packets lost at 60 fps (360 packets/s), not at 30 fps; independent
of placement. Cause in the firmware: Art-Net reception and the blocking LED output
(`outputs.show()`, 12.4 ms) share one `loop()`, the small lwIP UDP queue overflows.
Fix prepared: [`docs/ESP_FIRMWARE_RX_FIX_PROMPT.md`](../docs/ESP_FIRMWARE_RX_FIX_PROMPT.md)
(own receive task on core 0). Router-side double send / packet pacing were measured and do
not help.

**Update 2026-10-02:** .251 and .248 run the receive-task firmware (`rxQueueDrops` etc.). At
60 fps .251 now loses ~0.9 % (was 3-4 %), ~58 fps at the LEDs (`router/TEST_RESULTS.md`
H-INST3/H-INST4). The ESP32 `/api/status` has far fewer diagnostic fields than the Teensy;
alignment task: [`docs/FIRMWARE_DIAGNOSE_ANGLEICH_PROMPT.md`](../docs/FIRMWARE_DIAGNOSE_ANGLEICH_PROMPT.md).

## ESP test rig 10.0.0.248

One APA102 chain on output 6 (U149): 8x8 matrix (px 0-63) then DotStar FeatherWing 12x6
(px 64-135), both wired from the bottom left, rows left->right without serpentine, the 8x8
right-aligned under the FeatherWing. Output 0 disabled. PXLBLZ map:
`pxlblz-integration/maps/esp-test-8x8-12x6.js`. Same API as .251.

## Not connected yet

- **10.0.0.244** WS2812 node (GRB, U0-U3 / U6-U7 / U12-U14 per the earlier installation config)
  - router verification pending.
