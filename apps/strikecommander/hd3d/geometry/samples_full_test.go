package geometry

import (
	"math"
	"math/big"
	"testing"
)

func TestSamplesFullViewportOriginalEdges(t *testing.T) {
	// Legal int32 operands, Near=1 and a Q8 coordinate beyond int64. Every
	// viewport sample is checked independently from raw Point3/Camera operands.
	points := [3]Point3{{math.MinInt32, 1, math.MaxInt32}, {math.MaxInt32, 2, math.MaxInt32}, {0, 1, math.MinInt32}}
	cam := Camera{CX: 160, CY: 99, FX: math.MaxInt32, FY: math.MaxInt32}
	for _, n := range []int{2, 3} {
		mesh, err := ProjectFaceSamples(FaceInput{Vertices: points, Camera: cam, Near: 1, Flag17BC: 1}, n)
		if err != nil {
			t.Fatal(err)
		}
		var xy [3][2]float64
		var z [3]int32
		for k, p := range points {
			xy[k] = sampleIndependentProjection(p, cam, n)
			z[k] = p.Z
		}
		o := newSampleOracle(xy, z)
		for y := 0; y < 200*n; y++ {
			for x := 0; x < 320*n; x++ {
				for j := 0; j < 2; j++ {
					for i := 0; i < 2; i++ {
						checkSampleOracle(t, mesh.Triangles[0], o, x, y, i, j, false)
					}
				}
			}
		}
		t.Logf("N%d full viewport: %d independently checked phase samples", n, 320*n*200*n*4)
	}
}

func TestSamplesDepthOneAndReverseNearEdge(t *testing.T) {
	xy := [3][2]float64{{0, 0}, {4, 0}, {0, 4}}
	a := sampleTestTriangle(xy, [3]int32{16777216, 16777216, 16777216}, 2)
	b := sampleTestTriangle(xy, [3]int32{16777215, 16777215, 16777215}, 2)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			ha, _ := EvaluateSample(a, 0, 0, i, j)
			hb, _ := EvaluateSample(b, 0, 0, i, j)
			if !ha.Coverage || !hb.Coverage || ha.DepthQ48 != 16777216 || hb.DepthQ48 != 16777217 {
				t.Fatal("one Q48 unit", ha.DepthQ48, hb.DepthQ48)
			}
		}
	}
	// Same geometric edge traversed in opposite directions. Keep per-endpoint
	// attributes attached to the endpoint identity, not its new array index.
	p := Point3{X: -5, Z: 3, Y: 7}
	q := Point3{X: 0, Z: 0, Y: -8}
	faces := [][3]Point3{{p, q, {X: 4, Z: 4, Y: 4}}, {q, p, {X: 4, Z: 4, Y: 4}}}
	wantPoint := sampleOracleClip(p, q, 1)
	wantAttribute := big.NewRat(65535, 1)
	wantAttribute.Sub(wantAttribute, new(big.Rat).Mul(big.NewRat(65535-7, 1), big.NewRat(2, 3)))
	for _, points := range faces {
		m, err := ProjectFaceSamples(FaceInput{Vertices: points, Near: 1, Camera: Camera{FX: 256, FY: 256}}, 3)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, v := range m.Polygon {
			s := v.Source
			if s.Kind != SourceNearEdge || points[s.In] != p || points[s.Out] != q {
				continue
			}
			found = true
			attr := new(big.Rat).Sub(big.NewRat(65535, 1), new(big.Rat).Mul(big.NewRat(65535-7, 1), new(big.Rat).SetFrac(new(big.Int).SetUint64(s.N), new(big.Int).SetUint64(s.D))))
			if v.Point != wantPoint || attr.Cmp(wantAttribute) != 0 {
				t.Fatal("reverse edge provenance", v.Point, attr)
			}
		}
		if !found {
			t.Fatal("shared original edge not found")
		}
	}
}

func TestSamplesQ8Ties(t *testing.T) {
	for _, q := range []struct {
		x    float64
		want int64
	}{{.5 / 256, 0}, {1.5 / 256, 2}, {-1.5 / 256, -2}, {-2.5 / 256, -2}} {
		tri := sampleTestTriangle([3][2]float64{{q.x, 0}, {4, 0}, {0, 4}}, [3]int32{1, 1, 1}, 2)
		x, _ := tri.Vertices[0].Q8()
		if x.Cmp(big.NewInt(q.want)) != 0 {
			t.Fatal("Q8 tie", x, q.want)
		}
	}
}

func TestSamplesProjectionQ8RawOperands(t *testing.T) {
	f := FaceInput{Vertices: [3]Point3{{1, 3, 1}, {7, 5, -2}, {-5, 7, 3}}, Near: 1, Flag17BC: 1, Camera: Camera{CX: 160, CY: 99, FX: 256, FY: 256}}
	for _, n := range []int{2, 3} {
		m, err := ProjectFaceSamples(f, n)
		if err != nil {
			t.Fatal(err)
		}
		var xy [3][2]float64
		var z [3]int32
		for k, p := range f.Vertices {
			xy[k] = sampleIndependentProjection(p, f.Camera, n)
			z[k] = p.Z
			x, y := m.Polygon[k].Q8()
			if x.Cmp(sampleOracleRound(xy[k][0])) != 0 || y.Cmp(sampleOracleRound(xy[k][1])) != 0 {
				t.Fatal("Q8 original N projection", k, x, y)
			}
		}
		o := newSampleOracle(xy, z)
		for y := 98 * n; y <= 101*n; y++ {
			for x := 158 * n; x <= 163*n; x++ {
				for j := 0; j < 2; j++ {
					for i := 0; i < 2; i++ {
						checkSampleOracle(t, m.Triangles[0], o, x, y, i, j, false)
					}
				}
			}
		}
	}
}

func TestSamplesNearTwoFansIndependentCoverage(t *testing.T) {
	f := FaceInput{Vertices: [3]Point3{{-2, 0, -2}, {4, 4, 0}, {0, 4, 4}}, Near: 2, Camera: Camera{CX: 3, CY: 5, FX: 256, FY: 256}}
	// Hand-specified rational intersections, before seeing the clip result.
	polygon := [4]Point3{{1, 2, -1}, {4, 4, 0}, {0, 4, 4}, {-1, 2, 1}}
	for _, n := range []int{2, 3} {
		m, err := ProjectFaceSamples(f, n)
		if err != nil || len(m.Triangles) != 2 {
			t.Fatal("near quad", err)
		}
		var oracles [2]sampleOracle
		for fan := 0; fan < 2; fan++ {
			var xy [3][2]float64
			var z [3]int32
			for k, at := range [3]int{0, fan + 1, fan + 2} {
				xy[k] = sampleIndependentProjection(polygon[at], f.Camera, n)
				z[k] = polygon[at].Z
			}
			oracles[fan] = newSampleOracle(xy, z)
		}
		for y := 0; y < 9*n; y++ {
			for x := 0; x < 6*n; x++ {
				for j := 0; j < 2; j++ {
					for i := 0; i < 2; i++ {
						for fan := 0; fan < 2; fan++ {
							checkSampleOracle(t, m.Triangles[fan], oracles[fan], x, y, i, j, false)
						}
						a, _ := EvaluateSample(m.Triangles[0], x, y, i, j)
						b, _ := EvaluateSample(m.Triangles[1], x, y, i, j)
						if a.Coverage && b.Coverage {
							t.Fatal("near fan shared edge double coverage")
						}
					}
				}
			}
		}
		// Fan diagonal joins original polygon indices 0 and 2. Both immutable
		// source values and Q8 coordinates must remain identical across fans.
		for _, pair := range [][2]int{{0, 0}, {2, 1}} {
			a, b := m.Triangles[0].Vertices[pair[0]], m.Triangles[1].Vertices[pair[1]]
			ax, ay := a.Q8()
			bx, by := b.Q8()
			if a.Source != b.Source || ax.Cmp(bx) != 0 || ay.Cmp(by) != 0 {
				t.Fatal("fan provenance discontinuity")
			}
		}
	}
}
