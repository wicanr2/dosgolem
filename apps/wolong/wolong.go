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
	"sort"

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

// ---- 熱區圖 --------------------------------------------------------------

// 熱區查表的兩個指標（臥龍傳專案 `docs/re/22` §2）。
//
// `sub_1E453` 的定址式是 `offset = (x >> 3) + 10 × (y & 0xF8)`，
// 也就是**每格 8×8 像素、每列 80 個 byte、50 列**，全圖 4,000 bytes
// ——由定址式獨立推出的 640×400，不是從畫面量的。
const (
	dsHotzoneSeg = 0xE479 // sub_1E3C0 寫進去的段
	dsHotzoneOff = 0xE47B // 同上，偏移
)

// 熱區圖的格子大小與尺寸。
const (
	HotzoneCell            = 8
	HotzoneCols, HotzoneRows = 80, 50
)

// Hotzones 讀目前畫面的熱區圖。
//
// ⭐ **這是「點了沒反應」的直接答案。** 送出去的座標對不對，
// 用 `sub_1E453` 攔得到；但「那個位置到底有沒有東西可點」只有這張圖知道。
// 沒有它就只能一格一格試，而每一次試都要重跑整條開機流程。
func Hotzones(o *oracle.Oracle) []byte {
	seg := o.Word(o.DS(dsHotzoneSeg))
	off := o.Word(o.DS(dsHotzoneOff))
	if seg == 0 {
		return nil
	}
	return o.Bytes(oracle.Far(seg, off), HotzoneCols*HotzoneRows)
}

// Hotzone 是一個熱區編號涵蓋的像素矩形。
type Hotzone struct {
	ID      byte
	X, Y    int
	W, H    int
	Cells   int
}

// HotzoneBoxes 把熱區圖收成「編號 → 像素矩形」，按編號排序。
//
// ⚠ **同一個編號不一定是連通的**，所以回的是外接矩形加格數；
// 兩者差很多就表示那個編號散在好幾塊。
func HotzoneBoxes(o *oracle.Oracle) []Hotzone {
	return hotzoneBoxes(Hotzones(o))
}

// hotzoneBoxes 是純函式版本，好測。
func hotzoneBoxes(m []byte) []Hotzone {
	if len(m) < HotzoneCols*HotzoneRows {
		return nil
	}
	type acc struct{ x0, y0, x1, y1, n int }
	seen := map[byte]*acc{}
	for row := 0; row < HotzoneRows; row++ {
		for col := 0; col < HotzoneCols; col++ {
			id := m[row*HotzoneCols+col]
			if id == 0 {
				continue // 0 ＝ 沒有熱區
			}
			a := seen[id]
			if a == nil {
				a = &acc{x0: col, y0: row, x1: col, y1: row}
				seen[id] = a
			}
			if col < a.x0 {
				a.x0 = col
			}
			if col > a.x1 {
				a.x1 = col
			}
			if row < a.y0 {
				a.y0 = row
			}
			if row > a.y1 {
				a.y1 = row
			}
			a.n++
		}
	}
	out := make([]Hotzone, 0, len(seen))
	for id, a := range seen {
		out = append(out, Hotzone{
			ID: id,
			X:  a.x0 * HotzoneCell, Y: a.y0 * HotzoneCell,
			W: (a.x1 - a.x0 + 1) * HotzoneCell, H: (a.y1 - a.y0 + 1) * HotzoneCell,
			Cells: a.n,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// HotzoneAt 回某個像素座標上的熱區編號（0 ＝ 沒有）。
func HotzoneAt(o *oracle.Oracle, x, y int) byte {
	m := Hotzones(o)
	if m == nil || x < 0 || y < 0 || x >= 640 || y >= 400 {
		return 0
	}
	return m[(y/HotzoneCell)*HotzoneCols+x/HotzoneCell]
}

// ---- 大地圖的捲動原點 ----------------------------------------------------

// 捲動原點（臥龍傳專案 `docs/re/22` 的輸入模型之外的一層，
// 在 `sub_11F7F`／`loc_11FD0`，IDA `0x11FD0`）。
//
// ⚠ **進到遊戲之後，畫面座標不等於滑鼠座標。**
//
//	螢幕 x ＝ 滑鼠 x − 原點 x        （夾在 0..27Fh）
//	推成負的 → 原點跟著減，游標貼到 0，**地圖往那個方向捲**
//	推超過 27Fh → 原點跟著加，游標貼到 27Fh
//
// 這是遊戲自己實作的「游標推到邊緣就捲地圖」。選單畫面不走這一層
// （那邊 `sub_121E7` 直接拿原始游標查熱區），所以開機到選劇本那一段
// 用原始座標是對的——**同一個座標在兩種畫面代表不同的東西**。
//
// 症狀：在遊戲中送畫面座標，游標不會到那裡，地圖反而捲走了，
// 而**畫面看起來完全正常**（地圖本來就會捲）。
const (
	dsScrollOriginX = 0x9882
	dsScrollOriginY = 0x9884
	dsScreenCurX    = 0x9886 // sub_11F7F 算出來的畫面座標
	dsScreenCurY    = 0x9888
)

// ScrollOrigin 讀目前的捲動原點。
func ScrollOrigin(o *oracle.Oracle) (x, y int) {
	return int(o.Word(o.DS(dsScrollOriginX))), int(o.Word(o.DS(dsScrollOriginY)))
}

// ScreenCursor 讀遊戲算出來的畫面座標（＝滑鼠 − 原點）。
func ScreenCursor(o *oracle.Oracle) (x, y int) {
	return int(o.Word(o.DS(dsScreenCurX))), int(o.Word(o.DS(dsScreenCurY)))
}

// HomeCursor 把捲動原點歸零。
//
// 作法就是遊戲自己的規則：**把滑鼠推到 (0,0)**，負的差值會被原點吸收，
// 於是原點變 0、畫面游標也變 0。之後送的座標才等於畫面座標。
//
// ⚠ 這一步會**捲動地圖**（原點變了），所以它不是無副作用的。
// 要保持地圖位置就自己記下原點再捲回去。
func HomeCursor(o *oracle.Oracle, settle uint64) error {
	if x, y := ScrollOrigin(o); x == 0 && y == 0 {
		// 已經歸零了就不要再跑一趟 (0,0)。
		// **多跑那一趟不是沒有代價**：游標會先移到左上角，
		// 而彈出選單與 hover 反白都吃游標位置。
		return nil
	}
	o.MoveMouse(0, 0)
	if err := o.Run(settle); err != nil {
		return err
	}
	if x, y := ScrollOrigin(o); x != 0 || y != 0 {
		return fmt.Errorf("歸零之後原點還是 (%d,%d)——遊戲可能不在大地圖畫面", x, y)
	}
	return nil
}

// DefaultHomeSettle 是 HomeCursor 等遊戲處理的指令數。
//
// 主迴圈約 25,000 道指令跑一圈（攔 `sub_1E453` 量的），這裡取兩位數倍。
const DefaultHomeSettle = 400_000

// ClickScreen 在**畫面座標**點一下：先歸零原點，再移到目標。
func ClickScreen(o *oracle.Oracle, x, y int, opts ...oracle.ClickOpt) error {
	if err := HomeCursor(o, DefaultHomeSettle); err != nil {
		return err
	}
	return o.Click(x, y, opts...)
}

// TapScreen 是 ClickScreen 的瞬按版（彈出選單要用）。
func TapScreen(o *oracle.Oracle, x, y int, opts ...oracle.ClickOpt) error {
	if err := HomeCursor(o, DefaultHomeSettle); err != nil {
		return err
	}
	return o.Tap(x, y, opts...)
}
