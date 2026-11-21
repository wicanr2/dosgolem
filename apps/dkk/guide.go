package dkk

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// 規格 008：遊戲內攻略面板。前端 F6 開關，內容是 text/walkthrough.zh-TW.tsv 的章節，可翻頁、換章。

// GuideChapter 是一章攻略。
type GuideChapter struct{ Title, Body, Source string }

// LoadGuide 讀 text/walkthrough.zh-TW.tsv（chapter、title、body、source），依檔案順序。
func LoadGuide(b []byte) ([]GuideChapter, error) {
	rows, err := readTSV(b, []string{"chapter", "title", "body", "source"})
	if err != nil {
		return nil, fmt.Errorf("dkk: 攻略表：%w", err)
	}
	out := make([]GuideChapter, 0, len(rows))
	for i, r := range rows {
		if n, err := strconv.Atoi(r[0]); err != nil || n != i+1 {
			return nil, fmt.Errorf("dkk: 攻略表第 %d 列章號要是 %d", i+2, i+1)
		}
		out = append(out, GuideChapter{Title: r[1], Body: r[2], Source: r[3]})
	}
	return out, nil
}

// 面板幾何（規格 008 §2.2）：[0,320)×[0,192)，40 欄 × 24 列；第 1 列標題、第 24 列頁碼與出處。
const (
	guideCols     = 40
	guideRows     = 24
	guideBodyRows = guideRows - 2
)

// GuideFormat 是面板的固定文字（text/host-text.zh-TW.tsv 的 guide.*）。
type GuideFormat struct{ Title, Page, TOC, TOCLine, TOCHint string }

// GuideFormatOf 從前端文字表取面板格式；缺任一項回 false。
func GuideFormatOf(host map[string]string) (GuideFormat, bool) {
	f := GuideFormat{host["guide.title"], host["guide.page"], host["guide.toc"], host["guide.tocline"], host["guide.tochint"]}
	return f, f.Title != "" && f.Page != "" && f.TOC != "" && f.TOCLine != "" && f.TOCHint != ""
}

// GuidePanel 是攻略面板狀態。ch 0 是目錄頁，1 起是各章。
type GuidePanel struct {
	Chapters []GuideChapter
	Format   GuideFormat
	OnMove   func(ch, page int) // 位置改變時呼叫（前端存檔）

	open     bool
	ch, page int
	pages    [][][]rune
	opened   bool // 開過一次後再開回到上次位置
}

// Open 回面板是否顯示中。
func (g *GuidePanel) Open() bool { return g != nil && g.open }

// Toggle 開關面板（規格 008 §2.1）。
func (g *GuidePanel) Toggle() {
	if g == nil {
		return
	}
	if g.open {
		g.open = false
		return
	}
	g.open = true
	if !g.opened {
		g.opened = true
		g.show(g.ch, g.page)
	}
}

// Close 關閉面板。
func (g *GuidePanel) Close() {
	if g != nil {
		g.open = false
	}
}

// SetPos 設定位置（讀回存檔用）；超出範圍的值會夾回。
func (g *GuidePanel) SetPos(ch, page int) {
	g.opened = true
	g.show(ch, page)
}

// Pos 回目前的章與頁（0 起）。
func (g *GuidePanel) Pos() (int, int) { return g.ch, g.page }

func (g *GuidePanel) show(ch, page int) {
	ch = max(0, min(len(g.Chapters), ch))
	g.ch = ch
	if ch == 0 {
		g.pages = paginate(g.tocLines())
	} else {
		g.pages = paginate(wrapCols(g.Chapters[ch-1].Body, guideCols))
	}
	g.page = max(0, min(len(g.pages)-1, page))
	if g.OnMove != nil {
		g.OnMove(g.ch, g.page)
	}
}

func paginate(lines [][]rune) [][][]rune {
	var pages [][][]rune
	for len(lines) > 0 {
		k := min(guideBodyRows, len(lines))
		pages = append(pages, lines[:k])
		lines = lines[k:]
	}
	if len(pages) == 0 {
		pages = [][][]rune{nil}
	}
	return pages
}

func (g *GuidePanel) tocLines() [][]rune {
	var out [][]rune
	for i, c := range g.Chapters {
		r := strings.NewReplacer("{n}", strconv.Itoa(i+1), "{title}", c.Title, "{source}", c.Source)
		out = append(out, wrapCols(r.Replace(g.Format.TOCLine), guideCols)...)
	}
	out = append(out, nil)
	return append(out, wrapCols(g.Format.TOCHint, guideCols)...)
}

// TurnPage 翻頁；翻過章末接下一章第一頁，翻過章首接上一章最後一頁。
func (g *GuidePanel) TurnPage(delta int) {
	if !g.Open() {
		return
	}
	switch p := g.page + delta; {
	case p >= len(g.pages) && g.ch < len(g.Chapters):
		g.show(g.ch+1, 0)
	case p < 0 && g.ch > 0:
		g.show(g.ch-1, 1<<30)
	default:
		g.show(g.ch, p)
	}
}

// NextChapter 換章（delta ±1），到該章第一頁。
func (g *GuidePanel) NextChapter(delta int) {
	if g.Open() {
		g.show(g.ch+delta, 0)
	}
}

// Lines 回目前畫面的 guideRows 列。
func (g *GuidePanel) Lines() [guideRows][]rune {
	var out [guideRows][]rune
	if !g.Open() {
		return out
	}
	title, source := g.Format.TOC, ""
	if g.ch > 0 {
		c := g.Chapters[g.ch-1]
		title = strings.NewReplacer("{n}", strconv.Itoa(g.ch), "{title}", c.Title).Replace(g.Format.Title)
		source = c.Source
	}
	out[0] = PackHalfWidth([]rune(title))
	copy(out[1:], g.pages[g.page])
	r := strings.NewReplacer("{page}", strconv.Itoa(g.page+1), "{pages}", strconv.Itoa(len(g.pages)), "{source}", source)
	p := PackHalfWidth([]rune(r.Replace(g.Format.Page)))
	pad := make([]rune, max(0, guideCols-len(p)))
	for i := range pad {
		pad[i] = ' '
	}
	out[guideRows-1] = append(pad, p...)
	return out
}

// Draw 把面板畫進 scale 倍的 RGBA（320×200 原版畫面）。
func (g *GuidePanel) Draw(dst []uint8, scale int, font *xlate.Font, missing func(rune)) {
	if !g.Open() || font == nil || scale <= 0 {
		return
	}
	w := 320 * scale
	for y := 0; y < 8*guideRows*scale; y++ {
		for x := 0; x < 8*guideCols*scale; x++ {
			putRGB(dst, w, x, y, journalBG)
		}
	}
	for row, line := range g.Lines() {
		c := journalText
		switch row {
		case 0:
			c = journalTitle
		case guideRows - 1:
			c = journalPage
		}
		drawLine(dst, scale, font, 0, row*8, guideCols, line, c, missing)
	}
}

// InGuidePanel 回邏輯像素 (x, y) 是否在面板範圍內（收據判斷差異位置用）。
func InGuidePanel(x, y int) bool { return x >= 0 && x < 8*guideCols && y >= 0 && y < 8*guideRows }

// LoadGuideDir 從譯文目錄讀攻略表與面板格式；沒有 walkthrough.zh-TW.tsv 時回 nil（不啟用）。
func LoadGuideDir(dir string) (*GuidePanel, error) {
	b, err := os.ReadFile(filepath.Join(dir, "walkthrough.zh-TW.tsv"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	chs, err := LoadGuide(b)
	if err != nil {
		return nil, err
	}
	if b, err = os.ReadFile(filepath.Join(dir, "host-text.zh-TW.tsv")); err != nil {
		return nil, err
	}
	host, err := LoadHostText(b)
	if err != nil {
		return nil, err
	}
	f, ok := GuideFormatOf(host)
	if !ok {
		return nil, fmt.Errorf("dkk: host-text.zh-TW.tsv 缺 guide.title／page／toc／tocline／tochint")
	}
	return &GuidePanel{Chapters: chs, Format: f}, nil
}

// ReadGuidePos 讀存檔目錄的攻略位置（`章 頁`）；檔案不存在回 0, 0。
func ReadGuidePos(path string) (int, int, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	var ch, page int
	if _, err := fmt.Sscan(string(b), &ch, &page); err != nil {
		return 0, 0, fmt.Errorf("dkk: %s：%w", path, err)
	}
	return ch, page, nil
}

// WriteGuidePos 寫攻略位置。
func WriteGuidePos(path string, ch, page int) error {
	return os.WriteFile(path, []byte(fmt.Sprintf("%d %d\n", ch, page)), 0o644)
}
