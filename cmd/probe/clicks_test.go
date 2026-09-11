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

// 每一次腳本點擊都要先送一次移動事件，而且**按下仍落在腳本指定的步數**。
//
// ⚠ 這支測試釘的是一個會安靜消失的行為。移動與按下擠在同一道指令上時，
// 遊戲照樣開得了選單、框與內容逐位元組相同，只是「游標經過那裡」帶起來
// 的副作用（懸停面板、反白、狀態欄）整批不見——沒有任何一個環節會報錯。
// 2026-09-11 踩過一次：舊版寫死的 `moveLead` 在改寫命令列時掉了，40 份
// 收據因此重跑出另一個畫面，被當成 remake 的缺口追了一整輪
// （`docs/spec/004` §4.20.1）。
func TestPreMoveComesBeforeThepress(t *testing.T) {
	c := click{step: 1_000_000, x: 10, y: 20, btn: 1}
	const lead = 200_000
	if !preMoveNow(c, c.step-lead, 0, lead) {
		t.Error("提早 lead 道指令的那一步應該要送移動事件")
	}
	for _, steps := range []uint64{c.step - lead - 1, c.step - lead + 1, c.step} {
		if preMoveNow(c, steps, 0, lead) {
			t.Errorf("步數 %d 不該送移動事件", steps)
		}
	}
	// -click-premove 一開就由它作主（它會把按下的時間點往後移）。
	if preMoveNow(c, c.step-lead, 3, lead) {
		t.Error("-click-premove 開著時不該再送提早的移動事件")
	}
	// lead 0 ＝ 明講「移到就按」。
	if preMoveNow(c, c.step, 0, 0) {
		t.Error("lead 0 時不該送提早的移動事件")
	}
	// 點擊早於 lead 時不能讓步數往下溢位。
	early := click{step: 1000, x: 1, y: 2, btn: 1}
	if preMoveNow(early, 0, 0, lead) {
		t.Error("點擊步數小於 lead 時不該觸發")
	}
}
