package strikecommander

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

type FrontendPrepareRequest struct {
	Scale           int
	Language        string
	HD, Subtitles   bool
	SettingsVersion uint64
}
type FrontendCompositeKind uint8

const (
	FrontendNotPrepared FrontendCompositeKind = iota
	FrontendReady
	FrontendFixedFallback
	FrontendSafetyFallback
)

type FrontendCompositeInfo struct {
	key    hdPrepareKey
	Kind   FrontendCompositeKind
	Image  PresentInfo
	Safety HDSafetyState
	Scale  int
	SHA256 [32]byte
	Reason string
}

// FrontendComposite is an explicit read lease. CopyRGBA writes only into the
// frontend's own preallocated framebuffer; Release ends access to this lease.
type FrontendComposite struct {
	lease *hdPreparedLease
	info  FrontendCompositeInfo
}

func (c *FrontendComposite) Info() (FrontendCompositeInfo, error) {
	if _, e := c.lease.Info(); e != nil {
		return FrontendCompositeInfo{}, e
	}
	return c.info, nil
}
func (c *FrontendComposite) CopyRGBA(dst []byte) error { return c.lease.CopyRGBA(dst) }
func (c *FrontendComposite) Release() {
	if c != nil {
		c.lease.Release()
	}
}

// FrontendPresentation joins the014 immutable image and16 safety source to
// explicit callback-external preparation. It neither owns nor advances Session.
type FrontendPresentation struct {
	presenter                                  *Presenter
	prepared                                   *hdPreparation
	safety                                     hdSafetyTracker
	fontCatalogSHA                             [32]byte
	settingsVersion                            uint64
	sourceSerial, sourceEpoch                  uint64
	frameSubtitle                              Subtitle
	frameSubtitlePresent, frameSubtitleBlocked bool
	// The full3b application bridge supplies this after bounded capture is enabled.
	// It composes Overlay -> HD -> Flight -> check -> subtitle in one operation.
	composeHD         func(FrontendPrepareRequest, []byte) (hdPreparedKind, string, error)
	fixedFallbackHD   func(FrontendPrepareRequest, []byte, string) (hdPreparedKind, string, error)
	prepareHDIdentity func(*hdPrepareKey)
	menu              *frontendMenuPolicy
}

func NewFrontendPresentation(p *Presenter, fontCatalogSHA [32]byte) (*FrontendPresentation, error) {
	if p == nil || p.frontend != nil || p.counts.Frames != 0 || fontCatalogSHA == ([32]byte{}) {
		return nil, fmt.Errorf("frontend presentation must be configured before first Frame with asset identity")
	}
	f := &FrontendPresentation{presenter: p, prepared: newHDPreparation(), fontCatalogSHA: fontCatalogSHA}
	f.prepared.maxBuffers = 1
	f.prepared.consumeOnRelease = true
	p.frontend = f
	p.OnCapture(func(info PresentInfo) { f.safety.capture(info.Seq, true); f.invalidate() })
	f.safety.capture(p.Current().Seq, p.cur != nil)
	f.invalidate()
	return f, nil
}
func (f *FrontendPresentation) Current() PresentInfo       { return f.presenter.Current() }
func (f *FrontendPresentation) LastVerdict() FrameVerdict  { return f.presenter.LastVerdict() }
func (f *FrontendPresentation) SafetyState() HDSafetyState { return f.safety.state }
func (f *FrontendPresentation) validity() hdValidity {
	return hdValidity{Seq: f.Current().Seq, SafetyVersion: f.safety.state.Version, SourceSerial: f.sourceSerial, SourceEpoch: f.sourceEpoch, SettingsVersion: f.settingsVersion}
}
func (f *FrontendPresentation) invalidate() { f.prepared.invalidate(f.validity()) }
func cloneFrontendSubtitle(s Subtitle) Subtitle {
	v := s
	v.Lines = make(map[int][]SubLine, len(s.Lines))
	for n, a := range s.Lines {
		v.Lines[n] = append([]SubLine(nil), a...)
	}
	v.Forced = make(map[int]bool, len(s.Forced))
	for k, x := range s.Forced {
		v.Forced[k] = x
	}
	v.Truncated = make(map[int]bool, len(s.Truncated))
	for k, x := range s.Truncated {
		v.Truncated[k] = x
	}
	return v
}
func (f *FrontendPresentation) onVerdict(v FrameVerdict, indexed []byte, pal [256][3]uint8) {
	f.safety.verdict(v, indexed, pal)
	if v.FrameComplete && len(indexed) == 64000 {
		if f.menu != nil {
			f.menu.complete(f.safety.state.FrameCompleteKey, indexed)
		}
		sub, blocked := f.presenter.subState(indexed)
		f.frameSubtitlePresent = sub != nil
		f.frameSubtitleBlocked = blocked
		if sub != nil {
			f.frameSubtitle = cloneFrontendSubtitle(*sub)
		} else {
			f.frameSubtitle = Subtitle{}
		}
	}
	f.invalidate()
}
func (f *FrontendPresentation) Prepare(req FrontendPrepareRequest) (*FrontendComposite, error) {
	if f.menu != nil {
		f.menu.touched = true
	}
	f.prepared.mu.Lock()
	blocked := f.prepared.closed || f.prepared.callbackDepth != 0 || f.prepared.preparing
	f.prepared.mu.Unlock()
	if blocked {
		return nil, fmt.Errorf("NotPrepared")
	}
	if req.Scale != 2 && req.Scale != 3 {
		return nil, fmt.Errorf("frontend scale")
	}
	if req.Language == "" || len(req.Language) > 64 {
		return nil, fmt.Errorf("frontend language")
	}
	if req.SettingsVersion == 0 {
		return nil, fmt.Errorf("frontend settings generation")
	}
	f.settingsVersion = req.SettingsVersion
	f.invalidate()
	img := f.presenter.cur
	info := f.Current()
	safety := f.safety.state
	finalizer, e := f.selectMenuFinalizer(img, safety, req.Scale)
	if e != nil {
		return nil, e
	}
	k := hdPrepareKey{Validity: f.validity(), Safety: safety, Scale: req.Scale, HD: req.HD, Subtitles: req.Subtitles, Language: req.Language, FontCatalogSHA: f.fontCatalogSHA}
	if img != nil {
		k.IndexedSHA = sha256.Sum256(img.indexed)
		k.PaletteSHA = hashFrontendPalette(img.palette)
		sub, _ := json.Marshal(struct {
			Key, Text       string
			Start, End      uint64
			HasEnd, Blocked bool
		}{img.sub.Key, img.sub.Text, img.sub.Start, img.sub.End, img.sub.HasEnd, img.subBlocked})
		k.SubtitleSHA = sha256.Sum256(sub)
	}
	if req.HD && img != nil && safety.Eligibility == HDEligible && f.prepareHDIdentity != nil {
		f.prepareHDIdentity(&k)
	}
	if safety.Eligibility != HDEligible {
		k.IndexedSHA = sha256.Sum256(f.safety.frame[:])
		k.PaletteSHA = hashFrontendPalette(f.safety.palette)
	}
	produce := func(dst []byte) (hdPreparedKind, string, error) {
		english := req.Language == "en"
		if safety.Eligibility != HDEligible || img == nil {
			scaleFrontendInto(dst, f.safety.frame[:], f.safety.palette, req.Scale)
			if req.Subtitles && !english && f.presenter.subs != nil && f.frameSubtitlePresent && !f.frameSubtitleBlocked {
				v := cloneFrontendSubtitle(f.frameSubtitle)
				f.presenter.subs.drawText(&v, dst, req.Scale)
			}
			return hdSafetyFallback, "014 safety fallback", nil
		}
		if req.HD && f.composeHD != nil {
			return f.composeHD(req, dst)
		}
		if english {
			scaleFrontendInto(dst, img.indexed, img.palette, req.Scale)
		} else {
			f.presenter.composeImageInto(img, req.Scale, req.Subtitles, dst)
		}
		if req.HD {
			return hdFixedFallback, "HD scene source not configured", nil
		}
		return hdReady, "", nil
	}
	rebuild := func(dst []byte, reason string) (hdPreparedKind, string, error) {
		// This is the already-published failure for this exact key. Recompose only
		// its immutable 2D fallback, never retry the HD producer.
		if img == nil || safety.Eligibility != HDEligible || !req.HD {
			return hdNotPrepared, "", fmt.Errorf("NotPrepared: fixed fallback superseded")
		}
		if f.fixedFallbackHD != nil {
			return f.fixedFallbackHD(req, dst, reason)
		}
		if req.Language == "en" {
			scaleFrontendInto(dst, img.indexed, img.palette, req.Scale)
		} else {
			f.presenter.composeImageInto(img, req.Scale, req.Subtitles, dst)
		}
		return hdFixedFallback, reason, nil
	}
	if f.menu != nil {
		baseProduce, baseRebuild := produce, rebuild
		produce = func(dst []byte) (hdPreparedKind, string, error) {
			kind, reason, err := baseProduce(dst)
			return finalizer.finish(dst, kind, reason, err)
		}
		rebuild = func(dst []byte, reason string) (hdPreparedKind, string, error) {
			kind, why, err := baseRebuild(dst, reason)
			return finalizer.finish(dst, kind, why, err)
		}
	}
	lease, e := f.prepared.prepareWithFixedFallback(k, produce, rebuild)
	if e != nil {
		return nil, e
	}
	v, e := lease.Info()
	if e != nil {
		lease.Release()
		return nil, e
	}
	return &FrontendComposite{lease: lease, info: FrontendCompositeInfo{key: v.Key, Kind: FrontendCompositeKind(v.Kind), Image: info, Safety: safety, Scale: req.Scale, SHA256: v.SHA, Reason: v.Reason}}, nil
}
func (f *FrontendPresentation) Compose(req FrontendPrepareRequest) (*FrontendComposite, bool) {
	// No hashing, drawing or first-time render in this lookup. Prepare fixes the
	// current complete key; callbacks can only acquire an already-ready outcome.
	f.prepared.mu.Lock()
	defer f.prepared.mu.Unlock()
	p := f.prepared
	if p.closed || p.readers >= 2 {
		return nil, false
	}
	for i := range p.entries {
		e := &p.entries[i]
		k := e.info.Key
		if e.ready && len(e.rgba) != 0 && e.current && k.Validity == f.validity() && k.Scale == req.Scale && k.Language == req.Language && k.HD == req.HD && k.Subtitles == req.Subtitles && k.Validity.SettingsVersion == req.SettingsVersion {
			e.readers++
			p.readers++
			return &FrontendComposite{lease: &hdPreparedLease{owner: p, entry: e}, info: FrontendCompositeInfo{key: k, Kind: FrontendCompositeKind(e.info.Kind), Image: f.Current(), Safety: f.safety.state, Scale: req.Scale, SHA256: e.info.SHA, Reason: e.info.Reason}}, true
		}
	}
	return nil, false
}
func (f *FrontendPresentation) Close() {
	f.prepared.close()
	if f.menu != nil {
		f.menu.close()
		f.menu = nil
	}
	f.fixedFallbackHD = nil
	if f.presenter.frontend == f {
		f.presenter.frontend = nil
	}
}
func hashFrontendPalette(p [256][3]uint8) [32]byte {
	var raw [768]byte
	for i, c := range p {
		copy(raw[i*3:], c[:])
	}
	return sha256.Sum256(raw[:])
}

// IsCurrent lets the frontend retain its own sole RGBA when the full display
// request remains unchanged. It never renders or acquires a buffer lease.
func (f *FrontendPresentation) IsCurrent(req FrontendPrepareRequest, info FrontendCompositeInfo) bool {
	k := info.key
	return k.Validity == f.validity() && k.Safety == f.safety.state && k.Scale == req.Scale && k.Language == req.Language && k.HD == req.HD && k.Subtitles == req.Subtitles && k.Validity.SettingsVersion == req.SettingsVersion
}
