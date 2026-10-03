// Command phantasie-receipt 是無頭收據工具（docs/spec/005 §5）：重播路線檔、在每個 @check 輸出一列收據、
// 執行稽核與同狀態比對所需的欄位。缺原版檔時整個工具 SKIP（不算驗收）。
//
//	phantasie-receipt -root <原版目錄> -route tests/routes/title.route -lang zh-TW -text text -font <字型目錄> -out <輸出目錄>
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/dosgolem/apps/phantasie"
	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/xlate"
)

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(s string) error { *m = append(*m, s); return nil }

var tsvEsc = strings.NewReplacer("\\", "\\\\", "\t", "\\t", "\n", "\\n")

func join(ss []string) string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = tsvEsc.Replace(s)
	}
	return strings.Join(out, "|")
}

func set(ss []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range ss {
		m[s] = true
	}
	return m
}

var header = []string{"lang", "mode", "check", "image_hash", "img_seg", "hook_sig", "font_hash", "steps", "reads",
	"vram_hash", "mem_hash", "stamps", "layer_hash", "keys", "untranslated", "untranslated_args", "counters",
	"stale_cells", "exposed_events", "visible_hash", "content_hash", "mask_strict", "png", "verdict"}

func main() {
	root := flag.String("root", "", "原版目錄（必填；缺檔時 SKIP）")
	bat := flag.String("bat", "", "啟動批次檔名；空字串＝目錄內唯一的 .BAT")
	routePath := flag.String("route", "", "路線檔（必填）")
	var langs multi
	flag.Var(&langs, "lang", "語言（可重複）：zh-TW、zh-CN、ja、ko；預設 zh-TW")
	var extra multi
	flag.Var(&extra, "extra-lang", "額外載入的語言（可重複），供路線的 @lang 切換使用")
	overlay := flag.String("overlay", "on", "on 或 off（off：維護 Layer 但不呼叫 Draw）")
	hooks := flag.String("hooks", "all", "all 或 none（none：完全不掛鉤子，供唯讀證明）")
	fault := flag.String("fault", "", "故障注入（僅測試用）：noadd、noclear、nostrcat、verify-early")
	every := flag.Uint64("frame-every", 20_000, "無頭模式在 @check 以外的 Frame 間隔（步數）")
	maxSteps := flag.Uint64("max-steps", 4_000_000_000, "整條路線的步數上限")
	outDir := flag.String("out", "", "輸出目錄（收據 TSV 與 PNG；必填）")
	textDir := flag.String("text", "text", "譯文目錄")
	fontDir := flag.String("font", "", "字型目錄（<lang>.golemfnt；必填）")
	auditDebug := flag.Bool("audit-debug", false, "印出稽核找到的每個殘字格與外露事件的細節（診斷用）")
	dumpKeys := flag.Bool("dump-keys", false, "每個檢查點印出診斷鍵集合（untranslated_at、arg_unclassified 等，診斷用）")
	dumpStamps := flag.Bool("dump-stamps", false, "每個檢查點印出全部疊字（診斷用）")
	positionOracle := flag.Bool("position-oracle", true, "以 25A5 實際寫入的視訊足跡檢查每個提交事件的疊字位置（docs/spec/001 §10 第 3 項）")
	emitRoute := flag.String("emit-route", "", "把本次觀察到的疊字鍵與未譯鍵寫成路線檔（@expect、@known-untranslated 基線；只用第一個語言的結果）")
	dumpScroll := flag.String("dump-scroll", "", "把每次 INT 10h AH=06h 或 07h 的入口參數與前後視訊記憶體存成檔案（目錄；dosgolem 規格 250 的同狀態收據用）")
	flag.Parse()
	if *root == "" || *routePath == "" || *outDir == "" || *fontDir == "" {
		flag.Usage()
		os.Exit(2)
	}
	if _, err := os.Stat(*root); err != nil {
		fmt.Printf("SKIP：找不到原版目錄 %s，不算驗收\n", *root)
		return
	}
	if len(langs) == 0 {
		langs = multi{"zh-TW"}
	}
	rb, err := os.ReadFile(*routePath)
	if err != nil {
		fatal(err)
	}
	steps, err := phantasie.ParseRoute(string(rb))
	if err != nil {
		fatal(fmt.Errorf("%s：%w", *routePath, err))
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal(err)
	}
	routeName := strings.TrimSuffix(filepath.Base(*routePath), filepath.Ext(*routePath))
	failed := false
	for _, lang := range langs {
		ok, err := runLang(lang, runOpts{
			root: *root, bat: *bat, steps: steps, routeName: routeName, overlay: *overlay, hooks: *hooks,
			fault: *fault, every: *every, maxSteps: *maxSteps, outDir: *outDir, textDir: *textDir, fontDir: *fontDir, auditDebug: *auditDebug, dumpStamps: *dumpStamps, dumpKeys: *dumpKeys, positionOracle: *positionOracle, emitRoute: *emitRoute, dumpScroll: *dumpScroll, extra: extra,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s：%v\n", lang, err)
			failed = true
			continue
		}
		if !ok {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

type runOpts struct {
	root, bat, routeName, overlay, hooks, fault string
	steps                                       []phantasie.RouteStep
	every, maxSteps                             uint64
	outDir, textDir, fontDir                    string
	auditDebug                                  bool
	dumpStamps                                  bool
	dumpKeys                                    bool
	positionOracle                              bool
	emitRoute                                   string
	dumpScroll                                  string
	extra                                       multi
}

func runLang(lang string, op runOpts) (bool, error) {
	o, names, err := phantasie.Launch(op.root, op.bat)
	if err != nil {
		return false, err
	}
	defer o.Close()
	chain := oracle.NewCond("鏈載入完成", func(o *oracle.Oracle) bool { return len(o.ExecLog()) >= len(names)-1 })
	if err := o.RunUntil(chain, oracle.Budget(op.maxSteps)); err != nil {
		return false, fmt.Errorf("等鏈載入：%w", err)
	}
	img := phantasie.ImageSeg(o)
	dg := img + phantasie.DGroupParas

	var ov *phantasie.Overlay
	var hk *phantasie.Hooks
	if op.hooks != "none" {
		ov = phantasie.NewOverlay()
		l := phantasie.LoadLanguage(lang, op.textDir, op.fontDir)
		if !l.Enabled {
			return false, fmt.Errorf("語言 %s 無法載入：%s", lang, l.Err)
		}
		ov.AddLanguage(l)
		// 路線的 @lang 用到的語言自動載入。
		need := map[string]bool{}
		for _, name := range op.extra {
			need[name] = true
		}
		for _, st := range op.steps {
			if st.Kind == phantasie.RouteLang {
				need[st.Name] = true
			}
		}
		delete(need, lang)
		delete(need, "en")
		var extras []string
		for name := range need {
			extras = append(extras, name)
		}
		sort.Strings(extras)
		for _, name := range extras {
			x := phantasie.LoadLanguage(name, op.textDir, op.fontDir)
			if !x.Enabled {
				return false, fmt.Errorf("語言 %s 無法載入：%s", name, x.Err)
			}
			ov.AddLanguage(x)
		}
		if err := ov.SetDisplay(lang); err != nil {
			return false, err
		}
		if op.overlay == "off" {
			if err := ov.SetDisplay("en"); err != nil {
				return false, err
			}
		}
		if op.fault != "" {
			ov.Faults[op.fault] = true
		}
		regions, err := phantasie.DefaultRegions()
		if err != nil {
			return false, err
		}
		hk = phantasie.InstallHooks(o, img, ov, regions)
		switch op.fault {
		case "nostrcat":
			hk.NoStrcat = true
		case "verify-early":
			hk.VerifyNow()
		}
		if op.dumpScroll != "" {
			if err := os.MkdirAll(op.dumpScroll, 0o755); err != nil {
				return false, err
			}
			seq := 0
			hk.Int10Probe = func(ax, bx, cx, dx uint16, pre, post []byte) {
				buf := []byte{byte(ax), byte(ax >> 8), byte(bx), byte(bx >> 8), byte(cx), byte(cx >> 8), byte(dx), byte(dx >> 8)}
				buf = append(append(buf, pre...), post...)
				name := filepath.Join(op.dumpScroll, fmt.Sprintf("%s.%04d.bin", op.routeName, seq))
				seq++
				if err := os.WriteFile(name, buf, 0o644); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}
		}
		if op.auditDebug {
			ov.AuditDebug = func(s string) { fmt.Println("# " + s) }
		}
		if op.positionOracle {
			ov.CommitHook = func(rec *phantasie.EventRecord, stamps []*xlate.Stamp) {
				if why := phantasie.CheckPosition(rec, stamps, hk.Writes()); why != "" {
					ov.C.Inc("pos_bad")
					ov.C.Key("pos_bad", fmt.Sprintf("%04X|%d,%d|%s", rec.Caller, rec.Col, rec.Row, why))
				} else {
					ov.C.Inc("pos_ok")
				}
			}
		}
	}
	gate := phantasie.NewKeyGate(o, img)

	frame := func() {
		if ov == nil {
			return
		}
		idx, rgb := phantasie.FrameBuffers(o)
		ov.Frame(idx, rgb)
	}
	var staleMax, exposedMax, strictMax int
	sample := func() {
		if ov == nil || (hk != nil && hk.Busy()) {
			return
		}
		idx, _ := phantasie.FrameBuffers(o)
		if n := ov.AuditStale(idx); n > staleMax {
			staleMax = n
		}
		ex, strict := ov.AuditEvents(idx)
		if ex > exposedMax {
			exposedMax = ex
		}
		if strict > strictMax {
			strictMax = strict
		}
	}

	tsvPath := filepath.Join(op.outDir, fmt.Sprintf("%s.%s.%s.tsv", op.routeName, lang, op.overlay+"-"+op.hooks))
	f, err := os.Create(tsvPath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	fmt.Fprintln(f, strings.Join(header, "\t"))

	allOK := true
	cur := lang
	vis := map[string]uint64{}
	scr := map[string][2]uint64{} // 檢查點 → {vram_hash, content_hash}
	var emit strings.Builder
	for _, st := range op.steps {
		if st.Kind == phantasie.RouteAssert {
			name := "@assert-visible-same"
			if st.Screen {
				name = "@assert-same-screen"
			}
			fmt.Fprintf(&emit, "%s %s %s\n", name, st.Name, st.Other)
			if ov == nil {
				continue
			}
			verdict := "PASS"
			if vis[st.Name] != vis[st.Other] {
				verdict = fmt.Sprintf("FAIL：可見格集合不同（%016x 與 %016x）", vis[st.Name], vis[st.Other])
			}
			if st.Screen {
				a, b := scr[st.Name], scr[st.Other]
				switch {
				case a[0] != b[0]:
					verdict = fmt.Sprintf("FAIL：vram_hash 不同（%016x 與 %016x），兩個畫面不是同一狀態", a[0], b[0])
				case a[1] != b[1]:
					verdict = fmt.Sprintf("FAIL：疊字內容不同（content_hash %016x 與 %016x）", a[1], b[1])
				}
			}
			if verdict != "PASS" {
				allOK = false
			}
			fmt.Printf("assert\t%s\t%s\t%s\n", st.Name, st.Other, verdict)
			continue
		}
		if st.Kind == phantasie.RouteLang {
			fmt.Fprintf(&emit, "@lang %s\n", st.Name)
			if ov == nil {
				continue
			}
			target := st.Name
			if op.overlay == "off" {
				target = "en"
			}
			if err := ov.SetDisplay(target); err != nil {
				return false, fmt.Errorf("路線第 %d 行：%w", st.Line, err)
			}
			cur = st.Name
			continue
		}
		if st.Kind == phantasie.RouteKey {
			if st.Wait > 0 {
				fmt.Fprintf(&emit, "@wait %d\n", st.Wait)
			}
			fmt.Fprintln(&emit, st.Key)
			if st.Wait > 0 {
				gate.PressAfterReads(st.Wait, st.Key)
			} else {
				gate.Press(0, st.Key)
			}
			continue
		}
		// @check：所有排入的鍵都送出後，再等下一次讀鍵入口。
		cond := oracle.NewCond("檢查點 "+st.Name, func(*oracle.Oracle) bool {
			return gate.Pending() == 0 && gate.Reads > gate.LastSent
		})
		for {
			err := o.RunUntil(cond, oracle.Budget(op.every))
			if err == nil {
				break
			}
			var be *oracle.BudgetError
			if !errors.As(err, &be) {
				return false, fmt.Errorf("檢查點 %s：%w", st.Name, err)
			}
			if o.Steps() > op.maxSteps {
				return false, fmt.Errorf("檢查點 %s：超過步數上限 %d", st.Name, op.maxSteps)
			}
			frame()
			sample()
		}
		frame()
		sample()
		if op.dumpStamps && ov != nil {
			for _, s := range ov.Layer.Stamps {
				fmt.Printf("# stamp key=%s x=%d y=%d cells=%d cw=%d state=%d fg=%v bg=%v tr=%v text=%q\n", s.Key, s.X, s.Y, s.Cells, s.CellW, s.State, s.FG, s.BG, s.Transparent, string(s.Text))
			}
		}
		row, verdict := receipt(cur, op, st, o, ov, hk, gate, img, dg, staleMax, exposedMax, strictMax)
		if ov != nil {
			vis[st.Name] = ov.VisibleHash()
			scr[st.Name] = [2]uint64{phantasie.VramHash(o), ov.ContentHash()}
		}
		fmt.Fprintln(f, strings.Join(append(row, verdict), "\t"))
		fmt.Printf("%s\t%s\t%s\t%s\n", cur, st.Name, verdict, row[len(row)-1])
		if verdict != "PASS" && !strings.HasPrefix(verdict, "SKIP") {
			allOK = false
		}
		if hk != nil && hk.Failed() {
			return false, fmt.Errorf("鉤子簽章不符：%s", hk.Diag)
		}
		staleMax, exposedMax, strictMax = 0, 0, 0
		fmt.Fprintf(&emit, "@check %s\n", st.Name)
		if ov != nil {
			for _, k := range ov.KeysShown() {
				fmt.Fprintf(&emit, "@expect %s\n", k)
			}
			for _, k := range append(ov.C.KeySet("untranslated"), ov.C.KeySet("untranslated_args")...) {
				fmt.Fprintf(&emit, "@known-untranslated %s\n", k)
			}
		}
		if ov != nil {
			pngPath := filepath.Join(op.outDir, fmt.Sprintf("%s.%s.%s.png", op.routeName, st.Name, cur))
			idx, rgb := phantasie.FrameBuffers(o)
			if err := phantasie.WritePNG(pngPath, phantasie.ComposeImage(ov, idx, rgb, 2, nil)); err != nil {
				return false, err
			}
		}
	}
	if op.emitRoute != "" {
		if err := os.WriteFile(op.emitRoute, []byte(emit.String()), 0o644); err != nil {
			return false, err
		}
	}
	return allOK, nil
}

// receipt 組出一列收據與判定（005 §5、§5.1）。回傳不含判定欄的列與判定字串。
func receipt(lang string, op runOpts, st phantasie.RouteStep, o *oracle.Oracle, ov *phantasie.Overlay, hk *phantasie.Hooks,
	gate *phantasie.KeyGate, img, dg uint16, stale, exposed, strict int) ([]string, string) {
	mode := op.overlay + "-" + op.hooks
	sig, imageHash, fontHash := "none", "-", "-"
	if hk != nil {
		switch {
		case hk.Failed():
			sig = hk.Diag
		case hk.Armed():
			sig = "ok"
		default:
			sig = "unarmed"
		}
		imageHash, fontHash = hk.ImageHash, hk.FontHash
	}
	var keys, unt, untArgs []string
	counters := ""
	stamps := 0
	layerHash := uint64(0)
	visibleHash := uint64(0)
	contentHash := uint64(0)
	if ov != nil {
		keys = ov.KeysShown()
		unt = ov.C.KeySet("untranslated")
		untArgs = ov.C.KeySet("untranslated_args")
		counters = ov.C.String()
		if op.dumpKeys {
			for _, n := range []string{"untranslated_at", "untranslated_args_at", "arg_unclassified", "nonprintable", "truncated", "composed_miss_prefix", "fmt_other_ptr"} {
				if ks := ov.C.KeySet(n); len(ks) > 0 {
					fmt.Fprintf(os.Stderr, "KEYS %s %s: %s\n", st.Name, n, strings.Join(ks, " "))
				}
			}
		}
		stamps = len(ov.Layer.Stamps)
		layerHash = ov.LayerHash()
		visibleHash = ov.VisibleHash()
		contentHash = ov.ContentHash()
	}
	row := []string{lang, mode, st.Name, imageHash, fmt.Sprintf("%04X", img), sig, fontHash,
		fmt.Sprint(o.Steps()), fmt.Sprint(gate.Reads),
		fmt.Sprintf("%016x", phantasie.VramHash(o)), fmt.Sprintf("%016x", phantasie.MemHash(o, img, dg)),
		fmt.Sprint(stamps), fmt.Sprintf("%016x", layerHash), join(keys), join(unt), join(untArgs), counters,
		fmt.Sprint(stale), fmt.Sprint(exposed),
		fmt.Sprintf("%016x", visibleHash),
		fmt.Sprintf("%016x", contentHash), fmt.Sprint(strict),
		fmt.Sprintf("%s.%s.%s.png", op.routeName, st.Name, lang)}

	if ov == nil {
		return row, "SKIP（-hooks none：只供唯讀證明的雜湊欄位）"
	}
	var why []string
	have := set(keys)
	for _, k := range st.Expect {
		if !have[k] {
			why = append(why, "缺疊字鍵 "+k)
		}
	}
	if stamps < 1 && op.overlay == "on" {
		why = append(why, "疊字數為 0")
	}
	if n := ov.C.Get("unpaired"); n != 0 {
		why = append(why, fmt.Sprintf("unpaired=%d", n))
	}
	known := set(st.Known)
	for _, k := range append(append([]string(nil), unt...), untArgs...) {
		if !known[k] {
			why = append(why, "未譯鍵 "+k)
		}
	}
	if stale != 0 {
		why = append(why, fmt.Sprintf("stale_cells=%d", stale))
	}
	if exposed != 0 {
		why = append(why, fmt.Sprintf("exposed_events=%d", exposed))
	}
	if strict != 0 {
		why = append(why, fmt.Sprintf("mask_strict=%d", strict))
	}
	if n := ov.C.Get("pos_bad"); n != 0 {
		why = append(why, fmt.Sprintf("pos_bad=%d（%s）", n, strings.Join(ov.C.KeySet("pos_bad"), "；")))
	}
	if len(why) == 0 {
		return row, "PASS"
	}
	sort.Strings(why)
	return row, "FAIL：" + strings.Join(why, "；")
}
