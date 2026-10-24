package geometry

import (
	"image"
	"math"
	"math/big"
	"reflect"
	"testing"
)

// Test-only projected fixture. The oracle receives the original xy/z arrays,
// never these production-quantized ClipVertex values.
func sampleTestTriangle(xy [3][2]float64, z [3]int32, n int) MeshTriangle {
	var v [3]ClipVertex
	for k := range v {
		p := Projection{NX: xy[k][0], NY: xy[k][1], Z: z[k], InvZQ48: (uint64(1) << 48) / uint64(z[k])}
		v[k] = ClipVertex{Source: CornerSource{Kind: SourceOriginal, Original: k}, Point: Point3{Z: z[k]}, Projection: p, qx: quantize(p.NX), qy: quantize(p.NY)}
	}
	return newSampleTriangle(v, FaceRef{Object: 7, Face: 11}, n, 0, [3]int{0, 1, 2})
}

func TestSamplesFourPositionsOracle(t *testing.T) {
	fixtures := [][3][2]float64{
		{{.25, .25}, {4.75, .25}, {.25, 4.75}},
		{{.25, .25}, {.25, 4.75}, {4.75, .25}},
		{{-2097152, 0}, {2097152, 0}, {0, 3}},
		{{2097152 + .25, 0}, {2097152 - .25, 0}, {0, 3}},
		{{-2097152 - .25, 0}, {-2097152 + .25, 0}, {12, 3}},
		{{0, -2097152}, {0, 2097152}, {3, 0}},
		{{0, 2097152 + .25}, {0, 2097152 - .25}, {3, 0}},
		{{1.0 / 512, 65.0 / 256}, {1090921693185.0 / 256, 541165879361.0 / 256}, {-1090921693184.0 / 256, 1090921693184.0 / 256}},
		{{.25, .25}, {.25, .25}, {.25, .25}},
	}
	z := [3]int32{3, 5, 7}
	for _, n := range []int{2, 3} {
		for _, xy := range fixtures {
			tri := sampleTestTriangle(xy, z, n)
			o := newSampleOracle(xy, z)
			for y := 0; y < 8; y++ {
				for x := 0; x < 16; x++ {
					for j := 0; j < 2; j++ {
						for i := 0; i < 2; i++ {
							checkSampleOracle(t, tri, o, x, y, i, j, false)
							checkSampleOracle(t, tri, o, x, y, i, j, true)
						}
					}
				}
			}
			for j := 0; j < 2; j++ {
				for i := 0; i < 2; i++ {
					got, err := SampleBounds(tri, i, j)
					if err != nil {
						t.Fatal(err)
					}
					want := sampleOracleBounds(o, n, i, j)
					if got != want {
						t.Fatal("bounds", got, want)
					}
				}
			}
		}
	}
}

// Independent brute-force bounding-box membership, without production
// ceil/floor helpers, catches phase offsets and negative division mistakes.
func sampleOracleBounds(o sampleOracle, n, i, j int) image.Rectangle {
	if o.area.Sign() == 0 {
		return image.Rectangle{}
	}
	limits := [2]int{320 * n, 200 * n}
	phases := [2]int{64 + 128*i, 64 + 128*j}
	var lo, hi [2]int
	for axis := 0; axis < 2; axis++ {
		mn, mx := new(big.Int).Set(o.q[0][axis]), new(big.Int).Set(o.q[0][axis])
		for k := 1; k < 3; k++ {
			if o.q[k][axis].Cmp(mn) < 0 {
				mn.Set(o.q[k][axis])
			}
			if o.q[k][axis].Cmp(mx) > 0 {
				mx.Set(o.q[k][axis])
			}
		}
		lo[axis], hi[axis] = limits[axis], -1
		for p := 0; p < limits[axis]; p++ {
			v := big.NewInt(int64(256*p + phases[axis]))
			if v.Cmp(mn) >= 0 && v.Cmp(mx) <= 0 {
				if lo[axis] == limits[axis] {
					lo[axis] = p
				}
				hi[axis] = p
			}
		}
		if hi[axis] < lo[axis] {
			return image.Rectangle{}
		}
	}
	return image.Rect(lo[0], lo[1], hi[0]+1, hi[1]+1)
}

func TestSamplesNearSourcesAndLegacyProjection(t *testing.T) {
	cases := []struct {
		points  [3]Point3
		near    int32
		sources []CornerSource
	}{
		{[3]Point3{{0, 0, 0}, {4, 4, 0}, {0, 4, 4}}, 2, []CornerSource{
			{Kind: SourceNearEdge, Edge: [2]int{0, 1}, In: 1, Out: 0, N: 2, D: 4},
			{Kind: SourceOriginal, Original: 1}, {Kind: SourceOriginal, Original: 2},
			{Kind: SourceNearEdge, Edge: [2]int{2, 0}, In: 2, Out: 0, N: 2, D: 4}}},
		{[3]Point3{{0, 2, 0}, {4, 0, 0}, {0, 4, 4}}, 2, []CornerSource{
			{Kind: SourceOriginal, Original: 0},
			{Kind: SourceNearEdge, Edge: [2]int{0, 1}, In: 0, Out: 1, N: 0, D: 2},
			{Kind: SourceNearEdge, Edge: [2]int{1, 2}, In: 2, Out: 1, N: 2, D: 4},
			{Kind: SourceOriginal, Original: 2}}},
		{[3]Point3{{0, 4, 0}, {4, 0, 0}, {0, 0, 4}}, 2, []CornerSource{
			{Kind: SourceOriginal, Original: 0},
			{Kind: SourceNearEdge, Edge: [2]int{0, 1}, In: 0, Out: 1, N: 2, D: 4},
			{Kind: SourceNearEdge, Edge: [2]int{2, 0}, In: 0, Out: 2, N: 2, D: 4}}},
		{[3]Point3{{math.MaxInt32, math.MaxInt32, math.MinInt32}, {math.MinInt32, math.MinInt32, math.MaxInt32}, {0, math.MaxInt32, 0}}, 1, []CornerSource{
			{Kind: SourceOriginal, Original: 0},
			{Kind: SourceNearEdge, Edge: [2]int{0, 1}, In: 0, Out: 1, N: 2147483646, D: 4294967295},
			{Kind: SourceNearEdge, Edge: [2]int{1, 2}, In: 2, Out: 1, N: 2147483646, D: 4294967295},
			{Kind: SourceOriginal, Original: 2}}},
	}
	attrs := [3]int64{0, 65535, 17}
	for _, n := range []int{2, 3} {
		for _, tc := range cases {
			f := FaceInput{Vertices: tc.points, Near: tc.near, Camera: Camera{CX: 160, CY: 99, FX: 256, FY: 256}, Ref: FaceRef{Object: 2, Face: 3}}
			legacy, info, err := ProjectFace(f, n)
			if err != nil {
				t.Fatal(err)
			}
			mesh, err := ProjectFaceSamples(f, n)
			if err != nil {
				t.Fatal(err)
			}
			if mesh.Info != info || len(mesh.Polygon) != len(tc.sources) || len(mesh.Triangles) != len(legacy) {
				t.Fatal("shape/info", mesh.Info, info)
			}
			for k, want := range tc.sources {
				v := mesh.Polygon[k]
				if v.Source != want {
					t.Fatal("source", k, v.Source, want)
				}
				p := tc.points[want.Original]
				if want.Kind == SourceNearEdge {
					p = sampleOracleClip(tc.points[want.In], tc.points[want.Out], tc.near)
					// Source weights derive from raw Z, not truncated position.
					fraction := new(big.Rat).SetFrac(new(big.Int).SetUint64(want.N), new(big.Int).SetUint64(want.D))
					got := new(big.Rat).Sub(big.NewRat(attrs[want.In], 1), new(big.Rat).Mul(big.NewRat(attrs[want.In]-attrs[want.Out], 1), fraction))
					time := new(big.Rat).SetFrac(big.NewInt(int64(tc.near)-int64(tc.points[want.Out].Z)), big.NewInt(int64(tc.points[want.In].Z)-int64(tc.points[want.Out].Z)))
					expected := new(big.Rat).Add(big.NewRat(attrs[want.Out], 1), new(big.Rat).Mul(big.NewRat(attrs[want.In]-attrs[want.Out], 1), time))
					if got.Cmp(expected) != 0 || want.N >= want.D {
						t.Fatal("rational attribute source", got, expected)
					}
				}
				if v.Point != p {
					t.Fatal("independent rational clip", v.Point, p)
				}
			}
			for fan, tr := range mesh.Triangles {
				if tr.Fan != fan || tr.PolygonIndices != [3]int{0, fan + 1, fan + 2} {
					t.Fatal("fan", tr)
				}
				for k, v := range tr.Vertices {
					if v.Projection != legacy[fan].Vertices[k] {
						t.Fatal("legacy projection drift")
					}
				}
			}
			after, afterInfo, err := ProjectFace(f, n)
			if err != nil || afterInfo != info || !reflect.DeepEqual(after, legacy) {
				t.Fatal("legacy path changed")
			}
		}
	}
	// All outside; a lone plane endpoint retains duplicates and degenerate fan.
	for _, p := range [][3]Point3{{{0, 0, 0}, {1, 0, 0}, {0, 0, 1}}, {{0, 2, 0}, {1, 0, 0}, {0, 0, 1}}} {
		m, err := ProjectFaceSamples(FaceInput{Vertices: p, Near: 2, Camera: Camera{FX: 256, FY: 256}}, 2)
		if err != nil {
			t.Fatal(err)
		}
		if p[0].Z == 0 {
			if !m.Info.AllClipped || len(m.Polygon) != 0 {
				t.Fatal("all clipped")
			}
		} else {
			if len(m.Polygon) != 3 || len(m.Triangles) != 1 {
				t.Fatal("plane duplicates lost")
			}
			h, _ := EvaluateSample(m.Triangles[0], 0, 0, 0, 0)
			if !h.Degenerate || h.Coverage {
				t.Fatal("degenerate plane fan")
			}
		}
	}
}

func sampleIndependentProjection(p Point3, c Camera, n int) [2]float64 {
	// Explicit binary64 steps are the common numeric contract, independently
	// applied to raw operands without consulting a returned Projection.
	px := float64(float64(p.X) * float64(c.FX))
	py := float64(float64(p.Y) * float64(c.FY))
	den := float64(float64(256) * float64(p.Z))
	x := float64(float64(c.CX) + float64(px/den))
	y := float64(float64(int64(c.CY)+1) - float64(py/den))
	return [2]float64{float64(float64(n) * x), float64(float64(n) * y)}
}

func TestSamplesExtremeInt32FourPhases(t *testing.T) {
	points := [3]Point3{{math.MinInt32, 1, math.MaxInt32}, {math.MaxInt32, 2, math.MaxInt32}, {0, 1, math.MinInt32}}
	for _, n := range []int{2, 3} {
		for _, focal := range [][2]int32{{math.MaxInt32, math.MaxInt32}, {math.MaxInt32, math.MinInt32}, {math.MinInt32, math.MaxInt32}, {math.MinInt32, math.MinInt32}} {
			cam := Camera{CX: 160, CY: 99, FX: focal[0], FY: focal[1]}
			f := FaceInput{Vertices: points, Camera: cam, Near: 1, Flag17BC: 1}
			mesh, err := ProjectFaceSamples(f, n)
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
			for j := 0; j < 2; j++ {
				for i := 0; i < 2; i++ {
					got, err := SampleBounds(mesh.Triangles[0], i, j)
					if err != nil || got != sampleOracleBounds(o, n, i, j) {
						t.Fatal("extreme bounds", got, err)
					}
					for _, p := range [][2]int{{0, 0}, {320*n - 1, 0}, {0, 200*n - 1}, {320*n - 1, 200*n - 1}, {160 * n, 100 * n}} {
						checkSampleOracle(t, mesh.Triangles[0], o, p[0], p[1], i, j, false)
					}
				}
			}
		}
	}
}

func TestSamplesSharedEdgesWindingAndOwnedValues(t *testing.T) {
	z := [3]int32{3, 5, 7}
	xya := [3][2]float64{{.25, .25}, {3.75, .25}, {3.75, 3.75}}
	xyb := [3][2]float64{{.25, .25}, {3.75, 3.75}, {.25, 3.75}}
	a, b := sampleTestTriangle(xya, z, 2), sampleTestTriangle(xyb, z, 2)
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			for j := 0; j < 2; j++ {
				for i := 0; i < 2; i++ {
					ha, _ := EvaluateSample(a, x, y, i, j)
					hb, _ := EvaluateSample(b, x, y, i, j)
					if ha.Coverage && hb.Coverage {
						t.Fatal("shared edge double coverage")
					}
					if x < 3 && y < 3 && !(ha.Coverage || hb.Coverage) {
						t.Fatal("shared edge hole")
					}
				}
			}
		}
	}
	h, _ := EvaluateSample(a, 1, 1, 0, 0)
	if !h.Coverage {
		t.Fatal("owned fixture")
	}
	want := new(big.Int).Set(h.Area)
	h.Area.SetInt64(-1)
	h.Edges[0].SetInt64(-2)
	qx, qy := a.Vertices[0].Q8()
	qx.SetInt64(-10000)
	qy.SetInt64(-10000)
	again, _ := EvaluateSample(a, 1, 1, 0, 0)
	if again.Area.Cmp(want) != 0 {
		t.Fatal("returned area alias")
	}
	checkSampleOracle(t, a, newSampleOracle(xya, z), 1, 1, 0, 0, false)
	a.Vertices[0].Source.Original = 2
	if _, err := EvaluateSample(a, 1, 1, 0, 0); err == nil {
		t.Fatal("modified provenance accepted")
	}
}

func TestSamplesRejectInvalidAndFlaggedNear(t *testing.T) {
	f := FaceInput{Vertices: [3]Point3{{0, 1, 0}, {4, 4, 0}, {0, 4, 4}}, Near: 2, Flag17BC: 1, Camera: Camera{FX: 256, FY: 256}}
	m, err := ProjectFaceSamples(f, 2)
	if err != nil || m.Info.Clipped || m.Info.NearUnexpectedVertices != 1 || len(m.Triangles) != 1 {
		t.Fatal("flagged near", m.Info, err)
	}
	for _, n := range []int{0, 1, 4} {
		if _, err := ProjectFaceSamples(f, n); err == nil {
			t.Fatal("bad scale")
		}
	}
	f.Near = 0
	if _, err := ProjectFaceSamples(f, 2); err == nil {
		t.Fatal("bad Near")
	}
	f.Near = 2
	f.Vertices[0].Z = 0
	if _, err := ProjectFaceSamples(f, 2); err == nil {
		t.Fatal("bad Z")
	}
	for _, v := range [][4]int{{-1, 0, 0, 0}, {640, 0, 0, 0}, {0, 400, 0, 0}, {0, 0, -1, 0}, {0, 0, 0, 2}} {
		if _, err := EvaluateSample(m.Triangles[0], v[0], v[1], v[2], v[3]); err == nil {
			t.Fatal("bad sample accepted")
		}
	}
	if _, err := SampleBounds(MeshTriangle{}, 0, 0); err == nil {
		t.Fatal("zero triangle")
	}
}

func BenchmarkSamplesFast(b *testing.B) {
	tri := sampleTestTriangle([3][2]float64{{0, 0}, {20, 0}, {0, 20}}, [3]int32{3, 5, 7}, 3)
	b.ReportAllocs()
	b.ResetTimer()
	for k := 0; k < b.N; k++ {
		if _, err := EvaluateSample(tri, 1, 1, k&1, (k>>1)&1); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkSamplesBig(b *testing.B) {
	tri := sampleTestTriangle([3][2]float64{{-1e12, -1e12}, {1e12, -1e12}, {0, 1e12}}, [3]int32{3, 5, 7}, 3)
	b.ReportAllocs()
	b.ResetTimer()
	for k := 0; k < b.N; k++ {
		if _, err := EvaluateSample(tri, 1, 1, k&1, (k>>1)&1); err != nil {
			b.Fatal(err)
		}
	}
}
