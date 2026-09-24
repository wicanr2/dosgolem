//go:build draft_session_owner_view

package session

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/host"
)

// This file is an intentionally excluded DRAFT contract prototype.  It does
// not add a production Owner API and must not be read as a READY frontend
// connection.  It makes the proposed value-only view and its source-version
// boundary reviewable before a later spec change authorizes implementation.
type draftOwnerView struct {
	Phase            Phase
	Panel            host.PanelState
	Layout           host.MouseLayout
	HasLayout        bool
	Epoch            uint64
	SourceGeneration uint64
}

type draftViewOwner struct {
	panel      *host.PanelController
	phase      Phase
	layout     host.MouseLayout
	hasLayout  bool
	epoch      uint64
	generation uint64
	pending    bool
	paused     bool
	firstFault error
	closed     bool
	closer     *countingCloser
	panelReads uint64
	runCalls   uint64
	dosCalls   uint64 // synthetic only: proves overflow rejects before output.
}

func newDraftViewOwner(t *testing.T, layout host.MouseLayout) *draftViewOwner {
	t.Helper()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	return &draftViewOwner{panel: panel, phase: PhaseBooting, layout: layout,
		hasLayout: layout != (host.MouseLayout{}), closer: &countingCloser{}}
}

// view is the new 019 DRAFT candidate: only a Running owner whose private
// layout is normalized to the current panel may hand its input-capture values
// to a future sealed frontend. All other lifecycle observations use Status.
func (d *draftViewOwner) view() (draftOwnerView, error) {
	if d.phase != PhaseRunning {
		return draftOwnerView{}, fmt.Errorf("DRAFT View unavailable in phase %v; use Status", d.phase)
	}
	if !d.hasLayout {
		err := errors.New("DRAFT View unavailable without private layout")
		d.fail(err)
		return draftOwnerView{}, err
	}
	d.panelReads++
	state, err := d.panel.Snapshot()
	if err != nil {
		d.fail(fmt.Errorf("DRAFT owner view panel snapshot: %w", err))
		return draftOwnerView{}, err
	}
	projected, changed, err := host.ProjectPresentationLayout(d.layout, state)
	if err != nil || changed || projected != d.layout {
		if err == nil {
			err = errors.New("DRAFT View private layout is not normalized to panel")
		}
		d.fail(err)
		return draftOwnerView{}, err
	}
	return d.snapshot(state), nil
}

func (d *draftViewOwner) snapshot(state host.PanelState) draftOwnerView {
	return draftOwnerView{Phase: d.phase, Panel: state, Layout: d.layout,
		HasLayout: d.hasLayout, Epoch: d.epoch, SourceGeneration: d.generation}
}

// change is the DRAFT source-generation rule: each successful observable
// state mutation increments exactly once; failed/closed terminal handling
// does not wrap or fabricate a later generation.
func (d *draftViewOwner) change(apply func()) error {
	if d.generation == math.MaxUint64 {
		d.fail(errors.New("DRAFT owner source generation overflow"))
		return d.firstFault
	}
	apply()
	d.generation++
	return nil
}

func (d *draftViewOwner) start() error {
	if d.phase != PhaseBooting {
		return d.fail(errors.New("DRAFT start requires Booting"))
	}
	return d.change(func() { d.phase = PhaseRunning })
}

func (d *draftViewOwner) deliver(event host.PanelEvent) error {
	if d.phase != PhaseRunning || d.pending {
		return d.fail(errors.New("DRAFT Deliver requires idle Running"))
	}
	base, err := d.panel.Snapshot()
	if err != nil {
		return d.fail(err)
	}
	next, route, err := host.PlanPanelRoute(base, event)
	if err != nil {
		return d.fail(err)
	}
	nextLayout := d.layout
	if d.hasLayout {
		nextLayout, _, err = host.ProjectPresentationLayout(d.layout, next)
		if err != nil {
			return d.fail(err)
		}
	}
	return d.change(func() {
		state, _, routeErr := d.panel.Route(event)
		if routeErr != nil || state != next {
			panic("DRAFT owner view prototype: planned panel route diverged")
		}
		d.layout = nextLayout
		d.epoch++
		d.pending = true
		d.paused = base.Open || next.Open || route.ConsumedByHost
	})
}

func (d *draftViewOwner) advancePaused() error {
	if d.phase != PhaseRunning || !d.pending || !d.paused {
		return d.fail(errors.New("DRAFT Advance requires paused pending turn"))
	}
	return d.change(func() { d.pending, d.paused = false, false })
}

// advanceRunning is a synthetic no-observer step; it checks the route source
// version before its stand-in for an original machine step.
func (d *draftViewOwner) advanceRunning(exits bool) error {
	if d.phase != PhaseRunning || !d.pending || d.paused {
		return d.fail(errors.New("DRAFT Advance requires running pending turn"))
	}
	return d.change(func() {
		d.runCalls++
		d.pending = false
		if exits {
			d.phase = PhaseStopped
		}
	})
}

// deliverDOS is a synthetic stand-in for an already-prepared executable input
// batch. It exists only to prove the candidate's generation-overflow check is
// before the first DOS action; it is not a DOS or Owner implementation.
func (d *draftViewOwner) deliverDOS() error {
	if d.phase != PhaseRunning || d.pending {
		return d.fail(errors.New("DRAFT DOS Deliver requires idle Running"))
	}
	return d.change(func() {
		d.epoch++
		d.pending = true
		d.paused = false
		d.dosCalls++
	})
}

func (d *draftViewOwner) prepareCaptured(view draftOwnerView) error {
	if d.phase != PhaseRunning || !d.hasLayout || view.SourceGeneration != d.generation {
		return errors.New("DRAFT captured View token is stale")
	}
	state, err := d.panel.Snapshot()
	if err != nil {
		return err
	}
	if state != view.Panel || d.layout != view.Layout || d.epoch != view.Epoch {
		return errors.New("DRAFT captured View values no longer match owner")
	}
	return nil
}

// deliverCapturedDOS is the owner boundary around pure Prepare. A stale or
// faulting capture fails the owner before its first synthetic DOS action.
func (d *draftViewOwner) deliverCapturedDOS(view draftOwnerView) error {
	if err := d.prepareCaptured(view); err != nil {
		return d.fail(err)
	}
	return d.change(func() {
		d.epoch++
		d.pending = true
		d.dosCalls++
	})
}

func (d *draftViewOwner) fail(err error) error {
	if d.phase == PhaseFailed {
		return d.firstFault
	}
	if d.phase == PhaseClosed {
		return err
	}
	d.firstFault = err
	d.phase = PhaseFailed
	_ = d.close()
	return err
}

func (d *draftViewOwner) close() error {
	if !d.closed {
		d.closed = true
		d.closer.Close()
		if d.phase != PhaseFailed {
			d.phase = PhaseClosed
		}
	}
	return nil
}

func (d *draftViewOwner) status() Status {
	return Status{Phase: d.phase, FirstFault: d.firstFault}
}

func draftLayout(epoch uint64, scale host.OutputScale, open bool) host.MouseLayout {
	chrome := 18 * int(scale)
	if open {
		chrome = 92 * int(scale)
	}
	return host.MouseLayout{Epoch: epoch, Scale: scale, ChromeHeight: chrome,
		Canvas: host.Canvas{Width: 320, Height: 200}, FrameWidth: 320 * int(scale),
		FrameHeight: chrome + 200*int(scale), PanelOpen: open}
}

func TestDraftOwnerValueViewRunningOnlyAndValueOnly(t *testing.T) {
	d := newDraftViewOwner(t, host.MouseLayout{})
	if got, err := d.view(); err == nil || got != (draftOwnerView{}) {
		t.Fatalf("Booting View = (%+v, %v), want zero View error", got, err)
	}
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	if got, err := d.view(); err == nil || got != (draftOwnerView{}) || d.phase != PhaseFailed || d.closer.calls != 1 {
		t.Fatalf("Running without layout View = (%+v, %v), phase=%v close=%d; want fail/close", got, err, d.phase, d.closer.calls)
	}
	d = newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	got, err := d.view()
	if err != nil || got.Phase != PhaseRunning || !got.HasLayout || got.Layout != d.layout || got.Epoch != 0 || got.SourceGeneration != 1 {
		t.Fatalf("Running DRAFT View = (%+v, %v)", got, err)
	}
	for i := 0; i < reflect.TypeOf(draftOwnerView{}).NumField(); i++ {
		field := reflect.TypeOf(draftOwnerView{}).Field(i)
		if field.Type.Kind() == reflect.Pointer || field.Type.Kind() == reflect.Interface {
			t.Fatalf("DRAFT view field %s leaks mutable target type %v", field.Name, field.Type)
		}
	}
}

func TestDraftOwnerValueViewRejectsNoncanonicalRunningLayout(t *testing.T) {
	layout := draftLayout(1, host.OutputScale2, false)
	layout.FrameHeight++
	d := newDraftViewOwner(t, layout)
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	got, err := d.view()
	if err == nil || got != (draftOwnerView{}) || d.phase != PhaseFailed || d.closer.calls != 1 {
		t.Fatalf("noncanonical Running View = (%+v, %v), phase=%v close=%d", got, err, d.phase, d.closer.calls)
	}
}

func TestDraftOwnerValueViewProjects2xTo3xTo2xAndCountsSources(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	check := func(wantGeneration, wantEpoch uint64, want host.MouseLayout, open bool) {
		t.Helper()
		got, err := d.view()
		if err != nil || got.Phase != PhaseRunning || got.SourceGeneration != wantGeneration || got.Epoch != wantEpoch ||
			!got.HasLayout || got.Layout != want || got.Panel.Open != open {
			t.Fatalf("DRAFT view = (%+v, %v), want generation=%d epoch=%d layout=%+v open=%t", got, err, wantGeneration, wantEpoch, want, open)
		}
	}
	check(1, 0, draftLayout(1, host.OutputScale2, false), false)
	deliverPause := func(event host.PanelEvent, generation, epoch uint64, layout host.MouseLayout, open bool) {
		t.Helper()
		if err := d.deliver(event); err != nil {
			t.Fatal(err)
		}
		check(generation, epoch, layout, open)
		if err := d.advancePaused(); err != nil {
			t.Fatal(err)
		}
		check(generation+1, epoch, layout, open)
	}

	deliverPause(host.PanelEvent{Kind: host.PanelEventOpen}, 2, 1, draftLayout(2, host.OutputScale2, true), true)
	deliverPause(host.PanelEvent{Kind: host.PanelEventSelectScale, Scale: host.OutputScale3}, 4, 2, draftLayout(2, host.OutputScale2, true), true)
	deliverPause(host.PanelEvent{Kind: host.PanelEventApply}, 6, 3, draftLayout(3, host.OutputScale3, false), false)
	deliverPause(host.PanelEvent{Kind: host.PanelEventOpen}, 8, 4, draftLayout(4, host.OutputScale3, true), true)
	deliverPause(host.PanelEvent{Kind: host.PanelEventSelectScale, Scale: host.OutputScale2}, 10, 5, draftLayout(4, host.OutputScale3, true), true)
	deliverPause(host.PanelEvent{Kind: host.PanelEventApply}, 12, 6, draftLayout(5, host.OutputScale2, false), false)
	// Cancel retains active 2× while closing the host panel with a newer layout.
	deliverPause(host.PanelEvent{Kind: host.PanelEventOpen}, 14, 7, draftLayout(6, host.OutputScale2, true), true)
	deliverPause(host.PanelEvent{Kind: host.PanelEventSelectScale, Scale: host.OutputScale3}, 16, 8, draftLayout(6, host.OutputScale2, true), true)
	deliverPause(host.PanelEvent{Kind: host.PanelEventCancel}, 18, 9, draftLayout(7, host.OutputScale2, false), false)
}

func TestDraftOwnerValueViewSnapshotFaultReturnsZeroAndThenOnlyStatus(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	d.panel = nil // host.PanelController documents this as a Snapshot error.
	got, err := d.view()
	if err == nil || got != (draftOwnerView{}) || d.phase != PhaseFailed || d.firstFault == nil || d.closer.calls != 1 || d.generation != 1 || d.panelReads != 1 {
		t.Fatalf("faulting View = (%+v, %v), phase=%v fault=%v generation=%d close=%d", got, err, d.phase, d.firstFault, d.generation, d.closer.calls)
	}
	if got, err := d.view(); err == nil || got != (draftOwnerView{}) || d.closer.calls != 1 || d.panelReads != 1 {
		t.Fatalf("Failed View = (%+v, %v), want zero View and no re-close", got, err)
	}
	status := d.status()
	if status.Phase != PhaseFailed || !errors.Is(status.FirstFault, d.firstFault) {
		t.Fatalf("Failed Status = %+v, want latched first fault", status)
	}
}

func TestDraftOwnerValueViewOpenCancelRejectsOldTokenByGeneration(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	old, err := d.view()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.prepareCaptured(old); err != nil {
		t.Fatalf("fresh View token rejected: %v", err)
	}
	if err := d.deliver(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	if err := d.advancePaused(); err != nil {
		t.Fatal(err)
	}
	if err := d.deliver(host.PanelEvent{Kind: host.PanelEventCancel}); err != nil {
		t.Fatal(err)
	}
	if err := d.advancePaused(); err != nil {
		t.Fatal(err)
	}
	now, err := d.view()
	if err != nil || now.Panel != old.Panel || now.SourceGeneration == old.SourceGeneration {
		t.Fatalf("Open→Cancel should restore panel yet change token: old=%+v now=%+v err=%v", old, now, err)
	}
	if err := d.deliverCapturedDOS(old); err == nil || d.phase != PhaseFailed || d.closer.calls != 1 || d.dosCalls != 0 {
		t.Fatalf("Open→Cancel ABA old View token = err=%v phase=%v close=%d DOS=%d; want pre-action fail/close", err, d.phase, d.closer.calls, d.dosCalls)
	}
}

func TestDraftOwnerValueViewFreshTokenCommitsOnlyAtOwnerBoundary(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	view, err := d.view()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.prepareCaptured(view); err != nil || d.dosCalls != 0 || d.epoch != 0 {
		t.Fatalf("pure Prepare = err=%v DOS=%d epoch=%d", err, d.dosCalls, d.epoch)
	}
	if err := d.deliverCapturedDOS(view); err != nil || d.dosCalls != 1 || d.epoch != 1 || d.phase != PhaseRunning {
		t.Fatalf("owner Deliver = err=%v DOS=%d epoch=%d phase=%v", err, d.dosCalls, d.epoch, d.phase)
	}
}

func TestDraftOwnerValueViewValueDriftFailsBeforeOwnerDOSCommit(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	view, err := d.view()
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a private layout drift without a matching generation bump.
	d.layout.FrameHeight++
	if err := d.deliverCapturedDOS(view); err == nil || d.phase != PhaseFailed ||
		d.closer.calls != 1 || d.dosCalls != 0 || d.epoch != 0 {
		t.Fatalf("drifted owner Deliver = err=%v phase=%v close=%d DOS=%d epoch=%d", err, d.phase, d.closer.calls, d.dosCalls, d.epoch)
	}
}

func TestDraftOwnerValueViewPrepareSnapshotFaultFailsAndClosesOnce(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	view, err := d.view()
	if err != nil {
		t.Fatal(err)
	}
	d.panel = nil
	if err := d.deliverCapturedDOS(view); err == nil || d.phase != PhaseFailed || d.closer.calls != 1 || d.firstFault == nil || d.dosCalls != 0 || d.epoch != 0 {
		t.Fatalf("Prepare snapshot fault = err=%v phase=%v close=%d fault=%v DOS=%d", err, d.phase, d.closer.calls, d.firstFault, d.dosCalls)
	}
}

func TestDraftOwnerValueViewRunningAdvanceAndStopGeneration(t *testing.T) {
	for _, tc := range []struct {
		name  string
		exits bool
		phase Phase
	}{
		{"running", false, PhaseRunning},
		{"stopped", true, PhaseStopped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
			if err := d.start(); err != nil {
				t.Fatal(err)
			}
			if err := d.deliverDOS(); err != nil {
				t.Fatal(err)
			}
			before := d.generation
			if err := d.advanceRunning(tc.exits); err != nil {
				t.Fatal(err)
			}
			if d.generation != before+1 || d.phase != tc.phase || d.pending || d.runCalls != 1 {
				t.Fatalf("Advance outcome generation=%d phase=%v pending=%t runCalls=%d", d.generation, d.phase, d.pending, d.runCalls)
			}
			if tc.exits {
				if got, err := d.view(); err == nil || got != (draftOwnerView{}) {
					t.Fatalf("Stopped View = (%+v, %v), want unavailable", got, err)
				}
			}
		})
	}
}

func TestDraftOwnerValueViewRunningAdvanceOverflowBeforeStep(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	if err := d.deliverDOS(); err != nil {
		t.Fatal(err)
	}
	d.generation = math.MaxUint64
	if err := d.advanceRunning(false); err == nil || d.runCalls != 0 || d.phase != PhaseFailed || d.closer.calls != 1 || d.pending != true {
		t.Fatalf("overflow Advance = err=%v runCalls=%d phase=%v close=%d pending=%t", err, d.runCalls, d.phase, d.closer.calls, d.pending)
	}
}

func TestDraftOwnerValueViewAdvanceConsumeIncrementsGeneration(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	if err := d.deliver(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	before, err := d.view()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.advancePaused(); err != nil {
		t.Fatal(err)
	}
	after, err := d.view()
	if err != nil || after.Epoch != before.Epoch || after.SourceGeneration != before.SourceGeneration+1 {
		t.Fatalf("Advance consume = before=%+v after=%+v err=%v", before, after, err)
	}
}

func TestDraftOwnerValueViewSameValuesABATokenFailsClosed(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	if err := d.start(); err != nil {
		t.Fatal(err)
	}
	if err := d.deliver(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	old, err := d.view()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.advancePaused(); err != nil {
		t.Fatal(err)
	}
	now, err := d.view()
	if err != nil || old.Panel != now.Panel || old.Layout != now.Layout || old.Epoch != now.Epoch ||
		old.SourceGeneration == now.SourceGeneration {
		t.Fatalf("same-value ABA precondition: old=%+v now=%+v err=%v", old, now, err)
	}
	if err := d.deliverCapturedDOS(old); err == nil || d.phase != PhaseFailed ||
		d.closer.calls != 1 || d.dosCalls != 0 || d.epoch != old.Epoch {
		t.Fatalf("same-value ABA delivery = err=%v phase=%v close=%d DOS=%d epoch=%d", err, d.phase, d.closer.calls, d.dosCalls, d.epoch)
	}
}

func TestDraftOwnerValueViewGenerationOverflowRejectsBeforeSyntheticDOS(t *testing.T) {
	d := newDraftViewOwner(t, draftLayout(1, host.OutputScale2, false))
	d.phase, d.generation = PhaseRunning, math.MaxUint64
	before, err := d.panel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.deliverDOS(); err == nil {
		t.Fatal("generation overflow unexpectedly accepted synthetic DOS delivery")
	}
	after, err := d.panel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if after != before || d.epoch != 0 || d.dosCalls != 0 || d.layout != draftLayout(1, host.OutputScale2, false) ||
		d.phase != PhaseFailed || d.generation != math.MaxUint64 || d.closer.calls != 1 {
		t.Fatalf("overflow reached synthetic DOS or mutated DRAFT owner: panel=%+v epoch=%d dosCalls=%d layout=%+v phase=%v generation=%d close=%d", after, d.epoch, d.dosCalls, d.layout, d.phase, d.generation, d.closer.calls)
	}
}
