package machine

import (
	"io"

	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/dosfile"
)

// FD2StartupDOS 是保護模式（DOS/4GW 已載入）底下的 DOS 服務層。
//
// 名字裡的 FD2 現在只涵蓋**啟動握手**那一段：`calls` 計數器加 `PHAR` 判斷、
// 寫死的四個 selector、`AX=FF00h` 的 DOS/4G 私有呼叫。那一段是那一支
// 執行檔的形狀，換一支就不成立（`docs/spec/184-mvp-scope-review` 批次 4）。
//
// 其餘的部分**與程式無關**：`int 31h` 全部交給 `DPMIHost`（`dpmi.go`），
// 檔案語意共用 `internal/dosfile`（與 16 位元那條同一份），
// 主控台與結束是照 DOS 的定義做的。未列的呼叫仍然一律拒絕——
// 安靜地放行會讓「這支程式踩到我們沒做的服務」看不出來。
type FD2StartupDOS struct {
	calls     int
	timeCalls int

	// Console 收 `AH=40h`（handle 1／2）、`AH=09h`、`AH=02h` 的輸出。
	//
	// **這是保護模式程式對外面說話的主要管道。** 不收的話，程式印的
	// 錯誤訊息全部消失，看起來像「它什麼都沒說就停了」。
	Console []byte

	// Exited／ExitCode 記 `AH=4Ch`。要記下來而不是直接讓 CPU 亂走：
	// 程式結束之後那一段記憶體不再是有意義的碼。
	Exited   bool
	ExitCode uint8
	// DPMI 是**與程式無關**的 `int 31h` 主機（`dpmi.go`）。
	//
	// 這一支剩下的部分還是 FD2 專屬的（啟動握手、selector 值），
	// 但 DPMI 那一層已經搬出去了——換一支 DOS/4GW 程式時它照用，
	// 不必再抄一份（`docs/spec/184-mvp-scope-review` 批次 2）。
	DPMI       *DPMIHost
	dosVectors [256]uint64
	files      ReadOnlyFileProvider
	table      *dosfile.Table
}

var minimalFD2Environment = []byte{0, 0, 1, 0, 'F', 'D', '2', '.', 'E', 'X', 'E', 0}

func (s *FD2StartupDOS) Calls() int { return s.calls }

func NewFD2StartupDOS(files ReadOnlyFileProvider) *FD2StartupDOS {
	return &FD2StartupDOS{
		files: files,
		table: dosfile.NewTable(),
		DPMI:  NewDPMIHost(nil),
	}
}

// handles 回這一支的 handle 表，零值也能用（測試常常直接造 &FD2StartupDOS{}）。
func (s *FD2StartupDOS) handles() *dosfile.Table {
	if s.table == nil {
		s.table = dosfile.NewTable()
	}
	return s.table
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

func (s *FD2StartupDOS) HasHandle(handle uint16) bool { return s.handles().Has(handle) }

func (s *FD2StartupDOS) Close() error { return s.handles().CloseAll() }

func (s *FD2StartupDOS) openReadOnly(c *cpu386.CPU) {
	setError := func(code uint16) {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
		c.EFlags |= cpu386.CF
	}
	if uint8(c.R[cpu386.EAX]) != 0 || s.files == nil {
		setError(dosfile.ErrAccessDenied)
		return
	}
	path := make([]byte, 0, 32)
	terminated := false
	for offset := uint32(0); offset < 260; offset++ {
		if c.R[cpu386.EDX] > ^uint32(0)-offset {
			setError(dosfile.ErrPathNotFound)
			return
		}
		value, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+offset)
		if !ok {
			setError(dosfile.ErrPathNotFound)
			return
		}
		if value == 0 {
			terminated = true
			break
		}
		path = append(path, value)
	}
	if !terminated {
		setError(dosfile.ErrPathNotFound)
		return
	}
	file, err := s.files.OpenRead(string(path))
	if err != nil {
		setError(dosfile.ErrFileNotFound)
		return
	}
	handle, code := s.handles().Add(file, string(path))
	if code != 0 {
		file.Close()
		setError(code)
		return
	}
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(handle)
	c.EFlags &^= cpu386.CF
}

func (s *FD2StartupDOS) deviceInformation(c *cpu386.CPU) {
	setError := func(code uint16) {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
		c.EFlags |= cpu386.CF
	}
	if uint8(c.R[cpu386.EAX]) != 0 {
		setError(dosfile.ErrInvalidFunction)
		return
	}
	if !s.handles().Has(uint16(c.R[cpu386.EBX])) {
		setError(dosfile.ErrInvalidHandle)
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
	handle := uint16(c.R[cpu386.EBX])
	file, ok := s.handles().Get(handle)
	if !ok {
		setError(dosfile.ErrInvalidHandle)
		return
	}
	count := int(uint16(c.R[cpu386.ECX]))
	buffer := make([]byte, count)
	n, code := dosfile.Read(file, buffer)
	if code != 0 {
		setError(code)
		return
	}
	if !c.WriteSegmentBytes(c.Seg[cpu386.SegDS], c.R[cpu386.EDX], buffer[:n]) {
		// **寫不進去就把檔案指標退回去。** 不退的話，程式重試同一次讀取
		// 會從已經被吃掉的位置繼續，而它拿到的是檔案的下一段——
		// 那是一份看起來合法、內容錯位的資料。
		if n > 0 {
			_, _ = file.Seek(-int64(n), io.SeekCurrent)
		}
		setError(dosfile.ErrAccessDenied)
		return
	}
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(n)
	c.EFlags &^= cpu386.CF
}

func (s *FD2StartupDOS) seekFile(c *cpu386.CPU) {
	position, code := s.handles().Seek(uint16(c.R[cpu386.EBX]),
		dosfile.SignedOffset(uint16(c.R[cpu386.ECX]), uint16(c.R[cpu386.EDX])),
		uint8(c.R[cpu386.EAX]))
	if code != 0 {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
		c.EFlags |= cpu386.CF
		return
	}
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | position&0xffff
	c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | position>>16
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
	if function == 0x3e {
		s.closeFile(c)
		return true
	}
	if function == 0x40 {
		s.writeFile(c)
		return true
	}
	if function == 0x09 {
		s.printString(c)
		return true
	}
	if function == 0x02 {
		s.Console = append(s.Console, uint8(c.R[cpu386.EDX]))
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(uint8(c.R[cpu386.EDX]))
		c.EFlags &^= cpu386.CF
		return true
	}
	if function == 0x19 {
		// 目前磁碟機。2 ＝ C:，與 16 位元那條一致（`internal/dos` 的 Drive）。
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffffff00 | 2
		c.EFlags &^= cpu386.CF
		return true
	}
	if function == 0x4c || function == 0x00 {
		s.Exited = true
		s.ExitCode = uint8(c.R[cpu386.EAX])
		c.EFlags &^= cpu386.CF
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

// closeFile 是 `AH=3Eh`。
//
// **關過的號碼不重用**（`dosfile.Table` 保證）：重用的話，程式關掉之後
// 又拿舊號碼去讀，讀到的是別人的檔——而它不會報錯。
func (s *FD2StartupDOS) closeFile(c *cpu386.CPU) {
	if code := s.handles().Close(uint16(c.R[cpu386.EBX])); code != 0 {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
		c.EFlags |= cpu386.CF
		return
	}
	c.EFlags &^= cpu386.CF
}

// writeFile 是 `AH=40h`：BX ＝ handle、CX ＝ 位元組數、DS:EDX ＝ 資料。
//
// ⚠ **這是保護模式程式對外面說話的主要管道**，不是 `AH=09h`。
// Watcom 的 `printf`／`fputs` 最後都落到 handle 1 的 `AH=40h`，一次一小段。
// 不接的話主控台是空的——看起來像「程式什麼都沒說」，
// 而實際上它正在印錯誤訊息。
//
// 檔案 handle 一律回「拒絕存取」：這一層的檔案提供者是唯讀的
// （`ReadOnlyFileProvider`）。**回成功比較危險**——程式會以為資料落地了，
// 接著把它讀回來，然後拿到舊內容。
func (s *FD2StartupDOS) writeFile(c *cpu386.CPU) {
	handle := uint16(c.R[cpu386.EBX])
	count := uint32(uint16(c.R[cpu386.ECX]))
	if handle != 1 && handle != 2 {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | dosfile.ErrAccessDenied
		c.EFlags |= cpu386.CF
		return
	}
	buffer := make([]byte, 0, count)
	for offset := uint32(0); offset < count; offset++ {
		value, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+offset)
		if !ok {
			// 讀不到就照實回報「寫了幾個」，不要假裝整批都寫了。
			break
		}
		buffer = append(buffer, value)
	}
	s.Console = append(s.Console, buffer...)
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(len(buffer))
	c.EFlags &^= cpu386.CF
}

// printString 是 `AH=09h`：DS:EDX 起算、`$` 結尾的字串。
//
// ⚠ **結尾是 `$`，不是 NUL。** 當成 NUL 結尾的話，字串裡的 `$` 之後那一段
// 會一起印出來（多半是下一個字串），而畫面上看起來像「訊息接錯了」。
func (s *FD2StartupDOS) printString(c *cpu386.CPU) {
	for offset := uint32(0); offset < 65536; offset++ {
		value, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+offset)
		if !ok || value == '$' {
			break
		}
		s.Console = append(s.Console, value)
	}
	c.EFlags &^= cpu386.CF
}
