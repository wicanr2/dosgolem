package session

import (
	"bytes"
	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/internal/machine"
	"reflect"
	"strings"
	"testing"
)

type pcmRec struct {
	recordingObserver
	pcm      []machine.SBPCMEvent
	dma      []machine.SBDMAEvent
	panicAt  int
	badOrder bool
	mix      *mixer.Mixer
}

func (r *pcmRec) SBPCM(e machine.SBPCMEvent) {
	if len(r.dma) == 0 || r.dma[len(r.dma)-1].Kind != e.Kind {
		r.badOrder = true
	}
	r.pcm = append(r.pcm, e)
	if r.mix != nil {
		r.mix.SBPCM(e)
	}
	if r.panicAt == len(r.pcm) {
		panic("sbpcm boom")
	}
}
func (r *pcmRec) SBDMA(e machine.SBDMAEvent) { r.dma = append(r.dma, e) }

func TestSBPCMObserverForwardsAndPreservesState(t *testing.T) {
	r := &pcmRec{mix: mixer.New(11550000)}
	a, b := startSBOwner(t, true, r), startSBOwner(t, true, &recordingObserver{})
	for _, o := range []*Owner{a, b} {
		for i := 0; i < sbtLen; i++ {
			o.machine.Mem[sbtPage<<16|sbtAddr+i] = byte(i)
		}
	}
	for _, o := range []*Owner{a, b} {
		if _, err := runTurn(t, o, sbtBudget); err != nil {
			t.Fatal(err)
		}
	}
	if len(r.pcm) == 0 || r.badOrder {
		t.Fatalf("events=%d badOrder=%t", len(r.pcm), r.badOrder)
	}
	var filtered []machine.SBDMAEvent
	for _, e := range r.dma {
		switch e.Kind {
		case "start", "pause", "resume", "cancel", "complete":
			filtered = append(filtered, e)
		}
	}
	if len(r.pcm) != len(filtered) {
		t.Fatal("event filter")
	}
	for i, e := range r.pcm {
		d := filtered[i]
		if e.Kind != d.Kind || e.Step != d.Step || e.Due != d.Due || e.Transferred != d.Transferred {
			t.Fatalf("%+v != %+v", e, d)
		}
		if e.Kind == "start" && e.Command != 0x80 {
			for j, b := range e.Data {
				if b != byte(j) {
					t.Fatal("pattern")
				}
			}
		}
	}
	// 掛著 Mixer、Render 與否不改原版狀態或狀態檔。
	r.mix.Render(nil, 0, sbtBudget)
	da, ea := a.Digest()
	db, eb := b.Digest()
	if ea != nil || eb != nil || da != db {
		t.Fatalf("digest: %v %v", ea, eb)
	}
	var x, y bytes.Buffer
	if err := a.machine.SaveState(&x); err != nil {
		t.Fatal(err)
	}
	if err := b.machine.SaveState(&y); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(x.Bytes(), y.Bytes()) || !reflect.DeepEqual(r.dma, b.machine.SBDMALog) {
		t.Fatal("state bytes/SBDMA changed")
	}
}

func TestSBPCMObserverPanic(t *testing.T) {
	r := &pcmRec{panicAt: 1}
	o := startSBOwner(t, true, r)
	result, err := runTurn(t, o, sbtBudget)
	if err == nil || !strings.Contains(err.Error(), "SBPCM") {
		t.Fatal(err)
	}
	assertObserverFault(t, o, result, err, r.pcm[0].Step)
}

func TestSBPCMObserverRemovedAndDisabled(t *testing.T) {
	r := &pcmRec{}
	o := startSBOwner(t, true, r)
	events := sbtFullRun(t)
	n := events[0].Step
	if _, err := runTurn(t, o, InstructionBudget(n-1)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if err := o.machine.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(r.pcm) != 0 {
		t.Fatal("Advance 後回呼未移除")
	}
	r = &pcmRec{}
	o = startSBOwner(t, false, r)
	if result, _ := runTurn(t, o, 20000); result.Steps == 0 {
		t.Fatal("未前進")
	}
	if len(r.pcm) != 0 {
		t.Fatal("未開 SBDMA8 卻轉發")
	}
}

type pcmOnly struct {
	recordingObserver
	count int
}

func (r *pcmOnly) SBPCM(machine.SBPCMEvent) { r.count++ }
func TestSBPCMObserverIndependentOfSBDMA(t *testing.T) {
	r := &pcmOnly{}
	o := startSBOwner(t, true, r)
	if _, err := runTurn(t, o, sbtBudget); err != nil {
		t.Fatal(err)
	}
	if r.count == 0 || len(o.machine.SBDMALog) == 0 {
		t.Fatal("兩個觀察器必須獨立")
	}
}
