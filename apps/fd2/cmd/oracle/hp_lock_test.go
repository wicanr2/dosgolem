package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func hpLockFixture() []byte {
	mem := make([]byte, 800)
	for i, camp := range []byte{2, 0, 0} {
		r := mem[100+80*i:]
		r[6], r[8] = camp, byte(i)
		binary.LittleEndian.PutUint16(r[0x40:], uint16(100+i))
	}
	return mem
}

func TestHPLockCapturesRestoresRebasesAndReleases(t *testing.T) {
	mem := hpLockFixture()
	var l fd2HPLocks
	if err := l.update(mem, 100, 3, []fd2HPLockTarget{{Index: 0, Camp: 2, Identity: 0}, {Index: 2, Camp: 0, Identity: 2}}); err != nil {
		t.Fatal(err)
	}
	copy(mem[400:], mem[100:340])
	binary.LittleEndian.PutUint16(mem[400+0x40:], 0)
	if err := l.apply(mem, 400, 3); err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(mem[400+0x40:]) != 100 || l.writes[0] != 1 || !l.protects(2) || l.enemyLocks() != 1 {
		t.Fatal("lock did not follow the validated relocated records")
	}
	if err := l.update(mem, 0, 0, nil); err != nil {
		t.Fatal(err)
	}
	if l.protects(0) || l.enemyLocks() != 0 || l.updates != 2 {
		t.Fatal("release retained a target")
	}
}

func TestHPLockRejectsWholeUpdateAndWholeWriteAtomically(t *testing.T) {
	for _, targets := range [][]fd2HPLockTarget{
		{{Index: 0, Camp: 1, Identity: 0}},
		{{Index: 0, Camp: 2, Identity: 0}, {Index: 0, Camp: 2, Identity: 0}},
		{{Index: 3, Camp: 0, Identity: 3}},
		{{Index: 0, Camp: 2, Identity: 1}},
	} {
		mem := hpLockFixture()
		before := append([]byte(nil), mem...)
		var l fd2HPLocks
		if err := l.update(mem, 100, 3, targets); err == nil || len(l.cells) != 0 || !bytes.Equal(before, mem) {
			t.Fatal("invalid update changed locks or memory")
		}
	}
	mem := hpLockFixture()
	var l fd2HPLocks
	if err := l.update(mem, 100, 3, []fd2HPLockTarget{{Index: 0, Camp: 2, Identity: 0}, {Index: 2, Camp: 0, Identity: 2}}); err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint16(mem[100+0x40:], 4)
	mem[100+160+8] = 7
	before := append([]byte(nil), mem...)
	if err := l.apply(mem, 100, 3); err == nil || !bytes.Equal(before, mem) {
		t.Fatal("failed preflight partially restored HP")
	}
}

func TestHPLockOneHPAndStrictJSON(t *testing.T) {
	mem := hpLockFixture()
	var l fd2HPLocks
	one := 1
	if err := l.update(mem, 100, 3, []fd2HPLockTarget{{Index: 1, Camp: 0, Identity: 1, HP: &one}}); err != nil {
		t.Fatal(err)
	}
	if err := l.apply(mem, 100, 3); err != nil || binary.LittleEndian.Uint16(mem[180+0x40:]) != 1 {
		t.Fatal("explicit HP1 was not recorded and applied")
	}
	for _, raw := range []string{`{"index":0,"camp":2}`, `{"index":0,"camp":2,"identity":0,"unknown":1}`, `{"index":0.5,"camp":2,"identity":0}`} {
		var target fd2HPLockTarget
		if json.Unmarshal([]byte(raw), &target) == nil {
			t.Fatalf("accepted malformed binding: %s", raw)
		}
	}
	for _, hp := range []int{0, -1, 32768} {
		if l.update(mem, 100, 3, []fd2HPLockTarget{{Index: 0, Camp: 2, Identity: 0, HP: &hp}}) == nil {
			t.Fatal("accepted HP outside the positive signed-word range")
		}
	}
}
