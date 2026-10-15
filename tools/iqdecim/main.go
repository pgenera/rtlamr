// iqdecim shifts a .cu8 recording by -shift Hz and decimates it by -n with a
// windowed-sinc lowpass, writing .cu8 at rate/n. This stands in for tuning the
// dongle to (recorded center + shift) at the lower sample rate.
package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"math"
	"math/cmplx"
	"os"
)

func main() {
	in := flag.String("in", "", "input cu8")
	out := flag.String("out", "", "output cu8")
	rate := flag.Float64("rate", 2359296, "input sample rate")
	n := flag.Int("n", 9, "decimation factor")
	shift := flag.Float64("shift", 0, "new center minus recorded center, Hz")
	tapsPer := flag.Int("taps", 16, "filter taps per output phase")
	flag.Parse()

	// Lowpass at 0.45 of the output rate (passband just under output Nyquist).
	ntaps := *n * *tapsPer
	h := make([]float64, ntaps)
	fc := 0.45 / float64(*n)
	var sum float64
	for i := range h {
		x := float64(i) - float64(ntaps-1)/2
		s := 2 * fc
		if x != 0 {
			s = math.Sin(2*math.Pi*fc*x) / (math.Pi * x)
		}
		w := 0.42 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(ntaps-1)) + 0.08*math.Cos(4*math.Pi*float64(i)/float64(ntaps-1))
		h[i] = s * w
		sum += h[i]
	}
	for i := range h {
		h[i] /= sum
	}

	fi, err := os.Open(*in)
	if err != nil {
		log.Fatal(err)
	}
	fo, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	r := bufio.NewReaderSize(fi, 1<<20)
	w := bufio.NewWriterSize(fo, 1<<20)
	defer w.Flush()

	// Input samples go into a doubled ring so the filter window is always a
	// contiguous slice; the mixer is a renormalized phasor recurrence.
	hist := make([]complex64, 2*ntaps)
	hf := make([]float32, ntaps)
	for i := range h {
		hf[i] = float32(h[i])
	}
	pos := 0
	var k int64
	rot := cmplx.Rect(1, -2*math.Pi**shift/ *rate)
	ph := complex(1, 0)
	lut := make([]float32, 256)
	for i := range lut {
		lut[i] = (float32(i) - 127.5) / 127.5
	}
	inb := make([]byte, 1<<20)
	outb := make([]byte, 0, 1<<17)
	q := func(v float32) byte {
		v = v*127.5 + 127.5
		if v < 0 {
			v = 0
		} else if v > 255 {
			v = 255
		}
		return byte(v + 0.5)
	}
	for {
		nr, err := io.ReadFull(r, inb)
		nr &^= 1
		for b := 0; b < nr; b += 2 {
			s := complex(lut[inb[b]], lut[inb[b+1]])
			if *shift != 0 {
				s *= complex64(ph)
				ph *= rot
				if k&0xFFFF == 0 {
					ph /= complex(cmplx.Abs(ph), 0)
				}
			}
			hist[pos] = s
			hist[pos+ntaps] = s
			pos++
			if pos == ntaps {
				pos = 0
			}
			k++
			if k%int64(*n) != 0 || k < int64(ntaps) {
				continue
			}
			win := hist[pos : pos+ntaps]
			var re, im float32
			for i, c := range win {
				re += real(c) * hf[i]
				im += imag(c) * hf[i]
			}
			outb = append(outb, q(re), q(im))
		}
		w.Write(outb)
		outb = outb[:0]
		if err != nil {
			break
		}
	}
}
