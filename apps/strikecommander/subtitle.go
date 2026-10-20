package strikecommander

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

// 任務簡報與基地對話（CONV.PAK）的語音字幕（strike_commander 規格 010）。
//
// 觸發是 SB16 DMA 的 start（dosgolem 452 的 SBDMAObserver）：在 start 之後下一次 BeforeStep（Steps() == e.Step）
// 讀對話物件得到（段號、句序），在合成層的字幕區（第 152 到 199 列）畫譯文。只讀不寫：不寫 VGA、不寫原版記憶體。

// SubtitleView 是字幕讀取的唯讀介面（010 §3.12）：View 加 DI。session.StepView 直接滿足它。
type SubtitleView interface {
	View
	DI() uint16
}

// OPTTEST 對話直譯器的三個擷取點（010 §3.1）。位址是 CS:IP 的 IP；簽章不含重定位。
const (
	ipConvEntry    = 0x103F
	ipConvDispatch = 0x10F3
	ipConvFast     = 0x22BE

	convVptr = 0x1924 // 對話物件 +43h 的 vptr（RE 023 §2.1）

	// SubtitleDefaultT 是等待續段的逾時 T（010 §3.4）：1000000 步。
	SubtitleDefaultT = 1_000_000
	// subTC 是語音的時間常數（010 §3.3：VOCLIST.TRE 全部 A6h）。
	subTC = 166
)

var (
	sigConvEntry    = []byte{0x55, 0x8B, 0xEC, 0x81, 0xEC, 0xC2, 0x03, 0x56, 0x57, 0x8B, 0x7E, 0x06, 0xC6, 0x06, 0xD4, 0x69}
	sigConvDispatch = []byte{0xC4, 0x5D, 0x04, 0xFF, 0x45, 0x04, 0x26, 0x8A, 0x07, 0xB4, 0x00, 0x8B, 0xD8, 0x83, 0xFB, 0x12}
	sigConvFast     = []byte{0xFF, 0x45, 0x04, 0x57, 0x8D, 0x86, 0xA4, 0xFC, 0x50}
)

// 字幕條的狀態（010 §3.4）。
type SubState int

const (
	SubShowing SubState = iota // 顯示中
	SubWaiting                 // 等待續段
	SubPaused                  // 暫停
	SubEnded                   // 結束
)

// 結束原因（010 §3.4、§3.11）。
const (
	ReasonDone    = "播完"
	ReasonNext    = "換句"
	ReasonTimeout = "續段逾時"
	ReasonPause   = "暫停"
	ReasonCancel  = "取消"
	ReasonReset   = "重置"
	ReasonOpen    = "未結束"
)

// skip 與 miss 的原因（010 §3.3、§3.4、§3.5、§3.6、§3.7、§3.8）。
const (
	SkipClock      = "時鐘單位不符"
	SkipRate       = "非語音取樣率"
	SkipTiming     = "時點不符"
	SkipUnresolved = "停止未解"
	SkipNoObject   = "物件無效"    // a
	SkipSignature  = "分派點簽章不符" // b
	SkipDSSS       = "DS≠SS"   // c
	SkipVptr       = "vptr 不符" // d
	SkipFarSeg     = "遠指標段不同"  // e
	SkipSeg        = "段號或段長不符" // f
	SkipEnd        = "句尾不符"    // g
	SkipSamples    = "樣本數不符"   // h 第 1、6 列
	SkipPausedSame = "暫停中同句"   // h 第 3 列
	SkipPlayed     = "已播完"     // h 第 4 列
	SkipXfer       = "傳輸不符"
	SkipComplete   = "完成核對不符"
	SkipBlocked    = "字幕區非黑"
	SkipUnblocked  = "字幕區恢復"

	MissNoTranslation = "miss 缺譯"
	MissNoSpeaker     = "miss 說話人無譯文"
	MissSubGlyph      = "miss 缺字"
	MissFullwidth     = "miss 全形轉寫"
	MissForcedBreak   = "miss 強制斷行"
	MissTruncated     = "miss 截斷"
)

// xferID 是傳輸識別（010 §3.4）：不比 Phys、Page、Due。
type xferID struct {
	Command     uint8
	Addr, Count uint16
	Length      uint32
	Samples     uint64
}

func idOf(e machine.SBDMAEvent) xferID {
	return xferID{e.Command, e.Addr, e.Count, e.Length, e.Samples}
}

// SubLine 是一行字幕 stamp 的幾何（010 §3.7）。
type SubLine struct {
	X, Y, Cells, CellW, CellH, GlyphX, GlyphY int
	Text                                      []rune
}

// Subtitle 是一個字幕條（010 §3.4、§3.11）。
type Subtitle struct {
	Key     string
	Start   uint64
	Parts   int
	Acc     uint64 // 累計樣本數
	LastDue uint64 // 最近一次 start 或 resume 的 Due（訖點候選）
	End     uint64
	HasEnd  bool
	Reason  string
	State   SubState
	Text    string // 實際組出的字串（說話人前綴＋代換後的譯文）；缺譯、缺字時為空
	Lines   map[int][]SubLine
	id      xferID
	s       uint64 // S(k)
}

// SubSkip 是一筆 skip 或 miss（010 §3.11 的 subskips.tsv）。
type SubSkip struct {
	Step   uint64
	Reason string
	Obj    string // 線性位址十六進位；空為不適用
	Rel    string
	Seg    string
	Vptr   string
	Key    string
}

// subObj 是物件紀錄（010 §3.2）。
type subObj struct {
	valid          bool
	cs, ss, ds, di uint16
}

// Subtitles 是 010 的字幕狀態機與版面。單一 goroutine 使用；SBDMA 回呼內不讀記憶體（452 §2.4）。
type Subtitles struct {
	Index   *SpeechIndex
	Catalog map[string]string
	Font    *LabelFont
	E, T    uint64

	Subs  []*Subtitle
	Skips []SubSkip

	enabled  bool
	disabled bool // 時鐘單位不符後停用
	obj      subObj
	prevKey  string
	prevSub  *Subtitle
	prevAcc  uint64
	pending  []machine.SBDMAEvent

	// 最近一次 Frame（010 §3.8）。
	frameT     uint64
	frameBlack bool
	haveFrame  bool
	blocked    bool
}

// subSymbols 是允許的代換符號字母（010 §3.6）。
const subSymbols = "SNCWPT"

// NewSubtitles 建字幕。T 為 0 時取 SubtitleDefaultT。預設開啟（SetEnabled）。
func NewSubtitles(index *SpeechIndex, catalog map[string]string, font *LabelFont, e, t uint64) (*Subtitles, error) {
	if index == nil || font == nil || font.Font == nil {
		return nil, fmt.Errorf("strikecommander: 字幕缺結構表或字型")
	}
	if t == 0 {
		t = SubtitleDefaultT
	}
	return &Subtitles{Index: index, Catalog: catalog, Font: font, E: e, T: t, enabled: true}, nil
}

// SetEnabled 是前端字幕開關（010 §3.10）。關閉時不畫，字幕條照常追蹤。
func (s *Subtitles) SetEnabled(on bool) { s.enabled = on }

// Enabled 回開關。
func (s *Subtitles) Enabled() bool { return s.enabled }

func (s *Subtitles) skip(step uint64, reason, key string) {
	s.Skips = append(s.Skips, SubSkip{Step: step, Reason: reason, Key: key})
}

func sigAt(v View, cs, ip uint16, sig []byte) bool {
	for i, b := range sig {
		if v.Read8(lin(cs, ip+uint16(i))) != b {
			return false
		}
	}
	return true
}

// Observe 在每次 BeforeStep 呼叫（010 §3.2、§3.3、§3.4）：先解析待解的 start，再看續段逾時，最後看三個擷取點。
func (s *Subtitles) Observe(v SubtitleView) {
	if len(s.pending) != 0 {
		s.resolvePending(v)
	}
	if n := len(s.Subs); n != 0 {
		if c := s.Subs[n-1]; c.State == SubWaiting && v.Steps() > c.LastDue+s.T {
			c.State, c.End, c.HasEnd, c.Reason = SubEnded, c.LastDue, true, ReasonTimeout
		}
	}
	switch v.IP() {
	case ipConvEntry:
		if sigAt(v, v.CS(), ipConvEntry, sigConvEntry) {
			s.entryHit(v.Steps())
		}
	case ipConvDispatch:
		if sigAt(v, v.CS(), ipConvDispatch, sigConvDispatch) {
			s.obj = subObj{true, v.CS(), v.SS(), v.DS(), v.DI()}
		}
	case ipConvFast:
		if sigAt(v, v.CS(), ipConvFast, sigConvFast) {
			s.obj = subObj{true, v.CS(), v.SS(), v.DS(), v.DI()}
		}
	}
}

// entryHit 是入口命中（010 §3.2 第 1 項、§3.4 表末列）。
func (s *Subtitles) entryHit(step uint64) {
	s.obj = subObj{}
	s.prevKey, s.prevSub, s.prevAcc = "", nil, 0
	if n := len(s.Subs); n != 0 {
		if c := s.Subs[n-1]; c.State == SubShowing || c.State == SubWaiting {
			c.State, c.End, c.HasEnd, c.Reason = SubEnded, step, true, ReasonReset
		}
	}
}

// SBDMA 收件（010 §3.3 收件、§3.4）。回呼內不讀記憶體。
func (s *Subtitles) SBDMA(e machine.SBDMAEvent) {
	if s.disabled {
		return
	}
	if e.Clock != "steps" {
		s.skip(e.Step, SkipClock, "")
		s.disabled = true
		s.pending = nil
		return
	}
	switch e.Kind {
	case "start":
		switch {
		case e.Format == "pcm8" && e.TimeConstant != subTC:
			s.skip(e.Step, SkipRate, "")
		case e.Format == "pcm8" && e.Samples > 16:
			s.pending = append(s.pending, e)
		}
	case "complete", "pause", "resume", "cancel":
		s.xferEvent(e)
	}
}

// xferEvent 處理 complete、pause、resume、cancel（010 §3.4 的表）。只作用在最近一個字幕條、且傳輸識別相符時。
func (s *Subtitles) xferEvent(e machine.SBDMAEvent) {
	n := len(s.Subs)
	if n == 0 {
		return
	}
	c := s.Subs[n-1]
	if idOf(e) != c.id {
		s.skip(e.Step, SkipXfer, c.Key)
		return
	}
	switch e.Kind {
	case "complete":
		if c.State != SubShowing {
			return
		}
		if e.Step != c.LastDue {
			s.skip(e.Step, SkipComplete, c.Key)
			return
		}
		if c.Acc == c.s {
			c.State, c.End, c.HasEnd, c.Reason = SubEnded, e.Due, true, ReasonDone
		} else {
			c.State = SubWaiting
		}
	case "pause":
		if c.State == SubShowing || c.State == SubWaiting {
			c.State, c.End, c.HasEnd, c.Reason = SubPaused, e.Step, true, ReasonPause
		}
	case "resume":
		if c.State == SubPaused {
			c.State, c.End, c.HasEnd, c.Reason = SubShowing, 0, false, ""
			c.LastDue = e.Due
		}
	case "cancel":
		if c.State == SubShowing || c.State == SubWaiting {
			c.State, c.End, c.HasEnd, c.Reason = SubEnded, e.Step, true, ReasonCancel
		}
	}
}

// resolvePending 是 010 §3.3 的解析：Steps() == e.Step 才解析，否則丟棄。
func (s *Subtitles) resolvePending(v SubtitleView) {
	pend := s.pending
	s.pending = nil
	now := v.Steps()
	for _, e := range pend {
		if now != e.Step {
			s.skip(e.Step, SkipTiming, "")
			continue
		}
		s.resolve(v, e)
	}
}

func rd16At(v View, a uint32) uint16 { return uint16(v.Read8(a)) | uint16(v.Read8(a+1))<<8 }

func (s *Subtitles) resolve(v SubtitleView, e machine.SBDMAEvent) {
	sk := SubSkip{Step: e.Step}
	bad := func(reason string) {
		sk.Reason = reason
		s.Skips = append(s.Skips, sk)
	}
	o := s.obj
	if !o.valid { // a
		bad(SkipNoObject)
		return
	}
	at := func(off uint16) uint32 { return lin(o.ds, o.di+off) }
	sk.Obj = fmt.Sprintf("%05X", lin(o.ds, o.di))
	if !sigAt(v, o.cs, ipConvDispatch, sigConvDispatch) { // b
		bad(SkipSignature)
		return
	}
	if o.ds != o.ss { // c
		bad(SkipDSSS)
		return
	}
	vptr := rd16At(v, at(0x43))
	sk.Vptr = fmt.Sprintf("%04X", vptr)
	if vptr != convVptr { // d
		bad(SkipVptr)
		return
	}
	off0, seg0 := rd16At(v, at(0x00)), rd16At(v, at(0x02))
	off4, seg4 := rd16At(v, at(0x04)), rd16At(v, at(0x06))
	if seg0 != seg4 { // e
		bad(SkipFarSeg)
		return
	}
	rel := uint32(off4 - off0)
	sk.Rel = fmt.Sprintf("%X", rel)
	g := int(v.Read8(at(0x6D)))
	sk.Seg = strconv.Itoa(g)
	segLen := uint32(rd16At(v, at(0x08))) | uint32(rd16At(v, at(0x0A)))<<16
	if n, ok := s.Index.SegLen(g); !ok || n != segLen { // f
		bad(SkipSeg)
		return
	}
	line, ok := s.Index.LineAtEnd(g, rel) // g
	if !ok {
		bad(SkipEnd)
		return
	}
	k := line.Key
	sk.Key = k
	// h：由上而下第一個成立的列。
	switch {
	case !line.HasSamples || e.Samples > line.Samples:
		bad(SkipSamples)
	case k != s.prevKey:
		s.newSub(v, e, line, o)
	case s.prevSub != nil && s.prevSub.State == SubPaused:
		bad(SkipPausedSame)
	case s.prevSub == nil || s.prevSub.State == SubEnded || s.prevAcc == line.Samples:
		bad(SkipPlayed)
	case s.prevAcc+e.Samples <= line.Samples:
		c := s.prevSub
		s.prevAcc += e.Samples
		c.Acc, c.Parts, c.LastDue, c.id, c.State = s.prevAcc, c.Parts+1, e.Due, idOf(e), SubShowing
	default:
		bad(SkipSamples)
	}
}

// newSub 建新字幕條（010 §3.3 第 2 列、§3.4 表首兩列），並在這一步讀代換值、組字串（§3.6、§3.7）。
func (s *Subtitles) newSub(v SubtitleView, e machine.SBDMAEvent, line *SpeechLine, o subObj) {
	if n := len(s.Subs); n != 0 {
		if c := s.Subs[n-1]; c.State == SubShowing || c.State == SubWaiting {
			c.State, c.End, c.HasEnd, c.Reason = SubEnded, e.Step, true, ReasonNext
		}
	}
	c := &Subtitle{Key: line.Key, Start: e.Step, Parts: 1, Acc: e.Samples, LastDue: e.Due, State: SubShowing, id: idOf(e), s: line.Samples}
	s.Subs = append(s.Subs, c)
	s.prevKey, s.prevSub, s.prevAcc = line.Key, c, e.Samples
	vals := readSubValues(v, o)
	text, ok := s.compose(e.Step, line, vals)
	if !ok {
		return
	}
	c.Text = string(text)
	c.Lines = map[int][]SubLine{}
	for _, sc := range []int{2, 3} {
		lines, forced, cut := subLayout(text, sc)
		if forced && sc == 2 {
			s.skip(e.Step, MissForcedBreak, line.Key)
		}
		if cut && sc == 2 {
			s.skip(e.Step, MissTruncated, line.Key)
		}
		c.Lines[sc] = lines
	}
}

// subValues 是 §3.6 在解析那一步讀到的代換值（原始 bytes）。
type subValues struct {
	S, N, C, W []byte
	P, T       int16
}

// readCStr 從 seg:off 讀到 00h 為止，最多 20 bytes（欄寬），所以最多 19 字（010 §3.6）。
func readCStr(v View, seg, off uint16) []byte {
	var out []byte
	for i := uint16(0); i < 20; i++ {
		b := v.Read8(lin(seg, off+i))
		if b == 0 {
			return out
		}
		out = append(out, b)
	}
	return out[:19]
}

func readSubValues(v View, o subObj) subValues {
	p := rd16At(v, lin(o.ds, o.di+0x28))
	return subValues{
		S: readCStr(v, o.ds, rd16At(v, lin(o.ds, p+0))),
		N: readCStr(v, o.ds, rd16At(v, lin(o.ds, p+2))),
		C: readCStr(v, o.ds, rd16At(v, lin(o.ds, p+4))),
		W: readCStr(v, o.ds, rd16At(v, lin(o.ds, o.di+0x2A))+1),
		P: int16(rd16At(v, lin(o.ds, p+6))),
		T: int16(rd16At(v, lin(o.ds, p+8))),
	}
}

// Fullwidth 是 §3.6 的全形轉寫：21h–7Eh 換成 FF01h–FF5Eh，20h 換成 3000h，其他 byte 換成「？」（bad 計數）。
func Fullwidth(b []byte) (out []rune, bad int) {
	for _, c := range b {
		switch {
		case c == 0x20:
			out = append(out, '　')
		case c >= 0x21 && c <= 0x7E:
			out = append(out, rune(c)+0xFEE0)
		default:
			out = append(out, '？')
			bad++
		}
	}
	return out, bad
}

// fullwidthInt 是 $P、$T 的顯示：全形數字，負號「－」。
func fullwidthInt(n int16) []rune {
	r, _ := Fullwidth([]byte(strconv.Itoa(int(n))))
	return r
}

// wingmen 是 $W 對照譯名的名字表（010 §3.6，名字表 DS:15BC）。
var wingmen = map[string]string{"Billy": "speaker/billy", "Gwen": "speaker/gwen", "Lyle": "speaker/lyle", "Miguel": "speaker/miguel", "Tex": "speaker/tex"}

// substitute 把 tr 的代換符號換成玩家資料（§3.6）。bad 是全形轉寫換成「？」的 byte 數。
func (s *Subtitles) substitute(tr string, vals subValues) (out []rune, bad int) {
	rs := []rune(tr)
	for i := 0; i < len(rs); i++ {
		if rs[i] != '$' || i+1 >= len(rs) {
			out = append(out, rs[i])
			continue
		}
		var add []rune
		var b int
		switch rs[i+1] {
		case 'S':
			add, b = Fullwidth(vals.S)
		case 'N':
			add, b = Fullwidth(vals.N)
		case 'C':
			add, b = Fullwidth(vals.C)
		case 'W':
			if k, ok := wingmen[string(vals.W)]; ok && s.Catalog[k] != "" {
				add = []rune(s.Catalog[k])
			} else {
				add, b = Fullwidth(vals.W)
			}
		case 'P':
			add = fullwidthInt(vals.P)
		case 'T':
			add = fullwidthInt(vals.T)
		default:
			out = append(out, rs[i])
			continue
		}
		out = append(out, add...)
		bad += b
		i++
	}
	return out, bad
}

// compose 組字串：說話人顯示名＋「：」＋代換後的譯文（§3.7）。缺譯、缺字時記 miss、回 false。
func (s *Subtitles) compose(step uint64, line *SpeechLine, vals subValues) ([]rune, bool) {
	tr, ok := s.Catalog[line.Key]
	if !ok || tr == "" {
		s.skip(step, MissNoTranslation, line.Key)
		return nil, false
	}
	var text []rune
	bad := 0
	if sp, ok := s.Catalog["speaker/"+line.Speaker]; ok && sp != "" {
		p, b := s.substitute(sp, vals)
		text = append(p, '：')
		bad += b
	} else {
		s.skip(step, MissNoSpeaker, line.Key)
	}
	body, b := s.substitute(tr, vals)
	text = append(text, body...)
	bad += b
	if bad != 0 {
		s.skip(step, MissFullwidth, line.Key)
	}
	for _, r := range text {
		if !s.Font.Wide.Wide(r) {
			s.Skips = append(s.Skips, SubSkip{Step: step, Reason: MissSubGlyph + " " + string(r), Key: line.Key})
			return nil, false
		}
	}
	return text, true
}

// 換行禁則（010 §3.7.1）。
var (
	wrapHead = map[rune]bool{'，': true, '。': true, '、': true, '！': true, '？': true, '；': true, '：': true, '」': true, '』': true, '）': true, '…': true}
	wrapTail = map[rune]bool{'「': true, '『': true, '（': true}
)

// wrapLine 是 010 §3.7.1 的換行演算法；forced 回有沒有強制斷行。lint 與顯示共用。
func wrapLine(c []rune, w int) (lines [][]rune, forced bool) {
	i := 0
	for {
		if len(c)-i <= w {
			return append(lines, c[i:]), forced
		}
		j := i + w
		for j > i+1 && (wrapHead[c[j]] || wrapTail[c[j-1]]) {
			j--
		}
		if j == i+1 {
			j = i + w
			forced = true
		}
		lines = append(lines, c[i:j])
		i = j
	}
}

// subGeom 是 §3.7 的倍率表：CellW、CellH（原版像素）、GlyphX、GlyphY（輸出像素）、每行最多格數 W。
func subGeom(scale int) (cellW, cellH, gx, gy, w int) {
	if scale == 3 {
		return 6, 7, 1, 2, 50
	}
	return 8, 10, 0, 2, 38
}

const subMaxLines = 4

// subLayout 回 scale 倍的各行 stamp 幾何（§3.7）；cut 為超過 4 行而截斷。
func subLayout(text []rune, scale int) (out []SubLine, forced, cut bool) {
	cw, ch, gx, gy, w := subGeom(scale)
	lines, forced := wrapLine(text, w)
	if len(lines) > subMaxLines {
		lines = lines[:subMaxLines]
		last := append([]rune(nil), lines[subMaxLines-1]...)
		last[len(last)-1] = '…'
		lines[subMaxLines-1] = last
		cut = true
	}
	h := len(lines) * ch
	y0 := 152 + (48-h)/2
	for j, l := range lines {
		out = append(out, SubLine{X: (320 - len(l)*cw) / 2, Y: y0 + j*ch, Cells: len(l), CellW: cw, CellH: ch, GlyphX: gx, GlyphY: gy, Text: l})
	}
	return out, forced, cut
}

// subRegionBlack 回字幕區（第 152 到 199 列）15360 個像素全為色號 0（§3.8 條件 2）；長度不是 64000 視為非黑。
func subRegionBlack(indexed []byte) bool {
	if len(indexed) != 64000 {
		return false
	}
	for _, c := range indexed[152*320 : 200*320] {
		if c != 0 {
			return false
		}
	}
	return true
}

// Frame 在每次回掃呼叫（§3.8）：記下畫格步數 t 與字幕區是否全黑，阻擋狀態改變時記 skip。
func (s *Subtitles) Frame(indexed []byte, t uint64) {
	s.frameT, s.frameBlack, s.haveFrame = t, subRegionBlack(indexed), true
	_, cond1 := s.current()
	blocked := cond1 && !s.frameBlack
	if blocked != s.blocked {
		if blocked {
			s.skip(t, SkipBlocked, "")
		} else {
			s.skip(t, SkipUnblocked, "")
		}
		s.blocked = blocked
	}
}

// current 回最近一次 Frame 當下符合 §3.8 條件 1 的字幕條。
func (s *Subtitles) current() (*Subtitle, bool) {
	if !s.haveFrame {
		return nil, false
	}
	t := s.frameT
	for i := len(s.Subs) - 1; i >= 0; i-- {
		c := s.Subs[i]
		if (c.State == SubShowing || c.State == SubWaiting) && c.Start <= t {
			return c, true
		}
	}
	if s.E > 0 {
		for i := len(s.Subs) - 1; i >= 0; i-- {
			c := s.Subs[i]
			if c.State == SubEnded && c.Reason == ReasonDone && t < c.End+s.E && c.Start <= t &&
				(i == len(s.Subs)-1 || t < s.Subs[i+1].Start) {
				return c, true
			}
		}
	}
	return nil, false
}

// Blocked 回最近一次 Frame 的阻擋狀態（§3.8）。
func (s *Subtitles) Blocked() bool { return s.blocked }

// FrameStep 回最近一次 Frame 的畫格步數 t。
func (s *Subtitles) FrameStep() uint64 { return s.frameT }

// Visible 回最近一次 Frame 當下要畫的字幕條（§3.8 三個條件都成立；缺譯、缺字的條不畫）。
func (s *Subtitles) Visible() *Subtitle {
	c, ok := s.current()
	if !ok || !s.frameBlack || !s.enabled || c.Text == "" {
		return nil
	}
	return c
}

// Draw 把字幕畫進 dst（scale 倍的合成緩衝，Overlay.Draw 的輸出），回畫出的各行（§3.7）。
// 字形以外的像素維持 dst 原值（以 dst 的複本當背景圖，DrawWithBackground）。
func (s *Subtitles) Draw(dst []byte, scale int) []SubLine {
	c := s.Visible()
	if c == nil || scale < 2 || scale > 3 || len(dst) != 320*scale*200*scale*4 {
		return nil
	}
	lines := c.Lines[scale]
	layer := &xlate.Layer{W: 320, H: 200}
	for _, l := range lines {
		layer.Stamps = append(layer.Stamps, &xlate.Stamp{Key: c.Key, X: l.X, Y: l.Y, Cells: l.Cells, CellW: l.CellW, CellH: l.CellH,
			Font: s.Font.Font, GlyphX: l.GlyphX, GlyphY: l.GlyphY, GlyphScale: 1, Text: l.Text, State: xlate.Shown,
			FG: [3]uint8{255, 255, 255}, BG: [3]uint8{0, 0, 0}})
	}
	bg := append([]byte(nil), dst...)
	layer.DrawWithBackground(dst, scale, nil, bg, func(*xlate.Stamp) bool { return true })
	return lines
}

// Finish 在收據停止時呼叫（§3.3、§3.4）：待解清單記「停止未解」；顯示中、等待續段的字幕條原因「未結束」。
func (s *Subtitles) Finish() {
	for _, e := range s.pending {
		s.skip(e.Step, SkipUnresolved, "")
	}
	s.pending = nil
	for _, c := range s.Subs {
		if c.State == SubShowing || c.State == SubWaiting {
			c.Reason = ReasonOpen
		}
	}
}

// SubsTSV 編 subs.tsv（§3.11）。
func SubsTSV(subs []*Subtitle) []byte {
	var b strings.Builder
	b.WriteString("key\tstart\tparts\tsamples\tlast_due\tend\treason\ttext\n")
	for _, c := range subs {
		end := ""
		if c.HasEnd {
			end = strconv.FormatUint(c.End, 10)
		}
		fmt.Fprintf(&b, "%s\t%d\t%d\t%d\t%d\t%s\t%s\t%s\n", c.Key, c.Start, c.Parts, c.Acc, c.LastDue, end, c.Reason, c.Text)
	}
	return []byte(b.String())
}

// SubSkipsTSV 編 subskips.tsv（§3.11）。
func SubSkipsTSV(skips []SubSkip) []byte {
	var b strings.Builder
	b.WriteString("step\treason\tobj\trel\tseg\tvptr\tkey\n")
	for _, k := range skips {
		fmt.Fprintf(&b, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n", k.Step, k.Reason, k.Obj, k.Rel, k.Seg, k.Vptr, k.Key)
	}
	return []byte(b.String())
}
