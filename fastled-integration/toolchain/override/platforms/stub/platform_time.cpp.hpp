#pragma once
// PXLBLZ override of fastled/src/platforms/stub/platform_time.cpp.hpp
// (shadowed via -I toolchain/override placed before -I fastled/src).
// The stub platform's raw clock is replaced by the deterministic virtual
// clock of the PXLBLZ runtime (runtime/pxl_runtime.cpp): millis()/micros()
// read it, delay()/delayMicroseconds() advance it (and may suspend the
// sketch fiber until the IDE frame clock catches up).
#include "fl/stl/stdint.h"
#include "fl/stl/noexcept.h"
#include "platforms/time_platform.h"

extern "C" {
fl::u32 pxl_rt_millis(void);
fl::u32 pxl_rt_micros(void);
void pxl_rt_delay_ms(fl::u32 ms);
void pxl_rt_delay_us(fl::u32 us);
}

namespace fl {
namespace platforms {
void delay(fl::u32 ms) FL_NO_EXCEPT { pxl_rt_delay_ms(ms); }
void delayMicroseconds(fl::u32 us) FL_NO_EXCEPT { pxl_rt_delay_us(us); }
fl::u32 millis() FL_NO_EXCEPT { return pxl_rt_millis(); }
fl::u32 micros() FL_NO_EXCEPT { return pxl_rt_micros(); }
}  // namespace platforms
}  // namespace fl
