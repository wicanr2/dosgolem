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
	oldX, oldY := o.d.Mouse.X, o.d.Mouse.Y
	o.d.Mouse.X, o.d.Mouse.Y = uint16(x), uint16(y)
	if oldX != o.d.Mouse.X || oldY != o.d.Mouse.Y {
		o.d.MouseEvent(1, int16(o.d.Mouse.X)-int16(oldX), int16(o.d.Mouse.Y)-int16(oldY))
	}
}

// Mouse 回目前的游標座標。
func (o *Oracle) Mouse() (x, y int) {
	return int(o.d.Mouse.X), int(o.d.Mouse.Y)
}

// ClickOpt 調整一次點擊。
type ClickOpt func(*clickCfg)

type clickCfg struct {
	hover, hold, settle uint64
	watch               func(*Oracle)
}

// Hover 改「移到位置之後、按下之前」等多久。
func Hover(n uint64) ClickOpt { return func(c *clickCfg) { c.hover = n } }

// Watch 讓 Click 期間也取樣。
//
// ⚠ **沒有它的話，點擊那六百萬道指令是一段觀測不到的空窗。**
// `Click` 內部跑三段（hover／hold／settle），走 `o.Run`——條件函式一次都不會
// 被呼叫。實測棋子走的**第一格**常常就落在這段裡：`ds:1BE` 的軌跡因此
// 少一格，而序列其餘部分完全正確，看起來像「原版少走了一步」。
func Watch(f func(*Oracle)) ClickOpt { return func(c *clickCfg) { c.watch = f } }

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
	cfg := clickCfg{hover: DefaultHover, hold: DefaultHold, settle: DefaultHold}
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
	// ⚠ **移到位置之後要先停一下再按。**
	//
	// 頂端按鈕列是 hover-based：游標移過去先反白，反白之後的點擊才算數。
	// 移動與按下之間沒有間隔的話，遊戲在同一次輪詢裡同時看到新座標與
	// 按鍵——按鈕不會執行，只會反白。**畫面有反應**（真的反白了），
	// 所以看起來像「點到了但遊戲不理」。
	//
	// rich2 的 DOSBox 腳本用 `mousemove` → `sleep 0.4` → `mousedown`
	// 做同一件事（`tools/dosbox_session.py` 的 click）。
	if err := o.runWatched(cfg.hover, cfg.watch); err != nil {
		return fmt.Errorf("點 (%d,%d) 的 hover 期間：%w", x, y, err)
	}
	o.d.Mouse.Buttons = 1
	o.d.Mouse.Press++
	o.d.MouseEvent(2, 0, 0)
	if err := o.runWatched(cfg.hold, cfg.watch); err != nil {
		return fmt.Errorf("點 (%d,%d) 按住期間：%w", x, y, err)
	}
	o.d.Mouse.Buttons = 0
	o.d.Mouse.Release++
	o.d.MouseEvent(4, 0, 0)
	if err := o.runWatched(cfg.settle, cfg.watch); err != nil {
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

// Key 是IBM PC/AT鍵盤Set 1的make掃描碼。
type Key uint8

const (
	// KeyEscape 是Esc鍵。
	KeyEscape Key = 0x01
	// KeyEnter 是主鍵盤Enter鍵。
	KeyEnter Key = 0x1C
	// KeyDown 是向下方向鍵。
	KeyDown Key = 0x50
	// 目前EOB1具名姓名fixture使用的字母鍵。
	KeyA Key = 0x1E
	KeyF Key = 0x21
	KeyL Key = 0x26
)

// PressKey 透過硬體IRQ1送出一次按下與放開，不經DOS／BIOS輸入佇列。
// 這供自行掛接int 09h的遊戲使用；Type的既有語意維持不變。
func (o *Oracle) PressKey(key Key) {
	makeCode := uint8(key)
	o.m.QueueScanCodes(makeCode, makeCode|0x80)
}

// runWatched 跑 n 道指令，每一道都先呼叫 watch。watch 為 nil 時等同 Run。
func (o *Oracle) runWatched(n uint64, watch func(*Oracle)) error {
	if watch == nil {
		return o.Run(n)
	}
	start := o.m.Steps
	c := NewCond("跑滿指令數", func(o *Oracle) bool {
		watch(o)
		return o.m.Steps-start >= n
	})
	return o.RunUntil(c, Budget(n+1))
}
