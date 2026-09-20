package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMenuCatalogFlagsArePaired(t *testing.T) {
	for _, tc := range []struct {
		events, translations string
		valid                bool
	}{
		{"", "", true},
		{"events.tsv", "translations.tsv", true},
		{"events.tsv", "", false},
		{"", "translations.tsv", false},
	} {
		if got := validateMenuCatalogFlags(tc.events, tc.translations) == nil; got != tc.valid {
			t.Fatalf("events=%q translations=%q valid=%v，要 %v", tc.events, tc.translations, got, tc.valid)
		}
	}
}

func TestAllCatalogFlagsArePaired(t *testing.T) {
	tests := []struct {
		menuEvents, menuTranslations, genderEvents, genderTranslations string
		valid                                                          bool
	}{
		{"", "", "", "", true},
		{"m.tsv", "mt.tsv", "", "", true},
		{"", "", "g.tsv", "gt.tsv", true},
		{"m.tsv", "mt.tsv", "g.tsv", "gt.tsv", true},
		{"m.tsv", "", "", "", false},
		{"", "", "g.tsv", "", false},
		{"", "", "", "gt.tsv", false},
	}
	for _, tc := range tests {
		got := validateCatalogFlags(tc.menuEvents, tc.menuTranslations, tc.genderEvents, tc.genderTranslations) == nil
		if got != tc.valid {
			t.Fatalf("flags %#v valid=%v，要 %v", tc, got, tc.valid)
		}
	}
}

func TestEmitReceiptWritesIdenticalBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	var stdout bytes.Buffer
	value := struct {
		Count int `json:"count"`
	}{Count: 14}
	if err := emitReceipt(&stdout, path, value); err != nil {
		t.Fatal(err)
	}
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), file) || string(file) != "{\"count\":14}\n" {
		t.Fatalf("stdout=%q file=%q", stdout.Bytes(), file)
	}
}

func TestEmitReceiptRejectsOutputDirectory(t *testing.T) {
	if err := emitReceipt(&bytes.Buffer{}, t.TempDir(), struct{}{}); err == nil {
		t.Fatal("receipt-out 指向目錄時必須失敗")
	}
}

func TestParseScheduledBIOSKey(t *testing.T) {
	got, err := parseScheduledBIOSKey("100010000:50:00")
	if err != nil || got != (scheduledBIOSKey{100010000, 0x50, 0x00}) {
		t.Fatalf("parse = %#v, %v", got, err)
	}
	for _, value := range []string{
		"", "0:50:00", "1", "1:50", "1:50:00:00", "x:50:00",
		"1:5:00", "1:050:00", "1:GG:00", "1:5A:00", "1:50:0A",
	} {
		if _, err := parseScheduledBIOSKey(value); err == nil {
			t.Errorf("應拒絕 %q", value)
		}
	}
}

func TestMergeBIOSKeySchedule(t *testing.T) {
	keys, err := mergeBIOSKeySchedule(20, []scheduledBIOSKey{{30, 0x48, 0}, {10, 0x50, 0}}, 40)
	if err != nil {
		t.Fatal(err)
	}
	want := []scheduledBIOSKey{{10, 0x50, 0}, {20, 0x1C, 0x0D}, {30, 0x48, 0}}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys[%d] = %#v，要 %#v", i, keys[i], want[i])
		}
	}
	for _, tc := range []struct {
		enter uint64
		keys  []scheduledBIOSKey
		until uint64
	}{
		{10, []scheduledBIOSKey{{10, 0x50, 0}}, 20},
		{0, []scheduledBIOSKey{{20, 0x50, 0}}, 20},
	} {
		if _, err := mergeBIOSKeySchedule(tc.enter, tc.keys, tc.until); err == nil {
			t.Fatal("重複 step 或 until 邊界應拒絕")
		}
	}
}

func TestWriteIndexedScreen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screen.bin")
	data := make([]byte, 320*200)
	data[12345] = 0x0F
	if err := writeIndexedScreen(path, data); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || len(got) != len(data) || got[12345] != 0x0F {
		t.Fatalf("screen len=%d err=%v", len(got), err)
	}
	if err := writeIndexedScreen(path, data[:len(data)-1]); err == nil {
		t.Fatal("短 framebuffer 應拒絕")
	}
}
