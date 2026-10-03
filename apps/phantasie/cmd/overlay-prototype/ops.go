package main

// 證據原型：依 docs/spec/002 把畫面操作對應到 Layer 動作。不是實作。
// 目的是在規格升 READY 之前，用真實遊戲畫面檢驗反白重取色、整頁影子、視窗清除的假設。

import (
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/xlate"
)

// 映像位移（002 §3）。入口與完成（retn）。
const (
	opInvert, opInvertDone   = 0x0672, 0x073F
	opInvert2, opInvert2Done = 0x0740, 0x0799
	opSave1, opSave1Done     = 0x0D30, 0x0D4F
	opCopy12, opCopy12Done   = 0x0D50, 0x0D70
	opLoad1, opLoad1Done     = 0x0CF0, 0x0D0F
	opLoad2, opLoad2Done     = 0x0D10, 0x0D2F
	opRow24, opRow24Done     = 0x372D, 0x376D
	opInt86                  = 0x4E60
	pageSegVar               = 0x5BAA
	page1OffVar              = 0x5BAC
	page2OffVar              = 0x5BAE
	pageBytes                = 0x4000
)

func fnv64(b []byte) uint64 {
	h := uint64(14695981039346656037)
	for _, v := range b {
		h ^= uint64(v)
		h *= 1099511628211
	}
	return h
}

type opsTracker struct {
	o      *oracle.Oracle
	dg     uint16
	layer  *xlate.Layer
	fonts  map[string]*xlate.Font
	known  map[uint64][]byte
	counts map[string]int

	invRow, invCol, invW int
	loadHash             uint64
	row24Old, row24New   uint64
	verbose              bool
}

func installOps(o *oracle.Oracle, img uint16, layer *xlate.Layer, fonts map[string]*xlate.Font, verbose bool) *opsTracker {
	t := &opsTracker{o: o, dg: img + 0xC9F, layer: layer, fonts: fonts, known: map[uint64][]byte{}, counts: map[string]int{}, verbose: verbose}
	at := func(off uint16, fn func(*oracle.Oracle)) { o.OnCall(oracle.Far(img, off), fn) }
	at(opSave1Done, func(*oracle.Oracle) { t.save("save1") })
	at(opCopy12Done, func(*oracle.Oracle) { t.counts["copy12"]++ })
	at(opLoad1, func(*oracle.Oracle) { t.loadEnter(page1OffVar) })
	at(opLoad1Done, func(*oracle.Oracle) { t.loadDone("load1") })
	at(opLoad2, func(*oracle.Oracle) { t.loadEnter(page2OffVar) })
	at(opLoad2Done, func(*oracle.Oracle) { t.loadDone("load2") })
	at(opRow24, func(*oracle.Oracle) { t.row24Enter() })
	at(opRow24Done, func(*oracle.Oracle) { t.row24Done() })
	at(opInvert, func(o *oracle.Oracle) {
		t.invRow, t.invCol, t.invW = int(o.StackWord(1)), int(o.StackWord(2)), int(o.StackWord(3))
	})
	at(opInvertDone, func(*oracle.Oracle) { t.invertDone() })
	at(opInvert2, func(o *oracle.Oracle) {
		t.invRow, t.invCol, t.invW = int(o.StackWord(1)), int(o.StackWord(2)), int(o.StackWord(3))
	})
	at(opInvert2Done, func(*oracle.Oracle) { t.invertDone() })
	at(opInt86, func(o *oracle.Oracle) { t.int86(o) })
	return t
}

func (t *opsTracker) logf(f string, a ...any) {
	if t.verbose {
		fmt.Fprintf(os.Stderr, "[ops step=%d] "+f+"\n", append([]any{t.o.Steps()}, a...)...)
	}
}

func (t *opsTracker) pageSeg() uint16 { return t.o.Word(oracle.Far(t.dg, pageSegVar)) }

func (t *opsTracker) page(offVar uint16) []byte {
	return t.o.Bytes(oracle.Far(t.pageSeg(), t.o.Word(oracle.Far(t.dg, offVar))), pageBytes)
}

func (t *opsTracker) screen() []byte { return t.o.Bytes(oracle.Far(0xB800, 0), pageBytes) }

func (t *opsTracker) save(name string) {
	snap, err := t.layer.Snapshot()
	if err != nil {
		t.logf("%s: snapshot 失敗 %v", name, err)
		return
	}
	h := fnv64(t.screen())
	t.known[h] = snap
	t.counts[name]++
	t.logf("%s hash=%016x stamps=%d", name, h, len(t.layer.Stamps))
}

func (t *opsTracker) loadEnter(offVar uint16) { t.loadHash = fnv64(t.page(offVar)) }

func (t *opsTracker) loadDone(name string) {
	t.counts[name]++
	if snap, ok := t.known[t.loadHash]; ok {
		if err := t.layer.Restore(snap, t.fonts); err == nil {
			t.counts[name+"-hit"]++
			t.logf("%s hit hash=%016x stamps=%d", name, t.loadHash, len(t.layer.Stamps))
			return
		}
	}
	t.layer.Clear(0, 0, 320, 200)
	if snap, err := t.layer.Snapshot(); err == nil {
		t.known[t.loadHash] = snap // 空層是該內容的正確影子（spec 002 §4）
	}
	t.counts[name+"-miss"]++
	t.logf("%s miss hash=%016x", name, t.loadHash)
}

func (t *opsTracker) row24Enter() {
	p := append([]byte(nil), t.page(page1OffVar)...)
	t.row24Old = fnv64(p)
	copy(p[0x1E00:0x1E00+0x140], t.o.Bytes(oracle.Far(0xB800, 0x1E00), 0x140))
	copy(p[0x3E00:0x3E00+0x140], t.o.Bytes(oracle.Far(0xBA00, 0x1E00), 0x140))
	t.row24New = fnv64(p)
}

func (t *opsTracker) row24Done() {
	t.counts["row24"]++
	old, ok := t.known[t.row24Old]
	if !ok {
		t.logf("row24 old 不在 known")
		return
	}
	cur, err := t.layer.Snapshot()
	if err != nil {
		return
	}
	a, b := &xlate.Layer{}, &xlate.Layer{}
	if a.Restore(old, t.fonts) != nil || b.Restore(cur, t.fonts) != nil {
		return
	}
	merged := &xlate.Layer{W: a.W, H: a.H, FontRegistry: t.fonts}
	for _, s := range a.Stamps {
		if s.Y != 192 {
			merged.Stamps = append(merged.Stamps, s)
		}
	}
	for _, s := range b.Stamps {
		if s.Y == 192 {
			merged.Stamps = append(merged.Stamps, s)
		}
	}
	if snap, err := merged.Snapshot(); err == nil {
		t.known[t.row24New] = snap
		t.logf("row24 merged stamps=%d", len(merged.Stamps))
	}
}

func (t *opsTracker) invertDone() {
	x0, y0, x1, y1 := t.invCol*8, t.invRow*8, (t.invCol+t.invW)*8, (t.invRow+1)*8
	n := 0
	for _, s := range t.layer.Stamps {
		sx0, sy0, sx1, sy1 := s.Rect()
		if sx0 < x1 && x0 < sx1 && sy0 < y1 && y0 < sy1 {
			s.State = xlate.Pending
			n++
		}
	}
	t.counts["invert"]++
	t.logf("invert row=%d col=%d w=%d -> pending %d", t.invRow, t.invCol, t.invW, n)
}

func (t *opsTracker) int86(o *oracle.Oracle) {
	if o.StackWord(1) != 0x10 {
		return
	}
	in := o.StackWord(2)
	ds := o.DSReg()
	ax, cx, dx := o.Word(oracle.Far(ds, in)), o.Word(oracle.Far(ds, in+4)), o.Word(oracle.Far(ds, in+6))
	ah, al := ax>>8, ax&0xFF
	x0, y0, x1, y1 := int(cx&0xFF)*8, int(cx>>8)*8, int(dx&0xFF+1)*8, int(dx>>8+1)*8
	switch ah {
	case 0x00:
		t.layer.Clear(0, 0, 320, 200)
		t.known = map[uint64][]byte{}
		t.counts["mode"]++
	case 0x06:
		t.counts["scroll-up"]++
		if al == 0 {
			t.layer.Clear(x0, y0, x1, y1)
		} else {
			t.layer.Scroll(x0, y0, x1, y1, -8*int(al))
		}
	case 0x07:
		t.counts["scroll-down"]++
		if al == 0 {
			t.layer.Clear(x0, y0, x1, y1)
		} else {
			t.layer.Scroll(x0, y0, x1, y1, 8*int(al))
		}
	case 0x0B:
		for _, s := range t.layer.Stamps {
			s.State = xlate.Pending
		}
		t.counts["palette"]++
	}
}
