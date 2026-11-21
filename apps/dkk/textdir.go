package dkk

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LoadTextDir 從譯文目錄載入介面查表（含規格 004 的名稱、模板、物品名）與 ECL catalog。
// 檔名：ui-text-events.tsv、ui-text.zh-TW.tsv、ecl-text-events.tsv、ecl-text.zh-TW.tsv、
// name-events.tsv、name.zh-TW.tsv、composed-templates.tsv、menu-masters.tsv。
func LoadTextDir(dir string) (ui *Translator, ecl *Catalog, err error) {
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, name)) }
	load := func(events, trans string) (*Catalog, error) {
		e, err := read(events)
		if err != nil {
			return nil, err
		}
		t, err := read(trans)
		if err != nil {
			return nil, err
		}
		return LoadCatalog(e, t)
	}
	uiCat, err := load("ui-text-events.tsv", "ui-text.zh-TW.tsv")
	if err != nil {
		return nil, nil, err
	}
	if ecl, err = load("ecl-text-events.tsv", "ecl-text.zh-TW.tsv"); err != nil {
		return nil, nil, err
	}
	names, err := load("name-events.tsv", "name.zh-TW.tsv")
	if err != nil {
		return nil, nil, err
	}
	ui = &Translator{Exact: []Lookuper{uiCat}, Names: names, Words: uiCat}
	mm, err := read("menu-masters.tsv")
	if err != nil {
		return nil, nil, err
	}
	if ui.Menus, err = LoadMenuMasters(mm); err != nil {
		return nil, nil, err
	}
	tpl, err := read("composed-templates.tsv")
	if err != nil {
		return nil, nil, err
	}
	if err := ui.LoadTemplates(tpl); err != nil {
		return nil, nil, err
	}
	return ui, ecl, nil
}

// LoadQuizDir 從譯文目錄讀查驗題答案表與前端文字表的格式（規格 006）。
func LoadQuizDir(dir string) (map[QuizKey]string, QuizFormat, error) {
	ans, err := LoadQuizAnswersFile(filepath.Join(dir, "quiz-answers.tsv"))
	if err != nil {
		return nil, QuizFormat{}, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "host-text.zh-TW.tsv"))
	if err != nil {
		return nil, QuizFormat{}, err
	}
	host, err := LoadHostText(b)
	if err != nil {
		return nil, QuizFormat{}, err
	}
	f, ok := QuizFormatOf(host)
	if !ok {
		return nil, QuizFormat{}, fmt.Errorf("dkk: host-text.zh-TW.tsv 缺 quiz.entry／quiz.word／quiz.answer")
	}
	return ans, f, nil
}

// LoadJournalDir 從譯文目錄讀手札表與面板格式（規格 007）。
func LoadJournalDir(dir string) (*JournalPanel, error) {
	b, err := os.ReadFile(filepath.Join(dir, "journal.zh-TW.tsv"))
	if err != nil {
		return nil, err
	}
	entries, err := LoadJournal(b)
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
	f, ok := JournalFormatOf(host)
	if !ok {
		return nil, fmt.Errorf("dkk: host-text.zh-TW.tsv 缺 journal.title／journal.page／journal.none")
	}
	return &JournalPanel{Entries: entries, Format: f, Seen: map[int]bool{}}, nil
}

// ReadJournalSeen 讀已讀手札清單（每行一個編號）；檔案不存在時回空集合。
func ReadJournalSeen(path string) (map[int]bool, error) {
	seen := map[int]bool{}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return seen, nil
	}
	if err != nil {
		return nil, err
	}
	for i, l := range strings.Fields(string(b)) {
		n, err := strconv.Atoi(l)
		if err != nil {
			return nil, fmt.Errorf("dkk: %s 第 %d 筆：%w", path, i+1, err)
		}
		seen[n] = true
	}
	return seen, nil
}

// AppendJournalSeen 在已讀清單加一筆。
func AppendJournalSeen(path string, n int) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, n)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
