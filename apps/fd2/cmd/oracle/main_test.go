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
