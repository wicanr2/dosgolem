package strikecommander

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

// 規格 460 §5.3 期限單元測試 T1–T4、§4 O2、§5.5 介面測試。

// spFake 是合成視圖（記憶體全 0），實作 PresentView（含 View、FlightView、SubtitleView）。
type spFake struct {
	steps uint64
	ip    uint16
}

func (f *spFake) Steps() uint64                        { return f.steps }
func (f *spFake) CS() uint16                           { return 0x9000 }
func (f *spFake) IP() uint16                           { return f.ip }
func (f *spFake) SS() uint16                           { return 0x8000 }
func (f *spFake) SP() uint16                           { return 0xFF00 }
func (f *spFake) BP() uint16                           { return 0 }
func (f *spFake) DS() uint16                           { return 0x8000 }
func (f *spFake) DI() uint16                           { return 0 }
func (f *spFake) AX() uint16                           { return 0 }
func (f *spFake) CX() uint16                           { return 0 }
func (f *spFake) Read8(uint32) uint8                   { return 0 }
func (f *spFake) Deliveries() uint64                   { return 0 }
func (f *spFake) Palette() [256][3]uint8               { return [256][3]uint8{} }
func (f *spFake) ReadBlock(_ uint32, dst []byte) bool  { clear(dst); return true }
func (f *spFake) IndexedLayout() machine.IndexedLayout { return machine.IndexedLayout{} }
func (f *spFake) IndexedInto([]byte) (machine.IndexedLayout, bool) {
	return machine.IndexedLayout{}, false
}

var _ PresentView = (*spFake)(nil)

// spDriver 以 §3.1 的同一個 ShouldObserve 推進合成步驟：會觀察的步依 BeforeStep 的順序呼叫各觀察器，之後鎖存 NextWake。
type spDriver struct {
	f   machine.StepFilter
	so  *SessionObserver
	v   spFake
	obs int
}

func newSpDriver(so *SessionObserver) *spDriver {
	d := &spDriver{so: so}
	d.f.IPs = so.TriggerIPs()
	d.f.Wake, d.f.Force = session.NoWake, true
	return d
}

func (d *spDriver) observe() {
	v := &d.v
	d.so.Labels.Observe(v)
	if d.so.Flight != nil {
		d.so.Flight.Observe(v)
	}
	if d.so.Subtitles != nil {
		d.so.Subtitles.Observe(v)
	}
	if d.so.present != nil {
		d.so.present.Observe(v)
	}
}

// step 推進一步（IP 固定為 ip）；at 非 nil 時在觀察點之後、Wake 鎖存之前呼叫（白箱設定事件）。
func (d *spDriver) step(s uint64, ip uint16, at func()) {
	d.v.steps, d.v.ip = s, ip
	if d.f.ShouldObserve(ip, s) {
		d.f.Force = false
		d.obs++
		d.observe()
		if at != nil {
			at()
		}
		d.f.Wake = d.so.NextWake()
	}
}

const spOutside = 0x7777 // 不在任何觀察器的觸發集合內

// S9 的既有飛行收據沒有 CU 逾時。以真的 CU 入口與逾時返回補足前提，
// 再由稽核 I1 檢查完整觀察點之後的狀態，不直接改寫 cuOpen 或 cuTracker。
func TestSparseCUTimeoutAuditI1(t *testing.T) {
	r := newPresRig(t, 0)
	r.cuIn(ipCUDiff, 1, tCUSP)
	if in, _ := r.so.Flight.CUState(); !in || !r.p.cuOpen {
		t.Fatal("前置條件：CU 入口未開啟")
	}
	r.v.steps += cuTimeoutSteps
	r.cuRet(ipCUDiffRet, tCUSP)
	if r.so.Flight.counts.Timeouts != 1 || r.p.counts.CUTimeout != 1 {
		t.Fatal("未走到 CU 逾時返回路徑")
	}
	a := newSparseAudit(r.so)
	a.checkI1(r.so, r.v.steps, ipCUDiffRet, "CU 逾時返回後")
	if a.failN != 0 {
		t.Fatalf("I1 不成立：%v", a.fails)
	}
}

func TestSparseOutsideIPNotTrigger(t *testing.T) {
	so := spFull(t)
	set := so.TriggerIPs()
	if set.Has(spOutside) {
		t.Fatal("spOutside 在集合內")
	}
}

// spFull 建一個四個觀察器都掛上的 SessionObserver（呈現層用預設窗表）。
func spFull(t *testing.T) *SessionObserver {
	t.Helper()
	so := &SessionObserver{Labels: NewLabelObserver(), Flight: NewFlightObserver(func(Kind, string) (string, int, bool) { return "", 0, false }),
		Subtitles: &Subtitles{T: SubtitleDefaultT}}
	p, err := NewPresenter(PresenterConfig{Fonts: &TextFonts{}})
	if err != nil {
		t.Fatal(err)
	}
	p.Attach(so)
	return so
}

func spHasNote(log []Event, kind string, steps uint64) bool {
	for _, e := range log {
		if e.Kind == kind && e.Steps == steps {
			return true
		}
	}
	return false
}

// T1 Labels：可見集合非空、base 已知；在 lastSig + 5,000,000 那一步簽章不符（合成記憶體全 0）。
func TestSparseT1LabelsSig(t *testing.T) {
	lo := NewLabelObserver()
	lo.visible = []Label{{Key: "t1"}}
	lo.baseKnown, lo.baseOK, lo.base, lo.lastSig = true, true, 0x1000, 1000
	so := &SessionObserver{Labels: lo}
	d := newSpDriver(so)
	due := lo.lastSig + SigCheckSteps
	for s := uint64(1001); s <= due+10; s++ {
		d.step(s, spOutside, nil)
	}
	logs, _ := lo.Log()
	if len(lo.visible) != 0 || lo.gen != 1 || !spHasNote(logs, "sig", due) {
		t.Fatalf("T1：可見 %d、gen %d、sig 紀錄 %v（期限 %d）", len(lo.visible), lo.gen, logs, due)
	}
	if d.obs > 3 {
		t.Fatalf("T1：觀察 %d 次，不像稀疏", d.obs)
	}
}

// T2 Flight：先在回掃升格（lastSig = 回掃步，之後 session 設 Force），再在期限步簽章不符。
// 升格以白箱設定（可見集合、lastSig），Force 由驅動照 session 的回掃包裝設定（S8 的 Force 在 session，見 session 的 S8s）。
func TestSparseT2FlightSig(t *testing.T) {
	fo := NewFlightObserver(func(Kind, string) (string, int, bool) { return "", 0, false })
	fo.baseKnown, fo.baseOK, fo.base = true, true, 0x2000
	fo.cu.SetBase(fo.base, true)
	so := &SessionObserver{Labels: NewLabelObserver(), Flight: fo}
	d := newSpDriver(so)
	const frame = 500
	for s := uint64(100); s <= frame; s++ {
		d.step(s, spOutside, nil)
	}
	fo.visible, fo.lastSig = []FlightText{{Key: "t2"}}, frame // 回掃升格
	d.f.Force = true                                          // session：回掃之後 Force
	due := uint64(frame) + SigCheckSteps
	for s := uint64(frame + 1); s <= due+10; s++ {
		d.step(s, spOutside, nil)
	}
	logs, _ := fo.Log()
	if len(fo.visible) != 0 || fo.baseOK || !spHasNote(logs, "sig", due) {
		t.Fatalf("T2：可見 %d、baseOK %t、紀錄 %v（期限 %d）", len(fo.visible), fo.baseOK, logs, due)
	}
}

// T3 Presenter：開 blit 窗後不返回；WindowTimeout 紀錄的步數等於 entry + 200,000。
func TestSparseT3PresenterTimeout(t *testing.T) {
	so := spFull(t)
	p := so.present
	d := newSpDriver(so)
	const entry = 1000
	d.step(entry, spOutside, func() { p.entry(&d.v, &p.routines[0]) }) // 回合第一步（Force）開窗
	if len(p.stack) != 1 {
		t.Fatalf("窗沒開：%d", len(p.stack))
	}
	due := uint64(entry) + p.routines[0].timeout
	for s := uint64(entry + 1); s <= due+10; s++ {
		d.step(s, spOutside, nil)
	}
	var got []uint64
	for _, w := range p.windows {
		if w.Event == "timeout" {
			got = append(got, w.Steps)
		}
	}
	if p.counts.WindowTimeout != 1 || len(got) != 1 || got[0] != due {
		t.Fatalf("T3：WindowTimeout %d、紀錄 %v，要第 %d 步", p.counts.WindowTimeout, got, due)
	}
}

// T4 Subtitles：SBDMA complete 使字幕條進入等待；LastDue+T 的回掃看到仍在等待，期限步之後的回掃看到結束。
func TestSparseT4SubtitleTimeout(t *testing.T) {
	su := &Subtitles{T: SubtitleDefaultT}
	so := &SessionObserver{Labels: NewLabelObserver(), Subtitles: su}
	d := newSpDriver(so)
	const due0 = 2000
	e := machine.SBDMAEvent{Kind: "complete", Step: due0, Due: due0, Clock: "steps", Command: 0x14, Addr: 0x100, Count: 10, Length: 11, Samples: 11}
	c := &Subtitle{Key: "t4", Start: 10, State: SubShowing, LastDue: due0, id: idOf(e), Acc: 1, s: 2}
	su.Subs = []*Subtitle{c}
	for s := uint64(1000); s < due0; s++ {
		d.step(s, spOutside, nil)
	}
	su.SBDMA(e) // 回呼在步驟嘗試之內
	d.f.Force = true
	if c.State != SubWaiting {
		t.Fatalf("complete 之後 State=%v", c.State)
	}
	// 期限步是 LastDue+T+1（Observe 的條件 Steps() > LastDue+T）。期限前的回掃放在期限步的前兩步：它之後的 Force
	// 只喚醒期限前一步，期限步要靠 NextWake；期限步那一次步驟嘗試內的回掃看到結束。
	last := c.LastDue + su.T
	idx := make([]byte, 64000)
	var cur1, cur2 *Subtitle
	var ok1, ok2 bool
	for s := uint64(due0); s <= last+5; s++ {
		d.step(s, spOutside, nil)
		switch s {
		case last - 1:
			su.Frame(idx, s)
			d.f.Force = true
			cur1, ok1 = su.current()
		case last + 1:
			su.Frame(idx, s)
			d.f.Force = true
			cur2, ok2 = su.current()
		}
	}
	if !ok1 || cur1 != c {
		t.Fatalf("T4：期限前的回掃 current=%v,%t，要等待中的字幕條", cur1, ok1)
	}
	if ok2 || c.State != SubEnded || c.Reason != ReasonTimeout || c.End != c.LastDue {
		t.Fatalf("T4：期限步之後的回掃 current=%v,%t；State=%v Reason=%s End=%d", cur2, ok2, c.State, c.Reason, c.End)
	}
}

// §5.5：TriggerIPs、NextWake、FrameAt 不配置。
func TestSparseObserverAllocs(t *testing.T) {
	so := spFull(t)
	var set session.IPSet
	if n := testing.AllocsPerRun(1000, func() { set = so.TriggerIPs() }); n != 0 {
		t.Errorf("TriggerIPs 每次配置 %.1f 次", n)
	}
	_ = set
	if n := testing.AllocsPerRun(1000, func() { _ = so.NextWake() }); n != 0 {
		t.Errorf("NextWake 每次配置 %.1f 次", n)
	}
	if n := testing.AllocsPerRun(1000, func() { so.FrameAt(123) }); n != 0 {
		t.Errorf("FrameAt 每次配置 %.1f 次", n)
	}
}

// §3.4：TriggerIPs 只含已掛的觀察器；呈現層以實際窗表為準。
func TestSparseTriggerIPsAttached(t *testing.T) {
	so := &SessionObserver{Labels: NewLabelObserver()}
	set := so.TriggerIPs()
	for _, ip := range labelTriggerIPs {
		if !set.Has(ip) {
			t.Errorf("缺標籤 IP %04X", ip)
		}
	}
	for _, ip := range append(flightTriggerIPs[:], subtitleTriggerIPs[:]...) {
		if set.Has(ip) {
			t.Errorf("未掛飛行、字幕卻含 %04X", ip)
		}
	}
	p, err := NewPresenter(PresenterConfig{Fonts: &TextFonts{}, routines: []presRoutine{{name: "x", entryIP: 0x4321, retIP: 0x4325, timeout: 10}}})
	if err != nil {
		t.Fatal(err)
	}
	so.present = p
	set = so.TriggerIPs()
	if !set.Has(0x4321) || !set.Has(0x4325) {
		t.Error("注入窗表的 IP 不在集合內")
	}
	if session.IPSet(p.ipMask) != spMask(0x4321, 0x4325) {
		t.Error("ipMask 與窗表不符")
	}
	full := spFull(t).TriggerIPs()
	n := 0
	for _, w := range full {
		for ; w != 0; w &= w - 1 {
			n++
		}
	}
	if n != 19 {
		t.Errorf("全掛時集合有 %d 個 IP，§2.1 是 19 個", n)
	}
}

func spMask(ips ...uint16) session.IPSet {
	var s session.IPSet
	for _, ip := range ips {
		s.Add(ip)
	}
	return s
}

// §4 O2：以 go/types 取各檔 switch v.IP() 的 case 常數值，與 IP 表雙向比對。
func TestSparseO2SwitchCasesMatchTables(t *testing.T) {
	fset := token.NewFileSet()
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	byName := map[string]*ast.File{}
	for _, m := range matches {
		if strings.HasSuffix(m, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, m, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		byName[m] = f
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	conf := types.Config{Error: func(error) {}, Importer: spNoImporter{}}
	conf.Check("strikecommander", fset, files, info) // 匯入失敗照樣檢查套件內常數
	cases := func(name string) []uint16 {
		var out []uint16
		ast.Inspect(byName[name], func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			call, ok := sw.Tag.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "IP" {
				return true
			}
			for _, st := range sw.Body.List {
				for _, e := range st.(*ast.CaseClause).List {
					tv, ok := info.Types[e]
					if !ok || tv.Value == nil {
						t.Fatalf("%s：case %s 不是常數", name, types.ExprString(e))
					}
					v, _ := constant.Uint64Val(tv.Value)
					out = append(out, uint16(v))
				}
			}
			return true
		})
		if len(out) == 0 {
			t.Fatalf("%s：找不到 switch v.IP()", name)
		}
		return out
	}
	same := func(name string, got []uint16, table []uint16) {
		a := append([]uint16(nil), got...)
		b := append([]uint16(nil), table...)
		sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
		sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })
		if len(a) != len(b) {
			t.Errorf("%s：case %04X，表 %04X", name, a, b)
			return
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("%s：case %04X，表 %04X", name, a, b)
				return
			}
		}
	}
	same("observer.go", cases("observer.go"), labelTriggerIPs[:])
	same("flight.go", cases("flight.go"), flightTriggerIPs[:])
	same("subtitle.go", cases("subtitle.go"), subtitleTriggerIPs[:])
	// cutracker 的 case 必須全在飛行表內（它由 flight.go 的 CU case 轉入）。
	set := spMask(flightTriggerIPs[:]...)
	for _, ip := range cases("cutracker.go") {
		if !set.Has(ip) {
			t.Errorf("cutracker.go 的 case %04X 不在飛行表內", ip)
		}
	}
}

type spNoImporter struct{}

func (spNoImporter) Import(path string) (*types.Package, error) {
	return nil, os.ErrNotExist
}

// §3.6：編譯期斷言在 sparse.go；這裡確認 RunReceipt 交給 session 的觀察器只有 *SessionObserver 與 presentObserver
// （receipt.go 的 obs 指定），census 的觀察器不實作 SparseObserver 的方法、也不嵌入 SessionObserver。
func TestSparseObserverTypes(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "receipt.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var rhs []string
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, l := range as.Lhs {
			if id, ok := l.(*ast.Ident); ok && id.Name == "obs" && i < len(as.Rhs) {
				rhs = append(rhs, types.ExprString(as.Rhs[i]))
			}
		}
		return true
	})
	sort.Strings(rhs)
	if strings.Join(rhs, ",") != "presentObserver{…},so" {
		t.Errorf("RunReceipt 的 obs 指定：%v", rhs)
	}
	var _ session.SparseObserver = (*SessionObserver)(nil)
	var _ session.SparseObserver = presentObserver{}
	cf, err := parser.ParseFile(fset, filepath.Join("..", "..", "cmd", "strikecommander-census", "main.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range cf.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv != nil {
			switch fd.Name.Name {
			case "TriggerIPs", "NextWake", "FrameAt":
				t.Errorf("census 有方法 %s", fd.Name.Name)
			}
		}
	}
	ast.Inspect(cf, func(n ast.Node) bool {
		if st, ok := n.(*ast.StructType); ok {
			for _, fl := range st.Fields.List {
				if len(fl.Names) == 0 && strings.Contains(types.ExprString(fl.Type), "SessionObserver") {
					t.Errorf("census 嵌入 %s", types.ExprString(fl.Type))
				}
			}
		}
		return true
	})
}

// §5.2 第 3 層的深度快照：同一狀態兩次雜湊相同；改一個納入的欄位會報出那個欄位；改簿記欄位不影響。
func TestSparseAuditHash(t *testing.T) {
	so := spFull(t)
	so.present.unattr[[2]uint16{1, 2}] = &PresentUnattr{CS: 1, IP: 2, Count: 3}
	so.present.unattr[[2]uint16{3, 4}] = &PresentUnattr{CS: 3, IP: 4, Count: 5}
	so.Labels.artBadCS = map[uint16]bool{7: true, 9: false}
	names := sparseAuditFieldNames(so)
	h0 := sparseAuditHash(so)
	for i := 0; i < 20; i++ {
		if h := sparseAuditHash(so); !reflect.DeepEqual(h, h0) {
			t.Fatal("同一狀態的雜湊不同（map 走訪順序？）")
		}
	}
	diff := func(a, b []uint64) []string {
		var out []string
		for i := range a {
			if a[i] != b[i] {
				out = append(out, names[i])
			}
		}
		return out
	}
	so.present.steps, so.present.cuOpen = 999, true // 簿記欄位
	if d := diff(h0, sparseAuditHash(so)); len(d) != 0 {
		t.Fatalf("簿記欄位改變被報出：%v", d)
	}
	so.Labels.lastSig++
	so.present.unattr[[2]uint16{1, 2}].Count++
	if d := diff(h0, sparseAuditHash(so)); !reflect.DeepEqual(d, []string{"LabelObserver.lastSig", "Presenter.unattr"}) {
		t.Fatalf("報出的欄位 %v", d)
	}
}
