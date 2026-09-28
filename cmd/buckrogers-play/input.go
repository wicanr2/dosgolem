package main

// 輸入對映（`docs/spec/241`）：把一個畫格的主機輸入轉成 BIOS 鍵與前端動作。
// 這一層不認識 ebiten：鍵以名稱表示，ebiten 的轉接在 main.go。

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
)

const (
	repeatDelay    = 30 // 按住幾格後開始連發
	repeatInterval = 6  // 之後每幾格一次（約 10 次／秒）
	sessionScale   = 2  // session 固定版面（InitialLayout）
	sessionChrome  = 36
	clickHold      = 6 // 腳本 click 按住的格數
)

// hostAction 是前端自己處理的動作，原版看不到。
type hostAction int

const (
	actHelp hostAction = iota + 1
	actScale
	actMute
	actFullscreen
	actShot
)

// hostKeysReserved 是前端保留的鍵（§3.3）。
var hostKeysReserved = map[string]hostAction{
	"F1": actHelp, "F2": actScale, "F3": actMute, "F11": actFullscreen, "F12": actShot,
}

// frameInput 是一個畫格的主機鍵盤狀態。Held 的值是按住的格數（剛按下為 1）。
type frameInput struct {
	Chars     []rune
	Held      map[string]int
	Ctrl, Alt bool
}

// fires 回按住 d 格的鍵這一格要不要送：剛按下，或連發時點。
func fires(d int) bool {
	if d == 1 {
		return true
	}
	held := d - 1
	return held >= repeatDelay && (held-repeatDelay)%repeatInterval == 0
}

func isNumpad(name string) bool { return strings.HasPrefix(name, "KP") }

func isLetter(name string) bool { return len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z' }

// mapKeys 把一個畫格的輸入轉成 BIOS 鍵與前端動作。helpOpen 時只回前端動作。
// logbook 回 true 表示 PgUp／PgDn 被手札面板消耗（Buck 規格 030）。
func mapKeys(in frameInput, helpOpen bool, logbook func(dir int) bool) ([]dos.Key, []hostAction) {
	var keys []dos.Key
	var acts []hostAction
	names := make([]string, 0, len(in.Held))
	for n := range in.Held {
		names = append(names, n)
	}
	sortStrings(names)
	numpadNow, combo := false, in.Ctrl || in.Alt
	for _, n := range names {
		if isNumpad(n) && in.Held[n] == 1 {
			numpadNow = true
		}
	}
	for _, n := range names {
		if a, ok := hostKeysReserved[n]; ok && in.Held[n] == 1 {
			acts = append(acts, a)
		}
	}
	if helpOpen {
		return nil, acts
	}
	if !combo {
		for _, r := range in.Chars {
			if numpadNow && (r >= '0' && r <= '9' || r == '.') {
				continue
			}
			if k, ok := dos.KeyForRune(r); ok {
				keys = append(keys, k)
			}
		}
	}
	for _, n := range names {
		d := in.Held[n]
		if _, reserved := hostKeysReserved[n]; reserved || !fires(d) {
			continue
		}
		if isLetter(n) {
			var k dos.Key
			var ok bool
			switch {
			case in.Ctrl:
				k, ok = dos.KeyCtrl(n[0])
			case in.Alt:
				k, ok = dos.KeyAlt(n[0])
			}
			if ok {
				keys = append(keys, k)
			}
			continue
		}
		if d == 1 && logbook != nil && (n == "PgDn" && logbook(1) || n == "PgUp" && logbook(-1)) {
			continue
		}
		if k, ok := dos.KeyNamed(n); ok {
			keys = append(keys, k)
		}
	}
	return keys, acts
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// floorDiv 是向負無限大取整的除法。
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// mouseEvent 把 DOS 座標 (dx, dy) 換成 session 版面的事件（§3.2）。
func mouseEvent(kind host.MouseEventKind, dx, dy int) host.MouseEvent {
	target := host.MouseTargetCanvas
	if dx < 0 || dx >= 320 || dy < 0 || dy >= 200 {
		target = host.MouseTargetOutside
	}
	return host.MouseEvent{Kind: kind, Button: 0, X: dx * sessionScale, Y: sessionChrome + dy*sessionScale, Target: target}
}

// dosPoint 把視窗邏輯座標換成 DOS 座標；scale 是目前顯示倍率。
func dosPoint(cx, cy, scale int) (int, int) { return floorDiv(cx, scale), floorDiv(cy, scale) }

// scriptAction 是自動模式腳本的一個動作。
type scriptAction struct {
	key   *dos.Key
	mouse host.MouseEventKind // 0：不是滑鼠
	x, y  int
	blur  bool
	help  bool
	scale bool // 前端 F2：切換 2 倍／3 倍
}

// parseScript 解析 `畫格:動作[,…]`（§3.4）。
func parseScript(s string) (map[int][]scriptAction, error) {
	out := map[int][]scriptAction{}
	if s == "" {
		return out, nil
	}
	type press struct{ down, up int }
	var presses []press
	for _, item := range strings.Split(s, ",") {
		frameText, name, ok := strings.Cut(item, ":")
		frame, err := strconv.Atoi(frameText)
		if !ok || err != nil || frame < 1 {
			return nil, fmt.Errorf("script 項目 %q 須為 畫格:動作", item)
		}
		switch {
		case name == "help":
			out[frame] = append(out[frame], scriptAction{help: true})
		case name == "scale":
			out[frame] = append(out[frame], scriptAction{scale: true})
		case name == "blur":
			out[frame] = append(out[frame], scriptAction{blur: true})
		case strings.HasPrefix(name, "click@"), strings.HasPrefix(name, "press@"), strings.HasPrefix(name, "release@"):
			verb, coords, _ := strings.Cut(name, "@")
			xs, ys, ok := strings.Cut(coords, ";")
			x, ex := strconv.Atoi(xs)
			y, ey := strconv.Atoi(ys)
			if !ok || ex != nil || ey != nil {
				return nil, fmt.Errorf("script 項目 %q 的座標須為 x;y", item)
			}
			if verb == "click" && (x < 0 || x >= 320 || y < 0 || y >= 200) {
				return nil, fmt.Errorf("script 項目 %q 的 click 須在畫布內", item)
			}
			if verb == "click" || verb == "press" {
				out[frame] = append(out[frame], scriptAction{mouse: host.MouseEventDown, x: x, y: y})
			}
			switch verb {
			case "click":
				out[frame+clickHold] = append(out[frame+clickHold], scriptAction{mouse: host.MouseEventUp, x: x, y: y})
				presses = append(presses, press{frame, frame + clickHold})
			case "release":
				out[frame] = append(out[frame], scriptAction{mouse: host.MouseEventUp, x: x, y: y})
			}
		default:
			key, found := dos.KeyNamed(name)
			if !found {
				if mod, letter, ok := strings.Cut(name, "+"); ok && len(letter) == 1 {
					switch mod {
					case "Ctrl":
						key, found = dos.KeyCtrl(letter[0])
					case "Alt":
						key, found = dos.KeyAlt(letter[0])
					}
				}
			}
			if !found && len([]rune(name)) == 1 {
				key, found = dos.KeyForRune([]rune(name)[0])
			}
			if !found {
				return nil, fmt.Errorf("script 鍵 %q 無 BIOS 對映", name)
			}
			k := key
			out[frame] = append(out[frame], scriptAction{key: &k})
		}
	}
	for i := range presses {
		for j := range presses {
			if i != j && presses[j].down >= presses[i].down && presses[j].down <= presses[i].up {
				return nil, fmt.Errorf("script：click 在前一個按下放開前重疊（第 %d 格）", presses[j].down)
			}
		}
	}
	return out, nil
}
