package buckrogers

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/xlate/translit"
	"github.com/wicanr2/dosgolem/xlate/translitjk"
)

// Buck repo specs 044 and 045: the ja and ko lanes load the katakana and
// Hangul transliterators.  The inputs are the Buck repo root and the fonts
// (see ja_test.go, ko_test.go); every test skips with its reason without them.

var _ nameTransliterator = (*translitjk.Transliterator)(nil)

type jkLane struct {
	lang string
	// inputs returns the text dir, the zh-TW font and the lane's font.
	inputs func(t *testing.T) (string, string, string)
}

var jkLanes = []jkLane{{LangJa, jaInputs}, {LangKo, koInputs}}

func loadJkRuntime(t *testing.T, lang, text, twFont, font string) *LiveRuntime {
	t.Helper()
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: twFont, Langs: []string{lang},
		LangFonts: map[string]string{lang: font}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// textDirWithout is a text/ of symlinks without the named file.
func textDirWithout(t *testing.T, text string, skip string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == skip {
			continue
		}
		if err := os.Symlink(filepath.Join(text, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// fontWithout writes a copy of a GOLEMFNT file with one glyph removed.
func fontWithout(t *testing.T, src string, drop rune) string {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	w, h := int(binary.LittleEndian.Uint16(b[8:10])), int(binary.LittleEndian.Uint16(b[10:12]))
	size := 4 + 1 + h*((w+7)/8)
	out := append([]byte{}, b[:16]...)
	n, found := 0, false
	for o := 16; o+size <= len(b); o += size {
		if rune(binary.LittleEndian.Uint32(b[o:])) == drop {
			found = true
			continue
		}
		out = append(out, b[o:o+size]...)
		n++
	}
	if !found {
		t.Fatalf("字型沒有 U+%04X", drop)
	}
	binary.LittleEndian.PutUint32(out[12:], uint32(n))
	dst := filepath.Join(t.TempDir(), "font.golemfnt")
	if err := os.WriteFile(dst, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestJkPlayerNamesOnFormalText(t *testing.T) {
	for _, c := range jkLanes {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			text, twFont, font := c.inputs(t)
			r := loadJkRuntime(t, c.lang, text, twFont, font)
			l := r.lanes[r.laneIndex(c.lang)]
			if l.players == nil || l.playersOff != "" {
				t.Fatalf("玩家名未啟用：off=%q", l.playersOff)
			}
			if err := r.SetLanguage(c.lang); err != nil {
				t.Fatal(err)
			}
			if sum := r.DebugSummary(); strings.Contains(sum, "玩家名=off") {
				t.Errorf("DebugSummary 不應有玩家名=off：%s", sum)
			}
			for _, name := range []string{"NICOLE STEELE", "ROARKE", "MARY-JANE"} {
				if out, ok := l.players.Chinese(name, translit.Female); !ok || out == "" {
					t.Errorf("%s：沒有結果", name)
				}
			}
			// the name table still wins over the transliterator (spec 038 §3.2)
			if zh, ok := l.names.ChineseFor("BUCK ROGERS"); ok {
				if out, ok2 := l.players.Chinese("BUCK ROGERS", translit.Male); !ok2 || out != zh {
					t.Errorf("名字表人物 BUCK ROGERS：得 %q，名字表為 %q", out, zh)
				}
			}
			if tw := r.lanes[0]; tw.lang == LangZhTW && tw.players == nil {
				t.Error("zh-TW 的玩家名不應受影響")
			}
		})
	}
}

// A transliterator file that is missing only turns that language's player
// names off; the lane and the other languages go on (spec 044 §3.6).
func TestJkMissingFileDisablesOnlyPlayerNames(t *testing.T) {
	for _, c := range jkLanes {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			text, twFont, font := c.inputs(t)
			dir := textDirWithout(t, text, "translit-"+c.lang+"-names.tsv")
			r := loadJkRuntime(t, c.lang, dir, twFont, font)
			i := r.laneIndex(c.lang)
			if i < 0 {
				t.Fatalf("%s 應仍啟用：%v", c.lang, r.off)
			}
			if l := r.lanes[i]; l.players != nil || !strings.HasPrefix(l.playersOff, "translit-"+c.lang+":") {
				t.Errorf("players=%v off=%q", l.players != nil, l.playersOff)
			}
			if r.lanes[0].players == nil {
				t.Error("zh-TW 玩家名不應受影響")
			}
		})
	}
}

// A font that lacks one allowed character turns the player names off at load
// time (spec 044 §3.1), naming the smallest code point and the count.
func TestJkFontMissingGlyphDisablesPlayerNames(t *testing.T) {
	for _, c := range jkLanes {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			text, twFont, font := c.inputs(t)
			tr, err := translitjk.Load(text, c.lang)
			if err != nil {
				t.Fatal(err)
			}
			drop := unusedAllowedRune(t, text, c.lang, tr.Allowed())
			r := loadJkRuntime(t, c.lang, text, twFont, fontWithout(t, font, drop))
			i := r.laneIndex(c.lang)
			if i < 0 {
				t.Fatalf("%s 應仍啟用：%v", c.lang, r.off)
			}
			l := r.lanes[i]
			want := "translit-font: 缺 U+" + strings.ToUpper(hex4(drop)) + " 等 1 字"
			if l.players != nil || l.playersOff != want {
				t.Errorf("players=%v off=%q，應為 %q", l.players != nil, l.playersOff, want)
			}
		})
	}
}

// unusedAllowedRune is an allowed character no translation of the language
// uses, so that removing its glyph only affects the player names.
func unusedAllowedRune(t *testing.T, text, lang string, allowed []rune) rune {
	t.Helper()
	used := map[rune]bool{}
	files, err := filepath.Glob(filepath.Join(text, "*."+lang+".tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f), "translit-chars.") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			if cols := strings.Split(line, "\t"); len(cols) > 1 {
				for _, r := range cols[1] {
					used[r] = true
				}
			}
		}
	}
	for i := len(allowed) / 2; i < len(allowed); i++ {
		if !used[allowed[i]] {
			return allowed[i]
		}
	}
	t.Fatalf("%s：允許字集的每個字都被譯文用到，換驗證方式", lang)
	return 0
}

func hex4(r rune) string {
	const digits = "0123456789abcdef"
	s := []byte{digits[(r>>12)&15], digits[(r>>8)&15], digits[(r>>4)&15], digits[r&15]}
	return string(s)
}

// Spec 044 §5 point 6: the load time and heap of LoadLiveRuntimeOptions with
// the zh-TW lane alone and with the four languages.  Recorded, not gated.
func TestJkLoadCostRecord(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	fonts := map[string]string{
		LangZhTW: os.Getenv("BUCKROGERS_ZHTW_FONT"), LangZhCN: os.Getenv("BUCKROGERS_ZHCN_FONT"),
		LangJa: os.Getenv("BUCKROGERS_JA_FONT"), LangKo: os.Getenv("BUCKROGERS_KO_FONT"),
	}
	for lang, f := range fonts {
		if root == "" || f == "" {
			t.Skipf("BUCKROGERS_CHT_ROOT 或 %s 的字型環境變數未設定：載入成本未記錄", lang)
		}
	}
	text := filepath.Join(root, "text")
	measure := func(label string, langs []string) {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		opts := LiveOptions{TextDir: text, FontPath: fonts[LangZhTW], Langs: langs, LangFonts: map[string]string{}}
		for _, l := range langs {
			opts.LangFonts[l] = fonts[l]
		}
		r, err := LoadLiveRuntimeOptions(opts)
		if err != nil {
			t.Fatal(err)
		}
		d := time.Since(start)
		runtime.GC()
		runtime.ReadMemStats(&after)
		t.Logf("%s：載入 %v，HeapAlloc %d -> %d bytes（%+d）", label, d.Round(time.Millisecond), before.HeapAlloc, after.HeapAlloc, int64(after.HeapAlloc)-int64(before.HeapAlloc))
		runtime.KeepAlive(r)
	}
	measure("zh-TW 單語", nil)
	measure("四語", []string{LangZhCN, LangJa, LangKo})
}
