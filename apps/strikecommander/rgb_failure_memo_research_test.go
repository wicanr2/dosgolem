package strikecommander

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/opaque"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
)

// Receiver-only fixture: no VM, recorder callback or original mutation. The
// immutable JSON input is precharged separately for its entire test lifetime.
type rgbMemoRig struct {
	rt    *FrontendRuntime
	holds int
	t     *testing.T
}

const rgbMemoFixtureBytes = uint64(16 << 20)

func newRGBMemoRig(t *testing.T) *rgbMemoRig {
	t.Helper()
	root := os.Getenv("SC_RGB_MEMO_ROOT")
	if root == "" {
		t.Skip("explicit read-only original assets and captured fixtures required")
	}
	cfg := runtimeTestConfig(t)
	cfg.HD = &FrontendHDConfig{GameRoot: root, StateSHA: [32]byte{1}, AppGeneration: 1}
	r, err := NewFrontendRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	f := &rgbMemoRig{rt: r, t: t}
	owner, pool := r.hd.owner, r.hd.pool
	t.Cleanup(func() {
		r.Close()
		// End the separately owned fixture buffers after real registry/source
		// teardown. Query the original owner, never a cleared Runtime pointer.
		for f.holds > 0 {
			pool.ReleaseLegacy(rgbMemoFixtureBytes)
			f.holds--
		}
		if owner.OwnedBytes() != 0 || pool.Usage().LegacyOwned != 0 || pool.Usage().ExtraOwned != 0 {
			t.Errorf("Close kept owned buffers: owner=%d pool=%+v", owner.OwnedBytes(), pool.Usage())
		}
	})
	return f
}

func (f *rgbMemoRig) install(prefix string, delta uint64) *hdRegistrySlot {
	t := f.t
	t.Helper()
	r := f.rt
	if err := r.hd.pool.ReserveLegacy(rgbMemoFixtureBytes); err != nil {
		t.Fatal(err)
	}
	f.holds++
	var scene model.Input
	var packet shadesource.Packet
	var declaration shadesource.Declaration
	for suffix, dst := range map[string]any{"legacy": &scene, "packet": &packet, "declaration": &declaration} {
		p := prefix + "-" + suffix + ".json"
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) > 6<<20 {
			t.Fatal("fixture exceeds precharged encoded/decoded input allowance")
		}
		if err = json.Unmarshal(b, dst); err != nil {
			t.Fatal(err)
		}
	}
	if delta != 0 {
		// A synthetic new identity over the same read-only captured geometry.
		// These values are construction input, not a claimed new original trace.
		scene.Key += delta
		scene.Raster.Key = scene.Key
		packet.Identity.CUKey = scene.Key
		packet.Identity.GroupSerial += delta
		digest, err := opaque.InputDigest(scene)
		if err != nil {
			t.Fatal(err)
		}
		packet.LegacyInputDigest = digest
		declaration.LegacyInputDigest, declaration.Identity = packet.LegacyInputDigest, packet.Identity
		declaration.DeclaredSourceDigest, err = shadesource.SourceDigest(packet)
		if err != nil {
			t.Fatal(err)
		}
	}
	lease, err := r.hd.owner.Accept(scene, packet, declaration)
	if err != nil {
		t.Fatalf("controlled source Accept: %v", err)
	}
	transferred := false
	defer func() {
		if !transferred {
			lease.Release()
		}
	}()
	a, err := newLegacyArena(r.hd.pool)
	if err != nil {
		t.Fatal(err)
	}
	g, err := legacyNew[ModelGroup](a)
	if err != nil {
		t.Fatal(err)
	}
	*g = ModelGroup{Key: scene.Key, Cameras: scene.Cameras, Objects: scene.Objects, Calls: scene.Raster.Calls, List: scene.Raster.List, Buf3D: scene.Raster.Buf3D, Raster: scene.Original, ReplayReady: true}
	g.Desc = g.Calls[0].Desc
	for _, e := range scene.Elements {
		g.Elements = append(g.Elements, RasterElement{Pointer: e.Pointer, Header: e.Header})
	}
	c, err := legacyNew[hdSealedCapture](a)
	if err != nil {
		t.Fatal(err)
	}
	*c = hdSealedCapture{Group: g, Arena: a, Serial: packet.Identity.GroupSerial, CUKey: scene.Key, ShadeEpoch: packet.Identity.ShadeEpoch, SignatureGeneration: packet.Identity.ModelsGeneration, SignatureValid: true}
	a.seal()
	r.hd.registry.onSealed(c)
	a.retire()
	var slot *hdRegistrySlot
	for i := range r.hd.registry.slots {
		if r.hd.registry.slots[i].capture == c {
			slot = &r.hd.registry.slots[i]
		}
	}
	if slot == nil {
		t.Fatal("sealed source not accepted")
	}
	slot.source = lease
	transferred = true
	m := r.so.Models
	m.base, m.baseOK, m.sourceGeneration = packet.Identity.Base, true, packet.Identity.ModelsGeneration
	for _, o := range scene.Objects {
		for _, face := range o.Faces {
			if face.Source != nil && face.Source.Proof != nil {
				m.sourceProof = face.Source.Proof
				break
			}
		}
	}
	if m.sourceProof == nil {
		t.Fatal("fixture has no original proof for status")
	}
	m.tracker = cuTracker{base: m.base, baseOK: true, cuSteps: scene.Key}
	flight := r.so.Flight
	flight.base, flight.baseKnown, flight.baseOK = m.base, true, true
	flight.cu = cuTracker{base: m.base, baseOK: true, cuSteps: scene.Key}
	r.hd.valid, r.hd.generation = true, m.sourceGeneration
	img := &presentImage{sourceCU: scene.Key, indexed: append([]byte(nil), scene.Raster.Buf3D...), info: PresentInfo{Seq: r.present.seq + 1, CaptureSteps: scene.Key + 1, Kind: PresCUReturn, SourceCS: m.base + segFJemm, SourceIP: ipCUDiffRet + 1}}
	for i := range img.palette {
		img.palette[i] = [3]byte{byte(i), byte(i), byte(i)}
	}
	img.palette[20] = [3]byte{134, 134, 134}
	r.present.seq = img.info.Seq
	r.present.steps = img.info.CaptureSteps
	r.present.cur = img
	r.present.publish(img)
	if key, serial, ok := r.CurrentHDSource(); !ok || key != scene.Key || serial != c.Serial {
		t.Fatalf("not same source: key=%d serial=%d ok=%v", key, serial, ok)
	}
	return slot
}

func (f *rgbMemoRig) round() FrontendCompositeInfo {
	t := f.t
	t.Helper()
	req := FrontendPrepareRequest{Scale: 2, Language: "en", HD: true, SettingsVersion: 1}
	prepared, err := f.rt.Prepare(req)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Release()
	info, err := prepared.Info()
	if err != nil {
		t.Fatal(err)
	}
	composed, ok := f.rt.Compose(req)
	if !ok {
		t.Fatal("Compose missing after Prepare")
	}
	defer composed.Release()
	prepared.Release()
	if err = composed.CopyRGBA(make([]byte, 640*400*4)); err != nil {
		t.Fatal(err)
	}
	return info
}

func TestRGBFailureMemoMixedFixedThroughReleaseAndSafety(t *testing.T) {
	f := newRGBMemoRig(t)
	prefix := filepath.Join(os.Getenv("SC_RGB_MEMO_CASES"), "runtime-corpus-004/run-n2-n2-001/C-group-001")
	slot := f.install(prefix, 0)
	first := f.round()
	if !slot.memo[0].success || !slot.memo[0].failed || !strings.Contains(first.Reason, "MixedFallbackUnknown") {
		t.Fatalf("real Accept/Render then RGB Mixed not fixed: memo=%+v info=%+v", slot.memo[0], first)
	}
	if err := f.rt.hd.renderer.Close(); err != nil {
		t.Fatal(err)
	}
	second := f.round()
	if second.key != first.key || second.Reason != first.Reason || second.SHA256 != first.SHA256 || strings.Contains(second.Reason, "closed") {
		t.Fatal("same-key retry entered closed renderer", first.Reason, second.Reason)
	}
	f.rt.output.safety.set(HDStale)
	f.rt.output.invalidate()
	if info := f.round(); info.Kind != FrontendSafetyFallback {
		t.Fatal("expected independent SafetyFallback")
	}
	f.rt.output.safety.set(HDEligible)
	f.rt.output.invalidate()
	if info := f.round(); info.Reason != first.Reason || !slot.memo[0].failed {
		t.Fatal("Safety recovery washed fixed scene H3", info.Reason)
	}
	materials, err := opaque.LoadMaterialLibrary(os.Getenv("SC_RGB_MEMO_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	f.rt.hd.renderer, err = opaque.NewCompositionRenderer(f.rt.hd.registry.lib, materials, f.rt.hd.tables, f.rt.hd.owner)
	if err != nil {
		t.Fatal(err)
	}
	next := f.install(prefix, 1)
	if next.memo[0].success || next.memo[0].failed {
		t.Fatal("new CU/serial inherited old memo")
	}
	if info := f.round(); !next.memo[0].success || !next.memo[0].failed || !strings.Contains(info.Reason, "MixedFallbackUnknown") {
		t.Fatal("fresh renderer did not reach new CU's real RGB Mixed failure", info.Reason)
	}
}

func TestRGBFailureMemoPalette20IsImageLocalAndRecovers(t *testing.T) {
	f := newRGBMemoRig(t)
	prefix := filepath.Join(os.Getenv("SC_RGB_MEMO_CASES"), "runtime-corpus-003/run-s800f2-n3-001/C-group-001")
	slot := f.install(prefix, 0)
	first := f.round()
	if first.Kind != FrontendReady || f.rt.HDDiagnostics().RGB.GlassSamples == 0 {
		t.Fatal("fixture must show actual accepted visible glass", first.Reason)
	}
	pal := f.rt.present.cur.palette
	pal[20] = [3]byte{135, 135, 135}
	f.rt.present.derive(PresPalette, pal, nil, false)
	bad := f.round()
	if !strings.Contains(bad.Reason, "glass frozen palette profile differs") || slot.memo[0].failed {
		t.Fatal("image-local palette H2 was not isolated", bad.Reason)
	}
	pal[20] = [3]byte{134, 134, 134}
	f.rt.present.derive(PresPalette, pal, nil, false)
	good := f.round()
	if good.Kind != FrontendReady || slot.memo[0].failed || f.rt.HDDiagnostics().RGB.GlassSamples == 0 {
		t.Fatal("new valid palette stayed scene-blocked", good.Reason)
	}
}
