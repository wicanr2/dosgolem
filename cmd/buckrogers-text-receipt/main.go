// Command buckrogers-text-receipt replays a diagnostic state and emits only
// content-free metadata for completed 0763:0424 calls.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
	"github.com/wicanr2/dosgolem/xlate"
)

type eventJSON struct {
	EntryStep      uint64             `json:"entry_step"`
	PostCallStep   uint64             `json:"post_call_step"`
	Caller         buckrogers.Address `json:"caller"`
	OriginalLength uint8              `json:"original_length"`
	OriginalSHA256 string             `json:"original_sha256"`
	Background     uint8              `json:"background"`
	Foreground     uint8              `json:"foreground"`
	Row            uint8              `json:"row"`
	Column         uint8              `json:"column"`
}

type requestJSON struct {
	EventKey         string `json:"event_key"`
	TextKey          string `json:"text_key"`
	TranslationRunes int    `json:"translation_runes"`
}

type manualPresentationJSON struct {
	Step       uint64 `json:"step"`
	Kind       string `json:"kind"`
	Generation uint64 `json:"generation"`
	EventKey   string `json:"event_key,omitempty"`
	TextKey    string `json:"text_key,omitempty"`
	Runes      int    `json:"translation_runes,omitempty"`
}

type manualStyleJSON struct {
	Background uint8 `json:"background"`
	Foreground uint8 `json:"foreground"`
	Row        uint8 `json:"row"`
	Column     uint8 `json:"column"`
}

type manualOverlayJSON struct {
	Scale                 int           `json:"scale"`
	Actions               []requestJSON `json:"actions"`
	ActiveKeys            []string      `json:"active_keys"`
	MissingGlyphs         []string      `json:"missing_glyphs"`
	Drew                  bool          `json:"drew"`
	BaselineRGBA256       string        `json:"baseline_rgba_sha256"`
	OverlayRGBA256        string        `json:"overlay_rgba_sha256"`
	DiffOutsideClearRect  int           `json:"diff_outside_clear_rect"`
	DiffInsideClearRect   int           `json:"diff_inside_clear_rect"`
	AddedNonBaselinePixel int           `json:"added_nonbaseline_pixels"`
	StyleObserved         bool          `json:"style_observed"`
	FrameCallbacks        uint64        `json:"frame_callbacks"`
}

type actionBarOverlayJSON struct {
	Scale                 int                  `json:"scale"`
	Actions               []requestJSON        `json:"actions"`
	Styles                []actionBarStyleJSON `json:"styles"`
	ActiveKeys            []string             `json:"active_keys"`
	MissingGlyphs         []string             `json:"missing_glyphs"`
	Drew                  bool                 `json:"drew"`
	BaselineRGBA256       string               `json:"baseline_rgba_sha256"`
	OverlayRGBA256        string               `json:"overlay_rgba_sha256"`
	DiffOutsideSafeRects  int                  `json:"diff_outside_safe_rects"`
	DiffInsideSafeRects   int                  `json:"diff_inside_safe_rects"`
	AddedNonBaselinePixel int                  `json:"added_nonbaseline_pixels"`
}

// actionBarStyleJSON is content-safe receipt evidence for the READY visual
// contract: it records palette indices, never original glyph bytes.
type actionBarStyleJSON struct {
	EventKey        string `json:"event_key"`
	Background      uint8  `json:"background"`
	RuneForegrounds []int  `json:"rune_foregrounds"`
}

type actionBarEventJSON struct {
	EntryStep      uint64 `json:"entry_step"`
	PostCallStep   uint64 `json:"post_call_step"`
	Screen         string `json:"screen"`
	EventKey       string `json:"event_key"`
	Variant        string `json:"variant"`
	OriginalLength uint8  `json:"original_length"`
	OriginalSHA256 string `json:"original_sha256"`
	Row            uint8  `json:"row"`
	Column         uint8  `json:"column"`
	X0             uint8  `json:"x0"`
	Y0             uint8  `json:"y0"`
	X1             uint8  `json:"x1"`
	Y1             uint8  `json:"y1"`
}

type scheduledBIOSKey struct {
	Step  uint64
	Scan  uint8
	ASCII uint8
}

type scheduledBIOSKeys []scheduledBIOSKey

func (s *scheduledBIOSKeys) String() string { return "" }
func (s *scheduledBIOSKeys) Set(value string) error {
	key, err := parseScheduledBIOSKey(value)
	if err != nil {
		return err
	}
	*s = append(*s, key)
	return nil
}

type keyJSON struct {
	QueuedAt uint64 `json:"queued_at"`
	Scan     uint8  `json:"scan"`
	ASCII    uint8  `json:"ascii"`
}

type writeJSON struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

type fileOpJSON struct {
	Step   uint64 `json:"step"`
	Op     string `json:"op"`
	Fn     uint8  `json:"fn"`
	Handle uint16 `json:"handle"`
	Name   string `json:"name"`
	Arg    int64  `json:"arg"`
	Pos    int64  `json:"pos"`
	Len    int    `json:"len"`
	Whence uint8  `json:"whence"`
	Failed bool   `json:"failed"`
}

// clearJSON is content-free evidence for one original text-cell clear.  It is
// emitted only when the diagnostic --clear-trace switch is requested.
type clearJSON struct {
	Step   uint64 `json:"step"`
	Bottom uint8  `json:"bottom"`
	Right  uint8  `json:"right"`
	Top    uint8  `json:"top"`
	Left   uint8  `json:"left"`
}

// glyphJSON records the ABI-visible metadata of a completed 0763:026B glyph
// call. It intentionally never stores the original glyph byte stream.
type glyphJSON struct {
	EntryStep    uint64             `json:"entry_step"`
	PostCallStep uint64             `json:"post_call_step"`
	Caller       buckrogers.Address `json:"caller"`
	Mode         uint8              `json:"mode"`
	Repeat       uint8              `json:"repeat"`
	Background   uint8              `json:"background"`
	Foreground   uint8              `json:"foreground"`
	Row          uint8              `json:"row"`
	Column       uint8              `json:"column"`
}

type glyphFrame struct {
	event  glyphJSON
	ss, sp uint16
	glyph  uint8
}

// glyphRunJSON is a content-safe contiguous glyph run. Its hash is computed
// in-process from original bytes and the bytes are then discarded.
type glyphRunJSON struct {
	EntryStep      uint64             `json:"entry_step"`
	PostCallStep   uint64             `json:"post_call_step"`
	Caller         buckrogers.Address `json:"caller"`
	OriginalLength uint8              `json:"original_length"`
	OriginalSHA256 string             `json:"original_sha256"`
	Mode           uint8              `json:"mode"`
	Repeat         uint8              `json:"repeat"`
	Background     uint8              `json:"background"`
	Foreground     uint8              `json:"foreground"`
	Row            uint8              `json:"row"`
	Column         uint8              `json:"column"`
}

type glyphRunFrame struct {
	event glyphJSON
	bytes []byte
}

// pixelWriteJSON is a content-safe, first-change receipt for a caller-selected
// indexed framebuffer rectangle. It records no original pixels.
type pixelWriteJSON struct {
	Step         uint64             `json:"step"`
	Caller       buckrogers.Address `json:"caller"`
	VideoSegment uint16             `json:"video_segment"`
	VideoOffset  uint16             `json:"video_offset"`
	ByteCount    uint16             `json:"byte_count"`
	X0           uint16             `json:"x0"`
	Y0           uint16             `json:"y0"`
	X1           uint16             `json:"x1"`
	Y1           uint16             `json:"y1"`
}

func main() {
	statePath := flag.String("state", "", "既有 probe state")
	until := flag.Uint64("until", 0, "絕對指令步數上限")
	want := flag.Int("want", 0, "預期完成事件數；0 表示不檢查")
	wantRequests := flag.Int("want-requests", 0, "預期顯示請求數；0 表示不檢查")
	wantActionBarEvents := flag.Int("want-action-bar-events", 0, "預期底部操作列事件數；0 表示不檢查")
	wantActionBarRequests := flag.Int("want-action-bar-requests", 0, "預期底部操作列顯示請求數；0 表示不檢查")
	enterAt := flag.Uint64("bios-enter-at", 0, "在此絕對步數排入一個 BIOS Enter；0 表示不送")
	menuEvents := flag.String("menu-events", "", "正式 menu-events.tsv")
	menuTranslations := flag.String("menu-translations", "", "正式 menu.zh-TW.tsv")
	genderEvents := flag.String("gender-events", "", "正式 gender-events.tsv")
	genderTranslations := flag.String("gender-translations", "", "正式 gender.zh-TW.tsv")
	classEvents := flag.String("class-events", "", "正式 class-events.tsv")
	classTranslations := flag.String("class-translations", "", "正式 class.zh-TW.tsv")
	rosterEvents := flag.String("roster-events", "", "正式 save-roster-join-runtime-events.tsv")
	rosterTranslations := flag.String("roster-translations", "", "正式 save-roster-join.zh-TW.tsv")
	characterSheetEvents := flag.String("character-sheet-events", "", "正式 character-sheet-events.tsv")
	characterSheetTranslations := flag.String("character-sheet-translations", "", "正式 character-sheet.zh-TW.tsv")
	namePromptEvents := flag.String("name-prompt-events", "", "正式 name-prompt-events.tsv")
	namePromptTranslations := flag.String("name-prompt-translations", "", "正式 name-prompt.zh-TW.tsv")
	careerSkillEvents := flag.String("career-skill-events", "", "正式 career-skill-screen-events.tsv")
	careerSkillTranslations := flag.String("career-skill-translations", "", "正式 career-skill-screen.zh-TW.tsv")
	technicalSkillEvents := flag.String("technical-skill-events", "", "正式 technical-skill-screen-events.tsv")
	technicalSkillTranslations := flag.String("technical-skill-translations", "", "正式 technical-skill-screen.zh-TW.tsv")
	actionBarEvents := flag.String("skill-action-bar-events", "", "正式 skill-action-bar-events.tsv")
	actionBarTranslations := flag.String("skill-action-bar-translations", "", "正式 skill-action-bar.zh-TW.tsv")
	actionBarRects := flag.String("skill-action-bar-rects", "", "正式 skill-action-bar-text-safe-rects.tsv")
	actionBarOverlayFont := flag.String("skill-action-bar-overlay-font", "", "本機 16x16 操作列 GOLEMFNT")
	actionBarOverlayScale := flag.Int("skill-action-bar-overlay-scale", 0, "明示操作列覆繪倍率 2 或 3")
	actionBarOverlayOut := flag.String("skill-action-bar-overlay-rgba-out", "", "輸出操作列覆繪後 RGBA framebuffer")
	actionBarBaselineOut := flag.String("skill-action-bar-baseline-rgba-out", "", "輸出同 frame／palette、未覆繪的操作列 RGBA baseline")
	actionBarPNGOut := flag.String("skill-action-bar-overlay-png-out", "", "輸出操作列覆繪 PNG")
	actionBarBaselinePNGOut := flag.String("skill-action-bar-baseline-png-out", "", "輸出未覆繪操作列 PNG")
	careerSkillRects := flag.String("career-skill-rects", "", "正式 career-skill-screen-text-safe-rects.tsv")
	technicalSkillRects := flag.String("technical-skill-rects", "", "正式 technical-skill-screen-text-safe-rects.tsv")
	namePromptRects := flag.String("name-prompt-rects", "", "正式 name-prompt-text-safe-rects.tsv")
	characterSheetRects := flag.String("character-sheet-rects", "", "正式 character-sheet-text-safe-rects.tsv")
	menuRects := flag.String("menu-rects", "", "正式 menu-text-safe-rects.tsv")
	genderRects := flag.String("gender-rects", "", "正式 gender-text-safe-rects.tsv")
	classRects := flag.String("class-rects", "", "正式 class-text-safe-rects.tsv")
	rosterRects := flag.String("roster-rects", "", "正式 save-roster-join-text-safe-rects.tsv")
	overlayFont := flag.String("overlay-font", "", "16x16 GOLEMFNT")
	overlayScale := flag.Int("overlay-scale", 0, "明示覆繪倍率 2 或 3")
	overlayOut := flag.String("overlay-rgba-out", "", "輸出倍率後 RGBA framebuffer")
	baselineOut := flag.String("baseline-rgba-out", "", "輸出同 frame／palette、未覆繪的倍率後 RGBA baseline")
	manualEvents := flag.String("manual-events", "", "正式 manual-events.tsv")
	manualOrdinals := flag.String("manual-ordinals", "", "正式 manual-ordinals.tsv")
	manualTranslations := flag.String("manual-translations", "", "正式 manual.zh-TW.tsv")
	manualLayout := flag.String("manual-layout", "", "正式 manual-overlay-layout.tsv")
	manualFont := flag.String("manual-overlay-font", "", "本機 16x16 手冊 GOLEMFNT")
	manualScale := flag.Int("manual-overlay-scale", 0, "明示手冊覆繪倍率 2 或 3")
	manualOut := flag.String("manual-overlay-rgba-out", "", "輸出手冊覆繪後 RGBA framebuffer")
	manualBaselineOut := flag.String("manual-baseline-rgba-out", "", "輸出同 frame／palette、未覆繪的手冊 RGBA baseline")
	manualPNGOut := flag.String("manual-overlay-png-out", "", "輸出手冊覆繪 PNG")
	manualBaselinePNGOut := flag.String("manual-baseline-png-out", "", "輸出未覆繪手冊 PNG")
	screenOut := flag.String("screen-out", "", "成功後寫出終態 320×200 indexed framebuffer")
	receiptOut := flag.String("receipt-out", "", "成功後另寫出與 stdout 相同的 JSON 收據")
	stateOut := flag.String("state-out", "", "成功後保存終態 savestate（只供本機研究）")
	scratch := flag.String("scratch", "", "可選、已存在的 DOS 可寫暫存目錄")
	fileOps := flag.Bool("file-ops", false, "在收據加入 content-safe 檔案操作 metadata")
	unimplemented := flag.Bool("unimplemented", false, "在收據加入未實作 DOS／BIOS 服務統計")
	clearTrace := flag.Bool("clear-trace", false, "在收據加入 026F:029C 的 content-safe 清除矩形")
	glyphTrace := flag.Bool("glyph-trace", false, "在收據加入 0763:026B 的 content-safe glyph 呼叫")
	glyphTraceFrom := flag.Uint64("glyph-trace-from", 0, "glyph 追蹤的最早絕對步數；0 表示不過濾")
	storyPixelTrace := flag.Bool("story-pixel-trace", false, "記錄 story rows 17..21 的第一筆 indexed framebuffer 改寫")
	var genericKeys scheduledBIOSKeys
	flag.Var(&genericKeys, "bios-key-at", "可重複 STEP:SCAN_HEX:ASCII_HEX BIOS 鍵排程")
	flag.Parse()
	if *statePath == "" || *until == 0 {
		fail(fmt.Errorf("state 與 until 為必填"))
	}
	if err := validateCatalogFlags(*menuEvents, *menuTranslations, *genderEvents, *genderTranslations,
		*classEvents, *classTranslations, *rosterEvents, *rosterTranslations,
		*characterSheetEvents, *characterSheetTranslations); err != nil {
		fail(err)
	}
	if err := validateNamePromptCatalogFlags(*namePromptEvents, *namePromptTranslations); err != nil {
		fail(err)
	}
	if err := validateCareerSkillCatalogFlags(*careerSkillEvents, *careerSkillTranslations); err != nil {
		fail(err)
	}
	if err := validateTechnicalSkillCatalogFlags(*technicalSkillEvents, *technicalSkillTranslations, *careerSkillEvents); err != nil {
		fail(err)
	}
	if err := validateActionBarFlags(*actionBarEvents, *actionBarTranslations, *careerSkillEvents, *technicalSkillEvents); err != nil {
		fail(err)
	}
	if err := validateActionBarOverlayFlags(*actionBarEvents, *actionBarTranslations, *actionBarRects,
		*actionBarOverlayFont, *actionBarOverlayOut, *actionBarBaselineOut, *actionBarPNGOut, *actionBarBaselinePNGOut, *actionBarOverlayScale); err != nil {
		fail(err)
	}
	if err := validateOverlayFlags(*menuEvents, *menuRects, *genderEvents, *genderRects,
		*classEvents, *classRects, *rosterEvents, *rosterRects, *characterSheetEvents, *characterSheetRects,
		*namePromptEvents, *namePromptRects,
		*careerSkillEvents, *careerSkillRects,
		*technicalSkillEvents, *technicalSkillRects,
		*overlayFont, *overlayOut, *overlayScale); err != nil {
		fail(err)
	}
	if *baselineOut != "" && *overlayOut == "" {
		fail(fmt.Errorf("baseline-rgba-out 只能與 overlay-rgba-out 同時提供"))
	}
	if err := validateManualOverlayFlags(*manualEvents, *manualOrdinals, *manualTranslations, *manualLayout,
		*manualFont, *manualOut, *manualBaselineOut, *manualPNGOut, *manualBaselinePNGOut, *manualScale); err != nil {
		fail(err)
	}
	keys, err := mergeBIOSKeySchedule(*enterAt, genericKeys, *until)
	if err != nil {
		fail(err)
	}
	var catalogs []*buckrogers.MenuCatalog
	if *menuEvents != "" {
		eventsData, err := os.ReadFile(*menuEvents)
		if err != nil {
			fail(err)
		}
		translationsData, err := os.ReadFile(*menuTranslations)
		if err != nil {
			fail(err)
		}
		catalog, err := buckrogers.LoadMenuCatalog(eventsData, translationsData)
		if err != nil {
			fail(err)
		}
		catalogs = append(catalogs, catalog)
	}
	if *genderEvents != "" {
		eventsData, err := os.ReadFile(*genderEvents)
		if err != nil {
			fail(err)
		}
		translationsData, err := os.ReadFile(*genderTranslations)
		if err != nil {
			fail(err)
		}
		catalog, err := buckrogers.LoadGenderCatalog(eventsData, translationsData)
		if err != nil {
			fail(err)
		}
		catalogs = append(catalogs, catalog)
	}
	if *classEvents != "" {
		eventsData, err := os.ReadFile(*classEvents)
		if err != nil {
			fail(err)
		}
		translationsData, err := os.ReadFile(*classTranslations)
		if err != nil {
			fail(err)
		}
		catalog, err := buckrogers.LoadClassCatalog(eventsData, translationsData)
		if err != nil {
			fail(err)
		}
		catalogs = append(catalogs, catalog)
	}
	if *rosterEvents != "" {
		eventsData, err := os.ReadFile(*rosterEvents)
		if err != nil {
			fail(err)
		}
		translationsData, err := os.ReadFile(*rosterTranslations)
		if err != nil {
			fail(err)
		}
		catalog, err := buckrogers.LoadRosterCatalog(eventsData, translationsData)
		if err != nil {
			fail(err)
		}
		catalogs = append(catalogs, catalog)
	}
	if *characterSheetEvents != "" {
		eventsData, err := os.ReadFile(*characterSheetEvents)
		if err != nil {
			fail(err)
		}
		translationsData, err := os.ReadFile(*characterSheetTranslations)
		if err != nil {
			fail(err)
		}
		catalog, err := buckrogers.LoadCharacterSheetCatalog(eventsData, translationsData)
		if err != nil {
			fail(err)
		}
		catalogs = append(catalogs, catalog)
	}
	if *namePromptEvents != "" {
		eventsData, err := os.ReadFile(*namePromptEvents)
		if err != nil {
			fail(err)
		}
		translationsData, err := os.ReadFile(*namePromptTranslations)
		if err != nil {
			fail(err)
		}
		catalog, err := buckrogers.LoadNamePromptCatalog(eventsData, translationsData)
		if err != nil {
			fail(err)
		}
		catalogs = append(catalogs, catalog)
	}
	if *careerSkillEvents != "" {
		eventsData, err := os.ReadFile(*careerSkillEvents)
		if err != nil {
			fail(err)
		}
		translationsData, err := os.ReadFile(*careerSkillTranslations)
		if err != nil {
			fail(err)
		}
		catalog, err := buckrogers.LoadCareerSkillCatalog(eventsData, translationsData)
		if err != nil {
			fail(err)
		}
		catalogs = append(catalogs, catalog)
	}
	if *technicalSkillEvents != "" {
		eventsData, err := os.ReadFile(*technicalSkillEvents)
		if err != nil {
			fail(err)
		}
		translationsData, err := os.ReadFile(*technicalSkillTranslations)
		if err != nil {
			fail(err)
		}
		catalog, err := buckrogers.LoadTechnicalSkillCatalog(eventsData, translationsData)
		if err != nil {
			fail(err)
		}
		catalogs = append(catalogs, catalog)
	}
	catalog, err := buckrogers.MergeMenuCatalogs(catalogs...)
	if err != nil {
		fail(err)
	}
	var actionCatalog *buckrogers.ActionBarCatalog
	var actionRequestCatalog *buckrogers.ActionBarRequestCatalog
	if *actionBarEvents != "" {
		data, err := os.ReadFile(*actionBarEvents)
		if err != nil {
			fail(err)
		}
		if *actionBarTranslations == "" {
			actionCatalog, err = buckrogers.LoadActionBarCatalog(data)
		} else {
			translations, readErr := os.ReadFile(*actionBarTranslations)
			if readErr != nil {
				fail(readErr)
			}
			actionRequestCatalog, err = buckrogers.LoadActionBarRequestCatalog(data, translations)
		}
		if err != nil {
			fail(err)
		}
	}
	var actionBarPresenter *buckrogers.RuntimeActionBarOverlay
	if *actionBarOverlayOut != "" {
		if actionRequestCatalog == nil {
			fail(fmt.Errorf("操作列覆繪需要 action request catalog"))
		}
		rects, err := buckrogers.LoadActionBarOverlayRects("skill-action-bar-text-safe-rects.tsv", mustReadFile(*actionBarRects))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*actionBarOverlayFont)
		if err != nil {
			fail(err)
		}
		style := buckrogers.HotkeyPreservingActionBarNormalStyle()
		actionBarPresenter, err = buckrogers.NewRuntimeActionBarOverlay(actionRequestCatalog, rects, font, *actionBarOverlayScale, &style)
		if err != nil {
			fail(err)
		}
	}
	var manualCatalog *buckrogers.Catalog
	var manualPresenter *buckrogers.RuntimeManualOverlay
	if *manualOut != "" {
		var err error
		manualCatalog, err = buckrogers.LoadCatalog(mustReadFile(*manualEvents), mustReadFile(*manualOrdinals), mustReadFile(*manualTranslations))
		if err != nil {
			fail(err)
		}
		layout, err := buckrogers.LoadManualOverlayLayout("manual-overlay-layout.tsv", mustReadFile(*manualLayout))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*manualFont)
		if err != nil {
			fail(err)
		}
		manualPresenter, err = buckrogers.NewRuntimeManualOverlay(layout, manualCatalog, font, *manualScale)
		if err != nil {
			fail(err)
		}
	}
	m := machine.New()
	d := dos.New(m, ".")
	d.Install()
	if err := state.Load(*statePath, m, d); err != nil {
		fail(err)
	}
	if err := configureScratch(d, *scratch); err != nil {
		fail(err)
	}
	var manualFrameCallbacks uint64
	var presenter *buckrogers.RuntimeMenuOverlay
	if *overlayOut != "" {
		inputs := []struct{ name, path string }{
			{"menu-text-safe-rects.tsv", *menuRects},
			{"gender-text-safe-rects.tsv", *genderRects},
			{"class-text-safe-rects.tsv", *classRects},
			{"save-roster-join-text-safe-rects.tsv", *rosterRects},
			{"name-prompt-text-safe-rects.tsv", *namePromptRects},
			{"career-skill-screen-text-safe-rects.tsv", *careerSkillRects},
			{"technical-skill-screen-text-safe-rects.tsv", *technicalSkillRects},
		}
		var rectCatalogs []*buckrogers.MenuOverlayRects
		for _, input := range inputs {
			if input.path == "" {
				continue
			}
			data, err := os.ReadFile(input.path)
			if err != nil {
				fail(err)
			}
			rectCatalog, err := buckrogers.LoadMenuOverlayRects(input.name, data)
			if err != nil {
				fail(err)
			}
			rectCatalogs = append(rectCatalogs, rectCatalog)
		}
		if *characterSheetRects != "" {
			data, err := os.ReadFile(*characterSheetRects)
			if err != nil {
				fail(err)
			}
			rectCatalog, err := buckrogers.LoadCharacterSheetOverlayRects("character-sheet-text-safe-rects.tsv", data)
			if err != nil {
				fail(err)
			}
			rectCatalogs = append(rectCatalogs, rectCatalog)
		}
		rects, err := buckrogers.MergeMenuOverlayRects(rectCatalogs...)
		if err != nil {
			fail(err)
		}
		if err := buckrogers.ValidateMenuOverlayCoverage(catalog, rects); err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*overlayFont)
		if err != nil {
			fail(err)
		}
		presenter, err = buckrogers.NewRuntimeMenuOverlay(rects, font, *overlayScale)
		if err != nil {
			fail(err)
		}
		m.SetOnFrame(func() {
			presenter.Frame(m.Indexed(), m.Palette())
			if actionBarPresenter != nil {
				actionBarPresenter.Frame(m.Indexed(), m.Palette())
			}
			if manualPresenter != nil {
				manualFrameCallbacks++
				manualPresenter.Frame(m.Indexed(), m.Palette())
			}
		})
	}
	if actionBarPresenter != nil && presenter == nil && manualPresenter == nil {
		m.SetOnFrame(func() { actionBarPresenter.Frame(m.Indexed(), m.Palette()) })
	}
	if manualPresenter != nil && presenter == nil {
		m.SetOnFrame(func() {
			if actionBarPresenter != nil {
				actionBarPresenter.Frame(m.Indexed(), m.Palette())
			}
			manualFrameCallbacks++
			manualPresenter.Frame(m.Indexed(), m.Palette())
		})
	}
	start := m.Steps
	r := buckrogers.NewMenuRequestWatcher(catalog)
	var manualWatcher *buckrogers.Watcher
	var manualBridge *buckrogers.ManualPresentationBridge
	if manualPresenter != nil {
		manualWatcher = buckrogers.NewWatcher(manualCatalog)
		consumer, err := buckrogers.NewManualPresentationConsumer(manualPresenter)
		if err != nil {
			fail(err)
		}
		manualBridge, err = buckrogers.NewManualPresentationBridge(manualWatcher, consumer)
		if err != nil {
			fail(err)
		}
	}
	// 操作列協定與純手冊收據無關。只有呼叫端明示提供正式 catalog 時才觀測；
	// 未設定但尚在途中的操作列協定，不得拒絕另一條已選用的手冊收據。
	// 啟用後的既有終態檢查仍維持失敗即關閉。
	var actionWatcher *buckrogers.ActionBarWatcher
	switch actionWatcherModeForCatalogs(actionCatalog, actionRequestCatalog) {
	case actionWatcherRequest:
		actionWatcher = buckrogers.NewActionBarRequestWatcher(actionRequestCatalog)
	case actionWatcherEvent:
		actionWatcher = buckrogers.NewActionBarWatcher(actionCatalog)
	}
	nextKey := 0
	var clears []clearJSON
	var glyphs []glyphJSON
	var glyphPending *glyphFrame
	var glyphRun *glyphRunFrame
	var glyphRuns []glyphRunJSON
	glyphDrops := 0
	var storyWrite *pixelWriteJSON
	storyBefore := make([]byte, 320*40)
	if *storyPixelTrace {
		copy(storyBefore, m.Indexed()[17*8*320:22*8*320])
	}
	flushGlyphRun := func() {
		if glyphRun == nil {
			return
		}
		e := glyphRun.event
		hash := sha256.Sum256(glyphRun.bytes)
		glyphRuns = append(glyphRuns, glyphRunJSON{e.EntryStep, e.PostCallStep, e.Caller,
			uint8(len(glyphRun.bytes)), hex.EncodeToString(hash[:]),
			e.Mode, e.Repeat, e.Background, e.Foreground, e.Row, e.Column})
		glyphRun = nil
	}
	observeGlyph := func(e glyphJSON, b byte) {
		if glyphRun != nil && glyphRun.event.Caller == e.Caller && glyphRun.event.Mode == e.Mode &&
			glyphRun.event.Repeat == e.Repeat && glyphRun.event.Background == e.Background &&
			glyphRun.event.Foreground == e.Foreground && glyphRun.event.Row == e.Row &&
			int(e.Column) == int(glyphRun.event.Column)+len(glyphRun.bytes) {
			glyphRun.bytes = append(glyphRun.bytes, b)
			glyphRun.event.PostCallStep = e.PostCallStep
			return
		}
		flushGlyphRun()
		glyphRun = &glyphRunFrame{event: e, bytes: []byte{b}}
	}
	for m.Steps < *until && !d.Exited {
		for nextKey < len(keys) && m.Steps >= keys[nextKey].Step {
			key := keys[nextKey]
			if !m.PushBIOSKey(key.Scan, key.ASCII) {
				fail(fmt.Errorf("BIOS 鍵盤緩衝區已滿"))
			}
			nextKey++
		}
		at := buckrogers.Address{Segment: m.CPU.Seg[cpu.CS], Offset: m.CPU.IP}
		ss, sp := m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP]
		if glyphPending != nil && at == glyphPending.event.Caller {
			if ss == glyphPending.ss && sp == glyphPending.sp+0x12 {
				glyphPending.event.PostCallStep = m.Steps
				glyphs = append(glyphs, glyphPending.event)
				observeGlyph(glyphPending.event, glyphPending.glyph)
			} else {
				glyphDrops++
			}
			glyphPending = nil
		}
		actionRequestBefore := 0
		if actionWatcher != nil {
			actionRequestBefore = len(actionWatcher.Requests())
			actionWatcher.ObserveInstruction(at, ss, sp, m.Steps)
		}
		if at == (buckrogers.Address{Segment: 0x026F, Offset: 0x029C}) {
			bottom := m.Read8(cpu.Addr(ss, sp+4))
			right := m.Read8(cpu.Addr(ss, sp+6))
			top := m.Read8(cpu.Addr(ss, sp+8))
			left := m.Read8(cpu.Addr(ss, sp+10))
			if *clearTrace {
				clears = append(clears, clearJSON{Step: m.Steps, Bottom: bottom, Right: right, Top: top, Left: left})
			}
			flushGlyphRun()
			if actionWatcher != nil {
				actionWatcher.ObserveClear(bottom, right, top, left)
			}
			if manualWatcher != nil {
				// A proven clear can also be the guarded return instruction of
				// an in-flight manual dispatcher call. Preserve that post-call
				// before applying lifecycle invalidation.
				manualWatcher.ObserveInstruction(at, ss, sp, m.Steps)
				manualWatcher.ObserveClear(m.Steps)
			}
			if presenter != nil {
				if err := presenter.ClearTextCells(bottom, right, top, left); err != nil {
					fail(err)
				}
			}
			if actionBarPresenter != nil {
				if err := actionBarPresenter.ClearTextCells(bottom, right, top, left); err != nil {
					fail(err)
				}
			}
		} else if at == (buckrogers.Address{Segment: 0x0763, Offset: 0x0424}) {
			caller := buckrogers.Address{Segment: m.Read16(cpu.Addr(ss, sp+2)), Offset: m.Read16(cpu.Addr(ss, sp))}
			var args [6]uint16
			for i := range args {
				args[i] = m.Read16(cpu.Addr(ss, sp+4+uint16(i)*2))
			}
			base := cpu.Addr(args[1], args[0])
			n := int(m.Read8(base))
			original := make([]byte, n)
			for i := range original {
				original[i] = m.Read8(base + 1 + uint32(i))
			}
			r.ObserveDispatchEntry(caller, ss, sp, args, original, m.Steps)
			if manualWatcher != nil {
				manualWatcher.ObserveDispatchEntryWithStyle(caller, ss, sp, string(original), buckrogers.ManualTextStyle{
					Background: uint8(args[2]), Foreground: uint8(args[3]), Row: uint8(args[4]), Column: uint8(args[5]),
				}, m.Steps)
			}
		} else if at == (buckrogers.Address{Segment: 0x0763, Offset: 0x026B}) {
			caller := buckrogers.Address{Segment: m.Read16(cpu.Addr(ss, sp+2)), Offset: m.Read16(cpu.Addr(ss, sp))}
			var args [7]uint16
			for i := range args {
				args[i] = m.Read16(cpu.Addr(ss, sp+4+uint16(i)*2))
			}
			if *glyphTrace && m.Steps >= *glyphTraceFrom {
				if glyphPending != nil {
					glyphDrops++
				}
				glyphPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller,
					Mode: uint8(args[0]), Repeat: uint8(args[2]),
					Background: uint8(args[3]), Foreground: uint8(args[4]), Row: uint8(args[5]), Column: uint8(args[6])}, ss: ss, sp: sp, glyph: uint8(args[1])}
			}
			if actionWatcher != nil {
				actionWatcher.ObserveGlyphEntry(caller, ss, sp, args, m.Steps)
			}
		} else {
			eventBefore, requestBefore := r.EventCount(), r.RequestCount()
			r.ObserveInstruction(at, ss, sp, m.Steps)
			if manualWatcher != nil {
				manualWatcher.ObserveInstruction(at, ss, sp, m.Steps)
			}
			if r.RequestCount() > requestBefore {
				request, ok := r.LastRequest()
				if !ok {
					fail(fmt.Errorf("runtime request count advanced without request"))
				}
				if actionWatcher != nil {
					actionWatcher.ObserveAnchorEvent(request.EventKey)
				}
				if actionBarPresenter != nil {
					actionBarPresenter.ObserveAnchorEvent(request.EventKey)
				}
			}
			if presenter != nil && r.EventCount() > eventBefore && r.RequestCount() > requestBefore {
				event, eventOK := r.LastEvent()
				request, requestOK := r.LastRequest()
				if !eventOK || !requestOK || presenter.Apply(event, request, m.Palette()) != nil {
					fail(fmt.Errorf("runtime overlay apply 失敗"))
				}
			}
		}
		if actionBarPresenter != nil && actionWatcher != nil && len(actionWatcher.Requests()) > actionRequestBefore {
			events, requests := actionWatcher.Events(), actionWatcher.Requests()
			if len(events) == 0 || len(requests) == 0 || actionBarPresenter.Apply(events[len(events)-1], requests[len(requests)-1], m.Palette()) != nil {
				fail(fmt.Errorf("runtime action bar overlay apply 失敗"))
			}
		}
		if manualBridge != nil {
			if style, ok := manualWatcher.ManualStyle(); ok {
				if err := manualPresenter.SetStyle(style); err != nil {
					fail(err)
				}
			}
			if _, err := manualBridge.Sync(); err != nil {
				fail(err)
			}
		}
		storySegment, storyOffset, storyByteCount := uint16(0), uint16(0), uint16(0)
		if *storyPixelTrace && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			// 0CF4:1B3A is the observed REP STOSB instruction. Capture only
			// destination metadata before execution: it lets a future adapter
			// use a bounded video-write intersection instead of scanning this
			// whole region after every instruction.
			storySegment, storyOffset, storyByteCount = m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]
		}
		if err := m.Step(); err != nil {
			fail(err)
		}
		if *storyPixelTrace && storyWrite == nil {
			pixels := m.Indexed()[17*8*320 : 22*8*320]
			x0, y0, x1, y1 := 320, 40, -1, -1
			for i := range pixels {
				if pixels[i] == storyBefore[i] {
					continue
				}
				x, y := i%320, i/320
				if x < x0 {
					x0 = x
				}
				if x > x1 {
					x1 = x
				}
				if y < y0 {
					y0 = y
				}
				if y > y1 {
					y1 = y
				}
			}
			if x1 >= 0 {
				storyWrite = &pixelWriteJSON{Step: m.Steps - 1, Caller: at,
					VideoSegment: storySegment, VideoOffset: storyOffset, ByteCount: storyByteCount,
					X0: uint16(x0), Y0: uint16(y0 + 17*8), X1: uint16(x1), Y1: uint16(y1 + 17*8)}
			}
			copy(storyBefore, pixels)
		}
	}
	flushGlyphRun()
	events := r.Events()
	requests := r.Requests()
	var actionEvents []buckrogers.ActionBarEvent
	var actionRequests []buckrogers.DisplayRequest
	if actionWatcher != nil {
		actionEvents = actionWatcher.Events()
		actionRequests = actionWatcher.Requests()
	}
	actionPending, actionDrops, actionMisses, actionRequestMisses := actionWatcherStatus(actionWatcher)
	if nextKey != len(keys) || r.Pending() || r.Drops() != 0 || (*want != 0 && len(events) != *want) ||
		(*wantRequests != 0 && len(requests) != *wantRequests) || actionPending || actionDrops != 0 ||
		(*wantActionBarEvents != 0 && len(actionEvents) != *wantActionBarEvents) ||
		(*wantActionBarRequests != 0 && len(actionRequests) != *wantActionBarRequests) || actionRequestMisses != 0 {
		fail(fmt.Errorf("收據失敗：keys=%d/%d events=%d want=%d requests=%d want_requests=%d pending=%v drops=%d misses=%d action_events=%d want_action=%d action_pending=%v action_drops=%d action_misses=%d",
			nextKey, len(keys), len(events), *want, len(requests), *wantRequests, r.Pending(), r.Drops(), r.Misses(),
			len(actionEvents), *wantActionBarEvents, actionPending, actionDrops, actionMisses))
	}
	if err := saveTerminalState(*stateOut, m, d); err != nil {
		fail(err)
	}
	out := make([]eventJSON, len(events))
	for i, e := range events {
		out[i] = eventJSON{
			e.EntryStep, e.PostCallStep, e.Caller, e.OriginalLength,
			hex.EncodeToString(e.OriginalSHA256[:]), e.Background, e.Foreground, e.Row, e.Column,
		}
	}
	requestOut := make([]requestJSON, len(requests))
	for i, request := range requests {
		requestOut[i] = requestJSON{request.EventKey, request.TextKey, len([]rune(request.Translation))}
	}
	actionOut := make([]actionBarEventJSON, len(actionEvents))
	for i, event := range actionEvents {
		actionOut[i] = actionBarEventJSON{
			event.EntryStep, event.PostCallStep, event.Screen, event.EventKey, event.Variant,
			event.OriginalLength, hex.EncodeToString(event.OriginalSHA256[:]), event.Row, event.Column,
			event.X0, event.Y0, event.X1, event.Y1,
		}
	}
	actionRequestOut := make([]requestJSON, len(actionRequests))
	for i, request := range actionRequests {
		actionRequestOut[i] = requestJSON{request.EventKey, request.TextKey, len([]rune(request.Translation))}
	}
	result := struct {
		StateStart             uint64                   `json:"state_start"`
		StoppedAt              uint64                   `json:"stopped_at"`
		BIOSInput              string                   `json:"bios_input,omitempty"`
		Events                 []eventJSON              `json:"events"`
		Requests               []requestJSON            `json:"requests,omitempty"`
		CatalogMisses          *int                     `json:"catalog_misses,omitempty"`
		BIOSKeys               []keyJSON                `json:"bios_keys,omitempty"`
		Scratch                string                   `json:"scratch,omitempty"`
		Writes                 []writeJSON              `json:"writes,omitempty"`
		FileOps                []fileOpJSON             `json:"file_ops,omitempty"`
		Unimplemented          []string                 `json:"unimplemented,omitempty"`
		OverlayScale           int                      `json:"overlay_scale,omitempty"`
		OverlayActions         []requestJSON            `json:"overlay_actions,omitempty"`
		ActiveOverlayKeys      []string                 `json:"active_overlay_keys,omitempty"`
		OverlayMissing         []string                 `json:"overlay_missing_glyphs,omitempty"`
		OverlayDrew            *bool                    `json:"overlay_drew,omitempty"`
		ActionBarEvents        []actionBarEventJSON     `json:"action_bar_events,omitempty"`
		ActionBarMisses        *int                     `json:"action_bar_misses,omitempty"`
		ActionBarDrops         *int                     `json:"action_bar_drops,omitempty"`
		ActionBarRequests      []requestJSON            `json:"action_bar_requests,omitempty"`
		ActionBarCatalogMisses *int                     `json:"action_bar_catalog_misses,omitempty"`
		ActionBarOverlay       *actionBarOverlayJSON    `json:"action_bar_overlay,omitempty"`
		MemorySHA256           string                   `json:"memory_sha256"`
		IndexedSHA256          string                   `json:"indexed_sha256"`
		PaletteSHA256          string                   `json:"palette_sha256"`
		ManualPresentation     []manualPresentationJSON `json:"manual_presentation_events,omitempty"`
		ManualObservations     []buckrogers.Observation `json:"manual_observations,omitempty"`
		ManualStyle            *manualStyleJSON         `json:"manual_style,omitempty"`
		ManualOverlay          *manualOverlayJSON       `json:"manual_overlay,omitempty"`
		Clears                 []clearJSON              `json:"clears,omitempty"`
		Glyphs                 []glyphJSON              `json:"glyphs,omitempty"`
		GlyphDrops             int                      `json:"glyph_drops,omitempty"`
		GlyphRuns              []glyphRunJSON           `json:"glyph_runs,omitempty"`
		StoryPixelWrite        *pixelWriteJSON          `json:"story_pixel_write,omitempty"`
	}{StateStart: start, StoppedAt: m.Steps, Events: out, Scratch: *scratch, ActionBarEvents: actionOut, Clears: clears, Glyphs: glyphs, GlyphDrops: glyphDrops, GlyphRuns: glyphRuns, StoryPixelWrite: storyWrite,
		ActionBarRequests: actionRequestOut, MemorySHA256: sha256hex(m.Mem), IndexedSHA256: sha256hex(m.Indexed()), PaletteSHA256: sha256hex(flatPalette(m.Palette()))}
	if *fileOps {
		result.Writes = make([]writeJSON, len(d.Wrote))
		for i, write := range d.Wrote {
			result.Writes[i] = writeJSON{Name: write.Name, N: write.N}
		}
		result.FileOps = make([]fileOpJSON, len(d.FileOps))
		for i, op := range d.FileOps {
			result.FileOps[i] = fileOpJSON{op.Step, op.Op, op.Fn, op.Handle, op.Name,
				op.Arg, op.Pos, op.Len, op.Whence, op.Failed}
		}
	}
	result.Unimplemented = unimplementedReport(*unimplemented, d)
	if presenter != nil {
		if *baselineOut != "" {
			baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *overlayScale)
			if err := os.WriteFile(*baselineOut, baseline, 0o644); err != nil {
				fail(err)
			}
		}
		rgba, missingRunes, drew := presenter.Draw(m.Indexed(), m.Palette())
		activeKeys := presenter.ActiveKeys()
		if err := validateOverlayDraw(activeKeys, missingRunes, drew); err != nil {
			fail(err)
		}
		if err := os.WriteFile(*overlayOut, rgba, 0o644); err != nil {
			fail(err)
		}
		result.OverlayScale, result.OverlayDrew = *overlayScale, &drew
		result.ActiveOverlayKeys = activeKeys
		for _, action := range presenter.Actions() {
			result.OverlayActions = append(result.OverlayActions, requestJSON{action.EventKey, action.TextKey, action.TranslationRunes})
		}
	}
	if manualPresenter != nil {
		styleObserved := manualPresenter.HasStyle()
		if styleObserved {
			manualPresenter.Frame(m.Indexed(), m.Palette())
		}
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *manualScale)
		rgba, missing, drew := manualPresenter.Draw(m.Indexed(), m.Palette())
		if err := validateManualOverlayDraw(manualPresenter.ActiveKeys(), missing, drew); err != nil {
			fail(err)
		}
		outside, inside, added := manualDiff(baseline, rgba, *manualScale)
		if outside != 0 || (len(manualPresenter.ActiveKeys()) != 0 && added == 0) {
			fail(fmt.Errorf("手冊覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*manualOut, *manualBaselineOut, *manualPNGOut, *manualBaselinePNGOut, rgba, baseline, *manualScale); err != nil {
			fail(err)
		}
		for _, event := range manualWatcher.PresentationEvents() {
			item := manualPresentationJSON{Step: event.Step, Kind: string(event.Kind), Generation: event.Generation}
			if event.Kind == buckrogers.ManualPresentationRequest {
				item.EventKey, item.TextKey, item.Runes = event.Request.EventKey, event.Request.TextKey, len([]rune(event.Request.Translation))
			}
			result.ManualPresentation = append(result.ManualPresentation, item)
		}
		result.ManualObservations = manualWatcher.Observations()
		if style, ok := manualWatcher.ManualStyle(); ok {
			result.ManualStyle = &manualStyleJSON{style.Background, style.Foreground, style.Row, style.Column}
		}
		manualResult := &manualOverlayJSON{Scale: *manualScale, ActiveKeys: manualPresenter.ActiveKeys(), Drew: drew,
			BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideClearRect: outside,
			DiffInsideClearRect: inside, AddedNonBaselinePixel: added, StyleObserved: styleObserved, FrameCallbacks: manualFrameCallbacks}
		for _, r := range missing {
			manualResult.MissingGlyphs = append(manualResult.MissingGlyphs, string(r))
		}
		for _, action := range manualPresenter.Actions() {
			manualResult.Actions = append(manualResult.Actions, requestJSON{action.EventKey, action.TextKey, action.TranslationRunes})
		}
		result.ManualOverlay = manualResult
	}
	if catalog != nil {
		result.Requests = requestOut
		misses := r.Misses()
		result.CatalogMisses = &misses
	}
	if actionCatalog != nil && actionWatcher != nil {
		misses, drops := actionWatcher.Misses(), actionWatcher.Drops()
		result.ActionBarMisses, result.ActionBarDrops = &misses, &drops
	}
	if actionRequestCatalog != nil && actionWatcher != nil {
		misses, drops, requestMisses := actionWatcher.Misses(), actionWatcher.Drops(), actionWatcher.RequestMisses()
		result.ActionBarMisses, result.ActionBarDrops, result.ActionBarCatalogMisses = &misses, &drops, &requestMisses
	}
	if actionBarPresenter != nil {
		actionBarPresenter.Frame(m.Indexed(), m.Palette())
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *actionBarOverlayScale)
		rgba, missing, drew := actionBarPresenter.Draw(m.Indexed(), m.Palette())
		if err := validateOverlayDraw(actionBarPresenter.ActiveKeys(), missing, drew); err != nil {
			fail(err)
		}
		outside, inside, added := actionBarDiff(baseline, rgba, *actionBarOverlayScale)
		if outside != 0 || (len(actionBarPresenter.ActiveKeys()) != 0 && added == 0) {
			fail(fmt.Errorf("操作列覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*actionBarOverlayOut, *actionBarBaselineOut, *actionBarPNGOut, *actionBarBaselinePNGOut, rgba, baseline, *actionBarOverlayScale); err != nil {
			fail(err)
		}
		result.ActionBarOverlay = &actionBarOverlayJSON{Scale: *actionBarOverlayScale, ActiveKeys: actionBarPresenter.ActiveKeys(), Drew: drew,
			BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideSafeRects: outside,
			DiffInsideSafeRects: inside, AddedNonBaselinePixel: added}
		for _, r := range missing {
			result.ActionBarOverlay.MissingGlyphs = append(result.ActionBarOverlay.MissingGlyphs, string(r))
		}
		for _, action := range actionBarPresenter.Actions() {
			if err := validateActionBarHotkeyStyle(action); err != nil {
				fail(err)
			}
			result.ActionBarOverlay.Actions = append(result.ActionBarOverlay.Actions, requestJSON{action.EventKey, action.TextKey, action.TranslationRunes})
			result.ActionBarOverlay.Styles = append(result.ActionBarOverlay.Styles, actionBarStyleJSON{
				EventKey: action.EventKey, Background: action.Background,
				RuneForegrounds: paletteIndices(action.RuneForegrounds),
			})
		}
	}
	if *enterAt != 0 {
		result.BIOSInput = fmt.Sprintf("Enter(scan=0x1c,ascii=0x0d,queued_at=%d)", *enterAt)
	}
	if len(genericKeys) != 0 {
		result.BIOSKeys = make([]keyJSON, len(keys))
		for i, key := range keys {
			result.BIOSKeys[i] = keyJSON{key.Step, key.Scan, key.ASCII}
		}
	}
	if *screenOut != "" {
		if err := writeIndexedScreen(*screenOut, m.Indexed()); err != nil {
			fail(err)
		}
	}
	if err := emitReceipt(os.Stdout, *receiptOut, result); err != nil {
		fail(err)
	}
}

func unimplementedReport(enabled bool, d *dos.DOS) []string {
	if !enabled {
		return nil
	}
	return d.UnimplementedReport()
}

type actionWatcherMode uint8

const (
	actionWatcherDisabled actionWatcherMode = iota
	actionWatcherEvent
	actionWatcherRequest
)

// actionWatcherModeForCatalogs 在兩份正式操作列 catalog 同時提供時，維持既有的
// request watcher 優先順序。
func actionWatcherModeForCatalogs(events *buckrogers.ActionBarCatalog, requests *buckrogers.ActionBarRequestCatalog) actionWatcherMode {
	if requests != nil {
		return actionWatcherRequest
	}
	if events != nil {
		return actionWatcherEvent
	}
	return actionWatcherDisabled
}

// actionWatcherStatus 將省略的操作列 catalog 明確排除於本收據；已設定的
// watcher 仍會把所有終態失敗回報給呼叫端。
func actionWatcherStatus(w *buckrogers.ActionBarWatcher) (pending bool, drops, misses, requestMisses int) {
	if w == nil {
		return false, 0, 0, 0
	}
	return w.Pending(), w.Drops(), w.Misses(), w.RequestMisses()
}

func saveTerminalState(path string, m *machine.Machine, d *dos.DOS) error {
	if path == "" {
		return nil
	}
	return state.Save(path, m, d)
}

func configureScratch(d *dos.DOS, path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("檢查 scratch：%w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("scratch 不是目錄：%s", path)
	}
	d.Scratch = path
	return nil
}

func emitReceipt(stdout io.Writer, path string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("編碼 JSON 收據：%w", err)
	}
	b = append(b, '\n')
	if path != "" {
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return fmt.Errorf("寫出 JSON 收據：%w", err)
		}
	}
	if _, err := stdout.Write(b); err != nil {
		return fmt.Errorf("寫出 stdout 收據：%w", err)
	}
	return nil
}

func writeIndexedScreen(path string, data []byte) error {
	if len(data) != 320*200 {
		return fmt.Errorf("indexed framebuffer 大小為 %d，要 64000", len(data))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("寫出 indexed framebuffer：%w", err)
	}
	return nil
}

func mustReadFile(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	return b
}

func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func flatPalette(palette [256][3]uint8) []byte {
	out := make([]byte, 256*3)
	for i := range palette {
		copy(out[i*3:i*3+3], palette[i][:])
	}
	return out
}

func validateManualOverlayFlags(events, ordinals, translations, layout, font, out, baseline, pngOut, baselinePNG string, scale int) error {
	any := events != "" || ordinals != "" || translations != "" || layout != "" || font != "" || out != "" || baseline != "" || pngOut != "" || baselinePNG != "" || scale != 0
	if !any {
		return nil
	}
	if events == "" || ordinals == "" || translations == "" || layout == "" || font == "" || out == "" || baseline == "" || pngOut == "" || baselinePNG == "" || (scale != 2 && scale != 3) {
		return fmt.Errorf("手冊覆繪需要完整 catalog、layout、字型、2/3 倍 raw RGBA 與 PNG 輸出")
	}
	return nil
}

func validateManualOverlayDraw(active []string, missing []rune, drew bool) error {
	if len(active) == 0 && !drew && len(missing) == 0 {
		return nil
	}
	if len(active) != 14 || !drew || len(missing) != 0 {
		return fmt.Errorf("手冊覆繪未完成：active=%d drew=%v missing=%d", len(active), drew, len(missing))
	}
	return nil
}

func manualDiff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	w := 320 * scale
	x0, x1, y0, y1 := 7*scale, 312*scale, 72*scale, 184*scale
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if string(baseline[i:i+4]) == string(overlay[i:i+4]) {
				continue
			}
			if x >= x0 && x < x1 && y >= y0 && y < y1 {
				inside++
				added++
			} else {
				outside++
			}
		}
	}
	return outside, inside, added
}

func writeManualOutputs(out, baselineOut, pngOut, baselinePNG string, rgba, baseline []byte, scale int) error {
	if err := os.WriteFile(out, rgba, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(baselineOut, baseline, 0o644); err != nil {
		return err
	}
	if err := writeRGBApng(pngOut, rgba, 320*scale, 200*scale); err != nil {
		return err
	}
	return writeRGBApng(baselinePNG, baseline, 320*scale, 200*scale)
}

func writeRGBApng(path string, rgba []byte, width, height int) error {
	if len(rgba) != width*height*4 {
		return fmt.Errorf("RGBA 長度 %d 不符 %dx%d", len(rgba), width, height)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, rgba)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func validateMenuCatalogFlags(events, translations string) error {
	if (events == "") != (translations == "") {
		return fmt.Errorf("menu-events 與 menu-translations 必須同時提供")
	}
	return nil
}

func validateCatalogFlags(menuEvents, menuTranslations, genderEvents, genderTranslations,
	classEvents, classTranslations, rosterEvents, rosterTranslations,
	characterSheetEvents, characterSheetTranslations string) error {
	if err := validateMenuCatalogFlags(menuEvents, menuTranslations); err != nil {
		return err
	}
	if (genderEvents == "") != (genderTranslations == "") {
		return fmt.Errorf("gender-events 與 gender-translations 必須同時提供")
	}
	if (classEvents == "") != (classTranslations == "") {
		return fmt.Errorf("class-events 與 class-translations 必須同時提供")
	}
	if (rosterEvents == "") != (rosterTranslations == "") {
		return fmt.Errorf("roster-events 與 roster-translations 必須同時提供")
	}
	if (characterSheetEvents == "") != (characterSheetTranslations == "") {
		return fmt.Errorf("character-sheet-events 與 character-sheet-translations 必須同時提供")
	}
	return nil
}

func validateNamePromptCatalogFlags(events, translations string) error {
	if (events == "") != (translations == "") {
		return fmt.Errorf("name-prompt-events 與 name-prompt-translations 必須成對提供")
	}
	return nil
}

func validateCareerSkillCatalogFlags(events, translations string) error {
	if (events == "") != (translations == "") {
		return fmt.Errorf("career-skill-events 與 career-skill-translations 必須成對提供")
	}
	return nil
}

func validateTechnicalSkillCatalogFlags(events, translations, careerEvents string) error {
	if (events == "") != (translations == "") {
		return fmt.Errorf("technical-skill-events 與 technical-skill-translations 必須成對提供")
	}
	if events != "" && careerEvents == "" {
		return fmt.Errorf("technical-skill catalog 必須同時提供 career-skill catalog 以解析共享標題")
	}
	return nil
}

func validateActionBarFlags(events, translations, careerEvents, technicalEvents string) error {
	if translations != "" && events == "" {
		return fmt.Errorf("skill-action-bar-translations 必須與 events 同時提供")
	}
	if events != "" && (careerEvents == "" || technicalEvents == "") {
		return fmt.Errorf("skill-action-bar-events 必須同時提供 career 與 technical skill catalogs 作 exact anchors")
	}
	return nil
}

// validateActionBarOverlayFlags keeps the runtime path all-or-nothing: an
// action-bar renderer must have the same identity inputs as its watcher, plus
// explicit geometry, font, scale, and paired baseline/overlay artifacts.
func validateActionBarOverlayFlags(events, translations, rects, font, out, baseline, pngOut, baselinePNG string, scale int) error {
	// The watcher/catalog flags are independently useful without a renderer.
	// Only renderer-owned flags opt into this all-or-nothing artifact contract.
	any := rects != "" || font != "" || out != "" || baseline != "" || pngOut != "" || baselinePNG != "" || scale != 0
	if !any {
		return nil
	}
	if events == "" || translations == "" || rects == "" || font == "" || out == "" || baseline == "" || pngOut == "" || baselinePNG == "" {
		return fmt.Errorf("操作列覆繪需要完整 events、translations、矩形、字型、raw RGBA 與 PNG 輸出")
	}
	if scale != 2 && scale != 3 {
		return fmt.Errorf("操作列覆繪倍率必須是 2 或 3")
	}
	return nil
}

// actionBarDiff accepts only the exact row-24 rectangles authorized by spec
// 215. action.add includes its proven unused cell at x=24..32 for clearing.
func actionBarDiff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	w := 320 * scale
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if string(baseline[i:i+4]) == string(overlay[i:i+4]) {
				continue
			}
			logicalX, logicalY := x/scale, y/scale
			permitted := logicalY >= 192 && logicalY < 200 &&
				((logicalX >= 0 && logicalX < 32) ||
					(logicalX >= 32 && logicalX < 96) ||
					(logicalX >= 104 && logicalX < 136) ||
					(logicalX >= 144 && logicalX < 176) ||
					(logicalX >= 184 && logicalX < 216))
			if permitted {
				inside++
				added++
			} else {
				outside++
			}
		}
	}
	return outside, inside, added
}

func validateActionBarHotkeyStyle(action buckrogers.ActionBarOverlayAction) error {
	if len(action.RuneForegrounds) != 5 {
		return fmt.Errorf("操作列 %s 配色長度為 %d，要 5", action.EventKey, len(action.RuneForegrounds))
	}
	if strings.HasSuffix(action.EventKey, ".normal") {
		want := buckrogers.HotkeyPreservingActionBarNormalStyle().RuneForegrounds
		if action.Background != 0 {
			return fmt.Errorf("操作列 normal %s 背景色為 %d，要 0", action.EventKey, action.Background)
		}
		for i := range want {
			if action.RuneForegrounds[i] != want[i] {
				return fmt.Errorf("操作列 normal %s rune %d 色號為 %d，要 %d", action.EventKey, i, action.RuneForegrounds[i], want[i])
			}
		}
		return nil
	}
	if strings.HasSuffix(action.EventKey, ".focus") {
		if action.Background != 15 {
			return fmt.Errorf("操作列 focus %s 背景色為 %d，要 15", action.EventKey, action.Background)
		}
		for i, foreground := range action.RuneForegrounds {
			if foreground != 0 {
				return fmt.Errorf("操作列 focus %s rune %d 色號為 %d，要 0", action.EventKey, i, foreground)
			}
		}
		return nil
	}
	return fmt.Errorf("操作列事件沒有 normal/focus variant：%s", action.EventKey)
}

func paletteIndices(colors []uint8) []int {
	indices := make([]int, len(colors))
	for i, color := range colors {
		indices[i] = int(color)
	}
	return indices
}

func validateOverlayDraw(activeKeys []string, missingRunes []rune, drew bool) error {
	if len(missingRunes) != 0 {
		return fmt.Errorf("runtime overlay draw 失敗：missing=%d", len(missingRunes))
	}
	if drew != (len(activeKeys) != 0) {
		return fmt.Errorf("runtime overlay draw 與 active keys 不一致：drew=%v active=%d", drew, len(activeKeys))
	}
	return nil
}

func validateOverlayFlags(menuEvents, menuRects, genderEvents, genderRects,
	classEvents, classRects, rosterEvents, rosterRects, characterSheetEvents, characterSheetRects,
	namePromptEvents, namePromptRects,
	careerSkillEvents, careerSkillRects,
	technicalSkillEvents, technicalSkillRects,
	font, out string, scale int) error {
	rects := []string{menuRects, genderRects, classRects, rosterRects, characterSheetRects, namePromptRects,
		careerSkillRects, technicalSkillRects}
	anyRect := false
	for _, rect := range rects {
		anyRect = anyRect || rect != ""
	}
	any := anyRect || font != "" || out != "" || scale != 0
	if !any {
		return nil
	}
	if !anyRect || font == "" || out == "" || scale == 0 {
		return fmt.Errorf("overlay catalog rects、font、scale 與 output 必須同時提供")
	}
	for _, pair := range [][3]string{
		{"menu", menuEvents, menuRects}, {"gender", genderEvents, genderRects},
		{"class", classEvents, classRects}, {"roster", rosterEvents, rosterRects},
		{"character-sheet", characterSheetEvents, characterSheetRects},
		{"name-prompt", namePromptEvents, namePromptRects},
		{"career-skill", careerSkillEvents, careerSkillRects},
		{"technical-skill", technicalSkillEvents, technicalSkillRects},
	} {
		if (pair[1] == "") != (pair[2] == "") {
			return fmt.Errorf("overlay %s catalog 與 rect 必須同時提供", pair[0])
		}
	}
	if technicalSkillRects != "" && careerSkillRects == "" {
		return fmt.Errorf("overlay technical-skill rectangles 必須同時提供 career-skill rectangles 以解析共享標題")
	}
	if scale != 2 && scale != 3 {
		return fmt.Errorf("overlay-scale 必須是 2 或 3")
	}
	return nil
}

func parseScheduledBIOSKey(value string) (scheduledBIOSKey, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 || !asciiDecimal(parts[0]) {
		return scheduledBIOSKey{}, fmt.Errorf("bios-key-at 必須是 STEP:SCAN_HEX:ASCII_HEX")
	}
	step, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || step == 0 {
		return scheduledBIOSKey{}, fmt.Errorf("bios-key-at step 必須是非零 uint64")
	}
	scan, err := parseHexByte(parts[1])
	if err != nil {
		return scheduledBIOSKey{}, fmt.Errorf("bios-key-at scan：%w", err)
	}
	ascii, err := parseHexByte(parts[2])
	if err != nil {
		return scheduledBIOSKey{}, fmt.Errorf("bios-key-at ascii：%w", err)
	}
	return scheduledBIOSKey{step, scan, ascii}, nil
}

func mergeBIOSKeySchedule(enterAt uint64, generic []scheduledBIOSKey, until uint64) ([]scheduledBIOSKey, error) {
	keys := append([]scheduledBIOSKey(nil), generic...)
	if enterAt != 0 {
		keys = append(keys, scheduledBIOSKey{enterAt, 0x1C, 0x0D})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Step < keys[j].Step })
	for i, key := range keys {
		if i > 0 && keys[i-1].Step == key.Step {
			return nil, fmt.Errorf("BIOS 鍵排程 step 重複：%d", key.Step)
		}
		if key.Step >= until {
			return nil, fmt.Errorf("BIOS 鍵排程 step %d 必須早於 until %d", key.Step, until)
		}
	}
	return keys, nil
}

func asciiDecimal(value string) bool {
	if value == "" {
		return false
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func parseHexByte(value string) (uint8, error) {
	if len(value) != 2 {
		return 0, fmt.Errorf("必須恰有兩位小寫十六進位")
	}
	for i := range value {
		if !((value[i] >= '0' && value[i] <= '9') || (value[i] >= 'a' && value[i] <= 'f')) {
			return 0, fmt.Errorf("必須恰有兩位小寫十六進位")
		}
	}
	n, err := strconv.ParseUint(value, 16, 8)
	return uint8(n), err
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
