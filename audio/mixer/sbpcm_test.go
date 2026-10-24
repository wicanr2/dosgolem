package mixer

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/audio/sbpcm"
	"github.com/wicanr2/dosgolem/internal/machine"
	"math"
	"reflect"
	"testing"
)

func goldenOPL(m *Mixer) {
	for _, w := range [][2]uint8{{0x20, 0x01}, {0x23, 0x21}, {0x40, 0x3f}, {0x43, 0}, {0x60, 0xf0}, {0x63, 0xf0}, {0x80, 0x0f}, {0x83, 0x0f}, {0xc0, 0x31}, {0xa0, 0x44}, {0xb0, 0x32}} {
		m.OPLWrite(machine.OPLWrite{Reg: w[0], Val: w[1]})
	}
}

func TestSBPCMRenderAndTurnSplits(t *testing.T) {
	events := []machine.SBPCMEvent{
		{Kind: "start", Clock: "steps", Command: 0x14, Step: 1, Start: 1, Due: 250001, Length: 4, Data: []byte{0, 64, 128, 255}},
		{Kind: "pause", Clock: "steps", Step: 70003},
		{Kind: "resume", Clock: "steps", Step: 90007, Start: 20005, Due: 270005, Transferred: 1},
		{Kind: "complete", Clock: "steps", Step: 270005},
		{Kind: "start", Clock: "steps", Command: 0x14, Step: 270400, Start: 270400, Due: 350000, Length: 2, Data: []byte{255, 0}},
		{Kind: "cancel", Clock: "steps", Step: 333333},
	}
	render := func(chunk uint64) []float32 {
		m := New(sps)
		var out []float32
		next := 0
		for before := uint64(0); before < 385000; {
			after := min(before+chunk, uint64(385000))
			for next < len(events) && events[next].Step <= after {
				m.SBPCM(events[next])
				next++
			}
			out = m.Render(out, before, after)
			before = after
		}
		return out
	}
	want := render(385000)
	for _, chunk := range []uint64{1, 7, 192500} {
		if got := render(chunk); !reflect.DeepEqual(got, want) {
			t.Fatalf("chunk %d 改變輸出", chunk)
		}
	}
	source := sbpcm.New(sps)
	for _, e := range events {
		source.Apply(e)
	}
	for i := 0; i < len(want)/2; i++ {
		v := float32(float64(source.Sample(uint64(i))) / 32768)
		if want[2*i] != v || want[2*i+1] != v {
			t.Fatalf("sample %d: %v != %v", i, want[2*i:2*i+2], v)
		}
	}
}

func TestSBPCMOPLClipping(t *testing.T) {
	base, mix := New(sps), New(sps)
	goldenOPL(base)
	goldenOPL(mix)
	// 取已知 OPL 的正負兩半波，SBPCM 各加接近滿幅，直接驗相加後夾限。
	opl := base.Render(nil, 0, sps/5)
	for _, b := range []byte{0, 255} {
		mix = New(sps)
		goldenOPL(mix)
		mix.SBPCM(machine.SBPCMEvent{Kind: "start", Clock: "steps", Command: 0x14, Start: 0, Due: sps / 5, Length: 1, Data: []byte{b}})
		out := mix.Render(nil, 0, sps/5)
		clipped := 0
		v := float64(int(b)-128) / 128
		for i, x := range out {
			want := clamp(float64(opl[i]) + v)
			if math.Abs(float64(x-want)) > 1e-7 {
				t.Fatalf("mix sample %d: %v want %v", i, x, want)
			}
			if x == 1 || x == -1 {
				clipped++
			}
		}
		if clipped == 0 {
			t.Fatal("未測到削波")
		}
	}
}

// 462 §4.1.7：期望雜湊必須先由未接 SBPCM 的既有 mixer 取得。
func TestSBPCMOPLGolden(t *testing.T) {
	m := New(sps)
	goldenOPL(m)
	out := m.Render(nil, 0, sps/5)
	b := make([]byte, len(out)*4)
	for i, v := range out {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	got := fmt.Sprintf("%x", sha256.Sum256(b))
	const want = "19903dea962205221431fae5f9d8bcba83bed7c43b755ea9207deaef68126a83"
	if got != want {
		t.Fatalf("OPL golden: got %s want %s", got, want)
	}
}
