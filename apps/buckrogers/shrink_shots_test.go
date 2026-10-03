package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 056 §5.5: synthetic screenshots of the shrunk units (environment
// gate).  BUCKROGERS_SHRINK_SHOTS is the directory the PNGs and manifest.tsv
// go to (it must not be a directory under version control); the formal text/
// and fonts are loaded as the other formal tests do, and BUCKROGERS_ZHTW_ETEN_FONT
// adds the local Eten font (its images stay local).  The pictures are drawn
// from an empty indexed buffer: no frame of the original game is involved.
// Every picture asserts the level it shows, so a step-down cannot be taken
// for a shrunk unit.

// shotPlayer finds the cursor column at which a player-name call on the last
// row of the window is drawn at the wanted level.
func shotPlayer(t *testing.T, l *liveLane, lang, name string, win [4]uint8, level int) (*EclTextPage, int, bool) {
	t.Helper()
	party, err := ReadPartySnapshot(partyMem(1, partyRec{seg: 0x5747, off: 2, name: name}), testDS)
	if err != nil {
		t.Fatal(err)
	}
	for c := int(win[0]); c <= int(win[2]); c++ {
		w := NewEclTextWatcher(eclFixtureLang(t, lang, "UNUSED FIXTURE", "x"))
		w.SetNames(l.names)
		w.SetLayout(LayoutFor(lang))
		w.SetPlayerNames(l.players)
		e := eclPlayerEntry(name, false, uint8(c), win[3], eclCtx(party, eclNameCallerB79, 0x81, eclVarName, partyRec{}))
		e.Left, e.Top, e.Right, e.Bottom = win[0], win[1], win[2], win[3]
		w.ObserveEntry(e)
		if p := w.Page(); p != nil && w.Stats.PlayerShrunk[level-1] == 1 && w.Stats.PlayerNames == 1 && w.Stats.PlayerNameChineseOnly == 0 {
			return p, c, true
		}
	}
	return nil, 0, false
}

// shotNPC picks the longest catalog sentence with a name unit that has a
// start column on the last row of the window where the annotation is drawn at
// the wanted level.
func shotNPC(t *testing.T, l *liveLane, lang string, win [4]uint8, level int) (*EclTextPage, string, int, bool) {
	t.Helper()
	type cand struct {
		key, text string
		w         int
	}
	var cs []cand
	for _, k := range l.ecl.catalog.Keys() {
		txt := l.ecl.catalog.text[k]
		if v := l.names.Variants(txt, k, NameCaseUpper); len(v) > 0 && len(v[0].Units) > 0 && !strings.ContainsAny(txt, "\n\\") {
			cs = append(cs, cand{k, txt, textUnits([]rune(txt))})
		}
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].w > cs[j].w || cs[i].w == cs[j].w && cs[i].key < cs[j].key })
	for _, c := range cs {
		for col := int(win[0]); col <= int(win[2]); col++ {
			w := NewEclTextWatcher(eclFixtureLang(t, lang, "NPC SENTENCE", c.text))
			w.SetNames(l.names)
			w.SetLayout(LayoutFor(lang))
			e := eclEntry("NPC SENTENCE", false, uint8(col), win[3])
			e.Left, e.Top, e.Right, e.Bottom = win[0], win[1], win[2], win[3]
			w.ObserveEntry(e)
			if p := w.Page(); p != nil && w.Stats.NameShrunk[level-1] == 1 && w.Stats.Overflows == 0 {
				return p, c.key, col, true
			}
		}
	}
	return nil, "", 0, false
}

// shotHasVoiced reports whether a shrunk unit of the page has a kana with a
// voiced or semi-voiced sound (the second characters of jaVoicedPairs).
func shotHasVoiced(p *EclTextPage) bool {
	var set strings.Builder
	for _, pr := range jaVoicedPairs {
		set.WriteString(string([]rune(pr)[1]))
	}
	for _, l := range p.Lines {
		if l.Shrink != 0 && strings.ContainsAny(string(l.Text), set.String()) {
			return true
		}
	}
	return false
}

// shotAssertUnitWidths checks every shrunk unit of the pages against the
// overlay: its stamps start at the unit's column, add up to the width of its
// characters at the level, and the unit takes W_s half units, that is W_s × 4
// × scale pixels of the picture.
func shotAssertUnitWidths(t *testing.T, label, lang string, scale int, o *EclTextOverlay, pages []*EclTextPage) {
	t.Helper()
	levels := shrinkLevelsFor(lang)
	for pi, p := range pages {
		for _, l := range p.Lines {
			if l.Shrink == 0 {
				continue
			}
			spec, ok := shrinkSpecIn(levels, int(l.Shrink))
			if !ok {
				t.Fatalf("%s：頁面的等級 %d 不在表內", label, l.Shrink)
			}
			ws := shrinkUnitUnits(l.Text, spec)
			x0, sum, n := int(l.Col)*halfUnitPx, 0, 0
			prefix := fmt.Sprintf("ecl.%d.row.%d", pi, l.Row)
			for _, s := range o.layer.Stamps {
				if s.Font == nil || !strings.Contains(s.Font.Name, ".shrink") || rowKeyOf(s.Key) != prefix || s.X < x0 || s.X >= x0+ws*halfUnitPx {
					continue
				}
				if s.X != x0+sum {
					t.Errorf("%s %d×：單元 %q 的 stamp 在 %d，應接在 %d", label, scale, string(l.Text), s.X, x0+sum)
				}
				sum += s.Cells * s.CellW
				n += s.Cells
			}
			if n != len(l.Text) {
				t.Errorf("%s %d×：單元 %q 有 %d 個格的 stamp", label, scale, string(l.Text), n)
			}
			if sum > ws*halfUnitPx || sum <= (ws-1)*halfUnitPx {
				t.Errorf("%s %d×：單元 %q 的字寬 %d 邏輯像素，W_s=%d", label, scale, string(l.Text), sum, ws)
			}
			if got, want := ws*halfUnitPx*scale, (sum+halfUnitPx-1)/halfUnitPx*halfUnitPx*scale; got != want {
				t.Errorf("%s %d×：單元 %q 占 %d 像素，應為 %d", label, scale, string(l.Text), got, want)
			}
		}
	}
}

func TestShrinkSyntheticShots(t *testing.T) {
	dir := os.Getenv("BUCKROGERS_SHRINK_SHOTS")
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	tw := os.Getenv("BUCKROGERS_ZHTW_FONT")
	fonts := map[string]string{LangJa: os.Getenv("BUCKROGERS_JA_FONT"), LangKo: os.Getenv("BUCKROGERS_KO_FONT"), LangZhCN: os.Getenv("BUCKROGERS_ZHCN_FONT")}
	if dir == "" || root == "" || tw == "" || fonts[LangJa] == "" || fonts[LangKo] == "" || fonts[LangZhCN] == "" {
		t.Skip("BUCKROGERS_SHRINK_SHOTS 與正式文字、字型的環境變數未設定")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: root + "/text", FontPath: tw,
		Langs: []string{LangZhCN, LangJa, LangKo}, LangFonts: fonts})
	if err != nil {
		t.Fatal(err)
	}
	type group struct {
		label, lang string
		font        *xlate.Font
	}
	groups := []group{
		{"zh-TW-unifont", LangZhTW, r.lanes[r.laneIndex(LangZhTW)].font},
		{"ja", LangJa, r.lanes[r.laneIndex(LangJa)].font},
		{"ko", LangKo, r.lanes[r.laneIndex(LangKo)].font},
	}
	if p := os.Getenv("BUCKROGERS_ZHTW_ETEN_FONT"); p != "" {
		f, err := xlate.LoadFont(p)
		if err != nil {
			t.Fatal(err)
		}
		groups = append(groups, group{"zh-TW-eten", LangZhTW, f})
	}
	playerWin, npcWin := [4]uint8{23, 7, 38, 9}, [4]uint8{1, 17, 38, 22}
	var manifest strings.Builder
	manifest.WriteString("file\tsha256\tplayer\tplayer_cursor_col\tnpc_key\tnpc_cursor_col\n")
	n := 0
	for _, g := range groups {
		l := r.lanes[r.laneIndex(g.lang)]
		for _, scale := range []int{2, 3} {
			for level := 1; level <= 2; level++ {
				// Spec 057 §5.9: the L2 of ja shows a name with voiced sounds
				// (GILBERT is ギルバート), the sounds the table separates.
				names := []string{"CELESTE", "FLAVIUS"}
				if g.lang == LangJa && level == 2 {
					names = []string{"GILBERT"}
				}
				var (
					name string
					pp   *EclTextPage
					pcol int
					ok   bool
				)
				for _, name = range names {
					if pp, pcol, ok = shotPlayer(t, l, g.lang, name, playerWin, level); ok {
						break
					}
				}
				if !ok {
					t.Fatalf("%s L%d：找不到玩家名的起點", g.label, level)
				}
				if g.lang == LangJa && level == 2 && !shotHasVoiced(pp) {
					t.Fatalf("%s L%d：玩家名 %s 的片假名不含濁音或半濁音", g.label, level, name)
				}
				np, key, ncol, ok := shotNPC(t, l, g.lang, npcWin, level)
				if !ok {
					t.Fatalf("%s L%d：找不到 NPC 句子的起點", g.label, level)
				}
				for _, p := range []*EclTextPage{pp, np} {
					lv := 0
					for _, ln := range p.Lines {
						if int(ln.Shrink) > lv {
							lv = int(ln.Shrink)
						}
					}
					if lv != level {
						t.Fatalf("%s %d× L%d：頁面的縮小等級是 %d", g.label, scale, level, lv)
					}
				}
				o, err := NewEclTextOverlay(g.font, scale)
				if err != nil {
					t.Fatal(err)
				}
				pal := [256][3]uint8{}
				pal[0], pal[10] = [3]uint8{0, 0, 128}, [3]uint8{255, 255, 255}
				if miss := o.Sync([]*EclTextPage{pp, np}, 1, pal); len(miss) != 0 {
					t.Fatalf("%s %d× L%d：缺字 %q", g.label, scale, level, string(miss))
				}
				shotAssertUnitWidths(t, g.label, g.lang, scale, o, []*EclTextPage{pp, np})
				out, miss := o.Draw(make([]byte, 320*200), pal)
				if len(miss) != 0 {
					t.Fatalf("%s %d× L%d：繪製缺字 %q", g.label, scale, level, string(miss))
				}
				// Rows 6 to 22 of the screen (cells of 8 logical pixels) hold both windows.
				w := 320 * scale
				y0, y1 := 6*8*scale, 23*8*scale
				img := image.NewNRGBA(image.Rect(0, 0, w, y1-y0))
				copy(img.Pix, out[4*y0*w:4*y1*w])
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err != nil {
					t.Fatal(err)
				}
				file := fmt.Sprintf("shrink-%s-%dx-L%d.png", g.label, scale, level)
				if err := os.WriteFile(filepath.Join(dir, file), buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				fmt.Fprintf(&manifest, "%s\t%x\t%s\t%d\t%s\t%d\n", file, sha256.Sum256(buf.Bytes()), name, pcol, key, ncol)
				n++
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.tsv"), []byte(manifest.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d 張合成截圖寫入 %s", n, dir)
	t.Log("\n" + manifest.String())
	if want := 12; n < want || (os.Getenv("BUCKROGERS_ZHTW_ETEN_FONT") != "" && n != want+4) {
		t.Errorf("截圖數 %d", n)
	}
}
