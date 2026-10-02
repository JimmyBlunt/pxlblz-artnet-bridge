// Fire2012 (FastLED example, Mark Kriegsman) hand-ported to the Pixelblaze language
// on top of the FL8 lib8tion port. One loop() == one beforeRender() call.
// Feasibility spike only.

var NUM_LEDS = 30
var COOLING = 55
var SPARKING = 120
var heat = array(NUM_LEDS)
var outR = array(NUM_LEDS), outG = array(NUM_LEDS), outB = array(NUM_LEDS)
var rand16seed = 1337

function random16() {
  rand16seed = rand16seed * 2053 + 13849
  return rand16seed
}
function random8() {
  random16()
  return ((rand16seed & 255) + ((rand16seed >> 8) & 255)) & 255
}
function random8lim(lim) { return (random8() * lim) >> 8 } // r*lim would overflow 16.16
function random8range(mn, lim) { return (random8lim((lim - mn) & 255) + mn) & 255 }
function qadd8(a, b) { return min(a + b, 255) }
function qsub8(a, b) { return max(a - b, 0) }
function scale8_video(i, sc) { return ((i * sc) >> 8) + ((i && sc) ? 1 : 0) }

function heatColor(j, temperature) {
  var t192 = scale8_video(temperature, 191)
  var heatramp = (t192 & 63) << 2
  if (t192 & 128) { outR[j] = 255; outG[j] = 255; outB[j] = heatramp }
  else if (t192 & 64) { outR[j] = 255; outG[j] = heatramp; outB[j] = 0 }
  else { outR[j] = heatramp; outG[j] = 0; outB[j] = 0 }
}

function fire2012() {
  var cool = floor((COOLING * 10) / NUM_LEDS) + 2
  for (var i = 0; i < NUM_LEDS; i++) heat[i] = qsub8(heat[i], random8range(0, cool))
  for (var k = NUM_LEDS - 1; k >= 2; k--) heat[k] = floor((heat[k - 1] + heat[k - 2] + heat[k - 2]) / 3)
  if (random8() < SPARKING) {
    var y = random8lim(7)
    heat[y] = qadd8(heat[y], random8range(160, 255))
  }
  for (var j = 0; j < NUM_LEDS; j++) heatColor(j, heat[j])
}

export function beforeRender(delta) {
  // random16_add_entropy(random16()): C++ evaluates the argument before += reads
  // rand16seed, so the seed doubles; a naive `seed = seed + random16()` is wrong.
  var e = random16()
  rand16seed = rand16seed + e
  fire2012()
}

export function render(index) {
  rgb(outR[index] / 255, outG[index] / 255, outB[index] / 255)
}
