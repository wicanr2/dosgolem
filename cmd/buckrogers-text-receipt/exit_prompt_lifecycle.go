package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// This optional receipt records state transitions, not original glyph bytes.
type exitPromptLifecycleEvent struct {
	Kind       string                                `json:"kind"`
	Step       uint64                                `json:"step"`
	Before     buckrogers.PostJoinExitPromptSnapshot `json:"before"`
	After      buckrogers.PostJoinExitPromptSnapshot `json:"after"`
	WriterCS   uint16                                `json:"writer_cs,omitempty"`
	WriterIP   uint16                                `json:"writer_ip,omitempty"`
	A000Offset uint32                                `json:"a000_offset,omitempty"`
	Old        *uint8                                `json:"old_index,omitempty"`
	New        *uint8                                `json:"new_index,omitempty"`
}

type exitPromptLifecycleTrace struct {
	Schema string                     `json:"schema"`
	Case   string                     `json:"case"`
	Events []exitPromptLifecycleEvent `json:"events"`
}

func (t *exitPromptLifecycleTrace) append(kind string, step uint64, before, after buckrogers.PostJoinExitPromptSnapshot) {
	if t != nil {
		t.Events = append(t.Events, exitPromptLifecycleEvent{Kind: kind, Step: step, Before: before, After: after})
	}
}

func (t *exitPromptLifecycleTrace) prewrite(w machine.VideoWrite, old uint8, before, after buckrogers.PostJoinExitPromptSnapshot) {
	if t == nil || !before.Q1Active || after.Q1Active {
		return
	}
	o, n := old, w.Value
	t.Events = append(t.Events, exitPromptLifecycleEvent{Kind: "q1_first_clear_prewrite", Step: w.Step, Before: before, After: after, WriterCS: w.CS, WriterIP: w.IP, A000Offset: w.Offset, Old: &o, New: &n})
}

func (t *exitPromptLifecycleTrace) validate() error {
	if t == nil {
		return nil
	}
	want := []string{"q1_return", "q1_first_clear_prewrite"}
	if t.Case == "yy" || t.Case == "stop" {
		want = []string{"q1_return", "q2_entry", "q1_first_clear_prewrite", "q2_return"}
	}
	if t.Case == "stop" {
		want = append(want, "stop")
	}
	if t.Case != "n" && t.Case != "yy" && t.Case != "stop" {
		return fmt.Errorf("invalid Exit lifecycle case %q", t.Case)
	}
	if len(t.Events) != len(want) {
		return fmt.Errorf("Exit lifecycle %s event count %d, want %d", t.Case, len(t.Events), len(want))
	}
	for i, kind := range want {
		e := t.Events[i]
		if e.Kind != kind || e.Step == 0 || (i > 0 && e.Step <= t.Events[i-1].Step) || e.Before.Failed || e.After.Failed {
			return fmt.Errorf("Exit lifecycle %s event %d invalid", t.Case, i)
		}
	}
	q1 := t.Events[0]
	if q1.Before.Q1Active || !q1.After.Q1Active {
		return fmt.Errorf("Exit lifecycle q1 return invalid")
	}
	clearIndex := 1
	if t.Case != "n" {
		q2 := t.Events[1]
		if !q2.Before.Q1Active || q2.Before.Q2Pending || !q2.After.Q1Active || !q2.After.Q2Pending {
			return fmt.Errorf("Exit lifecycle q2 overlap invalid")
		}
		clearIndex = 2
	}
	clear := t.Events[clearIndex]
	bodyOffset := clear.A000Offset
	insideQ1Body := bodyOffset < 320*200 && bodyOffset/320 >= 192 && bodyOffset%320 < 96
	if !clear.Before.Q1Active || clear.After.Q1Active || clear.Old == nil || clear.New == nil || *clear.Old != *clear.New || !insideQ1Body || clear.WriterCS != 0x0763 || (clear.WriterIP != 0x184D && clear.WriterIP != 0x1854) {
		return fmt.Errorf("Exit lifecycle first equal-value A000 clear invalid")
	}
	if t.Case == "n" {
		if clear.Before.Q2Pending || clear.After.Q2Pending || clear.After.Q2Active {
			return fmt.Errorf("Exit lifecycle N clear invalid")
		}
	} else {
		if !clear.Before.Q2Pending || !clear.After.Q2Pending || clear.After.Q2Active {
			return fmt.Errorf("Exit lifecycle pending q2 clear invalid")
		}
		ret := t.Events[3]
		if !ret.Before.Q2Pending || ret.Before.Q1Active || ret.After.Q2Pending || !ret.After.Q2Active {
			return fmt.Errorf("Exit lifecycle q2 return invalid")
		}
	}
	if t.Case == "stop" {
		stop := t.Events[4]
		if !stop.Before.Q2Active || !stop.After.Closed || stop.After.Q1Active || stop.After.Q2Pending || stop.After.Q2Active {
			return fmt.Errorf("Exit lifecycle Stop invalid")
		}
	}
	return nil
}

func (t *exitPromptLifecycleTrace) write(path string) error {
	if err := t.validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}
