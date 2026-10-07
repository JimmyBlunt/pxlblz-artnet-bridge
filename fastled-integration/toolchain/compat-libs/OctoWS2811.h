// PXLBLZ stand-in for PJRC's OctoWS2811 (Teensy 3/4 DMA LED driver).
// The sketch's own CTeensy4Controller feeds setPixel()/show(); here the pixels are only
// kept in memory, because PXLBLZ itself takes leds[] and sends it through the LED router.
// Keeps the original sketch source unchanged.
#pragma once
#include <stdint.h>
#include <stddef.h>

#ifndef DMAMEM
#define DMAMEM
#endif

#define WS2811_RGB 0
#define WS2811_RBG 1
#define WS2811_GRB 2
#define WS2811_GBR 3
#define WS2811_BRG 4
#define WS2811_BGR 5
#define WS2811_800kHz 0x00
#define WS2811_400kHz 0x10
#define WS2813_800kHz 0x20

class OctoWS2811 {
public:
  OctoWS2811(uint32_t numPerStrip, void *frameBuf, void *drawBuf, uint8_t config = WS2811_GRB,
             uint8_t numPins = 8, const uint8_t *pinList = nullptr)
      : perStrip(numPerStrip), pins(numPins), draw(static_cast<uint8_t *>(drawBuf)) { (void)frameBuf; (void)config; (void)pinList; }
  void begin() {}
  void begin(uint32_t numPerStrip, void *frameBuf, void *drawBuf, uint8_t config = WS2811_GRB) {
    perStrip = numPerStrip; draw = static_cast<uint8_t *>(drawBuf); (void)frameBuf; (void)config;
  }
  void setPixel(uint32_t num, int color) {
    setPixel(num, (uint8_t)(color >> 16), (uint8_t)(color >> 8), (uint8_t)color);
  }
  void setPixel(uint32_t num, uint8_t red, uint8_t green, uint8_t blue) {
    if (!draw || num >= numPixels()) return;
    draw[num * 3] = red; draw[num * 3 + 1] = green; draw[num * 3 + 2] = blue;
  }
  int getPixel(uint32_t num) {
    if (!draw || num >= numPixels()) return 0;
    return (draw[num * 3] << 16) | (draw[num * 3 + 1] << 8) | draw[num * 3 + 2];
  }
  void show() {}
  int busy() { return 0; }
  int numPixels() { return (int)(perStrip * pins); }

private:
  uint32_t perStrip;
  uint8_t pins;
  uint8_t *draw;
};
