// Command buckrogers-receipt replays an existing diagnostic state and emits
// answer-free metadata for the original manual-check output path.
package main

import (
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

func main() {
	statePath := flag.String("state", "", "既有 probe state")
	eventsPath := flag.String("events", "", "manual-events.tsv")
	ordinalsPath := flag.String("ordinals", "", "manual-ordinals.tsv")
	translationsPath := flag.String("translations", "", "manual.zh-TW.tsv")
	until := flag.Uint64("until", 0, "絕對指令步數上限")
	flag.Parse()
	if *statePath == "" || *eventsPath == "" || *ordinalsPath == "" || *translationsPath == "" || *until == 0 {
		fail(fmt.Errorf("state、events、ordinals、translations、until 均為必填"))
	}
	catalog, err := buckrogers.LoadCatalog(mustRead(*eventsPath), mustRead(*ordinalsPath), mustRead(*translationsPath))
	if err != nil {
		fail(err)
	}
	m := machine.New()
	d := dos.New(m, ".")
	d.Install()
	if err := state.Load(*statePath, m, d); err != nil {
		fail(err)
	}
	start := m.Steps
	w := buckrogers.NewWatcher(catalog)
	for m.Steps < *until && !d.Exited && len(w.Requests()) == 0 {
		at := buckrogers.Address{Segment: m.CPU.Seg[cpu.CS], Offset: m.CPU.IP}
		ss, sp := m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP]
		switch at {
		case (buckrogers.Address{Segment: 0x0763, Offset: 0x0424}):
			caller := buckrogers.Address{Segment: m.Read16(cpu.Addr(ss, sp+2)), Offset: m.Read16(cpu.Addr(ss, sp))}
			off := m.Read16(cpu.Addr(ss, sp+4))
			seg := m.Read16(cpu.Addr(ss, sp+6))
			base := cpu.Addr(seg, off)
			n := int(m.Read8(base))
			textBytes := make([]byte, n)
			for i := range textBytes {
				textBytes[i] = m.Read8(base + 1 + uint32(i))
			}
			text := string(textBytes)
			w.ObserveDispatchEntry(caller, ss, sp, text, m.Steps)
		case (buckrogers.Address{Segment: 0x026F, Offset: 0x029C}):
			w.ObserveClear(m.Steps)
		default:
			w.ObserveInstruction(at, ss, sp, m.Steps)
		}
		if err := m.Step(); err != nil {
			fail(err)
		}
	}
	type requestMetadata struct {
		Generation       uint64 `json:"generation"`
		EventKey         string `json:"event_key"`
		TextKey          string `json:"text_key"`
		TranslationRunes int    `json:"translation_runes"`
	}
	result := struct {
		StateStart uint64                   `json:"state_start"`
		StoppedAt  uint64                   `json:"stopped_at"`
		Events     []buckrogers.Observation `json:"events"`
		Request    *requestMetadata         `json:"request,omitempty"`
	}{StateStart: start, StoppedAt: m.Steps, Events: w.Observations()}
	if requests := w.Requests(); len(requests) != 0 {
		r := requests[0]
		result.Request = &requestMetadata{r.Generation, r.EventKey, r.TextKey, len([]rune(r.Translation))}
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fail(err)
	}
}

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	return b
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
