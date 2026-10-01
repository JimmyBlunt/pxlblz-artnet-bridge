package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"pxlblz-fadecandy/internal/output"
	"pxlblz-fadecandy/internal/probe"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7890", "fcserver host:port")
	channel := flag.Int("channel", 0, "OPC channel 0..255")
	mode := flag.String("mode", "pixel", "pixel|strand|all|black|chase")
	pixel := flag.Int("pixel", 0, "physical pixel 0..511")
	strand := flag.Int("strand", 0, "Fadecandy output strand 0..7 (64 pixels each)")
	colorText := flag.String("color", "white", "named color or #RRGGBB")
	fps := flag.Int("fps", 10, "send rate for chase")
	duration := flag.Duration("duration", 5*time.Second, "chase duration")
	hexDump := flag.Bool("hex", false, "print full first OPC packet as hex")
	flag.Parse()

	if *channel < 0 || *channel > 255 {
		fatal("channel must be 0..255")
	}
	if *fps < 1 || *fps > 240 {
		fatal("fps must be 1..240")
	}

	c, err := probe.ParseColor(*colorText)
	fatalErr(err)

	client := output.New(*addr, byte(*channel))
	defer client.Close()

	frame := probe.BlankFrame()
	start := time.Now()
	ticker := time.NewTicker(time.Second / time.Duration(*fps))
	defer ticker.Stop()

	first := true
	for n := 0; ; n++ {
		for i := range frame {
			frame[i] = 0
		}

		switch strings.ToLower(*mode) {
		case "pixel":
			fatalErr(probe.SetPixel(frame, *pixel, c))
		case "strand":
			fatalErr(probe.FillStrand(frame, *strand, c))
		case "all":
			fatalErr(probe.FillAll(frame, c))
		case "black":
			fatalErr(probe.FillAll(frame, probe.Color{}))
		case "chase":
			fatalErr(probe.SetPixel(frame, n%probe.PixelCount, c))
		default:
			fatal("mode must be pixel|strand|all|black|chase")
		}

		packet, err := probe.BuildPacket(byte(*channel), frame)
		fatalErr(err)
		if first {
			fmt.Println(probe.PacketSummary(packet))
			if *hexDump {
				fmt.Println(strings.ToUpper(hex.EncodeToString(packet)))
			}
			first = false
		}

		fatalErr(client.SendRGB(frame))

		if strings.ToLower(*mode) != "chase" {
			break
		}
		if time.Since(start) >= *duration {
			break
		}
		<-ticker.C
	}

	fmt.Printf(
		"SENT to %s mode=%s pixel=%d strand=%d color=%s\n",
		*addr, *mode, *pixel, *strand, *colorText,
	)
}

func fatal(s string) {
	fmt.Fprintln(os.Stderr, "ERROR:", s)
	os.Exit(1)
}

func fatalErr(err error) {
	if err != nil {
		fatal(err.Error())
	}
}
