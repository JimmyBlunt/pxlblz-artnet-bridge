function (pixelCount) {
  // PXLBLZ custom map for the ESP32 test controller (172.20.10.2).
  // Paste into PXLBLZ: Maps -> new custom map -> source. 2D, 136 pixels.
  //
  // Pixel order = order sent to the Art-Net router
  // (router/config/routes.esp-test-172-8x8-12x6.json):
  //   0..63    8x8 WS2812B matrix                    ESP out0 -> U120
  //   64..135  Adafruit DotStar FeatherWing 12x6     ESP out6 -> U149
  //
  // Layout: the FeatherWing on top, the 8x8 matrix centered below it.
  // Units = one LED position per grid (the two boards have different pitches).
  // FeatherWing wiring follows Adafruit's DotStarMatrix example
  // (TOP + LEFT + ROWS + PROGRESSIVE). If an index test shows mirrored rows or
  // columns, flip the switches below; nothing else has to change.
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

  return grid(MATRIX).concat(grid(FEATHERWING))
}
