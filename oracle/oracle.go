// Package oracle 是 dosgolem 對外的契約：把原版《大富翁2》跑在同一個 Go
// 行程裡，讓對拍從「隔著 X 看畫面」變成「直接讀原版的記憶體」。
//
// 規格在 `docs/spec/005-oracle-api.md`（READY）。
//
//	o, err := oracle.Load(exe, root)
//	o.RunUntil(oracle.PasswordScreen())
//	o.Click(102, 125)
//	shot := o.Indexed()
//
// ⚠ **本套件不含任何原版檔案。** `exe` 與 `root` 都是玩家自備的路徑。
//
// # 為什麼不直接用 internal/
//
// `internal/` 是 Go 的可見性邊界，只有 dosgolem 自己 import 得到。
// 這一層是給別的 module（`rich2`）用的契約——**窄一點**，
// 外面看得到的每個型別以後都不能隨便改。
package oracle

import (
	"fmt"
	"math"
	"os"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 畫面尺寸。**這兩個是 mode 13h 的**（320×200，256 色）。
// 平面模式（mode 0Dh–12h）的尺寸跟著視訊模式走，用 Screen() 問。
const (
	Width  = machine.VideoWidth
	Height = machine.VideoHigh
)

// DefaultBudget 是 RunUntil 的預設指令數上限（約 5 秒）。
//
// **一定要有上限。** 跑不到條件而靜靜地回來的話，「條件寫錯」與
// 「程式真的沒走到」長得一模一樣。
const DefaultBudget = 100_000_000

// DefaultHold 是 Click 按住的指令數。
//
// **不能短。** 遊戲輪詢 `int 33h` 的頻率很低，按下與放開隔太近會整個被
// 跳過——DOSBox 那邊同一題要點三次才生效一次
// （`rich2/docs/playtest/001` §5.6）。
const DefaultHold = 2_000_000

// DefaultHover 是 Click 在「移到位置」與「按下」之間等的指令數。
//
// **不能是 0。** 頂端按鈕列是 hover-based：游標移過去先反白，反白之後的
// 點擊才算數。沒有間隔的話按鈕只會反白不會執行，而那看起來像
// 「點到了但遊戲不理」——畫面確實有反應。
//
// rich2 的 DOSBox 腳本用 `sleep 0.4` 做同一件事，以 3,000 cycles/ms 換算
// 約 120 萬道指令；這裡取 200 萬。
const DefaultHover = 2_000_000

// Oracle 是一台跑著原版的機器。用 Load 造。
//
// **不是 goroutine-safe**：一台機器一條時間線。要平行就開多台，
// 或用 Save／Restore 從同一個狀態展開。
type Oracle struct {
	m *machine.Machine
	d *dos.DOS

	// idaOffset 是執行期線性位址與 IDA 線性位址之差（§3.1）。
	// **算出來的，不是常數**——換載入位置就會變，而錯了不會報錯。
	idaOffset uint32
	dgroupSeg uint16

	onCall map[uint32][]func(*Oracle)

	// scratch 是 screen() 的重用緩衝。**條件函式會反覆呼叫它**，
	// 每次配置 150 KB 的話跑幾千萬道指令就慢到不能用。
	scratch []uint8
}

// Options 是「這一支 binary 長什麼樣」。
//
// **零值就是通用預設**，只有真的與眾不同才要填。
// 分層的判準見 `docs/spec/006`：位址屬於程式層，不該寫死在這裡。
type Options struct {
	// DGROUP 是 `ds:` 的 IDA 線性基底。0 表示「DS 與程式映像同段」，
	// 也就是 IDA 線性 0x10000——大部分組語寫成的 DOS 程式都是這樣。
	//
	// ⚠ 編譯後 BASIC 的 DGROUP 另有其處（rich2 是 0x41E90），
	// **拿錯的值不會報錯**，只會讀出一片看起來像資料的東西。
	DGROUP uint32

	// MouseXScale 是 `int 33h` 水平座標的倍率。
	// mode 13h 的標準驅動回報 0–639 對 320 個像素，所以是 2；
	// 640 寬的平面模式是 1。**0 當成 2**（沿用既有預設）。
	MouseXScale uint16

	// FontFull／FontHalf 是 DOS/V 字型服務要讀的全形／半形字模檔
	// （`docs/spec/008` §3）。空字串沿用預設。
	//
	// 做成參數的理由不只是彈性：《臥龍傳》的常駐服務 `STR.EXE` 寫死的
	// 檔名與封裝內的資料檔對不上（那邊的 `docs/re/29` §6 至今未裁決），
	// 換個檔名跑一次就是一個可執行的實驗。
	FontFull, FontHalf string
}

// Load 載入原版執行檔。
//
//	exe  ── RUN_full.EXE（兩層都解開的那一個，見 rich2/CLAUDE.md §4.1）
//	root ── 原版素材目錄（.PAK／.PIX／.RIX 那些）
func Load(exe, root string) (*Oracle, error) {
	return LoadWith(exe, root, Options{})
}

// LoadWith 是帶設定的 Load。
func LoadWith(exe, root string, opt Options) (*Oracle, error) {
	img, err := os.ReadFile(exe)
	if err != nil {
		return nil, err
	}
	m := machine.New()
	if err := m.LoadEXE(img); err != nil {
		return nil, fmt.Errorf("載入 %s：%w", exe, err)
	}
	d := dos.New(m, root)
	if opt.MouseXScale != 0 {
		d.Mouse.XScale = opt.MouseXScale
	}
	if opt.FontFull != "" {
		d.Font.Full = opt.FontFull
	}
	if opt.FontHalf != "" {
		d.Font.Half = opt.FontHalf
	}
	d.Install()

	o := &Oracle{m: m, d: d, onCall: map[uint32][]func(*Oracle){}}
	// IDA 線性位址 ＝ 執行期線性 ＋ idaOffset。
	// IDA 把 DOS 的 MZ 映像載在段 0x1000，也就是線性 0x10000；
	// 我們載在 machine.LoadSeg。**這一條與程式無關**，兩個案例都成立。
	o.idaOffset = 0x10000 - (uint32(machine.LoadSeg) * 16)
	dgroup := opt.DGROUP
	if dgroup == 0 {
		dgroup = 0x10000 // DS 與映像同段
	}
	o.dgroupSeg = uint16((dgroup - o.idaOffset) / 16)
	return o, nil
}

// Close 關掉還開著的檔。
func (o *Oracle) Close() { o.d.Close() }

// ---- 位址 ----------------------------------------------------------------

// Addr 是一個執行期位址。用 DS／IDA／At 造，不要自己填。
type Addr struct{ Seg, Off uint16 }

func (a Addr) String() string { return fmt.Sprintf("%04X:%04X", a.Seg, a.Off) }

// Linear 是 20 位元線性位址。
func (a Addr) Linear() uint32 { return cpu.Addr(a.Seg, a.Off) }

// DS 把 rich2 筆記裡的 `ds:XXXX` 轉成執行期位址。
//
// DGROUP 與堆疊同段（編譯後 BASIC 的慣例）；驗證見 `docs/spec/005` §3.1
// ——`DS(0x1B5A)` 讀到 IEEE 754 的 1.0f。
func (o *Oracle) DS(off uint16) Addr { return Addr{o.dgroupSeg, off} }

// Far 造一個明確的 段:偏移。
//
// **程式執行期自己配置的東西要用它**——那些不在 DGROUP 也不在映像裡，
// 位址得先從程式的變數讀出來（例：熱區圖的段與偏移）。
// 名字不叫 `At` 是因為那個已經是「CS:IP 走到這裡」的停止條件。
func Far(seg, off uint16) Addr { return Addr{seg, off} }

// IDA 把 rich2 筆記裡的五位線性位址轉成執行期位址。
//
// **rich2 的每一份 RE 筆記都用這種位址**，所以這是最常用的入口。
func (o *Oracle) IDA(linear uint32) Addr {
	run := linear - o.idaOffset
	return Addr{uint16(run >> 4), uint16(run & 0xF)}
}

// IDAIn 把 IDA 線性位址換成**指定段**底下的偏移。
//
// ⚠ **`IDA()` 回的是正規化的段**（`線性 >> 4`），拿它當 CS 跳過去，
// 常式裡每一個 `cs:xxxx` 的絕對定址就全部偏掉——而那**不會報錯**，
// 讀到的是另一塊記憶體。要跳進原版的程式碼一律用這一支，
// 段給程式自己現在的 CS。
func (o *Oracle) IDAIn(seg uint16, linear uint32) Addr {
	return Addr{seg, uint16(linear - o.idaOffset - uint32(seg)*16)}
}

// ToIDA 把執行期位址換回 IDA 線性位址（除錯訊息要用）。
func (o *Oracle) ToIDA(a Addr) uint32 { return a.Linear() + o.idaOffset }

// ---- 觀測 ----------------------------------------------------------------

// Byte／Word 讀原版的變數。
func (o *Oracle) Byte(a Addr) uint8  { return o.m.Read8(a.Linear()) }
func (o *Oracle) Word(a Addr) uint16 { return o.m.Read16(a.Linear()) }

// Bytes 讀一段。
func (o *Oracle) Bytes(a Addr, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = o.m.Read8(a.Linear() + uint32(i))
	}
	return out
}

// Float 讀一個 IEEE 754 單精度。**這個 binary 的浮點是 IEEE 不是 MBF**
// ——它走自帶的 Microsoft 浮點模擬器（`INT 34h`–`3Dh`），格式是 IEEE。
func (o *Oracle) Float(a Addr) float32 {
	bits := uint32(o.Word(a)) | uint32(o.Word(Addr{a.Seg, a.Off + 2}))<<16
	return math.Float32frombits(bits)
}

// video 是視訊記憶體的**直接切片，不複製**。
//
// 只給內部的條件判斷用。`Indexed()` 會複製一份給呼叫端——
// 那是對的，但**放進每道指令都跑的迴圈裡就是災難**：
// 64 KB 的配置加比對乘上四千兩百萬道指令。
func (o *Oracle) video() []uint8 {
	if w, h := o.m.PlanarSize(); w != 0 {
		// 平面模式：比四個平面的原始 bytes。
		// **不必攤成像素**——條件只問「畫面變了沒」，而攤開 30 萬個像素
		// 要多花十倍時間做同一個判斷。
		n := w * h / 8
		if cap(o.scratch) < n*4 {
			o.scratch = make([]uint8, n*4)
		}
		buf := o.scratch[:n*4]
		for p := 0; p < 4; p++ {
			copy(buf[p*n:], o.m.VGA.Planes[p][:n])
		}
		return buf
	}
	base := machine.VideoSeg * 16
	return o.m.Mem[base : base+Width*Height]
}

// Screen 回目前畫面：寬、高、每點一個色號。
//
// **平面模式回 4 bit 的像素值（0–15），mode 13h 回 8 bit 的 DAC 索引。**
// 兩者都是「色號」但不是同一個空間——要 RGB 就用 ScreenRGB。
func (o *Oracle) Screen() (w, h int, px []uint8) {
	if w, h, px := o.m.Planar(); px != nil {
		return w, h, px
	}
	return Width, Height, o.m.Indexed()
}

// ScreenRGB 回目前畫面，每點三個 byte。逐點對拍用這一支。
func (o *Oracle) ScreenRGB() (w, h int, rgb []uint8) {
	if w, h, rgb := o.m.PlanarRGB(); rgb != nil {
		return w, h, rgb
	}
	px, pal := o.m.Indexed(), o.Palette()
	rgb = make([]uint8, len(px)*3)
	for i, p := range px {
		rgb[i*3], rgb[i*3+1], rgb[i*3+2] = pal[p][0], pal[p][1], pal[p][2]
	}
	return Width, Height, rgb
}

// Indexed 回 320×200 的色號陣列。
//
// **色號，不是 RGB。** rich2 的比對在色號空間做，而且逐點比對在索引空間
// 才不會被調色盤循環干擾（`docs/spec/005` §3.3）。
func (o *Oracle) Indexed() []uint8 { return o.m.Indexed() }

// Palette 回 256×3 的 RGB。
func (o *Oracle) Palette() [256][3]uint8 { return o.m.Palette() }

// Steps 是已經執行的指令數，Opened 是開過的檔（依序）。
func (o *Oracle) Steps() uint64   { return o.m.Steps }
func (o *Oracle) Opened() []string { return o.d.Opened }

// Console 是程式印出來的東西（`int 21h AH=02h/06h/09h/40h` 與
// `int 10h AH=0Eh`）。**錯誤訊息走這條**，出問題先看它。
func (o *Oracle) Console() string { return string(o.d.Console) }

// FontStats 回字型常式被叫了幾次（全形、半形）與讀不到字模的次數。
//
// **「沒畫字」與「畫了但看不見」的畫面一樣空**，所以要先問常式被叫了幾次。
// 讀不到字模的次數 > 0 表示檔名或索引式有問題——那會畫出**空白**，
// 而空白在畫面上就只是「這裡沒有字」。
func (o *Oracle) FontStats() (full, half, missing int) {
	return o.d.Font.Calls[0], o.d.Font.Calls[1], o.d.Font.Missing
}

// Unimplemented 是沒實作的服務被叫了幾次，次數多的在前。
//
// **收工前看一眼。**「跑得動」與「跑得動但行為不對」的差別在這裡。
func (o *Oracle) Unimplemented() []string { return o.d.UnimplementedReport() }

// CPU 狀態，寫診斷訊息用。
func (o *Oracle) IP() Addr { return Addr{o.m.CPU.Seg[cpu.CS], o.m.CPU.IP} }

// MouseActivity 回「滑鼠被輪詢幾次、其中回報按著幾次」。
//
// **這是分辨「輸入沒送到」與「送到了但遊戲不接受」的唯一辦法**
// ——兩者的畫面表現一模一樣（`rich2/docs/re/005` 那一輪查了整整一天）。
func (o *Oracle) MouseActivity() (polls, pressed int) {
	for _, p := range o.d.Mouse.Polls {
		polls++
		if p.Buttons != 0 {
			pressed++
		}
	}
	return
}

// MouseCalls 回每個 `int 33h` 功能號被叫了幾次。
//
// **診斷「點了沒反應」的第一步**：先確認遊戲在讀哪一支。
// 讀 `AX=3`（即時狀態）與讀 `AX=5/6`（按下／放開的統計）要餵的東西不一樣。
func (o *Oracle) MouseCalls() map[uint16]int {
	out := map[uint16]int{}
	for k, v := range o.d.Mouse.Calls {
		out[k] = v
	}
	return out
}

// MouseSets 回程式每一次自己設游標位置（`AX=4`）的座標與時間點。
//
// **程式設過之後我們注入的位置就被蓋掉了**，而症狀是「點了沒反應」
// ——遊戲在按著的那一刻讀到的是它自己設的座標，不是我們要點的地方。
func (o *Oracle) MouseSets() []struct {
	X, Y uint16
	Step uint64
} {
	out := make([]struct {
		X, Y uint16
		Step uint64
	}, 0, len(o.d.Mouse.Sets))
	for _, s := range o.d.Mouse.Sets {
		out = append(out, struct {
			X, Y uint16
			Step uint64
		}{s.X, s.Y, s.Step})
	}
	return out
}

// LastPressedPoll 回最後一次「回報按著」的座標與當時的指令數。
func (o *Oracle) LastPressedPoll() (x, y int, step uint64, ok bool) {
	for i := len(o.d.Mouse.Polls) - 1; i >= 0; i-- {
		if p := o.d.Mouse.Polls[i]; p.Buttons != 0 {
			return int(p.X), int(p.Y), p.Step, true
		}
	}
	return 0, 0, 0, false
}
