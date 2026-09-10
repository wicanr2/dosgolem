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
	"strings"

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

	// EMSSeg 是 EMM 的 driver header（`docs/spec/014` §3.1）：
	// 位移 0 是 trampoline、位移 0Ah 是簽章 `EMMXXXX0`。
	// **偵測 EMS 的標準做法就是讀 int 67h 向量指的段、比對那 8 個位元組。**
	//
	// ⚠ 它在 MCBSeg 之前（0x0500–0x05FF），不是原本的 0x00A0——
	// 那一格落在 stub 區裡面。
	EMSSeg = 0x0050

	// StubSeg 放中斷向量的預設目標。
	//
	// 位移 0x000–0x3FF：256 個向量各一段 stub（`CD n` ＋ `CF`）。
	// 位移 0x400–0x4FF：特殊 stub（BIOS 計時器、服務型中斷的 trampoline）。
	// 合起來佔 0x800–0xCFF——見 initVectors。
	StubSeg = 0x0080

	// EnvSeg 是環境區塊；`PSP+2Ch` 指向它。內容只有幾十 bytes，
	// 但要留在 stub 區之後、PSP 之前。
	EnvSeg = 0x00D0

	// EMSFrameSeg 是 EMS 的 64 KB page frame。D0000–DFFFF 在 1 MB
	// 空間裡沒有別的用途，程式對它的讀寫就是普通記憶體存取。
	EMSFrameSeg = 0xD000

	// PSPSeg 是程式的 PSP，LoadSeg 是映像本體（PSP 佔 16 段）。
	PSPSeg  = 0x0100
	LoadSeg = PSPSeg + 0x10

	// MemTop 是傳統記憶體上緣（640 KB）。
	MemTop = 0x9FFF

	// VideoSeg 是 mode 13h 的畫面。
	VideoSeg = 0xA000

	// TextSeg 是彩色文字模式的畫面緩衝區。走文字模式的程式把字元與屬性
	// 交錯寫在這裡，不經過 VideoSeg。
	TextSeg    = 0xB800
	VideoWidth = 320
	VideoHigh  = 200
)

// stubStride 是每個向量的 stub 佔幾個 byte（`CD n` ＋ `CF` ＋ 對齊）。
const stubStride = 4

// StubOff 回向量 n 的 stub 在 StubSeg 內的位移。
func StubOff(n uint8) uint16 { return uint16(n) * stubStride }

// 特殊 stub 的位移。**一定要在 per-vector stub 陣列（0x000–0x3FF）之後**
// ——`cbRetInt` 是 0xFF，它的 stub 就落在 0x3FC。
const (
	specialStubBase = 0x400

	// 服務型中斷的 trampoline。**號碼要與被服務的中斷不同**：
	// 程式裝了自己的 `int 21h` 再 chain 回「舊向量」時，舊向量指的
	// 就是這段；如果它裡面寫的是 `CD 21`，服務層的向量檢查會看到
	// 向量已經被換掉而放行，CPU 於是又跳回程式自己的處理常式——
	// 一個看不出來的無窮迴圈。改成 `CD F2` 就沒有這個問題，
	// F2 的向量還指著 StubSeg。
	DosTrapOff   = specialStubBase + 0x00 // int 21h → int F2h
	VideoTrapOff = specialStubBase + 0x04 // int 10h → int F3h
	ATrapOff     = specialStubBase + 0x08 // int 15h → int F4h
	// XMSTrapOff 是 XMS driver entry（`docs/spec/011`）→ int F5h。
	XMSTrapOff = specialStubBase + 0x0C

	// biosTimerOff 是 `int 08h` 的 BIOS 預設處理：推進 `0040:006C`
	// 再轉呼 `int 1Ch`（見 initVectors）。
	//
	// ⚠ **它有 24 個 byte，不是 4 個。** 排在 trampoline 之後並留一整段，
	// 否則它會把後面的 trampoline 蓋掉——而症狀是「int 10h 的向量指到
	// `pop dx / pop ax / pop ds`」，看起來像向量算錯，不像佈局重疊。
	biosTimerOff = specialStubBase + 0x20
)

// DefaultIRQ0Every 是**分頻 65536 時**計時器中斷的間隔，單位是指令數。
//
// ⚠ **這是退路，不是預設走的路。** 現在的時鐘走 CPU 週期
// （`DefaultCPUHz`），一道指令算一格的模型會把繪圖迴圈算得太貴——
// 老遊戲的繪圖是「切平面（`out`）→ 讀改寫記憶體」的重複，那幾種指令
// 在真機上貴很多。設 `IRQ0Every` 非零就切回這個舊模型。
const DefaultIRQ0Every = 165_000

// PITHz 是 8253／8254 的輸入頻率：14.31818 MHz ÷ 12。
const PITHz = 1_193_182

// DefaultCPUHz 是模擬的 CPU 時脈。**這是「我們在假裝哪一台機器」**，
// 不是實測值。
//
// 選 33 MHz（386DX-33）的理由：源平合戰 1994 年的 DOS/V 目標機大約是
// 386 後期到 486 早期。校準的判準不是主觀的——`OPEN.EXE` 的扇面每一格
// 在程式碼裡等 50 個計時器 tick，模擬器上量到的間隔要接近它；
// 差額就是「繪圖被算了多少時間」。量測與校準過程見
// `docs/spec/004` §5.2。
const DefaultCPUHz = 33_000_000

// DefaultVGAFrameEvery 是每幾道指令一次**垂直回掃**（＝螢幕刷新一次）。
//
// 幀是顯示硬體的事實，不是任何一支程式的知識——DOSBox 也是這樣做的：
// `VGA_VerticalTimer` 由時序驅動，每次回掃跑一輪
// `RENDER_StartUpdate`…`RENDER_EndUpdate`，而 fps 從 CRTC 暫存器算
// （`src/hardware/vga_draw.cpp`：`fps = oscclock/(vtotal*htotal)`，
// mode 13h 的標準值 `vga_fps = 70`）。
//
// dosgolem 的時間由**指令數**驅動，所以這裡換算成指令數。它與
// `DefaultIRQ0Every` 同量級是**巧合**，不是恆等式：mode 13h 的回掃是
// 70 Hz，而計時刻的頻率由程式寫進 8254 的除數決定（`PITHz`）——
// rich2 寫 17,000 ＝ 70.187 Hz，剛好幾乎同頻；`YNSOUND.COM` 寫 4,096
// 就是 291 Hz，兩者差四倍。要一個程式的計時刻換算成指令數，
// 用 `PITStepsPerTick`，不要拿這個常數頂替。
//
// ⚠ **改了它，幀的長度就改了**，同 `IRQ0Every`。對拍要固定。
const DefaultVGAFrameEvery = 165_000

// DefaultKeyIRQEvery 是兩次鍵盤中斷的最小間隔，單位是**指令數**。
// 取計時器間隔的一半：比一次畫面更新久，程式一定來得及把掃描碼收走。
const DefaultKeyIRQEvery = DefaultIRQ0Every / 2

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

// VGAWrite 是一次 planar 寫入與當下的顯示卡狀態。
type VGAWrite struct {
	Step               uint64
	CS, IP             uint16
	Off                uint32
	Row                int
	Val                uint8
	Mode               uint8 // GC[5] 的 write mode（低兩位）
	MapMask            uint8 // SEQ[2]
	BitMask            uint8 // GC[8]
	SetReset, EnableSR uint8 // GC[0]／GC[1]
	Rotate             uint8 // GC[3]
	Latch              [4]uint8
}

type PortWrite struct {
	Port uint16
	Val  uint8
	// Step 是發生在第幾道指令，用來對齊時序。
	Step uint64
}

// SegChange 是一次 CS 的改變：far call／jmp／ret、中斷與 iret。
//
// **「執行流跑到不該去的地方」用 IP 的 trace 很難看出來**——ring buffer
// 裝得下的最後幾萬道通常全在飛掉之後的那一段，而飛掉的那一跳早就滾出去了。
// CS 的改變稀疏得多（一次跑幾百萬道也才幾千筆），整份留著就能回答
// 「第一次以這個段執行是什麼時候、從哪裡來的」。
type SegChange struct {
	Step             uint64
	FromSeg, FromOff uint16
	ToSeg, ToOff     uint16
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

	// 讀取監看（WatchReads）。rWatchLo > rWatchHi 表示關閉。
	rWatchLo, rWatchHi uint32
	onRead             func(addr uint32, v uint8)
	onWrite            func(addr uint32, old, new uint8)

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
	// Coverage 記每一個被執行過的線性位址。nil 表示不記。
	//
	// **這是「哪些 byte 是程式碼」唯一直接的答案。** 打包過的執行檔
	// 靜態反組譯是亂碼，IDA 對 raw dump 也不知道從哪裡開始；
	// 拿執行過的位址當種子，種到的位置一定是程式碼，不是猜的。
	Coverage []bool

	Ports   map[uint16]uint8
	PortLog []PortWrite

	// PortsIn 是每個埠被讀了幾次。**輪詢埠的次數會很大**，那是正常的。
	PortsIn map[uint16]uint64

	// SegLog 是 CS 的改變序列（最後 MaxSegLog 筆，ring），SegFirst 是
	// 每個段**第一次**被執行的那一筆（不設上限——段的種類就那幾個）。
	// 只有 TraceSegs 打開才記。
	//
	// 兩份都要：首見表回答「這個段是誰第一次跳過去的」，序列回答
	// 「飛掉之前那幾跳長什麼樣」。長跑時只留 ring 會把首見洗掉，
	// 只留首見又看不到現場。
	TraceSegs bool
	SegLog    []SegChange
	SegFirst  map[uint16]SegChange

	// WatchDSOn 打開時，DS **變成** WatchDS 的每一次都記進 DSLoads
	// （最多 MaxSegLog 筆）。段暫存器的錯值沒有記憶體寫入可以監看，
	// 只能盯它被載入的那一刻。**要能監看 DS＝0**，所以開關是獨立的
	// bool，不是「值不為 0 就啟用」。
	WatchDSOn bool
	WatchDS   uint16
	DSLoads   []SegChange

	// CycleClock 打開之後計時器走 **CPU 週期**（CPUHz），不走指令數。
	//
	// ⚠ **預設關著。** 這是共用的模擬器，別的專案的對拍收據都建立在
	// 指令數時鐘上；換時鐘會讓每一段動畫的相位都移位。要用的人自己開
	// （`probe -cpuhz`）。兩種模型的量測比較見 `docs/spec/004` §5.2。
	//
	// **`IRQ0Every == 0` 的意思沒有變**：那是「不送計時器中斷」，
	// 不是「改走週期」。把兩件事綁在同一個欄位上會讓
	// `m.IRQ0Every = 0`（測試裡關計時器的慣用寫法）悄悄變成開週期時鐘。
	CycleClock bool

	// CPUHz 是模擬的 CPU 時脈，CycleClock 打開時走它（見 DefaultCPUHz）。
	CPUHz uint64

	// cycPerIRQ0 是兩次 IRQ0 之間幾個週期：CPUHz × 分頻 / PITHz。
	cycPerIRQ0  uint64
	nextIRQ0Cyc uint64

	// IRQ0Base 是**舊模型**（指令數）在分頻 65536 時的間隔。
	// 程式改 PIT 的分頻時 IRQ0Every 由它按比例算出來（`pitWrite`）。
	IRQ0Base uint64

	// PITDiv 是 PIT 通道 0 現在的分頻值（1–65536）。唯讀，給報告用。
	PITDiv uint32

	// pitAccess／pitPhase／pitLo 是通道 0 分頻寫入的狀態機：
	// 控制位元組（埠 43h）先說怎麼寫，再往埠 40h 寫一或兩個位元組。
	pitAccess uint8
	pitPhase  uint8
	pitLo     uint8

	// IRQ0Every 是每幾道指令送一次計時器中斷（**舊模型**）。
	// 0 ＝ 走週期模型；負意義沒有。
	//
	// 預設 DefaultIRQ0Every。**這個值影響動畫跑多快，不影響最終停下來的
	// 畫面**——防拷畫面是靜態的，動畫播完就穩定。
	IRQ0Every uint64

	// Ticks 是送出去的計時器中斷次數。
	Ticks uint64

	// VGAFrameEvery 是每幾道指令一次垂直回掃。0 ＝ 不產生幀。
	//
	// 預設 DefaultVGAFrameEvery。
	VGAFrameEvery uint64

	// Frames 是垂直回掃次數，也就是**螢幕刷新了幾次**。
	//
	// 這是通用的幀：任何 DOS 程式都有它，不必知道那支程式在哪裡畫完一幀。
	Frames uint64

	// onFrame 在每一次垂直回掃時呼叫（`docs/spec/187`）。
	onFrame   func()
	nextFrame uint64

	// onTick 在每一次送出計時器中斷時呼叫（`docs/spec/187`）。
	//
	// **掛在送中斷那一點，不是每道指令輪詢**——幀時鐘自己不該變成熱點。
	onTick func()

	// periodic 是「每 n 道指令遠呼叫一次」。真機上這種東西是別人的 TSR
	// 掛在 PIT 上：《臥龍傳》的遊戲時鐘就是 `YNSOUND.COM` 用 291.3 Hz
	// 的回呼推的（`docs/spec/008` §6）。**沒有它遊戲時鐘不會走**，
	// 而畫面完全正常——只是日期永遠停在第一天。
	periodic struct {
		seg, off    uint16
		every, next uint64
		on          bool
		Calls       uint64
	}

	// 從外面插進去的遠呼叫佇列（`internal/machine/callback.go`）。
	cbQueue  []QueuedCall
	cbSaved  callbackFrame
	cbActive bool
	cbMade   uint64

	// a20 是位址線 20 的閘門，hma 是 1 MB 之上那 64 KB−16 的內容。
	// 見 A20Enabled 的說明：預設關著（8086 的環繞行為）。
	a20 bool
	hma [HMASize]uint8

	// 觀測用（probe.go）。沒設監看點時這幾個都是空的，
	// 熱路徑只多一次長度檢查。
	writeWatches []writeWatch
	wordWatches  []wordWatch
	breaks       map[int]uint32
	nextProbe    int
	insnCS       uint16
	insnIP       uint16

	// Speaker 是 PC 喇叭資料線的變化序列（`docs/spec/016`）。
	// 智冠《三國演義》的語音就是這條線上的一位元取樣。
	Speaker []SpeakerSample

	// IRQ0Clamped 是「算出來的 IRQ0 間隔太小、被夾到下限」發生幾次。
	// **非零表示波形的時間軸不可信**，不能安靜地夾。
	IRQ0Clamped int

	// picMask 是 8259 的 OCW1（埠 0x21）：bit0 遮蔽 IRQ0、bit1 遮蔽 IRQ1。
	//
	// 原版改分頻值前後各遮蔽／放行一次。不接的話，遮蔽期間送進去的中斷
	// 會在向量與分頻值都還沒設好的時候跑進處理常式——那段碼用到還沒
	// 初始化的變數，結果不固定。
	picMask uint8

	// portTicks 是所有 `in` 的累計，當作輪詢埠的時鐘。
	portTicks uint64

	// pit 記通道 0 的除數：程式自己寫的頻率（`PITHz`）。
	pit pit

	nextIRQ0    uint64
	irq0Pending bool

	// VGATraceRow0／Row1 圈出要記錄的畫面列，VGATrace 是前 40 筆寫入
	// 連同當下的 VGA 狀態。**「寫進去了」與「寫進去有用」是兩件事**，
	// 只看寫入位址分不出來。
	VGATraceRow0, VGATraceRow1 int
	// VGATraceCol0／Col1 是位元組欄的範圍（一欄八個像素）。Col1 ＝ 0 表示
	// 整列都要。**只圈列常常不夠**：一列上面畫的東西可能來自好幾個呼叫，
	// 而環狀緩衝只留最後四十筆。
	VGATraceCol0, VGATraceCol1 int
	VGATrace                   []VGAWrite

	// RowWritesFrom 是開始統計 planar 寫入的指令數（0 ＝ 不統計），
	// VideoRowWrites 是每一列被寫過幾個位元組。
	RowWritesFrom  uint64
	VideoRowWrites [1024]uint64

	// VGA 是平面模式的狀態（`docs/spec/009`／`013`）：四個平面、
	// 序列器、繪圖控制器、屬性控制器與 latch。planarOn 是
	// 「目前模式是不是平面」的快取，Read8／Write8 每次都要問。
	VGA      *VGA
	planarOn bool
	// DAC 是 VGA 調色盤，256×3 個 6 位元色值（`docs/formats/001` 的格式）。
	DAC [256 * 3]uint8

	dacIndex uint8
	dacPhase uint8

	// WriteModeUse 統計 planar 寫入用過哪些 write mode（診斷用）。
	WriteModeUse [4]uint64

	// VRAMSites 統計「誰在寫視訊記憶體」——key ＝ CS<<16|IP（那一道指令的
	// 起點）。**在沒有原始碼的情況下，這是找繪圖常式最短的一條路**：
	// 畫面上看得到的東西，一定有人把它寫進 plane。
	// nil ＝ 不統計（有額外開銷）。
	VRAMSites map[uint32]uint64

	// VRAMAt 限定只統計寫到這個位移的指令（−1 ＝ 全部）。盯單一像素用：
	// 「這一點是誰畫的」比「誰畫得最多」更能定位。
	VRAMAt int32

	// ModeChanges 記錄每次模式切換（bda.go SetVideoMode）。
	ModeChanges []ModeChange

	// 硬體鍵盤（`keyboard.go`）。kbdData 是埠 0x60 讀得到的值、
	// kbdPortB 是埠 0x61（程式用它做鍵盤 ack）。
	keyQueue []KeyEvent
	kbdData  uint8
	kbdPortB uint8

	// KeyIRQs 是實際送出去的鍵盤中斷數，keyStalls 是「有鍵但沒人裝
	// int 09h」而留著沒送的次數。**送不出去與遊戲不理會是兩件事**，
	// 沒有這兩個數字就分不開。
	KeyIRQs   uint64
	keyStalls uint64
	nextKey   uint64
	// KeyEvery 是兩次鍵盤中斷之間至少隔幾道指令。
	KeyEvery uint64

	// Steps 是已經執行的指令數。**不是週期數**——時序要等 M2
	// （`docs/spec/004` §5）。
	Steps uint64

	// FreeSeg 是映像之後第一個可配置的段。`int 21h AH=4Ah` 的探測要用它
	// （`docs/spec/004` §1.2）。
	FreeSeg uint16

	// ProgramPath 是放進環境區塊的程式全路徑（DOS 形式，如 C:\GAME\X.EXE）。
	// MSC 的啟動碼會讀它當 argv[0]。空的話用一個中性的預設值——
	// **不要放某一支程式的路徑**，那是 per-program 的值。
	ProgramPath string

	// ImageBase／ImageLen 是載進去的映像位置與長度。
	ImageBase uint32
	ImageLen  int
}

// New 造一台機器：記憶體清空、BDA 建好、向量表填好。
//
// **還沒有程式**——要呼叫 LoadEXE。
func New() *Machine {
	m := &Machine{
		Mem:           make([]uint8, MemSize),
		Ports:         map[uint16]uint8{},
		PortsIn:       map[uint16]uint64{},
		IRQ0Every:     DefaultIRQ0Every,
		VGAFrameEvery: DefaultVGAFrameEvery,
		KeyEvery:      DefaultKeyIRQEvery,
		IRQ0Base:      DefaultIRQ0Every,
		CPUHz:         DefaultCPUHz,
		PITDiv:        PITDefaultDivisor,
		pitAccess:     3,
		// **第一次計時器中斷排在一個完整週期之後。** 零值的話
		// `m.Steps >= m.nextIRQ0` 在第 0 步就成立，程式的第一道指令
		// 還沒執行就先被 int 08h 打斷——載入器把 IF 打開之後，
		// 那變成每一支程式都會遇到，而症狀是「跑幾步就跑到別的地方去」。
		nextIRQ0:  DefaultIRQ0Every,
		nextFrame: DefaultVGAFrameEvery,
		// 空區間 ＝ 監看關閉（見 WatchWrites）。零值的 lo=hi=0 會誤中位址 0。
		watchLo: 1, watchHi: 0,
		rWatchLo: 1, rWatchHi: 0,
		VGA: newVGA(),
	}
	m.CPU = cpu.New(m)
	// 取指令走直接索引，不走匯流排介面（`docs/spec/015` §4.2）。
	m.syncCodeFastPath()
	// **這台機器是拿來跑 1990 年代的 DOS 軟體的，不是拿來過語料的。**
	// `RUN_full.EXE` 的主程式區有 3,345 個 80186 的 `PUSH imm`；用 8086 的
	// 別名解讀會錯位一個 byte，然後安靜地飛掉（`docs/spec/002` §1.1）。
	// DOSJP.COM 另外需要 80386 的 0x66 子集（`docs/spec/012`）。
	// 語料驗收走 `cpu.New()`，那邊維持 8086 預設。
	m.CPU.Model = cpu.Model80386
	// Reset 在 cpu.New 裡就跑完了，那時 Model 還是 8086，旗標被套成 8086 的
	// 固定位元 0xF002。80386 的 reset EFLAGS 是 0x00000002，bit 15 必須是 0；
	// 宣告成 386 之後要重新正規化一次，否則之後任何走 SetFlags 的存回
	// （例如 callback.go 的回呼收尾）都會與存檔值不一致。
	m.CPU.SetFlags(m.CPU.Flags)
	m.recalcIRQ0()
	m.initBDA()
	m.initVectors()
	m.installCallbackStub()
	return m
}

// ---- cpu.Bus ------------------------------------------------------------

// A20 是位址線 20 的閘門。
//
// **關著（預設）＝ 8086 的行為**：位址在 1 MB 環繞回 0，`FFFF:0010` 與
// `0000:0000` 是同一個位元組。程式**靠這個環繞偵測 HMA 在不在**
// （寫 0000:0000 再讀 FFFF:0010，值一樣就是沒有 A20）。
// 開了之後 1 MB 之上的 64 KB−16 才定址得到，那就是 HMA。
//
// ⚠ **預設關著**：真機開機後 A20 是關的，HIMEM.SYS 載入時才打開。
// 預設開的話，靠環繞偵測的程式會判定「有 HMA」然後把資料搬進去，
// 而它可能根本沒要求過（XMS `AH=03h`）——那是安靜的行為差異。
func (m *Machine) A20Enabled() bool { return m.a20 }

// SetA20 開關 A20（XMS 的 `AH=03h`–`06h` 走它）。
func (m *Machine) SetA20(on bool) {
	m.a20 = on
	m.syncCodeFastPath()
}

// syncCodeFastPath 依目前的狀態決定取指令走不走快路徑
// （`CPU.Code`，`docs/spec/015` §4.2）。
//
// **快路徑跳過 `Read8`，所以 `Read8` 做的每一件事它都不做。** 判準是
// 「這件事對取指令成不成立」：
//
//   - 讀取監看：成立。取指令也是讀取，快路徑會讓監看看不到執行，
//     所以監看開著就得走慢路徑。
//   - 平面模式的 A0000 視窗：不成立。程式碼不放在視訊記憶體裡，
//     `CS:IP` 進到那裡本來就是飛掉了。
//   - HMA：成立。A20 打開之後 `段:偏移` 到得了 1 MB 之上
//     （`docs/spec/189`），而 HMA 不在 `Mem[]` 裡——快路徑會把那些
//     位址讀成低記憶體的內容，執行的是另一段程式碼。
//
// ⚠ **`Read8` 再多做一件事，就回來過一次這張判準表。** 分開判斷的話
// 遲早會漏一個，而漏掉的症狀都是「跑錯東西但不報錯」。
func (m *Machine) syncCodeFastPath() {
	if m.CPU == nil {
		return
	}
	if m.onRead != nil || m.a20 {
		m.CPU.Code = nil
		return
	}
	m.CPU.Code = m.Mem
}

// HMASize 是 HMA 的大小：64 KB 減 16 bytes（`FFFF:0010`–`FFFF:FFFF`）。
const HMASize = 0x10000 - 16

// hmaAddr 判斷一個線性位址落不落在 HMA，並回它在 HMA 裡的位移。
func (m *Machine) hmaAddr(a uint32) (uint32, bool) {
	if !m.a20 || a < MemSize || a >= MemSize+HMASize {
		return 0, false
	}
	return a - MemSize, true
}

func (m *Machine) Read8(a uint32) uint8 {
	if off, ok := m.hmaAddr(a); ok {
		return m.hma[off]
	}
	a &= 0xFFFFF
	// 平面模式的 A0000 視窗不在線性記憶體裡（`docs/spec/013` §3.1）。
	// ⚠ **讀它有副作用**：四個 latch 會被載入，而 latch 決定之後那次
	// 寫入在 Bit Mask 之外的位元（`docs/spec/011` §4）。
	if m.planarOn && a >= vgaLo && a < vgaHi {
		return m.VGA.Read(a - vgaLo)
	}
	if m.rWatchLo <= a && a <= m.rWatchHi {
		m.onRead(a, m.Mem[a])
	}
	return m.Mem[a]
}

func (m *Machine) Write8(a uint32, v uint8) {
	if off, ok := m.hmaAddr(a); ok {
		m.hma[off] = v
		return
	}
	a &= 0xFFFFF
	if m.planarOn && a >= vgaLo && a < vgaHi {
		// planar 的位元組不在 Mem[] 裡，WatchWrites 看不到它們。
		// VideoRowWrites 補這個洞：**要分辨「程式沒畫」與「畫了但沒生效」**，
		// 除了看結果還得看它到底有沒有寫進去。
		stride := m.planarStride()
		if m.RowWritesFrom > 0 && m.Steps >= m.RowWritesFrom {
			if r := int(a-vgaLo) / stride; r < len(m.VideoRowWrites) {
				m.VideoRowWrites[r]++
			}
		}
		// 用過哪些 write mode、誰在寫視訊記憶體——**畫面上看得到的東西，
		// 一定有人把它寫進 plane**，這是沒有原始碼時找繪圖常式最短的路。
		m.WriteModeUse[m.VGA.WriteMode()]++
		if m.VRAMSites != nil && (m.VRAMAt < 0 || uint32(m.VRAMAt) == a-vgaLo) {
			cs, ip := m.CPU.OpAddr()
			m.VRAMSites[uint32(cs)<<16|uint32(ip)]++
		}
		// **留最後 40 筆，不是前 40 筆**：要知道畫面上最後是誰寫的，
		// 前面那幾筆通常是被蓋掉的那一批。
		if m.VGATraceRow1 > m.VGATraceRow0 && m.Steps >= m.RowWritesFrom {
			off := int(a - vgaLo)
			r, c := off/stride, off%stride
			inCol := m.VGATraceCol1 == 0 || (c >= m.VGATraceCol0 && c < m.VGATraceCol1)
			if r >= m.VGATraceRow0 && r < m.VGATraceRow1 && inCol {
				m.VGATrace = append(m.VGATrace, m.vgaSnap(a-vgaLo, v, r))
				if len(m.VGATrace) > 40 {
					m.VGATrace = m.VGATrace[1:]
				}
			}
		}
		m.VGA.Write(a-vgaLo, v)
		return
	}
	if m.watchLo <= a && a <= m.watchHi && m.Mem[a] != v {
		m.onWrite(a, m.Mem[a], v)
	}
	if len(m.writeWatches) > 0 {
		old := m.Mem[a]
		m.Mem[a] = v
		// ⚠ **值沒變也算一次寫入**（與 WatchWrites 不同）：
		// 「誰在寫這個位址」與「這個值變了沒」是兩個問題，後者用 WatchWord。
		m.noteWrite(a, old, v)
		return
	}
	m.Mem[a] = v
}

// planarStride 是平面模式每一列的位元組數（寬 ÷ 8）。
func (m *Machine) planarStride() int {
	if w, _ := planarSize(m.VideoMode()); w > 0 {
		return w / 8
	}
	return 80
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

// WatchReads 監看一段線性位址的**讀取**。
//
// 寫入監看回答「誰寫這個變數」，讀取監看回答「這一次畫圖是從哪裡取的圖」
// ——素材放在一大塊緩衝區裡的時候，這是唯一直接的答案：
// 讀到的位移除以一格的大小就是格號，不必去猜挑格的規則。
//
// 傳 nil 關掉。**每一次讀都會呼叫**，量很大，回呼要自己節流。
func (m *Machine) WatchReads(lo, hi uint32, fn func(addr uint32, v uint8)) {
	if fn == nil {
		m.rWatchLo, m.rWatchHi, m.onRead = 1, 0, nil
		m.syncCodeFastPath()
		return
	}
	m.rWatchLo, m.rWatchHi, m.onRead = lo, hi, fn
	// **取指令也算讀取。** 監看的用途是回答「誰碰了這個位址」，執行
	// 也是一種碰。代價是監看碼段會被自己的指令提取洗版——監看資料
	// 位址才有意義。
	m.syncCodeFastPath()
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
	oplT1Start  = 0x01 // 啟動計時器 1
	oplT2Start  = 0x02 // 啟動計時器 2
	oplT2Mask   = 0x20 // 遮罩計時器 2 的狀態位元
	oplT1Mask   = 0x40 // 遮罩計時器 1 的狀態位元
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
	if v, ok := m.VGA.In(port); ok {
		return v
	}
	switch {
	case port == 0x60:
		// 鍵盤資料埠。**沒有硬體鍵盤時這裡回 0xFF**，而 0xFF 的 bit7 是
		// 「放開」——自己裝 IRQ1 的程式會把每一次都當成放開而忽略，
		// 所以它照樣跑、照樣輪詢，只是永遠收不到鍵（`docs/spec/014`）。
		return m.kbdData
	case port == 0x61:
		// 系統控制埠。ISR 用它 ack 鍵盤（bit7 設起再清掉）。
		return m.kbdPortB
	case port == 0x64:
		// 鍵盤控制器狀態：bit0 ＝ 輸出緩衝區有資料。
		if len(m.keyQueue) > 0 {
			return 0x15
		}
		return 0x14
	case port == 0x3DA:
		// ⚠ **讀 3DA 會重設屬性控制器的索引／資料 flip-flop。**
		// 程式就是用它來確保「下一次寫 3C0 是索引」；不做的話我們的
		// 相位會與程式相反，整份調色盤錯位。
		m.VGA.ResetACFlip()
		// bit3 ＝ 垂直回掃、bit0 ＝ 顯示中。**兩個都要會變**，
		// 這樣不管程式等的是哪一種邊緣都轉得出來。
		if (m.portTicks>>4)&1 != 0 {
			return 0x09
		}
		return 0x00
	case port >= 0x40 && port <= 0x42:
		return uint8(-int(m.portTicks)) // PIT 是遞減計數器
	case port == 0x21:
		// 8259 的中斷遮罩。**一定要讀得回自己寫進去的值。**
		// 「讀 → or 1 → 寫回」是遮蔽 IRQ0 的標準寫法（原版的語音初始化
		// 就是這樣，`docs/spec/016` §2.1）；讀回預設的 0xFF 之後寫回去
		// 就把**全部**中斷關掉了，包括鍵盤——之後按什麼都沒反應，
		// 而畫面照樣在動，看起來像遊戲卡住不像中斷被關。
		return m.picMask
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
	}
	return 0xFF
}

func (m *Machine) Out8(p uint16, v uint8) {
	m.Ports[p] = v
	m.PortLog = append(m.PortLog, PortWrite{Port: p, Val: v, Step: m.Steps})

	// PIT 通道 0：程式寫進去的除數決定它自己的時基（`pit.out`）。
	// **這裡只記設定、不推進時鐘**——時間由 `IRQ0Every` 的指令數驅動。
	m.pit.out(p, v)

	// 埠 0x61 要**讀得回自己寫進去的值**。鍵盤 ISR 的 ack 是
	// 「讀 → 設 bit7 → 寫回 → 清 bit7 → 寫回」，讀不回去的話
	// 它會把一個亂數寫進去，而那個亂數的低位元管的是喇叭與 RAM 檢查。
	if p == 0x61 {
		m.kbdPortB = v
		m.outSpeaker(v)
	}

	// 8259 的 OCW1。被遮蔽的中斷掛起不送，放行時補送。
	if p == 0x21 {
		m.picMask = v
	}

	// PIT（8253/8254）通道 0 的分頻。
	if p == 0x40 || p == 0x43 {
		m.pitWrite(p, v)
	}

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
	// 序列器、繪圖控制器與屬性控制器（`docs/spec/013` §3.2）。
	if m.VGA.Out(p, v) {
		// Map Mask 被寫成非預設值也算「進了平面模式」，見 planarActive。
		m.planarOn = m.planarActive()
	}

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

// pitWrite 追蹤 PIT 通道 0 的分頻，並照比例調整 IRQ0 的間隔。
//
// ⚠ **不追這個的話，改過分頻的程式全部跑在錯的速度上，而且不會報錯。**
// 常駐的音效驅動幾乎一定會把分頻調快（自己的音樂 tick 要細），再讓
// 自己的 `int 08h` 每 N 次才鏈回 BIOS，好讓 18.2 Hz 的系統 tick 不變。
// 只認 BIOS 的 18.2 Hz、不認分頻，等於把驅動的音樂 tick 也當成 18.2 Hz
// ——遊戲裡任何「等 N 個驅動 tick」的迴圈就慢上一個數量級。
//
// 量過（源平合戰）：`FMDRV.COM` 把分頻設成 4096（291.3 Hz），開場每一幕
// 都在等它的計數器。沒有這一段時 2 億道指令只走 268 個計數，開場一格都
// 播不出來——而畫面停在 KOEI 標誌上，看起來像「還沒畫完」。
//
// 控制位元組（埠 43h）的版面：bit7–6 通道、bit5–4 存取方式
// （0 ＝ 鎖存、1 ＝ 只寫低、2 ＝ 只寫高、3 ＝ 先低後高）、bit3–1 模式、
// bit0 ＝ BCD。這裡只管通道 0 的分頻，模式與 BCD 不影響 IRQ0 的頻率。
func (m *Machine) pitWrite(port uint16, v uint8) {
	if port == 0x43 {
		if v>>6 != 0 { // 只管通道 0
			return
		}
		if a := (v >> 4) & 3; a != 0 { // 0 ＝ 鎖存，不改存取方式
			m.pitAccess, m.pitPhase = a, 0
		}
		return
	}
	// 埠 40h：照存取方式組出分頻值。
	switch m.pitAccess {
	case 1: // 只寫低位元組
		m.setPITDiv(uint32(v))
	case 2: // 只寫高位元組
		m.setPITDiv(uint32(v) << 8)
	default: // 先低後高
		if m.pitPhase == 0 {
			m.pitLo, m.pitPhase = v, 1
			return
		}
		m.pitPhase = 0
		m.setPITDiv(uint32(m.pitLo) | uint32(v)<<8)
	}
}

// setPITDiv 換分頻值，並重算 IRQ0 的間隔。寫 0 的意思是 65536。
func (m *Machine) setPITDiv(div uint32) {
	if div == 0 {
		div = PITDefaultDivisor
	}
	m.PITDiv = div
	m.recalcIRQ0()
}

// CycPerIRQ0 回兩次 IRQ0 之間幾個週期，給報告用。
func (m *Machine) CycPerIRQ0() uint64 { return m.cycPerIRQ0 }

// RecalcIRQ0 是 recalcIRQ0 的對外版，改過 CPUHz 之後要叫一次。
func (m *Machine) RecalcIRQ0() { m.recalcIRQ0() }

// recalcIRQ0 依現在的分頻算出兩次 IRQ0 之間的間隔，週期與指令兩種模型
// 各算一份。**下一次中斷要照新的間隔重排**，不要沿用舊間隔算出來的時刻。
func (m *Machine) recalcIRQ0() {
	hz := m.CPUHz
	if hz == 0 {
		hz = DefaultCPUHz
	}
	cyc := hz * uint64(m.PITDiv) / PITHz
	if cyc == 0 {
		cyc = 1
	}
	m.cycPerIRQ0 = cyc
	// **第一次中斷排在一個完整週期之後**，理由與 nextIRQ0 那一行相同：
	// 零值的話 `Cycles >= nextIRQ0Cyc` 在第 0 步就成立，程式的第一道
	// 指令還沒執行就先被 int 08h 打斷。
	if m.nextIRQ0Cyc == 0 || m.nextIRQ0Cyc > m.CPU.Cycles+cyc {
		m.nextIRQ0Cyc = m.CPU.Cycles + cyc
	}

	if m.IRQ0Every == 0 {
		return // 計時器關著，下面那份用不到
	}
	base := m.IRQ0Base
	if base == 0 {
		base = DefaultIRQ0Every
	}
	every := base * uint64(m.PITDiv) / PITDefaultDivisor
	// 分頻值可以小到 1（約 1.19 MHz），照比例算會變成每兩道指令一次中斷
	// ——處理常式自己跑不完，機器卡死在中斷裡。**卡死看起來像當掉，
	// 不像設定太快**，所以夾住，而且夾住要記一次。
	if every < MinIRQ0Every {
		every = MinIRQ0Every
		m.IRQ0Clamped++
	}
	m.IRQ0Every = every
	if m.nextIRQ0 > m.Steps+every {
		m.nextIRQ0 = m.Steps + every
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

// ⚠ **這三支要走 Read8／Write8，不能直接碰 Mem。**
// 平面模式下 `A0000` 之後不在 Mem 裡，直接碰的話「程式把檔案讀進 VRAM」
// 這條路徑會安靜地寫到一塊沒人看的記憶體，畫面上什麼都不會出現。
func (m *Machine) Read16(a uint32) uint16 {
	return uint16(m.Read8(a)) | uint16(m.Read8(a+1))<<8
}

func (m *Machine) Write16(a uint32, v uint16) {
	m.Write8(a, uint8(v))
	m.Write8(a+1, uint8(v>>8))
}

func (m *Machine) WriteBytes(a uint32, b []byte) {
	// **監看也要涵蓋批次寫入。** 檔案讀取、映像載入與 EXEC 都走這一條，而那
	// 正是「這一格是誰寫的」最常見的答案。繞過 Write8 的話監看會安靜地漏掉
	// 它們——看起來像「沒有人寫過」，實際上是整塊被蓋掉了。
	for i, v := range b {
		m.Write8(a+uint32(i), v)
	}
}

// Indexed 回傳畫面的色號陣列：mode 13h 是 320×200，planar 模式
// （`docs/spec/013`）是 VideoSize() 那個尺寸的 0–15 色號。
//
// **回的是色號不是 RGB**——對拍在色號空間做（`docs/spec/005` §3）。
// 回傳的是複本，呼叫端改它不會動到機器。
func (m *Machine) Indexed() []uint8 {
	if m.planarOn {
		return m.planarIndexed()
	}
	out := make([]uint8, VideoWidth*VideoHigh)
	copy(out, m.Mem[VideoSeg*16:VideoSeg*16+len(out)])
	return out
}

// Step 執行一道指令，必要時先送 IRQ0。
// MaxSegLog 是 SegLog（ring）保留的筆數。
const MaxSegLog = 100_000

// Step 執行一道指令，必要時先送 IRQ1或IRQ0。
func (m *Machine) Step() error {
	// **兩個檢查都內聯在這裡**：`tick` 與 `keyTick` 絕大多數指令上
	// 第一行就 return，而函式呼叫本身佔了執行時間的 3.8%
	//（`docs/spec/015` §3）。把「要不要進去」提到呼叫端之後，
	// 常見情形連呼叫都不用發。
	//
	// ⚠ **這個條件要涵蓋 `tick` 會做的每一件事**，不只是計時器：
	// 排進來的回呼與週期遠呼叫跟 IRQ0 到不到期無關，漏了它們的話
	// 回呼永遠不發——而畫面照跑，看起來像「事件沒排進去」。
	// 在 `tick` 裡加東西，這裡要跟著加。
	if len(m.cbQueue) > 0 || m.periodic.on ||
		(m.VGAFrameEvery > 0 && m.Steps >= m.nextFrame) ||
		m.irq0Pending || (m.IRQ0Every > 0 && m.Steps >= m.nextIRQ0) ||
		(m.CycleClock && m.cycPerIRQ0 > 0 && m.CPU.Cycles >= m.nextIRQ0Cyc) {
		m.tick()
	}
	if len(m.keyQueue) > 0 {
		m.keyTick()
	}
	m.Steps++
	if m.Coverage != nil {
		if a := cpu.Addr(m.CPU.Seg[cpu.CS], m.CPU.IP); int(a) < len(m.Coverage) {
			m.Coverage[a] = true
		}
	}
	m.insnCS, m.insnIP = m.CPU.Seg[cpu.CS], m.CPU.IP
	if !m.TraceSegs && !m.WatchDSOn {
		err := m.CPU.Step()
		if len(m.wordWatches) > 0 {
			m.pollWords()
		}
		return err
	}
	fromSeg, fromOff := m.CPU.Seg[cpu.CS], m.CPU.IP
	prevDS := m.CPU.Seg[cpu.DS]
	err := m.CPU.Step()
	if len(m.wordWatches) > 0 {
		m.pollWords()
	}

	if m.WatchDSOn && m.CPU.Seg[cpu.DS] == m.WatchDS && prevDS != m.WatchDS &&
		len(m.DSLoads) < MaxSegLog {
		m.DSLoads = append(m.DSLoads, SegChange{
			Step: m.Steps, FromSeg: fromSeg, FromOff: fromOff,
			ToSeg: m.CPU.Seg[cpu.DS], ToOff: m.CPU.R[cpu.BX],
		})
	}
	if !m.TraceSegs {
		return err
	}
	if cs := m.CPU.Seg[cpu.CS]; cs != fromSeg {
		ch := SegChange{
			Step: m.Steps, FromSeg: fromSeg, FromOff: fromOff,
			ToSeg: cs, ToOff: m.CPU.IP,
		}
		if m.SegFirst == nil {
			m.SegFirst = map[uint16]SegChange{}
		}
		if _, seen := m.SegFirst[cs]; !seen {
			m.SegFirst[cs] = ch
		}
		m.SegLog = append(m.SegLog, ch)
		if len(m.SegLog) > 2*MaxSegLog { // 砍掉前半，留最近的 MaxSegLog 筆
			m.SegLog = append(m.SegLog[:0], m.SegLog[MaxSegLog:]...)
		}
	}
	return err
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
//
// 送法永遠是走向量表（向量 8 預設指向 StubSeg 的 BIOS stub，見
// initVectors）：**BIOS 的預設動作（推進 0040:006C、轉呼 int 1Ch）
// 本身就是向量指向的那段 stub 程式碼**。程式裝了自己的 int 08h 時
// 存起來的「舊向量」就是這個 stub，它 chain 回去 BIOS 行為就還在——
// 不 chain 的話 1Ch 與 BDA 計數停下來，那與真機一致。
//
// 舊版反過來做（程式裝了 08h 就不做 BIOS 的事），結果是：FMDRV.COM
// 掛了 int 08h 之後 OPEN.EXE 掛在 int 1Ch 的動畫計數永遠不動，
// 開場停在 GRPDRV 的重畫迴圈裡（`docs/spec/008` §4、
// yuan/workplace/boot-20260906-02）。
// tick 是每道指令之前要做的事：回呼、週期遠呼叫、幀、計時器中斷。
//
// ⚠ **這裡加一件事，`Step` 的進入條件也要加**——那邊的判斷是為了讓
// 絕大多數指令連呼叫都不用發（`docs/spec/015` §3），漏掉就等於那件事
// 永遠不會發生。
func (m *Machine) tick() {
	// 外面排進來的回呼（滑鼠事件常式那一類）。**優先於週期回呼**：
	// 事件是有時序意義的，週期回呼晚一格沒差。
	if m.startCallback() {
		return
	}
	// 週期遠呼叫。**不看 IF**：真機上這是別人的 ISR 在 `cli` 之後才呼叫
	// 遊戲的回呼，遊戲那一支自己 `cli/pushf … popf/retf`。
	// 掛在 IF 上的話初始化期間那一大段 `cli` 會把時鐘整個吃掉。
	if m.periodic.on && m.Steps >= m.periodic.next {
		m.periodic.next = m.Steps + m.periodic.every
		m.periodic.Calls++
		m.CPU.FarCall(m.periodic.seg, m.periodic.off)
		return
	}
	// 垂直回掃 ＝ 螢幕刷新一次 ＝ 一幀。
	//
	// **不看 IF**：它是顯示硬體的事實，不是中斷——程式 `cli` 的時候
	// 螢幕照樣在掃。這也是它比「程式畫完一幀的位址」通用的原因：
	// 不必知道那支程式長什麼樣。
	if m.VGAFrameEvery > 0 && m.Steps >= m.nextFrame {
		m.nextFrame = m.Steps + m.VGAFrameEvery
		m.Frames++
		if m.onFrame != nil {
			m.onFrame()
		}
	}
	switch {
	case m.CycleClock: // 週期時鐘（`docs/spec/004` §5.1）
		if m.cycPerIRQ0 > 0 && m.CPU.Cycles >= m.nextIRQ0Cyc {
			m.nextIRQ0Cyc = m.CPU.Cycles + m.cycPerIRQ0
			m.irq0Pending = true
		}
	case m.IRQ0Every > 0: // 指令數時鐘（預設）
		if m.Steps >= m.nextIRQ0 {
			m.nextIRQ0 = m.Steps + m.IRQ0Every
			// **先掛起來，不要直接送。** 初始化期間大量 `CLI`，
			// 當場丟掉的話那一段的 tick 全部消失。
			m.irq0Pending = true
		}
	}
	if !m.CPU.Flag(cpu.IF) || m.picMask&0x01 != 0 {
		return
	}
	if !m.irq0Pending {
		return
	}
	m.irq0Pending = false
	m.Ticks++
	if m.onTick != nil {
		m.onTick()
	}
	m.CPU.Interrupt(0x08)
}

// SetOnFrame 登記「每一次垂直回掃就呼叫」（`docs/spec/187`）。nil 取消。
//
// 回呼時 `Frames` 已經是這一幀的編號，畫面是這一幀要顯示的內容。
func (m *Machine) SetOnFrame(f func()) { m.onFrame = f }

// SetOnTick 登記「每送出一次計時器中斷就呼叫」（`docs/spec/187` §4）。
//
// nil 取消。回呼在 `Ticks++` 之後、`Interrupt(0x08)` 之前執行，
// 所以看到的 `Ticks` 已經是這一次的編號，而中斷處理常式還沒跑。
func (m *Machine) SetOnTick(f func()) { m.onTick = f }

// SetPeriodicFarCall 登記「每 every 道指令遠呼叫 seg:off 一次」。
//
// every ＝ 0 或位址是 0:0 都當成取消。
func (m *Machine) SetPeriodicFarCall(seg, off uint16, every uint64) {
	if every == 0 || (seg == 0 && off == 0) {
		m.periodic.on = false
		return
	}
	m.periodic.seg, m.periodic.off = seg, off
	m.periodic.every, m.periodic.next = every, m.Steps+every
	m.periodic.on = true
}

// ClearPeriodicFarCall 取消登記。
func (m *Machine) ClearPeriodicFarCall() { m.periodic.on = false }

// PeriodicCalls 是已經發出去幾次。**收工前看一眼**：0 次表示登記沒生效，
// 而那與「遊戲不看時鐘」長得一模一樣。
func (m *Machine) PeriodicCalls() uint64 { return m.periodic.Calls }

// QueueScanCodes 是 QueueScan 的別名（IBM PC/AT Set 1 掃描碼，
// bit7 立著是放開）。
func (m *Machine) QueueScanCodes(codes ...uint8) { m.QueueScan(codes...) }

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
//
// 三個向量另外指到特殊 stub（`docs/spec/004` §2.1、`docs/spec/011` §2）：
//
//   - `int 21h`／`int 10h`／`int 15h` 指到 `CD F2`／`CD F3`／`CD F4`。
//     **號碼要跟被服務的中斷不同**，理由見 DosTrapOff 的說明：TSR 掛了
//     自己的 int 21h 再 chain 回舊向量時，`CD 21` 會繞回它自己。
//   - `int 08h` 指到一段真的 BIOS 預設處理（推進 `0040:006C` 再轉呼
//     `int 1Ch`）。程式裝自己的 int 08h 時存下的「舊向量」就是它，
//     chain 回來 BIOS 行為就在（見 tick 的註解）。跨日重置（`0040:0070`）
//     不做——24 小時才會到，對拍跑不到。
//   - `int 67h` 指到 EMSSeg 的 EMM 驅動 header，那裡有 `EMMXXXX0` 簽章。
//
// ⚠ **其餘被服務層接手的中斷（16h／33h／13h／1Ah…）用的還是
// `CD n` 形式的 per-vector stub。** 那些中斷上還沒觀測到「掛勾再 chain
// 回舊向量」的程式；真的遇到就要照上面那條給它一個 trampoline 號碼。
// **這是假說待驗，不是量到的結論。**
func (m *Machine) initVectors() {
	for v := 0; v < 256; v++ {
		off := uint32(StubSeg)*16 + uint32(v)*stubStride
		m.Mem[off] = 0xCD // int n
		m.Mem[off+1] = uint8(v)
		m.Mem[off+2] = 0xCF // iret
		m.Write16(uint32(v)*4, StubOff(uint8(v)))
		m.Write16(uint32(v)*4+2, StubSeg)
	}

	// 服務型中斷的 trampoline。
	m.WriteBytes(StubSeg*16+DosTrapOff, []byte{0xCD, 0xF2, 0xCF})
	m.WriteBytes(StubSeg*16+VideoTrapOff, []byte{0xCD, 0xF3, 0xCF})
	m.WriteBytes(StubSeg*16+ATrapOff, []byte{0xCD, 0xF4, 0xCF})
	// ⚠ **XMS 的 entry 是 far call，不是中斷**——結尾要 `RETF`（0CBh），
	// 不是 `IRET`。用 IRET 的話每呼叫一次就多彈兩個位元組（呼叫端只推了
	// CS:IP，IRET 卻連 FLAGS 一起彈），堆疊一路歪掉，最後在某個 `RETF`／
	// `IRET` 跳進垃圾——而中間所有服務都「成功」。
	m.WriteBytes(StubSeg*16+XMSTrapOff, []byte{0xCD, 0xF5, 0xCB})

	// BIOS int 08h stub。⚠ **暫存器要保存，而且要在 `int 1Ch` 之後才還原。**
	//
	// IBM PC BIOS 的 TIMER_INT 順序是
	// `push ds/ax/dx` → `DS=40h` → 推進計數 → `int 1Ch` → `pop dx/ax/ds` →
	// `iret`。**還原在 `int 1Ch` 之後，所以 1Ch 的處理常式弄髒 DS 也沒關係**——
	// BIOS 幫被中斷的程式把它救回來。
	//
	// 源平合戰的 int 1Ch 常式就是這個形狀：`cli / pusha / mov ds,2A1E …
	// popa / sti / jmp far 舊向量`。`pusha` 不含 DS，所以它離開時 DS 是
	// 自己的段。舊版 stub 在 `int 1Ch` **之前**就把暫存器還原掉，於是那個
	// DS 一路漏回被中斷的位元碼直譯器：直譯器接著用錯的段抓位元碼，
	// 跑了四百道之後查表查到範圍外，`DS` 變成 0，程式走進低位記憶體結束。
	// 症狀離成因十九萬道指令遠，而且畫面停在正常的對話框上。
	//
	//	push ds / push ax / push dx / mov ax,40h / mov ds,ax /
	//	inc word [6Ch] / jnz +4 / inc word [6Eh] /
	//	int 1Ch / pop dx / pop ax / pop ds / iret
	m.WriteBytes(StubSeg*16+biosTimerOff, []byte{
		0x1E, 0x50, 0x52,
		0xB8, 0x40, 0x00, 0x8E, 0xD8,
		0xFF, 0x06, 0x6C, 0x00,
		0x75, 0x04,
		0xFF, 0x06, 0x6E, 0x00,
		0xCD, 0x1C,
		0x5A, 0x58, 0x1F,
		0xCF,
	})

	m.Write16(0x21*4, DosTrapOff)
	m.Write16(0x10*4, VideoTrapOff)
	m.Write16(0x15*4, ATrapOff)
	m.Write16(0x08*4, biosTimerOff)

	// EMM 的 driver header ＋ int 67h 向量（`docs/spec/014` §3.1）。
	m.WriteBytes(EMSSeg*16, []byte{0xCD, 0xF6, 0xCF})
	m.WriteBytes(EMSSeg*16+0x0A, []byte("EMMXXXX0"))
	m.Write16(0x67*4, 0)
	m.Write16(0x67*4+2, EMSSeg)
}

// VGAState 回目前的圖形控制器、序列器與 latch。
//
// 畫面上的位元不一定等於 CPU 寫進去的位元組：write mode、bit mask、
// set/reset 與 latch 會先改一次。查「為什麼寫 09 出來是 C3」的時候，
// 光看寫入指令沒有用，要看那一刻硬體的狀態。
func (m *Machine) VGAState() (gc [16]uint8, seq [8]uint8, latch [4]uint8) {
	return m.VGA.Regs()
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
			m.QueueScan(shift)
		}
		m.QueueScan(code, code|brk)
		if upper {
			m.QueueScan(shift | brk)
		}
	}
	return nil
}

// TextScreen 把彩色文字頁（B800）讀成一列一列的字串。
//
// cols 為 0 時用 BDA 記的欄數，再不合理就退回 80。
// 非可列印的字元換成空白——文字模式的緩衝區在程式清畫面之前是垃圾，
// 直接印出來只會蓋掉真正有內容的那幾列。
//
// 每一列的尾端空白會去掉，但**空白列會保留**：第幾列有東西本身是資訊。
func (m *Machine) TextScreen(cols int) []string {
	if cols <= 0 || cols > 132 {
		cols = int(m.Read16(0x44A))
	}
	if cols <= 0 || cols > 132 {
		cols = 80
	}
	rows := make([]string, 25)
	for row := range rows {
		line := make([]byte, cols)
		for col := 0; col < cols; col++ {
			ch := m.Read8(TextSeg*16 + uint32(row*cols+col)*2)
			if ch < 0x20 || ch > 0x7E {
				ch = ' '
			}
			line[col] = ch
		}
		rows[row] = strings.TrimRight(string(line), " ")
	}
	return rows
}

// SegmentBytes 讀一個段開頭的 n 個位元組。
//
// 差分比對常要把「原版此刻某個段的內容」整塊拿出來，與另一個來源對拍。
// 逐 byte 呼叫 Read8 也做得到，但那會讓呼叫端自己算線性位址——
// 而算錯的症狀是「比對出一堆差異」，不是「報錯」。
//
// 越界回 nil：回一段補零的東西會被當成「那裡真的是零」。
func (m *Machine) SegmentBytes(seg uint16, n int) []byte {
	base := uint32(seg) * 16
	if n <= 0 || base+uint32(n) > uint32(len(m.Mem)) {
		return nil
	}
	out := make([]byte, n)
	copy(out, m.Mem[base:base+uint32(n)])
	return out
}

// Find 掃整個位址空間，回傳 pattern 出現的每一個實體位址。
//
// 拿來定位「被程式自己搬進記憶體的東西」——那種東西的載入位址靜態看不出來，
// 而挑一段不會被改寫的內容當指紋是最直接的辦法。
//
// pattern 是空的就回 nil：回「到處都是」比回「找不到」更難查。
func (m *Machine) Find(pattern []byte) []uint32 {
	if len(pattern) == 0 || len(pattern) > len(m.Mem) {
		return nil
	}
	var hits []uint32
	last := uint32(len(m.Mem) - len(pattern))
	for a := uint32(0); a <= last; a++ {
		if m.Mem[a] != pattern[0] {
			continue
		}
		if string(m.Mem[a:a+uint32(len(pattern))]) == string(pattern) {
			hits = append(hits, a)
		}
	}
	return hits
}
