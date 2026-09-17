// Command shots 跑一段鍵序，每一步存一張畫面：色號陣列（對拍用）與
// PNG（給人看）。給「remake 的畫面與原版對不對得上」這類問題用。
//
// **走位靠程式自己的兩個訊號**——「讀走了幾個鍵」與「畫面連續多少道指令
// 沒動」——不靠 sleep。這是 dosgolem 存在的理由：外面那一套只能拿牆上的
// 時鐘猜，而猜錯與猜對在畫面上長得一樣（`docs/spec/005`）。
//
// 這支是通用的，不含任何一支程式的位址；鍵序由 `-keys` 給。
//
//	tools/go.sh run ./cmd/shots -exe /orig/start.exe -root /orig \
//	  -out /src/workplace/shots -keys "rep:9:Space,Return,Return,c"
//
// 色號輸出（`.idx`，320×200 一格一個位元組）才是對拍的依據：PNG 經過
// 調色盤，而調色盤可能在動（`docs/spec/005` §3.3）。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

type shotInfo struct {
	Index   int    `json:"index"`
	Label   string `json:"label"`
	Step    uint64 `json:"step"`
	NonZero int    `json:"non_zero"`
	Digest  string `json:"sha256"`
	Path    string `json:"path"`
	// Peek 是 -peek 讀到的記憶體：鍵是 `ds:5E85` 這種標籤，值是 hex 字串。
	Peek map[string]string `json:"peek,omitempty"`
	// PeekTrace 是 -trace-peek 在這一步跑的期間每次值變動的快照（含步數）。
	// 一個鍵之後可能發生好幾件事（戰鬥裡幾隻怪物依序行動），只看靜下來那一幀
	// 分不出順序；這裡每跑一小段就讀一次，變了就記。
	PeekTrace []peekSnapshot `json:"peek_trace,omitempty"`
	// Calls 是 -trace-call 在這一步跑的期間記到的每一次呼叫：進去時堆疊上的第一個
	// word 參數、回來時的 AX、是誰叫的。Turbo Pascal 的 `Random(n)` 就是這個形狀。
	Calls []callRecord `json:"calls,omitempty"`
	// Damage 是 -intercept-damage 在這一步跑的期間攔到的每一次扣血呼叫。
	Damage []damageRecord `json:"damage,omitempty"`
}

// damageRecord 是一次被攔到的扣血呼叫：目標記錄的位址、陣營、當下 HP、原本與改過的傷害。
type damageRecord struct {
	Step     uint64 `json:"step"`
	Target   string `json:"target"`
	Side     uint8  `json:"side"`
	InCombat bool   `json:"in_combat"`
	HP       uint8  `json:"hp"`
	Damage   uint8  `json:"damage"`
	Changed  uint8  `json:"changed"`
	CallerCS uint16 `json:"caller_cs"`
	CallerIP uint16 `json:"caller_ip"`
}

// damageHook 是 -intercept-damage 的設定。對象是「遠呼叫、Pascal 慣例、參數依序推
// (目標 far pointer, 傷害 byte)」的扣血入口；位址與記錄欄位全部由呼叫端給，這裡不認得任何遊戲。
type damageHook struct {
	seg, off       uint16 // 連結期的段（執行期加 LoadSeg）與 offset
	hpField        uint32 // 目標記錄裡「目前 HP」那一個 byte 的位移
	sideField      uint32 // 目標記錄裡陣營那一個 byte 的位移
	combatAt       uint32 // DS 相對位址：這一格等於 combatValue 時才算在戰鬥中
	combatValue    uint8
	partySide      uint8 // 戰鬥中陣營等於它的是我方；不在戰鬥中一律當我方
	lock, kill     bool
	partySideKnown bool
}

// parseDamageHook 讀 `stub=010A:00AC,hp=11B,side=10E,combat=4954:5,party=0,lock,kill`（數字都是十六進位）。
// 沒給 party 時只記錄、不改傷害——先量陣營值，再開 lock／kill。
func parseDamageHook(spec string) (*damageHook, error) {
	h := &damageHook{}
	for _, part := range strings.Split(spec, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		hex := func(text string, bits int) (uint64, error) { return strconv.ParseUint(text, 16, bits) }
		switch key {
		case "stub":
			f := strings.Split(value, ":")
			if len(f) != 2 {
				return nil, fmt.Errorf("stub 要寫成 seg:off：%q", value)
			}
			seg, err1 := hex(f[0], 16)
			off, err2 := hex(f[1], 16)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("stub 看不懂：%q", value)
			}
			h.seg, h.off = uint16(seg), uint16(off)
		case "hp", "side":
			v, err := hex(value, 16)
			if err != nil {
				return nil, fmt.Errorf("%s 看不懂：%q", key, value)
			}
			if key == "hp" {
				h.hpField = uint32(v)
			} else {
				h.sideField = uint32(v)
			}
		case "combat":
			f := strings.Split(value, ":")
			if len(f) != 2 {
				return nil, fmt.Errorf("combat 要寫成 ds位址:值：%q", value)
			}
			at, err1 := hex(f[0], 16)
			v, err2 := hex(f[1], 8)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("combat 看不懂：%q", value)
			}
			h.combatAt, h.combatValue = uint32(at), uint8(v)
		case "party":
			v, err := hex(value, 8)
			if err != nil {
				return nil, fmt.Errorf("party 看不懂：%q", value)
			}
			h.partySide, h.partySideKnown = uint8(v), true
		case "lock":
			h.lock = true
		case "kill":
			h.kill = true
		default:
			return nil, fmt.Errorf("-intercept-damage 不認得 %q", part)
		}
	}
	if h.seg == 0 && h.off == 0 {
		return nil, fmt.Errorf("-intercept-damage 缺 stub")
	}
	if (h.lock || h.kill) && !h.partySideKnown {
		return nil, fmt.Errorf("開 lock／kill 之前要先給 party（我方的陣營值）")
	}
	return h, nil
}

// decide 回傳改過的傷害：我方且 lock → 0；敵方且 kill 且傷害大於 0 → 目標目前 HP。
func (h *damageHook) decide(party bool, hp, damage uint8) uint8 {
	switch {
	case party && h.lock:
		return 0
	case !party && h.kill && damage > 0 && hp > damage:
		return hp
	}
	return damage
}

type callRecord struct {
	Step     uint64 `json:"step"`
	Arg      uint16 `json:"arg"`
	Result   uint16 `json:"result"`
	CallerCS uint16 `json:"caller_cs"`
	CallerIP uint16 `json:"caller_ip"`
}

type peekSnapshot struct {
	Step uint64            `json:"step"`
	Peek map[string]string `json:"peek"`
}

func main() {
	exe := flag.String("exe", "", "MZ 執行檔")
	root := flag.String("root", ".", "素材目錄（唯讀）")
	scratch := flag.String("scratch", "", "可寫的暫存層；程式存檔會落在這裡（docs/spec/009）")
	budget := flag.Uint64("budget", 60_000_000, "每一步最多跑幾道指令")
	idle := flag.Uint64("idle", 3_000_000, "畫面連續這麼多道指令沒動就算穩定")
	keytrace := flag.Bool("keytrace", false, "每一步印出程式讀走了哪些鍵、是誰讀的")
	out := flag.String("out", "", "輸出目錄")
	script := flag.String("keys", "", "鍵序，逗號分隔：Return、Space、c、type:HERO、rep:18:Space")
	peek := flag.String("peek", "", "每一步畫面靜下來之後讀這些記憶體，逗號分隔："+
		"`ds:<hex 偏移>:<長度>`（段取當下的 DS——Turbo Pascal 程式的 DS 固定是 DGROUP，"+
		"等鍵時停在 RTL 裡也還是它）或 `<seg>:<off>:<長度>`。"+
		"結果印在 stdout，也寫進 shots.json 的 peek 欄（hex 字串）。"+
		"**要帶一個已知值當正對照**（例如程式自己的常數表），否則 DS 抓錯時讀到的"+
		"是別段的零，看起來跟「表是空的」一樣")
	interceptDamage := flag.String("intercept-damage", "", "扣血入口的攔截："+
		"`stub=<seg>:<off>,hp=<位移>,side=<位移>,combat=<ds位址>:<值>,party=<我方陣營>,lock,kill`（十六進位）。"+
		"在遠呼叫進入 stub 的那一刻讀堆疊上的 (目標 far pointer, 傷害 byte)：lock 把我方的傷害改成 0，"+
		"kill 把敵方的傷害改成目標目前 HP。每一次都記進 shots.json 的 damage；沒給 party 時只記不改")
	traceCall := flag.String("trace-call", "", "`<seg>:<off>`（十六進位，seg 是連結期的段，執行期加 LoadSeg）："+
		"每次 CS:IP 走到這裡就記堆疊上的第一個 word 參數與回來時的 AX，連同呼叫端 CS:IP 寫進 shots.json 的 calls。"+
		"拿它記 Turbo Pascal `Random(n)` 的骰流：`5BB:C94`（Pool of Radiance）")
	tracePeek := flag.String("trace-peek", "", "跟 -peek 同格式；每跑 10 萬道指令讀一次，值變了就記一筆快照進 shots.json 的 peek_trace（看戰鬥裡誰先動、動到哪）")
	loadState := flag.String("load-state", "", "從狀態檔接著跑（`internal/state`，probe 的 -save-state 存的那種）；"+
		"這時不再等開機那一幀，第一個鍵直接送。逐步驅動（看畫面再決定下一個鍵）靠它：每一步從上一步"+
		"存的狀態展開，是秒級不是分鐘級")
	saveState := flag.String("save-state", "", "全部鍵送完、畫面靜下來之後把機器與 DOS 存到這個檔")
	watch := flag.String("watch", "", "`<線性hex>-<線性hex>`：監看這段位址的寫入（值變了才記），"+
		"每一筆印出步數、位址、舊值→新值與寫入當下的 CS:IP（IP 已推過那條指令）")
	serve := flag.Bool("serve", false, "鍵序送完之後不結束，改從 stdin 一行一道命令（見 serve.go）；"+
		"讓外面的駕駛程式看記憶體再決定下一個鍵，不必每一步重開一次程式")
	flag.Parse()

	img, err := os.ReadFile(*exe)
	if err != nil {
		fmt.Println("讀不到執行檔：", err)
		os.Exit(1)
	}
	m := machine.New()
	if err := m.LoadEXE(img); err != nil {
		fmt.Println("載入失敗：", err)
		os.Exit(1)
	}
	d := dos.New(m, *root)
	d.Scratch = *scratch
	d.Install()

	calls := map[string]int{}
	inner := m.CPU.IntHook
	m.CPU.IntHook = func(c *cpu.CPU, n uint8) bool {
		calls[fmt.Sprintf("int %02Xh AH=%02X", n, c.R[cpu.AX]>>8)]++
		return inner(c, n)
	}

	if *out != "" {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	var shots []shotInfo
	var trace []peekSnapshot
	lastTrace := ""
	var traced []callRecord
	callSeg, callOff, callOn := uint16(0), uint16(0), false
	if *traceCall != "" {
		f := strings.Split(*traceCall, ":")
		seg, err1 := strconv.ParseUint(f[0], 16, 16)
		off, err2 := strconv.ParseUint(f[1], 16, 16)
		if len(f) != 2 || err1 != nil || err2 != nil {
			fmt.Println("-trace-call 看不懂：", *traceCall)
			os.Exit(2)
		}
		callSeg, callOff, callOn = uint16(seg)+machine.LoadSeg, uint16(off), true
	}
	var hook *damageHook
	var damaged []damageRecord
	if *interceptDamage != "" {
		var err error
		if hook, err = parseDamageHook(*interceptDamage); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	}
	intercept := func() {
		c := m.CPU
		if c.Seg[cpu.CS] != hook.seg+machine.LoadSeg || c.IP != hook.off {
			return
		}
		ss, sp := uint32(c.Seg[cpu.SS])*16, uint32(c.R[cpu.SP])
		retIP, retCS := m.Read16(ss+sp), m.Read16(ss+sp+2)
		damage := m.Read8(ss + sp + 4)
		toff, tseg := m.Read16(ss+sp+6), m.Read16(ss+sp+8)
		record := uint32(tseg)*16 + uint32(toff)
		inCombat := m.Read8(uint32(c.Seg[cpu.DS])*16+hook.combatAt) == hook.combatValue
		side := m.Read8(record + hook.sideField)
		hp := m.Read8(record + hook.hpField)
		party := !inCombat || side == hook.partySide
		changed := damage
		if hook.partySideKnown {
			changed = hook.decide(party, hp, damage)
		}
		if changed != damage {
			m.Write8(ss+sp+4, changed)
		}
		damaged = append(damaged, damageRecord{Step: m.Steps, Target: fmt.Sprintf("%04X:%04X", tseg, toff),
			Side: side, InCombat: inCombat, HP: hp, Damage: damage, Changed: changed, CallerCS: retCS, CallerIP: retIP})
		fmt.Printf("[damage] #%d %04X:%04X side=%d combat=%t hp=%d damage=%d→%d ← %04X:%04X\n",
			m.Steps, tseg, toff, side, inCombat, hp, damage, changed, retCS, retIP)
	}
	// pending 是進去了還沒回來的那一次（`Random` 是葉子，不會巢狀）。
	var pending *callRecord
	retCS, retIP := uint16(0), uint16(0)
	watchCall := func() {
		c := m.CPU
		if pending != nil {
			if c.Seg[cpu.CS] == retCS && c.IP == retIP {
				pending.Result = c.R[cpu.AX]
				traced = append(traced, *pending)
				pending = nil
			}
			return
		}
		if c.Seg[cpu.CS] == callSeg && c.IP == callOff {
			ss, sp := uint32(c.Seg[cpu.SS])*16, uint32(c.R[cpu.SP])
			retIP, retCS = m.Read16(ss+sp), m.Read16(ss+sp+2)
			pending = &callRecord{Step: m.Steps, Arg: m.Read16(ss + sp + 4), CallerCS: retCS, CallerIP: retIP}
		}
	}
	traceKey := func(p map[string]string) string {
		keys := make([]string, 0, len(p))
		for k := range p {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := ""
		for _, k := range keys {
			out += k + "=" + p[k] + ";"
		}
		return out
	}
	sample := func() {
		if *tracePeek == "" {
			return
		}
		p := peekMemory(m, *tracePeek)
		if key := traceKey(p); key != lastTrace {
			lastTrace = key
			trace = append(trace, peekSnapshot{Step: m.Steps, Peek: p})
		}
	}
	save := func(label string, frame []uint8) {
		if *out == "" {
			return
		}
		name := fmt.Sprintf("%02d-%s", len(shots), label)
		if err := os.WriteFile(filepath.Join(*out, name+".idx"), frame, 0o644); err != nil {
			fmt.Println("寫色號失敗：", err)
			return
		}
		if err := writePNG(filepath.Join(*out, name+".png"), frame); err != nil {
			fmt.Println("寫 PNG 失敗：", err)
			return
		}
		nz := 0
		for _, v := range frame {
			if v != 0 {
				nz++
			}
		}
		sum := sha256.Sum256(frame)
		sample()
		shots = append(shots, shotInfo{len(shots), label, m.Steps, nz, fmt.Sprintf("%x", sum), name + ".png", peekMemory(m, *peek), trace, traced, damaged})
		trace = nil
		traced = nil
		damaged = nil
		fmt.Printf("  → %s：step %d 非零 %d\n", name, m.Steps, nz)
		for k, v := range shots[len(shots)-1].Peek {
			fmt.Printf("    %s = %s\n", k, v)
		}
	}

	// settle 跑到畫面連續 idle 道指令沒變為止，或用完 budget。
	// 回傳「有沒有真的靜下來」——**逾時與靜止在畫面上一模一樣**，
	// 不分開的話一張拍到一半的圖會被當成穩定畫面收進清冊。
	settle := func() ([]uint8, bool) {
		last := m.IndexedEGA()
		lastChange := m.Steps
		deadline := m.Steps + *budget
		for m.Steps < deadline {
			for i := 0; i < 100_000; i++ {
				if hook != nil {
					intercept()
				}
				if callOn {
					watchCall()
				}
				if err := m.Step(); err != nil {
					fmt.Println("  停下：", err)
					return m.IndexedEGA(), false
				}
			}
			sample()
			frame := m.IndexedEGA()
			if !same(last, frame) {
				last, lastChange = frame, m.Steps
				continue
			}
			if m.Steps-lastChange >= *idle {
				return frame, true
			}
		}
		return m.IndexedEGA(), false
	}

	if *loadState != "" {
		if err := state.Load(*loadState, m, d); err != nil {
			fmt.Println("讀狀態檔失敗：", err)
			os.Exit(1)
		}
		d.Scratch = *scratch
		fmt.Printf("從 %s 接著跑（第 %d 道指令）\n", *loadState, m.Steps)
		watchWrites(m, *watch)
		save("loaded", m.IndexedEGA())
	} else {
		fmt.Println("開機到第一個穩定畫面…")
		frame, ok := settle()
		if !ok {
			fmt.Println("  ⚠ 畫面沒有靜下來（用完 budget）")
		}
		save("boot", frame)
	}

	for _, item := range parseScript(*script) {
		before := d.KeysConsumed
		if err := pushKey(d, item); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		want := len(d.Keys)
		reads := len(d.KeyReads)
		fmt.Printf("送 %s（佇列 %d）\n", item, want)
		frame, ok := settle()
		if !ok {
			fmt.Println("  ⚠ 畫面沒有靜下來")
		}
		if d.KeysConsumed == before {
			fmt.Printf("  ⚠ 程式一個鍵都沒讀走（佇列還有 %d）\n", len(d.Keys))
		}
		// 「鍵送進去了」與「程式吃掉了但不理它」在畫面上長得一樣，
		// 而 KeysConsumed 這個計數兩者都算一次（`docs/spec/185`）。
		if *keytrace {
			for _, r := range d.KeyReads[reads:] {
				fmt.Printf("  讀走 %04X 經 %s，呼叫端 %04X:%04X ← %04X:%04X\n",
					r.Word, r.Via, r.CallerCS, r.CallerIP, r.Caller2CS, r.Caller2IP)
			}
		}
		save(strings.NewReplacer(":", "-", " ", "_").Replace(item), frame)
	}

	if *serve {
		runServe(os.Stdin, m, d, settle, func() []damageRecord {
			out := damaged
			damaged = nil
			return out
		})
		return
	}

	if *saveState != "" {
		if err := state.Save(*saveState, m, d); err != nil {
			fmt.Println("存狀態檔失敗：", err)
			os.Exit(1)
		}
		fmt.Printf("狀態存到 %s（第 %d 道指令）\n", *saveState, m.Steps)
	}

	fmt.Printf("\n跑了 %d 道指令，CS:IP=%04X:%04X，鍵讀走 %d、還剩 %d\n",
		m.Steps, m.CPU.Seg[cpu.CS], m.CPU.IP, d.KeysConsumed, len(d.Keys))
	keys := make([]string, 0, len(calls))
	for k := range calls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println("服務呼叫統計：")
	for _, k := range keys {
		fmt.Printf("  %-18s %d\n", k, calls[k])
	}
	fmt.Println("開過的檔：", len(d.Opened))

	// 顯示相關埠的寫入統計。**「這支程式沒用到圖形控制器」這種結論會過期**
	// ——量的時候程式還沒畫到那一段而已，所以每一輪都重新數。
	video := map[uint16]string{
		0x3C4: "序列器索引", 0x3C5: "序列器資料",
		0x3CE: "圖控索引", 0x3CF: "圖控資料",
		0x3C0: "屬性", 0x3C2: "雜項輸出",
		0x3D4: "CRTC 索引", 0x3D5: "CRTC 資料",
	}
	portCounts := map[uint16]int{}
	portValues := map[uint16]map[uint8]int{}
	for _, w := range m.PortLog {
		if _, ok := video[w.Port]; !ok {
			continue
		}
		portCounts[w.Port]++
		if portValues[w.Port] == nil {
			portValues[w.Port] = map[uint8]int{}
		}
		portValues[w.Port][w.Val]++
	}
	ports := make([]int, 0, len(portCounts))
	for p := range portCounts {
		ports = append(ports, int(p))
	}
	sort.Ints(ports)
	fmt.Println("顯示相關埠的寫入：")
	for _, p := range ports {
		port := uint16(p)
		vals := make([]int, 0, len(portValues[port]))
		for v := range portValues[port] {
			vals = append(vals, int(v))
		}
		sort.Ints(vals)
		fmt.Printf("  %03X %-12s %d 次，值 %v\n", p, video[port], portCounts[port], vals)
	}

	if *out != "" {
		manifest, _ := json.MarshalIndent(shots, "", "  ")
		os.WriteFile(filepath.Join(*out, "shots.json"), append(manifest, '\n'), 0o644)
	}
}

// peekMemory 讀 -peek 指定的幾段記憶體。`ds:` 的段取當下的 DS 暫存器；
// 格式錯的項目印一行警告後跳過，不讓一個打錯的位址把整輪跑掉。
func peekMemory(m *machine.Machine, spec string) map[string]string {
	if spec == "" {
		return nil
	}
	out := map[string]string{}
	for _, item := range strings.Split(spec, ",") {
		f := strings.Split(strings.TrimSpace(item), ":")
		if len(f) != 3 {
			fmt.Printf("  ⚠ -peek 看不懂 %q\n", item)
			continue
		}
		var seg uint64
		var err error
		if f[0] == "ds" {
			seg = uint64(m.CPU.Seg[cpu.DS])
		} else if seg, err = strconv.ParseUint(f[0], 16, 16); err != nil {
			fmt.Printf("  ⚠ -peek 看不懂 %q\n", item)
			continue
		}
		off, err1 := strconv.ParseUint(f[1], 16, 16)
		n, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil || n < 0 {
			fmt.Printf("  ⚠ -peek 看不懂 %q\n", item)
			continue
		}
		base := uint32(seg)*16 + uint32(off)
		buf := make([]byte, n)
		for i := range buf {
			buf[i] = m.Read8(base + uint32(i))
		}
		out[fmt.Sprintf("%s:%s", f[0], strings.ToUpper(f[1]))] = fmt.Sprintf("%04X|%x", seg, buf)
	}
	return out
}

// parseScript 把鍵序展開。`rep:18:Space` 是同一個鍵按 18 次。
func parseScript(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "rep:") {
			parts := strings.SplitN(item, ":", 3)
			if len(parts) == 3 {
				n, err := strconv.Atoi(parts[1])
				if err == nil {
					for i := 0; i < n; i++ {
						out = append(out, parts[2])
					}
					continue
				}
			}
		}
		out = append(out, item)
	}
	return out
}

func same(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func writePNG(path string, frame []uint8) error {
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, ega16[frame[y*320+x]&0x0F])
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

var ega16 = [16]color.RGBA{
	{0, 0, 0, 255}, {0, 0, 170, 255}, {0, 170, 0, 255}, {0, 170, 170, 255},
	{170, 0, 0, 255}, {170, 0, 170, 255}, {170, 85, 0, 255}, {170, 170, 170, 255},
	{85, 85, 85, 255}, {85, 85, 255, 255}, {85, 255, 85, 255}, {85, 255, 255, 255},
	{255, 85, 85, 255}, {255, 85, 255, 255}, {255, 255, 85, 255}, {255, 255, 255, 255},
}

// watchWrites 掛 -watch 的寫入監看。放在載入狀態之後：狀態檔會蓋掉整台機器。
func watchWrites(m *machine.Machine, spec string) {
	if spec == "" {
		return
	}
	var lo, hi uint32
	if _, err := fmt.Sscanf(spec, "%x-%x", &lo, &hi); err != nil {
		fmt.Println("  ⚠ -watch 看不懂", spec, err)
		return
	}
	m.WatchWrites(lo, hi, func(addr uint32, old, nw uint8) {
		fmt.Printf("[watch] #%d %05X: %02X → %02X  ← %04X:%04X\n",
			m.Steps, addr, old, nw, m.CPU.Seg[cpu.CS], m.CPU.IP)
	})
}
