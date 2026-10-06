package xlate

import (
	"bytes"
	"testing"
)

var artRed = [4]uint8{240, 30, 90, 255}

func artRow(key string, y int) *Stamp {
	pix := make([]uint8, 16*8*3*3*4)
	for i := 0; i < len(pix); i += 4 {
		copy(pix[i:i+4], artRed[:])
	}
	ref := bytes.Repeat([]byte{1}, 16*8)
	return &Stamp{Key: key, X: 0, Y: y, Cells: 2, CellW: 8, CellH: 8,
		Art: true, PixScale: 3, Pix: pix, Reference: ref}
}

func artBoard() ([]byte, []byte) {
	idx := bytes.Repeat([]byte{1}, 16*16)
	rgb := bytes.Repeat([]byte{10, 20, 30}, 16*16)
	return idx, rgb
}

func requireArt(t *testing.T, l *Layer, s *Stamp) {
	t.Helper()
	if err := l.AddArt(s); err != nil {
		t.Fatal(err)
	}
}

func regionIs(t *testing.T, dst []byte, x0, y0, x1, y1 int, color [4]byte) {
	t.Helper()
	for y := y0 * 3; y < y1*3; y++ {
		for x := x0 * 3; x < x1*3; x++ {
			i := 4 * (y*48 + x)
			if !bytes.Equal(dst[i:i+4], color[:]) {
				t.Fatalf("(%d,%d) 是 %v，應為 %v", x, y, dst[i:i+4], color)
			}
		}
	}
}

func drawSmall(l *Layer) ([]byte, bool) {
	dst := make([]byte, 48*48*4)
	return dst, l.Draw(dst, 3, nil)
}

func TestArtCellRecoveryAndImmutableReference(t *testing.T) {
	l := &Layer{W: 16, H: 16}
	s := artRow("row", 0)
	requireArt(t, l, s)
	// 呼叫端切片改動不得改掉已登記的素材或基準。
	s.Reference[0] = 9
	s.Pix[0] = 0
	idx, rgb := artBoard()
	l.Frame(idx, rgb)
	dst, drew := drawSmall(l)
	if !drew {
		t.Fatal("初始圖面應繪製")
	}
	regionIs(t, dst, 0, 0, 16, 8, artRed)
	idx[3*16+2] = 9 // 一像素足以遮整個左格。
	l.Frame(idx, rgb)
	dst, _ = drawSmall(l)
	regionIs(t, dst, 0, 0, 8, 8, [4]byte{})
	regionIs(t, dst, 8, 0, 16, 8, artRed)
	idx[3*16+2] = 1
	l.Frame(idx, rgb)
	dst, _ = drawSmall(l)
	regionIs(t, dst, 0, 0, 16, 8, artRed)
	for i := range idx {
		idx[i] = 9
	}
	l.Frame(idx, rgb)
	if _, drew = drawSmall(l); drew || len(l.Art) != 1 {
		t.Fatal("全遮時須保留登記且不繪製")
	}
	idx, rgb = artBoard()
	l.Frame(idx, rgb)
	dst, _ = drawSmall(l)
	regionIs(t, dst, 0, 0, 16, 8, artRed)
	l.Art[0].Transparent = []bool{true, false}
	l.Frame(idx, rgb)
	dst, _ = drawSmall(l)
	regionIs(t, dst, 0, 0, 8, 8, [4]byte{})
	regionIs(t, dst, 8, 0, 16, 8, artRed)
}

func TestArtSeparateFromTextAndExplicitOrder(t *testing.T) {
	l := &Layer{W: 16, H: 16}
	front := artRow("a-front", 0)
	front.Order = 20
	for i := 0; i < len(front.Pix); i += 4 {
		copy(front.Pix[i:i+4], []byte{40, 180, 50, 255})
	}
	back := artRow("z-back", 0)
	back.Order = 10
	requireArt(t, l, front)
	requireArt(t, l, back) // 逆序登記。
	l.Add(&Stamp{Key: "text", X: 8, Y: 0, Cells: 1, CellW: 8, CellH: 8, State: Pending})
	idx, rgb := artBoard()
	l.Frame(idx, rgb)
	dst, _ := drawSmall(l)
	if len(l.Art) != 2 || len(l.Stamps) != 1 {
		t.Fatal("圖與字不能互相移除")
	}
	regionIs(t, dst, 0, 0, 8, 8, [4]byte{40, 180, 50, 255})
	regionIs(t, dst, 8, 0, 16, 8, [4]byte{10, 20, 30, 255})
	l.Scroll(0, 0, 16, 16, 8)
	if l.Art[0].Y != 0 || l.Art[1].Y != 0 {
		t.Fatal("文字捲動不應移動圖面")
	}
	l.ClearArt()
	if len(l.Stamps) != 1 || len(l.Art) != 0 {
		t.Fatal("ClearArt 只能清圖")
	}
}

func artWatcher(made *int) *Watcher {
	return &Watcher{Key: "panel", Art: true, ArtKeys: []string{"row0", "row8"}, W: 16, H: 8,
		Want: bytes.Repeat([]byte{1}, 128), Make: func() []*Stamp {
			*made++
			return []*Stamp{artRow("row0", 0), artRow("row8", 8)}
		}}
}

func TestArtWatcherFirstDrawGroupRecoveryAndGate(t *testing.T) {
	l := &Layer{W: 16, H: 16}
	made := 0
	wa := artWatcher(&made)
	l.Watch(wa)
	idx, rgb := artBoard()
	idx[10*16+2] = 9
	l.Frame(idx, rgb)
	dst, _ := drawSmall(l)
	regionIs(t, dst, 0, 0, 16, 8, artRed)
	regionIs(t, dst, 0, 8, 8, 16, [4]byte{}) // match 只有第一列，不得從第二列當前狀態採樣。
	regionIs(t, dst, 8, 8, 16, 16, artRed)
	if made != 1 || len(l.Art) != 2 {
		t.Fatal("應完整建立兩列")
	}
	idx[10*16+2] = 1
	l.Frame(idx, rgb)
	dst, _ = drawSmall(l)
	regionIs(t, dst, 0, 0, 16, 16, artRed)
	if made != 1 {
		t.Fatal("逐格恢復不應重複建立")
	}
	l.DropArt("row0")
	l.Frame(idx, rgb)
	if made != 2 || len(l.Art) != 2 {
		t.Fatal("另一列存活不能阻止補齊缺列")
	}
	l.Frame(idx, rgb)
	if made != 2 || len(l.Art) != 2 {
		t.Fatal("完整集合不可重複加")
	}
	idx[0] = 9
	l.Frame(idx, rgb)
	if _, drew := drawSmall(l); drew {
		t.Fatal("整組啟用區不符時應全部停用")
	}
	idx[0] = 1
	l.Frame(idx, rgb)
	dst, _ = drawSmall(l)
	regionIs(t, dst, 0, 0, 16, 16, artRed)
	l.Unwatch("panel")
	l.Frame(idx, rgb)
	if _, drew := drawSmall(l); drew {
		t.Fatal("Unwatch 後不能失去啟用依據仍繪製")
	}
}

func TestArtWatcherRejectsIncompleteFactoryAtomically(t *testing.T) {
	cases := map[string]func() []*Stamp{
		"missing":   func() []*Stamp { return []*Stamp{artRow("row0", 0)} },
		"duplicate": func() []*Stamp { return []*Stamp{artRow("row0", 0), artRow("row0", 8)} },
		"extra":     func() []*Stamp { return []*Stamp{artRow("row0", 0), artRow("row8", 8), artRow("extra", 0)} },
		"nil":       func() []*Stamp { return []*Stamp{nil, artRow("row8", 8)} },
		"reference": func() []*Stamp { s := artRow("row8", 8); s.Reference = nil; return []*Stamp{artRow("row0", 0), s} },
	}
	for name, factory := range cases {
		t.Run(name, func(t *testing.T) {
			l := &Layer{W: 16, H: 16}
			made := 0
			wa := artWatcher(&made)
			l.Watch(wa)
			idx, rgb := artBoard()
			l.Frame(idx, rgb)
			l.DropArt("row0")
			survivor := l.Art[0]
			drops := 0
			l.OnDrop = func(s *Stamp, why string) {
				if s == nil {
					t.Fatal("通知不能傳 nil")
				}
				if why == "art" {
					drops++
				}
			}
			wa.Make = factory
			l.Frame(idx, rgb)
			if len(l.Art) != 1 || l.Art[0] != survivor || drops == 0 {
				t.Fatal("錯誤 factory 必須整組拒絕，不部分更新")
			}
			if _, drew := drawSmall(l); drew {
				t.Fatal("錯誤的半組不得繪製")
			}
		})
	}
}

func TestArtAlphaAndScaleRecovery(t *testing.T) {
	l := &Layer{W: 16, H: 16}
	s := artRow("alpha", 0)
	for i := 3; i < len(s.Pix); i += 4 {
		s.Pix[i] = 0
	}
	copy(s.Pix[4:8], []byte{200, 100, 40, 128})
	requireArt(t, l, s)
	idx, rgb := artBoard()
	l.Frame(idx, rgb)
	dst := bytes.Repeat([]byte{20, 40, 80, 255}, 48*48)
	if !l.Draw(dst, 3, nil) {
		t.Fatal("半透明像素應繪製")
	}
	if !bytes.Equal(dst[:4], []byte{20, 40, 80, 255}) || !bytes.Equal(dst[4:8], []byte{110, 70, 60, 255}) {
		t.Fatalf("透明／半透明合成不符 %v", dst[:8])
	}
	dst = make([]byte, 48*48*4)
	l.Draw(dst, 3, nil)
	if !bytes.Equal(dst[4:8], []byte{200, 100, 40, 128}) {
		t.Fatalf("透明目標不能二次預乘 %v", dst[4:8])
	}
	warnings := 0
	l.OnDrop = func(s *Stamp, why string) {
		if why == "scale" {
			warnings++
		}
	}
	for i := 0; i < 2; i++ {
		if l.Draw(make([]byte, 32*32*4), 2, nil) {
			t.Fatal("錯倍率不能繪製")
		}
	}
	if warnings != 1 || len(l.Art) != 1 {
		t.Fatal("每段倍率不符只通知一次且不清資產")
	}
	l.Draw(make([]byte, 48*48*4), 3, nil)
	l.Draw(make([]byte, 32*32*4), 2, nil)
	if warnings != 2 {
		t.Fatal("恢復後新的錯倍率應再通知")
	}
}

func TestArtSnapshotRebindAndFailedRestore(t *testing.T) {
	l := &Layer{W: 16, H: 16}
	made := 0
	wa := artWatcher(&made)
	l.Watch(wa)
	idx, rgb := artBoard()
	l.Frame(idx, rgb)
	snap, err := l.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(snap, []byte("reference")) || bytes.Contains(snap, []byte("\"pix\":")) {
		t.Fatal("快照不能保存資產")
	}
	if err = l.Restore(snap, nil); err != nil {
		t.Fatal(err)
	}
	if len(l.Art) != 0 {
		t.Fatal("還原不能留著沒有資產的半組")
	}
	l.Watch(wa)
	l.Frame(idx, rgb)
	l.Frame(idx, rgb)
	if len(l.Art) != 2 || made != 2 {
		t.Fatal("重登記應完整且不重複")
	}
	l.Add(&Stamp{Key: "font", Font: &Font{Name: "missing"}, Cells: 1, CellW: 8, CellH: 8})
	bad, err := l.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Restore(bad, nil); err == nil || len(l.Art) != 2 {
		t.Fatal("失敗還原不能清圖")
	}
	if err = l.Restore([]byte("{"), nil); err == nil || len(l.Art) != 2 {
		t.Fatal("壞 JSON 不能清圖")
	}
}

func TestArtValidationAndMalformedFrames(t *testing.T) {
	changes := map[string]func(*Stamp){
		"not-art": func(s *Stamp) { s.Art = false }, "key": func(s *Stamp) { s.Key = "" },
		"x": func(s *Stamp) { s.X = -1 }, "y": func(s *Stamp) { s.Y = 16 }, "cells": func(s *Stamp) { s.Cells = 0 },
		"overflow": func(s *Stamp) { s.Cells = int(^uint(0) >> 1) }, "scale": func(s *Stamp) { s.PixScale = 0 },
		"pix": func(s *Stamp) { s.Pix = s.Pix[:len(s.Pix)-1] }, "reference": func(s *Stamp) { s.Reference = nil },
	}
	l := &Layer{W: 16, H: 16}
	requireArt(t, l, artRow("keep", 0))
	original := l.Art[0]
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			s := artRow("keep", 0)
			change(s)
			if l.AddArt(s) == nil || len(l.Art) != 1 || l.Art[0] != original {
				t.Fatal("非法登記不能改已有圖面")
			}
		})
	}
	if l.AddArt(nil) == nil {
		t.Fatal("nil 必須拒絕")
	}
	idx, rgb := artBoard()
	l.Frame(idx, rgb)
	for _, scale := range []int{0, -1, int(^uint(0) >> 1)} {
		if l.Draw(make([]byte, 4), scale, nil) {
			t.Fatal("壞倍率不能繪製")
		}
	}
	short := bytes.Repeat([]byte{77}, 48*48*4-1)
	before := append([]byte(nil), short...)
	if l.Draw(short, 3, nil) || !bytes.Equal(short, before) {
		t.Fatal("截短目標不可寫入")
	}
	l.Frame(idx[:len(idx)-1], rgb)
	if _, drew := drawSmall(l); drew {
		t.Fatal("壞幀須停止繪圖")
	}
	l.Frame(idx, rgb)
	if _, drew := drawSmall(l); !drew {
		t.Fatal("有效幀應恢復")
	}
}

func TestArtWatcherCollisionAndNonTripleScale(t *testing.T) {
	l := &Layer{W: 16, H: 16}
	requireArt(t, l, artRow("row0", 0)) // 空 Owner 的既有圖不能被 watcher 奪走。
	foreign := l.Art[0]
	made := 0
	wa := artWatcher(&made)
	l.Watch(wa)
	idx, rgb := artBoard()
	l.Frame(idx, rgb)
	if len(l.Art) != 1 || l.Art[0] != foreign || made != 1 {
		t.Fatal("同 Key 衝突應拒絕整組")
	}
	l.UnwatchAll()
	l.ClearArt()
	s := artRow("scale2", 0)
	s.PixScale = 2
	s.Pix = bytes.Repeat(artRed[:], 16*8*4)
	requireArt(t, l, s)
	l.Frame(idx, rgb)
	if !l.Draw(make([]byte, 32*32*4), 2, nil) {
		t.Fatal("合法圖面可用非三倍數倍率")
	}
	s.Order = 3
	requireArt(t, l, s)
	if len(l.Art) != 1 || l.Art[0].Order != 3 {
		t.Fatal("同 Key 必須替換而非新增")
	}
	// 相同 Order 仍以 Key 決定順序，不取決於登記先後。
	l.ClearArt()
	z := artRow("z", 0)
	a := artRow("a", 0)
	for i := 0; i < len(z.Pix); i += 4 {
		copy(z.Pix[i:i+4], []byte{1, 2, 3, 255})
	}
	requireArt(t, l, z)
	requireArt(t, l, a)
	l.Frame(idx, rgb)
	dst, _ := drawSmall(l)
	regionIs(t, dst, 0, 0, 16, 8, [4]byte{1, 2, 3, 255})
}

func BenchmarkArtPlane(b *testing.B) {
	for _, withArt := range []bool{false, true} {
		name := "text"
		if withArt {
			name = "art1000-text"
		}
		b.Run(name, func(b *testing.B) {
			l := &Layer{}
			idx := make([]byte, 320*200)
			rgb := make([]byte, 320*200*3)
			l.Add(&Stamp{Key: "text", X: 8, Y: 8, Cells: 8, CellW: 8, CellH: 8, State: Pending})
			if withArt {
				for y := 0; y < 200; y += 8 {
					s := &Stamp{Key: string(rune('a' + y/8)), Y: y, Cells: 40, CellW: 8, CellH: 8, Art: true, PixScale: 3,
						Reference: make([]byte, 320*8), Pix: bytes.Repeat(artRed[:], 320*8*9)}
					if err := l.AddArt(s); err != nil {
						b.Fatal(err)
					}
				}
			}
			l.Frame(idx, rgb)
			dst := make([]byte, 960*600*4)
			b.Run("Frame", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					l.Frame(idx, rgb)
				}
			})
			b.Run("Draw", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					l.Draw(dst, 3, nil)
				}
			})
		})
	}
}
