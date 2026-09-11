package dm

import (
	"fmt"
	"strings"
)

// ParseMoves 把 `"f f f l r"` 這種記法解析成移動序列。
//
// 記法與 remake 專案的 `tools/dosgolem-shots.sh` 相同（`f` 前進、`b` 後退、
// `l` 左轉、`r` 右轉、`sl`／`sr` 側移），所以那邊記下來的序列可以直接抄過來。
func ParseMoves(s string) ([]Move, error) {
	var out []Move
	for _, tok := range strings.Fields(s) {
		m, ok := map[string]Move{
			"l": TurnLeft, "f": Forward, "r": TurnRight,
			"sl": StrafeLeft, "b": Backward, "sr": StrafeRight,
		}[strings.ToLower(tok)]
		if !ok {
			return nil, fmt.Errorf("不認得的動作 %q（要 f／b／l／r／sl／sr）", tok)
		}
		out = append(out, m)
	}
	return out, nil
}

// MustParseMoves 是 ParseMoves 的 panic 版，給套件層級的常數用。
func MustParseMoves(s string) []Move {
	m, err := ParseMoves(s)
	if err != nil {
		panic(err)
	}
	return m
}

// 招募流程（remake 專案
// `docs/verification/dosgolem-vs-dosbox-20260910.md`，在 DOS 原版上實跑過）。
//
// **為什麼巡檢非招募不可**：地板感應器**要隊伍裡有人才會觸發**
// （`SENSOR.C:474` 的 `PartyChampionCount == 0` 就跳過）。空隊伍走過壓力板
// 門不會開，路線走到一半就卡住——而畫面看起來只是「沒走那麼遠」。
var (
	// MirrorRoute 是從起點 `(1,3)` 面南走到 ELIJA 鏡子前的 31 步。
	MirrorRoute = MustParseMoves(
		"f f f f l f f f l f f f f f r f f r f l f f r f l f r f f f r")
	// StairsRoute 是招募之後走到側面樓梯 `(4,14)` 的 15 步。
	// 路上經過 `(6,9)`，那裡的壓力板會把 `(5,9)` 的門打開。
	StairsRoute = MustParseMoves("l f f r f f f f f l f f f f f")
)

// 招募面板的熱區（320 座標）。
//
// 指令編號在 remake 專案 `docs/spec/73` §5：`RESURRECT` 是 160
// （`x 104–158`、`y 86–142`），`REINCARNATE` 161（`x 163–217`），
// `CANCEL` 162（`y 146–156`）。
const (
	// MirrorX／MirrorY 點的是鏡子裡的勇士本人，不是鏡框。
	MirrorX, MirrorY = 112, 75
	// ResurrectX／ResurrectY 點下去畫面會出現 `ELIJA RESURRECTED.`。
	ResurrectX, ResurrectY = 130, 114
)

// ActGap 是兩個動作之間要跑多少道指令。
//
// ⚠ **畫面重繪要時間。** 太密的話後一個動作會點在還沒畫完的畫面上，
// 而那與「熱區點錯」的結果一樣是「畫面沒變」。3000 萬是 remake 專案
// `tools/dosgolem-shots.sh` 實測出來的值。
const ActGap = 30_000_000

// Walk 依序點一串移動鈕。
//
// ⚠ **不檢查隊伍有沒有真的動到預期的格子**——那是路線的事
// （每一步的 `expect`，`docs/spec/198` §3.4）。這裡只把點擊送出去。
func (b *Bridge) Walk(moves []Move) error {
	for i, m := range moves {
		if err := b.ClickMove(m, ActGap); err != nil {
			return fmt.Errorf("第 %d 步（%s）：%w", i+1, m, err)
		}
	}
	return nil
}

// RecruitFirstChampion 在勇士之廳招募第一位（ELIJA）：點鏡子，再點 RESURRECT。
//
// 呼叫前隊伍要已經站在鏡子前（[Bridge.Walk] 走完 [MirrorRoute]）。
func (b *Bridge) RecruitFirstChampion() error {
	if err := b.click(MirrorX, MirrorY, ActGap); err != nil {
		return fmt.Errorf("點鏡子：%w", err)
	}
	if err := b.click(ResurrectX, ResurrectY, ActGap); err != nil {
		return fmt.Errorf("點 RESURRECT：%w", err)
	}
	return nil
}

// 門鈕的點擊座標（320×200 螢幕座標系）。
//
// 來源是原版資料：門鈕在 D1C（正前方）畫完之後，原版會把**繪製框拷進
// 點擊框表**（remake 專案 `docs/re/66` §3），所以繪製框就是點擊框。
// DOS 版的框是 `x 167–174`、`y 43–51`（視窗座標），
// 地城視窗在螢幕上的原點是 `(0,33)`，所以中心落在 `(170,80)`。
// 用 remake 的 `dmtool dungeon doorbutton` 印得出來。
//
// ⚠ **DOS 與 ST 的框不一樣**（ST 是 `x 160–175`，中心 `(167,81)`）：
// 兩版的門鈕點陣圖寬度不同。拿 ST 的座標點 DOS 版**還是會落在按鈕上**，
// 但別因此以為兩張表通用——ST 的表在 DOS 素材包裡是全零。
//
// ⚠ **只有正前方（D1C）的門按得到。** 其他視位畫得出按鈕但點不到，
// 這與 remake 的 `World.PressDoorButton` 要求隊伍面向那扇門是同一回事。
//
// ⚠ **不是每扇門都有按鈕。** 第 2 層 25 扇裡只有 8 扇；沒有按鈕的要鑰匙
// 或打破，點下去不會有任何反應——而「點了沒反應」與「座標錯了」在畫面上
// 長得一模一樣。
const (
	DoorButtonX, DoorButtonY = 170, 80
)

// PressDoorButton 點正前方那扇門上的按鈕。
//
// ⚠ **呼叫前隊伍要面向那扇門。** 這一支不檢查——面前沒有門、或那扇門沒有
// 按鈕時，原版兩種情況都不會有反應，`Click` 會回 `NoResponseError`。
func (b *Bridge) PressDoorButton() error {
	if err := b.click(DoorButtonX, DoorButtonY, ActGap); err != nil {
		return fmt.Errorf("點門鈕（%d,%d）：%w", DoorButtonX, DoorButtonY, err)
	}
	return nil
}
