package strikecommander

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/maphash"
	"maps"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

// 規格 460 §5.2 的稽核。只由測試開啟（sparseAuditOn 等變數由 sparse_env_test.go 依環境變數設定；非測試碼不讀環境變數）。
// session 照逐步模式執行；稽核以影子 StepFilter（同一個 ShouldObserve）判斷每一步在稀疏模式會不會被觀察：
//   - 會觀察的步：照常呼叫觀察器，之後鎖存 NextWake()（與稀疏模式同一時點）。
//   - 略過步：在呼叫觀察器之前以 §2.2 的原始條件檢查（判斷式 oracle）、不變式 W 與 I1，並依取樣在呼叫前後取深度快照雜湊。
// 另把事件步（回掃、SBDMA、模式設定、觸發 IP、期限）以固定種子的分層水庫抽樣留下，供 §5.1 第 4 條的定向切分。

var (
	sparseAuditOn     bool
	sparseAuditSeed   int64        = 460
	sparseAuditWindow uint64       = 64 // 觀察步之後幾步內的略過步全取快照
	sparseAuditRate   float64           // 其餘略過步的隨機取樣率
	sparseAuditEvents = 8               // 每種事件的水庫大小（8 種，合計至多 64）
	sparseAuditSink   func(string)      // 稽核摘要的輸出（測試設定）；nil 時不輸出

	// receiptSplitsOverride 是 ReceiptConfig.splits 為 nil 時的切分點（測試設定，460 §5.1 第 4 條）。
	receiptSplitsOverride []uint64
)

// sparseAuditRawIPs 是 §2.1 表的 19 個 IP（獨立於各觀察器的 IP 表，供判斷式 oracle）：標籤、飛行、字幕三組。
var sparseAuditRawIPs = struct{ label, flight, sub []uint16 }{
	label:  []uint16{0x034B, 0x03D3, 0x02B2, 0x0000, 0x008B, 0x0002},
	flight: []uint16{0x0345, 0x02AC, 0x03CD, 0x0182, 0x01F2, 0x00A5, 0x00D6, 0x00A7, 0x00D8, 0x00EE},
	sub:    []uint16{0x103F, 0x10F3, 0x22BE},
}

type sparseAuditEvent struct {
	kind  string
	steps uint64
}

type sparseAudit struct {
	f        machine.StepFilter // 影子；Exited 為 nil（逐步模式在程式結束後的第一步即停）
	turn     bool               // 回合開始、下一步重讀觸發 IP 集合
	rng      *rand.Rand
	sinceObs uint64

	observed, skipped, forced, hashed uint64
	hashTime                          time.Duration
	fails                             []string
	failN                             int

	evSeen map[string]uint64
	ev     map[string][]uint64
}

func newSparseAudit(s *SessionObserver) *sparseAudit {
	a := &sparseAudit{turn: true, rng: rand.New(rand.NewSource(sparseAuditSeed)), evSeen: map[string]uint64{}, ev: map[string][]uint64{}}
	a.f.Wake = session.NoWake
	return a
}

func (a *sparseAudit) fail(steps uint64, ip uint16, layer, cond string) {
	a.failN++
	if len(a.fails) < 32 {
		a.fails = append(a.fails, fmt.Sprintf("第 %d 步 IP %04X：%s %s", steps, ip, layer, cond))
	}
}

// turnStart：RunReceipt 在每次 Advance 之前呼叫（§3.3 回合第一步的 Force；集合每回合重讀）。
func (a *sparseAudit) turnStart() { a.f.Force, a.turn = true, true }

// event：Frame、SBDMA、模式設定之後的 Force（§3.3），並記事件步與 I1。
func (a *sparseAudit) event(s *SessionObserver, kind string, steps uint64) {
	a.f.Force = true
	if kind != "mode" {
		a.record(kind, steps)
	}
	a.checkI1(s, steps, 0, kind)
}

func (a *sparseAudit) record(kind string, steps uint64) {
	n := a.evSeen[kind]
	a.evSeen[kind] = n + 1
	if n < uint64(sparseAuditEvents) {
		a.ev[kind] = append(a.ev[kind], steps)
		return
	}
	if j := a.rng.Int63n(int64(n + 1)); j < int64(sparseAuditEvents) {
		a.ev[kind][j] = steps
	}
}

// step 取代 BeforeStep（s.audit 非 nil 時）。
func (a *sparseAudit) step(s *SessionObserver, v session.StepView) error {
	if a.turn {
		a.f.IPs, a.turn = s.TriggerIPs(), false // 回合開始時重讀，與 session 相同
	}
	steps, ip := v.Steps(), v.IP()
	if a.f.ShouldObserve(ip, steps) {
		if a.f.Force {
			a.forced++
		}
		a.f.Force = false
		a.observed++
		a.sinceObs = 0
		if a.f.IPs[ip>>6]&(1<<(ip&63)) != 0 {
			a.record(a.ipKind(ip), steps)
		}
		err := s.beforeStep(v)
		w := s.NextWake()
		if w != a.f.Wake && w != session.NoWake && w > steps+1 {
			a.record("wake", w)
		}
		a.f.Wake = w
		a.checkI1(s, steps, ip, "BeforeStep 後")
		return err
	}
	a.skipped++
	a.sinceObs++
	a.oracle(s, steps, ip)
	if fresh := s.NextWake(); steps >= fresh {
		a.fail(steps, ip, "不變式", fmt.Sprintf("W：Steps %d ≥ NextWake %d（鎖存 %d）", steps, fresh, a.f.Wake))
	}
	a.checkI1(s, steps, ip, "略過步之前")
	sample := a.sinceObs <= sparseAuditWindow || (sparseAuditRate > 0 && a.rng.Float64() < sparseAuditRate)
	var h0 []uint64
	var t0 time.Time
	if sample {
		t0 = time.Now()
		h0 = sparseAuditHash(s)
	}
	err := s.beforeStep(v)
	if sample {
		h1 := sparseAuditHash(s)
		a.hashTime += time.Since(t0)
		a.hashed++
		for i := range h0 {
			if h0[i] != h1[i] {
				a.fail(steps, ip, "快照", "欄位 "+sparseAuditFieldNames(s)[i]+" 在略過步被改變")
				break
			}
		}
	}
	a.checkI1(s, steps, ip, "略過步之後")
	return err
}

func (a *sparseAudit) ipKind(ip uint16) string {
	switch ip {
	case 0x00A5, 0x00D6, 0x00A7, 0x00D8, 0x00EE:
		return "ip-cu"
	case 0x0345, 0x02AC, 0x03CD, 0x0182, 0x01F2:
		return "ip-flight"
	case 0x103F, 0x10F3, 0x22BE:
		return "ip-sub"
	}
	return "ip-label"
}

// oracle 是 §5.2 第 1 層：略過步在呼叫觀察器之前，以 §2.2 的原始條件直接檢查觀察器狀態（不經 NextWake）。
func (a *sparseAudit) oracle(s *SessionObserver, steps uint64, ip uint16) {
	has := func(l []uint16) bool {
		for _, x := range l {
			if x == ip {
				return true
			}
		}
		return false
	}
	if has(sparseAuditRawIPs.label) {
		a.fail(steps, ip, "oracle", "IP 在標籤觀察器的 IP 表內")
	}
	lo := s.Labels
	if len(lo.visible) != 0 && lo.baseKnown && steps-lo.lastSig >= SigCheckSteps {
		a.fail(steps, ip, "oracle", "Labels 簽章層到期")
	}
	if fo := s.Flight; fo != nil {
		if has(sparseAuditRawIPs.flight) {
			a.fail(steps, ip, "oracle", "IP 在飛行觀察器的 IP 表內")
		}
		if fo.pendOn {
			a.fail(steps, ip, "oracle", "Flight pendOn")
		}
		if len(fo.visible) != 0 && fo.baseOK && steps-fo.lastSig >= SigCheckSteps {
			a.fail(steps, ip, "oracle", "Flight 簽章層到期")
		}
	}
	if su := s.Subtitles; su != nil {
		if has(sparseAuditRawIPs.sub) {
			a.fail(steps, ip, "oracle", "IP 在字幕觀察器的 IP 表內")
		}
		if len(su.pending) != 0 {
			a.fail(steps, ip, "oracle", "Subtitles pending 非空")
		}
		if n := len(su.Subs); n != 0 {
			if c := su.Subs[n-1]; c.State == SubWaiting && steps > c.LastDue+su.T {
				a.fail(steps, ip, "oracle", "Subtitles 等待逾時")
			}
		}
	}
	if p := s.present; p != nil {
		for i := range p.routines {
			if r := &p.routines[i]; ip == r.entryIP || ip == r.retIP {
				a.fail(steps, ip, "oracle", "IP 在呈現層窗表內")
			}
		}
		if p.recheck != nil {
			a.fail(steps, ip, "oracle", "Presenter recheck")
		}
		if p.afterRet && steps != p.afterRetAt {
			a.fail(steps, ip, "oracle", "Presenter afterRet")
		}
		if p.cuPend {
			a.fail(steps, ip, "oracle", "Presenter cuPend")
		}
		for _, w := range p.stack {
			if steps-w.entry >= w.r.timeout {
				a.fail(steps, ip, "oracle", "Presenter 窗逾時")
				break
			}
		}
	}
}

// checkI1 是不變式 I1：p.cuOpen == flight.CUState().InProgress。
func (a *sparseAudit) checkI1(s *SessionObserver, steps uint64, ip uint16, where string) {
	if s.present == nil || s.Flight == nil {
		return
	}
	if in, _ := s.Flight.CUState(); s.present.cuOpen != in {
		a.fail(steps, ip, "不變式", fmt.Sprintf("I1（%s）：cuOpen %t、CUState %t", where, s.present.cuOpen, in))
	}
}

// finish 輸出摘要；有失敗時回錯誤。
func (a *sparseAudit) finish() error {
	var b strings.Builder
	per := time.Duration(0)
	if a.hashed != 0 {
		per = a.hashTime / time.Duration(a.hashed)
	}
	fmt.Fprintf(&b, "SPARSEAUDIT observed=%d skipped=%d forced=%d hashed=%d hash_avg=%s window=%d rate=%g seed=%d fails=%d\n",
		a.observed, a.skipped, a.forced, a.hashed, per, sparseAuditWindow, sparseAuditRate, sparseAuditSeed, a.failN)
	for _, f := range a.fails {
		fmt.Fprintf(&b, "SPARSEAUDITFAIL %s\n", f)
	}
	kinds := make([]string, 0, len(a.ev))
	for k := range a.ev {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		ev := append([]uint64(nil), a.ev[k]...)
		sort.Slice(ev, func(i, j int) bool { return ev[i] < ev[j] })
		for _, s := range ev {
			fmt.Fprintf(&b, "SPARSEEVENT\t%s\t%d\t%d\n", k, s, a.evSeen[k])
		}
	}
	if sparseAuditSink != nil {
		sparseAuditSink(b.String())
	}
	if a.failN != 0 {
		return fmt.Errorf("strikecommander: 稀疏觀察稽核失敗 %d 處（460 §5.2）：%s", a.failN, strings.Join(a.fails, "；"))
	}
	return nil
}

// ---- 深度快照雜湊（§5.2 第 3 層） ----

// sparseAuditExclude 是排除與只取長度的欄位（型別名.欄位名）。
var sparseAuditExclude = map[string]byte{
	// 簿記欄位（§4 L1）
	"Presenter.steps": 'x', "Presenter.cuOpen": 'x',
	// 不可變設定、圖集、窗表、呈現用實例（呈現用實例只在取像時改變，取像在觀察點）
	"Presenter.cfg": 'x', "Presenter.routines": 'x', "Presenter.ipMask": 'x', "Presenter.ovs": 'x', "Presenter.fovs": 'x',
	"Presenter.labels": 'x', "Presenter.flight": 'x', "Presenter.subs": 'x', "Presenter.onCapture": 'x',
	"Subtitles.Index": 'x', "Subtitles.Catalog": 'x', "Subtitles.Text": 'x',
	"FlightObserver.resolve": 'x', "FlightObserver.cuh": 'x',
	// 暫存緩衝
	"Presenter.vga": 'x', "Presenter.tmp": 'x', "Presenter.bufC": 'x',
	// 輸出累積只取長度
	"LabelObserver.log": 'n', "FlightObserver.log": 'n', "FlightObserver.clipped": 'n', "FlightObserver.redeliv": 'n',
	"Presenter.captures": 'n', "Presenter.stale": 'n', "Presenter.windows": 'n', "Presenter.cuRecs": 'n', "Presenter.cache": 'n',
	"Subtitles.Skips": 'n',
	// SHP 快取：原始檔與目錄不可變，只取長度（字形表照走）
	"flightShp.raw": 'n', "flightShp.ents": 'n',
	// 呈現像的色號不可變（同一張呈現像不再改），以 info 代表
	"presentImage.indexed": 'n',
}

func sparseAuditRoots(s *SessionObserver) []reflect.Value {
	r := []reflect.Value{reflect.ValueOf(s.Labels).Elem()}
	if s.Flight != nil {
		r = append(r, reflect.ValueOf(s.Flight).Elem())
	}
	if s.Subtitles != nil {
		r = append(r, reflect.ValueOf(s.Subtitles).Elem())
	}
	if s.present != nil {
		r = append(r, reflect.ValueOf(s.present).Elem())
	}
	return r
}

// sparseAuditFieldNames 與 sparseAuditHash 的輸出平行（每個根的每個頂層欄位一格）。
func sparseAuditFieldNames(s *SessionObserver) []string {
	var out []string
	for _, r := range sparseAuditRoots(s) {
		t := r.Type()
		for i := 0; i < t.NumField(); i++ {
			out = append(out, t.Name()+"."+t.Field(i).Name)
		}
	}
	return out
}

// sparseWalker 是一次快照的走訪狀態。visited：根以負號編號，其他指標第二次遇到時只寫常數（走訪順序固定）；
// map 的走訪順序不固定，每一筆另用只含根的新集合（roots 的複本），指標別名的編號才不隨順序改變。
type sparseWalker struct {
	visited map[uintptr]int
	roots   map[uintptr]int
	buf     [8]byte
}

// sparseFieldMode 依型別快取每個欄位的排除方式（0 照走、'x' 排除、'n' 只取長度）。
var sparseFieldMode = map[reflect.Type][]byte{}

func sparseModes(t reflect.Type) []byte {
	if m, ok := sparseFieldMode[t]; ok {
		return m
	}
	m := make([]byte, t.NumField())
	for i := range m {
		m[i] = sparseAuditExclude[t.Name()+"."+t.Field(i).Name]
	}
	sparseFieldMode[t] = m
	return m
}

// sparsePlain 回型別是否為不含指標、沒有填充的數值（bool、整數、浮點及其陣列），可直接以原始位元組雜湊。
func sparsePlain(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64:
		return true
	case reflect.Array:
		return sparsePlain(t.Elem())
	}
	return false
}

// sparseAuditHash 回每個根的每個頂層欄位的雜湊（Subtitles.Subs 只取長度與最後一條）。
func sparseAuditHash(s *SessionObserver) []uint64 {
	var out []uint64
	w := &sparseWalker{visited: map[uintptr]int{}, roots: map[uintptr]int{}}
	roots := sparseAuditRoots(s)
	for _, r := range roots {
		id := -(len(w.roots) + 1)
		w.roots[r.Addr().Pointer()] = id
		w.visited[r.Addr().Pointer()] = id
	}
	h := sparseNewHash()
	for _, r := range roots {
		t := r.Type()
		modes := sparseModes(t)
		for i := 0; i < t.NumField(); i++ {
			h.Reset()
			fv := r.Field(i)
			switch {
			case t.Name() == "Subtitles" && t.Field(i).Name == "Subs":
				n := fv.Len()
				w.u64(h, uint64(n))
				if n != 0 {
					w.walk(h, fv.Index(n-1))
				}
			case modes[i] == 'x':
			case modes[i] == 'n':
				w.u64(h, uint64(fv.Len()))
			default:
				w.walk(h, fv)
			}
			out = append(out, h.Sum64())
		}
	}
	return out
}

func (w *sparseWalker) u64(h hash.Hash64, x uint64) {
	binary.LittleEndian.PutUint64(w.buf[:], x)
	h.Write(w.buf[:])
}

func (w *sparseWalker) walk(h hash.Hash64, v reflect.Value) {
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			w.u64(h, 1)
		} else {
			w.u64(h, 0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		w.u64(h, uint64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		w.u64(h, v.Uint())
	case reflect.Float32, reflect.Float64:
		w.u64(h, math.Float64bits(v.Float()))
	case reflect.String:
		w.u64(h, uint64(v.Len()))
		h.Write([]byte(v.String()))
	case reflect.Slice:
		if v.IsNil() {
			w.u64(h, ^uint64(0))
			return
		}
		n := v.Len()
		w.u64(h, uint64(n))
		if et := v.Type().Elem(); sparsePlain(et) {
			if n != 0 {
				h.Write(unsafe.Slice((*byte)(v.UnsafePointer()), uintptr(n)*et.Size()))
			}
			return
		}
		for i := 0; i < n; i++ {
			w.walk(h, v.Index(i))
		}
	case reflect.Array:
		w.u64(h, uint64(v.Len()))
		if sparsePlain(v.Type().Elem()) && v.CanAddr() {
			h.Write(unsafe.Slice((*byte)(unsafe.Pointer(v.UnsafeAddr())), v.Type().Size()))
			return
		}
		for i := 0; i < v.Len(); i++ {
			w.walk(h, v.Index(i))
		}
	case reflect.Struct:
		modes := sparseModes(v.Type())
		for i, m := range modes {
			switch m {
			case 'x':
				continue
			case 'n':
				w.u64(h, uint64(v.Field(i).Len()))
				continue
			}
			w.walk(h, v.Field(i))
		}
	case reflect.Pointer:
		if v.IsNil() {
			w.u64(h, 0)
			return
		}
		p := v.Pointer()
		if id, ok := w.visited[p]; ok {
			if id < 0 {
				w.u64(h, uint64(-id))
			} else {
				w.u64(h, 1<<32)
			}
			return
		}
		w.visited[p] = 1
		w.u64(h, 1)
		w.walk(h, v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			w.u64(h, 0)
			return
		}
		h.Write([]byte(v.Elem().Type().String()))
		w.walk(h, v.Elem())
	case reflect.Map:
		if v.IsNil() {
			w.u64(h, ^uint64(0))
			return
		}
		type kv struct{ k, v uint64 }
		ents := make([]kv, 0, v.Len())
		saved := w.visited
		it := v.MapRange()
		for it.Next() {
			w.visited = maps.Clone(w.roots)
			hk, hv := sparseNewHash(), sparseNewHash()
			w.walk(hk, it.Key())
			w.walk(hv, it.Value())
			ents = append(ents, kv{hk.Sum64(), hv.Sum64()})
		}
		w.visited = saved
		sort.Slice(ents, func(i, j int) bool { return ents[i].k < ents[j].k || ents[i].k == ents[j].k && ents[i].v < ents[j].v })
		w.u64(h, uint64(len(ents)))
		for _, e := range ents {
			w.u64(h, e.k)
			w.u64(h, e.v)
		}
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		// 不比
	}
}

// sparseHashSeed 是一次執行內固定的種子：快照雜湊只在同一個行程內比較（略過步前後），不跨執行。
var sparseHashSeed = maphash.MakeSeed()

func sparseNewHash() *maphash.Hash {
	var h maphash.Hash
	h.SetSeed(sparseHashSeed)
	return &h
}
