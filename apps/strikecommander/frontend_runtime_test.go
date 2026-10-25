package strikecommander

import (
	"github.com/wicanr2/dosgolem/internal/machine"
	"os"
	"testing"
)

type runtimeSink struct {
	r                  *FrontendRuntime
	t                  *testing.T
	opl, spk, pit, pcm int
}

func (s *runtimeSink) check() {
	s.t.Helper()
	if _, e := s.r.Prepare(FrontendPrepareRequest{Scale: 2, Language: "en", SettingsVersion: 1}); e == nil {
		s.t.Fatal("audio callback admitted first Prepare")
	}
}
func (s *runtimeSink) OPLWrite(machine.OPLWrite)           { s.opl++; s.check() }
func (s *runtimeSink) SpeakerSample(machine.SpeakerSample) { s.spk++; s.check() }
func (s *runtimeSink) PITChannel2(machine.PIT2Change)      { s.pit++; s.check() }
func (s *runtimeSink) SBPCM(machine.SBPCMEvent)            { s.pcm++; s.check() }
func runtimeTestConfig(t *testing.T) FrontendRuntimeConfig {
	cats := flightTestCats()
	return FrontendRuntimeConfig{Fonts: flightTextFonts(t), FlightCatalogs: &cats, SpeechIndex: &SpeechIndex{}, FontCatalogSHA: [32]byte{1}, CacheRoot: t.TempDir()}
}
func TestFrontendRuntimeAssemblyForwardsAllOptionalCallbacks(t *testing.T) {
	sink := &runtimeSink{t: t}
	cfg := runtimeTestConfig(t)
	cfg.Audio = sink
	cfg.PCM = sink
	cfg.Sparse = true
	r, e := NewFrontendRuntime(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	sink.r = r
	if r.so.Flight == nil || r.so.Subtitles == nil || r.present.cfg.Receipt || len(r.present.ovs) != 2 || len(r.present.fovs) != 2 || r.so.present != r.present {
		t.Fatal("normal observer assembly incomplete")
	}
	if c := r.SessionConfig(); c.Observer != r || !c.SparseObserve || !c.Mode13Phase || !c.KbdIntercept || !c.SoundBlaster {
		t.Fatal("normal SessionConfig changed")
	}
	r.OPLWrite(machine.OPLWrite{})
	r.SpeakerSample(machine.SpeakerSample{})
	r.PITChannel2(machine.PIT2Change{})
	r.SBPCM(machine.SBPCMEvent{})
	if sink.opl != 1 || sink.spk != 1 || sink.pit != 1 || sink.pcm != 1 {
		t.Fatal("audio event dropped")
	}
	r.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: 17})
	if r.present.counts.ModeChanges != 1 || !r.present.needFallback {
		t.Fatal("mode did not reach real Presenter")
	}
	if r.output.prepared.callbackDepth != 0 {
		t.Fatal("callback guard leaked")
	}
}
func TestFrontendRuntimeFrameUsesRealVerdictAndExplicitPreparation(t *testing.T) {
	r, e := NewFrontendRuntime(runtimeTestConfig(t))
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	frame := make([]byte, 64000)
	frame[0] = 7
	var palette [256][3]byte
	palette[7] = [3]byte{11, 19, 31}
	r.FrameAt(123)
	r.Frame(frame, palette)
	if r.LastVerdict().Steps != 123 || !r.LastVerdict().FrameComplete {
		t.Fatal("dense/sparse frame step or completion lost", r.LastVerdict())
	}
	req := FrontendPrepareRequest{Scale: 2, Language: "en", SettingsVersion: 1}
	q, e := r.Prepare(req)
	if e != nil {
		t.Fatal(e)
	}
	info, e := q.Info()
	if e != nil {
		t.Fatal(e)
	}
	dst := make([]byte, 640*400*4)
	if e = q.CopyRGBA(dst); e != nil {
		t.Fatal(e)
	}
	q.Release()
	if dst[0] != 11 || dst[1] != 19 || dst[2] != 31 || dst[3] != 255 {
		t.Fatal("wrong no-image safety source")
	}
	if !r.IsCurrent(req, info) {
		t.Fatal("caller buffer cannot be retained")
	}
	if _, ok := r.Compose(req); ok {
		t.Fatal("engine held a third persistent RGBA")
	}
	r.Close()
	if _, e = r.Prepare(req); e == nil {
		t.Fatal("closed runtime prepared")
	}
}

func TestFrontendConfigureHDRealAssetsQuotaAndClose(t *testing.T) {
	root := os.Getenv("SC_MODEL_ROOT")
	if root == "" {
		t.Skip("explicit original read-only assets required")
	}
	cfg := runtimeTestConfig(t)
	cfg.HD = &FrontendHDConfig{GameRoot: root, StateSHA: [32]byte{1}, AppGeneration: 1}
	r, e := NewFrontendRuntime(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if r.so.Models == nil || r.so.Models.bounded == nil || r.so.Models.onHDSealed == nil || r.output.composeHD == nil {
		t.Fatal("bounded collector/real materializer not wired")
	}
	o, p := r.hd.owner, r.hd.pool
	if o.OwnedBytes() == 0 || o.OwnedBytes() > o.TotalBytes() {
		t.Fatal("invalid real global ledger", o.OwnedBytes())
	}
	before := r.ProducerUsage()
	if before.LegacyOwned == 0 || before.LegacyLimit != 48<<20 || before.ExtraLimit != 16<<20 {
		t.Fatal(before)
	}
	r.Close()
	if r.so.Models.onHDSealed != nil || r.present.voidHDCU != nil || r.output.prepared.refineKey != nil {
		t.Fatal("Close kept producer/registry callback aliases")
	}
	if r.HDTotalOwned() != o.OwnedBytes() || r.HDTotalOwned() != 0 {
		t.Fatal("closed actual owner nonzero", r.HDTotalOwned(), o.OwnedBytes())
	}
	if got := r.ProducerUsage(); got != p.Usage() || got.LegacyOwned != 0 || got.ExtraOwned != 0 {
		t.Fatal("closed actual producer not zero", got)
	}
}

func TestFrontendHDInitialNoImageAndInactiveKeysNeverMaterialize(t *testing.T) {
	root := os.Getenv("SC_MODEL_ROOT")
	if root == "" {
		t.Skip("explicit original read-only assets required")
	}
	cfg := runtimeTestConfig(t)
	cfg.HD = &FrontendHDConfig{GameRoot: root, StateSHA: [32]byte{1}, AppGeneration: 1}
	r, e := NewFrontendRuntime(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	for _, hd := range []bool{false, true} {
		for _, n := range []int{2, 3} {
			req := FrontendPrepareRequest{Scale: n, Language: "en", HD: hd, SettingsVersion: 1}
			l, e := r.Prepare(req)
			if e != nil {
				t.Fatal("initial no-image preparation", hd, n, e)
			}
			info, e := l.Info()
			if e != nil || info.Kind != FrontendSafetyFallback {
				t.Fatal("initial no-image wrong safety result", info, e)
			}
			l.Release()
		}
	}
	// A valid bound source deliberately has no materializer library. Any hidden
	// identity work in HD-off or safety fallback would panic or set failed/digest.
	c := registryCapture(t, r.hd.registry, 99)
	img := &presentImage{info: PresentInfo{Kind: PresCU}}
	r.hd.registry.bind(img, c.CUKey)
	r.present.cur = img
	s, e := r.hd.registry.lookup(img.hdBinding)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		hd     bool
		safety HDEligibility
	}{{false, HDEligible}, {true, HDNoImage}, {true, HDStale}} {
		k := prepareKey(1, 2)
		k.HD = tc.hd
		k.Safety.Eligibility = tc.safety
		before := r.HDTotalOwned()
		r.prepareHDIdentity(&k)
		if s.hasDigest || s.memo[0].failed || s.source != nil || r.HDTotalOwned() != before || k.SourceKey != ([32]byte{}) {
			t.Fatal("inactive key did hidden source work", tc)
		}
	}
}
