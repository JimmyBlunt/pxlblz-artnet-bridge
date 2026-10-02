// FL8 — candidate FastLED lib8tion port written in the Pixelblaze language.
// Feasibility spike only (not an IDE library). Conventions:
//   u8  -> integer 0..255
//   u16 -> stored as int16 two's complement (-32768..32767); 16.16 raw int32
//          add/mul wrap exactly at 2^16 in the value domain, which is u16 math.
//   u32 millis -> two u16 halves (msHi, msLo) in int16 form.

var b_m16 = [0, 49, 49, 41, 90, 27, 117, 10]
var s16base = [0, 6393, 12539, 18204, 23170, 27245, 30273, 32137]
var s16slope = [49, 48, 44, 38, 31, 23, 14, 4]
var rand16seed = 1337

function lo8(x) { return x & 255 }
function hi8(x) { return (x >> 8) & 255 }

function scale8(i, sc) { return floor(i / 256 * (sc + 1)) }
function qadd8(a, b) { return min(a + b, 255) }
function qsub8(a, b) { return max(a - b, 0) }

function sin8(theta) {
  var offset = theta
  if (theta & 64) offset = 255 - offset
  offset = offset & 63
  var secoffset = offset & 15
  if (theta & 64) secoffset = secoffset + 1
  var s2 = (offset >> 4) * 2
  var b = b_m16[s2]
  var m16 = b_m16[s2 + 1]
  var y = ((m16 * secoffset) >> 4) + b
  if (theta & 128) y = -y
  return y + 128
}

// theta is u16 in int16 form
function sin16(theta) {
  var offset = (theta & 16383) >> 3
  if (theta & 16384) offset = 2047 - offset
  var section = offset >> 8
  var secoffset8 = (offset & 255) >> 1
  var y = s16slope[section] * secoffset8 + s16base[section]
  if (theta < 0) y = -y
  return y
}

function random16() {
  rand16seed = rand16seed * 2053 + 13849 // wraps mod 2^16 in 16.16
  return rand16seed
}
function random8() {
  random16()
  return (lo8(rand16seed) + hi8(rand16seed)) & 255
}
function random8lim(lim) { return (random8() * lim) >> 8 }

// beat88: ((millis*bpm88*280) mod 2^32) >> 16, result u16 (int16 form).
// t = th:tl, K = kh:kl (16-bit halves, int16 form)
// r = th*kl + tl*kh + floor(tl*kl / 65536)   (mod 2^16)
function mulhi16(a, b) {
  // floor(u16(a)*u16(b)/65536) mod 2^16, a/b in int16 form
  var au = a < 0 ? a + 32768 : a // low 15 bits
  var ah = a < 0 ? 1 : 0         // bit 15
  var bl = b & 255
  var bh = (b >> 8) & 255
  // a/65536 kept as fraction: (au + ah*32768)/65536
  var af = au / 256 / 256 + ah * 0.5
  var s = af * bh * 256 + af * bl
  return floor(s)
}
function beat88(bpm88, th, tl) {
  // K = bpm88 * 280 as 32-bit: split bpm88 (<= 65535, int16 form) * 280
  var bu = bpm88 < 0 ? bpm88 + 32768 : bpm88
  var bt = bpm88 < 0 ? 1 : 0
  // K = (bu + bt*32768) * 280 ; kh = floor(K/65536), kl = K mod 65536 (int16 form)
  var kh = floor(bu / 256 / 256 * 280) + bt * 140
  var kl = bu * 280 // wraps mod 2^16; bit 15 * 280 contributes 0 mod 2^16
  return th * kl + tl * kh + mulhi16(tl, kl)
}
function beat16(bpm, th, tl) {
  if (bpm >= 0 && bpm < 256) bpm = bpm * 256
  return beat88(bpm, th, tl)
}
function beat8(bpm, th, tl) { return (beat16(bpm, th, tl) >> 8) & 255 }
function beatsin8(bpm, lowest, highest, th, tl, phase) {
  var beat = beat8(bpm, th, tl)
  var bs = sin8((beat + phase) & 255)
  var range = highest - lowest
  var result = lowest + scale8(bs, range)
  return result
}

export var P1 = 0, P2 = 0, P3 = 0, P4 = 0, P5 = 0, P6 = 0, MODE = 0, OUT = 0

export function render(index) {
  if (MODE == 1) OUT = scale8(P1, P2)
  else if (MODE == 2) OUT = qadd8(P1, P2)
  else if (MODE == 3) OUT = qsub8(P1, P2)
  else if (MODE == 4) OUT = sin8(P1)
  else if (MODE == 5) OUT = sin16(P1)
  else if (MODE == 6) OUT = random8()
  else if (MODE == 7) OUT = random16()
  else if (MODE == 8) { rand16seed = P1; OUT = 0 }
  else if (MODE == 9) OUT = beat8(P1, P2, P3)
  else if (MODE == 10) OUT = beatsin8(P1, P2, P3, P4, P5, P6)
  else if (MODE == 11) OUT = beat88(P1, P2, P3)
}
