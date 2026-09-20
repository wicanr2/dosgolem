// Command buckrogers-text-receipt replays a diagnostic state and emits only
// content-free metadata for completed 0763:0424 calls.
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

type eventJSON struct {
	EntryStep      uint64             `json:"entry_step"`
	PostCallStep   uint64             `json:"post_call_step"`
	Caller         buckrogers.Address `json:"caller"`
	OriginalLength uint8              `json:"original_length"`
	OriginalSHA256 string             `json:"original_sha256"`
	Background     uint8              `json:"background"`
	Foreground     uint8              `json:"foreground"`
	Row            uint8              `json:"row"`
	Column         uint8              `json:"column"`
}

func main() {
	statePath := flag.String("state", "", "既有 probe state")
	until := flag.Uint64("until", 0, "絕對指令步數上限")
	want := flag.Int("want", 0, "預期完成事件數；0 表示不檢查")
	enterAt := flag.Uint64("bios-enter-at", 0, "在此絕對步數排入一個 BIOS Enter；0 表示不送")
	flag.Parse()
	if *statePath == "" || *until == 0 {
		fail(fmt.Errorf("state 與 until 為必填"))
	}
	m := machine.New()
	d := dos.New(m, ".")
	d.Install()
	if err := state.Load(*statePath, m, d); err != nil {
		fail(err)
	}
	start := m.Steps
	r := &buckrogers.TextRecorder{}
	enterSent := false
	for m.Steps < *until && !d.Exited {
		if *enterAt != 0 && !enterSent && m.Steps >= *enterAt {
			if !m.PushBIOSKey(0x1C, 0x0D) {
				fail(fmt.Errorf("BIOS 鍵盤緩衝區已滿"))
			}
			enterSent = true
		}
		at := buckrogers.Address{Segment: m.CPU.Seg[cpu.CS], Offset: m.CPU.IP}
		ss, sp := m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP]
		if at == (buckrogers.Address{Segment: 0x0763, Offset: 0x0424}) {
			caller := buckrogers.Address{Segment: m.Read16(cpu.Addr(ss, sp+2)), Offset: m.Read16(cpu.Addr(ss, sp))}
			var args [6]uint16
			for i := range args {
				args[i] = m.Read16(cpu.Addr(ss, sp+4+uint16(i)*2))
			}
			base := cpu.Addr(args[1], args[0])
			n := int(m.Read8(base))
			original := make([]byte, n)
			for i := range original {
				original[i] = m.Read8(base + 1 + uint32(i))
			}
			r.ObserveDispatchEntry(caller, ss, sp, args, original, m.Steps)
		} else {
			r.ObserveInstruction(at, ss, sp, m.Steps)
		}
		if err := m.Step(); err != nil {
			fail(err)
		}
	}
	events := r.Events()
	if r.Pending() || r.Drops() != 0 || (*want != 0 && len(events) != *want) {
		fail(fmt.Errorf("收據失敗：events=%d want=%d pending=%v drops=%d", len(events), *want, r.Pending(), r.Drops()))
	}
	out := make([]eventJSON, len(events))
	for i, e := range events {
		out[i] = eventJSON{
			e.EntryStep, e.PostCallStep, e.Caller, e.OriginalLength,
			hex.EncodeToString(e.OriginalSHA256[:]), e.Background, e.Foreground, e.Row, e.Column,
		}
	}
	result := struct {
		StateStart uint64      `json:"state_start"`
		StoppedAt  uint64      `json:"stopped_at"`
		BIOSInput  string      `json:"bios_input,omitempty"`
		Events     []eventJSON `json:"events"`
	}{StateStart: start, StoppedAt: m.Steps, Events: out}
	if enterSent {
		result.BIOSInput = fmt.Sprintf("Enter(scan=0x1c,ascii=0x0d,queued_at=%d)", *enterAt)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
