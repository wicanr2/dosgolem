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
	// hookBits 是 onCall 的鍵在 1 MB 位址空間上的點陣圖。
	// nil ＝ 一個 hook 都沒註冊過。
	hookBits []uint64
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
func (o *Oracle) DS(off uint16) Addr { return Addr{o.dgroupSeg, off} }

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

// ---- 佈局 ----------------------------------------------------------------

// SetByte／SetWord／SetBytes 直接寫原版的變數。
//
// 對拍要的是「同一個局面下原版怎麼決定」，所以**局面要由對拍那一方
// 擺出來**，不能靠原版自己的亂數把局面湊出來。原版的 `RND()` 帶著
// 自己的種子與呼叫次數，跑兩次不見得一樣，而且它一動整張盤面都會變
// ——那樣比出來的差異分不出是「決策不同」還是「盤面不同」。
//
// 寫進去的位址由呼叫端負責：先用 `IDA`／`DS` 換算，寫完再讀回來確認。
func (o *Oracle) SetByte(a Addr, v uint8)  { o.m.Write8(a.Linear(), v) }
func (o *Oracle) SetWord(a Addr, v uint16) { o.m.Write16(a.Linear(), v) }

// SetBytes 寫一段，回傳寫了幾個位元組。
func (o *Oracle) SetBytes(a Addr, b []byte) int {
	for i, v := range b {
		o.m.Write8(a.Linear()+uint32(i), v)
	}
	return len(b)
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

// IndexedEGA 是 EGA 平面模式解出來的畫面。
//
// `Indexed()` 給的是線性的 A0000 視窗（64,000 個位元組）；EGA 的
// 640×350 是**四個位元平面**，要指定寬高才解得出來。拿線性那一份當畫面
// 存圖會得到一張有規律的條紋——看起來像畫面壞掉，而不像取錯了緩衝區。
func (o *Oracle) IndexedEGA(w, h int) []uint8 { return o.m.IndexedEGASize(w, h) }

// Palette 回 256×3 的 RGB。
func (o *Oracle) Palette() [256][3]uint8 { return o.m.Palette() }

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

// KeyQueueLen 是硬體鍵盤佇列裡還沒被取走的掃描碼數。
//
// **送了鍵沒反應時第一個要看的數字**：它大於 0 表示掃描碼還在佇列裡
// ——不是送錯路，是沒被中斷帶進程式。
func (o *Oracle) KeyQueueLen() int { return o.m.KeyQueueLen() }

// IRQ1Delivered 是鍵盤中斷送出去幾次；IRQ1ToProgram 是其中幾次進到
// 程式自己裝的處理常式。
//
// 兩者差很多就表示中斷被 BIOS 的預設常式吃掉了，程式沒看到。
func (o *Oracle) IRQ1Delivered() int { return o.m.IRQ1Delivered() }
func (o *Oracle) IRQ1ToProgram() int { return o.m.IRQ1ToProgram() }

// KeyWaits 數「佇列空的時候被要求讀一個鍵」發生了幾次。
//
// **這是「它在等鍵盤」與「它在做事」的分界**：送了鍵卻沒反應時，
// 這個數字告訴你程式到底有沒有在讀——它是 0 的話，鍵送到哪裡都沒用，
// 因為根本沒有人在讀那條路。
func (o *Oracle) KeyWaits() int { return o.d.KeyWaits }

// KeyReads 是被讀走的鍵，附讀取方式與步數。
func (o *Oracle) KeyReads() []KeyRead {
	out := make([]KeyRead, 0, len(o.d.KeyReads))
	for _, k := range o.d.KeyReads {
		out = append(out, KeyRead{Step: k.Step, Via: k.Via, Key: k.Key})
	}
	return out
}

// KeyRead 是一次讀鍵。Via 說它走的是哪一條路。
type KeyRead struct {
	Step uint64
	Via  string
	Key  uint8
}

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

// WatchWritesAt 監看一段**線性位址**的寫入，回一份逐次紀錄。
//
// `WatchWrites` 只認 DGROUP 偏移，而遊戲把資料表放在別的段是常態
// ——執行期用線性位址搜出來的東西沒有 DGROUP 偏移可用。
//
// 回傳的紀錄裡 `Off` 是「距離 lo 幾個位元組」，不是 DGROUP 偏移。
func (o *Oracle) WatchWritesAt(lo, hi uint32) *[]MemWrite {
	log := &[]MemWrite{}
	o.m.WatchWrites(lo, hi, func(a uint32, old, nw uint8) {
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

// WatchReadsAt 監看一段**線性位址**的讀取，回一份逐次紀錄。
//
// 靜態掃描找不到讀取端時用這一支：掃到零筆只證明「沒有絕對定址的
// 參考」，不證明沒有人讀——用算出來的指標取的存取在位元組層面看不見。
//
// `Off` 是「距離 lo 幾個位元組」。**每一次讀取都記一筆**，
// 所以範圍要開小，而且跑完記得 `StopWatchingReads`。
func (o *Oracle) WatchReadsAt(lo, hi uint32) *[]MemRead {
	log := &[]MemRead{}
	o.m.WatchReads(lo, hi, func(a uint32, v uint8) {
		*log = append(*log, MemRead{
			Off:  uint16(a - lo),
			Val:  v,
			IP:   o.IP(),
			Step: o.Steps(),
		})
	})
	return log
}

// StopWatchingReads 關掉讀取監看。
func (o *Oracle) StopWatchingReads() { o.m.WatchReads(0, 0, nil) }

// MemRead 是一次讀取。
//
// ⚠ **IP 是讀的那一刻的 CS:IP，也就是那道指令本身**，不是它的呼叫端。
type MemRead struct {
	Off  uint16
	Val  uint8
	IP   Addr
	Step uint64
}

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
	Op     string // open／seek／read
	Handle uint16
	Name   string
	Arg    int64 // seek 的位移、read 的要求量、open 的檔案大小
	Result int64 // seek 後的位置、read 實際讀到的量
}

// TraceFiles 打開檔案操作追蹤。要在 Run 之前呼叫。
//
// **開檔清單只說「開過什麼」，說不出「要求讀哪一段、拿到多少」。**
// 對拍容器格式要問的正是後者：原版 seek 到哪個位移、讀多長——
// 那組數字就是它自己算出來的項目邊界，拿來對 remake 的解碼器
// 比「兩條路徑算同一件事」。
func (o *Oracle) TraceFiles() { o.d.FileTrace = []dos.FileOp{} }

// FileOps 回傳目前為止的檔案操作。
func (o *Oracle) FileOps() []FileOp {
	out := make([]FileOp, 0, len(o.d.FileTrace))
	for _, f := range o.d.FileTrace {
		out = append(out, FileOp{
			Step: f.Step, Op: f.Op, Handle: f.Handle,
			Name: f.Name, Arg: f.Arg, Result: f.Result,
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
