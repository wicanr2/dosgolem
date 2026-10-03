package phantasie

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// posWrites 重現 25A5 對第 row 列、第 col 欄、n 個字元的寫入：4 輪 × 2 個 bank，每輪一條掃描線對。
func posWrites(row, col, n int) []VideoWrite {
	var ws []VideoWrite
	for k := 0; k < 4; k++ {
		off := uint16((4*row+k)*80 + col*2)
		ws = append(ws, VideoWrite{Seg: 0xB800, Off: off, Count: uint16(2 * n)},
			VideoWrite{Seg: 0xBA00, Off: off, Count: uint16(2 * n)})
	}
	return ws
}

func TestFootprint(t *testing.T) {
	// 第 13 列第 7 欄 21 個字元：x 56 至 224，y 104 至 112（字面值）。
	x0, y0, x1, y1, ok := Footprint(posWrites(13, 7, 21))
	if !ok || x0 != 56 || y0 != 104 || x1 != 224 || y1 != 112 {
		t.Fatalf("得 %d,%d,%d,%d,%v，期望 56,104,224,112,true", x0, y0, x1, y1, ok)
	}
	// 第 0 列第 0 欄 1 個字元（單字元事件）。
	x0, y0, x1, y1, ok = Footprint(posWrites(0, 0, 1))
	if !ok || x0 != 0 || y0 != 0 || x1 != 8 || y1 != 8 {
		t.Fatalf("得 %d,%d,%d,%d,%v，期望 0,0,8,8,true", x0, y0, x1, y1, ok)
	}
	// 缺一條掃描線（少一個 bank 的寫入）：不連續，回 ok=false。
	ws := posWrites(5, 3, 4)
	if _, _, _, _, ok := Footprint(ws[:len(ws)-1]); ok {
		t.Fatal("缺一條掃描線時應為 ok=false")
	}
	// 各掃描線範圍不同：ok=false。
	ws = posWrites(5, 3, 4)
	ws[2].Count = 4
	if _, _, _, _, ok := Footprint(ws); ok {
		t.Fatal("各掃描線範圍不同時應為 ok=false")
	}
	// 非視訊段的寫入被忽略；沒有任何視訊寫入回 ok=false。
	if _, _, _, _, ok := Footprint([]VideoWrite{{Seg: 0x1234, Off: 0, Count: 2}}); ok {
		t.Fatal("沒有視訊寫入時應為 ok=false")
	}
}

func posStamp(x, y, cells, cw int) *xlate.Stamp {
	return &xlate.Stamp{X: x, Y: y, Cells: cells, CellW: cw, CellH: 8}
}

func TestCheckPosition(t *testing.T) {
	rec := &EventRecord{Col: 7, Row: 13, Text: strings.Repeat("a", 21)}
	ws := posWrites(13, 7, 21)
	// 全形段 3 格（24 像素）加半形段 36 格（144 像素）恰好蓋滿 168 像素。
	good := []*xlate.Stamp{posStamp(56, 104, 3, 8), posStamp(80, 104, 36, 4)}
	if why := CheckPosition(rec, good, ws); why != "" {
		t.Fatalf("正確位置被判錯：%s", why)
	}
	cases := []struct {
		name   string
		stamps []*xlate.Stamp
		ws     []VideoWrite
		want   string
	}{
		{"疊字少一段", good[:1], ws, "沒有蓋滿"},
		{"疊字超出右緣", []*xlate.Stamp{posStamp(56, 104, 3, 8), posStamp(80, 104, 37, 4)}, ws, "超出寫入足跡"},
		{"疊字列錯一條", []*xlate.Stamp{posStamp(56, 105, 3, 8), posStamp(80, 105, 36, 4)}, ws, "超出寫入足跡"},
		{"事件座標與足跡不同", good, posWrites(13, 8, 21), "與寫入足跡"},
		{"沒有寫入足跡", good, nil, "足跡"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if why := CheckPosition(rec, c.stamps, c.ws); !strings.Contains(why, c.want) {
				t.Fatalf("得 %q，期望含 %q", why, c.want)
			}
		})
	}
	// 裁切事件（Col+len > 40）略過。
	clip := &EventRecord{Col: 30, Row: 2, Text: strings.Repeat("a", 20)}
	if why := CheckPosition(clip, nil, nil); why != "" {
		t.Fatalf("裁切事件應略過，得 %q", why)
	}
}
