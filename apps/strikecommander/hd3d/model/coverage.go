package model

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

// Fixed by 013 §3.8.4. Learning this set from the current E1 links would make
// Unlinked tautologically zero, so classification never uses E1 membership.
func objectVT(v uint16) bool {
	switch v {
	case 0x17c1, 0x17c5, 0x17c9, 0x17cd, 0x17d1, 0x17d5, 0x17d9:
		return true
	}
	return false
}

type pixelOwner struct {
	faceID
	present bool
}

func cUnavailable(r *Result, code, message string) {
	r.Diagnostics.CUnavailable = append(r.Diagnostics.CUnavailable, message)
	r.Diagnostics.Problems = append(r.Diagnostics.Problems, Problem{Code: code, Message: message, Object: -1, Face: -1, Call: -1, Pixel: -1})
}

func measureC(r *Result, in Input, tables hd3d.Tables, props map[faceID]faceProperty) {
	orig := in.Original
	if orig == nil || !orig.Valid || !orig.Checked || !orig.Repaintable {
		cUnavailable(r, "no-writer", "no valid checked 1x replay/writer evidence")
		return
	}
	if in.Raster.Key != in.Key || len(in.Raster.Buf3D) != hd3d.PixelCount {
		cUnavailable(r, "group-identity", "raster key or Buf3D length differs from captured group")
		return
	}
	headers := map[hd3d.FarPtr]uint16{}
	for _, e := range in.Elements {
		vt := binary.LittleEndian.Uint16(e.Header[:2])
		if prior, ok := headers[e.Pointer]; ok && prior != vt {
			cUnavailable(r, "vt-conflict", fmt.Sprintf("R10 element %04X:%04X has multiple raw vtables", e.Pointer.Seg, e.Pointer.Off))
			return
		}
		headers[e.Pointer] = vt
	}
	links := map[hd3d.FarPtr]faceID{}
	for oi, o := range in.Objects {
		for fi, f := range o.Faces {
			p := hd3d.FarPtr{Off: f.Elem.Off, Seg: f.Elem.Seg}
			if f.AL == 0 || p == (hd3d.FarPtr{}) {
				continue
			}
			id := faceID{uint32(oi), uint32(fi)}
			if prior, ok := links[p]; ok && prior != id {
				cUnavailable(r, "e1-conflict", fmt.Sprintf("E1 element %04X:%04X has multiple object/face owners", p.Seg, p.Off))
				return
			}
			links[p] = id
		}
	}
	// Replay only a shallow call copy: Object is a value field, all captured
	// buffers remain read-only. Replay's output owns fresh pixel/writer buffers.
	calls := append([]hd3d.Call(nil), in.Raster.Calls...)
	callBySeq := map[uint16]int{}
	for i := range calls {
		c := &calls[i]
		if c.Seq == 0xffff || int(c.Seq) != i {
			cUnavailable(r, "call-sequence", "captured call sequences are not contiguous 0..65534")
			return
		}
		callBySeq[c.Seq] = i
		vt, ok := headers[c.Element]
		if c.Element != (hd3d.FarPtr{}) && !ok {
			cUnavailable(r, "missing-header", fmt.Sprintf("call %d has no raw R10 element header", c.Seq))
			return
		}
		c.Object = c.Element != (hd3d.FarPtr{}) && ok && objectVT(vt)
		if c.Object {
			r.Diagnostics.ClassifiedObjectCalls++
		}
		if p, ok := orig.CallElements[c.Seq]; !ok || p != c.Element {
			cUnavailable(r, "call-element", fmt.Sprintf("call %d disagrees with captured writer's element metadata", c.Seq))
			return
		}
	}
	if len(orig.CallElements) != len(calls) {
		cUnavailable(r, "call-element", "writer element map has missing or extra calls")
		return
	}
	for i, seq := range orig.Writer {
		_, exists := callBySeq[seq]
		if !orig.Written[i] || seq == 0xffff || !exists || orig.Pixels[i] != in.Raster.Buf3D[i] {
			cUnavailable(r, "invalid-writer", fmt.Sprintf("pixel %d lacks valid writer/Buf3D evidence", i))
			return
		}
	}
	group := in.Raster
	group.Calls = calls
	replayed := hd3d.Replay(group, tables)
	r.Diagnostics.ReplayPixelsEqual = replayed.Pixels == orig.Pixels
	r.Diagnostics.ReplayWriterEqual = replayed.Writer == orig.Writer
	if !replayed.Valid || !replayed.Checked || !replayed.Repaintable || !r.Diagnostics.ReplayPixelsEqual || !r.Diagnostics.ReplayWriterEqual {
		cUnavailable(r, "classification-replay", fmt.Sprintf("classified replay failed: valid=%t checked=%t repaintable=%t pixels=%t writer=%t faults=%v", replayed.Valid, replayed.Checked, replayed.Repaintable, r.Diagnostics.ReplayPixelsEqual, r.Diagnostics.ReplayWriterEqual, replayed.Diagnostics.Faults))
		return
	}
	old := make([]pixelOwner, hd3d.PixelCount)
	for i, seq := range replayed.Writer {
		c := calls[callBySeq[seq]]
		id, linked := links[c.Element]
		if c.Object {
			if !linked {
				r.C.Unlinked++
			} else {
				old[i] = pixelOwner{id, true}
			}
		} else if linked {
			r.C.LinkedOutsideSet++
		}
	}
	r.C.Available = true
	classifyCoverage(r, old, replayed.ObjectWritten[:], props)
	r.C.Passed = r.C.Unlinked == 0 && r.C.LinkedOutsideSet == 0 && r.C.C2OutsideBand == 0
}

func classifyCoverage(r *Result, old []pixelOwner, ever []bool, props map[faceID]faceProperty) {
	n := r.Frame.Scale
	hd := make([]pixelOwner, hd3d.PixelCount)
	offset := (n - 1) / 2
	for y := 0; y < hd3d.Height; y++ {
		for x := 0; x < hd3d.Width; x++ {
			i := (y*n+offset)*r.Frame.Width + x*n + offset
			if r.Frame.DepthQ48[i] != 0 {
				ref := r.Frame.Owner[i]
				hd[y*hd3d.Width+x] = pixelOwner{faceID{ref.Object, ref.Face}, true}
			}
		}
	}
	band := boundary(old, hd)
	for i, a := range old {
		b := hd[i]
		if band[i] {
			r.C.BoundaryPixels++
		}
		if a.present && r.Objects[a.object].Status != realspace.Passed {
			r.C.ObjectNotDrawn++
			continue
		}
		if a.present && props[a.faceID].clipped || b.present && props[b.faceID].clipped {
			r.C.C5++
			continue
		}
		if a.present && props[a.faceID].clamped {
			r.C.C6++
			continue
		}
		switch {
		case a.present && b.present:
			if a.object == b.object {
				r.C.C1++
			} else {
				r.C.C4++
			}
		case a.present:
			r.C.C2++
			if !band[i] {
				r.C.C2OutsideBand++
			}
		case b.present:
			if ever[i] {
				r.C.C3a++
			} else {
				r.C.C3b++
			}
		default:
			r.C.Neither++
		}
	}
}

func boundary(old, hd []pixelOwner) []bool {
	band := make([]bool, hd3d.PixelCount)
	same := func(a, b pixelOwner) bool { return a.present == b.present && (!a.present || a.object == b.object) }
	for y := 0; y < hd3d.Height; y++ {
		for x := 0; x < hd3d.Width; x++ {
			i := y*hd3d.Width + x
			for yy := max(0, y-1); yy <= min(hd3d.Height-1, y+1) && !band[i]; yy++ {
				for xx := max(0, x-1); xx <= min(hd3d.Width-1, x+1); xx++ {
					j := yy*hd3d.Width + xx
					if !same(old[i], old[j]) || !same(hd[i], hd[j]) {
						band[i] = true
						break
					}
				}
			}
		}
	}
	return band
}
