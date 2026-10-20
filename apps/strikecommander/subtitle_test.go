package strikecommander

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// strike_commander 規格 010 §4.1 第 1、3 到 8 項的單元測試：合成的 View、記憶體與 SB16 事件序列。
// 期望值在測試內獨立寫出，不迭代實作的表。

const (
	sCS    = 0x0CE7 // 直譯器所在段（任意值；位址只看 IP）
	sDS    = 0x4E66
	sDI    = 0xFC80
	sBufSg = 0xC054 // 段緩衝的段
	sPTab  = 0xFCEE // 玩家名字指標表（obj+28h）
	sWing  = 0x1600 // $W 字串在 DS:(sWing+1)
)

// subFontRunes 是單元測試字型的全形字：測試譯文用到的字、禁則標點、全形 ASCII、U+3000。
var subFontRunes = func() string {
	var b strings.Builder
	b.WriteString("史登萊爾比利玩家好運輸機已回報測試一二三四五六七八九十甲乙丙丁戊己庚辛壬癸，。、！？；：」』）…「『（　中文字幕今天沒有錢拿")
	for r := rune(0xFF01); r <= 0xFF5E; r++ {
		b.WriteRune(r)
	}
	return b.String()
}()

func subTestFont(t *testing.T) *LabelFont {
	t.Helper()
	f, err := ParseLabelFont(buildTestFont(subFontRunes, "a"), "full16-subtest")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// subRow 是合成結構表的一列。samples 為 0 時表示不在 VOCLIST。
type subRow struct {
	key, op, speaker, en, voc string
	segLen, end               int
	samples                   int
	symbols                   string
}

var subRows = []subRow{
	{"conv/011/00", "03", "stern", "Stern", "2st139", 814, 0x53, 1000, "$N"},
	{"conv/011/01", "04", "stern", "Stern", "2st140", 814, 0x88, 500, ""},
	{"conv/011/02", "04", "stern", "Stern", "2st141", 814, 0xB8, 300, ""},
	{"conv/011/03", "03", "air", "(player)", "2ai621", 814, 0xF8, 400, "$S"},
	{"conv/011/05", "03", "stern", "Stern", "2ai437", 814, 0x14F, 0, ""},
	{"conv/011/06", "03", "men", "Mendez", "3me001", 814, 0x172, 400, ""},
	{"conv/011/07", "03", "lyle", "Lyle", "2ly001", 814, 0x1B8, 400, "$W"},
	{"conv/020/00", "03", "air", "(player)", "z1ai200", 256, 0x30, 600, ""},
	{"conv/020/02", "03", "air", "(player)", "z1ai200", 256, 0x70, 600, ""},
	{"conv/021/03", "03", "air", "(player)", "z1ai200", 288, 0x50, 600, ""},
}

func subIndexBytes(rows []subRow) []byte {
	var b strings.Builder
	b.WriteString("key\tseg_len\tend\top\tspeaker\tspeaker_en\tvoc\tsamples\tsymbols\n")
	for _, r := range rows {
		s := ""
		if r.samples != 0 {
			s = fmt.Sprint(r.samples)
		}
		fmt.Fprintf(&b, "%s\t%d\t%X\t%s\t%s\t%s\t%s\t%s\t%s\n", r.key, r.segLen, r.end, r.op, r.speaker, r.en, r.voc, s, r.symbols)
	}
	return []byte(b.String())
}

func subCatalog() map[string]string {
	return map[string]string{
		"conv/011/00":    "好，$N，運輸機已回報。",
		"conv/011/01":    "測試一",
		"conv/011/02":    "測試二",
		"conv/011/03":    "$S，測試三",
		"conv/011/06":    "測試六",
		"conv/011/07":    "$W，測試七",
		"conv/020/00":    "測試甲",
		"conv/020/02":    "測試乙",
		"conv/021/03":    "測試丙",
		"speaker/stern":  "史登",
		"speaker/air":    "$C",
		"speaker/lyle":   "萊爾",
		"speaker/billy":  "比利",
		"speaker/gwen":   "葛雯",
		"speaker/miguel": "米蓋爾",
		"speaker/tex":    "泰克斯",
	}
}

type subView struct {
	*fakeView
	di uint16
}

func (v subView) DI() uint16 { return v.di }

// subH 把 Subtitles 與合成記憶體綁在一起。每次 at／idle 是一次 BeforeStep（步數加 1）。
type subH struct {
	t  *testing.T
	f  *fakeView
	di uint16
	s  *Subtitles
}

func newSubH(t *testing.T) *subH {
	t.Helper()
	idx, err := ParseSpeechIndex(subIndexBytes(subRows))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSubtitles(idx, subCatalog(), subTestFont(t), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &subH{t: t, f: &fakeView{mem: map[uint32]byte{}, steps: 1000, cs: 0x1111, ip: 0x2222, ss: sDS, ds: sDS}, di: sDI, s: s}
	h.installProgram(0)
	h.setObject(11, 0x53, 814)
	h.setNames("lee", "sam", "ace", "Lyle", 2, -5)
	return h
}

func bytesOf(b []byte) []int {
	out := make([]int, len(b))
	for i, c := range b {
		out[i] = int(c)
	}
	return out
}

// installProgram 在 sCS 寫三個簽章；shift 非 0 時分派點簽章改寫在 10F3h − shift（模擬 TOPTTST1 的位移）。
func (h *subH) installProgram(shift uint16) {
	h.f.w8(sCS, ipConvEntry, bytesOf(sigConvEntry)...)
	h.f.w8(sCS, ipConvDispatch-shift, bytesOf(sigConvDispatch)...)
	h.f.w8(sCS, ipConvFast, bytesOf(sigConvFast)...)
}

// setObject 寫對話物件：vptr、兩個遠指標（基底 C054:0004、目前 C054:0004+rel）、段長、段號。
func (h *subH) setObject(seg int, rel uint16, segLen uint32) {
	o := uint16(sDI)
	h.f.w16(sDS, o+0x43, convVptr)
	h.f.w16(sDS, o+0x00, 0x0004)
	h.f.w16(sDS, o+0x02, sBufSg)
	h.f.w16(sDS, o+0x04, 0x0004+rel)
	h.f.w16(sDS, o+0x06, sBufSg)
	h.f.w16(sDS, o+0x08, uint16(segLen))
	h.f.w16(sDS, o+0x0A, uint16(segLen>>16))
	h.f.w8(sDS, o+0x6D, seg)
}

func (h *subH) setRel(rel uint16) { h.f.w16(sDS, uint16(sDI)+0x04, 0x0004+rel) }

// setNames 寫 $S $N $C 的字串、$W、$P、$T（RE 023 §4.1 的版面）。
func (h *subH) setNames(s, n, c, w string, p, tt int16) {
	h.f.w16(sDS, uint16(sDI)+0x28, sPTab)
	str := func(off uint16, v string) {
		h.f.w8(sDS, off, append(bytesOf([]byte(v)), 0)...)
	}
	h.f.w16(sDS, sPTab+0, 0x48D6)
	h.f.w16(sDS, sPTab+2, 0x48EA)
	h.f.w16(sDS, sPTab+4, 0x48FE)
	h.f.w16(sDS, sPTab+6, uint16(p))
	h.f.w16(sDS, sPTab+8, uint16(tt))
	str(0x48D6, s)
	str(0x48EA, n)
	str(0x48FE, c)
	h.f.w16(sDS, uint16(sDI)+0x2A, sWing)
	str(sWing+1, w)
}

func (h *subH) observe() { h.s.Observe(subView{h.f, h.di}) }

// at 是一次 BeforeStep：CS:IP 指向 ip（直譯器段），步數加 1。
func (h *subH) at(ip uint16) {
	h.f.steps++
	h.f.cs, h.f.ip = sCS, ip
	h.observe()
}

// idle 是一次 BeforeStep：CS:IP 在別處。
func (h *subH) idle() {
	h.f.steps++
	h.f.cs, h.f.ip = 0x1111, 0x2222
	h.observe()
}

// idleTo 推進到 Steps() == n（中間不逐步模擬，只在 n 做一次 BeforeStep）。
func (h *subH) idleTo(n uint64) {
	h.f.steps = n - 1
	h.idle()
}

// ev 組一筆事件；transfer 決定傳輸識別（Addr）。
func ev(kind string, step uint64, samples uint64, due uint64, addr uint16) machine.SBDMAEvent {
	return machine.SBDMAEvent{Kind: kind, Step: step, Command: 0x14, Format: "pcm8", Length: uint32(samples), Samples: samples,
		TimeConstant: subTC, TCKnown: true, Clock: "steps", Channel: 1, DMAMode: 0x48, Page: 0x09, Addr: addr, Count: uint16(samples - 1),
		Phys: 0x090000 | uint32(addr), Due: due}
}

// start 模擬一道指令產生 start（e.Step = 下一道指令的序號），之後一次 BeforeStep（Steps() == e.Step）解析。
func (h *subH) start(samples, dur uint64, addr uint16) machine.SBDMAEvent {
	e := ev("start", h.f.steps+1, samples, h.f.steps+1+dur, addr)
	h.s.SBDMA(e)
	h.idle()
	return e
}

// inInsn 送一筆指令中的事件（pause、resume、cancel），之後一次 BeforeStep。
func (h *subH) inInsn(kind string, of machine.SBDMAEvent, due uint64) {
	e := of
	e.Kind, e.Step, e.Due, e.SHA256 = kind, h.f.steps+1, due, ""
	h.s.SBDMA(e)
	h.idle()
}

// complete 在 Steps() == e.Due 時送 complete（步驟嘗試開頭，Step 是上一道指令的序號）。
func (h *subH) complete(of machine.SBDMAEvent, due uint64) {
	h.idleTo(due)
	e := of
	e.Kind, e.Step, e.Due = "complete", due, due
	h.s.SBDMA(e)
	h.idle()
}

func (h *subH) lastSkip() string {
	if len(h.s.Skips) == 0 {
		return ""
	}
	return h.s.Skips[len(h.s.Skips)-1].Reason
}

func (h *subH) skipCount(reason string) int {
	n := 0
	for _, k := range h.s.Skips {
		if k.Reason == reason {
			n++
		}
	}
	return n
}

func (h *subH) wantSubs(keys ...string) {
	h.t.Helper()
	var got []string
	for _, c := range h.s.Subs {
		got = append(got, c.Key)
	}
	if strings.Join(got, ",") != strings.Join(keys, ",") {
		h.t.Fatalf("字幕條 %v，要 %v（skips %+v）", got, keys, h.s.Skips)
	}
}

// ---- 第 1 項：簽章 ----------------------------------------------------------

func TestSubtitleSignatures(t *testing.T) {
	h := newSubH(t)
	h.at(ipConvDispatch)
	if !h.s.obj.valid || h.s.obj.di != sDI || h.s.obj.cs != sCS {
		t.Fatalf("分派點沒有命中：%+v", h.s.obj)
	}
	h.at(ipConvEntry)
	if h.s.obj.valid {
		t.Fatal("入口沒有命中（物件紀錄仍有效）")
	}
	h.at(ipConvFast)
	if !h.s.obj.valid {
		t.Fatal("快速路徑沒有命中")
	}
	// IP 相符、bytes 不符：不命中。
	h2 := newSubH(t)
	h2.f.w8(sCS, ipConvDispatch, 0x90)
	h2.at(ipConvDispatch)
	if h2.s.obj.valid {
		t.Fatal("bytes 不符仍命中")
	}
	// 分派點 bytes 寫在 10F0h（TOPTTST1 的 mod 16 為 0），IP 10F0h：不命中。
	h3 := newSubH(t)
	h3.f.mem = map[uint32]byte{}
	h3.installProgram(3)
	h3.setObject(11, 0x53, 814)
	h3.setNames("lee", "sam", "ace", "Lyle", 2, -5)
	h3.at(ipConvDispatch - 3)
	if h3.s.obj.valid {
		t.Fatal("IP 10F0h 也命中")
	}
	// 只有快速路徑、沒有分派點：之後的 start 仍能解析。
	h4 := newSubH(t)
	h4.at(ipConvFast)
	h4.start(1000, 5000, 0x1000)
	h4.wantSubs("conv/011/00")
}

// ---- 第 3 項：解析 ----------------------------------------------------------

func TestSubtitleResolveConditions(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(h *subH)
		want   string
	}{
		{"a 物件無效", func(h *subH) { h.s.obj.valid = false }, SkipNoObject},
		{"b 簽章改變", func(h *subH) { h.f.w8(sCS, ipConvDispatch+5, 0x00) }, SkipSignature},
		{"c DS≠SS", func(h *subH) { h.s.obj.ss = 0x1234 }, SkipDSSS},
		{"d vptr", func(h *subH) { h.f.w16(sDS, uint16(sDI)+0x43, 0x01CE) }, SkipVptr},
		{"e 遠指標段", func(h *subH) { h.f.w16(sDS, uint16(sDI)+0x06, 0xC055) }, SkipFarSeg},
		{"f 段號不在表內", func(h *subH) { h.f.w8(sDS, uint16(sDI)+0x6D, 99) }, SkipSeg},
		{"f 段長不符", func(h *subH) { h.f.w16(sDS, uint16(sDI)+0x08, 815) }, SkipSeg},
		{"g 句尾不符", func(h *subH) { h.setRel(0x54) }, SkipEnd},
		{"h 樣本數超過", func(h *subH) {}, SkipSamples},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newSubH(t)
			h.at(ipConvDispatch)
			c.mutate(h)
			samples := uint64(1000)
			if c.want == SkipSamples {
				samples = 1001
			}
			h.start(samples, 5000, 0x1000)
			if len(h.s.Subs) != 0 || h.lastSkip() != c.want {
				t.Fatalf("字幕條 %d、最後 skip %q，要 0、%q", len(h.s.Subs), h.lastSkip(), c.want)
			}
		})
	}
	// 全部成立：key 正確。
	h := newSubH(t)
	h.at(ipConvDispatch)
	h.start(1000, 5000, 0x1000)
	h.wantSubs("conv/011/00")
	// S(k) 不存在（2ai437）：樣本數不符。
	h = newSubH(t)
	h.setRel(0x14F)
	h.at(ipConvDispatch)
	h.start(100, 5000, 0x1000)
	if len(h.s.Subs) != 0 || h.lastSkip() != SkipSamples {
		t.Fatalf("S(k) 不存在：%d 條、%q", len(h.s.Subs), h.lastSkip())
	}
}

// rel 落在句中：以「起點 < rel 的最後一句」判定會得到 conv/011/01，正確規則記 skip。
func TestSubtitleRelInsideLine(t *testing.T) {
	h := newSubH(t)
	h.setRel(0x70) // 介於第 0 句句尾 53h 與第 1 句句尾 88h 之間
	h.at(ipConvDispatch)
	h.start(500, 5000, 0x1000)
	if len(h.s.Subs) != 0 || h.lastSkip() != SkipEnd {
		t.Fatalf("rel 落在句中：%d 條、%q，要 0、%q", len(h.s.Subs), h.lastSkip(), SkipEnd)
	}
}

// 多台詞語音：同語音編號在兩段、同段兩句；物件指到哪句就是哪句的 key。
func TestSubtitleMultiLineVoice(t *testing.T) {
	for _, c := range []struct {
		seg    int
		rel    uint16
		segLen uint32
		want   string
	}{{20, 0x70, 256, "conv/020/02"}, {21, 0x50, 288, "conv/021/03"}} {
		h := newSubH(t)
		h.setObject(c.seg, c.rel, c.segLen)
		h.at(ipConvDispatch)
		h.start(600, 5000, 0x1000)
		h.wantSubs(c.want)
	}
}

// 條件 h 的六列各一例。
func TestSubtitleSampleRules(t *testing.T) {
	// 第 1 列：Samples > S。
	h := newSubH(t)
	h.at(ipConvDispatch)
	h.start(1001, 5000, 0x1000)
	if h.lastSkip() != SkipSamples {
		t.Fatalf("第 1 列：%q", h.lastSkip())
	}
	// 第 2 列：新 key → 新字幕條。第 5 列：拆頁續段。
	h = newSubH(t)
	h.at(ipConvDispatch)
	e1 := h.start(600, 3000, 0x1000)
	h.complete(e1, e1.Due)
	h.start(400, 2000, 0x0000)
	h.wantSubs("conv/011/00")
	if c := h.s.Subs[0]; c.Parts != 2 || c.Acc != 1000 {
		t.Fatalf("續段：parts %d acc %d", c.Parts, c.Acc)
	}
	// 第 6 列：acc < S < acc + Samples。
	h = newSubH(t)
	h.at(ipConvDispatch)
	e1 = h.start(600, 3000, 0x1000)
	h.complete(e1, e1.Due)
	h.start(500, 2000, 0x0000)
	if h.lastSkip() != SkipSamples || h.s.Subs[0].Parts != 1 {
		t.Fatalf("第 6 列：%q parts %d", h.lastSkip(), h.s.Subs[0].Parts)
	}
	// 第 3 列：同 key、暫停中、acc == S → 「暫停中同句」（第 3 列先於第 4 列）。
	h = newSubH(t)
	h.at(ipConvDispatch)
	e1 = h.start(1000, 5000, 0x1000)
	h.inInsn("pause", e1, e1.Due)
	h.start(1000, 5000, 0x1000)
	if h.lastSkip() != SkipPausedSame || len(h.s.Subs) != 1 || h.s.Subs[0].State != SubPaused {
		t.Fatalf("第 3 列：%q %d", h.lastSkip(), len(h.s.Subs))
	}
	// 第 4 列：已結束（播完）→ 「已播完」。
	h = newSubH(t)
	h.at(ipConvDispatch)
	e1 = h.start(1000, 5000, 0x1000)
	h.complete(e1, e1.Due)
	h.start(1000, 5000, 0x1000)
	if h.lastSkip() != SkipPlayed || len(h.s.Subs) != 1 {
		t.Fatalf("第 4 列：%q %d", h.lastSkip(), len(h.s.Subs))
	}
}

func TestSubtitleResolveTiming(t *testing.T) {
	// 下一步改了 [+04]：只在 Steps() == e.Step 讀才對到 conv/011/00。
	h := newSubH(t)
	h.at(ipConvDispatch)
	e := ev("start", h.f.steps+1, 1000, h.f.steps+6000, 0x1000)
	h.s.SBDMA(e)
	h.idle() // Steps() == e.Step：解析
	h.setRel(0x88)
	h.idle()
	h.wantSubs("conv/011/00")
	// Steps() ≠ e.Step：丟棄並記「時點不符」。
	h = newSubH(t)
	h.at(ipConvDispatch)
	e = ev("start", h.f.steps+2, 1000, h.f.steps+6000, 0x1000)
	h.s.SBDMA(e)
	h.idle()
	if len(h.s.Subs) != 0 || h.lastSkip() != SkipTiming {
		t.Fatalf("時點不符：%d %q", len(h.s.Subs), h.lastSkip())
	}
	// 停止時待解清單非空：「停止未解」。
	h = newSubH(t)
	h.at(ipConvDispatch)
	h.s.SBDMA(ev("start", h.f.steps+1, 1000, h.f.steps+6000, 0x1000))
	h.s.Finish()
	if h.lastSkip() != SkipUnresolved {
		t.Fatalf("停止未解：%q", h.lastSkip())
	}
}

func TestSubtitleIntake(t *testing.T) {
	// Clock == cycles：記「時鐘單位不符」並停用。
	h := newSubH(t)
	h.at(ipConvDispatch)
	e := ev("start", h.f.steps+1, 1000, h.f.steps+6000, 0x1000)
	e.Clock = "cycles"
	h.s.SBDMA(e)
	h.idle()
	if h.lastSkip() != SkipClock || len(h.s.Subs) != 0 {
		t.Fatalf("cycles：%q", h.lastSkip())
	}
	h.start(1000, 5000, 0x1000)
	if len(h.s.Subs) != 0 {
		t.Fatal("停用後仍收件")
	}
	// 靜音、4 bytes、TimeConstant 165 的 start 不收。
	for _, c := range []func(e *machine.SBDMAEvent){
		func(e *machine.SBDMAEvent) { e.Format, e.Command = "silence", 0x80 },
		func(e *machine.SBDMAEvent) { e.Samples, e.Length = 4, 4 },
		func(e *machine.SBDMAEvent) { e.TimeConstant = 165 },
	} {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := ev("start", h.f.steps+1, 1000, h.f.steps+6000, 0x1000)
		c(&e)
		h.s.SBDMA(e)
		h.idle()
		if len(h.s.Subs) != 0 {
			t.Fatalf("%s/%d/%d 的 start 被收件", e.Format, e.Samples, e.TimeConstant)
		}
	}
	h = newSubH(t)
	e = ev("start", h.f.steps+1, 1000, 0, 0x1000)
	e.TimeConstant = 173
	h.s.SBDMA(e)
	if h.lastSkip() != SkipRate {
		t.Fatalf("173：%q", h.lastSkip())
	}
}

// ---- 第 4 項：狀態機 --------------------------------------------------------

// visibleAt 在步數 t 做一次全黑的 Frame，回有沒有字幕條可見。
func (h *subH) visibleAt(t uint64) bool {
	h.s.Frame(make([]byte, 64000), t)
	return h.s.Visible() != nil
}

func TestSubtitleStateMachine(t *testing.T) {
	t.Run("單段播完", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		if !h.visibleAt(e.Step + 10) {
			t.Fatal("播放中不可見")
		}
		h.complete(e, e.Due)
		c := h.s.Subs[0]
		if c.Reason != ReasonDone || c.End != e.Due || c.State != SubEnded || h.visibleAt(e.Due+1) {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("拆頁", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e1 := h.start(600, 3000, 0x1000)
		h.complete(e1, e1.Due)
		if c := h.s.Subs[0]; c.State != SubWaiting || !h.visibleAt(e1.Due+1) {
			t.Fatalf("第一段完成後 %+v", c)
		}
		h.idleTo(e1.Due + 378)
		e2 := h.start(400, 2000, 0x0000)
		h.complete(e2, e2.Due)
		c := h.s.Subs[0]
		if len(h.s.Subs) != 1 || c.Parts != 2 || c.Reason != ReasonDone || c.End != e2.Due || c.LastDue != e2.Due {
			t.Fatalf("%d 條 %+v", len(h.s.Subs), c)
		}
	})
	t.Run("續段逾時", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e1 := h.start(600, 3000, 0x1000)
		h.complete(e1, e1.Due)
		h.idleTo(e1.Due + SubtitleDefaultT)
		if h.s.Subs[0].State != SubWaiting {
			t.Fatal("T 步時已逾時")
		}
		h.idle()
		c := h.s.Subs[0]
		if c.Reason != ReasonTimeout || c.End != e1.Due {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("pause 無 resume", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		h.inInsn("pause", e, e.Due)
		c := h.s.Subs[0]
		if c.Reason != ReasonPause || c.End != h.f.steps || h.visibleAt(h.f.steps) {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("pause 後 cancel 同一步 start 另一句", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		h.inInsn("pause", e, e.Due)
		ps := h.f.steps
		h.idleTo(ps + 100)
		// 同一道指令：cancel（隱式取消，頁暫存器已改）再 start 下一句。
		h.setRel(0x88)
		c := e
		c.Kind, c.Step, c.Phys = "cancel", h.f.steps+1, 0x0A1000
		h.s.SBDMA(c)
		s := ev("start", h.f.steps+1, 500, h.f.steps+4000, 0x2000)
		h.s.SBDMA(s)
		if len(h.s.Subs) != 1 {
			t.Fatal("新字幕條在 start 回呼內就建立")
		}
		h.idle()
		h.wantSubs("conv/011/00", "conv/011/01")
		if o := h.s.Subs[0]; o.Reason != ReasonPause || o.End != ps {
			t.Fatalf("舊字幕條 %+v，要暫停、訖點 %d", o, ps)
		}
		if n := h.s.Subs[1]; n.Start != s.Step {
			t.Fatalf("新字幕條起點 %d，要 %d", n.Start, s.Step)
		}
	})
	t.Run("pause 後走過 Due 不變播完", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		h.inInsn("pause", e, e.Due)
		h.idleTo(e.Due + 10)
		h.idle()
		if c := h.s.Subs[0]; c.Reason != ReasonPause || c.State != SubPaused {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("pause 後 resume", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		h.inInsn("pause", e, e.Due)
		h.idleTo(h.f.steps + 300)
		newDue := e.Due + 300
		h.inInsn("resume", e, newDue)
		c := h.s.Subs[0]
		if c.State != SubShowing || c.HasEnd || c.LastDue != newDue || !h.visibleAt(h.f.steps) {
			t.Fatalf("%+v", c)
		}
		h.complete(e, newDue)
		if c.Reason != ReasonDone || c.End != newDue {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("顯示中收到識別相符的 cancel", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		h.inInsn("cancel", e, e.Due)
		if c := h.s.Subs[0]; c.Reason != ReasonCancel || c.End != h.f.steps {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("識別不符", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		other := ev("start", 0, 3000, 0, 0x5000)
		other.TimeConstant = 173
		silence := machine.SBDMAEvent{Command: 0x80, Format: "silence", Length: 1000, Samples: 1000, Clock: "steps"}
		for _, o := range []machine.SBDMAEvent{other, silence} {
			for _, k := range []string{"pause", "cancel", "complete"} {
				x := o
				x.Kind, x.Step, x.Due = k, h.f.steps+1, e.Due
				h.s.SBDMA(x)
				h.idle()
			}
		}
		if c := h.s.Subs[0]; c.State != SubShowing || c.HasEnd || h.skipCount(SkipXfer) != 6 {
			t.Fatalf("%+v，傳輸不符 %d 筆", c, h.skipCount(SkipXfer))
		}
		// 等待續段時收到靜音傳輸的 cancel：仍等待續段。
		h = newSubH(t)
		h.at(ipConvDispatch)
		e1 := h.start(600, 3000, 0x1000)
		h.complete(e1, e1.Due)
		x := silence
		x.Kind, x.Step = "cancel", h.f.steps+1
		h.s.SBDMA(x)
		h.idle()
		if c := h.s.Subs[0]; c.State != SubWaiting {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("識別不比 Phys", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		p := e
		p.Kind, p.Step, p.Phys, p.Page = "pause", h.f.steps+1, 0x075FF4, 0x07
		h.s.SBDMA(p)
		h.idle()
		if h.s.Subs[0].State != SubPaused {
			t.Fatal("pause 的 Phys 不同就不作用")
		}
		h2 := newSubH(t)
		h2.at(ipConvDispatch)
		e = h2.start(1000, 5000, 0x1000)
		c := e
		c.Kind, c.Step, c.Phys, c.Page = "cancel", h2.f.steps+1, 0x095FF4, 0x09
		h2.s.SBDMA(c)
		h2.idle()
		if h2.s.Subs[0].Reason != ReasonCancel {
			t.Fatal("cancel 的 Phys 不同就不作用")
		}
	})
	t.Run("入口重置", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		h.at(ipConvEntry)
		if c := h.s.Subs[0]; c.Reason != ReasonReset || c.End != h.f.steps {
			t.Fatalf("%+v", c)
		}
		h.at(ipConvDispatch)
		h.start(1000, 5000, 0x1000)
		h.wantSubs("conv/011/00", "conv/011/00")
		_ = e
		// 未經入口時同 key 且 acc == S：「已播完」。
		h = newSubH(t)
		h.at(ipConvDispatch)
		e = h.start(1000, 5000, 0x1000)
		h.complete(e, e.Due)
		h.at(ipConvDispatch)
		h.start(1000, 5000, 0x1000)
		if h.lastSkip() != SkipPlayed || len(h.s.Subs) != 1 {
			t.Fatalf("%q %d", h.lastSkip(), len(h.s.Subs))
		}
	})
	t.Run("完成核對不符", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		e := h.start(1000, 5000, 0x1000)
		h.complete(e, e.Due-7)
		if c := h.s.Subs[0]; c.State != SubShowing || h.lastSkip() != SkipComplete {
			t.Fatalf("%+v %q", c, h.lastSkip())
		}
	})
	t.Run("停止時顯示中", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		h.start(1000, 5000, 0x1000)
		h.s.Finish()
		c := h.s.Subs[0]
		if c.Reason != ReasonOpen || c.HasEnd {
			t.Fatalf("%+v", c)
		}
		if !strings.Contains(string(SubsTSV(h.s.Subs)), "\t\t未結束\t") {
			t.Fatalf("subs.tsv：%s", SubsTSV(h.s.Subs))
		}
	})
	t.Run("換句", func(t *testing.T) {
		h := newSubH(t)
		h.at(ipConvDispatch)
		h.start(1000, 5000, 0x1000)
		h.setRel(0x88)
		e2 := h.start(500, 3000, 0x2000)
		if c := h.s.Subs[0]; c.Reason != ReasonNext || c.End != e2.Step {
			t.Fatalf("%+v", c)
		}
	})
}

// ---- 第 5 項：代換 ----------------------------------------------------------

func TestSubtitleSubstitution(t *testing.T) {
	h := newSubH(t)
	h.at(ipConvDispatch)
	h.start(1000, 5000, 0x1000)
	if got := h.s.Subs[0].Text; got != "史登：好，ｓａｍ，運輸機已回報。" {
		t.Fatalf("$N：%q", got)
	}
	// $S 在 conv/011/03（說話人 air，前綴 $C）。
	h = newSubH(t)
	h.setRel(0xF8)
	h.at(ipConvDispatch)
	h.start(400, 5000, 0x1000)
	if got := h.s.Subs[0].Text; got != "ａｃｅ：ｌｅｅ，測試三" {
		t.Fatalf("$S、$C：%q", got)
	}
	vals := subValues{S: []byte("lee"), N: []byte("sam"), C: []byte("ace"), W: []byte("Lyle"), P: 2, T: -5}
	cases := []struct {
		tr   string
		w    string
		want string
	}{
		{"$W", "Lyle", "萊爾"},
		{"$W", "", ""},
		{"$W", "Bob", "Ｂｏｂ"},
		{"$P、$T", "Lyle", "２、－５"},
		{"$5", "Lyle", "$5"},
	}
	for _, c := range cases {
		v := vals
		v.W = []byte(c.w)
		got, _ := h.s.substitute(c.tr, v)
		if string(got) != c.want {
			t.Errorf("%q（$W=%q）→ %q，要 %q", c.tr, c.w, string(got), c.want)
		}
	}
	got, bad := Fullwidth([]byte{'a', 0x80, ' ', 'Z'})
	if string(got) != "ａ？　Ｚ" || bad != 1 {
		t.Errorf("全形轉寫 %q bad %d", string(got), bad)
	}
	// 記憶體路徑的 $W：Lyle 對照譯名。
	h = newSubH(t)
	h.setRel(0x1B8)
	h.at(ipConvDispatch)
	h.start(400, 5000, 0x1000)
	if got := h.s.Subs[0].Text; got != "萊爾：萊爾，測試七" {
		t.Fatalf("$W：%q", got)
	}
}

// ---- 第 6 項：版面與組字 ----------------------------------------------------

func TestSubtitleLayout(t *testing.T) {
	text := func(n int) []rune { return []rune(strings.Repeat("中", n)) }
	for _, c := range []struct {
		scale, n                           int
		cw, ch, gx, gy                     int
		wantX, wantY0, wantLines, wantCell []int
	}{
		{2, 10, 8, 10, 0, 2, []int{120}, []int{171}, nil, []int{10}},
		{2, 50, 8, 10, 0, 2, []int{8, 112}, []int{166, 176}, nil, []int{38, 12}},
		{2, 152, 8, 10, 0, 2, []int{8, 8, 8, 8}, []int{156, 166, 176, 186}, nil, []int{38, 38, 38, 38}},
		{3, 10, 6, 7, 1, 2, []int{130}, []int{172}, nil, []int{10}},
		{3, 60, 6, 7, 1, 2, []int{10, 130}, []int{169, 176}, nil, []int{50, 10}},
		{3, 200, 6, 7, 1, 2, []int{10, 10, 10, 10}, []int{162, 169, 176, 183}, nil, []int{50, 50, 50, 50}},
	} {
		ls, _, _ := subLayout(text(c.n), c.scale)
		if len(ls) != len(c.wantX) {
			t.Fatalf("%d 倍 %d 字：%d 行", c.scale, c.n, len(ls))
		}
		for j, l := range ls {
			if l.X != c.wantX[j] || l.Y != c.wantY0[j] || l.Cells != c.wantCell[j] || l.CellW != c.cw || l.CellH != c.ch || l.GlyphX != c.gx || l.GlyphY != c.gy {
				t.Errorf("%d 倍 %d 字第 %d 行：%+v", c.scale, c.n, j, l)
			}
		}
	}
}

func TestWrapLine(t *testing.T) {
	join := func(ls [][]rune) string {
		var s []string
		for _, l := range ls {
			s = append(s, string(l))
		}
		return strings.Join(s, "|")
	}
	r := func(s string) []rune { return []rune(s) }
	a38 := strings.Repeat("甲", 38)
	if got, _ := wrapLine(r(a38), 38); join(got) != a38 {
		t.Errorf("38 格：%s", join(got))
	}
	if got, _ := wrapLine(r(a38+"乙"), 38); join(got) != a38+"|乙" {
		t.Errorf("39 格：%s", join(got))
	}
	a37 := strings.Repeat("甲", 37)
	// 下一行開頭為 。：上一行最後一字移到下一行。
	if got, _ := wrapLine(r(a37+"乙"+"。丙"), 38); join(got) != a37+"|乙。丙" {
		t.Errorf("行首禁則：%s", join(got))
	}
	// 連續兩個行首禁則字。
	a36 := strings.Repeat("甲", 36)
	if got, _ := wrapLine(r(a36+"乙。」丁"), 38); join(got) != a36+"|乙。」丁" {
		t.Errorf("連續行首禁則：%s", join(got))
	}
	// 行尾 「：移到下一行。
	if got, _ := wrapLine(r(a37+"「乙」"), 38); join(got) != a37+"|「乙」" {
		t.Errorf("行尾禁則：%s", join(got))
	}
	// 整行禁則字：強制斷行，不接受一字行。
	s := "甲" + strings.Repeat("。", 40)
	got, forced := wrapLine(r(s), 38)
	if !forced || len(got[0]) != 38 {
		t.Errorf("強制斷行：forced %t 第一行 %d 字", forced, len(got[0]))
	}
	// 第 5 行截斷成 …。
	ls, _, cut := subLayout(r(strings.Repeat("甲", 38*5)), 2)
	if !cut || len(ls) != 4 || ls[3].Text[37] != '…' || ls[3].Text[36] != '甲' {
		t.Errorf("截斷：cut %t %d 行", cut, len(ls))
	}
}

func TestSubtitleCompose(t *testing.T) {
	// 說話人前綴：史登：；speaker/air 為 $C → ａｃｅ：（TestSubtitleSubstitution 已驗）。men 沒有譯文：無前綴、記 miss。
	h := newSubH(t)
	h.setRel(0x172)
	h.at(ipConvDispatch)
	h.start(400, 5000, 0x1000)
	if got := h.s.Subs[0].Text; got != "測試六" || h.skipCount(MissNoSpeaker) != 1 {
		t.Fatalf("men：%q，miss %d", got, h.skipCount(MissNoSpeaker))
	}
	// 缺譯：不顯示、記 miss、不出現英文。
	h = newSubH(t)
	delete(h.s.Catalog, "conv/011/00")
	h.at(ipConvDispatch)
	h.start(1000, 5000, 0x1000)
	if c := h.s.Subs[0]; c.Text != "" || h.skipCount(MissNoTranslation) != 1 || h.visibleAt(h.f.steps) {
		t.Fatalf("缺譯：%+v", c)
	}
	if h.s.Draw(make([]byte, 640*400*4), 2) != nil {
		t.Fatal("缺譯仍畫出")
	}
	// 缺字：整條不顯示、記 miss。
	h = newSubH(t)
	h.s.Catalog["conv/011/00"] = "測試鬱"
	h.at(ipConvDispatch)
	h.start(1000, 5000, 0x1000)
	if c := h.s.Subs[0]; c.Text != "" || h.visibleAt(h.f.steps) {
		t.Fatalf("缺字：%+v", c)
	}
	found := false
	for _, k := range h.s.Skips {
		found = found || strings.HasPrefix(k.Reason, MissSubGlyph)
	}
	if !found {
		t.Fatal("缺字沒有記 miss")
	}
}

// ---- 第 7 項：背景 ----------------------------------------------------------

func TestSubtitleBackground(t *testing.T) {
	h := newSubH(t)
	h.at(ipConvDispatch)
	h.start(1000, 5000, 0x1000)
	h.visibleAt(h.f.steps)
	sc := 2
	W := 320 * sc
	dst := make([]byte, W*200*sc*4)
	for i := 3; i < len(dst); i += 4 {
		dst[i] = 255
	}
	ls := h.s.Subs[0].Lines[sc]
	// 格子內、字形以外的像素：測試字型每列只有左右兩緣各一點，所以格內 x = 5 的輸出像素不是字形。
	x, y := ls[0].X*sc+5, ls[0].Y*sc+ls[0].GlyphY+3
	i := 4 * (y*W + x)
	dst[i], dst[i+1], dst[i+2] = 10, 200, 30
	drawn := h.s.Draw(dst, sc)
	if len(drawn) == 0 {
		t.Fatal("沒有畫")
	}
	if dst[i] != 10 || dst[i+1] != 200 || dst[i+2] != 30 {
		t.Fatalf("格內非字形像素 %v，要保留背景 (10,200,30)", dst[i:i+3])
	}
	// 字形像素是白。
	gx, gy := ls[0].X*sc+ls[0].GlyphX, ls[0].Y*sc+ls[0].GlyphY
	j := 4 * (gy*W + gx)
	if dst[j] != 255 || dst[j+1] != 255 || dst[j+2] != 255 {
		t.Fatalf("字形像素 %v，要白", dst[j:j+3])
	}
}

// ---- 第 8 項：顯示條件 ------------------------------------------------------

func TestSubtitleDisplayConditions(t *testing.T) {
	h := newSubH(t)
	h.at(ipConvDispatch)
	e := h.start(1000, 5000, 0x1000)
	frame := func(row int) []byte {
		f := make([]byte, 64000)
		if row >= 0 {
			f[row*320+17] = 3
		}
		return f
	}
	for _, c := range []struct {
		row  int
		want bool
	}{{152, false}, {199, false}, {151, true}, {-1, true}} {
		h.s.Frame(frame(c.row), e.Step+1)
		if (h.s.Visible() != nil) != c.want {
			t.Errorf("第 %d 列非 0：可見 %t", c.row, !c.want)
		}
	}
	h.s.Frame(make([]byte, 63999), e.Step+1)
	if h.s.Visible() != nil {
		t.Error("長度 63999 視為非黑卻可見")
	}
	// 阻擋只在改變時記：連續 3 個阻擋畫格 1 筆。
	h = newSubH(t)
	h.at(ipConvDispatch)
	e = h.start(1000, 5000, 0x1000)
	for i := 0; i < 3; i++ {
		h.s.Frame(frame(160), e.Step+uint64(i))
	}
	h.s.Frame(frame(-1), e.Step+5)
	if h.skipCount(SkipBlocked) != 1 || h.skipCount(SkipUnblocked) != 1 {
		t.Fatalf("阻擋 %d、恢復 %d", h.skipCount(SkipBlocked), h.skipCount(SkipUnblocked))
	}
	// 條件 1 不成立時不記。
	h = newSubH(t)
	h.s.Frame(frame(160), 10)
	if h.skipCount(SkipBlocked) != 0 {
		t.Fatal("沒有字幕條時記了阻擋")
	}
	// SetEnabled(false)：不畫，字幕條照常建立；再打開時立即可見。
	h = newSubH(t)
	h.s.SetEnabled(false)
	h.at(ipConvDispatch)
	e = h.start(1000, 5000, 0x1000)
	if len(h.s.Subs) != 1 || h.visibleAt(e.Step+1) || h.s.Draw(make([]byte, 640*400*4), 2) != nil {
		t.Fatal("關閉時仍畫或沒建字幕條")
	}
	h.s.SetEnabled(true)
	if h.s.Visible() == nil {
		t.Fatal("再打開時不可見")
	}
	// E = 5790466：播完後 0.4 秒仍可見；下一句開始後換成下一句。
	h = newSubH(t)
	h.s.E = 5790466
	h.at(ipConvDispatch)
	e = h.start(1000, 5000, 0x1000)
	h.complete(e, e.Due)
	if !h.visibleAt(e.Due+4632373) || h.s.Visible().Key != "conv/011/00" {
		t.Fatal("播完後 0.4 秒不可見")
	}
	if h.visibleAt(e.Due + 5790466) {
		t.Fatal("E 之後仍可見")
	}
	h.setRel(0x88)
	h.idleTo(e.Due + 100)
	e2 := h.start(500, 3000, 0x2000)
	if !h.visibleAt(e2.Step) || h.s.Visible().Key != "conv/011/01" {
		t.Fatal("下一句開始後沒有換成下一句")
	}
	// E = 0：播完後不可見。
	h = newSubH(t)
	h.at(ipConvDispatch)
	e = h.start(1000, 5000, 0x1000)
	h.complete(e, e.Due)
	if h.visibleAt(e.Due + 1) {
		t.Fatal("E = 0 時播完後仍可見")
	}
}
