// Sketch-side wrapper (first on the sketch include path only): the real FastLED.h followed
// by the Arduino compatibility names. The precompiled library never sees this file.
#pragma once
#include_next <FastLED.h>
#include "pxl_arduino_compat.h"
