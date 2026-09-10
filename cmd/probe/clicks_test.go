package main

import "testing"

// 點擊腳本的每一欄都要能單獨省略，而且**省略與寫錯要分得開**。
//
// 反面不會報錯：多解或少解一欄只是讓某一次點擊按在別的地方、按住別的
// 長度，收據跑出另一個畫面而旗標看起來完全正常。
func TestParseClicksFields(t *testing.T) {
	for _, tc := range []struct {
		spec string
		want click
	}{
		{"100:10:20", click{step: 100, x: 10, y: 20, btn: 1}},
		{"100:10:20:2", click{step: 100, x: 10, y: 20, btn: 2}},
		{"100:10:20:2:5000", click{step: 100, x: 10, y: 20, btn: 2, hold: 5000}},
		{"100:10:20::5000", click{step: 100, x: 10, y: 20, btn: 1, hold: 5000}},
	} {
		got, err := parseClicks(tc.spec)
		if err != nil {
			t.Fatalf("%q: %v", tc.spec, err)
		}
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("%q 解出 %+v，預期 %+v", tc.spec, got, tc.want)
		}
	}
	for _, bad := range []string{"100:10", "100:10:20:2:5000:9", "x:10:20", "100:10:20:2:zz"} {
		if _, err := parseClicks(bad); err == nil {
			t.Errorf("%q 應該要被拒絕", bad)
		}
	}
}

// 每一次的按住時間優先於全域值；沒寫才用全域。
func TestHoldOfPrefersPerClick(t *testing.T) {
	if got := holdOf(click{hold: 7}, 99); got != 7 {
		t.Errorf("寫了 7 卻用 %d", got)
	}
	if got := holdOf(click{}, 99); got != 99 {
		t.Errorf("沒寫時應該用全域的 99，得到 %d", got)
	}
}
