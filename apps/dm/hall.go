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
