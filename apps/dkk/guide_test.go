package dkk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testGuide(t *testing.T) *GuidePanel {
	t.Helper()
	long := strings.Repeat("甲乙丙丁戊己庚辛壬癸", 120) // 1200 格，40 欄要 30 列，兩頁
	tsv := "chapter\ttitle\tbody\tsource\n" +
		"1\t蓋加斯風雲\t第一段\\n第二段\t30 期 p.64\n" +
		"2\t卡拉瑪際會\t" + long + "\t30 期 p.65\n"
	chs, err := LoadGuide([]byte(tsv))
	if err != nil {
		t.Fatal(err)
	}
	return &GuidePanel{Chapters: chs, Format: GuideFormat{Title: "攻略 第 {n} 章：{title}", Page: "{source} 第 {page}／{pages} 頁",
		TOC: "目錄", TOCLine: "第 {n} 章 {title}", TOCHint: "按 F6 關閉"}}
}

// TestGuidePanelNavigation 檢查規格 008 §2.1：第一次開是目錄、翻頁跨章、換章、關閉後再開回到原位。
func TestGuidePanelNavigation(t *testing.T) {
	g := testGuide(t)
	g.Toggle()
	if ch, _ := g.Pos(); ch != 0 || string(g.Lines()[0]) != "目錄" {
		t.Fatalf("第一次開要是目錄：%d %q", ch, string(g.Lines()[0]))
	}
	if !strings.Contains(string(g.Lines()[1]), "蓋加斯風雲") {
		t.Errorf("目錄第一列：%q", string(g.Lines()[1]))
	}
	g.TurnPage(1) // 目錄只有一頁，翻到第 1 章
	if ch, p := g.Pos(); ch != 1 || p != 0 {
		t.Fatalf("目錄後翻頁：%d %d", ch, p)
	}
	if l := g.Lines(); string(l[1]) != "第一段" || string(l[2]) != "第二段" {
		t.Errorf("段落分列：%q %q", string(l[1]), string(l[2]))
	}
	g.NextChapter(1)
	g.TurnPage(1)
	if ch, p := g.Pos(); ch != 2 || p != 1 {
		t.Fatalf("第 2 章第 2 頁：%d %d", ch, p)
	}
	if len(g.pages) != 2 || !strings.HasSuffix(string(g.Lines()[guideRows-1]), "頁") {
		t.Errorf("第 2 章要兩頁：%d", len(g.pages))
	}
	g.TurnPage(1) // 最後一章最後一頁：停住
	if ch, p := g.Pos(); ch != 2 || p != 1 {
		t.Errorf("末頁不再翻：%d %d", ch, p)
	}
	g.TurnPage(-2) // 回到第 2 章首頁再往前：接第 1 章最後一頁
	if ch, _ := g.Pos(); ch != 1 {
		t.Errorf("跨章往前翻：%d", ch)
	}
	g.Toggle()
	if g.Open() {
		t.Fatal("再按要關閉")
	}
	g.Toggle()
	if ch, _ := g.Pos(); ch != 1 {
		t.Errorf("再開要回到第 1 章：%d", ch)
	}
}

func TestGuidePosFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pos.txt")
	if ch, page, err := ReadGuidePos(p); err != nil || ch != 0 || page != 0 {
		t.Fatalf("沒有檔案：%d %d %v", ch, page, err)
	}
	if err := WriteGuidePos(p, 3, 2); err != nil {
		t.Fatal(err)
	}
	if ch, page, err := ReadGuidePos(p); err != nil || ch != 3 || page != 2 {
		t.Errorf("讀回：%d %d %v", ch, page, err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadGuidePos(p); err == nil {
		t.Error("壞檔要回錯")
	}
}

func TestLoadGuideRejectsChapterGap(t *testing.T) {
	if _, err := LoadGuide([]byte("chapter\ttitle\tbody\tsource\n2\tx\ty\tz\n")); err == nil {
		t.Error("章號要從 1 連續")
	}
}

// TestGuideTableFits 檢查正式攻略表（規格 008 §3 靜態）：可載入、每章至少一頁。
func TestGuideTableFits(t *testing.T) {
	dir := os.Getenv("DOSGOLEM_DKK_TEXT_DIR")
	if dir == "" {
		t.Skip("DOSGOLEM_DKK_TEXT_DIR 未設定")
	}
	g, err := LoadGuideDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if g == nil {
		t.Skip("沒有 walkthrough.zh-TW.tsv")
	}
	for i := range g.Chapters {
		g.SetPos(i+1, 0)
		if len(g.pages) == 0 {
			t.Errorf("第 %d 章沒有頁", i+1)
		}
	}
}
