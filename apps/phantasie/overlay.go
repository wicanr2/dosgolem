package phantasie

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/wicanr2/dosgolem/xlate"
)

// 疊字核心（docs/spec/001 §4、§7，004 §4、§5）。純資料：不碰機器，鉤子（capture.go、screenops.go）把擷取到的資料送進來。

const (
	screenW = 320
	screenH = 200
	// MaxRecords 是 records 的上限（004 §5）。
	MaxRecords = 4096
	// MaxLog 是稽核事件日誌的上限（005 §5.1）。
	MaxLog = 4096
	// gcEvery 是每多少個提交的事件掃描一次 records（004 §5）。
	gcEvery = 256
)

// Language 是一個語言通道（004 §4）。
type Language struct {
	Name    string
	Cat     Lookup
	Font    *xlate.Font
	Wide    WideFunc
	Enabled bool
	Err     string // 停用原因
}

// LogEvent 是稽核事件日誌的一筆（005 §5.1）：T 類且提交時 Resolve 回 OK 的事件。
type LogEvent struct {
	ID       string
	Col, Row int
	Text     string
	Step     uint64
}

type eventIdentity struct {
	sp, bp, caller uint16
	col, row       int
	fmtPtr         uint16
	textHash       uint64
}

type openEvent struct {
	rec  *EventRecord
	skip bool // badlen、truncated_input：開啟並配對，但提交時不動 Layer
	id   eventIdentity
}

// Overlay 維護疊字層與事件記錄。單執行緒：Frame 與鉤子在同一個 goroutine。
type Overlay struct {
	Layer  *xlate.Layer
	C      *Counters
	Faults map[string]bool // 測試用故障注入：noadd、noclear（005 §5.1）
	// AuditDebug 非 nil 時，稽核把每個殘字格與外露事件的細節送給它（診斷用，不影響結果）。
	AuditDebug func(string)
	// CommitHook 非 nil 時，T 類事件的疊字加入 Layer 之後呼叫（位置 oracle 用，不影響結果）。
	CommitHook func(rec *EventRecord, stamps []*xlate.Stamp)

	langs      map[string]*Language
	langOrder  []string
	display    string
	shadowLang string

	records map[string]*EventRecord
	recSeq  []string
	hits    map[string][]string
	nextID  int
	sinceGC int

	open          *openEvent
	pendingSwitch bool

	font []byte // 原版 FONT 2032 bytes（001 §8）；nil 時不做字模遮罩
	gate map[*xlate.Stamp]struct{}
	sh   *shadowStore
	log  []LogEvent
}

// NewOverlay 建立空的疊字核心。
func NewOverlay() *Overlay {
	l := &xlate.Layer{W: screenW, H: screenH, FontRegistry: map[string]*xlate.Font{}}
	return &Overlay{
		Layer:   l,
		C:       NewCounters(),
		Faults:  map[string]bool{},
		langs:   map[string]*Language{},
		records: map[string]*EventRecord{},
		hits:    map[string][]string{},
		gate:    map[*xlate.Stamp]struct{}{},
		sh:      newShadowStore(),
	}
}

// SetFont 設定原版 FONT（2032 bytes，001 §8）。長度不符視為沒有字模，recolor 退回 xlate 的定色。
func (o *Overlay) SetFont(b []byte) {
	if len(b) != 2032 {
		o.font = nil
		return
	}
	o.font = append([]byte(nil), b...)
}

// AddLanguage 註冊語言通道。Font.Name 必須非空（001 §7），否則停用該語言。
func (o *Overlay) AddLanguage(l *Language) {
	if l.Enabled {
		switch {
		case l.Cat == nil:
			l.Enabled, l.Err = false, "沒有 catalog"
		case l.Font == nil || l.Font.Name == "":
			l.Enabled, l.Err = false, "字型缺少名稱"
		case l.Wide == nil:
			l.Enabled, l.Err = false, "沒有字型寬度表"
		}
	}
	if l.Enabled {
		o.Layer.FontRegistry[l.Font.Name] = l.Font
	}
	if _, ok := o.langs[l.Name]; !ok {
		o.langOrder = append(o.langOrder, l.Name)
	}
	o.langs[l.Name] = l
	if o.shadowLang == "" && l.Enabled && l.Name != "en" {
		o.shadowLang = l.Name
	}
}

// Language 回語言通道（沒有回 nil）。
func (o *Overlay) Language(name string) *Language { return o.langs[name] }

// Languages 回已註冊語言的順序。
func (o *Overlay) Languages() []string { return append([]string(nil), o.langOrder...) }

// Display 回顯示語言；ShadowLang 回事件解析使用的語言。
func (o *Overlay) Display() string    { return o.display }
func (o *Overlay) ShadowLang() string { return o.shadowLang }

// SetDisplay 切換顯示語言（004 §5）。事件中途延到 B 之後執行重建。語言不存在或停用回錯誤。
func (o *Overlay) SetDisplay(name string) error {
	l := o.langs[name]
	if name != "en" && (l == nil || !l.Enabled) {
		return fmt.Errorf("語言 %s 未啟用", name)
	}
	o.display = name
	if name != "en" {
		o.shadowLang = name
	}
	if o.open != nil {
		o.pendingSwitch = true
		return nil
	}
	o.rebuild()
	return nil
}

// Drawing 回目前是否要呼叫 Layer.Draw（顯示語言不是 en 且已設定）。
func (o *Overlay) Drawing() bool { return o.display != "" && o.display != "en" }

// Hits 回某事件記錄的 catalog 鍵（001 §7）。
func (o *Overlay) Hits(id string) []string { return o.hits[id] }

// Record 回事件記錄。
func (o *Overlay) Record(id string) *EventRecord { return o.records[id] }

// Log 回稽核事件日誌（最舊在前）。
func (o *Overlay) Log() []LogEvent { return o.log }

// ---- 事件開啟與關閉（001 §3.2） ----

func fnv64(b []byte) uint64 {
	h := uint64(14695981039346656037)
	for _, v := range b {
		h ^= uint64(v)
		h *= 1099511628211
	}
	return h
}

func identityOf(r *EventRecord) eventIdentity {
	return eventIdentity{r.SP, r.BP, r.Caller, r.Col, r.Row, r.FmtPtr, fnv64([]byte(r.Text))}
}

// Begin 是 A：開啟事件。已有開啟中的事件且識別相同視為重入（dup_open）；不同則上一個事件缺 B（unpaired），
// 先以已擷取的資料提交。skip 為 true 的事件（badlen、truncated_input）照常開啟與配對，提交時不動 Layer。
// 回傳 false 表示重入（dup_open），沒有開啟新事件。
func (o *Overlay) Begin(rec *EventRecord, skip bool) (opened bool) {
	id := identityOf(rec)
	if o.open != nil {
		if o.open.id == id {
			o.C.Inc("dup_open")
			return false
		}
		o.C.Inc("unpaired")
		o.finish()
	}
	o.nextID++
	rec.ID = "g" + strconv.Itoa(o.nextID)
	o.open = &openEvent{rec: rec, skip: skip, id: id}
	o.C.Inc("events")
	return true
}

// End 是 B：關閉並提交事件。沒有開啟中的事件是重複的 B（dup_close）。
func (o *Overlay) End() {
	if o.open == nil {
		o.C.Inc("dup_close")
		return
	}
	o.finish()
}

// Open 回是否有開啟中的事件，與其矩形（供稽核抽樣略過）。
func (o *Overlay) Open() (rec *EventRecord, ok bool) {
	if o.open == nil {
		return nil, false
	}
	return o.open.rec, true
}

func (o *Overlay) finish() {
	ev := o.open
	o.open = nil
	o.commit(ev)
	if o.pendingSwitch {
		o.pendingSwitch = false
		o.rebuild()
	}
}

// ---- 提交（001 §4、§7） ----

func isSpaces(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			return false
		}
	}
	return true
}

func nonPrintable(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7E {
			return true
		}
	}
	return false
}

// rectOf 回事件矩形（像素，已裁切到畫布）；Col+len > 40 或 Row ≥ 25 時 clipped 為真。
func rectOf(col, row, n int) (x0, y0, x1, y1 int, clipped bool) {
	x0, y0 = col*8, row*8
	x1, y1 = x0+n*8, y0+8
	if x1 > screenW || y1 > screenH {
		clipped = true
	}
	if x1 > screenW {
		x1 = screenW
	}
	if y1 > screenH {
		y1 = screenH
	}
	return
}

func (o *Overlay) clearRect(x0, y0, x1, y1 int) {
	if o.Faults["noclear"] {
		return
	}
	o.Layer.Clear(x0, y0, x1, y1)
}

func (o *Overlay) commit(ev *openEvent) {
	rec := ev.rec
	if ev.skip || o.shadowLang == "" || o.langs[o.shadowLang] == nil {
		return
	}
	t := rec.Text
	if len(t) == 0 {
		return
	}
	x0, y0, x1, y1, clipped := rectOf(rec.Col, rec.Row, len(t))
	if clipped {
		o.C.Inc("clipped")
	}
	switch {
	case isSpaces(t):
		o.clearRect(x0, y0, x1, y1)
		o.C.Inc("blank")
	case nonPrintable(t):
		o.clearRect(x0, y0, x1, y1)
		o.C.Inc("nonprintable")
		for i := 0; i < len(t); i++ {
			if t[i] < 0x20 || t[i] > 0x7E {
				o.C.Key("nonprintable", fmt.Sprintf("%02X", t[i]))
			}
		}
	default:
		if o.tryPatch(rec, x0, y0, x1, y1) {
			return
		}
		o.commitText(rec, x0, y0, x1, y1)
	}
}

// callerKey 是缺譯與診斷用的位置鍵：呼叫端與格式字串（格式字串不是靜態字串時不寫內容，避免放進玩家輸入）。
func callerKey(rec *EventRecord) string {
	if rec.FmtKind == KindStatic {
		return fmt.Sprintf("%04X|%s", rec.Caller, rec.Format)
	}
	return fmt.Sprintf("%04X|<%s>", rec.Caller, rec.FmtKind)
}

func (o *Overlay) commitText(rec *EventRecord, x0, y0, x1, y1 int) {
	lang := o.langs[o.shadowLang]
	res := (&Resolver{Cat: lang.Cat, Wide: lang.Wide}).Resolve(rec)
	if len(res.Missed) > 0 {
		o.C.Inc("untranslated_args")
		for _, k := range res.Missed {
			o.C.Key("untranslated_args", k)
		}
		o.C.Key("untranslated_args_at", callerKey(rec))
	}
	if !res.OK {
		switch res.Why {
		case WhyPassthrough:
			o.C.Inc("passthrough")
		case WhyProtected:
			o.C.Inc("protected")
		default:
			o.C.Inc("untranslated")
			if res.Key != "" {
				o.C.Key("untranslated", res.Key)
			}
			o.C.Key("untranslated_at", callerKey(rec))
			switch res.Why {
			case WhyBadFormat:
				o.C.Inc("badformat")
			case WhyBadArg:
				o.C.Inc("badarg")
			}
		}
		o.clearRect(x0, y0, x1, y1)
		return
	}
	line, truncated, err := layoutLine(res.Zh, res.Center, availH(rec.Col, len(rec.Text)), lang.Wide)
	if err != nil {
		o.C.Inc("untranslated")
		o.C.Inc("badformat")
		o.C.Key("untranslated_at", callerKey(rec))
		o.clearRect(x0, y0, x1, y1)
		return
	}
	if truncated {
		o.C.Inc("truncated")
		for _, k := range res.Hits {
			o.C.Key("truncated", k)
		}
	}
	if o.Faults["noadd"] {
		return
	}
	stamps := buildStamps(rec.ID, x0, y0, line, lang.Wide, lang.Font, nil)
	for _, s := range stamps {
		o.Layer.Add(s)
	}
	o.store(rec, res.Hits)
	o.C.Inc("translated")
	o.appendLog(rec)
	if o.CommitHook != nil {
		o.CommitHook(rec, stamps)
	}
}

func (o *Overlay) appendLog(rec *EventRecord) {
	o.log = append(o.log, LogEvent{ID: rec.ID, Col: rec.Col, Row: rec.Row, Text: rec.Text, Step: rec.Step})
	if len(o.log) > MaxLog {
		o.log = append(o.log[:0], o.log[len(o.log)-MaxLog:]...)
	}
}

// store 保存事件記錄與命中鍵；每 gcEvery 個事件掃描一次。
func (o *Overlay) store(rec *EventRecord, hits []string) {
	if _, ok := o.records[rec.ID]; !ok {
		o.recSeq = append(o.recSeq, rec.ID)
	}
	o.records[rec.ID] = rec
	o.hits[rec.ID] = hits
	o.sinceGC++
	if o.sinceGC >= gcEvery {
		o.gcRecords()
	}
}

// ---- 事件組（同 Key 的疊字） ----

func (o *Overlay) groupStamps(key string) []*xlate.Stamp {
	var out []*xlate.Stamp
	for _, s := range o.Layer.Stamps {
		if s.Key == key {
			out = append(out, s)
		}
	}
	return out
}

// groupKeys 回 Layer 內事件組的 Key，依各組第一筆疊字的出現順序。
func (o *Overlay) groupKeys() []string {
	seen := map[string]bool{}
	var keys []string
	for _, s := range o.Layer.Stamps {
		if !seen[s.Key] {
			seen[s.Key] = true
			keys = append(keys, s.Key)
		}
	}
	return keys
}

// groupDY 回組內疊字的 Y 相對 Row×8 的位移（001 §8、002 §4）；各疊字不一致或沒有疊字回 ok=false。
func groupDY(stamps []*xlate.Stamp, rec *EventRecord) (dy int, ok bool) {
	if len(stamps) == 0 {
		return 0, false
	}
	dy = stamps[0].Y - rec.Row*8
	for _, s := range stamps[1:] {
		if s.Y-rec.Row*8 != dy {
			return 0, false
		}
	}
	return dy, true
}

// visibleRanges 回組內非透明格的像素 x 範圍（已排序合併）。
func visibleRanges(stamps []*xlate.Stamp) []xrange {
	var rs []xrange
	for _, s := range stamps {
		for i := 0; i < s.Cells; i++ {
			if i < len(s.Transparent) && s.Transparent[i] {
				continue
			}
			rs = append(rs, xrange{s.X + i*s.CellW, s.X + (i+1)*s.CellW})
		}
	}
	return mergeRanges(rs)
}

// hiddenOf 是事件矩形的 x 範圍減去組內可見範圍的補集（002 §4）。影子登記、影子還原與語言切換共用。
func hiddenOf(rec *EventRecord, stamps []*xlate.Stamp) []xrange {
	x0 := rec.Col * 8
	x1 := x0 + len(rec.Text)*8
	if x1 > screenW {
		x1 = screenW
	}
	if x0 >= x1 {
		return nil
	}
	var out []xrange
	cur := x0
	for _, v := range visibleRanges(stamps) {
		if v.X1 <= cur {
			continue
		}
		if v.X0 >= x1 {
			break
		}
		if v.X0 > cur {
			out = append(out, xrange{cur, v.X0})
		}
		if v.X1 > cur {
			cur = v.X1
		}
	}
	if cur < x1 {
		out = append(out, xrange{cur, x1})
	}
	return out
}

func (o *Overlay) removeGroup(key string) {
	keep := make([]*xlate.Stamp, 0, len(o.Layer.Stamps))
	for _, s := range o.Layer.Stamps {
		if s.Key != key {
			keep = append(keep, s)
		}
	}
	o.Layer.Stamps = keep
}

// replaceGroup 以新疊字取代舊組：新疊字插在舊組第一筆的位置（保留疊序），不 Clear（不使用 Layer.Replace，004 §5）。
func (o *Overlay) replaceGroup(oldKey string, news []*xlate.Stamp) {
	out := make([]*xlate.Stamp, 0, len(o.Layer.Stamps)+len(news))
	inserted := false
	for _, s := range o.Layer.Stamps {
		if s.Key == oldKey {
			if !inserted {
				out = append(out, news...)
				inserted = true
			}
			continue
		}
		out = append(out, s)
	}
	if !inserted {
		out = append(out, news...)
	}
	o.Layer.Stamps = out
}

// ---- P 類：符號修補（001 §4） ----

// tryPatch 處理選項開關符號的單字元覆寫；不符合 P 類前提時回 false，事件走一般流程。
func (o *Overlay) tryPatch(rec *EventRecord, x0, y0, x1, y1 int) bool {
	if len(rec.Text) != 1 || (rec.Text[0] != '+' && rec.Text[0] != '-') {
		return false
	}
	var found *xlate.Stamp
	for i := len(o.Layer.Stamps) - 1; i >= 0; i-- {
		s := o.Layer.Stamps[i]
		if y0 < s.Y || y0 >= s.Y+s.CellH || x0 < s.X || x0 >= s.X+s.Cells*s.CellW {
			continue
		}
		if c := (x0 - s.X) / s.CellW; c < len(s.Transparent) && s.Transparent[c] {
			continue
		}
		found = s
		break
	}
	if found == nil {
		return false
	}
	old := o.records[found.Key]
	if old == nil {
		return false
	}
	stamps := o.groupStamps(found.Key)
	dy, ok := groupDY(stamps, old)
	if !ok {
		o.removeGroup(found.Key)
		o.C.Inc("rebuild_lost")
		return false
	}
	if old.Row*8+dy != y0 {
		return false
	}
	k := rec.Col - old.Col
	if k < 0 || k >= len(old.Text) || (old.Text[k] != '+' && old.Text[k] != '-') {
		return false
	}
	lang := o.langs[o.shadowLang]
	segs, err := ParseFormat(old.Format)
	if err != nil || old.Composed != nil {
		o.C.Inc("patch_fallback")
		o.clearRect(x0, y0, x1, y1)
		return true
	}
	identity := len(segs) == 1 && segs[0].Conv != nil && segs[0].Conv.Type == 's' &&
		segs[0].Conv.Width == 0 && segs[0].Conv.Prec < 0 &&
		len(old.ArgStrs) == 1 && len(old.ArgStrs[0].Content) == len(old.Text) && old.ArgStrs[0].Piece == nil
	literal := !HasConversions(segs) && old.Format == old.Text
	if !identity && !literal {
		o.C.Inc("patch_fallback")
		o.clearRect(x0, y0, x1, y1)
		return true
	}
	nw := *old
	nw.Cells = append([]byte(nil), old.Cells...)
	nw.Cells[k] = rec.Text[0]
	nw.Text = patchByte(old.Text, k, rec.Text[0])
	if identity {
		nw.ArgStrs = append([]ArgStr(nil), old.ArgStrs...)
		nw.ArgStrs[0].Content = patchByte(old.ArgStrs[0].Content, k, rec.Text[0])
	} else {
		nw.Format = patchByte(old.Format, k, rec.Text[0])
	}
	nw.ID = rec.ID
	nw.Step = rec.Step
	res := (&Resolver{Cat: lang.Cat, Wide: lang.Wide}).Resolve(&nw)
	if !res.OK {
		o.C.Inc("untranslated")
		if res.Key != "" {
			o.C.Key("untranslated", res.Key)
		}
		o.C.Key("untranslated_at", callerKey(&nw))
		o.clearRect(x0, y0, x1, y1)
		return true
	}
	line, _, err := layoutLine(res.Zh, res.Center, availH(old.Col, len(old.Text)), lang.Wide)
	if err != nil {
		o.C.Inc("untranslated")
		o.C.Inc("badformat")
		o.clearRect(x0, y0, x1, y1)
		return true
	}
	hidden := hiddenOf(old, stamps)
	news := buildStamps(nw.ID, old.Col*8, old.Row*8+dy, line, lang.Wide, lang.Font, hidden)
	o.replaceGroup(found.Key, news)
	o.store(&nw, res.Hits)
	o.C.Inc("patched")
	o.appendLog(&nw)
	return true
}

func patchByte(s string, i int, c byte) string {
	b := []byte(s)
	b[i] = c
	return string(b)
}

// ---- 語言切換（004 §5） ----

// rebuild 以 shadowLang 重新解析並排版 Layer 內的每個事件組，保留疊序、透明格與位移。
func (o *Overlay) rebuild() {
	lang := o.langs[o.shadowLang]
	if lang == nil {
		return
	}
	o.gcRecords()
	rs := &Resolver{Cat: lang.Cat, Wide: lang.Wide}
	for _, key := range o.groupKeys() {
		stamps := o.groupStamps(key)
		rec := o.records[key]
		if rec == nil {
			o.removeGroup(key)
			o.C.Inc("rebuild_lost")
			continue
		}
		dy, ok := groupDY(stamps, rec)
		if !ok {
			o.removeGroup(key)
			o.C.Inc("rebuild_lost")
			continue
		}
		res := rs.Resolve(rec)
		if !res.OK {
			o.removeGroup(key)
			o.C.Inc("switch_untranslated")
			continue
		}
		line, _, err := layoutLine(res.Zh, res.Center, availH(rec.Col, len(rec.Text)), lang.Wide)
		if err != nil {
			o.removeGroup(key)
			o.C.Inc("switch_untranslated")
			continue
		}
		hidden := hiddenOf(rec, stamps)
		o.replaceGroup(key, buildStamps(key, rec.Col*8, rec.Row*8+dy, line, lang.Wide, lang.Font, hidden))
		o.hits[key] = res.Hits
	}
}

// ---- records 的保留（004 §5） ----

// gcRecords 移除沒被 Layer 或影子引用的記錄；仍超過上限時最舊的優先移除。
func (o *Overlay) gcRecords() {
	o.sinceGC = 0
	ref := map[string]bool{}
	for _, s := range o.Layer.Stamps {
		ref[s.Key] = true
	}
	for id := range o.sh.referenced() {
		ref[id] = true
	}
	live := o.recSeq[:0]
	for _, id := range o.recSeq {
		if _, ok := o.records[id]; !ok {
			continue
		}
		if ref[id] {
			live = append(live, id)
			continue
		}
		delete(o.records, id)
		delete(o.hits, id)
	}
	o.recSeq = live
	for len(o.recSeq) > MaxRecords {
		id := o.recSeq[0]
		o.recSeq = o.recSeq[1:]
		delete(o.records, id)
		delete(o.hits, id)
	}
}

// KeysShown 回目前 Layer 內各事件組的 hits 聯集（排序；005 §5 的 keys 欄）。
func (o *Overlay) KeysShown() []string {
	set := map[string]struct{}{}
	for _, key := range o.groupKeys() {
		for _, k := range o.hits[key] {
			set[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
