// PXLBLZ compat library: <avr/pgmspace.h> for AVR sketches. Flash is ordinary memory here; the
// PROGMEM / pgm_read_* / memcpy_P ... macros come from toolchain/compat/pxl_arduino_compat.h
// (included with FastLED.h), this header only has to exist.
#pragma once
#include <string.h>
#include <stdint.h>
