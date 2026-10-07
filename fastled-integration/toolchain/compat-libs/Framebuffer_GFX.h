// PXLBLZ compat library: Framebuffer_GFX + FastLED_NeoMatrix stand-in (Marc Merlin's libraries).
// Draws into the sketch's CRGB buffer; layout is plain row-major (index = y * w + x) whatever
// NEO_MATRIX_* / NEO_TILE_* flags the sketch passes, so the 2D raster is exact for PXLBLZ's
// surface projection (the bundled neomatrix_config.h registers it with setScreenMap(mw, mh)).
// Accepts 16-bit (565), 24-bit and CRGB colours like Framebuffer_GFX; text is a no-op.
#pragma once
#include "Adafruit_GFX.h"

#ifndef NEO_MATRIX_TOP
#define NEO_MATRIX_TOP 0x00
#define NEO_MATRIX_BOTTOM 0x01
#define NEO_MATRIX_LEFT 0x00
#define NEO_MATRIX_RIGHT 0x02
#define NEO_MATRIX_CORNER 0x03
#define NEO_MATRIX_ROWS 0x00
#define NEO_MATRIX_COLUMNS 0x04
#define NEO_MATRIX_AXIS 0x04
#define NEO_MATRIX_PROGRESSIVE 0x00
#define NEO_MATRIX_ZIGZAG 0x08
#define NEO_MATRIX_SEQUENCE 0x08
#define NEO_TILE_TOP 0x00
#define NEO_TILE_BOTTOM 0x10
#define NEO_TILE_LEFT 0x00
#define NEO_TILE_RIGHT 0x20
#define NEO_TILE_CORNER 0x30
#define NEO_TILE_ROWS 0x00
#define NEO_TILE_COLUMNS 0x40
#define NEO_TILE_AXIS 0x40
#define NEO_TILE_PROGRESSIVE 0x00
#define NEO_TILE_ZIGZAG 0x80
#define NEO_TILE_SEQUENCE 0x80
#endif

class Framebuffer_GFX : public Adafruit_GFX {
 public:
  Framebuffer_GFX(CRGB *fb, uint16_t w, uint16_t h, void (*cb)() = nullptr) : Adafruit_GFX(w, h), _fb(fb), numpix(w * h), _cb(cb) {}
  using Adafruit_GFX::drawPixel;
  void drawPixel(int16_t x, int16_t y, uint16_t c) override { setpx(x, y, Color16toCRGB(c)); }
  void drawPixel(int16_t x, int16_t y, uint32_t c) { setpx(x, y, CRGB((c >> 16) & 0xFF, (c >> 8) & 0xFF, c & 0xFF)); }
  void drawPixel(int16_t x, int16_t y, CRGB c) { setpx(x, y, c); }
  void fillScreen(uint16_t c) override { CRGB v = Color16toCRGB(c); if (_fb) for (uint32_t i = 0; i < numpix; i++) _fb[i] = v; }
  void fillScreen(CRGB v) { if (_fb) for (uint32_t i = 0; i < numpix; i++) _fb[i] = v; }
  void clear() { if (_fb) for (uint32_t i = 0; i < numpix; i++) _fb[i] = CRGB::Black; }
  void begin() {}
  void newLedsPtr(CRGB *fb) { _fb = fb; }
  CRGB *getLedsPtr() { return _fb; }
  virtual void show() { if (_cb) _cb(); else FastLED.show(); }
  void show(bool) { show(); }
  void showfps() {}
  void setBrightness(int b) { FastLED.setBrightness(b); }
  void setPassThruColor(uint32_t c) { passThru = true; passColor = c; }
  void setPassThruColor() { passThru = false; }
  void setRemapFunction(uint16_t (*)(uint16_t, uint16_t)) {}
  void precal_gamma(float) {}
  static void show_free_mem(const char * = nullptr) {}
  uint16_t XY(int16_t x, int16_t y) {
    if (x < 0 || y < 0 || x >= _width || y >= _height) return numpix;  // hidden pixel (buffer has numpix+1 entries)
    return (uint16_t)(y * _width + x);
  }
  static uint16_t Color(uint8_t r, uint8_t g, uint8_t b) { return ((uint16_t)(r & 0xF8) << 8) | ((uint16_t)(g & 0xFC) << 3) | (b >> 3); }
  static uint16_t Color24to16(uint32_t c) { return Color((c >> 16) & 0xFF, (c >> 8) & 0xFF, c & 0xFF); }
  static uint32_t CRGBtoint32(CRGB c) { return ((uint32_t)c.r << 16) | ((uint32_t)c.g << 8) | c.b; }
  static CRGB Color16toCRGB(uint16_t c) {
    uint8_t r = (c >> 11) & 0x1F, g = (c >> 5) & 0x3F, b = c & 0x1F;
    return CRGB((r << 3) | (r >> 2), (g << 2) | (g >> 4), (b << 3) | (b >> 2));
  }
  static uint32_t Color24(uint8_t r, uint8_t g, uint8_t b) { return ((uint32_t)r << 16) | ((uint32_t)g << 8) | b; }
  // CRGB overloads of the GFX primitives (Marc Merlin's Framebuffer_GFX accepts 24-bit colours)
#define PXL_CRGB_PRIM(call) { bool pt = passThru; uint32_t pc = passColor; setPassThruColor(CRGBtoint32(c)); call; passThru = pt; passColor = pc; }
  void drawLine(int16_t x0, int16_t y0, int16_t x1, int16_t y1, CRGB c) PXL_CRGB_PRIM(Adafruit_GFX::drawLine(x0, y0, x1, y1, 0))
  void drawCircle(int16_t x, int16_t y, int16_t r, CRGB c) PXL_CRGB_PRIM(Adafruit_GFX::drawCircle(x, y, r, 0))
  void fillCircle(int16_t x, int16_t y, int16_t r, CRGB c) PXL_CRGB_PRIM(Adafruit_GFX::fillCircle(x, y, r, 0))
  void drawRect(int16_t x, int16_t y, int16_t w, int16_t h, CRGB c) PXL_CRGB_PRIM(Adafruit_GFX::drawRect(x, y, w, h, 0))
  void fillRect(int16_t x, int16_t y, int16_t w, int16_t h, CRGB c) PXL_CRGB_PRIM(Adafruit_GFX::fillRect(x, y, w, h, 0))
  void drawFastHLine(int16_t x, int16_t y, int16_t w, CRGB c) PXL_CRGB_PRIM(Adafruit_GFX::drawFastHLine(x, y, w, 0))
  void drawFastVLine(int16_t x, int16_t y, int16_t h, CRGB c) PXL_CRGB_PRIM(Adafruit_GFX::drawFastVLine(x, y, h, 0))
  void drawTriangle(int16_t x0, int16_t y0, int16_t x1, int16_t y1, int16_t x2, int16_t y2, CRGB c) PXL_CRGB_PRIM(Adafruit_GFX::drawTriangle(x0, y0, x1, y1, x2, y2, 0))
  void fillTriangle(int16_t x0, int16_t y0, int16_t x1, int16_t y1, int16_t x2, int16_t y2, CRGB c) PXL_CRGB_PRIM(Adafruit_GFX::fillTriangle(x0, y0, x1, y1, x2, y2, 0))
  using Adafruit_GFX::drawLine; using Adafruit_GFX::drawCircle; using Adafruit_GFX::fillCircle; using Adafruit_GFX::drawRect;
  using Adafruit_GFX::drawFastHLine; using Adafruit_GFX::drawFastVLine; using Adafruit_GFX::drawTriangle; using Adafruit_GFX::fillTriangle;
  using Adafruit_GFX::fillRect;
  void nscale8(uint8_t v) { if (_fb) for (uint32_t i = 0; i < numpix; i++) _fb[i].nscale8(v); }
  void fadeToBlackBy(uint8_t v) { nscale8(255 - v); }
  uint8_t gamma[256];
 protected:
  void setpx(int16_t x, int16_t y, CRGB c) {
    if (!_fb || x < 0 || y < 0 || x >= _width || y >= _height) return;
    if (passThru) c = CRGB((passColor >> 16) & 0xFF, (passColor >> 8) & 0xFF, passColor & 0xFF);
    _fb[y * _width + x] = c;
  }
  CRGB *_fb;
  uint32_t numpix;
  void (*_cb)();
  bool passThru = false;
  uint32_t passColor = 0;
};

class FastLED_NeoMatrix : public Framebuffer_GFX {
 public:
  FastLED_NeoMatrix(CRGB *fb, uint16_t w, uint16_t h, uint8_t = 0) : Framebuffer_GFX(fb, w, h) {}
  FastLED_NeoMatrix(CRGB *fb, uint16_t tw, uint16_t th, uint8_t tx, uint8_t ty, uint8_t = 0) : Framebuffer_GFX(fb, tw * tx, th * ty) {}
};
