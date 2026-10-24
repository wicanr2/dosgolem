package hd3d

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

func readVertexY(b []byte, i int) uint16 { return binary.LittleEndian.Uint16(b[i*22+2:]) }

const strikeSHA = "883270cd0038fe571376202233b8abf5efb77889e27f4f0caf97dfab7e529cb5"
const horizonSHA = "8cdc17f401184e97c84a978dfee33c057ed8e01dce1e09a66c06267045896792"

func TablesFromEXE(exe []byte) (Tables, error) {
	sum := sha256.Sum256(exe)
	if hex.EncodeToString(sum[:]) != strikeSHA {
		return Tables{}, fmt.Errorf("STRIKE.EXE SHA-256 mismatch")
	}
	const base = 0x8200 + (0x56f1-0x0241)*16
	if len(exe) < base+0xd0 {
		return Tables{}, fmt.Errorf("R6 table out of bounds")
	}
	b := exe[base+8 : base+0xd0]
	sum = sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != horizonSHA {
		return Tables{}, fmt.Errorf("R6 table SHA-256 mismatch")
	}
	return Tables{R6Table: append([]byte(nil), b...)}, nil
}

type capturedSegment struct {
	capture Capture
	label   string
	last    int
}

func (c *context) capture(ref Capture, label string) (*capturedSegment, error) {
	if len(ref.Spans) == 0 {
		c.capShort++
		return nil, fmt.Errorf("%s missing capture", label)
	}
	for _, s := range ref.Spans {
		if len(s.Data) > 65536 {
			c.capShort++
			return nil, fmt.Errorf("%s span exceeds segment", label)
		}
	}
	return &capturedSegment{capture: ref, label: label}, nil
}
func (c *context) readCapture(s *capturedSegment, off uint16) byte {
	if s != nil {
		spans := s.capture.Spans
		if len(spans) > 0 {
			span := spans[s.last]
			delta := int(uint16(off - span.Off))
			if delta < len(span.Data) {
				return span.Data[delta]
			}
		}
		for i, span := range spans {
			delta := int(uint16(off - span.Off))
			if delta < len(span.Data) {
				s.last = i
				return span.Data[delta]
			}
		}
	}
	c.capShort++
	label := "missing capture"
	if s != nil {
		label = s.label
	}
	c.fault("%s 擷取不足 off=%04X", label, off)
	return 0
}
func checkTable(b []byte) error {
	if len(b) < 0xc00 {
		return fmt.Errorf("R5 13DA table requires C00h bytes")
	}
	return nil
}

func newContext(list []uint16, tables Tables) *context {
	c := &context{br: NewBranches(), list: list, r5preUninit: map[string]int{}}
	for i := range c.writer {
		c.writer[i] = 0xffff
	}
	if len(tables.R6Table) == 200 {
		c.r6seg = make([]byte, 0xd0)
		copy(c.r6seg[8:], tables.R6Table)
	}
	return c
}

// Replay paints one captured group. It never reads the original executable or
// machine. Valid means structurally repaintable; Checked requires Buf3D.
func Replay(group Group, tables Tables) Result {
	c := newContext(group.List, tables)
	if len(group.Calls) == 0 || group.Calls[0].Kind != R6 {
		c.faults = append(c.faults, "first call is not R6")
	}
	if len(group.List) < 2 {
		c.faults = append(c.faults, "captured line list missing")
	} else {
		runCalls(c, group.Calls, true)
	}
	r := finish(c, group.Calls)
	if len(group.Buf3D) == PixelCount {
		r.Checked = true
		for i, x := range r.Pixels {
			if r.Written[i] && x != group.Buf3D[i] {
				r.Diagnostics.PixelMismatch++
				if r.Diagnostics.FirstMismatch < 0 {
					r.Diagnostics.FirstMismatch = i
				}
			}
		}
	} else if len(group.Buf3D) != 0 {
		r.Diagnostics.Faults = append(r.Diagnostics.Faults, "Buf3D length mismatch")
		r.Valid = false
	}
	r.Repaintable = r.Valid && r.Checked && r.Diagnostics.PixelMismatch == 0
	return r
}

func runCalls(c *context, calls []Call, strict bool) {
	var desc [6]uint16
	var haveDesc bool
	seen := map[uint16]bool{}
	for i := range calls {
		k := &calls[i]
		c.call, c.seq, c.seg = k, k.Seq, k.Desc[0]
		if seen[k.Seq] || k.Seq == 0xffff {
			c.fault("duplicate/reserved call sequence")
			continue
		}
		seen[k.Seq] = true
		if k.ReadFailed {
			c.fault("capture read failed")
			continue
		}
		if k.Kind != BlackBox {
			if !haveDesc {
				desc, haveDesc = k.Desc, true
			} else if desc != k.Desc {
				c.fault("descriptor changed")
			}
		}
		c.beginCall()
		runCall(c, k)
		c.endCall()
		if strict && i == 0 && c.br.n["R6.B6z"] != 0 {
			c.fault("first R6 does not initialize viewport")
		}
	}
}

func runCall(c *context, k *Call) {
	defer func() {
		if value := recover(); value != nil {
			c.fault("raster input panic: %v", value)
		}
	}()
	if k.Kind == R5 || k.Kind == R6 {
		if k.DS != k.SS {
			c.fault("DS != SS")
			return
		}
	}
	switch k.Kind {
	case R1, R2, R3, R4:
		n := int(k.Params[1])
		if n < 1 || len(k.Verts) < n*22 {
			c.capShort++
			c.fault("vertex capture short n=%d", n)
			return
		}
		if n > 16 {
			c.note("n 超界 %d；1 倍照做", n)
		}
	case R5:
		if len(k.Verts) < 66 {
			c.capShort++
			c.fault("R5 vertex capture short")
			return
		}
		if readVertexY(k.Verts, 0) == readVertexY(k.Verts, 1) && readVertexY(k.Verts, 1) == readVertexY(k.Verts, 2) && !k.Aux.HasResid {
			c.fault("R5 flat triangle residual capture missing")
			return
		}
	}
	switch k.Kind {
	case R1:
		c.r12(k, 1)
	case R2:
		if len(k.Table) < 256 {
			c.capShort++
			c.fault("R2 table capture short")
			return
		}
		c.r12(k, 2)
	case R3:
		c.r3(k)
	case R4:
		c.r4(k)
	case R5:
		c.r5(k)
	case R6:
		if !k.Aux.HasFlags {
			c.fault("R6 FLAGS capture missing")
			return
		}
		if k.Aux.Flags&0x400 != 0 {
			c.fault("R6 DF=1")
			return
		}
		if len(c.r6seg) != 0xd0 {
			c.fault("R6 original table missing")
			return
		}
		c.r6(k)
	case BlackBox:
		for _, p := range k.Patch {
			if p.Index >= PixelCount {
				c.oob++
				continue
			}
			i := int(p.Index)
			c.buf[i], c.written[i], c.writer[i] = p.Value, true, k.Seq
			if k.Object {
				c.objectWritten[i] = true
			}
			c.callWrites++
		}
	default:
		c.fault("unknown raster kind %s", k.Kind)
	}
}

func finish(c *context, calls []Call) Result {
	r := Result{Pixels: c.buf, Writer: c.writer, Written: c.written, ObjectWritten: c.objectWritten, CallElements: map[uint16]FarPtr{}}
	for _, call := range calls {
		r.CallElements[call.Seq] = call.Element
	}
	d := &r.Diagnostics
	d.Faults = append([]string(nil), c.faults...)
	d.Notes = append([]string(nil), c.notes...)
	d.Uninitialized = append([]string(nil), c.uninit...)
	d.UninitializedAfterLast = append([]string(nil), c.uninitAfterLast...)
	d.OutOfBounds, d.CaptureShort, d.FirstMismatch = c.oob, c.capShort, -1
	d.R5FlatCalls, d.R5FlatWrites, d.R5FlatPostWrites, d.R5ResidualSeeded = c.r5flatCalls, c.r5flatWrites, c.r5flatPostWrites, c.r5residSeeded
	for _, written := range c.written {
		if !written {
			d.Unwritten++
		}
	}
	for _, id := range sortedBranchIDs() {
		b := branchIndex[id]
		d.Branches = append(d.Branches, BranchCount{b.Routine, b.ID, b.Addr, b.Desc, c.br.n[id]})
	}
	r.Valid = len(d.Faults) == 0 && len(d.Uninitialized) == 0 && d.OutOfBounds == 0 && d.CaptureShort == 0 && d.Unwritten == 0
	return r
}

// MakePatch replays captured nested raster calls over Before, then retains only
// After pixels not explained by those calls (012 §3.6). It does not infer pixels
// from an unknown R5 variant; any replay fault returns no patch.
func MakePatch(before, after []byte, nested []Call, list []uint16, tables Tables) ([]PixelSet, Diagnostics) {
	if len(before) != PixelCount || len(after) != PixelCount {
		return nil, Diagnostics{Faults: []string{"blackbox snapshot length mismatch"}, FirstMismatch: -1}
	}
	c := newContext(list, tables)
	copy(c.buf[:], before)
	for i := range c.written {
		c.written[i] = true
		c.writer[i] = 0xfffe
	}
	if len(nested) > 0 && len(list) < 2 {
		c.faults = append(c.faults, "captured line list missing")
	} else {
		runCalls(c, nested, false)
	}
	r := finish(c, nested)
	if !r.Valid {
		return nil, r.Diagnostics
	}
	var patch []PixelSet
	for i, x := range after {
		if c.buf[i] != x {
			patch = append(patch, PixelSet{uint32(i), x})
		}
	}
	return patch, r.Diagnostics
}
