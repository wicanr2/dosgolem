package realspace

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

func readI32(b []byte) int32 { return int32(binary.LittleEndian.Uint32(b)) }
func readI16(b []byte) int16 { return int16(binary.LittleEndian.Uint16(b)) }

// MatrixFromRecord preserves the row order X,Z,Y and 24.8 values of spec 013.
func MatrixFromRecord(raw [0x53]byte) (m [12]int32) {
	for i := range m {
		m[i] = readI32(raw[0x23+i*4:])
	}
	return m
}

// Transform returns X,Z,Y, retaining signed 64-bit accumulation, SHRD truncation,
// and the final 32-bit wrapping addition. Input vertices are ordered x,y,z.
func Transform(v [3]int32, m [12]int32) (xyz [3]int32) {
	for r := 0; r < 3; r++ {
		var sum int64
		for j := 0; j < 3; j++ {
			sum += int64(v[j]) * int64(m[r*4+j])
		}
		xyz[r] = int32(sum>>8) + m[r*4+3]
	}
	return xyz
}

// Project reproduces the original integer projection, including the near-plane
// no-division branch. It performs no HD projection or clipping (phase 2).
func Project(cam Camera, flag uint8, near int32, xyz [3]int32) (sx, sy int32, noDivide, clamped bool, err error) {
	x, z, y := xyz[0], xyz[1], xyz[2]
	cx, cy := int32(readI16(cam.Raw[8:])), int32(readI16(cam.Raw[10:]))
	fx, fy := readI32(cam.Raw[12:]), readI32(cam.Raw[16:])
	noDivide = flag == 0 && (z < near || z == 0)
	if noDivide {
		sx = cx + int32((int64(x)*int64(fx))>>16)
		sy = cy - int32((int64(y)*int64(fy))>>16)
	} else {
		if z == 0 || (flag != 0 && z < 0) {
			err = fmt.Errorf("Z nonpositive")
			return
		}
		qx, qy := (int64(x)*int64(fx))/int64(z), (int64(y)*int64(fy))/int64(z)
		if qx < -1<<31 || qx > 1<<31-1 || qy < -1<<31 || qy > 1<<31-1 {
			err = fmt.Errorf("division overflow")
			return
		}
		sx = (int32(qx) >> 8) + cx
		sy = cy - (int32(qy) >> 8)
	}
	cl := func(x int32) int32 {
		if x < -0x7d00 {
			clamped = true
			return -0x7d00
		}
		if x > 0x7d00 {
			clamped = true
			return 0x7d00
		}
		return x
	}
	sx, sy = cl(sx), cl(sy)
	return
}

// ApexInRange is a defensive bound, not a claim about the game's LNTH semantics.
func ApexInRange(e *ModelEntry, rt [][3]int32) bool {
	if e == nil || len(rt) != len(e.Verts) {
		return false
	}
	abs := func(y int32) int64 {
		if y < 0 {
			return -int64(y)
		}
		return int64(y)
	}
	var limit int64
	for _, i := range e.Apex {
		if i < 0 || i >= len(rt) {
			return false
		}
		limit = max(limit, abs(e.Verts[i][1]))
	}
	for i := 0; i+4 <= len(e.LNTH); i += 4 {
		limit = max(limit, int64(binary.LittleEndian.Uint32(e.LNTH[i:])))
	}
	for _, i := range e.Apex {
		if rt[i][1] >= 0 || abs(rt[i][1]) > limit {
			return false
		}
	}
	return true
}

// CheckObject is read-only. The recorder assigns its return value to obj.Verdict.
// Comparison counts are vertex groups (XYZ and sx/sy), including repeated vertices.
func CheckObject(lib *Library, cameras []Camera, obj *ObjObs) ObjVerdict {
	v := ObjVerdict{Status: Failed}
	fail := func(reason string) ObjVerdict { v.Reason = reason; return v }
	if obj == nil {
		return fail("missing object")
	}
	if obj.ReadFailed {
		return fail("read failed")
	}
	if obj.OutOfBounds {
		return fail("out of bounds")
	}
	if len(obj.Problems) > 0 {
		return fail(obj.Problems[0])
	}
	if obj.CamIdx < 0 || obj.CamIdx >= len(cameras) {
		return fail("no camera")
	}
	cam := cameras[obj.CamIdx]
	if obj.CamObj != cam.CamObj {
		return fail("camera mismatch")
	}
	if obj.FaceTableMismatch {
		return fail("face table mismatch")
	}
	nv := int(binary.LittleEndian.Uint16(obj.ModelHead[2:]))
	if nv < 1 || nv > 255 || len(obj.VertXYZ) != nv*12 {
		return fail("vertex count out of bounds")
	}
	if obj.NFaces < 1 || obj.NFaces > 1024 {
		return fail("face count out of bounds")
	}
	faceCountOffset := 0x42 + int(obj.LOD)*2
	if faceCountOffset+2 > len(obj.ModelHead) || int16(binary.LittleEndian.Uint16(obj.ModelHead[faceCountOffset:])) != obj.NFaces {
		return fail("face table mismatch")
	}
	if len(obj.FaceList) != int(obj.NFaces) || len(obj.Faces) != int(obj.NFaces) {
		return fail("F1 count mismatch")
	}
	if obj.FaceRetMismatch != 0 || obj.F1RUnpaired != 0 || obj.UnmatchedFrame != 0 {
		return fail("face return/frame mismatch")
	}
	triples := make([][3]int16, len(obj.Faces))
	fvts := make([]uint16, len(obj.Faces))
	for i, f := range obj.Faces {
		if int(f.SI) != i || !f.Returned {
			return fail("face order/return mismatch")
		}
		for _, idx := range f.VIdx {
			if idx < 0 || int(idx) >= nv {
				return fail("vertex index out of bounds")
			}
		}
		triples[i], fvts[i] = f.VIdx, f.FaceVT
		if _, known := faceClass[f.FaceVT]; !known {
			v.UnknownFaceVT++
		}
	}
	var kind ModelKind
	switch obj.RecVT {
	case 0x10c2:
		kind = POLY
	case 0x314a:
		kind = APPR
	default:
		v.Unidentified = true
		return fail("unknown record class")
	}
	if obj.VertSHA != sha256.Sum256(obj.VertXYZ) {
		return fail("vertex hash mismatch")
	}
	if kind == POLY {
		v.Candidates = lib.Identify(obj.VertSHA, int(obj.LOD), triples, fvts)
	} else {
		v.Candidates = lib.IdentifyAppr(obj.VertRT, int(obj.LOD), triples, fvts)
	}
	if len(v.Candidates) == 0 {
		v.Unidentified = true
		if len(lib.identify(obj.VertSHA, obj.VertRT, int(obj.LOD), triples, fvts, kind, false)) > 0 {
			return fail("face class mismatch")
		}
		return fail("unidentified model")
	}
	e := v.Candidates[0]
	v.Selected = e
	keys := map[[32]byte]bool{}
	for _, c := range v.Candidates {
		key := c.GeoKey
		if kind == APPR {
			key = c.AppKey
		}
		keys[key] = true
	}
	v.KeyCount = len(keys)
	v.MultiGeoKey = v.KeyCount > 1
	if kind == APPR && !ApexInRange(e, obj.VertRT) {
		v.Unidentified = true
		return fail("apex out of range")
	}
	verts := e.Verts
	if kind == APPR {
		verts = append([][3]int32(nil), e.Verts...)
		for _, i := range e.Apex {
			verts[i] = obj.VertRT[i]
		}
	}
	matrix := MatrixFromRecord(obj.RecRaw)
	covered := true
	record := func(face, vertex int, field string, want, got int64) bool {
		if want == got {
			return false
		}
		if v.First == nil {
			v.First = &Mismatch{face, vertex, field, want, got}
		}
		return true
	}
	var arithmeticErr string
	for fi, f := range obj.Faces {
		for vi, idx := range f.VIdx {
			xyz := Transform(verts[idx], matrix)
			p := f.Proj[vi]
			flags := p[17]
			if flags&8 != 0 {
				v.ClipFlag++
			}
			if flags&2 != 0 {
				v.XYZCompared++
				bad := false
				for j, field := range []string{"X", "Z", "Y"} {
					if record(fi, vi, field, int64(xyz[j]), int64(readI32(p[4+j*4:]))) {
						bad = true
					}
				}
				if bad {
					v.XYZMismatch++
				}
			} else if f.AL != 0 {
				covered = false
			}
			if flags&4 != 0 {
				sx, sy, near, clamp, err := Project(cam, obj.Flag17BC, obj.Near, xyz)
				if err != nil {
					if arithmeticErr == "" {
						arithmeticErr = err.Error()
					}
					continue
				}
				v.ScreenCompared++
				if near {
					v.NearNoDivide++
				}
				if clamp {
					v.Clamped++
				}
				bx := record(fi, vi, "sx", int64(sx), int64(readI16(p[:])))
				by := record(fi, vi, "sy", int64(sy), int64(readI16(p[2:])))
				if bx || by {
					v.ScreenMismatch++
				}
			}
		}
	}
	if arithmeticErr != "" {
		return fail(arithmeticErr)
	}
	if v.XYZMismatch != 0 || v.ScreenMismatch != 0 {
		return fail("projection mismatch")
	}
	if v.XYZCompared == 0 || v.ScreenCompared == 0 || !covered {
		v.Status = Unchecked
		v.Reason = "insufficient compared fields"
		return v
	}
	v.Status = Passed
	return v
}
