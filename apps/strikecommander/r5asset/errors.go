package r5asset

// Code and Error are fixed pure values. They never retain input or parser views.
type Code uint8

const (
	Identity Code = iota + 1
	Structure
	Decode
	Capacity
	Closed
	Ownership
)

type Error struct {
	Code               Code
	Stage, Model, Slot uint8
	Offset             uint32
	Detail             uint16
}

func (e Error) Error() string {
	switch e.Code {
	case Identity:
		return "r5asset identity"
	case Structure:
		return "r5asset structure"
	case Decode:
		return "r5asset decode"
	case Capacity:
		return "r5asset capacity"
	case Closed:
		return "r5asset closed"
	default:
		return "r5asset ownership"
	}
}

func reject(c Code, offset int, detail uint16) Error {
	return Error{Code: c, Offset: uint32(offset), Detail: detail}
}
