package dkk

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func newQuizA(t *testing.T) (*PathA, fakeMem, *xlate.Layer) {
	t.Helper()
	m := make(fakeMem, 0x100000)
	plantSignature(m, 0x62AA, 0)
	tr := &Translator{Exact: []Lookuper{catalogOf(t, quizAsk, "請問字序 #")}}
	tpl := "template\ttranslation\tnote\n" +
		" of journal entry # {n}     \t在旅行手札第 {n} 則\t\n" +
		" (journal, page {n})?      \t（手札第 {n} 頁）？\t\n"
	if err := tr.LoadTemplates([]byte(tpl)); err != nil {
		t.Fatal(err)
	}
	layer := &xlate.Layer{}
	p, err := NewPathA(m, tr, &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, layer)
	if err != nil {
		t.Fatal(err)
	}
	p.Answers = map[QuizKey]string{{49, 60, 4}: "SNAKES"}
	host, err := LoadHostText([]byte("key\ttranslation\tnote\n" +
		"quiz.entry\t旅行手札第 {entry} 則\t\nquiz.word\t（英文手冊第 {page} 頁）的第 {word} 個字？\t\nquiz.answer\t答案：{answer}\t\n"))
	if err != nil {
		t.Fatal(err)
	}
	var ok bool
	if p.QuizFormat, ok = QuizFormatOf(host); !ok {
		t.Fatal("格式不全")
	}
	return p, m, layer
}

func printA(p *PathA, m fakeMem, step uint64, s string, row, col uint8) {
	b := linear(0x5777, 8)
	m[b] = byte(len(s))
	copy(m[b+1:], s)
	callFrame(m, 0x1CBD, 0x3F00, 0x34CF, 0x1605, 0x5777, 8, 0, 10, row, col)
	p.Before(step, 0x05F4, 0x036A, 0x1CBD, 0x3F00, m)
}

func quizRows(l *xlate.Layer) map[int]string {
	rows := map[int]string{}
	for _, s := range l.Stamps {
		rows[s.Y/8] = strings.TrimRight(string(s.Text), " ")
	}
	return rows
}

func askQuiz(p *PathA, m fakeMem, word string) {
	printA(p, m, 1, quizAsk, 2, 3)
	printA(p, m, 2, word, 2, 19)
	printA(p, m, 3, " of journal entry # 60     ", 4, 3)
	printA(p, m, 4, " (journal, page 49)?      ", 6, 3)
}

func TestQuizRewritesWithAnswer(t *testing.T) {
	p, m, l := newQuizA(t)
	askQuiz(p, m, "4    ")
	r := quizRows(l)
	want := map[int]string{
		2: string(PackHalfWidth([]rune("旅行手札第 60 則"))),
		4: string(PackHalfWidth([]rune("（英文手冊第 49 頁）的第 4 個字？"))),
		6: string(PackHalfWidth([]rune("答案：SNAKES"))),
	}
	for row, w := range want {
		if r[row] != w {
			t.Errorf("第 %d 列 %q，要 %q", row, r[row], w)
		}
	}
	for _, s := range l.Stamps {
		if s.Y == 16 && s.Cells != 21 {
			t.Errorf("第 2 列應延伸到字序結尾（21 格），得到 %d", s.Cells)
		}
	}
	p.SetHideAnswers(true)
	if r := quizRows(l); r[6] != "" {
		t.Errorf("隱藏答案後第 6 列應空白：%q", r[6])
	}
}

func TestQuizUnknownAnswerAndHidden(t *testing.T) {
	p, m, l := newQuizA(t)
	p.HideAnswers = true
	askQuiz(p, m, "4    ")
	if r := quizRows(l); r[6] != "" || r[2] == "" {
		t.Errorf("隱藏答案：%q", r)
	}
	p, m, l = newQuizA(t)
	askQuiz(p, m, "7    ")
	if r := quizRows(l); r[6] != "" || !strings.Contains(r[4], string(PackHalfWidth([]rune("第 7 個")))) {
		t.Errorf("表中無答案：%q", r)
	}
}

func TestQuizInterruptedKeepsNormalResult(t *testing.T) {
	p, m, l := newQuizA(t)
	printA(p, m, 1, quizAsk, 2, 3)
	printA(p, m, 2, "4    ", 2, 19)
	printA(p, m, 3, "ANNA", 10, 3) // 中間插入別的呼叫
	printA(p, m, 4, " of journal entry # 60     ", 4, 3)
	printA(p, m, 5, " (journal, page 49)?      ", 6, 3)
	if r := quizRows(l); r[2] != string(PackHalfWidth([]rune("請問字序 #"))) || p.QuizShown != nil {
		t.Errorf("順序被打斷不該改寫：%q", r)
	}
}
