// Package geometry implements specification 013 phase 2's pure geometry core.
// It consumes copied, typed values and never reads the machine or asset files.
package geometry

import (
	"image"
	"image/color"
)

type ModelPoint struct{ X, Y, Z int32 }
type Point3 struct{ X, Z, Y int32 }
type Matrix [12]int32 // Rows X/Z/Y, each with three 24.8 coefficients and translation.
type Camera struct {
	CX, CY int16
	FX, FY int32
}
type FaceRef struct {
	Object, Face     uint32
	Clipped, Clamped bool
}
type Projection struct {
	UX, UY, QX, QY, NX, NY float64
	Z                      int32
	InvZQ48                uint64
}
type Triangle struct {
	Vertices  [3]Projection
	Ref       FaceRef
	WireColor color.RGBA
}
type FaceInput struct {
	Vertices  [3]Point3
	Camera    Camera
	Flag17BC  uint8
	Near      int32
	Ref       FaceRef
	WireColor color.RGBA
}
type FaceInfo struct {
	Clipped, AllClipped    bool
	NearUnexpectedVertices int
}
type Stats struct {
	FastTriangles, BigTriangles, DegenerateTriangles uint64
	WireEdges, WrittenSamples                        uint64
}

// Frame is a view into its Target. DepthQ48==0 means no owner; valid positive
// int32 Z always yields nonzero Q48 depth. Owner is meaningful only then.
// Clear or Draw mutates prior views. Copy buffers to retain a frame.
type Frame struct {
	Scale, Width, Height int
	Owner                []FaceRef
	DepthQ48             []uint64
	Wire                 *image.RGBA
	Stats                Stats
}

// Target must be constructed with NewTarget and used by one goroutine at a time.
type Target struct{ frame Frame }
