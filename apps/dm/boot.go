package dm

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// Load 載入最外層的殼 `dm.exe`。root 是玩家自備的原版資料目錄。
//
// DGROUP 這裡給 0（映像同段）沒有意義：位址全部屬於被 EXEC 兩層的 `fires`，
// 基底由 [Bind] 從 EXEC 紀錄算出來。
func Load(exe, root string) (*oracle.Oracle, error) {
	return oracle.LoadWith(exe, root, oracle.Options{})
}

// 熱區（320 座標）。
//
// ⚠ **選單項目的可點區域是圖示不是文字。** `ENTER` 是那顆綠寶石；
// 點在 `ENTER` 那幾個字上事件全都送到、回呼也跑了、**畫面毫無變化**，
// 看起來像「點擊沒送到」。
const (
	EnterX, EnterY = 250, 52
)

// Move 是六顆移動鈕之一。
//
// 指令編號來自 remake 那一側的滑鼠對照表（dungeon_master 專案
// `docs/spec/73` §5：`x 234–318`、`y 125–167` 那一塊的六格）；
// 這裡的座標是那六格各自的點，`tools/dosgolem-shots.sh` 在 DOS 原版上實跑過。
type Move int

const (
	TurnLeft Move = iota
	Forward
	TurnRight
	StrafeLeft
	Backward
	StrafeRight
)

var moveHot = [...][2]int{
	TurnLeft:    {247, 135},
	Forward:     {275, 135},
	TurnRight:   {304, 135},
	StrafeLeft:  {247, 156},
	Backward:    {275, 156},
	StrafeRight: {304, 156},
}

var moveName = [...]string{
	TurnLeft: "左轉", Forward: "前進", TurnRight: "右轉",
	StrafeLeft: "左移", Backward: "後退", StrafeRight: "右移",
}

func (m Move) String() string {
	if int(m) < len(moveName) {
		return moveName[m]
	}
	return fmt.Sprintf("Move(%d)", int(m))
}

// Hot 是這顆鈕的點擊座標。
func (m Move) Hot() (x, y int) { return moveHot[m][0], moveHot[m][1] }

// 開機的兩個檢查點，單位是**絕對步數**，由 `tools/dosgolem-shots.sh` 實測
// （`fires` 讀完 `GRAPHICS.DAT` 進到入口選單要三億多道指令）。
//
// ⚠ 用固定步數而不是「畫面穩定」當條件，是因為開場有動畫：
// `swoosh` 與 `title` 中間有好幾段畫面會停住，`ScreenIdle` 會提早成立。
// dosgolem 是決定性的，固定步數每次跑到同一個地方。
const (
	// EntranceSteps 是點 ENTER 的時機。
	EntranceSteps = 300_000_000
	// HallSteps 是點完之後再跑多久，畫面才穩定在招募大廳。
	HallSteps = 100_000_000
	// clickHold 是按住幾道指令。DM 全程只輪詢滑鼠 3 次，按住太短會整個被跳過
	// ——而畫面看起來只是「沒反應」。
	clickHold = 2_000_000
)

// ToEntrance 跑到入口選單（那一排寶石）。
func ToEntrance(o *oracle.Oracle) error {
	// `selector` 問顯示卡與音效，答案從 DOS 標準輸入餵進去。
	// 送錯的話畫面會停在問題上，而這裡只會跑滿預算。
	o.Type(SelectorAnswers)
	if err := o.RunUntil(oracle.Steps(EntranceSteps),
		oracle.Budget(EntranceSteps+1)); err != nil {
		return fmt.Errorf("跑到入口選單：%w", err)
	}
	return nil
}

// Boot 從 `dm.exe` 一路跑到招募大廳（點過入口選單的 ENTER）。
//
// ⚠ **這裡停的地方隊伍是空的**：四格都還沒有勇士，[Bridge.ReadParty]
// 會回 0 位。要有隊伍得走完招募流程（鏡子、選人），那是另一個切片。
func Boot(o *oracle.Oracle) error {
	if err := ToEntrance(o); err != nil {
		return err
	}
	if err := o.Click(EnterX, EnterY,
		oracle.NoCursorWait(),
		oracle.Hold(clickHold),
		oracle.Settle(HallSteps),
	); err != nil {
		return fmt.Errorf("點入口選單的 ENTER（%d,%d）：%w", EnterX, EnterY, err)
	}
	return nil
}

// click 是這一支程式共用的點擊時序：不等游標（DM 全程只輪詢滑鼠 3 次，
// `MouseSettled` 等不到）、按住 [clickHold] 道、再等 settle 道讓畫面畫完。
func (b *Bridge) click(x, y int, settle uint64) error {
	return b.O.Click(x, y,
		oracle.NoCursorWait(),
		oracle.Hold(clickHold),
		oracle.Settle(settle),
	)
}

// ClickMove 點一顆移動鈕。
//
// ⚠ **不檢查隊伍有沒有真的動**——那是呼叫端的事（路線的每一步都帶
// `expect` 座標，`docs/spec/198` §3.4）。這裡只負責把點擊送出去。
func (b *Bridge) ClickMove(m Move, settle uint64) error {
	x, y := m.Hot()
	if err := b.click(x, y, settle); err != nil {
		return fmt.Errorf("點%s（%d,%d）：%w", m, x, y, err)
	}
	return nil
}
