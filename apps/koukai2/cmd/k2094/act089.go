package main

// 批次 089：資訊畫面、列表視窗、NPC、航海等的額外動作。全部只觀測；強制呼叫與 poke 是測試條件，一律寫進事件檔。
//   keyat:IDA:名字+名字…   第一次到 IDA 時送鍵（到了才送，不改被觀測程式的記憶體）。用在強制呼叫的互動函式（等鍵的列表視窗）。

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

func (e *env) act089(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	switch f[0] {
	case "fkeep":
		return e.fkeep089(f)
	case "bfshooks":
		e.installBFS089()
		return nil
	case "aihooks":
		e.installAI089()
		return nil
	case "jcase":
		spec := ""
		if len(f) > 1 {
			spec = f[1]
		}
		return e.installJump089(spec)
	case "keyat":
		lin, err := strconv.ParseUint(f[1], 16, 64)
		if err != nil {
			return err
		}
		keys := strings.Split(f[2], "+")
		done := false
		o.OnCall(b.Code(uint32(lin)), func(o *oracle.Oracle) {
			if done {
				return
			}
			done = true
			for _, k := range keys {
				_ = o.SendKeys(k)
			}
			e.log.Emit(Ev78{Kind: "note", Label: "keyat", IDA: f[1], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"keys": f[2]}})
		})
		return nil
	}
	return e.act081(f)
}

var _ = fmt.Sprintf
