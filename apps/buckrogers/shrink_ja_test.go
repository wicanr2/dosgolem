package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 057 §5.3 and §5.4: the Japanese L2 on the formal font.  These tests
// need BUCKROGERS_CHT_ROOT and BUCKROGERS_JA_FONT and skip without them.

// jaVoicedPairs are the 31 pairs of a kana and its voiced or semi-voiced
// partner (spec 057 §2).
var jaVoicedPairs = []string{
	"カガ", "キギ", "クグ", "ケゲ", "コゴ", "サザ", "シジ", "スズ", "セゼ", "ソゾ", "タダ", "チヂ", "ツヅ", "テデ", "トド",
	"ハバ", "ヒビ", "フブ", "ヘベ", "ホボ", "ハパ", "ヒピ", "フプ", "ヘペ", "ホポ", "ウヴ", "バパ", "ビピ", "ブプ", "ベペ", "ボポ",
}

func glyphDistance(a, b []byte) int {
	n := 0
	for i := range a {
		for x := a[i] ^ b[i]; x != 0; x &= x - 1 {
			n++
		}
	}
	return n
}

func glyphBit(g []byte, w, x, y int) bool { return g[y*((w+7)/8)+x/8]&(0x80>>uint(x%8)) != 0 }

func glyphArt(g []byte, w, h int) string {
	var sb strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if glyphBit(g, w, x, y) {
				sb.WriteByte('#')
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func jaFormal(t *testing.T) (*xlate.Font, []rune) {
	t.Helper()
	root, path := os.Getenv("BUCKROGERS_CHT_ROOT"), os.Getenv("BUCKROGERS_JA_FONT")
	if root == "" || path == "" {
		t.Skip("BUCKROGERS_CHT_ROOT／BUCKROGERS_JA_FONT 未設定")
	}
	f, err := xlate.LoadFont(path)
	if err != nil {
		t.Fatal(err)
	}
	g, err := loadNameGlossaryLang(filepath.Join(root, "text"), filepath.Join(root, "text"), LangJa)
	if err != nil {
		t.Fatal(err)
	}
	return f, shrinkRuneSet(t, filepath.Join(root, "text"), LangJa, g.names)
}

// The kana of voiced and unvoiced sounds are apart: every full-width character
// of the name set is distinct at every level and scale of the ja table, each of
// the 31 pairs differs by at least 2 pixels at L2 (with a difference in the
// upper right half, where the marks are) and by at least as many as today at L1.
func TestShrinkJaVoicedPairs(t *testing.T) {
	f, set := jaFormal(t)
	inSet := map[rune]bool{}
	for _, r := range set {
		inSet[r] = true
	}
	if len(jaVoicedPairs) != 31 {
		t.Fatalf("%d pairs", len(jaVoicedPairs))
	}
	for _, p := range jaVoicedPairs {
		for _, r := range p {
			if !inSet[r] {
				t.Fatalf("%q of the pair %q is not in the name set", r, p)
			}
		}
	}
	levels := shrinkLevelsFor(LangJa)
	for _, scale := range []int{2, 3} {
		for _, rep := range shrinkDistinctReports(f, set, levels, scale) {
			if len(rep.FullGroups) != 0 || len(rep.Empty) != 0 {
				t.Errorf("%d× L%d: groups %q empty %q", scale, rep.Level, rep.FullGroups, string(rep.Empty))
			}
		}
		for _, sp := range levels {
			fs := shrinkFontsOf(f, sp, scale)
			m := sp.metrics(scale)
			minD, minAt := 1<<30, ""
			for _, p := range jaVoicedPairs {
				rr := []rune(p)
				a, b := fs.Full.Glyphs[rr[0]], fs.Full.Glyphs[rr[1]]
				d := glyphDistance(a, b)
				if d < minD {
					minD, minAt = d, p
				}
				if sp.Level == 2 {
					ur := false
					for y := 0; y < m.FullGlyphH; y++ {
						for x := 0; x < m.FullGlyphW; x++ {
							if glyphBit(a, m.FullGlyphW, x, y) != glyphBit(b, m.FullGlyphW, x, y) && y < m.FullGlyphH/2 && x >= m.FullGlyphW/2 {
								ur = true
							}
						}
					}
					if !ur {
						t.Errorf("%d× L2 %s: no difference in the upper right half", scale, p)
					}
				}
			}
			want := 2
			if sp.Level == 1 {
				want = map[int]int{2: 1, 3: 4}[scale]
			}
			if minD < want {
				t.Errorf("%d× L%d: pair %s differs by %d pixels, need %d", scale, sp.Level, minAt, minD, want)
			}
			t.Logf("%d× L%d: voiced pairs differ by at least %d pixels (%s)", scale, sp.Level, minD, minAt)
		}
	}
	// Negative control: the table of spec 056 has pairs that do not differ.
	old := shrinkFontsOf(f, shrinkLevels[1], 3)
	if glyphDistance(old.Full.Glyphs['カ'], old.Full.Glyphs['ガ']) != 0 {
		t.Errorf("the 3× L2 of spec 056 was expected to draw カ and ガ the same")
	}
}

// Fixed bitmaps of ガ (frozen): the derivation really draws the rectangle of
// the table.
func TestShrinkJaSnapshots(t *testing.T) {
	f, _ := jaFormal(t)
	for _, c := range []struct {
		scale int
		w, h  int
		want  string
	}{
		{2, 8, 14, jaGa2x}, {3, 12, 16, jaGa3x},
	} {
		sp := shrinkLevelsFor(LangJa)[1]
		fs := shrinkFontsOf(f, sp, c.scale)
		got := glyphArt(fs.Full.Glyphs['ガ'], c.w, c.h)
		if fs.Full.W != c.w || fs.Full.H != c.h || got != c.want {
			t.Errorf("%d× ガ %dx%d:\n%s\nwant\n%s", c.scale, fs.Full.W, fs.Full.H, got, c.want)
		}
	}
}

// The derived glyphs of a fixed probe set (the 31 pairs and セレステ) at every
// level and scale of the ja table, as a digest (frozen).
func TestShrinkJaProbeDigests(t *testing.T) {
	f, _ := jaFormal(t)
	var probe []rune
	for _, p := range jaVoicedPairs {
		probe = append(probe, []rune(p)...)
	}
	probe = append(probe, []rune("セレステ")...)
	got := map[string]string{}
	for _, scale := range []int{2, 3} {
		for _, sp := range shrinkLevelsFor(LangJa) {
			got[fmt.Sprintf("L%d/%d×", sp.Level, scale)] = shrinkProbeDigest(shrinkFontsOf(f, sp, scale).Full, probe)
		}
	}
	for k, want := range jaProbeDigests {
		if got[k] != want {
			t.Errorf("%s: %s want %s", k, got[k], want)
		}
	}
	if len(jaProbeDigests) != len(got) {
		t.Errorf("%d digests, %d frozen", len(got), len(jaProbeDigests))
	}
}

// Frozen on the formal ja font (SHA-256 67fd7ac9…1c06) with the table of spec
// 057 §3.3.
const jaGa2x = `........
......#.
...#.##.
...#.#..
...###..
.###.#..
...#.#..
.....#..
..#..#..
..#..#..
........
.#..#...
....#...
........
`

const jaGa3x = `............
.........#..
....#..#..#.
.....#..#.#.
.....#..#...
.#...####...
..####..#...
....#...#...
....#...#...
....#..#....
....#..#....
...#...#....
..#..#.#....
.#....##....
......#.....
............
`

var jaProbeDigests = map[string]string{
	"L1/2×": "198dd133b09f4df03180b521811b18ed03f6fcd56626555d6aae0c86e67f47ec",
	"L1/3×": "717f99335a3f3bc4c5804b9e74979f339863cc8f10dc6ab0da96bd9b4082c124",
	"L2/2×": "f078369bd22c89743454f09c4cf10d8f5952c364d78848675dad79800eeaa559",
	"L2/3×": "34da08e12217259dbaa9aa1c5a60dced5137fcfc550a9e2e69502a0ddd243985",
}
