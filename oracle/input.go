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
// 回 error 是為了讓呼叫端能把「移動失敗」一路傳上去。目前的實作
// （事件排進機器的回呼佇列）不會失敗，永遠回 nil。
func (o *Oracle) MoveMouse(x, y int) error {
	o.d.MoveMouse(x, y)
	return nil
}

// Mouse 回目前的游標座標。
func (o *Oracle) Mouse() (x, y int) {
	return int(o.d.Mouse.X), int(o.d.Mouse.Y)
}

// MousePollSteps 回每一次「取位置與鍵狀態」（`INT 33h AH=3`）發生在第幾道指令。
//
// 這是**遊戲輪詢迴圈的節拍**：等輸入的畫面每跑一圈就問一次滑鼠，所以相鄰兩筆
// 的距離就是那個迴圈的週期。要判斷「畫面上的東西為什麼一閃一閃」時，
// 拿它比對螢幕幀比猜快得多。
func (o *Oracle) MousePollSteps() []uint64 {
	out := make([]uint64, len(o.d.Mouse.Polls))
	for i, p := range o.d.Mouse.Polls {
		out[i] = p.Step
	}
	return out
}

// ClickOpt 調整一次點擊。
type ClickOpt func(*clickCfg)

type clickCfg struct {
	hover, hold, settle uint64
	button              int
	watch               func(*Oracle)
	noCursorWait        bool
	frame               func(*Oracle) []uint8
}

// Button 選要按哪一個鍵（0 左／1 右／2 中）。
//
// **原版的「取消／退回」是右鍵**（`docs/spec/016`、臥龍傳專案
// `docs/re/53` §3）。DOS 遊戲普遍拿右鍵當「取消／關閉目前視窗」，
// 少了它被蓋住的視窗一個都點不到，而症狀是**畫面完全不動**——
// 跟點錯位置分不出來。
func Button(n int) ClickOpt { return func(c *clickCfg) { c.button = n } }

// Right 是 Button(1) 的別名。
func Right() ClickOpt { return Button(1) }

// Hover 改「移到位置之後、按下之前」等多久。
func Hover(n uint64) ClickOpt { return func(c *clickCfg) { c.hover = n } }

// Watch 讓 Click 期間也取樣。
//
// ⚠ **沒有它的話，點擊那六百萬道指令是一段觀測不到的空窗。**
// `Click` 內部跑三段（hover／hold／settle），走 `o.Run`——條件函式一次都不會
// 被呼叫。實測棋子走的**第一格**常常就落在這段裡：`ds:1BE` 的軌跡因此
// 少一格，而序列其餘部分完全正確，看起來像「原版少走了一步」。
func Watch(f func(*Oracle)) ClickOpt { return func(c *clickCfg) { c.watch = f } }

// NoCursorWait 跳過「等程式設過游標位置」那一步。
//
// ⚠ **游標歸誰畫決定要不要等。** 有些遊戲用 `int 33h AX=4` 把游標放到自己
// 要的位置，那時注入座標會被蓋掉，所以要先等它設完；《武士傳說》相反——
// 它從來不叫 `AX=4`，畫面上那隻小手完全是自己畫的。對這種遊戲那個等待
// **永遠不會成立**，`Click` 會在跑滿預算之後回「等程式設游標位置」失敗，
// 看起來像遊戲當掉了。
func NoCursorWait() ClickOpt { return func(c *clickCfg) { c.noCursorWait = true } }

// Frame 換掉「畫面有沒有變」的取樣器（預設 `Indexed`，也就是 mode 13h 的
// A0000）。
//
// ⚠ **模式不對的話取樣永遠是全 0**，於是每一次點擊都回 `NoResponseError`
// ——看起來像每一次都點空了。Tandy 模式 09h 傳 `(*Oracle).Tandy16`，
// CGA 模式 4 傳 `(*Oracle).CGA4`。
func Frame(f func(*Oracle) []uint8) ClickOpt { return func(c *clickCfg) { c.frame = f } }

// Hold 改按住的指令數。
func Hold(n uint64) ClickOpt { return func(c *clickCfg) { c.hold = n } }

// Settle 改放開之後再跑多久（讓遊戲把回饋畫出來）。
func Settle(n uint64) ClickOpt { return func(c *clickCfg) { c.settle = n } }

// RightButton 是 Button(1)（Microsoft 滑鼠右鍵）的別名。
func RightButton() ClickOpt { return Button(1) }

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
	// 預設取樣器是 video()：它會**看目前的視訊模式**挑來源。
	// 寫死 Indexed()（mode 13h 的 A0000）的話，平面模式下那裡永遠是全 0，
	// 於是「畫面沒變」恆成立——每一次點擊都被報成沒反應，而畫面其實變了。
	cfg := clickCfg{hover: DefaultHover, hold: DefaultHold, settle: DefaultHold,
		frame: (*Oracle).video}
	for _, f := range opts {
		f(&cfg)
	}
	if !cfg.noCursorWait && len(o.d.Mouse.Sets) == 0 {
		if err := o.RunUntil(MouseSettled); err != nil {
			return fmt.Errorf("點 (%d,%d) 之前等程式設游標位置：%w", x, y, err)
		}
	}

	// ⚠ **要抄一份。** video() 平面模式下回的是重用緩衝區，
	// 直接留參考的話 before 會跟著後面的取樣一起變，於是「畫面沒變」恆成立。
	before := append([]uint8(nil), cfg.frame(o)...)
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
	o.d.PressMouse(cfg.button)
	if err := o.runWatched(cfg.hold, cfg.watch); err != nil {
		return fmt.Errorf("點 (%d,%d) 按住期間：%w", x, y, err)
	}
	o.d.ReleaseMouse(cfg.button)
	if err := o.runWatched(cfg.settle, cfg.watch); err != nil {
		return fmt.Errorf("點 (%d,%d) 放開之後：%w", x, y, err)
	}

	if sameBytes(before, cfg.frame(o)) {
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

// Key 是IBM PC/AT鍵盤Set 1的make掃描碼。
type Key uint8

const (
	// KeyEscape 是Esc鍵。
	KeyEscape Key = 0x01
	// KeyEnter 是主鍵盤Enter鍵。
	KeyEnter Key = 0x1C
	// KeyDown 是向下方向鍵。
	KeyDown Key = 0x50
	// KeyUp 是向上方向鍵；KeyPageUp是數字鍵盤右轉鍵。
	KeyUp     Key = 0x48
	KeyPageUp Key = 0x49
	// 目前EOB1具名姓名fixture使用的字母鍵。
	KeyA Key = 0x1E
	KeyB Key = 0x30
	KeyD Key = 0x20
	KeyE Key = 0x12
	KeyF Key = 0x21
	KeyG Key = 0x22
	KeyL Key = 0x26
	KeyM Key = 0x32
	KeyT Key = 0x14
	KeyZ Key = 0x2C
)

// PressKey 透過硬體IRQ1送出一次按下與放開，不經DOS／BIOS輸入佇列。
// 這供自行掛接int 09h的遊戲使用；Type的既有語意維持不變。
func (o *Oracle) PressKey(key Key) {
	makeCode := uint8(key)
	o.m.QueueScanCodes(makeCode, makeCode|0x80)
}

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
func (o *Oracle) KeysPending() int  { return o.d.KeysPending() }
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

// ClearInput 丟掉還沒被讀走的按鍵。
//
// **上一張畫面沒吃掉的鍵會流進下一張。** 實測：對股市場所選單送了 ESC
// 而它沒有消化掉，那個 ESC 就留在佇列裡，於是下一張銀行選單一開就被
// 取消了——回傳 `0x63` 而不是我們要的第 1 列。
//
// 症狀是「明明送了 Enter，原版卻收到取消」，看起來像送鍵的方式錯了，
// 不像佇列有殘留。所以自動回答之前先清一次。
func (o *Oracle) ClearInput() { o.d.Stdin = nil }

// HoldOnEmptyInput 讓「鍵盤佇列空」回報**讀到 0 個位元組**。
//
// 編譯後 BASIC 的「按任意鍵繼續」是 `while INKEY$ = "" : wend`。
// dosgolem 預設在佇列空的時候餵一個 `00`（見 `dos.StdinFill` 的理由），
// 而 `CHR$(0)` 是**非空字串**——於是那種畫面**一閃而過**：
// 畫出來了、也畫對了，但下一次重繪就把它蓋掉，而外面用粗取樣
// 完全看不到它存在過（rich2 的地圖查詢就是這樣被誤判成「沒畫出來」）。
//
// ⚠⚠ **在 rich2 上它「停不住」畫面，只會把程式弄死。** 實測兩次：
// 打開之後主程式在別處的讀取拿到 0 個位元組 ＝ EOF，
// 直接 `int 21h AH=4Ch` 結束。
//
// **而且死掉之後畫面會凍在最後一幀**——那一幀正好是你想要的那張，
// 所以「非零像素穩定不變」看起來**很像成功停住了**。
// 判準要看 `Click`／`Run` 有沒有回錯，不是看畫面穩不穩。
//
// **想抓「畫完就等一個鍵」的畫面，正確做法是在它自然顯示的窗口內取樣**
// （rich2 的地圖查詢只顯示約四十萬道指令，而那段落在 `Click` 內部，
// 外面完全觀測不到）——用 `oracle.Watch` 加一個進入點的 `OnCall`。
//
// 這個開關留著是因為「讀到 0 個位元組」才是 DOS 的正確語意；
// 別的程式未必像 rich2 那樣把它當 EOF。用之前先確認。
func (o *Oracle) HoldOnEmptyInput(v bool) { o.d.StdinEmptyReadsZero = v }

// DefaultTapHold 是瞬按的按住長度。
//
// **與 DefaultHold 是兩個不同的目標，沒有一個值兩邊都對**
// （`docs/spec/010` §2）：按鈕要按夠久才會被輪詢看到，
// 而彈出選單如果在開起來的那一瞬間按鍵還按著，
// 下一次 `AX=5` 輪詢會用當時的游標位置立刻選一列——
// 框外的游標被夾到第 0 列，於是「第二列以後永遠點不到」。
//
// 20 萬道指令 ≈ DOSBox 在 `cycles=fixed 20000` 之下的 10 ms，
// 落在臥龍傳專案 `docs/playtest/54` 量出來的 5–60 ms 窗口裡。
const DefaultTapHold = 200_000

// Tap 是瞬按：移過去、很短地按一下。**彈出選單要用這個**，不要用 Click。
func (o *Oracle) Tap(x, y int, opts ...ClickOpt) error {
	return o.Click(x, y, append([]ClickOpt{Hold(DefaultTapHold)}, opts...)...)
}

// Press 在**目前位置**按一下，不移動游標。
//
// 既有的 DOSBox 擷取腳本大量用「移過去、再按一次」（`click:x,y` 之後接
// `press`），這一支讓那些腳本照抄得過來。
func (o *Oracle) Press(opts ...ClickOpt) error {
	cfg := clickCfg{hover: 0, hold: DefaultHold, settle: DefaultHold}
	for _, f := range opts {
		f(&cfg)
	}
	before := append([]uint8(nil), o.video()...)
	o.d.PressMouse(cfg.button)
	if err := o.runWatched(cfg.hold, cfg.watch); err != nil {
		return fmt.Errorf("原地按住期間：%w", err)
	}
	o.d.ReleaseMouse(cfg.button)
	if err := o.runWatched(cfg.settle, cfg.watch); err != nil {
		return fmt.Errorf("原地放開之後：%w", err)
	}
	if sameBytes(before, o.video()) {
		x, y := o.Mouse()
		return &NoResponseError{X: x, Y: y, Polls: len(o.d.Mouse.Polls)}
	}
	return nil
}

// MouseRange 回遊戲用 `int 33h AX=7`／`AX=8` 設的座標範圍。
//
// **範圍會變**：有些畫面（例如地圖捲動）會把它放大，好讓游標推得出畫面邊界。
// 範圍與畫面一樣大時**捲不動**——那不是 bug，是那個模式本來就不捲。
func (o *Oracle) MouseRange() (minX, maxX, minY, maxY int) {
	m := &o.d.Mouse
	return int(m.MinX), int(m.MaxX), int(m.MinY), int(m.MaxY)
}
