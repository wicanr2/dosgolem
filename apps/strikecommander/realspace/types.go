// Package realspace implements Strike Commander specification 013 phase 1:
// read-only model identification and integer projection checks. It does not render HD.
package realspace

type FarPtr struct{ Off, Seg uint16 }

func (p FarPtr) Linear() uint32 { return uint32(p.Seg)*16 + uint32(p.Off) }

type Camera struct {
	Steps  uint64
	DS     uint16
	CamObj uint16
	Raw    [0x54]byte
}

type FaceObs struct {
	SI       uint16
	Face     FarPtr
	Method   FarPtr
	Geom     [0x26]byte
	FaceVT   uint16
	VIdx     [3]int16
	AL       uint8
	Proj     [3][18]byte
	Returned bool
	Elem     FarPtr
}

type ObjObs struct {
	Steps     uint64
	CamObj    uint16
	CamIdx    int
	Rec       FarPtr
	RecRaw    [0x53]byte
	RecVT     uint16
	VertRT    [][3]int32
	Model     FarPtr
	ModelHead [0x80]byte
	VertXYZ   []byte
	VertSHA   [32]byte
	LOD       uint8
	NFaces    int16
	FaceList  []uint16
	FaceBase  FarPtr
	Flag17BC  uint8
	Near, Far int32
	Faces     []FaceObs
	Verdict   ObjVerdict

	// Recorder faults are separate from raw bytes; never fabricate a successful read.
	ReadFailed, OutOfBounds, FaceTableMismatch   bool
	FaceRetMismatch, F1RUnpaired, UnmatchedFrame uint64
	Problems                                     []string
}

type ModelKind string

const (
	POLY ModelKind = "POLY"
	APPR ModelKind = "APPR"
)

type ModelEntry struct {
	TRE, Name string
	VertSHA   [32]byte
	Verts     [][3]int32
	LOD       [][]uint16
	VTRI      [][8]byte
	GeoKey    [32]byte
	Kind      ModelKind
	Apex      []int
	AppKey    [32]byte
	LNTH      []byte
}

type ObjStatus string

const (
	Unchecked ObjStatus = "unchecked"
	Failed    ObjStatus = "failed"
	Passed    ObjStatus = "passed"
)

type Mismatch struct {
	Face, Vertex     int
	Field            string
	Expected, Actual int64
}

type ObjVerdict struct {
	Status                          ObjStatus
	Reason                          string
	Unidentified                    bool
	Selected                        *ModelEntry
	Candidates                      []*ModelEntry
	KeyCount                        int
	MultiGeoKey                     bool
	XYZCompared, ScreenCompared     uint64 // each vertex contributes one comparison group
	XYZMismatch, ScreenMismatch     uint64
	NearNoDivide, ClipFlag, Clamped uint64
	UnknownFaceVT                   uint64
	First                           *Mismatch
}
