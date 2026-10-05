// Arduino / older-FastLED compatibility for imported user sketches (sketch TUs only; the
// precompiled FastLED library is not affected). Included right after FastLED.h.
#pragma once
#include <stddef.h>

// lib8tion functions that FastLED 3.10 keeps in fl:: but no longer re-exports globally
// (lib8tion.h only has 'using fl::dim8_raw'). Same implementations, so results are identical.
using fl::dim8_video;
using fl::dim8_lin;
using fl::brighten8_raw;
using fl::brighten8_video;
using fl::brighten8_lin;

// memcpy/memmove/memset on CRGB (fl::) buffers: argument-dependent lookup also finds
// FastLED's fl::memcpy, so a plain call becomes ambiguous with ::memcpy. These typed
// overloads are an exact match and win; they do exactly what memcpy does.
template <class T> inline void *memcpy(T *dest, const T *src, size_t n) { return __builtin_memcpy(dest, src, n); }
template <class T> inline void *memmove(T *dest, const T *src, size_t n) { return __builtin_memmove(dest, src, n); }
template <class T> inline void *memset(T *dest, int value, size_t n) { return __builtin_memset(dest, value, n); }

#ifndef FASTLED_NO_ARDUINO_STUBS
// Arduino Stream parsing that FastLED's SerialEmulation lacks (sketches use it to switch
// animations from the serial monitor). There is no serial input on the PC, so these
// return Arduino's "nothing received" values; everything else is SerialEmulation's.
struct PxlSerialCompat : SerialEmulation {
  long parseInt() { return 0; }
  long parseInt(char) { return 0; }
  float parseFloat() { return 0.0f; }
  fl::string readString() { return fl::string(); }
  void setTimeout(unsigned long) {}
  bool find(const char *) { return false; }
};
inline PxlSerialCompat &pxl_serial_compat() { static PxlSerialCompat serial; return serial; }
#define Serial (pxl_serial_compat())
#endif
