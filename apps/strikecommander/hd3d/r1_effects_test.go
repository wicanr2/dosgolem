package hd3d

import (
	"encoding/binary"
	"reflect"
	"testing"
	"unsafe"
)

func r1EffectsTestInput() R1EffectsInput {
	var in R1EffectsInput
	in.ProgramSHA256 = [32]byte{0xf3, 0x6e, 0x6b, 0xc5, 0x07, 0x78, 0x88, 0x20, 0x99, 0xbf, 0xff, 0x4a, 0x43, 0xaf, 0xe0, 0x63, 0x35, 0xfa, 0x3a, 0x41, 0xf5, 0xb0, 0x60, 0xab, 0x67, 0xa1, 0xe8, 0x4e, 0x81, 0x6e, 0xa7, 0xdd}
	in.CPU = R1EffectsCPU{GPR: [8]uint32{0x009cd02c, 0x00480000, 0x005a0000, 0x00a0d026, 0x1234f000, 0x5678fe00, 0x00215162, 0x0016001c}, Seg: [4]uint16{0xd518, 0x1f9f, 0x5000, 0x3000}, FS: 0xc8ac, GS: 0xba30, IP: 0xc, Flags: 0x3682, EFlagsHi: 2, Model: 2, Steps: 100, Cycles: 1000, FPUCW: 0x37f}
	put := binary.LittleEndian.PutUint16
	put(in.Return[:], 0x0205)
	put(in.Return[2:], 0x2000)
	for i, v := range []uint16{0x100, 3, 0x100, 0x4000, 0xfeed, 0xbeef} {
		put(in.Args[i*2:], v)
	}
	for i, v := range []uint16{0x6000, 0x200, 0, 0, 319, 199} {
		put(in.Descriptor[i*2:], v)
	}
	put(in.RowTable[:], 16)
	put(in.RowTable[2:], 336)
	for i, xy := range [3][2]uint16{{160, 128}, {163, 132}, {160, 132}} {
		put(in.Vertices[i*22:], xy[0])
		put(in.Vertices[i*22+2:], xy[1])
		put(in.Vertices[i*22+4:], 59)
	}
	return in
}
func TestR1EffectsKnownCallAndOrderedStores(t *testing.T) {
	in := r1EffectsTestInput()
	p, r := BuildR1Effects(in)
	if r != R1EffectsOK {
		t.Fatal(r)
	}
	if p.Instructions != 353 || p.CyclesDelta != 1653 || p.CPU.Steps != 452 || p.CPU.Cycles != 2653 || p.EffectCount != 64 || p.ByteCommits != 144 {
		t.Fatal("known actual353/1653/144", p.Instructions, p.CyclesDelta, p.CPU.Steps, p.EffectCount, p.ByteCommits)
	}
	want := [8]uint32{0x00a08000, 0x00040000, 4, 0x00a38000, 0x1234f004, 0x5678fe00, 0x00215162, 0x0016001c}
	if p.CPU.GPR != want || p.CPU.Flags != 0x3296 || p.CPU.Seg != [4]uint16{0x6000, 0x2000, 0x5000, 0x3000} || p.CPU.IP != 0x205 || p.CPU.FS != in.CPU.FS || p.CPU.GS != in.CPU.GS || p.CPU.FPUCW != in.CPU.FPUCW || p.CPU.EFlagsHi != 2 {
		t.Fatal("CPU scalar/high halves", p.CPU)
	}
	if p.LastCS != 0x1f9f || p.LastIP != 0x2da || p.LastOpcode != 0xcb {
		t.Fatal("last RETF identity")
	}
	if p.Command.Color != 59 || p.Command.XY != [3][2]uint16{{160, 128}, {163, 132}, {160, 132}} || p.Command.Clip != [4]uint16{0, 0, 319, 199} {
		t.Fatal("flat command")
	}
	orCount, zeroStos := 0, 0
	for _, e := range p.Effects[:p.EffectCount] {
		if e.IP == 0xa9 {
			orCount++
			if e.Width != 1 || e.Count != 1 || e.Value != 0 {
				t.Fatal("OR store")
			}
		}
		if (e.IP == 0x2a6 || e.IP == 0x2ad) && e.Count == 0 {
			zeroStos++
		}
	}
	if orCount != 3 || zeroStos != 5 {
		t.Fatal("same-value/count0 lost", orCount, zeroStos)
	}
	if p.Effects[0].Offset != 0xeffe || p.Effects[33].IP != 0x280 || p.Effects[34].IP != 0x282 || p.Effects[35].IP != 0x2a6 || p.Effects[36].IP != 0x2ad || p.Effects[37].IP != 0x2b7 {
		t.Fatal("ordered row/template")
	}
}
func TestR1EffectsRejectDomainsAndZeroPlan(t *testing.T) {
	put := binary.LittleEndian.PutUint16
	cases := []struct {
		name   string
		mutate func(*R1EffectsInput)
	}{
		{"program", func(i *R1EffectsInput) { i.ProgramSHA256[31] ^= 1 }},
		{"ip", func(i *R1EffectsInput) { i.CPU.IP++ }},
		{"model", func(i *R1EffectsInput) { i.CPU.Model = 1 }},
		{"segmented", func(i *R1EffectsInput) { i.CPU.Segmented = true }},
		{"halt", func(i *R1EffectsInput) { i.CPU.Halted = true }},
		{"dosbox-cost", func(i *R1EffectsInput) { i.CPU.DOSBoxCost = true }},
		{"high-flags", func(i *R1EffectsInput) { i.CPU.EFlagsHi = 0 }},
		{"TF", func(i *R1EffectsInput) { i.CPU.Flags |= 0x100 }},
		{"flags-reserved-bit", func(i *R1EffectsInput) { i.CPU.Flags |= 8 }},
		{"flags-fixed-bit", func(i *R1EffectsInput) { i.CPU.Flags &^= 2 }},
		{"unaligned", func(i *R1EffectsInput) { i.CPU.GPR[4]++ }},
		{"stack-small", func(i *R1EffectsInput) { i.CPU.GPR[4] = 68 }},
		{"stack-large", func(i *R1EffectsInput) { i.CPU.GPR[4] = 65520 }},
		{"count", func(i *R1EffectsInput) { put(i.Args[2:], 4) }},
		{"vertex-wrap", func(i *R1EffectsInput) { put(i.Args[4:], 65470) }},
		{"descriptor-wrap", func(i *R1EffectsInput) { put(i.Args[:], 65525) }},
		{"table-wrap", func(i *R1EffectsInput) { put(i.Descriptor[2:], 65533) }},
		{"clip-inverted", func(i *R1EffectsInput) { put(i.Descriptor[4:], 320) }},
		{"clip-x", func(i *R1EffectsInput) { put(i.Descriptor[8:], 320) }},
		{"clip-y", func(i *R1EffectsInput) { put(i.Descriptor[10:], 200) }},
		{"vertex-outside", func(i *R1EffectsInput) { put(i.Vertices[:], 65535) }},
		{"not-v0-min", func(i *R1EffectsInput) { put(i.Vertices[2:], 132) }},
		{"not-flat", func(i *R1EffectsInput) { put(i.Vertices[46:], 133) }},
		{"winding", func(i *R1EffectsInput) { put(i.Vertices[22:], 159) }},
		{"zero-pitch", func(i *R1EffectsInput) { put(i.RowTable[2:], 16) }},
		{"wrapped-pitch", func(i *R1EffectsInput) { put(i.RowTable[:], 65530); put(i.RowTable[2:], 10) }},
		{"last-row-carry", func(i *R1EffectsInput) { put(i.RowTable[:], 16); put(i.RowTable[2:], 60016) }},
		{"steps-overflow", func(i *R1EffectsInput) { i.CPU.Steps = ^uint64(0) }},
		{"cycles-overflow", func(i *R1EffectsInput) { i.CPU.Cycles = ^uint64(0) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := r1EffectsTestInput()
			tc.mutate(&in)
			before := in
			p, r := BuildR1Effects(in)
			if r == R1EffectsOK || p != (R1EffectsPlan{}) || in != before {
				t.Fatal("rejection not atomic", r, p.EffectCount)
			}
		})
	}
}
func TestR1EffectsAddressAndUint64Edges(t *testing.T) {
	put := binary.LittleEndian.PutUint16
	for _, sp := range []uint32{72, 65516} {
		in := r1EffectsTestInput()
		in.CPU.GPR[4] = 0xabcd0000 | sp
		put(in.Args[:], 65524)
		put(in.Args[4:], 65469)
		put(in.Descriptor[2:], 65532)
		p, r := BuildR1Effects(in)
		if r != R1EffectsOK || p.CPU.GPR[4] != 0xabcd0000|uint32(sp+4) {
			t.Fatal("legal16bit endpoints rejected", sp, r)
		}
		for _, e := range p.Effects[:p.EffectCount] {
			if e.Count > 0 && uint32(e.Offset)+uint32(e.Width)*uint32(e.Count) > 65536 {
				t.Fatal("effect wrapped")
			}
		}
	}
	in := r1EffectsTestInput()
	in.CPU.Steps = ^uint64(0) - 352
	in.CPU.Cycles = ^uint64(0) - 1653
	p, r := BuildR1Effects(in)
	if r != R1EffectsOK || p.CPU.Steps != ^uint64(0) || p.CPU.Cycles != ^uint64(0) {
		t.Fatal("exact uint64 boundary")
	}
	in.CPU.Steps++
	if p, r = BuildR1Effects(in); r != R1EffectsTime || p != (R1EffectsPlan{}) {
		t.Fatal("N-1 overflow")
	}
	in = r1EffectsTestInput()
	in.CPU.Cycles = ^uint64(0) - 1652
	if p, r = BuildR1Effects(in); r != R1EffectsTime || p != (R1EffectsPlan{}) {
		t.Fatal("cycle overflow")
	}
	// Last ADD DI,pitch may equal65535; the last drawable byte must be <65535.
	in = r1EffectsTestInput()
	put(in.RowTable[:], 22975)
	put(in.RowTable[2:], 23295)
	p, r = BuildR1Effects(in)
	if r != R1EffectsOK {
		t.Fatal("exact last ADD boundary", r)
	}
	in = r1EffectsTestInput()
	put(in.RowTable[:], 65214)
	put(in.RowTable[2:], 65215)
	for k, xy := range [3][2]uint16{{160, 0}, {163, 1}, {160, 1}} {
		put(in.Vertices[k*22:], xy[0])
		put(in.Vertices[k*22+2:], xy[1])
	}
	if p, r = BuildR1Effects(in); r != R1EffectsOK {
		t.Fatal("strict destination endpoint legal", r)
	}
	put(in.RowTable[:], 65215)
	put(in.RowTable[2:], 65216)
	if p, r = BuildR1Effects(in); r != R1EffectsAddress || p != (R1EffectsPlan{}) {
		t.Fatal("last drawable strict65535")
	}
}
func TestR1EffectsH1H199AndCapacity(t *testing.T) {
	put := binary.LittleEndian.PutUint16
	for _, h := range []uint16{1, 199} {
		in := r1EffectsTestInput()
		for i, xy := range [3][2]uint16{{160, 0}, {319, h}, {0, h}} {
			put(in.Vertices[i*22:], xy[0])
			put(in.Vertices[i*22+2:], xy[1])
		}
		p, r := BuildR1Effects(in)
		if r != R1EffectsOK {
			t.Fatal(r)
		}
		// Count from the independent original template cardinalities, not a call
		// back into the planner to generate an expected plan.
		want := 33 + 5*(int(h)+1) + 2*(int(h)-1)
		if int(p.EffectCount) != want || p.EffectCount > 1429 || p.ByteCommits > 66861 {
			t.Fatal("fixed limits", h, p.EffectCount, p.ByteCommits)
		}
		var stackBytes uint32
		for _, e := range p.Effects[:p.EffectCount] {
			if e.Segment == in.CPU.Seg[2] {
				stackBytes += uint32(e.Width) * uint32(e.Count)
			}
		}
		if stackBytes != 75+14*uint32(h) {
			t.Fatal("stack cardinality", stackBytes)
		}
	}
	var p R1EffectsPlan
	p.EffectCount = R1EffectsCapacity
	old := p
	if p.store(1, 2, 3, 1, 1, 4) || p != old {
		t.Fatal("store capacity guard")
	}
}

var r1EffectsPlanSink R1EffectsPlan

func TestR1EffectsOwnedSizeAndNoAllocation(t *testing.T) {
	if unsafe.Sizeof(R1Effect{}) != 16 || unsafe.Sizeof(R1EffectsPlan{}) > 32768 {
		t.Fatal("fixed owned plan size", unsafe.Sizeof(R1Effect{}), unsafe.Sizeof(R1EffectsPlan{}))
	}
	var visit func(reflect.Type)
	visit = func(ty reflect.Type) {
		switch ty.Kind() {
		case reflect.Array:
			visit(ty.Elem())
		case reflect.Struct:
			for i := 0; i < ty.NumField(); i++ {
				visit(ty.Field(i).Type)
			}
		case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface, reflect.String, reflect.Func, reflect.Chan, reflect.UnsafePointer:
			t.Fatal("nonvalue payload", ty)
		}
	}
	visit(reflect.TypeOf(R1EffectsInput{}))
	visit(reflect.TypeOf(R1EffectsPlan{}))
	in := r1EffectsTestInput()
	before := in
	p, r := BuildR1Effects(in)
	if r != R1EffectsOK || in != before {
		t.Fatal("input mutated")
	}
	saved := p
	in.Vertices[0] ^= 1
	in.CPU.GPR[0] ^= 1
	if p != saved {
		t.Fatal("borrowed input")
	}
	p.Effects[0].Value ^= 1
	q, r := BuildR1Effects(before)
	if r != R1EffectsOK || q != saved {
		t.Fatal("shared output")
	}
	a := testing.AllocsPerRun(100, func() {
		var r R1EffectsReject
		r1EffectsPlanSink, r = BuildR1Effects(before)
		if r != R1EffectsOK {
			panic("valid input")
		}
	})
	if a != 0 {
		t.Fatal("per-call/per-pixel allocation", a)
	}
	t.Logf("effect=%dB input=%dB plan=%dB allocs/call=%g", unsafe.Sizeof(R1Effect{}), unsafe.Sizeof(R1EffectsInput{}), unsafe.Sizeof(R1EffectsPlan{}), a)
}
