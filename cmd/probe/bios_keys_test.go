package main

import "testing"

func TestParseBIOSKeysNamedDirections(t *testing.T) {
	got, err := parseBIOSKeys("a\\n", "Down, Enter")
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]byte{{0x1E, 'a'}, {0x1C, '\r'}, {0x50, 0}, {0x1C, '\r'}}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Scan != want[i][0] || got[i].ASCII != want[i][1] {
			t.Errorf("key %d = {%02X,%02X}, want {%02X,%02X}",
				i, got[i].Scan, got[i].ASCII, want[i][0], want[i][1])
		}
	}
}

func TestParseBIOSKeysRejectsUnknownAndEmptyNames(t *testing.T) {
	for _, names := range []string{"Bogus", "Down,,Enter"} {
		if _, err := parseBIOSKeys("", names); err == nil {
			t.Errorf("parseBIOSKeys(%q) unexpectedly succeeded", names)
		}
	}
}
