// Native (Windows) fiber driver for the reference build: plays the role that
// binaryen Asyncify + fastledWasmHost.ts play in the wasm build.
#include <windows.h>
#include <stdint.h>

extern "C" int pxl_frame_begin(uint32_t dt_us);
extern "C" void pxl_fiber_entry(void);
extern "C" uint32_t pxl_frame_end(void);

static LPVOID g_main_fiber = nullptr;
static LPVOID g_sketch_fiber = nullptr;

static void CALLBACK sketch_proc(LPVOID) { pxl_fiber_entry(); }

extern "C" void pxl_host_suspend(void) { SwitchToFiber(g_main_fiber); }

// Same contract as fastledWasmHost.frame(): returns the frame status.
extern "C" uint32_t pxl_native_frame(uint32_t dt_us) {
    if (pxl_frame_begin(dt_us)) {
        if (!g_main_fiber) g_main_fiber = ConvertThreadToFiber(nullptr);
        if (!g_sketch_fiber) g_sketch_fiber = CreateFiber(32u << 20, sketch_proc, nullptr);
        SwitchToFiber(g_sketch_fiber);
    }
    return pxl_frame_end();
}
