package dm

import (
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// 讀檔流程的熱區（`docs/spec/198` §3.6，dosgolem 實測 2026-10-31）。
const (
	// ResumeX／ResumeY 是入口選單 ENTER 下面那顆紅寶石。原版直接讀
	// `DMSAVE.DAT`，不換片也不選檔。
	ResumeX, ResumeY = 249, 86
	// OKX／OKY 是「GAME LOADED, READY TO PLAY.」那個對話框的 OK。
	OKX, OKY = 160, 143
)

// Resume 從 `dm.exe` 跑到讀完存檔、停在按 OK 之後的**第一個刻邊界**
// （第 0 個檢查點），回傳綁好的 Bridge 與開著巡檢寫回的 Clock，負重 stub 也掛好了。
//
// ⚠ **按 OK 之後遊戲就開始走刻**，所以 Clock 要在按 OK 之前掛好。
// 這裡也檢查「按 OK 的那幾百萬道指令裡沒有走過任何一刻」：走過的話
// 第 0 個檢查點已經錯過了，之後每一步的刻數都對不齊。
func Resume(o *oracle.Oracle) (*Bridge, *Clock, error) {
	if err := ToEntrance(o); err != nil {
		return nil, nil, err
	}
	if err := o.Click(ResumeX, ResumeY,
		oracle.NoCursorWait(), oracle.Hold(clickHold), oracle.Settle(ActGap),
	); err != nil {
		return nil, nil, fmt.Errorf("點 RESUME（%d,%d）：%w", ResumeX, ResumeY, err)
	}
	b, err := Bind(o)
	if err != nil {
		return nil, nil, fmt.Errorf("讀檔對話框上 Bind：%w", err)
	}
	if err := b.StubMaximumLoad(SweepMaximumLoad); err != nil {
		return nil, nil, err
	}
	c := b.Attach(true)
	before := c.Boundaries()
	if err := b.tap(OKX, OKY); err != nil {
		return nil, nil, fmt.Errorf("點 OK（%d,%d）：%w", OKX, OKY, err)
	}
	if n := c.Boundaries() - before; n != 0 {
		return nil, nil, fmt.Errorf("按 OK 的期間已經走過 %d 個刻邊界，第 0 個檢查點錯過了", n)
	}
	if err := o.RunUntil(c.NextBoundary(), oracle.Budget(TickBudget(1)+ActGap)); err != nil {
		return nil, nil, fmt.Errorf("按 OK 之後等第一個刻邊界：%w", err)
	}
	return b, c, nil
}

// tap 送一次點擊，放開之後不等畫面。
//
// 巡檢的每一步要在刻的邊界上推進固定刻數，所以點擊本身不能附帶 settle；
// 畫面有沒有回應由之後的 `expect` 判，因此 [oracle.NoResponseError] 不算錯
// ——放開的當下畫面還沒重繪是常態。
func (b *Bridge) tap(x, y int) error {
	err := b.O.Click(x, y, oracle.NoCursorWait(), oracle.Hold(clickHold), oracle.Settle(0))
	var none *oracle.NoResponseError
	if errors.As(err, &none) {
		return nil
	}
	return err
}

// RouteHover 是巡檢每一步「移游標之後、按下之前」跑幾道指令。**預設 0**：
// 在刻的邊界當下按下。
//
// 理由是主迴圈的順序。原版在 `GameTime++`（ST `MAIN.C:131`，DOS `0x10D94`）之後
// 進入等輸入的迴圈，反覆呼叫 `F380_xzzz_COMMAND_ProcessQueue_COPYPROTECTIONC`
// （`MAIN.C:177`），等滿這一刻的時間才進下一刻的時間軸。在邊界當下按下，指令就在
// 這一段處理完，下一刻的時間軸還沒跑——與 remake「在刻與刻之間執行指令」同一個語意。
//
// ⚠ 這一段等待**長短不定**：一刻從迴圈開頭（DOS `0x10CA4` 歸零）算四次 PIT 回呼，
// 時間軸與繪圖也算在裡面（dungeon_master 專案 `docs/re/75`），畫面越複雜，
// `GameTime++` 之後剩下的等待越短。`oracle.Click` 預設的 hover 200 萬道指令
// 在第 2 層 (15,12) 就跨出了等待：按門晚一刻才被處理，第 183 步往前走時門還沒開到
// 過得去（2026-10-31 實測，hover 0 同一步過得去；`GameTime` 在 hover 期間沒有遞增，
// 錯過的是遞增之後那一段等待）。
//
// DM 收滑鼠不靠輪詢（全程只輪詢 3 次，見 `clickHold` 的說明），hover 0 的點擊
// 照樣落在對的按鈕上：第 2 層路線前 207 步全部對上（2026-10-31 實測）。
var RouteHover uint64 = 0

// press 移游標、等 [RouteHover]、按下、按住 [clickHold] 道、放開。
func (b *Bridge) press(x, y int) error {
	if err := b.O.MoveMouse(x, y); err != nil {
		return err
	}
	if RouteHover > 0 {
		if err := b.O.Run(RouteHover); err != nil {
			return fmt.Errorf("hover：%w", err)
		}
	}
	b.O.PressButton(0)
	if err := b.O.Run(clickHold); err != nil {
		return fmt.Errorf("按住：%w", err)
	}
	b.O.ReleaseButton(0)
	return nil
}

// StepResult 是路線走完一步（或第 0 步）時的取樣。
type StepResult struct {
	Step  int // 0 是起點，k 是走完第 k 步
	Party Party
}

// ExpectError 是「走完這一步，隊伍不在路線說的地方」。
type ExpectError struct {
	Step   int
	Want   RoutePoint
	Map    int
	Got    Party
	Reason string
}

func (e *ExpectError) Error() string {
	return fmt.Sprintf("第 %d 步：%s（預期第 %d 層 (%d,%d) 面向 %d，實際第 %d 層 (%d,%d) 面向 %d，遊戲刻 %d）",
		e.Step, e.Reason, e.Map, e.Want.X, e.Want.Y, e.Want.Facing,
		e.Got.MapIndex, e.Got.X, e.Got.Y, e.Got.Facing, e.Got.GameTime)
}

// Sample 讀一次隊伍狀態，遊戲刻換成 Clock 在刻邊界讀到的值。
//
// ⚠ **不能用記憶體裡的 `GameTime`。** 條件在 hook 之前檢查，所以 `RunUntil`
// 是在 `add word_38B54, 1` 跑完、`adc` 之前停下來的：記憶體已經是 N+1，
// 而 Clock 記的 N 才是「第 N 刻做完」，與 remake 檢查點的 `game_time`
// （`World.GameTime − 1`）同一個語意。
func (b *Bridge) Sample(c *Clock) (Party, error) {
	p, err := b.ReadParty()
	p.GameTime = int(c.Tick())
	return p, err
}

// RunRoute 從第 0 個檢查點起照路線走，每一步之後 emit 一次。
//
// 呼叫前要停在 [Resume] 回來的那個刻邊界。第 0 步的核對：遊戲刻等於存檔的
// 遊戲刻、位置與面向等於 `start`。之後每一步：在刻邊界點擊，跑到
// 「這一步開始的刻 ＋ ticks」那個邊界，位置與面向要等於 `expect`。
// 對不上回 [*ExpectError]，emit 仍會先收到那一步的取樣，方便留證據。
func (b *Bridge) RunRoute(c *Clock, r Route, emit func(StepResult) error) error {
	p, err := b.Sample(c)
	if err != nil {
		return err
	}
	if err := emit(StepResult{Step: 0, Party: p}); err != nil {
		return err
	}
	if p.GameTime != r.Save.GameTime {
		return &ExpectError{Step: 0, Want: r.Start, Map: r.Map, Got: p,
			Reason: fmt.Sprintf("第一個刻邊界的遊戲刻是 %d，存檔是 %d", p.GameTime, r.Save.GameTime)}
	}
	if err := checkAt(0, r.Map, r.Start, p); err != nil {
		return err
	}
	at := r.Start
	for i, s := range r.Steps {
		k := i + 1
		x, y, err := s.Click(at.Facing)
		if err != nil {
			return fmt.Errorf("第 %d 步：%w", k, err)
		}
		target := c.Tick() + uint32(s.Ticks)
		if err := b.press(x, y); err != nil {
			return fmt.Errorf("第 %d 步點 (%d,%d)：%w", k, x, y, err)
		}
		if c.Tick() >= target {
			return fmt.Errorf("第 %d 步：點擊本身就走完了 %d 刻，刻數對不齊", k, s.Ticks)
		}
		if err := b.O.RunUntil(c.AtTick(target),
			oracle.Budget(TickBudget(uint32(s.Ticks)))); err != nil {
			return fmt.Errorf("第 %d 步跑到遊戲刻 %d：%w", k, target, err)
		}
		p, err := b.Sample(c)
		if err != nil {
			return fmt.Errorf("第 %d 步：%w", k, err)
		}
		if err := emit(StepResult{Step: k, Party: p}); err != nil {
			return err
		}
		if err := checkAt(k, r.Map, s.Expect, p); err != nil {
			return err
		}
		at = s.Expect
	}
	return nil
}

func checkAt(step, mapIndex int, want RoutePoint, p Party) error {
	if p.MapIndex != mapIndex || p.X != want.X || p.Y != want.Y || p.Facing != want.Facing {
		return &ExpectError{Step: step, Want: want, Map: mapIndex, Got: p, Reason: "位置或面向不符"}
	}
	return nil
}
