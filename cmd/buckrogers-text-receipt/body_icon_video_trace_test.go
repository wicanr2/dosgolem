package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestBodyIconA000ObserverKeepsContentSafeOrderedEvidence(t *testing.T) {
	rects := []bodyIconRect{{EventKey: "safe", X: 10, Y: 1, Width: 4, Height: 1}}
	writes := []machine.VideoWrite{
		{Step: 10, CS: 0x1000, IP: 0x20, Offset: 330, Value: 0x5a, WriteMode: 0},
		{Step: 10, CS: 0x1000, IP: 0x20, Offset: 331, Value: 0x5a, WriteMode: 0},
		{Step: 11, CS: 0x1000, IP: 0x20, Offset: 333, Value: 0x5a, WriteMode: 0}, // gap starts a new span
		{Step: 12, CS: 0x2000, IP: 0x30, Offset: 332, Value: 0x5a, WriteMode: 1}, // distinct writer and mode
		{Step: 13, CS: 0x2000, IP: 0x30, Offset: 900, Value: 0x5a, WriteMode: 1}, // observed A000, outside safe rect
	}
	observer := newBodyIconA000Observer(rects, 10, 20, []uint64{11})
	for _, write := range writes {
		observer.Observe(write)
	}
	report := observer.Report()
	if report.A000WriteCount != 5 || report.RectIntersectingWriteCount != 4 {
		t.Fatalf("all-A000/rect counts=%d/%d", report.A000WriteCount, report.RectIntersectingWriteCount)
	}
	if report.GlobalEarliest == nil || report.GlobalEarliest.Ordinal != 1 || report.GlobalEarliest.VideoOffset != 330 {
		t.Fatalf("global earliest=%+v", report.GlobalEarliest)
	}
	if len(report.PerKeyEarliest) != 1 || report.PerKeyEarliest[0].Writes != 4 || report.PerKeyEarliest[0].First.Ordinal != 1 {
		t.Fatalf("per-key earliest=%+v", report.PerKeyEarliest)
	}
	if len(report.FirstAfterSteps) != 1 || len(report.FirstAfterSteps[0].FirstByKey) != 1 || report.FirstAfterSteps[0].FirstByKey[0].First.Step != 12 {
		t.Fatalf("threshold first=%+v", report.FirstAfterSteps)
	}
	if len(report.WriterGroups) != 2 || len(report.ContiguousSpans) != 3 {
		t.Fatalf("groups/spans=%d/%d; expected writer+mode groups and the noncontinuous-offset split", len(report.WriterGroups), len(report.ContiguousSpans))
	}
	if report.ContiguousSpans[0].Writes != 2 || report.ContiguousSpans[1].VideoOffsetStart != 333 || report.ContiguousSpans[2].Instruction.Segment != 0x2000 {
		t.Fatalf("span boundaries=%+v", report.ContiguousSpans)
	}
	encoded, err := json.Marshal(report)
	if err != nil || strings.Contains(string(encoded), `"value":`) || strings.Contains(string(encoded), `"0x5a"`) {
		t.Fatalf("receipt leaked a raw stored byte: %s, err=%v", encoded, err)
	}

	duplicate := newBodyIconA000Observer(rects, 10, 20, nil)
	duplicate.Observe(writes[0])
	duplicate.Observe(writes[0]) // same address and same value is still a second observed store
	if duplicate.Report().A000WriteCount != 2 || duplicate.Report().A000SequenceSHA256 == observer.Report().A000SequenceSHA256 {
		t.Fatal("same-value write count/order did not affect the A000 digest")
	}

	otherWriter := writes[0]
	otherWriter.CS++
	changedWriter := newBodyIconA000Observer(rects, 10, 20, nil)
	changedWriter.Observe(otherWriter)
	if changedWriter.Report().RectIntersectingSequenceSHA256 == duplicate.Report().RectIntersectingSequenceSHA256 {
		t.Fatal("writer identity change did not affect the rect digest")
	}

	continuous := newBodyIconA000Observer(rects, 10, 20, nil)
	continuous.Observe(writes[0])
	continuous.Observe(writes[1])
	if len(continuous.Report().ContiguousSpans) != 1 {
		t.Fatalf("consecutive offsets did not coalesce: %+v", continuous.Report().ContiguousSpans)
	}
	discontinuous := newBodyIconA000Observer(rects, 10, 20, nil)
	discontinuous.Observe(writes[0])
	writes[1].Offset = 332
	discontinuous.Observe(writes[1])
	if len(discontinuous.Report().ContiguousSpans) != 2 || discontinuous.Report().RectIntersectingSequenceSHA256 == continuous.Report().RectIntersectingSequenceSHA256 {
		t.Fatal("offset gap was not preserved by span boundaries and sequence digest")
	}
	orderChanged := newBodyIconA000Observer(rects, 10, 20, nil)
	orderChanged.Observe(writes[1])
	orderChanged.Observe(writes[0])
	if orderChanged.Report().RectIntersectingSequenceSHA256 == continuous.Report().RectIntersectingSequenceSHA256 {
		t.Fatal("write order change did not affect the rect digest")
	}
}

func TestValidateBodyIconAfterStepsRequiresUniqueInRangeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		steps   []uint64
		from    uint64
		until   uint64
		valid   bool
	}{
		{"none", true, nil, 100, 200, true},
		{"valid", true, []uint64{110, 120}, 100, 200, true},
		{"without observer", false, []uint64{110}, 100, 200, false},
		{"before range", true, []uint64{99}, 100, 200, false},
		{"at end", true, []uint64{200}, 100, 200, false},
		{"duplicate", true, []uint64{110, 110}, 100, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateBodyIconAfterSteps(tc.enabled, tc.steps, tc.from, tc.until) == nil; got != tc.valid {
				t.Fatalf("valid=%v, want %v", got, tc.valid)
			}
		})
	}
}
