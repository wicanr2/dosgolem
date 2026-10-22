package strikecommander

import (
	"fmt"
	"strconv"

	"github.com/wicanr2/dosgolem/xlate"
)

// 語音字幕的代換、版面與反鋸齒字（strike_commander 規格 011 §3.6.7）：半形代換、像素換行（wrapPixels）、
// 最多 3 行與附加截斷、行距 14 原版列、行內置中，以 DrawAlphaText 畫白字。

// 011 §3.6.7 的常數。
const (
	subTextPitch    = 14 // 行距（原版列）
	subTextMaxRows  = 3  // 最多行數
	MissEmptyString = "miss 空字串"
)

// NewSubtitlesText 建字幕（011 §3.9）：index、fonts 不得為 nil。T 為 0 時取 SubtitleDefaultT。預設開啟（SetEnabled）。
func NewSubtitlesText(index *SpeechIndex, catalog map[string]string, fonts *TextFonts, e, t uint64) (*Subtitles, error) {
	if index == nil || fonts == nil {
		return nil, fmt.Errorf("strikecommander: 字幕缺結構表或圖集")
	}
	if t == 0 {
		t = SubtitleDefaultT
	}
	return &Subtitles{Index: index, Catalog: catalog, Text: fonts, E: e, T: t, enabled: true}, nil
}

// halfBytes 是 011 §3.6.7 的代換：位元組 21h–7Eh 照 ASCII，20h 為 U+0020，其他換成「？」（bad 計數）。
func halfBytes(b []byte) (out []rune, bad int) {
	for _, c := range b {
		switch {
		case c >= 0x20 && c <= 0x7E:
			out = append(out, rune(c))
		default:
			out = append(out, '？')
			bad++
		}
	}
	return out, bad
}

// substituteHalf 把 tr 的代換符號換成玩家資料（011 §3.6.7，取代 010 §3.6 的全形轉寫）：
// $S $N $C 與 $W 的非僚機字串照 halfBytes；$P、$T 為半形數字、負號 -；僚機名換成 speaker/<名> 的譯文。
func (s *Subtitles) substituteHalf(tr string, vals subValues) (out []rune, bad int) {
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
			add, b = halfBytes(vals.S)
		case 'N':
			add, b = halfBytes(vals.N)
		case 'C':
			add, b = halfBytes(vals.C)
		case 'W':
			if k, ok := wingmen[string(vals.W)]; ok && s.Catalog[k] != "" {
				add = []rune(s.Catalog[k])
			} else {
				add, b = halfBytes(vals.W)
			}
		case 'P':
			add = []rune(strconv.Itoa(int(vals.P)))
		case 'T':
			add = []rune(strconv.Itoa(int(vals.T)))
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

// composeText 組字串（011 §3.6.7）：說話人顯示名＋「：」＋代換後的譯文（N() 前）。缺譯、N() 後缺字時記 miss、回 false。
func (s *Subtitles) composeText(step uint64, line *SpeechLine, vals subValues) ([]rune, bool) {
	tr, ok := s.Catalog[line.Key]
	if !ok || tr == "" {
		s.skip(step, MissNoTranslation, line.Key)
		return nil, false
	}
	var text []rune
	bad := 0
	if sp, ok := s.Catalog["speaker/"+line.Speaker]; ok && sp != "" {
		p, b := s.substituteHalf(sp, vals)
		text = append(p, '：')
		bad += b
	} else {
		s.skip(step, MissNoSpeaker, line.Key)
	}
	body, b := s.substituteHalf(tr, vals)
	text = append(text, body...)
	bad += b
	if bad != 0 {
		s.skip(step, MissFullwidth, line.Key)
	}
	for _, r := range NarrowAlnum(text) {
		ok := s.Text.Chars[r]
		for _, sc := range []int{2, 3} {
			if f := s.Text.Font(pxSub[sc]); f == nil || f.Glyphs[r] == nil {
				ok = false
			}
		}
		if !ok {
			s.Skips = append(s.Skips, SubSkip{Step: step, Reason: MissSubGlyph + " " + string(r), Key: line.Key})
			return nil, false
		}
	}
	return text, true
}

// truncateLines 是 011 §3.6.7 的最多 3 行與附加截斷：超過 3 行時保留前 3 行，第 3 行從行尾逐字刪，
// 直到「刪後的第 3 行（去掉行尾 U+0020）＋…」的寬 ≤ maxW，再附加 …。cut 回是否截斷。
func truncateLines(lines [][]rune, f *xlate.AlphaFont, maxW int) (out [][]rune, cut bool) {
	if len(lines) <= subTextMaxRows {
		return lines, false
	}
	out = append([][]rune(nil), lines[:subTextMaxRows]...)
	third := append([]rune(nil), out[subTextMaxRows-1]...)
	trim := func(rs []rune) []rune {
		for len(rs) > 0 && rs[len(rs)-1] == ' ' {
			rs = rs[:len(rs)-1]
		}
		return rs
	}
	ell, _ := f.Advance([]rune{'…'})
	for {
		third = trim(third)
		w, _ := f.Advance(third)
		if len(third) == 0 || w+ell <= maxW {
			break
		}
		third = third[:len(third)-1]
	}
	out[subTextMaxRows-1] = append(third, '…')
	return out, true
}

// subTextLayout 是 011 §3.6.7 的版面：n 行，y0 = 152s + ⌊(48s − n × 14s) / 2⌋，第 j 行上緣 y0 + j × 14s，
// em 上緣 = 行上緣 + ⌊(14s − px) / 2⌋，baseline = em 上緣 + asc(px)；每行 tx = ⌊(320s − Wline) / 2⌋。
func subTextLayout(lines [][]rune, f *xlate.AlphaFont, scale int) []SubLine {
	n := len(lines)
	y0 := 152*scale + floorDiv(48*scale-n*subTextPitch*scale, 2)
	var out []SubLine
	for j, l := range lines {
		w, _ := f.Advance(l)
		top := y0 + j*subTextPitch*scale
		base := top + (subTextPitch*scale-f.Px)/2 + TextAscent(f.Px)
		out = append(out, SubLine{Text: append([]rune(nil), l...), Px: f.Px, TxOut: floorDiv(320*scale-w, 2), Baseline: base})
	}
	return out
}

// subTextLines 把（代換後、N() 前的）字串排成 scale 倍的各行：N()、wrapPixels、截斷與版面。
func subTextLines(text []rune, f *xlate.AlphaFont, scale int) (lines []SubLine, forced, cut, empty bool) {
	maxW := 300 * scale
	ls, forced := wrapPixels(NarrowAlnum(text), f, maxW)
	if len(ls) == 0 {
		return nil, forced, false, true
	}
	ls, cut = truncateLines(ls, f, maxW)
	return subTextLayout(ls, f, scale), forced, cut, false
}

// newSubText 組字串，2、3 倍各自換行；miss 強制斷行、miss 截斷 2、3 倍各記一筆並帶倍率（011 §3.6.7）。
func (s *Subtitles) newSubText(c *Subtitle, step uint64, line *SpeechLine, vals subValues) {
	text, ok := s.composeText(step, line, vals)
	if !ok {
		return
	}
	c.Text = string(text)
	c.Lines, c.Forced, c.Truncated = map[int][]SubLine{}, map[int]bool{}, map[int]bool{}
	for _, sc := range []int{2, 3} {
		lines, forced, cut, empty := subTextLines(text, s.Text.Font(pxSub[sc]), sc)
		if empty {
			s.Skips = append(s.Skips, SubSkip{Step: step, Reason: MissEmptyString, Key: line.Key, Scale: sc})
		}
		if forced {
			s.Skips = append(s.Skips, SubSkip{Step: step, Reason: MissForcedBreak, Key: line.Key, Scale: sc})
		}
		if cut {
			s.Skips = append(s.Skips, SubSkip{Step: step, Reason: MissTruncated, Key: line.Key, Scale: sc})
		}
		c.Forced[sc], c.Truncated[sc] = forced, cut
		c.Lines[sc] = lines
	}
}

// subRegionClip 是字幕區 [0, 320s) × [152s, 200s)（輸出像素）。
func subRegionClip(scale int) xlate.AlphaRect {
	return xlate.AlphaRect{X0: 0, Y0: 152 * scale, X1: 320 * scale, Y1: 200 * scale}
}

// drawText 以 DrawAlphaText 把各行畫進 dst（白字、無外框），回畫出的各行（Ink 為該行的文字框）。
func (s *Subtitles) drawText(c *Subtitle, dst []byte, scale int) []SubLine {
	f := s.Text.Font(pxSub[scale])
	clip := subRegionClip(scale)
	var out []SubLine
	for _, l := range c.Lines[scale] {
		l.Ink, _, _ = xlate.DrawAlphaText(dst, 320*scale, 200*scale, f, l.Text, l.TxOut, l.Baseline, xlate.AlphaFill, [3]uint8{255, 255, 255}, clip)
		out = append(out, l)
	}
	return out
}

// SubtitleRegionCheck 是 011 §3.9 的字幕區檢查結果。
type SubtitleRegionCheck struct {
	Diffs, Outside, InkPixels int
	Visible                   bool
}

// CheckSubtitleRegion 比較畫字幕之後的 after 與之前的 pre（011 §3.9）：每個不同的像素都要落在 inks 的聯集內，
// 且聯集在輸出列 [152s, 200s) 內；回差異數、框外數、聯集像素數與 visible。
func CheckSubtitleRegion(after, pre []byte, scale int, inks []xlate.AlphaRect, visible bool) SubtitleRegionCheck {
	c := SubtitleRegionCheck{Visible: visible}
	W := 320 * scale
	region := subRegionClip(scale)
	in := func(x, y int) bool {
		if x < region.X0 || x >= region.X1 || y < region.Y0 || y >= region.Y1 {
			return false
		}
		for _, b := range inks {
			if x >= b.X0 && x < b.X1 && y >= b.Y0 && y < b.Y1 {
				return true
			}
		}
		return false
	}
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < W; x++ {
			if in(x, y) {
				c.InkPixels++
			}
		}
	}
	for i := 0; i+3 < len(after) && i+3 < len(pre); i += 4 {
		if after[i] == pre[i] && after[i+1] == pre[i+1] && after[i+2] == pre[i+2] && after[i+3] == pre[i+3] {
			continue
		}
		c.Diffs++
		if !in((i/4)%W, (i/4)/W) {
			c.Outside++
		}
	}
	return c
}

// subShotInks 回 SubShot 某倍率各行的文字框。
func subShotInks(ss SubShot, scale int) []xlate.AlphaRect {
	var out []xlate.AlphaRect
	for _, l := range ss.Lines[scale] {
		if !l.Ink.Empty() {
			out = append(out, l.Ink)
		}
	}
	return out
}

// NewSubtitleMetaText 組 subsmeta（011 §3.8）：記結構表、譯文的雜湊與兩個倍率圖集的名稱、字級與 SHA-256。
func NewSubtitleMetaText(fonts *TextFonts, indexB, catalogB []byte, e, t uint64) SubtitleMeta {
	m := SubtitleMeta{IndexSHA256: sha256Hex(indexB), CatalogSHA256: sha256Hex(catalogB), E: e, T: t}
	for i, sc := range []int{2, 3} {
		px := pxSub[sc]
		m.TextName[i], m.TextPx[i] = TextFontName(px), px
		if fonts != nil {
			for _, ff := range fonts.Files {
				if ff.Name == TextAtlasName(px) {
					m.TextSHA256[i] = ff.SHA256
				}
			}
		}
	}
	return m
}
