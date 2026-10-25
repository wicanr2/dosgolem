package opaque

import (
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"runtime"
	"sync"
	"testing"
	"time"
)

type handBudget struct {
	mu           sync.Mutex
	owned, limit uint64
	blocked      uint64
	workspace    sync.Mutex
}
type handReservation struct {
	b    *handBudget
	n    uint64
	once sync.Once
}

func (r *handReservation) Release() {
	r.once.Do(func() { r.b.mu.Lock(); defer r.b.mu.Unlock(); r.b.owned -= r.n })
}
func (b *handBudget) ReserveComposition(n uint64) (CompositionReservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n > b.limit-b.owned {
		return nil, fmt.Errorf("hand budget full")
	}
	b.owned += n
	return &handReservation{b: b, n: n}, nil
}
func (b *handBudget) CompositionWorkspace(f func() error) error {
	b.workspace.Lock()
	defer b.workspace.Unlock()
	return f()
}
func (b *handBudget) OwnedBytes() uint64 { b.mu.Lock(); defer b.mu.Unlock(); return b.owned }
func (b *handBudget) CompositionEpochEligible(epoch uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.blocked != 0 && epoch <= b.blocked {
		return gateFailure(H4, -1, -1, "hand source revoked")
	}
	return nil
}
func (b *handBudget) TotalBytes() uint64 { return b.limit }
func compositionFixture(t *testing.T, specs ...objectSpec) (*CompositionRenderer, CompositionInput, PaletteSnapshot, *handBudget) {
	t.Helper()
	r, in := sceneOracle(t, specs...)
	// Literal horizon0 is the initial background. The retained empty BB precedes
	// every removed model operator; no Patch After is used as counterfactual BG.
	for i := 0; i < 192; i++ {
		r.tables.R6Table[i] = 0
	}
	in.Scene.Raster.Calls[0].Aux.Color0 = 0
	in.Scene.Raster.Calls[0].Aux.Color1 = 0
	in.Scene.Raster.Calls[1].Patch = nil
	for i, v := range in.Scene.Original.Writer {
		if v == 1 {
			in.Scene.Original.Writer[i] = 0
		}
	}
	var e error
	in.Identity.Group, e = InputDigest(in.Scene)
	if e != nil {
		t.Fatal(e)
	}
	pal := bindRevisionCapture(&in, in.Scene.Raster.Buf3D)
	budget := &handBudget{owned: 40 << 20, limit: 256 << 20}
	c, e := NewCompositionRenderer(r.lib, r.materials, r.tables, budget)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c, CompositionInput{Opaque: in, SourceEpoch: 1}, pal, budget
}
func TestCompositionR1NoExtraActualSeqAndLiteralPixel(t *testing.T) {
	c, in, p, b := compositionFixture(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	s, e := c.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Render(2)
	if e != nil {
		t.Fatal(e)
	}
	v, seq, ok := r.Sample(20, 41, 0, 0)
	if !ok || !v.Coverage || v.Color.Lo != 17 || seq != 2 || !r.NoExtra() {
		t.Fatalf("sample%+v seq%d noextra%t", v, seq, r.NoExtra())
	}
	out := make([]byte, 640*400*4)
	d, e := r.WriteRGBA(p, out)
	if e != nil {
		t.Fatal(e)
	}
	// The four quarter samples in this pixel contain one interior point of the
	// literal triangle(20,42),(22,42),(20,40). The diagonal is top-left excluded.
	at := (40*640 + 20) * 4
	want := [4]byte{4, 6, 10, 255}
	var got [4]byte
	copy(got[:], out[at:at+4])
	if got != want {
		t.Fatalf("pixel%v want%v", got, want)
	}
	if d.RGBSHA != sha256.Sum256(out) || d.OpaqueSamples == 0 {
		t.Fatal(d)
	}
	hash := r.Hash()
	if e = s.ReleaseResult(2); e != nil {
		t.Fatal(e)
	}
	if r.Hash() != hash {
		t.Fatal("fixed hash lost")
	}
	again, e := s.Render(2)
	if e != nil || again.Hash() != hash || again == r {
		t.Fatal("successful re-render lost fixed SHA", e)
	}
	if _, _, ok := r.Sample(20, 41, 0, 0); ok {
		t.Fatal("released alias regained eligibility")
	}
	if e = s.Release(); e != nil {
		t.Fatal(e)
	}
	if e = c.Close(); e != nil {
		t.Fatal(e)
	}
	if b.OwnedBytes() != 40<<20 {
		t.Fatalf("leak%d", b.OwnedBytes())
	}
}
func TestCompositionFFBottomKeepsActualWriterAndNoExtra(t *testing.T) {
	c, in, _, _ := compositionFixture(t, objectSpec{[]faceSpec{{vt: 0x33b9, kinds: []hd3d.CallKind{hd3d.R1, hd3d.R4}, color: 17}}})
	// Optional source failure is intentionally unused: every captured texel FF.
	in.SourceError = &Failure{Gate: H3, Reason: "optional source capacity"}
	s, e := c.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Render(2)
	if e != nil {
		t.Fatal(e)
	}
	v, seq, ok := r.Sample(20, 41, 0, 0)
	if !ok || !v.Coverage || v.Color.Lo != 17 || seq != 2 || !r.NoExtra() {
		t.Fatalf("FF changed bottom%+v seq%d", v, seq)
	}
}
func TestCompositionCounterfactualRejectsLatePatchAndUnresolvedR2(t *testing.T) {
	c, in, p, _ := compositionFixture(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	in.Opaque.Scene.Raster.Calls[1].Patch = []hd3d.PixelSet{{Index: 0, Value: 0}}
	in.Opaque.Scene.Original.Writer[0] = 1
	in.Opaque.Identity.Group, _ = InputDigest(in.Opaque.Scene)
	p = bindRevisionCapture(&in.Opaque, in.Opaque.Scene.Raster.Buf3D)
	s, e := c.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Render(2)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.WriteRGBA(p, make([]byte, 640*400*4)); e == nil {
		t.Fatal("Patch After accepted as background")
	}
	c2, in2, p2, _ := compositionFixture(t, objectSpec{[]faceSpec{{vt: 0x3495, kinds: []hd3d.CallKind{hd3d.R2}}}})
	s2, e := c2.Accept(in2)
	if e != nil {
		t.Fatal(e)
	}
	r2, e := s2.Render(2)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r2.WriteRGBA(p2, make([]byte, 640*400*4)); e == nil {
		t.Fatal("unmapped raw R2 disappeared")
	}
}

func TestCompositionConcurrentSlotAndMidflightNoExtraRevocation(t *testing.T) {
	c, in, _, b := compositionFixture(t, objectSpec{[]faceSpec{{vt: 0x3441, kinds: []hd3d.CallKind{hd3d.R1}, color: 17}}})
	s, e := c.Accept(in)
	if e != nil {
		t.Fatal(e)
	}
	type reply struct {
		r *CompositionResult
		e error
	}
	done := make(chan reply, 2)
	for i := 0; i < 2; i++ {
		go func() { r, e := s.Render(2); done <- reply{r, e} }()
	}
	a, z := <-done, <-done
	if a.e != nil || z.e != nil || a.r != z.r || c.results != 1 {
		t.Fatal(a.e, z.e, c.results)
	}
	// A different current-N request is held at the shared source worker; an
	// independent source H4 arrives while the request is already active.
	b.workspace.Lock()
	go func() { r, e := s.Render(3); done <- reply{r, e} }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		c.mu.Lock()
		active := s.active
		c.mu.Unlock()
		if active != 0 {
			break
		}
		if time.Now().After(deadline) {
			b.workspace.Unlock()
			t.Fatal("no active request")
		}
		runtime.Gosched()
	}
	b.mu.Lock()
	b.blocked = 1
	b.mu.Unlock()
	b.workspace.Unlock()
	q := <-done
	if q.e == nil || q.r != nil {
		t.Fatal("revoked NoExtra published", q.e)
	}
	if a.r.Eligible() == nil {
		t.Fatal("prior unpublished qualification survived H4")
	}
}

func (b *handBudget) OwnsCompositionSource(ShadeSource) bool { return true }
func (b *handBudget) PublishComposition(epoch uint64, fn func() error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.blocked != 0 && epoch <= b.blocked {
		return gateFailure(H4, -1, -1, "hand revoked before publish")
	}
	return fn()
}
