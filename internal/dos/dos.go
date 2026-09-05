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

// Mouse 是滑鼠狀態。座標用**像素**存，回報時才乘上 XScale
// （mode 13h 的標準驅動水平回報 0–639，`rich2/docs/re/182` §3）。
type Mouse struct {
	X, Y    uint16
	Buttons uint16
	// Press／Release 是 AX=5／AX=6 的統計，**逐鍵分開**（0 ＝ 左、1 ＝ 右、
	// 2 ＝ 中），讀走就歸零。
	//
	// ⚠ **`AX=5` 的 `BX` 是輸入（問哪一個鍵），不只是輸出。**
	// 不分鍵的話「按左鍵」會被問右鍵的那一次先領走——而《臥龍傳》的等待迴圈
	// （IDA `0x121E9`）**先問右鍵**，於是每一次左鍵都被讀成右鍵，
	// 畫面上看起來像「點了沒反應」或「點什麼都是取消」。
	Press, Release [3]uint16
	// PressAt／ReleaseAt 是該鍵最後一次按下／放開的座標。
	// `AX=5`／`AX=6` 回的是**那一刻**的位置，不是現在的位置。
	PressAt, ReleaseAt [3][2]uint16
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

	// Now 是固定時刻，Mouse 是滑鼠狀態，Font 是 DOS/V 字型服務。
	Now   Time
	Mouse Mouse
	Font  Font

	// Sound 記下 `int 61h`（音源 TSR）每個 command 被叫了幾次。
	// **這一輪不模擬音源**（`docs/spec/008` §6），只留下「走到了沒」。
	Sound map[uint8]int

	// MouseHandler 是遊戲用 `int 33h AX=000Ch` 裝的事件常式。
	// **記下來但不呼叫**；記的理由是診斷——出現「點了沒反應」時，
	// 第一件要問的事就是「遊戲是不是只等事件不輪詢」。
	MouseHandler struct {
		Seg, Off, Mask uint16
		Set            bool
	}

	// Console 收 `AH=02h`／`06h`／`09h` 與 `int 10h AH=0Eh` 印出來的字。
	// **錯誤訊息走這條**，收不到就等於什麼都不知道。
	Console []byte

	// Stdin 是 `AH=3Fh` 讀 handle 0 時要餵的位元組；空的時候餵 StdinFill。
	//
	// ⚠ **不能回「讀到 0 個」**（等同 EOF）：主程式會當成輸入結束、
	// 還原中斷向量然後 exit（`rich2/docs/re/005`「死點修掉了」）。
	Stdin     []byte
	StdinFill uint8

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
	nextHandle uint16
	freeSeg    uint16
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
		Font:          DefaultFont(),
		Sound:         map[uint8]int{},
		Drive:         2, // C:，見 Drive 欄位的說明
		Dir:           "RICH2",
		Unimplemented: map[Call]int{},
		handles:       map[uint16]*handle{},
		nextHandle:    5, // 0–4 是標準 handle
	}
}

// Install 把服務層掛到機器的中斷鉤子上。**要在 LoadEXE 之後叫**——
// 它會記下映像後面的第一個可配置段。
func (d *DOS) Install() {
	d.freeSeg = d.M.FreeSeg
	d.installFont()
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
	case 0x15:
		d.int15(c)
	case 0x1A:
		d.int1A(c)
	case 0x61:
		d.int61(c)
	case intFontFull:
		d.fontGlyph(c, true)
	case intFontHalf:
		d.fontGlyph(c, false)
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

// int1A 是 BIOS 的計時器／即時鐘服務。
//
// **時刻要與 `int 21h AH=2Ch` 同一份。** 兩份不一致時亂數種子會跟著飄，
// 而症狀是「同一組輸入跑出不同畫面」——那看起來像模擬器不穩定，
// 不像少接了一支服務。
func (d *DOS) int1A(c *cpu.CPU) {
	switch ah(c) {
	case 0x00: // 取計時器計數 → CX:DX，AL ＝ 有沒有跨過午夜
		ticks := d.M.Read16(0x0040*16+0x6C) | 0
		c.R[cpu.DX] = ticks
		c.R[cpu.CX] = d.M.Read16(0x0040*16 + 0x6E)
		setAL(c, 0)
	case 0x02: // 取 RTC 時刻 → CH:CL:DH（BCD）
		setCH(c, bcd(d.Now.Hour))
		setCL(c, bcd(d.Now.Min))
		setDH(c, bcd(d.Now.Sec))
		setDL(c, 0) // 沒有日光節約
		clearCarry(c)
	default:
		d.note(0x1A, ah(c), al(c))
	}
}

// int61 是松崗 DOS/V 版的音源 TSR（`YNSOUND.COM`）。
//
// **只記錄不模擬**（`docs/spec/008` §6）。對拍比的是畫面，
// 音訊 parity 在臥龍傳專案那邊用錄音比過。
func (d *DOS) int61(c *cpu.CPU) {
	d.Sound[ah(c)]++
	// AH=0Ah 回旗標。回 0 ＝ 沒有任何旗標，是安全的預設；
	// **但它要留在未實作清單裡**，不要安靜地變成「有旗標」。
	if ah(c) == 0x0A {
		d.note(0x61, 0x0A, al(c))
		setAL(c, 0)
	}
	clearCarry(c)
}

func bcd(v uint8) uint8 { return v/10<<4 | v%10 }

func setCH(c *cpu.CPU, v uint8) { c.R[cpu.CX] = c.R[cpu.CX]&0x00FF | uint16(v)<<8 }
func setCL(c *cpu.CPU, v uint8) { c.R[cpu.CX] = c.R[cpu.CX]&0xFF00 | uint16(v) }
func setDH(c *cpu.CPU, v uint8) { c.R[cpu.DX] = c.R[cpu.DX]&0x00FF | uint16(v)<<8 }
func setDL(c *cpu.CPU, v uint8) { c.R[cpu.DX] = c.R[cpu.DX]&0xFF00 | uint16(v) }
