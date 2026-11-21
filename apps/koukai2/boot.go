package koukai2

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// MainPSP 是實測的子行程 PSP：`OPEN.EXE` 與 `MAIN.EXE` 都載在這裡。
//
// 只用於從狀態檔接續的探索（狀態檔不保存 EXEC 紀錄）。收據一律從冷啟動跑，
// 由 EXEC 紀錄算基底。
const MainPSP uint16 = 0x02CF

// MainAt 回 PSP 固定在 MainPSP 的 `MAIN.EXE` 換算。
func MainAt() Bind { return Bind{Name: "MAIN.EXE", PSP: MainPSP, Running: true} }

// `MAIN.EXE` 的位址（IDA 線性）與資料段欄位（koukai2 `docs/spec/`）。
const (
	AddrRandRaw uint32 = 0x14D92 // 051：seed = seed*0x41C64E6D + 0x3039
	AddrRandN   uint32 = 0x14DC4 // 051：rand_n，參數在 AX
	AddrTitle   uint32 = 0x2706A // 010：標題選單
	AddrChoose  uint32 = 0x26564 // 063：主角選擇
	AddrNewGame uint32 = 0x26949 // 063：新遊戲初始化
	AddrOneDay  uint32 = 0x2B2BA // 050：一天
	AddrPassKey uint32 = 0x45E92 // 063：防拷畫面讀鍵

	DSSeedLo    uint16 = 0xBD88 // 051
	DSSeedHi    uint16 = 0xBD8A
	DSDiskFlag  uint16 = 0x03D5 // byte_48525，010：恆為 1
	DSClock     uint16 = 0x0532 // byte_48682..48685：月、日、時段等
	DSClockSave uint16 = 0x0CAA // 存檔用的時鐘四欄位
	DSStateLo   uint16 = 0x0CA4 // 存檔範圍起點（043）
	DSStateHi   uint16 = 0x8A18 // 存檔範圍終點（不含）
)

// LCGNext 是 051 的亂數遞推。
func LCGNext(seed uint32) uint32 { return seed*0x41C64E6D + 0x3039 }

// RNGTrace 記錄 `MAIN.EXE` 的亂數呼叫，並逐次比對 051 的遞推。
//
// 種子只應由 `14D92` 改動。任何別的寫入（包含間接呼叫 `14DBA`）都會讓下一次
// 進入時的種子與預期不符，所以 Mismatch 為 0 同時驗證了公式與「沒有別的寫入端」。
//
// ⚠ **`OnCall` 在中斷返回時會再觸發一次。** hook 在每道指令執行前檢查；計時器中斷
// 剛好落在入口那一道時，返回位址就是入口本身，iret 回來 hook 又跑一次。沒有去重的話，
// 會看到「實際種子的下一個才是預期值」，看起來像種子被改了。實測開機路徑有 24 次。
// Repeats 記這種重複觸發，不算呼叫。
type RNGTrace struct {
	Calls     uint64            // 14D92 被呼叫幾次
	First     uint32            // 第一次進入時的種子
	Last      uint32            // 最後一次進入時的種子
	Expect    uint32            // 下一次進入應看到的種子
	Mismatch  uint64            // 不符次數
	Repeats   uint64            // 中斷返回造成的重複觸發
	NCalls    uint64            // 14DC4 被呼叫幾次
	ByN       map[uint16]uint64 // 依參數 n 統計
	FirstBad  string            // 第一筆不符的描述
	firstSeen bool
	rawSP     uint16
	nSP, nArg uint16
	nFresh    bool // 上一次 14DC4 之後還沒進過 14D92
}

// WatchRNG 掛上亂數追蹤。要在 `MAIN.EXE` 載入之後呼叫。
func WatchRNG(o *oracle.Oracle, b Bind) *RNGTrace {
	t := &RNGTrace{ByN: map[uint16]uint64{}}
	lo, hi := b.DS(DSSeedLo), b.DS(DSSeedHi)
	o.OnCall(b.Code(AddrRandRaw), func(o *oracle.Oracle) {
		seed := uint32(o.Word(lo)) | uint32(o.Word(hi))<<16
		if t.firstSeen && seed == t.Last && o.SP() == t.rawSP {
			t.Repeats++
			return
		}
		if !t.firstSeen {
			t.firstSeen, t.First = true, seed
		} else if seed != t.Expect {
			t.Mismatch++
			if t.FirstBad == "" {
				t.FirstBad = fmt.Sprintf("第 %d 次：預期 %08X，實際 %08X，步數 %d", t.Calls, t.Expect, seed, o.Steps())
			}
		}
		t.Last, t.rawSP = seed, o.SP()
		t.Expect = LCGNext(seed)
		t.Calls++
		t.nFresh = false
	})
	o.OnCall(b.Code(AddrRandN), func(o *oracle.Oracle) {
		// n 非 0 時，上一次進入之後一定會先進 14D92；沒有就是重複觸發。
		if t.nFresh && o.AX() == t.nArg && o.AX() != 0 && o.SP() == t.nSP {
			t.Repeats++
			return
		}
		t.nFresh, t.nArg, t.nSP = true, o.AX(), o.SP()
		t.NCalls++
		t.ByN[o.AX()]++
	})
	return t
}

// Checkpoint 是一個比對點：完整機器可見狀態的雜湊。
type Checkpoint struct {
	Name    string `json:"name"`
	Steps   uint64 `json:"steps"`
	Ticks   uint64 `json:"irq0_ticks"`
	Mode    uint8  `json:"video_mode"`
	Screen  string `json:"screen_sha256"`
	Palette string `json:"palette_sha256"`
	Mem     string `json:"mem_1mb_sha256"`
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// Capture 取一個比對點。Mem 是整個 1 MB 位址空間（含程式、資料與 EMS 頁框）。
func Capture(o *oracle.Oracle, name string) Checkpoint {
	_, _, px := o.Screen()
	pal := o.Palette()
	flat := make([]byte, 0, 768)
	for _, c := range pal {
		flat = append(flat, c[0], c[1], c[2])
	}
	return Checkpoint{
		Name: name, Steps: o.Steps(), Ticks: o.Ticks(), Mode: o.VideoMode(),
		Screen: sum(px), Palette: sum(flat), Mem: sum(o.Bytes(oracle.Addr{}, 1<<20)),
	}
}

// BootStep 是開機路徑的一步：先送鍵，再跑指令數。
type BootStep struct {
	Name string   // 這一步結束時看到的畫面
	Keys []string // 依序送的鍵，每鍵之間跑 KeyGap
	Run  uint64   // 送完之後跑幾道
}

// KeyGap 是連續送鍵之間的指令數，讓輪詢迴圈有機會讀走前一鍵。
const KeyGap = 2_000_000

// BootPath 是冷啟動到主迴圈的固定輸入序列（第一位主角、不改名）。
//
// 每一步的指令數都是固定值，所以同一個 dosgolem 版本一定走同一條時間線。
var BootPath = []BootStep{
	{Name: "password", Keys: []string{"Return"}, Run: 150_000_000},
	{Name: "title", Keys: []string{"Down", "Return"}, Run: 60_000_000},
	{Name: "choose", Keys: []string{"Return"}, Run: 40_000_000},
	{Name: "confirm", Keys: []string{"Return"}, Run: 40_000_000},
	{Name: "rename", Keys: []string{"Left", "Left", "Return"}, Run: 30_000_000},
	{Name: "confirm-all", Keys: []string{"Return"}, Run: 80_000_000},
}

// OpenSkipAt 是片頭開始後多久送 Esc（場景 1 只有 Esc 能跳過，063）。
const OpenSkipAt = 110_000_000

// PasswordWait 是 `MAIN.EXE` 載入到防拷畫面讀鍵的指令數上限之內的固定等待。
const PasswordWait = 60_000_000

// Sender 把一個鍵送進 BIOS 鍵盤。
func send(o *oracle.Oracle, keys []string) error {
	for i, k := range keys {
		if i > 0 {
			if err := o.Run(KeyGap); err != nil {
				return err
			}
		}
		if err := o.SendKeys(k); err != nil {
			return err
		}
	}
	return nil
}

// ToMain 從冷啟動跑到 `MAIN.EXE` 被載入。skip 為真時在 OpenSkipAt 送 Esc 跳過片頭。
func ToMain(o *oracle.Oracle, skip bool) error {
	if skip {
		if err := o.Run(OpenSkipAt); err != nil {
			return err
		}
		if err := o.SendKeys("Esc"); err != nil {
			return err
		}
	}
	return o.RunUntil(oracle.NewCond("MAIN.EXE 載入", IsMain), oracle.Budget(3_000_000_000))
}

// ToMainLoop 從 `MAIN.EXE` 載入走完 BootPath，每一步之前呼叫 cp（可為 nil）。
//
// 呼叫端要先 ToMain，並在 `MAIN.EXE` 載入之後才掛 hook。
func ToMainLoop(o *oracle.Oracle, cp func(step string)) error {
	if err := o.Run(PasswordWait); err != nil {
		return err
	}
	for _, st := range BootPath {
		if cp != nil {
			cp(st.Name)
		}
		if err := send(o, st.Keys); err != nil {
			return fmt.Errorf("%s 送鍵：%w", st.Name, err)
		}
		if err := o.Run(st.Run); err != nil {
			return fmt.Errorf("%s 之後：%w", st.Name, err)
		}
	}
	if cp != nil {
		cp("main-loop")
	}
	return nil
}

// SeedWrite 是一筆不是 `14D92` 寫入的種子改動。
type SeedWrite struct {
	Steps uint64 `json:"steps"`
	IDA   string `json:"ida"`
	Addr  uint32 `json:"linear"`
	Old   uint8  `json:"old"`
	Value uint8  `json:"value"`
}

// WatchSeedWriters 記下 `14DAD`、`14DB0` 以外對種子的寫入。
//
// WriteHit.At 以最外層程式換算，先減回執行期線性位址，再用 b 換成 `MAIN.EXE`
// 的 IDA 位址。
func WatchSeedWriters(o *oracle.Oracle, b Bind, out *[]SeedWrite) {
	lo := b.DS(DSSeedLo).Linear()
	ok1, ok2 := b.Code(0x14DAD).Linear(), b.Code(0x14DB0).Linear()
	base := o.ToIDA(oracle.Addr{})
	o.OnWrite(lo, lo+4, func(o *oracle.Oracle, h oracle.WriteHit) {
		at := h.At - base
		if at == ok1 || at == ok2 {
			return
		}
		*out = append(*out, SeedWrite{
			Steps: o.Steps(), IDA: fmt.Sprintf("%05X", at-uint32(b.loadSeg())*16+0x10000),
			Addr: h.Addr, Old: h.Old, Value: h.Val,
		})
	})
}
