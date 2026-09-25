package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"time"

	"pxlblz-router/internal/artnet"
	cfgpkg "pxlblz-router/internal/config"
)

type Distribution struct {
	Count int     `json:"count"`
	P50Ms float64 `json:"p50_ms"`
	P95Ms float64 `json:"p95_ms"`
	P99Ms float64 `json:"p99_ms"`
	MaxMs float64 `json:"max_ms"`
	AvgMs float64 `json:"avg_ms"`
}

type Report struct {
	Config                 string            `json:"config"`
	Bind                   string            `json:"bind"`
	DurationSeconds        float64           `json:"duration_seconds"`
	ExpectedUniverses      int               `json:"expected_universes"`
	ExpectedPacketsPerFrame int              `json:"expected_packets_per_frame"`
	Packets                uint64            `json:"packets"`
	Bytes                  uint64            `json:"bytes"`
	CompleteFrames         uint64            `json:"complete_frames"`
	IncompleteFrames       uint64            `json:"incomplete_frames"`
	InvalidPackets         uint64            `json:"invalid_packets"`
	UnexpectedUniverses    uint64            `json:"unexpected_universes"`
	PayloadMismatches      uint64            `json:"payload_mismatches"`
	DuplicateUniverses     uint64            `json:"duplicate_universes"`
	LateSameSequence       uint64            `json:"late_same_sequence_packets"`
	SequenceGapEvents      uint64            `json:"sequence_gap_events"`
	FramesPerSecond        float64           `json:"frames_per_second"`
	PacketsPerSecond       float64           `json:"packets_per_second"`
	MegabitsPerSecond      float64           `json:"megabits_per_second"`
	Assembly               Distribution      `json:"assembly"`
	FrameInterval          Distribution      `json:"frame_interval"`
	PerUniverse            map[string]uint64 `json:"per_universe"`
}

type candidate struct {
	seq   byte
	first time.Time
	seen  map[uint16]bool
}

func main() {
	configPath := flag.String("config", "", "router config used to derive expected universes/payloads")
	bind := flag.String("bind", "127.0.0.1:6454", "UDP bind address")
	duration := flag.Duration("duration", 10*time.Second, "measurement duration")
	reportPath := flag.String("report", "", "optional JSON report path")
	quiet := flag.Bool("quiet", false, "suppress periodic text stats")
	flag.Parse()

	if *configPath == "" {
		fatal(fmt.Errorf("--config is required"))
	}
	cfg, err := cfgpkg.Load(*configPath)
	fatal(err)
	expected, err := expectedPayloads(cfg)
	fatal(err)

	addr, err := net.ResolveUDPAddr("udp4", *bind)
	fatal(err)
	conn, err := net.ListenUDP("udp4", addr)
	fatal(err)
	defer conn.Close()

	fmt.Printf("Art-Net performance probe on %s | expected universes=%d\n", *bind, len(expected))

	buf := make([]byte, 2048)
	perUniverse := make(map[uint16]uint64, len(expected))
	var packets, bytes uint64
	var complete, incomplete, invalid, unexpected, payloadMismatch uint64
	var duplicate, lateSameSeq, seqGap uint64
	var current *candidate
	var lastCompletedSeq byte
	var haveLastCompleted bool
	var lastCompleteAt time.Time
	assemblySamples := make([]time.Duration, 0, 4096)
	intervalSamples := make([]time.Duration, 0, 4096)

	var started time.Time
	var deadline time.Time
	syncDeadline := time.Now().Add(10 * time.Second)
	var lastPrint time.Time
	synced := false

	for deadline.IsZero() || time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if !synced && time.Now().After(syncDeadline) {
					fatal(fmt.Errorf("timed out waiting for first complete frame"))
				}
				continue
			}
			fatal(err)
		}
		now := time.Now()

		p, err := artnet.ParseDmx(buf[:n])
		if err != nil {
			if synced {
				packets++
				bytes += uint64(n)
				invalid++
			}
			continue
		}

		if synced {
			packets++
			bytes += uint64(n)
		}

		wantLen, ok := expected[p.Universe]
		if !ok {
			if synced {
				unexpected++
			}
			continue
		}
		if synced {
			perUniverse[p.Universe]++
		}
		if len(p.Data) != wantLen {
			if synced {
				payloadMismatch++
			}
			continue
		}

		if current == nil {
			if haveLastCompleted && p.Sequence == lastCompletedSeq {
				lateSameSeq++
				continue
			}
			if synced && haveLastCompleted && p.Sequence != nextSeq(lastCompletedSeq) {
				seqGap++
			}
			current = &candidate{seq: p.Sequence, first: now, seen: make(map[uint16]bool, len(expected))}
		} else if p.Sequence != current.seq {
			if synced && len(current.seen) != len(expected) {
				incomplete++
			}
			if synced && haveLastCompleted && p.Sequence != nextSeq(lastCompletedSeq) {
				seqGap++
			}
			current = &candidate{seq: p.Sequence, first: now, seen: make(map[uint16]bool, len(expected))}
		}

		if current.seen[p.Universe] {
			if synced {
				duplicate++
			}
			continue
		}
		current.seen[p.Universe] = true

		if len(current.seen) == len(expected) {
			if !synced {
				synced = true
				started = now
				deadline = started.Add(*duration)
				lastPrint = started
				packets = uint64(len(expected))
				for u, payloadLen := range expected {
					perUniverse[u] = 1
					bytes += uint64(artnet.HeaderSize + payloadLen)
				}
				complete = 1
				assemblySamples = append(assemblySamples, now.Sub(current.first))
			} else {
				complete++
				assemblySamples = append(assemblySamples, now.Sub(current.first))
				if !lastCompleteAt.IsZero() {
					intervalSamples = append(intervalSamples, now.Sub(lastCompleteAt))
				}
			}
			lastCompleteAt = now
			lastCompletedSeq = current.seq
			haveLastCompleted = true
			current = nil
		}

		if synced && !*quiet && now.Sub(lastPrint) >= time.Second {
			elapsed := now.Sub(started).Seconds()
			fmt.Printf("PROBE complete=%d fps=%.2f packets=%d pps=%.0f invalid=%d unexpected=%d payload=%d incomplete=%d gaps=%d dup=%d\n",
				complete, float64(complete)/elapsed, packets, float64(packets)/elapsed,
				invalid, unexpected, payloadMismatch, incomplete, seqGap, duplicate)
			lastPrint = now
		}
	}
	if synced && current != nil && len(current.seen) > 0 && len(current.seen) != len(expected) {
		incomplete++
	}
	if !synced {
		fatal(fmt.Errorf("no complete frame observed"))
	}

	elapsed := time.Since(started).Seconds()
	if elapsed <= 0 {
		elapsed = 0.001
	}
	report := Report{
		Config: *configPath,
		Bind: *bind,
		DurationSeconds: elapsed,
		ExpectedUniverses: len(expected),
		ExpectedPacketsPerFrame: len(expected),
		Packets: packets,
		Bytes: bytes,
		CompleteFrames: complete,
		IncompleteFrames: incomplete,
		InvalidPackets: invalid,
		UnexpectedUniverses: unexpected,
		PayloadMismatches: payloadMismatch,
		DuplicateUniverses: duplicate,
		LateSameSequence: lateSameSeq,
		SequenceGapEvents: seqGap,
		FramesPerSecond: float64(complete) / elapsed,
		PacketsPerSecond: float64(packets) / elapsed,
		MegabitsPerSecond: float64(bytes*8) / elapsed / 1_000_000,
		Assembly: distribution(assemblySamples),
		FrameInterval: distribution(intervalSamples),
		PerUniverse: make(map[string]uint64, len(perUniverse)),
	}
	keys := make([]int, 0, len(perUniverse))
	for u, count := range perUniverse {
		report.PerUniverse[strconv.Itoa(int(u))] = count
		keys = append(keys, int(u))
	}
	sort.Ints(keys)

	out, err := json.MarshalIndent(report, "", "  ")
	fatal(err)
	fmt.Println(string(out))
	if *reportPath != "" {
		fatal(os.WriteFile(*reportPath, append(out, '\n'), 0o644))
	}
}

func expectedPayloads(cfg cfgpkg.Config) (map[uint16]int, error) {
	out := map[uint16]int{}
	for _, route := range cfg.Routes {
		if !route.Enabled {
			continue
		}
		remaining := route.PixelCount * 3
		universe := route.UniverseStart
		for remaining > 0 {
			n := artnet.MaxChannelsPerUniverse
			if remaining < n {
				n = remaining
			}
			wire := n
			if wire&1 != 0 {
				wire++
			}
			u := uint16(universe)
			if old, exists := out[u]; exists && old != wire {
				return nil, fmt.Errorf("universe %d has conflicting expected lengths %d and %d", u, old, wire)
			}
			out[u] = wire
			remaining -= n
			universe++
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("config has no enabled route universes")
	}
	return out, nil
}

func nextSeq(seq byte) byte {
	seq++
	if seq == 0 {
		seq = 1
	}
	return seq
}

func distribution(samples []time.Duration) Distribution {
	if len(samples) == 0 {
		return Distribution{}
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var total time.Duration
	for _, d := range sorted {
		total += d
	}
	return Distribution{
		Count: len(sorted),
		P50Ms: ms(percentile(sorted, 0.50)),
		P95Ms: ms(percentile(sorted, 0.95)),
		P99Ms: ms(percentile(sorted, 0.99)),
		MaxMs: ms(sorted[len(sorted)-1]),
		AvgMs: ms(total / time.Duration(len(sorted))),
	}
}

func percentile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1)*q + 0.5)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted)-1
	}
	return sorted[idx]
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}
