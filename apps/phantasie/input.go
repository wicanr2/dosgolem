package phantasie

import (
	"github.com/wicanr2/dosgolem/oracle"
)

// OffReadKey 是讀鍵包裝函式的入口（映像偏移）：它先做一次 `INT 16h AH=00`，
// 返回後 AL 是 ASCII、AH 是掃描碼。每個等待按鍵的畫面都經過它。
const OffReadKey = 0x37C2

// KeyGate 讓按鍵在「原版即將讀鍵」的那一刻才放進 BIOS 鍵盤佇列。
//
// 原因：原版每次讀鍵前會先把鍵盤緩衝清空（`INT 16h AH=01`／`AH=00` 迴圈，
// 映像偏移 3933h），而 dosgolem 的 `INT 16h AH=00` 在佇列空時不阻塞、直接回 0。
// 外部在任意步數放進佇列的鍵因此會被清空動作吃掉。真機上按鍵發生在程式阻塞等待的期間，
// 在清空之後；KeyGate 以讀鍵入口當「阻塞等待」的起點，等價地重現這件事。
//
// 只讀原版狀態；放進佇列的是與真實鍵盤相同的 BIOS 按鍵。
type KeyGate struct {
	o       *oracle.Oracle
	pending []gateKey
	Gated   int // 已放進佇列的鍵數
}

type gateKey struct {
	after uint64 // 不早於這個步數
	name  string
}

// NewKeyGate 在映像段 img 的讀鍵入口掛 hook。
func NewKeyGate(o *oracle.Oracle, img uint16) *KeyGate {
	g := &KeyGate{o: o}
	o.OnCall(oracle.Far(img, OffReadKey), func(o *oracle.Oracle) { g.fire() })
	return g
}

// Press 排入一個按鍵（名稱見 oracle.SendKeys）。依呼叫順序送出，每次讀鍵入口最多送一個，
// 且不早於 after 步。
func (g *KeyGate) Press(after uint64, name string) {
	g.pending = append(g.pending, gateKey{after, name})
}

// Pending 回還沒送出的鍵數。
func (g *KeyGate) Pending() int { return len(g.pending) }

func (g *KeyGate) fire() {
	if len(g.pending) == 0 || g.o.Steps() < g.pending[0].after {
		return
	}
	k := g.pending[0]
	g.pending = g.pending[1:]
	if err := g.o.SendKeys(k.name); err == nil {
		g.Gated++
		return
	}
	// 有名字的鍵之外，單一可列印字元（字母、數字）走 TypeKeys。
	if r := []rune(k.name); len(r) == 1 {
		if err := g.o.TypeKeys(k.name); err == nil {
			g.Gated++
		}
	}
}
