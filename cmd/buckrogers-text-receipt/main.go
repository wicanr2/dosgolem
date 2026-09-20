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

type requestJSON struct {
	EventKey         string `json:"event_key"`
	TextKey          string `json:"text_key"`
	TranslationRunes int    `json:"translation_runes"`
}

func main() {
	statePath := flag.String("state", "", "既有 probe state")
	until := flag.Uint64("until", 0, "絕對指令步數上限")
	want := flag.Int("want", 0, "預期完成事件數；0 表示不檢查")
	wantRequests := flag.Int("want-requests", 0, "預期顯示請求數；0 表示不檢查")
	enterAt := flag.Uint64("bios-enter-at", 0, "在此絕對步數排入一個 BIOS Enter；0 表示不送")
	menuEvents := flag.String("menu-events", "", "正式 menu-events.tsv")
	menuTranslations := flag.String("menu-translations", "", "正式 menu.zh-TW.tsv")
	flag.Parse()
	if *statePath == "" || *until == 0 {
		fail(fmt.Errorf("state 與 until 為必填"))
	}
	if err := validateMenuCatalogFlags(*menuEvents, *menuTranslations); err != nil {
		fail(err)
	}
	var catalog *buckrogers.MenuCatalog
	if *menuEvents != "" {
		eventsData, err := os.ReadFile(*menuEvents)
		if err != nil {
			fail(err)
		}
		translationsData, err := os.ReadFile(*menuTranslations)
		if err != nil {
			fail(err)
		}
		catalog, err = buckrogers.LoadMenuCatalog(eventsData, translationsData)
		if err != nil {
			fail(err)
		}
	}
	m := machine.New()
	d := dos.New(m, ".")
	d.Install()
	if err := state.Load(*statePath, m, d); err != nil {
		fail(err)
	}
	start := m.Steps
	r := buckrogers.NewMenuRequestWatcher(catalog)
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
	requests := r.Requests()
	if r.Pending() || r.Drops() != 0 || (*want != 0 && len(events) != *want) ||
		(*wantRequests != 0 && len(requests) != *wantRequests) {
		fail(fmt.Errorf("收據失敗：events=%d want=%d requests=%d want_requests=%d pending=%v drops=%d misses=%d",
			len(events), *want, len(requests), *wantRequests, r.Pending(), r.Drops(), r.Misses()))
	}
	out := make([]eventJSON, len(events))
	for i, e := range events {
		out[i] = eventJSON{
			e.EntryStep, e.PostCallStep, e.Caller, e.OriginalLength,
			hex.EncodeToString(e.OriginalSHA256[:]), e.Background, e.Foreground, e.Row, e.Column,
		}
	}
	requestOut := make([]requestJSON, len(requests))
	for i, request := range requests {
		requestOut[i] = requestJSON{request.EventKey, request.TextKey, len([]rune(request.Translation))}
	}
	result := struct {
		StateStart    uint64        `json:"state_start"`
		StoppedAt     uint64        `json:"stopped_at"`
		BIOSInput     string        `json:"bios_input,omitempty"`
		Events        []eventJSON   `json:"events"`
		Requests      []requestJSON `json:"requests,omitempty"`
		CatalogMisses *int          `json:"catalog_misses,omitempty"`
	}{StateStart: start, StoppedAt: m.Steps, Events: out}
	if catalog != nil {
		result.Requests = requestOut
		misses := r.Misses()
		result.CatalogMisses = &misses
	}
	if enterSent {
		result.BIOSInput = fmt.Sprintf("Enter(scan=0x1c,ascii=0x0d,queued_at=%d)", *enterAt)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fail(err)
	}
}

func validateMenuCatalogFlags(events, translations string) error {
	if (events == "") != (translations == "") {
		return fmt.Errorf("menu-events 與 menu-translations 必須同時提供")
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
