// PXLBLZ compat library: Adafruit_NeoPixel stand-in that only lets sketches compile which include
// it without driving LEDs through it (it is a no-op sink; PXLBLZ shows the FastLED leds[] only).
#pragma once
#include <stdint.h>
#define NEO_GRB 0x52
#define NEO_RGB 0x06
#define NEO_KHZ800 0x0000
class Adafruit_NeoPixel {
 public:
  Adafruit_NeoPixel(uint16_t n = 0, int16_t = 6, uint16_t = 0) : num(n) {}
  void begin() {}
  void show() {}
  void setPixelColor(uint16_t, uint32_t) {}
  void setPixelColor(uint16_t, uint8_t, uint8_t, uint8_t) {}
  void setBrightness(uint8_t) {}
  void clear() {}
  uint16_t numPixels() const { return num; }
  static uint32_t Color(uint8_t r, uint8_t g, uint8_t b) { return ((uint32_t)r << 16) | ((uint32_t)g << 8) | b; }
 private:
  uint16_t num;
};
