package buckrogers

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

var storyPage8EventHeader = []string{"event_key", "sequence", "original_length", "original_sha256", "caller", "glyph_guard", "background", "foreground", "row", "column", "entry_step", "post_call_step", "evidence_level", "catalog_status"}

var storyPage8Approved = [4]struct {
	length                  uint8
	hash                    string
	entryStep, postCallStep uint64
}{
	{38, "fe4920d51364241526b326e9dcb2a44100fa891298fdcbe00269af612c5f0cf3", 341020346, 342641523},
	{37, "93e4a1278b2e315c1a37c44c40be891519e0b2de988d6084a729f61209865932", 342684953, 344262194},
	{33, "a0a0128ae4b64bf9396830ca0150ace8a80ab9b0668486e311b8eea9b11e8845", 344305840, 345708016},
	{22, "b174b4ddd624675cd321067f2f0663d72fe82c55405f175b8348309596af6b1f", 345752448, 346672243},
}

func LoadStoryPage8Catalog(eventName string, eventData []byte, translationName string, translationData []byte) (*StoryPage8Catalog, map[string]string, error) {
	eventRows, err := readTSV(eventName, eventData, storyPage8EventHeader)
	if err != nil {
		return nil, nil, err
	}
	translationRows, err := readTSV(translationName, translationData, storyOpeningTextHeader)
	if err != nil {
		return nil, nil, err
	}
	if len(eventRows) != 4 || len(translationRows) != 4 {
		return nil, nil, fmt.Errorf("buckrogers: 第 8 頁 catalog 必須各有四行")
	}
	translations := make(map[string]string, 4)
	for _, row := range translationRows {
		if row[0] == "" || row[1] == "" || row[2] != "runtime-editorial" || translations[row[0]] != "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 8 頁譯文 key 無效")
		}
		translations[row[0]] = row[1]
	}
	identities := make([]StoryPage8Identity, 4)
	for i, row := range eventRows {
		approved := storyPage8Approved[i]
		key := fmt.Sprintf("story.page8.line.%03d", i+1)
		length, parseErr := strconv.Atoi(row[2])
		if row[0] != key || row[1] != strconv.Itoa(i+1) || parseErr != nil || length != int(approved.length) ||
			row[3] != approved.hash || row[4] != "0763:04FF" || row[5] != "0763:026B" ||
			row[6] != "0" || row[7] != "10" || row[8] != strconv.Itoa(17+i) || row[9] != "1" ||
			row[10] != strconv.FormatUint(approved.entryStep, 10) || row[11] != strconv.FormatUint(approved.postCallStep, 10) ||
			row[12] != "confirmed" || row[13] != "READY" || translations[key] == "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 8 頁非 READY identity")
		}
		digest, decodeErr := hex.DecodeString(row[3])
		if decodeErr != nil || len(digest) != 32 {
			return nil, nil, fmt.Errorf("buckrogers: 第 8 頁 hash 無效")
		}
		var hash [32]byte
		copy(hash[:], digest)
		identities[i] = StoryPage8Identity{
			Sequence: uint8(i + 1), OriginalLength: uint8(length), Mode: 1, Repeat: 1,
			Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1,
			EntryStep: approved.entryStep, PostCallStep: approved.postCallStep,
			EventKey: key, OriginalSHA256: hash,
			Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive,
		}
	}
	catalog, err := NewStoryPage8Catalog(identities)
	return catalog, translations, err
}
