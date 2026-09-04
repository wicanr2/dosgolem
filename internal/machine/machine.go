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
		Mem:   make([]uint8, MemSize),
		Ports: map[uint16]uint8{},
	}
	m.CPU = cpu.New(m)
	m.initBDA()
	m.initVectors()
	return m
}

// ---- cpu.Bus ------------------------------------------------------------

func (m *Machine) Read8(a uint32) uint8 { return m.Mem[a&0xFFFFF] }

func (m *Machine) Write8(a uint32, v uint8) { m.Mem[a&0xFFFFF] = v }

// In8 回 0xFF。**空的匯流排上讀到的就是 0xFF，不是 0**——
// 有些偵測用「讀回來不是 FF」判定裝置存在，回 0 會讓它們誤判。
func (m *Machine) In8(uint16) uint8 { return 0xFF }

func (m *Machine) Out8(p uint16, v uint8) {
	m.Ports[p] = v
	m.PortLog = append(m.PortLog, PortWrite{Port: p, Val: v, Step: m.Steps})
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

// Step 執行一道指令。
func (m *Machine) Step() error {
	m.Steps++
	return m.CPU.Step()
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
