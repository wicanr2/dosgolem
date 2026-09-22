package buckrogers

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

var storyPage2EventHeader = []string{"event_key", "sequence", "original_length", "original_sha256", "caller", "glyph_guard", "background", "foreground", "row", "column", "entry_step", "post_call_step", "evidence_level", "catalog_status"}

// LoadStoryPage2Catalog accepts only the reviewed READY four-line pair.
func LoadStoryPage2Catalog(eventsName string, eventsData []byte, textName string, textData []byte) (*StoryPage2Catalog, map[string]string, error) {
	events, e := readTSV(eventsName, eventsData, storyPage2EventHeader)
	if e != nil {
		return nil, nil, e
	}
	texts, e := readTSV(textName, textData, storyOpeningTextHeader)
	if e != nil {
		return nil, nil, e
	}
	if len(events) != 4 || len(texts) != 4 {
		return nil, nil, fmt.Errorf("buckrogers: 第 2 頁 catalog 必須各有四行")
	}
	tr := map[string]string{}
	for _, r := range texts {
		if r[0] == "" || r[1] == "" || tr[r[0]] != "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 2 頁譯文 key 無效")
		}
		tr[r[0]] = r[1]
	}
	es := make([]StoryPage2Identity, 4)
	for i, r := range events {
		if r[12] != "confirmed" || r[13] != "READY" || tr[r[0]] == "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 2 頁非 READY identity")
		}
		n, x := strconv.Atoi(r[2])
		if x != nil || n < 1 || n > 255 {
			return nil, nil, fmt.Errorf("buckrogers: 第 2 頁長度無效")
		}
		b, x := hex.DecodeString(r[3])
		if x != nil || len(b) != 32 {
			return nil, nil, fmt.Errorf("buckrogers: 第 2 頁 hash 無效")
		}
		var h [32]byte
		copy(h[:], b)
		parse := func(s string) (uint8, error) {
			v, z := strconv.Atoi(s)
			if z != nil || v < 0 || v > 255 {
				return 0, fmt.Errorf("byte")
			}
			return uint8(v), nil
		}
		bg, z := parse(r[6])
		if z != nil {
			return nil, nil, z
		}
		fg, z := parse(r[7])
		if z != nil {
			return nil, nil, z
		}
		row, z := parse(r[8])
		if z != nil {
			return nil, nil, z
		}
		col, z := parse(r[9])
		if z != nil {
			return nil, nil, z
		}
		es[i] = StoryPage2Identity{uint8(i + 1), r[0], uint8(n), h, Address{0x0763, 0x04ff}, storyGlyphPrimitive, 1, 1, bg, fg, row, col}
	}
	c, e := NewStoryPage2Catalog(es)
	return c, tr, e
}
