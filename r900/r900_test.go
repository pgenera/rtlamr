package r900

import (
	"bytes"
	"math/rand"
	"sync"
	"testing"

	"github.com/bemasher/rtlamr/protocol"
)

// Parse skips filter for blocks with no preamble. That is only safe if filter
// recomputes all of p.quantized from p.signal, regardless of what an earlier
// call left in p.quantized and p.csum.
func TestFilterIgnoresPreviousOutput(t *testing.T) {
	d := protocol.NewDecoder()
	p := NewParser(72).(*Parser)
	d.RegisterProtocol(p)
	d.Allocate()

	// Allocate the parser's buffers.
	msgCh := make(chan protocol.Message)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	p.Parse(nil, msgCh, wg)

	rng := rand.New(rand.NewSource(1))
	for idx := range p.signal {
		p.signal[idx] = rng.Float32()
	}

	p.filter()
	want := append([]byte(nil), p.quantized...)

	// Scribble over what filter writes. csum[0] and the quantized tail past
	// limit are never written by anything, so they stay as allocated.
	limit := d.Cfg.BufferLength - 4*d.Cfg.ChipLength
	for idx := range p.quantized[:limit] {
		p.quantized[idx] = byte(rng.Intn(6))
	}
	for idx := range p.csum[1:] {
		p.csum[idx+1] = rng.Float32()
	}

	p.filter()
	if !bytes.Equal(p.quantized, want) {
		t.Fatal("filter output depends on its previous output")
	}
}
