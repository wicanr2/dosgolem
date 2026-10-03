package phantasie

import (
	"reflect"
	"testing"
)

// 診斷計數器（docs/spec/001 §9）的單元測試。期望值都是手算的字面值。
// 本檔的輔助函式一律加前綴 ct。

// ctSnapEq 比對 Snapshot 與期望的計數（空與空視為相同）。
func ctSnapEq(t *testing.T, got, want map[string]uint64) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Snapshot = %v，要 %v", got, want)
	}
}

func TestCountersIncAddGet(t *testing.T) {
	c := NewCounters()
	if got := c.Get("events"); got != 0 {
		t.Fatalf("新計數器 Get(events) = %d，要 0", got)
	}
	c.Inc("events")
	c.Inc("events")
	c.Inc("events")
	if got := c.Get("events"); got != 3 {
		t.Errorf("Inc 三次後 Get(events) = %d，要 3", got)
	}
	c.Add("events", 10)
	if got := c.Get("events"); got != 13 {
		t.Errorf("再 Add 10 後 Get(events) = %d，要 13", got)
	}
	c.Add("composed", 5)
	c.Inc("dup_open")
	if got := c.Get("composed"); got != 5 {
		t.Errorf("Get(composed) = %d，要 5", got)
	}
	if got := c.Get("dup_open"); got != 1 {
		t.Errorf("Get(dup_open) = %d，要 1", got)
	}
	if got := c.Get("events"); got != 13 {
		t.Errorf("其他計數器的更動影響了 events：%d，要 13", got)
	}
	// 沒計過的名稱是 0
	if got := c.Get("never"); got != 0 {
		t.Errorf("Get(never) = %d，要 0", got)
	}
	// 64 位元：2 的 40 次方加兩次是 2 的 41 次方 = 2199023255552
	c.Add("big", 1<<40)
	c.Add("big", 1<<40)
	if got := c.Get("big"); got != 2199023255552 {
		t.Errorf("Get(big) = %d，要 2199023255552", got)
	}
}

func TestCountersGetDoesNotCreateEntry(t *testing.T) {
	c := NewCounters()
	_ = c.Get("never")
	_ = c.Get("never")
	ctSnapEq(t, c.Snapshot(), map[string]uint64{})
	if got := c.String(); got != "" {
		t.Errorf("只讀過的計數器 String() = %q，要空字串", got)
	}
}

func TestCountersSnapshotIsCopy(t *testing.T) {
	c := NewCounters()
	c.Inc("a")
	c.Add("b", 3)
	s := c.Snapshot()
	ctSnapEq(t, s, map[string]uint64{"a": 1, "b": 3})

	// 改複本不影響計數器
	s["a"] = 99
	s["new"] = 7
	if got := c.Get("a"); got != 1 {
		t.Errorf("改 Snapshot 後 Get(a) = %d，要 1", got)
	}
	if got := c.Get("new"); got != 0 {
		t.Errorf("改 Snapshot 後 Get(new) = %d，要 0", got)
	}
	ctSnapEq(t, c.Snapshot(), map[string]uint64{"a": 1, "b": 3})

	// 計數器之後的更動不影響先前取的複本
	c.Inc("b")
	if got := s["b"]; got != 3 {
		t.Errorf("Inc 之後先前複本的 b = %d，要 3", got)
	}
	ctSnapEq(t, c.Snapshot(), map[string]uint64{"a": 1, "b": 4})
}

func TestCountersSnapshotEmptyIsUsable(t *testing.T) {
	c := NewCounters()
	s := c.Snapshot()
	if s == nil {
		t.Fatal("空計數器的 Snapshot 是 nil，要空 map")
	}
	if len(s) != 0 {
		t.Errorf("空計數器的 Snapshot 有 %d 項，要 0", len(s))
	}
	s["x"] = 1 // 空 map 可寫入，不會 panic
	if got := c.Get("x"); got != 0 {
		t.Errorf("寫入空 Snapshot 後 Get(x) = %d，要 0", got)
	}
}

func TestCountersStringNonZeroSorted(t *testing.T) {
	c := NewCounters()
	if got := c.String(); got != "" {
		t.Errorf("新計數器 String() = %q，要空字串", got)
	}
	c.Add("composed_miss_prefix", 2)
	c.Inc("composed")
	c.Add("composed_miss_dest", 11)
	c.Inc("events")
	c.Add("zero", 0) // 值為 0 不列
	// 名稱依位元組排序：composed 是 composed_miss_dest 的前綴所以在前，dest 的 d 在 prefix 的 p 之前
	want := "composed=1 composed_miss_dest=11 composed_miss_prefix=2 events=1"
	if got := c.String(); got != want {
		t.Errorf("String() = %q，要 %q", got, want)
	}
	// 只有 0 值時是空字串
	z := NewCounters()
	z.Add("a", 0)
	z.Add("b", 0)
	if got := z.String(); got != "" {
		t.Errorf("全是 0 的 String() = %q，要空字串", got)
	}
	// 64 位元值照實印出
	c2 := NewCounters()
	c2.Add("big", 1<<40)
	if got := c2.String(); got != "big=1099511627776" {
		t.Errorf("String() = %q，要 big=1099511627776", got)
	}
}

func TestCountersKeySetDedupSorted(t *testing.T) {
	c := NewCounters()
	if got := c.KeySet("untranslated"); len(got) != 0 {
		t.Errorf("沒記過鍵的 KeySet = %q，要空", got)
	}
	// 插入順序刻意亂排並含重複；位元組排序：數字 < 大寫 < 小寫，短的在長的前面，"10" 在 "9" 之前
	for _, k := range []string{"b", "a", "b", "B", "ab", "9", "10", "a"} {
		c.Key("untranslated", k)
	}
	want := []string{"10", "9", "B", "a", "ab", "b"}
	if got := c.KeySet("untranslated"); !reflect.DeepEqual(got, want) {
		t.Errorf("KeySet = %q，要 %q", got, want)
	}
}

func TestCountersKeySetNamesAreSeparate(t *testing.T) {
	c := NewCounters()
	c.Key("untranslated", "FIGHT")
	c.Key("untranslated_args", "MONK")
	c.Key("untranslated_args", "NO")
	if got, want := c.KeySet("untranslated"), []string{"FIGHT"}; !reflect.DeepEqual(got, want) {
		t.Errorf("KeySet(untranslated) = %q，要 %q", got, want)
	}
	if got, want := c.KeySet("untranslated_args"), []string{"MONK", "NO"}; !reflect.DeepEqual(got, want) {
		t.Errorf("KeySet(untranslated_args) = %q，要 %q", got, want)
	}
	if got := c.KeySet("arg_unclassified"); len(got) != 0 {
		t.Errorf("沒記過的名稱 KeySet = %q，要空", got)
	}
}

func TestCountersKeySetIsCopy(t *testing.T) {
	c := NewCounters()
	c.Key("k", "b")
	c.Key("k", "a")
	got := c.KeySet("k")
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("KeySet = %q，要 [a b]", got)
	}
	got[0] = "zzz"
	if again, want := c.KeySet("k"), []string{"a", "b"}; !reflect.DeepEqual(again, want) {
		t.Errorf("改動回傳的切片後 KeySet = %q，要 %q", again, want)
	}
}

// 計數與鍵集合各自獨立：同名的計數器與鍵集合互不影響，String 只列計數，不列鍵。
func TestCountersKeysIndependentFromCounts(t *testing.T) {
	c := NewCounters()
	c.Inc("untranslated")
	c.Key("untranslated", "FIGHT")
	c.Key("untranslated", "FIGHT")
	// 記鍵不增加計數；計數只由 Inc、Add 增加
	if got := c.Get("untranslated"); got != 1 {
		t.Errorf("Get(untranslated) = %d，要 1（Key 不計數）", got)
	}
	if got, want := c.KeySet("untranslated"), []string{"FIGHT"}; !reflect.DeepEqual(got, want) {
		t.Errorf("KeySet = %q，要 %q", got, want)
	}
	if got := c.String(); got != "untranslated=1" {
		t.Errorf("String() = %q，要 untranslated=1（不列鍵）", got)
	}
	// 只記鍵、沒有計數時，Snapshot 與 String 都沒有該名稱
	k := NewCounters()
	k.Key("arg_unclassified", "(107E,0,0000)")
	ctSnapEq(t, k.Snapshot(), map[string]uint64{})
	if got := k.String(); got != "" {
		t.Errorf("只有鍵集合時 String() = %q，要空字串", got)
	}
}

func TestCountersInstancesAreIndependent(t *testing.T) {
	a, b := NewCounters(), NewCounters()
	a.Inc("events")
	a.Key("k", "x")
	if got := b.Get("events"); got != 0 {
		t.Errorf("另一個實例的 Get(events) = %d，要 0", got)
	}
	if got := b.KeySet("k"); len(got) != 0 {
		t.Errorf("另一個實例的 KeySet = %q，要空", got)
	}
}
