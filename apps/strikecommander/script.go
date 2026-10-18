package strikecommander

import (
	"fmt"
	"strconv"
	"strings"
)

// 收據工具的腳本輸入（002 §2.4）。每列一個指令，步數是絕對步數，座標是 DOS 邏輯像素：
//
//	move STEP X Y
//	down STEP X Y
//	up   STEP X Y
//	shot STEP [NAME]   以最後一次回掃的畫格出合成圖；NAME 可省，有就當輸出檔名的一部分
//	stop STEP
//
// 空白列與 # 開頭的列略過。cmd/strikecommander-census 的腳本不含 grid 時也能讀。

// ScriptedInput 是腳本的一筆滑鼠輸入。Kind 是 "move"、"down" 或 "up"。
type ScriptedInput struct {
	Steps uint64
	Kind  string
	X, Y  int
}

// Shot 是腳本的一筆 shot。
type Shot struct {
	Steps uint64
	Name  string
}

// Script 是解析後的腳本。Stop 是 stop 的步數。
type Script struct {
	Inputs []ScriptedInput
	Shots  []Shot
	Stop   uint64
}

// scriptEvent 是驗證用的一筆事件（含 stop），依檔案順序。
type scriptEvent struct {
	steps uint64
	kind  string
}

// ParseScript 解析腳本並以 start（狀態檔步數）驗證。狀態檔步數未知時給 0，
// RunReceipt 啟動後會以 BootReceipt.Steps 再驗一次。
func ParseScript(b []byte, start uint64) (Script, error) {
	var sc Script
	var evs []scriptEvent
	stops := 0
	for i, raw := range strings.Split(string(b), "\n") {
		n := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fs := strings.Fields(line)
		bad := func() error { return fmt.Errorf("strikecommander: 腳本第 %d 列格式不對：%q", n, line) }
		if len(fs) < 2 {
			return Script{}, bad()
		}
		s, err := strconv.ParseUint(fs[1], 10, 64)
		if err != nil {
			return Script{}, bad()
		}
		switch fs[0] {
		case "move", "down", "up":
			if len(fs) != 4 {
				return Script{}, bad()
			}
			x, ex := strconv.ParseUint(fs[2], 10, 16)
			y, ey := strconv.ParseUint(fs[3], 10, 16)
			if ex != nil || ey != nil {
				return Script{}, bad()
			}
			sc.Inputs = append(sc.Inputs, ScriptedInput{Steps: s, Kind: fs[0], X: int(x), Y: int(y)})
		case "shot":
			if len(fs) > 3 {
				return Script{}, bad()
			}
			sh := Shot{Steps: s}
			if len(fs) == 3 {
				sh.Name = fs[2]
			}
			sc.Shots = append(sc.Shots, sh)
		case "stop":
			if len(fs) != 2 {
				return Script{}, bad()
			}
			sc.Stop = s
			stops++
		default:
			return Script{}, fmt.Errorf("strikecommander: 腳本第 %d 列：不認得的指令 %q", n, fs[0])
		}
		evs = append(evs, scriptEvent{s, fs[0]})
	}
	if stops > 1 {
		return Script{}, fmt.Errorf("strikecommander: 腳本有 %d 個 stop，只能 1 個", stops)
	}
	if err := validateScript(evs, stops == 1, sc.Stop, start); err != nil {
		return Script{}, err
	}
	return sc, nil
}

// validateScript 依檔案順序驗證事件步數（002 §2.4）。輸入事件後要 Advance(1)，下一個事件再交付空更新、
// Advance 到它的步數，預算不得為 0，所以相鄰事件至少差 2 步。各條規則分開判定、各有訊息。
func validateScript(evs []scriptEvent, hasStop bool, stop, start uint64) error {
	if !hasStop {
		return fmt.Errorf("strikecommander: 腳本缺 stop")
	}
	for i, e := range evs {
		if i == 0 {
			if e.steps <= start {
				return fmt.Errorf("strikecommander: 腳本第一個事件第 %d 步不大於起始步數 %d", e.steps, start)
			}
			continue
		}
		p := evs[i-1].steps
		if e.steps < p {
			return fmt.Errorf("strikecommander: 腳本步數不遞增：%s 第 %d 步在第 %d 步之後", e.kind, e.steps, p)
		}
		if e.steps == p {
			return fmt.Errorf("strikecommander: 腳本第 %d 步有兩個事件", e.steps)
		}
		if e.steps == p+1 {
			return fmt.Errorf("strikecommander: 腳本 %s 第 %d 步與前一事件只差 1 步，至少要差 2 步", e.kind, e.steps)
		}
		if e.kind != "stop" && e.steps > stop {
			return fmt.Errorf("strikecommander: 腳本 %s 第 %d 步超過 stop 第 %d 步", e.kind, e.steps, stop)
		}
	}
	return nil
}

// receiptEvents 把 ReceiptConfig 的 Inputs、Shots 依步數合併（同步數保留 Inputs 在前），最後接 stop。
func receiptEvents(inputs []ScriptedInput, shots []Shot, stop uint64) ([]scriptEvent, []any) {
	var evs []scriptEvent
	var items []any
	i, j := 0, 0
	for i < len(inputs) || j < len(shots) {
		if j >= len(shots) || (i < len(inputs) && inputs[i].Steps <= shots[j].Steps) {
			evs, items = append(evs, scriptEvent{inputs[i].Steps, inputs[i].Kind}), append(items, inputs[i])
			i++
		} else {
			evs, items = append(evs, scriptEvent{shots[j].Steps, "shot"}), append(items, shots[j])
			j++
		}
	}
	return append(evs, scriptEvent{stop, "stop"}), items
}
