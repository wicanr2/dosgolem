package wolong_test

import (
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/apps/wolong"
	"github.com/wicanr2/dosgolem/oracle"
)

// 這一份要真的跑原版，所以吃環境變數；沒有素材就跳過。
//
//	DOSGOLEM_TEST_EXE=…/KI.EXE DOSGOLEM_TEST_ROOT=…/dosv \
//	    tools/go.sh test ./apps/wolong -v
func load(t *testing.T) *oracle.Oracle {
	t.Helper()
	exe, root := os.Getenv("DOSGOLEM_TEST_EXE"), os.Getenv("DOSGOLEM_TEST_ROOT")
	if exe == "" || root == "" {
		t.Skip("要 DOSGOLEM_TEST_EXE 與 DOSGOLEM_TEST_ROOT（玩家自備的原版素材）")
	}
	o, err := wolong.Load(exe, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	return o
}

// TestBootReachesNewGamePrompt 是整條鏈的煙霧測試。
//
// ⚠ **「沒有崩潰」不是驗收。** 字型讀錯一個 byte 也不會崩潰，
// 只會畫出看起來像字的東西。所以這裡另外釘三件可以量的事：
// 沒有未實作的服務、畫面是 640×480、內容確實落在第 40–439 列。
func TestBootReachesNewGamePrompt(t *testing.T) {
	o := load(t)
	if err := wolong.ToNewGamePrompt(o); err != nil {
		t.Fatalf("開機：%v", err)
	}
	if rep := o.Unimplemented(); len(rep) > 0 {
		t.Errorf("還有沒實作的服務：%v", rep)
	}
	w, h, px := o.Screen()
	if w != wolong.ScreenW || h != wolong.ScreenH {
		t.Fatalf("畫面是 %d×%d，預期 %d×%d", w, h, wolong.ScreenW, wolong.ScreenH)
	}
	top, bottom := -1, -1
	for y := 0; y < h; y++ {
		lit := 0
		for x := 0; x < w; x++ {
			if px[y*w+x] != 0 {
				lit++
			}
		}
		if lit > 20 {
			if top < 0 {
				top = y
			}
			bottom = y
		}
	}
	// 內容的 y 原點是第 40 列（`docs/spec/007` §2）。這是兩個獨立來源
	// 對上的數字：繪製常式的 VRAM 段 A0C8h，與 DOSBox-X 截圖量到的偏移。
	if top != wolong.ContentTop || bottom != wolong.ContentTop+wolong.ContentHigh-1 {
		t.Errorf("內容落在第 %d–%d 列，預期 %d–%d",
			top, bottom, wolong.ContentTop, wolong.ContentTop+wolong.ContentHigh-1)
	}
	if o.Steps() < 3_000_000 {
		t.Errorf("只跑了 %d 道指令就說畫面停住了——條件是不是在畫之前就成立？",
			o.Steps())
	}
}

// TestScenarioMenuNeedsAClick 釘住「點得進去」。
//
// 點擊路徑上有三個各自會安靜失效的東西：`int 33h AX=5` 的 BX 是輸入
// （問哪一個鍵）、平面模式的畫面不在 `Indexed()` 裡、座標是遊戲座標
// 不是 DOSBox 的視窗座標。**三個都會表現成「點了沒反應」。**
func TestScenarioMenuNeedsAClick(t *testing.T) {
	o := load(t)
	if err := wolong.ToNewGamePrompt(o); err != nil {
		t.Fatal(err)
	}
	_, _, before := o.Screen()
	before = append([]uint8(nil), before...)
	if err := wolong.ToScenarioMenu(o); err != nil {
		t.Fatalf("點 NEW GAME 的 YES：%v", err)
	}
	_, _, after := o.Screen()
	same := true
	for i := range before {
		if before[i] != after[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("點完畫面一點都沒變")
	}
}

// TestFromDOSBoxY 釘住視窗座標 → 遊戲座標的換算。
//
// ⚠ **分母是 479 不是 480。** 大部分點兩種算法一樣，只有少數差 1，
// 而那 1 個像素會讓主畫面的游標整塊對不上（62 點）——
// 看起來像「只差一點點」，不像「換算式錯了」。
func TestFromDOSBoxY(t *testing.T) {
	for _, c := range []struct{ win, game int }{
		{215, 179}, {190, 158}, {154, 128},
		{336, 279}, // ← 兩種算法在這裡分家：×400÷480 會得到 280
		{0, 0}, {479, 399},
	} {
		if got := wolong.FromDOSBoxY(c.win); got != c.game {
			t.Errorf("視窗 y=%d → %d，預期 %d", c.win, got, c.game)
		}
	}
}
