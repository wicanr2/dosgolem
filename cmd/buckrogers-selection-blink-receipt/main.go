// Command buckrogers-selection-blink-receipt samples selected-row pixels and palette over time.
package main

import (
	"crypto/sha256"
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

type rgbJSON struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
}

type regionJSON struct {
	SHA256 string        `json:"sha256"`
	Counts map[uint8]int `json:"counts"`
}

type sampleJSON struct {
	Step             uint64            `json:"step"`
	PaletteSHA256    string            `json:"palette_sha256"`
	Palette          map[uint8]rgbJSON `json:"palette"`
	SelectedContrast bool              `json:"selected_contrast"`
	Row3             regionJSON        `json:"row3"`
	Row4             regionJSON        `json:"row4"`
	CompletedEvents  int               `json:"completed_events"`
}

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

type resultJSON struct {
	Tool        string       `json:"tool"`
	StateSHA256 string       `json:"state_sha256"`
	MenuSHA256  string       `json:"menu_events_sha256"`
	StateStart  uint64       `json:"state_start"`
	StoppedAt   uint64       `json:"stopped_at"`
	EnterAt     uint64       `json:"enter_at"`
	DownAt      uint64       `json:"down_at"`
	SampleFrom  uint64       `json:"sample_from"`
	SampleEvery uint64       `json:"sample_every"`
	Samples     []sampleJSON `json:"samples"`
	Events      []eventJSON  `json:"events"`
	Pending     bool         `json:"pending"`
	Drops       int          `json:"drops"`
}

func main() {
	statePath := flag.String("state", "", "既有 state")
	menuPath := flag.String("menu-events", "", "正式 menu-events.tsv（只固定輸入雜湊）")
	until := flag.Uint64("until", 0, "絕對停止 step")
	enterAt := flag.Uint64("enter-at", 100010000, "Enter 排程")
	downAt := flag.Uint64("down-at", 0, "Down 排程；0 表示不送")
	sampleFrom := flag.Uint64("sample-from", 100220000, "第一筆取樣 step")
	sampleEvery := flag.Uint64("sample-every", 1000, "固定取樣間隔")
	flag.Parse()
	if *statePath == "" || *menuPath == "" || *until == 0 || *sampleEvery == 0 || *sampleFrom >= *until || *enterAt >= *until || (*downAt != 0 && (*downAt <= *enterAt || *downAt >= *until)) {
		fail(fmt.Errorf("state、until、sample 範圍或按鍵排程無效"))
	}

	m := machine.New()
	d := dos.New(m, ".")
	d.Install()
	if err := state.Load(*statePath, m, d); err != nil {
		fail(err)
	}
	start := m.Steps
	r := buckrogers.NewMenuRequestWatcher(nil)
	enterSent, downSent := false, false
	nextSample := *sampleFrom
	samples := make([]sampleJSON, 0, int((*until-*sampleFrom) / *sampleEvery)+1)
	for m.Steps < *until && !d.Exited {
		if !enterSent && m.Steps >= *enterAt {
			if !m.PushBIOSKey(0x1c, 0x0d) {
				fail(fmt.Errorf("Enter：BIOS buffer full"))
			}
			enterSent = true
		}
		if *downAt != 0 && !downSent && m.Steps >= *downAt {
			if !m.PushBIOSKey(0x50, 0x00) {
				fail(fmt.Errorf("Down：BIOS buffer full"))
			}
			downSent = true
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
		for m.Steps >= nextSample && nextSample < *until {
			samples = append(samples, takeSample(m, len(r.Events())))
			nextSample += *sampleEvery
		}
		if err := m.Step(); err != nil {
			fail(err)
		}
	}
	if !enterSent || (*downAt != 0 && !downSent) || m.Steps != *until {
		fail(fmt.Errorf("重播未達排程／終點：enter=%v down=%v step=%d", enterSent, downSent, m.Steps))
	}
	events := r.Events()
	outEvents := make([]eventJSON, len(events))
	for i, e := range events {
		outEvents[i] = eventJSON{e.EntryStep, e.PostCallStep, e.Caller, e.OriginalLength, hex.EncodeToString(e.OriginalSHA256[:]), e.Background, e.Foreground, e.Row, e.Column}
	}
	result := resultJSON{
		"dosgolem/cmd/buckrogers-selection-blink-receipt", hash(mustRead(*statePath)), hash(mustRead(*menuPath)),
		start, m.Steps, *enterAt, *downAt, *sampleFrom, *sampleEvery, samples, outEvents, r.Pending(), r.Drops(),
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fail(err)
	}
}

func takeSample(m *machine.Machine, completed int) sampleJSON {
	pal := m.Palette()
	flat := make([]byte, 0, 256*3)
	for _, c := range pal {
		flat = append(flat, c[0], c[1], c[2])
	}
	p := map[uint8]rgbJSON{}
	for _, i := range []uint8{0, 10, 13, 15} {
		p[i] = rgbJSON{pal[i][0], pal[i][1], pal[i][2]}
	}
	return sampleJSON{
		Step: m.Steps, PaletteSHA256: hash(flat), Palette: p,
		SelectedContrast: pal[15] != pal[0],
		Row3:             region(m.Indexed(), 24, 72, 24, 32),
		Row4:             region(m.Indexed(), 24, 80, 32, 40),
		CompletedEvents:  completed,
	}
}

func region(indexed []byte, x0, x1, y0, y1 int) regionJSON {
	b := make([]byte, 0, (x1-x0)*(y1-y0))
	counts := map[uint8]int{}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			v := indexed[y*320+x]
			b = append(b, v)
			counts[v]++
		}
	}
	return regionJSON{hash(b), counts}
}

func hash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
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
