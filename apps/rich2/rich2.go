// Package rich2 是《大富翁2》(1993) 專屬的導航：從冷啟動走到棋盤。
//
// 這些是**遊戲專屬知識**，所以不放在 oracle 核心裡——那一層是契約，要窄。
// 這裡放「這個 binary 的哪個畫面要怎麼過」。
//
//	o, _ := oracle.Load(exe, root)
//	rich2.ToBoard(o)          // 冷啟動 → 防拷 → 主選單 → 棋盤
//	shot := o.Indexed()
//
// ⚠ **不含任何原版檔案**，素材由玩家自備。
package rich2

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// 四個色塊的中心（`rich2/docs/re/005`「色塊的精確位置」實測）。
const swatchX = 102

var swatchY = []int{125, 143, 161, 179} // 綠、黃、紅、藍

// 地圖在畫面左側；標題文字也是白的，所以掃描要避開上緣。
const (
	mapTop, mapBottom = 25, 195
	mapLeft, mapRight = 5, 92
)

// SolvePassword 答完防拷的三題。
//
// 答案在紙本說明書上，程式裡沒有——所以這裡**從快照展開四個變體**，
// 看哪一個被接受。快照是 1 毫秒，比拿說明書掃描圖做影像比對簡單也可靠
// （`rich2/tools/dosbox_play.py` 走的是後者）。
//
// ⚠ **判準是留白區跑到哪，不是畫面變了幾點。** 答對會進下一題，而下一題的
// 佈局完全相同——只有留白的縣市與標題「密碼圖之X」不一樣，那大概幾百點，
// 與「答錯多印一行訊息」同一個量級。
func SolvePassword(o *oracle.Oracle) error {
	if err := o.RunUntil(oracle.PasswordScreen()); err != nil {
		return fmt.Errorf("走到防拷畫面：%w", err)
	}
	for round := 1; round <= 4; round++ {
		blank := FindBlank(o)
		if blank.N == 0 {
			return nil // 已經不在防拷畫面
		}
		if err := answerOne(o, blank); err != nil {
			return fmt.Errorf("第 %d 題：%w", round, err)
		}
	}
	return fmt.Errorf("答了四題還在防拷畫面")
}

func answerOne(o *oracle.Oracle, blank Region) error {
	snap := o.Save()
	for _, y := range swatchY {
		o.Restore(snap)
		// 兩步作答：先選色，再點留白區。
		if err := o.Click(swatchX, y); err != nil {
			continue
		}
		// 點留白區「當下沒有回饋」不代表沒收到——答錯的訊息要一會兒才畫出來。
		_ = o.Click(blank.CX, blank.CY)
		if err := o.Run(20_000_000); err != nil {
			return err
		}
		after := FindBlank(o)
		if after.N == 0 || after.CX != blank.CX || after.CY != blank.CY {
			return nil // 換題或過關
		}
	}
	return fmt.Errorf("四個顏色都不被接受")
}

// Region 是畫面上的一塊區域。
type Region struct {
	CX, CY, N int
	Colour    uint8
}

// 縣市的色號範圍。台灣地圖上每個縣市都有自己的色號，實測落在 192–206。
const (
	countyFirst = 192
	countyLast  = 206
)

// FindBlank 找地圖上被留白的那一區。
//
// ⚠ **判準是調色盤，不是色號。**
//
// 每個縣市都有自己的色號（`countyFirst`–`countyLast`），而**調色盤把它們
// 全部設成同一個綠 (0,130,0)**，只有這一題要問的那一區設成白。
// 換句話說「哪一區被留白」是調色盤的狀態，不是像素的狀態——
// 直接找色號 15（一般的白）會找到畫面上的白色文字，不是縣市。
//
// ⚠⚠ **色號範圍這一條是必要的，不是保險。**
//
// 本函式原本只看「調色盤是不是白」，沒有限制色號，理由是註解裡寫的
// 「色號 15 一個點都找不到」。**那句話只在一個已經修掉的 bug 底下成立**：
// `int 10h AH=10h` 的 `AL=10h`（設單一 DAC）先前被接成「設屬性暫存器」，
// 所以色號 15 的 DAC 從來沒被寫成白。`769b030`（spec 011）把 AL 分支接對
// 之後，色號 15 變成真正的白，於是 `SolvePassword` 在**三題都答對之後**
// 把畫面上的白色文字（實測 67 點）當成第四題的留白區，
// 然後回報「四個顏色都不被接受」。
//
// **共用層變正確了，是這裡的假設過期。** 教訓寫成規則：
// 「某個東西從來不出現」如果沒有結構上的理由，就不能當判準——
// 它只是還沒出現。
func FindBlank(o *oracle.Oracle) Region {
	return findBlankIn(o.Indexed(), o.Palette())
}

// findBlankIn 是 FindBlank 的純函式版本，好讓判準本身有不必開模擬器的測試。
func findBlankIn(px []uint8, pal [256][3]uint8) Region {
	var sx, sy, n int
	var colour uint8
	for y := mapTop; y < mapBottom; y++ {
		for x := mapLeft; x < mapRight; x++ {
			i := y*oracle.Width + x
			if i >= len(px) {
				continue
			}
			c := px[i]
			if c < countyFirst || c > countyLast {
				continue
			}
			if pal[c][0] > 240 && pal[c][1] > 240 && pal[c][2] > 240 {
				sx, sy, n, colour = sx+x, sy+y, n+1, c
			}
		}
	}
	if n == 0 {
		return Region{}
	}
	return Region{sx / n, sy / n, n, colour}
}

// ToMainMenu 過完防拷，跑到主選單（標題畫面 ＋「單人遊戲／多人遊戲／遊戲說明」）。
//
// 路標是 RICHT.RIX——主選單的配樂（`rich2/docs/re/173`）。
// 用開檔當路標比等指令數穩，比看畫面早。
func ToMainMenu(o *oracle.Oracle) error {
	if err := SolvePassword(o); err != nil {
		return err
	}
	if err := o.RunUntil(oracle.Opened("RICHT.RIX"),
		oracle.Budget(120_000_000)); err != nil {
		return fmt.Errorf("走到主選單：%w", err)
	}
	// 配樂開了之後畫面還要一會兒才畫完。
	return o.RunUntil(oracle.ScreenIdle(8_000_000), oracle.Budget(60_000_000))
}

// cycling 是**循環動畫**的色號，比較調色盤時要跳過。
//
// 240–249 與 250–254 一直在轉（`rich2/docs/re/146` §4.1），
// 不跳過的話「等調色盤穩定」永遠等不到。
func cycling(i int) bool { return i >= 240 && i <= 254 }

// ToBoard 從冷啟動一路走到棋盤：防拷 → 主選單 → 單人遊戲 → 新局。
//
// 選單第一項就是「單人遊戲」，所以送一個 Enter 就進去
// （原版選擇器只認 Enter／ESC／上下鍵，`rich2/docs/re/100`）。
//
// ⚠ **鍵盤不走 `int 16h`**，走 `int 21h AH=3Fh` 讀 handle 0
// （BASIC 的 INKEY$，`rich2/docs/re/005`「輸入路徑」）。
//
// ⚠ **要等調色盤淡入走完**（`rich2/docs/re/146`）。太早取畫面會拿到淡到
// 一半的版本——顏色整片偏暗，而色號完全正確，所以逐點比對照樣可能過。
func ToBoard(o *oracle.Oracle) error {
	if err := ToMainMenu(o); err != nil {
		return err
	}
	o.Type("\r")
	// RICHA.RIX 是棋盤的輪替配樂之一，SAVE_7.DSK 是新局的空白樣板。
	if err := o.RunUntil(oracle.Opened("RICHA.RIX"),
		oracle.Budget(120_000_000)); err != nil {
		return fmt.Errorf("進棋盤：%w", err)
	}
	// 淡入走完為止。
	//
	// ⚠ **等調色盤，不要等畫面。** 棋盤上的角色一直在動，`ScreenIdle`
	// 在這裡跑不完（實測跑滿兩億道指令仍未達成）。
	if err := o.RunUntil(oracle.PaletteIdle(20_000_000, cycling),
		oracle.Budget(250_000_000)); err != nil {
		return err
	}
	// ⚠ **再等到遊戲真的開始讀滑鼠。**
	//
	// 淡入走完的時候遊戲還在跑收尾，那段期間一次都不輪詢滑鼠——
	// 這時點下去等於沒點，而按鈕還是會反白（那是遊戲自己畫的），
	// 所以看起來像「點到了但遊戲不理」。
	return o.RunUntil(oracle.MousePolled(50), oracle.Budget(400_000_000))
}
