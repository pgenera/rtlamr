// fakertltcp replays a recorded rtl_sdr .cu8 file to one rtl_tcp client,
// paced at -speed times the recorded sample rate, then closes.
package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:1236", "listen address")
	file := flag.String("file", "", "cu8 file")
	rate := flag.Float64("rate", 2359296, "recorded sample rate")
	speed := flag.Float64("speed", 1, "playback speed multiple (0 = unpaced)")
	flag.Parse()

	f, err := os.Open(*file)
	if err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	conn, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}
	ln.Close()

	// DongleInfo: "RTL0", tuner (5 = R820T), gain count.
	hdr := make([]byte, 12)
	copy(hdr, "RTL0")
	binary.BigEndian.PutUint32(hdr[4:], 5)
	binary.BigEndian.PutUint32(hdr[8:], 29)
	conn.Write(hdr)
	go io.Copy(io.Discard, conn) // ignore tuning commands

	r := bufio.NewReaderSize(f, 1<<20)
	buf := make([]byte, 1<<16)
	start := time.Now()
	fmt.Printf("start %d\n", start.UnixNano())
	var sent int64
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			if *speed > 0 {
				due := start.Add(time.Duration(float64(sent/2) / (*rate * *speed) * float64(time.Second)))
				if d := time.Until(due); d > 0 {
					time.Sleep(d)
				}
			}
			if _, werr := conn.Write(buf[:n]); werr != nil {
				break
			}
			sent += int64(n)
		}
		if err != nil {
			break
		}
	}
	el := time.Since(start)
	fmt.Printf("end %d samples=%d recorded=%.1fs wall=%.1fs speed=%.2fx\n", time.Now().UnixNano(),
		sent/2, float64(sent/2) / *rate, el.Seconds(), float64(sent/2) / *rate / el.Seconds())
	conn.Close()
}
