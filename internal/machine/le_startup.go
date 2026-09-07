package machine

import (
	"io"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// FD2StartupDOS 是固定雜湊 FD2.EXE 在 DOS/4GW 已載入後所需的啟動服務。
// 它不是一般 DOS 或 DOS/4GW 模擬器；未列呼叫與錯誤順序一律拒絕。
type FD2StartupDOS struct {
	calls     int
	timeCalls int
	// DPMI 是**與程式無關**的 `int 31h` 主機（`dpmi.go`）。
	//
	// 這一支剩下的部分還是 FD2 專屬的（啟動握手、selector 值），
	// 但 DPMI 那一層已經搬出去了——換一支 DOS/4GW 程式時它照用，
	// 不必再抄一份（`docs/spec/184-mvp-scope-review` 批次 2）。
	DPMI       *DPMIHost
	dosVectors [256]uint64
	files           ReadOnlyFileProvider
	handles         map[uint16]io.ReadSeekCloser
	nextHandle      uint16
}

var minimalFD2Environment = []byte{0, 0, 1, 0, 'F', 'D', '2', '.', 'E', 'X', 'E', 0}

func (s *FD2StartupDOS) Calls() int { return s.calls }

func NewFD2StartupDOS(files ReadOnlyFileProvider) *FD2StartupDOS {
	return &FD2StartupDOS{
		files:      files,
		handles:    make(map[uint16]io.ReadSeekCloser),
		nextHandle: 5,
		DPMI:       NewDPMIHost(nil),
	}
}

// dpmi 回這一支的 DPMI 主機，零值也能用（測試常常直接造 &FD2StartupDOS{}）。
func (s *FD2StartupDOS) dpmi() *DPMIHost {
	if s.DPMI == nil {
		s.DPMI = NewDPMIHost(nil)
	}
	return s.DPMI
}

// SetRealModeVector 把實模式向量交給 DPMI 主機（`AX=0200h` 問的就是它）。
func (s *FD2StartupDOS) SetRealModeVector(n uint8, seg, off uint16) {
	s.dpmi().SetRealModeVector(n, seg, off)
}

// AttachMachine 讓描述子與線性記憶體那兩組 DPMI 功能可用。
func (s *FD2StartupDOS) AttachMachine(m *LEMachine) { s.dpmi().Attach(m) }

func (s *FD2StartupDOS) HasHandle(handle uint16) bool {
	_, ok := s.handles[handle]
	return ok
}

func (s *FD2StartupDOS) Close() error {
	var first error
	for handle, file := range s.handles {
		if err := file.Close(); err != nil && first == nil {
			first = err
		}
		delete(s.handles, handle)
	}
	return first
}

func (s *FD2StartupDOS) openReadOnly(c *cpu386.CPU) {
	setError := func(code uint16) {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
		c.EFlags |= cpu386.CF
	}
	if uint8(c.R[cpu386.EAX]) != 0 || s.files == nil {
		setError(5)
		return
	}
	path := make([]byte, 0, 32)
	terminated := false
	for offset := uint32(0); offset < 260; offset++ {
		if c.R[cpu386.EDX] > ^uint32(0)-offset {
			setError(3)
			return
		}
		value, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+offset)
		if !ok {
			setError(3)
			return
		}
		if value == 0 {
			terminated = true
			break
		}
		path = append(path, value)
	}
	if !terminated {
		setError(3)
		return
	}
	file, err := s.files.OpenRead(string(path))
	if err != nil {
		setError(2)
		return
	}
	if s.handles == nil {
		s.handles = make(map[uint16]io.ReadSeekCloser)
	}
	if s.nextHandle < 5 {
		s.nextHandle = 5
	}
	handle := s.nextHandle
	if handle == 0xffff {
		file.Close()
		setError(4)
		return
	}
	s.nextHandle++
	s.handles[handle] = file
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(handle)
	c.EFlags &^= cpu386.CF
}

func (s *FD2StartupDOS) deviceInformation(c *cpu386.CPU) {
	setError := func(code uint16) {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
		c.EFlags |= cpu386.CF
	}
	if uint8(c.R[cpu386.EAX]) != 0 {
		setError(1)
		return
	}
	if _, ok := s.handles[uint16(c.R[cpu386.EBX])]; !ok {
		setError(6)
		return
	}
	c.R[cpu386.EDX] &= 0xffff0000
	c.EFlags &^= cpu386.CF
}

func (s *FD2StartupDOS) readFile(c *cpu386.CPU) {
	setError := func(code uint16) {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
		c.EFlags |= cpu386.CF
	}
	file, ok := s.handles[uint16(c.R[cpu386.EBX])]
	if !ok {
		setError(6)
		return
	}
	count := int(uint16(c.R[cpu386.ECX]))
	if count == 0 {
		c.R[cpu386.EAX] &= 0xffff0000
		c.EFlags &^= cpu386.CF
		return
	}
	buffer := make([]byte, count)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		setError(5)
		return
	}
	buffer = buffer[:n]
	if !c.WriteSegmentBytes(c.Seg[cpu386.SegDS], c.R[cpu386.EDX], buffer) {
		if n > 0 {
			_, _ = file.Seek(-int64(n), io.SeekCurrent)
		}
		setError(5)
		return
	}
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(n)
	c.EFlags &^= cpu386.CF
}

func (s *FD2StartupDOS) seekFile(c *cpu386.CPU) {
	setError := func(code uint16) {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
		c.EFlags |= cpu386.CF
	}
	file, ok := s.handles[uint16(c.R[cpu386.EBX])]
	if !ok {
		setError(6)
		return
	}
	origin := uint8(c.R[cpu386.EAX])
	if origin > 2 {
		setError(1)
		return
	}
	oldPosition, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		setError(1)
		return
	}
	rawOffset := uint32(uint16(c.R[cpu386.ECX]))<<16 | uint32(uint16(c.R[cpu386.EDX]))
	position, err := file.Seek(int64(int32(rawOffset)), int(origin))
	if err != nil || position < 0 || uint64(position) > uint64(^uint32(0)) {
		_, _ = file.Seek(oldPosition, io.SeekStart)
		setError(1)
		return
	}
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(position)&0xffff
	c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | uint32(position)>>16
	c.EFlags &^= cpu386.CF
}

func (s *FD2StartupDOS) Handle(c *cpu386.CPU, number uint8) bool {
	if number == 0x31 {
		// 整支交給通用的 DPMI 主機。沒實作的功能由它記一筆再回 false，
		// 與這裡原本的行為一致（未列的呼叫一律拒絕）。
		return s.dpmi().Handle(c)
	}
	if number != 0x21 {
		return false
	}
	function := uint8(c.R[cpu386.EAX] >> 8)
	vectorNumber := uint8(c.R[cpu386.EAX])
	if function == 0x35 {
		vector := s.dosVectors[vectorNumber]
		c.Seg[cpu386.SegES] = uint16(vector >> 32)
		c.R[cpu386.EBX] = uint32(vector)
		c.EFlags &^= cpu386.CF
		return true
	}
	if function == 0x25 {
		s.dosVectors[vectorNumber] = uint64(c.Seg[cpu386.SegDS])<<32 | uint64(c.R[cpu386.EDX])
		c.EFlags &^= cpu386.CF
		return true
	}
	if function == 0x3d {
		s.openReadOnly(c)
		return true
	}
	if function == 0x44 {
		s.deviceInformation(c)
		return true
	}
	if function == 0x3f {
		s.readFile(c)
		return true
	}
	if function == 0x42 {
		s.seekFile(c)
		return true
	}
	switch s.calls {
	case 0:
		if uint8(c.R[cpu386.EAX]>>8) != 0x30 || c.R[cpu386.EBX] != 0x50484152 {
			return false
		}
		c.Seg[cpu386.SegDS] = 0x0160
		c.Seg[cpu386.SegES] = 0x0028
		c.Seg[cpu386.SegGS] = 0x0020
		c.Seg[cpu386.SegSS] = 0x0160
		c.SetDescriptor(0x0160, cpu386.Descriptor{Base: 0, Limit: 0xffffffff, Writable: true})
		c.SegmentLoadOK = func(selector uint16, destination int) bool {
			return selector == 0x0028 && (destination == cpu386.SegDS || destination == cpu386.SegES) ||
				selector == 0x0030 && (destination == cpu386.SegDS || destination == cpu386.SegES || destination == cpu386.SegFS)
		}
		c.SegmentRead8 = func(selector uint16, offset uint32) (uint8, bool) {
			if selector == 0x0028 && offset == 0x0080 {
				return 0, true
			}
			if selector == 0x0030 && uint64(offset) < uint64(len(minimalFD2Environment)) {
				return minimalFD2Environment[offset], true
			}
			return 0, false
		}
		c.SegmentRead16 = func(selector uint16, offset uint32) (uint16, bool) {
			if selector == 0x0028 && offset == 0x002c {
				return 0x0030, true
			}
			return 0, false
		}
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | 0x1606
	case 1:
		if uint16(c.R[cpu386.EAX]) != 0xff00 || uint16(c.R[cpu386.EDX]) != 0x0078 {
			return false
		}
		c.R[cpu386.EAX] = 0x4734ffff
		c.Seg[cpu386.SegGS] = 0x0020
	default:
		if uint8(c.R[cpu386.EAX]>>8) != 0x2c {
			return false
		}
		second := uint8(s.timeCalls % 60)
		c.R[cpu386.ECX] &= 0xffff0000
		c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | uint32(second)<<8
		s.timeCalls++
	}
	s.calls++
	return true
}
