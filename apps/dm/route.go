package dm

import (
	"encoding/json"
	"fmt"
	"os"
)

// 巡檢路線（`docs/spec/198` §3.4）。路線由 remake 那一側產生
// （dungeon_master 專案的 `dmtool sweep route`），這一側只消費。

// RoutePoint 是隊伍的一個位置與面向（0＝北 1＝東 2＝南 3＝西）。
type RoutePoint struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Facing int `json:"facing"`
}

// RouteSave 是路線的起點存檔（§3.6）。
type RouteSave struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	// GameTime 是存檔裡的遊戲刻。按 OK 之後第一個刻邊界讀到的值要等於它，
	// 那是第 0 個檢查點。
	GameTime int `json:"game_time"`
}

// RouteStep 是路線的一步。
type RouteStep struct {
	// Action 是 `move`（省略）、`turn` 或 `press`。
	Action string `json:"action,omitempty"`
	// Dir 是**絕對**方向，只有 `move` 用。
	Dir string `json:"dir,omitempty"`
	// Ticks 是 remake 做完這一步之後推進了幾刻。原版側在刻的邊界照同樣的刻數推進，
	// **不照指令數等**：照指令數的話兩側每一步走的刻數不同，之後怪物的位置與
	// 事件的時間全部岔開。
	Ticks  int        `json:"ticks"`
	Expect RoutePoint `json:"expect"`
}

// Route 是整條路線。
type Route struct {
	Map         int         `json:"map"`
	Save        *RouteSave  `json:"save,omitempty"`
	Start       RoutePoint  `json:"start"`
	Steps       []RouteStep `json:"steps"`
	Checkpoints []int       `json:"checkpoints"`
}

// ReadRoute 讀一份路線並檢查它自洽。
func ReadRoute(path string) (Route, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Route{}, err
	}
	var r Route
	if err := json.Unmarshal(raw, &r); err != nil {
		return Route{}, fmt.Errorf("解析路線：%w", err)
	}
	if err := r.Validate(); err != nil {
		return Route{}, err
	}
	return r, nil
}

// Validate 檢查每一步都能換成一次點擊，而且刻數是正的。
//
// ⚠ **不認得的動作要拒絕**，不要當成 `move`。規格 198 §3.4 預留了 `teleport`，
// 這一支還沒實作它；靜靜地當成移動的話，那一步會點錯鈕，
// 而 `expect` 對不上的錯誤訊息會指向錯的地方。
func (r Route) Validate() error {
	if r.Save == nil {
		return fmt.Errorf("路線沒有 save：§3.6 定案從存檔出發，原版側無從核對起點")
	}
	if r.Start.Facing < 0 || r.Start.Facing > 3 {
		return fmt.Errorf("start 的面向 %d 不在 0–3", r.Start.Facing)
	}
	at := r.Start
	for i, s := range r.Steps {
		if s.Ticks <= 0 {
			return fmt.Errorf("第 %d 步的 ticks ＝ %d，要是正數", i+1, s.Ticks)
		}
		if _, _, err := s.Click(at.Facing); err != nil {
			return fmt.Errorf("第 %d 步：%w", i+1, err)
		}
		at = s.Expect
	}
	for _, k := range r.Checkpoints {
		if k < 0 || k > len(r.Steps) {
			return fmt.Errorf("檢查點 %d 不在 0–%d", k, len(r.Steps))
		}
	}
	return nil
}

// absDir 是絕對方向的字母（與 remake 的 `dirLetter` 同一張表）。
var absDir = map[string]int{"N": 0, "E": 1, "S": 2, "W": 3}

// relMove 是 `相對方向 ＝ (絕對 − 面向 + 4) % 4` 對到的鈕。
var relMove = [4]Move{0: Forward, 1: StrafeRight, 2: Backward, 3: StrafeLeft}

// Click 是這一步要點的座標。facing 是**這一步之前**的面向，取路線上一步的
// `expect`（第一步取 `start`）——兩側用同一個公式、從同一份路線算，
// 不讀執行時的狀態，岔開時錯誤才會落在 `expect` 那一關。
func (s RouteStep) Click(facing int) (x, y int, err error) {
	switch s.Action {
	case "", "move":
		d, ok := absDir[s.Dir]
		if !ok {
			return 0, 0, fmt.Errorf("move 的 dir ＝ %q，要 N／E／S／W", s.Dir)
		}
		x, y = relMove[(d-facing+4)%4].Hot()
		return x, y, nil
	case "turn":
		switch (s.Expect.Facing - facing + 4) % 4 {
		case 1:
			x, y = TurnRight.Hot()
		case 3:
			x, y = TurnLeft.Hot()
		default:
			// ⚠ 一步只轉一個直角（§3.4），轉兩次就是兩步。
			return 0, 0, fmt.Errorf("turn 從面向 %d 到 %d 不是一個直角", facing, s.Expect.Facing)
		}
		return x, y, nil
	case "press":
		return DoorButtonX, DoorButtonY, nil
	}
	return 0, 0, fmt.Errorf("不認得的動作 %q（要 move／turn／press）", s.Action)
}
