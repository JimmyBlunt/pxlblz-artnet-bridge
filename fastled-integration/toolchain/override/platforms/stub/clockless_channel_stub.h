#pragma once

// IWYU pragma: private

// PXLBLZ override of fastled/src/platforms/stub/clockless_channel_stub.h
//
// Same controller hierarchy as upstream (ClocklessController -> SlimBridgeController ->
// CPixelLEDController), with three changes for the PXLBLZ runtime:
//  1. The controller reports its compile-time RGB_ORDER / RGBW mode
//     (pxl_rt_note_strip), so the wire bytes (L2) can be returned in RGB order.
//  2. Lazy encoding (default, runtime switch pxl_set_lazy): showPixels() stores a copy of
//     the PixelController (incl. its brightness/correction/dither state) and of the pixel
//     data instead of encoding immediately. The encode runs only when the host asks for the
//     wire bytes, i.e. once per IDE frame instead of once per show() (FastLED.delay() calls
//     show() every millisecond). The encode itself is FastLED's own
//     PixelIterator::writeWS2812() on an identical PixelController copy, so the bytes are
//     the same as the eager path (verified: tests/verify.mjs compares native eager vs wasm lazy).
//  3. The upstream ActiveStripTracker capture (a second, un-adjusted encode used only by
//     FastLED's own web visualizer) is dropped.
extern "C" void pxl_rt_note_strip(void *ctrl, int rgb_order, int rgbw);
extern "C" int pxl_rt_lazy_encode(void);
extern "C" void pxl_rt_lazy_pending(void *ctrl, void (*encode)(void *self));
extern "C" void pxl_rt_lazy_wire(void *ctrl, int pin, const unsigned char *data, unsigned len);

#define FL_CLOCKLESS_CONTROLLER_DEFINED 1
#define FL_CLOCKLESS_STUB_CHANNEL_ENGINE_DEFINED 1
#define FASTLED_CLOCKLESS_STUB_DEFINED 1

#include "eorder.h"
#include "fl/stl/compiler_control.h"
#include "fl/chipsets/timing_traits.h"
#include "fl/channels/bus.h"
#include "fl/channels/data.h"
#include "fl/channels/slim_bridge_controller.h"
#include "pixel_iterator.h"
#include "fl/log/log.h"
#include "fl/stl/vector.h"
#include "platforms/stub/bus_traits.h"
#include "platforms/stub/stub_gpio.h"
#include "fl/stl/noexcept.h"

namespace fl {

template <int DATA_PIN, typename TIMING, EOrder RGB_ORDER = RGB, int XTRA0 = 0, bool FLIP = false, int WAIT_TIME = 0>
class ClocklessController : public SlimBridgeController<DATA_PIN, TIMING, RGB_ORDER, WAIT_TIME, BusTraits<Bus::BIT_BANG>> {
    using Base = SlimBridgeController<DATA_PIN, TIMING, RGB_ORDER, WAIT_TIME, BusTraits<Bus::BIT_BANG>>;

    PixelController<RGB_ORDER> *mLazy = nullptr;
    fl::vector<u8> mLazyData;
    fl::vector_psram<u8> mLazyOut;
    Rgbw mLazyRgbw;
    Rgbww mLazyRgbww;
    bool mLazyPending = false;

    int rgbwMode() const FL_NO_EXCEPT {
        return this->getRgbww().active() ? 2 : (this->getRgbw().active() ? 1 : 0);
    }

    static void encodeThunk(void *self) FL_NO_EXCEPT { static_cast<ClocklessController *>(self)->encodeLazy(); }

    void encodeLazy() FL_NO_EXCEPT {
        if (!mLazyPending || !mLazy) return;
        mLazyPending = false;
        mLazyOut.clear();
        fl::PixelIterator iterator(mLazy, mLazyRgbw, mLazyRgbww);
        iterator.writeWS2812(&mLazyOut);
        pxl_rt_lazy_wire(static_cast<CLEDController *>(this), DATA_PIN, mLazyOut.data(), (unsigned)mLazyOut.size());
    }

public:
    ~ClocklessController() { delete mLazy; }

protected:
    void showPixels(PixelController<RGB_ORDER> &pixels) FL_NO_EXCEPT override {
        if (!pxl_rt_lazy_encode()) {  // eager: upstream SlimBridgeController path (encode + driver)
            Base::showPixels(pixels);
            return;
        }
        pxl_rt_note_strip(static_cast<CLEDController *>(this), (int)RGB_ORDER, rgbwMode());
        // Snapshot the pixel bytes the controller will read (any advance/direction).
        const int len = pixels.mLen;
        const int adv = pixels.mAdvance;
        const u8 *first = pixels.mData;
        const u8 *last = len > 0 ? pixels.mData + (len - 1) * adv : pixels.mData;
        const u8 *lo = first < last ? first : last;
        const u8 *hi = (first < last ? last : first) + 3;
        mLazyData.resize((fl::size)(hi - lo));
        if (hi > lo) fl::memcpy(mLazyData.data(), lo, (fl::size)(hi - lo));
        if (!mLazy) mLazy = new PixelController<RGB_ORDER>(pixels);
        else *mLazy = pixels;
        mLazy->mData = mLazyData.data() + (pixels.mData - lo);
        mLazyRgbw = this->getRgbw();
        mLazyRgbww = this->getRgbww();
        mLazyPending = true;
        pxl_rt_lazy_pending(static_cast<CLEDController *>(this), &ClocklessController::encodeThunk);
    }

    void onBeforeEncode(PixelController<RGB_ORDER> &pixels) FL_NO_EXCEPT override {
        (void)pixels;
        pxl_rt_note_strip(static_cast<CLEDController *>(this), (int)RGB_ORDER, rgbwMode());
    }
};

// Adapter for timing-like objects via duck typing
template <int DATA_PIN, typename TIMING_LIKE, EOrder RGB_ORDER = RGB, int XTRA0 = 0, bool FLIP = false, int WAIT_TIME = 0>
struct ClocklessControllerAdapter : public ClocklessController<DATA_PIN, TIMING_LIKE, RGB_ORDER, XTRA0, FLIP, WAIT_TIME> {
};

// ClocklessBlockController for type-based timing
template <int DATA_PIN, typename TIMING, EOrder RGB_ORDER = RGB, int XTRA0 = 0, bool FLIP = false, int WAIT_TIME = 0>
class ClocklessBlockController : public ClocklessController<DATA_PIN, TIMING, RGB_ORDER, XTRA0, FLIP, WAIT_TIME> {
};

}  // namespace fl
