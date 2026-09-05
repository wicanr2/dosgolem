package rich2

import (
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

	Chosen int  // 玩家選了第幾列（1 起算）；0 ＝ 還沒選或沒選
	Done   bool // 有沒有攔到回傳
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
		switch row := log.pick(log.cur); {
		case row < 0: // 不回答
		case row == 0:
			o.Type(KeyEsc)
		default:
			_ = ChooseRow(o, log.cur, row)
		}
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

// ChooseRow 送出「選第 row 列」的按鍵序列。
//
// **走的是正常玩家路徑**：游標從第 1 列起算，往下按 row−1 次再按 Enter。
// 不直接寫選擇結果——那會跳過遊戲自己的游標處理，證明不了玩家走得到
// （`rich2/CLAUDE.md` §8）。
//
// row 從 1 起算。超出範圍回錯誤而不是硬送，因為送過頭之後游標停在哪裡
// 取決於選擇器有沒有 wrap，那還沒查。
func ChooseRow(o *oracle.Oracle, s *Selector, row int) error {
	if s == nil {
		return fmt.Errorf("現在沒有選單在等輸入")
	}
	if row < 1 || row > s.Rows {
		return fmt.Errorf("選第 %d 列，但這張選單只有 %d 列", row, s.Rows)
	}
	for i := 1; i < row; i++ {
		o.Type(KeyDown)
	}
	o.Type(KeyEnter)
	return nil
}
