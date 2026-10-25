package opaque

import (
	"math/big"
	"testing"
)

func TestContinuousLiteralRowsAndNoIntermediateFloor(t *testing.T) {
	table := make([]byte, 4096)
	for r := 0; r < 16; r++ {
		for c := 0; c < 256; c++ {
			table[r*256+c] = byte(r)
		}
	}
	// class7 s=247: (255-247)/16=1/2. Class8 s=220 -> S=247.5,
	// r=15/32. An intermediate integer floor incorrectly gives 1/2.
	a, e := continuousR4(big.NewRat(247, 1), 7, 17, table)
	if e != nil || a != (SampleColor{0, 1, 1 << 23}) {
		t.Fatalf("class7 %+v %v", a, e)
	}
	b, e := continuousR4(big.NewRat(220, 1), 8, 17, table)
	if e != nil || b != (SampleColor{0, 1, 15 << 19}) {
		t.Fatalf("class8 %+v %v", b, e)
	}
	for _, s := range []int64{0, 255, 256, 286} {
		v, e := continuousR4(big.NewRat(s, 1), 7, 255, table)
		if e != nil {
			t.Fatal(e)
		}
		if s == 0 && v.Lo != 15 || s >= 255 && v.Lo != 0 {
			t.Fatal(v)
		}
	}
	// All 766 possible sums use integer original3sum/8, including saturation.
	for sum := 0; sum <= 765; sum++ {
		x := min(sum, 255)
		y := min(sum-x, 255)
		z := sum - x - y
		got, e := originalShadeRow(8, [3]byte{byte(x), byte(y), byte(z)})
		want := (255 - min(3*sum/8, 255)) / 16
		if e != nil || got != want {
			t.Fatalf("sum%d: %d want%d", sum, got, want)
		}
	}
	if row, _ := originalShadeRow(7, [3]byte{255, 255, 255}); row != 0 {
		t.Fatal(row)
	}
}
func TestContinuousExactGlassAndSinglePixelRound(t *testing.T) {
	// Two layers over0 give67 then100.5; rounding each layer would give101.
	v, e := applyGlass134(big.NewRat(0, 1))
	if e != nil || v.Cmp(big.NewRat(67, 1)) != 0 {
		t.Fatal(v, e)
	}
	v, e = applyGlass134(v)
	if e != nil || v.Cmp(big.NewRat(201, 2)) != 0 {
		t.Fatal(v, e)
	}
	out, e := resolveFour([4]*big.Rat{v, v, v, v})
	if e != nil || out != 100 {
		t.Fatal(out, e)
	}
	out, e = resolveFour([4]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 2)})
	if e != nil || out != 0 {
		t.Fatal(out, e)
	}
	out, e = resolveFour([4]*big.Rat{big.NewRat(3, 2), big.NewRat(3, 2), big.NewRat(3, 2), big.NewRat(3, 2)})
	if e != nil || out != 2 {
		t.Fatal(out, e)
	}
	huge := new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(1), 513), big.NewInt(1))
	if _, e := applyGlass134(huge); e == nil {
		t.Fatal("512bit bound ignored")
	}
}

func TestContinuousInteger128AgainstExactFraction(t *testing.T) {
	var table [4096]byte
	for r := 0; r < 16; r++ {
		for x := 0; x < 256; x++ {
			table[r*256+x] = byte((r*13 + x) % 256)
		}
	}
	for _, class := range []uint8{7, 8} {
		for _, area := range []uint64{1, 3, 7, 16, 257, 65537, (1 << 46) - 1} {
			for _, num := range []uint64{0, area, area * 117, area*220 + area/3, area * 247, area * 255} {
				got, ok := continuousRatio(num, area, class, 19, &table)
				if !ok {
					t.Fatal("guard unexpectedly rejected")
				}
				scalar := new(big.Rat).SetFrac(new(big.Int).SetUint64(num), new(big.Int).SetUint64(area))
				want, e := continuousR4(scalar, class, 19, table[:])
				if e != nil || got != want {
					t.Fatal(class, num, area, got, want, e)
				}
			}
		}
	}
	if _, ok := continuousRatio(1, 1<<47, 8, 0, &table); ok {
		t.Fatal("outside arithmetic guard accepted")
	}
}

func TestCompositionSharedResolverLiteralRoundAndVisibility(t *testing.T) {
	var palette [256][3]byte
	var colors [4]SampleColor
	got, e := resolveCompositionPixel(colors, [4]uint16{2, 2, 2, 2}, palette)
	if e != nil || got != ([4]byte{100, 100, 100, 255}) {
		t.Fatal(got, e)
	}
	for _, v := range []struct {
		bg, os, gs uint16
		od, gd     uint64
		want       bool
	}{
		{0, 4, 5, 17, 17, true}, {0, 5, 4, 17, 17, false}, {5, 0, 5, 0, 19, false},
		{0, 0, 5, 0, 19, true}, {0, 2, 5, 20, 19, false}, {0, 8, 5, 20, 21, true},
	} {
		if got := compositionGlassVisible(v.bg, v.od, v.os, v.gd, v.gs); got != v.want {
			t.Fatal(v, got)
		}
	}
	if _, e := resolveCompositionPixel(colors, [4]uint16{600, 0, 0, 0}, palette); e == nil {
		t.Fatal("512-bit rational bound ignored")
	}
}
