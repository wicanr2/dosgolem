package geometry

import (
	"fmt"
	"math"
	"math/bits"
)

func Transform(v ModelPoint, m Matrix) Point3 {
	a := [3]int32{v.X, v.Y, v.Z}
	var r [3]int32
	for i := 0; i < 3; i++ {
		var sum uint64
		for j := 0; j < 3; j++ {
			sum += uint64(int64(a[j]) * int64(m[i*4+j]))
		}
		r[i] = int32((int64(sum) >> 8) + int64(m[i*4+3]))
	}
	return Point3{X: r[0], Z: r[1], Y: r[2]}
}

func validScale(n int) bool { return n == 2 || n == 3 }
func finite(x float64) bool { return !math.IsInf(x, 0) && !math.IsNaN(x) }

func ProjectPoint(p Point3, c Camera, n int) (Projection, error) {
	if !validScale(n) || p.Z <= 0 {
		return Projection{}, fmt.Errorf("geometry: scale must be 2/3 and Z positive")
	}
	px := float64(float64(p.X) * float64(c.FX))
	py := float64(float64(p.Y) * float64(c.FY))
	den := float64(float64(256) * float64(p.Z))
	ux := float64(px / den)
	uy := float64(py / den)
	qx := float64(float64(c.CX) + ux)
	qy := float64(float64(int64(c.CY)+1) - uy)
	nx := float64(float64(n) * qx)
	ny := float64(float64(n) * qy)
	if !finite(nx) || !finite(ny) {
		return Projection{}, fmt.Errorf("geometry: nonfinite projection")
	}
	return Projection{UX: ux, UY: uy, QX: qx, QY: qy, NX: nx, NY: ny, Z: p.Z, InvZQ48: (uint64(1) << 48) / uint64(p.Z)}, nil
}

// nearIntersection uses the inside endpoint in either traversal direction.
// Original 24.8 camera coordinates are preserved (013 §3.10.2).
func nearIntersection(a, b Point3, near int32) Point3 {
	if a.Z < near {
		a, b = b, a
	}
	d := uint64(int64(a.Z) - int64(b.Z))
	n := uint64(int64(a.Z) - int64(near))
	lerp := func(x, y int32) int32 {
		diff := int64(x) - int64(y)
		neg := diff < 0
		if neg {
			diff = -diff
		}
		hi, lo := bits.Mul64(uint64(diff), n)
		q, _ := bits.Div64(hi, lo, d)
		v := int64(q)
		if neg {
			v = -v
		}
		return int32(int64(x) - v)
	}
	return Point3{X: lerp(a.X, b.X), Z: near, Y: lerp(a.Y, b.Y)}
}

func ProjectFace(f FaceInput, n int) ([]Triangle, FaceInfo, error) {
	var info FaceInfo
	if !validScale(n) || f.Near <= 0 {
		return nil, info, fmt.Errorf("geometry: invalid scale or Near")
	}
	for _, p := range f.Vertices {
		if p.Z < f.Near {
			if f.Flag17BC == 0 {
				info.Clipped = true
			} else {
				info.NearUnexpectedVertices++
			}
		}
		if f.Flag17BC != 0 && p.Z <= 0 {
			return nil, info, fmt.Errorf("geometry: unflagged clipping with nonpositive Z")
		}
	}
	polygon := make([]Point3, 0, 4)
	if info.Clipped {
		// Pa-first, fixed P0->P1->P2->P0 start, including plane duplicates.
		for i, a := range f.Vertices {
			b := f.Vertices[(i+1)%3]
			ai, bi := a.Z >= f.Near, b.Z >= f.Near
			if ai {
				polygon = append(polygon, a)
			}
			if ai != bi {
				polygon = append(polygon, nearIntersection(a, b, f.Near))
			}
		}
	} else {
		polygon = append(polygon, f.Vertices[:]...)
	}
	if len(polygon) == 0 {
		info.AllClipped = true
		return nil, info, nil
	}
	var projected [4]Projection
	for i, p := range polygon {
		q, err := ProjectPoint(p, f.Camera, n)
		if err != nil {
			return nil, info, err
		}
		projected[i] = q
	}
	ref := f.Ref
	ref.Clipped = info.Clipped
	out := make([]Triangle, 0, len(polygon)-2)
	for i := 1; i+1 < len(polygon); i++ {
		out = append(out, Triangle{Vertices: [3]Projection{projected[0], projected[i], projected[i+1]}, Ref: ref, WireColor: f.WireColor})
	}
	return out, info, nil
}
