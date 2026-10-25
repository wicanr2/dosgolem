package strikecommander

import (
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
	"image"
)

type hdImageBinding struct {
	slot              int
	serial, epoch, cu uint64
	reason            string
}
type hdCoreMemo struct {
	failed       bool
	reason       string
	success      bool
	hash         [32]byte
	noExtra      bool
	tableVersion uint64
}
type hdRegistrySlot struct {
	digest           [32]byte
	hasDigest        bool
	memo             [2]hdCoreMemo
	capture          *hdSealedCapture
	raw              *shadesource.RawLease
	source           *shadesource.Lease
	sourceFailure    opaqueSourceFailure
	hasSourceFailure bool
	policy           hdPolicyState
	revoked          bool
}
type opaqueSourceFailure struct {
	status shadesource.Status
	reason shadesource.Reason
}
type hdSceneRegistry struct {
	owner    *shadesource.Owner
	pool     *shadesource.ProtectedProducer
	arena    *legacyArena
	slots    [2]hdRegistrySlot
	image    int
	epoch    uint64
	rejected HDCaptureRejected
	lib      *realspace.Library
	tables   hd3d.Tables
}

func newHDSceneRegistry(owner *shadesource.Owner, pool *shadesource.ProtectedProducer, lib *realspace.Library, tables hd3d.Tables) (*hdSceneRegistry, error) {
	a, e := newLegacyArena(pool)
	if e != nil {
		return nil, e
	}
	r, e := legacyNew[hdSceneRegistry](a)
	if e != nil {
		a.retire()
		return nil, e
	}
	r.owner, r.pool, r.arena, r.lib, r.tables = owner, pool, a, lib, tables
	r.image = -1
	r.epoch = 1
	return r, nil
}
func (r *hdSceneRegistry) drop(i int) {
	s := &r.slots[i]
	if s.capture == nil {
		return
	}
	if s.source != nil {
		s.source.Release()
	}
	if s.raw != nil {
		s.raw.Release()
	}
	s.capture.Arena.release()
	*s = hdRegistrySlot{}
	if r.image == i {
		r.image = -1
	}
}
func (r *hdSceneRegistry) onSealed(c *hdSealedCapture) {
	// At most one image and one pending CU; superseded candidates are retired
	// independently of whether HD rendering is enabled.
	for i := range r.slots {
		if i != r.image {
			r.drop(i)
		}
	}
	idx := -1
	for i := range r.slots {
		if r.slots[i].capture == nil {
			idx = i
			break
		}
	}
	if idx < 0 {
		r.onRejected(HDCaptureRejected{Serial: c.Serial, CUKey: c.CUKey, Reason: HDCaptureQuota})
		return
	}
	if e := c.Arena.acquire(); e != nil {
		r.onRejected(HDCaptureRejected{Serial: c.Serial, CUKey: c.CUKey, Reason: HDCaptureQuota})
		return
	}
	s := &r.slots[idx]
	s.capture = c
	s.policy.viewport = image.Rect(int(c.Group.Desc[2]), int(c.Group.Desc[3]), int(c.Group.Desc[4])+1, int(c.Group.Desc[5])+1)
	if c.Group.ShadeSource != nil {
		id := c.Group.ShadeSource.ID()
		if id.CUKey != c.CUKey || id.ShadeEpoch != c.ShadeEpoch || id.ModelsGeneration != c.SignatureGeneration {
			s.hasSourceFailure = true
			s.sourceFailure = opaqueSourceFailure{shadesource.H3, shadesource.IdentityMismatch}
		} else {
			raw, e := c.Group.ShadeSource.Acquire()
			if e != nil {
				s.hasSourceFailure = true
				s.sourceFailure = opaqueSourceFailure{shadesource.H3, shadesource.ResourceLimit}
			} else {
				s.raw = raw
			}
		}
	} else if e := c.Group.ShadeSourceError; e != nil {
		s.hasSourceFailure = true
		s.sourceFailure = opaqueSourceFailure{e.Status, e.Reason}
	}
}
func (r *hdSceneRegistry) onRejected(v HDCaptureRejected) {
	r.rejected = v
	for i := range r.slots {
		if i != r.image {
			r.drop(i)
		}
	}
}
func (r *hdSceneRegistry) invalidateCU(key uint64) {
	for i := range r.slots {
		s := &r.slots[i]
		if s.capture != nil && s.capture.CUKey == key {
			s.revoked = true
		}
	}
	r.epoch++
}
func (r *hdSceneRegistry) invalidateAll() {
	for i := range r.slots {
		r.drop(i)
	}
	r.epoch++
}

// Revocation only invalidates new presentation eligibility. Immutable historical
// bytes stay held until their real registry/source readers retire.
func (r *hdSceneRegistry) refreshRevocations() {
	changed := false
	for i := range r.slots {
		s := &r.slots[i]
		if s.capture != nil && !s.revoked && r.owner.CompositionEpochEligible(s.capture.ShadeEpoch) != nil {
			s.revoked = true
			changed = true
		}
	}
	if changed {
		r.epoch++
	}
}
func (r *hdSceneRegistry) close() {
	r.invalidateAll()
	r.lib = nil
	r.tables = hd3d.Tables{}
	r.arena.retire()
}
func (r *hdSceneRegistry) bind(img *presentImage, key uint64) {
	prior := img.hdBinding
	b := hdImageBinding{slot: -1, epoch: r.epoch, cu: key, reason: "no same-CU sealed source"}
	switch img.info.Kind {
	case PresPalette, PresSubtitle:
		if prior.slot >= 0 && prior.slot < 2 {
			s := &r.slots[prior.slot]
			if s.capture != nil && s.capture.Serial == prior.serial && !s.revoked {
				b = prior
			}
		}
	case PresCU, PresCUReturn:
		for i := range r.slots {
			s := &r.slots[i]
			if s.capture != nil && s.capture.CUKey == key && !s.revoked {
				b = hdImageBinding{slot: i, serial: s.capture.Serial, epoch: r.epoch, cu: key}
				break
			}
		}
	}
	if b.slot >= 0 {
		s := &r.slots[b.slot]
		if !s.policy.checkPalette(img.palette) {
			b.slot = -1
			b.reason = s.policy.reason
		}
	}
	old := r.image
	r.image = b.slot
	if old >= 0 && old != r.image {
		r.drop(old)
	}
	img.hdBinding = b
}
func (r *hdSceneRegistry) lookup(b hdImageBinding) (*hdRegistrySlot, error) {
	if b.slot < 0 || b.slot >= 2 || b.epoch != r.epoch {
		return nil, fmt.Errorf("NoHD: %s", b.reason)
	}
	s := &r.slots[b.slot]
	if s.capture == nil || s.capture.Serial != b.serial || s.capture.CUKey != b.cu || s.revoked || s.policy.revoked {
		return nil, fmt.Errorf("NoHD: source retired/void/stale")
	}
	return s, nil
}
func (r *hdSceneRegistry) frame(shown *ModelGroup, steps uint64, indexed []byte, palette [256][3]byte) {
	for i := range r.slots {
		s := &r.slots[i]
		if s.capture != nil && s.capture.Group == shown {
			old := s.policy.revoked
			_ = s.policy.frame(steps, indexed, palette)
			if !old && s.policy.revoked {
				r.epoch++
			}
		}
	}
}

type hdMaterialized struct {
	group       ModelGroup
	scene       model.Input
	reservation *shadesource.OwnedReservation
}

func (m *hdMaterialized) release() {
	m.scene = model.Input{}
	m.group = ModelGroup{}
	m.reservation.Release()
}
func (r *hdSceneRegistry) materialize(s *hdRegistrySlot) (*hdMaterialized, error) {
	c := s.capture
	g := c.Group
	// Conservative preflight includes old/new metadata, every possible residual
	// Patch and CheckObject candidate output. It is a consumer charge, never a
	// producer alias disguised as a second allocation. Working arrays use the
	// same fixed source worker; no VM read occurs here.
	bytes := uint64(2<<20) + uint64(len(g.Objects))*(64<<10) + uint64(len(g.Calls))*512 + uint64(len(c.PatchJobs))*uint64(hd3d.PixelCount)*8
	if bytes > 32<<20 {
		return nil, fmt.Errorf("materialize intermediate bound")
	}
	hold, e := r.owner.ReserveOwned(bytes)
	if e != nil {
		return nil, e
	}
	m := &hdMaterialized{reservation: hold}
	e = r.owner.CompositionWorkspace(func() error {
		m.group = *g
		out := &m.group
		out.Objects = make([]realspace.ObjObs, len(g.Objects))
		copy(out.Objects, g.Objects)
		out.Calls = make([]hd3d.Call, len(g.Calls))
		copy(out.Calls, g.Calls)
		out.RasterProblems = append([]string(nil), g.RasterProblems...)
		for _, job := range c.PatchJobs {
			if job.Call < 0 || job.Call >= len(out.Calls) || job.Start < 0 || job.End < job.Start || job.End > job.Call || out.Calls[job.Call].Kind != hd3d.BlackBox {
				return fmt.Errorf("invalid residual job order")
			}
			patch, d := hd3d.MakePatch(job.Before, job.After, out.Calls[job.Start:job.End], out.List, r.tables)
			if len(d.Faults) != 0 {
				return fmt.Errorf("residual replay failed: %v", d.Faults)
			}
			out.Calls[job.Call].Patch = patch
		}
		for i := range out.Objects {
			out.Objects[i].Verdict = realspace.CheckObject(r.lib, out.Cameras, &out.Objects[i])
		}
		if len(c.Reads) != 0 {
			out.Reads = make(map[uint16]ModelReadCount, len(c.Reads))
			for _, x := range c.Reads {
				out.Reads[x.IP] = x.Count
			}
		}
		if out.ReadFailed {
			out.RasterProblems = append(out.RasterProblems, "群組必要輸入讀取失敗")
		}
		result := hd3d.Replay(hd3d.Group{Key: out.Key, Calls: out.Calls, List: out.List, Buf3D: out.Buf3D}, r.tables)
		if len(out.RasterProblems) != 0 {
			result.Valid, result.Repaintable = false, false
			result.Diagnostics.Faults = append(result.Diagnostics.Faults, out.RasterProblems...)
		}
		out.Raster = &result
		out.ReplayReady = result.Repaintable
		elements := make([]model.Element, len(out.Elements))
		for i, x := range out.Elements {
			elements[i] = model.Element{Pointer: x.Pointer, Header: x.Header}
		}
		m.scene = model.Input{Key: out.Key, Cameras: out.Cameras, Objects: out.Objects, Raster: hd3d.Group{Key: out.Key, Calls: out.Calls, List: out.List, Buf3D: out.Buf3D}, Original: out.Raster, Elements: elements}
		return nil
	})
	if e != nil {
		m.release()
		return nil, e
	}
	return m, nil
}
