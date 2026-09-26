package buckrogers

import (
	"crypto/sha256"
	"hash/crc32"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// syntheticTable is not the original: glyph i is filled with byte i+1.
func syntheticTable() []byte {
	t := make([]byte, origASCIITableLen)
	for i := range t {
		t[i] = byte(i/8 + 1)
	}
	return t
}

func TestFindGlyphTableByHash(t *testing.T) {
	tab := syntheticTable()
	m := make(fakeMem, 1<<16)
	copy(m[0x3211:], tab)
	sum := sha256.Sum256(tab)
	got, ok := findGlyphTable(m, uint32(len(m)), crc32.ChecksumIEEE(tab[:16]), sum[:])
	if !ok || string(got) != string(tab) {
		t.Fatal("table not found")
	}
	m[0x3211+100] ^= 0xFF // prefix still matches, hash no longer does
	if _, ok := findGlyphTable(m, uint32(len(m)), crc32.ChecksumIEEE(tab[:16]), sum[:]); ok {
		t.Fatal("hash mismatch accepted")
	}
	if _, ok := FindOriginalASCII(m, uint32(len(m))); ok {
		t.Fatal("synthetic memory matched the original hash")
	}
}

func TestOriginalASCIIFontMapping(t *testing.T) {
	base := &xlate.Font{W: 16, H: 16, Name: "base", Glyphs: map[rune][]byte{'中': make([]byte, 32), '{': make([]byte, 32)}}
	base.Glyphs['中'][0] = 0x5A
	f := OriginalASCIIFont(base, syntheticTable())
	if f.Name != "base.orig-ascii" || f.Glyphs['中'][0] != 0x5A || len(base.Glyphs) != 2 {
		t.Fatal("base font changed or name wrong")
	}
	for r, idx := range map[rune]int{'@': 0, 'A': 1, 'a': 1, 'z': 26, '0': 48, '(': 40, '?': 63} {
		v := byte(idx + 1) // synthetic row byte
		var want uint16
		for x := 0; x < 8; x++ {
			if v&(0x80>>x) != 0 {
				want |= 0xC000 >> (2 * x)
			}
		}
		g := f.Glyphs[r]
		if g == nil || uint16(g[0])<<8|uint16(g[1]) != want || uint16(g[30])<<8|uint16(g[31]) != want {
			t.Errorf("%q: %x want %x", r, g[:2], want)
		}
	}
	if f.Glyphs['{'][0] != 0 || origASCIIIndex('{') != -1 || origASCIIIndex('`') != -1 {
		t.Error("characters outside the table must keep the base glyph")
	}
}
