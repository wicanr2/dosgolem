//go:build frontendlinked

package main

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/session"
)

type frontendProgramMemory struct {
	frontendBootMemory
	count uint64
}

func (m *frontendProgramMemory) CurrentPSP() uint16 { return m.psp }
func (m *frontendProgramMemory) LoadCount() uint64  { return m.count }
func (m *frontendProgramMemory) ReadPSP(r []session.BootPSPRange) (session.BootPSPReceipt, error) {
	return m.ReadBootPSP(r)
}

func observeProgramFixture(tracker *frontendProgramTracker, m *frontendProgramMemory) error {
	return tracker.observe(m.CurrentPSP(), m.LoadCount(), m.ReadPSP)
}

func TestFrontendProgramInvalidation(t *testing.T) {
	a := frontendProgram{name: "A", image: bytes.Repeat([]byte{0x31}, 320), entry: 256}
	b := frontendProgram{name: "B", image: bytes.Repeat([]byte{0x72}, 320), entry: 256}
	m := &frontendProgramMemory{frontendBootMemory: frontendBootMemory{psp: 0x210, image: a.image}, count: 19}
	tracker := frontendProgramTracker{library: []frontendProgram{a, b}}
	if e := observeProgramFixture(&tracker, m); e != nil || tracker.name != "A" || m.calls != 1 {
		t.Fatal("first callback must classify its real nonzero load baseline", e)
	}
	if e := observeProgramFixture(&tracker, m); e != nil || m.calls != 1 {
		t.Fatal("unchanged key should not reread", e)
	}
	// Reload at the same PSP. A PSP-only implementation would miss this.
	m.image, m.count = b.image, 20
	if e := observeProgramFixture(&tracker, m); e != nil || tracker.name != "B" || m.calls != 2 {
		t.Fatal("same PSP successful load was missed", e)
	}
	// Parent return can change PSP without adding a load entry.
	m.image, m.psp = a.image, 0x300
	if e := observeProgramFixture(&tracker, m); e != nil || tracker.name != "A" || m.calls != 3 {
		t.Fatal("parent return was missed", e)
	}
	// An overlay load invalidates, but it is allowed to classify as the same EXE.
	m.count++
	if e := observeProgramFixture(&tracker, m); e != nil || tracker.name != "A" || m.calls != 4 {
		t.Fatal("overlay must invalidate without inventing a new program", e)
	}
	m.image = bytes.Repeat([]byte{0x99}, 320)
	m.count++
	if e := observeProgramFixture(&tracker, m); e == nil || tracker.name != "A" || tracker.loadCount == m.count {
		t.Fatal("unknown bytes were accepted or failed key was committed")
	}
	// A different Owner has its own baseline even if its count is smaller.
	m.image, m.count = b.image, 0
	newTracker := frontendProgramTracker{library: []frontendProgram{a, b}}
	if e := observeProgramFixture(&newTracker, m); e != nil || newTracker.name != "B" {
		t.Fatal("new Owner reused the old baseline", e)
	}
}
