// Force-included (-include) before every FastLED TU in the wasm32-wasi build.
// Works around upstream issues of FastLED's single-threaded stub path
// without patching FastLED itself:
//  1. zig's wasm32-wasi libc++ has no std::thread, but wasi-libc ships <pthread.h>,
//     so FastLED would pick the std::thread path -> force the no-op thread path
//     (together with -DFASTLED_MULTITHREADED=0 -DFL_STUB_HAS_MULTITHREADED=0).
//  2. semaphore_stub_noop.h forward-declares std::chrono outside libc++'s inline
//     namespace -> include <chrono> first so the declaration reopens the real one.
//  3. thread_stub_noop.h's sleep_for template cannot deduce 'Period' -> add a
//     deducible no-op overload.
//  4. host_timer.cpp.hpp (SPI bit-bang host simulation, unused) calls
//     std::this_thread directly -> no-op shim (there are no threads in wasm).
// Networking (sockets) is disabled: there is no network in the sandbox.
#pragma once
#ifdef __cplusplus
#include <chrono>
#include "platforms/stub/thread_stub_noop.h"
#include "platforms/stub/is_stub.h"
#undef FASTLED_HAS_NETWORKING
namespace fl { namespace platforms { namespace this_thread {
template <typename D> inline void sleep_for(const D &) {}
}}}
#if defined(__wasi__)  // native libc++ has real threads
namespace std { namespace this_thread {
template <typename D> inline void sleep_for(const D &) {}
inline void yield() {}
}}
#else
#include <thread>
#endif
#endif
