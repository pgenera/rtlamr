package protocol

import (
	"math/rand"
	"reflect"
	"testing"
)

func preambleBits(s string) []byte {
	b := make([]byte, len(s))
	for idx, c := range s {
		if c == '1' {
			b[idx] = 1
		}
	}
	return b
}

// naiveSearch checks every index of the first block for the preamble.
func naiveSearch(d *Decoder, preamble []byte) (indices []int) {
	for idx := 0; idx < d.Cfg.BlockSize; idx++ {
		found := true
		for pIdx, pBit := range preamble {
			if d.Quantized[idx+pIdx*d.Cfg.SymbolLength] != pBit {
				found = false
				break
			}
		}
		if found {
			indices = append(indices, idx)
		}
	}
	return
}

// Decode packs the quantized signal once and then searches it for each
// registered preamble, so Search must find every preamble from the same
// packed signal.
func TestSearchMatchesNaive(t *testing.T) {
	preambles := [][]byte{
		preambleBits("111110010101001100000"),            // scm
		preambleBits("00000000000000001110010101100100"), // r900
		preambleBits("0001011010100011"),                 // scm+
	}

	for _, chipLength := range []int{4, 36} {
		d := NewDecoder()
		d.Cfg = PacketConfig{
			DataRate:        32768,
			ChipLength:      chipLength,
			PreambleSymbols: 32,
			PacketSymbols:   116,
		}
		d.Allocate()

		rng := rand.New(rand.NewSource(int64(chipLength)))
		for trial := 0; trial < 200; trial++ {
			for idx := range d.Quantized {
				d.Quantized[idx] = byte(rng.Intn(2))
			}

			// Plant whole and partial preambles, including at the edges of the
			// searched block.
			for n := 0; n < 8; n++ {
				preamble := preambles[rng.Intn(len(preambles))]
				var idx int
				switch n {
				case 0:
					idx = 0
				case 1:
					idx = d.Cfg.BlockSize - 1
				default:
					idx = rng.Intn(d.Cfg.BlockSize)
				}
				length := len(preamble)
				if n%2 == 1 {
					length = rng.Intn(len(preamble))
				}
				for pIdx, pBit := range preamble[:length] {
					d.Quantized[idx+pIdx*d.Cfg.SymbolLength] = pBit
				}
			}

			d.pack()
			for _, preamble := range preambles {
				want := naiveSearch(&d, preamble)
				got := append([]int(nil), d.Search(preamble)...)
				if len(got) == 0 && len(want) == 0 {
					continue
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("chipLength %d trial %d preamble %v: got %v, want %v",
						chipLength, trial, preamble, got, want)
				}
			}
		}
	}
}
