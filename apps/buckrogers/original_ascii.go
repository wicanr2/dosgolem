package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 031 (Buck repo): the generic families draw Latin letters and digits
// with the original 8×8 text glyphs, taken from emulated memory. Only the
// hashes below live in the source; the glyph bytes never leave memory.
const (
	origASCIITableLen  = 64 * 8
	origASCIIPrefixCRC = 0xc9799eae // CRC-32 of the first 16 bytes
	origASCIISHA256    = "83a33300409a76191d6d8224bf154ec2a39ccd0198052d55075c8b43b032631e"
	origASCIIScanLimit = 0x100000
)

// origASCIIIndex maps a character to its glyph index in the original table
// (phase-264 §3), or -1 when the table has no glyph for it.
func origASCIIIndex(r rune) int {
	switch {
	case r >= 0x21 && r <= 0x5F:
		return int(r & 0x3F)
	case r >= 'a' && r <= 'z':
		return int(r - 0x60)
	}
	return -1
}

// FindOriginalASCII searches memory for the 512-byte text glyph table.
func FindOriginalASCII(m MemReader, limit uint32) ([]byte, bool) {
	want, _ := hex.DecodeString(origASCIISHA256)
	return findGlyphTable(m, limit, origASCIIPrefixCRC, want)
}

func findGlyphTable(m MemReader, limit uint32, prefixCRC uint32, want []byte) ([]byte, bool) {
	var win [16]byte
	for a := uint32(0); a+origASCIITableLen <= limit; a++ {
		for i := range win {
			win[i] = m.Read8(a + uint32(i))
		}
		if crc32.ChecksumIEEE(win[:]) != prefixCRC {
			continue
		}
		t := make([]byte, origASCIITableLen)
		for i := range t {
			t[i] = m.Read8(a + uint32(i))
		}
		if sum := sha256.Sum256(t); string(sum[:]) == string(want) {
			return t, true
		}
	}
	return nil, false
}

// OriginalASCIIFont copies a 16×16 font and replaces the characters the
// original table covers with its glyphs doubled to 16×16.
func OriginalASCIIFont(base *xlate.Font, table []byte) *xlate.Font {
	f := &xlate.Font{W: base.W, H: base.H, Name: base.Name + ".orig-ascii", Glyphs: make(map[rune][]byte, len(base.Glyphs))}
	for r, g := range base.Glyphs {
		f.Glyphs[r] = g
	}
	if base.W != 16 || base.H != 16 || len(table) != origASCIITableLen {
		return f
	}
	for r := rune(0x21); r <= 0x7E; r++ {
		i := origASCIIIndex(r)
		if i < 0 {
			continue
		}
		g := make([]byte, 32)
		for y := 0; y < 8; y++ {
			b := table[i*8+y]
			var row uint16
			for x := 0; x < 8; x++ {
				if b&(0x80>>x) != 0 {
					row |= 0xC000 >> (2 * x)
				}
			}
			for dy := 0; dy < 2; dy++ {
				g[(2*y+dy)*2], g[(2*y+dy)*2+1] = byte(row>>8), byte(row)
			}
		}
		f.Glyphs[r] = g
	}
	return f
}
