package session

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// StateDigest is a value-only fingerprint of the owned machine, for same-state
// A/B receipts (docs/spec/238 §4).  It exposes no resource.
type StateDigest struct {
	Steps         uint64
	MemorySHA256  [32]byte
	CPUSHA256     [32]byte
	IndexedSHA256 [32]byte
	PaletteSHA256 [32]byte
	DOSExited     bool
}

// Digest fingerprints the machine between turns.  It is refused while a turn
// is pending and after Close.
func (o *Owner) Digest() (StateDigest, error) {
	if o == nil {
		return StateDigest{}, errors.New("session: Owner 不得為 nil")
	}
	if o.phase == PhaseClosed || o.closed {
		return StateDigest{}, errors.New("session: 已關閉的 owner 沒有摘要")
	}
	if o.turnPending {
		return StateDigest{}, errors.New("session: 回合進行中不可取摘要")
	}
	m := o.machine
	d := StateDigest{Steps: m.Steps, DOSExited: o.dos.Exited}
	d.MemorySHA256 = sha256.Sum256(m.Mem[:])
	cpu := make([]byte, 0, 2*(len(m.CPU.R)+len(m.CPU.Seg)+2))
	for _, v := range m.CPU.R {
		cpu = binary.LittleEndian.AppendUint16(cpu, v)
	}
	for _, v := range m.CPU.Seg {
		cpu = binary.LittleEndian.AppendUint16(cpu, v)
	}
	cpu = binary.LittleEndian.AppendUint16(cpu, m.CPU.IP)
	cpu = binary.LittleEndian.AppendUint16(cpu, m.CPU.Flags)
	d.CPUSHA256 = sha256.Sum256(cpu)
	d.IndexedSHA256 = sha256.Sum256(m.Indexed())
	pal := m.Palette()
	flat := make([]byte, 0, 768)
	for _, c := range pal {
		flat = append(flat, c[:]...)
	}
	d.PaletteSHA256 = sha256.Sum256(flat)
	return d, nil
}
