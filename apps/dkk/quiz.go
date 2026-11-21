package dkk

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// 規格 006：手冊查驗題。路徑 A 依序印四段（問句、字序、手札、頁），第 4 段時改寫成一段中文並附答案。

// QuizKey 是答案表的索引：英文手冊頁、手札編號、字序。
type QuizKey struct{ Page, Entry, Word int }

// LoadQuizAnswers 讀 text/quiz-answers.tsv（page、entry、word、answer）。
func LoadQuizAnswers(b []byte) (map[QuizKey]string, error) {
	rows, err := readTSV(b, []string{"page", "entry", "word", "answer"})
	if err != nil {
		return nil, fmt.Errorf("dkk: 查驗答案表：%w", err)
	}
	m := make(map[QuizKey]string, len(rows))
	for i, r := range rows {
		var k [3]int
		for j := range k {
			if k[j], err = strconv.Atoi(r[j]); err != nil {
				return nil, fmt.Errorf("dkk: 查驗答案表第 %d 列：%w", i+2, err)
			}
		}
		m[QuizKey{k[0], k[1], k[2]}] = r[3]
	}
	return m, nil
}

// LoadQuizAnswersFile 讀答案表檔案。
func LoadQuizAnswersFile(path string) (map[QuizKey]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadQuizAnswers(b)
}

var (
	quizAsk   = " what is word # "
	quizWord  = regexp.MustCompile(`^ *([0-9]+) *$`)
	quizEntry = regexp.MustCompile(`^ of journal entry # ([0-9]+) +$`)
	quizPage  = regexp.MustCompile(`^ \(journal, page ([0-9]+)\)\? +$`)
)

type quizTrack struct {
	stage      int
	row, col   int
	word, n    int
	ask, entry *xlate.Stamp
	bg         uint8
}

// QuizFormat 是改寫後三列的格式（text/host-text.zh-TW.tsv 的 quiz.entry、quiz.word、quiz.answer），
// 佔位 {entry}、{page}、{word}、{answer}。
type QuizFormat [3]string

// LoadHostText 讀 text/host-text.zh-TW.tsv（key、translation、note）：前端自己畫的固定中文。
func LoadHostText(b []byte) (map[string]string, error) {
	rows, err := readTSV(b, []string{"key", "translation"})
	if err != nil {
		return nil, fmt.Errorf("dkk: 前端文字表：%w", err)
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r[0]] = r[1]
	}
	return m, nil
}

// QuizFormatOf 從前端文字表取查驗題格式；缺任一項回 false。
func QuizFormatOf(host map[string]string) (QuizFormat, bool) {
	var f QuizFormat
	for i, k := range []string{"quiz.entry", "quiz.word", "quiz.answer"} {
		if f[i] = host[k]; f[i] == "" {
			return f, false
		}
	}
	return f, true
}

// lines 是改寫後的三列（規格 006 §2.2）。
func (f QuizFormat) lines(k QuizKey, answer string, show bool) [3]string {
	r := strings.NewReplacer("{entry}", strconv.Itoa(k.Entry), "{page}", strconv.Itoa(k.Page),
		"{word}", strconv.Itoa(k.Word), "{answer}", answer)
	l := [3]string{r.Replace(f[0]), r.Replace(f[1])}
	if show && answer != "" {
		l[2] = r.Replace(f[2])
	}
	return l
}

func fitText(s string, cells int) ([]rune, bool) {
	r := PackHalfWidth([]rune(s))
	if len(r) > cells {
		return nil, false
	}
	for len(r) < cells {
		r = append(r, ' ')
	}
	return r, true
}

// observeQuiz 在每次路徑 A 呼叫後呼叫；st 是這次加入的 Stamp（未覆繪為 nil）。
func (p *PathA) observeQuiz(text []byte, row, col, bg int, st *xlate.Stamp) {
	q := &p.quiz
	s := string(text)
	switch {
	case q.stage == 1 && row == q.row && col == q.col+len(quizAsk) && quizWord.MatchString(s):
		q.word, _ = strconv.Atoi(quizWord.FindStringSubmatch(s)[1])
		q.stage = 2
		return
	case q.stage == 2 && st != nil && quizEntry.MatchString(s):
		q.n, _ = strconv.Atoi(quizEntry.FindStringSubmatch(s)[1])
		q.entry, q.stage = st, 3
		return
	case q.stage == 3 && st != nil && quizPage.MatchString(s) && p.QuizFormat[0] != "":
		page, _ := strconv.Atoi(quizPage.FindStringSubmatch(s)[1])
		p.finishQuiz(QuizKey{page, q.n, q.word}, st)
	}
	*q = quizTrack{}
	if s == quizAsk && st != nil {
		*q = quizTrack{stage: 1, row: row, col: col, ask: st, bg: uint8(bg)}
	}
}

func (p *PathA) finishQuiz(k QuizKey, last *xlate.Stamp) {
	q := &p.quiz
	ans := p.Answers[k]
	lines := p.QuizFormat.lines(k, ans, !p.HideAnswers)
	askCells := len(quizAsk) + 5
	t0, ok0 := fitText(lines[0], askCells)
	t1, ok1 := fitText(lines[1], q.entry.Cells)
	t2, ok2 := fitText(lines[2], last.Cells)
	if !ok0 || !ok1 || !ok2 {
		return
	}
	// 第 1 列延伸到字序數字結尾：同原點換成較長的 Stamp。
	ask := &xlate.Stamp{Key: "quiz.ask", X: q.ask.X, Y: q.ask.Y, Cells: askCells, CellW: 8, CellH: 8,
		Font: p.Font, Text: t0, State: xlate.Pending}
	p.Layer.Replace(ask)
	p.bgOf[ask] = q.bg
	q.entry.Text, last.Text = t1, t2
	p.QuizShown = &QuizShown{Key: k, Answer: ans, stamp: last}
}

// QuizShown 記最近一次改寫的查驗題，供切換答案顯示。
type QuizShown struct {
	Key    QuizKey
	Answer string
	stamp  *xlate.Stamp
}

// SetHideAnswers 切換答案顯示（只改覆繪，規格 006 §2.2）。
func (p *PathA) SetHideAnswers(hide bool) {
	p.HideAnswers = hide
	if s := p.QuizShown; s != nil {
		if t, ok := fitText(p.QuizFormat.lines(s.Key, s.Answer, !hide)[2], s.stamp.Cells); ok {
			s.stamp.Text = t
		}
	}
}
