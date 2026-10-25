package opaque

// These are controlled value views of013b, not another packet schema. The
// source owner remains responsible for canonical verification and revocation.
type ShadeIdentity struct {
	EXESHA, SignatureSetSHA                                           [32]byte
	Base                                                              uint16
	ResetGeneration, ModelsGeneration, ShadeEpoch, GroupSerial, CUKey uint64
}
type ShadeDeclaration struct {
	LegacyInputDigest    [32]byte
	Identity             ShadeIdentity
	DeclaredSourceDigest [32]byte
}
type ShadeWitness struct {
	CallIndex                          uint32
	Object, Face                       uint32
	HasFace                            bool
	Status, Reason                     uint64
	TableSeg, TableOff, RowSeg, RowOff uint16
	TableComplete, RowComplete         bool
	Raw5                               [5]byte
	HasRaw5                            bool
}
type ShadeSource interface {
	CompositionDeclaration() ShadeDeclaration
	ReadShadeCall(uint32, *[4096]byte, *[256]byte) (ShadeWitness, error)
	CompositionEligible() error
	Release()
}
type CompositionReservation interface{ Release() }
type CompositionBudget interface {
	OwnsCompositionSource(ShadeSource) bool
	PublishComposition(uint64, func() error) error
	ReserveComposition(uint64) (CompositionReservation, error)
	CompositionWorkspace(func() error) error
	CompositionEpochEligible(uint64) error
	OwnedBytes() uint64
	TotalBytes() uint64
}

const shadeTableSHA = "44e27a820cbb7661b1c1f999f5b458553f1287c1e59e53fdc2eccd0bde5a7e8f"

func hashHex(s string) (h [32]byte) {
	for i := range h {
		var v byte
		for _, c := range []byte(s[2*i : 2*i+2]) {
			v <<= 4
			if c >= 'a' {
				v += c - 'a' + 10
			} else {
				v += c - '0'
			}
		}
		h[i] = v
	}
	return
}
