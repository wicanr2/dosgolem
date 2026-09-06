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
	"sort"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 畫面尺寸。mode 13h 固定 320×200，256 色。
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
	stubs  map[uint32]func(*Oracle) uint32
}

// Load 載入原版執行檔。
//
//	exe  ── RUN_full.EXE（兩層都解開的那一個，見 rich2/CLAUDE.md §4.1）
//	root ── 原版素材目錄（.PAK／.PIX／.RIX 那些）
func Load(exe, root string) (*Oracle, error) {
	img, err := os.ReadFile(exe)
	if err != nil {
		return nil, err
	}
	m := machine.New()
	if err := m.LoadEXE(img); err != nil {
		return nil, fmt.Errorf("載入 %s：%w", exe, err)
	}
	d := dos.New(m, root)
	d.Install()

	o := &Oracle{m: m, d: d, onCall: map[uint32][]func(*Oracle){}}
	// IDA 線性位址 ＝ 執行期線性 ＋ idaOffset。
	// 由映像的載入位置反推：IDA 那邊的程式碼從線性 10000 開始，
	// 對應檔案位移 4100（`rich2/CLAUDE.md` §4.1：線性 ＝ 檔案位移 ＋ BF00）。
	o.idaOffset = 0x10000 - (uint32(machine.LoadSeg) * 16)
	// DGROUP：`ds:` 的 IDA 線性基底是 41E90。
	o.dgroupSeg = uint16((0x41E90 - o.idaOffset) / 16)
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
//
// ⚠ **預設基底是 rich2 專用的常數**（IDA 線性 0x41E90）。換一支執行檔就要先
// SetDGroup 或 SyncDGroupFromDS，否則讀到的是別的東西**而且不會報錯**——
// `DS()` 沒有任何辦法察覺基底錯了。不確定的話改用 `IDA()`，那個對每一支
// 執行檔都是從載入位置算出來的。
func (o *Oracle) DS(off uint16) Addr { return Addr{o.dgroupSeg, off} }

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
	base := machine.VideoSeg * 16
	return o.m.Mem[base : base+Width*Height]
}

// Indexed 回 320×200 的色號陣列。
//
// **色號，不是 RGB。** rich2 的比對在色號空間做，而且逐點比對在索引空間
// 才不會被調色盤循環干擾（`docs/spec/005` §3.3）。
func (o *Oracle) Indexed() []uint8 { return o.m.Indexed() }

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
func (o *Oracle) Steps() uint64    { return o.m.Steps }
func (o *Oracle) Opened() []string { return o.d.Opened }

// Console 是程式印出來的東西（`int 21h AH=02h/06h/09h/40h` 與
// `int 10h AH=0Eh`）。**錯誤訊息走這條**，出問題先看它。
func (o *Oracle) Console() string { return string(o.d.Console) }

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

// FileOp 是一次 seek 或 read（`FileOps`）。
type FileOp = dos.FileOp

// FileOps 是每一次 `AH=42h` seek 與 `AH=3Fh` 讀的參數，依序。
//
// **「開了哪些檔」不夠。** 遊戲把一個檔當成資料庫用時，讀對檔案卻 seek 到錯的
// 位置，症狀是「解出來的東西是垃圾」——而 `Opened()` 那一份看起來完全正常。
func (o *Oracle) FileOps() []FileOp { return o.d.Reads }

// Missing 是開不起來的檔名，依序。
//
// **程式多半不檢查開檔結果**，所以少一個檔的症狀是它在很後面的地方跑進沒有映射
// 的記憶體——看起來像模擬器壞了。跑不動先看這一份。
func (o *Oracle) Missing() []string { return o.d.Missing }

// VideoMode 是目前的 BIOS 視訊模式（`int 10h AH=00h` 設的那個）。
//
// **載入進度的路標**：還停在 03h 就表示程式連圖形模式都還沒切進去。
func (o *Oracle) VideoMode() uint8 { return o.m.VideoMode() }

// CPU 狀態，寫診斷訊息用。
func (o *Oracle) IP() Addr { return Addr{o.m.CPU.Seg[cpu.CS], o.m.CPU.IP} }

// Registers 是一份暫存器快照。
type Registers struct {
	AX, CX, DX, BX, SP, BP, SI, DI uint16
	ES, CS, SS, DS                 uint16
	IP, Flags                      uint16
}

func (r Registers) String() string {
	return fmt.Sprintf(
		"AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X BP=%04X SP=%04X "+
			"CS=%04X DS=%04X ES=%04X SS=%04X IP=%04X FL=%04X",
		r.AX, r.BX, r.CX, r.DX, r.SI, r.DI, r.BP, r.SP,
		r.CS, r.DS, r.ES, r.SS, r.IP, r.Flags)
}

// Regs 回目前的暫存器。
//
// **診斷「寫到哪裡去了」的關鍵。** 一個搬移或清空迴圈寫錯地方時，錯的通常是
// 段暫存器（`ES` 指向 0 段就會把中斷向量表清掉），而那從指令本身看不出來。
func (o *Oracle) Regs() Registers {
	c := o.m.CPU
	return Registers{
		AX: c.R[cpu.AX], CX: c.R[cpu.CX], DX: c.R[cpu.DX], BX: c.R[cpu.BX],
		SP: c.R[cpu.SP], BP: c.R[cpu.BP], SI: c.R[cpu.SI], DI: c.R[cpu.DI],
		ES: c.Seg[cpu.ES], CS: c.Seg[cpu.CS], SS: c.Seg[cpu.SS], DS: c.Seg[cpu.DS],
		IP: c.IP, Flags: c.Flags,
	}
}

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

// PortReads 是每個 I/O 埠被讀了幾次，次數多的在前。
//
// **「程式在等什麼」最直接的答案。** 卡在一個緊密迴圈時，看它輪詢哪個埠就知道
// 它在等誰：`3DA` 是 CRT 狀態（等垂直回掃）、`60`／`64` 是鍵盤、`40`–`43` 是計時器、
// `388` 是 OPL。輪詢次數很大是正常的，那正是「在等」的樣子。
func (o *Oracle) PortReads() []PortCount {
	out := make([]PortCount, 0, len(o.m.PortsIn))
	for p, n := range o.m.PortsIn {
		out = append(out, PortCount{Port: p, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Port < out[j].Port
	})
	return out
}

// PortWrites 是程式寫過的每個 I/O 埠與最後寫進去的值，埠號小的在前。
//
// **配合 `PortReads` 一起看。** 讀是「在等什麼」，寫是「設了什麼」——EGA／VGA
// 直接設模式走 `3C0`–`3CF` 與 `3D4`／`3D5`，完全不經過 `int 10h`，所以
// `VideoMode()` 會一直回 03h 而畫面其實已經是圖形模式了。
func (o *Oracle) PortWrites() []PortValue {
	out := make([]PortValue, 0, len(o.m.Ports))
	for p, v := range o.m.Ports {
		out = append(out, PortValue{Port: p, Last: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

// PortValue 是一個埠與最後寫進去的值。
type PortValue struct {
	Port uint16
	Last uint8
}

func (p PortValue) String() string { return fmt.Sprintf("%03X=%02X", p.Port, p.Last) }

// PortCount 是一個埠被讀了幾次。
type PortCount struct {
	Port  uint16
	Count uint64
}

func (p PortCount) String() string { return fmt.Sprintf("%03X×%d", p.Port, p.Count) }

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
// `WatchWrites` 收的是 DGROUP 偏移，只涵蓋程式自己的資料段。配置來的記憶體、
// 中斷向量表、視訊記憶體都在那之外——而「誰把這一格寫壞的」正是最常要問的問題。
//
// 紀錄裡的 `Off` 是**相對 lo 的位移**（區間可能超過 64 KB，所以不放絕對位址；
// 要絕對位址就 `lo + Off`）。同一個監看一次只能有一個，後設的取代前一個。
func (o *Oracle) WatchLinear(lo, hi uint32) *[]MemWrite {
	log := &[]MemWrite{}
	o.m.WatchWrites(lo, hi,
		func(a uint32, old, nw uint8) {
			*log = append(*log, MemWrite{
				Off:  uint16(a - lo),
				Old:  old,
				New:  nw,
				IP:   o.IP(),
				Step: o.Steps(),
			})
		})
	return log
}

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
