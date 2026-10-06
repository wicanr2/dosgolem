package xlate

import (
	"bytes"
	"reflect"
	"testing"
)

// spec 202 §2.3.1：色號不變，黑色色盤恢复白色，失效狀態不可被重置。
func TestShownPaletteRecoveryAndSnapshot(t *testing.T) {
	idx := make([]uint8, DefaultScreenW*DefaultScreenH)
	idx[8*DefaultScreenW+9] = 15
	idx[8*DefaultScreenW+17] = 15
	black := make([]uint8, len(idx)*3)
	white := make([]uint8, len(black))
	for i, c := range idx {
		if c == 15 {
			copy(white[i*3:i*3+3], []byte{255, 255, 255})
		}
	}
	for _, swap := range []bool{false, true} {
		name := "plain"
		if swap {
			name = "swap"
		}
		t.Run(name, func(t *testing.T) {
			s := &Stamp{Key: "palette", X: 8, Y: 8, Cells: 2, CellW: 8, CellH: 8, State: Pending, SwapColors: swap}
			l := &Layer{Stamps: []*Stamp{s}}
			l.Frame(idx, black)
			b, e := l.Snapshot()
			if e != nil {
				t.Fatal(e)
			}
			hashes := append([]uint64(nil), s.hashes...)
			anchors := append([]bool(nil), s.anchors...)
			misses := append([]int(nil), s.misses...)
			l.Frame(idx, white)
			expectedFG, expectedBG := [3]byte{255, 255, 255}, [3]byte{}
			if swap {
				expectedFG, expectedBG = expectedBG, expectedFG
			}
			if s.FG != expectedFG || s.BG != expectedBG {
				t.Fatalf("色盤恢復：FG=%v BG=%v", s.FG, s.BG)
			}
			if !reflect.DeepEqual(hashes, s.hashes) || !reflect.DeepEqual(anchors, s.anchors) || !reflect.DeepEqual(misses, s.misses) || len(l.Stamps) != 1 {
				t.Fatal("色盤改變了來源或失效狀態")
			}
			// 記憶體色號不加入JSON；舊快照的原字串仍可讀，先保留RGB再由吻合格更新。
			if bytes.Contains(b, []byte("color_indices")) || bytes.Contains(b, []byte("fg_index")) {
				t.Fatal("不應改快照格式")
			}
			var restored Layer
			if e := restored.Restore(b, nil); e != nil {
				t.Fatal(e)
			}
			if restored.Stamps[0].FG != [3]byte{} || restored.Stamps[0].BG != [3]byte{} {
				t.Fatal("Restore本身不應猜顏色")
			}
			restored.Frame(idx, white)
			if restored.Stamps[0].FG != expectedFG || restored.Stamps[0].BG != expectedBG {
				t.Fatal("舊快照未追隨原版色盤")
			}
		})
	}
}

// 失配格不能供定色；剩下的吻合格可追隨色盤，失效計數繼續累計。
func TestPaletteIgnoresChangedCells(t *testing.T) {
	idx := make([]uint8, DefaultScreenW*DefaultScreenH)
	idx[8*DefaultScreenW+9] = 15
	idx[8*DefaultScreenW+17] = 15
	rgb := make([]uint8, len(idx)*3)
	for i, c := range idx {
		if c == 15 {
			copy(rgb[i*3:i*3+3], []byte{255, 255, 255})
		}
	}
	s := &Stamp{Key: "valid", X: 8, Y: 8, Cells: 2, CellW: 8, CellH: 8, State: Pending}
	l := &Layer{Stamps: []*Stamp{s}}
	l.Frame(idx, rgb)
	changed := append([]byte(nil), idx...)
	changed[8*DefaultScreenW+10] = 15
	next := make([]uint8, len(rgb))
	copy(next[(8*DefaultScreenW+9)*3:], []byte{1, 2, 3})
	copy(next[(8*DefaultScreenW+17)*3:], []byte{100, 110, 120})
	l.Frame(changed, next)
	if s.FG != [3]byte{100, 110, 120} || s.misses[0] != 1 {
		t.Fatalf("應取第二格RGB且累計一次失配：%v %v", s.FG, s.misses)
	}
	l.Frame(changed, next)
	l.Frame(changed, next)
	if !s.transparent(0) || s.transparent(1) || len(l.Stamps) != 1 {
		t.Fatal("原逐格失效規則改變")
	}
	// 色號15在剩餘吻合格消失時，不取已改變格的新RGB。
	changed[8*DefaultScreenW+17] = 0
	l.Frame(changed, next)
	if s.FG != [3]byte{100, 110, 120} {
		t.Fatal("從失配格猜出新前景")
	}
	l.Frame(changed, next)
	l.Frame(changed, next)
	if len(l.Stamps) != 0 {
		t.Fatal("錨定格失效後應整筆移除")
	}
}
