package rich2

import (
	"bytes"
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/runtime/basic"
)

// 場所畫面的操作。
//
// 對拍規則層的瓶頸在這裡：`ForceSteps` 已經可以把棋子開到任何一格
// （`steer.go`），但**踩到銀行之後那張畫面等輸入，沒辦法回答它**——
// 遊戲就停在那裡（`rich2/docs/playtest/054` §3.2）。
//
// 出路不是辨識畫面，是攔**選擇器本體**：全遊戲的選單共用同一支常式
// （`rich2/docs/re/099` §2、`docs/re/141` §2，都是 confirmed），
// 而且它的六個參數裡就有「有幾列」。攔到它就知道
// 「現在有一張選單、幾個選項、文字從哪裡取」——**比從像素反推可靠**。
const (
	// IDASelector 是選擇器本體的**進入點**（副程式 #22，`0CED:35E6`）。
	//
	// ⚠ `rich2/docs/re/099` §2 寫的是 `204B4`，**差 2**：
	// `204B0` 是前一支的 `retf 0Ch`、`204B3` 是一個 `jmp`，
	// 真正的進入點是 `204B6`（`mov cx,16h` ＝ BASIC 的框架配置）。
	// 段值換算也對得上：`0CED` ＋ 0x1000 → `1CED:35E6` ＝ `0x204B6`。
	//
	// 攔在進入點是刻意的：**那一刻 `SP` 還指著返回位址與參數**，
	// 框架一配置就讀不到了。
	IDASelector = 0x204B6

	// IDAMenuHit 是**滑鼠命中判定**（`0CED:6624`，5 參數，
	// `rich2/docs/re/141` §1，confirmed）。
	//
	// 攔它比從選擇器的六個參數推算幾何可靠得多——那六個要經過
	// `+36`／`+10`／`<<4 +42` 幾層換算，而**哪一個是 x 哪一個是 y
	// 我推錯過一次**（算出來的 y ＝ 202，超出 320×200 的畫面）。
	// 命中判定收到的已經是換算完的 x0／x1／y0／間距／列數。
	IDAMenuHit = 0x234F4

	// IDASelectorRet 是取回傳值那一段。
	//
	// ⚠ **要攔 `2057F` 不是 `2057C`**：`2057C` 是 `mov ax,[bp-18h]`，
	// 攔在它上面時那一道還沒執行，`AX` 裡是別的東西。
	// 攔錯的症狀是「每一張選單都選了同一個奇怪的數字」。
	IDASelectorRet = 0x2057F
)

// 原版選擇器只認這四種鍵（`rich2/docs/re/100`，confirmed）。
//
// ⚠ **鍵盤不走 `int 16h`**，走 `int 21h AH=3Fh` 讀 handle 0
// （BASIC 的 `INKEY$`），所以送鍵就是往 stdin 塞位元組。
// 方向鍵是**兩個位元組**：`00` 之後才是掃描碼。
const (
	KeyEnter = "\r"
	KeyEsc   = "\x1b"
	KeyUp    = "\x00\x48"
	KeyDown  = "\x00\x50"
	KeyLeft  = "\x00\x4B"
	KeyRight = "\x00\x4D"
)

// SelectorCancel 是「取消／離場」的回傳值（`rich2/docs/re/077` §2.1）。
const SelectorCancel = 0x63

// Cancelled 回報這一張選單是被取消的。
func (s Selector) Cancelled() bool { return s.Chosen == SelectorCancel }

// Selector 是一次選單的觀測。
//
// 六個參數的語意見 `rich2/docs/re/141` §2（confirmed）。
type Selector struct {
	Step  uint64
	X, Y  int // 選單左上角
	Width int // 每列寬度，單位是全形字
	Rows  int // **列數**（原版第 4 參數 ＋ 2）
	Text  int // 文字起始索引
	Style int // 樣式／模式

	// Chosen 是玩家選了第幾列（1 起算）。
	//
	// ⚠ **取消不是 0，是 `SelectorCancel`（0x63 ＝ 99）**
	// （`rich2/docs/re/077` §2.1，confirmed，股市三個呼叫端共用）。
	// 把 99 當成「選了第 99 列」會得到一個看起來很合理的錯誤。
	Chosen int
	Done   bool // 有沒有攔到回傳

	// 命中判定實際收到的幾何（攔 `IDAMenuHit`）。**這是量到的不是算的。**
	HitX0, HitX1, HitY0, HitPitch, HitRows int
	HitSeen                                bool
}

// SelectorLog 收集整場的選單，也可以自動回答它們。
type SelectorLog struct {
	All  []Selector
	cur  *Selector
	pick func(*Selector) int
}

// Answer 讓接下來每一張選單一開就自動回答。
//
// **這是讓對拍穿過場所畫面的關鍵**：`PlayTurn` 是一段長跑，中間跳出來的
// 選單沒有人回答就會一直卡著（`rich2/docs/playtest/054` §3.2 記過：
// 踩到股市之後每一回合都在空燒一億五千萬道指令）。
//
// pick 回第幾列（1 起算）；回 0 表示按 ESC 退出；回負數表示不回答
// （留給呼叫端自己處理）。
//
// ⚠ **自動回答只做得到「第 1 列」與「ESC」**，因為它跑在指令迴圈裡，
// 不能點滑鼠（`Click` 會重入執行迴圈）。要選中間某一列就在迴圈外用
// `ChooseRow`。
//
// ⚠ **鍵是在選擇器進入的那一刻就排進 stdin 的**，不是等它讀。
// 所以 pick 只看得到選單的形狀（幾列、文字起始索引），
// 看不到選單畫出來的內容——要按內容決定，得先解出文字索引怎麼對到字串。
func (l *SelectorLog) Answer(pick func(*Selector) int) { l.pick = pick }

// Open 回目前有沒有一張選單在等輸入。
func (l *SelectorLog) Open() *Selector {
	if l == nil || l.cur == nil {
		return nil
	}
	return l.cur
}

// WatchSelectors 掛上攔截。**要在 Run／Click 之前叫。**
func WatchSelectors(o *oracle.Oracle) *SelectorLog {
	log := &SelectorLog{}

	o.OnCall(o.IDA(IDASelector), func(o *oracle.Oracle) {
		a := basic.CallArgs(o, 6)
		log.All = append(log.All, Selector{
			Step: o.Steps(),
			X:    int(int16(a[0])), Y: int(int16(a[1])),
			Width: int(int16(a[2])),
			// **第 4 參數是「列數 − 2」**，不是列數。
			// 照抄成列數會讓「往下按幾次」少兩次，而那看起來像
			// 「選錯了」而不是「算錯了」。
			Rows: int(int16(a[3])) + 2,
			Text: int(int16(a[4])), Style: int(int16(a[5])),
		})
		log.cur = &log.All[len(log.All)-1]

		if log.pick == nil {
			return
		}
		// ⚠ **這裡只排鍵，不點滑鼠。**
		// 這是 `OnCall` 的 hook，正跑在指令迴圈裡；`Click` 內部會
		// `RunUntil`，在 hook 裡重入執行迴圈會炸。要按列選就用
		// `ChooseRow`，那是給呼叫端在迴圈外用的。
		// **先清掉上一張沒吃掉的鍵。** 不清的話它會把這一張取消掉，
		// 而症狀是「送了 Enter 卻收到取消碼」（見 `Oracle.ClearInput`）。
		o.ClearInput()
		switch row := log.pick(log.cur); {
		case row < 0: // 不回答
		case row == 0:
			o.Type(KeyEsc) // 0 ＝ 取消
		case row == 1:
			o.Type(KeyEnter) // 第 1 列就是游標的起點
		default:
			// **第 2 列以後要靠滑鼠。** 鍵盤只有 Enter 與 ESC 送得進去
			// （方向鍵走 stdin 送不進去，`rich2/docs/playtest/054` §3.2），
			// 而選擇器的反白**每一圈都直接由滑鼠位置決定**
			// （`rich2/docs/spec/017` 的「反白跟著滑鼠走」，`1F5A5`）。
			// 所以把游標放到那一列上，再送 Enter。
			//
			// 幾何用進場那六個參數算（`RowPoint` 在 `HitSeen` 之前就是
			// 走這一條）：x0 ＝ 第 1 參數、y0 ＝ 第 2 參數、間距 18、
			// x1 ＝ x0 + 16×寬 + 42。命中判定實測與這組吻合。
			x, y := log.cur.RowPoint(row)
			o.MoveMouse(x, y)
			o.Type(KeyEnter)
		}
	})

	o.OnCall(o.IDA(IDAMenuHit), func(o *oracle.Oracle) {
		if log.cur == nil || log.cur.HitSeen {
			return
		}
		// **參數順序是 (y0, 間距, x0, x1, 列數)。**
		//
		// 這是量出來的，不是從 `docs/re/141` §1 的敘述推的——那一份講的是
		// 命中公式，沒有寫參數順序。主選單實測收到 `(147, 18, 123, 197, 3)`，
		// 三個獨立的對照同時成立才定下這個順序：
		// 間距 ＝ 18（文件寫死的值）、列數 ＝ 3（選擇器第 4 參數 ＋ 2）、
		// x1 ＝ 123 ＋ 16×2 ＋ 42 ＝ 197（文件的算式）。
		a := basic.CallArgs(o, 5)
		log.cur.HitY0 = int(int16(a[0]))
		log.cur.HitPitch = int(int16(a[1]))
		log.cur.HitX0 = int(int16(a[2]))
		log.cur.HitX1 = int(int16(a[3]))
		log.cur.HitRows = int(int16(a[4]))
		log.cur.HitSeen = true
	})

	o.OnCall(o.IDA(IDASelectorRet), func(o *oracle.Oracle) {
		if log.cur == nil {
			return
		}
		log.cur.Chosen = int(int16(o.AX()))
		log.cur.Done = true
		log.cur = nil
	})

	return log
}

// 選單的版面常數（`rich2/docs/re/099` §2、`docs/re/141` §1，都是 confirmed）。
//
//	x0 = 第1參數        y0 = 第2參數
//	x1 = 第1參數 + 16 × 第3參數 + 42
//	間距 = 18
//
// ⚠ `docs/re/099` §2 的 `+36`／`+10` 是**繪製**用的（`[bp-14h]`／`[bp-16h]`），
// 不是命中範圍。拿它們算點擊座標會偏兩列。
//
// 命中規則：
//
//	在 [x0, x1] × [y0, y0 + 間距 × 列數] 之外 → 沒中
//	否則 列 = min((滑鼠Y − y0) / 間距 + 1, 列數)
const (
	menuEdgeX = 42
	menuPitch = 18
	menuCharW = 16
)

// RowPoint 算第 row 列的螢幕座標（1 起算）。
//
// **優先用命中判定實際收到的幾何**（`HitSeen`），那是量到的；
// 攔不到才退回從選擇器的六個參數推算。
//
// ⚠ 推算那條路我錯過一次：把第 1／2 參數當成 x／y 再各加 36／10，
// 算出第 3 列在 y ＝ 202——**超出 320×200 的畫面**。實測命中判定收到的
// x0／y0 就是第 1／2 參數本身，沒有那兩個偏移（那兩個是給繪製用的）。
func (s Selector) RowPoint(row int) (x, y int) {
	x0, x1, y0, pitch := s.X, s.X+menuCharW*s.Width+menuEdgeX, s.Y, menuPitch
	if s.HitSeen {
		x0, x1, y0, pitch = s.HitX0, s.HitX1, s.HitY0, s.HitPitch
	}
	return (x0 + x1) / 2, y0 + pitch*(row-1) + pitch/2
}

// ChooseRow 選第 row 列（1 起算）。
//
// # 為什麼走滑鼠不走方向鍵
//
// `rich2/docs/re/100` 說方向鍵是 `CHR$(0)+CHR$(72/80/75/77)`，但**送進
// stdin 沒有用**：實測 `00 50`、`50`、`E0 50`、`00 50 00 50` 四種編碼
// 加上只送 Enter 的對照組，原版一律收到第 1 列
// （`rich2/internal/parity/keytest_test.go`）。
//
// 嫌疑是 `StdinFill` ＝ 0：佇列空的時候餵 0 表示「沒按鍵」，
// 而擴充鍵的前綴也是 0，遊戲分不出來。**還沒查完**，所以那條路先擱著。
//
// 滑鼠那條路反而是**完全指定的**：命中公式與六個參數的語意都 confirmed
// （`docs/re/141`），而且原版執行期的滑鼠旗標實測為 1（`docs/re/106`）。
// 所以這裡照公式算出第 row 列的座標再點下去——**仍然是正常玩家路徑**，
// 不是直接寫選擇結果。
func ChooseRow(o *oracle.Oracle, s *Selector, row int, opts ...oracle.ClickOpt) error {
	if s == nil {
		return fmt.Errorf("現在沒有選單在等輸入")
	}
	if row < 1 || row > s.Rows {
		return fmt.Errorf("選第 %d 列，但這張選單只有 %d 列", row, s.Rows)
	}
	x, y := s.RowPoint(row)
	return o.Click(x, y, opts...)
}

// Labels 讀這張選單每一列的文字。
//
// 第 5 參數是**文字起始索引**（`rich2/docs/re/141` §2，confirmed），
// 指進定長字串表 `17ECh`（`state.go` 的 `Texts`）。所以
// 第 k 列的文字就是 `17ECh[Text + k - 1]`。
//
// **有了這個，回答選單就能按內容決定而不是按列號。** 「第 3 列」在不同
// 場所是不同的東西，「存款」在哪裡都是存款——按列號寫的對拍測試，
// 換一張選單就默默對到別的項目上。
//
// ⚠ 回傳的是 **Big5 位元組**，不是 UTF-8：原版文字裡混著版面控制碼，
// 解成字串會失真（`rich2/internal/assets` 的 `TextEntry.Raw` 同一個理由）。
// 要顯示的話呼叫端自己轉。
//
// ⚠ 這是**強證據不是 confirmed**：文字索引指進哪一張表，是拿主選單的
// 三列反查對上的，還沒逐張驗過。
func (s Selector) Labels(o *oracle.Oracle) [][]byte {
	if s.Rows <= 0 || s.Text < 0 {
		return nil
	}
	a := Texts(o)
	out := make([][]byte, 0, s.Rows)
	for k := 0; k < s.Rows; k++ {
		i := s.Text + k
		if i < 0 || i >= TextSlots {
			out = append(out, nil)
			continue
		}
		out = append(out, trimTextSlot(a.Bytes(i)))
	}
	return out
}

// trimTextSlot 去掉定長欄位的尾端填充與結尾標記。
func trimTextSlot(raw []byte) []byte {
	end := len(raw)
	for end > 0 && (raw[end-1] == ' ' || raw[end-1] == 0) {
		end--
	}
	if end > 0 && raw[end-1] == '\r' {
		end--
	}
	return raw[:end]
}

// RowByText 找出文字含有 want 的那一列（1 起算）；找不到回 0。
//
// **按內容找，不按列號。** 「第 5 列」在不同場所是不同的東西，
// 「離開」在哪裡都是離開——按列號寫的對拍測試，換一張選單就默默對到
// 別的項目上。
//
// want 是 **Big5 位元組**（原版的編碼）。比對前兩邊都會去掉半形與全形空白：
// 原版的選單文字有對齊用的填充，例如股市場所的第 5 列是 `" 離　開"`
// ——中間那個是全形空白，逐字比會找不到。
func (s Selector) RowByText(o *oracle.Oracle, want string) int {
	w := squeezeMenuText([]byte(want))
	if len(w) == 0 {
		return 0
	}
	for i, b := range s.Labels(o) {
		if bytes.Contains(squeezeMenuText(b), w) {
			return i + 1
		}
	}
	return 0
}

// squeezeMenuText 去掉半形空白與 Big5 的全形空白（`A1 40`）。
func squeezeMenuText(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		if b[i] == ' ' {
			continue
		}
		if b[i] == 0xA1 && i+1 < len(b) && b[i+1] == 0x40 {
			i++
			continue
		}
		out = append(out, b[i])
	}
	return out
}

// 原版選單裡幾個常用的字（Big5），**從 `PART1.PAK` 區段 2 的位元組讀出來的**。
// 那張表 §3.1 驗過與執行期的 `17ECh` 逐格相同，所以這就是原版真正用的編碼。
//
// ⚠ **不要憑印象打 Big5。** 第一版的「賣出」寫成 `BD E0`，實際是 `BD E6`
// ——差一個位元組，而 `RowByText` 找不到的時候只會回 0，
// 看起來像「這張選單沒有賣出這一項」，不像編碼打錯。
//
// 原始位元組（含對齊用的半形與全形空白，`RowByText` 會去掉）：
//
//	356  20 A4 55 A4 40 AD B6     下一頁
//	357  20 B6 52 A1 40 B6 69     買　進
//	358  20 BD E6 A1 40 A5 58     賣　出
//	359  20 AC 64 A1 40 B8 DF     查　詢
//	360  20 C2 F7 A1 40 B6 7D     離　開
const (
	MenuLeave = "\xc2\xf7\xb6\x7d"   // 離開
	MenuBuy   = "\xb6R\xb6i"         // 買進
	MenuSell  = "\xbd\xe6\xa5\x58"   // 賣出
	MenuQuery = "\xacd\xb8\xdf"      // 查詢
	MenuNext  = "\xa4U\xa4@\xad\xb6" // 下一頁

	// 銀行那張是另一組字（`PART1.PAK` 區段 2 的 178–180，三列，
	// 對得上 `rich2/docs/re/135` 的「銀行三列選單」）：
	//
	//	178  A6 73 B4 DA     存款
	//	179  BB E2 B4 DA     領款
	//	180  A9 F1 B1 F3     放棄
	//
	// ⚠ **離場的字不是每個場所都一樣**：股市寫「離開」，銀行寫「放棄」。
	// 只找「離開」會在銀行找不到，而找不到只會回 0——
	// 看起來像「這張選單沒有離場的項目」。用 `RowByExit` 一次試兩個。
	MenuDeposit  = "\xa6\x73\xb4\xda" // 存款
	MenuWithdraw = "\xbb\xe2\xb4\xda" // 領款
	MenuGiveUp   = "\xa9\xf1\xb1\xf3" // 放棄
)

// RowByExit 找出「離場」的那一列（1 起算）；找不到回 0。
//
// 依序試「離開」與「放棄」——**不同場所用不同的字**（股市是離開、
// 銀行是放棄）。只認一個會在另一個場所安靜地失敗。
func (s Selector) RowByExit(o *oracle.Oracle) int {
	for _, w := range []string{MenuLeave, MenuGiveUp} {
		if r := s.RowByText(o, w); r > 0 {
			return r
		}
	}
	return 0
}

// 頂端按鈕列的下拉：**不走全遊戲共用的那支選擇器**。
//
// 一開始以為它走 `0x204B6`（`rich2/docs/re/107` §5 講的是選擇器把參數往下
// 傳給 `#132`），實測攔不到——點「查詢」之後 `WatchSelectors` 一張都沒多。
// 追 `ds:186h`（`rich2/docs/re/108` §2 的「列數 − 2」）才看到呼叫端：
//
//	1159E  call 0CED:E122   ; ＝ 0x2AFF2，#132，只畫不收輸入
//	11631  call 0CED:263A   ; ★ ＝ 0x1F50A，十個參數，收輸入 ＋ 命中判定
//
// ⚠ **這個位址心算過一次，錯了**：`0x1CED0 + 0x263A` 是 `0x1F50A`，
// 不是 `0x2F50A`。掛錯的症狀是**一次都不觸發**，而畫下拉那一支
// （`0x2AFF2`）照樣觸發——看起來像「下拉畫出來了但沒有互動層」。
// `rich2/CLAUDE.md` §4.1 第 2 條就寫著「far call 目標不要心算」。
// 對得起來的旁證：`0x1F57E` 是這一支的滑鼠分支（`rich2/docs/spec/017`），
// 而它在 `0x1F50A` 後面 116 bytes；它呼叫的 `0CED:6624` ＝ `0x234F4`
// 正是選擇器也在用的那支命中判定。
//
// 幾何在呼叫端就算好了（`0x115C2`–`0x11610`）：
//
//	ds:18Ch = 12h(18)                        ; 間距
//	ds:18Eh = 136Eh[按鈕] × 16 + ds:15Eh + 2Ah(42)  ; ★ x1
//	ds:190h = 1340h[按鈕]                     ; 文字起始
//	ds:192h = 1312h[按鈕]                     ; 列數
//
// `x1 = x0 + 16 × 寬 + 42` 與選擇器的命中公式**同一條**（`docs/re/141` §1），
// 所以 `136Eh` 是**每列的全形字數**——`rich2/docs/re/108` 的表頭把它寫成
// 「游標欄」，那一欄的語意要訂正。
const IDATopBarMenu = 0x1F50A

// TopBarMenu 是一次頂端下拉的觀測。
type TopBarMenu struct {
	Step             uint64
	Rows             int // 列數（1312h[按鈕]）
	Text             int // 文字起始（1340h[按鈕]）
	X0, X1           int // 命中的左右界
	Y                int // 上緣
	Pitch            int // 列高，實測 18
	Chosen           int // 回傳之後才有
	Done             bool
}

// TopBarLog 收集整場的下拉。
type TopBarLog struct {
	All []TopBarMenu
	cur *TopBarMenu
}

// Open 回目前有沒有一張下拉在等輸入。
func (l *TopBarLog) Open() *TopBarMenu {
	if l == nil {
		return nil
	}
	return l.cur
}

// RowPoint 回第 row 列（1 起算）的點擊座標。
//
// 與選擇器同一條命中公式，所以取 x 的中線、y 取該列的中間。
func (m TopBarMenu) RowPoint(row int) (x, y int) {
	return (m.X0 + m.X1) / 2, m.Y + m.Pitch*(row-1) + m.Pitch/2
}

// Labels 讀每一列的文字（Big5 位元組，同 Selector.Labels）。
func (m TopBarMenu) Labels(o *oracle.Oracle) [][]byte {
	if m.Rows <= 0 || m.Text < 0 {
		return nil
	}
	a := Texts(o)
	out := make([][]byte, 0, m.Rows)
	for k := 0; k < m.Rows; k++ {
		i := m.Text + k
		if i < 0 || i >= TextSlots {
			out = append(out, nil)
			continue
		}
		out = append(out, trimTextSlot(a.Bytes(i)))
	}
	return out
}

// WatchTopBar 掛上頂端下拉的攔截。**要在 Run／Click 之前叫。**
func WatchTopBar(o *oracle.Oracle) *TopBarLog {
	log := &TopBarLog{}
	o.OnCall(o.IDA(IDATopBarMenu), func(o *oracle.Oracle) {
		// ⚠ **`CallArgs` 回的是「源碼推入順序」，不是堆疊順序。**
		// 它內部用 `StackWord(2 + (n-1-k))`，所以 `a[0]` 是**最先推**的那個。
		// 一開始照堆疊順序對，六個欄位全部錯位——而且錯得很像真的
		// （列數 0、x 348..−1），看起來像「參數個數猜錯」。
		//
		// 推入順序：17E 166 180 182 10A8 18C 15E 18E 190 192。
		// 實測值一一對得上：`17Eh=0`（還沒選）、`166h=20`（y 的複本）、
		// `180h=−1`（有滑鼠）、`182h=348`（查詢下拉的文字起始）、
		// `18Ch=18`（間距）。
		a := basic.CallArgs(o, 10)
		log.All = append(log.All, TopBarMenu{
			Step:  o.Steps(),
			Y:     int(int16(a[4])), // 10A8h
			Pitch: int(int16(a[5])), // 18Ch
			X0:    int(int16(a[6])), // 15Eh
			X1:    int(int16(a[7])), // 18Eh
			Text:  int(int16(a[8])), // 190h ＝ 1340h[按鈕]
			Rows:  int(int16(a[9])), // 192h ＝ 1312h[按鈕]
		})
		log.cur = &log.All[len(log.All)-1]
	})
	return log
}
