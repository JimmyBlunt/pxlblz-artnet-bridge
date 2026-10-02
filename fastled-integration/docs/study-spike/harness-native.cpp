// Native reference runner for an unmodified FastLED example sketch (stub platform).
// Virtual, deterministic clock: fl::millis() is injected, delay() advances it.
// Writes per loop() iteration: leds[] (logical CRGB) and the strip's wire bytes
// as last seen by the stub controller (after brightness/correction/dither).
//   build: zig c++ ... -DSKETCH=\"path.ino\" -DFRAMES=600
#include <cstdio>
#include <cstdint>
#include "FastLED.h"
#include "platforms/stub/time_stub.h"
#include "platforms/shared/active_strip_data/active_strip_data.h"

static fl::u32 g_ms = 0;

#include SKETCH

int main() {
    fl::inject_time_provider([]() -> fl::u32 { return g_ms; });
    setDelayFunction([](fl::u32 ms) { g_ms += ms; });
    FILE *fl_ = fopen(OUT_PREFIX ".leds.bin", "wb");
    FILE *fw = fopen(OUT_PREFIX ".wire.bin", "wb");
    setup();
    for (int f = 0; f < FRAMES; f++) {
        loop();
        fwrite(leds, sizeof(CRGB), NUM_LEDS, fl_);
        const auto &data = fl::ActiveStripData::Instance().getData();
        bool wrote = false;
        for (const auto &kv : data) { fwrite(kv.second.data(), 1, kv.second.size(), fw); wrote = true; break; }
        if (!wrote) { static uint8_t z[3 * NUM_LEDS] = {0}; fwrite(z, 1, sizeof z, fw); }
    }
    fclose(fl_); fclose(fw);
    printf("%d frames, %d leds, virtual ms %u\n", FRAMES, NUM_LEDS, (unsigned)g_ms);
    return 0;
}
