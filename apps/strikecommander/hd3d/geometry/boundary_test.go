package geometry

import (
	"bytes"
	"image/color"
	"math/big"
	"reflect"
	"testing"
)

func TestGuardReroundingCounterexample(t *testing.T) {
	// The original edge is y=.5+x/(2*M). At the pixel center (.5,.5),
	// it is strictly outside. Clipping at +/-G then requantizing flattens
	// both intersection Y values to Q8=128, incorrectly including the point.
	const m = float64(1 << 31)
	const g = float64(1 << 21)
	original := Triangle{Vertices: [3]Projection{projected(-m, 0, 1), projected(m, 1, 1), projected(m, 10, 1)}}
	want, inside := oracleAt(original, 0, 0)
	if inside || want != 0 {
		t.Fatal("bad independent counterexample")
	}
	target, _ := NewTarget(2)
	target.Draw(original)
	if target.frame.DepthQ48[0] != 0 {
		t.Fatal("original Q8 edge changed")
	}
	y0, y1 := .5-g/(2*m), .5+g/(2*m)
	if oracleQ(y0).Cmp(big.NewInt(128)) != 0 || oracleQ(y1).Cmp(big.NewInt(128)) != 0 {
		t.Fatal("counterexample intersections")
	}
	mutant := Triangle{Vertices: [3]Projection{projected(-g, .5, 1), projected(g, .5, 1), projected(g, 5.00390625, 1)}}
	bad, _ := NewTarget(2)
	bad.Draw(mutant)
	if bad.frame.DepthQ48[0] == 0 {
		t.Fatal("rerounding mutation was not detected")
	}
}
func TestSharedEdgeAndDeterminism(t *testing.T) {
	a := Triangle{Vertices: [3]Projection{projected(.5, .5, 1), projected(3.5, .5, 1), projected(3.5, 3.5, 1)}, Ref: FaceRef{Object: 1}, WireColor: color.RGBA{1, 2, 3, 255}}
	b := Triangle{Vertices: [3]Projection{projected(.5, .5, 1), projected(3.5, 3.5, 1), projected(.5, 3.5, 1)}, Ref: FaceRef{Object: 2}, WireColor: color.RGBA{4, 5, 6, 255}}
	t1, _ := NewTarget(2)
	t2, _ := NewTarget(2)
	t1.Draw(a)
	t1.Draw(b)
	t2.Draw(a)
	t2.Draw(b)
	if !reflect.DeepEqual(t1.frame.DepthQ48, t2.frame.DepthQ48) || !reflect.DeepEqual(t1.frame.Owner, t2.frame.Owner) || !bytes.Equal(t1.frame.Wire.Pix, t2.frame.Wire.Pix) {
		t.Fatal("same triangles nondeterministic")
	}
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			da, ina := oracleAt(a, x, y)
			db, inb := oracleAt(b, x, y)
			if ina && inb {
				t.Fatal("shared edge double coverage")
			}
			want := uint64(0)
			if ina {
				want = da
			}
			if inb {
				want = db
			}
			if t1.frame.DepthQ48[y*t1.frame.Width+x] != want {
				t.Fatal("shared coverage", x, y)
			}
		}
	}
}
func TestDepth128Oracle(t *testing.T) {
	// Independent exact numerator/denominator oracle with products exceeding64.
	for _, tc := range []struct {
		e [3]int64
		z [3]uint64
	}{{[3]int64{1, 2, 3}, [3]uint64{10, 20, 30}}, {[3]int64{1 << 59, (1 << 59) - 1, 1}, [3]uint64{1 << 48, (1 << 48) - 1, 1}}, {[3]int64{3, 4, 5}, [3]uint64{17, 17, 17}}} {
		area := uint64(tc.e[0] + tc.e[1] + tc.e[2])
		sum := new(big.Int)
		for i, v := range tc.e {
			sum.Add(sum, new(big.Int).Mul(big.NewInt(v), new(big.Int).SetUint64(tc.z[i])))
		}
		sum.Quo(sum, new(big.Int).SetUint64(area))
		if got := depth128(tc.e, tc.z, area); got != sum.Uint64() {
			t.Fatal(got, sum)
		}
	}
}
func TestRoundingTies(t *testing.T) {
	for _, tc := range []struct {
		x    float64
		want int64
	}{{.5, 128}, {1.5, 384}, {-0.5, -128}, {.5 / 256, 0}, {1.5 / 256, 2}, {-1.5 / 256, -2}, {-2.5 / 256, -2}} {
		if got := quantize(tc.x); got.Cmp(big.NewInt(tc.want)) != 0 {
			t.Fatal(tc.x, got, tc.want)
		}
	}
}

func TestExtremeScreenFullOracle(t *testing.T) {
	// Explicit +/-2^31 one-times screen coordinates, in addition to the
	// legal int32 X/fx case whose Q8 exceeds int64 in TestBigOriginalEdges.
	for _, n := range []int{2, 3} {
		m := float64(n) * float64(uint64(1)<<31)
		tri := Triangle{Vertices: [3]Projection{projected(-m, -m, 3), projected(m, -m, 5), projected(0, m, 7)}, Ref: FaceRef{Object: 1}}
		target, _ := NewTarget(n)
		if err := target.Draw(tri); err != nil {
			t.Fatal(err)
		}
		checkFullOracle(t, tri, target.frame)
	}
}
