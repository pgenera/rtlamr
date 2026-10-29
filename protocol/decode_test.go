package protocol_test

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bemasher/rtlamr/protocol"
	_ "github.com/bemasher/rtlamr/r900"
	_ "github.com/bemasher/rtlamr/scm"
	_ "github.com/bemasher/rtlamr/scmplus"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata")

// testdata/*.cu8 are rtl_sdr captures (uint8 IQ) at 2359296 S/s, 912.38 MHz,
// i.e. the default -symbollength=72.
const symbolLength = 72

func newDecoder(tb testing.TB, msgTypes ...string) *protocol.Decoder {
	tb.Helper()

	d := protocol.NewDecoder()
	for _, name := range msgTypes {
		p, err := protocol.NewParser(name, symbolLength)
		if err != nil {
			tb.Fatal(err)
		}
		d.RegisterProtocol(p)
	}
	d.Allocate()

	return &d
}

// decodeFile feeds a capture to the decoder one block at a time, as
// Receiver.Run does, and returns one line per message. Messages seen in
// consecutive blocks are reported once.
func decodeFile(tb testing.TB, d *protocol.Decoder, name string) []string {
	tb.Helper()

	f, err := os.Open(name)
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()

	var lines []string
	prev := map[protocol.Digest]bool{}
	block := make([]byte, d.Cfg.BlockSize2)
	for blockIdx := 0; ; blockIdx++ {
		if _, err := io.ReadFull(f, block); err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		} else if err != nil {
			tb.Fatal(err)
		}

		next := map[protocol.Digest]bool{}
		var blockLines []string
		for msg := range d.Decode(block) {
			digest := protocol.NewDigest(msg)
			if next[digest] = true; prev[digest] {
				continue
			}

			js, err := json.Marshal(msg)
			if err != nil {
				tb.Fatal(err)
			}
			blockLines = append(blockLines, fmt.Sprintf("%d %s %s", blockIdx, msg.MsgType(), js))
		}
		prev = next

		// Parsers run concurrently, so order within a block is arbitrary.
		sortStrings(blockLines)
		lines = append(lines, blockLines...)
	}

	return lines
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func TestDecodeGolden(t *testing.T) {
	captures, err := filepath.Glob(filepath.Join("testdata", "*.cu8"))
	if err != nil {
		t.Fatal(err)
	}
	if len(captures) == 0 {
		t.Fatal("no captures in testdata")
	}

	for _, capture := range captures {
		t.Run(filepath.Base(capture), func(t *testing.T) {
			d := newDecoder(t, "scm", "scm+", "r900")
			got := decodeFile(t, d, capture)

			golden := strings.TrimSuffix(capture, ".cu8") + ".golden"
			if *update {
				if err := os.WriteFile(golden, []byte(strings.Join(got, "\n")+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			gf, err := os.Open(golden)
			if err != nil {
				t.Fatal(err)
			}
			defer gf.Close()

			var want []string
			for s := bufio.NewScanner(gf); s.Scan(); {
				want = append(want, s.Text())
			}

			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("decoded messages differ from %s\ngot:\n%s\nwant:\n%s",
					golden, strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// BenchmarkDecode reports the time to decode one block of a real capture,
// most of which, like most of any capture, contains no packets.
func BenchmarkDecode(b *testing.B) {
	samples, err := os.ReadFile(filepath.Join("testdata", "scm-scmplus-r900.cu8"))
	if err != nil {
		b.Fatal(err)
	}

	for _, msgTypes := range [][]string{
		{"scm"},
		{"r900"},
		{"scm", "scm+", "r900"},
	} {
		b.Run(strings.Join(msgTypes, ","), func(b *testing.B) {
			d := newDecoder(b, msgTypes...)
			blockSize := d.Cfg.BlockSize2
			blocks := len(samples) / blockSize

			b.SetBytes(int64(blockSize))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				blockIdx := i % blocks
				for range d.Decode(samples[blockIdx*blockSize : (blockIdx+1)*blockSize]) {
				}
			}
		})
	}
}
