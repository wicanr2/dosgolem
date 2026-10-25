package opaque

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

func normalizeLimits(v Limits) (Limits, error) {
	d := DefaultLimits()
	if v.Objects == 0 {
		v.Objects = d.Objects
	}
	if v.Faces == 0 {
		v.Faces = d.Faces
	}
	if v.Calls == 0 {
		v.Calls = d.Calls
	}
	if v.CapturedBytes == 0 {
		v.CapturedBytes = d.CapturedBytes
	}
	if v.IntermediateBytes == 0 {
		v.IntermediateBytes = d.IntermediateBytes
	}
	if v.TotalBytes == 0 {
		v.TotalBytes = d.TotalBytes
	}
	if v.Snapshots == 0 {
		v.Snapshots = d.Snapshots
	}
	if v.Results == 0 {
		v.Results = d.Results
	}
	if v.Objects < 1 || v.Objects > d.Objects || v.Faces < 1 || v.Faces > d.Faces || v.Calls < 1 || v.Calls > d.Calls || v.CapturedBytes > d.CapturedBytes || v.IntermediateBytes > d.IntermediateBytes || v.TotalBytes > d.TotalBytes || v.Snapshots < 1 || v.Snapshots > d.Snapshots || v.Results < 1 || v.Results > d.Results {
		return Limits{}, fmt.Errorf("limits may only lower013a ceilings")
	}
	return v, nil
}
func NewRenderer(lib *realspace.Library, mat *MaterialLibrary, tables hd3d.Tables, limits ...Limits) (*Renderer, error) {
	if lib == nil || mat == nil || len(limits) > 1 {
		return nil, fmt.Errorf("missing libraries or invalid limits")
	}
	v := Limits{}
	if len(limits) == 1 {
		v = limits[0]
	}
	l, e := normalizeLimits(v)
	if e != nil {
		return nil, e
	}
	if mat.OwnedBytes()+(8<<20)+sourceLedgerBytes+uint64(len(tables.R6Table)) > l.TotalBytes {
		return nil, Failure{Gate: H3, Object: -1, Face: -1, Pass: -1, Reason: "library/scratch capacity"}
	}
	tables.R6Table = append([]byte(nil), tables.R6Table...)
	return &Renderer{lib: lib, materials: mat, tables: tables, limits: l, snapshots: map[[32]byte]*Snapshot{}, blocked: make(map[[32]byte]bool, maxSourceEpochs), owned: (8 << 20) + sourceLedgerBytes + uint64(len(tables.R6Table))}, nil
}
func canonicalScene(s model.Input) model.Input {
	s.Objects = append([]realspace.ObjObs(nil), s.Objects...)
	for i := range s.Objects {
		s.Objects[i].Verdict = realspace.ObjVerdict{}
	}
	return s
}
func inputCounts(s model.Input, lim Limits) error {
	if len(s.Objects) > lim.Objects || len(s.Raster.Calls) > lim.Calls {
		return fmt.Errorf("object/call limit")
	}
	faces := 0
	var captured, metadata uint64
	metadata += uint64(len(s.Cameras))*128 + uint64(len(s.Elements))*256 + uint64(len(s.Raster.List))*2
	countStrings := func(v []string) {
		for _, s := range v {
			metadata += uint64(len(s))*6 + 32
		}
	}
	if s.Original != nil {
		d := s.Original.Diagnostics
		countStrings(d.Faults)
		countStrings(d.Notes)
		countStrings(d.Uninitialized)
		countStrings(d.UninitializedAfterLast)
		metadata += uint64(len(s.Original.CallElements)) * 24
		for _, b := range d.Branches {
			metadata += 128 + 6*uint64(len(b.Routine)+len(b.ID)+len(b.Addr)+len(b.Desc))
		}
	}
	for _, o := range s.Objects {
		faces += len(o.Faces)
		metadata += uint64(len(o.FaceList)) * 2
		countStrings(o.Problems)
		metadata += uint64(len(o.VertXYZ)) + uint64(len(o.VertRT))*12
		for _, f := range o.Faces {
			metadata += 128
			if f.Source != nil {
				metadata += 1024
				if f.Source.Proof != nil {
					for _, b := range f.Source.Proof.Bytes {
						metadata += uint64(len(b.Data)) + 16
					}
				}
			}
		}
	}
	for _, c := range s.Raster.Calls {
		captured += uint64(len(c.Verts)+len(c.Table)+len(c.Aux.Table)) + uint64(len(c.Patch))*8
		for _, cap := range []hd3d.Capture{c.Tex.Pixels, c.Tex.Shade} {
			metadata += uint64(len(cap.Spans)) * 64
			for _, span := range cap.Spans {
				captured += uint64(len(span.Data))
			}
		}
	}
	if faces > lim.Faces || captured > lim.CapturedBytes || metadata > lim.IntermediateBytes {
		return fmt.Errorf("face/captured/intermediate limit")
	}
	return nil
}

// InputDigest hashes captured fields only. Serialized Selected/candidate geometry
// is neither authoritative input nor part of the observation identity.
func InputDigest(s model.Input) ([32]byte, error) {
	if e := inputCounts(s, DefaultLimits()); e != nil {
		return [32]byte{}, e
	}
	b, e := json.Marshal(canonicalScene(s))
	if e != nil {
		return [32]byte{}, e
	}
	if len(b) > 32<<20 {
		return [32]byte{}, fmt.Errorf("canonical input exceeds intermediate capacity")
	}
	return sha256.Sum256(b), nil
}
func epochKey(id SceneIdentity) [32]byte {
	b, _ := json.Marshal(struct {
		State    [32]byte
		App, Sig uint64
	}{id.State, id.AppGeneration, id.SignatureGeneration})
	return sha256.Sum256(b)
}

// typed profile never borrows encoding/json's process-global encodeState pool.
func (r *Renderer) sourceEpochKey(id SceneIdentity) [32]byte {
	if !r.typedFreeze {
		return epochKey(id)
	}
	key, _ := boundedJSONDigest(struct {
		State    [32]byte
		App, Sig uint64
	}{id.State, id.AppGeneration, id.SignatureGeneration}, 32<<20)
	return key // fixed fields have a compile-time small bound and no failing kind.
}

const maxSourceEpochs = 4096

// Reserve a conservative logical allowance for every map entry up front. Both
// recorder and renderer signature failures use this one bounded ledger.
const sourceLedgerBytes = uint64(maxSourceEpochs * 128)

// The caller holds r.mu. A full ledger prevents all new, unrecorded epochs from
// publishing; otherwise an invalid epoch could be forgotten after capacity fails.
func (r *Renderer) sourceStatusLocked(id SceneIdentity) error {
	if r.typedFreeze && r.limits.IntermediateBytes < boundedJSONFixedWorkspace {
		return gateFailure(H3, -1, -1, "typed JSON workspace capacity")
	}
	if r.blocked[r.sourceEpochKey(id)] {
		return gateFailure(H4, -1, -1, "source generation disabled")
	}
	if len(r.blocked) >= maxSourceEpochs {
		return gateFailure(H3, -1, -1, "source-generation ledger capacity")
	}
	return nil
}
func (r *Renderer) blockSourceLocked(id SceneIdentity) error {
	if r.typedFreeze && r.limits.IntermediateBytes < boundedJSONFixedWorkspace {
		return gateFailure(H3, -1, -1, "typed JSON workspace capacity")
	}
	key := r.sourceEpochKey(id)
	if r.blocked[key] {
		return nil
	}
	if len(r.blocked) >= maxSourceEpochs {
		return gateFailure(H3, -1, -1, "source-generation ledger capacity")
	}
	r.blocked[key] = true
	return nil
}

func (r *Renderer) Accept(in Input) (*Snapshot, error) {
	fail := func(g FailureGate, s string) (*Snapshot, error) {
		return nil, Failure{Gate: g, Object: -1, Face: -1, Pass: -1, Reason: s}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !in.SignatureValid {
		if err := r.blockSourceLocked(in.Identity); err != nil {
			return nil, err
		}
		return fail(H4, "recorder signature generation invalid")
	}
	if err := r.sourceStatusLocked(in.Identity); err != nil {
		return nil, err
	}
	id := in.Identity
	if id.EXE == ([32]byte{}) || id.State == ([32]byte{}) || id.Group == ([32]byte{}) || id.AppGeneration == 0 || id.SignatureGeneration == 0 || id.Assets != r.materials.Key() || id.CUKey != in.Scene.Key || in.Scene.Key != in.Scene.Raster.Key {
		return fail(H3, "incomplete/mixed source identity")
	}
	if in.Capture != nil && (!captureValid(id, *in.Capture) || in.Capture.Kind == CaptureBufCU && !r.typedFreeze) {
		return fail(H3, "capture identity differs")
	}
	if in.Capture != nil && in.Capture.Kind == CaptureBuf3D && (len(in.Scene.Raster.Buf3D) != hd3d.PixelCount || in.Capture.IndexedSHA != sha256.Sum256(in.Scene.Raster.Buf3D)) {
		return fail(H3, "Buf3D capture differs from canonical indexed bytes")
	}
	if e := inputCounts(in.Scene, r.limits); e != nil {
		return fail(H3, e.Error())
	}
	var digest [32]byte
	var e error
	if r.typedFreeze {
		digest, e = BoundedInputDigest(in.Scene, r.limits.IntermediateBytes)
	} else {
		digest, e = InputDigest(in.Scene)
	}
	if e != nil {
		return fail(H3, e.Error())
	}
	if digest != id.Group {
		return fail(H3, "captured group digest differs")
	}
	keyValue := struct {
		ID         SceneIdentity
		Cap        *CaptureIdentity
		ReadFailed bool
		Rule       string
	}{id, in.Capture, in.ReadFailed, RuleVersion}
	var key [32]byte
	if r.typedFreeze {
		key, e = boundedJSONDigest(keyValue, r.limits.IntermediateBytes)
		if e != nil {
			return fail(H3, e.Error())
		}
	} else {
		keyBytes, _ := json.Marshal(keyValue)
		key = sha256.Sum256(keyBytes)
	}
	if prior := r.snapshots[key]; prior != nil && !prior.released {
		prior.refs++
		return prior, nil
	}
	if len(r.snapshots) >= r.limits.Snapshots {
		return fail(H3, "snapshot slots full")
	}
	// Reserve both N-mode buffers and the bounded freeze workspace before copying.
	const sampleBuffers = uint64((640*400*4 + 960*600*4) * 24)
	reservation := sampleBuffers + r.limits.IntermediateBytes
	if r.owned+reservation+r.materials.OwnedBytes() > r.limits.TotalBytes {
		return fail(H3, "snapshot logical reservation capacity")
	}
	var frozen Input
	var retained uint64
	if r.typedFreeze {
		frozen, retained, e = typedFreezeInput(in, r.limits.IntermediateBytes)
		if e != nil {
			return fail(H3, e.Error())
		}
	} else {
		src := in
		src.Scene = canonicalScene(in.Scene)
		raw, x := json.Marshal(src)
		if x != nil {
			return fail(H3, x.Error())
		}
		if uint64(len(raw)) > r.limits.IntermediateBytes {
			return fail(H3, "freeze workspace capacity")
		}
		if x = json.Unmarshal(raw, &frozen); x != nil {
			return fail(H3, x.Error())
		}
		retained = uint64(len(raw))
	}
	reservation = sampleBuffers + retained
	snap := &Snapshot{renderer: r, input: frozen, key: key, owned: reservation, refs: 1}
	r.snapshots[key] = snap
	r.owned += reservation
	return snap, nil
}
func (s *Snapshot) Release() error {
	r := s.renderer
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.released {
		return fmt.Errorf("snapshot already released")
	}
	if s.rendering != 0 {
		return fmt.Errorf("snapshot render is in progress")
	}
	s.refs--
	if s.refs > 0 {
		return nil
	}
	s.released = true
	delete(r.snapshots, s.key)
	r.owned -= s.owned
	for _, slot := range s.slots {
		if slot.result != nil {
			r.results--
		}
	}
	return nil
}
func (s *Snapshot) Render(n int) (*Result, error) {
	if n != 2 && n != 3 {
		return nil, gateFailure(H3, -1, -1, "scale must be2/3")
	}
	r := s.renderer
	idx := n - 2
	r.mu.Lock()
	if s.released {
		r.mu.Unlock()
		return nil, fmt.Errorf("released snapshot")
	}
	if slot := s.slots[idx]; slot.done {
		r.mu.Unlock()
		return slot.result, slot.err
	}
	// Count synchronous waiters as active users, so Release cannot discard their
	// snapshot. No goroutine, background queue, or mutable input is retained.
	s.rendering++
	r.mu.Unlock()
	r.worker.Lock()
	r.mu.Lock()
	defer func() { s.rendering--; r.mu.Unlock(); r.worker.Unlock() }()
	// Another caller may have completed this same slot while we waited.
	if slot := s.slots[idx]; slot.done {
		return slot.result, slot.err
	}
	fixedFailure := func(err error) (*Result, error) { s.slots[idx] = resultSlot{done: true, err: err}; return nil, err }
	if err := r.sourceStatusLocked(s.input.Identity); err != nil {
		return fixedFailure(err)
	}
	if r.results >= r.limits.Results {
		return fixedFailure(gateFailure(H3, -1, -1, "result slots full"))
	}
	r.mu.Unlock()
	result, err := r.renderSnapshot(s.input, n)
	r.mu.Lock()
	if f, ok := err.(Failure); ok && f.Gate == H4 {
		if ledgerErr := r.blockSourceLocked(s.input.Identity); ledgerErr != nil {
			err = ledgerErr
		}
	}
	// Accept may have invalidated this epoch during the calculation. Publication,
	// capacity registration, and this final generation check are one transaction.
	if sourceErr := r.sourceStatusLocked(s.input.Identity); sourceErr != nil {
		result = nil
		err = sourceErr
	}
	if err == nil && r.owned+r.materials.OwnedBytes() > r.limits.TotalBytes {
		result = nil
		err = gateFailure(H3, -1, -1, "material cache total capacity")
	}
	if err == nil {
		result.renderer = r
		r.results++
	} else {
		result = nil
	}
	s.slots[idx] = resultSlot{done: true, result: result, err: err}
	return result, err
}
