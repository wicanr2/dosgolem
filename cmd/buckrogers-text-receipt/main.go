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

type storyOpeningOverlayJSON struct {
	Scale                 int      `json:"scale"`
	ActiveKeys            []string `json:"active_keys"`
	MissingGlyphs         []string `json:"missing_glyphs"`
	Drew                  bool     `json:"drew"`
	BaselineRGBA256       string   `json:"baseline_rgba_sha256"`
	OverlayRGBA256        string   `json:"overlay_rgba_sha256"`
	DiffOutsideStoryRect  int      `json:"diff_outside_story_rect"`
	DiffInsideStoryRect   int      `json:"diff_inside_story_rect"`
	AddedNonBaselinePixel int      `json:"added_nonbaseline_pixels"`
}
type storyPage2InvalidationJSON struct {
	Step             uint64             `json:"step"`
	Instruction      buckrogers.Address `json:"instruction"`
	VideoSegment     uint16             `json:"video_segment"`
	VideoOffset      uint16             `json:"video_offset"`
	ByteCount        uint16             `json:"byte_count"`
	ActiveKeysBefore int                `json:"active_keys_before"`
}

// storyOpeningInvalidationJSON is a content-safe lifecycle receipt.  It never
// includes source glyphs, player input, or video bytes: the address and the
// pre-execution span are enough to prove that an already active first-screen
// group was removed at the bounded READY transition edge.
type storyOpeningInvalidationJSON struct {
	Step             uint64             `json:"step"`
	Instruction      buckrogers.Address `json:"instruction"`
	VideoSegment     uint16             `json:"video_segment"`
	VideoOffset      uint16             `json:"video_offset"`
	ByteCount        uint16             `json:"byte_count"`
	Generation       uint64             `json:"generation"`
	ActiveKeysBefore int                `json:"active_keys_before"`
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

// instructionTraceJSON is deliberately bounded diagnostic metadata. It
// contains control-flow and register state, not game bytes or screen content.
type instructionTraceJSON struct {
	Step  uint64             `json:"step"`
	At    buckrogers.Address `json:"at"`
	AX    uint16             `json:"ax"`
	Flags uint16             `json:"flags"`
	SS    uint16             `json:"ss"`
	SP    uint16             `json:"sp"`
}

// storyFillWriteJSON records only pre-execution Mode 13h fill metadata. It
// deliberately excludes video bytes and text content.
type storyFillWriteJSON struct {
	Step  uint64             `json:"step"`
	At    buckrogers.Address `json:"at"`
	ES    uint16             `json:"es"`
	DI    uint16             `json:"di"`
	Count uint16             `json:"count"`
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

// keyReadJSON is deliberately content-safe: it records only the input word,
// the DOS/BIOS intake path and the original consumer's real-mode locations.
// It never serializes displayed text, memory contents, or host input.
type keyReadJSON struct {
	Step      uint64 `json:"step"`
	Via       string `json:"via"`
	Word      uint16 `json:"word"`
	CS        uint16 `json:"cs"`
	IP        uint16 `json:"ip"`
	CallerCS  uint16 `json:"caller_cs"`
	CallerIP  uint16 `json:"caller_ip"`
	Caller2CS uint16 `json:"caller2_cs"`
	Caller2IP uint16 `json:"caller2_ip"`
}

// keyPollJSON records only the BIOS function and whether a key was available.
// It intentionally excludes the key word and all original game content.
type keyPollJSON struct {
	Step      uint64 `json:"step"`
	AH        uint8  `json:"ah"`
	Available bool   `json:"available"`
	CS        uint16 `json:"cs"`
	IP        uint16 `json:"ip"`
}

// loadBIOSKeysReceipt imports only the already-content-free numeric keyboard
// schedule from a local receipt. It avoids copying a manual answer into shell
// history or a versioned replay script.
func loadBIOSKeysReceipt(path string) ([]scheduledBIOSKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var receipt struct {
		BIOSKeys []keyJSON `json:"bios_keys"`
	}
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("讀取 BIOS receipt：%w", err)
	}
	if len(receipt.BIOSKeys) == 0 || len(receipt.BIOSKeys) > 64 {
		return nil, fmt.Errorf("BIOS receipt 的鍵盤排程數量無效")
	}
	keys := make([]scheduledBIOSKey, len(receipt.BIOSKeys))
	for i, key := range receipt.BIOSKeys {
		keys[i] = scheduledBIOSKey{Step: key.QueuedAt, Scan: key.Scan, ASCII: key.ASCII}
	}
	return keys, nil
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
	event        glyphJSON
	ss, sp       uint16
	glyph        uint8
	highWordMask uint8
}

// glyphReturnEdgeJSON is content-safe control-flow evidence only; it never
// retains a glyph byte or a stack word.
type glyphReturnEdgeJSON struct {
	EntryStep         uint64             `json:"entry_step"`
	EntrySS           uint16             `json:"entry_ss"`
	EntrySP           uint16             `json:"entry_sp"`
	ReturnStep        uint64             `json:"return_step"`
	ReturnInstruction buckrogers.Address `json:"return_instruction"`
	ReturnOpcode      uint8              `json:"return_opcode"`
	HighWordMask      uint8              `json:"high_word_mask"`
	Mode              uint8              `json:"mode"`
	Repeat            uint8              `json:"repeat"`
	Background        uint8              `json:"background"`
	Foreground        uint8              `json:"foreground"`
	Row               uint8              `json:"row"`
	Column            uint8              `json:"column"`
	Caller            buckrogers.Address `json:"caller"`
	PostAddress       buckrogers.Address `json:"post_address"`
	SS                uint16             `json:"ss"`
	SP                uint16             `json:"sp"`
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

type bodyIconRect struct {
	EventKey string
	X, Y     int
	Width    int
	Height   int
}

// bodyIconFramebufferWriteJSON is content-safe: it identifies the
// pre-execution instruction and changed bounds, never the original pixels.
type bodyIconFramebufferWriteJSON struct {
	Step        uint64             `json:"step"`
	Instruction buckrogers.Address `json:"instruction"`
	EventKeys   []string           `json:"event_keys"`
	X0          uint16             `json:"x0"`
	Y0          uint16             `json:"y0"`
	X1          uint16             `json:"x1"`
	Y1          uint16             `json:"y1"`
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
	storyOpeningEvents := flag.String("story-opening-events", "", "正式 story-opening-events.tsv")
	storyOpeningTranslations := flag.String("story-opening-translations", "", "正式 story-opening.zh-TW.tsv")
	storyOpeningFont := flag.String("story-opening-overlay-font", "", "本機 16x16 首屏劇情 GOLEMFNT")
	storyOpeningScale := flag.Int("story-opening-overlay-scale", 0, "明示首屏劇情覆繪倍率 2 或 3")
	storyOpeningOut := flag.String("story-opening-overlay-rgba-out", "", "輸出首屏劇情覆繪後 RGBA framebuffer")
	storyOpeningBaselineOut := flag.String("story-opening-baseline-rgba-out", "", "輸出同 frame／palette、未覆繪的首屏劇情 RGBA baseline")
	storyOpeningPNGOut := flag.String("story-opening-overlay-png-out", "", "輸出首屏劇情覆繪 PNG")
	storyOpeningBaselinePNGOut := flag.String("story-opening-baseline-png-out", "", "輸出未覆繪首屏劇情 PNG")
	storyPage2Events := flag.String("story-page2-events", "", "正式 story-page2-events.tsv")
	storyPage2Translations := flag.String("story-page2-translations", "", "正式 story-page2.zh-TW.tsv")
	storyPage2Font := flag.String("story-page2-overlay-font", "", "本機 16x16 第 2 頁劇情 GOLEMFNT")
	storyPage2Scale := flag.Int("story-page2-overlay-scale", 0, "明示第 2 頁劇情覆繪倍率 2 或 3")
	storyPage2Out := flag.String("story-page2-overlay-rgba-out", "", "輸出第 2 頁劇情覆繪後 RGBA framebuffer")
	storyPage2BaselineOut := flag.String("story-page2-baseline-rgba-out", "", "輸出同 frame／palette、未覆繪的第 2 頁 RGBA baseline")
	storyPage2PNGOut := flag.String("story-page2-overlay-png-out", "", "輸出第 2 頁劇情覆繪 PNG")
	storyPage2BaselinePNGOut := flag.String("story-page2-baseline-png-out", "", "輸出未覆繪第 2 頁劇情 PNG")
	storyPage3Events := flag.String("story-page3-events", "", "正式 story-page3-events.tsv")
	storyPage3Translations := flag.String("story-page3-translations", "", "正式 story-page3.zh-TW.tsv")
	storyPage3Font := flag.String("story-page3-overlay-font", "", "本機 16x16 第 3 頁劇情 GOLEMFNT")
	storyPage3Scale := flag.Int("story-page3-overlay-scale", 0, "明示第 3 頁劇情覆繪倍率 2 或 3")
	storyPage3Out := flag.String("story-page3-overlay-rgba-out", "", "輸出第 3 頁劇情覆繪後 RGBA framebuffer")
	storyPage3BaselineOut := flag.String("story-page3-baseline-rgba-out", "", "輸出同 frame／palette、未覆繪的第 3 頁 RGBA baseline")
	storyPage3PNGOut := flag.String("story-page3-overlay-png-out", "", "輸出第 3 頁劇情覆繪 PNG")
	storyPage3BaselinePNGOut := flag.String("story-page3-baseline-png-out", "", "輸出未覆繪第 3 頁劇情 PNG")
	storyPage4Events := flag.String("story-page4-events", "", "正式 story-page4-events.tsv")
	storyPage4Translations := flag.String("story-page4-translations", "", "正式 story-page4.zh-TW.tsv")
	storyPage4Font := flag.String("story-page4-overlay-font", "", "本機 16x16 第 4 頁劇情 GOLEMFNT")
	storyPage4Scale := flag.Int("story-page4-overlay-scale", 0, "明示第 4 頁劇情覆繪倍率 2 或 3")
	storyPage4Out := flag.String("story-page4-overlay-rgba-out", "", "輸出第 4 頁劇情覆繪後 RGBA framebuffer")
	storyPage4BaselineOut := flag.String("story-page4-baseline-rgba-out", "", "輸出未覆繪第 4 頁劇情 RGBA baseline")
	storyPage4PNGOut := flag.String("story-page4-overlay-png-out", "", "輸出第 4 頁劇情覆繪 PNG")
	storyPage4BaselinePNGOut := flag.String("story-page4-baseline-png-out", "", "輸出未覆繪第 4 頁劇情 PNG")
	storyPage5Events := flag.String("story-page5-events", "", "正式 story-page5-events.tsv")
	storyPage5Translations := flag.String("story-page5-translations", "", "正式 story-page5.zh-TW.tsv")
	storyPage5Font := flag.String("story-page5-overlay-font", "", "本機 16x16 第 5 頁劇情 GOLEMFNT")
	storyPage5Scale := flag.Int("story-page5-overlay-scale", 0, "明示第 5 頁劇情覆繪倍率 2 或 3")
	storyPage5Out := flag.String("story-page5-overlay-rgba-out", "", "輸出第 5 頁劇情覆繪後 RGBA framebuffer")
	storyPage5BaselineOut := flag.String("story-page5-baseline-rgba-out", "", "輸出同 frame／palette、未覆繪第 5 頁 RGBA baseline")
	storyPage5PNGOut := flag.String("story-page5-overlay-png-out", "", "輸出第 5 頁劇情覆繪 PNG")
	storyPage5BaselinePNGOut := flag.String("story-page5-baseline-png-out", "", "輸出未覆繪第 5 頁劇情 PNG")
	storyPage6Events := flag.String("story-page6-events", "", "正式 story-page6-events.tsv")
	storyPage6Translations := flag.String("story-page6-translations", "", "正式 story-page6.zh-TW.tsv")
	storyPage6Font := flag.String("story-page6-overlay-font", "", "本機 16x16 第 6 頁劇情 GOLEMFNT")
	storyPage6Scale := flag.Int("story-page6-overlay-scale", 0, "明示第 6 頁劇情覆繪倍率 2 或 3")
	storyPage6Out := flag.String("story-page6-overlay-rgba-out", "", "輸出第 6 頁劇情覆繪後 RGBA framebuffer")
	storyPage6BaselineOut := flag.String("story-page6-baseline-rgba-out", "", "輸出同 frame／palette、未覆繪的第 6 頁 RGBA baseline")
	storyPage6PNGOut := flag.String("story-page6-overlay-png-out", "", "輸出第 6 頁劇情覆繪 PNG")
	storyPage6BaselinePNGOut := flag.String("story-page6-baseline-png-out", "", "輸出未覆繪第 6 頁劇情 PNG")
	storyPage7Events := flag.String("story-page7-events", "", "正式 story-page7-events.tsv")
	storyPage7Translations := flag.String("story-page7-translations", "", "正式 story-page7.zh-TW.tsv")
	storyPage7Font := flag.String("story-page7-overlay-font", "", "本機 16x16 第 7 頁劇情 GOLEMFNT")
	storyPage7Scale := flag.Int("story-page7-overlay-scale", 0, "明示第 7 頁劇情覆繪倍率 2 或 3")
	storyPage7Out := flag.String("story-page7-overlay-rgba-out", "", "輸出第 7 頁劇情覆繪後 RGBA framebuffer")
	storyPage7BaselineOut := flag.String("story-page7-baseline-rgba-out", "", "輸出同 frame／palette、未覆繪的第 7 頁 RGBA baseline")
	storyPage7PNGOut := flag.String("story-page7-overlay-png-out", "", "輸出第 7 頁劇情覆繪 PNG")
	storyPage7BaselinePNGOut := flag.String("story-page7-baseline-png-out", "", "輸出未覆繪第 7 頁劇情 PNG")
	storyPage8Events := flag.String("story-page8-events", "", "正式 story-page8-events.tsv")
	storyPage8Translations := flag.String("story-page8-translations", "", "正式 story-page8.zh-TW.tsv")
	storyPage8Font := flag.String("story-page8-overlay-font", "", "本機 16x16 第 8 頁劇情 GOLEMFNT")
	storyPage8Scale := flag.Int("story-page8-overlay-scale", 0, "明示第 8 頁劇情覆繪倍率 2 或 3")
	storyPage8Out := flag.String("story-page8-overlay-rgba-out", "", "輸出第 8 頁劇情覆繪後 RGBA framebuffer")
	storyPage8BaselineOut := flag.String("story-page8-baseline-rgba-out", "", "輸出同 frame／palette、未覆繪的第 8 頁 RGBA baseline")
	storyPage8PNGOut := flag.String("story-page8-overlay-png-out", "", "輸出第 8 頁劇情覆繪 PNG")
	storyPage8BaselinePNGOut := flag.String("story-page8-baseline-png-out", "", "輸出未覆繪第 8 頁劇情 PNG")
	screenOut := flag.String("screen-out", "", "成功後寫出終態 320×200 indexed framebuffer")
	receiptOut := flag.String("receipt-out", "", "成功後另寫出與 stdout 相同的 JSON 收據")
	stateOut := flag.String("state-out", "", "成功後保存終態 savestate（只供本機研究）")
	scratch := flag.String("scratch", "", "可選、已存在的 DOS 可寫暫存目錄")
	fileOps := flag.Bool("file-ops", false, "在收據加入 content-safe 檔案操作 metadata")
	unimplemented := flag.Bool("unimplemented", false, "在收據加入未實作 DOS／BIOS 服務統計")
	clearTrace := flag.Bool("clear-trace", false, "在收據加入 026F:029C 的 content-safe 清除矩形")
	glyphTrace := flag.Bool("glyph-trace", false, "在收據加入 0763:026B 的 content-safe glyph 呼叫")
	glyphTraceFrom := flag.Uint64("glyph-trace-from", 0, "glyph 追蹤的最早絕對步數；0 表示不過濾")
	glyphReturnTrace := flag.Bool("glyph-return-edge-trace", false, "記錄 0763:026B 的 content-safe RETF control-flow edge")
	storyPixelTrace := flag.Bool("story-pixel-trace", false, "記錄 story rows 17..21 的第一筆 indexed framebuffer 改寫")
	storyFillTrace := flag.Bool("story-fill-trace", false, "記錄前 64 筆與明示 story-fill-rows 範圍相交的原版 pre-write fill metadata")
	storyFillRows := flag.Uint("story-fill-rows", 5, "story-fill-trace 的列數：5（預設 rows 17..21）或 6（rows 17..22）")
	bodyIconFramebufferTrace := flag.Bool("body-icon-framebuffer-trace", false, "逐 step 記錄與正式身體圖示安全矩形相交的 framebuffer 變化")
	bodyIconRects := flag.String("body-icon-rects", "", "body-icon-framebuffer-trace 必須搭配的正式 body-icon-text-safe-rects.tsv")
	bodyIconFramebufferTraceFrom := flag.Uint64("body-icon-framebuffer-trace-from", 0, "身體圖示 framebuffer 診斷建立 baseline 並開始觀測的絕對 step")
	keyTrace := flag.Bool("key-trace", false, "記錄 content-safe BIOS/DOS 鍵盤取用 metadata（不改變輸入）")
	instructionTraceFrom := flag.Uint64("instruction-trace-from", 0, "有界指令追蹤的最早絕對步數；0 表示停用")
	instructionTraceLimit := flag.Uint64("instruction-trace-limit", 0, "有界指令追蹤最多記錄的指令數；0 表示停用")
	stopAtSegment := flag.Uint("stop-at-segment", 0, "診斷重播在此 runtime CS 停止；須與 stop-at-offset、stop-after-step 同時提供")
	stopAtOffset := flag.Uint("stop-at-offset", 0, "診斷重播在此 runtime IP 停止；須與 stop-at-segment、stop-after-step 同時提供")
	stopAfterStep := flag.Uint64("stop-after-step", 0, "診斷停止點生效的最早絕對步數；0 表示停用")
	biosKeysReceipt := flag.String("bios-keys-receipt", "", "本機私有 receipt 的 bios_keys 排程；不得加入版本控制")
	var genericKeys scheduledBIOSKeys
	flag.Var(&genericKeys, "bios-key-at", "可重複 STEP:SCAN_HEX:ASCII_HEX BIOS 鍵排程")
	flag.Parse()
	if *statePath == "" || *until == 0 {
		fail(fmt.Errorf("state 與 until 為必填"))
	}
	if *storyFillRows != 5 && *storyFillRows != 6 {
		fail(fmt.Errorf("story-fill-rows 只允許 5 或 6"))
	}
	if err := validateBodyIconTraceFlags(*bodyIconFramebufferTrace, *bodyIconRects, *bodyIconFramebufferTraceFrom, *until); err != nil {
		fail(err)
	}
	var bodyRects []bodyIconRect
	if *bodyIconFramebufferTrace {
		var err error
		bodyRects, err = loadBodyIconRects(*bodyIconRects)
		if err != nil {
			fail(err)
		}
	}
	if (*stopAtSegment != 0 || *stopAtOffset != 0 || *stopAfterStep != 0) && (*stopAtSegment > 0xFFFF || *stopAtOffset > 0xFFFF || *stopAtSegment == 0 || *stopAtOffset == 0 || *stopAfterStep == 0) {
		fail(fmt.Errorf("stop-at-segment、stop-at-offset、stop-after-step 必須同時為有效值"))
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
	if err := validateStoryOpeningOverlayFlags(*storyOpeningEvents, *storyOpeningTranslations, *storyOpeningFont,
		*storyOpeningOut, *storyOpeningBaselineOut, *storyOpeningPNGOut, *storyOpeningBaselinePNGOut, *storyOpeningScale); err != nil {
		fail(err)
	}
	if err := validateStoryOpeningOverlayFlags(*storyPage2Events, *storyPage2Translations, *storyPage2Font,
		*storyPage2Out, *storyPage2BaselineOut, *storyPage2PNGOut, *storyPage2BaselinePNGOut, *storyPage2Scale); err != nil {
		fail(fmt.Errorf("第 2 頁劇情覆繪：%w", err))
	}
	if err := validateStoryOpeningOverlayFlags(*storyPage3Events, *storyPage3Translations, *storyPage3Font,
		*storyPage3Out, *storyPage3BaselineOut, *storyPage3PNGOut, *storyPage3BaselinePNGOut, *storyPage3Scale); err != nil {
		fail(fmt.Errorf("第 3 頁劇情覆繪：%w", err))
	}
	if err := validateStoryOpeningOverlayFlags(*storyPage4Events, *storyPage4Translations, *storyPage4Font,
		*storyPage4Out, *storyPage4BaselineOut, *storyPage4PNGOut, *storyPage4BaselinePNGOut, *storyPage4Scale); err != nil {
		fail(fmt.Errorf("第 4 頁劇情覆繪：%w", err))
	}
	if err := validateStoryOpeningOverlayFlags(*storyPage5Events, *storyPage5Translations, *storyPage5Font,
		*storyPage5Out, *storyPage5BaselineOut, *storyPage5PNGOut, *storyPage5BaselinePNGOut, *storyPage5Scale); err != nil {
		fail(fmt.Errorf("第 5 頁劇情覆繪：%w", err))
	}
	if err := validateStoryOpeningOverlayFlags(*storyPage6Events, *storyPage6Translations, *storyPage6Font,
		*storyPage6Out, *storyPage6BaselineOut, *storyPage6PNGOut, *storyPage6BaselinePNGOut, *storyPage6Scale); err != nil {
		fail(fmt.Errorf("第 6 頁劇情覆繪：%w", err))
	}
	if err := validateStoryOpeningOverlayFlags(*storyPage7Events, *storyPage7Translations, *storyPage7Font,
		*storyPage7Out, *storyPage7BaselineOut, *storyPage7PNGOut, *storyPage7BaselinePNGOut, *storyPage7Scale); err != nil {
		fail(fmt.Errorf("第 7 頁劇情覆繪：%w", err))
	}
	if err := validateStoryOpeningOverlayFlags(*storyPage8Events, *storyPage8Translations, *storyPage8Font,
		*storyPage8Out, *storyPage8BaselineOut, *storyPage8PNGOut, *storyPage8BaselinePNGOut, *storyPage8Scale); err != nil {
		fail(fmt.Errorf("第 8 頁劇情覆繪：%w", err))
	}
	if *biosKeysReceipt != "" {
		receiptKeys, err := loadBIOSKeysReceipt(*biosKeysReceipt)
		if err != nil {
			fail(err)
		}
		genericKeys = append(genericKeys, receiptKeys...)
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
	var storyOpeningCatalog *buckrogers.StoryOpeningCatalog
	var storyOpeningText map[string]string
	var storyOpeningPresenter *buckrogers.RuntimeStoryOpeningOverlay
	if *storyOpeningEvents != "" {
		var err error
		storyOpeningCatalog, storyOpeningText, err = buckrogers.LoadStoryOpeningCatalog(
			*storyOpeningEvents, mustReadFile(*storyOpeningEvents), *storyOpeningTranslations, mustReadFile(*storyOpeningTranslations))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*storyOpeningFont)
		if err != nil {
			fail(err)
		}
		storyOpeningPresenter, err = buckrogers.NewRuntimeStoryOpeningOverlay(storyOpeningText, font, *storyOpeningScale)
		if err != nil {
			fail(err)
		}
	}
	var storyPage2Catalog *buckrogers.StoryPage2Catalog
	var storyPage2Presenter *buckrogers.RuntimeStoryPage2Overlay
	if *storyPage2Events != "" {
		var err error
		var text map[string]string
		storyPage2Catalog, text, err = buckrogers.LoadStoryPage2Catalog(*storyPage2Events, mustReadFile(*storyPage2Events), *storyPage2Translations, mustReadFile(*storyPage2Translations))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*storyPage2Font)
		if err != nil {
			fail(err)
		}
		storyPage2Presenter, err = buckrogers.NewRuntimeStoryPage2Overlay(text, font, *storyPage2Scale)
		if err != nil {
			fail(err)
		}
	}
	var storyPage3Catalog *buckrogers.StoryPage3Catalog
	var storyPage3Presenter *buckrogers.RuntimeStoryPage3Overlay
	if *storyPage3Events != "" {
		var err error
		var text map[string]string
		storyPage3Catalog, text, err = buckrogers.LoadStoryPage3Catalog(*storyPage3Events, mustReadFile(*storyPage3Events), *storyPage3Translations, mustReadFile(*storyPage3Translations))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*storyPage3Font)
		if err != nil {
			fail(err)
		}
		storyPage3Presenter, err = buckrogers.NewRuntimeStoryPage3Overlay(text, font, *storyPage3Scale)
		if err != nil {
			fail(err)
		}
	}
	var storyPage4Catalog *buckrogers.StoryPage4Catalog
	var storyPage4Presenter *buckrogers.RuntimeStoryPage4Overlay
	if *storyPage4Events != "" {
		var err error
		var text map[string]string
		storyPage4Catalog, text, err = buckrogers.LoadStoryPage4Catalog(*storyPage4Events, mustReadFile(*storyPage4Events), *storyPage4Translations, mustReadFile(*storyPage4Translations))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*storyPage4Font)
		if err != nil {
			fail(err)
		}
		storyPage4Presenter, err = buckrogers.NewRuntimeStoryPage4Overlay(text, font, *storyPage4Scale)
		if err != nil {
			fail(err)
		}
	}
	var storyPage5Catalog *buckrogers.StoryPage5Catalog
	var storyPage5Presenter *buckrogers.RuntimeStoryPage5Overlay
	if *storyPage5Events != "" {
		var err error
		var text map[string]string
		storyPage5Catalog, text, err = buckrogers.LoadStoryPage5Catalog(*storyPage5Events, mustReadFile(*storyPage5Events), *storyPage5Translations, mustReadFile(*storyPage5Translations))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*storyPage5Font)
		if err != nil {
			fail(err)
		}
		storyPage5Presenter, err = buckrogers.NewRuntimeStoryPage5Overlay(text, font, *storyPage5Scale)
		if err != nil {
			fail(err)
		}
	}
	var storyPage6Catalog *buckrogers.StoryPage6Catalog
	var storyPage6Presenter *buckrogers.RuntimeStoryPage6Overlay
	if *storyPage6Events != "" {
		var err error
		var text map[string]string
		storyPage6Catalog, text, err = buckrogers.LoadStoryPage6Catalog(*storyPage6Events, mustReadFile(*storyPage6Events), *storyPage6Translations, mustReadFile(*storyPage6Translations))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*storyPage6Font)
		if err != nil {
			fail(err)
		}
		storyPage6Presenter, err = buckrogers.NewRuntimeStoryPage6Overlay(text, font, *storyPage6Scale)
		if err != nil {
			fail(err)
		}
	}
	var storyPage7Catalog *buckrogers.StoryPage7Catalog
	var storyPage7Presenter *buckrogers.RuntimeStoryPage7Overlay
	if *storyPage7Events != "" {
		var err error
		var text map[string]string
		storyPage7Catalog, text, err = buckrogers.LoadStoryPage7Catalog(*storyPage7Events, mustReadFile(*storyPage7Events), *storyPage7Translations, mustReadFile(*storyPage7Translations))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*storyPage7Font)
		if err != nil {
			fail(err)
		}
		storyPage7Presenter, err = buckrogers.NewRuntimeStoryPage7Overlay(text, font, *storyPage7Scale)
		if err != nil {
			fail(err)
		}
	}
	var storyPage8Catalog *buckrogers.StoryPage8Catalog
	var storyPage8Presenter *buckrogers.RuntimeStoryPage8Overlay
	if *storyPage8Events != "" {
		var err error
		var text map[string]string
		storyPage8Catalog, text, err = buckrogers.LoadStoryPage8Catalog(*storyPage8Events, mustReadFile(*storyPage8Events), *storyPage8Translations, mustReadFile(*storyPage8Translations))
		if err != nil {
			fail(err)
		}
		font, err := xlate.LoadFont(*storyPage8Font)
		if err != nil {
			fail(err)
		}
		storyPage8Presenter, err = buckrogers.NewRuntimeStoryPage8Overlay(text, font, *storyPage8Scale)
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
	if *keyTrace {
		d.KeyPollTraceFrom = *instructionTraceFrom
		d.KeyPollTraceLimit = 4096
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
	}
	if presenter != nil || actionBarPresenter != nil || manualPresenter != nil || storyOpeningPresenter != nil || storyPage2Presenter != nil || storyPage3Presenter != nil || storyPage4Presenter != nil || storyPage5Presenter != nil || storyPage6Presenter != nil || storyPage7Presenter != nil || storyPage8Presenter != nil {
		m.SetOnFrame(func() {
			if presenter != nil {
				presenter.Frame(m.Indexed(), m.Palette())
			}
			if actionBarPresenter != nil {
				actionBarPresenter.Frame(m.Indexed(), m.Palette())
			}
			if manualPresenter != nil {
				manualFrameCallbacks++
				manualPresenter.Frame(m.Indexed(), m.Palette())
			}
			if storyOpeningPresenter != nil {
				storyOpeningPresenter.Frame(m.Indexed(), m.Palette())
			}
			if storyPage2Presenter != nil {
				storyPage2Presenter.Frame(m.Indexed(), m.Palette())
			}
			if storyPage3Presenter != nil {
				storyPage3Presenter.Frame(m.Indexed(), m.Palette())
			}
			if storyPage4Presenter != nil {
				storyPage4Presenter.Frame(m.Indexed(), m.Palette())
			}
			if storyPage5Presenter != nil {
				storyPage5Presenter.Frame(m.Indexed(), m.Palette())
			}
			if storyPage6Presenter != nil {
				storyPage6Presenter.Frame(m.Indexed(), m.Palette())
			}
			if storyPage7Presenter != nil {
				storyPage7Presenter.Frame(m.Indexed(), m.Palette())
			}
			if storyPage8Presenter != nil {
				storyPage8Presenter.Frame(m.Indexed(), m.Palette())
			}
		})
	}
	start := m.Steps
	keyPollsStart := d.KeyPolls
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
	var storyOpeningWatcher *buckrogers.StoryOpeningWatcher
	if storyOpeningCatalog != nil {
		var err error
		storyOpeningWatcher, err = buckrogers.NewStoryOpeningWatcher(storyOpeningCatalog)
		if err != nil {
			fail(err)
		}
	}
	var storyPage2Watcher *buckrogers.StoryPage2Watcher
	var storyPage2ReturnPending *glyphFrame
	storyPage2Generation := uint64(0)
	if storyPage2Catalog != nil {
		var err error
		storyPage2Watcher, err = buckrogers.NewStoryPage2Watcher(storyPage2Catalog)
		if err != nil {
			fail(err)
		}
	}
	var storyPage3Watcher *buckrogers.StoryPage3Watcher
	var storyPage3ReturnPending *glyphFrame
	storyPage3Generation := uint64(0)
	if storyPage3Catalog != nil {
		var err error
		storyPage3Watcher, err = buckrogers.NewStoryPage3Watcher(storyPage3Catalog)
		if err != nil {
			fail(err)
		}
	}
	var storyPage4Watcher *buckrogers.StoryPage4Watcher
	var storyPage4ReturnPending *glyphFrame
	storyPage4Generation := uint64(0)
	if storyPage4Catalog != nil {
		var err error
		storyPage4Watcher, err = buckrogers.NewStoryPage4Watcher(storyPage4Catalog)
		if err != nil {
			fail(err)
		}
	}
	var storyPage5Watcher *buckrogers.StoryPage5Watcher
	var storyPage5ReturnPending *glyphFrame
	storyPage5Generation := uint64(0)
	if storyPage5Catalog != nil {
		var err error
		storyPage5Watcher, err = buckrogers.NewStoryPage5Watcher(storyPage5Catalog)
		if err != nil {
			fail(err)
		}
	}
	var storyPage6Watcher *buckrogers.StoryPage6Watcher
	var storyPage6ReturnPending *glyphFrame
	storyPage6Generation := uint64(0)
	if storyPage6Catalog != nil {
		var err error
		storyPage6Watcher, err = buckrogers.NewStoryPage6Watcher(storyPage6Catalog)
		if err != nil {
			fail(err)
		}
	}
	var storyPage7Watcher *buckrogers.StoryPage7Watcher
	var storyPage7ReturnPending *glyphFrame
	storyPage7Generation := uint64(0)
	if storyPage7Catalog != nil {
		var err error
		storyPage7Watcher, err = buckrogers.NewStoryPage7Watcher(storyPage7Catalog)
		if err != nil {
			fail(err)
		}
	}
	var storyPage8Watcher *buckrogers.StoryPage8Watcher
	var storyPage8ReturnPending *glyphFrame
	storyPage8Generation := uint64(0)
	if storyPage8Catalog != nil {
		var err error
		storyPage8Watcher, err = buckrogers.NewStoryPage8Watcher(storyPage8Catalog)
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
	var glyphReturnPending *glyphFrame
	var storyGlyphReturnPending *glyphFrame
	storyOpeningGeneration := uint64(0)
	var storyOpeningInvalidations []storyOpeningInvalidationJSON
	var storyPage2Invalidations []storyPage2InvalidationJSON
	var storyPage3Invalidations []storyPage2InvalidationJSON
	var storyPage4Invalidations []storyPage2InvalidationJSON
	var storyPage5Invalidations []storyPage2InvalidationJSON
	var storyPage6Invalidations []storyPage2InvalidationJSON
	var storyPage7Invalidations []storyPage2InvalidationJSON
	var storyPage8Invalidations []storyPage2InvalidationJSON
	var storyFillWrites []storyFillWriteJSON
	var glyphReturnEdges []glyphReturnEdgeJSON
	var previousInstruction buckrogers.Address
	var previousOpcode uint8
	previousValid := false
	var glyphRun *glyphRunFrame
	var glyphRuns []glyphRunJSON
	glyphDrops := 0
	var storyWrite *pixelWriteJSON
	var bodyIconFramebufferWrites []bodyIconFramebufferWriteJSON
	var bodyIconBefore []byte
	var instructionTrace []instructionTraceJSON
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
		if *bodyIconFramebufferTrace && bodyIconBefore == nil && m.Steps >= *bodyIconFramebufferTraceFrom {
			bodyIconBefore = append([]byte(nil), m.Indexed()...)
		}
		ss, sp := m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP]
		if *instructionTraceFrom != 0 && *instructionTraceLimit != 0 && m.Steps >= *instructionTraceFrom && uint64(len(instructionTrace)) < *instructionTraceLimit {
			instructionTrace = append(instructionTrace, instructionTraceJSON{m.Steps, at, m.CPU.R[cpu.AX], m.CPU.Flags, ss, sp})
		}
		if *stopAfterStep != 0 && m.Steps >= *stopAfterStep && at == (buckrogers.Address{Segment: uint16(*stopAtSegment), Offset: uint16(*stopAtOffset)}) {
			break
		}
		if glyphReturnPending != nil && previousValid && at == glyphReturnPending.event.Caller && ss == glyphReturnPending.ss && sp == glyphReturnPending.sp+0x12 {
			pending := glyphReturnPending
			glyphReturnEdges = append(glyphReturnEdges, glyphReturnEdgeJSON{pending.event.EntryStep, pending.ss, pending.sp, m.Steps, previousInstruction, previousOpcode, pending.highWordMask, pending.event.Mode, pending.event.Repeat, pending.event.Background, pending.event.Foreground, pending.event.Row, pending.event.Column, pending.event.Caller, at, ss, sp})
			glyphReturnPending = nil
		}
		if previousValid && isVerifiedStoryOpeningReturn(previousInstruction, previousOpcode, at, ss, sp, storyGlyphReturnPending) {
			pending := storyGlyphReturnPending
			storyOpeningWatcher.ObserveVerifiedGlyphReturn(buckrogers.StoryOpeningVerifiedReturn{
				EntryStep: pending.event.EntryStep, PostCallStep: m.Steps, Caller: pending.event.Caller, SS: ss, SP: sp,
			})
			storyGlyphReturnPending = nil
		}
		if previousValid && isVerifiedStoryOpeningReturn(previousInstruction, previousOpcode, at, ss, sp, storyPage2ReturnPending) {
			p := storyPage2ReturnPending
			storyPage2Watcher.ObserveVerifiedGlyphReturn(buckrogers.StoryPage2VerifiedReturn{EntryStep: p.event.EntryStep, PostCallStep: m.Steps, Caller: p.event.Caller, SS: ss, SP: sp})
			storyPage2ReturnPending = nil
		}
		if previousValid && isVerifiedStoryOpeningReturn(previousInstruction, previousOpcode, at, ss, sp, storyPage3ReturnPending) {
			p := storyPage3ReturnPending
			storyPage3Watcher.ObserveVerifiedGlyphReturn(buckrogers.StoryPage3VerifiedReturn{EntryStep: p.event.EntryStep, PostCallStep: m.Steps, Caller: p.event.Caller, SS: ss, SP: sp})
			storyPage3ReturnPending = nil
		}
		if previousValid && isVerifiedStoryOpeningReturn(previousInstruction, previousOpcode, at, ss, sp, storyPage4ReturnPending) {
			storyPage4Watcher.ObserveVerifiedGlyphReturn(previousInstruction, previousOpcode, at, ss, sp, m.Steps)
			storyPage4ReturnPending = nil
		}
		if previousValid && isVerifiedStoryOpeningReturn(previousInstruction, previousOpcode, at, ss, sp, storyPage5ReturnPending) {
			p := storyPage5ReturnPending
			storyPage5Watcher.ObserveVerifiedGlyphReturn(buckrogers.StoryPage5VerifiedReturn{EntryStep: p.event.EntryStep, PostCallStep: m.Steps, Caller: p.event.Caller, SS: ss, SP: sp})
			storyPage5ReturnPending = nil
		}
		if previousValid && isVerifiedStoryOpeningReturn(previousInstruction, previousOpcode, at, ss, sp, storyPage6ReturnPending) {
			storyPage6Watcher.ObserveVerifiedGlyphReturn(previousInstruction, previousOpcode, at, ss, sp, m.Steps)
			storyPage6ReturnPending = nil
		}
		if previousValid && isVerifiedStoryOpeningReturn(previousInstruction, previousOpcode, at, ss, sp, storyPage7ReturnPending) {
			storyPage7Watcher.ObserveVerifiedGlyphReturn(previousInstruction, previousOpcode, at, ss, sp, m.Steps)
			storyPage7ReturnPending = nil
		}
		if previousValid && isVerifiedStoryOpeningReturn(previousInstruction, previousOpcode, at, ss, sp, storyPage8ReturnPending) {
			storyPage8Watcher.ObserveVerifiedGlyphReturn(previousInstruction, previousOpcode, at, ss, sp, m.Steps)
			storyPage8ReturnPending = nil
		}
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
			// This narrow prototype only needs the five story rows; retaining
			// every unrelated glyph return would create a huge receipt without
			// strengthening the first-screen return-edge contract.
			if *glyphReturnTrace && m.Steps >= *glyphTraceFrom && caller == (buckrogers.Address{Segment: 0x0763, Offset: 0x04FF}) {
				glyphReturnPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller, Mode: uint8(args[0]), Repeat: uint8(args[2]), Background: uint8(args[3]), Foreground: uint8(args[4]), Row: uint8(args[5]), Column: uint8(args[6])}, ss: ss, sp: sp, highWordMask: glyphWordHighMask(args)}
			}
			if storyOpeningWatcher != nil {
				if storyGlyphReturnPending != nil {
					storyOpeningWatcher.ObserveExecutionDiscontinuity()
				}
				storyOpeningWatcher.ObserveGlyphEntry(buckrogers.Address{Segment: 0x0763, Offset: 0x026B}, caller, ss, sp, args, m.Steps)
				storyGlyphReturnPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller}, ss: ss, sp: sp}
			}
			if storyPage2Watcher != nil {
				if storyPage2ReturnPending != nil {
					storyPage2Watcher.ObserveExecutionDiscontinuity()
				}
				storyPage2Watcher.ObserveGlyphEntry(buckrogers.Address{Segment: 0x0763, Offset: 0x026B}, caller, ss, sp, args, m.Steps)
				storyPage2ReturnPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller}, ss: ss, sp: sp}
			}
			if storyPage3Watcher != nil {
				if storyPage3ReturnPending != nil {
					storyPage3Watcher.ObserveExecutionDiscontinuity()
				}
				storyPage3Watcher.ObserveGlyphEntry(buckrogers.Address{Segment: 0x0763, Offset: 0x026B}, caller, ss, sp, args, m.Steps)
				storyPage3ReturnPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller}, ss: ss, sp: sp}
			}
			if storyPage4Watcher != nil {
				if storyPage4ReturnPending != nil {
					storyPage4Watcher.ObserveExecutionDiscontinuity()
				}
				storyPage4Watcher.ObserveGlyphEntry(buckrogers.Address{Segment: 0x0763, Offset: 0x026B}, caller, ss, sp, args, m.Steps)
				storyPage4ReturnPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller}, ss: ss, sp: sp}
			}
			if storyPage5Watcher != nil {
				if storyPage5ReturnPending != nil {
					storyPage5Watcher.ObserveExecutionDiscontinuity()
				}
				storyPage5Watcher.ObserveGlyphEntry(buckrogers.Address{Segment: 0x0763, Offset: 0x026B}, caller, ss, sp, args, m.Steps)
				storyPage5ReturnPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller}, ss: ss, sp: sp}
			}
			if storyPage6Watcher != nil {
				if storyPage6ReturnPending != nil {
					storyPage6Watcher.ObserveExecutionDiscontinuity()
				}
				storyPage6Watcher.ObserveGlyphEntry(buckrogers.Address{Segment: 0x0763, Offset: 0x026B}, caller, ss, sp, args, m.Steps)
				storyPage6ReturnPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller}, ss: ss, sp: sp}
			}
			if storyPage7Watcher != nil {
				if storyPage7ReturnPending != nil {
					storyPage7Watcher.ObserveExecutionDiscontinuity()
				}
				storyPage7Watcher.ObserveGlyphEntry(buckrogers.Address{Segment: 0x0763, Offset: 0x026B}, caller, ss, sp, args, m.Steps)
				storyPage7ReturnPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller}, ss: ss, sp: sp}
			}
			if storyPage8Watcher != nil {
				if storyPage8ReturnPending != nil {
					storyPage8Watcher.ObserveExecutionDiscontinuity()
				}
				storyPage8Watcher.ObserveGlyphEntry(buckrogers.Address{Segment: 0x0763, Offset: 0x026B}, caller, ss, sp, args, m.Steps)
				storyPage8ReturnPending = &glyphFrame{event: glyphJSON{EntryStep: m.Steps, Caller: caller}, ss: ss, sp: sp}
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
		if storyOpeningWatcher != nil && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			invalidated := storyOpeningWatcher.ObserveVideoWrite(at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX])
			if item, ok := storyOpeningInvalidation(at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX], m.Steps,
				storyOpeningWatcher.Generation(), storyOpeningPresenter.ActiveKeys(), invalidated); ok {
				storyOpeningInvalidations = append(storyOpeningInvalidations, item)
			}
			if invalidated {
				storyOpeningPresenter.Clear()
				storyOpeningGeneration = 0
			}
		}
		if storyPage2Watcher != nil && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			before := storyPage2Presenter.ActiveKeys()
			if storyPage2Watcher.ObserveVideoWrite(at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]) {
				storyPage2Invalidations = append(storyPage2Invalidations, storyPage2InvalidationJSON{m.Steps, at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX], len(before)})
				storyPage2Presenter.Clear()
				storyPage2Generation = 0
			}
		}
		if storyPage3Watcher != nil && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			before := storyPage3Presenter.ActiveKeys()
			if storyPage3Watcher.ObserveVideoWrite(at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]) {
				storyPage3Invalidations = append(storyPage3Invalidations, storyPage2InvalidationJSON{m.Steps, at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX], len(before)})
				storyPage3Presenter.Clear()
				storyPage3Generation = 0
			}
		}
		if storyPage4Watcher != nil && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			before := storyPage4Presenter.ActiveKeys()
			if storyPage4Watcher.ObserveVideoWrite(at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]) {
				storyPage4Invalidations = append(storyPage4Invalidations, storyPage2InvalidationJSON{m.Steps, at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX], len(before)})
				storyPage4Presenter.Clear()
				storyPage4Generation = 0
			}
		}
		if storyPage5Watcher != nil && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			before := storyPage5Presenter.ActiveKeys()
			if storyPage5Watcher.ObserveVideoWrite(at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]) {
				storyPage5Invalidations = append(storyPage5Invalidations, storyPage2InvalidationJSON{m.Steps, at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX], len(before)})
				storyPage5Presenter.Clear()
				storyPage5Generation = 0
			}
		}
		if storyPage6Watcher != nil && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			before := storyPage6Presenter.ActiveKeys()
			if storyPage6Watcher.ObserveVideoWrite(at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]) {
				storyPage6Invalidations = append(storyPage6Invalidations, storyPage2InvalidationJSON{m.Steps, at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX], len(before)})
				storyPage6Presenter.Clear()
				storyPage6Generation = 0
			}
		}
		if storyPage7Watcher != nil && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			before := storyPage7Presenter.ActiveKeys()
			if storyPage7Watcher.ObserveVideoWrite(at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]) {
				storyPage7Invalidations = append(storyPage7Invalidations, storyPage2InvalidationJSON{m.Steps, at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX], len(before)})
				storyPage7Presenter.Clear()
				storyPage7Generation = 0
			}
		}
		if storyPage8Watcher != nil && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			before := storyPage8Presenter.ActiveKeys()
			if storyPage8Watcher.ObserveVideoWrite(at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]) {
				storyPage8Invalidations = append(storyPage8Invalidations, storyPage2InvalidationJSON{m.Steps, at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX], len(before)})
				storyPage8Presenter.Clear()
				storyPage8Generation = 0
			}
		}
		if *storyFillTrace && len(storyFillWrites) < 64 && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) &&
			m.CPU.Seg[cpu.ES] == 0xA000 && storyFillIntersects(m.CPU.R[cpu.DI], m.CPU.R[cpu.CX], uint32(*storyFillRows)) {
			storyFillWrites = append(storyFillWrites, storyFillWriteJSON{m.Steps, at, m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]})
		}
		if storyOpeningWatcher != nil && storyOpeningWatcher.Active() && storyOpeningWatcher.Generation() != storyOpeningGeneration {
			events := storyOpeningEventsForGeneration(storyOpeningWatcher.Events(), storyOpeningWatcher.Generation())
			if err := storyOpeningPresenter.Apply(events, m.Palette()); err != nil {
				fail(fmt.Errorf("runtime story opening overlay apply 失敗：%w", err))
			}
			storyOpeningGeneration = storyOpeningWatcher.Generation()
		}
		if storyPage2Watcher != nil && storyPage2Watcher.Active() && storyPage2Watcher.Generation() != storyPage2Generation {
			events := make([]buckrogers.StoryPage2Event, 0, 4)
			for _, e := range storyPage2Watcher.Events() {
				if e.Generation == storyPage2Watcher.Generation() {
					events = append(events, e)
				}
			}
			if err := storyPage2Presenter.Apply(events, m.Palette()); err != nil {
				fail(fmt.Errorf("runtime story page2 overlay apply 失敗：%w", err))
			}
			storyPage2Generation = storyPage2Watcher.Generation()
		}
		if storyPage3Watcher != nil && storyPage3Watcher.Active() && storyPage3Watcher.Generation() != storyPage3Generation {
			events := make([]buckrogers.StoryPage3Event, 0, 5)
			for _, e := range storyPage3Watcher.Events() {
				if e.Generation == storyPage3Watcher.Generation() {
					events = append(events, e)
				}
			}
			if err := storyPage3Presenter.Apply(events, m.Palette()); err != nil {
				fail(fmt.Errorf("runtime story page3 overlay apply 失敗：%w", err))
			}
			storyPage3Generation = storyPage3Watcher.Generation()
		}
		if storyPage4Watcher != nil && storyPage4Watcher.Active() && storyPage4Watcher.Generation() != storyPage4Generation {
			events := make([]buckrogers.StoryPage4Event, 0, 6)
			for _, e := range storyPage4Watcher.Events() {
				if e.Generation == storyPage4Watcher.Generation() {
					events = append(events, e)
				}
			}
			if err := storyPage4Presenter.Apply(events, m.Palette()); err != nil {
				fail(fmt.Errorf("runtime story page4 overlay apply 失敗：%w", err))
			}
			storyPage4Generation = storyPage4Watcher.Generation()
		}
		if storyPage5Watcher != nil && storyPage5Watcher.Active() && storyPage5Watcher.Generation() != storyPage5Generation {
			events := make([]buckrogers.StoryPage5Event, 0, 5)
			for _, e := range storyPage5Watcher.Events() {
				if e.Generation == storyPage5Watcher.Generation() {
					events = append(events, e)
				}
			}
			if err := storyPage5Presenter.Apply(events, m.Palette()); err != nil {
				fail(fmt.Errorf("runtime story page5 overlay apply 失敗：%w", err))
			}
			storyPage5Generation = storyPage5Watcher.Generation()
		}
		if storyPage6Watcher != nil && storyPage6Watcher.Active() && storyPage6Watcher.Generation() != storyPage6Generation {
			events := make([]buckrogers.StoryPage6Event, 0, 6)
			for _, event := range storyPage6Watcher.Events() {
				if event.Generation == storyPage6Watcher.Generation() {
					events = append(events, event)
				}
			}
			if err := storyPage6Presenter.Apply(events, m.Palette()); err != nil {
				fail(fmt.Errorf("runtime story page6 overlay apply 失敗：%w", err))
			}
			storyPage6Generation = storyPage6Watcher.Generation()
		}
		if storyPage7Watcher != nil && storyPage7Watcher.Active() && storyPage7Watcher.Generation() != storyPage7Generation {
			events := make([]buckrogers.StoryPage7Event, 0, 6)
			for _, event := range storyPage7Watcher.Events() {
				if event.Generation == storyPage7Watcher.Generation() {
					events = append(events, event)
				}
			}
			if err := storyPage7Presenter.Apply(events, m.Palette()); err != nil {
				fail(fmt.Errorf("runtime story page7 overlay apply 失敗：%w", err))
			}
			storyPage7Generation = storyPage7Watcher.Generation()
		}
		if storyPage8Watcher != nil && storyPage8Watcher.Active() && storyPage8Watcher.Generation() != storyPage8Generation {
			events := make([]buckrogers.StoryPage8Event, 0, 4)
			for _, event := range storyPage8Watcher.Events() {
				if event.Generation == storyPage8Watcher.Generation() {
					events = append(events, event)
				}
			}
			if err := storyPage8Presenter.Apply(events, m.Palette()); err != nil {
				fail(fmt.Errorf("runtime story page8 overlay apply 失敗：%w", err))
			}
			storyPage8Generation = storyPage8Watcher.Generation()
		}
		storySegment, storyOffset, storyByteCount := uint16(0), uint16(0), uint16(0)
		if *storyPixelTrace && at == (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) {
			// 0CF4:1B3A is the observed REP STOSB instruction. Capture only
			// destination metadata before execution: it lets a future adapter
			// use a bounded video-write intersection instead of scanning this
			// whole region after every instruction.
			storySegment, storyOffset, storyByteCount = m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.CX]
		}
		previousInstruction, previousOpcode, previousValid = at, m.Read8(cpu.Addr(at.Segment, at.Offset)), true
		if err := m.Step(); err != nil {
			fail(err)
		}
		if bodyIconBefore != nil {
			if item, changed := observeBodyIconFramebuffer(m.Steps-1, at, bodyRects, bodyIconBefore, m.Indexed()); changed {
				if len(bodyIconFramebufferWrites) >= 65536 {
					fail(fmt.Errorf("body-icon framebuffer trace 超過 65536 筆"))
				}
				bodyIconFramebufferWrites = append(bodyIconFramebufferWrites, item)
			}
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
	if storyOpeningWatcher != nil && storyGlyphReturnPending != nil {
		storyOpeningWatcher.ObserveExecutionDiscontinuity()
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
	var storyFillRowsReceipt *uint
	if *storyFillTrace {
		rows := *storyFillRows
		storyFillRowsReceipt = &rows
	}
	result := struct {
		StateStart                uint64                         `json:"state_start"`
		StoppedAt                 uint64                         `json:"stopped_at"`
		BIOSInput                 string                         `json:"bios_input,omitempty"`
		Events                    []eventJSON                    `json:"events"`
		Requests                  []requestJSON                  `json:"requests,omitempty"`
		CatalogMisses             *int                           `json:"catalog_misses,omitempty"`
		BIOSKeys                  []keyJSON                      `json:"bios_keys,omitempty"`
		Scratch                   string                         `json:"scratch,omitempty"`
		Writes                    []writeJSON                    `json:"writes,omitempty"`
		FileOps                   []fileOpJSON                   `json:"file_ops,omitempty"`
		Unimplemented             []string                       `json:"unimplemented,omitempty"`
		OverlayScale              int                            `json:"overlay_scale,omitempty"`
		OverlayActions            []requestJSON                  `json:"overlay_actions,omitempty"`
		ActiveOverlayKeys         []string                       `json:"active_overlay_keys,omitempty"`
		OverlayMissing            []string                       `json:"overlay_missing_glyphs,omitempty"`
		OverlayDrew               *bool                          `json:"overlay_drew,omitempty"`
		ActionBarEvents           []actionBarEventJSON           `json:"action_bar_events,omitempty"`
		ActionBarMisses           *int                           `json:"action_bar_misses,omitempty"`
		ActionBarDrops            *int                           `json:"action_bar_drops,omitempty"`
		ActionBarRequests         []requestJSON                  `json:"action_bar_requests,omitempty"`
		ActionBarCatalogMisses    *int                           `json:"action_bar_catalog_misses,omitempty"`
		ActionBarOverlay          *actionBarOverlayJSON          `json:"action_bar_overlay,omitempty"`
		MemorySHA256              string                         `json:"memory_sha256"`
		IndexedSHA256             string                         `json:"indexed_sha256"`
		PaletteSHA256             string                         `json:"palette_sha256"`
		ManualPresentation        []manualPresentationJSON       `json:"manual_presentation_events,omitempty"`
		ManualObservations        []buckrogers.Observation       `json:"manual_observations,omitempty"`
		ManualStyle               *manualStyleJSON               `json:"manual_style,omitempty"`
		ManualOverlay             *manualOverlayJSON             `json:"manual_overlay,omitempty"`
		StoryOpeningOverlay       *storyOpeningOverlayJSON       `json:"story_opening_overlay,omitempty"`
		StoryOpeningInvalidations []storyOpeningInvalidationJSON `json:"story_opening_invalidations,omitempty"`
		StoryPage2Overlay         *storyOpeningOverlayJSON       `json:"story_page2_overlay,omitempty"`
		StoryPage2Invalidations   []storyPage2InvalidationJSON   `json:"story_page2_invalidations,omitempty"`
		StoryPage3Overlay         *storyOpeningOverlayJSON       `json:"story_page3_overlay,omitempty"`
		StoryPage3Invalidations   []storyPage2InvalidationJSON   `json:"story_page3_invalidations,omitempty"`
		StoryPage4Overlay         *storyOpeningOverlayJSON       `json:"story_page4_overlay,omitempty"`
		StoryPage4Invalidations   []storyPage2InvalidationJSON   `json:"story_page4_invalidations,omitempty"`
		StoryPage5Overlay         *storyOpeningOverlayJSON       `json:"story_page5_overlay,omitempty"`
		StoryPage5Invalidations   []storyPage2InvalidationJSON   `json:"story_page5_invalidations,omitempty"`
		StoryPage6Overlay         *storyOpeningOverlayJSON       `json:"story_page6_overlay,omitempty"`
		StoryPage6Invalidations   []storyPage2InvalidationJSON   `json:"story_page6_invalidations,omitempty"`
		StoryPage7Overlay         *storyOpeningOverlayJSON       `json:"story_page7_overlay,omitempty"`
		StoryPage7Invalidations   []storyPage2InvalidationJSON   `json:"story_page7_invalidations,omitempty"`
		StoryPage8Overlay         *storyOpeningOverlayJSON       `json:"story_page8_overlay,omitempty"`
		StoryPage8Invalidations   []storyPage2InvalidationJSON   `json:"story_page8_invalidations,omitempty"`
		Clears                    []clearJSON                    `json:"clears,omitempty"`
		Glyphs                    []glyphJSON                    `json:"glyphs,omitempty"`
		GlyphDrops                int                            `json:"glyph_drops,omitempty"`
		GlyphRuns                 []glyphRunJSON                 `json:"glyph_runs,omitempty"`
		GlyphReturnEdges          []glyphReturnEdgeJSON          `json:"glyph_return_edges,omitempty"`
		StoryPixelWrite           *pixelWriteJSON                `json:"story_pixel_write,omitempty"`
		StoryFillWrites           []storyFillWriteJSON           `json:"story_fill_writes,omitempty"`
		StoryFillRows             *uint                          `json:"story_fill_rows,omitempty"`
		BodyIconFramebufferWrites []bodyIconFramebufferWriteJSON `json:"body_icon_framebuffer_writes,omitempty"`
		KeyReads                  []keyReadJSON                  `json:"key_reads,omitempty"`
		KeyPollTrace              []keyPollJSON                  `json:"key_poll_trace,omitempty"`
		KeysPending               *int                           `json:"keys_pending,omitempty"`
		KeyPolls                  *int                           `json:"key_polls,omitempty"`
		KeyPollsDelta             *int                           `json:"key_polls_delta,omitempty"`
		InstructionTrace          []instructionTraceJSON         `json:"instruction_trace,omitempty"`
	}{StateStart: start, StoppedAt: m.Steps, Events: out, Scratch: *scratch, ActionBarEvents: actionOut, Clears: clears, Glyphs: glyphs, GlyphDrops: glyphDrops, GlyphReturnEdges: glyphReturnEdges, StoryPixelWrite: storyWrite, StoryFillWrites: storyFillWrites, StoryFillRows: storyFillRowsReceipt, BodyIconFramebufferWrites: bodyIconFramebufferWrites, StoryOpeningInvalidations: storyOpeningInvalidations, InstructionTrace: instructionTrace,
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
	result.StoryPage2Invalidations = storyPage2Invalidations
	result.StoryPage3Invalidations = storyPage3Invalidations
	result.StoryPage4Invalidations = storyPage4Invalidations
	result.StoryPage5Invalidations = storyPage5Invalidations
	result.StoryPage6Invalidations = storyPage6Invalidations
	result.StoryPage7Invalidations = storyPage7Invalidations
	result.StoryPage8Invalidations = storyPage8Invalidations
	result.Unimplemented = unimplementedReport(*unimplemented, d)
	if *keyTrace {
		pending := d.KeysPending()
		result.KeysPending = &pending
		polls := d.KeyPolls
		pollsDelta := polls - keyPollsStart
		result.KeyPolls = &polls
		result.KeyPollsDelta = &pollsDelta
		result.KeyReads = make([]keyReadJSON, len(d.KeyReads))
		for i, read := range d.KeyReads {
			result.KeyReads[i] = keyReadJSON{read.Step, read.Via, read.Word, read.CS, read.IP,
				read.CallerCS, read.CallerIP, read.Caller2CS, read.Caller2IP}
		}
		for _, poll := range d.KeyPollsTrace {
			if *instructionTraceFrom != 0 && poll.Step < *instructionTraceFrom {
				continue
			}
			result.KeyPollTrace = append(result.KeyPollTrace, keyPollJSON{poll.Step, poll.AH, poll.Available, poll.CS, poll.IP})
		}
	}
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
	if storyOpeningPresenter != nil {
		storyOpeningPresenter.Frame(m.Indexed(), m.Palette())
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *storyOpeningScale)
		rgba, missing, drew := storyOpeningPresenter.Draw(m.Indexed(), m.Palette())
		active := storyOpeningPresenter.ActiveKeys()
		if err := validateStoryOpeningOverlayDraw(active, missing, drew); err != nil {
			fail(err)
		}
		outside, inside, added := storyOpeningDiff(baseline, rgba, *storyOpeningScale)
		if outside != 0 || (len(active) != 0 && added == 0) {
			fail(fmt.Errorf("首屏劇情覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*storyOpeningOut, *storyOpeningBaselineOut, *storyOpeningPNGOut, *storyOpeningBaselinePNGOut, rgba, baseline, *storyOpeningScale); err != nil {
			fail(err)
		}
		storyResult := &storyOpeningOverlayJSON{Scale: *storyOpeningScale, ActiveKeys: active, Drew: drew,
			BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideStoryRect: outside,
			DiffInsideStoryRect: inside, AddedNonBaselinePixel: added}
		for _, r := range missing {
			storyResult.MissingGlyphs = append(storyResult.MissingGlyphs, string(r))
		}
		result.StoryOpeningOverlay = storyResult
	}
	if storyPage2Presenter != nil {
		storyPage2Presenter.Frame(m.Indexed(), m.Palette())
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *storyPage2Scale)
		rgba, missing, drew := storyPage2Presenter.Draw(m.Indexed(), m.Palette())
		active := storyPage2Presenter.ActiveKeys()
		if (len(active) == 0 && (drew || len(missing) != 0)) || (len(active) != 0 && (len(active) != 4 || !drew || len(missing) != 0)) {
			fail(fmt.Errorf("第 2 頁劇情覆繪未完成：active=%d drew=%v missing=%d", len(active), drew, len(missing)))
		}
		outside, inside, added := storyPage2Diff(baseline, rgba, *storyPage2Scale)
		if outside != 0 || (len(active) != 0 && added == 0) {
			fail(fmt.Errorf("第 2 頁劇情覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*storyPage2Out, *storyPage2BaselineOut, *storyPage2PNGOut, *storyPage2BaselinePNGOut, rgba, baseline, *storyPage2Scale); err != nil {
			fail(err)
		}
		item := &storyOpeningOverlayJSON{Scale: *storyPage2Scale, ActiveKeys: active, Drew: drew, BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideStoryRect: outside, DiffInsideStoryRect: inside, AddedNonBaselinePixel: added}
		for _, r := range missing {
			item.MissingGlyphs = append(item.MissingGlyphs, string(r))
		}
		result.StoryPage2Overlay = item
	}
	if storyPage3Presenter != nil {
		storyPage3Presenter.Frame(m.Indexed(), m.Palette())
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *storyPage3Scale)
		rgba, missing, drew := storyPage3Presenter.Draw(m.Indexed(), m.Palette())
		active := storyPage3Presenter.ActiveKeys()
		if (len(active) == 0 && (drew || len(missing) != 0)) || (len(active) != 0 && (len(active) != 5 || !drew || len(missing) != 0)) {
			fail(fmt.Errorf("第 3 頁劇情覆繪未完成：active=%d drew=%v missing=%d", len(active), drew, len(missing)))
		}
		outside, inside, added := storyPage3Diff(baseline, rgba, *storyPage3Scale)
		if outside != 0 || (len(active) != 0 && added == 0) {
			fail(fmt.Errorf("第 3 頁劇情覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*storyPage3Out, *storyPage3BaselineOut, *storyPage3PNGOut, *storyPage3BaselinePNGOut, rgba, baseline, *storyPage3Scale); err != nil {
			fail(err)
		}
		item := &storyOpeningOverlayJSON{Scale: *storyPage3Scale, ActiveKeys: active, Drew: drew, BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideStoryRect: outside, DiffInsideStoryRect: inside, AddedNonBaselinePixel: added}
		for _, r := range missing {
			item.MissingGlyphs = append(item.MissingGlyphs, string(r))
		}
		result.StoryPage3Overlay = item
	}
	if storyPage4Presenter != nil {
		storyPage4Presenter.Frame(m.Indexed(), m.Palette())
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *storyPage4Scale)
		rgba, missing, drew := storyPage4Presenter.Draw(m.Indexed(), m.Palette())
		active := storyPage4Presenter.ActiveKeys()
		if (len(active) == 0 && (drew || len(missing) != 0)) || (len(active) != 0 && (len(active) != 6 || !drew || len(missing) != 0)) {
			fail(fmt.Errorf("第 4 頁劇情覆繪未完成：active=%d drew=%v missing=%d", len(active), drew, len(missing)))
		}
		outside, inside, added := storyPage4Diff(baseline, rgba, *storyPage4Scale)
		if outside != 0 || (len(active) != 0 && added == 0) {
			fail(fmt.Errorf("第 4 頁劇情覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*storyPage4Out, *storyPage4BaselineOut, *storyPage4PNGOut, *storyPage4BaselinePNGOut, rgba, baseline, *storyPage4Scale); err != nil {
			fail(err)
		}
		item := &storyOpeningOverlayJSON{Scale: *storyPage4Scale, ActiveKeys: active, Drew: drew, BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideStoryRect: outside, DiffInsideStoryRect: inside, AddedNonBaselinePixel: added}
		for _, r := range missing {
			item.MissingGlyphs = append(item.MissingGlyphs, string(r))
		}
		result.StoryPage4Overlay = item
	}
	if storyPage5Presenter != nil {
		storyPage5Presenter.Frame(m.Indexed(), m.Palette())
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *storyPage5Scale)
		rgba, missing, drew := storyPage5Presenter.Draw(m.Indexed(), m.Palette())
		active := storyPage5Presenter.ActiveKeys()
		if (len(active) == 0 && (drew || len(missing) != 0)) || (len(active) != 0 && (len(active) != 5 || !drew || len(missing) != 0)) {
			fail(fmt.Errorf("第 5 頁劇情覆繪未完成：active=%d drew=%v missing=%d", len(active), drew, len(missing)))
		}
		outside, inside, added := storyPage5Diff(baseline, rgba, *storyPage5Scale)
		if outside != 0 || (len(active) != 0 && added == 0) {
			fail(fmt.Errorf("第 5 頁劇情覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*storyPage5Out, *storyPage5BaselineOut, *storyPage5PNGOut, *storyPage5BaselinePNGOut, rgba, baseline, *storyPage5Scale); err != nil {
			fail(err)
		}
		item := &storyOpeningOverlayJSON{Scale: *storyPage5Scale, ActiveKeys: active, Drew: drew, BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideStoryRect: outside, DiffInsideStoryRect: inside, AddedNonBaselinePixel: added}
		for _, r := range missing {
			item.MissingGlyphs = append(item.MissingGlyphs, string(r))
		}
		result.StoryPage5Overlay = item
	}
	if storyPage6Presenter != nil {
		storyPage6Presenter.Frame(m.Indexed(), m.Palette())
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *storyPage6Scale)
		rgba, missing, drew := storyPage6Presenter.Draw(m.Indexed(), m.Palette())
		active := storyPage6Presenter.ActiveKeys()
		if (len(active) == 0 && (drew || len(missing) != 0)) || (len(active) != 0 && (len(active) != 6 || !drew || len(missing) != 0)) {
			fail(fmt.Errorf("第 6 頁劇情覆繪未完成：active=%d drew=%v missing=%d", len(active), drew, len(missing)))
		}
		outside, inside, added := storyPage6Diff(baseline, rgba, *storyPage6Scale)
		if outside != 0 || (len(active) != 0 && added == 0) {
			fail(fmt.Errorf("第 6 頁劇情覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*storyPage6Out, *storyPage6BaselineOut, *storyPage6PNGOut, *storyPage6BaselinePNGOut, rgba, baseline, *storyPage6Scale); err != nil {
			fail(err)
		}
		item := &storyOpeningOverlayJSON{Scale: *storyPage6Scale, ActiveKeys: active, Drew: drew, BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideStoryRect: outside, DiffInsideStoryRect: inside, AddedNonBaselinePixel: added}
		for _, runeValue := range missing {
			item.MissingGlyphs = append(item.MissingGlyphs, string(runeValue))
		}
		result.StoryPage6Overlay = item
	}
	if storyPage7Presenter != nil {
		storyPage7Presenter.Frame(m.Indexed(), m.Palette())
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *storyPage7Scale)
		rgba, missing, drew := storyPage7Presenter.Draw(m.Indexed(), m.Palette())
		active := storyPage7Presenter.ActiveKeys()
		if (len(active) == 0 && (drew || len(missing) != 0)) || (len(active) != 0 && (len(active) != 6 || !drew || len(missing) != 0)) {
			fail(fmt.Errorf("第 7 頁劇情覆繪未完成：active=%d drew=%v missing=%d", len(active), drew, len(missing)))
		}
		outside, inside, added := storyPage7Diff(baseline, rgba, *storyPage7Scale)
		if outside != 0 || (len(active) != 0 && added == 0) {
			fail(fmt.Errorf("第 7 頁劇情覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*storyPage7Out, *storyPage7BaselineOut, *storyPage7PNGOut, *storyPage7BaselinePNGOut, rgba, baseline, *storyPage7Scale); err != nil {
			fail(err)
		}
		item := &storyOpeningOverlayJSON{Scale: *storyPage7Scale, ActiveKeys: active, Drew: drew, BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideStoryRect: outside, DiffInsideStoryRect: inside, AddedNonBaselinePixel: added}
		for _, runeValue := range missing {
			item.MissingGlyphs = append(item.MissingGlyphs, string(runeValue))
		}
		result.StoryPage7Overlay = item
	}
	if storyPage8Presenter != nil {
		storyPage8Presenter.Frame(m.Indexed(), m.Palette())
		baseline := buckrogers.ScaleIndexedRGBA(m.Indexed(), m.Palette(), *storyPage8Scale)
		rgba, missing, drew := storyPage8Presenter.Draw(m.Indexed(), m.Palette())
		active := storyPage8Presenter.ActiveKeys()
		if (len(active) == 0 && (drew || len(missing) != 0)) || (len(active) != 0 && (len(active) != 4 || !drew || len(missing) != 0)) {
			fail(fmt.Errorf("第 8 頁劇情覆繪未完成：active=%d drew=%v missing=%d", len(active), drew, len(missing)))
		}
		outside, inside, added := storyPage8Diff(baseline, rgba, *storyPage8Scale)
		if outside != 0 || (len(active) != 0 && added == 0) {
			fail(fmt.Errorf("第 8 頁劇情覆繪幾何或字模驗證失敗：outside=%d added=%d", outside, added))
		}
		if err := writeManualOutputs(*storyPage8Out, *storyPage8BaselineOut, *storyPage8PNGOut, *storyPage8BaselinePNGOut, rgba, baseline, *storyPage8Scale); err != nil {
			fail(err)
		}
		item := &storyOpeningOverlayJSON{Scale: *storyPage8Scale, ActiveKeys: active, Drew: drew, BaselineRGBA256: sha256hex(baseline), OverlayRGBA256: sha256hex(rgba), DiffOutsideStoryRect: outside, DiffInsideStoryRect: inside, AddedNonBaselinePixel: added}
		for _, runeValue := range missing {
			item.MissingGlyphs = append(item.MissingGlyphs, string(runeValue))
		}
		result.StoryPage8Overlay = item
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

// validateStoryOpeningOverlayFlags makes the first-screen runtime path
// all-or-nothing.  The catalog alone is not useful without the output-only
// presenter that makes its result observable.
func validateStoryOpeningOverlayFlags(events, translations, font, out, baseline, pngOut, baselinePNG string, scale int) error {
	any := events != "" || translations != "" || font != "" || out != "" || baseline != "" || pngOut != "" || baselinePNG != "" || scale != 0
	if !any {
		return nil
	}
	if events == "" || translations == "" || font == "" || out == "" || baseline == "" || pngOut == "" || baselinePNG == "" || (scale != 2 && scale != 3) {
		return fmt.Errorf("首屏劇情覆繪需要完整 events、translations、字型、2/3 倍 raw RGBA 與 PNG 輸出")
	}
	return nil
}

func storyOpeningEventsForGeneration(events []buckrogers.StoryOpeningEvent, generation uint64) []buckrogers.StoryOpeningEvent {
	filtered := make([]buckrogers.StoryOpeningEvent, 0, 5)
	for _, event := range events {
		if event.Generation == generation {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// isVerifiedStoryOpeningReturn encodes the READY return-edge adapter gate.
// Reaching 0763:04FF with the right stack is not enough: the immediately
// preceding instruction must be the observed 0763:03D6 RETF imm16 (0xCA).
func isVerifiedStoryOpeningReturn(previous buckrogers.Address, opcode uint8, at buckrogers.Address, ss, sp uint16, pending *glyphFrame) bool {
	return pending != nil && previous == (buckrogers.Address{Segment: 0x0763, Offset: 0x03D6}) && opcode == 0xCA &&
		at == pending.event.Caller && ss == pending.ss && sp == pending.sp+0x12
}

func validateStoryOpeningOverlayDraw(active []string, missing []rune, drew bool) error {
	if len(active) == 0 && !drew && len(missing) == 0 {
		return nil
	}
	if len(active) != 5 || !drew || len(missing) != 0 {
		return fmt.Errorf("首屏劇情覆繪未完成：active=%d drew=%v missing=%d", len(active), drew, len(missing))
	}
	return nil
}

// storyOpeningInvalidation only reports a lifecycle event after both layers
// agree: the watcher proved the READY video span and the presenter still held
// the complete atomic five-line group.  Any partial or stale layer is not
// silently cleared and never becomes receipt evidence.
func storyOpeningInvalidation(at buckrogers.Address, es, di, count uint16, step, generation uint64, active []string, invalidated bool) (storyOpeningInvalidationJSON, bool) {
	if !invalidated || at != (buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}) || es != 0xA000 || count == 0 || len(active) != 5 {
		return storyOpeningInvalidationJSON{}, false
	}
	return storyOpeningInvalidationJSON{Step: step, Instruction: at, VideoSegment: es, VideoOffset: di,
		ByteCount: count, Generation: generation, ActiveKeysBefore: len(active)}, true
}

// storyOpeningDiff permits only the READY first-screen text rectangle
// [8,320)x[136,176), at the explicitly requested output scale.
func storyOpeningDiff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	w := 320 * scale
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if string(baseline[i:i+4]) == string(overlay[i:i+4]) {
				continue
			}
			if x >= 8*scale && x < 320*scale && y >= 136*scale && y < 176*scale {
				inside++
				added++
			} else {
				outside++
			}
		}
	}
	return outside, inside, added
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

// storyFillIntersects checks an explicit five- or six-row diagnostic window.
// The 16-bit destination is interpreted as a bounded Mode 13h byte span.
func storyFillIntersects(di, count uint16, rows uint32) bool {
	if count == 0 {
		return false
	}
	if rows != 5 && rows != 6 {
		return false
	}
	start, end := uint32(di), uint32(di)+uint32(count)
	const width, top, left, right uint32 = 320, 136, 8, 320
	bottom := top + rows*8
	if end <= top*width || start >= bottom*width {
		return false
	}
	if start < top*width {
		start = top * width
	}
	if end > bottom*width {
		end = bottom * width
	}
	for row := start / width; row <= (end-1)/width; row++ {
		rowStart, rowEnd := row*width, (row+1)*width
		a, b := start, end
		if a < rowStart {
			a = rowStart
		}
		if b > rowEnd {
			b = rowEnd
		}
		if a-rowStart < right && b-rowStart > left {
			return true
		}
	}
	return false
}

func loadBodyIconRects(path string) ([]bodyIconRect, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("讀取 body icon rectangles：%w", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) != 8 || lines[0] != "screen\tevent_key\tx\ty\twidth\theight\tdraw_x\tdraw_y\tcapacity_cells\tline_count\toverflow_policy" {
		return nil, fmt.Errorf("body icon rectangles schema 或筆數不符")
	}
	expected := map[string]bodyIconRect{
		"body.icon.confirmation":          {EventKey: "body.icon.confirmation", X: 0, Y: 192, Width: 136, Height: 8},
		"body.icon.save_prompt":           {EventKey: "body.icon.save_prompt", X: 0, Y: 192, Width: 64, Height: 8},
		"body.icon.old.label":             {EventKey: "body.icon.old.label", X: 64, Y: 48, Width: 24, Height: 8},
		"body.icon.old.action":            {EventKey: "body.icon.old.action", X: 24, Y: 80, Width: 112, Height: 8},
		"body.icon.new.label":             {EventKey: "body.icon.new.label", X: 64, Y: 96, Width: 24, Height: 8},
		"body.icon.new.action":            {EventKey: "body.icon.new.action", X: 24, Y: 128, Width: 112, Height: 8},
		"body.icon.selection.instruction": {EventKey: "body.icon.selection.instruction", X: 0, Y: 192, Width: 160, Height: 8},
	}
	seen := make(map[string]bool, len(expected))
	out := make([]bodyIconRect, 0, len(expected))
	for _, line := range lines[1:] {
		fields := strings.Split(strings.TrimSuffix(line, "\r"), "\t")
		if len(fields) != 11 {
			return nil, fmt.Errorf("body icon rectangle 欄數不符")
		}
		key := fields[1]
		want, ok := expected[key]
		if !ok || seen[key] {
			return nil, fmt.Errorf("未知或重複 body icon rectangle：%q", key)
		}
		values := make([]int, 8)
		for i := range values {
			values[i], err = strconv.Atoi(fields[i+2])
			if err != nil {
				return nil, fmt.Errorf("body icon rectangle 數值無效：%q", key)
			}
		}
		if values[0] != want.X || values[1] != want.Y || values[2] != want.Width || values[3] != want.Height || values[4] != want.X || values[5] != want.Y || values[6] != want.Width/8 || values[7] != 1 || fields[10] != "single-line-reject" {
			return nil, fmt.Errorf("body icon rectangle 身分或幾何漂移：%q", key)
		}
		seen[key] = true
		out = append(out, want)
	}
	if len(seen) != len(expected) {
		return nil, fmt.Errorf("body icon rectangles 不完整")
	}
	return out, nil
}

func validateBodyIconTraceFlags(enabled bool, rects string, from, until uint64) error {
	any := enabled || rects != "" || from != 0
	if !any {
		return nil
	}
	if !enabled || rects == "" || from == 0 || from >= until {
		return fmt.Errorf("body-icon framebuffer trace、rects 與有效 trace-from 必須同時提供")
	}
	return nil
}

func observeBodyIconFramebuffer(step uint64, at buckrogers.Address, rects []bodyIconRect, before, after []byte) (bodyIconFramebufferWriteJSON, bool) {
	if len(before) != 320*200 || len(after) != len(before) {
		return bodyIconFramebufferWriteJSON{}, false
	}
	keys := make([]string, 0, len(rects))
	x0, y0, x1, y1 := 320, 200, -1, -1
	for _, rect := range rects {
		hit := false
		for y := rect.Y; y < rect.Y+rect.Height; y++ {
			for x := rect.X; x < rect.X+rect.Width; x++ {
				i := y*320 + x
				if before[i] == after[i] {
					continue
				}
				hit = true
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
		}
		if hit {
			keys = append(keys, rect.EventKey)
		}
	}
	if x1 < 0 {
		return bodyIconFramebufferWriteJSON{}, false
	}
	for _, rect := range rects {
		for y := rect.Y; y < rect.Y+rect.Height; y++ {
			copy(before[y*320+rect.X:y*320+rect.X+rect.Width], after[y*320+rect.X:y*320+rect.X+rect.Width])
		}
	}
	sort.Strings(keys)
	return bodyIconFramebufferWriteJSON{Step: step, Instruction: at, EventKeys: keys, X0: uint16(x0), Y0: uint16(y0), X1: uint16(x1), Y1: uint16(y1)}, true
}

// storyPage2Diff permits only the READY page-two rectangle [8,320)x[136,168).
func storyPage2Diff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	w := 320 * scale
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if string(baseline[i:i+4]) == string(overlay[i:i+4]) {
				continue
			}
			if x >= 8*scale && x < 320*scale && y >= 136*scale && y < 168*scale {
				inside++
				added++
			} else {
				outside++
			}
		}
	}
	return outside, inside, added
}

// storyPage3Diff permits only the READY third-page rectangle
// [8,320)x[136,176), at the explicitly requested output scale.
func storyPage3Diff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	w := 320 * scale
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if string(baseline[i:i+4]) == string(overlay[i:i+4]) {
				continue
			}
			if x >= 8*scale && x < 320*scale && y >= 136*scale && y < 176*scale {
				inside++
				added++
			} else {
				outside++
			}
		}
	}
	return outside, inside, added
}

// storyPage4Diff permits only the READY six-line rectangle
// [8,320)x[136,184), at the explicitly requested output scale.
func storyPage4Diff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	w := 320 * scale
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if string(baseline[i:i+4]) == string(overlay[i:i+4]) {
				continue
			}
			if x >= 8*scale && x < 320*scale && y >= 136*scale && y < 184*scale {
				inside++
				added++
			} else {
				outside++
			}
		}
	}
	return outside, inside, added
}

// storyPage5Diff permits only the READY five-line rectangle [8,320)x[136,176).
func storyPage5Diff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	return storyPage3Diff(baseline, overlay, scale)
}

// storyPage6Diff permits only the READY six-line rectangle [8,320)x[136,184).
func storyPage6Diff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	return storyPage4Diff(baseline, overlay, scale)
}

// storyPage7Diff permits only the READY six-line rectangle [8,320)x[136,184).
func storyPage7Diff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	return storyPage4Diff(baseline, overlay, scale)
}

// storyPage8Diff permits only the READY four-line full-English rectangle
// [8,312)x[136,168), at the explicitly requested output scale.
func storyPage8Diff(baseline, overlay []byte, scale int) (outside, inside, added int) {
	w := 320 * scale
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if string(baseline[i:i+4]) == string(overlay[i:i+4]) {
				continue
			}
			if x >= 8*scale && x < 312*scale && y >= 136*scale && y < 168*scale {
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

func glyphWordHighMask(args [7]uint16) (mask uint8) {
	for i, word := range args {
		if word > 0xff {
			mask |= 1 << uint(i)
		}
	}
	return mask
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
