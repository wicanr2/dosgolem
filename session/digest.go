package session

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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
	// AdLib tells receipts taken with the OPL2 present apart (docs/spec/240).
	AdLib bool
	// ClockPercent 是機器時脈比例（docs/spec/242）。
	ClockPercent int
	// ExtSHA256 是延伸記憶體 Ext 的 SHA-256（十六進位；docs/spec/430）。
	// 沒有延伸記憶體（預設）時為空字串，不影響既有收據。
	ExtSHA256 string
	// IRQ0Clamped 是計時器間隔被下限夾住的次數（docs/spec/242 §4）。
	IRQ0Clamped int
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
	d := StateDigest{Steps: m.Steps, DOSExited: o.dos.Exited, AdLib: m.AdLibPresent(), ClockPercent: m.ClockPercent(), IRQ0Clamped: m.IRQ0Clamped}
	d.MemorySHA256 = sha256.Sum256(m.Mem[:])
	if m.Ext != nil {
		sum := sha256.Sum256(m.Ext)
		d.ExtSHA256 = hex.EncodeToString(sum[:])
	}
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
