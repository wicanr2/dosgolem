package xlate

import "testing"

// spec 203 §3：以畫面內容當觸發點。
func screen(w, h int) ([]uint8, []uint8) { return make([]uint8, w*h), make([]uint8, 3*w*h) }

func blockWatcher(made *int) *Watcher {
	want := make([]uint8, 16)
	for i := range want {
		want[i] = uint8(i%4) + 1
	}
	return &Watcher{Key: "panel", X: 8, Y: 16, W: 4, H: 4, Want: want, Make: func() []*Stamp {
		*made++
		return []*Stamp{{Key: "t", X: 8, Y: 16, Cells: 1, CellW: 4, CellH: 4, State: Pending}}
	}}
}

func put(idx []uint8, w int, wa *Watcher, vals []uint8) {
	for i, v := range vals {
		idx[(wa.Y+i/wa.W)*w+wa.X+i%wa.W] = v
	}
}

func TestWatcherAddsAndRecovers(t *testing.T) {
	idx, rgb := screen(DefaultScreenW, DefaultScreenH)
	made := 0
	l := &Layer{}
	wa := blockWatcher(&made)
	l.Watch(wa)
	l.Frame(idx, rgb) // 反向對照：內容不符時不建立
	if made != 0 || len(l.Stamps) != 0 {
		t.Fatalf("內容不符卻建立了：made=%d", made)
	}
	put(idx, DefaultScreenW, wa, wa.Want)
	l.Frame(idx, rgb)
	if made != 1 || len(l.Stamps) != 1 || l.Stamps[0].Owner != "panel" {
		t.Fatalf("內容相符應該建立並帶 Owner：made=%d 筆數=%d", made, len(l.Stamps))
	}
	l.Frame(idx, rgb) // 疊字還活著就不再比對
	if made != 1 {
		t.Fatalf("疊字還在時不該重複建立：made=%d", made)
	}
	changed := append([]uint8(nil), idx...)
	changed[17*DefaultScreenW+9] = 9
	for i := 0; i < 3; i++ {
		l.Frame(changed, rgb)
	}
	if len(l.Stamps) != 0 {
		t.Fatal("內容變了應該失效")
	}
	l.Frame(idx, rgb) // 回到原樣就再蓋一次
	if made != 2 || len(l.Stamps) != 1 {
		t.Fatalf("回到原樣應該重新建立：made=%d", made)
	}
}

func TestWatcherProbeDoesNotDecide(t *testing.T) {
	idx, rgb := screen(DefaultScreenW, DefaultScreenH)
	made := 0
	wa := blockWatcher(&made)
	wa.Probe = []int{0}
	l := &Layer{}
	l.Watch(wa)
	vals := append([]uint8(nil), wa.Want...)
	vals[7] ^= 0xF // Probe 以外的一格不同
	put(idx, DefaultScreenW, wa, vals)
	l.Frame(idx, rgb)
	if made != 0 {
		t.Fatal("Probe 只是加速，不能決定結果")
	}
}

func TestUnwatch(t *testing.T) {
	idx, rgb := screen(DefaultScreenW, DefaultScreenH)
	made := 0
	wa := blockWatcher(&made)
	l := &Layer{}
	l.Watch(wa)
	l.Watch(wa) // 同一個 Key 只留一個
	if l.Watchers() != 1 {
		t.Fatalf("同 Key 應該只留一個：%d", l.Watchers())
	}
	l.Unwatch("panel")
	put(idx, DefaultScreenW, wa, wa.Want)
	l.Frame(idx, rgb)
	if made != 0 || l.Watchers() != 0 {
		t.Fatalf("移除後不該再比對：made=%d 剩 %d", made, l.Watchers())
	}
}

func TestWatcherSnapshotKeepsOwner(t *testing.T) {
	idx, rgb := screen(DefaultScreenW, DefaultScreenH)
	made := 0
	wa := blockWatcher(&made)
	l := &Layer{}
	l.Watch(wa)
	put(idx, DefaultScreenW, wa, wa.Want)
	l.Frame(idx, rgb)
	b, err := l.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	l2 := &Layer{}
	if err := l2.Restore(b, nil); err != nil {
		t.Fatal(err)
	}
	if len(l2.Stamps) != 1 || l2.Stamps[0].Owner != "panel" {
		t.Fatal("快照要保存 Owner")
	}
	l2.Watch(wa) // 還原後重新登記，不該重複加
	l2.Frame(idx, rgb)
	if len(l2.Stamps) != 1 || made != 1 {
		t.Fatalf("還原後不該重複建立：筆數=%d made=%d", len(l2.Stamps), made)
	}
}

// spec 203 §2.1：watcher 的疊字整筆失效，不逐格遮。
func TestWatcherStampDropsWholeOnPartialChange(t *testing.T) {
	idx, rgb := screen(DefaultScreenW, DefaultScreenH)
	made := 0
	wa := &Watcher{Key: "panel", X: 8, Y: 16, W: 16, H: 8, Want: make([]uint8, 16*8), Make: func() []*Stamp {
		made++
		return []*Stamp{{Key: "t", X: 8, Y: 16, Cells: 2, CellW: 8, CellH: 8, State: Pending}}
	}}
	l := &Layer{}
	l.Watch(wa)
	l.Frame(idx, rgb)
	if made != 1 || len(l.Stamps) != 1 {
		t.Fatalf("應該蓋上：made=%d 筆數=%d", made, len(l.Stamps))
	}
	changed := append([]uint8(nil), idx...)
	changed[17*DefaultScreenW+9] = 7 // 只改第 0 格
	for i := 0; i < 3; i++ {
		l.Frame(changed, rgb)
	}
	if len(l.Stamps) != 0 {
		t.Fatalf("watcher 的疊字應該整筆移除，剩 %d 筆（透明=%v）", len(l.Stamps), l.Stamps[0].Transparent)
	}
	l.Frame(idx, rgb) // 畫面回到原樣就再蓋一次
	if made != 2 || len(l.Stamps) != 1 {
		t.Fatalf("回到原樣應該重蓋：made=%d 筆數=%d", made, len(l.Stamps))
	}
}
