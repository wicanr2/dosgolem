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
	"strings"

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

	// stubs 是「走到這個位址就換成我的回傳值」的替身表（`docs/spec/005` §5）。
	stubs map[uint32]func(*Oracle) uint32

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

// SetCmdLine 把命令列尾巴寫進最外層程式的 PSP（`PSP+80h`：長度、文字、CR）。
//
// EXEC 出來的子程式本來就會拿到參數，但最外層是 `Load` 直接載入的，
// 原本沒有地方放。`ENDING.EXE` 這種「靠參數決定要演哪一個結局」的程式
// 沒有它就只印一行字然後結束。
func (o *Oracle) SetCmdLine(tail string) {
	if len(tail) > 126 {
		tail = tail[:126]
	}
	base := uint32(machine.PSPSeg)*16 + 0x80
	o.m.Write8(base, uint8(len(tail)))
	for i := 0; i < len(tail); i++ {
		o.m.Write8(base+1+uint32(i), tail[i])
	}
	o.m.Write8(base+1+uint32(len(tail)), 0x0D)
}

// Close 關掉還開著的檔。
func (o *Oracle) Close() { o.d.Close() }

// ---- 程式鏈（`docs/spec/008`／`009`）--------------------------------------

// LoadProgram 是 Load 的通用版：依檔頭 MZ magic 分派 COM／EXE
// （**副檔名不是判準**，與 `cmd/probe` 一致），並把 args 寫進 PSP+80h
// 的命令列尾（長度 ＋ 內容 ＋ CR）。
//
// 源平合戰的啟動鏈第一支是 DOSJP.COM（要帶 `-F:font.dat -TJ`），
// Load 只會載 MZ，所以另開這個入口。
func LoadProgram(exe, root, args string) (*Oracle, error) {
	img, err := os.ReadFile(exe)
	if err != nil {
		return nil, err
	}
	m := machine.New()
	if len(img) >= 2 && img[0] == 'M' && img[1] == 'Z' {
		err = m.LoadEXE(img)
	} else {
		err = m.LoadCOM(img)
	}
	if err != nil {
		return nil, fmt.Errorf("載入 %s：%w", exe, err)
	}
	// 命令列尾（PSP+80h）。上限 126 bytes。
	tail := []byte(args)
	if len(tail) > 126 {
		tail = tail[:126]
	}
	psp := uint32(machine.PSPSeg) * 16
	m.Write8(psp+0x80, uint8(len(tail)))
	m.WriteBytes(psp+0x81, tail)
	m.Write8(psp+0x81+uint32(len(tail)), 0x0D)

	d := dos.New(m, root)
	d.Install()

	o := &Oracle{m: m, d: d, onCall: map[uint32][]func(*Oracle){}}
	o.idaOffset = 0x10000 - (uint32(machine.LoadSeg) * 16)
	o.dgroupSeg = uint16((0x41E90 - o.idaOffset) / 16)
	return o, nil
}

// Enqueue 排入監督佇列的一支程式：目前行程結束（或 TSR 常駐）後
// 由服務層推出來跑（`docs/spec/009` §4）。name 照 basename 解析，
// args 會寫進它的命令列尾。
func (o *Oracle) Enqueue(name, args string) { o.d.Enqueue(name, args) }

// ExecRecord 是一次 EXEC／監督載入的紀錄。
type ExecRecord = dos.ExecRecord

// ExecLog 回目前為止的 EXEC／監督載入紀錄——**「殼鏈走到哪一跳」
// 唯一的直接答案**。
func (o *Oracle) ExecLog() []ExecRecord {
	return append([]ExecRecord(nil), o.d.ExecLog...)
}

// AllowFileWrites 對可丟棄Root覆蓋層逐檔開啟實際寫入；預設仍完全唯讀。
func (o *Oracle) AllowFileWrites(names ...string) error { return o.d.AllowFileWrites(names...) }

// SetScratch 打開可寫的暫存層（`docs/spec/009`）：程式的存檔落在這個目錄，
// **原版素材目錄永遠不寫**。空字串＝關掉，寫入只記帳不落地。
//
// 要在程式開始存檔之前設。走到建角、存檔、讀檔那類流程一定要開——
// 不開的話寫入被吞掉，而症狀是「選單按了畫面完全沒變」，
// 完全不指向寫檔（`docs/spec/009` §1）。
func (o *Oracle) SetScratch(dir string) { o.d.Scratch = dir }

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
//
// ⚠ **預設基底是 rich2 專用的常數**（IDA 線性 0x41E90）。換一支執行檔就要先
// SetDGroup 或 SyncDGroupFromDS，否則讀到的是別的東西**而且不會報錯**——
// `DS()` 沒有任何辦法察覺基底錯了。不確定的話改用 `IDA()`，那個對每一支
// 執行檔都是從載入位置算出來的。
func (o *Oracle) DS(off uint16) Addr { return Addr{o.dgroupSeg, off} }

// Far 造一個明確的 段:偏移。
//
// **程式執行期自己配置的東西要用它**——那些不在 DGROUP 也不在映像裡，
// 位址得先從程式的變數讀出來（例：熱區圖的段與偏移）。
// 名字不叫 `At` 是因為那個已經是「CS:IP 走到這裡」的停止條件。
func Far(seg, off uint16) Addr { return Addr{seg, off} }

// DGroupSeg 回目前 `DS()` 用的基底段。
func (o *Oracle) DGroupSeg() uint16 { return o.dgroupSeg }

// SetDGroup 用 IDA 線性位址指定 DGROUP 的起點。
//
// 判準是**讀一個已知值**：拿內容已知的全域（版本字串、浮點常數）用 `DS()` 讀
// 出來比對。設完不驗等於沒設。
func (o *Oracle) SetDGroup(idaLinear uint32) {
	o.dgroupSeg = uint16((idaLinear - o.idaOffset) / 16)
}

// SyncDGroupFromDS 把基底改成**程式現在的 `DS`**。
//
// 用在跑到 `main` 之後：C 的啟動碼這時已經把 DS 設成 DGROUP，抄它比從筆記換算
// 可靠。⚠ 只在確定 DS 沒被暫時改掉時呼叫——遠指標操作、字串搬移與中斷處理常式
// 都可能讓 DS 短暫指向別處。
func (o *Oracle) SyncDGroupFromDS() { o.dgroupSeg = o.m.CPU.Seg[cpu.DS] }

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

// Indexed 回畫面的色號陣列。mode 13h 是 320×200（`Width`×`Height`）；
// 平面模式是 `ScreenSize()` 那個尺寸的 0–15 色號。
//
// **色號，不是 RGB。** rich2 的比對在色號空間做，而且逐點比對在索引空間
// 才不會被調色盤循環干擾（`docs/spec/005` §3.3）。
func (o *Oracle) Indexed() []uint8 { return o.m.Indexed() }

// ScreenSize 是目前模式的畫面尺寸。
//
// ⚠ **`Width`／`Height` 兩個常數是 mode 13h 的**，planar 模式下不對；
// 會遇到 16 色模式的程式一律問這一支。
func (o *Oracle) ScreenSize() (w, h int) { return o.m.VideoSize() }

// IndexedEGA 回 EGA mode 0Dh 的 320×200 色號陣列（`docs/spec/007`）。
//
// **和 `Indexed()` 是兩種定址，不是兩種格式**：`Indexed()` 讀 A0000 起的
// 線性 64000 bytes（mode 13h），這一支把四個位元平面組起來（mode 0Dh）。
// 拿錯的那一支去對拍會得到**一張看起來正常但錯的圖**，不會報錯。
func (o *Oracle) IndexedEGA() []uint8 { return o.m.IndexedEGA() }

// EGAPlanarActive 回報序列器的 Map Mask 曾被寫成不是 `0Fh` 的值，也就是
// 「這支程式在逐平面輸出」。
//
// **它是訊號不是事實**：《Pool of Radiance》從來沒呼叫 `int 10h AH=00`，
// BDA 的模式位元組一路是 `03h`，所以沒有比它更硬的依據
// （`docs/spec/007` §2.1）。要不要照它切換由呼叫端決定。
func (o *Oracle) EGAPlanarActive() bool { return o.m.EGAPlanarActive() }

// EGAPlane 回傳一個位元平面的複本（0..3），畫面不對時用來分辨是遮罩錯
// 還是組裝錯。
func (o *Oracle) EGAPlane(plane int) []uint8 { return o.m.EGAPlane(plane) }

// Palette 回 256×3 的 RGB。
func (o *Oracle) Palette() [256][3]uint8 { return o.m.Palette() }

// 文字模式畫面的形狀（mode 03h：80 欄 25 列，一格「字元 ＋ 屬性」兩個 byte）。
const (
	TextCols = 80
	TextRows = 25
	textSeg  = 0xB800
)

// TextScreen 回文字模式畫面，一列一個字串（右邊的空白已經修掉）。
//
// **`Indexed()` 讀的是圖形記憶體（A0000）。** 程式還在文字模式時那裡當然是全黑，
// 而「全黑」看起來就像「什麼都沒畫」——實際上畫面上可能寫滿了字。判斷載入進度
// 或找錯誤訊息時，先看 `VideoMode()`：還是 03h 就要讀這一份，不是 `Indexed()`。
func (o *Oracle) TextScreen() []string {
	out := make([]string, TextRows)
	for r := 0; r < TextRows; r++ {
		line := make([]byte, TextCols)
		for c := 0; c < TextCols; c++ {
			ch := o.m.Read8(cpu.Addr(textSeg, uint16(2*(r*TextCols+c))))
			if ch < 0x20 || ch > 0x7E {
				ch = ' ' // 制表符與高位字元用空白代替，方便肉眼比對
			}
			line[c] = ch
		}
		out[r] = strings.TrimRight(string(line), " ")
	}
	return out
}

// CGA 模式 4 的版面（320×200，四色，兩個 bit 一個像素）。
const (
	cgaSeg = 0xB800
	// cgaOddPlane 是奇數列那一半的位移。**掃描線是交錯的**：偶數列從 0 起、
	// 奇數列從 0x2000 起，各 8000 bytes。照線性讀會得到一張梳子。
	cgaOddPlane = 0x2000
	cgaStride   = 80 // 320 像素 × 2 bit ÷ 8
)

// CGA4 回 CGA 模式 4 的畫面，320×200 個色號（0…3）。
//
// **不是 `Indexed()`。** 那一支讀的是 mode 13h 的 `A0000`；CGA 的視訊記憶體在
// `B8000`，而且掃描線交錯——兩邊都對不上，所以 CGA 程式在 `Indexed()` 底下
// 永遠是全黑。判斷「畫了沒」要用這一支。
//
// 色號是**調色盤索引**，不是 RGB：模式 4 的四色由 `3D9` 埠選盤，這一層不解釋它，
// 比對在索引空間做（同 `Indexed()` 的理由，§3.3）。
func (o *Oracle) CGA4() []uint8 {
	out := make([]uint8, Width*Height)
	for y := 0; y < Height; y++ {
		base := uint32(y/2) * cgaStride
		if y%2 == 1 {
			base += cgaOddPlane
		}
		row := out[y*Width:]
		for x := 0; x < Width; x++ {
			b := o.m.Read8(cpu.Addr(cgaSeg, 0) + base + uint32(x/4))
			row[x] = (b >> (6 - 2*uint(x%4))) & 3
		}
	}
	return out
}

// Tandy16 回 Tandy／PCjr 模式 09h 的畫面，320×200 個色號（0…15）。
//
// 版面與 CGA 模式 4 同一個家族但**四段交錯**（CGA 是兩段）：一列 160 bytes、
// 四個 bit 一個像素，第 y 列在 `(y%4) × 0x2000 + (y/4) × 160`。
// 段數記錯的話畫面會變成四張交疊的梳子——**看起來像雜訊，不像「解錯了」**。
//
// 對 oracle 來說這個模式比 EGA 好用：視訊記憶體是線性的，不必模擬位元平面。
func (o *Oracle) Tandy16() []uint8 {
	const (
		stride = 160
		banks  = 4
		bank   = 0x2000
	)
	out := make([]uint8, Width*Height)
	for y := 0; y < Height; y++ {
		base := uint32(y%banks)*bank + uint32(y/banks)*stride
		row := out[y*Width:]
		for x := 0; x < Width; x++ {
			b := o.m.Read8(cpu.Addr(cgaSeg, 0) + base + uint32(x/2))
			if x%2 == 0 {
				row[x] = b >> 4
			} else {
				row[x] = b & 0x0F
			}
		}
	}
	return out
}

// Steps 是已經執行的指令數，Opened 是開過的檔（依序）。
func (o *Oracle) Steps() uint64                     { return o.m.Steps }

// Frames 是**螢幕刷新了幾次**（VGA 垂直回掃次數，`docs/spec/187`）。
//
// **這是通用的幀**：任何 DOS 程式都有它，不必知道那支程式在哪裡畫完一幀。
// DOSBox 的 `VGA_VerticalTimer` 是同一個東西（`src/hardware/vga_draw.cpp`）。
//
// 它由指令數驅動（`Machine.VGAFrameEvery`），不是真實時間，所以是決定性的。
func (o *Oracle) Frames() uint64 { return o.m.Frames }

// TimerHz 回**程式自己把 PIT 設成幾 Hz**。
//
// 8253／8254 的輸入時脈是固定的 `machine.PITBaseHz`（1,193,181.82 Hz），
// 程式寫進去的是除數，所以頻率 ＝ 基頻 ÷ 除數。BIOS 開機是 65,536
// （18.2 Hz），程式要更細的時基就自己重寫——**不要把某一支遊戲的頻率
// 寫死在呼叫端**：《大富翁2》是 17,000（70.187 Hz），《臥龍傳》的
// `YNSOUND.COM` 是 4,096（291.3 Hz）。
//
// 這個值是「程式要求多快」，不是 dosgolem 的時間流速——後者由
// `machine.IRQ0Every` 的指令數決定。要換算就配 `StepsPerTick`。
func (o *Oracle) TimerHz() float64 { return o.m.PITHz() }

// TimerDivisor 回 PIT 通道 0 目前的除數。
func (o *Oracle) TimerDivisor() uint32 { return o.m.PITDivisor() }

// TimerProgrammed 回報程式有沒有自己設過 PIT 通道 0。
//
// **要分得出「量到 18.2 Hz」與「沒被設過所以是預設值」**：數字一樣，
// 但一個是結論、一個是「還沒發生」。
func (o *Oracle) TimerProgrammed() bool { return o.m.PITProgrammed() }

// StepsPerTick 回一個計時器刻等於幾道指令。
//
// dosgolem 的時間由指令數驅動，所以「幾刻」與「幾道指令」的換算走這裡。
//
// ⚠ 它回的是**機器現在跑的**間隔（`machine.IRQ0Every`），不是程式要求的
// 那個頻率換算出來的值。兩者在《大富翁2》上相同（除數 17,000 就是
// `DefaultIRQ0Every` 的基準），別的程式不見得——要程式要求的那個值，
// 用 `machine.PITStepsPerTick`（`docs/spec/188-pit-divisor-decoding` §4）。
func (o *Oracle) StepsPerTick() uint64 { return o.m.IRQ0Every }

// OnFrame 在每一次垂直回掃時呼叫 f。nil 取消。
//
// **錄影要用這個**：回呼時的畫面就是這一幀要顯示的內容，
// 逐幀存下來就是螢幕實際輸出的序列。
//
// ⚠ 與 `apps/rich2.EachFrame` 不是同一種幀：那一支是**動畫幀**
// （原版一個像素幀 ≈ 4 次螢幕刷新），這一支是**螢幕幀**。
func (o *Oracle) OnFrame(f func(*Oracle)) {
	if f == nil {
		o.m.SetOnFrame(nil)
		return
	}
	o.m.SetOnFrame(func() { f(o) })
}

// OnTick 在每一次送出計時器中斷時呼叫 f。nil 取消。
//
// 回呼點在中斷送出**之前**，所以這時的畫面是上一幀畫完的結果——
// 逐幀取畫面要的正是這個時刻。
func (o *Oracle) OnTick(f func(*Oracle)) {
	if f == nil {
		o.m.SetOnTick(nil)
		return
	}
	o.m.SetOnTick(func() { f(o) })
}
func (o *Oracle) Opened() []string                  { return o.d.Opened }
func (o *Oracle) Missing() []string                 { return o.d.Missing }
func (o *Oracle) Wrote() []dos.Write                { return o.d.Wrote }
func (o *Oracle) MissingAccesses() []dos.FileAccess { return o.d.MissingAccess }

// OnFileOpen 在每一次成功開檔之後叫一次（參數是檔名，不含路徑）。
//
// **這是「把一段執行期產物框到某個檔上」唯一的錨。**
// 音樂對拍就是這樣用的：開機會連放好幾首而且暫存器串是接起來的，
// 事後從序列裡找分界只能用猜的；掛在開檔那一刻，
// `ClearOPL()` 就能框出「只有這一首」。
//
// 傳 nil 取消。
func (o *Oracle) OnFileOpen(fn func(name string)) { o.d.OnOpen = fn }

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

// MemOp 是一次記憶體服務（`MemOps`）。
type MemOp = dos.MemOp

// MemOps 是每一次 `AH=48h`／`49h`／`4Ah` 與 EXEC 的配置紀錄，依序。
//
// **配置器把程式自己佔著的段配出去時，症狀是程式碼被自己寫壞。** 那看起來像
// 模擬器把記憶體寫爛了，實際上是 `alloc` 回了落在映像裡的段。
func (o *Oracle) MemOps() []MemOp { return o.d.MemOps }

// VideoMode 是目前的 BIOS 視訊模式（`int 10h AH=00h` 設的那個）。
//
// **載入進度的路標**：還停在 03h 就表示程式連圖形模式都還沒切進去。
func (o *Oracle) VideoMode() uint8 { return o.m.VideoMode() }

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

// PortReads 回每個I/O埠被讀取的次數副本。
// 這只供診斷直接硬體輪詢，不把埠號自行解釋成裝置語意。
func (o *Oracle) PortReads() map[uint16]uint64 {
	out := make(map[uint16]uint64, len(o.m.PortsIn))
	for port, count := range o.m.PortsIn {
		out[port] = count
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

// ── OPL2（AdLib）───────────────────────────────────────────────────

// AdLib 決定偵測時要不要讓 OPL2 存在。**要在 Run 之前叫。**
//
// ⚠ **預設不存在**：偵測失敗，整段音樂路徑被跳過，開機因此快很多。
// 要做音樂對拍就打開它——打開之後 `OPLWrites` 才會有東西。
func (o *Oracle) AdLib(present bool) { o.m.SetAdLib(present) }

// OPLWrite 是一次 OPL2 暫存器寫入。
type OPLWrite = machine.OPLWrite

// OPLWrites 回目前為止的 OPL2 暫存器寫入序列。
//
// **這是音樂 parity 的對拍對象。** 0x388 選暫存器、0x389 寫值，兩個埠是
// 一組；這裡已經配好對。暫存器串是決定性的、可以逐筆比，
// 而波形要逐樣本一致屬於既定停止線（`rich2/docs/spec/049`）。
func (o *Oracle) OPLWrites() []OPLWrite { return o.m.OPL }

// OPLRegs 回某一組 OPL 暫存器的**目前狀態**（256 bytes）。
//
// bank 0 ＝ 0x388/0x389（OPL2 相容），1 ＝ 0x38A/0x38B（OPL3 第二組）。
//
// **序列與狀態問的是不同的問題**：`OPLWrites` 看得出「怎麼做的」，
// 這一份看得出「現在是什麼」。要對「某一刻的音色」就用這一份——
// 它不受「多寫了一次同樣的值」或「寫入順序不同」影響，
// 那兩件事在序列上會分岔、在狀態上不會。
func (o *Oracle) OPLRegs(bank int) [256]uint8 { return o.m.OPLRegs(bank) }

// ClearOPL 清掉暫存器寫入序列，**但不動暫存器檔與計時器狀態**。
//
// 用來框出「只有這一段」的寫入：走到某個畫面之後清一次，
// 之後收到的就只有那一段。
//
// ⚠ **不要為了「乾淨」連狀態一起清**：驅動程式開機時把一個預設音色灌進
// 全部 18 個 operator，之後的曲子是**疊在那個狀態上**的。狀態清掉之後
// 收到的序列會與原版的實際情況不同，而它看起來完全正常。
func (o *Oracle) ClearOPL() { o.m.ClearOPL() }

// WatchWrites 監看一段 DGROUP 偏移的寫入，回一份逐次紀錄。
//
// **這是「誰寫這個變數」唯一直接的答案。** 靜態 xref 只涵蓋直接參考
// （`mov ds:XXXXh, ax`）；`mov [si+456h], ax` 這種間接寫入抓不到，
// 而「掃不到寫入端」的變數多半就是這樣寫的。
//
// lo／hi 是 DGROUP 偏移（含端點）。回傳的 slice 會隨執行成長。
func (o *Oracle) WatchWrites(lo, hi uint16) *[]MemWrite {
	log := &[]MemWrite{}
	base := o.DS(0).Linear()
	o.m.WatchWrites(base+uint32(lo), base+uint32(hi),
		func(a uint32, old, nw uint8) {
			*log = append(*log, MemWrite{
				Off:  uint16(a - base),
				Old:  old,
				New:  nw,
				IP:   o.IP(),
				Step: o.Steps(),
			})
		})
	return log
}

// WatchLinear 監看**任意線性位址區間**的寫入，回一份逐次紀錄。
//
// StopWatchingWrites 關掉監看。
func (o *Oracle) StopWatchingWrites() { o.m.WatchWrites(0, 0, nil) }

// MemWrite 是一次寫入。
//
// ⚠ **IP 是「寫的那一刻的 CS:IP」，也就是那道指令本身**，
// 不是它的呼叫端。要找呼叫端就拿這個位址去反組譯它前後幾行。
type MemWrite struct {
	Off      uint16 // DGROUP 偏移
	Old, New uint8
	IP       Addr
	Step     uint64
}

// FileOp 是原版的一次檔案操作。
type FileOp struct {
	Step   uint64
	Op     string // open／seek／read／write／close
	Fn     uint8  // 對應的 int 21h AH
	Handle uint16
	Name   string
	Arg    int64 // seek 的位移、read 的要求量、open 的檔案大小
	Pos    int64 // seek 後的位置、read 的起點
	Len    int   // read／write 實際的位元組數
}

// TraceFiles 過去用來打開檔案操作追蹤；**現在永遠開著**，留著只為相容。
//
// **開檔清單只說「開過什麼」，說不出「要求讀哪一段、拿到多少」。**
// 對拍容器格式要問的正是後者：原版 seek 到哪個位移、讀多長——
// 那組數字就是它自己算出來的項目邊界，拿來對 remake 的解碼器
// 比「兩條路徑算同一件事」。
func (o *Oracle) TraceFiles() {}

// FileOps 回傳目前為止的檔案操作。
func (o *Oracle) FileOps() []FileOp {
	out := make([]FileOp, 0, len(o.d.FileOps))
	for _, f := range o.d.FileOps {
		out = append(out, FileOp{
			Step: f.Step, Op: f.Op, Fn: f.Fn, Handle: f.Handle,
			Name: f.Name, Arg: f.Arg, Pos: f.Pos, Len: f.Len,
		})
	}
	return out
}

// MemCall 是原版的一次配置器呼叫。
type MemCall struct {
	Step   uint64
	Op     uint8  // 0x48 配置、0x49 釋放
	Want   uint16 // 要幾段
	Seg    uint16 // 給出去的段（0x49 是被釋放的段）
	Got    uint16 // 成功時是實得段數，失敗時是回報的最大自由段數
	OK     bool
	CS, IP uint16
}

// TraceMem 打開配置器追蹤。要在 Run 之前呼叫。
//
// **「總共佔了多少」答不出「哪一次開始偏離」。** 要判斷我們的配置器
// 是不是比真 DOS 大方，得逐筆看每一次要多少、給多少、誰要的。
func (o *Oracle) TraceMem() { o.d.MemTrace = []dos.MemCall{} }

// MemCalls 回傳目前為止的配置器呼叫。
func (o *Oracle) MemCalls() []MemCall {
	out := make([]MemCall, 0, len(o.d.MemTrace))
	for _, m := range o.d.MemTrace {
		out = append(out, MemCall{
			Step: m.Step, Op: m.Op, Want: m.Want, Seg: m.Seg,
			Got: m.Got, OK: m.OK, CS: m.CS, IP: m.IP,
		})
	}
	return out
}
