package opaque

import (
	"crypto/sha256"
	"encoding/binary"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"runtime"
	"testing"
	"time"
)

func TestRevision002R1RawGeometryBinding(t *testing.T) {
	t.Run("shifted-self-consistent-original", func(t *testing.T) {
		r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
		c := &in.Scene.Raster.Calls[2]
		for j := 0; j < 3; j++ {
			x := binary.LittleEndian.Uint16(c.Verts[j*22:])
			binary.LittleEndian.PutUint16(c.Verts[j*22:], x+10)
		}
		// Independently prescribed original closed-triangle pixels, translated +10.
		for _, at := range []int{19*320 + 10, 20*320 + 10, 20*320 + 11} {
			in.Scene.Raster.Buf3D[at] = 0
			in.Scene.Original.Pixels[at] = 0
			in.Scene.Original.Writer[at] = 1
			in.Scene.Original.ObjectWritten[at] = false
			in.Scene.Raster.Buf3D[at+10] = 17
			in.Scene.Original.Pixels[at+10] = 17
			in.Scene.Original.Writer[at+10] = 2
			in.Scene.Original.ObjectWritten[at+10] = true
		}
		in.Identity.Group, _ = InputDigest(in.Scene)
		s, out := drawOracle(t, r, in, 2)
		defer s.Release()
		if out.Report().Diagnostics.H1Objects != 0 || out.Report().Diagnostics.H2Objects != 1 || out.Report().Diagnostics.SuccessfulObjects != 0 || out.Report().Diagnostics.WrittenSamples != 0 {
			t.Fatalf("raw geometry differs but material accepted: %+v", out.Report())
		}
	})
	for _, perm := range [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		t.Run(string(rune('a'+perm[0]*3+perm[1])), func(t *testing.T) {
			r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
			c := &in.Scene.Raster.Calls[2]
			old := append([]byte(nil), c.Verts...)
			// Only XY changes order. First low colour remains17 and has no ctor proof.
			c.Verts[26] = 44
			c.Verts[48] = 55
			for j, k := range perm {
				copy(c.Verts[j*22:j*22+4], old[k*22:k*22+4])
			}
			in.Identity.Group, _ = InputDigest(in.Scene)
			s, out := drawOracle(t, r, in, 2)
			defer s.Release()
			q, _ := out.Sample(20, 41, 0, 0)
			if out.Report().Diagnostics.H2Objects != 0 || !q.Coverage || q.Color.Lo != 17 {
				t.Fatal("legal XY permutation changed first colour or required R3 proof", out.Report(), q)
			}
		})
	}
}

func TestRevision002UnresolvedR2NeverDisappears(t *testing.T) {
	for _, kind := range []string{"ambiguous", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3495, kinds: []hd3d.CallKind{hd3d.R2}}, {vt: 0x3495, kinds: []hd3d.CallKind{hd3d.R2}}}})
			if kind == "ambiguous" {
				ptr := in.Scene.Raster.Calls[2].Element
				in.Scene.Objects[0].Faces[1].Elem = in.Scene.Objects[0].Faces[0].Elem
				in.Scene.Raster.Calls[3].Element = ptr
				in.Scene.Raster.Calls[3].Params[2] = ptr.Off + 6
				in.Scene.Original.CallElements[3] = ptr
			} else {
				in.Scene.Objects = nil
				for j := range in.Scene.Elements {
					binary.LittleEndian.PutUint16(in.Scene.Elements[j].Header[:], 0x9999)
				}
			}
			in.Identity.Group, _ = InputDigest(in.Scene)
			s, out := drawOracle(t, r, in, 2)
			defer s.Release()
			ops := out.DeferredOps()
			if !out.Report().Needs3b || len(ops) != 2 {
				t.Fatalf("unresolved R2 lost: %+v ops=%+v", out.Report(), ops)
			}
			for j, op := range ops {
				if op.Object != -1 || op.Face != -1 || op.Call.Seq != uint16(2+j) || op.Call.Kind != hd3d.R2 || len(op.Call.Table) != 256 {
					t.Fatal("invented R2 owner or lost original call", op)
				}
			}
		})
	}
}

func bindRevisionCapture(in *Input, idx []byte) PaletteSnapshot {
	var p [256][3]uint8
	p[17] = [3]uint8{17, 23, 41}
	p[9] = [3]uint8{91, 93, 97}
	c := CaptureIdentity{Scene: in.Identity, Kind: CaptureBuf3D, Steps: 2000, IndexedSHA: sha256.Sum256(idx), OriginalPaletteSHA: PaletteHash(p)}
	in.Capture = &c
	pal, _ := NewPaletteSnapshot(in.Identity, c, p)
	return pal
}
func TestRevision002Buf3DHasCanonicalCapture(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	wrong := append([]byte(nil), in.Scene.Raster.Buf3D...)
	wrong[0] = 9
	bindRevisionCapture(&in, wrong)
	if s, e := r.Accept(in); e == nil {
		s.Release()
		t.Fatal("foreign indexed background accepted as Buf3D capture")
	} else {
		requireGate(t, e, H3)
	}
	for _, kind := range []CaptureKind{CaptureBuf3D, CaptureVGA, CaptureFrame} {
		idx := in.Scene.Raster.Buf3D
		if kind != CaptureBuf3D {
			idx = wrong
		} // Other kinds have no canonical indexed bytes in model.Input.
		bindRevisionCapture(&in, idx)
		in.Capture.Kind = kind
		s, e := r.Accept(in)
		if e != nil {
			t.Fatal("valid/caller-bound capture rejected", kind, e)
		}
		s.Release()
	}
}
func TestRevision002ResolveHonoursConfiguredLimit(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	// An inert, bounded captured field exercises real frozen-input accounting.
	in.Scene.Raster.Calls[2].Table = make([]byte, 8<<20)
	in.Identity.Group, _ = InputDigest(in.Scene)
	r.limits.TotalBytes = 117 << 20
	pal := bindRevisionCapture(&in, in.Scene.Raster.Buf3D)
	s, out := drawOracle(t, r, in, 3)
	defer s.Release()
	bg, e := NewBackgroundComponents(in.Identity, *in.Capture, 3, in.Scene.Raster.Buf3D, pal)
	if e != nil {
		t.Fatal(e)
	}
	need := r.owned + r.materials.OwnedBytes() + uint64(len(bg.colors))*8 + uint64(out.width*out.height)*4
	if need <= r.limits.TotalBytes {
		t.Fatalf("fixture did not exceed configured limit: need=%d limit=%d", need, r.limits.TotalBytes)
	}
	if _, e = ResolveRGB(out, bg, pal); e == nil {
		t.Fatalf("RGB exceeded configured limit: need=%d limit=%d", need, r.limits.TotalBytes)
	} else {
		requireGate(t, e, H3)
	}
	r.limits.TotalBytes = DefaultLimits().TotalBytes
	if _, e = ResolveRGB(out, bg, pal); e != nil {
		t.Fatal("same source within default limit rejected", e)
	}
}

type renderReply struct {
	result *Result
	err    error
}

func awaitRenderEntered(t *testing.T, r *Renderer, s *Snapshot) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		r.mu.Lock()
		active := s.rendering != 0
		r.mu.Unlock()
		if active {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not enter")
		}
		runtime.Gosched()
	}
}
func awaitReply(t *testing.T, c <-chan renderReply) renderReply {
	t.Helper()
	select {
	case v := <-c:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("render did not finish")
		return renderReply{}
	}
}
func TestRevision002ConcurrentSameSlotSharesResult(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x33e9, kinds: []hd3d.CallKind{hd3d.R4}}}})
	s, e := r.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Release()
	r.materials.mu.Lock()
	a, b := make(chan renderReply, 1), make(chan renderReply, 1)
	go func() { v, e := s.Render(2); a <- renderReply{v, e} }()
	awaitRenderEntered(t, r, s)
	go func() { v, e := s.Render(2); b <- renderReply{v, e} }()
	time.Sleep(10 * time.Millisecond)
	r.materials.mu.Unlock()
	x, y := awaitReply(t, a), awaitReply(t, b)
	if x.err != nil || y.err != nil || x.result == nil || x.result != y.result || r.results != 1 {
		t.Fatalf("same slot poisoned or recomputed: A=%v B=%v same=%t count=%d", x.err, y.err, x.result == y.result, r.results)
	}
}
func TestRevision002ConcurrentThirdResultAdmission(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x33e9, kinds: []hd3d.CallKind{hd3d.R4}}}})
	a, first := drawOracle(t, r, in, 2)
	defer a.Release()
	firstHash := first.Hash()
	other := in
	other.Scene.Key = 8
	other.Scene.Raster.Key = 8
	other.Identity.CUKey = 8
	other.Identity.Group, _ = InputDigest(other.Scene)
	b, e := r.Accept(other)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Release()
	r.materials.mu.Lock()
	ca, cb := make(chan renderReply, 1), make(chan renderReply, 1)
	go func() { v, e := a.Render(3); ca <- renderReply{v, e} }()
	awaitRenderEntered(t, r, a)
	r.mu.Lock()
	go func() { v, e := b.Render(2); cb <- renderReply{v, e} }()
	time.Sleep(10 * time.Millisecond)
	r.materials.mu.Unlock()
	// Let the old unlock-before-commit gap become visible. With a correct
	// transaction the worker remains owned until this publication lock is freed.
	deadline := time.Now().Add(100 * time.Millisecond)
	publicationGap := false
	for time.Now().Before(deadline) {
		if r.worker.TryLock() {
			publicationGap = true
			r.worker.Unlock()
			break
		}
		runtime.Gosched()
	}
	r.mu.Unlock()
	x, y := awaitReply(t, ca), awaitReply(t, cb)
	if publicationGap || r.results > 2 || x.err == nil && y.err == nil {
		t.Fatalf("third result admitted: count=%d A=%v B=%v", r.results, x.err, y.err)
	}
	if first.Hash() != firstHash {
		t.Fatal("previous published result changed")
	}
}
func TestRevision002InvalidatedInFlightCannotPublish(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x33e9, kinds: []hd3d.CallKind{hd3d.R4}}}})
	old, first := drawOracle(t, r, in, 2)
	defer old.Release()
	firstHash := first.Hash()
	s, e := r.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Release()
	r.materials.mu.Lock()
	done := make(chan renderReply, 1)
	go func() { v, e := s.Render(3); done <- renderReply{v, e} }()
	awaitRenderEntered(t, r, s)
	bad := in
	bad.SignatureValid = false
	_, e = r.Accept(bad)
	requireGate(t, e, H4)
	r.materials.mu.Unlock()
	got := awaitReply(t, done)
	requireGate(t, got.err, H4)
	if got.result != nil || r.results != 1 || first.Hash() != firstHash {
		t.Fatal("invalid in-flight result published or old immutable changed", got.result, r.results)
	}
	_, e = s.Render(3)
	requireGate(t, e, H4)
}

func TestRevision002R1XYMultiplicity(t *testing.T) {
	var proj [3][18]byte
	var raw [66]byte
	for i, x := range []uint16{1, 1, 2} {
		binary.LittleEndian.PutUint16(proj[i][:], x)
		binary.LittleEndian.PutUint16(raw[i*22:], x)
	}
	if !sameXYMultiset(raw[:], proj) {
		t.Fatal("legal repeated XY rejected")
	}
	binary.LittleEndian.PutUint16(raw[44:], 1)
	if sameXYMultiset(raw[:], proj) {
		t.Fatal("raw repeats a corner more often than the original face")
	}
}
func TestRevision002SourceLedgerSharedAndBounded(t *testing.T) {
	r, in := sceneOracle(t, objectSpec{[]faceSpec{{vt: 0x3415, kinds: []hd3d.CallKind{hd3d.R3}, color: 1344}}})
	if r.owned < (8<<20)+(512<<10) {
		t.Fatal("ledger capacity omitted from owned reservation", r.owned)
	}
	bad := in
	bad.SignatureValid = false
	for k := uint64(2); k <= 4096; k++ {
		bad.Identity.SignatureGeneration = k
		_, e := r.Accept(bad)
		requireGate(t, e, H4)
	}
	if len(r.blocked) != 4095 {
		t.Fatal("incorrect bounded ledger size", len(r.blocked))
	}
	in.Scene.Objects[0].Faces[0].Source.Proof.Bytes[0].Data[0] ^= 1
	in.Identity.Group, _ = InputDigest(in.Scene)
	snap, e := r.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	defer snap.Release()
	_, e = snap.Render(2)
	requireGate(t, e, H4)
	if len(r.blocked) != 4096 {
		t.Fatal("render H4 did not share the bounded ledger", len(r.blocked))
	}
	bad.Identity.SignatureGeneration = 4097
	_, e = r.Accept(bad)
	requireGate(t, e, H3)
	bad.SignatureValid = true
	_, e = r.Accept(bad)
	requireGate(t, e, H3)
	bad.Identity.SignatureGeneration = 2
	_, e = r.Accept(bad)
	requireGate(t, e, H4)
	if len(r.blocked) != 4096 {
		t.Fatal("ledger grew past4096", len(r.blocked))
	}
	lower := r.materials.OwnedBytes() + (8 << 20) + (512 << 10) + uint64(len(r.tables.R6Table)) - 1
	_, e = NewRenderer(r.lib, r.materials, r.tables, Limits{TotalBytes: lower})
	requireGate(t, e, H3)
}
