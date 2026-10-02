package main

import "testing"

func TestFD2KeyNamesIncludeSecretShopFunctionChords(t *testing.T) {
	keys := fd2KeyNames()
	want := map[string]uint16{
		"shift-f1":  0x5400,
		"shift-f5":  0x5800,
		"shift-f10": 0x5d00,
		"ctrl-f1":   0x5e00,
		"ctrl-f5":   0x6200,
		"ctrl-f6":   0x6300,
		"ctrl-f10":  0x6700,
		"alt-f1":    0x6800,
		"alt-f10":   0x7100,
	}
	for name, value := range want {
		if got := keys[name]; got != value {
			t.Fatalf("%s=%04X，應為 %04X", name, got, value)
		}
	}
	if len(keys) != 36 {
		t.Fatalf("按鍵表有 %d 筆，應為 6 個基本鍵加 30 個 F 鍵 chord", len(keys))
	}
}

func TestEIPTraceWindowBoundariesAndEntryBudget(t *testing.T) {
	w := eipTraceWindow{100, 200, 2}
	for _, c := range []struct {
		step, entries int
		want          bool
	}{
		{99, 0, false}, {100, 0, true}, {200, 1, true}, {201, 0, false}, {150, 2, false},
	} {
		if got := w.allows(c.step, c.entries); got != c.want {
			t.Fatalf("step=%d entries=%d got=%v want=%v", c.step, c.entries, got, c.want)
		}
	}
	if !(eipTraceWindow{0, 0, 200000}).allows(30000000000, 199999) {
		t.Fatal("預設窗口應保留既有全程追蹤")
	}
	for _, w := range []eipTraceWindow{{-1, 0, 1}, {0, -1, 1}, {2, 1, 1}, {0, 0, 0}, {0, 0, 200001}} {
		if w.valid() {
			t.Fatalf("接受非法窗口 %+v", w)
		}
	}
	for _, w := range []eipTraceWindow{{0, 0, 200000}, {100, 100, 1}, {100, 0, 2}} {
		if !w.valid() {
			t.Fatalf("拒絕合法窗口 %+v", w)
		}
	}
}
