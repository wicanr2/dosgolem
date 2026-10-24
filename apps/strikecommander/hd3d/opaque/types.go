package opaque

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"sync"
)

const RuleVersion = "013a-opaque-four-q24-v1"

type SceneIdentity struct {
	EXE, Assets, State, Group                 [32]byte
	AppGeneration, SignatureGeneration, CUKey uint64
}
type CaptureKind string

const (
	CaptureBuf3D CaptureKind = "buf3d"
	CaptureVGA   CaptureKind = "completion-vga"
	CaptureFrame CaptureKind = "frame"
)

type CaptureIdentity struct {
	Scene                          SceneIdentity
	Kind                           CaptureKind
	Steps                          uint64
	SourceCS, SourceIP             uint16
	IndexedSHA, OriginalPaletteSHA [32]byte
}
type Input struct {
	Identity SceneIdentity
	Capture  *CaptureIdentity
	Scene    model.Input
	// This is the caller's actual recorder status, not an automatically invented
	// epoch. Every R3 source also verifies original signature bytes independently.
	SignatureValid bool
	ReadFailed     bool
}
type FailureGate string

const (
	H1 FailureGate = "H1"
	H2 FailureGate = "H2"
	H3 FailureGate = "H3"
	H4 FailureGate = "H4"
)

type Failure struct {
	Gate               FailureGate
	Object, Face, Pass int
	Address            uint16
	Reason             string
}

func (e Failure) Error() string {
	return fmt.Sprintf("%s object=%d face=%d pass=%d: %s", e.Gate, e.Object, e.Face, e.Pass, e.Reason)
}

type ObjectReport struct {
	Object                                              int
	Status                                              realspace.ObjStatus
	Gate                                                FailureGate
	Reason                                              string
	Candidates, VisibleFaces, Triangles, WrittenSamples int
	MaterialKey                                         [32]byte
}
type Diagnostics struct {
	Objects, SuccessfulObjects, H1Objects, H2Objects, BackfaceFaces, ClippedFaces, AllClippedFaces, Triangles, FastSamples, BigSamples, WrittenSamples, TransparentSamples, DeferredCalls uint64
	OriginalChecked                                                                                                                                                                       bool
	Problems                                                                                                                                                                              []Failure
}
type Report struct {
	Identity       SceneIdentity
	Scale          int
	Valid, Needs3b bool
	Diagnostics    Diagnostics
	Objects        []ObjectReport
}
type DeferredOp struct {
	Object, Face int
	Call         hd3d.Call
}
type sample struct {
	color SampleColor
	depth uint64
	owner uint32
	flags uint32
}
type Sample struct {
	Color    SampleColor
	DepthQ48 uint64
	Owner    geometry.FaceRef
	Coverage bool
}

// Result owns immutable samples. Getter copies never expose its backing buffers.
type Result struct {
	identity             SceneIdentity
	capture              *CaptureIdentity
	scale, width, height int
	samples              []sample
	report               Report
	deferred             []DeferredOp
	renderer             *Renderer
}

func (r *Result) Identity() SceneIdentity { return r.identity }
func (r *Result) Capture() (CaptureIdentity, bool) {
	if r.capture == nil {
		return CaptureIdentity{}, false
	}
	return *r.capture, true
}
func (r *Result) Dimensions() (int, int) { return r.width, r.height }
func (r *Result) Report() Report {
	v := r.report
	v.Objects = append([]ObjectReport(nil), v.Objects...)
	v.Diagnostics.Problems = append([]Failure(nil), v.Diagnostics.Problems...)
	return v
}
func (r *Result) Sample(x, y, i, j int) (Sample, bool) {
	if x < 0 || y < 0 || x >= r.width || y >= r.height || i < 0 || i > 1 || j < 0 || j > 1 {
		return Sample{}, false
	}
	s := r.samples[((y*r.width+x)*4)+j*2+i]
	return Sample{Color: s.color, DepthQ48: s.depth, Owner: geometry.FaceRef{Object: s.owner >> 16, Face: s.owner & 65535, Clipped: s.flags&2 != 0, Clamped: s.flags&4 != 0}, Coverage: s.depth != 0}, true
}
func (r *Result) Hash() [32]byte {
	h := sha256.New()
	var b [24]byte
	for _, s := range r.samples {
		clear(b[:])
		b[0], b[1] = s.color.Lo, s.color.Hi
		binary.LittleEndian.PutUint32(b[4:], s.color.W24)
		binary.LittleEndian.PutUint64(b[8:], s.depth)
		binary.LittleEndian.PutUint32(b[16:], s.owner)
		binary.LittleEndian.PutUint32(b[20:], s.flags)
		h.Write(b[:])
	}
	var v [32]byte
	copy(v[:], h.Sum(nil))
	return v
}
func (r *Result) DeferredOps() []DeferredOp {
	out := make([]DeferredOp, len(r.deferred))
	for i, v := range r.deferred {
		out[i] = v
		out[i].Call = cloneCall(v.Call)
	}
	return out
}

// Limits may only lower the specification's ceilings. Zero fields use defaults.
type Limits struct {
	Objects, Faces, Calls                        int
	CapturedBytes, IntermediateBytes, TotalBytes uint64
	Snapshots, Results                           int
}

func DefaultLimits() Limits { return Limits{256, 32768, 65536, 16 << 20, 32 << 20, 256 << 20, 2, 2} }

type resultSlot struct {
	done   bool
	result *Result
	err    error
}
type Snapshot struct {
	refs      int
	rendering int
	renderer  *Renderer
	input     Input
	key       [32]byte
	owned     uint64
	slots     [2]resultSlot
	released  bool
}
type Renderer struct {
	lib       *realspace.Library
	materials *MaterialLibrary
	tables    hd3d.Tables
	limits    Limits
	mu        sync.Mutex
	worker    sync.Mutex
	snapshots map[[32]byte]*Snapshot
	blocked   map[[32]byte]bool
	owned     uint64
	results   int
}

type PaletteSnapshot struct {
	Identity SceneIdentity
	Capture  CaptureIdentity
	Colors   [256][3]uint8
	SHA      [32]byte
}
type BackgroundComponents struct {
	identity             SceneIdentity
	capture              CaptureIdentity
	scale, width, height int
	rule                 string
	paletteSHA, key      [32]byte
	colors               []SampleColor
}

func (b *BackgroundComponents) Key() [32]byte { return b.key }
