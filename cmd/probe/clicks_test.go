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

// 放開判準的優先序：腳本逐次寫死的 > 全域 -click-polls > 全域 -click-hold。
//
// ⚠ **這支測試釘的是「越具體越優先」，不是實作細節。** 把
// `-click-polls` 提到第一位的話，凡是同時給了它的那一次執行，收據裡
// 逐次寫死的按住時間就全部失效——畫面照樣畫得出來，只是不是原本那一張，
// 所以沒有測試會自己開口。理由見 `click.hold` 的註解與 `docs/spec/004` §4.5。
func TestHoldForPrefersTheMoreSpecific(t *testing.T) {
	for _, tc := range []struct {
		name  string
		c     click
		polls int
		hold  uint64
		want  holdMode
	}{
		{"逐次寫死的贏全域輪詢數", click{hold: 7}, 3, 99, holdMode{steps: 7}},
		{"逐次寫死的贏全域指令數", click{hold: 7}, 0, 99, holdMode{steps: 7}},
		{"沒寫就用全域輪詢數", click{}, 3, 99, holdMode{polls: 3}},
		{"兩個全域都在時輪詢數優先", click{}, 3, 99, holdMode{polls: 3}},
		{"都沒有就用全域指令數", click{}, 0, 99, holdMode{steps: 99}},
	} {
		if got := holdFor(tc.c, tc.polls, tc.hold); got != tc.want {
			t.Errorf("%s：得到 %+v，預期 %+v", tc.name, got, tc.want)
		}
	}
}
