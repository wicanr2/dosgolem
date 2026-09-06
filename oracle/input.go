package oracle

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/dos"
)

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
	if err := o.runWatched(cfg.hold, cfg.watch); err != nil {
		return fmt.Errorf("點 (%d,%d) 按住期間：%w", x, y, err)
	}
	o.d.Mouse.Buttons = 0
	o.d.Mouse.Release++
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

// Type 把字串排進 **handle 0** 的輸入，餵給 `int 21h AH=3Fh`
// （編譯後 MS BASIC 的 `INKEY$` 走這條，`rich2/docs/re/005`「輸入路徑」）。
//
// ⚠ **不是每一支程式都走這條。** Turbo Pascal 的 `ReadKey` 走 BIOS 鍵盤，
// 要用 `TypeKeys`／`SendKeys`（`docs/spec/008`）。送錯路徑的症狀是
// 「按了完全沒反應」，與「程式當掉」在畫面上分不出來。
func (o *Oracle) Type(s string) {
	o.d.Stdin = append(o.d.Stdin, []byte(s)...)
}

// Pending 回 handle 0 那條還沒被讀走的位元組數。
func (o *Oracle) Pending() int { return len(o.d.Stdin) }

// TypeKeys 把一段可列印文字排進 **BIOS 鍵盤**佇列（`int 16h`，
// `docs/spec/008`）。有字元不在掃描碼表裡就整段拒絕並回錯——
// 安靜地跳過一個字會讓後面整串輸入錯位，而那要很久以後才看得出來。
func (o *Oracle) TypeKeys(s string) error {
	for _, r := range s {
		if _, ok := dos.KeyForRune(r); !ok {
			return fmt.Errorf("鍵盤掃描碼表沒有 %q", r)
		}
	}
	o.d.PushText(s)
	return nil
}

// SendKeys 依序排幾個有名字的鍵（`Return`、`Space`、`Esc`、方向鍵…）。
func (o *Oracle) SendKeys(names ...string) error {
	for _, name := range names {
		if _, ok := dos.KeyNamed(name); !ok {
			return fmt.Errorf("不認得按鍵 %q", name)
		}
	}
	for _, name := range names {
		o.d.PushKeyNamed(name)
	}
	return nil
}

// KeysPending 回 BIOS 鍵盤佇列裡還沒被讀走的鍵數，
// KeysConsumed 回程式實際讀走的鍵數。
//
// **兩個都要看。** 「送進去了」與「讀走了」在畫面上都是「沒反應」，
// 只有這兩個數字分得開。
func (o *Oracle) KeysPending() int  { return len(o.d.Keys) }
func (o *Oracle) KeysConsumed() int { return o.d.KeysConsumed }

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
