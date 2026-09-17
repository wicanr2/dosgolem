package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

// -serve：程式開著，從 stdin 一行讀一道命令，每道命令回一行以 `@@ ` 開頭的 JSON。
// stdout 上其他的字（`[damage]`、`[watch]`）照印，駕駛程式只認 `@@ ` 那一行。
//
//	key <鍵序>        跟 -keys 同格式；每個鍵都等畫面靜下來。回 steps、settled、consumed（程式讀走幾個鍵）
//	wait              不送鍵，再跑到畫面靜下來
//	peek <規格>       跟 -peek 同格式，回 peek
//	poke <seg>:<off> <hex>  寫記憶體
//	frame <檔名>      把目前 320×200 色號寫成 .idx
//	save <檔名>       存狀態檔
//	quit
//
// 每一個回覆都帶 damage：上一道命令以來 -intercept-damage 攔到的每一次。
// 錯誤回 {"error": "..."}，不結束程式——一個打錯的命令不該把一整段狀態丟掉。

type serveReply struct {
	Steps    uint64            `json:"steps"`
	Settled  bool              `json:"settled"`
	Consumed int               `json:"consumed"`
	Queued   int               `json:"queued"`
	CSIP     string            `json:"cs_ip"`
	Peek     map[string]string `json:"peek,omitempty"`
	Damage   []damageRecord    `json:"damage,omitempty"`
	Error    string            `json:"error,omitempty"`
}

func runServe(in io.Reader, m *machine.Machine, d *dos.DOS, settle func() ([]uint8, bool), takeDamage func() []damageRecord) {
	reader := bufio.NewScanner(in)
	reader.Buffer(make([]byte, 1<<20), 1<<20)
	reply := func(r serveReply) {
		r.Steps = m.Steps
		r.Queued = len(d.Keys)
		r.CSIP = fmt.Sprintf("%04X:%04X", m.CPU.Seg[cpu.CS], m.CPU.IP)
		r.Damage = takeDamage()
		// serve 不報連接埠統計；不清的話長時間駕駛（野外、動畫）會讓這份紀錄吃光記憶體。
		m.PortLog = m.PortLog[:0]
		b, _ := json.Marshal(r)
		fmt.Printf("@@ %s\n", b)
		os.Stdout.Sync()
	}
	fmt.Println("@@ {\"ready\":true}")
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		command, arg, _ := strings.Cut(line, " ")
		switch command {
		case "key":
			r := serveReply{Settled: true}
			before := d.KeysConsumed
			for _, item := range parseScript(arg) {
				if err := pushKey(d, item); err != nil {
					r.Error = err.Error()
					break
				}
				if _, ok := settle(); !ok {
					r.Settled = false
				}
			}
			r.Consumed = d.KeysConsumed - before
			reply(r)
		case "wait":
			// 不送鍵，讓程式再跑到畫面靜下來（過場動畫、延遲出字）。
			_, ok := settle()
			reply(serveReply{Settled: ok})
		case "peek":
			reply(serveReply{Settled: true, Peek: peekMemory(m, arg)})
		case "poke":
			// poke <seg>:<off> <hex bytes>：直接寫記憶體（駕駛的暫時穿牆用，寫了什麼由呼叫端記收據）。
			r := serveReply{Settled: true}
			where, data, _ := strings.Cut(arg, " ")
			var seg, off uint32
			raw, err := hex.DecodeString(strings.TrimSpace(data))
			if _, scanErr := fmt.Sscanf(where, "%x:%x", &seg, &off); scanErr != nil || err != nil || len(raw) == 0 {
				r.Error = "poke 看不懂：" + arg
			} else {
				for i, b := range raw {
					m.Write8(seg*16+off+uint32(i), b)
				}
			}
			reply(r)
		case "frame":
			r := serveReply{Settled: true}
			if err := os.WriteFile(arg, m.IndexedEGA(), 0o644); err != nil {
				r.Error = err.Error()
			}
			reply(r)
		case "save":
			r := serveReply{Settled: true}
			if err := state.Save(arg, m, d); err != nil {
				r.Error = err.Error()
			}
			reply(r)
		case "quit":
			reply(serveReply{Settled: true})
			return
		default:
			reply(serveReply{Error: "看不懂的命令：" + command})
		}
	}
}

// pushKey 把鍵序裡的一項放進鍵盤佇列：`type:文字`、具名鍵，或單一字元。
func pushKey(d *dos.DOS, item string) error {
	if strings.HasPrefix(item, "type:") {
		text := strings.TrimPrefix(item, "type:")
		if !d.PushText(text) {
			return fmt.Errorf("鍵盤表沒有 %q 裡的某個字元", text)
		}
		return nil
	}
	if !d.PushKeyNamed(item) && !d.PushText(item) {
		return fmt.Errorf("不認得按鍵 %q", item)
	}
	return nil
}
