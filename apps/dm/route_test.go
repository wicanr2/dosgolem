package dm

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// TestStepClickUsesTheRelativeFormula 釘住 `相對 ＝ (絕對 − 面向 + 4) % 4`
// （`docs/spec/198` §3.4）。面北時絕對方向就是相對方向，所以另外各測一個
// 面東與面西的，公式的正負號寫反才會紅。
func TestStepClickUsesTheRelativeFormula(t *testing.T) {
	cases := []struct {
		dir    string
		facing int
		want   Move
	}{
		{"N", 0, Forward}, {"E", 0, StrafeRight}, {"S", 0, Backward}, {"W", 0, StrafeLeft},
		{"N", 1, StrafeLeft}, {"E", 1, Forward}, {"S", 1, StrafeRight}, {"W", 1, Backward},
		{"N", 3, StrafeRight}, {"W", 3, Forward}, {"E", 3, Backward},
	}
	for _, c := range cases {
		x, y, err := RouteStep{Dir: c.dir, Ticks: 8}.Click(c.facing)
		if err != nil {
			t.Fatalf("%s 面向 %d：%v", c.dir, c.facing, err)
		}
		if wx, wy := c.want.Hot(); x != wx || y != wy {
			t.Errorf("%s 面向 %d：點 (%d,%d)，預期%s (%d,%d)", c.dir, c.facing, x, y, c.want, wx, wy)
		}
	}
}

func TestStepClickTurnAndPress(t *testing.T) {
	right := RouteStep{Action: "turn", Ticks: 8, Expect: RoutePoint{Facing: 0}}
	if x, y, err := right.Click(3); err != nil {
		t.Fatal(err)
	} else if wx, wy := TurnRight.Hot(); x != wx || y != wy {
		t.Errorf("面西轉到面北要右轉，點了 (%d,%d)", x, y)
	}
	left := RouteStep{Action: "turn", Ticks: 8, Expect: RoutePoint{Facing: 3}}
	if x, y, err := left.Click(0); err != nil {
		t.Fatal(err)
	} else if wx, wy := TurnLeft.Hot(); x != wx || y != wy {
		t.Errorf("面北轉到面西要左轉，點了 (%d,%d)", x, y)
	}
	if _, _, err := (RouteStep{Action: "turn", Ticks: 8, Expect: RoutePoint{Facing: 2}}).Click(0); err == nil {
		t.Error("一步轉兩個直角被接受了")
	}
	if x, y, err := (RouteStep{Action: "press", Ticks: 4}).Click(1); err != nil || x != DoorButtonX || y != DoorButtonY {
		t.Errorf("press 點 (%d,%d) err=%v，預期門鈕 (%d,%d)", x, y, err, DoorButtonX, DoorButtonY)
	}
}

func TestRouteValidateRejects(t *testing.T) {
	good := Route{
		Map:   2,
		Save:  &RouteSave{Name: "DMSAVE.DAT", SHA256: "x", GameTime: 19662},
		Start: RoutePoint{X: 0, Y: 13, Facing: 0},
		Steps: []RouteStep{
			{Dir: "E", Ticks: 8, Expect: RoutePoint{X: 1, Y: 13, Facing: 0}},
			{Action: "turn", Ticks: 8, Expect: RoutePoint{X: 1, Y: 13, Facing: 1}},
		},
		Checkpoints: []int{0, 2},
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("合法的路線被拒絕：%v", err)
	}
	bad := map[string]func(r *Route){
		"沒有 save": func(r *Route) { r.Save = nil },
		"刻數是零":    func(r *Route) { r.Steps[0].Ticks = 0 },
		"不認得的動作":  func(r *Route) { r.Steps[0].Action = "teleport" },
		"方向字母錯":   func(r *Route) { r.Steps[0].Dir = "X" },
		"一步轉兩個直角": func(r *Route) { r.Steps[1].Expect.Facing = 2 },
		"檢查點超出步數": func(r *Route) { r.Checkpoints = []int{3} },
	}
	for name, mutate := range bad {
		r := good
		r.Steps = append([]RouteStep(nil), good.Steps...)
		s := *good.Save
		r.Save = &s
		mutate(&r)
		if err := r.Validate(); err == nil {
			t.Errorf("%s：被接受了", name)
		}
	}
}

// TestPartyJSONKeys 釘住 `party.json` 的鍵（§3.5）。remake 側的檢查點檔用同一組，
// 改了一邊沒改另一邊的話比對工具會把整欄當成缺漏。
func TestPartyJSONKeys(t *testing.T) {
	raw, err := json.Marshal(Party{Champions: []Champion{{}}})
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	var champs []map[string]json.RawMessage
	if err := json.Unmarshal(top["champions"], &champs); err != nil {
		t.Fatal(err)
	}
	keys := func(m map[string]json.RawMessage) string {
		var k []string
		for n := range m {
			k = append(k, n)
		}
		sort.Strings(k)
		return strings.Join(k, ",")
	}
	if got, want := keys(top), "champions,facing,game_time,map_index,x,y"; got != want {
		t.Errorf("隊伍的鍵 %s，預期 %s", got, want)
	}
	want := "current_health,food,index,load,max_health,name,poison_event_count,slots,title,water"
	if got := keys(champs[0]); got != want {
		t.Errorf("勇士的鍵 %s，預期 %s", got, want)
	}
	var slots []uint16
	if err := json.Unmarshal(champs[0]["slots"], &slots); err != nil || len(slots) != SlotCount {
		t.Errorf("slots 要是 %d 個數字：%s（%v）", SlotCount, champs[0]["slots"], err)
	}
}
