package dm_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/apps/dm"
)

// bundledSave 是 DOS 原版附的那一份 `DMSAVE.DAT`（`docs/spec/198` §3.6）。
// 下面的路線是 remake 從它產生的第 2 層巡檢路線的前三步。
const bundledSave = "b9f6a287e833c35b6bf926691b18536b643a4c2ccdd3431dc3404ca27b044789"

func shortRoute(t *testing.T) dm.Route {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(os.Getenv("DOSGOLEM_TEST_ROOT"), "DMSAVE.DAT"))
	if err != nil {
		t.Skipf("資料根沒有 DMSAVE.DAT：%v", err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != bundledSave {
		t.Skip("DMSAVE.DAT 不是原版附的那一份，這條路線不適用")
	}
	r := dm.Route{
		Map:   2,
		Save:  &dm.RouteSave{Name: "DMSAVE.DAT", SHA256: bundledSave, GameTime: 19662},
		Start: dm.RoutePoint{X: 0, Y: 13, Facing: 0},
		Steps: []dm.RouteStep{
			{Dir: "S", Ticks: 8, Expect: dm.RoutePoint{X: 0, Y: 14, Facing: 0}},
			{Dir: "S", Ticks: 8, Expect: dm.RoutePoint{X: 0, Y: 15, Facing: 0}},
			{Dir: "E", Ticks: 8, Expect: dm.RoutePoint{X: 1, Y: 15, Facing: 0}},
		},
		Checkpoints: []int{0, 3},
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	return r
}

func runShort(t *testing.T, r dm.Route) ([]dm.StepResult, error) {
	t.Helper()
	o := load(t)
	b, c, err := dm.Resume(o)
	if err != nil {
		t.Fatalf("讀檔：%v", err)
	}
	var got []dm.StepResult
	err = b.RunRoute(c, r, func(s dm.StepResult) error {
		got = append(got, s)
		return nil
	})
	return got, err
}

// TestLiveResumeRunsAShortRoute：讀檔之後第一個刻邊界的遊戲刻等於存檔、
// 位置等於 start，三步的 expect 都對上；連續兩次取樣的遊戲刻差正好是那一步的 ticks
// （停在刻邊界時取的是遞增前的值，§3.3）。
func TestLiveResumeRunsAShortRoute(t *testing.T) {
	r := shortRoute(t)
	got, err := runShort(t, r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(r.Steps)+1 {
		t.Fatalf("取樣 %d 次，預期 %d", len(got), len(r.Steps)+1)
	}
	if got[0].Party.GameTime != r.Save.GameTime {
		t.Errorf("第 0 步遊戲刻 %d，存檔是 %d", got[0].Party.GameTime, r.Save.GameTime)
	}
	if n := len(got[0].Party.Champions); n != 4 {
		t.Errorf("讀檔之後有 %d 位勇士，存檔是 4 位", n)
	}
	for k := 1; k < len(got); k++ {
		if d := got[k].Party.GameTime - got[k-1].Party.GameTime; d != r.Steps[k-1].Ticks {
			t.Errorf("第 %d 步走了 %d 刻，路線是 %d", k, d, r.Steps[k-1].Ticks)
		}
	}
}

// TestLiveRouteStopsOnWrongStart 是反對照：start 改錯，第 0 步就要停。
// 少了它分不出「起點對上了」與「根本沒在檢查」。
func TestLiveRouteStopsOnWrongStart(t *testing.T) {
	r := shortRoute(t)
	r.Start.X = 5
	_, err := runShort(t, r)
	var e *dm.ExpectError
	if !errors.As(err, &e) || e.Step != 0 {
		t.Fatalf("start 改錯卻得到 %v", err)
	}
}

// TestLiveRouteStopsOnWrongExpect 是反對照：第 2 步的 expect 改錯，要停在第 2 步，
// 而且第 1 步照樣通過。
func TestLiveRouteStopsOnWrongExpect(t *testing.T) {
	r := shortRoute(t)
	r.Steps[1].Expect.Y = 20
	got, err := runShort(t, r)
	var e *dm.ExpectError
	if !errors.As(err, &e) || e.Step != 2 {
		t.Fatalf("第 2 步的 expect 改錯卻得到 %v", err)
	}
	if last := got[len(got)-1].Party; last.X != 0 || last.Y != 15 {
		t.Errorf("停下時隊伍在 (%d,%d)，路線前兩步應該走到 (0,15)", last.X, last.Y)
	}
}

// TestLiveDungeonImageMatchesSave：讀檔之後第一個刻邊界的地城映像，長度與幾段不會在
// 一刻裡變的內容要等於存檔（§3.7）。指標或長度錯一個，這裡就對不上。
//
// 期望值來自原版附的 `DMSAVE.DAT` 地城段（dungeon_master 專案 `dmtool save info` 的段表）：
// 欄累計表 818 位元組、SquareFirstThings 3958、thing 資料 16470、RawMapData 12283。
func TestLiveDungeonImageMatchesSave(t *testing.T) {
	shortRoute(t) // 只為了確認存檔是原版附的那一份
	o := load(t)
	b, _, err := dm.Resume(o)
	if err != nil {
		t.Fatalf("讀檔：%v", err)
	}
	img, err := b.DungeonImage()
	if err != nil {
		t.Fatal(err)
	}
	if len(img) != 818+3958+16470+12283 {
		t.Fatalf("地城映像 %d 位元組，存檔是 %d", len(img), 818+3958+16470+12283)
	}
	at := func(off int, want string) {
		t.Helper()
		if got := hex.EncodeToString(img[off : off+16]); got != want {
			t.Errorf("位移 %d：%s，存檔是 %s", off, got, want)
		}
	}
	at(0, "0000000005000500070008000b000d00")              // 欄累計表
	at(818, "278e6b004104400ca6003f4c984c6c0e")            // SquareFirstThings
	at(818+3958+680, "940c0660000afeffe7e00001feff68f0")   // 型別 1（傳送器）
	at(818+3958+16470, "00000004002000000000000404040006") // RawMapData
}
