package dkk

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// 規格 007：旅行手札面板。原版經路徑 B 印出「This is recorded as journal entry N.」時，
// 前端在 3D 視窗區畫出中文手札；另有翻閱模式看已讀過的手札。

// JournalEntry 是一則手札（text/journal.zh-TW.tsv）。
type JournalEntry struct{ Title, Body string }

// LoadJournal 讀 text/journal.zh-TW.tsv（entry、title、body、source）；body 段落以字面 \n 分隔。
func LoadJournal(b []byte) (map[int]JournalEntry, error) {
	rows, err := readTSV(b, []string{"entry", "title", "body"})
	if err != nil {
		return nil, fmt.Errorf("dkk: 手札表：%w", err)
	}
	m := make(map[int]JournalEntry, len(rows))
	for i, r := range rows {
		n, err := strconv.Atoi(r[0])
		if err != nil {
			return nil, fmt.Errorf("dkk: 手札表第 %d 列：%w", i+2, err)
		}
		if _, dup := m[n]; dup {
			return nil, fmt.Errorf("dkk: 手札 %d 重複", n)
		}
		m[n] = JournalEntry{Title: r[1], Body: r[2]}
	}
	return m, nil
}

// 面板幾何（規格 007 §2.3）：邏輯像素 [8,312)×[8,128)，38 欄 × 15 列；第 1 列標題、第 15 列頁碼。
const (
	journalX, journalY = 8, 8
	journalCols        = 38
	journalRows        = 15
	journalBodyRows    = journalRows - 2
	JournalMaxPages    = 3
)

var journalMsg = regexp.MustCompile(`^This is recorded as journal entry ([0-9]{1,3})\.$`)

// JournalEntryOf 從路徑 B 字串取出手札編號（規格 007 §2.2）。
func JournalEntryOf(text []byte) (int, bool) {
	m := journalMsg.FindSubmatch(text)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(string(m[1]))
	return n, err == nil
}

// wrapJournal 把段落排成每列 journalCols 格（規格 007 §2.3）。
func wrapJournal(body string) [][]rune { return wrapCols(body, journalCols) }

// wrapCols 把段落（以字面 \n 分隔）排成每列 cols 格：半形合併（規格 005）、拉丁詞與數字不拆、全形句讀不放行首。
func wrapCols(body string, cols int) [][]rune {
	var lines [][]rune
	for _, para := range strings.Split(body, `\n`) {
		rs := PackHalfWidth([]rune(para))
		var line []rune
		for i := 0; i < len(rs); {
			j := i + 1
			if isWordRune(rs[i]) {
				for j < len(rs) && isWordRune(rs[j]) {
					j++
				}
			}
			word := rs[i:j]
			if len(line)+len(word) > cols && len(line) > 0 && len(word) <= cols {
				var carry []rune
				if isClosePunct(word[0]) && len(line) > 1 {
					carry, line = []rune{line[len(line)-1]}, line[:len(line)-1]
				}
				lines = append(lines, line)
				line = carry
			}
			for _, r := range word {
				if len(line) == cols {
					lines = append(lines, line)
					line = nil
				}
				line = append(line, r)
			}
			i = j
		}
		lines = append(lines, line)
	}
	return lines
}

// JournalPages 回排好的頁（每頁最多 journalBodyRows 列）。
func JournalPages(e JournalEntry) [][][]rune {
	lines := wrapJournal(e.Body)
	var pages [][][]rune
	for len(lines) > 0 {
		k := min(journalBodyRows, len(lines))
		pages = append(pages, lines[:k])
		lines = lines[k:]
	}
	return pages
}

// JournalFormat 是面板的固定文字（text/host-text.zh-TW.tsv 的 journal.*）。
type JournalFormat struct{ Title, Page, None string }

// JournalFormatOf 從前端文字表取面板格式；缺任一項回 false。
func JournalFormatOf(host map[string]string) (JournalFormat, bool) {
	f := JournalFormat{Title: host["journal.title"], Page: host["journal.page"], None: host["journal.none"]}
	return f, f.Title != "" && f.Page != "" && f.None != ""
}

// JournalPanel 是面板狀態。自動模式由原版訊息開啟；翻閱模式由前端鍵開啟。
type JournalPanel struct {
	Entries map[int]JournalEntry
	Format  JournalFormat
	Seen    map[int]bool
	OnSeen  func(n int) // 第一次讀到某則時呼叫（前端存已讀清單）

	open, browse bool
	n, page      int
	pages        [][][]rune
	head         uint16 // 自動模式開啟時的 BDA 鍵盤 head
	last         int    // 最近一次自動開過的手札
}

// Open 回面板是否顯示中；Browsing 回是否為翻閱模式。
func (j *JournalPanel) Open() bool     { return j != nil && j.open }
func (j *JournalPanel) Browsing() bool { return j.Open() && j.browse }

func (j *JournalPanel) show(n int) {
	j.n, j.page, j.pages = n, 0, JournalPages(j.Entries[n])
}

// OpenAuto 依原版訊息開面板（規格 007 §2.4）；n 不在表中回 false。
func (j *JournalPanel) OpenAuto(n int, head uint16) bool {
	if _, ok := j.Entries[n]; !ok {
		return false
	}
	j.open, j.browse, j.head, j.last = true, false, head, n
	j.show(n)
	if j.Seen == nil {
		j.Seen = map[int]bool{}
	}
	if !j.Seen[n] {
		j.Seen[n] = true
		if j.OnSeen != nil {
			j.OnSeen(n)
		}
	}
	return true
}

// CloseAuto 關閉自動模式的面板；翻閱模式不受影響。
func (j *JournalPanel) CloseAuto() {
	if j.Open() && !j.browse {
		j.open = false
	}
}

func (j *JournalPanel) closeAutoSafe() {
	if j != nil {
		j.CloseAuto()
	}
}

// KeyTaken 在自動模式下比對 BDA head；遊戲取走了鍵就關閉（規格 007 §2.4 條件 2）。
func (j *JournalPanel) KeyTaken(head uint16) {
	if j.Open() && !j.browse && head != j.head {
		j.open = false
	}
}

// ToggleBrowse 開關翻閱模式（規格 007 §2.5）。
func (j *JournalPanel) ToggleBrowse() {
	if j.Browsing() {
		j.open, j.browse = false, false
		return
	}
	j.open, j.browse = true, true
	n := j.last
	if !j.Seen[n] {
		n = 0
		if s := j.seenList(); len(s) > 0 {
			n = s[len(s)-1]
		}
	}
	j.n, j.page, j.pages = n, 0, nil
	if n != 0 {
		j.show(n)
	}
}

func (j *JournalPanel) seenList() []int {
	var s []int
	for n := range j.Seen {
		if _, ok := j.Entries[n]; ok && j.Seen[n] {
			s = append(s, n)
		}
	}
	sort.Ints(s)
	return s
}

// TurnPage 翻頁（delta ±1）。
func (j *JournalPanel) TurnPage(delta int) {
	if j.Open() {
		j.page = max(0, min(len(j.pages)-1, j.page+delta))
	}
}

// NextEntry 在翻閱模式切到上一則／下一則已讀手札。
func (j *JournalPanel) NextEntry(delta int) {
	if !j.Browsing() {
		return
	}
	s := j.seenList()
	for k, n := range s {
		if n == j.n && k+delta >= 0 && k+delta < len(s) {
			j.show(s[k+delta])
			return
		}
	}
}

// Lines 回目前畫面的 journalRows 列（標題、正文、頁碼列）。
func (j *JournalPanel) Lines() [journalRows][]rune {
	var out [journalRows][]rune
	if !j.Open() {
		return out
	}
	if j.n == 0 {
		out[0] = PackHalfWidth([]rune(j.Format.None))
		return out
	}
	r := strings.NewReplacer("{entry}", strconv.Itoa(j.n), "{title}", j.Entries[j.n].Title,
		"{page}", strconv.Itoa(j.page+1), "{pages}", strconv.Itoa(len(j.pages)))
	out[0] = PackHalfWidth([]rune(r.Replace(j.Format.Title)))
	if j.page < len(j.pages) {
		copy(out[1:], j.pages[j.page])
	}
	if len(j.pages) > 1 {
		p := PackHalfWidth([]rune(r.Replace(j.Format.Page)))
		pad := make([]rune, max(0, journalCols-len(p)))
		for i := range pad {
			pad[i] = ' '
		}
		out[journalRows-1] = append(pad, p...)
	}
	return out
}

var (
	journalBG    = [3]uint8{0, 0, 0}
	journalTitle = [3]uint8{255, 255, 85}
	journalText  = [3]uint8{255, 255, 255}
	journalPage  = [3]uint8{170, 170, 170}
)

// Draw 把面板畫進 scale 倍的 RGBA（320×200 原版畫面）。
func (j *JournalPanel) Draw(dst []uint8, scale int, font *xlate.Font, missing func(rune)) {
	if !j.Open() || font == nil || scale <= 0 {
		return
	}
	w := 320 * scale
	for y := journalY * scale; y < (journalY+8*journalRows)*scale; y++ {
		for x := journalX * scale; x < (journalX+8*journalCols)*scale; x++ {
			putRGB(dst, w, x, y, journalBG)
		}
	}
	for row, line := range j.Lines() {
		c := journalText
		switch row {
		case 0:
			c = journalTitle
		case journalRows - 1:
			c = journalPage
		}
		drawLine(dst, scale, font, journalX, journalY+row*8, journalCols, line, c, missing)
	}
}

// drawLine 以 8×8 邏輯格在 (x0, y0) 起畫一列字（scale 倍輸出），空白不畫；超過 cols 格的部分略去。
func drawLine(dst []uint8, scale int, font *xlate.Font, x0, y0, cols int, line []rune, c [3]uint8, missing func(rune)) {
	w := 320 * scale
	font, k, ox, oy := font.ForScale(scale) // 3× 用 22×22 變體（AddScaledVariants）
	rb := (font.W + 7) / 8
	for col, r := range line {
		if col >= cols || r == ' ' {
			continue
		}
		g, ok := font.Glyphs[r]
		if !ok {
			if missing != nil {
				missing(r)
			}
			continue
		}
		cx, cy := (x0+col*8)*scale, y0*scale
		for gy := 0; gy < font.H; gy++ {
			for gx := 0; gx < font.W; gx++ {
				if g[gy*rb+gx/8]&(0x80>>uint(gx%8)) == 0 {
					continue
				}
				for dy := 0; dy < k; dy++ {
					for dx := 0; dx < k; dx++ {
						if lx, ly := ox+gx*k+dx, oy+gy*k+dy; lx < 8*scale && ly < 8*scale {
							putRGB(dst, w, cx+lx, cy+ly, c)
						}
					}
				}
			}
		}
	}
}

func putRGB(dst []uint8, w, x, y int, c [3]uint8) {
	i := 4 * (y*w + x)
	if x < 0 || x >= w || i < 0 || i+3 >= len(dst) {
		return
	}
	dst[i], dst[i+1], dst[i+2], dst[i+3] = c[0], c[1], c[2], 0xFF
}

// InJournalPanel 回邏輯像素 (x, y) 是否在面板範圍內（收據判斷差異位置用）。
func InJournalPanel(x, y int) bool {
	return x >= journalX && x < journalX+8*journalCols && y >= journalY && y < journalY+8*journalRows
}
