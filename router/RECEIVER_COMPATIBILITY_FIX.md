# v0.1.1 receiver compatibility fix

The active Teensy receiver validates ArtDmx payload length before adding an
universe to the controller-wide frame-completion mask. ArtDmx payload length
must be even.

With OUT1 = 203 RGB LEDs:

- U120 carries 510 RGB bytes.
- U121 carries 99 required RGB bytes.
- v0.1.0 declared/sent 99 bytes, so the receiver rejected U121.
- v0.1.1 sends the same 99 RGB bytes plus one zero padding byte and declares
  an ArtDmx length of 100 bytes.

The padding does not add a logical pixel.

The router uses one common non-zero sequence value for every universe in a
logical frame and increments it only after all routes/universes are sent.
