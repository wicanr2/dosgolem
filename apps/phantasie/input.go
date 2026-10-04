package phantasie

import (
	"bytes"
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// OffReadKey 是讀鍵包裝函式的入口（映像偏移）：它先做一次 `INT 16h AH=00`，
// 返回後 AL 是 ASCII、AH 是掃描碼。丟棄回傳值的 37A0 等鍵另由 guard 處理。
const OffReadKey = 0x37C2

// OffReadKeyDone 是讀鍵包裝函式的 retn（映像偏移 37FA，以執行期位元組核對：
// 入口 55 8B EC，結尾 5D C3）。KeyGate 用它關閉「開啟旗標」做重複觸發去重。
const OffReadKeyDone = 0x37FA

// 第二條等鍵在清空佇列後的 call 前暫停，見專案規格 008。
const (
	OffWaitKey     = 0x37A0
	OffWaitKeyCall = 0x37B8
	OffWaitKeyDone = 0x37C1
)

var waitKeySignature = []byte{
	0x55, 0x8B, 0xEC, 0xE8, 0x8D, 0x01, 0xC7, 0x06, 0xCE, 0xB8, 0x00, 0x00,
	0xB8, 0xDE, 0xB8, 0x50, 0xB8, 0xCE, 0xB8, 0x50, 0xB8, 0x16, 0x00, 0x50,
	0xE8, 0xA5, 0x16, 0x83, 0xC4, 0x06, 0x8B, 0xE5, 0x5D, 0xC3,
}

// KeyGate 讓按鍵在「原版即將讀鍵」的那一刻才放進 BIOS 鍵盤佇列。
//
// 原因：原版每次讀鍵前會先把鍵盤緩衝清空（`INT 16h AH=01`／`AH=00` 迴圈，
// 映像偏移 3933h），而 dosgolem 的 `INT 16h AH=00` 在佇列空時不阻塞、直接回 0。
// 外部在任意步數放進佇列的鍵因此會被清空動作吃掉。真機上按鍵發生在程式阻塞等待的期間，
// 在清空之後；KeyGate 以讀鍵入口當「阻塞等待」的起點，等價地重現這件事。
//
// 時脈中斷（IRQ0）在 Step 內送出，處理常式 IRET 回到 hook 位址時，同一個入口會觸發兩次
// （docs/spec/001 §3.2）。入口設開啟旗標並記 SP，完成（retn）時關閉：開啟期間 SP 相同的入口是重複觸發，
// 不計入讀鍵次數也不送鍵（Dups）。
//
// 只讀原版狀態；放進佇列的是與真實鍵盤相同的 BIOS 按鍵。
type KeyGate struct {
	o        *oracle.Oracle
	img      uint16
	pending  []gateKey
	Gated    int    // 已放進佇列的鍵數
	Reads    uint64 // 讀鍵入口累計次數（去重後）
	LastSent uint64 // 最後一次送出按鍵時的 Reads（路線的 @check 以此判斷「下一次讀鍵入口」）
	Dups     int    // 被去重忽略的重複入口與完成
	Err      string // 讀鍵函式簽章不符時的診斷（此時不去重）

	checked  bool
	dedupe   bool
	open     bool
	openSP   uint16
	waitOpen bool
	waitSP   uint16
	waitSeen bool
	waitSent bool
}

type gateKey struct {
	after uint64 // 不早於這個步數
	skip  uint64 // 成為佇列頭之後，還要先放過幾次讀鍵入口
	name  string
}

// NewKeyGate 在映像段 img 的讀鍵入口與結尾掛 hook。
func NewKeyGate(o *oracle.Oracle, img uint16) *KeyGate {
	g := &KeyGate{o: o, img: img}
	o.OnCall(oracle.Far(img, OffReadKey), func(o *oracle.Oracle) { g.enter(o.SP()) })
	o.OnCall(oracle.Far(img, OffReadKeyDone), func(o *oracle.Oracle) { g.leave() })
	o.OnCall(oracle.Far(img, OffWaitKeyDone), func(*oracle.Oracle) { g.waitOpen = false })
	o.SetStepGuard(g.guard)
	return g
}

func (g *KeyGate) guard(o *oracle.Oracle) error {
	ip := o.IP().Linear()
	if ip == oracle.Far(g.img, OffWaitKey).Linear() {
		if got := o.Bytes(oracle.Far(g.img, OffWaitKey), len(waitKeySignature)); !bytes.Equal(got, waitKeySignature) {
			g.Err = fmt.Sprintf("確認等鍵函式簽章不符：% X", got)
			return fmt.Errorf("%s", g.Err)
		}
		if !g.waitOpen || g.waitSP != o.SP() {
			g.waitOpen, g.waitSP, g.waitSeen, g.waitSent = true, o.SP(), false, false
		}
	}
	if ip != oracle.Far(g.img, OffWaitKeyCall).Linear() {
		return nil
	}
	if !g.waitOpen || o.SP() != g.waitSP-uint16(8) {
		return fmt.Errorf("確認等鍵堆疊不符：開啟=%t SP=%04X 入口=%04X", g.waitOpen, o.SP(), g.waitSP)
	}
	if !g.waitSeen {
		g.waitSeen = true
		g.Reads++
	}
	if g.waitSent {
		return nil
	}
	if len(g.pending) == 0 {
		return &oracle.InputWaitError{Stopped: o.IP()}
	}
	k := g.pending[0]
	if o.Steps() < k.after || k.skip != 0 {
		return fmt.Errorf("確認等鍵不能略過或等待未來指令：after=%d skip=%d 目前=%d", k.after, k.skip, o.Steps())
	}
	if err := sendGateKey(o, k.name); err != nil {
		return err
	}
	g.pending = g.pending[1:]
	g.Gated++
	g.LastSent = g.Reads
	g.waitSent = true
	return nil
}

func sendGateKey(o *oracle.Oracle, name string) error {
	if err := o.SendKeys(name); err == nil {
		return nil
	}
	if len([]rune(name)) == 1 {
		return o.TypeKeys(name)
	}
	return fmt.Errorf("不認得確認按鍵 %q", name)
}

// Press 排入一個按鍵（名稱見 oracle.SendKeys）。依呼叫順序送出，每次讀鍵入口最多送一個，
// 且不早於 after 步。
func (g *KeyGate) Press(after uint64, name string) {
	g.pending = append(g.pending, gateKey{after: after, name: name})
}

// PressAfterReads 排入一個按鍵：成為佇列頭之後，先放過 n 次讀鍵入口（去重後的次數）才送出
// （路線檔 `@wait N`，docs/spec/005 §4）。
func (g *KeyGate) PressAfterReads(n uint64, name string) {
	g.pending = append(g.pending, gateKey{skip: n, name: name})
}

// Pending 回還沒送出的鍵數。
func (g *KeyGate) Pending() int { return len(g.pending) }

// verify 在第一次入口核對簽章：入口 55 8B EC、結尾 C3。不符則不去重（避免開啟旗標永遠不關而吃掉所有鍵）。
func (g *KeyGate) verify() {
	g.checked = true
	in := g.o.Bytes(oracle.Far(g.img, OffReadKey), 3)
	out := g.o.Bytes(oracle.Far(g.img, OffReadKeyDone), 1)
	if !bytes.Equal(in, []byte{0x55, 0x8B, 0xEC}) || !bytes.Equal(out, []byte{0xC3}) {
		g.Err = fmt.Sprintf("讀鍵函式簽章不符：入口 % X、結尾 % X", in, out)
		return
	}
	g.dedupe = true
}

func (g *KeyGate) enter(sp uint16) {
	if !g.checked {
		g.verify()
	}
	if g.dedupe {
		if g.open && g.openSP == sp {
			g.Dups++
			return
		}
		g.open, g.openSP = true, sp
	}
	g.Reads++
	g.fire()
}

func (g *KeyGate) leave() {
	if !g.dedupe {
		return
	}
	if !g.open {
		g.Dups++
		return
	}
	g.open = false
}

func (g *KeyGate) fire() {
	if len(g.pending) == 0 || g.o.Steps() < g.pending[0].after {
		return
	}
	if g.pending[0].skip > 0 {
		g.pending[0].skip--
		return
	}
	k := g.pending[0]
	g.pending = g.pending[1:]
	if err := g.o.SendKeys(k.name); err == nil {
		g.Gated++
		g.LastSent = g.Reads
		return
	}
	// 有名字的鍵之外，單一可列印字元（字母、數字）走 TypeKeys。
	if r := []rune(k.name); len(r) == 1 {
		if err := g.o.TypeKeys(k.name); err == nil {
			g.Gated++
			g.LastSent = g.Reads
		}
	}
}
