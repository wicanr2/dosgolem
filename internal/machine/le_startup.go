package machine

import (
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/dosfile"
)

// FD2StartupDOS 是保護模式（DOS/4GW 已載入）底下的 DOS 服務層。
//
// 名字裡的 FD2 現在只涵蓋**啟動握手**那一段：`calls` 計數器加 `PHAR` 判斷、
// 寫死的四個 selector、`AX=FF00h` 的 DOS/4G 私有呼叫。這些回傳與 selector
// 只在 FD2 路徑驗過；MOO2 暫定入口會明示沿用此平台近似，不把它當原版
// MOO2 服務證據（`docs/spec/184-mvp-scope-review` 批次 4、規格 200）。
//
// 其餘的部分**與程式無關**：`int 31h` 全部交給 `DPMIHost`（`dpmi.go`），
// 檔案語意共用 `internal/dosfile`（與 16 位元那條同一份），
// 主控台與結束是照 DOS 的定義做的。未列的呼叫仍然一律拒絕——
// 安靜地放行會讓「這支程式踩到我們沒做的服務」看不出來。
type FD2StartupDOS struct {
	calls     int
	timeCalls int
	calendar  *leDOSCalendar
	// environment 只供明示的測試啟動設定使用；零值保留 FD2 歷史設定。
	environment []byte
	moo2Profile bool
	// MOO2 固定啟動診斷只記錄已設定的模式；不代表 BDA 或實際畫面。
	videoModeSet bool
	videoMode    uint8
	// 固定 0101h 模式；未附掛時只記狀態，顯存延伸見規格 265。
	vbeModeSet bool
	vbeMode    uint16
	vbeVideo   *moo2VBEVideo
	// 未啟用時只記零座標；附掛顯存的有效起點消費見規格 266。
	vbeStartSet          bool
	vbeStartX, vbeStartY uint16
	// 只供已明示 MOO2 啟動設定的受控滑鼠平台服務使用。
	mouseQueryEnabled        bool
	mouseX, mouseY           uint16
	mouseButtons             uint16
	mouseRangeX, mouseRangeY mouseCoordinateRange
	// 規格 230／254 的裁切設定值；不改未建模的實體移動速度。
	mouseSensitivityX, mouseSensitivityY, mouseDoubleSpeed uint16
	mouseCallback                                          *leMouseCallbackDispatcher

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
	DPMI          *DPMIHost
	dosVectors    [256]uint64
	protectedIRQ0 *leProtectedIRQ0
	dtaSelector   uint16
	dtaOffset     uint32
	dtaSet        bool
	files         ReadOnlyFileProvider
	table         *dosfile.Table
}

var minimalFD2Environment = []byte{0, 0, 1, 0, 'F', 'D', '2', '.', 'E', 'X', 'E', 0}
var minimalMOO2Environment = []byte{0, 0, 1, 0, 'O', 'R', 'I', 'O', 'N', '2', '.', 'E', 'X', 'E', 0}

const moo2MouseCenterX, moo2MouseCenterY = 320, 100

// 規格 253：有號 16 位包含端點的受控範圍，不建模主機游標比例與粒度。
type mouseCoordinateRange struct {
	set              bool
	minimum, maximum int16
}

func newMouseCoordinateRange(a, b uint16) mouseCoordinateRange {
	lower, upper := int16(a), int16(b)
	if lower > upper {
		lower, upper = upper, lower
	}
	return mouseCoordinateRange{set: true, minimum: lower, maximum: upper}
}

func (r mouseCoordinateRange) constrain(raw uint16) uint16 {
	if !r.set {
		return raw
	}
	value := int16(raw)
	if value < r.minimum {
		return uint16(r.minimum)
	}
	if value > r.maximum {
		return uint16(r.maximum)
	}
	return raw
}

// MOO2StartupDOS 的兩次啟動回傳以固定 1.31 DOSBox-X 輔助收據為基線；
// PSP／環境讀取仍是明示的合成輸入，不是完整原版對拍收據。
type MOO2StartupDOS struct{ *FD2StartupDOS }

func NewMOO2StartupDOS(files ReadOnlyFileProvider) *MOO2StartupDOS {
	s := NewFD2StartupDOS(files)
	s.environment = minimalMOO2Environment
	s.moo2Profile = true
	s.mouseQueryEnabled = true
	s.mouseX, s.mouseY = moo2MouseCenterX, moo2MouseCenterY
	s.mouseSensitivityX, s.mouseSensitivityY, s.mouseDoubleSpeed = 50, 50, 50
	s.DPMI.RealModeBIOS = moo2VBEControllerInfo
	return &MOO2StartupDOS{s}
}

// MOO2 的兩種 CPU 模式使用同一份已存在的受限 DSP／OPL／DMA 埠狀態。
// 這只接線平台模型；未知埠仍由裝置自行拒絕。
func (s *MOO2StartupDOS) AttachMachine(m *LEMachine) error {
	if err := InstallDOS4GWBIOSData(m); err != nil {
		return err
	}
	ports := NewLEOPLPorts()
	if !InstallLEBIOSClock(m, ports) {
		return errors.New("MOO2 BIOS 時鐘安裝失敗")
	}
	s.FD2StartupDOS.AttachMachine(m)
	s.protectedIRQ0 = installLEProtectedIRQ0(m, s.FD2StartupDOS, ports)
	installLEHardwareKeyboardIRQ1(m, s.FD2StartupDOS, ports)
	s.mouseCallback = installLEMouseCallback(m, s.DPMI)
	m.CPU.PortIn, m.CPU.PortOut = ports.In8, ports.Out8
	s.DPMI.RealModeIO = ports
	s.vbeVideo = newMOO2VBEVideo(ports)
	m.vbeVideo = s.vbeVideo
	if s.vbeModeSet && s.vbeMode == 0x0101 {
		s.vbeVideo.setMode()
	}
	return nil
}

// SetMouseState 設定下一次保護模式滑鼠查詢要回報的受控輸入。
func (s *MOO2StartupDOS) SetMouseState(x, y, buttons uint16) {
	s.mouseX, s.mouseY, s.mouseButtons = s.mouseRangeX.constrain(x), s.mouseRangeY.constrain(y), buttons
}

func (s *FD2StartupDOS) Calls() int { return s.calls }

func (s *FD2StartupDOS) startupEnvironment() []byte {
	if s.environment != nil {
		return s.environment
	}
	return minimalFD2Environment
}

func NewFD2StartupDOS(files ReadOnlyFileProvider) *FD2StartupDOS {
	s := &FD2StartupDOS{
		files: files,
		table: dosfile.NewReusingTable(),
		DPMI:  NewDPMIHost(nil),
	}
	s.DPMI.RealModeInterrupt = s.HandleRealMode
	return s
}

// handles 回這一支的 handle 表，零值也能用（測試常常直接造 &FD2StartupDOS{}）。
func (s *FD2StartupDOS) handles() *dosfile.Table {
	if s.table == nil {
		s.table = dosfile.NewReusingTable()
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
	mode := uint8(c.R[cpu386.EAX])
	if mode > 2 || s.files == nil {
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
	var file io.ReadSeekCloser
	var err error
	if mode == 0 {
		file, err = s.files.OpenRead(string(path))
	} else {
		provider, ok := s.files.(WriteFileProvider)
		if !ok {
			setError(dosfile.ErrAccessDenied)
			return
		}
		file, err = provider.OpenWrite(string(path), mode == 2)
	}
	if err != nil {
		code := uint16(dosfile.ErrAccessDenied)
		if errors.Is(err, fs.ErrNotExist) {
			code = dosfile.ErrFileNotFound
		}
		setError(code)
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
	count32 := c.R[cpu386.ECX]
	if count32 > 64*1024*1024 || uint64(c.R[cpu386.EDX])+uint64(count32) > uint64(1)<<32 {
		setError(dosfile.ErrAccessDenied)
		return
	}
	if count32 > 0 {
		if _, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+count32-1); !ok {
			setError(dosfile.ErrAccessDenied)
			return
		}
	}
	count := int(count32)
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
	c.R[cpu386.EAX] = uint32(n)
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

// exactDOSName 接受這個已驗證切片所需的單一 8.3 檔名；其他 DOS 搜尋樣式留待另證。
func exactDOSName(name string) (base, ext string, ok bool) {
	if len(name) == 0 || len(name) > 12 || strings.Count(name, ".") > 1 {
		return "", "", false
	}
	parts := strings.Split(name, ".")
	if len(parts[0]) == 0 || len(parts[0]) > 8 || len(parts) == 2 && (len(parts[1]) == 0 || len(parts[1]) > 3) {
		return "", "", false
	}
	for _, part := range parts {
		for i := 0; i < len(part); i++ {
			ch := part[i]
			if ch >= 'a' && ch <= 'z' {
				continue
			}
			if ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-' {
				continue
			}
			return "", "", false
		}
	}
	base = strings.ToUpper(parts[0])
	if len(parts) == 2 {
		ext = strings.ToUpper(parts[1])
	}
	return base, ext, true
}

func (s *FD2StartupDOS) findFirstExact(c *cpu386.CPU) bool {
	if !s.dtaSet || c.R[cpu386.ECX]&0xffff != 0 {
		return false
	}
	maximum := uint32(12)
	if s.moo2Profile {
		// 規格 261：只多容許一次目前目錄前綴，提供者仍只接單一檔名。
		maximum += 2
	}
	name := make([]byte, 0, maximum)
	terminated := false
	for i := uint32(0); i <= maximum; i++ {
		if c.R[cpu386.EDX] > ^uint32(0)-i {
			return false
		}
		ch, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+i)
		if !ok {
			return false
		}
		if ch == 0 {
			terminated = true
			break
		}
		name = append(name, ch)
	}
	lookupName := string(name)
	if s.moo2Profile {
		lookupName = strings.TrimPrefix(lookupName, ".\\")
	}
	base, ext, ok := exactDOSName(lookupName)
	if !terminated || !ok {
		return false
	}
	var dta [43]byte
	for i := range dta {
		if s.dtaOffset > ^uint32(0)-uint32(i) {
			return false
		}
		value, readable := c.ReadSegment8(s.dtaSelector, s.dtaOffset+uint32(i))
		if !readable {
			return false
		}
		dta[i] = value
	}
	dta[0] = 2 // 此啟動環境的 C:。
	for i := 1; i < 12; i++ {
		dta[i] = 0
	}
	copy(dta[1:9], base)
	copy(dta[9:12], ext)

	var file io.ReadSeekCloser
	var err error
	if s.files != nil {
		file, err = s.files.OpenRead(lookupName)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if s.files != nil && file == nil && err == nil {
		return false
	}
	if file != nil {
		defer file.Close()
		statFile, hasStat := file.(interface{ Stat() (fs.FileInfo, error) })
		if !hasStat {
			return false
		}
		info, statErr := statFile.Stat()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > 0xffffffff {
			return false
		}
		stamp := info.ModTime().UTC()
		year := stamp.Year()
		if year < 1980 {
			year = 1980
		} else if year > 2107 {
			year = 2107
		}
		dta[0x15] = 0x20
		binary.LittleEndian.PutUint16(dta[0x16:], uint16(stamp.Hour()<<11|stamp.Minute()<<5|stamp.Second()/2))
		binary.LittleEndian.PutUint16(dta[0x18:], uint16((year-1980)<<9|int(stamp.Month())<<5|stamp.Day()))
		binary.LittleEndian.PutUint32(dta[0x1a:], uint32(info.Size()))
		for i := 0x1e; i < len(dta); i++ {
			dta[i] = 0
		}
		copy(dta[0x1e:], base)
		if ext != "" {
			dta[0x1e+len(base)] = '.'
			copy(dta[0x1f+len(base):], ext)
		}
	}
	if !c.WriteSegmentBytes(s.dtaSelector, s.dtaOffset, dta[:]) {
		return false
	}
	if file == nil {
		c.R[cpu386.EAX] = 0x12 // 此原版缺檔收據返回完整 EAX=12h。
		c.EFlags |= cpu386.CF
	} else {
		c.R[cpu386.EAX] &= 0xffff0000
		c.EFlags &^= cpu386.CF
	}
	return true
}

func (s *FD2StartupDOS) Handle(c *cpu386.CPU, number uint8) bool {
	if number == 0x2f {
		if !s.moo2Profile {
			return false
		}
		switch uint16(c.R[cpu386.EAX]) {
		case 0x160a:
			// 規格 258：未安裝 Windows 的版本查詢保持架構狀態。
			return true
		case 0x1684:
			// 規格 275：未安裝 VTD，空入口輸入與高半部均保持。
			return uint16(c.R[cpu386.EBX]) == 5 &&
				c.Seg[cpu386.SegES] == 0 && uint16(c.R[cpu386.EDI]) == 0
		}
		return false
	}
	if number == 0x10 {
		if !s.moo2Profile {
			return false
		}
		switch c.R[cpu386.EAX] {
		case 3:
			s.videoMode, s.videoModeSet = 3, true
			s.vbeModeSet = false
			if s.vbeVideo != nil {
				s.vbeVideo.active = false
			}
			s.vbeStartSet = false
			return true
		case 0x4f05:
			return s.vbeVideo.control(c)
		case 0x4f07:
			if s.vbeVideo != nil && s.vbeVideo.active {
				if !s.vbeVideo.displayStart(c) {
					return false
				}
				s.vbeStartX, s.vbeStartY, s.vbeStartSet = 0, s.vbeVideo.startY, true
				return true
			}
			if c.R[cpu386.EBX] != 0 || c.R[cpu386.ECX] != 0 || c.R[cpu386.EDX] != 0 {
				return false
			}
			s.vbeStartX, s.vbeStartY, s.vbeStartSet = 0, 0, true
			c.R[cpu386.EAX] = 0x4f
			return true
		case 0x4f02:
			if c.R[cpu386.EBX] != 0x0101 || c.R[cpu386.ECX] != 0 || c.R[cpu386.EDX] != 0 {
				return false
			}
			s.vbeMode, s.vbeModeSet = 0x0101, true
			s.vbeStartX, s.vbeStartY, s.vbeStartSet = 0, 0, true
			if s.vbeVideo != nil {
				s.vbeVideo.setMode()
			}
			c.R[cpu386.EAX] = 0x4f
			return true
		default:
			return false
		}
	}
	if number == 0x33 {
		if !s.mouseQueryEnabled {
			return false
		}
		switch uint16(c.R[cpu386.EAX]) {
		case 4:
			// 規格 257：程式設定座標不製造一般輸入事件。
			s.mouseX = s.mouseRangeX.constrain(uint16(c.R[cpu386.ECX]))
			s.mouseY = s.mouseRangeY.constrain(uint16(c.R[cpu386.EDX]))
			return true
		case 0x0c:
			return s.mouseCallback.register(c)
		case 0x14:
			return s.mouseCallback.exchange(c)
		case 7, 8:
			bounds := newMouseCoordinateRange(uint16(c.R[cpu386.ECX]), uint16(c.R[cpu386.EDX]))
			if uint16(c.R[cpu386.EAX]) == 7 {
				s.mouseRangeX, s.mouseX = bounds, bounds.constrain(s.mouseX)
			} else {
				s.mouseRangeY, s.mouseY = bounds, bounds.constrain(s.mouseY)
			}
			return true
		case 3:
			c.R[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | uint32(s.mouseButtons)
			c.R[cpu386.ECX] = c.R[cpu386.ECX]&0xffff0000 | uint32(s.mouseX)
			c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | uint32(s.mouseY)
			return true
		case 0x1a:
			s.mouseSensitivityX = min(uint16(c.R[cpu386.EBX]), 100)
			s.mouseSensitivityY = min(uint16(c.R[cpu386.ECX]), 100)
			s.mouseDoubleSpeed = min(uint16(c.R[cpu386.EDX]), 100)
			return true
		case 0x1b:
			c.R[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | uint32(s.mouseSensitivityX)
			c.R[cpu386.ECX] = c.R[cpu386.ECX]&0xffff0000 | uint32(s.mouseSensitivityY)
			c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | uint32(s.mouseDoubleSpeed)
			return true
		case 0, 0x21:
			s.mouseCallback.reset()
			// 規格 229／251：三按鍵返回；受控座標依目前模式中心近似。
			s.mouseRangeX, s.mouseRangeY = mouseCoordinateRange{}, mouseCoordinateRange{}
			s.mouseButtons = 0
			s.mouseX, s.mouseY = moo2MouseCenterX, moo2MouseCenterY
			if s.vbeModeSet && s.vbeMode == 0x0101 {
				s.mouseY = 240
			}
			c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | 0xffff
			c.R[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | 3
			return true
		default:
			return false
		}
	}
	if number == 0x31 {
		// 整支交給通用的 DPMI 主機。沒實作的功能由它記一筆再回 false，
		// 與這裡原本的行為一致（未列的呼叫一律拒絕）。
		return s.dpmi().Handle(c)
	}
	if number != 0x21 {
		return false
	}
	function := uint8(c.R[cpu386.EAX] >> 8)
	if function == 0x2a {
		return s.getCalendarDate(c)
	}
	vectorNumber := uint8(c.R[cpu386.EAX])
	if function == 0x1a {
		// DOS DTA 只保存呼叫當下的指標；後續搜尋服務才讀寫該記憶體。
		s.dtaSelector = c.Seg[cpu386.SegDS]
		s.dtaOffset = c.R[cpu386.EDX]
		s.dtaSet = true
		return true
	}
	if function == 0x4e {
		return s.findFirstExact(c)
	}
	if function == 0x35 {
		if s.protectedIRQ0 != nil && s.protectedIRQ0.m.CPU != c {
			return false
		}
		vector := s.dosVectors[vectorNumber]
		if s.protectedIRQ0 != nil && vector == 0 {
			var ok bool
			vector, ok = s.protectedIRQ0.defaultVector(vectorNumber)
			if !ok {
				return false
			}
			s.dosVectors[vectorNumber] = vector
		}
		if s.protectedIRQ0 != nil {
			if !s.protectedIRQ0.validTarget(c, vector) {
				return false
			}
			if s.protectedIRQ0.isDefault(vector, vectorNumber) {
				if _, ok := s.protectedIRQ0.defaultVector(vectorNumber); !ok {
					return false
				}
			}
		}
		c.Seg[cpu386.SegES] = uint16(vector >> 32)
		c.R[cpu386.EBX] = uint32(vector)
		c.EFlags &^= cpu386.CF
		return true
	}
	if function == 0x25 {
		if s.protectedIRQ0 != nil && s.protectedIRQ0.m.CPU != c {
			return false
		}
		vector := uint64(c.Seg[cpu386.SegDS])<<32 | uint64(c.R[cpu386.EDX])
		if s.protectedIRQ0 != nil && !s.protectedIRQ0.validTarget(c, vector) {
			return false
		}
		s.dosVectors[vectorNumber] = vector
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
		dataSelector := uint16(0x0160)
		if s.moo2Profile {
			dataSelector = 0x0188
		}
		c.Seg[cpu386.SegDS] = dataSelector
		c.Seg[cpu386.SegES] = 0x0028
		c.Seg[cpu386.SegGS] = 0x0020
		c.Seg[cpu386.SegSS] = dataSelector
		c.SetDescriptor(dataSelector, cpu386.Descriptor{Base: 0, Limit: 0xffffffff, Writable: true})
		c.SegmentLoadOK = func(selector uint16, destination int) bool {
			return s.moo2Profile && selector == 0x0020 && destination == cpu386.SegGS ||
				selector == 0x0028 && (destination == cpu386.SegDS || destination == cpu386.SegES) ||
				selector == 0x0030 && (destination == cpu386.SegDS || destination == cpu386.SegES || destination == cpu386.SegFS)
		}
		c.SegmentRead8 = func(selector uint16, offset uint32) (uint8, bool) {
			environment := s.startupEnvironment()
			if selector == 0x0028 && offset == 0x0080 {
				return 0, true
			}
			if selector == 0x0030 && uint64(offset) < uint64(len(environment)) {
				return environment[offset], true
			}
			return 0, false
		}
		c.SegmentRead16 = func(selector uint16, offset uint32) (uint16, bool) {
			if selector == 0x0028 && offset == 0x002c {
				return 0x0030, true
			}
			return 0, false
		}
		if s.moo2Profile {
			// MOO2 1.31：固定 DOSBox-X 的 0180:00333FB7 返回收據。
			c.R[cpu386.EAX] = 0x00000005
			c.R[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | 0xff00
		} else {
			c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | 0x1606
		}
	case 1:
		if uint16(c.R[cpu386.EAX]) != 0xff00 || uint16(c.R[cpu386.EDX]) != 0x0078 {
			return false
		}
		c.R[cpu386.EAX] = 0x4734ffff
		c.Seg[cpu386.SegGS] = 0x0020
		if s.moo2Profile {
			c.EFlags &^= cpu386.CF
		}
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
// FD2 採 DOS 的最低空閒代號重用；關閉後至下次配置前，此代號無效。
func (s *FD2StartupDOS) closeFile(c *cpu386.CPU) {
	if code := s.handles().Close(uint16(c.R[cpu386.EBX])); code != 0 {
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(code)
		c.EFlags |= cpu386.CF
		return
	}
	c.EFlags &^= cpu386.CF
}

// writeFile 是保護模式 `AH=40h`：BX ＝ handle、ECX ＝ 位元組數、DS:EDX ＝ 資料。
//
// ⚠ **這是保護模式程式對外面說話的主要管道**，不是 `AH=09h`。
// Watcom 的 `printf`／`fputs` 最後都落到 handle 1 的 `AH=40h`，一次一小段。
// 不接的話主控台是空的——看起來像「程式什麼都沒說」，
// 而實際上它正在印錯誤訊息。
//
// 只有可寫覆蓋層接受檔案寫入；唯讀提供者拒絕，不能假裝資料已落地。
func (s *FD2StartupDOS) writeFile(c *cpu386.CPU) {
	handle := uint16(c.R[cpu386.EBX])
	count := c.R[cpu386.ECX]
	if count > 64*1024*1024 || uint64(c.R[cpu386.EDX])+uint64(count) > uint64(1)<<32 {
		c.R[cpu386.EAX] = dosfile.ErrAccessDenied
		c.EFlags |= cpu386.CF
		return
	}
	if handle != 1 && handle != 2 {
		setError := func(code uint32) { c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | code; c.EFlags |= cpu386.CF }
		file, ok := s.handles().Get(handle)
		if !ok {
			setError(dosfile.ErrInvalidHandle)
			return
		}
		writer, ok := file.(io.Writer)
		if !ok {
			setError(dosfile.ErrAccessDenied)
			return
		}
		if count == 0 {
			truncate, ok := file.(interface{ Truncate(int64) error })
			if !ok {
				setError(dosfile.ErrAccessDenied)
				return
			}
			pos, err := file.Seek(0, io.SeekCurrent)
			if err != nil || truncate.Truncate(pos) != nil {
				setError(dosfile.ErrAccessDenied)
				return
			}
			c.R[cpu386.EAX] = 0
			c.EFlags &^= cpu386.CF
			return
		}
		buffer := make([]byte, count)
		for i := uint32(0); i < count; i++ {
			if c.R[cpu386.EDX] > ^uint32(0)-i {
				setError(dosfile.ErrAccessDenied)
				return
			}
			value, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+i)
			if !ok {
				setError(dosfile.ErrAccessDenied)
				return
			}
			buffer[i] = value
		}
		n, err := writer.Write(buffer)
		if err != nil {
			setError(dosfile.ErrAccessDenied)
			return
		}
		c.R[cpu386.EAX] = uint32(n)
		c.EFlags &^= cpu386.CF
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
	c.R[cpu386.EAX] = uint32(len(buffer))
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

// HandleRealMode只轉接已支援的DOS檔案服務，與保護模式共用檔案表。
func (s *FD2StartupDOS) HandleRealMode(r *cpu.CPU, n uint8) bool {
	if n != 0x21 || s.DPMI == nil || s.DPMI.m == nil {
		return false
	}
	switch uint8(r.R[cpu.AX] >> 8) {
	case 0x3d, 0x3e, 0x3f, 0x42, 0x44:
	default:
		return false
	}
	c := cpu386.New(s.DPMI.m)
	for i, v := range r.R {
		c.R[i] = uint32(v)
	}
	c.R[cpu386.EAX] |= uint32(r.EAXHi) << 16
	c.EFlags = uint32(r.Flags)
	for _, v := range []struct {
		dst, src int
		selector uint16
	}{
		{cpu386.SegDS, cpu.DS, 0x10}, {cpu386.SegES, cpu.ES, 0x18}, {cpu386.SegSS, cpu.SS, 0x20},
	} {
		c.Seg[v.dst] = v.selector
		c.SetDescriptor(v.selector, cpu386.Descriptor{Base: uint32(r.Seg[v.src]) << 4, Limit: 0xffff, Writable: true})
	}
	if !s.Handle(c, n) {
		return false
	}
	for i, v := range c.R {
		r.R[i] = uint16(v)
	}
	r.EAXHi = uint16(c.R[cpu386.EAX] >> 16)
	r.SetFlags(uint16(c.EFlags))
	return true
}
