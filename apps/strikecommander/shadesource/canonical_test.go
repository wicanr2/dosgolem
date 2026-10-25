package shadesource

import (
	"encoding/hex"
	"testing"
)

// The literal is the normative 013b 754-byte packet, not a call to a capture helper.
func TestCanonicalLiteral(t *testing.T) {
	id := Identity{EXESHA: HashLiteral(EXESHAHex), SignatureSetSHA: HashLiteral(SignatureSetSHAHex), Base: 577, ResetGeneration: 1, ModelsGeneration: 1, ShadeEpoch: 1, GroupSerial: 1, CUKey: 1}
	p := InitialPacket(id)
	h, e := SourceDigest(p)
	if e != nil {
		t.Fatal(e)
	}
	if hex.EncodeToString(h[:]) != "cb3ad4c70d731da030916887ff383d0ca57d37d8f7e223b53ba357a09829b65d" {
		t.Fatalf("literal digest %x", h)
	}
	p.Identity.CUKey++
	h2, e := SourceDigest(p)
	if e != nil || h2 == h {
		t.Fatal("identity not hashed")
	}
	p.Status = Status(4)
	if _, e = SourceDigest(p); e == nil {
		t.Fatal("unknown enum accepted")
	}
}
func TestReadWrapLiteral(t *testing.T) {
	calls := []uint32{}
	r := CaptureRead(func(a uint32, b []byte) bool {
		calls = append(calls, a)
		for i := range b {
			b[i] = byte(a + uint32(i))
		}
		return true
	}, Far{Seg: 0x100, Off: 0xfffd}, 5)
	if r.State != Complete || len(r.Spans) != 2 || calls[0] != 0x10ffd || calls[1] != 0x1000 || r.Spans[0].Requested != 3 || r.Spans[1].Requested != 2 {
		t.Fatalf("not same-segment wrap: %+v %x", r, calls)
	}
	if string(readBytes(r)) != string([]byte{0xfd, 0xfe, 0xff, 0, 1}) {
		t.Fatal("literal bytes")
	}
	bad := CaptureRead(func(a uint32, b []byte) bool { b[0] = 7; return false }, Far{1, 0xffff}, 5)
	if bad.State != Failed || len(bad.Spans) != 1 || bad.Spans[0].OK || len(bad.Spans[0].Data) != 1 {
		t.Fatal("failed read invented suffix")
	}
}

func TestCanonicalRejectsNilAndOversizedBeforeOutput(t *testing.T) {
	p := InitialPacket(Identity{})
	p.Proofs = nil
	if _, e := SourceDigest(p); e == nil {
		t.Fatal("nil required array")
	}
	p = InitialPacket(Identity{})
	p.Proofs = []Proof{{Reads: nil}}
	if _, e := SourceDigest(p); e == nil {
		t.Fatal("nil proof Reads")
	}
	p = InitialPacket(Identity{})
	p.Builds = []Build{{Raw5: &Read{Spans: nil}}}
	if _, e := SourceDigest(p); e == nil {
		t.Fatal("nil Read.Spans")
	}
	p = InitialPacket(Identity{})
	span := Span{Requested: 4096, OK: true, Data: make([]byte, 4096)}
	r := &Read{Requested: 4096, State: Complete, Spans: []Span{span}}
	for i := 0; i < 3000; i++ {
		p.Calls = append(p.Calls, R4{CallIndex: uint32(i), Table: r})
	}
	if _, e := MarshalPacket(p); e == nil {
		t.Fatal("oversized JSON output preflight")
	}
}
