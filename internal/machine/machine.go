// Package machine 是一台跑得動 `RUN_full.EXE` 的 8086 機器：
// 1 MB 平坦記憶體、BIOS 資料區、中斷向量表、I/O 埠。
//
// 規格在 `docs/spec/003-machine-and-loader.md`（READY）。
//
// **它不認識 DOS 服務**——那是 `internal/dos` 的事。這一層只負責
// 「記憶體長什麼樣」與「程式怎麼被載進去」。
package machine

import "github.com/wicanr2/dosgolem/internal/cpu"

// 記憶體佈局。取值沿用 `rich2/tools/dosemu.py` 那一版——**它是實際把
// `RUN.EXE` 跑到資產全部載完的那一份**，不是重新挑的。
const (
	// MemSize 是整個真實模式位址空間。
	MemSize = 1 << 20

	// MCBSeg 是假的記憶體控制區塊鏈起點，LOLSeg 是假的 DOS「list of lists」。
	MCBSeg = 0x0060
	LOLSeg = 0x0070

	// StubSeg 放中斷向量的預設目標。每個向量各有一段 stub，
	// 共佔 256×stubStride bytes（0x800–0xBFF）——見 initVectors。
	StubSeg = 0x0080

	// EnvSeg 是環境區塊；`PSP+2Ch` 指向它。內容只有二十幾 bytes，
	// 但要留在 stub 區之後、PSP 之前。
	EnvSeg = 0x00C0

	// PSPSeg 是程式的 PSP，LoadSeg 是映像本體（PSP 佔 16 段）。
	PSPSeg  = 0x0100
	LoadSeg = PSPSeg + 0x10

	// MemTop 是傳統記憶體上緣（640 KB）。
	MemTop = 0x9FFF

	// VideoSeg 是 mode 13h 的畫面。
	VideoSeg   = 0xA000
	VideoWidth = 320
	VideoHigh  = 200
)

// stubStride 是每個向量的 stub 佔幾個 byte（`CD n` ＋ `CF` ＋ 對齊）。
const stubStride = 4

// StubOff 回向量 n 的 stub 在 StubSeg 內的位移。
func StubOff(n uint8) uint16 { return uint16(n) * stubStride }

// DefaultIRQ0Every 是計時器中斷的間隔，單位是**指令數**。
//
// 真機是 18.2 Hz（55 ms）。以 DOSBox 預設的 3,000 cycles/ms 換算大約
// 165,000 道指令，這裡取整。用指令數而不是時間，是為了讓對拍決定性——
// 同一組輸入永遠得到同一個畫面。
const DefaultIRQ0Every = 165_000

// PortWrite 是一次埠寫入。音訊 parity 只需要這份序列，不必合成聲音
// （`docs/spec/004` §6）。
// OPLWrite 是一次 OPL2 暫存器寫入。
//
// **暫存器串是「樂譜」，波形是「演奏」。** 對拍暫存器串驗的是前者——
// 那是決定性的、可以逐筆比；波形要逐樣本一致屬於既定停止線
// （`rich2/docs/spec/049`）。
type OPLWrite struct {
	Reg  uint8
	Val  uint8
	Step uint64
	// Bank 是 OPL3 的哪一組暫存器：0 ＝ 0x388/0x389（OPL2 相容），
	// 1 ＝ 0x38A/0x38B（OPL3 才有的第二組）。
	//
	// **只有 OPL2 的程式一律是 0**，所以既有的呼叫端不必理它。
	Bank uint8
}

type PortWrite struct {
	Port uint16
	Val  uint8
	// Step 是發生在第幾道指令，用來對齊時序。
	Step uint64
}

// Machine 是一台機器。用 New 造。
type Machine struct {
	Mem []uint8
	CPU *cpu.CPU

	// OPL 是解碼過的 OPL2 暫存器寫入序列（見 Out8）。
	// **這是音樂 parity 的對拍對象**：兩邊的暫存器串一樣，
	// 合成器再怎麼不同，送給晶片的指令是一樣的。
	OPL []OPLWrite

	// 寫入監看（WatchWrites）。watchLo > watchHi 表示關閉。
	watchLo, watchHi uint32
	onWrite          func(addr uint32, old, new uint8)

	// OPL2／OPL3 的狀態。
	//
	// `oplReg` 是兩組各自的暫存器索引（0x388 與 0x38A 分開選）。
	// `oplRegs` 是暫存器檔本身——**寫入序列與最終狀態是兩件事**：
	// 序列看得出「怎麼做的」，狀態看得出「現在是什麼」。
	// 對拍解碼器用前者，對拍某一刻的音色用後者。
	oplReg     [2]uint8
	oplRegs    [2][256]uint8
	oplPresent bool
	// 計時器：兩個各自有「啟動」與「遮罩」，狀態埠的 bit7 是兩者的 OR。
	oplT1Started, oplT2Started bool
	oplT1Masked, oplT2Masked   bool

	// Ports 是每個埠最後一次寫進去的值；PortLog 是完整序列。
	Ports   map[uint16]uint8
	PortLog []PortWrite

	// PortsIn 是每個埠被讀了幾次。**輪詢埠的次數會很大**，那是正常的。
	PortsIn map[uint16]uint64

	// IRQ0Every 是每幾道指令送一次計時器中斷。0 ＝ 不送。
	//
	// 預設 DefaultIRQ0Every。**這個值影響動畫跑多快，不影響最終停下來的
	// 畫面**——防拷畫面是靜態的，動畫播完就穩定。
	IRQ0Every uint64

	// Ticks 是送出去的計時器中斷次數。
	Ticks uint64

	// portTicks 是所有 `in` 的累計，當作輪詢埠的時鐘。
	portTicks uint64

	nextIRQ0    uint64
	irq0Pending bool

	// DAC 是 VGA 調色盤，256×3 個 6 位元色值（`docs/formats/001` 的格式）。
	DAC [256 * 3]uint8

	// AttrPal 是屬性控制器的 16 色對映（預設 identity）。
	// 16 色 planar 模式的色彩鏈是「4 位元色號 → AttrPal → DAC」
	// （`docs/spec/009` §1）。
	AttrPal [16]uint8

	// Overscan 是邊框色（屬性控制器暫存器 11h）。遊戲會讀回來存檔再還原，
	// 沒有它的話「讀回 → 還原」那條路會拿到垃圾。
	Overscan uint8

	dacIndex uint8
	dacPhase uint8

	// WriteModeUse 統計 planar 寫入用過哪些 write mode（診斷用）。
	WriteModeUse [4]uint64

	// ModeChanges 記錄每次模式切換（bda.go SetVideoMode）。
	ModeChanges []ModeChange

	// planar VRAM 狀態（`docs/spec/009`）：四個 plane、sequencer／GC
	// 暫存器檔、latch。planar 生效與否看 BDA 的目前模式。
	vram   [4][0x10000]uint8
	seq    [8]uint8
	seqIdx uint8
	gc     [16]uint8
	gcIdx  uint8
	latch  [4]uint8

	// Steps 是已經執行的指令數。**不是週期數**——時序要等 M2
	// （`docs/spec/004` §5）。
	Steps uint64

	// FreeSeg 是映像之後第一個可配置的段。`int 21h AH=4Ah` 的探測要用它
	// （`docs/spec/004` §1.2）。
	FreeSeg uint16

	// ImageBase／ImageLen 是載進去的映像位置與長度。
	ImageBase uint32
	ImageLen  int
}

// New 造一台機器：記憶體清空、BDA 建好、向量表填好。
//
// **還沒有程式**——要呼叫 LoadEXE。
func New() *Machine {
	m := &Machine{
		Mem:       make([]uint8, MemSize),
		Ports:     map[uint16]uint8{},
		PortsIn:   map[uint16]uint64{},
		IRQ0Every: DefaultIRQ0Every,
		// 空區間 ＝ 監看關閉（見 WatchWrites）。零值的 lo=hi=0 會誤中位址 0。
		watchLo: 1, watchHi: 0,
	}
	m.CPU = cpu.New(m)
	// **這台機器是拿來跑 1993 年的 DOS 軟體的，不是拿來過語料的。**
	// `RUN_full.EXE` 的主程式區有 3,345 個 80186 的 `PUSH imm`；用 8086 的
	// 別名解讀會錯位一個 byte，然後安靜地飛掉（`docs/spec/002` §1.1）。
	// 語料驗收走 `cpu.New()`，那邊維持 8086 預設。
	m.CPU.Model = cpu.Model80186
	m.seq[2] = 0x0F // map mask：四個 plane 都開（BIOS mode-set 後的常態）
	m.gc[8] = 0xFF  // 位元遮罩：全開
	for i := range m.AttrPal {
		m.AttrPal[i] = uint8(i)
	}
	m.initBDA()
	m.initVectors()
	return m
}

// ---- cpu.Bus ------------------------------------------------------------

func (m *Machine) Read8(a uint32) uint8 {
	a &= 0xFFFFF
	if a >= 0xA0000 && a < 0xB0000 && m.planarVideo() {
		return m.planarRead(a - 0xA0000)
	}
	return m.Mem[a]
}

func (m *Machine) Write8(a uint32, v uint8) {
	a &= 0xFFFFF
	if m.watchLo <= a && a <= m.watchHi && m.Mem[a] != v {
		m.onWrite(a, m.Mem[a], v)
	}
	m.Mem[a] = v
	if a >= 0xA0000 && a < 0xB0000 && m.planarVideo() {
		m.planarWrite(a-0xA0000, v)
	}
}

// WatchWrites 監看一段線性位址的寫入。
//
// **這是「誰寫這個位址」唯一直接的答案。** 靜態 xref 只涵蓋直接參考——
// `mov ds:XXXXh, ax` 抓得到，`mov [si+456h], ax` 抓不到，而後者正是
// 那些「掃不到寫入端」的變數的寫法（`rich2/CLAUDE.md` §4.1 第 4 條）。
//
// 只在**值真的變了**的時候通知，所以重複寫同一個值不會洗版。
// 傳 nil 關掉監看。CPU 的每一次寫入都走 Write8，所以 16 位寫入會來兩次。
func (m *Machine) WatchWrites(lo, hi uint32, fn func(addr uint32, old, new uint8)) {
	if fn == nil {
		m.watchLo, m.watchHi, m.onWrite = 1, 0, nil // 空區間 ＝ 永遠不命中
		return
	}
	m.watchLo, m.watchHi, m.onWrite = lo, hi, fn
}

// In8 回 0xFF。**空的匯流排上讀到的就是 0xFF，不是 0**——
// 有些偵測用「讀回來不是 FF」判定裝置存在，回 0 會讓它們誤判。
// In8 回應 `in`。
//
// ⚠ **輪詢埠的值一定要會變，否則程式死在等待迴圈裡**——而那看起來像
// 「程式沒走到那一步」，不像模擬器缺東西。VGA 的垂直回掃等待長這樣：
//
//	mov dx,0x3DA
//	in al,dx ; test al,8 ; jnz -5   ; 等這一次回掃結束
//	in al,dx ; test al,8 ; jz  -5   ; 等下一次回掃開始 ← 定值就死在這
//
// 不管回定值 0 還是定值 0FFh，兩段一定有一段轉不出來。
// 出處是 `rich2/tools/dosemu.py` 的 `on_in`（那支跑到畫出防拷畫面）。
//
// **這是行為模型，不是時序模型**：拿 `in` 的累計次數當時鐘，
// 不是週期精確的（時序在 M2，`docs/spec/004` §5）。
// SetAdLib 決定偵測時要不要讓 OPL2 存在。
//
// ⚠ **預設是不存在**，因為那讓開機少跑一大段。打開之後開機會慢，
// 但音樂路徑才會真的執行、`OPL` 才會有東西。
func (m *Machine) SetAdLib(present bool) { m.oplPresent = present }

// OPL 暫存器 04h（計時器控制）的位元（YM3812 資料表）。
const (
	oplT1Start = 0x01 // 啟動計時器 1
	oplT2Start = 0x02 // 啟動計時器 2
	oplT2Mask  = 0x20 // 遮罩計時器 2 的狀態位元
	oplT1Mask  = 0x40 // 遮罩計時器 1 的狀態位元
	oplIRQReset = 0x80 // 重置 IRQ 與兩個逾時旗標（**此時其他位元一律忽略**）
)

// oplStatus 組出狀態埠要回的值。
func (m *Machine) oplStatus() uint8 {
	if !m.oplPresent {
		return 0x00
	}
	var v uint8
	if m.oplT1Started && !m.oplT1Masked {
		v |= 0x40
	}
	if m.oplT2Started && !m.oplT2Masked {
		v |= 0x20
	}
	if v != 0 {
		v |= 0x80 // bit7 是兩者的 OR
	}
	return v
}

// oplWrite 記一次 OPL 暫存器寫入，並更新暫存器檔與計時器狀態。
//
// bank 0 ＝ 0x388/0x389（OPL2 相容），1 ＝ 0x38A/0x38B（OPL3 第二組）。
func (m *Machine) oplWrite(bank int, v uint8) {
	reg := m.oplReg[bank]
	m.oplRegs[bank][reg] = v
	// 計時器控制只在第一組（OPL3 的第二組沒有 02h/03h/04h）。
	if bank == 0 && reg == 0x04 {
		if v&oplIRQReset != 0 {
			// **bit7 一設，其他位元就不看了**（資料表如此）。
			// 寫成 `switch` 之外的 `if` 會讓 `04h←80h` 順便去動遮罩。
			m.oplT1Started, m.oplT2Started = false, false
		} else {
			m.oplT1Masked = v&oplT1Mask != 0
			m.oplT2Masked = v&oplT2Mask != 0
			if v&oplT1Start != 0 {
				m.oplT1Started = true
			}
			if v&oplT2Start != 0 {
				m.oplT2Started = true
			}
		}
	}
	m.OPL = append(m.OPL, OPLWrite{Reg: reg, Val: v, Step: m.Steps, Bank: uint8(bank)})
}

// OPLRegs 回某一組暫存器的**目前狀態**（256 bytes）。
//
// **寫入序列與最終狀態是兩件事**：`OPL` 那一串看得出「怎麼做的」，
// 這一份看得出「現在是什麼」。對拍解碼器用前者，
// 對拍某一刻的音色用後者——後者不受「多寫了一次同樣的值」影響。
func (m *Machine) OPLRegs(bank int) [256]uint8 {
	if bank < 0 || bank > 1 {
		return [256]uint8{}
	}
	return m.oplRegs[bank]
}

// ClearOPL 清掉暫存器寫入序列，**但不動暫存器檔與計時器狀態**。
//
// 用來框出「只有這一段」的寫入：走到某個畫面之後清一次，
// 接下來收到的就只有那一段的。清掉狀態的話下一段會從一台
// 剛開機的晶片開始，那與原版的實際情況不同。
func (m *Machine) ClearOPL() { m.OPL = m.OPL[:0] }

func (m *Machine) In8(port uint16) uint8 {
	m.PortsIn[port]++
	m.portTicks++
	switch {
	case port == 0x3DA:
		// bit3 ＝ 垂直回掃、bit0 ＝ 顯示中。**兩個都要會變**，
		// 這樣不管程式等的是哪一種邊緣都轉得出來。
		if (m.portTicks>>4)&1 != 0 {
			return 0x09
		}
		return 0x00
	case port >= 0x40 && port <= 0x42:
		return uint8(-int(m.portTicks)) // PIT 是遞減計數器
	case port == 0x388:
		// OPL2／OPL3 狀態埠。
		//
		// ⚠ **只有 0x388 當狀態埠，0x38A 維持預設的 0xFF。**
		// OPL3 的偵測序列讀的是基底埠，沒有任何已知的序列讀 0x38A；
		// 把它也接成狀態埠會讓「以前讀到 0xFF 的程式」改讀到 0x00，
		// 而這是一個**共用**的機器層——別的專案的偵測邏輯可能靠那個值。
		// 0x38A／0x38B 的**寫入**照樣走 OPL3 第二組（見 Out8）。
		//
		// **預設回 0 ＝ 偵測不到 AdLib，整段音樂路徑會被跳過**——那讓
		// 開機快很多，所以是預設。要對拍音樂就得讓偵測過關（`AdLib(true)`）。
		//
		// 過關要的不是一個定值：原版走的是標準的 AdLib 偵測序列
		// （`rich2/docs/re/011` §4），它**先要求狀態是 0、啟動計時器之後
		// 再要求是 0xC0**。回定值 0xC0 會在第一次檢查就被判定失敗，
		// 回定值 0 則在第二次失敗——兩種定值都過不了。
		//
		// 這裡照 YM3812 的狀態位元組合：
		//
		//	bit7 = 兩個計時器的逾時旗標的 OR（IRQ）
		//	bit6 = 計時器 1 逾時   bit5 = 計時器 2 逾時
		//
		// 「逾時」在這裡的模型是「啟動了而且沒有被遮罩」——這台機器沒有
		// 真實時間，而偵測序列在啟動之後一定會先延遲再讀。
		// **遮罩位元要照做**：偵測序列的第一步就是 `04h←60h`（兩個都遮），
		// 不理它的話那一步就會讀到非零而判定失敗。
		return m.oplStatus()
	case port == 0x61:
		return 0x00
	// sequencer／GC 讀回（spec 009）：有程式會讀回索引或資料確認。
	case port == 0x3C4:
		return m.seqIdx
	case port == 0x3C5:
		return m.seq[m.seqIdx]
	case port == 0x3CE:
		return m.gcIdx
	case port == 0x3CF:
		return m.gc[m.gcIdx]
	}
	return 0xFF
}

func (m *Machine) Out8(p uint16, v uint8) {
	m.Ports[p] = v
	m.PortLog = append(m.PortLog, PortWrite{Port: p, Val: v, Step: m.Steps})

	// OPL2：0x388 選暫存器、0x389 寫值。**兩個埠是一組**，
	// 單看其中一個看不出寫了什麼。
	switch p {
	case 0x388:
		m.oplReg[0] = v
	case 0x38A:
		m.oplReg[1] = v
	case 0x389:
		m.oplWrite(0, v)
	case 0x38B:
		m.oplWrite(1, v)
	}

	// VGA DAC。**沒有它就只有色號沒有顏色**，而色號陣列自己看起來完全正常
	// ——畫面比對會變成「圖形對了但顏色全錯」，卻查不出顏色是誰的責任。
	switch p {
	case 0x3C8: // 設寫入索引
		m.dacIndex, m.dacPhase = v, 0
	case 0x3C9: // 連寫三次 ＝ R、G、B（各 6 位元）
		m.DAC[int(m.dacIndex)*3+int(m.dacPhase)] = v & 0x3F
		m.dacPhase++
		if m.dacPhase == 3 {
			m.dacPhase = 0
			m.dacIndex++ // 索引自動前進，所以整份調色盤可以一次寫完
		}
	}

	// sequencer／GC 索引對（`docs/spec/009` §1）。
	switch p {
	case 0x3C4:
		m.seqIdx = v & 7
	case 0x3C5:
		m.seq[m.seqIdx] = v
	case 0x3CE:
		m.gcIdx = v & 0x0F
	case 0x3CF:
		m.gc[m.gcIdx] = v
	}
}

// Palette 把 DAC 的 6 位元色值轉成 8 位元 RGB。
func (m *Machine) Palette() [256][3]uint8 {
	var out [256][3]uint8
	for i := 0; i < 256; i++ {
		for ch := 0; ch < 3; ch++ {
			v := m.DAC[i*3+ch]
			// 6 → 8 位元用「高位補到低位」，不是乘 255/63 四捨五入。
			out[i][ch] = v<<2 | v>>4
		}
	}
	return out
}

// ---- 記憶體存取的便利函式 ------------------------------------------------

func (m *Machine) Read16(a uint32) uint16 {
	return uint16(m.Mem[a&0xFFFFF]) | uint16(m.Mem[(a+1)&0xFFFFF])<<8
}

func (m *Machine) Write16(a uint32, v uint16) {
	m.Mem[a&0xFFFFF] = uint8(v)
	m.Mem[(a+1)&0xFFFFF] = uint8(v >> 8)
}

func (m *Machine) WriteBytes(a uint32, b []byte) {
	for i, v := range b {
		m.Mem[(a+uint32(i))&0xFFFFF] = v
	}
}

// Indexed 回傳 mode 13h 畫面的 320×200 色號陣列。
//
// **回的是色號不是 RGB**——對拍在色號空間做（`docs/spec/005` §3）。
// 回傳的是複本，呼叫端改它不會動到機器。
func (m *Machine) Indexed() []uint8 {
	out := make([]uint8, VideoWidth*VideoHigh)
	copy(out, m.Mem[VideoSeg*16:VideoSeg*16+len(out)])
	return out
}

// Step 執行一道指令，必要時先送 IRQ0。
func (m *Machine) Step() error {
	m.tick()
	m.Steps++
	return m.CPU.Step()
}

// tick 是計時器中斷（IRQ0 ＝ `int 08h`）。
//
// ⚠ **沒有它，任何等計時器的迴圈都轉不出來。** `RUN_full.EXE` 的防拷畫面
// 就停在這一個形狀上（執行期 `3014:167F` ＝ 線性 `406BF`）：
//
//	cmp cx, cs:[1727h]
//	jg  −5              ; 等計數器追上 CX
//
// `cs:1727h` 由程式自己的 ISR 遞增。中斷不送 ＝ 值永遠不變 ＝ 死迴圈，
// **而且畫面看起來是對的**——文字動畫停在第一個字，像是「還沒畫完」。
//
// **這是指令數模型，不是時間模型。** 拿執行過的指令數當時鐘，
// 好處是對拍完全決定性（同樣的輸入永遠得到同樣的畫面）；
// 代價是動畫速度與真機不同。週期精確的時序在 M2（`docs/spec/004` §5）。
func (m *Machine) tick() {
	if m.IRQ0Every > 0 && m.Steps >= m.nextIRQ0 {
		m.nextIRQ0 = m.Steps + m.IRQ0Every
		// **先掛起來，不要直接送。** 初始化期間大量 `CLI`，
		// 當場丟掉的話那一段的 tick 全部消失。
		m.irq0Pending = true
	}
	if !m.irq0Pending || !m.CPU.Flag(cpu.IF) {
		return
	}
	m.irq0Pending = false
	m.Ticks++
	m.bumpBDATicks()

	// 程式自己裝了 `int 08h` 就跑它的。
	if m.Read16(0x08*4+2) != StubSeg {
		m.CPU.Interrupt(0x08)
		return
	}
	// 否則做 BIOS 預設的事：更新計數之後轉呼 `int 1Ch`（那是給應用程式的
	// 掛鉤點，只裝 `1Ch` 不裝 `08h` 的程式很多）。
	if m.Read16(0x1C*4+2) != StubSeg {
		m.CPU.Interrupt(0x1C)
	}
}

// bumpBDATicks 推進 `0040:006C` 的 32 位元計數，並在跨日時設 `0040:0070`。
// 有些程式直接讀它算時間，不裝任何 ISR。
func (m *Machine) bumpBDATicks() {
	const at = 0x0040*16 + 0x6C
	v := uint32(m.Read16(at)) | uint32(m.Read16(at+2))<<16
	v++
	if v >= 0x001800B0 { // 一天的 tick 數
		v = 0
		m.Write8(0x0040*16+0x70, m.Read8(0x0040*16+0x70)+1)
	}
	m.Write16(at, uint16(v))
	m.Write16(at+2, uint16(v>>16))
}

// ---- 中斷向量表 ----------------------------------------------------------

// initVectors 給每個向量一段自己的 stub：`int n` ＋ `iret`。
//
// 三件事同時要滿足（`docs/spec/003` §2、`docs/spec/011` §2）：
//
//  1. **每一個向量都要是合法位址。** 取到 `0000:0000` 的程式跳過去會執行
//     到垃圾（`rich2/docs/re/005` §3.2）。
//  2. **第一個位元組不能是 `CFh`。** 滑鼠偵測直接讀 `0000:00CC` 拿到
//     段:位移，再讀**那個位址的第一個位元組**，是 `CFh` 就判定
//     「沒有驅動」（`rich2/docs/re/182` §2）。所有向量都指到同一個 IRET
//     的話，遊戲從此不發 `int 33h`——而且沒有任何錯誤訊息。
//     每個 stub 的第一個位元組都是 `CDh`，這條自然成立。
//  3. **[HARD] stub 裡要真的有 `int n`，不能只是 `iret`。** 服務層掛在
//     CPU 執行 `INT` 指令的 hook 上；程式若改用「`AH=35h` 取向量 → 直接
//     跳過去」（C 的 `int86x` 就是這樣做的），那條路完全繞過 hook，
//     落在 `iret` 上就是**安靜地什麼都不做**：暫存器原樣回去，沒有錯誤、
//     沒有 unimplemented 記錄，只有畫面或資料悄悄不對
//     （`~/cht/logh3/docs/re/06`：整份調色盤因此永遠是黑的）。
func (m *Machine) initVectors() {
	for v := 0; v < 256; v++ {
		off := uint32(StubSeg)*16 + uint32(v)*stubStride
		m.Mem[off] = 0xCD // int n
		m.Mem[off+1] = uint8(v)
		m.Mem[off+2] = 0xCF // iret
		m.Write16(uint32(v)*4, StubOff(uint8(v)))
		m.Write16(uint32(v)*4+2, StubSeg)
	}
}
