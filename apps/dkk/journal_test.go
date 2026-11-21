package dkk

import (
	"os"
	"strings"
	"testing"
)

func TestJournalEntryOf(t *testing.T) {
	for in, want := range map[string]int{
		"This is recorded as journal entry 7.":   7,
		"This is recorded as journal entry 60.":  60,
		"This is recorded as journal entry 123.": 123,
	} {
		if n, ok := JournalEntryOf([]byte(in)); !ok || n != want {
			t.Errorf("%q → %d %v", in, n, ok)
		}
	}
	for _, in := range []string{"This is recorded as journal entry .", "recorded as journal entry 7.", "This is recorded as journal entry 7"} {
		if _, ok := JournalEntryOf([]byte(in)); ok {
			t.Errorf("%q 不該命中", in)
		}
	}
}

func TestWrapJournal(t *testing.T) {
	long := strings.Repeat("字", journalCols-1) + "。後"
	lines := wrapJournal(long + `\n` + "第二段 Enter 鍵")
	if len(lines) != 3 {
		t.Fatalf("要 3 列，得到 %d：%q", len(lines), lines)
	}
	if string(lines[0]) != strings.Repeat("字", journalCols-1)+"。" {
		t.Errorf("句號剛好放得下時留在行尾：%q", string(lines[0]))
	}
	lines = wrapJournal(strings.Repeat("字", journalCols) + "。")
	if len(lines) != 2 || string(lines[1]) != "字。" {
		t.Errorf("句號不放行首，前一字帶下來：%q", lines)
	}
	lines = wrapJournal(strings.Repeat("字", journalCols-2) + "Sudulto")
	if len(lines) != 2 || len(lines[0]) != journalCols-2 {
		t.Errorf("拉丁詞不拆行：%q", lines)
	}
	if string(wrapJournal("第二段 Enter 鍵")[0]) != string(PackHalfWidth([]rune("第二段 Enter 鍵"))) {
		t.Error("正文要半形合併")
	}
}

func newPanel() *JournalPanel {
	body := strings.Repeat("字", journalCols*journalBodyRows+5)
	return &JournalPanel{
		Entries: map[int]JournalEntry{1: {"甲", "短"}, 5: {"乙", body}, 9: {"丙", "短"}},
		Format:  JournalFormat{Title: "旅行手札第 {entry} 則：{title}", Page: "第 {page}／{pages} 頁", None: "尚未記錄任何手札"},
	}
}

func TestJournalAutoClose(t *testing.T) {
	j := newPanel()
	var seen []int
	j.OnSeen = func(n int) { seen = append(seen, n) }
	if j.OpenAuto(13, 0x20) || j.Open() {
		t.Fatal("表中沒有的編號不開面板")
	}
	if !j.OpenAuto(5, 0x20) || !j.Open() {
		t.Fatal("應開啟")
	}
	l := j.Lines()
	if !strings.HasPrefix(string(l[journalRows-1]), " ") || !strings.Contains(string(l[journalRows-1]), "頁") {
		t.Errorf("兩頁時頁碼靠右：%q", string(l[journalRows-1]))
	}
	j.TurnPage(1)
	if j.page != 1 {
		t.Error("翻頁")
	}
	j.KeyTaken(0x20)
	if !j.Open() {
		t.Error("head 沒變不關")
	}
	j.KeyTaken(0x22)
	if j.Open() {
		t.Error("遊戲取走鍵後關閉")
	}
	j.OpenAuto(5, 0x3C)
	j.KeyTaken(0x1E) // 環繞也算變動
	if j.Open() {
		t.Error("head 環繞也要關")
	}
	j.OpenAuto(1, 0x20)
	j.CloseAuto()
	if j.Open() || len(seen) != 2 {
		t.Errorf("CloseAuto；已讀回呼只在第一次：%v", seen)
	}
}

func TestJournalBrowse(t *testing.T) {
	j := newPanel()
	j.ToggleBrowse()
	if !j.Browsing() || !strings.HasPrefix(string(j.Lines()[0]), "尚未") {
		t.Fatal("沒有已讀時顯示提示")
	}
	j.ToggleBrowse()
	j.Seen = map[int]bool{1: true, 9: true}
	j.OpenAuto(5, 0)
	j.CloseAuto()
	j.ToggleBrowse()
	if j.n != 5 {
		t.Fatalf("翻閱從最近一次自動開的那則開始，得到 %d", j.n)
	}
	j.KeyTaken(9)
	if !j.Browsing() {
		t.Fatal("翻閱模式不受遊戲取鍵影響")
	}
	j.NextEntry(1)
	if j.n != 9 {
		t.Errorf("下一則已讀：%d", j.n)
	}
	j.NextEntry(1)
	if j.n != 9 {
		t.Error("最後一則不再往後")
	}
	j.NextEntry(-1)
	j.NextEntry(-1)
	if j.n != 1 {
		t.Errorf("往前到第一則：%d", j.n)
	}
	j.CloseAuto()
	if !j.Browsing() {
		t.Error("CloseAuto 不關翻閱模式")
	}
	j.ToggleBrowse()
	if j.Open() {
		t.Error("再按一次關閉")
	}
}

// TestJournalTableFits 檢查正式手札表（規格 007 §3 靜態）：每則排版後不超過 JournalMaxPages 頁、
// 標題列放得下、Big5 由翻譯流程另驗。DOSGOLEM_DKK_TEXT_DIR 指到 text/ 才跑。
func TestJournalTableFits(t *testing.T) {
	dir := os.Getenv("DOSGOLEM_DKK_TEXT_DIR")
	if dir == "" {
		t.Skip("DOSGOLEM_DKK_TEXT_DIR 未設定")
	}
	j, err := LoadJournalDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(j.Entries) == 0 {
		t.Fatal("手札表是空的")
	}
	for n := range j.Entries {
		j.OpenAuto(n, 0)
		if len(j.pages) > JournalMaxPages {
			t.Errorf("手札 %d 排成 %d 頁", n, len(j.pages))
		}
		if l := j.Lines()[0]; len(l) > journalCols {
			t.Errorf("手札 %d 標題 %d 格", n, len(l))
		}
	}
}
