package player

import (
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/session"
	"sort"
	"strconv"
	"strings"
)

// Host key names preserve physical keys, including left/right modifiers and keypad.
var ScanCodes = map[string]uint8{
	"Escape": 1, "1": 2, "2": 3, "3": 4, "4": 5, "5": 6, "6": 7, "7": 8, "8": 9, "9": 10, "0": 11, "Minus": 12, "Equal": 13, "Backspace": 14, "Tab": 15,
	"Q": 16, "W": 17, "E": 18, "R": 19, "T": 20, "Y": 21, "U": 22, "I": 23, "O": 24, "P": 25, "BracketLeft": 26, "BracketRight": 27, "Enter": 28, "NumpadEnter": 28, "ControlLeft": 29, "ControlRight": 29,
	"A": 30, "S": 31, "D": 32, "F": 33, "G": 34, "H": 35, "J": 36, "K": 37, "L": 38, "Semicolon": 39, "Quote": 40, "Backquote": 41, "ShiftLeft": 42, "Backslash": 43,
	"Z": 44, "X": 45, "C": 46, "V": 47, "B": 48, "N": 49, "M": 50, "Comma": 51, "Period": 52, "Slash": 53, "NumpadDivide": 53, "ShiftRight": 54, "NumpadMultiply": 55, "AltLeft": 56, "AltRight": 56, "Space": 57, "CapsLock": 58,
	"F1": 59, "F2": 60, "F3": 61, "F4": 62, "F5": 63, "F6": 64, "F7": 65, "F8": 66, "F9": 67, "F10": 68, "NumLock": 69, "ScrollLock": 70,
	"Numpad7": 71, "Numpad8": 72, "Numpad9": 73, "NumpadSubtract": 74, "Numpad4": 75, "Numpad5": 76, "Numpad6": 77, "NumpadAdd": 78, "Numpad1": 79, "Numpad2": 80, "Numpad3": 81, "Numpad0": 82, "NumpadDecimal": 83,
	"ArrowUp": 72, "ArrowDown": 80, "ArrowLeft": 75, "ArrowRight": 77, "Home": 71, "End": 79, "PageUp": 73, "PageDown": 81, "Insert": 82, "Delete": 83, "IntlBackslash": 86,
}

type HostInput struct {
	Held           map[string]bool
	MouseX, MouseY int
	Left, Right    bool
	CapsToggle     bool
}
type InputContext struct {
	Suppress, MenuPeriod, MenuVisible, Resume, ForceFocusLost bool
	Mac                                                       bool
}
type InputState struct {
	sent            [128]bool
	suppressed      map[string]bool
	mouseSuppressed bool
	mouseSent       bool
	mouseKnown      bool
	mouseX, mouseY  int
	capsPhase       int // 0 idle; 1 make pending; 2 break pending; 3 break then next make.
}

func DOSPoint(x, y, scale int) (int, int) {
	floor := func(a int) int {
		q := a / scale
		if a < 0 && a%scale != 0 {
			q--
		}
		return q
	}
	return floor(x), floor(y)
}
func ClampMENU(x, y int) (int, int) {
	if x < 90 || x > 254 || y < 71 || y > 96 {
		return x, y
	}
	c := [][2]int{{x, 97}, {x, 70}, {89, y}, {255, y}}
	best := c[0]
	dist := func(v [2]int) int { dx := v[0] - x; dy := v[1] - y; return dx*dx + dy*dy }
	for _, p := range c[1:] {
		if dist(p) < dist(best) {
			best = p
		}
	}
	return best[0], best[1]
}
func mouse(kind host.MouseEventKind, x, y int) host.MouseEvent {
	t := host.MouseTargetCanvas
	if x < 0 || x >= 320 || y < 0 || y >= 200 {
		t = host.MouseTargetOutside
	}
	return host.MouseEvent{Kind: kind, Button: 0, X: x * 2, Y: 36 + y*2, Target: t}
}

// Observe is called on every host Update, including n=0 and paused updates.
// Build computes a proposed commit; Commit is called only after Deliver succeeds.
func (s *InputState) Observe(in HostInput, c InputContext) {
	if s.suppressed == nil {
		s.suppressed = map[string]bool{}
	}
	capsWasSuppressed := s.suppressed["CapsLock"]
	for k := range s.suppressed {
		if !in.Held[k] {
			delete(s.suppressed, k)
		}
	}
	for k, held := range in.Held {
		if held && (c.Suppress || c.MenuVisible || k == "F11" || k == "F12" || c.Mac && (k == "MetaLeft" || k == "MetaRight" || (in.Held["MetaLeft"] || in.Held["MetaRight"]) && k == "Slash")) {
			s.suppressed[k] = true
		}
	}
	if c.MenuVisible && (in.Left || in.Right) && (in.MouseX < 89 || in.MouseX > 254 || in.MouseY < 33 || in.MouseY > 58) && !s.mouseSent {
		s.mouseSuppressed = true
	}
	if c.Resume && (in.Left || in.Right) {
		s.mouseSuppressed = true
	}
	if !(in.Left || in.Right) {
		s.mouseSuppressed = false
	}
	if c.Suppress && (in.Left || in.Right) {
		s.mouseSuppressed = true
	}
	if c.ForceFocusLost && (in.Left || in.Right) {
		s.mouseSuppressed = true
	}
	if c.Mac {
		blocked := c.Suppress || c.MenuVisible || c.ForceFocusLost || capsWasSuppressed || s.suppressed["CapsLock"]
		if blocked {
			// Cancel an unsent make, but preserve a sent key's pending break.
			// A queued follow-up toggle must not cross the suppression interval.
			if s.sent[58] {
				s.capsPhase = 2
			} else {
				s.capsPhase = 0
			}
		} else if in.CapsToggle {
			if s.sent[58] {
				s.capsPhase = 3
			} else {
				s.capsPhase = 1
			}
		}
	}
}

type InputCommit struct {
	Update     session.CapturedUpdate
	target     [128]bool
	mouseSent  bool
	mouseKnown bool
	x, y       int
	capsPhase  int
}

func (s *InputState) Build(in HostInput, c InputContext) InputCommit {
	r := InputCommit{mouseSent: s.mouseSent, mouseKnown: s.mouseKnown, x: s.mouseX, y: s.mouseY, capsPhase: s.capsPhase}
	u := &r.Update
	if !c.Suppress && !c.MenuVisible {
		for k, h := range in.Held {
			if h && !s.suppressed[k] {
				if code, ok := scanCode(k); ok {
					r.target[code] = true
				}
			}
		}
		if c.Mac && !s.suppressed["CapsLock"] {
			r.target[58] = s.capsPhase == 1
			if s.capsPhase == 1 {
				r.capsPhase = 2
			} else if s.capsPhase == 2 {
				r.capsPhase = 0
			} else if s.capsPhase == 3 {
				r.capsPhase = 1
			}
		}
	}
	if c.Mac && (c.Suppress || c.MenuVisible || s.suppressed["CapsLock"]) {
		// This advancing commit sends the break for any previously sent make.
		r.target[58] = false
		r.capsPhase = 0
	}
	u.FocusLost = c.ForceFocusLost
	for code := 1; code < 128; code++ {
		if s.sent[code] && !r.target[code] {
			u.ScanEvents = append(u.ScanEvents, session.ScanEvent{Scan: uint8(code), Break: true})
		}
	}
	if !u.FocusLost {
		for code := 1; code < 128; code++ {
			if !s.sent[code] && r.target[code] {
				u.ScanEvents = append(u.ScanEvents, session.ScanEvent{Scan: uint8(code)})
			}
		}
	} else {
		r.target = [128]bool{}
		r.mouseSent = false
		r.capsPhase = 0
	}
	x, y := in.MouseX, in.MouseY
	if c.MenuPeriod {
		x, y = ClampMENU(x, y)
	}
	if !r.mouseKnown || x != r.x || y != r.y {
		u.PointerMoves = []host.MouseEvent{mouse(host.MouseEventMove, x, y)}
		r.mouseKnown = true
		r.x = x
		r.y = y
	}
	want := (in.Left || in.Right) && !s.mouseSuppressed && !c.Suppress && !c.ForceFocusLost
	allowed := !c.MenuVisible || (x >= 89 && x <= 254 && y >= 33 && y <= 58)
	if want && !s.mouseSent && allowed {
		v := mouse(host.MouseEventDown, x, y)
		u.PointerDown = &v
		r.mouseSent = true
	}
	if !want && s.mouseSent && !c.ForceFocusLost {
		v := mouse(host.MouseEventUp, x, y)
		u.PointerUp = &v
		r.mouseSent = false
	}
	return r
}
func (s *InputState) Commit(r InputCommit) {
	s.sent = r.target
	s.mouseSent = r.mouseSent
	s.mouseKnown = r.mouseKnown
	s.mouseX = r.x
	s.mouseY = r.y
	s.capsPhase = r.capsPhase
}
func (s *InputState) HeldCodes() []uint8 {
	var a []uint8
	for n, b := range s.sent {
		if b {
			a = append(a, uint8(n))
		}
	}
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	return a
}

func scanCode(k string) (uint8, bool) {
	if code, ok := ScanCodes[k]; ok {
		return code, true
	}
	if strings.HasPrefix(k, "@scan:") {
		v, e := strconv.ParseUint(strings.TrimPrefix(k, "@scan:"), 16, 8)
		if e == nil && v > 0 && v <= 127 && v != 0x57 && v != 0x58 {
			return uint8(v), true
		}
	}
	return 0, false
}
func ScriptHostName(code uint8) string {
	if code == 0x57 {
		return "F11"
	}
	if code == 0x58 {
		return "F12"
	}
	return "@scan:" + strconv.FormatUint(uint64(code), 16)
}
