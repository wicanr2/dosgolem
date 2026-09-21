package buckrogers

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

type menuIdentity struct {
	length     uint8
	hash       [32]byte
	caller     Address
	background uint8
	foreground uint8
	row        uint8
	column     uint8
}

// MenuCatalog is an immutable exact-match resolver for completed menu text
// events. It implements docs/spec/009-buck-rogers-menu-display-request.md.
type MenuCatalog struct {
	byIdentity map[menuIdentity]catalogEntry
}

var menuEventHeader = []string{
	"event_key", "sequence", "text_key", "original_length", "original_sha256",
	"caller", "background", "foreground", "row", "column",
}

// LoadMenuCatalog validates both formal TSV inputs before exposing a resolver.
func LoadMenuCatalog(events, translations []byte) (*MenuCatalog, error) {
	return loadExactCatalog("menu-events.tsv", "menu.zh-TW.tsv", events, translations)
}

// LoadGenderCatalog validates the formal gender selection TSV inputs while
// reusing the exact same identity resolver as menu events.
func LoadGenderCatalog(events, translations []byte) (*MenuCatalog, error) {
	return loadExactCatalog("gender-events.tsv", "gender.zh-TW.tsv", events, translations)
}

// LoadClassCatalog validates the formal class selection TSV inputs while
// reusing the exact same identity resolver as menu and gender events.
func LoadClassCatalog(events, translations []byte) (*MenuCatalog, error) {
	return loadExactCatalog("class-events.tsv", "class.zh-TW.tsv", events, translations)
}

// LoadRosterCatalog validates the saved-character roster TSV inputs while
// reusing the exact same identity resolver as the other interface catalogs.
func LoadRosterCatalog(events, translations []byte) (*MenuCatalog, error) {
	return loadExactCatalog("save-roster-join-runtime-events.tsv", "save-roster-join.zh-TW.tsv", events, translations)
}

// LoadCharacterSheetCatalog validates the static character-sheet TSV inputs.
// Dynamic identity, ability, skill and RNG values do not belong in this catalog.
func LoadCharacterSheetCatalog(events, translations []byte) (*MenuCatalog, error) {
	return loadExactCatalog("character-sheet-events.tsv", "character-sheet.zh-TW.tsv", events, translations)
}

// LoadNamePromptCatalog validates the static character-name prompt catalog.
// Player-entered name bytes deliberately remain outside this resolver.
func LoadNamePromptCatalog(events, translations []byte) (*MenuCatalog, error) {
	return loadExactCatalog("name-prompt-events.tsv", "name-prompt.zh-TW.tsv", events, translations)
}

// LoadCareerSkillCatalog validates the static career-skill allocation screen
// catalog. Dynamic points, bonuses and totals remain exact misses.
func LoadCareerSkillCatalog(events, translations []byte) (*MenuCatalog, error) {
	return loadExactCatalog("career-skill-screen-events.tsv", "career-skill-screen.zh-TW.tsv", events, translations)
}

// LoadTechnicalSkillCatalog validates the static technical-skill allocation
// screen catalog. Dynamic points, bonuses and totals remain exact misses.
func LoadTechnicalSkillCatalog(events, translations []byte) (*MenuCatalog, error) {
	return loadExactCatalog("technical-skill-screen-events.tsv", "technical-skill-screen.zh-TW.tsv", events, translations)
}

func loadExactCatalog(eventName, translationName string, events, translations []byte) (*MenuCatalog, error) {
	eventRows, err := readTSV(eventName, events, menuEventHeader)
	if err != nil {
		return nil, err
	}
	textRows, err := readTSV(translationName, translations, textHeader)
	if err != nil {
		return nil, err
	}

	texts := make(map[string]string, len(textRows))
	for _, row := range textRows {
		if _, exists := texts[row[0]]; exists {
			return nil, fmt.Errorf("%s: 重複文字鍵 %q", translationName, row[0])
		}
		texts[row[0]] = row[1]
	}

	byIdentity := make(map[menuIdentity]catalogEntry, len(eventRows))
	eventKeys := make(map[string]bool, len(eventRows))
	usedTextKeys := make(map[string]bool, len(eventRows))
	for i, row := range eventRows {
		sequence, err := positiveDecimal(row[1])
		if err != nil || sequence != i+1 {
			return nil, fmt.Errorf("%s: sequence 必須從 1 連續，取得 %q", eventName, row[1])
		}
		length, err := menuByte(row[3])
		if err != nil {
			return nil, fmt.Errorf("%s: 無效 original_length %q", eventName, row[3])
		}
		hash, err := menuHash(row[4])
		if err != nil {
			return nil, fmt.Errorf("%s: 無效 original_sha256 %q", eventName, row[4])
		}
		caller, err := menuAddress(row[5])
		if err != nil {
			return nil, fmt.Errorf("%s: 無效 caller %q", eventName, row[5])
		}
		values := [4]uint8{}
		for j := range values {
			values[j], err = menuByte(row[6+j])
			if err != nil {
				return nil, fmt.Errorf("%s: 無效畫面欄位 %q", eventName, row[6+j])
			}
		}
		id := menuIdentity{length, hash, caller, values[0], values[1], values[2], values[3]}
		if _, exists := byIdentity[id]; exists {
			return nil, fmt.Errorf("%s: 重複事件 identity", eventName)
		}
		if eventKeys[row[0]] {
			return nil, fmt.Errorf("%s: 重複事件鍵 %q", eventName, row[0])
		}
		translation, exists := texts[row[2]]
		if !exists {
			return nil, fmt.Errorf("%s: 文字鍵不在 catalog %q", eventName, row[2])
		}
		eventKeys[row[0]], usedTextKeys[row[2]] = true, true
		byIdentity[id] = catalogEntry{eventKey: row[0], textKey: row[2], translation: translation}
	}
	for key := range texts {
		if !usedTextKeys[key] {
			return nil, fmt.Errorf("%s: 孤兒文字鍵 %q", translationName, key)
		}
	}
	return &MenuCatalog{byIdentity: byIdentity}, nil
}

// MergeMenuCatalogs combines independently validated exact catalogs. Identity
// collisions fail closed even when both entries would produce the same text.
func MergeMenuCatalogs(catalogs ...*MenuCatalog) (*MenuCatalog, error) {
	total := 0
	for _, catalog := range catalogs {
		if catalog != nil {
			total += len(catalog.byIdentity)
		}
	}
	if total == 0 {
		return nil, nil
	}
	merged := &MenuCatalog{byIdentity: make(map[menuIdentity]catalogEntry, total)}
	for _, catalog := range catalogs {
		if catalog == nil {
			continue
		}
		for id, entry := range catalog.byIdentity {
			if _, exists := merged.byIdentity[id]; exists {
				return nil, fmt.Errorf("catalog 合併：重複事件 identity")
			}
			merged.byIdentity[id] = entry
		}
	}
	return merged, nil
}

// Resolve returns output-side data only for a completed, exact event identity.
func (c *MenuCatalog) Resolve(event TextEvent) (DisplayRequest, bool) {
	if c == nil || event.PostCallStep <= event.EntryStep {
		return DisplayRequest{}, false
	}
	id := menuIdentity{event.OriginalLength, event.OriginalSHA256, event.Caller,
		event.Background, event.Foreground, event.Row, event.Column}
	entry, ok := c.byIdentity[id]
	if !ok {
		return DisplayRequest{}, false
	}
	return DisplayRequest{EventKey: entry.eventKey, TextKey: entry.textKey, Translation: entry.translation}, true
}

func menuByte(s string) (uint8, error) {
	n, err := nonnegativeDecimal(s)
	if err != nil || n > 255 {
		return 0, fmt.Errorf("不是 0–255")
	}
	return uint8(n), nil
}

func menuHash(s string) ([32]byte, error) {
	var out [32]byte
	if len(s) != 64 {
		return out, fmt.Errorf("長度不是 64")
	}
	for i := range s {
		if !((s[i] >= '0' && s[i] <= '9') || (s[i] >= 'a' && s[i] <= 'f')) {
			return out, fmt.Errorf("不是小寫十六進位")
		}
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return out, err
	}
	copy(out[:], b)
	return out, nil
}

func menuAddress(s string) (Address, error) {
	if len(s) != 9 || s[4] != ':' {
		return Address{}, fmt.Errorf("格式不符")
	}
	for i := range s {
		if i == 4 {
			continue
		}
		if !((s[i] >= '0' && s[i] <= '9') || (s[i] >= 'A' && s[i] <= 'F')) {
			return Address{}, fmt.Errorf("不是大寫十六進位")
		}
	}
	segment, err := strconv.ParseUint(s[:4], 16, 16)
	if err != nil {
		return Address{}, err
	}
	offset, err := strconv.ParseUint(s[5:], 16, 16)
	if err != nil {
		return Address{}, err
	}
	return Address{uint16(segment), uint16(offset)}, nil
}
