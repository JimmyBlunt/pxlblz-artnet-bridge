// PXLBLZ FastLED runtime: deterministic, single-threaded execution of an unmodified
// Arduino/FastLED sketch (setup()/loop()) driven by an external frame clock.
//
// The same file is compiled into the wasm32-wasi library (driven by the JS host
// through binaryen Asyncify) and into the native reference build (driven by a
// Win32 fiber, native/fiber_win.cpp). Both builds therefore run identical
// scheduling logic; the golden-frame test compares compiler/target behaviour.
//
// Frame semantics (see README.md, "Was ist ein IDE-Frame"):
//   * pxl_init(seed): runs setup() to completion. delay() inside setup() simply
//     advances the virtual clock. Afterwards the frame clock is aligned to the
//     sketch clock (frame time = end of setup).
//   * frame(dt): frame time += dt, then the sketch (loop() running in a fiber)
//     resumes and runs until it cannot proceed without the frame clock moving:
//       - delay(ms)/delayMicroseconds(us) whose wake-up time lies beyond the
//         frame time suspends until a later frame reaches it; the sketch clock
//         then continues exactly at the wake-up time (no drift, no dependence
//         on the IDE frame rate),
//       - a loop() iteration that consumed no virtual time (no delay) ends the
//         frame; the next iteration starts at the next frame time,
//       - polling millis()/micros() in a busy loop (> PXL_SPIN_LIMIT reads without
//         the clock moving) ends the frame the same way,
//       - a runaway budget (loop iterations / show() calls per frame) ends the
//         frame and reports PXL_STATUS_BUDGET.
//     The presented frame is the state of the last FastLED.show() that happened
//     at a virtual time <= frame time (shows in between are not visible, exactly
//     like sampling a real strip at the frame instants).
//   * frame(0) after the first frame does nothing (returns the cached frame).
//     The very first frame() call (any dt, also 0) runs the sketch.

#include "FastLED.h"
#include "fl/system/engine_events.h"
#include "platforms/shared/ui/json/ui.h"
#include "platforms/stub/time_stub.h"
#include <stdlib.h>
#include <string.h>

void setup();
void loop();

#define PXL_ABI_VERSION 1

#if defined(__wasi__) || defined(__wasm32__)
#define PXL_WASM 1
#define PXL_EXPORT(name) extern "C" __attribute__((used, export_name(#name)))
extern "C" __attribute__((import_module("env"), import_name("pxl_suspend"))) void pxl_host_suspend(void);
#else
#define PXL_WASM 0
#define PXL_EXPORT(name) extern "C"
extern "C" void pxl_host_suspend(void);  // native/fiber_win.cpp
#endif

typedef fl::u8 u8;
typedef fl::u32 u32;
typedef fl::u64 u64;

enum {
    PXL_STATUS_OK = 0,
    PXL_STATUS_BUDGET = 1,        // runaway loop()/show() budget hit this frame (sketch is resumable)
    PXL_STATUS_NOT_INIT = 2,
    PXL_STATUS_SKIPPED = 3,       // frame(0) after the first frame: nothing ran
};
enum {
    PXL_SUSPEND_NONE = 0,
    PXL_SUSPEND_DELAY = 1,        // waiting in delay() for the frame clock
    PXL_SUSPEND_IDLE = 2,         // loop() iteration without virtual time
    PXL_SUSPEND_SPIN = 3,         // busy polling of millis()/micros()
    PXL_SUSPEND_BUDGET = 4,       // runaway budget hit
};
enum { PHASE_BOOT = 0, PHASE_SETUP = 1, PHASE_LOOP = 2 };

#ifndef PXL_SPIN_LIMIT
#define PXL_SPIN_LIMIT 200000u    // clock reads without time advance -> busy wait
#endif
#ifndef PXL_SETUP_SPIN_LIMIT
#define PXL_SETUP_SPIN_LIMIT 10000u  // in setup(): every N reads, 1 ms passes
#endif
#define PXL_MAX_STRIPS 64
#ifndef PXL_FIRST_FRAME_MAX_US
#define PXL_FIRST_FRAME_MAX_US 60000000ull  // first frame follows the sketch at most 60 s (virtual)
#endif

namespace {

// ------------------------------------------------------------------ state
u64 g_now_us = 0;       // sketch (virtual) clock
u64 g_target_us = 0;    // frame clock
int g_phase = PHASE_BOOT;
bool g_in_fiber = false;
bool g_fiber_started = false;
bool g_first_frame_done = false;
u64 g_first_frame_start = 0;
u32 g_polls = 0;
u32 g_suspend_reason = PXL_SUSPEND_NONE;
u32 g_frame_status = PXL_STATUS_OK;
u32 g_budget_loops = 100000;    // loop() iterations per frame
u32 g_budget_shows = 20000;     // FastLED.show() calls per frame
u32 g_frame_loops = 0, g_frame_shows = 0;
u32 g_total_loops = 0, g_total_shows = 0, g_total_frames = 0;

// The very first frame() runs until the first show(): the frame clock follows the
// sketch (delays / busy-waits / idle iterations move it forward) instead of ending
// the frame, so the first call always renders a frame (bounded by PXL_FIRST_FRAME_MAX_US).
inline bool first_frame_follow() {
    return !g_first_frame_done && g_total_shows == 0 && g_now_us - g_first_frame_start < PXL_FIRST_FRAME_MAX_US;
}
u32 g_rand_next = 1;            // Arduino random()/rand() state (avr-libc algorithm)

struct Strip {                  // exported as 10 x i32
    u32 ledOffset, ledCount;    // into the L1 buffer (in LEDs)
    u32 wireOffset, wireLen;    // into the L2 buffers (in bytes)
    int32_t pin, colorOrder;    // EOrder value (octal digits, e.g. GRB = 0102)
    int32_t rgbw;               // 0 = RGB, 1 = RGBW, 2 = RGBWW
    int32_t isSpi, enabled, reserved;
};
Strip g_strips[PXL_MAX_STRIPS];
u32 g_strip_count = 0;

// presented frame (state of the last show)
u8 *g_l1 = nullptr; u32 g_l1_cap = 0, g_l1_leds = 0;
u8 *g_l2 = nullptr; u32 g_l2_cap = 0, g_l2_len = 0;
u8 *g_l2rgb = nullptr; u32 g_l2rgb_cap = 0, g_l2rgb_len = 0; bool g_l2rgb_valid = false;

// per-show pending capture (filled by the driver/controller overrides)
struct PendingWire { int32_t pin, isSpi; u32 off, len; };
PendingWire g_pw[PXL_MAX_STRIPS]; u32 g_pw_count = 0;
u8 *g_pw_buf = nullptr; u32 g_pw_cap = 0, g_pw_len = 0;
struct PendingNote { void *ctrl; int32_t order, rgbw; };
PendingNote g_pn[PXL_MAX_STRIPS]; u32 g_pn_count = 0;
// lazy encoding (see toolchain/override/platforms/stub/clockless_channel_stub.h)
typedef void (*EncodeFn)(void *);
struct PendingLazy { void *ctrl; EncodeFn fn; };
PendingLazy g_lz[PXL_MAX_STRIPS]; u32 g_lz_count = 0;
EncodeFn g_strip_fn[PXL_MAX_STRIPS];
void *g_strip_ctrl[PXL_MAX_STRIPS];
bool g_wire_dirty = false;
int g_lazy = 1;
u32 g_mat_off = 0; int g_mat_strip = -1;

// UI
char *g_ui_json = nullptr;
u32 g_ui_version = 0;
fl::JsonUiUpdateInput g_ui_input;

void ensure(u8 **buf, u32 *cap, u32 need) {
    if (need <= *cap) return;
    u32 n = *cap ? *cap : 1024;
    while (n < need) n *= 2;
    *buf = (u8 *)realloc(*buf, n);
    *cap = n;
}

// ------------------------------------------------------------------ scheduler
void suspend(u32 reason) {
    g_suspend_reason = reason;
    pxl_host_suspend();
    g_suspend_reason = PXL_SUSPEND_NONE;
}

// Called after a suspension that ended a frame early (idle / spin / budget):
// the sketch continues at the (new) frame time.
void catch_up() {
    if (g_target_us > g_now_us) g_now_us = g_target_us;
    g_polls = 0;
}

void advance(u64 us) {
    const u64 wake = g_now_us + us;
    if (g_phase == PHASE_LOOP && g_in_fiber) {
        if (wake > g_target_us && first_frame_follow()) g_target_us = wake;
        while (wake > g_target_us) suspend(PXL_SUSPEND_DELAY);
    }
    g_now_us = wake;
    g_polls = 0;
}

void poll_clock() {
    ++g_polls;
    if (g_phase == PHASE_LOOP && g_in_fiber) {
        if (g_polls > PXL_SETUP_SPIN_LIMIT && first_frame_follow()) {
            g_now_us += 1000;  // busy-wait before the first show: let 1 ms pass
            if (g_now_us > g_target_us) g_target_us = g_now_us;
            g_polls = 0;
        } else if (g_polls > PXL_SPIN_LIMIT) {
            suspend(PXL_SUSPEND_SPIN);
            catch_up();
        }
    } else if (g_polls > PXL_SETUP_SPIN_LIMIT) {
        g_now_us += 1000;  // busy-wait in setup(): let 1 ms pass
        g_polls = 0;
    }
}

void check_budget() {
    if (g_phase != PHASE_LOOP || !g_in_fiber) return;
    if (g_frame_loops > g_budget_loops || g_frame_shows > g_budget_shows) {
        g_frame_status = PXL_STATUS_BUDGET;
        suspend(PXL_SUSPEND_BUDGET);
        catch_up();
    }
}

// ------------------------------------------------------------------ frame capture
void finalize_show() {
    // L1: logical leds[] of every controller, in addLeds() order.
    u32 nStrips = 0, nLeds = 0;
    for (CLEDController *c = CLEDController::head(); c && nStrips < PXL_MAX_STRIPS; c = c->next()) {
        nLeds += (u32)c->size();
        ++nStrips;
    }
    ensure(&g_l1, &g_l1_cap, nLeds * 3 + 3);
    ensure(&g_l2, &g_l2_cap, g_pw_len + 4);
    u32 ledOff = 0, wireOff = 0, wireIdx = 0, i = 0;
    for (CLEDController *c = CLEDController::head(); c && i < PXL_MAX_STRIPS; c = c->next(), ++i) {
        Strip &s = g_strips[i];
        const u32 n = (u32)c->size();
        s.ledOffset = ledOff;
        s.ledCount = n;
        s.enabled = c->getEnabled() ? 1 : 0;
        if (n) memcpy(g_l1 + ledOff * 3, (const void *)c->leds(), n * 3);
        ledOff += n;
        // Pair with the wire capture: controllers that encoded this show reported a
        // note (in show order) and enqueued exactly one channel (same order).
        int noteIdx = -1;
        for (u32 k = 0; k < g_pn_count; ++k)
            if (g_pn[k].ctrl == (void *)c) { noteIdx = (int)k; break; }
        int lazyIdx = -1;
        for (u32 k = 0; k < g_lz_count; ++k)
            if (g_lz[k].ctrl == (void *)c) { lazyIdx = (int)k; break; }
        g_strip_fn[i] = nullptr;
        g_strip_ctrl[i] = (void *)c;
        if (lazyIdx >= 0) {  // encoded on demand (materialize_wire)
            g_strip_fn[i] = g_lz[lazyIdx].fn;
            s.colorOrder = noteIdx >= 0 ? g_pn[noteIdx].order : 0012;
            s.rgbw = noteIdx >= 0 ? g_pn[noteIdx].rgbw : 0;
            s.isSpi = 0;
            s.wireOffset = 0;
            s.wireLen = 0;
            g_wire_dirty = true;
        } else if (noteIdx >= 0 && (u32)noteIdx < g_pw_count && g_pn_count == g_pw_count) {
            const PendingWire &w = g_pw[noteIdx];
            s.colorOrder = g_pn[noteIdx].order;
            s.rgbw = g_pn[noteIdx].rgbw;
            s.pin = w.pin;
            s.isSpi = w.isSpi;
            s.wireOffset = wireOff;
            s.wireLen = w.len;
            memcpy(g_l2 + wireOff, g_pw_buf + w.off, w.len);
            wireOff += w.len;
            ++wireIdx;
        } else {
            // Not a stub clockless controller (or disabled): no wire bytes.
            s.colorOrder = 0012;
            s.rgbw = 0;
            s.pin = -1;
            s.isSpi = 0;
            s.wireOffset = wireOff;
            s.wireLen = 0;
        }
    }
    // Channels without a matching controller (e.g. Channel API): append raw.
    if (g_pn_count != g_pw_count) {
        wireOff = 0;
        for (u32 k = 0; k < g_pw_count; ++k) {
            memcpy(g_l2 + wireOff, g_pw_buf + g_pw[k].off, g_pw[k].len);
            wireOff += g_pw[k].len;
        }
    }
    g_strip_count = i;
    g_l1_leds = nLeds;
    g_l2_len = wireOff;
    g_l2rgb_valid = false;
    g_pw_count = 0; g_pw_len = 0; g_pn_count = 0; g_lz_count = 0;
    ++g_total_shows;
    ++g_frame_shows;
}

// Runs the pending lazy encodes of the last show (once per presented frame at most).
void materialize_wire() {
    if (!g_wire_dirty) return;
    g_wire_dirty = false;
    g_mat_off = 0;
    for (u32 i = 0; i < g_strip_count; ++i) {
        Strip &s = g_strips[i];
        s.wireOffset = g_mat_off;
        if (!g_strip_fn[i]) { s.wireLen = 0; continue; }
        g_mat_strip = (int)i;
        g_strip_fn[i](g_strip_ctrl[i]);  // -> pxl_rt_lazy_wire()
        g_mat_strip = -1;
    }
    g_l2_len = g_mat_off;
    g_l2rgb_valid = false;
}

struct ShowListener : public fl::EngineEvents::Listener {
    void onEndShowLeds() FL_NO_EXCEPT override {
        finalize_show();
        check_budget();
    }
};
ShowListener *g_listener = nullptr;

// Wire bytes -> RGB order (undo RGB_ORDER). For RGB strips only; RGBW/RGBWW strips
// are passed through unchanged (their per-pixel layout is chipset specific).
void build_wire_rgb() {
    materialize_wire();
    if (g_l2rgb_valid) return;
    ensure(&g_l2rgb, &g_l2rgb_cap, g_l1_leds * 3 + 3);
    memset(g_l2rgb, 0, g_l1_leds * 3);
    for (u32 i = 0; i < g_strip_count; ++i) {
        const Strip &s = g_strips[i];
        u8 *dst = g_l2rgb + s.ledOffset * 3;
        const u8 *src = g_l2 + s.wireOffset;
        if (s.wireLen == 0) continue;
        if (s.rgbw == 0 && !s.isSpi && s.wireLen >= s.ledCount * 3) {
            const int o = s.colorOrder;
            const int p0 = (o >> 6) & 3, p1 = (o >> 3) & 3, p2 = o & 3;  // wire byte k = channel pk
            for (u32 j = 0; j < s.ledCount; ++j) {
                const u8 *w = src + j * 3;
                u8 *d = dst + j * 3;
                d[p0] = w[0]; d[p1] = w[1]; d[p2] = w[2];
            }
        } else {
            u32 n = s.wireLen < s.ledCount * 3 ? s.wireLen : s.ledCount * 3;
            memcpy(dst, src, n);
        }
    }
    g_l2rgb_len = g_l1_leds * 3;
    g_l2rgb_valid = true;
}

void ui_output(const char *json) {
    const size_t n = strlen(json);
    g_ui_json = (char *)realloc(g_ui_json, n + 1);
    memcpy(g_ui_json, json, n + 1);
    ++g_ui_version;
}

}  // namespace

// ====================================================================== hooks
// Called from the FastLED overrides in toolchain/override/.
extern "C" {

u32 pxl_rt_millis(void) { poll_clock(); return (u32)(g_now_us / 1000u); }
u32 pxl_rt_micros(void) { poll_clock(); return (u32)g_now_us; }
void pxl_rt_delay_ms(u32 ms) { advance((u64)ms * 1000u); }
void pxl_rt_delay_us(u32 us) { advance((u64)us); }

void pxl_rt_wire_add(int pin, const u8 *data, u32 len, int is_spi) {
    if (g_pw_count >= PXL_MAX_STRIPS) return;
    ensure(&g_pw_buf, &g_pw_cap, g_pw_len + len + 4);
    memcpy(g_pw_buf + g_pw_len, data, len);
    g_pw[g_pw_count].pin = pin;
    g_pw[g_pw_count].isSpi = is_spi;
    g_pw[g_pw_count].off = g_pw_len;
    g_pw[g_pw_count].len = len;
    ++g_pw_count;
    g_pw_len += len;
}

int pxl_rt_lazy_encode(void) { return g_lazy; }
void pxl_rt_lazy_pending(void *ctrl, EncodeFn fn) {
    for (u32 k = 0; k < g_lz_count; ++k) if (g_lz[k].ctrl == ctrl) { g_lz[k].fn = fn; return; }
    if (g_lz_count >= PXL_MAX_STRIPS) return;
    g_lz[g_lz_count].ctrl = ctrl;
    g_lz[g_lz_count].fn = fn;
    ++g_lz_count;
}
void pxl_rt_lazy_wire(void *ctrl, int pin, const u8 *data, unsigned len) {
    (void)ctrl;
    if (g_mat_strip < 0) return;
    Strip &s = g_strips[g_mat_strip];
    ensure(&g_l2, &g_l2_cap, g_mat_off + len + 4);
    memcpy(g_l2 + g_mat_off, data, len);
    s.pin = pin;
    s.wireOffset = g_mat_off;
    s.wireLen = len;
    g_mat_off += len;
}

void pxl_rt_note_strip(void *ctrl, int rgb_order, int rgbw) {
    if (g_pn_count >= PXL_MAX_STRIPS) return;
    g_pn[g_pn_count].ctrl = ctrl;
    g_pn[g_pn_count].order = rgb_order;
    g_pn[g_pn_count].rgbw = rgbw;
    ++g_pn_count;
}

// Arduino random()/rand(): FastLED's stub implements random(a,b) as a + rand() % (b-a).
// libc rand() differs between platforms (msvcrt vs musl), so rand/srand are renamed
// (-Drand=pxl_rand -Dsrand=pxl_srand) to this deterministic avr-libc random()
// (Park-Miller "minimal standard", default seed 1) = AVR Arduino semantics.
int pxl_rand(void) {
    int32_t x = (int32_t)g_rand_next;
    if (x == 0) x = 123459876;
    const int32_t hi = x / 127773, lo = x % 127773;
    x = 16807 * lo - 2836 * hi;
    if (x < 0) x += 0x7fffffff;
    g_rand_next = (u32)x;
    return (int)((u32)x % 0x80000000u);
}
void pxl_srand(unsigned s) { g_rand_next = s; }

}  // extern "C"

// ====================================================================== ABI
PXL_EXPORT(pxl_abi_version) int pxl_abi_version(void) { return PXL_ABI_VERSION; }

PXL_EXPORT(pxl_init) int pxl_init(u32 seed) {
    if (g_phase != PHASE_BOOT) return -1;  // one sketch run per instance
    g_rand_next = seed ? seed : 1u;
    // Makes fl::delay() call the platform delay directly (skips async task pumping).
    setDelayFunction([](u32 ms) { advance((u64)ms * 1000u); });
    g_listener = new ShowListener();
    fl::EngineEvents::addListener(g_listener, -1000);
    g_ui_input = fl::setJsonUiHandlers([](const char *json) { ui_output(json); });
    g_phase = PHASE_SETUP;
    setup();
    fl::processJsonUiPendingUpdates();
    g_phase = PHASE_LOOP;
    g_target_us = g_now_us;
    g_polls = 0;
    return 0;
}

// Returns 1 if the sketch fiber must be resumed, 0 if the frame is skipped.
PXL_EXPORT(pxl_frame_begin) int pxl_frame_begin(u32 dt_us) {
    if (g_phase != PHASE_LOOP) { g_frame_status = PXL_STATUS_NOT_INIT; return 0; }
    if (dt_us == 0 && g_first_frame_done) { g_frame_status = PXL_STATUS_SKIPPED; return 0; }
    g_target_us += dt_us;
    if (!g_first_frame_done) g_first_frame_start = g_now_us;
    g_frame_status = PXL_STATUS_OK;
    g_frame_loops = 0;
    g_frame_shows = 0;
    return 1;
}

// Sketch fiber body. Never returns (it is suspended/unwound at frame boundaries).
PXL_EXPORT(pxl_fiber_entry) void pxl_fiber_entry(void) {
    g_in_fiber = true;
    g_fiber_started = true;
    for (;;) {
        const u64 t0 = g_now_us;
        ++g_frame_loops;
        ++g_total_loops;
        loop();
        if (g_now_us == t0 && first_frame_follow()) {
            g_now_us += 1000;  // idle iteration before the first show: let 1 ms pass
            if (g_now_us > g_target_us) g_target_us = g_now_us;
            g_polls = 0;
            check_budget();
        } else if (g_now_us == t0) {
            suspend(PXL_SUSPEND_IDLE);
            catch_up();
        } else {
            check_budget();
        }
    }
}

PXL_EXPORT(pxl_frame_end) u32 pxl_frame_end(void) {
    if (g_frame_status != PXL_STATUS_SKIPPED && g_frame_status != PXL_STATUS_NOT_INIT) {
        g_first_frame_done = true;
        ++g_total_frames;
    }
    return g_frame_status;
}

PXL_EXPORT(pxl_set_budget) void pxl_set_budget(u32 loops, u32 shows) {
    g_budget_loops = loops ? loops : 100000;
    g_budget_shows = shows ? shows : 20000;
}

// --- frame data
PXL_EXPORT(pxl_led_count) u32 pxl_led_count(void) {
    if (g_total_shows) return g_l1_leds;
    u32 n = 0;  // before the first show: what addLeds() registered so far
    for (CLEDController *c = CLEDController::head(); c; c = c->next()) n += (u32)c->size();
    return n;
}
PXL_EXPORT(pxl_leds_ptr) u8 *pxl_leds_ptr(void) {
    if (!g_total_shows) {  // nothing shown yet: black frame of the registered size
        const u32 n = pxl_led_count();
        ensure(&g_l1, &g_l1_cap, n * 3 + 3);
        memset(g_l1, 0, n * 3);
    }
    return g_l1;
}
PXL_EXPORT(pxl_wire_raw_ptr) u8 *pxl_wire_raw_ptr(void) { materialize_wire(); ensure(&g_l2, &g_l2_cap, 4); return g_l2; }
PXL_EXPORT(pxl_wire_raw_len) u32 pxl_wire_raw_len(void) { materialize_wire(); return g_l2_len; }
// 1 (default): encode wire bytes only when requested (once per frame); 0: FastLED's eager path.
PXL_EXPORT(pxl_set_lazy) void pxl_set_lazy(int on) { g_lazy = on ? 1 : 0; }
PXL_EXPORT(pxl_wire_rgb_ptr) u8 *pxl_wire_rgb_ptr(void) {
    if (!g_total_shows) return pxl_leds_ptr();  // black frame
    build_wire_rgb();
    return g_l2rgb;
}
PXL_EXPORT(pxl_strip_count) u32 pxl_strip_count(void) { return g_strip_count; }
PXL_EXPORT(pxl_strips_ptr) Strip *pxl_strips_ptr(void) { materialize_wire(); return g_strips; }

// --- status / clock
PXL_EXPORT(pxl_now_ms) double pxl_now_ms(void) { return (double)g_now_us / 1000.0; }
PXL_EXPORT(pxl_target_ms) double pxl_target_ms(void) { return (double)g_target_us / 1000.0; }
PXL_EXPORT(pxl_now_us_lo) u32 pxl_now_us_lo(void) { return (u32)g_now_us; }
PXL_EXPORT(pxl_now_us_hi) u32 pxl_now_us_hi(void) { return (u32)(g_now_us >> 32); }
PXL_EXPORT(pxl_show_count) u32 pxl_show_count(void) { return g_total_shows; }
PXL_EXPORT(pxl_frame_show_count) u32 pxl_frame_show_count(void) { return g_frame_shows; }
PXL_EXPORT(pxl_loop_count) u32 pxl_loop_count(void) { return g_total_loops; }
PXL_EXPORT(pxl_frame_loop_count) u32 pxl_frame_loop_count(void) { return g_frame_loops; }
PXL_EXPORT(pxl_suspend_reason) u32 pxl_suspend_reason(void) { return g_suspend_reason; }
PXL_EXPORT(pxl_brightness) u32 pxl_brightness(void) { return FastLED.getBrightness(); }

// --- UI (FastLED JsonUi: UISlider, UICheckbox, UINumberField, UIButton, UIDropdown, ...)
PXL_EXPORT(pxl_ui_json) const char *pxl_ui_json(void) {
    fl::processJsonUiPendingUpdates();
    return g_ui_json ? g_ui_json : "[]";
}
PXL_EXPORT(pxl_ui_version) u32 pxl_ui_version(void) { return g_ui_version; }
// json: {"<id or name>": value, ...}; applied immediately (visible to the next loop code).
PXL_EXPORT(pxl_ui_set) int pxl_ui_set(const char *json) {
    if (!g_ui_input || !json) return -1;
    g_ui_input(json);
    fl::processJsonUiPendingUpdates();
    return 0;
}

// --- memory helpers for the host
PXL_EXPORT(pxl_alloc) void *pxl_alloc(u32 n) { return malloc(n ? n : 1); }
PXL_EXPORT(pxl_free) void pxl_free(void *p) { free(p); }

#if PXL_WASM
// Asyncify scratch area: {i32 cur, i32 end} followed by the save stack.
#ifndef PXL_ASYNCIFY_STACK
#define PXL_ASYNCIFY_STACK (1u << 20)
#endif
static int32_t g_async_hdr[2];
static u8 g_async_stack[PXL_ASYNCIFY_STACK];
PXL_EXPORT(pxl_asyncify_data) int32_t *pxl_asyncify_data(void) {
    g_async_hdr[0] = (int32_t)(intptr_t)g_async_stack;
    g_async_hdr[1] = (int32_t)(intptr_t)(g_async_stack + PXL_ASYNCIFY_STACK);
    return g_async_hdr;
}
#endif

// Arduino random() (no arguments) == avr-libc random(); `random` is renamed to pxl_random.
extern "C" long pxl_random(void) { return (long)pxl_rand(); }

#if PXL_WASM
// fl::platforms I/O for wasm32-wasi (FastLED has POSIX/Win/WASM(emscripten) variants
// only). Output goes to WASI fd 1 -> the JS host's console callback. No input.
#include "platforms/io.h"
#include <stdio.h>
namespace fl {
namespace platforms {
void begin(u32) FL_NO_EXCEPT {}
void print(const char *str) FL_NO_EXCEPT { if (str) { fputs(str, stdout); fflush(stdout); } }
void println(const char *str) FL_NO_EXCEPT { if (str) fputs(str, stdout); fputc('\n', stdout); fflush(stdout); }
int available() FL_NO_EXCEPT { return 0; }
int peek() FL_NO_EXCEPT { return -1; }
int read() FL_NO_EXCEPT { return -1; }
int readLineNative(char, char *out, int outLen) FL_NO_EXCEPT { if (out && outLen > 0) out[0] = 0; return 0; }
bool flush(u32) FL_NO_EXCEPT { fflush(stdout); return true; }
size_t write_bytes(const u8 *buffer, size_t size) FL_NO_EXCEPT { size_t n = fwrite(buffer, 1, size, stdout); fflush(stdout); return n; }
bool serial_ready() FL_NO_EXCEPT { return true; }
bool serial_is_buffered() FL_NO_EXCEPT { return false; }
}  // namespace platforms
}  // namespace fl
#endif
