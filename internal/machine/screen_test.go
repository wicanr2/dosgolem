package machine

import "testing"

func TestTextScreenReadsCharactersNotAttributes(t *testing.T) {
	m := New()
	m.Write16(0x44A, 80)
	for i, ch := range []byte("HELLO") {
		m.Write8(TextSeg*16+uint32(i)*2, ch)
		m.Write8(TextSeg*16+uint32(i)*2+1, 0x07) // 屬性，不該出現在結果裡
	}
	rows := m.TextScreen(0)
	if len(rows) != 25 {
		t.Fatalf("回了 %d 列", len(rows))
	}
	if rows[0] != "HELLO" {
		t.Errorf("第 0 列是 %q", rows[0])
	}
}

func TestTextScreenKeepsBlankRows(t *testing.T) {
	m := New()
	m.Write8(TextSeg*16+uint32(3*80)*2, 'X')
	rows := m.TextScreen(80)
	// 「第幾列有東西」本身是資訊，壓掉空白列會讓畫面對不起來。
	if rows[0] != "" || rows[3] != "X" {
		t.Errorf("列 0=%q 列 3=%q", rows[0], rows[3])
	}
}

func TestTextScreenFallsBackWhenBDAIsNonsense(t *testing.T) {
	m := New()
	m.Write16(0x44A, 9999)
	if got := len(m.TextScreen(0)); got != 25 {
		t.Fatalf("回了 %d 列", got)
	}
}

func TestFindReportsEveryHit(t *testing.T) {
	m := New()
	pat := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	m.WriteBytes(0x1000, pat)
	m.WriteBytes(0x50000, pat)
	hits := m.Find(pat)
	if len(hits) != 2 || hits[0] != 0x1000 || hits[1] != 0x50000 {
		t.Fatalf("找到 %v", hits)
	}
}

func TestFindRefusesAnEmptyPattern(t *testing.T) {
	// 回「到處都是」比回「找不到」更難查。
	if hits := New().Find(nil); hits != nil {
		t.Fatalf("空指紋回了 %d 個位址", len(hits))
	}
}

func TestSegmentBytesReadsFromTheRightPlace(t *testing.T) {
	m := New()
	m.WriteBytes(0x12340, []byte{1, 2, 3, 4})
	got := m.SegmentBytes(0x1234, 4)
	for i, w := range []byte{1, 2, 3, 4} {
		if got[i] != w {
			t.Fatalf("讀出 % X", got)
		}
	}
}

func TestSegmentBytesRefusesToRunOffTheEnd(t *testing.T) {
	// 回一段補零的東西會被當成「那裡真的是零」。
	if b := New().SegmentBytes(0xFFFF, 0x1000); b != nil {
		t.Fatalf("越界卻回了 %d 個 byte", len(b))
	}
}

func TestSegmentBytesAllowsAnExactFitAtTheTop(t *testing.T) {
	// 上界差一格的症狀是「最後一段讀不出來」而且回 nil，
	// 看起來像那一段不存在。
	m := New()
	const top = uint16(0xFFFF) // 線性 FFFF0，剩 16 個 byte
	if b := m.SegmentBytes(top, 16); len(b) != 16 {
		t.Errorf("剛好放得下卻回了 %d 個 byte", len(b))
	}
	if b := m.SegmentBytes(top, 17); b != nil {
		t.Errorf("超出一個 byte 卻回了 %d 個", len(b))
	}
}

func TestFindReportsOverlappingHits(t *testing.T) {
	// 重疊的命中要全部回報。跳過已命中的長度會讓 "aaa" 裡的第二個 "aa" 消失，
	// 而那看起來像「只出現一次」。
	m := New()
	m.WriteBytes(0x2000, []byte{0xAA, 0xAA, 0xAA})
	if hits := m.Find([]byte{0xAA, 0xAA}); len(hits) != 2 {
		t.Fatalf("找到 %d 個，該是 2：%v", len(hits), hits)
	}
}
