package buckrogers

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

var storyPage3EventHeader = []string{"event_key", "sequence", "original_length", "original_sha256", "caller", "glyph_guard", "background", "foreground", "row", "column", "entry_step", "post_call_step", "evidence_level", "catalog_status"}

// The reviewed READY pair is an exact original-output identity, not merely a
// five-row TSV with a READY label. These are content-safe length/digest anchors.
var storyPage3Approved = [5]struct {
	length uint8
	digest string
}{
	{34, "84c6fba5f613f52a02e935f0937bb66eea6632840d9465cf8fc9c3d7edd0dbff"},
	{37, "6ea1adb513486d4687926d744f1d9ff22638ee3a9eaeef6fe6ff97282c361536"},
	{31, "fd9dbc141cf71e523e4f8a45fe5ced142f55d5257f5801f4259142162bf5f37a"},
	{37, "93c17e4396f05f1e17c659f42233369c205ae8f23280c796e0bf8b023873479d"},
	{5, "8f0cfc1b387ddcad6870b6a960f67f71d044bd7163d01db49e753b6e71afb7d5"},
}

// LoadStoryPage3Catalog accepts only the reviewed READY five-line pair.
func LoadStoryPage3Catalog(eventsName string, eventsData []byte, textName string, textData []byte) (*StoryPage3Catalog, map[string]string, error) {
	events, e := readTSV(eventsName, eventsData, storyPage3EventHeader)
	if e != nil {
		return nil, nil, e
	}
	texts, e := readTSV(textName, textData, storyOpeningTextHeader)
	if e != nil {
		return nil, nil, e
	}
	if len(events) != 5 || len(texts) != 5 {
		return nil, nil, fmt.Errorf("buckrogers: 第 3 頁 catalog 必須各有五行")
	}
	tr := map[string]string{}
	for _, r := range texts {
		if r[0] == "" || r[1] == "" || tr[r[0]] != "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 3 頁譯文 key 無效")
		}
		tr[r[0]] = r[1]
	}
	es := make([]StoryPage3Identity, 5)
	for i, r := range events {
		wantKey := fmt.Sprintf("story.page3.line.%03d", i+1)
		if r[0] != wantKey || r[1] != strconv.Itoa(i+1) || r[12] != "confirmed" || r[13] != "READY" ||
			r[4] != "0763:04FF" || r[5] != "0763:026B" || tr[r[0]] == "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 3 頁非 READY identity")
		}
		n, x := strconv.Atoi(r[2])
		if x != nil || n < 1 || n > 255 {
			return nil, nil, fmt.Errorf("buckrogers: 第 3 頁長度無效")
		}
		if uint8(n) != storyPage3Approved[i].length || r[3] != storyPage3Approved[i].digest {
			return nil, nil, fmt.Errorf("buckrogers: 第 3 頁原文 identity 不符已審核收據")
		}
		b, x := hex.DecodeString(r[3])
		if x != nil || len(b) != 32 {
			return nil, nil, fmt.Errorf("buckrogers: 第 3 頁 hash 無效")
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
		es[i] = StoryPage3Identity{uint8(i + 1), r[0], uint8(n), h, Address{0x0763, 0x04ff}, storyGlyphPrimitive, 1, 1, bg, fg, row, col}
	}
	c, e := NewStoryPage3Catalog(es)
	return c, tr, e
}
