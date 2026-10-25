package opaque

import (
	"crypto/sha256"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"math/big"
	"os"
	"testing"
)

func TestCompositionTableEpochSourceBytesAndFixedFailure(t *testing.T) {
	raw, e := os.ReadFile("/project/workplace/m6asset/probe/s800b/shadetab.bin")
	if os.IsNotExist(e) {
		t.Skip("original shade table unavailable")
	}
	if e != nil {
		t.Fatal(e)
	}
	if len(raw) != 4096 || sha256.Sum256(raw) != hashHex(shadeTableSHA) {
		t.Fatal("fixture source hash")
	}
	var table [4096]byte
	copy(table[:], raw)
	id := ShadeIdentity{ResetGeneration: 1, ModelsGeneration: 2, ShadeEpoch: 3, CUKey: 4}
	var evidence TableEvidence
	if _, e = evidence.adopt(id, &table, false); e == nil {
		t.Fatal("bad row created verified table")
	}
	if evidence.epoch.verified {
		t.Fatal("unqualified row verified table")
	}
	v, e := evidence.adopt(id, &table, true)
	if e != nil || !evidence.Eligible(v) {
		t.Fatal(e)
	}
	changed := table
	changed[0] ^= 1
	if _, e = evidence.adopt(id, &changed, true); e == nil {
		t.Fatal("verified content change passed")
	} else {
		requireGate(t, e, H3)
	}
	if evidence.Eligible(v) {
		t.Fatal("old epoch newly eligible")
	}
	if _, e = evidence.adopt(id, &table, true); e == nil {
		t.Fatal("same CU healed poisoned epoch")
	}
	id.CUKey++
	v, e = evidence.adopt(id, &table, true)
	if e != nil || !evidence.Eligible(v) {
		t.Fatal("later CU did not recover", e)
	}
	var first TableEvidence
	if _, e = first.adopt(id, &changed, true); e == nil {
		t.Fatal("first bad SHA passed")
	} else {
		requireGate(t, e, H2)
	}
}
func TestContinuousQ24TiesAndPalette255(t *testing.T) {
	table := make([]byte, 4096)
	table[9] = 255
	table[256+9] = 3
	for _, tc := range []struct {
		twice int64
		want  uint32
	}{{1, 0}, {3, 2}, {5, 2}} {
		// Desired row fraction twice/(2*2^24). Class8 s=(8/9)*(255-16*r).
		frac := big.NewRat(tc.twice, 2*int64(WeightDenominator))
		scalar := new(big.Rat).Sub(big.NewRat(255, 1), new(big.Rat).Mul(big.NewRat(16, 1), frac))
		scalar.Mul(scalar, big.NewRat(8, 9))
		got, e := continuousR4(scalar, 8, 9, table)
		if e != nil || got.W24 != tc.want || got.Lo != 255 {
			t.Fatal(tc, got, e)
		}
	}
	c, e := continuousR4(big.NewRat(247, 1), 7, 9, table)
	if e != nil || c != (SampleColor{255, 3, 1 << 23}) {
		t.Fatal(c, e)
	}
}

type handShadeLease struct {
	d        ShadeDeclaration
	w        ShadeWitness
	table    [4096]byte
	row      [256]byte
	released bool
}

func (s *handShadeLease) CompositionDeclaration() ShadeDeclaration { return s.d }
func (s *handShadeLease) CompositionEligible() error {
	if s.released {
		return gateFailure(H3, -1, -1, "released hand source")
	}
	return nil
}
func (s *handShadeLease) ReadShadeCall(i uint32, t *[4096]byte, r *[256]byte) (ShadeWitness, error) {
	if i != s.w.CallIndex {
		return ShadeWitness{}, gateFailure(H2, -1, -1, "wrong hand call")
	}
	*t = s.table
	*r = s.row
	return s.w, s.CompositionEligible()
}
func (s *handShadeLease) Release() { s.released = true }
func TestCompositionConsumesDemandedClass7And8SameCallWitness(t *testing.T) {
	raw, e := os.ReadFile("/project/workplace/m6asset/probe/s800b/shadetab.bin")
	if os.IsNotExist(e) {
		t.Skip("original shade table unavailable")
	}
	if e != nil || len(raw) != 4096 || sha256.Sum256(raw) != hashHex(shadeTableSHA) {
		t.Fatal("source table fixture", e)
	}
	for _, tc := range []struct {
		vt     uint16
		base   hd3d.CallKind
		scalar byte
		weight uint32
	}{{0x33b9, hd3d.R1, 247, 1 << 23}, {0x338d, hd3d.R3, 220, 15 << 19}} {
		c, in, _, _ := compositionFixture(t, objectSpec{[]faceSpec{{vt: tc.vt, kinds: []hd3d.CallKind{tc.base, hd3d.R4}, textureAlt: true, color: 1344}}})
		call := &in.Opaque.Scene.Raster.Calls[3]
		call.Tex.Pixels.Spans[0].Data[0] = 7
		call.Params[7] = 0x4000
		call.Params[8] = 0x8000
		call.Tex.Shade.Spans = []hd3d.Span{{Off: 0x4000, Data: append([]byte(nil), raw[:256]...)}}
		if tc.vt == 0x338d {
			for k := 0; k < 3; k++ {
				in.Opaque.Scene.Objects[0].Faces[0].Source.BuildCall.FullProj[k][16] = tc.scalar
				in.Opaque.Scene.Objects[0].Faces[0].Source.BuildCall.ConventionProj[k][16] = tc.scalar
				in.Opaque.Scene.Objects[0].Faces[0].Source.ElementReturn.FullProj[k][16] = tc.scalar
				in.Opaque.Scene.Objects[0].Faces[0].Source.ElementReturn.ConventionProj[k][16] = tc.scalar
			}
		}
		//012 Replay supplies only the lower-level admission fixture. HD color and
		// scalar/Q24 expectations below are literal, independent of3b rendering.
		g := in.Opaque.Scene.Raster
		g.Buf3D = nil
		original := hd3d.Replay(g, c.core.tables)
		if !original.Valid {
			t.Fatal(original.Diagnostics)
		}
		original.Checked = true
		original.Repaintable = true
		in.Opaque.Scene.Original = &original
		in.Opaque.Scene.Raster.Buf3D = append([]byte(nil), original.Pixels[:]...)
		in.Opaque.Identity.Group, _ = InputDigest(in.Opaque.Scene)
		bindRevisionCapture(&in.Opaque, in.Opaque.Scene.Raster.Buf3D)
		src := &handShadeLease{d: ShadeDeclaration{LegacyInputDigest: in.Opaque.Identity.Group, Identity: ShadeIdentity{EXESHA: in.Opaque.Identity.EXE, ModelsGeneration: in.Opaque.Identity.SignatureGeneration, ResetGeneration: 1, ShadeEpoch: 1, CUKey: in.Opaque.Identity.CUKey}, DeclaredSourceDigest: [32]byte{1}}, w: ShadeWitness{CallIndex: 3, Object: 0, Face: 0, HasFace: true, TableSeg: 0x8000, TableOff: 0x4000, RowSeg: 0x8000, RowOff: 0x4000, TableComplete: true, RowComplete: true, Raw5: [5]byte{0, 0, 0, 0, tc.scalar}, HasRaw5: true}}
		copy(src.table[:], raw)
		copy(src.row[:], raw[:256])
		in.Source = src
		snap, e := c.Accept(in)
		if e != nil {
			t.Fatal(e)
		}
		result, e := snap.Render(2)
		if e != nil {
			t.Fatal(e)
		}
		v, seq, ok := result.Sample(20, 41, 0, 0)
		want := SampleColor{raw[7], raw[263], tc.weight}
		if !ok || !v.Coverage || v.Color != want || seq != 3 || result.NoExtra() {
			t.Fatalf("class%04x got%+v seq%d noextra%t want%+v report%+v", tc.vt, v, seq, result.NoExtra(), want, result.Report())
		}
		snap.Release()
		if !src.released {
			t.Fatal("accepted source lease leaked")
		}
	}
}
