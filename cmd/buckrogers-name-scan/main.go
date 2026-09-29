// buckrogers-name-scan 對正式 catalog 做規格 036 §3.3 的靜態掃描：
// 手札每則本文與標題採用哪一段加註、ECL 譯文在標準敘事窗的預期段別，
// 以及因已緊接括號而不加註的位置。只讀 text/，輸出 TSV 到 -out。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
)

func main() {
	text := flag.String("text", "", "Buck repo 的 text/ 目錄")
	out := flag.String("out", "", "輸出目錄")
	flag.Parse()
	if err := run(*text, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func read(dir, name string) []byte {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return b
}

type tsv struct {
	f *os.File
	w *bufio.Writer
}

func create(dir, name string, header ...string) *tsv {
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	t := &tsv{f, bufio.NewWriter(f)}
	t.row(header...)
	return t
}

func (t *tsv) row(v ...string) { fmt.Fprintln(t.w, strings.Join(v, "\t")) }
func (t *tsv) close() {
	t.w.Flush()
	t.f.Close()
}

func names(g *buckrogers.NameGlossary, s, key string) (en []string, paren []string) {
	for _, m := range g.Matches([]rune(s), key) {
		if m.Paren {
			paren = append(paren, m.Entry.Chinese)
		} else {
			en = append(en, m.Entry.English)
		}
	}
	return
}

func run(textDir, outDir string) error {
	if textDir == "" || outDir == "" {
		return fmt.Errorf("需要 -text 與 -out")
	}
	g, err := buckrogers.LoadNameGlossaryDir(textDir)
	if err != nil || g == nil {
		return fmt.Errorf("譯名表：%v", err)
	}
	summary := create(outDir, "summary.tsv", "item", "value")
	defer summary.close()
	paren := create(outDir, "paren-follow.tsv", "family", "key", "chinese")
	defer paren.close()

	// 手札
	lb, err := buckrogers.LoadLogbookCatalog(read(textDir, "logbook.zh-TW.tsv"), g)
	if err != nil {
		return err
	}
	if err := lb.LoadLogbookPanelText(read(textDir, "logbook-panel.zh-TW.tsv")); err != nil {
		return err
	}
	raw, err := buckrogers.ReadCatalogRows("logbook.zh-TW.tsv", read(textDir, "logbook.zh-TW.tsv"))
	if err != nil {
		return err
	}
	lt := create(outDir, "logbook-tiers.tsv", "entry", "names", "pages_all", "pages_first", "pages_none", "body_tier",
		"title_names", "title_cells_all", "title_cells_first", "title_cells_none", "title_tier", "title_cells")
	defer lt.close()
	count := map[string]int{}
	for _, n := range lb.LogbookEntries() {
		key := "logbook." + strconv.Itoa(n)
		e := lb.Entry(n)
		body, title := raw[key], raw[key+".title"]
		en, pf := names(g, body, key)
		for _, z := range pf {
			paren.row("logbook", key, z)
		}
		var pages [3]string
		for i, tier := range []buckrogers.NameTier{buckrogers.NameTierAll, buckrogers.NameTierFirst, buckrogers.NameTierNone} {
			p, err := buckrogers.LayoutLogbookAnnotated(g.Annotate(body, key, buckrogers.NameCaseMixed, tier))
			if err != nil {
				pages[i] = ">3"
			} else {
				pages[i] = strconv.Itoa(len(p))
			}
		}
		ten, tpf := names(g, title, key+".title")
		for _, z := range tpf {
			paren.row("logbook", key+".title", z)
		}
		var cells [3]string
		for i, tier := range []buckrogers.NameTier{buckrogers.NameTierAll, buckrogers.NameTierFirst, buckrogers.NameTierNone} {
			a := g.Annotate(title, key+".title", buckrogers.NameCaseMixed, tier)
			cells[i] = strconv.Itoa(len([]rune(lb.TitleRow(n, string(a.Text)))))
		}
		lt.row(strconv.Itoa(n), strings.Join(en, ";"), pages[0], pages[1], pages[2], e.BodyTier.String(),
			strings.Join(ten, ";"), cells[0], cells[1], cells[2], e.TitleTier.String(),
			strconv.Itoa(len([]rune(lb.TitleRow(n, e.Title)))))
		count["logbook.body."+e.BodyTier.String()]++
		count["logbook.title."+e.TitleTier.String()]++
		if len(en) > 0 {
			count["logbook.body_with_names"]++
		}
		if len(ten) > 0 {
			count["logbook.title_with_names"]++
		}
	}

	// ECL：以 phase-255 的標準敘事窗（左 1、頂 17、右 38、底 22，新頁）估段別；
	// 實際視窗是執行期參數，此表只是預估。
	ecl, err := buckrogers.LoadEclTextCatalog(read(textDir, "ecl-text-events.tsv"), read(textDir, "ecl-text.zh-TW.tsv"))
	if err != nil {
		return err
	}
	et := create(outDir, "ecl-standard-window.tsv", "key", "names", "rows_all", "rows_first", "rows_none", "tier")
	defer et.close()
	for _, key := range ecl.Keys() {
		s := ecl.Text(key)
		en, pf := names(g, s, key)
		for _, z := range pf {
			paren.row("ecl", key, z)
		}
		if len(en) == 0 {
			continue
		}
		var rows [3]string
		tier := "overflow"
		for i, t := range []buckrogers.NameTier{buckrogers.NameTierAll, buckrogers.NameTierFirst, buckrogers.NameTierNone} {
			n, ok := buckrogers.LayoutEclAnnotated(g.Annotate(s, key, buckrogers.NameCaseUpper, t), 17, 1, 1, 38, 22)
			if ok {
				rows[i] = strconv.Itoa(n)
				if tier == "overflow" {
					tier = t.String()
				}
			} else {
				rows[i] = ">6"
			}
		}
		et.row(key, strings.Join(en, ";"), rows[0], rows[1], rows[2], tier)
		count["ecl.with_names"]++
		count["ecl.tier."+tier]++
	}
	for _, k := range []string{"logbook.body_with_names", "logbook.body.all", "logbook.body.first", "logbook.body.none",
		"logbook.title_with_names", "logbook.title.all", "logbook.title.first", "logbook.title.none",
		"ecl.with_names", "ecl.tier.all", "ecl.tier.first", "ecl.tier.none", "ecl.tier.overflow"} {
		summary.row(k, strconv.Itoa(count[k]))
	}
	return nil
}
