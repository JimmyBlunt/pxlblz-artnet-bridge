// PXLBLZ compat library: replacement for Marc Merlin's neomatrix_config.h (the configuration
// header of the FastLED_NeoMatrix / SmartMatrix / LEDMatrix GFX demos and of sketches derived
// from them). Instead of the 20 hardware backends it provides ONE FastLED controller with a
// row-major mw x mh raster, registered with setScreenMap(mw, mh) so PXLBLZ's surface projection
// recognises the 2D raster. Same globals as the original's M32BY8X3 block (the user's 24 x 32
// matrix of three 8 x 32 tiles): matrixleds, matrix (FastLED_NeoMatrix*), mw, mh, NUMMATRIX,
// NUM_LEDS, MATRIX_WIDTH/HEIGHT, kMatrixWidth/Height, MATRIX_TILE_*, matrix_brightness, gHue,
// speed, matrix_gamma, XY(), XY2(), wrapX(), die(), mallocordie(), show_free_mem(), matrix_setup().
//
// neomatrix_config.h (next to this file) includes it with the defaults. A sketch that needs other
// values ships its own short neomatrix_config.h (a sketch's file wins over this folder):
//   #define PXL_MATRIX_WIDTH 32        // default 24
//   #define PXL_MATRIX_HEIGHT 32       // default 32
//   #define PXL_MATRIX_BRIGHTNESS 50   // matrix_brightness, default 64 (applied in matrix_setup())
//   #define PXL_MATRIXLEDS2            // also declare a second buffer 'matrixleds2'
//   #define PXL_NO_GHUE                // the sketch declares its own 'gHue'
//   #define PXL_NO_SPEED               // the sketch declares its own 'speed' (or must not see one)
//   #include <pxl_neomatrix_config.h>
// Text output (matrix->print ...) compiles but draws nothing (no fonts, see Adafruit_GFX.h).
#ifndef neomatrix_config_h
#define neomatrix_config_h
#include <FastLED.h>
#include "Framebuffer_GFX.h"
#ifdef LEDMATRIX
#include "LEDMatrix.h"
#endif

#ifndef PXL_MATRIX_WIDTH
#define PXL_MATRIX_WIDTH 24
#endif
#ifndef PXL_MATRIX_HEIGHT
#define PXL_MATRIX_HEIGHT 32
#endif
#ifndef PXL_MATRIX_BRIGHTNESS
#define PXL_MATRIX_BRIGHTNESS 64
#endif
#define FASTLED_NEOMATRIX
#ifndef ARRAY_SIZE
#define ARRAY_SIZE(A) (sizeof(A) / sizeof((A)[0]))
#endif

bool init_done = 0;
uint32_t tft_spi_speed;
uint8_t matrix_brightness = PXL_MATRIX_BRIGHTNESS;
const uint16_t MATRIX_TILE_WIDTH = PXL_MATRIX_WIDTH;
const uint16_t MATRIX_TILE_HEIGHT = PXL_MATRIX_HEIGHT;
const uint8_t MATRIX_TILE_H = 1;
const uint8_t MATRIX_TILE_V = 1;
const uint16_t mw = PXL_MATRIX_WIDTH;
const uint16_t mh = PXL_MATRIX_HEIGHT;
const uint32_t NUMMATRIX = mw * mh;
const uint32_t NUM_LEDS = NUMMATRIX;
const uint16_t MATRIX_HEIGHT = mh;
const uint16_t MATRIX_WIDTH = mw;
const uint16_t kMatrixWidth = mw;
const uint16_t kMatrixHeight = mh;

// One extra entry: XY() returns NUMMATRIX for coordinates outside the raster (a hidden pixel,
// like the original's out-of-range handling), so writes there never corrupt memory.
CRGB matrixleds_storage[NUMMATRIX + 1];
CRGB *matrixleds = matrixleds_storage;
#ifdef PXL_MATRIXLEDS2
CRGB matrixleds2_storage[NUMMATRIX + 1];
CRGB *matrixleds2 = matrixleds2_storage;
#endif
FastLED_NeoMatrix *matrix = new FastLED_NeoMatrix(matrixleds, mw, mh);
#ifdef LEDMATRIX
cLEDMatrix<PXL_MATRIX_WIDTH, PXL_MATRIX_HEIGHT, HORIZONTAL_MATRIX> ledmatrix(false);
#endif
#ifndef PXL_NO_GHUE
uint8_t gHue = 0;  // rotating "base color" used by many of the patterns
#endif
#ifndef PXL_NO_SPEED
uint16_t speed = 255;  // global animation speed of the GFX demos
#endif
float matrix_gamma = 1;

// Like XY, but mirrored vertically (used by some demos).
int XY2(int x, int y, bool wrap = false) { (void)wrap; return matrix->XY(x, MATRIX_HEIGHT - 1 - y); }
uint16_t XY(uint8_t x, uint8_t y) { return matrix->XY(x, y); }
int wrapX(int x) { if (x < 0) return 0; if (x >= MATRIX_WIDTH) return MATRIX_WIDTH - 1; return x; }
void show_free_mem(const char * = NULL) {}
void die(const char *mesg) { Serial.println(mesg); while (1) delay(1); }
void *mallocordie(const char *, uint32_t req, bool = true) { return malloc(req); }

void matrix_setup(int reservemem = 40000) {
  (void)reservemem;
  if (init_done) return;
  init_done = 1;
  Serial.begin(115200);
  // Clockless controller (L2 wire bytes available); the 2D raster goes to PXLBLZ as a screen map.
  FastLED.addLeds<NEOPIXEL, 13>(matrixleds, NUMMATRIX).setScreenMap(mw, mh);
#ifdef LEDMATRIX
  ledmatrix.SetLEDArray(matrixleds);
#endif
  matrix->begin();
  FastLED.setBrightness(matrix_brightness);
}
#endif  // neomatrix_config_h
