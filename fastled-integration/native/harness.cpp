// Native golden-frame runner (reference). Drives the same runtime ABI as the
// JS host (runtime/fastledWasmHost.ts) through a Win32 fiber and writes a
// frame trace in the format of tests/trace.mjs.
//   usage: sketch.exe <schedule.bin> <out.trace> [events.txt]
//   schedule.bin: u32 LE dt_us per frame (the frame count is the file length / 4)
//   eager:        use FastLED's eager encode path (reference) instead of lazy encoding
//   events.txt:   lines "<frameIndex> <json>"  -> pxl_ui_set(json) before that frame
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <vector>
#include <string>

extern "C" {
int pxl_init(uint32_t seed);
uint32_t pxl_native_frame(uint32_t dt_us);
uint32_t pxl_led_count(void);
uint8_t *pxl_leds_ptr(void);
uint8_t *pxl_wire_rgb_ptr(void);
uint8_t *pxl_wire_raw_ptr(void);
uint32_t pxl_wire_raw_len(void);
uint32_t pxl_show_count(void);
uint32_t pxl_now_us_lo(void);
uint32_t pxl_now_us_hi(void);
int pxl_ui_set(const char *json);
void pxl_set_lazy(int on);
const char *pxl_ui_json(void);
}

static void put32(FILE *f, uint32_t v) { fwrite(&v, 4, 1, f); }

int main(int argc, char **argv) {
    if (argc < 3) { fprintf(stderr, "usage: %s schedule.bin out.trace [events.txt|-] [seed] [eager|lazy]\n", argv[0]); return 2; }
    FILE *fs = fopen(argv[1], "rb");
    if (!fs) { perror("schedule"); return 2; }
    std::vector<uint32_t> sched;
    uint32_t v;
    while (fread(&v, 4, 1, fs) == 1) sched.push_back(v);
    fclose(fs);
    std::vector<std::pair<int, std::string>> events;
    if (argc > 3 && strcmp(argv[3], "-") != 0) {
        FILE *fe = fopen(argv[3], "rb");
        if (fe) {
            char line[8192];
            while (fgets(line, sizeof line, fe)) {
                char *sp = strchr(line, ' ');
                if (!sp) continue;
                *sp = 0;
                std::string js(sp + 1);
                while (!js.empty() && (js.back() == '\n' || js.back() == '\r')) js.pop_back();
                events.push_back({atoi(line), js});
            }
            fclose(fe);
        }
    }
    uint32_t seed = argc > 4 ? (uint32_t)strtoul(argv[4], nullptr, 10) : 1u;
    FILE *fo = fopen(argv[2], "wb");
    if (!fo) { perror("out"); return 2; }
    if (argc > 5 && strcmp(argv[5], "eager") == 0) pxl_set_lazy(0);  // FastLED eager encode path
    pxl_init(seed);
    fwrite("PXLT", 1, 4, fo);
    put32(fo, (uint32_t)sched.size());
    const char *ui = pxl_ui_json();
    put32(fo, (uint32_t)strlen(ui));
    fwrite(ui, 1, strlen(ui), fo);
    for (size_t f = 0; f < sched.size(); ++f) {
        for (auto &e : events) if (e.first == (int)f) pxl_ui_set(e.second.c_str());
        uint32_t st = pxl_native_frame(sched[f]);
        uint32_t n = pxl_led_count();
        uint32_t rawLen = pxl_wire_raw_len();
        put32(fo, 0x464c5850u);  // 'PXLF'
        put32(fo, st);
        put32(fo, pxl_show_count());
        put32(fo, pxl_now_us_lo());
        put32(fo, pxl_now_us_hi());
        put32(fo, n);
        put32(fo, rawLen);
        fwrite(pxl_leds_ptr(), 1, n * 3, fo);
        fwrite(pxl_wire_rgb_ptr(), 1, n * 3, fo);
        fwrite(pxl_wire_raw_ptr(), 1, rawLen, fo);
    }
    fclose(fo);
    return 0;
}
