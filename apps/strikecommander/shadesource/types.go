// Package shadesource implements 013b source capture and validation only.
// It never renders, resolves light policy, or retains a machine view.
package shadesource

import "fmt"

const RuleVersion = "013b-shade-capture-v1"
const SchemaVersion uint16 = 1
const MaxTotalBytes uint64 = 256 << 20
const MaxGraphBytes uint64 = 16 << 20
const SignatureSetSHAHex = "b4abf1fc54b5f8bec62120361fd8cdca516f0034f119d8eb481af70110a536fa"
const EXESHAHex = "883270cd0038fe571376202233b8abf5efb77889e27f4f0caf97dfab7e529cb5"
const TableSHAHex = "44e27a820cbb7661b1c1f999f5b458553f1287c1e59e53fdc2eccd0bde5a7e8f"

type Status uint64

const (
	Ready Status = iota
	H2
	H3
	H4
)

type Reason uint64

const (
	None Reason = iota
	MissingProof
	ProofUnavailable
	ProofMismatch
	MissingBinding
	RoleMismatch
	MissingRaw5
	ReadFailed
	ResourceLimit
	RowMismatch
	TableMismatch
	IdentityMismatch
	InvalidSchema
	GenerationMismatch
	NotConfirmed
	UnsupportedClass
	SpanMismatch
	SourceEpochPoisoned
	Bounds
)

type ReadState uint64

const (
	NotAttempted ReadState = iota
	Complete
	Failed
	Budget
	NotApplicable
)

type RowOrigin uint64

const (
	NoRow RowOrigin = iota
	OriginalCall
	ExtraRead
)

type Far struct{ Seg, Off uint16 }
type FaceRef struct{ Object, Face uint32 }
type Context struct {
	Steps, Deliveries  uint64
	CS, IP, DS, SS, SP uint16
}
type Identity struct {
	EXESHA, SignatureSetSHA                                           [32]byte
	Base                                                              uint16
	ResetGeneration, ModelsGeneration, ShadeEpoch, GroupSerial, CUKey uint64
}
type Span struct {
	Off       uint16
	Requested uint32
	OK        bool
	Data      []byte
}
type Read struct {
	Start     Far
	Requested uint32
	State     ReadState
	Spans     []Span
}
type ProofRead struct {
	SignatureIndex uint16
	Capture        Read
}
type Proof struct {
	Steps, Deliveries            uint64
	Base                         uint16
	ModelsGeneration, ShadeEpoch uint64
	Reads                        []ProofRead
	Status                       Status
	Reason                       Reason
}
type Build struct {
	FaceRef        FaceRef
	Context        Context
	ProofIndex     *uint32
	Method24, Raw5 *Read
	Status         Status
	Reason         Reason
}
type R4 struct {
	CallIndex                uint32
	FaceRef                  *FaceRef
	ElementIndex, BuildIndex *uint32
	Context                  Context
	ProofIndex               *uint32
	RowOrigin                RowOrigin
	Row, Table               *Read
	Status                   Status
	Reason                   Reason
}
type Counters struct {
	ReadCalls, ReadBytes, Candidates, Confirmed, Voided, ShortReads uint64
	ResourceRejected, BindingRejected, RowMismatch, TableMismatch   uint64
}
type Packet struct {
	SchemaVersion     uint16
	RuleVersion       string
	LegacyInputDigest [32]byte
	Identity          Identity
	Proofs            []Proof
	Builds            []Build
	Calls             []R4
	Counters          Counters
	Status            Status
	Reason            Reason
}

// Declaration is independent of Packet and never included in SourceDigest.
type Declaration struct {
	LegacyInputDigest    [32]byte
	Identity             Identity
	DeclaredSourceDigest [32]byte
}
type Error struct {
	Status Status
	Reason Reason
	Detail string
}

func (e *Error) Error() string {
	return fmt.Sprintf("013b status=%d reason=%d: %s", e.Status, e.Reason, e.Detail)
}
func failure(reason Reason, detail string) *Error { return &Error{statusFor(reason), reason, detail} }
func statusFor(r Reason) Status {
	switch r {
	case None:
		return Ready
	case ProofMismatch, SourceEpochPoisoned:
		return H4
	case ResourceLimit, IdentityMismatch, InvalidSchema, Bounds:
		return H3
	default:
		return H2
	}
}
func Index(n int) *uint32 { x := uint32(n); return &x }
func HashLiteral(s string) [32]byte {
	var h [32]byte
	for i := range h {
		var a, b byte
		a = unhex(s[i*2])
		b = unhex(s[i*2+1])
		h[i] = a*16 + b
	}
	return h
}
func unhex(c byte) byte {
	if c >= '0' && c <= '9' {
		return c - '0'
	}
	return c - 'a' + 10
}
func InitialPacket(id Identity) Packet {
	return Packet{SchemaVersion: SchemaVersion, RuleVersion: RuleVersion, Identity: id, Proofs: []Proof{}, Builds: []Build{}, Calls: []R4{}, Status: H2, Reason: MissingProof}
}
