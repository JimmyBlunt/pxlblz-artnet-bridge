// Native reference: calls FastLED's real lib8tion functions (stub/host build).
// Emits the same vectors as run-spike.ts so the JS C-semantics reference can be
// validated against the actual library.
#include <cstdio>
#include <cstdint>
#include "FastLED.h"

static uint32_t g_ms = 0;

int main() {
    unsigned long long h = 1469598103934665603ULL; // FNV-1a over all outputs
    auto mix = [&](uint32_t v) { for (int k = 0; k < 4; k++) { h ^= (v >> (8 * k)) & 0xff; h *= 1099511628211ULL; } };
    for (int a = 0; a < 256; a++) for (int b = 0; b < 256; b++) mix(scale8(a, b));
    printf("scale8 %016llx\n", h); h = 1469598103934665603ULL;
    for (int a = 0; a < 256; a++) for (int b = 0; b < 256; b++) mix(qadd8(a, b));
    printf("qadd8 %016llx\n", h); h = 1469598103934665603ULL;
    for (int a = 0; a < 256; a++) for (int b = 0; b < 256; b++) mix(qsub8(a, b));
    printf("qsub8 %016llx\n", h); h = 1469598103934665603ULL;
    for (int a = 0; a < 256; a++) mix(sin8(a));
    printf("sin8 %016llx\n", h); h = 1469598103934665603ULL;
    for (int a = 0; a < 65536; a++) mix((uint16_t)sin16(a));
    printf("sin16 %016llx\n", h); h = 1469598103934665603ULL;
    random16_set_seed(1337);
    for (int k = 0; k < 100000; k++) mix(random16());
    printf("random16 %016llx\n", h); h = 1469598103934665603ULL;
    random16_set_seed(1337);
    for (int k = 0; k < 100000; k++) mix(random8());
    printf("random8 %016llx\n", h); h = 1469598103934665603ULL;
    // beat8 via timebase: beat88 uses (millis() - timebase); emulate t by timebase = millis() - t
    const int bpms[] = {1, 10, 30, 60, 62, 120, 140, 200, 255};
    auto times = [&](auto fn) {
        for (int bpm : bpms) {
            for (uint32_t t = 0; t < 600000; t += 997) fn(bpm, t);
            for (uint32_t t = 86400000u; t < 86400000u + 200000u; t += 991) fn(bpm, t);
            for (uint64_t t = 4294967295ull - 100000ull; t <= 4294967295ull; t += 983) fn(bpm, (uint32_t)t);
        }
    };
    fl::inject_time_provider([]() -> fl::u32 { return g_ms; });
    times([&](int bpm, uint32_t t) { g_ms = t; mix(beat8(bpm)); });
    printf("beat8 %016llx\n", h); h = 1469598103934665603ULL;
    times([&](int bpm, uint32_t t) { g_ms = t; mix(beatsin8(bpm, 20, 230, 0, 64)); });
    printf("beatsin8 %016llx\n", h);
    (void)g_ms;
    return 0;
}
