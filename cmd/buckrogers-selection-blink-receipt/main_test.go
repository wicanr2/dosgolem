package main

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestRegionHashesAndCountsExactRectangle(t *testing.T) {
	indexed := make([]byte, 320*200)
	indexed[24*320+24] = 15
	indexed[31*320+71] = 10
	got := region(indexed, 24, 72, 24, 32)
	if len(got.Counts) != 3 || got.Counts[0] != 382 || got.Counts[10] != 1 || got.Counts[15] != 1 {
		t.Fatalf("counts = %#v", got.Counts)
	}
	wantBytes := make([]byte, 48*8)
	wantBytes[0], wantBytes[len(wantBytes)-1] = 15, 10
	if got.SHA256 != hash(wantBytes) {
		t.Fatalf("hash = %s，要 %s", got.SHA256, hash(wantBytes))
	}
}

func TestTakeSampleReportsPaletteContrast(t *testing.T) {
	m := machine.New()
	m.DAC[0], m.DAC[1], m.DAC[2] = 0, 0, 0
	m.DAC[15*3], m.DAC[15*3+1], m.DAC[15*3+2] = 0, 0, 0
	// Palette() 會做 6→8 轉換；index 15 先與 0 相同。
	got := takeSample(m, 0)
	if got.SelectedContrast {
		t.Fatal("相同 palette 0／15 不得有 contrast")
	}
	m.DAC[15*3] = 63
	got = takeSample(m, 0)
	if !got.SelectedContrast || got.Palette[15].R != 255 {
		t.Fatalf("palette contrast 未反映 DAC：%+v", got.Palette[15])
	}
}

func TestHashDeterministic(t *testing.T) {
	data := bytes.Repeat([]byte{1, 2, 3}, 10)
	if hash(data) != hash(append([]byte(nil), data...)) {
		t.Fatal("相同 bytes 的 hash 不一致")
	}
}
