package rich2

import (
	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/runtime/basic"
)

// 棋盤上的遊戲狀態。
//
// 位址與維度取自 `rich2/docs/re/014` §2（DIM 對照表）與 `docs/re/013`
// （玩家陣列的欄位）。**這裡只放已經有出處的**，沒解出來的欄位不猜。

// 陣列描述子位址。
const (
	DescMoney  = 0x11A2 // 玩家金錢與屬性 1..6 × 0..59，4B
	DescPlayer = 0x1146 // 玩家狀態 1..6 × 0..29，2B
	DescLand   = 0x1174 // 土地表 0..44 × 0..9，2B
	DescBoard  = 0x122C // 棋盤 0..282 × 0..19，2B
	DescCoord  = 0x11FE // 座標 → 格號對照表 0..35 × 0..35，2B（不在存檔裡）
	DescDraw   = 0x11D0 // 顯示層：每格要畫什麼 0..35 × 0..35，2B
	DescPos    = 0x125A // 玩家位置層 0..35 × 0..35，2B
)

// 單一變數（`rich2/CLAUDE.md` §4.1、`rich2/docs/re/015`、`docs/re/014` §3）。
const (
	VarTile      = 0x01BE // 目前格號
	VarDirection = 0x10DE // 方向
	VarPlayer    = 0x10AA // 目前玩家（11A2h 的第一索引幾乎都用它）

	// 擲骰（`rich2/docs/re/015` §187–196）。
	VarDiceRaw  = 0x01AE // 擲骰常式的回傳
	VarSteps    = 0x01B0 // 步數 ＝ 兩顆骰子的和
	VarStepLoop = 0x01B6 // 移動迴圈的計數（從 VarSteps 複製過來）

	// 抽方向（`rich2/docs/re/014` §312）。
	VarDirPick = 0x01C0 // 候選方向 ＝ INT(RND×4)+1，1..4
)

// 玩家金錢陣列的欄位（`rich2/docs/re/013` §3）。
const (
	ColCash    = 0 // 現金，開局寫 25000
	ColDeposit = 1 // 存款，開局寫 25000
)

// MaxPlayers 是玩家槽數。第一維是 1..6。
const MaxPlayers = 6

// Money 開啟玩家金錢陣列。
func Money(o *oracle.Oracle) *basic.Array {
	return basic.NewArray(o, DescMoney,
		[]basic.Dim{{Lo: 1, N: 6}, {Lo: 0, N: 60}}, 4)
}

// PlayerState 開啟玩家狀態陣列（16 位欄位，語意多數未解）。
func PlayerState(o *oracle.Oracle) *basic.Array {
	return basic.NewArray(o, DescPlayer,
		[]basic.Dim{{Lo: 1, N: 6}, {Lo: 0, N: 30}}, 2)
}

// Land 開啟土地表。
func Land(o *oracle.Oracle) *basic.Array {
	return basic.NewArray(o, DescLand,
		[]basic.Dim{{Lo: 0, N: 45}, {Lo: 0, N: 10}}, 2)
}

// 棋盤陣列 `122Ch` 的欄位（`rich2/internal/assets/board.go`、
// `rich2/docs/re/016` §82、`docs/re/184` §2.1）。
const (
	ColMapRow = 0  // 在 36×36 地圖上的列
	ColMapCol = 1  // 同上，欄
	ColKind   = 2  // 非街道格的種類 0–10
	ColLink   = 4  // 4–7 是四個方向的鄰接
	ColStreet = 8  // 土地編號，0 表示不是土地
	ColOrder  = 9  // 該格在街道內的序號
	ColOwner  = 12 // 地主編號，0 ＝ 無主（`docs/re/184` §2.1，強證據）
	ColLevel  = 15 // 建物等級 0–5（`docs/re/016` §82，confirmed）
)

// Owner／Street／Level 讀某一格的地主、街道編號、建物等級。
func Owner(o *oracle.Oracle, square int) int {
	return int(Board(o).Int16(square, ColOwner))
}

func Street(o *oracle.Oracle, square int) int {
	return int(Board(o).Int16(square, ColStreet))
}

func Level(o *oracle.Oracle, square int) int {
	return int(Board(o).Int16(square, ColLevel))
}

// StreetLevels 回「同一條街上、同一位地主名下」每一格的建物等級。
//
// 原版的租金就是這樣算的：不是只看踩到的那一格，而是同街同主的每一格
// 各按自己的等級查表再加總（`rich2/docs/spec/004` §0.1）。
func StreetLevels(o *oracle.Oracle, square int) (street int, levels []int) {
	bd := Board(o)
	street = int(bd.Int16(square, ColStreet))
	if street == 0 {
		return 0, nil
	}
	owner := int(bd.Int16(square, ColOwner))
	if owner == 0 {
		return street, nil
	}
	for i := 0; i < 283; i++ {
		if int(bd.Int16(i, ColStreet)) != street {
			continue
		}
		if int(bd.Int16(i, ColOwner)) != owner {
			continue
		}
		levels = append(levels, int(bd.Int16(i, ColLevel)))
	}
	return street, levels
}

// Coord 開啟「座標 → 格號」對照表。
//
// 原版把玩家的位置存成 36×36 地圖上的座標，格號是查這張表算出來的
// （`rich2/docs/re/014` §3a）。
func Coord(o *oracle.Oracle) *basic.Array {
	return basic.NewArray(o, DescCoord,
		[]basic.Dim{{Lo: 0, N: 36}, {Lo: 0, N: 36}}, 2)
}

// Board 開啟棋盤陣列。
func Board(o *oracle.Oracle) *basic.Array {
	return basic.NewArray(o, DescBoard,
		[]basic.Dim{{Lo: 0, N: 283}, {Lo: 0, N: 20}}, 2)
}

// Cash／Deposit 讀某個玩家的現金與存款。player 從 1 起。
func Cash(o *oracle.Oracle, player int) int32 {
	return Money(o).Int32(player, ColCash)
}

func Deposit(o *oracle.Oracle, player int) int32 {
	return Money(o).Int32(player, ColDeposit)
}

// ActivePlayers 回「有錢的玩家槽」——開局時沒人的槽是 0。
//
// ⚠ **這是啟發式的，不是權威判準。** 真正的「這個位子有沒有人」在
// `10EAh`（`rich2/docs/re/013`），但那個陣列同時被別的東西用，
// 語意還沒完全解開。破產的玩家現金也可能是 0。
func ActivePlayers(o *oracle.Oracle) []int {
	m := Money(o)
	var out []int
	for i := 1; i <= MaxPlayers; i++ {
		if m.Int32(i, ColCash) != 0 || m.Int32(i, ColDeposit) != 0 {
			out = append(out, i)
		}
	}
	return out
}

// 玩家狀態陣列的欄位。
//
// 欄 0／1 是玩家在 36×36 地圖上的座標（`rich2/docs/spec/014` §1a 第 3 項），
// **不是格號**。格號要拿座標查 `11FEh`——實測 `1146h(1,0)=21`、
// `1146h(1,1)=8`，而 `11FEh(21,8)=117`，正是那一步走到的格。
const (
	ColRow = 0
	ColCol = 1

	// ColJail／ColHospital／ColFrozen 是被關著的剩餘天數
	// （`rich2/docs/re/165` §17：`FOR ds:84h = 15 DOWNTO 13`，
	// `ds:EEh = ds:84h − 12` → 1 監獄／2 醫院／3 冬眠）。
	//
	// ⚠ **被關著的時候點「前進」不會擲骰。** 連走多步時遇到它，
	// 症狀是「等棋子動」跑滿上限——看起來像操作失效。
	ColJail     = 13
	ColHospital = 14
	ColFrozen   = 15

	// ColDir 是玩家自己的目前方向（1..4）。
	//
	// ⚠ **不要用 `ds:10DEh`。** 那是全域的「目前玩家的方向」，
	// 回合一推進就變成別人的——同一個形狀在 `ds:1BE`（格號）與
	// `ds:1B0h`（骰子點數）上都踩過。
	//
	// 判準：第一步走完之後 `1146h(1,2)` 是 2，而下一次輪到玩家 1 時
	// `ds:10DEh` 讀出來也是 2。
	ColDir = 2
)

// Position 回某個玩家目前的格號。
//
// ⚠ **不要用 `ds:1BE`。** 那是「目前正在處理的格號」，會隨著**任何**玩家
// 的移動而變——AI 在走的時候它也在跳。實測：人類走完停在 117，
// 下一次輪到人類時 `ds:1BE` 讀出來是 61（那是別人的）。
func Position(o *oracle.Oracle, player int) int {
	ps := PlayerState(o)
	row := int(ps.Int16(player, ColRow))
	col := int(ps.Int16(player, ColCol))
	if row < 0 || row >= 36 || col < 0 || col >= 36 {
		return 0
	}
	return int(Coord(o).Int16(row, col))
}

// MapCoord 回某個玩家在 36×36 地圖上的座標。
func MapCoord(o *oracle.Oracle, player int) (row, col int) {
	ps := PlayerState(o)
	return int(ps.Int16(player, ColRow)), int(ps.Int16(player, ColCol))
}

// Steps 回上一次擲骰的步數（兩顆骰子的和）。
//
// **這是移動 parity 的關鍵輸入。** 有了它，remake 不必重現原版的亂數消耗
// 次數（原版走一步抽數十次，因為擲骰動畫每一幀都真的擲）——
// 直接餵同樣的點數就能比「同一步走到同一格」。
func Steps(o *oracle.Oracle) int { return int(o.Word(o.DS(VarSteps))) }

// DiceRaw 回擲骰常式的原始回傳，DirPick 回最後一次抽到的候選方向。
func DiceRaw(o *oracle.Oracle) int { return int(o.Word(o.DS(VarDiceRaw))) }
func DirPick(o *oracle.Oracle) int { return int(o.Word(o.DS(VarDirPick))) }

// Held 回某個玩家被關著的剩餘天數。三個都是 0 才動得了。
func Held(o *oracle.Oracle, player int) (jail, hospital, frozen int) {
	ps := PlayerState(o)
	return int(ps.Int16(player, ColJail)),
		int(ps.Int16(player, ColHospital)),
		int(ps.Int16(player, ColFrozen))
}

// IsHeld 回「這個玩家現在動不了」。
func IsHeld(o *oracle.Oracle, player int) bool {
	j, h, f := Held(o, player)
	return j > 0 || h > 0 || f > 0
}

// PlayerDir 回某個玩家自己的目前方向（1..4）。
func PlayerDir(o *oracle.Oracle, player int) int {
	return int(PlayerState(o).Int16(player, ColDir))
}

// Tile 回 `ds:1BE`——**「目前正在處理的格號」，不是某個玩家的位置**。
// 要玩家位置用 Position。
func Tile(o *oracle.Oracle) int      { return int(o.Word(o.DS(VarTile))) }
func Direction(o *oracle.Oracle) int { return int(o.Word(o.DS(VarDirection))) }
func Turn(o *oracle.Oracle) int      { return int(o.Word(o.DS(VarPlayer))) }
