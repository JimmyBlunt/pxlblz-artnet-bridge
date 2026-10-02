// Force-included (-include) into every user sketch TU, like the Arduino IDE
// implicitly includes <Arduino.h>. FastLED.h itself is NOT included here, so
// '#define FASTLED_xxx' lines before '#include <FastLED.h>' keep working
// (see pxl_sketch_fastled.h for the precompiled fast path).
#pragma once
#include "platforms/stub/Arduino.h"  // Serial, map(), random(a,b), pinMode/digitalWrite stubs ...
#include "fl/system/delay.h"

// Arduino random() without arguments and randomSeed(). rand/srand/random are renamed
// to the deterministic pxl_* implementations by -D flags (see toolchain/toolchain.mjs).
extern "C" long pxl_random(void);
extern "C" void pxl_srand(unsigned);
inline void randomSeed(unsigned long seed) { if (seed != 0) pxl_srand((unsigned)seed); }
using fl::delayMicroseconds;  // Arduino core function (FastLED keeps it in fl::)
