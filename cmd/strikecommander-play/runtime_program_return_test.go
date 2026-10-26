//go:build frontendlinked

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/session"
)

func returnFixture001(t *testing.T) ([]frontendProgram, []byte) {
	t.Helper()
	// Independent synthetic MZ: 32-byte header, 1024-byte image, true entry300h.
	// Relocation at image102h: 1234h + load0220h = 1454h at PSP0210h.
	raw := make([]byte, 32+1024)
	copy(raw, "MZ")
	for at, value := range map[int]uint16{2: 32, 4: 3, 6: 1, 8: 2, 20: 0x300, 24: 28, 28: 0x102} {
		binary.LittleEndian.PutUint16(raw[at:], value)
	}
	for i := 0; i < 1024; i++ {
		raw[32+i] = byte(17*i + 3)
	}
	binary.LittleEndian.PutUint16(raw[32+0x102:], 0x1234)
	p, e := parseFrontendProgram("SCCD.EXE", raw)
	if e != nil || p.entry != 0x300 {
		t.Fatal("original MZ entry metadata changed", p.entry, e)
	}
	actual := append([]byte(nil), raw[32:]...)
	actual[0x102], actual[0x103] = 0x54, 0x14
	for i := 0x300; i < 0x340; i++ {
		actual[i] = 0xd7 // old startup entry is reused; first320 remains intact.
	}
	library := []frontendProgram{p}
	for i, name := range []string{"MENU.EXE", "OPTTEST.EXE", "MKTERR.EXE", "STRIKE.EXE", "TOPTTST1.EXE"} {
		library = append(library, frontendProgram{name: name, image: bytes.Repeat([]byte{byte(0x80 + i)}, 1024), entry: uint32(320 + 16*i)})
	}
	return library, actual
}

func TestSCCDResidentReturn001(t *testing.T) {
	library, actual := returnFixture001(t)
	for _, mode := range []string{"boot", "runtime"} {
		t.Run(mode, func(t *testing.T) {
			memory := &frontendBootMemory{psp: 0x210, image: actual}
			var requests []session.BootPSPRange
			read := func(r []session.BootPSPRange) (session.BootPSPReceipt, error) {
				requests = r
				return memory.ReadBootPSP(r)
			}
			var name string
			var e error
			if mode == "boot" {
				name, e = identifyFrontendBootProgram(returnReader001{read}, library)
			} else {
				name, e = identifyFrontendProgram(read, library)
			}
			if e != nil || name != "SCCD.EXE" {
				t.Fatalf("SCCD_RESIDENT_RETURN mode=%s name=%q error=%v", mode, name, e)
			}
			if memory.calls != 1 || len(requests) != 12 || library[0].entry != 0x300 {
				t.Fatal("batch count or original entry metadata changed")
			}
			total := 0
			for i, p := range library {
				first, second := requests[2*i], requests[2*i+1]
				want := uint32(0x100) + p.entry
				if i == 0 {
					want = 0x200 // PSP-relative; not the original MZ entry.
				}
				if first.Offset != 0x100 || len(first.Dst) != 256 || second.Offset != want || len(second.Dst) != 64 {
					t.Fatalf("profile %s: %+v %+v", p.name, first, second)
				}
				total += len(first.Dst) + len(second.Dst)
			}
			if total != 1920 {
				t.Fatal("changed total batch bytes", total)
			}
		})
	}
}

type returnReader001 struct {
	read func([]session.BootPSPRange) (session.BootPSPReceipt, error)
}

func (r returnReader001) ReadBootPSP(v []session.BootPSPRange) (session.BootPSPReceipt, error) {
	return r.read(v)
}

func TestSCCDResidentRejects001(t *testing.T) {
	library, actual := returnFixture001(t)
	for _, at := range []int{0, 255, 256, 258, 319} {
		memory := &frontendBootMemory{psp: 0x210, image: append([]byte(nil), actual...)}
		memory.image[at] ^= 1
		if _, e := identifyFrontendProgram(memory.ReadBootPSP, library); e == nil {
			t.Fatal("accepted changed resident byte", at)
		}
	}
	memory := &frontendBootMemory{psp: 0x210, image: make([]byte, 1024)}
	if _, e := identifyFrontendProgram(memory.ReadBootPSP, library); e == nil {
		t.Fatal("accepted unknown image")
	}
	memory.image = actual
	if _, e := identifyFrontendProgram(memory.ReadBootPSP, []frontendProgram{library[0], library[0]}); e == nil {
		t.Fatal("accepted ambiguous resident image")
	}
	// A different candidate's byte-exact image must still classify by its own
	// unchanged entry profile, never by a name guessed from the old key.
	memory.image = library[3].image
	if name, e := identifyFrontendProgram(memory.ReadBootPSP, library); e != nil || name != "MKTERR.EXE" {
		t.Fatal("other candidate profile changed", name, e)
	}
	sentinel := errors.New("physical batch read denied")
	memory.err = sentinel
	if _, e := identifyFrontendProgram(memory.ReadBootPSP, library); !errors.Is(e, sentinel) {
		t.Fatal("physical error hidden", e)
	}
	for _, p := range []frontendProgram{
		{name: "SCCD.EXE", image: make([]byte, 319), entry: 0},
		{name: "SCCD.EXE", image: make([]byte, 1024), entry: 1000},
		{name: "SCCD.EXE", image: make([]byte, 1024), entry: ^uint32(0)},
		{name: "MENU.EXE", image: make([]byte, 1024), entry: ^uint32(0)},
	} {
		calls := 0
		read := func([]session.BootPSPRange) (session.BootPSPReceipt, error) {
			calls++
			return session.BootPSPReceipt{PSP: 0x210}, nil
		}
		if _, e := identifyFrontendProgram(read, []frontendProgram{p}); e == nil || calls != 0 {
			t.Fatal("invalid original entry or resident window reached I/O", p.entry, len(p.image), calls, e)
		}
	}
	// MZ with valid old entry0 and 300-byte body is short for the new64 window.
	raw := make([]byte, 32+300)
	copy(raw, "MZ")
	for at, value := range map[int]uint16{2: 332, 4: 1, 8: 2} {
		binary.LittleEndian.PutUint16(raw[at:], value)
	}
	if _, e := parseFrontendProgram("SCCD.EXE", raw); e == nil {
		t.Fatal("accepted short SCCD resident range")
	}
}

func TestSCCDReturnTracker001(t *testing.T) {
	library, actual := returnFixture001(t)
	tracker := frontendProgramTracker{library: library}
	menu := &frontendBootMemory{psp: 0x231, image: library[1].image}
	if e := tracker.observe(0x231, 23, menu.ReadBootPSP); e != nil || tracker.name != "MENU.EXE" {
		t.Fatal("MENU baseline", e)
	}
	resident := &frontendBootMemory{psp: 0x210, image: actual}
	if e := tracker.observe(0x210, 23, resident.ReadBootPSP); e != nil || tracker.name != "SCCD.EXE" || tracker.psp != 0x210 || tracker.loadCount != 23 {
		t.Fatal("SCCD_RESIDENT_RETURN same-load parent return", e)
	}
	if e := tracker.observe(0x210, 23, resident.ReadBootPSP); e != nil || resident.calls != 1 {
		t.Fatal("unchanged key re-read", e)
	}
	before := tracker
	resident.image = make([]byte, 1024)
	if e := tracker.observe(0x210, 24, resident.ReadBootPSP); e == nil || !reflect.DeepEqual(tracker, before) {
		t.Fatal("unknown new load committed tracker state", e)
	}
	resident.image = actual
	if e := tracker.observe(0x211, 24, resident.ReadBootPSP); e == nil || !reflect.DeepEqual(tracker, before) {
		t.Fatal("PSP mismatch accepted or committed", e)
	}
	resident.err = errors.New("read failed")
	if e := tracker.observe(0x210, 24, resident.ReadBootPSP); !errors.Is(e, resident.err) || !reflect.DeepEqual(tracker, before) {
		t.Fatal("read failure committed tracker state", e)
	}
}

func TestSCCDResidentPhysicalWitness001(t *testing.T) {
	root, path := os.Getenv("SC_FRONTEND_EXE_ROOT"), os.Getenv("SC_R8_RETURN_WITNESS")
	if root == "" || path == "" {
		t.Skip("explicit immutable EXEs and physical008 witness required; no VM")
	}
	library, e := loadFrontendPrograms(root)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(path)
	if e != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != "044ffe9174a7ff4b9e90e313aee21bcfcb0b7801bc7a4676997b5f7e1cf7234e" {
		t.Fatal("008 witness identity", e)
	}
	var witness struct {
		Evidence []struct {
			At         struct{ PSP uint16 }
			Candidates []struct {
				Entry                     uint32
				ActualPrefix, ActualEntry string
			}
			Resident512 *struct{ Actual string }
		}
	}
	if e = json.Unmarshal(raw, &witness); e != nil || len(witness.Evidence) == 0 {
		t.Fatal("witness decode", e)
	}
	last := witness.Evidence[len(witness.Evidence)-1]
	if last.At.PSP != 0x100 || last.Resident512 == nil || len(last.Candidates) != 6 {
		t.Fatal("witness ranges incomplete")
	}
	memory := make(map[uint32]byte)
	add := func(offset uint32, encoded string) {
		b, e := hex.DecodeString(encoded)
		if e != nil {
			t.Fatal(e)
		}
		for i, value := range b {
			at := offset + uint32(i)
			if old, found := memory[at]; found && old != value {
				t.Fatal("inconsistent overlapping physical reads", at)
			}
			memory[at] = value
		}
	}
	for _, c := range last.Candidates {
		add(0x100, c.ActualPrefix)
		add(0x100+c.Entry, c.ActualEntry)
	}
	add(0x100, last.Resident512.Actual)
	read := func(ranges []session.BootPSPRange) (session.BootPSPReceipt, error) {
		for _, r := range ranges {
			for i := range r.Dst {
				value, found := memory[r.Offset+uint32(i)]
				if !found {
					return session.BootPSPReceipt{}, errors.New("requested byte absent from frozen physical witness")
				}
				r.Dst[i] = value
			}
		}
		return session.BootPSPReceipt{PSP: last.At.PSP}, nil
	}
	if name, e := identifyFrontendProgram(read, library); e != nil || name != "SCCD.EXE" || library[0].entry != 0x1840 {
		t.Fatalf("SCCD_RESIDENT_RETURN_WITNESS name=%s entry=%x error=%v", name, library[0].entry, e)
	}
}
