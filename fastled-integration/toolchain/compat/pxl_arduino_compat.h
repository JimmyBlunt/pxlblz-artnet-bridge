// Arduino / older-FastLED compatibility for imported user sketches (sketch TUs only; the
// precompiled FastLED library is not affected). Included right after FastLED.h: on the PCH
// path by -include after -include-pch, otherwise by the FastLED.h wrapper in this folder.
// Everything here only adds names that are missing on FastLED's stub platform, so sketches
// that compiled before keep their exact results. Documented in README.md
// ("Arduino-Kompatibilität"). Two further items live in toolchain/toolchain.mjs because no
// header can do them: "struct CRGB" and a global variable named "index" (see the end).
#pragma once
#include <stddef.h>

// lib8tion functions that FastLED 3.10 keeps in fl:: but no longer re-exports globally
// (lib8tion.h only has 'using fl::dim8_raw'). Same implementations, so results are identical.
using fl::dim8_video;
using fl::dim8_lin;
using fl::brighten8_raw;
using fl::brighten8_video;
using fl::brighten8_lin;

// memcpy/memmove/memset/memcmp on CRGB (fl::) buffers: argument-dependent lookup also finds
// FastLED's fl::memcpy etc., so a plain call becomes ambiguous with ::memcpy. These typed
// overloads are an exact match and win; they do exactly what the C functions do.
template <class T> inline void *memcpy(T *dest, const T *src, size_t n) { return __builtin_memcpy(dest, src, n); }
template <class T> inline void *memmove(T *dest, const T *src, size_t n) { return __builtin_memmove(dest, src, n); }
template <class T> inline void *memset(T *dest, int value, size_t n) { return __builtin_memset(dest, value, n); }
template <class T> inline int memcmp(const T *a, const T *b, size_t n) { return __builtin_memcmp(a, b, n); }

// FastLED < 3.10 namespace macro ("FASTLED_USING_NAMESPACE" on its own line): nothing to do.
#ifndef FASTLED_USING_NAMESPACE
#define FASTLED_USING_NAMESPACE
#endif

// Arduino.h math constants. Exactly Arduino's token sequences: FastLED's own animartrix
// header defines PI with the same digits, so including it afterwards stays warning-free.
#ifndef PI
#define PI 3.1415926535897932384626433832795
#endif
#ifndef HALF_PI
#define HALF_PI 1.5707963267948966192313216916398
#endif
#ifndef TWO_PI
#define TWO_PI 6.283185307179586476925286766559
#endif
#ifndef DEG_TO_RAD
#define DEG_TO_RAD 0.017453292519943295769236907684886
#endif
#ifndef RAD_TO_DEG
#define RAD_TO_DEG 57.295779513082320876798154814105
#endif

// analogReference() modes (AVR values; the stub's analogReference(int) ignores them).
#ifndef DEFAULT
#define DEFAULT 1
#endif
#ifndef EXTERNAL
#define EXTERNAL 0
#endif
#ifndef INTERNAL
#define INTERNAL 3
#endif
#ifndef INTERNAL1V1
#define INTERNAL1V1 2
#endif
#ifndef INTERNAL2V56
#define INTERNAL2V56 3
#endif

// Memory-placement attributes (Teensy / ESP / AVR): plain RAM here.
#ifndef DMAMEM
#define DMAMEM
#endif
#ifndef FASTMEM
#define FASTMEM
#endif
#ifndef FLASHMEM
#define FLASHMEM
#endif
#ifndef EXTMEM
#define EXTMEM
#endif
#ifndef PROGMEM
#define PROGMEM
#endif

// avr/pgmspace.h helpers: flash is ordinary memory here, so a read is a plain load.
#ifndef PGM_P
#define PGM_P const char *
#endif
#ifndef PSTR
#define PSTR(s) (s)
#endif
#ifndef pgm_read_byte
#define pgm_read_byte(addr) (*(const unsigned char *)(addr))
#endif
#ifndef pgm_read_word
#define pgm_read_word(addr) (*(const unsigned short *)(addr))
#endif
#ifndef pgm_read_dword
#define pgm_read_dword(addr) (*(const unsigned long *)(addr))
#endif
#ifndef pgm_read_float
#define pgm_read_float(addr) (*(const float *)(addr))
#endif
#ifndef pgm_read_ptr
#define pgm_read_ptr(addr) (*(void *const *)(addr))
#endif
#ifndef pgm_read_byte_near
#define pgm_read_byte_near(addr) pgm_read_byte(addr)
#define pgm_read_word_near(addr) pgm_read_word(addr)
#define pgm_read_dword_near(addr) pgm_read_dword(addr)
#endif
#ifndef pgm_read_byte_far
#define pgm_read_byte_far(addr) pgm_read_byte(addr)
#define pgm_read_word_far(addr) pgm_read_word(addr)
#define pgm_read_dword_far(addr) pgm_read_dword(addr)
#endif
#ifndef memcpy_P
#define memcpy_P(dest, src, n) __builtin_memcpy((dest), (src), (n))
#endif
#ifndef strcpy_P
#define strcpy_P(dest, src) __builtin_strcpy((dest), (src))
#endif
#ifndef strlen_P
#define strlen_P(s) __builtin_strlen(s)
#endif
#ifndef strcmp_P
#define strcmp_P(a, b) __builtin_strcmp((a), (b))
#endif

// Arduino binary.h (B0 .. B11111111).
#include "pxl_binary.h"

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

// A global variable named 'index' (Tuline's inoise8_fire, FunkyNoise ...) collides with
// POSIX 'char *index(const char *, int)' from wasi-libc's <strings.h>: it is declared because
// -std=gnu++17 enables _BSD_SOURCE, and the precompiled header has already included it, so it
// cannot be hidden. toolchain.mjs passes -DPXL_COMPAT_RENAME_INDEX only for a TU whose own
// text declares 'index' at file scope; the name is then renamed for the rest of that TU
// (consistently, so the sketch behaves the same). TUs without such a global are untouched.
#ifdef PXL_COMPAT_RENAME_INDEX
#define index pxl_sketch_index
#endif

// "struct CRGB" (FastLED < 3.10 sketches): CRGB is now 'using CRGB = fl::CRGB', and C++ forbids
// an elaborated type specifier on an alias ([dcl.type.elab]). A macro cannot fix it ('#define
// CRGB ::fl::CRGB' would break 'fl::CRGB'), so toolchain.mjs rewrites 'struct CRGB' used as a
// type to six spaces + ' CRGB' (same length, columns stay exact) in the sketch-folder sources it compiles.
