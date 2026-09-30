function (pixelCount) {
  // Pixelblaze / PXLBLZ 2D mapping for the ESP32 test rig (172.20.10.2).
  // Works as a Pixelblaze mapper function and as a PXLBLZ custom-map source.
  //
  // ONE APA102 chain on ESP out6 (U149), 136 pixels, data flows:
  //   ESP out6 -> 8x8 matrix (pixels 0..63) -> DotStar FeatherWing 12x6 (pixels 64..135)
  //
  // Physical layout: the FeatherWing sits ABOVE the 8x8 matrix, matrix centered.
  // Units = one LED position per board (the boards have different pitches).
  // FeatherWing wiring follows Adafruit's DotStarMatrix example
  // (TOP + LEFT + ROWS + PROGRESSIVE). If a test pattern shows mirrored rows
  // or columns, flip the switches below; nothing else has to change.
  var MATRIX = { w: 8, h: 8, x0: 2, y0: 7, serpentine: true, flipX: false, flipY: false }
  var FEATHERWING = { w: 12, h: 6, x0: 0, y0: 0, serpentine: false, flipX: false, flipY: false }

  function grid(g) {
    var pts = []
    for (var i = 0; i < g.w * g.h; i++) {
      var row = Math.floor(i / g.w)
      var col = i % g.w
      if (g.serpentine && row % 2 === 1) col = g.w - 1 - col
      if (g.flipX) col = g.w - 1 - col
      if (g.flipY) row = g.h - 1 - row
      pts.push([g.x0 + col, g.y0 + row])
    }
    return pts
  }

  // chain order: matrix first, FeatherWing last
  return grid(MATRIX).concat(grid(FEATHERWING))
}
