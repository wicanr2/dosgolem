package main

// 輸入對映：把一個畫格的主機輸入轉成 BIOS 鍵與前端動作。
// 規則沿用 cmd/buckrogers-play/input.go（docs/spec/241）；這一層不認識 ebiten。

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
)

const (
	repeatDelay    = 30 // 按住幾格後開始連發
	repeatInterval = 6  // 之後每幾格一次
	sessionScale   = 2  // session 固定版面（InitialLayout）
	sessionChrome  = 36
)

// hostAction 是前端自己處理的動作，原版看不到。
type hostAction int

const (
	actToggle       hostAction = iota + 1 // F12：中文／原版
	actMute                               // F3
	actShot                               // F10
	actFullscreen                         // F11
	actAnswers                            // F9：查驗題答案顯示（規格 006）
	actJournal                            // F5：手札翻閱模式（規格 007）
	actPageUp                             // 面板開啟時的 PgUp
	actPageDown                           // 面板開啟時的 PgDn
	actPrevEntry                          // 翻閱模式的 ←
	actNextEntry                          // 翻閱模式的 →
	actCloseJournal                       // 翻閱模式的 Esc
	actGuide                              // F6：攻略面板（規格 008）
	actMap                                // F7：地圖疊層（規格 009）
	actReveal                             // 地圖開啟時的 V：全圖／只看走過
	actScale                              // F2：2× ↔ 3×
)

// panelMode 是手札面板的狀態：開啟時部分鍵由前端吃掉（規格 007 §2.4、§2.5）。
type panelMode int

const (
	panelNone panelMode = iota
	panelAuto
	panelBrowse
)

var panelKeys = map[panelMode]map[string]hostAction{
	panelAuto:   {"PgUp": actPageUp, "PgDn": actPageDown},
	panelBrowse: {"PgUp": actPageUp, "PgDn": actPageDown, "Left": actPrevEntry, "Right": actNextEntry, "Escape": actCloseJournal, "V": actReveal},
}

var hostKeysReserved = map[string]hostAction{"F3": actMute, "F10": actShot, "F11": actFullscreen, "F12": actToggle, "F9": actAnswers, "F5": actJournal, "F6": actGuide, "F7": actMap, "F2": actScale}

// frameInput 是一個畫格的主機鍵盤狀態。Held 的值是按住的格數（剛按下為 1）。
type frameInput struct {
	Chars     []rune
	Held      map[string]int
	Ctrl, Alt bool
}

func fires(d int) bool {
	if d == 1 {
		return true
	}
	held := d - 1
	return held >= repeatDelay && (held-repeatDelay)%repeatInterval == 0
}

func isNumpad(name string) bool { return strings.HasPrefix(name, "KP") }
func isLetter(name string) bool { return len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z' }

// mapKeys 把一個畫格的輸入轉成 BIOS 鍵與前端動作。翻閱模式下其餘鍵都不送進遊戲。
func mapKeys(in frameInput, mode panelMode) ([]dos.Key, []hostAction) {
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
		if a, ok := hostKeysReserved[n]; ok && in.Held[n] == 1 {
			acts = append(acts, a)
		}
		if a, ok := panelKeys[mode][n]; ok && fires(in.Held[n]) {
			acts = append(acts, a)
		}
	}
	if mode == panelBrowse {
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
		_, taken := panelKeys[mode][n]
		if _, reserved := hostKeysReserved[n]; reserved || taken || !fires(d) {
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

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// mouseEvent 把 DOS 座標 (dx, dy) 換成 session 版面的事件。
func mouseEvent(kind host.MouseEventKind, dx, dy int) host.MouseEvent {
	target := host.MouseTargetCanvas
	if dx < 0 || dx >= 320 || dy < 0 || dy >= 200 {
		target = host.MouseTargetOutside
	}
	return host.MouseEvent{Kind: kind, Button: 0, X: dx * sessionScale, Y: sessionChrome + dy*sessionScale, Target: target}
}

func dosPoint(cx, cy, scale int) (int, int) { return floorDiv(cx, scale), floorDiv(cy, scale) }

// scriptAction 是自動模式腳本的一個動作：送一個 BIOS 鍵，或切換中文／原版。
type scriptAction struct {
	key    *dos.Key
	toggle bool
}

// parseScript 解析 `畫格:動作[,…]`。動作是鍵名（dos.KeyNamed，例 Enter、Down）、
// 單一可列印字元，或 toggle（等同 F12）。
func parseScript(s string) (map[int][]scriptAction, error) {
	out := map[int][]scriptAction{}
	if s == "" {
		return out, nil
	}
	for _, item := range strings.Split(s, ",") {
		frameText, name, ok := strings.Cut(item, ":")
		frame, err := strconv.Atoi(frameText)
		if !ok || err != nil || frame < 1 || name == "" {
			return nil, fmt.Errorf("script 項目 %q 須為 畫格:動作", item)
		}
		switch {
		case name == "toggle":
			out[frame] = append(out[frame], scriptAction{toggle: true})
		case len([]rune(name)) == 1:
			k, ok := dos.KeyForRune([]rune(name)[0])
			if !ok {
				return nil, fmt.Errorf("script 項目 %q 的字元送不出", item)
			}
			out[frame] = append(out[frame], scriptAction{key: &k})
		default:
			k, ok := dos.KeyNamed(name)
			if !ok {
				return nil, fmt.Errorf("script 項目 %q 的鍵名不認得", item)
			}
			out[frame] = append(out[frame], scriptAction{key: &k})
		}
	}
	return out, nil
}
