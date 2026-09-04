package rich2_test

import (
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/oracle/rich2"
)

func load(t *testing.T) *oracle.Oracle {
	t.Helper()
	exe, root := os.Getenv("DOSGOLEM_TEST_EXE"), os.Getenv("DOSGOLEM_TEST_ROOT")
	if exe == "" || root == "" {
		t.Skip("要 DOSGOLEM_TEST_EXE 與 DOSGOLEM_TEST_ROOT（玩家自備的原版素材）")
	}
	o, err := oracle.Load(exe, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	return o
}

// TestToBoard 釘住整條路徑：冷啟動 → 防拷 → 主選單 → 棋盤。
//
// 這是對拍的主戰場入口。任何一段斷掉，之後的對拍全部做不了，
// 所以這條路徑本身要有測試護著。
func TestToBoard(t *testing.T) {
	o := load(t)
	if err := rich2.ToBoard(o); err != nil {
		t.Fatal(err)
	}
	t.Logf("到棋盤：%d 道指令，開了 %v", o.Steps(), o.Opened())

	// 新局該載入的東西（`rich2/docs/re/173`、`docs/formats/005`）。
	for _, f := range []string{"SAVE_7.DSK", "RICHA.RIX", "WOR.PAK", "PART1.PAK"} {
		found := false
		for _, g := range o.Opened() {
			if g == f {
				found = true
			}
		}
		if !found {
			t.Errorf("沒開 %s——走到的可能不是棋盤", f)
		}
	}

	// 棋盤用的色號比防拷畫面多得多（防拷是 81 種）。
	used := map[uint8]bool{}
	for _, c := range o.Indexed() {
		used[c] = true
	}
	if len(used) < 100 {
		t.Errorf("畫面只用了 %d 種色號，棋盤應該遠多於此——可能還在淡入", len(used))
	}
	t.Logf("棋盤用了 %d 種色號", len(used))
}

// TestToBoardIsDeterministic 釘住可重現性。
//
// **這是整個對拍的地基。** 同一組輸入不保證同一個畫面的話，
// 任何逐點比對的結果都不能拿來下結論——不合可能只是這一次跑得不一樣。
func TestToBoardIsDeterministic(t *testing.T) {
	if testing.Short() {
		t.Skip("要跑兩次完整路徑")
	}
	var shots [2][]uint8
	var steps [2]uint64
	for i := range shots {
		o := load(t)
		if err := rich2.ToBoard(o); err != nil {
			t.Fatal(err)
		}
		shots[i] = append([]uint8(nil), o.Indexed()...)
		steps[i] = o.Steps()
	}
	if steps[0] != steps[1] {
		t.Errorf("兩次走到棋盤的指令數不同：%d 與 %d", steps[0], steps[1])
	}
	diff := 0
	for i := range shots[0] {
		if shots[0][i] != shots[1][i] {
			diff++
		}
	}
	if diff != 0 {
		t.Errorf("兩次的棋盤畫面差 %d 點——對拍的地基不成立", diff)
	}
}
