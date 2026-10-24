// Package model implements specification 013 phase 2's pure model and alignment
// layer. It consumes captured copies and a trusted, original-file model library.
package model

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

type Element struct {
	Pointer hd3d.FarPtr
	Header  [16]byte
}

type Input struct {
	Key      uint64
	Cameras  []realspace.Camera
	Objects  []realspace.ObjObs
	Raster   hd3d.Group
	Original *hd3d.Result
	Elements []Element
}

// Result owns its target buffers. A later Render does not mutate this Frame.
// Passed is deliberately stronger than C.Passed: every object must pass its
// independent original-file self-check and B's hard error sets must pass.
// The caller must aggregate B counts across the state's prescribed groups to
// apply the 99% per-axis gate; it is not a per-group gate.
type Result struct {
	Key         uint64
	Scale       int
	Frame       geometry.Frame
	B           BMetrics
	C           CMetrics
	Objects     []ObjectReport
	Diagnostics Diagnostics
	Passed      bool
}

type BException struct {
	Object, Face uint32
	Vertex       int
	Axis         string
	Error        int64
	U, Q         float64
}

// Passed checks only this group's hard error sets. An empty denominator is
// valid for a fully clamped/no-division group (Applicable=false). Nonempty
// denominator and the 99% ratio belong to the aggregate state receipt.
type BMetrics struct {
	Eligible, Duplicates, NotProjected                uint64
	ExcludedNoDivide, ExcludedClamped, SkippedObjects uint64
	ZeroX, ZeroY, HardFailureX, HardFailureY          uint64
	ZeroXRatio, ZeroYRatio                            float64
	ErrorsX, ErrorsY                                  map[int64]uint64
	ULPExceptions                                     []BException
	Applicable, Passed                                bool
}

// BackfaceMetrics uses the camera-space cross(P1-P0,P2-P0) dot P0 diagnostic,
// not a parity claim about the game's stored normal. Signs use dot sign + 1.
// AbsDot uses exact decimal keys to avoid overflow for extreme int32 inputs.
type BackfaceMetrics struct {
	ALZeroSigns, ALNonzeroSigns [3]uint64
	Mismatch                    uint64
	AbsDot                      map[string]uint64
}

type ObjectReport struct {
	Object                                  uint32
	Status                                  realspace.ObjStatus
	Reason, TRE, Name                       string
	Kind                                    realspace.ModelKind
	GeoKey, AppKey                          [32]byte
	LOD                                     uint8
	CandidateCount, KeyCount                int
	XYZCompared, ScreenCompared             uint64
	Faces, VisibleFaces, Triangles          uint64
	ClippedFaces, ClampedFaces              uint64
	ClampedVertices, ClampExactVertices     uint64
	NearUnexpectedVertices, AllClippedFaces uint64
	Drawn                                   bool
	Backface                                BackfaceMetrics
}

// Counts refer to the 64000 original pixels, using the specified representative
// subpixel. Unlinked and LinkedOutsideSet count final-writer pixels.
type CMetrics struct {
	Available, Passed                                         bool
	C1, C2, C3a, C3b, C4, C5, C6, Neither                     uint64
	C2OutsideBand, Unlinked, LinkedOutsideSet, ObjectNotDrawn uint64
	BoundaryPixels                                            uint64
}

type Problem struct {
	Code, Message string
	Object, Face  int
	Call, Pixel   int
}

type Diagnostics struct {
	Problems              []Problem
	CUnavailable          []string
	ClassifiedObjectCalls uint64
	ReplayPixelsEqual     bool
	ReplayWriterEqual     bool
}
