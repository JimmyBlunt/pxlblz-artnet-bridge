#include <FastLED.h>
#define NUM_LEDS 10
CRGB leds[NUM_LEDS];

void setup() {
  FastLED.addLeds<WS2812B, 2, GRB>(leds, NUM_LEDS);
}

void loop() {
  fill_rainbow(leds, NUM_LEDS, hue++, 7);
  FastLED.show();
  undefinedHelper(3);
}
