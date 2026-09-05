package rich2

import (
	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/runtime/basic"
)

// 落地分派器。
//
// `rich2/docs/re/018`：落地結算把 `棋盤[格][2]`（格子種類）存進 `ds:2FAh`，
// 然後 `call 1351A`；`1351A` 開頭就是 `mov bx, ds:2FAh`，按它 `ON x GOTO`
// 到十一種格子之一（`rich2/docs/re/020` §1–2）。
//
// **攔這一支就夠了，不必掛十一個進入點。** 掛進入點會踩到分派表的
// fall-through：公園（種類 0）落在 `13574`，往下會執行到銀行（種類 1）的
// 那道 `ret`，於是進一次公園記到兩筆。
//
// 更重要的是**運氣格會把事件的動作碼寫進 `ds:2FAh`、再叫一次分派器**
// （`rich2/docs/re/020` §4，`13665`）——所以同一個攔截點也看得到事件，
// 用 `Caller` 分得出是哪一種呼叫端。
//
// ⚠ 位址出自 `RUN_unpacked.EXE` 的筆記，但**程式碼在 `3AE58` 以下兩個檔
// 完全一樣**（`rich2/CLAUDE.md` §4.1 第 1 條），這兩個位址都在那之下。
const (
	IDADispatch = 0x1351A // 分派器入口
	IDALuckAgain = 0x13665 // 運氣格再次呼叫分派器的那一處
	VarKind     = 0x02FA  // 分派器的輸入：格子種類，或運氣事件的動作碼
)

// 十一種非街道格的種類編號（`rich2/docs/re/020` §2）。
const (
	KindPark   = 0  // 公園（落地不做事）
	KindBank   = 1  // 銀行（落地不做事，路過才有效果）
	KindLuck   = 2  // 運氣
	KindCard   = 3  // 卡片
	KindNews   = 4  // 新聞
	KindStock  = 5  // 股市
	KindCourt  = 6  // 法院
	KindMarket = 7  // 黑市
	KindCasino = 8  // 賭場
	KindArcade = 9  // 遊樂場
	KindTax    = 10 // 稅捐處
)

// KindNames 是十一種格子的名字，索引就是種類編號。
var KindNames = []string{
	"公園", "銀行", "運氣", "卡片", "新聞",
	"股市", "法院", "黑市", "賭場", "遊樂場", "稅捐處",
}

// KindName 回種類的名字；超出十一種（運氣事件的動作碼會）回空字串。
func KindName(kind int) string {
	if kind < 0 || kind >= len(KindNames) {
		return ""
	}
	return KindNames[kind]
}

// Dispatch 是分派器被呼叫的一次。
type Dispatch struct {
	Code   int    // ds:2FAh：落地是格子種類 0–10，運氣再分派時是事件的動作碼
	Name   string // 種類名，動作碼沒有名字就是空字串
	FromLuck bool // 是不是運氣格再叫的那一次（呼叫端 13665）
	Player int    // 輪到誰（含 AI）
	Square int    // ds:1BE
	Cash   int32
	Step   uint64
}

// DispatchLog 收集分派紀錄。
type DispatchLog struct{ Calls []Dispatch }

// WatchDispatch 掛上落地分派器，之後每一次落地與每一次事件再分派都記一筆。
//
// hook 在**任何** `RunUntil` 期間都會觸發（`PlayTurn` 也是），所以掛好之後
// 一路走下去就行，不必先想辦法走到某一格。
//
// ⚠ **AI 的回合也會記。** 要只看某個玩家，用 `Call.Player` 篩，
// 或用 `Since` 切出自己那一步的時間窗。
func WatchDispatch(o *oracle.Oracle) *DispatchLog {
	log := &DispatchLog{}
	luck := o.IDA(IDALuckAgain)
	o.OnCall(o.IDA(IDADispatch), func(o *oracle.Oracle) {
		code := int(int16(o.Word(o.DS(VarKind))))
		player := Turn(o)
		log.Calls = append(log.Calls, Dispatch{
			Code: code, Name: KindName(code),
			FromLuck: o.Caller() == luck,
			Player:   player, Square: Tile(o),
			Cash: Cash(o, player), Step: o.Steps(),
		})
	})
	return log
}

// Count 回某一種被分派幾次。
func (l *DispatchLog) Count(code int) int {
	n := 0
	for _, c := range l.Calls {
		if c.Code == code {
			n++
		}
	}
	return n
}

// Since 回第 n 筆之後的紀錄（n 通常是上一步結束時的 len）。
func (l *DispatchLog) Since(n int) []Dispatch {
	if n < 0 || n > len(l.Calls) {
		return nil
	}
	return l.Calls[n:]
}

// ---- 卡片 ----------------------------------------------------------------

// DescDeck 是卡片牌堆的描述子（`rich2/docs/re/014` §55，0..119、2B）。
const DescDeck = 0x13CA

// DeckSize 是牌堆真正在用的格數。陣列宣告到 119，但抽卡只取前 100 格
// （`INT(RND × 100)`，`rich2/docs/re/048`）。
const DeckSize = 100

// 手牌在玩家狀態陣列的欄 20–28（`rich2/docs/re/048`：找空槽，
// 全滿時預設用欄 28）。
const (
	ColHand0 = 20
	ColHandN = 28
)

// Deck 開啟牌堆陣列。
func Deck(o *oracle.Oracle) *basic.Array {
	return basic.NewArray(o, DescDeck, []basic.Dim{{Lo: 0, N: 120}}, 2)
}

// DeckCards 回牌堆前 100 格的卡片編號（0 ＝ 這一格被抽走了）。
func DeckCards(o *oracle.Oracle) []int {
	a := Deck(o)
	out := make([]int, DeckSize)
	for i := range out {
		out[i] = int(a.Int16(i))
	}
	return out
}

// Hand 回某個玩家的手牌九格（0 ＝ 空槽）。
func Hand(o *oracle.Oracle, player int) []int {
	ps := PlayerState(o)
	out := make([]int, 0, ColHandN-ColHand0+1)
	for c := ColHand0; c <= ColHandN; c++ {
		out = append(out, int(ps.Int16(player, c)))
	}
	return out
}
