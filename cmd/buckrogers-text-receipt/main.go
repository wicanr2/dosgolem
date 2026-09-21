// Command buckrogers-text-receipt replays a diagnostic state and emits only
// content-free metadata for completed 0763:0424 calls.
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
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

func main() {
	statePath := flag.String("state", "", "既有 probe state")
	until := flag.Uint64("until", 0, "絕對指令步數上限")
	want := flag.Int("want", 0, "預期完成事件數；0 表示不檢查")
	wantRequests := flag.Int("want-requests", 0, "預期顯示請求數；0 表示不檢查")
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
	careerSkillRects := flag.String("career-skill-rects", "", "正式 career-skill-screen-text-safe-rects.tsv")
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
	screenOut := flag.String("screen-out", "", "成功後寫出終態 320×200 indexed framebuffer")
	receiptOut := flag.String("receipt-out", "", "成功後另寫出與 stdout 相同的 JSON 收據")
	stateOut := flag.String("state-out", "", "成功後保存終態 savestate（只供本機研究）")
	scratch := flag.String("scratch", "", "可選、已存在的 DOS 可寫暫存目錄")
	fileOps := flag.Bool("file-ops", false, "在收據加入 content-safe 檔案操作 metadata")
	unimplemented := flag.Bool("unimplemented", false, "在收據加入未實作 DOS／BIOS 服務統計")
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
	if err := validateOverlayFlags(*menuEvents, *menuRects, *genderEvents, *genderRects,
		*classEvents, *classRects, *rosterEvents, *rosterRects, *characterSheetEvents, *characterSheetRects,
		*namePromptEvents, *namePromptRects,
		*careerSkillEvents, *careerSkillRects,
		*overlayFont, *overlayOut, *overlayScale); err != nil {
		fail(err)
	}
	if *baselineOut != "" && *overlayOut == "" {
		fail(fmt.Errorf("baseline-rgba-out 只能與 overlay-rgba-out 同時提供"))
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
	m := machine.New()
	d := dos.New(m, ".")
	d.Install()
	if err := state.Load(*statePath, m, d); err != nil {
		fail(err)
	}
	if err := configureScratch(d, *scratch); err != nil {
		fail(err)
	}
	var presenter *buckrogers.RuntimeMenuOverlay
	if *overlayOut != "" {
		inputs := []struct{ name, path string }{
			{"menu-text-safe-rects.tsv", *menuRects},
			{"gender-text-safe-rects.tsv", *genderRects},
			{"class-text-safe-rects.tsv", *classRects},
			{"save-roster-join-text-safe-rects.tsv", *rosterRects},
			{"name-prompt-text-safe-rects.tsv", *namePromptRects},
			{"career-skill-screen-text-safe-rects.tsv", *careerSkillRects},
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
		m.SetOnFrame(func() { presenter.Frame(m.Indexed(), m.Palette()) })
	}
	start := m.Steps
	r := buckrogers.NewMenuRequestWatcher(catalog)
	nextKey := 0
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
		if presenter != nil && at == (buckrogers.Address{Segment: 0x026F, Offset: 0x029C}) {
			bottom := m.Read8(cpu.Addr(ss, sp+4))
			right := m.Read8(cpu.Addr(ss, sp+6))
			top := m.Read8(cpu.Addr(ss, sp+8))
			left := m.Read8(cpu.Addr(ss, sp+10))
			if err := presenter.ClearTextCells(bottom, right, top, left); err != nil {
				fail(err)
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
		} else {
			eventBefore, requestBefore := r.EventCount(), r.RequestCount()
			r.ObserveInstruction(at, ss, sp, m.Steps)
			if presenter != nil && r.EventCount() > eventBefore && r.RequestCount() > requestBefore {
				event, eventOK := r.LastEvent()
				request, requestOK := r.LastRequest()
				if !eventOK || !requestOK || presenter.Apply(event, request, m.Palette()) != nil {
					fail(fmt.Errorf("runtime overlay apply 失敗"))
				}
			}
		}
		if err := m.Step(); err != nil {
			fail(err)
		}
	}
	events := r.Events()
	requests := r.Requests()
	if nextKey != len(keys) || r.Pending() || r.Drops() != 0 || (*want != 0 && len(events) != *want) ||
		(*wantRequests != 0 && len(requests) != *wantRequests) {
		fail(fmt.Errorf("收據失敗：keys=%d/%d events=%d want=%d requests=%d want_requests=%d pending=%v drops=%d misses=%d",
			nextKey, len(keys), len(events), *want, len(requests), *wantRequests, r.Pending(), r.Drops(), r.Misses()))
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
	result := struct {
		StateStart        uint64        `json:"state_start"`
		StoppedAt         uint64        `json:"stopped_at"`
		BIOSInput         string        `json:"bios_input,omitempty"`
		Events            []eventJSON   `json:"events"`
		Requests          []requestJSON `json:"requests,omitempty"`
		CatalogMisses     *int          `json:"catalog_misses,omitempty"`
		BIOSKeys          []keyJSON     `json:"bios_keys,omitempty"`
		Scratch           string        `json:"scratch,omitempty"`
		Writes            []writeJSON   `json:"writes,omitempty"`
		FileOps           []fileOpJSON  `json:"file_ops,omitempty"`
		Unimplemented     []string      `json:"unimplemented,omitempty"`
		OverlayScale      int           `json:"overlay_scale,omitempty"`
		OverlayActions    []requestJSON `json:"overlay_actions,omitempty"`
		ActiveOverlayKeys []string      `json:"active_overlay_keys,omitempty"`
		OverlayMissing    []string      `json:"overlay_missing_glyphs,omitempty"`
		OverlayDrew       *bool         `json:"overlay_drew,omitempty"`
	}{StateStart: start, StoppedAt: m.Steps, Events: out, Scratch: *scratch}
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
	if catalog != nil {
		result.Requests = requestOut
		misses := r.Misses()
		result.CatalogMisses = &misses
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
	font, out string, scale int) error {
	rects := []string{menuRects, genderRects, classRects, rosterRects, characterSheetRects, namePromptRects, careerSkillRects}
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
	} {
		if (pair[1] == "") != (pair[2] == "") {
			return fmt.Errorf("overlay %s catalog 與 rect 必須同時提供", pair[0])
		}
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
