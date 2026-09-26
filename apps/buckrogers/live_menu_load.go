package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"

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
	font, err := xlate.LoadFont(fontPath)
	if err != nil {
		return nil, err
	}
	return NewLiveMenuRuntime(catalog, merged, font)
}
