package buckrogers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Buck repo spec 054 §5.1: the real signatures.  The strings come from the
// local evidence of the run (the original English of the game: ignored by
// Git, never copied into a file of the repository); without it the test
// skips.  The ko and ja lanes are the real ones (formal text, real
// transliterators).

type q1Occurrence struct {
	Cls    string `json:"cls"`
	Struct string `json:"struct"`
	Caller string `json:"caller"`
	Name   string `json:"name"`
	Text   string `json:"text"`
}

func inlineEvidenceDir(t *testing.T) string {
	t.Helper()
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	dir := filepath.Join(root, "workplace", "phase317-evidence")
	if _, err := os.Stat(filepath.Join(dir, "q1_occurrences.json")); err != nil {
		t.Skip("沒有本機證據 q1_occurrences.json")
	}
	return dir
}

// inlineSignatures are the strings of the sentence callers (types B and D) the
// run saw: the sentence, the party member it names ("" for a monster), and
// the caller.
func inlineSignatures(t *testing.T, dir string) (out []q1Occurrence) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "q1_occurrences.json"))
	if err != nil {
		t.Fatal(err)
	}
	var all []q1Occurrence
	if err := json.Unmarshal(b, &all); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, o := range all {
		if !strings.HasPrefix(o.Struct, "embedded") || o.Cls != "party" && o.Cls != "monster" {
			continue
		}
		var caller string
		for _, c := range []string{"0AE5", "1E42", "2813"} {
			if strings.HasSuffix(o.Caller, ":"+c) {
				caller = c
			}
		}
		if caller == "" || seen[o.Text] {
			continue
		}
		seen[o.Text] = true
		out = append(out, o)
	}
	return out
}

func TestInlineNamesRealSignatures(t *testing.T) {
	dir := inlineEvidenceDir(t)
	sigs := inlineSignatures(t, dir)
	if len(sigs) == 0 {
		t.Fatal("沒有簽章")
	}
	for _, c := range jkLanes {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			text, twFont, font := c.inputs(t)
			r := loadJkRuntime(t, c.lang, text, twFont, font)
			l := r.lanes[r.laneIndex(c.lang)]
			eng := l.ecl.engine
			if eng == nil || l.players == nil {
				t.Fatal("通道沒有引擎譯文或玩家名")
			}
			var members []PartyMember
			have := map[string]bool{}
			for _, o := range sigs {
				if o.Cls == "party" && !have[o.Name] {
					have[o.Name] = true
					members = append(members, PartyMember{Name: o.Name, Gender: translit.GenderUnknown})
				}
			}
			names := (&PartySnapshot{Members: members}).NameFunc(l.players)
			var party, monster int
			for _, o := range sigs {
				got, ok, inline := eng.TranslateParty(o.Text, names)
				plain, pok := eng.Translate(o.Text)
				if !ok || !pok {
					t.Errorf("%s 的簽章譯不出（%v %v）", o.Caller, ok, pok)
					continue
				}
				if o.Cls == "monster" {
					monster++
					if inline != 0 || got != plain {
						t.Errorf("怪物句不應變：%q 對 %q", got, plain)
					}
					continue
				}
				party++
				reading, rok := l.players.Chinese(o.Name, translit.GenderUnknown)
				if !rok || inline != 1 || !strings.Contains(got, reading) || strings.Contains(got, o.Name) {
					t.Errorf("%s／%s：inline=%d，讀音 %q（%v），結果 %q", o.Caller, o.Name, inline, reading, rok, got)
					continue
				}
				if c.lang == LangKo {
					// Only the name and the marks next to it change.
					if want := koResolveMarkers(strings.Replace(plain, o.Name, reading, 1)); got != want {
						t.Errorf("ko %s：%q，應為 %q", o.Caller, got, want)
					}
					if strings.Contains(got, reading+"은(는)") || strings.Contains(got, reading+"이(가)") {
						t.Errorf("ko %s：名字後的標記未解析：%q", o.Caller, got)
					}
				}
			}
			if party == 0 || monster == 0 {
				t.Fatalf("簽章需含隊員句與怪物句：隊員 %d、怪物 %d", party, monster)
			}
			t.Logf("%s：隊員句 %d、怪物句 %d", c.lang, party, monster)
		})
	}
}

// The dispatcher sentence caller (type D) in the formal catalogs: the sentence
// that names a party member and leads a monster.
func TestInlineNamesRealDispatcherSentence(t *testing.T) {
	dir := inlineEvidenceDir(t)
	runs, err := filepath.Glob(filepath.Join(dir, "runs", "*.dbg.tsv"))
	if err != nil || len(runs) == 0 {
		t.Skip("沒有本機 dbg 紀錄")
	}
	party := map[string]bool{"FLAVIUS": true, "CELESTE": true, "PIERRE": true, "NICOLE STEELE": true, "ROARKE": true, "JANELLE": true}
	sentences := map[string]string{}
	for _, f := range runs {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			cols := strings.Split(line, "\t")
			if len(cols) <= 9 || cols[0] != "D" || cols[3] != "0763:1282" {
				continue
			}
			text := strings.Trim(cols[9], "|")
			for name := range party {
				if strings.HasPrefix(text, name+" ") {
					sentences[text] = name
				}
			}
		}
	}
	if len(sentences) == 0 {
		t.Skip("dbg 紀錄沒有 1282 的隊員句")
	}
	for _, c := range jkLanes {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			text, twFont, font := c.inputs(t)
			r := loadJkRuntime(t, c.lang, text, twFont, font)
			l := r.lanes[r.laneIndex(c.lang)]
			if !r.inlineNames {
				t.Fatal("inlineNames 應成立")
			}
			for s, name := range sentences {
				snap := &PartySnapshot{Members: []PartyMember{{Name: name, Gender: translit.GenderUnknown}}}
				w := l.engDisp
				before := w.Stats
				dispatchSentence(w, inlineNameCaller, s, snap)
				reading, _ := l.players.Chinese(name, translit.GenderUnknown)
				got := inlineLine(w)
				if w.Stats.Hits != before.Hits+1 || w.Stats.InlineNames != before.InlineNames+1 || !strings.HasPrefix(got, reading) {
					t.Errorf("%s：%q %+v（讀音 %q）", name, got, w.Stats, reading)
				}
			}
		})
	}
}
