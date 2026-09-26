package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"pxlblz-router/internal/artnet"
	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/virtualcontroller"
)

const version = "0.1.0"

func main() {
	configPath := flag.String("config", "config/routes.backpanel-all.json", "receiver profile route config")
	profileIP := flag.String("profile-ip", "10.0.0.253", "controller IP whose universe profile to use")
	target := flag.String("target", "127.0.0.1:6454", "virtual/real receiver UDP target")
	scenario := flag.String("scenario", "complete", "complete|short-tail|missing|packet-seq|extras|seq0|duplicate0")
	frames := flag.Int("frames", 1, "number of scenario frames")
	fps := flag.Int("fps", 30, "frame rate for multi-frame scenarios")
	faultUniverse := flag.Int("fault-universe", 0, "fault universe override (default scenario-specific)")
	faultBytes := flag.Int("fault-bytes", 78, "payload bytes for short-tail")
	flag.Parse()

	if *frames < 1 { fatal(fmt.Errorf("frames must be >= 1")) }
	if *fps < 1 || *fps > 240 { fatal(fmt.Errorf("fps must be 1..240")) }

	cfg, err := cfgpkg.Load(*configPath)
	fatal(err)
	model, err := virtualcontroller.New(cfg, *profileIP, virtualcontroller.Options{})
	fatal(err)
	universes := model.UniverseInfo()
	if len(universes) == 0 { fatal(fmt.Errorf("no expected universes")) }

	addr, err := net.ResolveUDPAddr("udp4", *target)
	fatal(err)
	conn, err := net.DialUDP("udp4", nil, addr)
	fatal(err)
	defer conn.Close()

	fmt.Printf("PXLBLZ Receiver Probe v%s -> %s\n", version, *target)
	fmt.Printf("Profile %s: %d expected universes | scenario=%s | frames=%d\n", *profileIP, len(universes), *scenario, *frames)

	period := time.Second / time.Duration(*fps)
	seq := byte(1)
	for frame := 0; frame < *frames; frame++ {
		start := time.Now()
		switch strings.ToLower(*scenario) {
		case "complete":
			sendFrame(conn, universes, seq, -1, 0, false)
		case "short-tail":
			u := *faultUniverse
			if u == 0 { u = 145 }
			sendFrame(conn, universes, seq, u, *faultBytes, false)
		case "missing":
			u := *faultUniverse
			if u == 0 { u = 149 }
			sendFrame(conn, universes, seq, u, 0, true)
		case "packet-seq":
			packetSeq := seq
			for _, info := range universes {
				sendUniverse(conn, info.Universe, packetSeq, info.DataBytes)
				packetSeq++
				if packetSeq == 0 { packetSeq = 1 }
			}
		case "extras":
			for _, u := range []uint16{138,150,151,152} {
				sendUniverse(conn, u, seq, 510)
			}
			sendFrame(conn, universes, seq, -1, 0, false)
		case "seq0":
			sendFrame(conn, universes, 0, -1, 0, false)
		case "duplicate0":
			first := universes[0]
			sendUniverse(conn, first.Universe, 0, first.DataBytes)
			sendUniverse(conn, first.Universe, 0, first.DataBytes)
			for _, info := range universes[1:] {
				sendUniverse(conn, info.Universe, 0, info.DataBytes)
			}
		default:
			fatal(fmt.Errorf("unknown scenario %q", *scenario))
		}

		seq++
		if seq == 0 { seq = 1 }
		if frame+1 < *frames {
			if remain := period - time.Since(start); remain > 0 {
				time.Sleep(remain)
			}
		}
	}
	fmt.Println("Probe packets sent.")
}

func sendFrame(conn *net.UDPConn, infos []virtualcontroller.UniverseInfo, seq byte, faultUniverse int, faultBytes int, omit bool) {
	for _, info := range infos {
		if int(info.Universe) == faultUniverse {
			if omit { continue }
			sendUniverse(conn, info.Universe, seq, faultBytes)
			continue
		}
		sendUniverse(conn, info.Universe, seq, info.DataBytes)
	}
}

func sendUniverse(conn *net.UDPConn, universe uint16, seq byte, dataBytes int) {
	if dataBytes < 1 { return }
	data := make([]byte, dataBytes)
	for i := range data {
		data[i] = byte((int(universe) + i) & 0xff)
	}
	buf := make([]byte, artnet.HeaderSize+artnet.MaxDmxPayload)
	pkt, err := artnet.BuildDmx(buf, universe, seq, data)
	fatal(err)
	_, err = conn.Write(pkt)
	fatal(err)
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
}
