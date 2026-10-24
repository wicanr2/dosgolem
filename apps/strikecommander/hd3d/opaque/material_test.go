package opaque

import (
	"encoding/binary"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"math/big"
	"testing"
)

// T03: literal per-sample components. This oracle never calls the renderer.
func TestR3LiteralPhasesAndBand(t *testing.T) {
	want := []uint32{1 << 22, 3 << 22, 3 << 22, 1 << 22}
	for n := 0; n < 4; n++ {
		got, err := r3Color(big.NewRat(21, 4), Band{5, 99, 1}, 37, 19, n%2, n/2)
		if err != nil || got != (SampleColor{5, 6, want[n]}) {
			t.Fatalf("sample%d=%+v %v", n, got, err)
		}
	}
	got, err := r3Color(big.NewRat(6, 1), Band{5, 99, 1}, 0, 0, 1, 0)
	if err != nil || got != (SampleColor{6, 6, 0}) {
		t.Fatal(got, err)
	}
	if _, err = r3Color(big.NewRat(-1, 2), Band{5, 99, 1}, 0, 0, 0, 0); err == nil {
		t.Fatal("out-of-band accepted")
	}
	for _, tc := range []struct {
		n, d int64
		want uint32
	}{{1, 2, 0}, {3, 2, 2}, {5, 2, 2}} {
		v := new(big.Rat).SetFrac(big.NewInt(tc.n), new(big.Int).Mul(big.NewInt(tc.d), big.NewInt(1<<24)))
		if q, err := weight24(v); err != nil || q != tc.want {
			t.Fatal(tc, q, err)
		}
	}
}

// T04/T05: FF short-circuits shade; 16-bit address wrap is not file wrap.
func TestR4FFShadeAndAddressOracle(t *testing.T) {
	c := hd3d.Call{Kind: hd3d.R4}
	c.Params[4] = 0xffff
	c.Params[6] = 3
	c.Params[7] = 0x100
	c.Params[8] = 0x9000
	c.Tex.Pixels.Spans = []hd3d.Span{{Off: 2, Data: []byte{0xff}}}
	got, wrote, err := r4Color(c, big.NewRat(0, 1), big.NewRat(256, 1))
	if err != nil || wrote || got != (SampleColor{}) {
		t.Fatal(got, wrote, err)
	}
	c.Tex.Pixels.Spans[0].Data[0] = 7
	if _, _, err = r4Color(c, big.NewRat(0, 1), big.NewRat(256, 1)); err == nil {
		t.Fatal("missing required shade accepted")
	}
	c.Tex.Shade.Spans = []hd3d.Span{{Off: 0x107, Data: []byte{255}}}
	got, wrote, err = r4Color(c, big.NewRat(0, 1), big.NewRat(256, 1))
	if err != nil || !wrote || got != (SampleColor{255, 255, 0}) {
		t.Fatal(got, wrote, err)
	}
	c.Tex.Pixels.Spans = append(c.Tex.Pixels.Spans, hd3d.Span{Off: 2, Data: []byte{8}})
	if _, _, err = r4Color(c, big.NewRat(0, 1), big.NewRat(256, 1)); err == nil {
		t.Fatal("conflicting captured byte accepted")
	}
}
func TestR1LowByteOracle(t *testing.T) {
	c := hd3d.Call{Kind: hd3d.R1, Verts: make([]byte, 66)}
	c.Params[1] = 3
	binary.LittleEndian.PutUint16(c.Verts[4:], 0xa517)
	got, err := r1Color(c)
	if err != nil || got != (SampleColor{23, 23, 0}) {
		t.Fatal(got, err)
	}
}

func TestClipAttributeExactOracle(t *testing.T) {
	// n/d=1/3 between 256 and 1024: exact result 512, not geometric-X interpolation.
	got, err := clipAttribute([3]uint16{256, 1024, 7}, 1, 0, 1, 1, 3)
	if err != nil || got.Cmp(big.NewRat(512, 1)) != 0 {
		t.Fatal(got, err)
	}
	got, err = clipAttribute([3]uint16{1, 2, 3}, 1, 0, 1, 1, 3)
	if err != nil || got.Cmp(big.NewRat(4, 3)) != 0 {
		t.Fatal(got, err)
	}
}

func TestFastR3LiteralFractionsAndTies(t *testing.T) {
	for _, c := range []struct {
		num, area uint64
		phase     int
		want      SampleColor
	}{{4097, 3, 0, SampleColor{5, 6, 5614251}}, {4097, 3, 1, SampleColor{5, 6, 14002859}}, {5*(1<<25) + 1, 1 << 17, 0, SampleColor{5, 5, 0}}, {5*(1<<25) + 3, 1 << 17, 0, SampleColor{5, 6, 2}}, {5*(1<<25) + 5, 1 << 17, 0, SampleColor{5, 6, 2}}} {
		if got := fastR3(c.num, c.area, Band{1, 99, 30}, c.phase); got != c.want {
			t.Fatal(c, got)
		}
	}
	w := weights{large: geometry.SampleHit{Area: big.NewInt(4), Edges: [3]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(1)}, SourceIndices: [3]int{0, 2, 1}}}
	got, e := w.attribute([3]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3), big.NewRat(5, 3)})
	if e != nil || got.Cmp(big.NewRat(13, 12)) != 0 {
		t.Fatal(got, e)
	}
}
