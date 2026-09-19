// Package artnet implements the small ArtDmx subset required by pxlblz-router.
//
// The packet layout and design intentionally mirror MoonModules/projectMM's
// src/light/util/ArtNetPacket.h (GPLv3). See third_party/projectMM/NOTICE.md.
package artnet

import (
	"encoding/binary"
	"errors"
)

const (
	Port                   = 6454
	HeaderSize             = 18
	MaxChannelsPerUniverse = 510
	MaxDmxPayload          = 512
	OpDmx                  = 0x5000
	ProtocolVersion        = 14
)

var magic = [8]byte{'A', 'r', 't', '-', 'N', 'e', 't', 0}

func BuildDmx(dst []byte, universe uint16, sequence byte, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("ArtDmx payload must not be empty")
	}
	if len(data) > MaxChannelsPerUniverse {
		return nil, errors.New("ArtDmx RGB payload exceeds 510 channels")
	}
	payloadLen := len(data)
	if payloadLen&1 != 0 {
		payloadLen++
	}
	if payloadLen < 2 || payloadLen > MaxDmxPayload {
		return nil, errors.New("ArtDmx payload length must be 2..512 bytes")
	}
	if len(dst) < HeaderSize+payloadLen {
		return nil, errors.New("destination packet buffer too small")
	}

	copy(dst[0:8], magic[:])
	binary.LittleEndian.PutUint16(dst[8:10], OpDmx)
	binary.BigEndian.PutUint16(dst[10:12], ProtocolVersion)
	dst[12] = sequence
	dst[13] = 0
	binary.LittleEndian.PutUint16(dst[14:16], universe)
	binary.BigEndian.PutUint16(dst[16:18], uint16(payloadLen))
	copy(dst[18:18+len(data)], data)
	if payloadLen != len(data) {
		dst[18+len(data)] = 0
	}
	return dst[:HeaderSize+payloadLen], nil
}

type ParsedDmx struct {
	Universe uint16
	Sequence byte
	Data     []byte
}

func ParseDmx(pkt []byte) (ParsedDmx, error) {
	if len(pkt) < HeaderSize {
		return ParsedDmx{}, errors.New("packet too short")
	}
	for i := range magic {
		if pkt[i] != magic[i] {
			return ParsedDmx{}, errors.New("invalid Art-Net magic")
		}
	}
	if binary.LittleEndian.Uint16(pkt[8:10]) != OpDmx {
		return ParsedDmx{}, errors.New("not ArtDmx")
	}
	n := int(binary.BigEndian.Uint16(pkt[16:18]))
	if n < 2 || n > MaxDmxPayload || n&1 != 0 || n != len(pkt)-HeaderSize {
		return ParsedDmx{}, errors.New("invalid ArtDmx payload length")
	}
	return ParsedDmx{
		Universe: binary.LittleEndian.Uint16(pkt[14:16]),
		Sequence: pkt[12],
		Data:     pkt[18 : 18+n],
	}, nil
}
