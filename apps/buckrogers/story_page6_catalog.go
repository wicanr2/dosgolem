package buckrogers

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

var storyPage6EventHeader = []string{"event_key", "sequence", "original_length", "original_sha256", "caller", "glyph_guard", "background", "foreground", "row", "column", "entry_step", "post_call_step", "evidence_level", "catalog_status"}
var storyPage6Approved = [6]struct {
	length uint8
	digest string
}{
	{37, "d053056eb6958d1b38d28ff193b8102c7eaa64fb6db0864582d9f060201f463e"},
	{34, "fa20332aaf7be72ca4ba8b7b745a9691a5de12d3f917f07843657a48891f4d01"},
	{36, "0f61c2ab249eed0ca1e8d89ac8f2a6ac0d2886fad2cd8f9312592d0cdf5bb3c1"},
	{38, "2099600ba5c88af0aae899a1739eab8c791dfe1385ca8493c64aabdab27d2a65"},
	{36, "beb4798b2f69f82186693d602ddcd76f30557d6797a45f4b506c797d4b4872cf"},
	{19, "276a43ecca15ed7b57fe5b73f7e9bfefb04eec4a686ddb442fb0fc8712ceb48a"},
}

// LoadStoryPage6Catalog accepts only the independently reviewed READY pair.
func LoadStoryPage6Catalog(eventsName string, eventsData []byte, textName string, textData []byte) (*StoryPage6Catalog, map[string]string, error) {
	events, err := readTSV(eventsName, eventsData, storyPage6EventHeader)
	if err != nil {
		return nil, nil, err
	}
	texts, err := readTSV(textName, textData, storyOpeningTextHeader)
	if err != nil {
		return nil, nil, err
	}
	if len(events) != 6 || len(texts) != 6 {
		return nil, nil, fmt.Errorf("buckrogers: 第 6 頁 catalog 必須各有六行")
	}
	translations := map[string]string{}
	for _, row := range texts {
		if row[0] == "" || row[1] == "" || translations[row[0]] != "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 6 頁譯文 key 無效")
		}
		translations[row[0]] = row[1]
	}
	identities := make([]StoryPage6Identity, 6)
	for i, row := range events {
		key := fmt.Sprintf("story.page6.line.%03d", i+1)
		length, parseErr := strconv.Atoi(row[2])
		if row[0] != key || row[1] != strconv.Itoa(i+1) || parseErr != nil || length != int(storyPage6Approved[i].length) || row[3] != storyPage6Approved[i].digest || row[4] != "0763:04FF" || row[5] != "0763:026B" || row[6] != "0" || row[7] != "10" || row[8] != strconv.Itoa(17+i) || row[9] != "1" || row[12] != "confirmed" || row[13] != "READY" || translations[key] == "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 6 頁非 READY identity")
		}
		decoded, decodeErr := hex.DecodeString(row[3])
		if decodeErr != nil || len(decoded) != 32 {
			return nil, nil, fmt.Errorf("buckrogers: 第 6 頁 hash 無效")
		}
		var digest [32]byte
		copy(digest[:], decoded)
		identities[i] = StoryPage6Identity{Sequence: uint8(i + 1), EventKey: key, OriginalLength: uint8(length), OriginalSHA256: digest, Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1}
	}
	catalog, err := NewStoryPage6Catalog(identities)
	return catalog, translations, err
}
