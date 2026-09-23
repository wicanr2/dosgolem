package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const storyPage9ApprovedHash = "39a751ca9f384a77491b1e4399c0a72afb8b1ef146a77db2623394590ff1ca78"
const storyPage9TranslationTSVHash = "baf9ba8b56757261ba1e022e69c2df34c971c5679b593c83a694c0b4b809cb2e"

func LoadStoryPage9Catalog(eventName string, eventData []byte, translationName string, translationData []byte) (*StoryPage9Catalog, map[string]string, error) {
	if fmt.Sprintf("%x", sha256.Sum256(translationData)) != storyPage9TranslationTSVHash {
		return nil, nil, fmt.Errorf("buckrogers: 第 9 頁譯文版本未驗證")
	}
	events, err := readTSV(eventName, eventData, storyPage8EventHeader)
	if err != nil {
		return nil, nil, err
	}
	translations, err := readTSV(translationName, translationData, storyOpeningTextHeader)
	if err != nil {
		return nil, nil, err
	}
	if len(events) != 1 || len(translations) != 1 {
		return nil, nil, fmt.Errorf("buckrogers: 第 9 頁 catalog 必須各一筆")
	}
	e, t := events[0], translations[0]
	if e[0] != "story.page9.line.001" || e[1] != "1" || e[2] != "20" || e[3] != storyPage9ApprovedHash ||
		e[4] != "0763:04FF" || e[5] != "0763:026B" || e[6] != "0" || e[7] != "10" ||
		e[8] != "17" || e[9] != "1" || e[10] != "351155910" || e[11] != "351988536" ||
		e[12] != "confirmed" || e[13] != "READY" || t[0] != e[0] || t[1] == "" || t[2] != "runtime-editorial" {
		return nil, nil, fmt.Errorf("buckrogers: 第 9 頁非 READY identity／譯文")
	}
	decoded, err := hex.DecodeString(e[3])
	if err != nil || len(decoded) != 32 {
		return nil, nil, fmt.Errorf("buckrogers: 第 9 頁 hash 無效")
	}
	var digest [32]byte
	copy(digest[:], decoded)
	identity := StoryPage9Identity{EventKey: e[0], OriginalLength: 20, OriginalSHA256: digest,
		Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive,
		Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: 17, Column: 1,
		EntryStep: 351155910, PostCallStep: 351988536}
	catalog, err := NewStoryPage9Catalog(identity)
	if err != nil {
		return nil, nil, err
	}
	return catalog, map[string]string{e[0]: t[1]}, nil
}
