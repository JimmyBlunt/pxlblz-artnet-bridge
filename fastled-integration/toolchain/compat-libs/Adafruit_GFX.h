// PXLBLZ compat library (toolchain/compat-libs, on every sketch's include path after the sketch
// folder): minimal Adafruit_GFX stand-in for imported Arduino sketches. Pixels, lines, rectangles,
// circles, triangles and bitmaps are drawn through the subclass's drawPixel(); there is no font
// data, so text calls (print, setCursor, setTextColor ...) compile and do nothing.
// Not Adafruit's code; same method names and signatures as Adafruit_GFX 1.x.
#pragma once
#include <FastLED.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#ifndef _swap_int16_t
#define _swap_int16_t(a, b) { int16_t t = a; a = b; b = t; }
#endif

struct GFXfont;
class Adafruit_GFX {
 public:
  Adafruit_GFX(int16_t w, int16_t h) : WIDTH(w), HEIGHT(h), _width(w), _height(h) {}
  virtual ~Adafruit_GFX() {}
  virtual void drawPixel(int16_t x, int16_t y, uint16_t color) = 0;
  virtual void startWrite() {}
  virtual void endWrite() {}
  virtual void writePixel(int16_t x, int16_t y, uint16_t c) { drawPixel(x, y, c); }
  virtual void writeFillRect(int16_t x, int16_t y, int16_t w, int16_t h, uint16_t c) { fillRect(x, y, w, h, c); }
  virtual void writeFastVLine(int16_t x, int16_t y, int16_t h, uint16_t c) { drawFastVLine(x, y, h, c); }
  virtual void writeFastHLine(int16_t x, int16_t y, int16_t w, uint16_t c) { drawFastHLine(x, y, w, c); }
  virtual void writeLine(int16_t x0, int16_t y0, int16_t x1, int16_t y1, uint16_t c) { drawLine(x0, y0, x1, y1, c); }
  virtual void drawFastVLine(int16_t x, int16_t y, int16_t h, uint16_t c) { for (int16_t i = 0; i < h; i++) drawPixel(x, y + i, c); }
  virtual void drawFastHLine(int16_t x, int16_t y, int16_t w, uint16_t c) { for (int16_t i = 0; i < w; i++) drawPixel(x + i, y, c); }
  virtual void fillRect(int16_t x, int16_t y, int16_t w, int16_t h, uint16_t c) { for (int16_t i = x; i < x + w; i++) drawFastVLine(i, y, h, c); }
  virtual void fillScreen(uint16_t c) { fillRect(0, 0, _width, _height, c); }
  virtual void drawLine(int16_t x0, int16_t y0, int16_t x1, int16_t y1, uint16_t c) {
    int16_t steep = abs(y1 - y0) > abs(x1 - x0);
    if (steep) { _swap_int16_t(x0, y0); _swap_int16_t(x1, y1); }
    if (x0 > x1) { _swap_int16_t(x0, x1); _swap_int16_t(y0, y1); }
    int16_t dx = x1 - x0, dy = abs(y1 - y0), err = dx / 2, ystep = y0 < y1 ? 1 : -1;
    for (; x0 <= x1; x0++) {
      if (steep) drawPixel(y0, x0, c); else drawPixel(x0, y0, c);
      err -= dy;
      if (err < 0) { y0 += ystep; err += dx; }
    }
  }
  virtual void drawRect(int16_t x, int16_t y, int16_t w, int16_t h, uint16_t c) {
    drawFastHLine(x, y, w, c); drawFastHLine(x, y + h - 1, w, c); drawFastVLine(x, y, h, c); drawFastVLine(x + w - 1, y, h, c);
  }
  void drawCircle(int16_t x0, int16_t y0, int16_t r, uint16_t c) {
    int16_t f = 1 - r, ddF_x = 1, ddF_y = -2 * r, x = 0, y = r;
    drawPixel(x0, y0 + r, c); drawPixel(x0, y0 - r, c); drawPixel(x0 + r, y0, c); drawPixel(x0 - r, y0, c);
    while (x < y) {
      if (f >= 0) { y--; ddF_y += 2; f += ddF_y; }
      x++; ddF_x += 2; f += ddF_x;
      drawPixel(x0 + x, y0 + y, c); drawPixel(x0 - x, y0 + y, c); drawPixel(x0 + x, y0 - y, c); drawPixel(x0 - x, y0 - y, c);
      drawPixel(x0 + y, y0 + x, c); drawPixel(x0 - y, y0 + x, c); drawPixel(x0 + y, y0 - x, c); drawPixel(x0 - y, y0 - x, c);
    }
  }
  void fillCircle(int16_t x0, int16_t y0, int16_t r, uint16_t c) {
    for (int16_t y = -r; y <= r; y++) for (int16_t x = -r; x <= r; x++) if (x * x + y * y <= r * r + r) drawPixel(x0 + x, y0 + y, c);
  }
  void drawTriangle(int16_t x0, int16_t y0, int16_t x1, int16_t y1, int16_t x2, int16_t y2, uint16_t c) {
    drawLine(x0, y0, x1, y1, c); drawLine(x1, y1, x2, y2, c); drawLine(x2, y2, x0, y0, c);
  }
  void fillTriangle(int16_t x0, int16_t y0, int16_t x1, int16_t y1, int16_t x2, int16_t y2, uint16_t c) {
    int16_t minx = x0 < x1 ? (x0 < x2 ? x0 : x2) : (x1 < x2 ? x1 : x2), maxx = x0 > x1 ? (x0 > x2 ? x0 : x2) : (x1 > x2 ? x1 : x2);
    int16_t miny = y0 < y1 ? (y0 < y2 ? y0 : y2) : (y1 < y2 ? y1 : y2), maxy = y0 > y1 ? (y0 > y2 ? y0 : y2) : (y1 > y2 ? y1 : y2);
    for (int16_t y = miny; y <= maxy; y++) for (int16_t x = minx; x <= maxx; x++) {
      long d1 = (long)(x - x1) * (y0 - y1) - (long)(x0 - x1) * (y - y1), d2 = (long)(x - x2) * (y1 - y2) - (long)(x1 - x2) * (y - y2), d3 = (long)(x - x0) * (y2 - y0) - (long)(x2 - x0) * (y - y0);
      bool neg = d1 < 0 || d2 < 0 || d3 < 0, pos = d1 > 0 || d2 > 0 || d3 > 0;
      if (!(neg && pos)) drawPixel(x, y, c);
    }
  }
  void drawRoundRect(int16_t x, int16_t y, int16_t w, int16_t h, int16_t, uint16_t c) { drawRect(x, y, w, h, c); }
  void fillRoundRect(int16_t x, int16_t y, int16_t w, int16_t h, int16_t, uint16_t c) { fillRect(x, y, w, h, c); }
  void drawBitmap(int16_t x, int16_t y, const uint8_t *bmp, int16_t w, int16_t h, uint16_t c) {
    int16_t bw = (w + 7) / 8;
    for (int16_t j = 0; j < h; j++) for (int16_t i = 0; i < w; i++) if (bmp[j * bw + i / 8] & (128 >> (i & 7))) drawPixel(x + i, y + j, c);
  }
  void drawBitmap(int16_t x, int16_t y, const uint8_t *bmp, int16_t w, int16_t h, uint16_t c, uint16_t bg) {
    int16_t bw = (w + 7) / 8;
    for (int16_t j = 0; j < h; j++) for (int16_t i = 0; i < w; i++) drawPixel(x + i, y + j, (bmp[j * bw + i / 8] & (128 >> (i & 7))) ? c : bg);
  }
  void drawRGBBitmap(int16_t x, int16_t y, const uint16_t *bmp, int16_t w, int16_t h) {
    for (int16_t j = 0; j < h; j++) for (int16_t i = 0; i < w; i++) drawPixel(x + i, y + j, bmp[j * w + i]);
  }
  // text: accepted, not rendered (no font data in the shim)
  void setCursor(int16_t x, int16_t y) { cursor_x = x; cursor_y = y; }
  int16_t getCursorX() const { return cursor_x; }
  int16_t getCursorY() const { return cursor_y; }
  void setTextColor(uint16_t c) { textcolor = c; }
  void setTextColor(uint16_t c, uint16_t) { textcolor = c; }
  void setTextSize(uint8_t s) { textsize = s; }
  void setTextSize(uint8_t s, uint8_t) { textsize = s; }
  void setTextWrap(bool w) { wrap = w; }
  void setFont(const GFXfont * = nullptr) {}
  void cp437(bool = true) {}
  void getTextBounds(const char *s, int16_t x, int16_t y, int16_t *x1, int16_t *y1, uint16_t *w, uint16_t *h) {
    *x1 = x; *y1 = y; *w = (uint16_t)(6 * textsize * (s ? strlen(s) : 0)); *h = 8 * textsize;
  }
  virtual size_t write(uint8_t) { cursor_x += 6 * textsize; return 1; }
  template <typename T> size_t print(const T &) { return 0; }
  template <typename T> size_t print(const T &, int) { return 0; }
  template <typename T> size_t println(const T &) { return 0; }
  template <typename T> size_t println(const T &, int) { return 0; }
  size_t println() { return 0; }
  void setRotation(uint8_t r) { rotation = r & 3; if (rotation & 1) { _width = HEIGHT; _height = WIDTH; } else { _width = WIDTH; _height = HEIGHT; } }
  uint8_t getRotation() const { return rotation; }
  int16_t width() const { return _width; }
  int16_t height() const { return _height; }
  void invertDisplay(bool) {}
 protected:
  int16_t WIDTH, HEIGHT, _width, _height, cursor_x = 0, cursor_y = 0;
  uint16_t textcolor = 0xFFFF;
  uint8_t textsize = 1, rotation = 0;
  bool wrap = true;
};
