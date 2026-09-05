// Package machine 是一台跑得動 `RUN_full.EXE` 的 8086 機器：
// 1 MB 平坦記憶體、BIOS 資料區、中斷向量表、I/O 埠。
//
// 規格在 `docs/spec/003-machine-and-loader.md`（READY）。
//
// **它不認識 DOS 服務**——那是 `internal/dos` 的事。這一層只負責
// 「記憶體長什麼樣」與「程式怎麼被載進去」。
package machine

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 記憶體佈局。取值沿用 `rich2/tools/dosemu.py` 那一版——**它是實際把
// `RUN.EXE` 跑到資產全部載完的那一份**，不是重新挑的。
const (
	// MemSize 是整個真實模式位址空間。
	MemSize = 1 << 20

	// MCBSeg 是假的記憶體控制區塊鏈起點，LOLSeg 是假的 DOS「list of lists」。
	MCBSeg = 0x0060
	LOLSeg = 0x0070

	// StubSeg 放中斷向量的預設目標。**不是只有一個 IRET**——見 §2。
	StubSeg = 0x0080

	// EnvSeg 是環境區塊；`PSP+2Ch` 指向它。
	EnvSeg = 0x0090

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

// mouseStubOff 是 `int 33h` 的向量指向的位移。
//
// `[HARD]` **那裡放的是 `90 CF`（`nop; iret`），不是 `CF`。**
// 滑鼠偵測直接讀 `0000:00CC` 拿到段:位移，再讀**那個位址的第一個位元組**，
// 是 `CFh` 就判定「沒有驅動」（`rich2/docs/re/182` §2）。
// 所有向量都指到同一個 IRET 的話，遊戲從此不發 `int 33h`——
// **而且沒有任何錯誤訊息**。
const mouseStubOff = 0x10

// DefaultKeyIRQEvery 是兩次鍵盤中斷的最小間隔，單位是指令數。
// 取這個值只要求「處理常式來得及跑完」，不模擬真實打字速度。
const DefaultKeyIRQEvery = 20_000

// DefaultIRQ0Every 是計時器中斷的間隔，單位是**指令數**。
//
// 真機是 18.2 Hz（55 ms）。以 DOSBox 預設的 3,000 cycles/ms 換算大約
// 165,000 道指令，這裡取整。用指令數而不是時間，是為了讓對拍決定性——
// 同一組輸入永遠得到同一個畫面。
const DefaultIRQ0Every = 165_000

// PortWrite 是一次埠寫入。音訊 parity 只需要這份序列，不必合成聲音
// （`docs/spec/004` §6）。
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

	// KeyQueue 是待送的鍵盤掃描碼（IBM set 1，通碼與斷碼都要自己排）。
	// 有東西而且程式裝了自己的 `int 09h` 時，每 KeyIRQEvery 條指令送一個。
	//
	// **這條路與 `dos.Keys`（`int 16h`）是兩件事。** 走 BIOS 服務的程式用
	// 前者；自己接 IRQ1 的程式（DOS 版 UCSD p-System 就是）只認這裡。
	KeyQueue []uint8

	// KeyIRQEvery 是兩次鍵盤中斷至少隔幾條指令。0 用 DefaultKeyIRQEvery。
	// 隔太近的話前一個掃描碼還沒被處理常式讀走就被蓋掉。
	KeyIRQEvery uint64

	// KeyIRQs 是送出去的鍵盤中斷次數。
	KeyIRQs uint64

	// kbData 是埠 60h 目前的值。
	kbData    uint8
	nextKeyIRQ uint64

	// portTicks 是所有 `in` 的累計，當作輪詢埠的時鐘。
	portTicks uint64

	nextIRQ0    uint64
	irq0Pending bool

	// DAC 是 VGA 調色盤，256×3 個 6 位元色值（`docs/formats/001` 的格式）。
	DAC [256 * 3]uint8

	dacIndex uint8
	dacPhase uint8

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
	}
	m.CPU = cpu.New(m)
	// **這台機器是拿來跑 1993 年的 DOS 軟體的，不是拿來過語料的。**
	// `RUN_full.EXE` 的主程式區有 3,345 個 80186 的 `PUSH imm`；用 8086 的
	// 別名解讀會錯位一個 byte，然後安靜地飛掉（`docs/spec/002` §1.1）。
	// 語料驗收走 `cpu.New()`，那邊維持 8086 預設。
	m.CPU.Model = cpu.Model80186
	m.initBDA()
	m.initVectors()
	return m
}

// ---- cpu.Bus ------------------------------------------------------------

func (m *Machine) Read8(a uint32) uint8 { return m.Mem[a&0xFFFFF] }

func (m *Machine) Write8(a uint32, v uint8) { m.Mem[a&0xFFFFF] = v }

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
		// OPL 狀態埠。回 0 ＝ 偵測不到 AdLib，音樂路徑會被跳過。
		// 要觀察音樂路徑時改回 0xC0（`rich2/docs/re/011` §4）。
		return 0x00
	case port == 0x60:
		// 鍵盤資料埠。`int 09h` 的處理常式從這裡讀掃描碼。
		return m.kbData
	case port == 0x61:
		return 0x00
	}
	return 0xFF
}

func (m *Machine) Out8(p uint16, v uint8) {
	m.Ports[p] = v
	m.PortLog = append(m.PortLog, PortWrite{Port: p, Val: v, Step: m.Steps})

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
	m.keyTick()
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
// keyTick 在條件成立時送一次鍵盤中斷（IRQ1 → `int 09h`）。
//
// 三個條件缺一不可：佇列有東西、`IF` 開著、程式**自己裝了** `int 09h`。
// 最後一條是關鍵——向量還指著我們的 stub 就表示沒有人要處理掃描碼，
// 這時送中斷只會讓掃描碼消失。
func (m *Machine) keyTick() {
	if len(m.KeyQueue) == 0 || !m.CPU.Flag(cpu.IF) {
		return
	}
	if m.Read16(0x09*4+2) == StubSeg {
		return
	}
	every := m.KeyIRQEvery
	if every == 0 {
		every = DefaultKeyIRQEvery
	}
	if m.Steps < m.nextKeyIRQ {
		return
	}
	m.nextKeyIRQ = m.Steps + every
	m.kbData = m.KeyQueue[0]
	m.KeyQueue = m.KeyQueue[1:]
	m.KeyIRQs++
	m.CPU.Interrupt(0x09)
}

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

// initVectors 把 256 個向量全部指到 StubSeg 的 stub。
//
// 兩件事同時要滿足（`docs/spec/003` §2）：
//
//  1. **每一個向量都要是合法位址。** 取到 `0000:0000` 的程式跳過去會執行
//     到垃圾（`rich2/docs/re/005` §3.2）。
//  2. **`int 33h` 的目標第一個位元組不能是 `CFh`。** 見 mouseStubOff。
func (m *Machine) initVectors() {
	m.Mem[StubSeg*16] = 0xCF                   // iret
	m.Mem[StubSeg*16+mouseStubOff] = 0x90      // nop
	m.Mem[StubSeg*16+mouseStubOff+1] = 0xCF    // iret
	for v := 0; v < 256; v++ {
		m.Write16(uint32(v)*4, 0)
		m.Write16(uint32(v)*4+2, StubSeg)
	}
	m.Write16(0x33*4, mouseStubOff)
	m.Write16(0x33*4+2, StubSeg)
}

// scanSet1 是 IBM set 1 的通碼，只列本專案用得到的鍵。
// 斷碼是通碼加 0x80，由 TypeScan 自己補。
var scanSet1 = map[byte]uint8{
	'1': 0x02, '2': 0x03, '3': 0x04, '4': 0x05, '5': 0x06,
	'6': 0x07, '7': 0x08, '8': 0x09, '9': 0x0A, '0': 0x0B,
	'-': 0x0C, '=': 0x0D, '\b': 0x0E, '\t': 0x0F,
	'q': 0x10, 'w': 0x11, 'e': 0x12, 'r': 0x13, 't': 0x14,
	'y': 0x15, 'u': 0x16, 'i': 0x17, 'o': 0x18, 'p': 0x19,
	'[': 0x1A, ']': 0x1B, '\r': 0x1C, '\n': 0x1C,
	'a': 0x1E, 's': 0x1F, 'd': 0x20, 'f': 0x21, 'g': 0x22,
	'h': 0x23, 'j': 0x24, 'k': 0x25, 'l': 0x26, ';': 0x27,
	'\'': 0x28, '`': 0x29, '\\': 0x2B,
	'z': 0x2C, 'x': 0x2D, 'c': 0x2E, 'v': 0x2F, 'b': 0x30,
	'n': 0x31, 'm': 0x32, ',': 0x33, '.': 0x34, '/': 0x35,
	' ': 0x39, 0x1B: 0x01, // ESC
}

// TypeScan 把一串字元排進鍵盤佇列，通碼與斷碼成對。
//
// 大寫字母會補上 Shift 的通碼與斷碼包住它——處理常式要靠 Shift 的狀態
// 才分得出 `f` 與 `F`。認不得的字元回錯誤，**不安靜跳過**：
// 少送一個鍵會讓後面的輸入整串對不上，而且從結果看不出來。
func (m *Machine) TypeScan(s string) error {
	const shift, brk = 0x2A, 0x80
	for i := 0; i < len(s); i++ {
		ch := s[i]
		upper := ch >= 'A' && ch <= 'Z'
		if upper {
			ch += 'a' - 'A'
		}
		code, ok := scanSet1[ch]
		if !ok {
			return fmt.Errorf("machine: 沒有 %q 的掃描碼", s[i])
		}
		if upper {
			m.KeyQueue = append(m.KeyQueue, shift)
		}
		m.KeyQueue = append(m.KeyQueue, code, code|brk)
		if upper {
			m.KeyQueue = append(m.KeyQueue, shift|brk)
		}
	}
	return nil
}
