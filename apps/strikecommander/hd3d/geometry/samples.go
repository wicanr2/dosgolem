package geometry

// 013a §5: four fixed samples on the same N-times Q8 triangle. This adds
// attribute provenance without changing ProjectFace, Target, or phase-2 B/C.
import (
	"fmt"
	"image"
	"math"
	"math/big"
)

const SampleRuleVersion = "013a-q8-q48-pa-first-four-v1"

type SourceKind uint8

const (
	SourceOriginal SourceKind = iota
	SourceNearEdge
)

// CornerSource records the original slot or original directed edge. N/D is
// measured from In toward Out, irrespective of the edge's traversal direction.
type CornerSource struct {
	Kind     SourceKind
	Original int
	Edge     [2]int
	In, Out  int
	N, D     uint64
}

// ClipVertex's exported descriptors are owned value copies. Its Q8 integers
// are immutable internally; Q8 returns fresh owned integers. Modifying a
// descriptor invalidates evaluation instead of silently changing its geometry.
type ClipVertex struct {
	Source     CornerSource
	Point      Point3
	Projection Projection
	qx, qy     *big.Int
}

func (v ClipVertex) Q8() (x, y *big.Int) {
	if v.qx == nil || v.qy == nil {
		return nil, nil
	}
	return new(big.Int).Set(v.qx), new(big.Int).Set(v.qy)
}

type FaceMesh struct {
	Input       FaceInput
	Scale       int
	RuleVersion string
	Info        FaceInfo
	Polygon     []ClipVertex
	Triangles   []MeshTriangle
}

type MeshTriangle struct {
	Scale          int
	Vertices       [3]ClipVertex
	Ref            FaceRef
	Fan            int
	PolygonIndices [3]int
	data           *sampleTriangleData
}

// SampleHit owns Area and all Edges, with no aliases into the mesh or another
// hit. Edges correspond to Vertices[SourceIndices[k]] after positive-area
// normalization. Uncovered hits have zero DepthQ48; material values are only
// meaningful when Coverage is true. Degenerate fans report Degenerate=true.
type SampleHit struct {
	Coverage, Degenerate bool
	Area                 *big.Int
	Edges                [3]*big.Int
	DepthQ48             uint64
	Owner                FaceRef
	Fan                  int
	SourceIndices        [3]int
}

// FastSampleHit contains only owned scalar values. Area/Edges are valid
// weights when Coverage is true; uncovered Edges are zero. The compact API
// uses the same primitive as EvaluateSample, without allocating big.Ints.
type FastSampleHit struct {
	Coverage, Degenerate bool
	Area                 uint64
	Edges                [3]uint64
	DepthQ48             uint64
	Owner                FaceRef
	Fan                  int
	SourceIndices        [3]int
}

type sampleTriangleData struct {
	vertices   [3]ClipVertex
	ref        FaceRef
	fan, scale int
	indices    [3]int
	order      [3]int
	q          [3]pointBig
	q64        [3]point64
	z          [3]uint64
	inclusive  [3]bool
	area       *big.Int
	area64     uint64
	fast       bool
}

func ProjectFaceSamples(f FaceInput, n int) (FaceMesh, error) {
	m := FaceMesh{Input: f, Scale: n, RuleVersion: SampleRuleVersion}
	if !validScale(n) || f.Near <= 0 {
		return FaceMesh{}, fmt.Errorf("geometry samples: invalid scale or Near")
	}
	for _, p := range f.Vertices {
		if p.Z < f.Near {
			if f.Flag17BC == 0 {
				m.Info.Clipped = true
			} else {
				m.Info.NearUnexpectedVertices++
			}
		}
		if f.Flag17BC != 0 && p.Z <= 0 {
			return FaceMesh{}, fmt.Errorf("geometry samples: unflagged clipping with nonpositive Z")
		}
	}
	m.Polygon = make([]ClipVertex, 0, 4)
	appendVertex := func(p Point3, source CornerSource) error {
		projection, err := ProjectPoint(p, f.Camera, n)
		if err != nil {
			return err
		}
		m.Polygon = append(m.Polygon, ClipVertex{Source: source, Point: p, Projection: projection, qx: quantize(projection.NX), qy: quantize(projection.NY)})
		return nil
	}
	for a, p := range f.Vertices {
		b := (a + 1) % 3
		inside := p.Z >= f.Near
		if !m.Info.Clipped || inside {
			if err := appendVertex(p, CornerSource{Kind: SourceOriginal, Original: a}); err != nil {
				return FaceMesh{}, err
			}
		}
		if m.Info.Clipped && inside != (f.Vertices[b].Z >= f.Near) {
			in, out := a, b
			if !inside {
				in, out = out, in
			}
			s := CornerSource{Kind: SourceNearEdge, Edge: [2]int{a, b}, In: in, Out: out,
				N: uint64(int64(f.Vertices[in].Z) - int64(f.Near)), D: uint64(int64(f.Vertices[in].Z) - int64(f.Vertices[out].Z))}
			if err := appendVertex(nearIntersection(p, f.Vertices[b], f.Near), s); err != nil {
				return FaceMesh{}, err
			}
		}
	}
	if len(m.Polygon) == 0 {
		m.Info.AllClipped = true
		return m, nil
	}
	ref := f.Ref
	ref.Clipped = m.Info.Clipped
	for k := 1; k+1 < len(m.Polygon); k++ {
		indices := [3]int{0, k, k + 1}
		tr := newSampleTriangle([3]ClipVertex{m.Polygon[0], m.Polygon[k], m.Polygon[k+1]}, ref, n, k-1, indices)
		m.Triangles = append(m.Triangles, tr)
	}
	return m, nil
}

// newSampleTriangle prepares only exact immutable geometric data. It does not
// reorder the public polygon or fan and is not an alternate projection API.
func newSampleTriangle(vertices [3]ClipVertex, ref FaceRef, n, fan int, indices [3]int) MeshTriangle {
	d := &sampleTriangleData{vertices: vertices, ref: ref, fan: fan, scale: n, indices: indices, order: [3]int{0, 1, 2}, fast: true}
	for k, v := range vertices {
		d.q[k] = pointBig{v.qx, v.qy}
		d.z[k] = v.Projection.InvZQ48
		if math.Abs(v.Projection.NX) > 1<<21 || math.Abs(v.Projection.NY) > 1<<21 {
			d.fast = false
		}
	}
	d.area = edgeBig(d.q[0], d.q[1], d.q[2])
	if d.area.Sign() < 0 {
		d.q[1], d.q[2] = d.q[2], d.q[1]
		d.z[1], d.z[2] = d.z[2], d.z[1]
		d.order = [3]int{0, 2, 1}
		d.area.Neg(d.area)
	}
	if d.fast {
		for k, p := range d.q {
			d.q64[k] = point64{p.x.Int64(), p.y.Int64()}
		}
		d.area64 = d.area.Uint64()
	}
	for k := range d.inclusive {
		d.inclusive[k] = leftBig(d.q[(k+1)%3], d.q[(k+2)%3])
	}
	return MeshTriangle{Scale: n, Vertices: vertices, Ref: ref, Fan: fan, PolygonIndices: indices, data: d}
}

func (t MeshTriangle) valid() bool {
	d := t.data
	if d == nil || !validScale(t.Scale) || t.Scale != d.scale || t.Ref != d.ref || t.Fan != d.fan || t.PolygonIndices != d.indices {
		return false
	}
	for k, v := range t.Vertices {
		w := d.vertices[k]
		if v.Source != w.Source || v.Point != w.Point || v.Projection != w.Projection || v.qx != w.qx || v.qy != w.qy {
			return false
		}
	}
	return true
}

// SampleBounds returns a half-open rectangle for one of the four phases.
// Bounds are calculated from original arbitrary-precision Q8 vertices and
// clamped to the viewport before conversion to machine integers.
func SampleBounds(t MeshTriangle, i, j int) (image.Rectangle, error) {
	if !t.valid() || i < 0 || i > 1 || j < 0 || j > 1 {
		return image.Rectangle{}, fmt.Errorf("geometry samples: invalid triangle or phase")
	}
	if t.data.area.Sign() == 0 {
		return image.Rectangle{}, nil
	}
	x0, x1 := sampleBound(t.data.q, false, 64+128*i, 320*t.Scale)
	y0, y1 := sampleBound(t.data.q, true, 64+128*j, 200*t.Scale)
	if x1 < x0 || y1 < y0 {
		return image.Rectangle{}, nil
	}
	return image.Rect(x0, y0, x1+1, y1+1), nil
}

func sampleBound(q [3]pointBig, y bool, phase, limit int) (int, int) {
	get := func(p pointBig) *big.Int {
		if y {
			return p.y
		}
		return p.x
	}
	lo, hi := new(big.Int).Set(get(q[0])), new(big.Int).Set(get(q[0]))
	for _, p := range q[1:] {
		v := get(p)
		if v.Cmp(lo) < 0 {
			lo.Set(v)
		}
		if v.Cmp(hi) > 0 {
			hi.Set(v)
		}
	}
	lo.Sub(lo, big.NewInt(int64(phase)))
	lo.Neg(lo)
	lo.Div(lo, big.NewInt(256))
	lo.Neg(lo)
	hi.Sub(hi, big.NewInt(int64(phase)))
	hi.Div(hi, big.NewInt(256))
	if lo.Sign() < 0 {
		lo.SetInt64(0)
	}
	if hi.Cmp(big.NewInt(int64(limit-1))) > 0 {
		hi.SetInt64(int64(limit - 1))
	}
	if lo.Cmp(hi) > 0 {
		return 0, -1
	}
	return int(lo.Int64()), int(hi.Int64())
}

func EvaluateSample(t MeshTriangle, x, y, i, j int) (SampleHit, error) {
	return evaluateSample(t, x, y, i, j, false)
}

// EvaluateSampleFast returns ok=false for a valid triangle requiring BigInt
// arithmetic. The caller then uses EvaluateSample. It never narrows an
// out-of-guard Q8 coordinate. Successful calls allocate no sample buffers.
func EvaluateSampleFast(t MeshTriangle, x, y, i, j int) (hit FastSampleHit, ok bool, err error) {
	if !validSample(t, x, y, i, j) {
		return FastSampleHit{}, false, fmt.Errorf("geometry samples: invalid triangle, pixel or phase")
	}
	if !t.data.fast && t.data.area.Sign() != 0 {
		return FastSampleHit{}, false, nil
	}
	hit, _ = sampleFastData(t, x, y, i, j)
	return hit, true, nil
}

func validSample(t MeshTriangle, x, y, i, j int) bool {
	return t.valid() && i >= 0 && i <= 1 && j >= 0 && j <= 1 && x >= 0 && y >= 0 && x < 320*t.Scale && y < 200*t.Scale
}

func sampleFastData(t MeshTriangle, x, y, i, j int) (FastSampleHit, [3]int64) {
	d := t.data
	h := FastSampleHit{Area: d.area64, Owner: t.Ref, Fan: t.Fan, SourceIndices: d.order, Degenerate: d.area.Sign() == 0}
	var e [3]int64
	if h.Degenerate {
		return h, e
	}
	p := point64{int64(256*x + 64 + 128*i), int64(256*y + 64 + 128*j)}
	inside := true
	for k := range e {
		e[k] = edge64(d.q64[(k+1)%3], d.q64[(k+2)%3], p)
		if e[k] < 0 || e[k] == 0 && !d.inclusive[k] {
			inside = false
		}
	}
	if inside {
		for k := range e {
			h.Edges[k] = uint64(e[k])
		}
		h.DepthQ48 = depth128(e, d.z, d.area64)
	}
	h.Coverage = inside
	return h, e
}

func evaluateSample(t MeshTriangle, x, y, i, j int, forceBig bool) (SampleHit, error) {
	if !validSample(t, x, y, i, j) {
		return SampleHit{}, fmt.Errorf("geometry samples: invalid triangle, pixel or phase")
	}
	d := t.data
	h := SampleHit{Area: new(big.Int).Set(d.area), Owner: t.Ref, Fan: t.Fan, SourceIndices: d.order, Degenerate: d.area.Sign() == 0}
	if h.Degenerate {
		return h, nil
	}
	inside := true
	if d.fast && !forceBig {
		compact, e := sampleFastData(t, x, y, i, j)
		inside = compact.Coverage
		h.DepthQ48 = compact.DepthQ48
		for k := range e {
			h.Edges[k] = big.NewInt(e[k])
		}
	} else {
		p := pointBig{big.NewInt(int64(256*x + 64 + 128*i)), big.NewInt(int64(256*y + 64 + 128*j))}
		sum := new(big.Int)
		for k := range h.Edges {
			e := edgeBig(d.q[(k+1)%3], d.q[(k+2)%3], p)
			h.Edges[k] = e
			if e.Sign() < 0 || e.Sign() == 0 && !d.inclusive[k] {
				inside = false
			}
			sum.Add(sum, new(big.Int).Mul(e, new(big.Int).SetUint64(d.z[k])))
		}
		if inside {
			h.DepthQ48 = sum.Quo(sum, d.area).Uint64()
		}
	}
	h.Coverage = inside
	return h, nil
}
