// Package dosgolem 是這個執行器的對外介面。
//
// 實作分成三層放在 internal/ 底下（cpu／machine／dos），別的模組 import 不到——
// 那是刻意的：分層與型別可以在不驚動使用者的情況下改。**跨模組要用的東西一律
// 從這裡走。**
//
// 這一層只有型別別名與轉呼叫，沒有邏輯。加東西進來之前先問一次：
// 它是「任何 DOS 程式都用得到」，還是「某一個程式才需要」？
// 後者屬於使用端的 repo，不屬於這裡。
//
//	m := dosgolem.New()
//	m.LoadCOM(image)
//	d := dosgolem.NewDOS(m, "/orig")
//	d.Install()
//	for i := 0; i < 20_000_000; i++ { m.Step() }
//	fmt.Println(m.TextScreen(0))
package dosgolem

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 三個核心型別。別名而不是包裝：使用端拿到的就是本體，
// 包一層只會讓「哪些方法有、哪些沒有」變成要維護的東西。
type (
	// Machine 是一台 8086 機器：1 MB 記憶體、BDA、中斷向量表、I/O 埠。
	Machine = machine.Machine

	// DOS 是 DOS 與 BIOS 服務層。要在載入映像之後 Install。
	DOS = dos.DOS

	// CPU 是處理器狀態。R 是通用暫存器、Seg 是段暫存器，索引用下面的常數。
	CPU = cpu.CPU

	// Key 是 int 16h 佇列裡的一個按鍵。
	Key = dos.Key

	// Snapshot 是一份完整的機器狀態，用 Machine.Snapshot 拍、Restore 還原。
	// 差分比對要「從同一個狀態展開多個變體」時就靠它。
	Snapshot = machine.Snapshot

	// WriteHook／WordHook 是監看點的回呼，Stop 是 RunUntil 停下來的原因。
	WriteHook = machine.WriteHook
	WordHook  = machine.WordHook
	Stop      = machine.Stop
)

// RunUntil 停下來的原因。
const (
	StopBudget     = machine.StopBudget
	StopPredicate  = machine.StopPredicate
	StopBreakpoint = machine.StopBreakpoint
)

// New 造一台機器：記憶體清空、BDA 建好、向量表填好。還沒有程式。
func New() *Machine { return machine.New() }

// NewDOS 造一個服務層。root 是程式看得到的目錄。
func NewDOS(m *Machine, root string) *DOS { return dos.New(m, root) }

// 通用暫存器索引，照 8086 的編碼順序。
const (
	AX = cpu.AX
	CX = cpu.CX
	DX = cpu.DX
	BX = cpu.BX
	SP = cpu.SP
	BP = cpu.BP
	SI = cpu.SI
	DI = cpu.DI
)

// 段暫存器索引，照 8086 的編碼順序。
const (
	ES = cpu.ES
	CS = cpu.CS
	SS = cpu.SS
	DS = cpu.DS
)

// 旗標位元。
const (
	CF = cpu.CF
	PF = cpu.PF
	AF = cpu.AF
	ZF = cpu.ZF
	SF = cpu.SF
	TF = cpu.TF
	IF = cpu.IF
	DF = cpu.DF
	OF = cpu.OF
)

// 記憶體佈局。
const (
	// MemSize 是整個真實模式位址空間。
	MemSize = machine.MemSize

	// PSPSeg 是程式的 PSP，LoadSeg 是 MZ 映像本體。
	PSPSeg  = machine.PSPSeg
	LoadSeg = machine.LoadSeg

	// MemTop 是傳統記憶體上緣。
	MemTop = machine.MemTop

	// VideoSeg 是 mode 13h 的畫面，TextSeg 是彩色文字模式的畫面。
	VideoSeg = machine.VideoSeg
	TextSeg  = machine.TextSeg
)

// 輸入的節奏。
const (
	// DefaultIRQ0Every 是計時器中斷的間隔，單位是指令數。
	DefaultIRQ0Every = machine.DefaultIRQ0Every

	// DefaultKeyIRQEvery 是兩次鍵盤中斷的最小間隔，單位是指令數。
	DefaultKeyIRQEvery = machine.DefaultKeyIRQEvery
)
