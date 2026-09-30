// SnowFlake IceSparkle -> Pixelblaze / PXLBLZ, ESP test rig (2D)
// Carpet v0.6 - WATER RESONANCE TAIL LAB, ported from the 8x8x8 cube version.
//
// Test rig: ONE APA102 chain, 136 px, map pxlblz-integration/maps/esp-test-8x8-12x6.js
//   pixels   0..63   8x8 matrix         (bottom, centered)
//   pixels  64..135  DotStar FeatherWing 12x6 (top)
// Both boards form one continuous 12 x 15 cell carpet (FeatherWing rows 0..5,
// one empty row, matrix rows 7..14 at columns 2..9).
//
// Cell positions come from the MAP (x, y), not from index arithmetic: the first
// frame measures each board's coordinate range, so flipping the map's
// flipX / flipY / serpentine switches keeps the pattern correct.
//
// Unchanged from the cube version: entry, living peak, the four tails
// (A Long Reverse, B Water Resonance, C Reflected Waves, D Organic Water),
// 60 s per variant, timing and hardness mapping.
//
// Indicator: 1/2/3/4 BLUE dots in the bottom row of the 8x8, left side.
// Router config while the ESP out6 is still 127 px: routes.esp-test-172-chain-compat.json
// (the last 9 FeatherWing LEDs stay dark until out6 = 136 px).

export var hue = 0.95
export var saturation = 0.78
export var featherWing = 1
export var speed = 0.38
export var entry = 0.58
export var carpetFlutter = 0.56

export function sliderHue(v) { hue = v }
export function sliderSaturation(v) { saturation = v }
export function sliderFeatherWing(v) { featherWing = v }
export function sliderSpeed(v) { speed = v }
export function sliderEntry(v) { entry = v }
export function sliderCarpetFlutter(v) { carpetFlutter = v }

var t = 0
var compareClock = 0
var TAU = 6.28318530718
var PI = 3.14159265359

export var activeVariant = 1
export var secondsInVariant = 0

// --- rig geometry ----------------------------------------------------------
var MATRIX_PIXELS = 64          // chain order: matrix first, FeatherWing last
var WORLD_SPAN = 11 + 14        // max gx + max gy, normalises the diagonal to 0..1

// per-board coordinate range, measured during the first frame
var ready = 0
var framesSeen = 0
// 9999: Pixelblaze numbers are 16.16 fixed point (about +-32767)
var mMinX = 9999
var mMaxX = -9999
var mMinY = 9999
var mMaxY = -9999
var wMinX = 9999
var wMaxX = -9999
var wMinY = 9999
var wMaxY = -9999

// set per pixel in render2D, read by the resonance functions
var curSpatial = 0

function fract(v) { return v - floor(v) }
function clamp01(v) { return max(0, min(1, v)) }
function smooth01(v) { v = clamp01(v); return v*v*(3-2*v) }
function smoother01(v) { v = clamp01(v); return v*v*v*(v*(v*6-15)+10) }
function hash2(a,b) { return fract(sin(a*12.9898 + b*78.233 + 0.123) * 43758.5453) }
function hash1(a) { return fract(sin(a*91.3458 + 12.345) * 47453.5453) }

function toCell(v, lo, hi, cells) {
  if (hi - lo < 0.0001) return 0
  return max(0, min(cells, floor((v - lo) / (hi - lo) * cells + 0.5)))
}

function smoothRandom(pixel, phase, count, salt) {
  var p = fract(phase) * count
  var i = floor(p)
  var f = smooth01(fract(p))
  var a = hash2(pixel + salt, i)
  var b = hash2(pixel + salt, i + 1)
  return a + (b-a)*f
}

function hardRandom(pixel, phase, count, salt) {
  return hash2(pixel + salt, floor(fract(phase)*count))
}

// ---------------------------------------------------------------------------
// GOLD ENTRY SHAPE from v05.
// e=0 -> quiet, e=1 -> full brightness.
// ---------------------------------------------------------------------------
function carpetShape(e,index) {
  e = clamp01(e)
  var rise = smoother01(e)
  var window = sin(PI*e)
  var ep1 = hash2(index,91)*TAU
  var ep2 = hash2(index,92)*TAU
  var et1 = TAU*(0.85*e + 2.65*e*e) + ep1
  var et2 = TAU*(1.55*e + 4.20*e*e) + ep2
  var flutter = 0.62*sin(et1) + 0.38*sin(et2)
  var localStrength = 0.78 + 0.34*hash2(index,93)
  var amount = (0.075 + 0.11*carpetFlutter)*localStrength
  return clamp01(rise + window*amount*flutter)
}

function entryEnvelope(q,index) {
  var entrySpan = 6 + entry*10
  var entryStart = 56-entrySpan
  if (q < entryStart) return 0
  return carpetShape((q-entryStart)/entrySpan,index)
}

function livingPeak(u,cycle,index) {
  var p1 = hash2(cycle,111)*TAU + hash2(index,112)*0.55
  var p2 = hash2(cycle,113)*TAU + hash2(index,114)*0.35
  var w1 = sin(TAU*(1.10*u + 0.18*sin(TAU*u)) + p1)
  var w2 = sin(TAU*(2.15*u + 0.11*sin(TAU*u*0.71)) + p2)
  var flutter = 0.68*w1 + 0.32*w2
  var high = clamp01(0.885 + 0.105*flutter + 0.03*sin(TAU*(3.2*u)+p2))
  var join = smoother01((u-0.82)/0.18)
  return high*(1-join) + join
}

function peakHold(cycle) {
  return 4.2 + 2.0*hash1(cycle*4.73 + 2.0)
}

function resonanceWindow(u) {
  var s = sin(PI*clamp01(u))
  return s*s
}

function resonanceDecay(u,power) {
  return pow(max(0,1-u),power)
}

function tailSpan(slot,cycle) {
  var base = 6 + entry*10
  if (slot == 0) return base*1.72
  if (slot == 1) return base*1.95
  if (slot == 2) return base*2.08
  return base*(1.78 + 0.46*hash1(cycle*5.89 + 27.0))
}

function reverseBackbone(u,index) {
  return carpetShape(1-clamp01(u),index)
}

function waterResonance(u,cycle,index,strength) {
  var win = resonanceWindow(u)
  var decay = resonanceDecay(u,0.78)
  var p1 = hash2(cycle,201)*TAU + hash2(index,202)*0.38
  var p2 = hash2(cycle,203)*TAU + hash2(index,204)*0.26
  var w1 = sin(TAU*(2.55*u) + p1)
  var w2 = sin(TAU*(4.15*u) + p2)
  return strength * win * decay * (0.68*w1 + 0.32*w2)
}

// cube version: spatial = (ix + iz)/14 -> here the carpet diagonal curSpatial
function reflectedResonance(u,cycle,index,strength) {
  var spatial = curSpatial
  var win = resonanceWindow(u)
  var decay = resonanceDecay(u,0.72)
  var p = hash2(cycle,211)*TAU
  var outgoing = sin(TAU*(2.15*u - spatial*0.38) + p)
  var reflected = sin(TAU*(2.85*u + spatial*0.31) + p*0.63 + 1.1)
  return strength * win * decay * (0.62*outgoing + 0.38*reflected)
}

function organicWaterResonance(u,cycle,index) {
  var spatial = curSpatial
  var win = resonanceWindow(u)
  var decayPower = 0.58 + 0.26*hash1(cycle*7.13 + 41.0)
  var decay = resonanceDecay(u,decayPower)
  var strength = 0.055 + 0.055*hash1(cycle*3.91 + 42.0)
  var f1 = 2.05 + 0.85*hash1(cycle*6.07 + 43.0)
  var f2 = 3.65 + 1.10*hash1(cycle*4.31 + 44.0)
  var p1 = hash2(cycle,221)*TAU + hash2(index,222)*0.35
  var p2 = hash2(cycle,223)*TAU + hash2(index,224)*0.28
  var w1 = sin(TAU*(f1*u - spatial*0.28) + p1)
  var w2 = sin(TAU*(f2*u + spatial*0.23) + p2)
  var late = smoother01((u-0.48)/0.22) * (1-smoother01((u-0.90)/0.10))
  var w3 = sin(TAU*(1.75*u + spatial*0.19) + p1*0.47 + 2.0)
  return strength * win * decay * (0.58*w1 + 0.29*w2 + 0.13*late*w3)
}

function waterTail(q,cycle,index,slot) {
  var hold = peakHold(cycle)
  var span = tailSpan(slot,cycle)
  var u = clamp01((q-hold)/span)

  var env = reverseBackbone(u,index)

  if (slot == 1) {
    env += waterResonance(u,cycle,index,0.075)
  } else if (slot == 2) {
    env += waterResonance(u,cycle,index,0.060)
    env += reflectedResonance(u,cycle,index,0.055)
  } else if (slot == 3) {
    env += organicWaterResonance(u,cycle,index)
  }
  return clamp01(env)
}

function envelope(q,cycle,index,slot) {
  if (q >= 38) return entryEnvelope(q,index)

  var hold = peakHold(cycle)
  if (q < hold) return livingPeak(q/hold,cycle,index)

  var endQ = hold + tailSpan(slot,cycle)
  if (q < endQ) return waterTail(q,cycle,index,slot)
  return 0
}

export function beforeRender(delta) {
  // one full pass over all pixels measures the board ranges, then go live
  if (!ready) {
    framesSeen += 1
    if (framesSeen >= 2) ready = 1
    return
  }
  var dt = delta*0.001
  var rate = 0.55 + speed*0.9
  t += dt*rate
  compareClock += dt
  activeVariant = (floor(compareClock/60)%4)+1
  secondsInVariant = compareClock - floor(compareClock/60)*60
}

export function render2D(index,x,y) {
  var isMatrix = index < MATRIX_PIXELS

  if (!ready) {
    if (isMatrix) {
      mMinX = min(mMinX, x); mMaxX = max(mMaxX, x); mMinY = min(mMinY, y); mMaxY = max(mMaxY, y)
    } else {
      wMinX = min(wMinX, x); wMaxX = max(wMaxX, x); wMinY = min(wMinY, y); wMaxY = max(wMaxY, y)
    }
    hsv(0,0,0)
    return
  }

  // board-local cell -> carpet cell (gx 0..11, gy 0..14)
  var col, row, gx, gy, layerGain
  if (isMatrix) {
    col = toCell(x, mMinX, mMaxX, 7)
    row = toCell(y, mMinY, mMaxY, 7)
    gx = 2 + col
    gy = 7 + row
    layerGain = 1
  } else {
    col = toCell(x, wMinX, wMaxX, 11)
    row = toCell(y, wMinY, wMaxY, 5)
    gx = col
    gy = row
    layerGain = featherWing
  }

  var slot = floor(compareClock/60)%4

  // 1..4 blue markers: bottom row of the 8x8, left side
  if (isMatrix && row == 7 && col < 4) {
    if (col <= slot) hsv(0.62,1,0.35)
    else hsv(0.62,1,0)
    return
  }
  if (layerGain <= 0) { hsv(0,0,0); return }

  // Gold-reference micro glitter core.
  var basePeriod = 12.8 + 6.4*hash2(index,11)
  var basePhase = t/basePeriod + hash2(index,12)
  var base = 0.30 + 0.30*smoothRandom(index,basePhase,32,20)

  var sparklePeriod = 1.28 + 15.104*hash2(index,31)
  var sparklePhase = t/sparklePeriod + hash2(index,32)
  var hardRnd = hardRandom(index,sparklePhase,256,40)*0.8 - 0.4
  var softRnd = smoothRandom(index,sparklePhase,256,40)*0.8 - 0.4

  var absolutePhase = t/9.6
  var cycle = floor(absolutePhase)
  var scenePhase = fract(absolutePhase)
  curSpatial = (gx + gy)/WORLD_SPAN
  var localPhase = scenePhase - curSpatial*0.10
  localPhase -= (hash2(index,71)-0.5)*0.012*carpetFlutter
  var bend1 = sin(TAU*(t/2.7 + gx*0.115 - gy*0.071))
  var bend2 = sin(TAU*(t/4.9 - gx*0.057 + gy*0.133) + 1.7)
  localPhase -= (0.0018*bend1 + 0.0011*bend2)*carpetFlutter

  var qLocal = fract(localPhase)*56
  var env = envelope(qLocal,cycle,index,slot)

  var hardMix = smoother01((env-0.18)/0.58)
  var rnd = softRnd + (hardRnd-softRnd)*hardMix

  var v = clamp01(base + rnd*env)*layerGain
  v = pow(v,2.45)
  hsv(hue,saturation,v)
}
