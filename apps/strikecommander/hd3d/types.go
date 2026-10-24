// Package hd3d implements specification 012's 1x integer raster replay core.
// It consumes captured copies, never StepView, guest memory, or filesystem assets.
package hd3d

const Width, Height, PixelCount = 320, 200, 64000

type CallKind string

const (
	R1       CallKind = "R1"
	R2       CallKind = "R2"
	R3       CallKind = "R3"
	R4       CallKind = "R4"
	R5       CallKind = "R5"
	R6       CallKind = "R6"
	BlackBox CallKind = "BlackBox"
)

type FarPtr struct{ Off, Seg uint16 }

// Span is a captured contiguous range in a 16-bit segment. Data may wrap at
// FFFFh; separate rows/spans permit texture rectangles without 64 KiB captures.
type Span struct {
	Off  uint16
	Data []byte
}
type Capture struct{ Spans []Span }
type TexRef struct{ Pixels, Shade Capture }
type AuxRef struct {
	Color0, Color1 byte
	HasFlags       bool
	Flags          uint16
	HasResid       bool
	ResA5, Res9F   uint16
	Table          []byte // R5 13DA table, relative to TableOff, C00h bytes
	TableOff       uint16
}
type PixelSet struct {
	Index uint32
	Value byte
}
type Call struct {
	Seq          uint16
	Kind         CallKind
	Steps        uint64
	DS, SS, SP   uint16
	Params       [13]uint16
	Desc         [6]uint16
	Verts, Table []byte
	Tex          TexRef
	Aux          AuxRef
	Patch        []PixelSet
	Element      FarPtr
	Object       bool // caller metadata only; does not infer an object-vtable set
	ReadFailed   bool
}
type Group struct {
	Key   uint64
	Calls []Call
	List  []uint16
	Buf3D []byte
}

// R6Table holds original 56F1:0008..00CF bytes (200 bytes, including four pointers).
// TablesFromEXE validates the source/table hashes and returns a private copy.
type Tables struct{ R6Table []byte }
type BranchCount struct {
	Routine, ID, Addr, Desc string
	Count                   int
}
type Diagnostics struct {
	Faults, Notes, Uninitialized, UninitializedAfterLast []string
	OutOfBounds, CaptureShort, Unwritten                 int
	PixelMismatch, WriterMismatch                        int
	FirstMismatch                                        int // -1 if absent
	R5FlatCalls, R5FlatWrites, R5FlatPostWrites          int
	R5ResidualSeeded                                     int
	Branches                                             []BranchCount // includes zero-count definitions
}
type Result struct {
	Pixels                      [PixelCount]byte
	Writer                      [PixelCount]uint16
	Written, ObjectWritten      [PixelCount]bool
	CallElements                map[uint16]FarPtr
	Diagnostics                 Diagnostics
	Valid, Checked, Repaintable bool
}
