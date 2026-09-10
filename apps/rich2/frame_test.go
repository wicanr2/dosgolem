package rich2_test

import (
	"testing"

	"github.com/wicanr2/dosgolem/apps/rich2"
	"github.com/wicanr2/dosgolem/oracle"
)

// TestOnTickCountsEveryTick 釘住 `docs/spec/187` §5 第 3 條：
// `OnTick` 的呼叫次數等於同一段內 `Ticks()` 的增量。
//
// 少算或多算都不會報錯，只會讓「第 N 幀」錯位——而錯位的症狀是
// 「畫面看起來都對，只是比對永遠差一點」。
func TestOnTickCountsEveryTick(t *testing.T) {
	o := load(t)
	if err := rich2.ToMainMenu(o); err != nil {
		t.Fatal(err)
	}
	var fired uint64
	o.OnTick(func(*oracle.Oracle) { fired++ })
	before := o.Ticks()
	if err := o.RunUntil(oracle.AtTick(before+50), oracle.Budget(200_000_000)); err != nil {
		t.Fatal(err)
	}
	o.OnTick(nil)
	if want := o.Ticks() - before; fired != want {
		t.Errorf("OnTick 觸發 %d 次，Ticks 增量 %d", fired, want)
	}
	// 取消之後不該再觸發。
	was := fired
	if err := o.RunUntil(oracle.AtTick(o.Ticks()+5), oracle.Budget(50_000_000)); err != nil {
		t.Fatal(err)
	}
	if fired != was {
		t.Errorf("OnTick(nil) 之後又觸發了 %d 次", fired-was)
	}
}

// TestEachFrameOnlyFiresDuringAnimation 釘住 `docs/spec/187` 的核心主張：
// **幀是原版自己的繪製點，不是計時器刻**。
//
// 主選單沒有移動動畫，而計時器照樣走。若 `EachFrame` 用「每 4 個 tick」
// 實作，這裡會觸發一堆**沒有畫面更新的假幀**——而假幀不報錯，
// 只會讓「第 N 幀」整段錯位。
func TestEachFrameOnlyFiresDuringAnimation(t *testing.T) {
	o := load(t)
	if err := rich2.ToMainMenu(o); err != nil {
		t.Fatal(err)
	}
	var frames int
	stop := rich2.EachFrame(o, func(*oracle.Oracle) { frames++ })
	defer stop()
	before := o.Ticks()
	if err := o.RunUntil(oracle.AtTick(before+40), oracle.Budget(200_000_000)); err != nil {
		t.Fatal(err)
	}
	if frames != 0 {
		t.Errorf("主選單期間觸發了 %d 幀（計時器走了 %d 刻）——幀被當成 tick 了",
			frames, o.Ticks()-before)
	}
}

// TestScreenFramesAreUniversal 驗通用的**螢幕幀**（`docs/spec/187` §3）。
//
// 三件事一起釘：
//
//  1. 垂直回掃會推進，而且 `OnFrame` 的觸發次數等於 `Frames()` 的增量。
//  2. **主選單也會推進**——螢幕刷新與程式在做什麼無關。這正是它比
//     「程式畫完一幀的位址」通用的地方：同一段期間動畫幀是 0
//     （見 TestEachFrameOnlyFiresDuringAnimation），螢幕幀不是。
//  3. 取消之後不再觸發。
func TestScreenFramesAreUniversal(t *testing.T) {
	o := load(t)
	if err := rich2.ToMainMenu(o); err != nil {
		t.Fatal(err)
	}
	var fired uint64
	o.OnFrame(func(*oracle.Oracle) { fired++ })
	before, beforeTick := o.Frames(), o.Ticks()
	if err := o.RunUntil(oracle.AtFrame(before+30), oracle.Budget(200_000_000)); err != nil {
		t.Fatal(err)
	}
	o.OnFrame(nil)

	if fired == 0 {
		t.Fatal("一幀都沒觸發——垂直回掃沒有推進")
	}
	if got := o.Frames() - before; fired != got {
		t.Errorf("OnFrame 觸發 %d 次，Frames 增量 %d", fired, got)
	}
	t.Logf("主選單期間：螢幕幀 +%d、計時器 +%d 刻", o.Frames()-before, o.Ticks()-beforeTick)

	was := fired
	if err := o.RunUntil(oracle.AtFrame(o.Frames()+5), oracle.Budget(50_000_000)); err != nil {
		t.Fatal(err)
	}
	if fired != was {
		t.Errorf("OnFrame(nil) 之後又觸發了 %d 次", fired-was)
	}
}

// TestAnimationFramesMatchSpeedTable 是 `docs/spec/187` §5 第 5 條的驗收，
// 也是兩條獨立的路互相對帳：
//
//   - **位址節拍**（本測試）：`EachFrame` 掛在 `FrameWait`，數觸發次數。
//   - **螢幕幀**（`cmd/frames` 錄影後做畫面差分）：大範圍重繪的間隔。
//
// 兩邊都該得到「骰數 × 每格幀數」。實測（2026-09-09，新開局、骰 8）：
// 畫面差分得到 48 個像素幀、每格 6.0 幀、間隔 4 出現 47 次，
// 對上 `rich2/docs/spec/059` 檔位 1 的「每格 6 幀」與
// `rich2/docs/re/097`「新開局跑檔位 1」。
//
// ⚠ **移動時整個地圖區都會重繪**（`rich2/docs/re/093`：逐步移動動畫**與視窗
// 跟隨**），所以畫面差分要看大範圍變化；小區塊的變化是別的東西
// （買地對話框的 Yes／No 閃爍就是）。
func TestAnimationFramesMatchSpeedTable(t *testing.T) {
	o := load(t)
	if err := rich2.ToBoard(o); err != nil {
		t.Fatal(err)
	}
	player := rich2.Turn(o)
	from := rich2.Position(o, player)

	var frames int
	stop := rich2.EachFrame(o, func(*oracle.Oracle) { frames++ })
	defer stop()

	if err := o.Click(rich2.BtnMoveX, rich2.BtnY); err != nil {
		t.Fatal(err)
	}
	moved := oracle.NewCond("玩家位置改變", func(o *oracle.Oracle) bool {
		return rich2.Position(o, player) != from || rich2.Turn(o) != player
	})
	if err := o.RunUntil(moved, oracle.Budget(150_000_000)); err != nil {
		t.Fatal(err)
	}
	stop()

	dice := rich2.Steps(o)
	t.Logf("骰 %d：格 %d → %d，動畫幀 %d（每格 %.1f）",
		dice, from, rich2.Position(o, player), frames, float64(frames)/float64(dice))
	if dice <= 0 {
		t.Fatal("骰數讀不到")
	}
	// 新開局是速度檔位 1 ＝ 每格 6 個像素幀（`rich2/docs/spec/059`）。
	if want := dice * 6; frames != want {
		t.Errorf("動畫幀 %d，預期 %d（骰 %d × 每格 6 幀）——"+
			"節拍取錯了，或這一局不是檔位 1", frames, want, dice)
	}
}
