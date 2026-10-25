package strikecommander

import (
	"fmt"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

// FrontendRuntimeConfig contains already-validated immutable application assets.
// Audio and PCM are optional synchronous collectors, never synthesizers inside
// callbacks. A frontend can pass the same mixer object to both interfaces.
type FrontendRuntimeConfig struct {
	HD                   *FrontendHDConfig
	Fonts                *TextFonts
	Catalog, GfxCatalog  map[string]string
	FlightCatalogs       *FlightCatalogs
	SpeechIndex          *SpeechIndex
	SpeechCatalog        map[string]string
	FontCatalogSHA       [32]byte
	CacheRoot            string
	Subtitles            bool
	SubtitleE, SubtitleT uint64
	Sparse               bool
	Audio                session.AudioObserver
	PCM                  session.SBPCMObserver
}

// FrontendRuntime owns the single normal observer assembly and prepared output.
// It never owns a Session or changes its input. Source collection is added only
// by the explicit bounded-HD setup; an unconfigured HD request fixes fallback.
type FrontendRuntime struct {
	hd      *frontendHD
	so      *SessionObserver
	present *Presenter
	output  *FrontendPresentation
	audio   session.AudioObserver
	pcm     session.SBPCMObserver
	sparse  bool
	closed  bool
}

func NewFrontendRuntime(cfg FrontendRuntimeConfig) (*FrontendRuntime, error) {
	if cfg.Fonts == nil || cfg.FlightCatalogs == nil || cfg.SpeechIndex == nil || cfg.CacheRoot == "" || cfg.FontCatalogSHA == ([32]byte{}) {
		return nil, fmt.Errorf("frontend runtime requires fonts/catalog identity, flight/speech assets and cache root")
	}
	cats := *cfg.FlightCatalogs
	labels := NewLabelObserver()
	flight := NewFlightObserver(func(k Kind, s string) (string, int, bool) { return ResolveFlight(k, s, cats) })
	flight.discoverCompletionCU = true
	subs, e := NewSubtitlesText(cfg.SpeechIndex, cfg.SpeechCatalog, cfg.Fonts, cfg.SubtitleE, cfg.SubtitleT)
	if e != nil {
		return nil, e
	}
	subs.SetEnabled(cfg.Subtitles)
	so := &SessionObserver{Labels: labels, Flight: flight, Subtitles: subs}
	pr, e := NewPresenter(PresenterConfig{Fonts: cfg.Fonts, Catalog: cfg.Catalog, GfxCatalog: cfg.GfxCatalog, FlightCatalogs: &cats, Scales: []int{2, 3}, Subtitles: cfg.Subtitles, Receipt: false, MenuRoot: cfg.CacheRoot})
	if e != nil {
		return nil, e
	}
	pr.Attach(so)
	output, e := NewFrontendPresentation(pr, cfg.FontCatalogSHA)
	if e != nil {
		return nil, e
	}
	r := &FrontendRuntime{so: so, present: pr, output: output, audio: cfg.Audio, pcm: cfg.PCM, sparse: cfg.Sparse}
	if cfg.HD != nil {
		if e = r.ConfigureHD(*cfg.HD); e != nil {
			r.Close()
			return nil, e
		}
	}
	return r, nil
}
func (r *FrontendRuntime) SessionConfig() session.Config {
	c := SessionConfig(r)
	c.SparseObserve = sparseOn(r.sparse)
	return c
}
func (r *FrontendRuntime) BeforeStep(v session.StepView) error {
	r.output.prepared.enterCallback()
	defer r.output.prepared.leaveCallback()
	e := r.so.BeforeStep(v)
	r.syncHDStatus()
	return e
}
func (r *FrontendRuntime) VideoWrite(w machine.VideoWrite) {
	r.output.prepared.enterCallback()
	defer r.output.prepared.leaveCallback()
	r.so.VideoWrite(w)
}
func (r *FrontendRuntime) Frame(b []byte, p [256][3]uint8) {
	r.output.prepared.enterCallback()
	defer r.output.prepared.leaveCallback()
	r.so.Frame(b, p)
	if r.hd != nil {
		r.hd.registry.frame(r.so.Models.shown, r.so.steps, b, p)
		r.output.sourceEpoch = r.hd.registry.epoch
		r.output.invalidate()
	}
}
func (r *FrontendRuntime) VideoModeChange(c machine.ModeChange) {
	r.output.prepared.enterCallback()
	defer r.output.prepared.leaveCallback()
	if r.hd != nil {
		r.hd.registry.invalidateAll()
	}
	presentObserver{r.so}.VideoModeChange(c)
	r.syncHDStatus()
}
func (r *FrontendRuntime) SBDMA(e machine.SBDMAEvent) {
	r.output.prepared.enterCallback()
	defer r.output.prepared.leaveCallback()
	r.so.SBDMA(e)
}
func (r *FrontendRuntime) SBPCM(e machine.SBPCMEvent) {
	r.output.prepared.enterCallback()
	defer r.output.prepared.leaveCallback()
	if r.pcm != nil {
		r.pcm.SBPCM(e)
	}
}
func (r *FrontendRuntime) OPLWrite(e machine.OPLWrite) {
	r.output.prepared.enterCallback()
	defer r.output.prepared.leaveCallback()
	if r.audio != nil {
		r.audio.OPLWrite(e)
	}
}
func (r *FrontendRuntime) SpeakerSample(e machine.SpeakerSample) {
	r.output.prepared.enterCallback()
	defer r.output.prepared.leaveCallback()
	if r.audio != nil {
		r.audio.SpeakerSample(e)
	}
}
func (r *FrontendRuntime) PITChannel2(e machine.PIT2Change) {
	r.output.prepared.enterCallback()
	defer r.output.prepared.leaveCallback()
	if r.audio != nil {
		r.audio.PITChannel2(e)
	}
}
func (r *FrontendRuntime) TriggerIPs() session.IPSet { return r.so.TriggerIPs() }
func (r *FrontendRuntime) NextWake() uint64          { return r.so.NextWake() }
func (r *FrontendRuntime) FrameAt(s uint64)          { r.so.FrameAt(s) }
func (r *FrontendRuntime) Prepare(req FrontendPrepareRequest) (*FrontendComposite, error) {
	return r.output.Prepare(req)
}
func (r *FrontendRuntime) Compose(req FrontendPrepareRequest) (*FrontendComposite, bool) {
	return r.output.Compose(req)
}
func (r *FrontendRuntime) IsCurrent(req FrontendPrepareRequest, last FrontendCompositeInfo) bool {
	return r.output.IsCurrent(req, last)
}
func (r *FrontendRuntime) Current() PresentInfo            { return r.output.Current() }
func (r *FrontendRuntime) LastVerdict() FrameVerdict       { return r.output.LastVerdict() }
func (r *FrontendRuntime) SafetyState() HDSafetyState      { return r.output.SafetyState() }
func (r *FrontendRuntime) MenuPreload() PresentMenuPreload { return r.present.MenuPreload() }
func (r *FrontendRuntime) Close() {
	if !r.closed {
		r.closed = true
		r.output.Close()
		r.present.bindHDImage = nil
		r.present.voidHDCU = nil
		r.output.composeHD = nil
		r.output.prepareHDIdentity = nil
		r.output.prepared.refineKey = nil
		if r.hd != nil {
			r.hd.close()
		}
		if r.so.Models != nil {
			r.so.Models.CloseShadeSources()
			if r.hd != nil {
				r.so.Models.Library = nil
			}
		}
	}
}

var (
	_ session.StepObserver   = (*FrontendRuntime)(nil)
	_ session.SparseObserver = (*FrontendRuntime)(nil)
	_ session.ModeObserver   = (*FrontendRuntime)(nil)
	_ session.AudioObserver  = (*FrontendRuntime)(nil)
	_ session.SBDMAObserver  = (*FrontendRuntime)(nil)
	_ session.SBPCMObserver  = (*FrontendRuntime)(nil)
)
