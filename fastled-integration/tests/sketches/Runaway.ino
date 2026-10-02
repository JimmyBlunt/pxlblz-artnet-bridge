// Runaway / busy-wait behaviour test.
#include <FastLED.h>
#define NUM_LEDS 8
CRGB leds[NUM_LEDS];
UICheckbox runaway("Runaway", false);

void setup() { FastLED.addLeds<WS2812B, 2, GRB>(leds, NUM_LEDS); }

void loop() {
  // busy-wait 500 ms on millis(): must not freeze the host; time passes per frame
  unsigned long t = millis();
  while (millis() - t < 500) { }
  leds[0] = leds[0] ? CRGB::Black : CRGB::Red;
  FastLED.show();
  if (runaway) {
    for (;;) { leds[1].g++; FastLED.show(); }  // never yields time -> budget
  }
}
