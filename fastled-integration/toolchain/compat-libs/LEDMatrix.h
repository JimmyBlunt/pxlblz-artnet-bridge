// PXLBLZ compat library: minimal LEDMatrix (cLEDMatrix) stand-in on a plain row-major raster
// (index = y * width + x), the same raster as the bundled neomatrix_config.h. Negative sizes and
// zigzag/block layouts are accepted but always mapped row-major (PXLBLZ maps the raster itself).
#pragma once
#include <FastLED.h>
#include <stdlib.h>
enum MatrixType_t { HORIZONTAL_MATRIX, VERTICAL_MATRIX, HORIZONTAL_ZIGZAG_MATRIX, VERTICAL_ZIGZAG_MATRIX };
enum BlockType_t { HORIZONTAL_BLOCKS, VERTICAL_BLOCKS, HORIZONTAL_ZIGZAG_BLOCKS, VERTICAL_ZIGZAG_BLOCKS };
template <int16_t tW, int16_t tH, MatrixType_t tMT = HORIZONTAL_MATRIX, int8_t tBW = 1, int8_t tBH = 1, BlockType_t tBT = HORIZONTAL_BLOCKS>
class cLEDMatrix {
 public:
  static const int16_t W = (tW < 0 ? -tW : tW) * tBW, H = (tH < 0 ? -tH : tH) * tBH;
  explicit cLEDMatrix(bool = true) {}
  void SetLEDArray(CRGB *p) { m = p; }
  int16_t Width() const { return W; }
  int16_t Height() const { return H; }
  CRGB *operator[](int i) { return &m[i]; }
  CRGB &operator()(int16_t x, int16_t y) { return (m && x >= 0 && y >= 0 && x < W && y < H) ? m[y * W + x] : dummy; }
  CRGB &operator()(int16_t i) { return (m && i >= 0 && i < W * H) ? m[i] : dummy; }
  void DrawPixel(int16_t x, int16_t y, CRGB c) { (*this)(x, y) = c; }
  void DrawLine(int16_t x0, int16_t y0, int16_t x1, int16_t y1, CRGB c) {
    int16_t dx = abs(x1 - x0), sx = x0 < x1 ? 1 : -1, dy = -abs(y1 - y0), sy = y0 < y1 ? 1 : -1, err = dx + dy;
    for (;;) { DrawPixel(x0, y0, c); if (x0 == x1 && y0 == y1) break; int16_t e2 = 2 * err; if (e2 >= dy) { err += dy; x0 += sx; } if (e2 <= dx) { err += dx; y0 += sy; } }
  }
  void DrawRectangle(int16_t x0, int16_t y0, int16_t x1, int16_t y1, CRGB c) { DrawLine(x0, y0, x1, y0, c); DrawLine(x1, y0, x1, y1, c); DrawLine(x1, y1, x0, y1, c); DrawLine(x0, y1, x0, y0, c); }
  void DrawFilledRectangle(int16_t x0, int16_t y0, int16_t x1, int16_t y1, CRGB c) { for (int16_t y = (y0 < y1 ? y0 : y1); y <= (y0 < y1 ? y1 : y0); y++) DrawLine(x0, y, x1, y, c); }
  void DrawCircle(int16_t xc, int16_t yc, uint16_t r, CRGB c) { for (int a = 0; a < 360; a += 3) DrawPixel(xc + (int16_t)(r * cos(a * 0.0174533)), yc + (int16_t)(r * sin(a * 0.0174533)), c); }
  void DrawFilledCircle(int16_t xc, int16_t yc, uint16_t r, CRGB c) { for (int16_t y = -r; y <= (int16_t)r; y++) for (int16_t x = -r; x <= (int16_t)r; x++) if (x * x + y * y <= r * r) DrawPixel(xc + x, yc + y, c); }
  void ShiftLeft() { for (int16_t y = 0; y < H; y++) { for (int16_t x = 0; x < W - 1; x++) (*this)(x, y) = (*this)(x + 1, y); (*this)(W - 1, y) = CRGB::Black; } }
  void ShiftRight() { for (int16_t y = 0; y < H; y++) { for (int16_t x = W - 1; x > 0; x--) (*this)(x, y) = (*this)(x - 1, y); (*this)(0, y) = CRGB::Black; } }
  void ShiftUp() { for (int16_t y = H - 1; y > 0; y--) for (int16_t x = 0; x < W; x++) (*this)(x, y) = (*this)(x, y - 1); for (int16_t x = 0; x < W; x++) (*this)(x, 0) = CRGB::Black; }
  void ShiftDown() { for (int16_t y = 0; y < H - 1; y++) for (int16_t x = 0; x < W; x++) (*this)(x, y) = (*this)(x, y + 1); for (int16_t x = 0; x < W; x++) (*this)(x, H - 1) = CRGB::Black; }
  void HorizontalMirror(bool = true) { for (int16_t y = 0; y < H; y++) for (int16_t x = 0; x < W / 2; x++) (*this)(W - 1 - x, y) = (*this)(x, y); }
  void VerticalMirror() { for (int16_t y = 0; y < H / 2; y++) for (int16_t x = 0; x < W; x++) (*this)(x, H - 1 - y) = (*this)(x, y); }
  void QuadrantMirror() { HorizontalMirror(); VerticalMirror(); }
 private:
  CRGB *m = nullptr;
  CRGB dummy;
};
