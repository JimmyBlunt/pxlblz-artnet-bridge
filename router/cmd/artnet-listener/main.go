package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"pxlblz-router/internal/artnet"
)

func main() {
	bind := flag.String("bind", "0.0.0.0:6454", "UDP bind address")
	verbose := flag.Bool("verbose", false, "print every ArtDmx packet")
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp4", *bind)
	fatal(err)
	conn, err := net.ListenUDP("udp4", addr)
	fatal(err)
	defer conn.Close()

	fmt.Println("Art-Net listener on", *bind)
	fmt.Println("Ctrl-C to stop")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	buf := make([]byte, 2048)
	universes := map[uint16]uint64{}
	var total, invalid uint64
	var lastSeq byte
	last := time.Now()

	go func() {
		<-sig
		conn.Close()
	}()

	for {
		conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if time.Since(last) >= time.Second {
					printStats(total, invalid, universes, lastSeq)
					total, invalid = 0, 0
					universes = map[uint16]uint64{}
					last = time.Now()
				}
				continue
			}
			return
		}
		p, err := artnet.ParseDmx(buf[:n])
		if err != nil {
			invalid++
			continue
		}
		total++
		universes[p.Universe]++
		lastSeq = p.Sequence
		if *verbose {
			fmt.Printf("%s U%d seq=%d channels=%d first=% x\n", from.IP, p.Universe, p.Sequence, len(p.Data), p.Data[:min(12, len(p.Data))])
		}
		if time.Since(last) >= time.Second {
			printStats(total, invalid, universes, lastSeq)
			total, invalid = 0, 0
			universes = map[uint16]uint64{}
			last = time.Now()
		}
	}
}

func printStats(total, invalid uint64, universes map[uint16]uint64, seq byte) {
	keys := make([]int, 0, len(universes))
	for u := range universes {
		keys = append(keys, int(u))
	}
	sort.Ints(keys)
	fmt.Printf("RX %d pkt/s | invalid %d | last seq %d | universes", total, invalid, seq)
	for _, k := range keys {
		fmt.Printf(" U%d:%d", k, universes[uint16(k)])
	}
	fmt.Println()
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}
