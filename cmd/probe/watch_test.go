package main

import (
	"reflect"
	"testing"
)

// TestParseWatchRangesKeepsEveryRange：逗號後面的每一段都要解出來（issue #54）。
func TestParseWatchRangesKeepsEveryRange(t *testing.T) {
	got, err := parseWatchRanges("5E00C-5E00F,60BA4-60BA4, 60BE0 ,53d20-53d21")
	if err != nil {
		t.Fatal(err)
	}
	want := []watchRange{{0x5E00C, 0x5E00F}, {0x60BA4, 0x60BA4}, {0x60BE0, 0x60BE0}, {0x53D20, 0x53D21}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %X, want %X", got, want)
	}
}

// TestParseWatchRangesRejectsJunk：解不出來的那一段要報錯，不要安靜略過。
func TestParseWatchRangesRejectsJunk(t *testing.T) {
	for _, s := range []string{"5E00C-5E00F,xyz", "10-5", "5E00C-", "", ","} {
		if _, err := parseWatchRanges(s); err == nil {
			t.Errorf("%q 應該報錯", s)
		}
	}
}

// TestRegsAtKeepsNearbyPointers：預設不跳任何命中。`fcx-assault-ast` 的逐擊函式
// SI 從 0000 換到 0004，舊版寫死的 blit 去重把那一擊吞掉（issue #54）。
func TestRegsAtKeepsNearbyPointers(t *testing.T) {
	const ds = 0x54B3 << 16
	if skipRegsHit(false, true, ds|0x0000, ds|0x0004) {
		t.Fatal("預設不該跳過")
	}
	if !skipRegsHit(true, true, ds|0x0000, ds|0x0004) {
		t.Fatal("-regs-skip-blit 開著時要跳過往前 4 的那一筆")
	}
	if skipRegsHit(true, true, ds|0x0004, ds|0x0004) || skipRegsHit(true, false, 0, ds|4) {
		t.Fatal("位置相同或第一筆不該跳過")
	}
}
