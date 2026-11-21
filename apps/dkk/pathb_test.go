package dkk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

const (
	bEntry = 0x63EB // 05F4:04AB 的線性位址（合成）
	bDS    = 0x0CDE
	bSS    = 0x1CBD
	bSP    = 0x3F00
	bStr   = 0x5777
)

func plantB(m fakeMem) {
	for i, x := range PathBSignature.Bytes {
		m[bEntry+uint32(i)] = x
	}
}

// bCallSetup 依規格 002 §1 放一次呼叫：返回位址、字串遠指標、旗標、bg、fg、底、右、頂、左（高位元組填殘值），
// 並設定原版游標。
func bCallSetup(m fakeMem, text string, flag int, r Rect, curRow, curCol int) {
	base := linear(bStr, 8)
	m[base] = byte(len(text))
	copy(m[base+1:], text)
	s := linear(bSS, bSP)
	m.put16(s, 0x0B90)
	m.put16(s+2, 0x3140)
	m.put16(s+4, 8)
	m.put16(s+6, bStr)
	for i, v := range []int{flag, 0, 10, r.Bottom, r.Right, r.Top, r.Left} {
		m.put16(s+8+uint32(2*i), 0x7200|uint16(v))
	}
	m[linear(bDS, cursorColAddr)] = byte(curCol)
	m[linear(bDS, cursorRowAddr)] = byte(curRow)
}

func catalogOf(t *testing.T, pairs ...string) *Catalog {
	t.Helper()
	var ev, tr strings.Builder
	ev.WriteString("event_key\toriginal_length\toriginal_sha256\tsources\n")
	tr.WriteString("key\ttranslation\tsource\n")
	for i := 0; i+1 < len(pairs); i += 2 {
		sum := sha256.Sum256([]byte(pairs[i]))
		key := fmt.Sprintf("k%d", i/2)
		fmt.Fprintf(&ev, "%s\t%d\t%s\tX:1\n", key, len(pairs[i]), hex.EncodeToString(sum[:]))
		fmt.Fprintf(&tr, "%s\t%s\tt\n", key, pairs[i+1])
	}
	c, err := LoadCatalog([]byte(ev.String()), []byte(tr.String()))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func newB(t *testing.T, cat *Catalog) (*PathB, fakeMem, *xlate.Layer) {
	t.Helper()
	m := make(fakeMem, 0x100000)
	plantB(m)
	layer := &xlate.Layer{}
	b, err := NewPathB(m, &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, layer, cat)
	if err != nil {
		t.Fatal(err)
	}
	return b, m, layer
}

var story = Rect{Top: 17, Left: 1, Bottom: 22, Right: 38}

func call(b *PathB, m fakeMem, step uint64) {
	b.Before(step, 0x05F4, 0x04AB, bSS, bSP, bDS, m)
}

func ret(b *PathB, m fakeMem) {
	b.Before(0, 0x3140, 0x0B90, bSS, bSP+4+pathBArgBytes, bDS, m)
}

func rowText(l *xlate.Layer, row int) string {
	for _, s := range l.Stamps {
		if s.Y == row*8 {
			return strings.TrimRight(string(s.Text), " ")
		}
	}
	return ""
}

func TestPathBNewPageAndContinuation(t *testing.T) {
	b, m, l := newB(t, catalogOf(t, "EVEN AFTER ", "即使已擊敗麥坦尼，終結了", "THE THREAT.", "龍人的威脅。"))
	bCallSetup(m, "EVEN AFTER ", 1, story, 0, 0)
	call(b, m, 1)
	ret(b, m)
	if got := rowText(l, 17); got != "即使已擊敗麥坦尼，終結了" {
		t.Fatalf("第 17 列 %q", got)
	}
	// 旗標 0、原版游標在視窗內且不在左欄：接在中文結尾之後
	bCallSetup(m, "THE THREAT.", 0, story, 17, 12)
	call(b, m, 2)
	if got := rowText(l, 17); got != "即使已擊敗麥坦尼，終結了龍人的威脅。" {
		t.Fatalf("接續後第 17 列 %q", got)
	}
	for _, s := range l.Stamps {
		if s.State != xlate.Shown || !l.Frozen(s) {
			t.Fatal("路徑 B 的 Stamp 應為 Shown 且凍結")
		}
	}
}

func TestPathBMissPassthroughAndNewPageMiss(t *testing.T) {
	b, m, l := newB(t, catalogOf(t, "attacks", "攻擊"))
	right := Rect{Top: 11, Left: 23, Bottom: 21, Right: 38}
	bCallSetup(m, "attacks", 1, right, 0, 0)
	call(b, m, 1)
	bCallSetup(m, " DREAD WOLF", 0, right, 11, 30)
	call(b, m, 2)
	if got := rowText(l, 11); got != "攻擊 DREAD WOLF" {
		t.Fatalf("未命中的接續應照原文接入：%q", got)
	}
	bCallSetup(m, "UNKNOWN", 1, right, 0, 0)
	call(b, m, 3)
	if len(l.Stamps) != 0 || len(b.pages) != 0 {
		t.Fatal("旗標非零的未命中應移除該頁")
	}
}

func TestPathBOverflowFailsClosed(t *testing.T) {
	small := Rect{Top: 13, Left: 23, Bottom: 13, Right: 26}
	b, m, l := newB(t, catalogOf(t, "X", "這一句太長放不下"))
	bCallSetup(m, "X", 1, small, 0, 0)
	call(b, m, 1)
	if len(l.Stamps) != 0 || b.Hits[0].Rejected != "溢出" {
		t.Fatalf("溢出應不畫：%+v", b.Hits)
	}
}

func TestPathBWritesInsideCallIgnoredOutsideRemoveRow(t *testing.T) {
	b, m, l := newB(t, catalogOf(t, "HELLO", "你好"))
	bCallSetup(m, "HELLO", 1, story, 0, 0)
	call(b, m, 1)
	off := uint32(17*8*planarStride + 3) // 第 17 列、第 3 欄
	b.VideoWrite(off)
	if len(l.Stamps) != story.Bottom-story.Top+1 {
		t.Fatalf("呼叫期間的寫入不該失效；遮罩應到底列：%d 列", len(l.Stamps))
	}
	ret(b, m)
	n := len(l.Stamps)
	b.VideoWrite(uint32(5*8*planarStride + 3)) // 別處的寫入
	if len(l.Stamps) != n {
		t.Fatal("遮罩外的寫入不該失效")
	}
	b.VideoWrite(off)
	if len(l.Stamps) != n-1 || rowText(l, 17) != "" {
		t.Fatal("呼叫期間外落在遮罩列的寫入應移除該列")
	}
}

func TestPathBNewPageRemovesIntersectingRows(t *testing.T) {
	b, m, l := newB(t, catalogOf(t, "A", "甲", "B", "乙"))
	big := Rect{Top: 2, Left: 23, Bottom: 21, Right: 38}
	bCallSetup(m, "A", 1, big, 0, 0)
	call(b, m, 1)
	ret(b, m)
	other := Rect{Top: 13, Left: 23, Bottom: 16, Right: 38}
	bCallSetup(m, "B", 1, other, 0, 0)
	call(b, m, 2)
	if rowText(l, 2) != "甲" || rowText(l, 13) != "乙" {
		t.Fatalf("不相交的列應保留、新頁應建立：2=%q 13=%q", rowText(l, 2), rowText(l, 13))
	}
	bCallSetup(m, "B", 1, Rect{Top: 2, Left: 23, Bottom: 3, Right: 38}, 0, 0)
	call(b, m, 3)
	if rowText(l, 2) != "乙" {
		t.Fatalf("相交列應被新頁取代：%q", rowText(l, 2))
	}
}

func TestLayoutKeepsLatinWordsAndPunctuation(t *testing.T) {
	p := &bPage{rect: Rect{Top: 0, Left: 0, Bottom: 3, Right: 5}, grid: map[int][]rune{}, stamps: map[int]*xlate.Stamp{}}
	if !layout(p, []rune("一二三四 ABCDE")) {
		t.Fatal("不該溢出")
	}
	if string(p.grid[1][:5]) != "ABCDE" {
		t.Fatalf("拉丁詞應整個換到下一列：%q", string(p.grid[1]))
	}
	q := &bPage{rect: Rect{Top: 0, Left: 0, Bottom: 3, Right: 2}, grid: map[int][]rune{}, stamps: map[int]*xlate.Stamp{}}
	layout(q, []rune("甲乙丙。"))
	if q.grid[1][0] != '丙' || q.grid[1][1] != '。' {
		t.Fatalf("句號不放行首，應連同前一字換行：%q／%q", string(q.grid[0]), string(q.grid[1]))
	}
}
