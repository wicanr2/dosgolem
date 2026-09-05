// Package wolong 是《臥龍傳－三國制霸之計》（NEO･GETEN 1994／松崗 1995）
// 專屬的導航與對拍支援。
//
// 這一層放的是**這一支 binary 的知識**（`docs/spec/006` 的分層判準）：
// 畫面幾何、開機流程、座標空間。機器層與觀測層看不到它們。
//
//	o, _ := wolong.Load(exe, root)
//	wolong.ToScenarioMenu(o)
//	o.WritePNGCrop("shot.png", wolong.ContentTop, wolong.ContentHigh)
//
// ⚠ **不含任何原版檔案**，素材由玩家自備。
package wolong

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// 畫面幾何。
//
// 遊戲設 mode 12h（640×480），但**內容只有 640×400，y 原點在第 40 列**：
// 繪製常式的 VRAM 段是 `A0C8h` ＝ `A000h` ＋ 0xC80 bytes ＝ 40 列
// （臥龍傳專案 `docs/re/28` §1）。同一個 40 也是 `tools/parity_crop.py`
// 在 DOSBox-X 截圖上量出來的——兩個獨立來源對上。
const (
	ScreenW, ScreenH        = 640, 480
	ContentTop, ContentHigh = 40, 400
)

// Load 載入 `KI.EXE`。root 是原版素材目錄（玩家自備）。
//
// 兩個與 rich2 不同的設定，兩個弄錯都不會報錯：
//
//   - **`DGROUP` ＝ 映像段**：這一支是組語寫的，`ds:` 就是 `cs:`。
//     rich2 是編譯後的 BASIC，DGROUP 另有其處。
//   - **`MouseXScale` ＝ 1**：滑鼠驅動的座標範圍被遊戲設成
//     0–27Fh × 0–18Fh ＝ 640×400（`docs/re/01`），與像素 1:1。
//     mode 13h 的 2 套過來的話送出去的 X 全部差一倍，
//     而畫面上只看得到「點不到東西」。
func Load(exe, root string) (*oracle.Oracle, error) {
	return LoadWith(exe, root, "", "")
}

// LoadWith 是指定字型檔的 Load。
//
// 空字串沿用預設（`END_S13.DAT`／`END_S14.DAT`）。想重現
// `STR.EXE` 寫死的那一組就給 `END_S10.DAT`／`END_S11.DAT`——
// 那是臥龍傳專案 `docs/re/29` §6 掛著的未解項，換個檔名跑一次就知道。
func LoadWith(exe, root, fontFull, fontHalf string) (*oracle.Oracle, error) {
	return oracle.LoadWith(exe, root, oracle.Options{
		MouseXScale: 1,
		FontFull:    fontFull,
		FontHalf:    fontHalf,
	})
}

// 座標：遊戲自己的空間就是內容座標（0–639 × 0–399）。
//
// ⚠ **DOSBox-X 那邊不是。** 它的視窗是 640×480，而 `int 33h` 把**整個視窗**
// 等比對映到遊戲的 640×400，所以那邊送點擊要 `視窗 y ＝ 遊戲 y × 1.2`
// （臥龍傳專案 `docs/re/43`）。dosgolem 沒有視窗，送進來的就是遊戲座標——
// **把舊腳本的視窗座標照抄過來會差幾個像素**，而那幾個像素正好落在
// 按鈕之間的空隙。
//
// FromDOSBoxY 把舊腳本的視窗 y 換算回遊戲 y，讓既有的對拍腳本搬得過來。
//
// ⚠ **分母是 479 不是 480。** DOSBox-X 把視窗的 0–479 對映到遊戲的 0–399，
// 兩端對齊，所以是 `y × 399 ÷ 479`。用 `× 400 ÷ 480` 大部分點算出來一樣，
// **只有少數點差 1**——視窗 336 是 279 不是 280。實測：那 1 個像素讓
// 主畫面的游標整塊對不上（62 點），而其餘四區全 0，
// 看起來像「只差一點點」而不像「換算式錯了」。
func FromDOSBoxY(windowY int) int {
	return windowY * (ContentHigh - 1) / (ScreenH - 1)
}

// NewGameYes 是開機那個 NEW GAME 確認框的「YES」。
//
// 座標是從原版擷取腳本的視窗座標 (320,215) 換算來的，
// 換算式見 FromDOSBoxY。
var NewGameYes = struct{ X, Y int }{320, FromDOSBoxY(215)}

// Booted 是「畫面畫完並且停住」。
//
// ⚠ **只用「畫面沒變」會在開機前就成立。** 剛載入時畫面是全 0，
// 那當然「連續兩百萬道指令沒變」——條件在 220 萬道就達成，
// 而遊戲要到 330 萬道才把主畫面畫出來。症狀是接下來的點擊全部落空，
// 錯誤訊息卻說「點了畫面沒變」，指向座標而不是時機。
//
// 所以要先過一道「畫面上真的有東西」的閘，再看它停住。
//
// ⚠ **不要用「跑滿 N 道指令」當判準。** 這一款是即時制，
// 同一個指令數在不同狀態下停在不同的地方。
func Booted() oracle.Cond {
	idle := oracle.ScreenIdle(2_000_000)
	var next uint64
	var painted bool
	return oracle.NewCond("畫面畫完並停住", func(o *oracle.Oracle) bool {
		if !painted {
			// 取樣，不是每道指令都算——30 萬個像素數一遍不便宜。
			if o.Steps() < next {
				return false
			}
			next = o.Steps() + 100_000
			_, _, px := o.Screen()
			nz := 0
			for _, v := range px {
				if v != 0 {
					nz++
				}
			}
			if nz*10 < len(px) {
				return false
			}
			painted = true
		}
		return idle.Ready(o)
	})
}

// ToNewGamePrompt 從冷啟動跑到 NEW GAME 確認框。
func ToNewGamePrompt(o *oracle.Oracle) error {
	if err := o.RunUntil(Booted(), oracle.Budget(20_000_000)); err != nil {
		return fmt.Errorf("跑到 NEW GAME 確認框：%w", err)
	}
	return nil
}

// ToScenarioMenu 再往前一步：按下 YES，進劇本選單。
func ToScenarioMenu(o *oracle.Oracle) error {
	if err := ToNewGamePrompt(o); err != nil {
		return err
	}
	if err := o.Click(NewGameYes.X, NewGameYes.Y); err != nil {
		return fmt.Errorf("點 NEW GAME 的 YES：%w", err)
	}
	return o.RunUntil(Booted(), oracle.Budget(20_000_000))
}

// Shot 存一張與原版可以直接逐點比的 640×400 PNG。
func Shot(o *oracle.Oracle, path string) error {
	return o.WritePNGCrop(path, ContentTop, ContentHigh)
}

// ---- 遊戲時鐘 ------------------------------------------------------------

// 時鐘欄位在 DGROUP 的位移（臥龍傳專案 `docs/re/06`，機器碼直接讀出來的）。
//
// 這一支執行檔程式碼與資料同段，所以 `ds:` 偏移就是 IDA 線性減 0x10000。
const (
	dsDay   = 0x0CF0 // u8 日
	dsHour  = 0x0CF3 // u8 時（1–24）
	dsMonth = 0x0CF4 // u8 月
	dsYear  = 0x0CF6 // u16 年
)

// Date 是遊戲內的日期。
type Date struct{ Year, Month, Day, Hour int }

func (d Date) String() string {
	return fmt.Sprintf("%d年%d月%d日 %d時", d.Year, d.Month, d.Day, d.Hour)
}

// Clock 讀原版的遊戲時鐘。
//
// ⭐ **這是不用 DOSBox 最大的好處。** 這一款是即時制，同一串操作在
// 牆上時鐘的不同時刻會停在不同的遊戲日期——DOSBox 那邊只能靠
// `wait:3` 之類的秒數去猜，猜錯就是「畫面差了幾天」而看起來像版面不對
// （臥龍傳專案 `docs/playtest/39`：原版擷取停在 4 月 9 日，
// remake 停在 4 月 1 日，那 158 個不同像素其實是日期）。
// 在這裡日期是**問得到的**，所以取樣點可以直接寫成日期。
func Clock(o *oracle.Oracle) Date {
	return Date{
		Year:  int(o.Word(o.DS(dsYear))),
		Month: int(o.Byte(o.DS(dsMonth))),
		Day:   int(o.Byte(o.DS(dsDay))),
		Hour:  int(o.Byte(o.DS(dsHour))),
	}
}

// UntilDate 是「跑到遊戲日期到某一天」。
//
// ⚠ **已經過了就永遠不會成立。** 條件只往前看，回頭要靠快照。
func UntilDate(year, month, day int) oracle.Cond {
	want := Date{Year: year, Month: month, Day: day}
	return oracle.NewCond(
		fmt.Sprintf("遊戲日期到 %d年%d月%d日", year, month, day),
		func(o *oracle.Oracle) bool {
			d := Clock(o)
			if d.Year != want.Year {
				return d.Year > want.Year
			}
			if d.Month != want.Month {
				return d.Month > want.Month
			}
			return d.Day >= want.Day
		})
}
