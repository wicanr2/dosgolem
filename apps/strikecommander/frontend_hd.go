package strikecommander

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/opaque"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
	"github.com/wicanr2/dosgolem/xlate"
	"io"
	"os"
	"path/filepath"
)

type FrontendHDConfig struct {
	GameRoot                            string
	StateSHA                            [32]byte
	AppGeneration                       uint64
	TotalBytes, LegacyBytes, ExtraBytes uint64
}
type FrontendHDDiagnostics struct {
	CUKey, Serial  uint64
	Scale          int
	Reason         string
	Report         opaque.Report
	RGB            opaque.CompositionDiagnostics
	ReplacedPixels uint64
	Check          CompositeCheck
}
type frontendHD struct {
	prepared           *hdPreparation
	vga                bool
	owner              *shadesource.Owner
	pool               *shadesource.ProtectedProducer
	registry           *hdSceneRegistry
	renderer           *opaque.CompositionRenderer
	models             *ModelRecorder
	tables             hd3d.Tables
	state, exe, assets [32]byte
	app                uint64
	rgbaReservation    *shadesource.OwnedReservation
	tableReservation   *shadesource.OwnedReservation
	generation         uint64
	valid              bool
	diagnostic         FrontendHDDiagnostics
}

// ConfigureHD is explicit and must finish before the first observation/Prepare.
// Both HD-off and HD-on runs use this same bounded collector configuration.
func (r *FrontendRuntime) ConfigureHD(cfg FrontendHDConfig) (err error) {
	if r.closed || r.hd != nil || r.present.counts.Frames != 0 || r.so.steps != 0 || cfg.GameRoot == "" || cfg.StateSHA == ([32]byte{}) || cfg.AppGeneration == 0 {
		return fmt.Errorf("HD configuration requires unused Runtime and actual state/app identity")
	}
	exe, e := os.ReadFile(filepath.Join(cfg.GameRoot, "STRIKE.EXE"))
	if e != nil {
		return e
	}
	tables, e := hd3d.TablesFromEXE(exe)
	if e != nil {
		return e
	}
	lib, e := realspace.LoadLibrary(cfg.GameRoot)
	if e != nil {
		return e
	}
	materials, e := opaque.LoadMaterialLibrary(cfg.GameRoot)
	if e != nil {
		return e
	}
	models := NewSceneRecorder(lib, tables)
	models.EnableMaterialSources()
	if e = models.EnableShadeSourceCapture(exe, cfg.TotalBytes); e != nil {
		return e
	}
	committed := false
	var h *frontendHD
	defer func() {
		if !committed {
			if h != nil {
				h.close()
			}
			models.CloseShadeSources()
		}
	}()
	if e = models.EnableBoundedHDCapture(cfg.LegacyBytes, cfg.ExtraBytes); e != nil {
		return e
	}
	owner := models.ShadeSourceOwner()
	// These R6 bytes belong to the consumer, independently of recorder Close.
	tableHold, e := owner.ReserveOwned(uint64(cap(tables.R6Table)))
	if e != nil {
		return e
	}
	h = &frontendHD{owner: owner, models: models, tables: tables, state: cfg.StateSHA, exe: sha256.Sum256(exe), assets: materials.Key(), app: cfg.AppGeneration, tableReservation: tableHold}
	registry, e := newHDSceneRegistry(owner, models.bounded.pool, lib, tables)
	if e != nil {
		return e
	}
	h.registry = registry
	h.pool = registry.pool
	renderer, e := opaque.NewCompositionRenderer(lib, materials, tables, owner)
	if e != nil {
		return e
	}
	h.renderer = renderer
	// One engine-ready RGBA plus the frontend's one preallocated N3 backing.
	rgba, e := owner.ReserveOwned(2 * 320 * 200 * 3 * 3 * 4)
	if e != nil {
		return e
	}
	h.rgbaReservation = rgba
	h.prepared = r.output.prepared
	h.prepared.onDrained = func() {
		if h.rgbaReservation != nil {
			h.rgbaReservation.Release()
			h.rgbaReservation = nil
		}
	}
	models.onHDSealed = registry.onSealed
	models.onHDRejected = registry.onRejected
	r.so.Models = models
	r.hd = h
	r.present.bindHDImage = func(img *presentImage) { r.bindHDImage(img) }
	r.present.voidHDCU = registry.invalidateCU
	r.output.composeHD = r.composeHD
	r.output.fixedFallbackHD = r.fallbackHD
	r.output.prepareHDIdentity = r.prepareHDIdentity
	h.prepared.refineKey = r.prepareHDIdentity
	committed = true
	return nil
}

// Preparation binds the immutable CU token and actual adopted extra declaration.
// This runs after callbacks, never while a machine StepView is live.
func (r *FrontendRuntime) prepareHDIdentity(k *hdPrepareKey) {
	if !k.HD || k.Safety.Eligibility != HDEligible || r.present.cur == nil || r.hd == nil || r.hd.registry == nil {
		return
	}
	h := r.hd
	k.SourceProfile = shadesource.ProtectedProducerVersion
	k.PolicyVersion = HDPolicyVersion
	k.VisibilityVersion = opaque.VisibilityRuleVersion
	k.CompositionVersion = opaque.CompositionRuleVersion
	k.GlassVersion = opaque.GlassProfileVersion
	u := h.pool.Usage()
	k.LegacyLimit, k.ExtraLimit = u.LegacyLimit, u.ExtraLimit
	s, e := h.registry.lookup(r.present.cur.hdBinding)
	if e != nil {
		return
	}
	if !s.hasDigest && !s.memo[k.Scale-2].failed {
		m, x := h.registry.materialize(s)
		if x != nil {
			s.memo[k.Scale-2].failed = true
			s.memo[k.Scale-2].reason = x.Error()
		} else {
			d, x := h.owner.BoundedLegacyDigest(m.scene)
			if x == nil {
				s.digest = d
				s.hasDigest = true
				r.qualifyHDSource(s, m.scene)
			} else {
				s.memo[k.Scale-2].failed = true
				s.memo[k.Scale-2].reason = x.Error()
			}
			m.release()
		}
	}
	if s.hasDigest {
		k.SourceKey = s.digest
	}
	memo := &s.memo[k.Scale-2]
	k.NoExtra = memo.success && memo.noExtra
	k.TableEpoch = memo.tableVersion
	if s.source != nil && !k.NoExtra {
		d := s.source.Declaration()
		k.ExtraKey = [32]byte(d.DeclaredSourceDigest)
	} else {
		k.ExtraKey = [32]byte{}
	}

}
func (r *FrontendRuntime) qualifyHDSource(s *hdRegistrySlot, scene model.Input) {

	if s.source == nil && s.raw != nil {
		accepted, x := s.raw.ShareValidated(scene)
		if x != nil {
			s.hasSourceFailure = true
			s.sourceFailure = opaqueSourceFailure{status: shadesource.H3, reason: shadesource.ResourceLimit}
			if q, ok := x.(*shadesource.Error); ok {
				s.sourceFailure = opaqueSourceFailure{q.Status, q.Reason}
			}
		} else {
			s.source = accepted
			s.raw = nil
		}
	}
}
func (h *frontendHD) close() {
	if h.models != nil {
		h.models.onHDSealed = nil
		h.models.onHDRejected = nil
	}
	if h.renderer != nil {
		_ = h.renderer.Close()
		h.renderer = nil
	}
	if h.registry != nil {
		h.registry.close()
		h.registry = nil
	}
	if h.prepared == nil && h.rgbaReservation != nil {
		h.rgbaReservation.Release()
		h.rgbaReservation = nil
	}
	h.tables = hd3d.Tables{}
	if h.tableReservation != nil {
		h.tableReservation.Release()
		h.tableReservation = nil
	}
}
func (r *FrontendRuntime) bindHDImage(img *presentImage) {
	h := r.hd
	if h == nil {
		return
	}
	// Publication may occur within SessionObserver.BeforeStep, before its outer
	// runtime sync. Revoke stale qualifications before binding the new image.
	r.syncHDStatus()
	key := uint64(0)
	if (img.info.Kind == PresCU || img.info.Kind == PresCUReturn) && r.so != nil && r.so.Flight != nil && r.so.Models != nil {
		key = img.sourceCU
		f, m := r.so.Flight.cu.Entry(), r.so.Models.tracker.Entry()
		fb, fok := r.so.Flight.Base()
		mb, mok := r.so.Models.Base()
		if key == 0 || f.Steps != key || m.Steps != key || !fok || !mok || fb != mb {
			key = 0
		}
	}
	h.registry.bind(img, key)
	r.output.sourceSerial = img.hdBinding.serial
	r.output.sourceEpoch = h.registry.epoch
	r.output.invalidate()
}
func (r *FrontendRuntime) syncHDStatus() {
	h := r.hd
	if h == nil {
		return
	}
	// Use the same observer assembly that validates CU bindings. The h.models
	// reference belongs to the HD collector's shutdown lifecycle.
	if r.so == nil || r.so.Models == nil || r.so.Flight == nil {
		h.registry.invalidateAll()
		r.output.sourceEpoch = h.registry.epoch
		r.output.invalidate()
		return
	}
	models := r.so.Models
	gen, valid := models.MaterialSourceStatus()
	h.registry.refreshRevocations()
	if !h.vga && models.vga {
		h.registry.invalidateAll()
	}
	h.vga = models.vga
	if h.registry.image >= 0 {
		fb, fo := r.so.Flight.Base()
		mb, mo := models.Base()
		if !fo || !mo || fb != mb {
			h.registry.invalidateAll()
		}
	}
	if h.valid && (!valid || gen != h.generation) {
		h.registry.invalidateAll()
	}
	h.generation, h.valid = gen, valid
	r.output.sourceEpoch = h.registry.epoch
	r.output.invalidate()
}
func (r *FrontendRuntime) HDDiagnostics() FrontendHDDiagnostics {
	if r.hd == nil {
		return FrontendHDDiagnostics{Reason: "HD not configured"}
	}
	return r.hd.diagnostic
}
func (r *FrontendRuntime) ProducerUsage() shadesource.ProducerUsage {
	if r.hd == nil {
		return shadesource.ProducerUsage{}
	}
	return r.hd.pool.Usage()
}
func (r *FrontendRuntime) HDTotalOwned() uint64 {
	if r.hd == nil {
		return 0
	}
	return r.hd.owner.OwnedBytes()
}
func (r *FrontendRuntime) fallbackHD(req FrontendPrepareRequest, dst []byte, reason string) (hdPreparedKind, string, error) {
	img := r.present.cur
	if req.Language == "en" {
		scaleFrontendInto(dst, img.indexed, img.palette, req.Scale)
	} else {
		r.present.composeImageInto(img, req.Scale, req.Subtitles, dst)
	}
	r.hd.diagnostic = FrontendHDDiagnostics{Reason: reason, Scale: req.Scale}
	return hdFixedFallback, reason, nil
}
func (r *FrontendRuntime) composeHD(req FrontendPrepareRequest, dst []byte) (hdPreparedKind, string, error) {
	h := r.hd
	img := r.present.cur
	s, e := h.registry.lookup(img.hdBinding)
	if e != nil {
		return r.fallbackHD(req, dst, e.Error())
	}
	memo := &s.memo[req.Scale-2]
	if memo.failed {
		return r.fallbackHD(req, dst, memo.reason)
	}
	if !s.capture.SignatureValid {
		return r.fallbackHD(req, dst, "H4 primary source signature invalid")
	}
	m, e := h.registry.materialize(s)
	if e != nil {
		return r.fallbackHD(req, dst, e.Error())
	}
	defer m.release()
	if !m.group.ReplayReady {
		return r.fallbackHD(req, dst, "H3 original1x not repaintable")
	}
	digest, e := h.owner.BoundedLegacyDigest(m.scene)
	if e != nil {
		return r.fallbackHD(req, dst, e.Error())
	}
	s.digest, s.hasDigest = digest, true
	r.qualifyHDSource(s, m.scene)
	id := opaque.SceneIdentity{EXE: h.exe, Assets: h.assets, State: h.state, Group: digest, AppGeneration: h.app, SignatureGeneration: s.capture.SignatureGeneration, CUKey: s.capture.CUKey}
	kind := opaque.CaptureVGA
	if img.info.Kind == PresCU {
		kind = opaque.CaptureBufCU
	}
	capture := opaque.CaptureIdentity{Scene: id, Kind: kind, Steps: img.info.CaptureSteps, SourceCS: img.info.SourceCS, SourceIP: img.info.SourceIP, IndexedSHA: sha256.Sum256(img.indexed), OriginalPaletteSHA: opaque.PaletteHash(img.palette)}
	palette, e := opaque.NewPaletteSnapshot(id, capture, img.palette)
	if e != nil {
		return r.fallbackHD(req, dst, e.Error())
	}
	input := opaque.CompositionInput{Opaque: opaque.Input{Identity: id, Capture: &capture, Scene: m.scene, SignatureValid: s.capture.SignatureValid, ReadFailed: m.group.ReadFailed}, SourceEpoch: s.capture.ShadeEpoch}
	{
		input.Completion = &opaque.CompletionImageEvidence{Indexed: img.indexed, Palette: img.palette, Steps: img.info.CaptureSteps, CUKey: s.capture.CUKey, SourceCS: img.info.SourceCS, SourceIP: img.info.SourceIP}
	}
	if s.hasSourceFailure {
		gate := opaque.H3
		if s.sourceFailure.status == shadesource.H4 {
			gate = opaque.H4
		} else if s.sourceFailure.status == shadesource.H2 {
			gate = opaque.H2
		}
		input.SourceError = &opaque.Failure{Gate: gate, Object: -1, Face: -1, Pass: -1, Reason: fmt.Sprintf("source reason%d", s.sourceFailure.reason)}
	}
	n := 320 * 200 * req.Scale * req.Scale
	// Actual RGB/mask scratch is explicitly reserved before allocation. Retained
	// ready frames keep their separate two-buffer reservation.
	work, e := h.owner.ReserveOwned(uint64(n * 10))
	if e != nil {
		return r.fallbackHD(req, dst, e.Error())
	}
	defer work.Release()
	rgb, mask := make([]byte, n*4), make([]byte, n)
	applied, plain := make([]byte, n), make([]byte, n*4)
	scaleFrontendInto(plain, img.indexed, img.palette, req.Scale)
	var report opaque.Report
	var diagnostics opaque.CompositionDiagnostics
	render := func(source opaque.ShadeSource) error {
		input.Source = source
		snapshot, e := h.renderer.Accept(input)
		if e != nil {
			memo.failed = true
			memo.reason = e.Error()
			return e
		}
		defer snapshot.Release()
		result, e := snapshot.Render(req.Scale)
		if e != nil {
			memo.failed = true
			memo.reason = e.Error()
			return e
		}
		hash := result.Hash()
		if memo.success && memo.hash != hash {
			memo.failed = true
			memo.reason = "H3 core recomputed with different SHA"
			return fmt.Errorf("%s", memo.reason)
		}
		memo.success = true
		memo.hash = hash
		memo.noExtra = result.NoExtra()
		memo.tableVersion = result.TableVersion()
		report = result.Report()
		diagnostics, e = result.WriteRGBAWithMask(palette, rgb, mask)
		// This structural ambiguity depends only on the immutable scene and N.
		// Palette applicability and all other RGB errors keep their own scope.
		var failure opaque.Failure
		if errors.As(e, &failure) && failure.Gate == opaque.H3 && failure.Reason == "MixedFallbackUnknown" {
			memo.failed = true
			memo.reason = e.Error()
		}
		return e
	}
	if s.source != nil {
		e = s.source.WithCompositionSource(render)
	} else {
		e = render(nil)
	}
	if e != nil {
		return r.fallbackHD(req, dst, e.Error())
	}
	// Overlay first; only an entirely untouched original N*N block can take HD.
	if req.Language == "en" {
		scaleFrontendInto(dst, img.indexed, img.palette, req.Scale)
	} else {
		scaleFrontendInto(dst, img.indexed, img.palette, req.Scale)
		ov := r.present.ovs[req.Scale]
		ov.Sync(img.labels, img.labelGen)
		ov.drawOverlayInto(dst, img.indexed, img.palette)
	}
	replaced := uint64(0)
	box := s.policy.viewport
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			at := y*320 + x
			if img.indexed[at] != m.group.Buf3D[at] {
				continue
			}
			original := img.palette[img.indexed[at]]
			untouched := true
			for j := 0; j < req.Scale && untouched; j++ {
				for i := 0; i < req.Scale; i++ {
					q := 4 * ((y*req.Scale+j)*320*req.Scale + x*req.Scale + i)
					if dst[q] != original[0] || dst[q+1] != original[1] || dst[q+2] != original[2] || dst[q+3] != 255 {
						untouched = false
						break
					}
				}
			}
			if !untouched {
				continue
			}
			for j := 0; j < req.Scale; j++ {
				for i := 0; i < req.Scale; i++ {
					q := (y*req.Scale+j)*320*req.Scale + x*req.Scale + i
					if mask[q] != 0 {
						copy(dst[4*q:4*q+4], rgb[4*q:4*q+4])
						replaced++
						applied[q] = 1
					}
				}
			}
		}
	}
	if req.Language != "en" {
		if fov := r.present.fovs[req.Scale]; fov != nil {
			fov.Sync(img.flight, img.presGen)
			fov.drawHD(dst, img.indexed, img.palette, &hdFlightBackground{buf: m.group.Buf3D, rgb: rgb, qualified: mask, applied: applied})
		}

	}

	copy(rgb, dst)
	for i, on := range applied {
		if on != 0 {
			copy(rgb[4*i:4*i+4], plain[4*i:4*i+4])
		}
	}
	var boxes []xlate.AlphaRect
	var stamps []*xlate.Stamp
	var art []ArtRect
	if req.Language != "en" {
		ov := r.present.ovs[req.Scale]
		boxes = ov.TextBoxes()
		stamps, art = ov.Stamps(), ov.Art()
	}
	var covers []FlightCover
	if fov := r.present.fovs[req.Scale]; req.Language != "en" && fov != nil {
		boxes = append(boxes, fov.TextBoxes()...)
		covers = fov.Covers()
	}
	check := CheckCompositeMask(rgb, plain, req.Scale, stamps, art, boxes, nil, covers...)
	if check.Outside != 0 {
		return r.fallbackHD(req, dst, "H3 final composite outside actual HD/text mask")
	}
	if req.Language != "en" {
		if req.Subtitles && r.present.subs != nil && img.subPtr != nil && !img.subBlocked && img.sub.Text != "" {
			v := img.sub
			r.present.subs.drawText(&v, dst, req.Scale)
		}
	}
	h.diagnostic = FrontendHDDiagnostics{CUKey: s.capture.CUKey, Serial: s.capture.Serial, Scale: req.Scale, Report: report, RGB: diagnostics, ReplacedPixels: replaced, Check: check}
	return hdReady, "", nil
}

// CurrentHDSource returns only the already-frozen binding token. It is safe in
// OnCapture and never hashes, materializes or acquires a new source.
func (r *FrontendRuntime) CurrentHDSource() (cuKey, serial uint64, ok bool) {
	if r.hd == nil || r.present.cur == nil {
		return 0, 0, false
	}
	b := r.present.cur.hdBinding
	s, e := r.hd.registry.lookup(b)
	if e != nil {
		return 0, 0, false
	}
	return s.capture.CUKey, s.capture.Serial, true
}
func (r *FrontendRuntime) OnCapture(fn func(PresentInfo)) { r.present.OnCapture(fn) }

// WriteHDScene is a controlled receipt stream, outside every callback. It never
// returns ModelGroup, model.Input, Packet, or raw slices to the caller.
func (r *FrontendRuntime) WriteHDScene(cuKey uint64, legacy, packet io.Writer) (shadesource.Declaration, error) {
	if r.hd == nil || legacy == nil || packet == nil {
		return shadesource.Declaration{}, fmt.Errorf("HD receipt requires configured runtime and both writers")
	}
	r.output.prepared.mu.Lock()
	inCallback := r.output.prepared.callbackDepth != 0
	r.output.prepared.mu.Unlock()
	if inCallback {
		return shadesource.Declaration{}, fmt.Errorf("receipt stream inside callback")
	}
	var slot *hdRegistrySlot
	for i := range r.hd.registry.slots {
		s := &r.hd.registry.slots[i]
		if s.capture != nil && s.capture.CUKey == cuKey {
			slot = s
			break
		}
	}
	if slot == nil {
		return shadesource.Declaration{}, fmt.Errorf("CU source no longer retained")
	}
	m, e := r.hd.registry.materialize(slot)
	if e != nil {
		return shadesource.Declaration{}, e
	}
	defer m.release()
	if !m.group.ReplayReady {
		return shadesource.Declaration{}, fmt.Errorf("original1x materialization failed")
	}
	if e = r.hd.owner.WriteBoundedLegacyJSON(m.scene, hdExactWriter{legacy}); e != nil {
		return shadesource.Declaration{}, e
	}
	if slot.source != nil {
		return slot.source.Declaration(), slot.source.WriteJSON(packet)
	}
	if slot.raw == nil {
		return shadesource.Declaration{}, fmt.Errorf("optional source packet not retained")
	}
	return slot.raw.Export(m.scene, packet)
}

type hdExactWriter struct{ io.Writer }

func (w hdExactWriter) Write(p []byte) (int, error) {
	n, e := w.Writer.Write(p)
	if e == nil && n != len(p) {
		e = io.ErrShortWrite
	}
	return n, e
}
