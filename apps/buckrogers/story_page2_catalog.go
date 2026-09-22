package buckrogers

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

var storyPage2EventHeader = []string{"event_key", "sequence", "original_length", "original_sha256", "caller", "glyph_guard", "background", "foreground", "row", "column", "entry_step", "post_call_step", "evidence_level", "catalog_status"}

// The reviewed READY pair is an exact original-output identity, not merely a
// four-row TSV with a READY label. These are content-safe length/digest anchors.
var storyPage2Approved = [4]struct {
	length uint8
	digest string
}{
	{37, "d5e1af5a30c6c4954aec9e63c0ab8454f5b451d333b9c58c0b66a89c43733364"},
	{31, "8834f68a0a932af699b5f090e02a652184341ef8c107ca30d714097b6f9e91c7"},
	{38, "57910b8786c4b13181ed37f12b273e7f43886747dbab9fb360f09071ae39c7bd"},
	{37, "a114f1a715a848c0280677ab70d806145ae5e1991f6f570f5ace5d49c9dc85b4"},
}

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
		wantKey := fmt.Sprintf("story.page2.line.%03d", i+1)
		if r[0] != wantKey || r[1] != strconv.Itoa(i+1) || r[12] != "confirmed" || r[13] != "READY" ||
			r[4] != "0763:04FF" || r[5] != "0763:026B" || tr[r[0]] == "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 2 頁非 READY identity")
		}
		n, x := strconv.Atoi(r[2])
		if x != nil || n < 1 || n > 255 {
			return nil, nil, fmt.Errorf("buckrogers: 第 2 頁長度無效")
		}
		if uint8(n) != storyPage2Approved[i].length || r[3] != storyPage2Approved[i].digest {
			return nil, nil, fmt.Errorf("buckrogers: 第 2 頁原文 identity 不符已審核收據")
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
