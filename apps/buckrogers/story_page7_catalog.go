package buckrogers

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

var storyPage7EventHeader = []string{"event_key", "sequence", "original_length", "original_sha256", "caller", "glyph_guard", "background", "foreground", "row", "column", "entry_step", "post_call_step", "evidence_level", "catalog_status"}
var storyPage7Approved = [6]struct {
	n uint8
	h string
}{{36, "aab9b4e77e5b9084aeb3c71af6e986373153552d689e0ed1a04f019ea094fb80"}, {36, "286012ec11f1bc1eddcbf52a62ef47b50edcb19a764235e9655b2c5ea71f0354"}, {37, "367a5f1a691a589cd87deecab045512cf1a99a540c798170522747130e2848e1"}, {38, "3c1411ce5cb9e9665cce5d643d090cb085ac8904d8ea77e25e5879ae20c876d5"}, {38, "c56c5c9e8f6f627d7209170bbec80a08d184101a44b81fd2938e433dd6efacd6"}, {5, "3566467963c88d979218b9230a14086b61d4bc146492ea5ed15a3b5855a7fb03"}}

func LoadStoryPage7Catalog(en string, ed []byte, tn string, td []byte) (*StoryPage7Catalog, map[string]string, error) {
	e, x := readTSV(en, ed, storyPage7EventHeader)
	if x != nil {
		return nil, nil, x
	}
	t, x := readTSV(tn, td, storyOpeningTextHeader)
	if x != nil {
		return nil, nil, x
	}
	if len(e) != 6 || len(t) != 6 {
		return nil, nil, fmt.Errorf("buckrogers: 第 7 頁 catalog 必須各有六行")
	}
	text := map[string]string{}
	for _, r := range t {
		if r[0] == "" || r[1] == "" || text[r[0]] != "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 7 頁譯文 key 無效")
		}
		text[r[0]] = r[1]
	}
	ids := make([]StoryPage7Identity, 6)
	for i, r := range e {
		key := fmt.Sprintf("story.page7.line.%03d", i+1)
		n, z := strconv.Atoi(r[2])
		if r[0] != key || r[1] != strconv.Itoa(i+1) || z != nil || n != int(storyPage7Approved[i].n) || r[3] != storyPage7Approved[i].h || r[4] != "0763:04FF" || r[5] != "0763:026B" || r[6] != "0" || r[7] != "10" || r[8] != strconv.Itoa(17+i) || r[9] != "1" || r[12] != "confirmed" || r[13] != "READY" || text[key] == "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 7 頁非 READY identity")
		}
		b, z := hex.DecodeString(r[3])
		if z != nil || len(b) != 32 {
			return nil, nil, fmt.Errorf("buckrogers: 第 7 頁 hash 無效")
		}
		var h [32]byte
		copy(h[:], b)
		ids[i] = StoryPage7Identity{uint8(i + 1), uint8(n), key, h, Address{0x0763, 0x04ff}, storyGlyphPrimitive}
	}
	c, x := NewStoryPage7Catalog(ids)
	return c, text, x
}
