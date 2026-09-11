package dm

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// IDA 位址（dungeon_master 專案 `docs/re/73`、`docs/spec/198` §2）。
const (
	// IDATickBoundary 是主迴圈裡 `add word_38B54, 1`（`GameTime++` 的低位），
	// 執行期是 `070a:0144`。**寫回與取樣都掛在這一道之前**，
	// 與 remake 的 `sweepRestore()` 位置（`Tick()` 裡 `GameTime++` 之前）同語意。
	IDATickBoundary = 0x10D94
	// IDAMaximumLoad 是 `F309_CHAMPION_GetMaximumLoad`（`sub_1E063`）。
	IDAMaximumLoad = 0x1E063
	// IDAMovementTicks 是 `F310_CHAMPION_GetMovementTicks`（`sub_1E0DC`），
	// 驗證負重 stub 有沒有生效時用它。
	IDAMovementTicks = 0x1E0DC
)

// SweepMaximumLoad 是巡檢模式下的負重上限，單位是十分之一公斤
// （＝ 1000 公斤），**與 remake 的 `SweepMaximumLoad` 同值**。
//
// ⚠ **這個值要放得進 16 位元，而且 ×5 之後也要**（`10000×5 ＝ 50000 < 65536`）：
//
//   - `oracle.Stub` 把回傳值放進 `DX:AX`，而 `F309` 的呼叫端只收 `AX`。
//     給它 `1<<20` 的話 `AX` ＝ `0`，**全隊反而變成永遠超重**，效果正好相反。
//   - `×5` 是 `F310_GetMovementTicks` 拿來跟 `負重×8` 比的量。DOS 版那一段走的是
//     32 位元 helper 所以不會溢位，但 remake 那一側同一條算式用 Go 的 `int`，
//     **兩側要得出同一個答案**。
//
// 正常上限是 `力量×8 + 100` 再按體力與傷勢打折，力量頂天也只到九百多，
// 所以 10000 遠在任何真實負重之外。
const SweepMaximumLoad = 10000

// SweepRestore 是一刻的寫回（`docs/spec/198` §3.2）：生命力寫回上限、
// 食物與水寫回滿值、中毒事件數清零。
//
// **做法是「每刻寫回」不是「攔截函式」。** 理由有兩個，都不是偏好問題：
// `F331_ApplyTimeEffects` 帶 `_COPYPROTECTIONF` 後綴（被防拷邏輯穿插過的函式，
// 不移植也不改），而且它同時做體力回復與法術倒數，擋掉會改到別的規則。
//
// ⚠ **傷勢（`Wounds`）與受傷音效照樣發生。** 那是刻意的：兩側都會發生，
// 比較得出來。
func (b *Bridge) SweepRestore() {
	for i := 0; i < MaxChampions; i++ {
		base := uint16(dsChampions + i*ChampionSize)
		maxHP := int16(b.O.Word(b.DS(base + offMaximumHealth)))
		if maxHP <= 0 {
			continue // 這一格沒有勇士
		}
		b.O.SetWord(b.DS(base+offCurrentHealth), uint16(maxHP))
		b.O.SetWord(b.DS(base+offFood), FoodWaterMaximum)
		b.O.SetWord(b.DS(base+offWater), FoodWaterMaximum)
		b.O.SetByte(b.DS(base+offPoisonEventCount), 0)
	}
}

// StubMaximumLoad 把 `F309` 換成固定回傳值。傳 0 取消。
//
// 三個前提都查過了（`docs/spec/198` §3.2）：cdecl far（`retf` 沒有立即數、
// 呼叫端 `add sp, 6` 清參數）、**一個** far 指標參數、整棵呼叫樹零記憶體寫入
// ——`oracle.Stub` 的邊界正好對上。
func (b *Bridge) StubMaximumLoad(value int) error {
	a := b.IDA(IDAMaximumLoad)
	if value == 0 {
		b.O.Stub(a, nil)
		return nil
	}
	if value < 0 || value*5 >= 1<<16 {
		return fmt.Errorf("負重上限 %d 不合用：要是正數，而且 ×5（＝%d）"+
			"要放得進 16 位元——`F309` 只回 `AX`", value, value*5)
	}
	b.O.StubValue(a, uint32(value))
	return nil
}

// Clock 把「刻的邊界」變成可以停下來的條件。
//
// ⚠ **不要在 `oracle.Cond` 裡讀記憶體。** `RunUntil` 在**每道指令執行前**
// 都會呼叫 `Cond.ready`，而一刻約 254 萬道指令——條件裡讀兩個 word
// 等於做幾億次記憶體存取。`OnCall` 只在那一個位址觸發，條件只讀一個 Go 變數。
type Clock struct {
	tick     uint32 // 最近一次在刻邊界看到的 GameTime
	boundary uint64 // 經過幾次刻邊界
	sweep    bool
	b        *Bridge
}

// Attach 把 Clock 掛到刻的邊界上。sweep 為真時順便做每刻寫回。
//
// ⚠ **`oracle.OnCall` 沒有取消**（它是 append）。同一個 Oracle 上
// 掛兩次就會跑兩份回呼，寫回做兩次不礙事，但 `Boundaries()` 會多算一倍。
func (b *Bridge) Attach(sweep bool) *Clock {
	// ⚠ **起始值要是現在的 `GameTime`，不是零。** 留零的話
	// [Clock.AfterTicks] 的目標會算成「n」而不是「現在＋n」，
	// 第一次刻邊界就滿足——跑一刻就回來，看起來像「遊戲沒在動」。
	c := &Clock{sweep: sweep, b: b, tick: b.gameTime()}
	b.O.OnCall(b.IDA(IDATickBoundary), func(*oracle.Oracle) {
		// ⚠ 這一道（`add word_38B54, 1`）**還沒執行**，所以讀到的是遞增
		// **前**的值。停在「GameTime ＝ N」的語意是「第 N 刻結束、
		// 第 N+1 刻要開始的那一瞬間」。
		c.tick = b.gameTime()
		c.boundary++
		if c.sweep {
			b.SweepRestore()
		}
	})
	return c
}

// Tick 是最近一次在刻邊界看到的 `GameTime`。
func (c *Clock) Tick() uint32 { return c.tick }

// Boundaries 是到目前為止經過幾次刻邊界。
func (c *Clock) Boundaries() uint64 { return c.boundary }

// SetSweep 開關每刻寫回（旗標預設關，見 `docs/spec/198` §1）。
func (c *Clock) SetSweep(on bool) { c.sweep = on }

// AtTick 是「`GameTime` 走到 n（含）」的停止條件。
func (c *Clock) AtTick(n uint32) oracle.Cond {
	return oracle.NewCond(fmt.Sprintf("GameTime≥%d", n),
		func(*oracle.Oracle) bool { return c.tick >= n })
}

// AfterTicks 是「從現在起再過 n 刻」。
//
// ⚠ 條件是在第一次呼叫 `ready` 時定下目標的（`oracle.Steps` 同樣的做法），
// 所以**要在 `RunUntil` 之前現做**，不要把同一個條件留著重用。
func (c *Clock) AfterTicks(n uint32) oracle.Cond {
	var target uint32
	first := true
	return oracle.NewCond(fmt.Sprintf("再過 %d 刻", n), func(*oracle.Oracle) bool {
		if first {
			target, first = c.tick+n, false
		}
		return c.tick >= target
	})
}

// TickBudget 是跑 n 刻要給的指令預算。
//
// 一刻約 **254 萬**道指令（`game.state` 展開實測），這裡取 400 萬再加一個
// 起步的餘量——寧可寬。⚠ `oracle.DefaultBudget`（1 億）只夠約 **39 刻**，
// 不指定的話跑不完會回 `BudgetError`，那是錯誤不是靜默。
func TickBudget(n uint32) uint64 { return uint64(n)*4_000_000 + 20_000_000 }

// SetFoodWater 直接寫某一格勇士的食物與水。
//
// **只給測試與反對照用**：巡檢本身走的是 [Bridge.SweepRestore]。
// 拿它把值壓低，「會不會掉」才在幾十刻內看得出來——原版的食物掉得很慢，
// 不壓的話「沒掉」與「時間不夠」分不出來。
func (b *Bridge) SetFoodWater(index, food, water int) error {
	if index < 0 || index >= MaxChampions {
		return fmt.Errorf("勇士編號 %d 不在 0–%d", index, MaxChampions-1)
	}
	base := uint16(dsChampions + index*ChampionSize)
	if int16(b.O.Word(b.DS(base+offMaximumHealth))) <= 0 {
		return fmt.Errorf("第 %d 格沒有勇士", index)
	}
	b.O.SetWord(b.DS(base+offFood), uint16(int16(food)))
	b.O.SetWord(b.DS(base+offWater), uint16(int16(water)))
	return nil
}
