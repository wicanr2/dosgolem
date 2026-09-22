package buckrogers

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

var storyPage5EventHeader = []string{"event_key", "sequence", "original_length", "original_sha256", "caller", "glyph_guard", "background", "foreground", "row", "column", "entry_step", "post_call_step", "evidence_level", "catalog_status"}
var storyPage5Approved = [5]struct {
	length uint8
	digest string
}{
	{34, "03a272d4abbd18dec91119382124d6c884f0e3774282ea09da5d99398cf80198"},
	{38, "db72900ffc61c01cb49f5a3919819c4069a2582a9177dcf57917399d3f39abe1"},
	{35, "d688b262bd262f45a25d351f8b85c919af3254b41944086f1f595f15bbd3878a"},
	{36, "0b9c76df8c266950d6fcf8cb4dea52c4f9ccf91e41a8248095e1649b874e0a16"},
	{25, "1572bf767f5ecfe323e56db37bdd2d96051aecc186203677c4f5e4db75b222c9"},
}

// LoadStoryPage5Catalog accepts only the reviewed exact READY five-line pair.
func LoadStoryPage5Catalog(eventsName string, eventsData []byte, textName string, textData []byte) (*StoryPage5Catalog, map[string]string, error) {
	events, e := readTSV(eventsName, eventsData, storyPage5EventHeader)
	if e != nil {
		return nil, nil, e
	}
	texts, e := readTSV(textName, textData, storyOpeningTextHeader)
	if e != nil {
		return nil, nil, e
	}
	if len(events) != 5 || len(texts) != 5 {
		return nil, nil, fmt.Errorf("buckrogers: 第 5 頁 catalog 必須各有五行")
	}
	tr := map[string]string{}
	for _, r := range texts {
		if r[0] == "" || r[1] == "" || tr[r[0]] != "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 5 頁譯文 key 無效")
		}
		tr[r[0]] = r[1]
	}
	es := make([]StoryPage5Identity, 5)
	for i, r := range events {
		key := fmt.Sprintf("story.page5.line.%03d", i+1)
		if r[0] != key || r[1] != strconv.Itoa(i+1) || r[12] != "proven" || r[13] != "READY" || r[4] != "0763:04FF" || r[5] != "0763:026B" || tr[r[0]] == "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 5 頁非 READY identity")
		}
		n, x := strconv.Atoi(r[2])
		if x != nil || n < 1 || n > 255 || uint8(n) != storyPage5Approved[i].length || r[3] != storyPage5Approved[i].digest {
			return nil, nil, fmt.Errorf("buckrogers: 第 5 頁原文 identity 不符已審核收據")
		}
		b, x := hex.DecodeString(r[3])
		if x != nil || len(b) != 32 {
			return nil, nil, fmt.Errorf("buckrogers: 第 5 頁 hash 無效")
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
		es[i] = StoryPage5Identity{uint8(i + 1), r[0], uint8(n), h, Address{0x0763, 0x04ff}, storyGlyphPrimitive, 1, 1, bg, fg, row, col}
	}
	c, e := NewStoryPage5Catalog(es)
	return c, tr, e
}
