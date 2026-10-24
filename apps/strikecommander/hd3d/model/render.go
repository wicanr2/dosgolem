package model

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"math"
	"math/big"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

type faceID struct{ object, face uint32 }
type faceProperty struct{ clipped, clamped bool }
type bKey struct {
	x, z, y int32
	sx, sy  int16
}

func camera(c realspace.Camera) geometry.Camera {
	return geometry.Camera{CX: int16(binary.LittleEndian.Uint16(c.Raw[8:])), CY: int16(binary.LittleEndian.Uint16(c.Raw[10:])), FX: int32(binary.LittleEndian.Uint32(c.Raw[12:])), FY: int32(binary.LittleEndian.Uint32(c.Raw[16:]))}
}

func wireColor(e *realspace.ModelEntry) color.RGBA {
	k := e.GeoKey
	if e.Kind == realspace.APPR {
		k = e.AppKey
	}
	return color.RGBA{64 + k[0]%192, 64 + k[1]%192, 64 + k[2]%192, 255}
}

func runtimeBytesMatch(o *realspace.ObjObs) bool {
	if len(o.VertXYZ) != len(o.VertRT)*12 {
		return false
	}
	for i, v := range o.VertRT {
		for j, n := range v {
			if int32(binary.LittleEndian.Uint32(o.VertXYZ[i*12+j*4:])) != n {
				return false
			}
		}
	}
	return true
}

// Render does not trust obj.Verdict or its serialized Selected geometry. Every
// object is reidentified against lib before its original-file vertices are used.
// Missing raster evidence leaves C unavailable, while geometry/B remain usable
// for the explicitly limited legacy-near diagnostic path.
func Render(in Input, lib *realspace.Library, tables hd3d.Tables, scale int) (Result, error) {
	r := Result{Key: in.Key, Scale: scale, B: BMetrics{ErrorsX: map[int64]uint64{}, ErrorsY: map[int64]uint64{}}}
	if lib == nil {
		return r, fmt.Errorf("model: missing original-file library")
	}
	target, err := geometry.NewTarget(scale)
	if err != nil {
		return r, err
	}
	props := map[faceID]faceProperty{}
	seen := map[bKey]bool{}
	allObjects := true
	for oi := range in.Objects {
		o := &in.Objects[oi]
		if o.Near <= 0 {
			return r, fmt.Errorf("model: object %d has nonpositive Near", oi)
		}
		v := realspace.CheckObject(lib, in.Cameras, o)
		// RecRaw is the captured source for these typed fields. Serialized
		// summaries cannot substitute a different record class or LOD.
		if !o.ReadFailed {
			if o.RecVT != binary.LittleEndian.Uint16(o.RecRaw[:2]) {
				v.Status, v.Reason = realspace.Failed, "record class metadata mismatch"
			} else if o.LOD != o.RecRaw[9] {
				v.Status, v.Reason = realspace.Failed, "record LOD metadata mismatch"
			}
		}
		if v.Status == realspace.Passed && o.RecVT == 0x314a && !runtimeBytesMatch(o) {
			v.Status, v.Reason = realspace.Failed, "runtime vertex bytes mismatch"
		}
		report := ObjectReport{Object: uint32(oi), Status: v.Status, Reason: v.Reason, LOD: o.LOD, CandidateCount: len(v.Candidates), KeyCount: v.KeyCount, XYZCompared: v.XYZCompared, ScreenCompared: v.ScreenCompared, Faces: uint64(len(o.Faces)), Backface: BackfaceMetrics{AbsDot: map[string]uint64{}}}
		if v.Status != realspace.Passed || v.Selected == nil {
			allObjects = false
			r.B.SkippedObjects++
			r.Objects = append(r.Objects, report)
			continue
		}
		e := v.Selected
		report.TRE, report.Name, report.Kind, report.GeoKey, report.AppKey = e.TRE, e.Name, e.Kind, e.GeoKey, e.AppKey
		m := geometry.Matrix(realspace.MatrixFromRecord(o.RecRaw))
		points := make([]geometry.Point3, len(e.Verts))
		apex := map[int]bool{}
		if e.Kind == realspace.APPR {
			for _, idx := range e.Apex {
				apex[idx] = true
			}
		}
		for i, p := range e.Verts {
			if apex[i] {
				p = o.VertRT[i]
			}
			points[i] = geometry.Transform(geometry.ModelPoint{X: p[0], Y: p[1], Z: p[2]}, m)
		}
		cam := camera(in.Cameras[o.CamIdx])
		for fi, f := range o.Faces {
			// CheckObject has checked SI, FaceList, original LOD/VTRI and indices.
			indices := e.VTRI[e.LOD[o.LOD][fi]]
			var p [3]geometry.Point3
			prop := faceProperty{}
			for j := range p {
				p[j] = points[indices[j+1]]
				prop.clipped = prop.clipped || (o.Flag17BC == 0 && p[j].Z < o.Near)
				raw := f.Proj[j]
				sx, sy := int16(binary.LittleEndian.Uint16(raw[:])), int16(binary.LittleEndian.Uint16(raw[2:]))
				clamp := raw[17]&4 != 0 && (sx == 0x7d00 || sx == -0x7d00 || sy == 0x7d00 || sy == -0x7d00)
				prop.clamped = prop.clamped || clamp
				if clamp {
					report.ClampedVertices++
					ux, uy := originalUnclamped(p[j], cam, o.Flag17BC, o.Near)
					if (sx == 0x7d00 || sx == -0x7d00) && (ux == 0x7d00 || ux == -0x7d00) || (sy == 0x7d00 || sy == -0x7d00) && (uy == 0x7d00 || uy == -0x7d00) {
						report.ClampExactVertices++
					}
				}
				if err := recordB(&r.B, seen, p[j], cam, o, f, oi, fi, j, scale, clamp); err != nil {
					return r, err
				}
			}
			props[faceID{uint32(oi), uint32(fi)}] = prop
			if prop.clipped {
				report.ClippedFaces++
			}
			if prop.clamped {
				report.ClampedFaces++
			}
			recordBackface(&report.Backface, p, f.AL != 0)
			if f.AL == 0 {
				continue
			}
			report.VisibleFaces++
			tris, info, err := geometry.ProjectFace(geometry.FaceInput{Vertices: p, Camera: cam, Flag17BC: o.Flag17BC, Near: o.Near, Ref: geometry.FaceRef{Object: uint32(oi), Face: uint32(fi), Clipped: prop.clipped, Clamped: prop.clamped}, WireColor: wireColor(e)}, scale)
			if err != nil {
				return r, fmt.Errorf("model: object %d face %d: %w", oi, fi, err)
			}
			report.NearUnexpectedVertices += uint64(info.NearUnexpectedVertices)
			if info.AllClipped {
				report.AllClippedFaces++
			}
			for _, tri := range tris {
				if err := target.Draw(tri); err != nil {
					return r, fmt.Errorf("model: object %d face %d: %w", oi, fi, err)
				}
				report.Triangles++
			}
		}
		report.Drawn = report.Triangles != 0
		r.Objects = append(r.Objects, report)
	}
	r.Frame = target.Frame()
	r.B.Applicable = r.B.Eligible > 0
	r.B.Passed = r.B.HardFailureX == 0 && r.B.HardFailureY == 0
	if r.B.Applicable {
		r.B.ZeroXRatio = float64(r.B.ZeroX) / float64(r.B.Eligible)
		r.B.ZeroYRatio = float64(r.B.ZeroY) / float64(r.B.Eligible)
	}
	measureC(&r, in, tables, props)
	r.Passed = allObjects && r.B.Passed && r.C.Available && r.C.Passed
	return r, nil
}

func originalUnclamped(p geometry.Point3, c geometry.Camera, flag uint8, near int32) (int32, int32) {
	if flag == 0 && (p.Z < near || p.Z == 0) {
		return int32(c.CX) + int32((int64(p.X)*int64(c.FX))>>16), int32(c.CY) - int32((int64(p.Y)*int64(c.FY))>>16)
	}
	// Only called after CheckObject has checked the original division domain.
	return (int32((int64(p.X)*int64(c.FX))/int64(p.Z)) >> 8) + int32(c.CX), int32(c.CY) - (int32((int64(p.Y)*int64(c.FY))/int64(p.Z)) >> 8)
}

func recordB(b *BMetrics, seen map[bKey]bool, p geometry.Point3, cam geometry.Camera, o *realspace.ObjObs, f realspace.FaceObs, oi, fi, vi, scale int, clamp bool) error {
	raw := f.Proj[vi]
	if raw[17]&4 == 0 {
		b.NotProjected++
		return nil
	}
	noDivide := o.Flag17BC == 0 && (p.Z < o.Near || p.Z == 0)
	if noDivide {
		b.ExcludedNoDivide++
	}
	if clamp {
		b.ExcludedClamped++
	}
	if noDivide || clamp {
		return nil
	}
	sx, sy := int16(binary.LittleEndian.Uint16(raw[:])), int16(binary.LittleEndian.Uint16(raw[2:]))
	k := bKey{p.X, p.Z, p.Y, sx, sy}
	if seen[k] {
		b.Duplicates++
		return nil
	}
	seen[k] = true
	q, err := geometry.ProjectPoint(p, cam, scale)
	if err != nil {
		return fmt.Errorf("model B object %d face %d vertex %d: %w", oi, fi, vi, err)
	}
	ex, ey := int64(math.Floor(q.QX))-int64(sx), int64(math.Floor(q.QY))-int64(sy)
	b.Eligible++
	b.ErrorsX[ex]++
	b.ErrorsY[ey]++
	if ex == 0 {
		b.ZeroX++
	}
	if ey == 0 {
		b.ZeroY++
	}
	for _, axis := range []struct {
		name string
		err  int64
		u, q float64
		bad  bool
	}{{"x", ex, q.UX, q.QX, ex < -1 || ex > 0}, {"y", ey, q.UY, q.QY, ey < 0 || ey > 1}} {
		if !axis.bad {
			continue
		}
		x := float64(256 * axis.u)
		ax := math.Abs(x)
		ulp := math.Nextafter(ax, math.Inf(1)) - ax
		if math.Abs(x-math.RoundToEven(x)) <= ulp {
			b.ULPExceptions = append(b.ULPExceptions, BException{uint32(oi), uint32(fi), vi, axis.name, axis.err, axis.u, axis.q})
		} else if axis.name == "x" {
			b.HardFailureX++
		} else {
			b.HardFailureY++
		}
	}
	return nil
}

func recordBackface(b *BackfaceMetrics, p [3]geometry.Point3, visible bool) {
	// Diagnostic geometric normal in conventional XYZ camera coordinates.
	vec := func(p geometry.Point3) [3]int64 { return [3]int64{int64(p.X), int64(p.Y), int64(p.Z)} }
	a, c, d := vec(p[0]), vec(p[1]), vec(p[2])
	var u, v [3]int64
	for i := range u {
		u[i], v[i] = c[i]-a[i], d[i]-a[i]
	}
	dot := new(big.Int)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		n := new(big.Int).Sub(new(big.Int).Mul(big.NewInt(u[j]), big.NewInt(v[k])), new(big.Int).Mul(big.NewInt(u[k]), big.NewInt(v[j])))
		dot.Add(dot, n.Mul(n, big.NewInt(a[i])))
	}
	idx := dot.Sign() + 1
	if visible {
		b.ALNonzeroSigns[idx]++
	} else {
		b.ALZeroSigns[idx]++
	}
	if (dot.Sign() < 0) != visible {
		b.Mismatch++
	}
	b.AbsDot[new(big.Int).Abs(dot).String()]++
}
