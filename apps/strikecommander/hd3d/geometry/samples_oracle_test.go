package geometry

// The sample oracle is defined before the production implementation. It uses
// homogeneous line equations and rational clipping, never a production edge,
// quantizer, bounds, clip result or SampleHit as its expectation.
import (
	"math/big"
	"testing"
)

type sampleOracle struct {
	q      [3][2]*big.Int
	z      [3]uint64
	order  [3]int
	area   *big.Int
	lines  [3][3]*big.Int
	closed [3]bool
}

func sampleOracleRound(v float64) *big.Int {
	r := new(big.Rat).SetFloat64(v)
	r.Mul(r, big.NewRat(256, 1))
	floor := new(big.Int).Div(r.Num(), r.Denom())
	f := new(big.Rat).Sub(r, new(big.Rat).SetInt(floor))
	if c := f.Cmp(big.NewRat(1, 2)); c > 0 || c == 0 && floor.Bit(0) == 1 {
		floor.Add(floor, big.NewInt(1))
	}
	return floor
}

// Homogeneous line ax+by+c=0; substituting a point evaluates its side.
func sampleOracleLine(a, b [2]*big.Int) [3]*big.Int {
	return [3]*big.Int{
		new(big.Int).Sub(a[1], b[1]),
		new(big.Int).Sub(b[0], a[0]),
		new(big.Int).Sub(new(big.Int).Mul(a[0], b[1]), new(big.Int).Mul(a[1], b[0])),
	}
}
func sampleOracleSide(l [3]*big.Int, p [2]*big.Int) *big.Int {
	r := new(big.Int).Mul(l[0], p[0])
	r.Add(r, new(big.Int).Mul(l[1], p[1]))
	return r.Add(r, l[2])
}
func newSampleOracle(xy [3][2]float64, z [3]int32) sampleOracle {
	o := sampleOracle{order: [3]int{0, 1, 2}}
	for k := range xy {
		o.q[k] = [2]*big.Int{sampleOracleRound(xy[k][0]), sampleOracleRound(xy[k][1])}
		o.z[k] = (uint64(1) << 48) / uint64(z[k])
	}
	o.area = sampleOracleSide(sampleOracleLine(o.q[0], o.q[1]), o.q[2])
	if o.area.Sign() < 0 {
		o.q[1], o.q[2] = o.q[2], o.q[1]
		o.z[1], o.z[2] = o.z[2], o.z[1]
		o.order = [3]int{0, 2, 1}
		o.area.Neg(o.area)
	}
	for k := range o.lines {
		a, b := o.q[(k+1)%3], o.q[(k+2)%3]
		o.lines[k] = sampleOracleLine(a, b)
		o.closed[k] = b[1].Cmp(a[1]) < 0 || b[1].Cmp(a[1]) == 0 && b[0].Cmp(a[0]) > 0
	}
	return o
}
func (o sampleOracle) at(x, y, i, j int) (bool, uint64, [3]*big.Int) {
	var e [3]*big.Int
	if o.area.Sign() == 0 {
		return false, 0, e
	}
	p := [2]*big.Int{big.NewInt(int64(256*x + 64 + 128*i)), big.NewInt(int64(256*y + 64 + 128*j))}
	sum := new(big.Int)
	inside := true
	for k := range e {
		e[k] = sampleOracleSide(o.lines[k], p)
		inside = inside && (e[k].Sign() > 0 || e[k].Sign() == 0 && o.closed[k])
		sum.Add(sum, new(big.Int).Mul(e[k], new(big.Int).SetUint64(o.z[k])))
	}
	if !inside {
		return false, 0, e
	}
	return true, sum.Quo(sum, o.area).Uint64(), e
}

// Independent exact point on the line, followed by the required displacement
// truncation relative to the inside endpoint. This deliberately differs from
// the production bits128 integer multiply/divide implementation.
func sampleOracleClip(in, out Point3, near int32) Point3 {
	t := new(big.Rat).SetFrac(big.NewInt(int64(near)-int64(out.Z)), big.NewInt(int64(in.Z)-int64(out.Z)))
	axis := func(a, b int32) int32 {
		exact := new(big.Rat).Add(new(big.Rat).SetInt64(int64(b)), new(big.Rat).Mul(new(big.Rat).SetInt64(int64(a)-int64(b)), t))
		displacement := new(big.Rat).Sub(new(big.Rat).SetInt64(int64(a)), exact)
		q := new(big.Int).Quo(displacement.Num(), displacement.Denom())
		return int32(int64(a) - q.Int64())
	}
	return Point3{X: axis(in.X, out.X), Z: near, Y: axis(in.Y, out.Y)}
}

func checkSampleOracle(t *testing.T, tri MeshTriangle, o sampleOracle, x, y, i, j int, forceBig bool) {
	t.Helper()
	got, err := evaluateSample(tri, x, y, i, j, forceBig)
	if err != nil {
		t.Fatal(err)
	}
	want, depth, edges := o.at(x, y, i, j)
	compact, ok, compactErr := EvaluateSampleFast(tri, x, y, i, j)
	if compactErr != nil {
		t.Fatal(compactErr)
	}
	if ok {
		if compact.Coverage != want || compact.DepthQ48 != depth || compact.SourceIndices != o.order {
			t.Fatal("compact sample differs from independent oracle")
		}
		if want {
			if compact.Area != o.area.Uint64() {
				t.Fatal("compact area")
			}
			for k := range edges {
				if compact.Edges[k] != edges[k].Uint64() {
					t.Fatal("compact edge", k)
				}
			}
		}
	}
	if got.Coverage != want || got.DepthQ48 != depth {
		t.Fatalf("sample (%d,%d)/%d,%d got %t/%d want %t/%d", x, y, i, j, got.Coverage, got.DepthQ48, want, depth)
	}
	if want {
		if got.Area.Cmp(o.area) != 0 || got.SourceIndices != o.order {
			t.Fatal("area/source ordering", got.SourceIndices, o.order)
		}
		for k := range edges {
			if got.Edges[k].Cmp(edges[k]) != 0 {
				t.Fatal("edge", k, got.Edges[k], edges[k])
			}
		}
	}
}
