package strikecommander

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// --- 002 §3.1 第 3 項：ParseScript ---

func TestParseScript(t *testing.T) {
	src := `# 註解與空白列略過

move 1000 160 75
down 1010 160 75
up   1020 160 75
shot 1030
shot 1040 hangar
stop 1050
`
	sc, err := ParseScript([]byte(src), 999)
	if err != nil {
		t.Fatal(err)
	}
	want := Script{
		Inputs: []ScriptedInput{{1000, "move", 160, 75}, {1010, "down", 160, 75}, {1020, "up", 160, 75}},
		Shots:  []Shot{{1030, ""}, {1040, "hangar"}},
		Stop:   1050,
	}
	if !reflect.DeepEqual(sc, want) {
		t.Fatalf("解析 %+v，要 %+v", sc, want)
	}
	// 只有 stop 也可以。
	if sc, err := ParseScript([]byte("stop 5\n"), 4); err != nil || sc.Stop != 5 || sc.Inputs != nil || sc.Shots != nil {
		t.Fatalf("只有 stop：%+v %v", sc, err)
	}
}

func TestParseScriptErrors(t *testing.T) {
	cases := []struct {
		name, src string
		start     uint64
		want      string
	}{
		{"不遞增", "move 30 1 1\nmove 20 1 1\nstop 40\n", 0, "不遞增"},
		{"第一個事件不大於起始步數", "move 10 1 1\nstop 40\n", 10, "不大於起始步數"},
		{"相鄰差 1", "move 20 1 1\nmove 21 1 1\nstop 40\n", 0, "只差 1 步"},
		{"同一步兩事件", "move 20 1 1\nshot 20\nstop 40\n", 0, "兩個事件"},
		{"超過 stop", "move 10 1 1\nstop 20\nmove 30 1 1\n", 0, "超過 stop"},
		{"缺 stop", "move 10 1 1\nshot 20\n", 0, "缺 stop"},
		{"未知指令", "grid 10 0 0 10 10 5 5 100\nstop 2000\n", 0, "不認得"},
		{"兩個 stop", "stop 10\nstop 20\n", 0, "只能 1 個"},
		{"move 欄位不足", "move 10 1\nstop 20\n", 0, "格式"},
		{"shot 欄位過多", "shot 10 a b\nstop 20\n", 0, "格式"},
		{"步數不是數字", "move x 1 1\nstop 20\n", 0, "格式"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseScript([]byte(c.src), c.start)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("要含 %q 的錯誤，得 %v", c.want, err)
			}
		})
	}
}

// testdata/case_d.script 是 002 §3.2 案例 D 的腳本（長測試用），這裡只核對它能解析、內容照規格。
func TestCaseDScript(t *testing.T) {
	b, err := os.ReadFile("testdata/case_d.script")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := ParseScript(b, caseDStart)
	if err != nil {
		t.Fatal(err)
	}
	want := Script{
		Inputs: []ScriptedInput{
			{5000650000, "move", 160, 75}, {5001000000, "down", 160, 75}, {5004000000, "up", 160, 75},
			{5600000000, "move", 56, 120}, {5620000000, "move", 56, 88},
		},
		Shots: []Shot{{5606000000, ""}, {5626000000, ""}},
		Stop:  5630000000,
	}
	if !reflect.DeepEqual(sc, want) {
		t.Fatalf("案例 D 腳本 %+v，要 %+v", sc, want)
	}
}
