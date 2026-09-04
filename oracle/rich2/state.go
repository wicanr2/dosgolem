package rich2

import "github.com/wicanr2/dosgolem/oracle"

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
)

// 單一變數（`rich2/CLAUDE.md` §4.1）。
const (
	VarTile      = 0x01BE // 目前格號
	VarDirection = 0x10DE // 方向
	VarPlayer    = 0x10AA // 目前玩家（11A2h 的第一索引幾乎都用它）
)

// 玩家金錢陣列的欄位（`rich2/docs/re/013` §3）。
const (
	ColCash    = 0 // 現金，開局寫 25000
	ColDeposit = 1 // 存款，開局寫 25000
)

// MaxPlayers 是玩家槽數。第一維是 1..6。
const MaxPlayers = 6

// Money 開啟玩家金錢陣列。
func Money(o *oracle.Oracle) *oracle.Array {
	return o.Array(DescMoney,
		[]oracle.Dim{{Lo: 1, N: 6}, {Lo: 0, N: 60}}, 4)
}

// PlayerState 開啟玩家狀態陣列（16 位欄位，語意多數未解）。
func PlayerState(o *oracle.Oracle) *oracle.Array {
	return o.Array(DescPlayer,
		[]oracle.Dim{{Lo: 1, N: 6}, {Lo: 0, N: 30}}, 2)
}

// Land 開啟土地表。
func Land(o *oracle.Oracle) *oracle.Array {
	return o.Array(DescLand,
		[]oracle.Dim{{Lo: 0, N: 45}, {Lo: 0, N: 10}}, 2)
}

// Board 開啟棋盤陣列。
func Board(o *oracle.Oracle) *oracle.Array {
	return o.Array(DescBoard,
		[]oracle.Dim{{Lo: 0, N: 283}, {Lo: 0, N: 20}}, 2)
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

// Tile 回目前格號，Direction 回方向。
func Tile(o *oracle.Oracle) int      { return int(o.Word(o.DS(VarTile))) }
func Direction(o *oracle.Oracle) int { return int(o.Word(o.DS(VarDirection))) }
func Turn(o *oracle.Oracle) int      { return int(o.Word(o.DS(VarPlayer))) }
