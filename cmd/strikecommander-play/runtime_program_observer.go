//go:build frontendlinked

package main

import (
	"fmt"

	sc "github.com/wicanr2/dosgolem/apps/strikecommander"
	"github.com/wicanr2/dosgolem/session"
)

// This private bridge observes 465's invalidation scalars. Neither scalar is a
// program identity. Every new (PSP, LoadCount) must pass the same physical byte
// classification as 464. A new bridge starts without a count baseline.
type frontendProgramTracker struct {
	library   []frontendProgram
	name      string
	baseline  bool
	psp       uint16
	loadCount uint64
}

func (t *frontendProgramTracker) sameKey(psp uint16, count uint64) bool {
	return t.baseline && t.psp == psp && t.loadCount == count
}

func (t *frontendProgramTracker) observe(psp uint16, count uint64, read func([]session.BootPSPRange) (session.BootPSPReceipt, error)) error {
	if t.sameKey(psp, count) {
		return nil
	}
	name, e := identifyFrontendProgram(func(r []session.BootPSPRange) (session.BootPSPReceipt, error) {
		receipt, e := read(r)
		if e == nil && receipt.PSP != psp {
			return session.BootPSPReceipt{}, fmt.Errorf("016 R8 同回呼 PSP 身分不同")
		}
		return receipt, e
	}, t.library)
	if e != nil {
		return e
	}
	t.name, t.psp, t.loadCount, t.baseline = name, psp, count, true
	return nil
}

// Reverified byte classifications are the only program notifications supplied
// to the app's initialized 016a policy; scalars themselves never name a program.
type frontendPlayerObserver struct {
	*sc.FrontendRuntime
	program frontendProgramTracker
}

func (o *frontendPlayerObserver) BeforeStep(v session.StepView) error {
	psp, count := v.CurrentPSP(), v.LoadCount()
	// Keep the normal instruction path free from interface boxing and closures.
	// The temporary method value exists only when the bounded key changes.
	if !o.program.sameKey(psp, count) {
		if e := o.program.observe(psp, count, v.ReadPSP); e != nil {
			return e
		}
		p, e := frontendMenuProgram(o.program.name)
		if e != nil {
			return e
		}
		if e = o.FrontendRuntime.ObserveMenuProgram(p); e != nil {
			return e
		}
	}
	return o.FrontendRuntime.BeforeStep(v)
}
