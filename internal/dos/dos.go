// Package dos 是 DOS 與 BIOS 服務層：`int 21h`、`int 10h`、`int 33h`、`int 16h`。
//
// 規格在 `docs/spec/004-dos-bios-services.md`（§2／§3／§4 都是 READY）。
//
// 三條貫穿全部的原則（`docs/spec/004` §1），每一條都是用一整輪換來的：
//
//  1. **未實作的服務只宣告成功，不碰呼叫端沒要求改的暫存器。**
//     清 `AX` 會讓「設中斷向量」的迴圈把 `AH` 變成 0 ＝ 結束程式。
//  2. **有些服務的「失敗」才是正確答案。** `AH=4Ah` 是記憶體探測。
//  3. **沒建的東西不會抱怨，只會給 0，而 0 是合法值。**
package dos

import (
	"fmt"
	"sort"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// Time 是回給 `int 21h AH=2Ch` 的時刻。
//
// **固定值。** 時間會餵進亂數種子，浮動的話同一組按鍵每次跑到不同畫面
// （`rich2/CLAUDE.md` §8：截圖驗收要帶固定 seed）。
//
// **預設全 0，這是刻意與原版那邊的固定種子版對齊的。**
// `rich2/tools/patch_seed.py` 把 `TIMER` 內部的 `mov ah,2Ch / int 21h`
// 換成 `xor cx,cx / xor dx,dx`；我們這邊不改 binary，讓 `AH=2Ch` 直接
// 回 CX=DX=0，效果相同。兩邊的 `RANDOMIZE TIMER` 因此拿到同一個種子，
// 逐點對拍才有意義（`docs/spec/001` MVP-B）。
type Time struct{ Hour, Min, Sec, Hundredth uint8 }

// clock 是 `AH=2Ch` 回報的時刻：Now 當基準，再加上 PIT tick 推出來的時間。
//
// ⚠ **時鐘不能是固定值。** 第一版 `AH=2Ch` 直接回 `Now`，於是任何
// 「等 N 個百分之一秒」的迴圈都轉不出來——智冠《三國演義》的識別畫面
// 就卡在這裡，六百一十二萬次呼叫問同一個時刻。從外面看是「程式還活著」，
// 與「在算很久」分不開。
//
// 推進的依據是 **PIT tick 不是牆上的時間**：時鐘掛在指令數上，
// 同樣的輸入永遠得到同樣的時刻，對拍才是決定性的
// （與 `machine.tick` 的模型一致）。18.2 Hz ＝ 一個 tick 約 5.4925 個
// 百分之一秒。
func (d *DOS) clock() Time {
	base := (uint64(d.Now.Hour)*3600+uint64(d.Now.Min)*60+uint64(d.Now.Sec))*100 +
		uint64(d.Now.Hundredth)
	// 5493/1000 ≈ 100/18.2，用整數算避免浮點進到決定性的路徑上。
	total := (base + d.M.Ticks*5493/1000) % (24 * 3600 * 100)
	return Time{
		Hour:      uint8(total / 360000),
		Min:       uint8(total / 6000 % 60),
		Sec:       uint8(total / 100 % 60),
		Hundredth: uint8(total % 100),
	}
}

// Mouse 是滑鼠狀態。座標用**像素**存，回報時才乘上 XScale
// （mode 13h 的標準驅動水平回報 0–639，`rich2/docs/re/182` §3）。
type Mouse struct {
	X, Y    uint16
	Buttons uint16
	// Press／Release 是 AX=5／AX=6 的統計，讀走就歸零。
	Press, Release uint16
	// XScale 是水平的虛擬座標倍率。**2 是標準**。
	XScale uint16
	// Polls 記下每一次 AX=3 回報出去的東西。**這是分辨「輸入沒送到」與
	// 「送到了但答錯」的唯一辦法**——兩者的畫面表現一模一樣。
	Polls []Poll

	// Sets 記下每一次 AX=4（程式自己設游標位置）。
	// 程式設過之後我們注入的位置就被蓋掉了，而症狀是「滑鼠移不動」。
	Sets []Poll

	// Calls 是每個 int 33h 功能號被叫了幾次。
	// **診斷「點了沒反應」的第一步**：先確認遊戲到底在讀哪一支。
	Calls map[uint16]int
}

// Poll 是一次 `AX=3` 的回報內容。
type Poll struct {
	X, Y, Buttons uint16
	Step          uint64
}

// DOS 是服務層。用 Install 掛到機器上。
type DOS struct {
	M *machine.Machine

	// Root 是原版素材的目錄（玩家自備）。**本專案不含任何原版檔案。**
	Root string

	// Now 是固定時刻，Mouse 是滑鼠狀態。
	Now   Time
	Mouse Mouse

	// Console 收 `AH=02h`／`06h`／`09h` 與 `int 10h AH=0Eh` 印出來的字。
	// **錯誤訊息走這條**，收不到就等於什麼都不知道。
	Console []byte

	// Calls 是每一種 (中斷, AH) 的呼叫次數；nil 表示不記。
	Calls map[Call]int

	// Stdin 是 `AH=3Fh` 讀 handle 0 時要餵的位元組；空的時候餵 StdinFill。
	//
	// ⚠ **不能回「讀到 0 個」**（等同 EOF）：主程式會當成輸入結束、
	// 還原中斷向量然後 exit（`rich2/docs/re/005`「死點修掉了」）。
	Stdin     []byte
	StdinFill uint8

	// StdinEmptyReadsZero 讓「佇列空」回報**讀到 0 個位元組**，
	// 而不是餵一個 StdinFill。
	//
	// **為什麼需要它**：編譯後 BASIC 的 `INKEY$` 空轉是
	// `while INKEY$ = "" : wend`（`rich2/docs/re/153` 的 `0x2CB42`：
	// `while ds:1094h == ds:2232h`，後者是空字串）。餵一個 `00` 會讓
	// `INKEY$` 拿到 `CHR$(0)`——**非空字串**，於是空轉立刻結束。
	// 症狀是**所有「按任意鍵繼續」的畫面一閃而過**：畫出來了、
	// 也真的畫對了，但幾十萬道指令之後就被下一次重繪蓋掉，
	// 而外面用粗取樣根本看不到它存在過。
	//
	// ⚠ **預設 false，不要全域打開。** 主程式在別的地方會把「讀到 0 個」
	// 當成 EOF，然後還原中斷向量並結束（那正是 StdinFill 存在的理由）。
	// 這個開關是給「我現在就是要讓某張畫面停住」用的，
	// 用完要關掉。
	StdinEmptyReadsZero bool

	// KeyWaits 數「佇列空的時候被要求讀一個鍵」發生了幾次
	// （`AH=01h`／`07h`／`08h`）。
	//
	// **為什麼要數它**：真 DOS 的這幾個功能是**阻塞**的——呼叫一次就停在
	// 那裡等人按鍵。dosgolem 是步進式的，停不下來，只能立刻返回；
	// 於是程式在同一個迴圈裡空轉，從外面看是「跑滿指令上限、程式還活著」，
	// 與「跑掛了」「在算很久」長得一模一樣。
	//
	// 數出來就分得開了：KeyWaits 很大 ＝ **它在等鍵盤，不是在做事**，
	// 該餵鍵而不是加大 `-steps`。
	KeyWaits int

	// NonBlockingKeys 關掉阻塞語意，讓 `AH=01h`／`07h`／`08h` 在佇列空時
	// 回 `AL=0` 繼續跑（舊行為）。
	//
	// **零值 ＝ false ＝ 阻塞 ＝ 真 DOS 的語意**：忘記設的人得到的是對的那一邊，
	// 不是一個安靜說謊的模型。規格 `docs/spec/008`。
	NonBlockingKeys bool

	// Blocked 表示這一步停在阻塞式輸入上（佇列空）。取到鍵時清掉。
	// 上層可以據此停下來、餵鍵、再繼續。
	Blocked bool

	// Drive 是 `AH=19h` 的目前磁碟（0 ＝ A:、1 ＝ B:、**2 ＝ C:**），
	// Dir 是 `AH=47h` 的目前目錄。
	//
	// ⚠ **預設一定要是 C:（硬碟）。** 回 A: 的話程式判定自己是從磁片跑，
	// 停在「Please put Disk#2 in A: and put Disk#3 in B:」等按鍵——
	// 而在接上 `AH=40h` 之前，那個畫面在主控台上是**一片空白**。
	Drive uint8
	Dir   string

	// Unimplemented 記下每一個沒實作的功能號被叫了幾次。
	//
	// **「宣告成功」本身也會說謊**：該填的緩衝區沒填就是垃圾，症狀出現在
	// 很後面而且完全不指向這裡。所以要數、要印（`docs/spec/004` §1.3）。
	Unimplemented map[Call]int

	// Opened 是開過的檔（依序），Missing 是找不到的。兩份都是診斷用。
	Opened  []string
	Missing []string

	// Wrote 記下「程式想寫檔」的每一次。**我們不寫**（原版素材唯讀），
	// 但安靜地報成功會讓「存檔壞掉」查不出來。
	Wrote []Write

	// Exited 為真表示程式呼叫了 `AH=4Ch`／`AH=00h`；ExitCode 是它的回傳碼。
	Exited   bool
	ExitCode uint8

	handles    map[uint16]*handle
	// MaxHandles 是這個程序同時開得了幾個檔（含 0–4 的標準 handle）。
	// DOS 的預設是 20，`AH=67h` 可以調高。
	//
	// ⚠ **上限不是拿來擋人的，是號碼配置的邊界。** MSC 的低階 I/O
	// 用 handle 當索引查自己的表，表和 JFT 一樣大；handle 超出範圍時
	// `fopen` 會**先開成功再把它關掉並回 NULL**，看起來像開檔失敗，
	// 實際上是號碼太大。
	MaxHandles uint16
	freeSeg    uint16

	// arena 是 [freeSeg, MemTop) 這段的區塊表，依段位址排序、首尾相接、
	// 沒有空隙。每個區塊佔 1 段的假 MCB ＋ size 段的資料，
	// 交給程式的是 seg+1。規格 `docs/spec/009`。
	//
	// nil 表示還沒初始化；第一次配置時用當時的 freeSeg 建起來，
	// 這樣測試裡先設 freeSeg 再用的寫法仍然成立。
	arena []memBlock

	// Overlays 記每一次成功的 overlay 載入。
	//
	// **這是「程式載了哪些模組」的唯一觀測點。** overlay 沒有 PSP、
	// 不動 CS:IP，從暫存器與開檔清單都看不出它發生過。
	Overlays []OverlayLoad

	// Resizes 記每一次 AH=4Ah，用來查「可配置區起點是怎麼被決定的」。
	Resizes []ResizeCall

	// CallTrace 記每一次 int 21h 進入與離開時的 ES:BX。
	//
	// 用途只有一個：分辨「程式自己設的 ES:BX」與「某個服務把它改壞了」。
	// 服務回傳時動到不該動的暫存器，症狀會出現在很後面的另一個呼叫上。
	CallTrace []CallRec

	// MemTrace 記每一次配置／釋放。非 nil 才記（筆數可達上萬）。
	MemTrace []MemCall

	// KeyReads 記每一次按鍵被取走。永遠記——筆數是按鍵數，很少。
	KeyReads []KeyRead

	// FileTrace 記每一次檔案操作的參數與結果。
	//
	// **開檔清單只說「開過什麼」，說不出「要求讀哪一段、拿到多少」。**
	// 遊戲抱怨某個項目找不到時，要分辨「它算錯位移」與「我們回錯資料」
	// 就得看這個。
	FileTrace []FileOp
}

// Write 是一次被擋下來的寫檔。
type Write struct {
	Name string
	N    int
}

// Call 是一次沒實作的服務呼叫：哪一個中斷、AH、AL。
type Call struct {
	Int, AH, AL uint8
}

func (c Call) String() string {
	return fmt.Sprintf("int %02Xh AH=%02X AL=%02X", c.Int, c.AH, c.AL)
}

// New 造一個服務層。root 是原版素材目錄。
func New(m *machine.Machine, root string) *DOS {
	return &DOS{
		M: m, Root: root,
		Now:           Time{}, // 全 0：與原版的固定種子版對齊，見 Time 的說明
		Mouse:         Mouse{XScale: 2, Calls: map[uint16]int{}},
		Drive:         2, // C:，見 Drive 欄位的說明
		Dir:           "RICH2",
		Unimplemented: map[Call]int{},
		handles:       map[uint16]*handle{},
		MaxHandles:    20, // DOS 預設；AH=67h 可調
	}
}

// Install 把服務層掛到機器的中斷鉤子上。**要在 LoadEXE 之後叫**——
// 它會記下映像後面的第一個可配置段。
func (d *DOS) Install() {
	d.freeSeg = d.M.FreeSeg
	d.M.CPU.IntHook = d.handle
}

// handle 是中斷分派。回 true 表示「處理完了」，CPU 不走向量表。
//
// ⚠ **只在向量還指著我們的 stub 時才接手。** 程式會用 `AH=25h` 裝自己的
// 處理常式——最重要的是 `INT 34h`–`3Dh`（binary 自帶的 Microsoft 浮點
// 模擬器，全檔 876 個呼叫）。那些一定要讓 CPU 真的跳過去，
// 攔下來的話**所有浮點運算都會落空**，而 BASIC 的金錢運算全靠它。
//
// 這條規則也順便處理了計時器（`int 08h`／`1Ch`）與 Ctrl-Break（`int 23h`）：
// 程式裝了誰就跑誰的，不必逐個列白名單。
func (d *DOS) handle(c *cpu.CPU, n uint8) bool {
	if seg := d.M.Read16(uint32(n)*4 + 2); seg != machine.StubSeg {
		return false // 程式自己裝了處理常式
	}
	// Calls 記每一種 (中斷, AH) 呼叫過幾次。**只記不改行為**——
	// 「它到底在做什麼」在沒有畫面可看的時候只剩這個問得到。
	if d.Calls != nil {
		d.Calls[Call{Int: n, AH: uint8(c.R[cpu.AX] >> 8)}]++
	}
	switch n {
	case 0x21:
		d.int21(c)
	case 0x10:
		d.int10(c)
	case 0x33:
		d.int33(c)
	case 0x16:
		d.int16(c)
	case 0x11:
		// 取設備清單：就是 BDA 那一格（`docs/spec/003` §1）。
		// 不實作的話 AX 保持呼叫端傳進來的值，而顯示卡欄位是垃圾。
		c.R[cpu.AX] = d.M.Read16(0x0040*16 + 0x10)
	case 0x12:
		// 取常規記憶體大小，單位 KB。
		c.R[cpu.AX] = d.M.Read16(0x0040*16 + 0x13)
	case 0x13:
		d.int13(c)
	case 0x1A:
		d.int1A(c)
	case 0x20:
		d.exit(c, 0)
	default:
		d.note(n, uint8(c.R[cpu.AX]>>8), uint8(c.R[cpu.AX]))
		clearCarry(c)
	}
	return true
}

func (d *DOS) exit(c *cpu.CPU, code uint8) {
	d.Exited, d.ExitCode = true, code
	c.Halted = true
}

// note 記一筆沒實作的呼叫。
func (d *DOS) note(intNo, ah, al uint8) {
	d.Unimplemented[Call{Int: intNo, AH: ah, AL: al}]++
}

// UnimplementedReport 把統計排成可讀的清單，次數多的在前面。
//
// **收工前一定要看它。** 「跑得動」與「跑得動但行為不對」的差別就在這裡。
func (d *DOS) UnimplementedReport() []string {
	type row struct {
		c Call
		n int
	}
	rows := make([]row, 0, len(d.Unimplemented))
	for c, n := range d.Unimplemented {
		rows = append(rows, row{c, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].c.String() < rows[j].c.String()
	})
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprintf("%s ×%d", r.c, r.n))
	}
	return out
}

// Close 關掉所有還開著的檔。
func (d *DOS) Close() {
	for _, h := range d.handles {
		if h.f != nil {
			h.f.Close()
		}
	}
	d.handles = map[uint16]*handle{}
}

// ---- 旗標與暫存器的半邊 --------------------------------------------------

func setCarry(c *cpu.CPU)   { c.SetFlags(c.Flags | cpu.CF) }
func clearCarry(c *cpu.CPU) { c.SetFlags(c.Flags &^ cpu.CF) }

// setAL／setAH 只動 AX 的一半，不碰另一半——原則 1。
func setAL(c *cpu.CPU, v uint8) { c.R[cpu.AX] = c.R[cpu.AX]&0xFF00 | uint16(v) }
func setAH(c *cpu.CPU, v uint8) { c.R[cpu.AX] = c.R[cpu.AX]&0x00FF | uint16(v)<<8 }

func setBL(c *cpu.CPU, v uint8) { c.R[cpu.BX] = c.R[cpu.BX]&0xFF00 | uint16(v) }
func setBH(c *cpu.CPU, v uint8) { c.R[cpu.BX] = c.R[cpu.BX]&0x00FF | uint16(v)<<8 }

func ah(c *cpu.CPU) uint8 { return uint8(c.R[cpu.AX] >> 8) }
func al(c *cpu.CPU) uint8 { return uint8(c.R[cpu.AX]) }
func bl(c *cpu.CPU) uint8 { return uint8(c.R[cpu.BX]) }

// ArenaDump 把配置器目前的區塊表印成一行一塊，供診斷用。
func (d *DOS) ArenaDump() []string {
	out := make([]string, 0, len(d.arena)+1)
	out = append(out, fmt.Sprintf("freeSeg=%04X arena=%d 塊（nil=%v）",
		d.freeSeg, len(d.arena), d.arena == nil))
	for _, b := range d.arena {
		st := "已配置"
		if b.free {
			st = "自由"
		}
		out = append(out, fmt.Sprintf("  seg=%04X size=%04X %s", b.seg, b.size, st))
	}
	return out
}

// memBlock 是配置器的一個區塊。seg 是假 MCB 的位置，資料從 seg+1 開始。
type memBlock struct {
	seg  uint16
	size uint16 // 資料段數，不含 MCB
	free bool
}

// OverlayLoad 是一次 `AH=4Bh AL=03` 的載入紀錄。
type OverlayLoad struct {
	Name  string
	Seg   uint16
	Reloc uint16
	Size  int

	// PBSeg/PBOff 是參數區塊的位置，PBRaw 是它前 8 個 byte。
	// 載入段看起來不合理時，要能分辨「程式真的這樣要求」與
	// 「我們讀錯了地方」——沒有這個就只能猜。
	PBSeg, PBOff uint16
	PBRaw        [8]byte

	// CallCS/CallIP 是呼叫端（INT 之後的下一道指令）。
	// 參數看起來不合理時要能直接跳去反組譯那裡。
	CallCS, CallIP uint16

	// Steps 是載入發生在第幾道指令。要看「載完之後發生什麼」就靠它
	// 把 -steps 停在正確的位置。
	Steps uint64

	// CallSite 是呼叫端 INT 指令前後的位元組（前 24、後 8）。
	//
	// 呼叫端常常是執行期搬到高位段的 stub，**檔案裡找不到**，
	// 事後也可能被覆蓋。要看它就得在呼叫發生的當下抄。
	CallSite [32]byte
}

// MemCall 是一次配置器呼叫（`AH=48h` 配置／`AH=49h` 釋放）的逐筆帳。
//
// 存在的理由是**總量對不對答不了「哪一次開始偏離」**。原版跑到
// `DATA5.GRP` 那一層時要 `BX=FFFF`（探測上限），拿到 122 KB 就收工；
// 當下它自己握著約 373 KB 而整趟一次都沒釋放。要判斷是「真 DOS 底下
// 它也拿這麼多」還是「我們給多了」，只能一筆一筆比對要求與回應。
type MemCall struct {
	Step   uint64
	Op     uint8  // 0x48 配置、0x49 釋放
	Want   uint16 // 要幾段（0x49 時無意義）
	Seg    uint16 // 成功時給出去的段；0x49 時是 ES
	Got    uint16 // 失敗時回報的最大自由段數
	OK     bool
	CS, IP uint16 // 呼叫端
	DS, ES uint16
}

// ResizeCall 是一次 `AH=4Ah` 的紀錄。
//
// Before／After 是這個區塊在 arena 裡調整前後的段數（`InArena` 為假時無意義）。
// **要求的大小不等於區塊最後的大小**：程式可以一路把同一塊撐大，
// 而只看 `AH=48h` 的要求量會漏掉這一段成長。
type ResizeCall struct {
	Seg, Want, FreeSeg uint16
	Before, After      uint16
	InArena, OK        bool
	CS, IP             uint16
}

// KeyRead 是一次「把按鍵從佇列取走」的紀錄。
//
// 存在的理由是**餵進去的鍵不見了的時候，看不出是誰吃的**。
// 同一個佇列有三條出口（`int 21h AH=01/07/08`、`AH=3Fh` 讀 handle 0、
// `int 16h AH=00/10`），而「程式沒反應」既可能是沒收到、
// 也可能是被另一條路提前取走。
type KeyRead struct {
	Step uint64
	Via  string // "int21-AH08"／"int21-3F"／"int16-AH00"
	Key  uint8
}

// CallRec 是一次 int 21h 的暫存器快照。
type CallRec struct {
	Step         uint64
	AH, AL       uint8
	ESIn, BXIn   uint16
	ESOut, BXOut uint16
}

// FileOp 是一次檔案操作。
type FileOp struct {
	Step   uint64
	Op     string // open／seek／read／write／close
	Handle uint16
	Name   string
	Arg    int64  // seek 的位移、read 的要求量
	Result int64  // seek 後的位置、read 實際讀到的量；<0 表示錯誤碼
}
