package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// liveMenuSources lists the menu-family catalogs by their formal file names in
// the Buck Rogers text directory, in the receipt runner's merge order.
var liveMenuSources = []struct {
	events, translations, rects string
	load                        func(events, translations []byte) (*MenuCatalog, error)
	characterSheet              bool
}{
	{"menu-events.tsv", "menu.zh-TW.tsv", "menu-text-safe-rects.tsv", LoadMenuCatalog, false},
	{"gender-events.tsv", "gender.zh-TW.tsv", "gender-text-safe-rects.tsv", LoadGenderCatalog, false},
	{"class-events.tsv", "class.zh-TW.tsv", "class-text-safe-rects.tsv", LoadClassCatalog, false},
	{"save-roster-join-runtime-events.tsv", "save-roster-join.zh-TW.tsv", "save-roster-join-text-safe-rects.tsv", LoadRosterCatalog, false},
	{"character-sheet-events.tsv", "character-sheet.zh-TW.tsv", "character-sheet-text-safe-rects.tsv", LoadCharacterSheetCatalog, true},
	{"name-prompt-events.tsv", "name-prompt.zh-TW.tsv", "name-prompt-text-safe-rects.tsv", LoadNamePromptCatalog, false},
	{"career-skill-screen-events.tsv", "career-skill-screen.zh-TW.tsv", "career-skill-screen-text-safe-rects.tsv", LoadCareerSkillCatalog, false},
	{"technical-skill-screen-events.tsv", "technical-skill-screen.zh-TW.tsv", "technical-skill-screen-text-safe-rects.tsv", LoadTechnicalSkillCatalog, false},
}

// LoadLiveMenuRuntime builds the menu family from a Buck Rogers text
// directory and a local 16×16 GOLEMFNT.  Every formal file must be present.
func LoadLiveMenuRuntime(textDir, fontPath string) (*LiveMenuRuntime, error) {
	font, err := xlate.LoadFont(fontPath)
	if err != nil {
		return nil, err
	}
	return loadLiveMenuRuntimeFont(textDir, font)
}

// loadLiveMenuRuntimeFont is LoadLiveMenuRuntime with an already loaded
// font, so the live runtime can share one font pointer, and with it one
// half-font derivation, across every family (spec 039 §3.3).
func loadLiveMenuRuntimeFont(textDir string, font *xlate.Font) (*LiveMenuRuntime, error) {
	read := func(name string) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(textDir, name))
		if err != nil {
			return nil, fmt.Errorf("buckrogers: 讀取 %s：%w", name, err)
		}
		return b, nil
	}
	var catalogs []*MenuCatalog
	var rects []*MenuOverlayRects
	for _, src := range liveMenuSources {
		events, err := read(src.events)
		if err != nil {
			return nil, err
		}
		translations, err := read(src.translations)
		if err != nil {
			return nil, err
		}
		c, err := src.load(events, translations)
		if err != nil {
			return nil, err
		}
		catalogs = append(catalogs, c)
		rectData, err := read(src.rects)
		if err != nil {
			return nil, err
		}
		var rc *MenuOverlayRects
		if src.characterSheet {
			rc, err = LoadCharacterSheetOverlayRects(src.rects, rectData)
		} else {
			rc, err = LoadMenuOverlayRects(src.rects, rectData)
		}
		if err != nil {
			return nil, err
		}
		rects = append(rects, rc)
	}
	catalog, err := MergeMenuCatalogs(catalogs...)
	if err != nil {
		return nil, err
	}
	merged, err := MergeMenuOverlayRects(rects...)
	if err != nil {
		return nil, err
	}
	r, err := NewLiveMenuRuntime(catalog, merged, font)
	if err != nil {
		return nil, err
	}
	// Spec 039 §3.4 欄名列: the white list is optional (absent: no header
	// is anchored); a present list must pass the load check.
	data, err := os.ReadFile(filepath.Join(textDir, HeaderColumnsFile))
	if os.IsNotExist(err) {
		return r, nil
	} else if err != nil {
		return nil, fmt.Errorf("buckrogers: 讀取 %s：%w", HeaderColumnsFile, err)
	}
	h, err := LoadHeaderColumns(HeaderColumnsFile, data)
	if err != nil {
		return nil, err
	}
	if err := r.SetHeaderColumns(h); err != nil {
		return nil, err
	}
	return r, nil
}

// menuFamilyStem is the language-file family name of a menu source.
func menuFamilyStem(translations string) string {
	return strings.TrimSuffix(translations, "."+LangZhTW+".tsv")
}

// liveMenuShared is the language-independent part of the menu family
// (spec 040 §3.1): identities (validated against the zh-TW reference),
// safe rectangles and the header white list.
type liveMenuShared struct {
	full     *MenuCatalog   // merged zh-TW catalog (reference, exact)
	families []*MenuCatalog // per source, same order as liveMenuSources
	rects    *MenuOverlayRects
	headers  *HeaderColumns
}

// loadLiveMenuShared loads the menu events with the zh-TW reference texts
// (exact key match, spec 040 §3.1), the safe rectangles and header list.
func loadLiveMenuShared(textDir string) (*liveMenuShared, error) {
	read := func(name string) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(textDir, name))
		if err != nil {
			return nil, fmt.Errorf("buckrogers: 讀取 %s：%w", name, err)
		}
		return b, nil
	}
	out := &liveMenuShared{}
	var rects []*MenuOverlayRects
	for _, src := range liveMenuSources {
		events, err := read(src.events)
		if err != nil {
			return nil, err
		}
		translations, err := read(src.translations)
		if err != nil {
			return nil, err
		}
		c, err := src.load(events, translations)
		if err != nil {
			return nil, err
		}
		out.families = append(out.families, c)
		rectData, err := read(src.rects)
		if err != nil {
			return nil, err
		}
		var rc *MenuOverlayRects
		if src.characterSheet {
			rc, err = LoadCharacterSheetOverlayRects(src.rects, rectData)
		} else {
			rc, err = LoadMenuOverlayRects(src.rects, rectData)
		}
		if err != nil {
			return nil, err
		}
		rects = append(rects, rc)
	}
	var err error
	if out.full, err = MergeMenuCatalogs(out.families...); err != nil {
		return nil, err
	}
	if out.rects, err = MergeMenuOverlayRects(rects...); err != nil {
		return nil, err
	}
	if err := ValidateMenuOverlayCoverage(out.full, out.rects); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(textDir, HeaderColumnsFile))
	if os.IsNotExist(err) {
		return out, nil
	} else if err != nil {
		return nil, fmt.Errorf("buckrogers: 讀取 %s：%w", HeaderColumnsFile, err)
	}
	h, err := LoadHeaderColumns(HeaderColumnsFile, data)
	if err != nil {
		return nil, err
	}
	if err := h.ValidateMenu(out.full); err != nil {
		return nil, err
	}
	out.headers = h
	return out, nil
}

// laneTexts returns one language's menu texts by text key.  zh-TW is the
// reference catalog itself; another language reads <family>.<lang>.tsv
// from dir, whose keys must be the family's text keys (rows may be missing).
func (s *liveMenuShared) laneTexts(dir, lang string) (map[string]string, error) {
	if lang == LangZhTW {
		return s.full.texts(), nil
	}
	out := map[string]string{}
	for i, src := range liveMenuSources {
		stem := menuFamilyStem(src.translations)
		data, err := readLangFile(dir, stem, lang)
		if err != nil {
			return nil, err
		}
		texts, err := langTexts(LangFile(stem, lang), data, s.families[i].textKeys())
		if err != nil {
			return nil, err
		}
		for k, v := range texts {
			if prior, ok := out[k]; ok && prior != v {
				return nil, fmt.Errorf("buckrogers: %s: 文字鍵 %q 與其他選單家族譯文不同", LangFile(stem, lang), k)
			}
			out[k] = v
		}
	}
	return out, nil
}

// validateMenuLane prebuilds every translated menu row of one language at
// one scale (spec 040 §3.3): capacity, glyphs, ink and header anchoring.
func validateMenuLane(s *liveMenuShared, texts map[string]string, headers *HeaderColumns, font *xlate.Font, scale int) error {
	keys := make([]menuIdentity, 0, len(s.full.byIdentity))
	for id := range s.full.byIdentity {
		keys = append(keys, id)
	}
	sort.Slice(keys, func(i, j int) bool {
		return s.full.byIdentity[keys[i]].eventKey+fmt.Sprint(keys[i]) < s.full.byIdentity[keys[j]].eventKey+fmt.Sprint(keys[j])
	})
	for _, id := range keys {
		e := s.full.byIdentity[id]
		t, ok := texts[e.textKey]
		if !ok || t == "" {
			continue
		}
		r, ok := s.rects.byEvent[e.eventKey]
		if !ok {
			return fmt.Errorf("buckrogers: %s 沒有安全矩形", e.eventKey)
		}
		entry := MenuOverlayEntry{EventKey: e.eventKey, TextKey: e.textKey, Translation: t,
			Background: id.background, Foreground: id.foreground,
			X: r.x, Y: r.y, Width: r.width, Height: r.height, DrawX: r.drawX, DrawY: r.drawY,
			Capacity: r.capacity, LineCount: r.lines, Overflow: r.overflow, Columns: headers.MenuColumns(e.eventKey)}
		if _, err := BuildMenuOverlay([]MenuOverlayEntry{entry}, font, [256][3]uint8{}, scale); err != nil {
			return fmt.Errorf("buckrogers: 選單 %s（%d×）：%w", e.eventKey, scale, err)
		}
	}
	return nil
}
