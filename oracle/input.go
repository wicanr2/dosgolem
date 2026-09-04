package oracle

import "fmt"

// 輸入（`docs/spec/005` §4）。
//
// **一切對齊指令數，不對齊牆上的時鐘。** rich2 現在 54 支腳本的瓶頸不是
// IPC 是 sleep——0.058 秒的 docker exec 旁邊掛著 0.35–2.2 秒的等待
// （`rich2/docs/spec/082` §1）。那些 sleep 存在的唯一理由是「主機沒辦法問
// 模擬器你跑完了沒」。在這裡可以問，所以不要再猜。

// MoveMouse 把游標移到某個像素座標。
//
// ⚠ **要在程式的 `AX=4` 之後叫**，否則會被它蓋掉而且畫面看起來完全正常。
// 用 Click 的話已經幫你等了。
func (o *Oracle) MoveMouse(x, y int) {
	o.d.Mouse.X, o.d.Mouse.Y = uint16(x), uint16(y)
}

// Mouse 回目前的游標座標。
func (o *Oracle) Mouse() (x, y int) {
	return int(o.d.Mouse.X), int(o.d.Mouse.Y)
}

// ClickOpt 調整一次點擊。
type ClickOpt func(*clickCfg)

type clickCfg struct{ hold, settle uint64 }

// Hold 改按住的指令數。
func Hold(n uint64) ClickOpt { return func(c *clickCfg) { c.hold = n } }

// Settle 改放開之後再跑多久（讓遊戲把回饋畫出來）。
func Settle(n uint64) ClickOpt { return func(c *clickCfg) { c.settle = n } }

// Click 在某個像素座標點一下：移動 → 按下 → 按住 → 放開 → 等畫面回應。
//
// 三件事是實測出來的，每一件的反面都不會報錯：
//
//  1. **按住不能短。** 遊戲輪詢 `int 33h` 的頻率很低，按下與放開隔太近會
//     整個被跳過——DOSBox 那邊同一題要點三次才生效一次
//     （`rich2/docs/playtest/001` §5.6）。
//  2. **要等程式設過游標位置**（`AX=4`）才移動，見 MoveMouse。
//  3. **回 error**：點了畫面完全沒動要說出來，不要讓呼叫端拿「畫面沒變」
//     去猜是點錯位置還是遊戲還沒準備好。
func (o *Oracle) Click(x, y int, opts ...ClickOpt) error {
	cfg := clickCfg{hold: DefaultHold, settle: DefaultHold}
	for _, f := range opts {
		f(&cfg)
	}
	if len(o.d.Mouse.Sets) == 0 {
		if err := o.RunUntil(MouseSettled); err != nil {
			return fmt.Errorf("點 (%d,%d) 之前等程式設游標位置：%w", x, y, err)
		}
	}

	before := o.Indexed()
	o.MoveMouse(x, y)
	o.d.Mouse.Buttons = 1
	o.d.Mouse.Press++
	if err := o.Run(cfg.hold); err != nil {
		return fmt.Errorf("點 (%d,%d) 按住期間：%w", x, y, err)
	}
	o.d.Mouse.Buttons = 0
	o.d.Mouse.Release++
	if err := o.Run(cfg.settle); err != nil {
		return fmt.Errorf("點 (%d,%d) 放開之後：%w", x, y, err)
	}

	if sameBytes(before, o.Indexed()) {
		return &NoResponseError{X: x, Y: y, Polls: len(o.d.Mouse.Polls)}
	}
	return nil
}

// NoResponseError 是「點了但畫面一點都沒變」。
//
// 這在 dosemu.py（Python + unicorn 那一版）是個死結：輪詢讀到了正確座標
// 卻畫面不動，查了一整輪（`rich2/docs/re/005`「防拷：輸入確實送到了」）。
// 把它做成具名錯誤，是為了讓下一次遇到時**立刻知道是這個形狀**。
type NoResponseError struct {
	X, Y  int
	Polls int
}

func (e *NoResponseError) Error() string {
	return fmt.Sprintf("點 (%d,%d) 之後畫面完全沒變（期間滑鼠被輪詢 %d 次）"+
		"——不是座標不對，就是遊戲還沒準備好收這個點擊", e.X, e.Y, e.Polls)
}

// Type 把字串排進鍵盤佇列，餵給 `int 21h AH=3Fh`（BASIC 的 INKEY$）。
//
// ⚠ **遊戲的鍵盤輸入不走 `int 16h`**，走讀 handle 0
// （`rich2/docs/re/005`「輸入路徑」）。
func (o *Oracle) Type(s string) {
	o.d.Stdin = append(o.d.Stdin, []byte(s)...)
}

// Pending 回還沒被讀走的鍵數。
func (o *Oracle) Pending() int { return len(o.d.Stdin) }
