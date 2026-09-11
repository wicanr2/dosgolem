package dm

import "testing"

// TestChampionOffsets 釘住 `CHAMPION` 的偏移表（`docs/spec/198` §2）。
//
// **不必跑原版**：合成一段 319 位元組，每個欄位塞一個只屬於它的值，
// 解出來要一一對上。偏移寫錯一個位元組，這裡就會紅。
//
// ⚠ **`Statistics[7][3]` 在 x86 沒有補齊位元組**，所以 `Skills` 從**奇數**
// 位址 `+0x5B` 起算、`Slots` 落在 `+0xD3`。照 68000 的對齊習慣去推，
// 從 `Skills` 開始每一個欄位都會偏一——而那解出來的東西**看起來仍然像數字**。
func TestChampionOffsets(t *testing.T) {
	raw := make([]byte, ChampionSize)
	put16 := func(off int, v uint16) { raw[off], raw[off+1] = byte(v), byte(v>>8) }

	copy(raw[offName:], "ELIJA\x00XX")
	copy(raw[offTitle:], "LION OF DARKNESS\x00")
	raw[offPoisonEventCount] = 7
	put16(offCurrentHealth, 123)
	put16(offMaximumHealth, 456)
	put16(offFood, 1500)
	put16(offWater, 0xFE00) // ＝ −512，DOS 版直接讀到過的「開始挨餓」
	put16(offLoad, 321)
	for n := 0; n < SlotCount; n++ {
		put16(offSlots+n*2, uint16(0x1000+n))
	}

	c, ok := decodeChampion(2, raw)
	if !ok {
		t.Fatal("解不出勇士，但 MaximumHealth ＝ 456 > 0")
	}
	if c.Index != 2 {
		t.Errorf("Index ＝ %d，預期 2", c.Index)
	}
	if c.Name != "ELIJA" {
		t.Errorf("Name ＝ %q，預期 %q", c.Name, "ELIJA")
	}
	if c.Title != "LION OF DARKNESS" {
		t.Errorf("Title ＝ %q，預期 %q", c.Title, "LION OF DARKNESS")
	}
	if c.CurrentHealth != 123 || c.MaxHealth != 456 {
		t.Errorf("生命力 ＝ %d／%d，預期 123／456", c.CurrentHealth, c.MaxHealth)
	}
	if c.Food != 1500 {
		t.Errorf("Food ＝ %d，預期 1500", c.Food)
	}
	if c.Water != -512 {
		t.Errorf("Water ＝ %d，預期 −512（有號才讀得對）", c.Water)
	}
	if c.PoisonEventCount != 7 {
		t.Errorf("PoisonEventCount ＝ %d，預期 7", c.PoisonEventCount)
	}
	if c.Load != 321 {
		t.Errorf("Load ＝ %d，預期 321", c.Load)
	}
	for n := 0; n < SlotCount; n++ {
		if want := uint16(0x1000 + n); c.Slots[n] != want {
			t.Fatalf("Slots[%d] ＝ %04X，預期 %04X——"+
				"第一個對不上的格子就停，後面全是同一個偏移錯", n, c.Slots[n], want)
		}
	}
}

// TestChampionSlotEnd 守住「`Slots` 的最後一格剛好接到 `Load`」。
//
// 這是偏移表自己的正對照：`0xD3 + 30×2 ＝ 0x10F`，而 `Load` 的偏移是
// 反組譯直接讀到的（`sub_1E0DC` 的 `es:[bx+10Fh]`）。兩個數字對得上，
// 表示中間 30 格一個不多一個不少。
func TestChampionSlotEnd(t *testing.T) {
	if got := offSlots + SlotCount*2; got != offLoad {
		t.Errorf("Slots 尾端 ＝ %#x，Load 在 %#x——中間差了 %d 位元組",
			got, offLoad, offLoad-got)
	}
	if offLoad+2+2+44 != ChampionSize {
		t.Errorf("Load＋ShieldDefense＋cUnreferenced[44] ＝ %#x，"+
			"總長是 %#x", offLoad+2+2+44, ChampionSize)
	}
}

// TestEmptySlotIsNotAChampion 是反對照：`MaximumHealth ＝ 0` 的那一格
// **沒有勇士**，不是「一位生命力 0 的勇士」。
//
// ⚠ 少了這一條，招募大廳（四格都空）會解出四位無名勇士，
// 而那看起來只是「名字是空字串」。
func TestEmptySlotIsNotAChampion(t *testing.T) {
	raw := make([]byte, ChampionSize)
	copy(raw[offName:], "GHOST\x00")
	raw[offCurrentHealth], raw[offCurrentHealth+1] = 99, 0 // 當前生命力非 0
	if _, ok := decodeChampion(0, raw); ok {
		t.Error("MaximumHealth ＝ 0 的那一格解出了勇士")
	}

	// 正對照：同一段只把 MaximumHealth 填起來就該解得出來。
	raw[offMaximumHealth] = 1
	if _, ok := decodeChampion(0, raw); !ok {
		t.Error("MaximumHealth ＝ 1 反而解不出勇士——判準寫反了")
	}
}

// TestDeadChampionStillCounts 守著「死掉的勇士那一格仍然有人」。
//
// 巡檢要把死掉的救回來（remake 的 `sweepRestore` 同樣的判準），
// 用當前生命力當判準的話這一格會被跳過，隊伍走到一半就少人。
func TestDeadChampionStillCounts(t *testing.T) {
	raw := make([]byte, ChampionSize)
	raw[offMaximumHealth] = 200 // CurrentHealth 留 0
	c, ok := decodeChampion(1, raw)
	if !ok {
		t.Fatal("死掉的勇士那一格被當成空的")
	}
	if c.CurrentHealth != 0 || c.MaxHealth != 200 {
		t.Errorf("生命力 ＝ %d／%d，預期 0／200", c.CurrentHealth, c.MaxHealth)
	}
}

// TestSweepMaximumLoadFitsAX 守住 `docs/spec/198` §3.2 的兩個 16 位元條件。
//
// ⚠ `oracle.Stub` 把回傳值放進 `DX:AX`，而 `F309` 的呼叫端只收 `AX`：
// 給它 `1<<20` 的話 `AX` ＝ `0`，**全隊反而變成永遠超重**。
func TestSweepMaximumLoadFitsAX(t *testing.T) {
	if SweepMaximumLoad <= 0 || SweepMaximumLoad >= 1<<16 {
		t.Errorf("SweepMaximumLoad ＝ %d 放不進 AX", SweepMaximumLoad)
	}
	if SweepMaximumLoad*5 >= 1<<16 {
		t.Errorf("SweepMaximumLoad×5 ＝ %d 放不進 16 位元——"+
			"F310 拿它跟「負重×8」比", SweepMaximumLoad*5)
	}
	// 遠大於任何真實負重：正常上限是 `力量×8 + 100` 再打折，
	// 力量上限 100 的話也只有 900。
	if SweepMaximumLoad < 10*(100*8+100) {
		t.Errorf("SweepMaximumLoad ＝ %d 不夠遠離真實負重", SweepMaximumLoad)
	}
}

// TestTickBudgetCoversRealTicks 守著「預算不要用預設值」。
//
// 一刻約 254 萬道指令，而 `oracle.DefaultBudget` 是 1 億——只夠 39 刻。
func TestTickBudgetCoversRealTicks(t *testing.T) {
	const measuredStepsPerTick = 2_540_000
	for _, n := range []uint32{1, 10, 100, 500} {
		if got, need := TickBudget(n), uint64(n)*measuredStepsPerTick; got <= need {
			t.Errorf("跑 %d 刻的預算 %d 沒有蓋過實測的 %d 道", n, got, need)
		}
	}
}

// TestCodeAddressesMatchMeasuredRuntime 把段表與**實測到的執行期位址**對起來。
//
// dungeon_master 專案用 `-watch` 監看 `GameTime` 時，刻的邊界停在
// **`070a:0144`**；這裡用段表 ＋ 實測的換算常數 `0x9BB0` 算出同一個位址。
// 兩個獨立來源對得上，段表才算站得住。
//
// ⚠ 這一支**不跑原版**：`idaOffset` 寫死成實測值，驗的是段表的算術。
// 配置真的變了會由 `TestLiveBindFindsEngineBase` 那一支抓到。
func TestCodeAddressesMatchMeasuredRuntime(t *testing.T) {
	b := &Bridge{idaOffset: 0x9BB0}

	got, err := b.Code(IDATickBoundary)
	if err != nil {
		t.Fatalf("刻的邊界：%v", err)
	}
	if got.Seg != 0x070A || got.Off != 0x0144 {
		t.Errorf("刻的邊界算出 %s，實測是 070A:0144", got)
	}
	if s := CodeSegName(IDATickBoundary); s != "seg001" {
		t.Errorf("刻的邊界在 %s，預期 seg001（主迴圈 sub_10C94 那一段）", s)
	}

	// `F309` 在另一個段——**這正是不能拿主迴圈的 CS 去呼叫它的理由**。
	load, err := b.Code(IDAMaximumLoad)
	if err != nil {
		t.Fatalf("F309：%v", err)
	}
	if s := CodeSegName(IDAMaximumLoad); s != "seg010" {
		t.Errorf("F309 在 %s，預期 seg010", s)
	}
	if load.Seg == got.Seg {
		t.Error("F309 與主迴圈算出同一個段——段表沒有生效")
	}
	// 線性位址與 oracle.IDA() 的算法一致（只有段的切法不同）。
	if want := uint32(IDAMaximumLoad - 0x9BB0); load.Linear() != want {
		t.Errorf("F309 的線性位址 ＝ %05X，預期 %05X", load.Linear(), want)
	}
}

// TestCodeSegsAreContiguous 守著段表本身：段要照順序、不重疊、不留洞，
// 而且每個段的基底不能晚於它的起點。
//
// ⚠ `sel_base` **可以早於 `Start`**（IDA 把段基底對齊到 16 的倍數），
// 但不能晚——晚了就表示欄位對調或抄錯。
func TestCodeSegsAreContiguous(t *testing.T) {
	for i, s := range codeSegs {
		if s.Base > s.Start {
			t.Errorf("%s 的基底 %05X 晚於起點 %05X", s.Name, s.Base, s.Start)
		}
		if s.Base%16 != 0 {
			t.Errorf("%s 的基底 %05X 不是 16 的倍數", s.Name, s.Base)
		}
		if s.End <= s.Start {
			t.Errorf("%s 的範圍 %05X–%05X 是空的", s.Name, s.Start, s.End)
		}
		if i > 0 && codeSegs[i-1].End != s.Start {
			t.Errorf("%s 的起點 %05X 接不上 %s 的終點 %05X",
				s.Name, s.Start, codeSegs[i-1].Name, codeSegs[i-1].End)
		}
	}
	// 程式碼段之後就是資料段。中間那一小塊 `seg035`（`34EC0`–`34ED0`）
	// 是 UNK，不在這張表裡，所以這裡只檢查不越界。
	if last := codeSegs[len(codeSegs)-1]; last.End > DSegBase {
		t.Errorf("最後一個程式碼段 %s 到 %05X，越過資料段起點 %05X",
			last.Name, last.End, DSegBase)
	}
	if DGroupParaOffset*16+0x10000 != DSegBase {
		t.Errorf("DGroupParaOffset ＝ %#X 推不回 DSegBase %#X",
			DGroupParaOffset, DSegBase)
	}
}

// TestRoutesParse 守著兩條實跑過的路線沒有被打錯字。
//
// 步數是 remake 專案 `docs/verification/dosgolem-vs-dosbox-20260910.md`
// 記下來的：走到鏡子 31 步、招募後到側面樓梯 15 步。
func TestRoutesParse(t *testing.T) {
	if len(MirrorRoute) != 31 {
		t.Errorf("到鏡子的路線 %d 步，收據記的是 31", len(MirrorRoute))
	}
	if len(StairsRoute) != 15 {
		t.Errorf("到樓梯的路線 %d 步，收據記的是 15", len(StairsRoute))
	}
	if MirrorRoute[0] != Forward || MirrorRoute[4] != TurnLeft {
		t.Errorf("到鏡子的前五步 ＝ %v，預期 f f f f l", MirrorRoute[:5])
	}
	if _, err := ParseMoves("f x f"); err == nil {
		t.Error("不認得的動作被接受了——打錯字會變成少走一步")
	}
	got, err := ParseMoves("f b l r sl sr")
	if err != nil {
		t.Fatal(err)
	}
	want := []Move{Forward, Backward, TurnLeft, TurnRight, StrafeLeft, StrafeRight}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 個解成 %s，預期 %s", i, got[i], want[i])
		}
	}
}

// TestMoveHotzonesAreInTheButtonBlock 守著六顆移動鈕的座標落在原版的熱區裡。
//
// 範圍來自 remake 專案 `docs/spec/73` §5 的滑鼠對照表（指令 1／3／2／6／5／4，
// `x 234–318`、`y 125–167`）。⚠ 座標寫錯的症狀是「畫面沒變」，
// 與「遊戲還沒準備好收這個點擊」長得一模一樣。
func TestMoveHotzonesAreInTheButtonBlock(t *testing.T) {
	seen := map[[2]int]bool{}
	for m := TurnLeft; m <= StrafeRight; m++ {
		x, y := m.Hot()
		if x < 234 || x > 318 || y < 125 || y > 167 {
			t.Errorf("%s 的座標 (%d,%d) 在按鈕區 x234–318 y125–167 之外", m, x, y)
		}
		if seen[[2]int{x, y}] {
			t.Errorf("%s 的座標 (%d,%d) 與別顆鈕重複", m, x, y)
		}
		seen[[2]int{x, y}] = true
	}
	// 上下兩排：轉向與前進在上排，側移與後退在下排。
	if uy, dy := moveHot[Forward][1], moveHot[Backward][1]; uy >= dy {
		t.Errorf("前進在 y=%d、後退在 y=%d，上下排反了", uy, dy)
	}
}

// TestDoorButtonIsInsideTheDungeonView 守著門鈕的點擊座標落在地城視窗裡。
//
// 地城視窗是 `x 0–223`、`y 33–168`（remake 專案 `docs/spec/73` §5 的
// 滑鼠對照表，指令 80）。點到視窗外面的話原版收到的是**別的指令**，
// 而畫面上看起來只是「門沒開」。
func TestDoorButtonIsInsideTheDungeonView(t *testing.T) {
	if DoorButtonX < 0 || DoorButtonX > 223 {
		t.Errorf("門鈕 x ＝ %d 不在地城視窗的 0–223 裡", DoorButtonX)
	}
	if DoorButtonY < 33 || DoorButtonY > 168 {
		t.Errorf("門鈕 y ＝ %d 不在地城視窗的 33–168 裡", DoorButtonY)
	}
	// 框是 x 167–174、y 43–51（視窗座標），加上視窗原點 (0,33)。
	if DoorButtonX < 167 || DoorButtonX > 174 {
		t.Errorf("門鈕 x ＝ %d 不在 DOS 版的框 167–174 裡", DoorButtonX)
	}
	if DoorButtonY < 43+33 || DoorButtonY > 51+33 {
		t.Errorf("門鈕 y ＝ %d 不在 DOS 版的框 76–84 裡", DoorButtonY)
	}
}
