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
		Inputs: []ScriptedInput{{Steps: 1000, Kind: "move", X: 160, Y: 75}, {Steps: 1010, Kind: "down", X: 160, Y: 75}, {Steps: 1020, Kind: "up", X: 160, Y: 75}},
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
			{Steps: 5000650000, Kind: "move", X: 160, Y: 75}, {Steps: 5001000000, Kind: "down", X: 160, Y: 75}, {Steps: 5004000000, Kind: "up", X: 160, Y: 75},
			{Steps: 5600000000, Kind: "move", X: 56, Y: 120}, {Steps: 5620000000, Kind: "move", X: 56, Y: 88},
		},
		Shots: []Shot{{5606000000, ""}, {5626000000, ""}},
		Stop:  5630000000,
	}
	if !reflect.DeepEqual(sc, want) {
		t.Fatalf("案例 D 腳本 %+v，要 %+v", sc, want)
	}
}

// 003 §4.2 案例 E 的腳本（RE 014 kb3.script 的輸入部分）。狀態檔步數在第一個事件之前，這裡以 0 解析。
func TestCaseEScript(t *testing.T) {
	b, err := os.ReadFile("testdata/case_e.script")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := ParseScript(b, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := Script{
		Inputs: []ScriptedInput{
			{Steps: 4490600000, Kind: "move", X: 40, Y: 96}, {Steps: 4492000000, Kind: "down", X: 40, Y: 96}, {Steps: 4495000000, Kind: "up", X: 40, Y: 96},
			{Steps: 4730000000, Kind: "move", X: 296, Y: 56}, {Steps: 4732000000, Kind: "down", X: 296, Y: 56}, {Steps: 4735000000, Kind: "up", X: 296, Y: 56},
			{Steps: 4984000000, Kind: "move", X: 8, Y: 156},
		},
		Shots: []Shot{{caseEShot, ""}},
		Stop:  caseETo,
	}
	if !reflect.DeepEqual(sc, want) {
		t.Fatalf("案例 E 腳本 %+v，要 %+v", sc, want)
	}
}

// --- 007 §3.8：kdown、kup ---

func TestParseScriptKeys(t *testing.T) {
	src := "kdown 1000 1E\nkup 1010 1e\nkdown 1020 01\nkup 1030 7F\nmove 1040 3 4\nstop 1050\n"
	sc, err := ParseScript([]byte(src), 999)
	if err != nil {
		t.Fatal(err)
	}
	want := []ScriptedInput{
		{Steps: 1000, Kind: "kdown", Scan: 0x1E}, {Steps: 1010, Kind: "kup", Scan: 0x1E},
		{Steps: 1020, Kind: "kdown", Scan: 0x01}, {Steps: 1030, Kind: "kup", Scan: 0x7F},
		{Steps: 1040, Kind: "move", X: 3, Y: 4},
	}
	if !reflect.DeepEqual(sc.Inputs, want) || sc.Stop != 1050 {
		t.Fatalf("解析 %+v，要 %+v", sc, want)
	}
	cases := []struct{ name, src, want string }{
		{"掃描碼 80（含 bit 7）", "kdown 10 80\nstop 20\n", "01–7F"},
		{"掃描碼 FF", "kup 10 FF\nstop 20\n", "01–7F"},
		{"掃描碼 0", "kdown 10 0\nstop 20\n", "01–7F"},
		{"掃描碼 100", "kdown 10 100\nstop 20\n", "01–7F"},
		{"掃描碼不是十六進位", "kdown 10 zz\nstop 20\n", "01–7F"},
		{"缺掃描碼", "kdown 10\nstop 20\n", "格式"},
		{"欄位過多", "kup 10 1E 2\nstop 20\n", "格式"},
		{"步數不是數字", "kdown x 1E\nstop 20\n", "格式"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseScript([]byte(c.src), 0)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("要含 %q 的錯誤，得 %v", c.want, err)
			}
		})
	}
}

// 007 §4.2 到 §4.4 的案例腳本：能解析，鍵盤、shot 與 stop 照規格。
func TestCaseIJKScripts(t *testing.T) {
	type sum struct {
		inputs, keys int
		shots        []Shot
		stop         uint64
	}
	want := map[string]sum{
		"testdata/case_i.script": {14, 0, []Shot{{4770000000, "confirm"}, {4806000000, "nopressed"}, {4820000000, "no10"}, {4831000000, "savepressed"}}, 4832000000},
		"testdata/case_j.script": {32, 16, []Shot{{4904000000, "bs"}, {4909000000, "c"}, {4920000000, "ow"}, {4936000000, "nopressed"}, {4950000000, "no10"},
			{4975000000, "ow2"}, {4991000000, "yespressed"}, {4998000000, "yesredraw"}, {5005000000, "yes10"}}, 5006000000},
		"testdata/case_k.script": {12, 0, []Shot{{5261000000, "tsave"}, {5280000000, "tq"}, {5295000000, "tnopressed"}, {5320000000, "tno"}}, 5330000001},
	}
	for path, w := range want {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sc, err := ParseScript(b, 0)
		if err != nil {
			t.Fatalf("%s：%v", path, err)
		}
		keys := 0
		for _, in := range sc.Inputs {
			if in.Kind == "kdown" || in.Kind == "kup" {
				keys++
			}
		}
		if len(sc.Inputs) != w.inputs || keys != w.keys || !reflect.DeepEqual(sc.Shots, w.shots) || sc.Stop != w.stop {
			t.Errorf("%s：輸入 %d（鍵盤 %d）、shot %+v、stop %d", path, len(sc.Inputs), keys, sc.Shots, sc.Stop)
		}
	}
	// 案例 I：savepressed 前的 SAVE 按下不放開（007 §4.2）。
	b, _ := os.ReadFile("testdata/case_i.script")
	sc, _ := ParseScript(b, 0)
	tail := sc.Inputs[len(sc.Inputs)-2:]
	if !reflect.DeepEqual(tail, []ScriptedInput{{Steps: 4821000000, Kind: "move", X: 171, Y: 132}, {Steps: 4825000000, Kind: "down", X: 171, Y: 132}}) {
		t.Errorf("案例 I 最後兩筆輸入 %+v", tail)
	}
}
