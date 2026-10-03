// libm parity probe: one LED per math function = 24-bit hash of the exact result bits over
// 4096 pseudo-random arguments. Native reference and wasm must produce identical LEDs.
// Used by tests/verify.mjs (case MathParity) to catch libm differences between
// wasi-libc (musl / zig compiler_rt) and the native mingw-w64 build.
#include <FastLED.h>
#include <math.h>
#include <string.h>

#define N_FUNCS 48
CRGB leds[N_FUNCS];

static uint32_t rng = 12345;
static float rf(float lo, float hi) { rng = rng * 1664525u + 1013904223u; return lo + (hi - lo) * (float)(rng >> 8) / 16777216.0f; }
static double rd(double lo, double hi) { rng = rng * 1664525u + 1013904223u; uint32_t a = rng; rng = rng * 1664525u + 1013904223u;
  return lo + (hi - lo) * ((double)a * 4294967296.0 + (double)rng) / 18446744073709551616.0; }
static uint32_t hf(uint32_t h, float v) { uint32_t b; memcpy(&b, &v, 4); return (h ^ b) * 16777619u; }
static uint32_t hd(uint32_t h, double v) { uint64_t b; memcpy(&b, &v, 8); h = (h ^ (uint32_t)b) * 16777619u; return (h ^ (uint32_t)(b >> 32)) * 16777619u; }

typedef uint32_t (*Probe)(uint32_t h);
#define PF(name, expr, lo, hi) [](uint32_t h) { for (int i = 0; i < 4096; ++i) { float x = rf(lo, hi); float y = rf(lo, hi); (void)y; h = hf(h, expr); } return h; }
#define PD(name, expr, lo, hi) [](uint32_t h) { for (int i = 0; i < 4096; ++i) { double x = rd(lo, hi); double y = rd(lo, hi); (void)y; h = hd(h, expr); } return h; }

static const Probe probes[N_FUNCS] = {
  PF(sinf, sinf(x), -100, 100), PF(cosf, cosf(x), -100, 100), PF(tanf, tanf(x), -10, 10), PF(asinf, asinf(x), -1, 1),
  PF(acosf, acosf(x), -1, 1), PF(atanf, atanf(x), -50, 50), PF(atan2f, atan2f(x, y), -50, 50), PF(sqrtf, sqrtf(x), 0, 1e6),
  PF(expf, expf(x), -50, 50), PF(exp2f, exp2f(x), -50, 50), PF(logf, logf(x), 0.001f, 1e6), PF(log2f, log2f(x), 0.001f, 1e6),
  PF(log10f, log10f(x), 0.001f, 1e6), PF(powf, powf(x, y), 0.01f, 8), PF(fmodf, fmodf(x, y), -100, 100), PF(sinhf, sinhf(x), -20, 20),
  PF(coshf, coshf(x), -20, 20), PF(tanhf, tanhf(x), -10, 10), PF(hypotf, hypotf(x, y), -1e3f, 1e3f), PF(cbrtf, cbrtf(x), -1e6f, 1e6f),
  PF(floorf, floorf(x), -1e4f, 1e4f), PF(roundf, roundf(x), -1e4f, 1e4f), PF(expm1f, expm1f(x), -5, 5), PF(log1pf, log1pf(x), -0.9f, 100),
  PD(sin, sin(x), -100, 100), PD(cos, cos(x), -100, 100), PD(tan, tan(x), -10, 10), PD(asin, asin(x), -1, 1),
  PD(acos, acos(x), -1, 1), PD(atan, atan(x), -50, 50), PD(atan2, atan2(x, y), -50, 50), PD(sqrt, sqrt(x), 0, 1e6),
  PD(exp, exp(x), -50, 50), PD(exp2, exp2(x), -50, 50), PD(log, log(x), 0.001, 1e6), PD(log2, log2(x), 0.001, 1e6),
  PD(log10, log10(x), 0.001, 1e6), PD(pow, pow(x, y), 0.01, 8), PD(fmod, fmod(x, y), -100, 100), PD(sinh, sinh(x), -20, 20),
  PD(cosh, cosh(x), -20, 20), PD(tanh, tanh(x), -10, 10), PD(hypot, hypot(x, y), -1e3, 1e3), PD(cbrt, cbrt(x), -1e6, 1e6),
  PD(floor, floor(x), -1e4, 1e4), PD(round, round(x), -1e4, 1e4), PD(expm1, expm1(x), -5, 5), PD(log1p, log1p(x), -0.9, 100),
};

void setup() {
  FastLED.addLeds<WS2812B, 2, RGB>(leds, N_FUNCS);
  for (int f = 0; f < N_FUNCS; ++f) {
    uint32_t h = probes[f](2166136261u);
    leds[f] = CRGB(h & 0xff, (h >> 8) & 0xff, (h >> 16) & 0xff);
  }
  for (int f = 0; f < N_FUNCS; ++f) { Serial.print(f); Serial.print(' '); Serial.println((uint32_t)leds[f].r | ((uint32_t)leds[f].g << 8) | ((uint32_t)leds[f].b << 16)); }
}

void loop() {
  FastLED.show();
  delay(100);
}
