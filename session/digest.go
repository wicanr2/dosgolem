package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
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
	return digestOf(o.machine, o.dos), nil
}

// digestOf 是 Digest 的本體，也給測試對存檔端的 machine／DOS 取同一種摘要（docs/spec/450 §3.2）。
func digestOf(m *machine.Machine, dd *dos.DOS) StateDigest {
	d := StateDigest{Steps: m.Steps, DOSExited: dd.Exited, AdLib: m.AdLibPresent(), ClockPercent: m.ClockPercent(), IRQ0Clamped: m.IRQ0Clamped}
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
	return d
}

// FullDigest 是機器與 DOS 層的狀態摘要（docs/spec/454 §2.3）。
type FullDigest struct {
	Machine [32]byte // machine.SaveState 的 gob 經 machine.StateDigest
	DOS     [32]byte // dos.SaveState 的 gob 經 dos.StateDigest
}

// MachineDigest 回機器與 DOS 層的狀態摘要（docs/spec/454 §2.3）。呼叫限制同 Digest：
// 已關閉、回合進行中回錯誤。兩份 SaveState 都只讀，不改變狀態；成本是編碼整份狀態，
// 只在收據的截圖點與停止點呼叫。
//
// 已知缺漏（不在兩份 SaveState 內，摘要相同不代表它們相同）：待送的按鍵（keyQueue、timedKeys）、
// 待發與進行中的回呼（cbQueue、cbActive、cbSaved、cbMade）、週期遠呼叫（periodic）、Frames、
// CPU.FPUCW、454 §2.1 的送達計數。
func (o *Owner) MachineDigest() (FullDigest, error) {
	if o == nil {
		return FullDigest{}, errors.New("session: Owner 不得為 nil")
	}
	if o.phase == PhaseClosed || o.closed {
		return FullDigest{}, errors.New("session: 已關閉的 owner 沒有摘要")
	}
	if o.turnPending {
		return FullDigest{}, errors.New("session: 回合進行中不可取摘要")
	}
	return fullDigestOf(o.machine, o.dos)
}

// fullDigestOf 是 MachineDigest 的本體（測試也對存檔端的 machine／DOS 取同一種摘要）。
func fullDigestOf(m *machine.Machine, dd *dos.DOS) (FullDigest, error) {
	var d FullDigest
	var buf bytes.Buffer
	if err := m.SaveState(&buf); err != nil {
		return FullDigest{}, fmt.Errorf("session: 機器摘要：%w", err)
	}
	sum, err := machine.StateDigest(&buf)
	if err != nil {
		return FullDigest{}, fmt.Errorf("session: 機器摘要：%w", err)
	}
	d.Machine = sum
	buf.Reset()
	if err := dd.SaveState(&buf); err != nil {
		return FullDigest{}, fmt.Errorf("session: DOS 摘要：%w", err)
	}
	if d.DOS, err = dos.StateDigest(&buf); err != nil {
		return FullDigest{}, fmt.Errorf("session: DOS 摘要：%w", err)
	}
	return d, nil
}

// DOSMouseState 是 DOS 滑鼠狀態的值複本（docs/spec/451 §2.4），欄位型別同 dos.Mouse。
// 它不帶任何寫入能力。
type DOSMouseState struct {
	X, Y             uint16
	MickeyX, MickeyY int16
	Buttons          uint16
}

// DOSMouse 回傳目前的 DOS 游標座標、累計 mickey 與按鍵狀態，供同狀態比對與前端除錯顯示。
// 只在 PhaseRunning 時可呼叫；與 Digest 不同，回合進行中（turnPending）也可以，
// 因為 Deliver 對 DOS 滑鼠的寫入在 commit 內一次完成，沒有半寫入狀態。
// 其他 phase 回錯誤，不改 phase。
func (o *Owner) DOSMouse() (DOSMouseState, error) {
	if o == nil {
		return DOSMouseState{}, errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseRunning {
		return DOSMouseState{}, fmt.Errorf("session: phase %d 不可讀 DOS 滑鼠狀態", o.phase)
	}
	m := &o.dos.Mouse
	return DOSMouseState{X: m.X, Y: m.Y, MickeyX: m.MickeyX, MickeyY: m.MickeyY, Buttons: m.Buttons}, nil
}
