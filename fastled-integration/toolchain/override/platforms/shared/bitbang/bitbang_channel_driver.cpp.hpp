// PXLBLZ override of fastled/src/platforms/shared/bitbang/bitbang_channel_driver.cpp.hpp
// The stub platform routes every legacy addLeds<>() clockless controller through
// SlimBridgeController -> BitBangChannelDriver. The original driver bit-bangs the
// encoded bytes onto a simulated GPIO with nanosecond busy-waits. Here the driver is
// a pure sink: show() hands the already encoded wire bytes (FastLED's own
// PixelController pipeline: brightness, color correction/temperature, dithering,
// RGB_ORDER) to the PXLBLZ runtime and transmits nothing.
#include "platforms/shared/bitbang/bitbang_channel_driver.h"
#include "fl/stl/noexcept.h"

extern "C" void pxl_rt_wire_add(int pin, const fl::u8 *data, fl::u32 len, int is_spi);

namespace fl {

BitBangChannelDriver::BitBangChannelDriver() FL_NO_EXCEPT : mNumActiveSlots(0), mActiveSlotMask(0) {
    for (int i = 0; i < 256; ++i) mSlotForPin[i] = -1;
    for (int i = 0; i < 8; ++i) mPinForSlot[i] = -1;
}
BitBangChannelDriver::~BitBangChannelDriver() = default;
bool BitBangChannelDriver::canHandle(const ChannelDataPtr &) const FL_NO_EXCEPT { return true; }
void BitBangChannelDriver::enqueue(ChannelDataPtr channelData) FL_NO_EXCEPT {
    if (channelData) mEnqueuedChannels.push_back(fl::move(channelData));
}
IChannelDriver::DriverState BitBangChannelDriver::poll() FL_NO_EXCEPT {
    return DriverState(DriverState::READY);
}
fl::string BitBangChannelDriver::getName() const FL_NO_EXCEPT {
    return fl::string::from_literal("BIT_BANG");
}
IChannelDriver::Capabilities BitBangChannelDriver::getCapabilities() const FL_NO_EXCEPT {
    return Capabilities(true, true);
}
void BitBangChannelDriver::rebuildPinConfig(fl::span<const ChannelDataPtr>) FL_NO_EXCEPT {}
void BitBangChannelDriver::transmitClocklessBit(u8, u32, u32, u32, u32) FL_NO_EXCEPT {}
void BitBangChannelDriver::transmitClockless(fl::span<const ChannelDataPtr>) FL_NO_EXCEPT {}
void BitBangChannelDriver::transmitSpi(fl::span<const ChannelDataPtr>) FL_NO_EXCEPT {}
void BitBangChannelDriver::show() FL_NO_EXCEPT {
    for (fl::size i = 0; i < mEnqueuedChannels.size(); ++i) {
        const ChannelDataPtr &ch = mEnqueuedChannels[i];
        if (!ch) continue;
        const auto &d = ch->getData();
        pxl_rt_wire_add(ch->getPin(), d.data(), (fl::u32)d.size(), ch->isSpi() ? 1 : 0);
    }
    mEnqueuedChannels.clear();
}

}  // namespace fl
