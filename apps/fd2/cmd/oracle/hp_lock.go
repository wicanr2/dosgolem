package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// Spec 430: explicit test injection, never an original-game rule.
type fd2HPLockTarget struct {
	Index    int  `json:"index"`
	Camp     int  `json:"camp"`
	Identity int  `json:"identity"`
	HP       *int `json:"hp,omitempty"`
}

func (t *fd2HPLockTarget) UnmarshalJSON(raw []byte) error {
	var v struct {
		Index, Camp, Identity *int
		HP                    *int
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	if v.Index == nil || v.Camp == nil || v.Identity == nil {
		return fmt.Errorf("hp_lock requires index, camp and identity")
	}
	*t = fd2HPLockTarget{Index: *v.Index, Camp: *v.Camp, Identity: *v.Identity, HP: v.HP}
	return nil
}

type fd2HPLockCell struct {
	Index    int    `json:"index"`
	Camp     int    `json:"camp"`
	Identity int    `json:"identity"`
	HP       uint16 `json:"hp"`
}

type fd2HPLocks struct {
	cells   []fd2HPLockCell
	updates int
	writes  map[int]int
}

func fd2HPLockRecord(mem []byte, base, count uint32, index int) ([]byte, error) {
	if count == 0 || count > 128 || index < 0 || uint32(index) >= count ||
		uint64(base)+uint64(count)*80 > uint64(len(mem)) {
		return nil, fmt.Errorf("hp_lock runtime array unavailable")
	}
	off := int(base) + 80*index
	return mem[off : off+80], nil
}

func (l *fd2HPLocks) update(mem []byte, base, count uint32, targets []fd2HPLockTarget) error {
	if len(targets) > 4 {
		return fmt.Errorf("hp_lock accepts at most four targets")
	}
	var next []fd2HPLockCell
	seen := map[int]bool{}
	for _, target := range targets {
		if seen[target.Index] || (target.Camp != 0 && target.Camp != 2) || target.Identity < 0 || target.Identity > 255 {
			return fmt.Errorf("hp_lock invalid or duplicate binding")
		}
		seen[target.Index] = true
		r, err := fd2HPLockRecord(mem, base, count, target.Index)
		if err != nil {
			return err
		}
		hp := binary.LittleEndian.Uint16(r[0x40:])
		if int(r[6]) != target.Camp || int(r[8]) != target.Identity || r[5]&1 != 0 || hp == 0 || hp > 32767 {
			return fmt.Errorf("hp_lock binding or active HP disagrees with record %d", target.Index)
		}
		for _, prior := range l.cells {
			if prior.Index == target.Index && prior.Camp == target.Camp && prior.Identity == target.Identity {
				hp = prior.HP
			}
		}
		if target.HP != nil {
			if *target.HP < 1 || *target.HP > 32767 {
				return fmt.Errorf("hp_lock HP must be 1..32767")
			}
			hp = uint16(*target.HP)
		}
		next = append(next, fd2HPLockCell{target.Index, target.Camp, target.Identity, hp})
	}
	l.cells = next
	l.updates++
	return nil
}

func (l *fd2HPLocks) apply(mem []byte, base, count uint32) error {
	// Preflight every binding before writing any record. Re-resolve after append.
	for _, cell := range l.cells {
		r, err := fd2HPLockRecord(mem, base, count, cell.Index)
		if err != nil {
			return err
		}
		if int(r[6]) != cell.Camp || int(r[8]) != cell.Identity || r[5]&1 != 0 {
			return fmt.Errorf("hp_lock active binding changed at record %d", cell.Index)
		}
	}
	for _, cell := range l.cells {
		r, _ := fd2HPLockRecord(mem, base, count, cell.Index)
		if binary.LittleEndian.Uint16(r[0x40:]) != cell.HP {
			binary.LittleEndian.PutUint16(r[0x40:], cell.HP)
			if l.writes == nil {
				l.writes = map[int]int{}
			}
			l.writes[cell.Index]++
		}
	}
	return nil
}

func (l *fd2HPLocks) protects(index int) bool {
	for _, cell := range l.cells {
		if cell.Index == index {
			return true
		}
	}
	return false
}

func (l *fd2HPLocks) enemyLocks() int {
	n := 0
	for _, cell := range l.cells {
		if cell.Camp == 0 {
			n++
		}
	}
	return n
}

func (l *fd2HPLocks) report(enabled bool) map[string]any {
	active := append([]fd2HPLockCell{}, l.cells...)
	return map[string]any{"enabled": enabled, "updates": l.updates, "active": active,
		"writes_by_index": l.writes, "method": "explicit modified path; record +0x40 restored before each guest instruction"}
}
