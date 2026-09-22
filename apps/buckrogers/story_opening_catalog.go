package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
)

var storyOpeningEventHeader = []string{"event_key", "sequence", "original_length", "original_sha256", "caller", "glyph_guard", "background", "foreground", "row", "column", "entry_step", "post_call_step", "evidence_level", "catalog_status"}
var storyOpeningTextHeader = []string{"key", "translation", "source"}

// LoadStoryOpeningCatalog accepts exactly the READY five-line first-screen pair.
func LoadStoryOpeningCatalog(eventsName string, eventsData []byte, textName string, textData []byte) (*StoryOpeningCatalog, map[string]string, error) {
	events, err := readTSV(eventsName, eventsData, storyOpeningEventHeader)
	if err != nil {
		return nil, nil, err
	}
	texts, err := readTSV(textName, textData, storyOpeningTextHeader)
	if err != nil {
		return nil, nil, err
	}
	if len(events) != 5 || len(texts) != 5 {
		return nil, nil, fmt.Errorf("buckrogers: 首屏 catalog 必須各有五行")
	}
	translations := map[string]string{}
	for _, row := range texts {
		if row[0] == "" || row[1] == "" || translations[row[0]] != "" {
			return nil, nil, fmt.Errorf("buckrogers: 首屏譯文 key 無效")
		}
		translations[row[0]] = row[1]
	}
	entries := make([]StoryOpeningIdentity, 5)
	for i, row := range events {
		if row[12] != "confirmed" || row[13] != "READY" || translations[row[0]] == "" {
			return nil, nil, fmt.Errorf("buckrogers: 首屏非 READY identity")
		}
		length, e := strconv.Atoi(row[2])
		if e != nil || length < 1 || length > 255 {
			return nil, nil, fmt.Errorf("buckrogers: 首屏長度無效")
		}
		b, e := hex.DecodeString(row[3])
		if e != nil || len(b) != 32 {
			return nil, nil, fmt.Errorf("buckrogers: 首屏 hash 無效")
		}
		var hash [32]byte
		copy(hash[:], b)
		parse := func(s string) (uint8, error) {
			n, e := strconv.Atoi(s)
			return uint8(n), func() error {
				if e != nil || n < 0 || n > 255 {
					return fmt.Errorf("byte")
				}
				return nil
			}()
		}
		bg, e := parse(row[6])
		if e != nil {
			return nil, nil, e
		}
		fg, e := parse(row[7])
		if e != nil {
			return nil, nil, e
		}
		r, e := parse(row[8])
		if e != nil {
			return nil, nil, e
		}
		c, e := parse(row[9])
		if e != nil {
			return nil, nil, e
		}
		entries[i] = StoryOpeningIdentity{Sequence: uint8(i + 1), EventKey: row[0], OriginalLength: uint8(length), OriginalSHA256: hash, Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive, Mode: 1, Repeat: 1, Background: bg, Foreground: fg, Row: r, Column: c}
	}
	catalog, err := NewStoryOpeningCatalog(entries)
	if err != nil {
		return nil, nil, err
	}
	_ = sha256.Size
	return catalog, translations, nil
}
