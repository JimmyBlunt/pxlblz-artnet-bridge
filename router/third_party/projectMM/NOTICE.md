# projectMM provenance

This prototype's Art-Net packet layout and network-send design are derived from
MoonModules/projectMM, GPLv3:

- Repository: https://github.com/MoonModules/projectMM
- Source snapshot reviewed: `eece5e0eef6348520f7be28a279ea37bee1f6215`
- `src/light/util/ArtNetPacket.h`
- `src/light/drivers/NetworkSendDriver.h`

The Go implementation in `internal/artnet` mirrors the ArtDmx packet-builder semantics.
The routing/sending model follows projectMM concepts including unicast, one sequence
per complete logical frame, universe chunking, and keeping route preparation out of
the hot path.

projectMM is licensed under GNU GPL version 3.
