package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
	"github.com/wicanr2/dosgolem/xlate"
)

func TestLoadBIOSKeysReceiptAcceptsOnlyBoundedSchedule(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(valid, []byte(`{"bios_keys":[{"queued_at":10,"scan":28,"ascii":13},{"queued_at":20,"scan":0,"ascii":65}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	keys, err := loadBIOSKeysReceipt(valid)
	if err != nil || len(keys) != 2 || keys[0] != (scheduledBIOSKey{Step: 10, Scan: 28, ASCII: 13}) {
		t.Fatalf("keys=%#v err=%v", keys, err)
	}
	for name, data := range map[string][]byte{
		"malformed": []byte(`{`),
		"missing":   []byte(`{"bios_keys":[]}`),
		"too_many":  []byte(`{"bios_keys":[{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}]}`),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".json")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadBIOSKeysReceipt(path); err == nil {
				t.Fatal("無效 receipt 必須失敗即關閉")
			}
		})
	}
}

func TestPostJoinRuntimeGateSymmetry(t *testing.T) {
	for _, row := range []uint8{12, 17} {
		if postJoinRuntimeGate(buckrogers.TextEvent{Caller: buckrogers.Address{Segment: 0x37f1, Offset: 0x15bd}, Row: row}) {
			t.Fatalf("row %d must not create unmatched return", row)
		}
	}
	if !postJoinRuntimeGate(buckrogers.TextEvent{Caller: buckrogers.Address{Segment: 0x37f1, Offset: 0x175d}, Row: 21}) {
		t.Fatal("row21 selected must reach fail-closed watcher")
	}
}

type postJoinReceiptWatcher struct {
	active bool
	retain bool
	calls  int
}

func (w *postJoinReceiptWatcher) Active() bool { return w.active }
func (w *postJoinReceiptWatcher) Prewrite(machine.VideoWrite) {
	w.calls++
	if !w.retain {
		w.active = false
	}
}

type postJoinReceiptPresenter struct {
	keys   int
	retain bool
	calls  int
}

func (p *postJoinReceiptPresenter) ActiveKeys() []string {
	return make([]string, p.keys)
}
func (p *postJoinReceiptPresenter) Prewrite(machine.VideoWrite) {
	p.calls++
	if !p.retain {
		p.keys = 0
	}
}

func TestPostJoinInvalidationReceiptIsContentSafe(t *testing.T) {
	write := machine.VideoWrite{Step: 124700875, CS: 0x0763, IP: 0x184d, Offset: 20*8*320 + 72, Value: 0x7f, WriteMode: 3}
	watcher := &postJoinReceiptWatcher{active: true}
	presenter := &postJoinReceiptPresenter{keys: 7}
	var records []postJoinInvalidationJSON
	observePostJoinPrewrite(watcher, presenter, write, &records)
	if len(records) != 1 {
		t.Fatalf("records=%d, want 1", len(records))
	}
	b, err := json.Marshal(records[0])
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["watcher_active_before"] != true || got["watcher_active_after"] != false || got["presenter_keys_before"] != float64(7) || got["presenter_keys_after"] != float64(0) {
		t.Fatalf("unexpected lifecycle receipt: %s", b)
	}
	if _, leaked := got["value"]; leaked {
		t.Fatalf("receipt leaked VideoWrite.Value: %s", b)
	}
	if got["step"] != float64(write.Step) || got["offset"] != float64(write.Offset) || got["write_mode"] != float64(write.WriteMode) {
		t.Fatalf("receipt lost prewrite metadata: %s", b)
	}
	// An already inactive layer must not create a synthetic lifecycle event.
	observePostJoinPrewrite(watcher, presenter, write, &records)
	if len(records) != 1 {
		t.Fatalf("inactive prewrite added a record: %#v", records)
	}
	if watcher.calls != 1 || presenter.calls != 1 {
		t.Fatalf("inactive prewrite called components: watcher=%d presenter=%d", watcher.calls, presenter.calls)
	}
	stillActive := &postJoinReceiptWatcher{active: true, retain: true}
	stillDrawn := &postJoinReceiptPresenter{keys: 7}
	observePostJoinPrewrite(stillActive, stillDrawn, write, &records)
	if stillActive.calls != 1 || stillDrawn.calls != 1 || len(records) != 1 {
		t.Fatalf("active watcher changed Prewrite call sequence or wrote a false record: watcher=%d presenter=%d records=%d", stillActive.calls, stillDrawn.calls, len(records))
	}
	stalePresenter := &postJoinReceiptPresenter{keys: 7, retain: true}
	observePostJoinPrewrite(&postJoinReceiptWatcher{active: true}, stalePresenter, write, &records)
	if len(records) != 2 || records[1].PresenterKeysBefore != 7 || records[1].PresenterKeysAfter != 7 {
		t.Fatalf("watcher invalidation hid presenter residue: %#v", records)
	}
}

func TestStoryFillIntersectsFiveRowSafeRect(t *testing.T) {
	tests := []struct {
		name  string
		di    uint16
		count uint16
		want  bool
	}{
		{"empty", 136*320 + 8, 0, false},
		{"before top", 136*320 - 304, 304, false},
		{"left margin only", 136 * 320, 8, false},
		{"first safe pixel", 136*320 + 8, 1, true},
		{"measured first fill", 0xAA08, 304, true},
		{"last row", 175*320 + 8, 1, true},
		{"after bottom", 176*320 + 8, 1, false},
		{"crosses row edge", 136*320 + 319, 2, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := storyFillIntersects(tc.di, tc.count, 5); got != tc.want {
				t.Fatalf("intersects(%#x,%d)=%v, want %v", tc.di, tc.count, got, tc.want)
			}
		})
	}
}

func TestStoryFillIntersectsSixRowDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		name      string
		di, count uint16
		want      bool
	}{
		{"row22 only", 176*320 + 8, 1, true},
		{"row23 only", 184*320 + 8, 1, false},
		{"invalid rows fail closed", 176*320 + 8, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := uint32(6)
			if tc.name == "invalid rows fail closed" {
				rows = 7
			}
			if got := storyFillIntersects(tc.di, tc.count, rows); got != tc.want {
				t.Fatalf("intersects(%#x,%d,%d)=%v, want %v", tc.di, tc.count, rows, got, tc.want)
			}
		})
	}
}

func TestLoadBodyIconRectsFailsClosedAndAcceptsExactCatalog(t *testing.T) {
	valid := "screen\tevent_key\tx\ty\twidth\theight\tdraw_x\tdraw_y\tcapacity_cells\tline_count\toverflow_policy\n" +
		"confirmation\tbody.icon.confirmation\t0\t192\t136\t8\t0\t192\t17\t1\tsingle-line-reject\n" +
		"save_prompt\tbody.icon.save_prompt.prefix\t0\t192\t40\t8\t0\t192\t5\t1\tsingle-line-reject\n" +
		"body_icon\tbody.icon.old.label\t64\t48\t24\t8\t64\t48\t3\t1\tsingle-line-reject\n" +
		"body_icon\tbody.icon.old.action\t24\t80\t112\t8\t24\t80\t14\t1\tsingle-line-reject\n" +
		"body_icon\tbody.icon.new.label\t64\t96\t24\t8\t64\t96\t3\t1\tsingle-line-reject\n" +
		"body_icon\tbody.icon.new.action\t24\t128\t112\t8\t24\t128\t14\t1\tsingle-line-reject\n" +
		"selection\tbody.icon.selection.instruction\t0\t192\t160\t8\t0\t192\t20\t1\tsingle-line-reject\n"
	for name, mutate := range map[string]func(string) string{
		"exact":          func(s string) string { return s },
		"unknown":        func(s string) string { return strings.Replace(s, "body.icon.old.label", "body.icon.unknown", 1) },
		"boundary drift": func(s string) string { return strings.Replace(s, "64\t48\t24\t8\t64\t48", "63\t48\t24\t8\t63\t48", 1) },
		"missing": func(s string) string {
			return strings.Replace(s, "selection\tbody.icon.selection.instruction\t0\t192\t160\t8\t0\t192\t20\t1\tsingle-line-reject\n", "", 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rects.tsv")
			if err := os.WriteFile(path, []byte(mutate(valid)), 0o600); err != nil {
				t.Fatal(err)
			}
			rects, err := loadBodyIconRects(path)
			if name == "exact" {
				if err != nil || len(rects) != 7 {
					t.Fatalf("rects=%v err=%v", rects, err)
				}
			} else if err == nil {
				t.Fatal("漂移矩形必須失敗即關閉")
			}
		})
	}
}

func TestBodyIconTraceFlagsAreAtomicAndBounded(t *testing.T) {
	for _, tc := range []struct {
		enabled     bool
		rects       string
		from, until uint64
		valid       bool
	}{
		{false, "", 0, 10, true},
		{true, "rects.tsv", 5, 10, true},
		{true, "", 5, 10, false},
		{false, "rects.tsv", 5, 10, false},
		{true, "rects.tsv", 0, 10, false},
		{true, "rects.tsv", 10, 10, false},
	} {
		if got := validateBodyIconTraceFlags(tc.enabled, tc.rects, tc.from, tc.until) == nil; got != tc.valid {
			t.Fatalf("enabled=%v rects=%q from=%d until=%d valid=%v, want %v", tc.enabled, tc.rects, tc.from, tc.until, got, tc.valid)
		}
	}
}

func TestBodyIconA000TraceFlagsAreBoundedWithoutFramebufferCap(t *testing.T) {
	for _, tc := range []struct {
		enabled     bool
		rects       string
		from, until uint64
		valid       bool
	}{{false, "", 0, 10, true}, {true, "rects.tsv", 5, 10, true}, {true, "", 5, 10, false}, {true, "rects.tsv", 0, 10, false}, {true, "rects.tsv", 10, 10, false}} {
		if got := validateBodyIconA000TraceFlags(tc.enabled, tc.rects, tc.from, tc.until) == nil; got != tc.valid {
			t.Fatalf("enabled=%v rects=%q from=%d until=%d got valid=%v, want %v", tc.enabled, tc.rects, tc.from, tc.until, got, tc.valid)
		}
	}
}

func TestObserveBodyIconFramebufferBoundariesAndFirstIntersection(t *testing.T) {
	rects := []bodyIconRect{{EventKey: "body.icon.old.label", X: 64, Y: 48, Width: 24, Height: 8}}
	before, after := make([]byte, 320*200), make([]byte, 320*200)
	after[48*320+63] = 1
	if _, ok := observeBodyIconFramebuffer(10, buckrogers.Address{}, rects, before, after); ok {
		t.Fatal("左 exclusive 邊界外不得命中")
	}
	after[48*320+64] = 2
	item, ok := observeBodyIconFramebuffer(11, buckrogers.Address{Segment: 1, Offset: 2}, rects, before, after)
	if !ok || item.Step != 11 || item.X0 != 64 || item.Y0 != 48 || len(item.EventKeys) != 1 {
		t.Fatalf("item=%+v ok=%v", item, ok)
	}
	if _, ok := observeBodyIconFramebuffer(12, buckrogers.Address{}, rects, before, after); ok {
		t.Fatal("未再變化不得重複命中")
	}
	after[55*320+87] = 3
	if _, ok := observeBodyIconFramebuffer(13, buckrogers.Address{}, rects, before, after); !ok {
		t.Fatal("右下內界應命中")
	}
	after[56*320+64] = 4
	if _, ok := observeBodyIconFramebuffer(14, buckrogers.Address{}, rects, before, after); ok {
		t.Fatal("下 exclusive 邊界外不得命中")
	}
}

func TestBodyIconSpanIntersectionsBoundariesUnknownAndNonIntersecting(t *testing.T) {
	rects := []bodyIconRect{{EventKey: "known", X: 64, Y: 48, Width: 24, Height: 8}}
	for _, tc := range []struct {
		name          string
		offset, count uint16
		want          bool
	}{
		{"empty", 48*320 + 64, 0, false},
		{"left exclusive", 48*320 + 63, 1, false},
		{"first", 48*320 + 64, 1, true},
		{"last", 55*320 + 87, 1, true},
		{"right exclusive", 55*320 + 88, 1, false},
		{"bottom exclusive", 56*320 + 64, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := bodyIconSpanIntersections(rects, tc.offset, tc.count)
			if (len(got) != 0) != tc.want {
				t.Fatalf("keys=%v want hit=%v", got, tc.want)
			}
		})
	}
	if got := bodyIconSpanIntersections(nil, 48*320+64, 1); len(got) != 0 {
		t.Fatalf("未知矩形不得命中：%v", got)
	}
}

func TestMenuCatalogFlagsArePaired(t *testing.T) {
	for _, tc := range []struct {
		events, translations string
		valid                bool
	}{
		{"", "", true},
		{"events.tsv", "translations.tsv", true},
		{"events.tsv", "", false},
		{"", "translations.tsv", false},
	} {
		if got := validateMenuCatalogFlags(tc.events, tc.translations) == nil; got != tc.valid {
			t.Fatalf("events=%q translations=%q valid=%v，要 %v", tc.events, tc.translations, got, tc.valid)
		}
	}
}

func TestAllCatalogFlagsArePaired(t *testing.T) {
	tests := []struct {
		menuEvents, menuTranslations, genderEvents, genderTranslations string
		classEvents, classTranslations                                 string
		rosterEvents, rosterTranslations                               string
		characterSheetEvents, characterSheetTranslations               string
		valid                                                          bool
	}{
		{"", "", "", "", "", "", "", "", "", "", true},
		{"m.tsv", "mt.tsv", "", "", "", "", "", "", "", "", true},
		{"", "", "g.tsv", "gt.tsv", "", "", "", "", "", "", true},
		{"m.tsv", "mt.tsv", "g.tsv", "gt.tsv", "c.tsv", "ct.tsv", "r.tsv", "rt.tsv", "s.tsv", "st.tsv", true},
		{"", "", "", "", "c.tsv", "ct.tsv", "", "", "", "", true},
		{"", "", "", "", "", "", "r.tsv", "rt.tsv", "", "", true},
		{"", "", "", "", "", "", "", "", "s.tsv", "st.tsv", true},
		{"m.tsv", "", "", "", "", "", "", "", "", "", false},
		{"", "", "g.tsv", "", "", "", "", "", "", "", false},
		{"", "", "", "gt.tsv", "", "", "", "", "", "", false},
		{"", "", "", "", "c.tsv", "", "", "", "", "", false},
		{"", "", "", "", "", "ct.tsv", "", "", "", "", false},
		{"", "", "", "", "", "", "r.tsv", "", "", "", false},
		{"", "", "", "", "", "", "", "rt.tsv", "", "", false},
		{"", "", "", "", "", "", "", "", "s.tsv", "", false},
		{"", "", "", "", "", "", "", "", "", "st.tsv", false},
	}
	for _, tc := range tests {
		got := validateCatalogFlags(tc.menuEvents, tc.menuTranslations, tc.genderEvents, tc.genderTranslations,
			tc.classEvents, tc.classTranslations, tc.rosterEvents, tc.rosterTranslations,
			tc.characterSheetEvents, tc.characterSheetTranslations) == nil
		if got != tc.valid {
			t.Fatalf("flags %#v valid=%v，要 %v", tc, got, tc.valid)
		}
	}
}

func TestNamePromptCatalogFlagsArePaired(t *testing.T) {
	for _, tc := range []struct {
		name, events, translations string
		wantOK                     bool
	}{
		{"皆省略", "", "", true},
		{"皆提供", "events", "translations", true},
		{"缺譯文", "events", "", false},
		{"缺事件", "", "translations", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateNamePromptCatalogFlags(tc.events, tc.translations) == nil; got != tc.wantOK {
				t.Fatalf("有效性 = %v，要 %v", got, tc.wantOK)
			}
		})
	}
}

func TestCareerSkillCatalogFlagsArePaired(t *testing.T) {
	for _, tc := range []struct {
		events, translations string
		wantOK               bool
	}{
		{"", "", true}, {"events", "translations", true},
		{"events", "", false}, {"", "translations", false},
	} {
		if got := validateCareerSkillCatalogFlags(tc.events, tc.translations) == nil; got != tc.wantOK {
			t.Fatalf("events=%q translations=%q valid=%v，要 %v", tc.events, tc.translations, got, tc.wantOK)
		}
	}
}

func TestTechnicalSkillCatalogFlagsArePaired(t *testing.T) {
	for _, tc := range []struct {
		events, translations, careerEvents string
		wantOK                             bool
	}{
		{"", "", "", true}, {"events", "translations", "career-events", true},
		{"events", "translations", "", false},
		{"events", "", "career-events", false}, {"", "translations", "career-events", false},
	} {
		if got := validateTechnicalSkillCatalogFlags(tc.events, tc.translations, tc.careerEvents) == nil; got != tc.wantOK {
			t.Fatalf("events=%q translations=%q valid=%v，要 %v", tc.events, tc.translations, got, tc.wantOK)
		}
	}
}

func TestActionBarFlagsRequireBothSkillAnchors(t *testing.T) {
	for _, tc := range []struct {
		action, translations, career, technical string
		wantOK                                  bool
	}{
		{"", "", "", "", true},
		{"", "", "career", "", true},
		{"actions", "", "career", "technical", true},
		{"actions", "translations", "career", "technical", true},
		{"", "translations", "career", "technical", false},
		{"actions", "", "", "technical", false},
		{"actions", "", "career", "", false},
	} {
		if got := validateActionBarFlags(tc.action, tc.translations, tc.career, tc.technical) == nil; got != tc.wantOK {
			t.Fatalf("flags=%#v valid=%v, want %v", tc, got, tc.wantOK)
		}
	}
}

func TestActionWatcherStatusOnlyParticipatesWhenConfigured(t *testing.T) {
	if pending, drops, misses, requestMisses := actionWatcherStatus(nil); pending || drops != 0 || misses != 0 || requestMisses != 0 {
		t.Fatalf("未啟用 action watcher 的狀態 = pending=%v drops=%d misses=%d requestMisses=%d", pending, drops, misses, requestMisses)
	}
	// A non-nil watcher remains part of the terminal contract; this command
	// must not replace the fail-closed watcher with an implicit no-op when the
	// formal action-bar catalog is enabled.
	w := buckrogers.NewActionBarWatcher(nil)
	if pending, drops, misses, requestMisses := actionWatcherStatus(w); pending || drops != 0 || misses != 0 || requestMisses != 0 {
		t.Fatalf("新建 action watcher 的初始狀態 = pending=%v drops=%d misses=%d requestMisses=%d", pending, drops, misses, requestMisses)
	}
}

func TestActionWatcherModePrefersRequestCatalog(t *testing.T) {
	events := &buckrogers.ActionBarCatalog{}
	requests := &buckrogers.ActionBarRequestCatalog{}
	for _, tc := range []struct {
		name    string
		e       *buckrogers.ActionBarCatalog
		request *buckrogers.ActionBarRequestCatalog
		want    actionWatcherMode
	}{
		{"皆省略", nil, nil, actionWatcherDisabled},
		{"僅事件 catalog", events, nil, actionWatcherEvent},
		{"僅請求 catalog", nil, requests, actionWatcherRequest},
		{"兩者同設，請求優先", events, requests, actionWatcherRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := actionWatcherModeForCatalogs(tc.e, tc.request); got != tc.want {
				t.Fatalf("mode=%d，要 %d", got, tc.want)
			}
		})
	}
}

func TestActionBarOverlayFlagsRequireCompletePairedArtifacts(t *testing.T) {
	valid := []struct {
		name                               string
		events, translations, rects, font  string
		out, baseline, pngOut, baselinePNG string
		scale                              int
		wantOK                             bool
	}{
		{"全部省略", "", "", "", "", "", "", "", "", 0, true},
		{"僅 watcher catalog", "e", "t", "", "", "", "", "", "", 0, true},
		{"完整 2 倍", "e", "t", "r", "f", "o", "b", "p", "bp", 2, true},
		{"完整 3 倍", "e", "t", "r", "f", "o", "b", "p", "bp", 3, true},
		{"缺矩形", "e", "t", "", "f", "o", "b", "p", "bp", 2, false},
		{"缺字型", "e", "t", "r", "", "o", "b", "p", "bp", 2, false},
		{"缺 baseline", "e", "t", "r", "f", "o", "", "p", "bp", 2, false},
		{"缺 PNG", "e", "t", "r", "f", "o", "b", "", "bp", 2, false},
		{"錯誤倍率", "e", "t", "r", "f", "o", "b", "p", "bp", 1, false},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got := validateActionBarOverlayFlags(tc.events, tc.translations, tc.rects, tc.font, tc.out, tc.baseline, tc.pngOut, tc.baselinePNG, tc.scale) == nil
			if got != tc.wantOK {
				t.Fatalf("有效性=%v，要 %v", got, tc.wantOK)
			}
		})
	}
}

func TestStoryOpeningOverlayFlagsRequireCompletePairedArtifacts(t *testing.T) {
	complete := []string{"events", "translations", "font", "out", "baseline", "png", "baseline-png"}
	if err := validateStoryOpeningOverlayFlags("", "", "", "", "", "", "", 0); err != nil {
		t.Fatal("全部省略應可關閉首屏覆繪")
	}
	for _, scale := range []int{2, 3} {
		if err := validateStoryOpeningOverlayFlags(complete[0], complete[1], complete[2], complete[3], complete[4], complete[5], complete[6], scale); err != nil {
			t.Fatalf("完整 %dx 旗標被拒絕: %v", scale, err)
		}
		for omitted := range complete {
			missing := append([]string(nil), complete...)
			missing[omitted] = ""
			if err := validateStoryOpeningOverlayFlags(missing[0], missing[1], missing[2], missing[3], missing[4], missing[5], missing[6], scale); err == nil {
				t.Fatalf("%dx 缺第 %d 個旗標被接受", scale, omitted)
			}
		}
	}
	for _, scale := range []int{-1, 0, 1, 4} {
		if err := validateStoryOpeningOverlayFlags(complete[0], complete[1], complete[2], complete[3], complete[4], complete[5], complete[6], scale); err == nil {
			t.Fatalf("無效倍率 %d 被接受", scale)
		}
	}
	if err := validateStoryOpeningOverlayFlags("", "", "", "", "", "", "", 2); err == nil {
		t.Fatal("僅提供倍率被接受")
	}
}

func TestStoryOpeningReturnRequiresObservedRETFEdge(t *testing.T) {
	pending := &glyphFrame{event: glyphJSON{EntryStep: 10, Caller: buckrogers.Address{Segment: 0x0763, Offset: 0x04FF}}, ss: 0x1841, sp: 0x3900}
	for _, tc := range []struct {
		name         string
		previous, at buckrogers.Address
		opcode       uint8
		ss, sp       uint16
		pending      *glyphFrame
		want         bool
	}{
		{"已確認 RETF 邊", buckrogers.Address{Segment: 0x0763, Offset: 0x03D6}, pending.event.Caller, 0xCA, 0x1841, 0x3912, pending, true},
		{"同位址但非 RETF", buckrogers.Address{Segment: 0x0763, Offset: 0x03D6}, pending.event.Caller, 0xCB, 0x1841, 0x3912, pending, false},
		{"錯誤前一指令", buckrogers.Address{Segment: 0x0763, Offset: 0x03D7}, pending.event.Caller, 0xCA, 0x1841, 0x3912, pending, false},
		{"錯誤 return caller", buckrogers.Address{Segment: 0x0763, Offset: 0x03D6}, buckrogers.Address{Segment: 0x0763, Offset: 0x0500}, 0xCA, 0x1841, 0x3912, pending, false},
		{"錯誤 SS", buckrogers.Address{Segment: 0x0763, Offset: 0x03D6}, pending.event.Caller, 0xCA, 0x1842, 0x3912, pending, false},
		{"錯誤 stack", buckrogers.Address{Segment: 0x0763, Offset: 0x03D6}, pending.event.Caller, 0xCA, 0x1841, 0x3911, pending, false},
		{"沒有 pending", buckrogers.Address{Segment: 0x0763, Offset: 0x03D6}, pending.event.Caller, 0xCA, 0x1841, 0x3912, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isVerifiedStoryOpeningReturn(tc.previous, tc.opcode, tc.at, tc.ss, tc.sp, tc.pending); got != tc.want {
				t.Fatalf("return edge=%v，要 %v", got, tc.want)
			}
		})
	}
}

func TestStoryOpeningGenerationAndSafeRectHelpers(t *testing.T) {
	events := []buckrogers.StoryOpeningEvent{{Generation: 1, EventKey: "old"}, {Generation: 2, EventKey: "new-1"}, {Generation: 2, EventKey: "new-2"}}
	got := storyOpeningEventsForGeneration(events, 2)
	if len(got) != 2 || got[0].EventKey != "new-1" || got[1].EventKey != "new-2" {
		t.Fatalf("generation events=%#v", got)
	}
	for _, scale := range []int{2, 3} {
		baseline := make([]byte, 320*200*scale*scale*4)
		overlay := append([]byte(nil), baseline...)
		inside := ((136*scale)*(320*scale) + 8*scale) * 4
		overlay[inside] = 1
		outside := ((135*scale)*(320*scale) + 8*scale) * 4
		overlay[outside] = 1
		out, in, added := storyOpeningDiff(baseline, overlay, scale)
		if out != 1 || in != 1 || added != 1 {
			t.Fatalf("%dx diff outside=%d inside=%d added=%d", scale, out, in, added)
		}
	}
	if err := validateStoryOpeningOverlayDraw(nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := validateStoryOpeningOverlayDraw([]string{"one"}, nil, true); err == nil {
		t.Fatal("不完整首屏 stamp 必須失敗即關閉")
	}
}

func TestStoryOpeningInvalidationRequiresActiveCompleteGroup(t *testing.T) {
	at := buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}
	keys := []string{"story.opening.line.001", "story.opening.line.002", "story.opening.line.003", "story.opening.line.004", "story.opening.line.005"}
	item, ok := storyOpeningInvalidation(at, 0xA000, 0xAB48, 304, 281020572, 2, keys, true)
	if !ok || item.Step != 281020572 || item.Generation != 2 || item.ActiveKeysBefore != 5 || item.VideoOffset != 0xAB48 || item.ByteCount != 304 {
		t.Fatalf("invalidation=%#v ok=%v", item, ok)
	}
	for _, tc := range []struct {
		name string
		at   buckrogers.Address
		es   uint16
		keys []string
		hit  bool
	}{
		{"not active", at, 0xA000, keys, false},
		{"partial group", at, 0xA000, keys[:4], true},
		{"wrong fill address", buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3C}, 0xA000, keys, true},
		{"not vram", at, 0xB800, keys, true},
		{"duplicate key", at, 0xA000, []string{keys[0], keys[1], keys[2], keys[3], keys[3]}, true},
		{"wrong key", at, 0xA000, []string{keys[0], keys[1], keys[2], keys[3], "story.opening.line.999"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, got := storyOpeningInvalidation(tc.at, tc.es, 1, 1, 1, 1, tc.keys, tc.hit); got {
				t.Fatal("不完整或未證實轉場不得產生 lifecycle receipt")
			}
		})
	}
	if _, ok := storyOpeningInvalidation(at, 0xA000, 0xAB48, 304, 1, 0, keys, true); ok {
		t.Fatal("零 generation 不得產生 lifecycle receipt")
	}
}

func TestActionBarDiffRejectsPixelsOutsideApprovedRow24Rects(t *testing.T) {
	for _, tc := range []struct {
		name        string
		scale, x, y int
		outside     int
	}{
		{"add 空白格允許", 2, 31, 199, 0},
		{"subtract 允許", 3, 32, 192, 0},
		{"底框拒絕", 2, 0, 191, 1},
		{"標籤間隙拒絕", 3, 100, 192, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseline := make([]byte, 320*tc.scale*200*tc.scale*4)
			overlay := append([]byte(nil), baseline...)
			i := ((tc.y*tc.scale)*(320*tc.scale) + tc.x*tc.scale) * 4
			overlay[i] = 1
			outside, inside, added := actionBarDiff(baseline, overlay, tc.scale)
			if outside != tc.outside || inside != 1-tc.outside || added != inside {
				t.Fatalf("outside=%d inside=%d added=%d", outside, inside, added)
			}
		})
	}
}

func TestActionBarHotkeyStylePreservesWhiteMnemonicOnly(t *testing.T) {
	normal := buckrogers.ActionBarOverlayAction{EventKey: "career.action.add.normal", Background: 0, RuneForegrounds: []uint8{10, 10, 10, 15, 10}}
	focus := buckrogers.ActionBarOverlayAction{EventKey: "career.action.add.focus", Background: 15, RuneForegrounds: []uint8{0, 0, 0, 0, 0}}
	for _, action := range []buckrogers.ActionBarOverlayAction{normal, focus} {
		if err := validateActionBarHotkeyStyle(action); err != nil {
			t.Fatal(err)
		}
	}
	normal.RuneForegrounds[0] = 15
	if err := validateActionBarHotkeyStyle(normal); err == nil {
		t.Fatal("括號不可改成白色")
	}
	focus.EventKey = "career.action.add.disabled"
	if err := validateActionBarHotkeyStyle(focus); err == nil {
		t.Fatal("未證實 disabled 不可接受")
	}
}

func TestPaletteIndicesAreContentSafeJSONNumbers(t *testing.T) {
	got := paletteIndices([]uint8{10, 15, 10})
	want := []int{10, 15, 10}
	if len(got) != len(want) {
		t.Fatalf("長度=%d，要 %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]=%d，要 %d", i, got[i], want[i])
		}
	}
}

func TestValidateOverlayDrawAllowsEmptyTerminalFrame(t *testing.T) {
	for _, tc := range []struct {
		name    string
		active  []string
		missing []rune
		drew    bool
		wantOK  bool
	}{
		{"有 stamp 且有繪製", []string{"prompt"}, nil, true, true},
		{"清除後空終態", nil, nil, false, true},
		{"有 stamp 卻未繪製", []string{"prompt"}, nil, false, false},
		{"無 stamp 卻聲稱繪製", nil, nil, true, false},
		{"缺字", []string{"prompt"}, []rune{'缺'}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateOverlayDraw(tc.active, tc.missing, tc.drew) == nil; got != tc.wantOK {
				t.Fatalf("有效性 = %v，要 %v", got, tc.wantOK)
			}
		})
	}
}

func TestOverlayFlagsRequireCompleteExplicitTwoOrThreeScale(t *testing.T) {
	valid := []struct {
		menuEvents, menuRects, genderEvents, genderRects   string
		classEvents, classRects, rosterEvents, rosterRects string
		characterSheetEvents, characterSheetRects          string
		namePromptEvents, namePromptRects                  string
		careerSkillEvents, careerSkillRects                string
		technicalSkillEvents, technicalSkillRects          string
		font, out                                          string
		scale                                              int
		want                                               bool
	}{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", 0, true},
		{"m.e", "m.r", "", "", "", "", "r.e", "r.r", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 2, true},
		{"m.e", "m.r", "g.e", "g.r", "c.e", "c.r", "", "", "s.e", "s.r", "", "", "", "", "", "", "f.bin", "o.rgba", 3, true},
		{"", "", "", "", "", "", "", "", "s.e", "s.r", "", "", "", "", "", "", "f.bin", "o.rgba", 2, true},
		{"", "", "", "", "", "", "", "", "", "", "n.e", "n.r", "", "", "", "", "f.bin", "o.rgba", 2, true},
		{"", "", "", "", "", "", "", "", "", "", "", "", "k.e", "k.r", "", "", "f.bin", "o.rgba", 3, true},
		{"", "", "", "", "", "", "", "", "", "", "", "", "k.e", "k.r", "t.e", "t.r", "f.bin", "o.rgba", 2, true},
		{"m.e", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 2, false},
		{"", "m.r", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 2, false},
		{"m.e", "m.r", "g.e", "", "", "", "", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 2, false},
		{"", "", "", "", "", "", "", "", "s.e", "", "", "", "", "", "", "", "f.bin", "o.rgba", 2, false},
		{"", "", "", "", "", "", "", "", "", "s.r", "", "", "", "", "", "", "f.bin", "o.rgba", 2, false},
		{"", "", "", "", "", "", "", "", "", "", "n.e", "", "", "", "", "", "f.bin", "o.rgba", 2, false},
		{"", "", "", "", "", "", "", "", "", "", "", "", "k.e", "", "", "", "f.bin", "o.rgba", 2, false},
		{"", "", "", "", "", "", "", "", "", "", "", "", "k.e", "k.r", "t.e", "", "f.bin", "o.rgba", 2, false},
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "t.e", "t.r", "f.bin", "o.rgba", 2, false},
		{"m.e", "m.r", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "f.bin", "", 2, false},
		{"m.e", "m.r", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 1, false},
		{"m.e", "m.r", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 4, false},
	}
	for _, tc := range valid {
		got := validateOverlayFlags(tc.menuEvents, tc.menuRects, tc.genderEvents, tc.genderRects,
			tc.classEvents, tc.classRects, tc.rosterEvents, tc.rosterRects,
			tc.characterSheetEvents, tc.characterSheetRects,
			tc.namePromptEvents, tc.namePromptRects,
			tc.careerSkillEvents, tc.careerSkillRects,
			tc.technicalSkillEvents, tc.technicalSkillRects,
			tc.font, tc.out, tc.scale) == nil
		if got != tc.want {
			t.Fatalf("flags=%#v valid=%v，要 %v", tc, got, tc.want)
		}
	}
}

func TestEmitReceiptWritesIdenticalBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	var stdout bytes.Buffer
	value := struct {
		Count int `json:"count"`
	}{Count: 14}
	if err := emitReceipt(&stdout, path, value); err != nil {
		t.Fatal(err)
	}
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), file) || string(file) != "{\"count\":14}\n" {
		t.Fatalf("stdout=%q file=%q", stdout.Bytes(), file)
	}
}

func TestEmitReceiptRejectsOutputDirectory(t *testing.T) {
	if err := emitReceipt(&bytes.Buffer{}, t.TempDir(), struct{}{}); err == nil {
		t.Fatal("receipt-out 指向目錄時必須失敗")
	}
}

func TestConfigureScratch(t *testing.T) {
	d := dos.New(machine.New(), t.TempDir())
	if err := configureScratch(d, ""); err != nil || d.Scratch != "" {
		t.Fatalf("空 scratch = %q, %v", d.Scratch, err)
	}
	dir := t.TempDir()
	if err := configureScratch(d, dir); err != nil || d.Scratch != dir {
		t.Fatalf("scratch = %q, %v，要 %q", d.Scratch, err, dir)
	}
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := configureScratch(d, file); err == nil {
		t.Fatal("一般檔案不可作為 scratch")
	}
	if err := configureScratch(d, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("不存在的 scratch 必須失敗")
	}
}

func TestUnimplementedReportIsOptionalAndSorted(t *testing.T) {
	d := dos.New(machine.New(), t.TempDir())
	d.Unimplemented[dos.Call{Int: 0x21, AH: 0x22, AL: 0x00}] = 1
	d.Unimplemented[dos.Call{Int: 0x16, AH: 0x99, AL: 0x01}] = 3
	if got := unimplementedReport(false, d); got != nil {
		t.Fatalf("旗標關閉時 report = %#v，要 nil", got)
	}
	got := unimplementedReport(true, d)
	want := d.UnimplementedReport()
	if len(got) != 2 || len(want) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("report = %#v，要 %#v", got, want)
	}
	if empty := unimplementedReport(true, dos.New(machine.New(), t.TempDir())); len(empty) != 0 {
		t.Fatalf("空集合 report = %#v，要空", empty)
	}
}

func TestSaveTerminalState(t *testing.T) {
	m := machine.New()
	d := dos.New(m, t.TempDir())
	d.Install()
	m.Steps = 12345
	m.Write8(0x23456, 0xA5)
	if err := saveTerminalState("", m, d); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "terminal.state")
	if err := saveTerminalState(path, m, d); err != nil {
		t.Fatal(err)
	}
	m2 := machine.New()
	d2 := dos.New(m2, t.TempDir())
	d2.Install()
	if err := state.Load(path, m2, d2); err != nil {
		t.Fatal(err)
	}
	if m2.Steps != m.Steps || m2.Read8(0x23456) != 0xA5 {
		t.Fatalf("回讀 state：steps=%d byte=%02X", m2.Steps, m2.Read8(0x23456))
	}
	if err := saveTerminalState(t.TempDir(), m, d); err == nil {
		t.Fatal("state-out 指向目錄時必須失敗")
	}
}

func TestParseScheduledBIOSKey(t *testing.T) {
	got, err := parseScheduledBIOSKey("100010000:50:00")
	if err != nil || got != (scheduledBIOSKey{100010000, 0x50, 0x00}) {
		t.Fatalf("parse = %#v, %v", got, err)
	}
	for _, value := range []string{
		"", "0:50:00", "1", "1:50", "1:50:00:00", "x:50:00",
		"1:5:00", "1:050:00", "1:GG:00", "1:5A:00", "1:50:0A",
	} {
		if _, err := parseScheduledBIOSKey(value); err == nil {
			t.Errorf("應拒絕 %q", value)
		}
	}
}

func TestMergeBIOSKeySchedule(t *testing.T) {
	keys, err := mergeBIOSKeySchedule(20, []scheduledBIOSKey{{30, 0x48, 0}, {10, 0x50, 0}}, 40)
	if err != nil {
		t.Fatal(err)
	}
	want := []scheduledBIOSKey{{10, 0x50, 0}, {20, 0x1C, 0x0D}, {30, 0x48, 0}}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys[%d] = %#v，要 %#v", i, keys[i], want[i])
		}
	}
	for _, tc := range []struct {
		enter uint64
		keys  []scheduledBIOSKey
		until uint64
	}{
		{10, []scheduledBIOSKey{{10, 0x50, 0}}, 20},
		{0, []scheduledBIOSKey{{20, 0x50, 0}}, 20},
	} {
		if _, err := mergeBIOSKeySchedule(tc.enter, tc.keys, tc.until); err == nil {
			t.Fatal("重複 step 或 until 邊界應拒絕")
		}
	}
}

func TestQueueIndexedScreen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screen.bin")
	data := make([]byte, 320*200)
	data[12345] = 0x0F
	outputs := &receiptOutputs{}
	if err := queueIndexedScreen(outputs, path, data); err != nil {
		t.Fatal(err)
	}
	if err := outputs.commitReceipt(&bytes.Buffer{}, "", struct{}{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || len(got) != len(data) || got[12345] != 0x0F {
		t.Fatalf("screen len=%d err=%v", len(got), err)
	}
	if err := queueIndexedScreen(&receiptOutputs{}, path, data[:len(data)-1]); err == nil {
		t.Fatal("短 framebuffer 應拒絕")
	}
}

func TestStoryDiagnosticsSerializeMetadataNotOriginalBytes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		keys  []string
	}{
		{
			name: "single glyph",
			value: glyphJSON{EntryStep: 1, PostCallStep: 2, Caller: buckrogers.Address{Segment: 0x0763, Offset: 0x04FF},
				Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: 17, Column: 1},
			keys: []string{"entry_step", "post_call_step", "caller", "mode", "repeat", "background", "foreground", "row", "column"},
		},
		{
			name: "glyph run",
			value: glyphRunJSON{EntryStep: 1, PostCallStep: 2, Caller: buckrogers.Address{Segment: 0x0763, Offset: 0x04FF},
				OriginalLength: 37, OriginalSHA256: "digest", Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: 17, Column: 1},
			keys: []string{"entry_step", "post_call_step", "caller", "original_length", "original_sha256", "mode", "repeat", "background", "foreground", "row", "column"},
		},
		{
			name:  "story pixel write",
			value: pixelWriteJSON{Step: 3, Caller: buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}, VideoSegment: 0xA000, VideoOffset: 0xAB0A, ByteCount: 285, X0: 10, Y0: 137, X1: 294, Y1: 137},
			keys:  []string{"step", "caller", "video_segment", "video_offset", "byte_count", "x0", "y0", "x1", "y1"},
		},
		{
			name:  "glyph return edge",
			value: glyphReturnEdgeJSON{EntryStep: 1, ReturnStep: 2, ReturnInstruction: buckrogers.Address{Segment: 0x0763, Offset: 0x1809}, ReturnOpcode: 0xCA, HighWordMask: 0x7f, Caller: buckrogers.Address{Segment: 0x0763, Offset: 0x04FF}, PostAddress: buckrogers.Address{Segment: 0x0763, Offset: 0x04FF}, SS: 0x1234, SP: 0x5678},
			keys:  []string{"entry_step", "entry_ss", "entry_sp", "return_step", "return_instruction", "return_opcode", "high_word_mask", "mode", "repeat", "background", "foreground", "row", "column", "caller", "post_address", "ss", "sp"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(data, &object); err != nil {
				t.Fatal(err)
			}
			if len(object) != len(tc.keys) {
				t.Fatalf("metadata keys=%v，要 %v", object, tc.keys)
			}
			for _, key := range tc.keys {
				if _, ok := object[key]; !ok {
					t.Fatalf("缺少 metadata key %q：%s", key, data)
				}
			}
			for _, forbidden := range []string{"glyph_byte", "original_bytes", "indexed", "pixels", "answer"} {
				if _, ok := object[forbidden]; ok {
					t.Fatalf("content-safe 診斷不得序列化 %q：%s", forbidden, data)
				}
			}
		})
	}
}

func TestStoryPage3OverlayFlagsAndSafeRectangle(t *testing.T) {
	if err := validateStoryOpeningOverlayFlags("events", "translations", "font", "out", "baseline", "png", "baseline-png", 3); err != nil {
		t.Fatalf("完整第三頁旗標被拒絕：%v", err)
	}
	if err := validateStoryOpeningOverlayFlags("events", "", "font", "out", "baseline", "png", "baseline-png", 3); err == nil {
		t.Fatal("不完整第三頁旗標被接受")
	}
	for _, scale := range []int{2, 3} {
		base := make([]byte, 320*200*scale*scale*4)
		inside := append([]byte(nil), base...)
		inside[((136*scale)*(320*scale)+8*scale)*4] = 1
		outside := append([]byte(nil), base...)
		outside[((176*scale)*(320*scale)+8*scale)*4] = 1
		if out, in, added := storyPage3Diff(base, inside, scale); out != 0 || in != 1 || added != 1 {
			t.Fatalf("%dx 內部差異 = (%d,%d,%d)", scale, out, in, added)
		}
		if out, in, added := storyPage3Diff(base, outside, scale); out != 1 || in != 0 || added != 0 {
			t.Fatalf("%dx 邊界外差異 = (%d,%d,%d)", scale, out, in, added)
		}
	}
}

func TestStoryPage6OverlayFlagsReceiptAndSafeRectangle(t *testing.T) {
	for _, scale := range []int{2, 3} {
		if err := validateStoryOpeningOverlayFlags("events", "translations", "font", "out", "baseline", "png", "baseline-png", scale); err != nil {
			t.Fatalf("完整第 6 頁 %dx 旗標被拒絕：%v", scale, err)
		}
	}
	for _, missing := range [][]string{
		{"events", "", "font", "out", "baseline", "png", "baseline-png"},
		{"events", "translations", "", "out", "baseline", "png", "baseline-png"},
		{"events", "translations", "font", "", "baseline", "png", "baseline-png"},
	} {
		if err := validateStoryOpeningOverlayFlags(missing[0], missing[1], missing[2], missing[3], missing[4], missing[5], missing[6], 2); err == nil {
			t.Fatal("第 6 頁不完整旗標被接受")
		}
	}
	for _, scale := range []int{2, 3} {
		base := make([]byte, 320*200*scale*scale*4)
		inside := append([]byte(nil), base...)
		inside[((183*scale)*(320*scale)+8*scale)*4] = 1
		outside := append([]byte(nil), base...)
		outside[((184*scale)*(320*scale)+8*scale)*4] = 1
		if out, in, added := storyPage6Diff(base, inside, scale); out != 0 || in != 1 || added != 1 {
			t.Fatalf("第 6 頁 %dx 內部差異 = (%d,%d,%d)", scale, out, in, added)
		}
		if out, in, added := storyPage6Diff(base, outside, scale); out != 1 || in != 0 || added != 0 {
			t.Fatalf("第 6 頁 %dx 邊界外差異 = (%d,%d,%d)", scale, out, in, added)
		}
	}
	item := storyPage2InvalidationJSON{Step: 331026784, Instruction: buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}, VideoSegment: 0xA000, VideoOffset: 0xAA08, ByteCount: 304, ActiveKeysBefore: 6}
	encoded, err := json.Marshal(item)
	if err != nil || !bytes.Contains(encoded, []byte(`"active_keys_before":6`)) || bytes.Contains(encoded, []byte("original_bytes")) {
		t.Fatalf("第 6 頁 invalidation receipt 非 content-safe：%s, %v", encoded, err)
	}
}

func TestStoryPage7OverlayFlagsReceiptAndSafeRectangle(t *testing.T) {
	for _, scale := range []int{2, 3} {
		if err := validateStoryOpeningOverlayFlags("events", "translations", "font", "out", "baseline", "png", "baseline-png", scale); err != nil {
			t.Fatalf("完整第 7 頁 %dx 旗標被拒絕：%v", scale, err)
		}
	}
	complete := []string{"events", "translations", "font", "out", "baseline", "png", "baseline-png"}
	for omitted := range complete {
		missing := append([]string(nil), complete...)
		missing[omitted] = ""
		if err := validateStoryOpeningOverlayFlags(missing[0], missing[1], missing[2], missing[3], missing[4], missing[5], missing[6], 2); err == nil {
			t.Fatalf("第 7 頁缺第 %d 個旗標被接受", omitted)
		}
	}
	for _, scale := range []int{-1, 0, 1, 4} {
		if err := validateStoryOpeningOverlayFlags(complete[0], complete[1], complete[2], complete[3], complete[4], complete[5], complete[6], scale); err == nil {
			t.Fatalf("第 7 頁無效倍率 %d 被接受", scale)
		}
	}
	if err := validateStoryOpeningOverlayFlags("", "", "", "", "", "", "", 2); err == nil {
		t.Fatal("第 7 頁只提供倍率被接受")
	}
	for _, scale := range []int{2, 3} {
		base := make([]byte, 320*200*scale*scale*4)
		inside := append([]byte(nil), base...)
		inside[((183*scale)*(320*scale)+8*scale)*4] = 1
		outside := append([]byte(nil), base...)
		outside[((184*scale)*(320*scale)+8*scale)*4] = 1
		if out, in, added := storyPage7Diff(base, inside, scale); out != 0 || in != 1 || added != 1 {
			t.Fatalf("第 7 頁 %dx 內部差異 = (%d,%d,%d)", scale, out, in, added)
		}
		if out, in, added := storyPage7Diff(base, outside, scale); out != 1 || in != 0 || added != 0 {
			t.Fatalf("第 7 頁 %dx 邊界外差異 = (%d,%d,%d)", scale, out, in, added)
		}
	}
	item := storyPage2InvalidationJSON{Step: 341018656, Instruction: buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}, VideoSegment: 0xA000, VideoOffset: 0xAA08, ByteCount: 304, ActiveKeysBefore: 6}
	encoded, err := json.Marshal(item)
	if err != nil || !bytes.Contains(encoded, []byte(`"active_keys_before":6`)) || bytes.Contains(encoded, []byte("original_bytes")) {
		t.Fatalf("第 7 頁 invalidation receipt 非 content-safe：%s, %v", encoded, err)
	}
}

func TestStoryPage8OverlayFlagsReceiptAndActiveInvalidation(t *testing.T) {
	complete := []string{"events", "translations", "font", "out", "baseline", "png", "baseline-png"}
	for _, scale := range []int{2, 3} {
		if err := validateStoryOpeningOverlayFlags(complete[0], complete[1], complete[2], complete[3], complete[4], complete[5], complete[6], scale); err != nil {
			t.Fatalf("完整第 8 頁 %dx 旗標被拒絕：%v", scale, err)
		}
	}
	for omitted := range complete {
		missing := append([]string(nil), complete...)
		missing[omitted] = ""
		if err := validateStoryOpeningOverlayFlags(missing[0], missing[1], missing[2], missing[3], missing[4], missing[5], missing[6], 2); err == nil {
			t.Fatalf("第 8 頁缺第 %d 個旗標被接受", omitted)
		}
	}
	for _, scale := range []int{-1, 0, 1, 4} {
		if err := validateStoryOpeningOverlayFlags(complete[0], complete[1], complete[2], complete[3], complete[4], complete[5], complete[6], scale); err == nil {
			t.Fatalf("第 8 頁無效倍率 %d 被接受", scale)
		}
	}
	if err := validateStoryOpeningOverlayFlags("", "", "", "", "", "", "", 2); err == nil {
		t.Fatal("第 8 頁只提供倍率被接受")
	}

	for _, scale := range []int{2, 3} {
		base := make([]byte, 320*200*scale*scale*4)
		inside := append([]byte(nil), base...)
		inside[((167*scale)*(320*scale)+311*scale)*4] = 1
		right := append([]byte(nil), base...)
		right[((136*scale)*(320*scale)+312*scale)*4] = 1
		bottom := append([]byte(nil), base...)
		bottom[((168*scale)*(320*scale)+8*scale)*4] = 1
		if outside, in, added := storyPage8Diff(base, inside, scale); outside != 0 || in != 1 || added != 1 {
			t.Fatalf("第 8 頁 %dx 內部差異=(%d,%d,%d)", scale, outside, in, added)
		}
		for name, candidate := range map[string][]byte{"right": right, "bottom": bottom} {
			if outside, in, added := storyPage8Diff(base, candidate, scale); outside != 1 || in != 0 || added != 0 {
				t.Fatalf("第 8 頁 %dx %s 界外差異=(%d,%d,%d)", scale, name, outside, in, added)
			}
		}

		font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
		text := make(map[string]string, 4)
		events := make([]buckrogers.StoryPage8Event, 4)
		for i, r := range []rune("甲乙丙丁") {
			font.Glyphs[r] = make([]byte, 32)
			key := fmt.Sprintf("story.page8.line.%03d", i+1)
			text[key] = string(r)
			events[i] = buckrogers.StoryPage8Event{Generation: 1, EntryStep: uint64(100 + i*20), PostCallStep: uint64(110 + i*20), EventKey: key, Row: uint8(17 + i), Column: 1}
		}
		overlay, err := buckrogers.NewRuntimeStoryPage8Overlay(text, font, scale)
		if err != nil {
			t.Fatal(err)
		}
		if err := overlay.Apply(events, [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
		before := len(overlay.ActiveKeys())
		if before != 4 || !buckrogers.StoryPage8VideoSpanIntersects(0xaa08, 304) {
			t.Fatalf("前置 active/prewrite 無效：before=%d", before)
		}
		item := storyPage2InvalidationJSON{Step: 351154334, Instruction: buckrogers.Address{Segment: 0x0CF4, Offset: 0x1B3A}, VideoSegment: 0xA000, VideoOffset: 0xAA08, ByteCount: 304, ActiveKeysBefore: before}
		encoded, err := json.Marshal(item)
		if err != nil || !bytes.Contains(encoded, []byte(`"active_keys_before":4`)) || bytes.Contains(encoded, []byte("original_bytes")) {
			t.Fatalf("第 8 頁 invalidation receipt 非 content-safe：%s, %v", encoded, err)
		}
		overlay.Clear()
		indexed := make([]byte, 320*200)
		palette := [256][3]uint8{}
		baseline := buckrogers.ScaleIndexedRGBA(indexed, palette, scale)
		got, missing, drew := overlay.Draw(indexed, palette)
		if len(overlay.ActiveKeys()) != 0 || drew || len(missing) != 0 || !bytes.Equal(got, baseline) {
			t.Fatalf("active→prewrite 未清除：keys=%v drew=%v missing=%q equal=%v", overlay.ActiveKeys(), drew, string(missing), bytes.Equal(got, baseline))
		}
	}
}
