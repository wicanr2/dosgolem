package dm_test

import (
	"os"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/apps/dm"
	"github.com/wicanr2/dosgolem/oracle"
)

// 這一份要真的跑原版，所以吃環境變數；沒有素材就跳過。
//
//	DOSGOLEM_ORIG=~/cht/dungeon_master/workplace/dosgolem/dmdata \
//	DOSGOLEM_TEST_EXE=/orig/dm.exe DOSGOLEM_TEST_ROOT=/orig \
//	    tools/go.sh test ./apps/dm -run Live -v
//
// ⚠ **缺素材只能 skip，不能偽造 pass**（`docs/spec/198` §4）。
func load(t *testing.T) *oracle.Oracle {
	t.Helper()
	exe, root := os.Getenv("DOSGOLEM_TEST_EXE"), os.Getenv("DOSGOLEM_TEST_ROOT")
	if exe == "" || root == "" {
		t.Skip("要 DOSGOLEM_TEST_EXE 與 DOSGOLEM_TEST_ROOT（玩家自備的原版素材）")
	}
	if testing.Short() {
		t.Skip("-short：開機要三億道指令")
	}
	o, err := dm.Load(exe, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	return o
}

// TestLiveBindFindsEngineBase 是位址基底的驗收。
//
// ⚠ **「沒有崩潰」不是驗收。** 基底錯了讀到的是另一塊記憶體，
// 而且**不會報錯**——解出來的東西看起來仍然像數字。所以這裡釘三件事：
// 殼鏈真的走到 `fires`、算出來的換算常數等於 dungeon_master 專案量到的
// `0x9BB0`、而且 DGROUP 與程式自己的 `DS` 一致。
func TestLiveBindFindsEngineBase(t *testing.T) {
	o := load(t)
	if err := dm.Boot(o); err != nil {
		t.Fatalf("開機：%v", err)
	}
	// ⚠ `int 21h AH=3Bh`（切目錄）在 dosgolem 是**有實作但刻意記一筆**的：
	// 目錄是假的，開檔一律相對 root。DM 切到遊戲目錄而 root 就是它，
	// 所以無害——但「無害」要有訊號證明，所以下面連帶檢查沒有開不到的檔。
	for _, r := range o.Unimplemented() {
		if strings.HasPrefix(r, "int 21h AH=3B") {
			continue
		}
		t.Errorf("還有沒實作的服務：%s", r)
	}
	// ⚠ `dmset` 開不到是**正常的**：那是 `selector` 存設定用的檔，
	// 第一次跑本來就沒有——主控台上那三個問題（顯示卡／音效／輸入）
	// 正是因為它不存在才會問。`SelectorAnswers` 回答的就是這三個。
	for _, name := range o.Missing() {
		if strings.EqualFold(name, "dmset") {
			continue
		}
		t.Errorf("有開不到的檔：%s（假目錄的副作用會長成這個樣子）", name)
	}

	b, err := dm.Bind(o)
	if err != nil {
		t.Fatalf("Bind：%v（EXEC 紀錄 %v）", err, o.ExecLog())
	}
	// dungeon_master 專案 `docs/re/71`／`73` 的所有位址都建立在這個常數上。
	if got := b.IDAOffset(); got != 0x9BB0 {
		t.Errorf("IDA 換算常數 ＝ %#X，預期 0x9BB0——"+
			"DOS 的記憶體配置與取位址時不同，整張位址表要重新對過", got)
	}
	t.Logf("fires 映像段 %04X，DGROUP %04X，IDA 偏移 %#X",
		b.ImageSeg(), b.DGroupSeg(), b.IDAOffset())
}

// TestLiveTickBoundaryAdvancesGameTime 是刻邊界的驗收。
//
// 掛在 `0x10D94` 的回呼每觸發一次，`GameTime` 就該多一。兩個獨立的計數
// （回呼觸發次數與遊戲自己的 `GameTime`）要一致——只看其中一個的話，
// 「掛錯位址」與「遊戲沒在跑」分不出來。
func TestLiveTickBoundaryAdvancesGameTime(t *testing.T) {
	o := load(t)
	if err := dm.Boot(o); err != nil {
		t.Fatalf("開機：%v", err)
	}
	b, err := dm.Bind(o)
	if err != nil {
		t.Fatalf("Bind：%v", err)
	}

	c := b.Attach(false) // 巡檢關著：這一支只驗時鐘
	const want = 20
	before := b.GameTime()
	if err := o.RunUntil(c.AfterTicks(want), oracle.Budget(dm.TickBudget(want))); err != nil {
		t.Fatalf("跑 %d 刻：%v", want, err)
	}
	after := b.GameTime()

	// ⚠ 回呼是在 `add word_38B54, 1` **之前**觸發的，所以 `c.Tick()`
	// 是遞增前的值，比 `GameTime` 少一。
	if got := after - before; got < want {
		t.Errorf("GameTime 走了 %d 刻（%d → %d），預期至少 %d",
			got, before, after, want)
	}
	if c.Boundaries() == 0 {
		t.Fatal("刻邊界一次都沒觸發——位址掛錯了")
	}
	if d := int64(after-before) - int64(c.Boundaries()); d < 0 || d > 1 {
		t.Errorf("GameTime 走了 %d 刻，但回呼觸發 %d 次——"+
			"兩個計數對不上，表示那個位址不是唯一的遞增處",
			after-before, c.Boundaries())
	}
	t.Logf("GameTime %d → %d，回呼 %d 次", before, after, c.Boundaries())
}

// TestLiveStubMaximumLoad 是負重 stub 的驗收，配反對照。
//
// 直接呼叫 `F309` 比等遊戲自己叫它可靠：招募大廳還沒有勇士，
// 而 stub 是否生效與有沒有隊伍無關。
//
// ⚠ 少了反對照就分不出「stub 生效了」與「這個值本來就是它」。
func TestLiveStubMaximumLoad(t *testing.T) {
	o := load(t)
	if err := dm.Boot(o); err != nil {
		t.Fatalf("開機：%v", err)
	}
	b, err := dm.Bind(o)
	if err != nil {
		t.Fatalf("Bind：%v", err)
	}

	// 參數是一個 far 指標，指向 CHAMPION。招募大廳裡第 0 格是空的，
	// 但 stub 掉之後常式根本不會執行，指向哪裡都無所謂——
	// 這正是「整棵呼叫樹零副作用」的另一面。
	champ := b.DS(0x3592)

	// ⚠ **要跳進去執行就得用 [dm.Bridge.Code]**，不是 `IDA()`：
	// `F309` 在 `seg010`，而 `IDA()` 回的是正規化的段。用錯段的話
	// 機器碼位置是對的，但常式裡第一個 `call near ptr` 就跳到別的地方
	// ——實測過一次，跑滿預算停在 `seg028` 的繪圖常式裡。
	call, err := b.Code(dm.IDAMaximumLoad)
	if err != nil {
		t.Fatalf("F309 的執行位址：%v", err)
	}

	if err := b.StubMaximumLoad(dm.SweepMaximumLoad); err != nil {
		t.Fatalf("掛 stub：%v", err)
	}
	got, err := o.Call(call, champ.Off, champ.Seg)
	if err != nil {
		t.Fatalf("呼叫 F309：%v", err)
	}
	if got&0xFFFF != dm.SweepMaximumLoad {
		t.Errorf("stub 開著時 F309 回 %d（AX ＝ %d），預期 %d",
			got, got&0xFFFF, dm.SweepMaximumLoad)
	}

	// 反對照：取消 stub，同一個呼叫要走真正的計算。
	if err := b.StubMaximumLoad(0); err != nil {
		t.Fatalf("取消 stub：%v", err)
	}
	real, err := o.Call(call, champ.Off, champ.Seg)
	if err != nil {
		t.Fatalf("取消 stub 後呼叫 F309：%v", err)
	}
	if real&0xFFFF == dm.SweepMaximumLoad {
		t.Errorf("取消 stub 之後還是回 %d——stub 沒拆掉，"+
			"這一支證明不了任何事", real&0xFFFF)
	}
	t.Logf("F309：stub ＝ %d，真算 ＝ %d", got&0xFFFF, real&0xFFFF)
}

// TestLiveStubMaximumLoadRejectsOverflow 守住「值要放得進 AX」。
//
// ⚠ 給 `1<<20` 的話 `AX` ＝ `0`，**全隊反而變成永遠超重**——
// 效果正好相反，而且不會有任何錯誤訊息。
func TestLiveStubMaximumLoadRejectsOverflow(t *testing.T) {
	o := load(t)
	if err := dm.Boot(o); err != nil {
		t.Fatalf("開機：%v", err)
	}
	b, err := dm.Bind(o)
	if err != nil {
		t.Fatalf("Bind：%v", err)
	}
	if err := b.StubMaximumLoad(1 << 20); err == nil {
		t.Error("1<<20 被接受了——AX 會截成 0，全隊變成永遠超重")
	}
}

// TestLiveReadPartyInHall 讀招募大廳的狀態。
//
// 那裡**還沒有勇士**，所以正確答案是「0 位」。這一支驗的是
// 「空的那幾格不會被解成無名勇士」，以及地圖索引與面向落在合法範圍。
func TestLiveReadPartyInHall(t *testing.T) {
	o := load(t)
	if err := dm.Boot(o); err != nil {
		t.Fatalf("開機：%v", err)
	}
	b, err := dm.Bind(o)
	if err != nil {
		t.Fatalf("Bind：%v", err)
	}
	p, err := b.ReadParty()
	if err != nil {
		t.Fatalf("ReadParty：%v", err)
	}
	if len(p.Champions) != 0 {
		t.Errorf("招募大廳解出 %d 位勇士，預期 0：%+v", len(p.Champions), p.Champions)
	}
	if p.MapIndex != 0 {
		t.Errorf("地圖索引 ＝ %d，招募大廳在第 0 層", p.MapIndex)
	}
	if p.X < 0 || p.X > 31 || p.Y < 0 || p.Y > 31 {
		t.Errorf("座標 (%d,%d) 不在 32×32 之內——位址或基底不對", p.X, p.Y)
	}
	t.Logf("隊伍：地圖 %d，(%d,%d) 面向 %d，GameTime %d",
		p.MapIndex, p.X, p.Y, p.Facing, p.GameTime)
}

// hall 走完招募流程，回到「隊伍裡有一位 ELIJA」的狀態。
//
// ⚠ **巡檢非招募不可**：地板感應器要隊伍裡有人才會觸發
// （`SENSOR.C:474` 的 `PartyChampionCount == 0` 就跳過）。空隊伍走過壓力板
// 門不會開，路線走到一半就卡住——而畫面看起來只是「沒走那麼遠」。
func hall(t *testing.T) (*dm.Bridge, *oracle.Oracle) {
	t.Helper()
	o := load(t)
	if err := dm.Boot(o); err != nil {
		t.Fatalf("開機：%v", err)
	}
	b, err := dm.Bind(o)
	if err != nil {
		t.Fatalf("Bind：%v", err)
	}
	if err := b.Walk(dm.MirrorRoute); err != nil {
		t.Fatalf("走到鏡子：%v", err)
	}
	if err := b.RecruitFirstChampion(); err != nil {
		t.Fatalf("招募：%v", err)
	}
	return b, o
}

// TestLiveRecruitReadsChampion 是 `ReadParty` 對真實資料的驗收。
//
// 招募之後隊伍要有**一位**勇士，名字是 `ELIJA`——那個名字在
// remake 專案的 `hired.state` 傾印裡讀到過，兩邊是同一位。
func TestLiveRecruitReadsChampion(t *testing.T) {
	b, _ := hall(t)
	p, err := b.ReadParty()
	if err != nil {
		t.Fatalf("ReadParty：%v", err)
	}
	if len(p.Champions) != 1 {
		t.Fatalf("招募之後解出 %d 位勇士，預期 1：%+v", len(p.Champions), p.Champions)
	}
	c := p.Champions[0]
	if c.Name != "ELIJA" {
		t.Errorf("名字 ＝ %q，預期 %q", c.Name, "ELIJA")
	}
	if c.MaxHealth <= 0 || c.CurrentHealth <= 0 {
		t.Errorf("生命力 ＝ %d／%d，招募完應該是滿的", c.CurrentHealth, c.MaxHealth)
	}
	if c.Food <= 0 || c.Water <= 0 {
		t.Errorf("食物／水 ＝ %d／%d，招募完不該是負的", c.Food, c.Water)
	}
	t.Logf("%s「%s」：生命力 %d/%d，食物 %d，水 %d，負重 %d，中毒 %d",
		c.Name, c.Title, c.CurrentHealth, c.MaxHealth, c.Food, c.Water,
		c.Load, c.PoisonEventCount)
}

// TestLiveSweepHoldsFoodAndWater 是每刻寫回的驗收，**配反對照**。
//
// ⚠ 少了反對照就分不出「擋住了」與「這個情境本來就不會發生」：
// 食物與水掉得很慢，跑幾十刻看不出變化也可能只是時間不夠。所以這裡先用
// 關著的那一輪證明「同樣的刻數確實會掉」，開著的那一輪才有意義。
func TestLiveSweepHoldsFoodAndWater(t *testing.T) {
	b, o := hall(t)

	// 先把食物與水壓低，讓「會不會掉」在幾十刻內就看得出來。
	// ⚠ 壓到 −512（`0FE00h`）以下就進入挨餓，那是另一條路徑；
	// 這裡只壓到一個明顯低於滿值、又還在正常範圍的數。
	const pressed = 300
	if err := b.SetFoodWater(0, pressed, pressed); err != nil {
		t.Fatalf("壓低食物與水：%v", err)
	}

	c := b.Attach(false) // 先跑反對照：巡檢關著
	const ticks = 60
	if err := o.RunUntil(c.AfterTicks(ticks), oracle.Budget(dm.TickBudget(ticks))); err != nil {
		t.Fatalf("巡檢關著跑 %d 刻：%v", ticks, err)
	}
	off, err := b.ReadParty()
	if err != nil {
		t.Fatal(err)
	}
	if len(off.Champions) != 1 {
		t.Fatalf("反對照那一輪解出 %d 位勇士", len(off.Champions))
	}
	if off.Champions[0].Food >= pressed {
		t.Fatalf("巡檢關著跑 %d 刻，食物從 %d 變成 %d——沒有下降，"+
			"這個情境證明不了巡檢有沒有作用",
			ticks, pressed, off.Champions[0].Food)
	}

	// 正對照：同一個機器狀態接著跑，這次開著。
	c.SetSweep(true)
	if err := o.RunUntil(c.AfterTicks(ticks), oracle.Budget(dm.TickBudget(ticks))); err != nil {
		t.Fatalf("巡檢開著跑 %d 刻：%v", ticks, err)
	}
	on, err := b.ReadParty()
	if err != nil {
		t.Fatal(err)
	}
	got := on.Champions[0]
	if got.Food != dm.FoodWaterMaximum || got.Water != dm.FoodWaterMaximum {
		t.Errorf("巡檢開著跑 %d 刻之後食物 ＝ %d、水 ＝ %d，預期都是 %d",
			ticks, got.Food, got.Water, dm.FoodWaterMaximum)
	}
	if got.CurrentHealth != got.MaxHealth {
		t.Errorf("生命力 ＝ %d／%d，巡檢應該每刻寫回上限",
			got.CurrentHealth, got.MaxHealth)
	}
	if got.PoisonEventCount != 0 {
		t.Errorf("中毒事件數 ＝ %d，巡檢應該清零", got.PoisonEventCount)
	}
	t.Logf("反對照：食物 %d → %d；巡檢開著：→ %d",
		pressed, off.Champions[0].Food, got.Food)
}
