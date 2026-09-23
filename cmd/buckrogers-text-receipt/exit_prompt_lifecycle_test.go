package main

import (
	"testing"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestExitPromptLifecycleTraceValidation(t *testing.T) {
	q1 := buckrogers.PostJoinExitPromptSnapshot{Q1Active: true}
	q1q2 := buckrogers.PostJoinExitPromptSnapshot{Q1Active: true, Q2Pending: true}
	q2pending := buckrogers.PostJoinExitPromptSnapshot{Q2Pending: true}
	q2active := buckrogers.PostJoinExitPromptSnapshot{Q2Active: true}
	base := exitPromptLifecycleTrace{Schema: "post-join-exit-prompt-lifecycle-v1", Case: "yy"}
	base.append("q1_return", 1, buckrogers.PostJoinExitPromptSnapshot{}, q1)
	base.append("q2_entry", 2, q1, q1q2)
	base.prewrite(machine.VideoWrite{Step: 3, CS: 0x0763, IP: 0x1854, Offset: 0xF000, Value: 0}, 0, q1q2, q2pending)
	base.append("q2_return", 4, q2pending, q2active)
	if err := base.validate(); err != nil {
		t.Fatalf("valid lifecycle: %v", err)
	}
	mutate := func(name string, fn func(*exitPromptLifecycleTrace)) {
		t.Helper()
		copyTrace := base
		copyTrace.Events = append([]exitPromptLifecycleEvent(nil), base.Events...)
		fn(&copyTrace)
		if err := copyTrace.validate(); err == nil {
			t.Errorf("%s: accepted invalid lifecycle", name)
		}
	}
	mutate("no overlap", func(x *exitPromptLifecycleTrace) { x.Events[1].Before.Q1Active = false })
	mutate("different value", func(x *exitPromptLifecycleTrace) { v := uint8(1); x.Events[2].New = &v })
	mutate("q2 accidentally cleared", func(x *exitPromptLifecycleTrace) { x.Events[2].After.Q2Pending = false })
	mutate("wrong writer", func(x *exitPromptLifecycleTrace) { x.Events[2].WriterIP = 0x9999 })
	mutate("tail cell", func(x *exitPromptLifecycleTrace) { x.Events[2].A000Offset = 0xF000 + 96 })
	mutate("wrong order", func(x *exitPromptLifecycleTrace) { x.Events[2].Step = 2 })
	mutate("missing return", func(x *exitPromptLifecycleTrace) { x.Events = x.Events[:3] })
	n := exitPromptLifecycleTrace{Schema: "post-join-exit-prompt-lifecycle-v1", Case: "n"}
	n.append("q1_return", 1, buckrogers.PostJoinExitPromptSnapshot{}, q1)
	n.prewrite(machine.VideoWrite{Step: 2, CS: 0x0763, IP: 0x184D, Offset: 0xF000, Value: 0}, 0, q1, buckrogers.PostJoinExitPromptSnapshot{})
	if err := n.validate(); err != nil {
		t.Fatalf("valid N lifecycle: %v", err)
	}
	n.Events[1].Before.Q2Pending = true
	if err := n.validate(); err == nil {
		t.Error("N wrongly accepted q2 pending")
	}
	stop := base
	stop.Case = "stop"
	stop.Events = append(append([]exitPromptLifecycleEvent(nil), base.Events...), exitPromptLifecycleEvent{Kind: "stop", Step: 5, Before: q2active, After: buckrogers.PostJoinExitPromptSnapshot{Closed: true}})
	if err := stop.validate(); err != nil {
		t.Fatalf("valid Stop lifecycle: %v", err)
	}
	stop.Events[4].After.Q2Active = true
	if err := stop.validate(); err == nil {
		t.Error("Stop wrongly accepted active layer")
	}
}
