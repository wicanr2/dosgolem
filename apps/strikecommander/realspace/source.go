package realspace

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const FaceSourceRule = "013a-raw18-ctor-order-v1"

// FaceSourceRole is an owned snapshot at a face-loop call site, not a retained
// view. Identity is compared between captured roles, never against callee registers.
type FaceSourceRole struct {
	FaceArgument                  FarPtr
	HasFaceArgument               bool
	Steps, Deliveries, Generation uint64
	SS, SP, BP, SI                uint16
	Rec, Face                     FarPtr
	Raw                           [18]byte
	FullFar                       [3]FarPtr
	FullProj, ConventionProj      [3][18]byte
	MethodF1, MethodBuild         FarPtr
	ReadFailed                    bool
}
type SourceBytes struct {
	Seg, Off uint16
	Data     []byte
}
type FaceSourceProof struct {
	Rule       string
	Base       uint16
	Generation uint64
	Bytes      []SourceBytes
}
type FaceSourceObs struct {
	Generation, O1Steps                        uint64
	F1Call, F1Return, BuildCall, ElementReturn FaceSourceRole
	Element                                    FarPtr
	Proof                                      *FaceSourceProof
}
type SourceSignature struct {
	Seg, Off uint16
	Hex      string
	Reloc    []int
}

// These are original file bytes and relative segments. IDA's loaded runtime
// addresses in order-audit.json used base 0241h; the table below removes it.
var faceSourceSignatures = []SourceSignature{
	{Seg: 0x1ad5, Off: 0x0a72, Hex: "ff769e", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0a75, Hex: "52", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0a7c, Hex: "ff5f18", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0a8b, Hex: "66ff769c", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0a95, Hex: "ff5f24", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0a9b, Hex: "66ff769c", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0aa5, Hex: "ff5f1c", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x10ef, Hex: "66ff7606", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x10f3, Hex: "666a00", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1b73, Hex: "66ff7606", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1b77, Hex: "666a00", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1151, Hex: "26c45f06", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1174, Hex: "26c45f0a", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1197, Hex: "26c45f0e", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x115b, Hex: "26894706", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x117e, Hex: "2689471c", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x11a1, Hex: "26894732", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1bda, Hex: "26c45f06", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1bfd, Hex: "26c45f0a", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1c20, Hex: "26c45f0e", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1be4, Hex: "26894706", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1c07, Hex: "2689471c", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1c2a, Hex: "26894732", Reloc: []int{}},
	{Seg: 0x20a8, Off: 0x006b, Hex: "c57406", Reloc: []int{}},
	{Seg: 0x20a8, Off: 0x006e, Hex: "26c45f1a", Reloc: []int{}},
	{Seg: 0x20a8, Off: 0x025f, Hex: "658b740a", Reloc: []int{}},
	{Seg: 0x20a8, Off: 0x0266, Hex: "648b5f1e", Reloc: []int{}},
	{Seg: 0x20a8, Off: 0x0454, Hex: "658b740e", Reloc: []int{}},
	{Seg: 0x20a8, Off: 0x045b, Hex: "648b5f22", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x0405, Hex: "6626894706", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x040e, Hex: "662689470a", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x0417, Hex: "662689470e", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x12eb, Hex: "9a3d085e1d", Reloc: []int{3}},
	{Seg: 0x2579, Off: 0x1e8d, Hex: "9a3d085e1d", Reloc: []int{3}},
	{Seg: 0x20a8, Off: 0x0002, Hex: "c80e000056571ec57606c474028976f2", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x10ec, Hex: "558bec66ff7606666a00900ee8050083c4085dcb", Reloc: []int{}},
	{Seg: 0x2579, Off: 0x1b70, Hex: "558bec66ff7606666a00900ee8050083c4085dcb", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0a7c, Hex: "ff5f1883c4080ac074578d8634ff5066", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0a7f, Hex: "83c4080ac074578d8634ff5066ff769c", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0aa5, Hex: "ff5f1c83c40489569a89469866837e98", Reloc: []int{}},
	{Seg: 0x1ad5, Off: 0x0aab, Hex: "89569a89469866837e98007425668b46", Reloc: []int{}},
}

func FaceSourceSignatures() []SourceSignature {
	out := append([]SourceSignature(nil), faceSourceSignatures...)
	for i := range out {
		out[i].Reloc = append([]int(nil), out[i].Reloc...)
	}
	return out
}
func SourceSignatureBytes(s SourceSignature, base uint16) []byte {
	b, e := hex.DecodeString(s.Hex)
	if e != nil {
		panic(e)
	}
	for _, i := range s.Reloc {
		if i < 0 || i+2 > len(b) {
			panic("source signature relocation")
		}
		binary.LittleEndian.PutUint16(b[i:], binary.LittleEndian.Uint16(b[i:])+base)
	}
	return b
}

type SourceSignatureError struct{ Reason string }

func (e *SourceSignatureError) Error() string { return e.Reason }
func ValidateFaceSource(o *ObjObs, f *FaceObs) error {
	s := f.Source
	if s == nil || s.Proof == nil {
		return fmt.Errorf("missing general FaceSourceObs")
	}
	p := s.Proof
	if p.Rule != FaceSourceRule || p.Generation == 0 || p.Generation != s.Generation {
		return fmt.Errorf("missing/unknown source rule or generation metadata")
	}
	if len(p.Bytes) != len(faceSourceSignatures) {
		return fmt.Errorf("missing source signature metadata")
	}
	for i, sig := range faceSourceSignatures {
		got := p.Bytes[i]
		want := SourceSignatureBytes(sig, p.Base)
		if got.Seg != p.Base+sig.Seg || got.Off != sig.Off || len(got.Data) != len(want) {
			return fmt.Errorf("missing/mislocated/truncated source signature metadata")
		}
		if !bytes.Equal(got.Data, want) {
			return &SourceSignatureError{fmt.Sprintf("source signature %04x:%04x differs", sig.Seg, sig.Off)}
		}
	}
	rs := [4]FaceSourceRole{s.F1Call, s.F1Return, s.BuildCall, s.ElementReturn}
	for _, i := range []int{0, 2} {
		if !rs[i].HasFaceArgument || rs[i].FaceArgument != f.Face {
			return fmt.Errorf("actual stack face argument differs from local Face")
		}
	}
	for i, r := range rs {
		if r.ReadFailed || r.Generation != s.Generation || r.Rec != o.Rec || r.Face != f.Face || r.SI != f.SI || r.SS != rs[0].SS || r.BP != rs[0].BP || r.Steps == 0 || i > 0 && r.Steps <= rs[i-1].Steps {
			return fmt.Errorf("source role identity/order/read failure")
		}
		if r.Raw != rs[0].Raw || binary.LittleEndian.Uint16(r.Raw[:]) != f.FaceVT {
			return fmt.Errorf("source raw face differs")
		}
		for j, at := range []int{6, 10, 14} {
			far := FarPtr{Off: binary.LittleEndian.Uint16(r.Raw[at:]), Seg: binary.LittleEndian.Uint16(r.Raw[at+2:])}
			if r.FullFar[j] != far || far.Seg != r.FullFar[0].Seg || r.FullProj[j] != r.ConventionProj[j] {
				return fmt.Errorf("source full far/common segment differs")
			}
		}
		ctor := uint16(0x10ec)
		if f.FaceVT == 0x338d {
			ctor = 0x1b70
		} else if f.FaceVT != 0x3415 {
			return fmt.Errorf("unknown source constructor")
		}
		if r.MethodF1 != (FarPtr{Seg: p.Base + 0x20a8, Off: 2}) || r.MethodBuild != (FarPtr{Seg: p.Base + 0x2579, Off: ctor}) {
			return fmt.Errorf("source method differs")
		}
	}
	if s.O1Steps != o.Steps || s.Element != f.Elem {
		return fmt.Errorf("source O1/E1 identity differs")
	}
	if rs[1].FullProj != f.Proj {
		return fmt.Errorf("F1 return snapshot differs")
	}
	// Build and E1 bytes must be kept distinct from earlier F1-return bytes: the
	// original writes offsets10h/11h before construction. They are not masked.
	if rs[2].FullProj != rs[3].FullProj {
		return fmt.Errorf("projection changed during constructor")
	}
	return nil
}
