// PXLBLZ engine test: multiple strips / color orders, brightness, correction,
// temperature, dithering, UI elements, Arduino random(), micros(), EVERY_N,
// delayMicroseconds, functions used before their definition (auto prototypes).
#include <FastLED.h>

#ifndef PXLBLZ_NUM_LEDS
#define PXLBLZ_NUM_LEDS 60
#endif
#define NUM_A PXLBLZ_NUM_LEDS
#define NUM_B 24
#define NUM_C 16

CRGB stripA[NUM_A];
CRGB stripB[NUM_B];
CRGB stripC[NUM_C];

UISlider speed("Speed", 40, 1, 200, 1);
UISlider bright("Brightness", 180, 0, 255, 1);
UICheckbox sparkle("Sparkle", true);
UINumberField hueShift("Hue shift", 0, -128, 127);

void setup() {
  Serial.begin(115200);
  FastLED.addLeds<WS2812B, 2, GRB>(stripA, NUM_A).setCorrection(TypicalLEDStrip);
  FastLED.addLeds<WS2811, 3, BGR>(stripB, NUM_B).setTemperature(Candle);
  FastLED.addLeds<WS2812, 4, RGB>(stripC, NUM_C);
  FastLED.setDither(BINARY_DITHER);
  randomSeed(42);
  delay(250);
  Serial.println("PipelineTest ready");
}

void loop() {
  FastLED.setBrightness((uint8_t)bright.value());
  uint8_t base = beat8((uint8_t)speed.value()) + (int)hueShift.value();
  drawA(base);
  drawB(base);
  EVERY_N_MILLISECONDS(37) { shiftC(); }
  if (sparkle) addSparkle(random(0, 100));
  delayMicroseconds(1500);
  FastLED.show();
  delay(7);
}

void drawA(uint8_t base) {
  fill_rainbow(stripA, NUM_A, base, 5);
  fadeLightBy(stripA, NUM_A, (millis() / 13) & 0x3f);
}

void drawB(uint8_t base) {
  for (int i = 0; i < NUM_B; i++) stripB[i] = CHSV(base + i * 9, 200, sin8(micros() / 4096 + i * 11));
}

void shiftC() {
  static uint8_t pos = 0;
  fadeToBlackBy(stripC, NUM_C, 64);
  stripC[pos++ % NUM_C] = CRGB(random8(), 0, random8(64));
}

void addSparkle(long chance) {
  if (chance < 30) stripA[random16(NUM_A)] += CRGB::White;
}
