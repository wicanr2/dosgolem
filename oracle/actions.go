package oracle

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/dos"
)

// 逐步操作腳本（`docs/spec/201`）：讓代理（或測試腳本）像玩家一樣操作——
// 看一眼畫面、決定按什麼、按多久、等多久，再看下一眼。
//
// ParseActions 把逗號分隔的腳本解成一串原子動作；RunActions 依機器時間
// （不是牆上時間）把它們跑完，時間都用 199 的即時介面（RunCycles、
// KeyDown／KeyUp）接起來。

// ActionKind 是一個原子動作的種類。
type ActionKind int

const (
	// ActionTap 是「按下、跑 MS、放開」；MS 沒給時解析時填 defaultTapMS
	// （`tap:<鍵>[:<ms>]`）。
	ActionTap ActionKind = iota
	// ActionHold 執行起來與 ActionTap 完全相同（按下、跑 MS、放開），
	// 差別只在解析時 MS 是必填的（`hold:<鍵>:<ms>`，`docs/spec/201` §2.1）。
	ActionHold
	// ActionDown 只按下，不等、不放開（`down:<鍵>`）。
	ActionDown
	// ActionUp 只放開（`up:<鍵>`）。
	ActionUp
	// ActionWait 不碰按鍵，單純跑 MS（`wait:<ms>`）。
	ActionWait
)

// Action 是腳本解析出來的一個原子動作。
//
// Scan 只有 Tap／Hold／Down／Up 用得到；MS 只有 Tap／Hold／Wait 用得到。
type Action struct {
	Kind ActionKind
	Scan uint8
	MS   float64
}

// defaultTapMS 是 `tap:<鍵>` 沒給毫秒數、以及 `type:` 展開時的預設毫秒數
// （`docs/spec/201` §2.1）。
const defaultTapMS = 150.0

// namedKeyNames 鏡射 `internal/dos`（`scancode.go` 的 namedKeys）具名鍵表的
// 拼法，只用來做大小寫不敏感比對。
//
// ⚠ **這是特意維護的複本，不是偷懶。** `dos.KeyNamed` 本身區分大小寫，而
// 規格 201 §2.1 要求鍵名不分大小寫；但 201 的邊界只准動 `oracle/` 與
// `cmd/step/`，不能改 `internal/dos/scancode.go` 去加一個不分大小寫的查表版。
// 所以在這裡自己列一份候選拼法去試 `dos.KeyNamed`——查到的鍵一定是那張表
// 裡真的存在的鍵（用查到的正確拼法再查一次 `dos.KeyNamed`），不會因為這份
// 複本而多出一個「看起來合理但原表沒有」的鍵。`scancode.go` 加新鍵名時，
// 這份清單要跟著補。
var namedKeyNames = []string{
	"Return", "Enter", "Space", "Esc", "Escape", "Backspace", "Tab",
	"Up", "Down", "Left", "Right",
	"Home", "End", "PgUp", "PageUp", "PgDn", "PageDown",
	"Insert", "Ins", "Delete", "Del",
	"KP7", "KP8", "KP9", "KP4", "KP5", "KP6", "KP1", "KP2", "KP3", "KP0", "KPDot",
}

// lookupKey 依鍵名查掃描碼，不分大小寫；查不到具名鍵就退回單一可列印字元
// （`dos.KeyForRune`：字母、數字、空白）。查不到回 false——**不要猜一個看
// 起來合理的掃描碼**，那會讓「鍵名打錯」變成「原版行為不同」，同
// `dos.KeyNamed` 自己的準則。
func lookupKey(name string) (uint8, bool) {
	for _, n := range namedKeyNames {
		if strings.EqualFold(n, name) {
			if k, ok := dos.KeyNamed(n); ok {
				return k.Scan, true
			}
		}
	}
	if r := []rune(name); len(r) == 1 {
		if k, ok := dos.KeyForRune(r[0]); ok {
			return k.Scan, true
		}
	}
	return 0, false
}

// parseMS 解一個毫秒數；要求 > 0（`docs/spec/201` §2.1：「ms ≤ 0…整串拒絕」）。
func parseMS(s string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("毫秒數看不懂：%q", s)
	}
	if v <= 0 {
		return 0, fmt.Errorf("毫秒數要 > 0：%v", v)
	}
	return v, nil
}

// ParseActions 把逗號分隔的動作腳本解成一串原子動作（`docs/spec/201` §2.1）。
//
//	tap:<鍵>[:<ms>]   按下、跑 ms（預設 150）、放開
//	hold:<鍵>:<ms>    同 tap，ms 必填
//	down:<鍵>         只按下
//	up:<鍵>           只放開
//	wait:<ms>         不動，跑 ms
//	type:<文字>       每個字元 tap:<字元>:150 後 wait:150
//
// 認不得的鍵、ms ≤ 0、未知動作：**整串拒絕並回錯，不回傳任何一項**——
// 呼叫端只要檢查一次 error，不會拿到一份「跑到一半解析失敗」的清單。
func ParseActions(s string) ([]Action, error) {
	var out []Action
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		verb, rest, ok := strings.Cut(item, ":")
		if !ok {
			return nil, fmt.Errorf("動作 %q 看不懂，要寫成 <動作>:<...>", item)
		}
		switch verb {
		case "tap":
			key, msStr, hasMS := strings.Cut(rest, ":")
			scan, ok := lookupKey(strings.TrimSpace(key))
			if !ok {
				return nil, fmt.Errorf("動作 %q 看不懂的鍵：%q", item, key)
			}
			ms := defaultTapMS
			if hasMS {
				v, err := parseMS(msStr)
				if err != nil {
					return nil, fmt.Errorf("動作 %q：%w", item, err)
				}
				ms = v
			}
			out = append(out, Action{Kind: ActionTap, Scan: scan, MS: ms})

		case "hold":
			key, msStr, hasMS := strings.Cut(rest, ":")
			if !hasMS {
				return nil, fmt.Errorf("動作 %q 要寫成 hold:<鍵>:<ms>（ms 必填）", item)
			}
			scan, ok := lookupKey(strings.TrimSpace(key))
			if !ok {
				return nil, fmt.Errorf("動作 %q 看不懂的鍵：%q", item, key)
			}
			ms, err := parseMS(msStr)
			if err != nil {
				return nil, fmt.Errorf("動作 %q：%w", item, err)
			}
			out = append(out, Action{Kind: ActionHold, Scan: scan, MS: ms})

		case "down":
			scan, ok := lookupKey(strings.TrimSpace(rest))
			if !ok {
				return nil, fmt.Errorf("動作 %q 看不懂的鍵：%q", item, rest)
			}
			out = append(out, Action{Kind: ActionDown, Scan: scan})

		case "up":
			scan, ok := lookupKey(strings.TrimSpace(rest))
			if !ok {
				return nil, fmt.Errorf("動作 %q 看不懂的鍵：%q", item, rest)
			}
			out = append(out, Action{Kind: ActionUp, Scan: scan})

		case "wait":
			ms, err := parseMS(rest)
			if err != nil {
				return nil, fmt.Errorf("動作 %q：%w", item, err)
			}
			out = append(out, Action{Kind: ActionWait, MS: ms})

		case "type":
			for _, r := range rest {
				scan, ok := lookupKey(string(r))
				if !ok {
					return nil, fmt.Errorf("動作 %q 裡的字元 %q 沒有對應的鍵", item, string(r))
				}
				out = append(out, Action{Kind: ActionTap, Scan: scan, MS: defaultTapMS})
				out = append(out, Action{Kind: ActionWait, MS: defaultTapMS})
			}

		default:
			return nil, fmt.Errorf("看不懂的動作 %q", verb)
		}
	}
	return out, nil
}

// RunActions 依機器時間（不是牆上時間）把 acts 跑完（`docs/spec/201` §2.2）。
//
// 需要先 SetDOSBoxCycles；沒開就整個呼叫失敗、不執行任何一項（在動任何
// 按鍵之前就檢查，呼叫端不會落在跑一半的狀態）。
//
// 每個「跑 ms」的動作切成每段 chunkMs（≤ 0 用 1000/60，也就是一幀）機器
// 毫秒的 RunCycles，每段之後呼叫 onChunk（可為 nil）——前端每幀要做的事
// （例如疊字定色）放在這裡，時間粒度才跟得上前端的幀。
//
// 程式結束（*ExitError）照樣回傳，呼叫端自己決定要不要當成正常收尾。
func (o *Oracle) RunActions(acts []Action, chunkMs float64, onChunk func()) error {
	perMs := o.DOSBoxCycles()
	if perMs == 0 {
		return fmt.Errorf("RunActions 需要先 SetDOSBoxCycles：指令數時鐘沒有機器時間可以對齊")
	}
	if chunkMs <= 0 {
		chunkMs = 1000.0 / 60.0
	}
	chunkCycles := uint64(math.Round(chunkMs * float64(perMs)))
	if chunkCycles == 0 {
		chunkCycles = 1
	}
	// runMS 把 ms 換算成 cycles，一次算好總量再切段——**不要每段各自四捨五入**，
	// 那樣切得越細、累積誤差越大；只算一次總量，只有最後一段可能比整段短。
	runMS := func(ms float64) error {
		remain := uint64(math.Round(ms * float64(perMs)))
		for remain > 0 {
			n := chunkCycles
			if n > remain {
				n = remain
			}
			if err := o.RunCycles(n); err != nil {
				return err
			}
			remain -= n
			if onChunk != nil {
				onChunk()
			}
		}
		return nil
	}
	for _, a := range acts {
		switch a.Kind {
		case ActionTap, ActionHold:
			o.KeyDown(a.Scan)
			if err := runMS(a.MS); err != nil {
				return err
			}
			o.KeyUp(a.Scan)
		case ActionDown:
			o.KeyDown(a.Scan)
		case ActionUp:
			o.KeyUp(a.Scan)
		case ActionWait:
			if err := runMS(a.MS); err != nil {
				return err
			}
		default:
			return fmt.Errorf("RunActions：不認得的動作種類 %d", a.Kind)
		}
	}
	return nil
}
