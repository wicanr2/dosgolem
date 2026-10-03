package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 058 §5.2 and §5.3: the Korean L2 on the formal font.  These tests need
// BUCKROGERS_CHT_ROOT and BUCKROGERS_KO_FONT and skip without them.

func koFormal(t *testing.T) (*xlate.Font, []rune) {
	t.Helper()
	root, path := os.Getenv("BUCKROGERS_CHT_ROOT"), os.Getenv("BUCKROGERS_KO_FONT")
	if root == "" || path == "" {
		t.Skip("BUCKROGERS_CHT_ROOT／BUCKROGERS_KO_FONT 未設定")
	}
	f, err := xlate.LoadFont(path)
	if err != nil {
		t.Fatal(err)
	}
	g, err := loadNameGlossaryLang(filepath.Join(root, "text"), filepath.Join(root, "text"), LangKo)
	if err != nil {
		t.Fatal(err)
	}
	return f, shrinkRuneSet(t, filepath.Join(root, "text"), LangKo, g.names)
}

// koSyllable splits a precomposed syllable into the indexes of its initial,
// medial and final (the final 0 is "none").
func koSyllable(r rune) (l, v, f int, ok bool) {
	if r < 0xAC00 || r > 0xD7A3 {
		return 0, 0, 0, false
	}
	i := int(r - 0xAC00)
	return i / 588, i % 588 / 28, i % 28, true
}

// koFamily is a family of easily confused phonemes (spec 058 §2): the syllables
// that share the other two positions and have phoneme a or b at the position.
type koFamily struct {
	name     string
	pos      int // 0 initial, 1 medial, 2 final
	a, b     int
	pairs    int    // the pairs of the name set
	min2, m3 int    // lower bounds of the smallest difference at 2× and 3× L2
	rep      string // a pair that reaches the 2× minimum
}

var koFamilies = []koFamily{
	{"ㅏ/ㅣ", 1, 0, 20, 112, 1, 2, "가기"},
	{"ㅔ/ㅐ", 1, 5, 1, 112, 2, 3, "게개"},
	{"ㅔ/ㅖ", 1, 5, 7, 112, 2, 3, "겍곅"},
	{"ㅓ/ㅔ", 1, 4, 5, 112, 17, 34, "걸겔"},
	{"終聲 ㅁ/ㅇ", 2, 16, 21, 266, 2, 3, "감강"},
	{"終聲 ㄴ/ㅇ", 2, 4, 21, 266, 8, 13, "곤공"},
	{"初聲 ㄷ/ㅌ", 0, 3, 16, 152, 3, 5, "다타"},
	{"初聲 ㄱ/ㅋ", 0, 0, 15, 152, 3, 4, "각칵"},
}

// koFamilyPairs lists the pairs of a family in the name set: two syllables whose
// other two positions are equal and whose position has a in one and b in the
// other.
func koFamilyPairs(set []rune, fam koFamily) [][2]rune {
	byKey := map[[3]int]rune{}
	for _, r := range set {
		if l, v, f, ok := koSyllable(r); ok {
			byKey[[3]int{l, v, f}] = r
		}
	}
	var out [][2]rune
	for k, r := range byKey {
		if k[fam.pos] != fam.a {
			continue
		}
		k2 := k
		k2[fam.pos] = fam.b
		if r2, ok := byKey[k2]; ok {
			out = append(out, [2]rune{r, r2})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// The families of confused phonemes keep their distance at L2 (spec 058 §5.3):
// the smallest difference, in pixels, of any pair of a family is at least the
// recorded value, the number of pairs is the recorded one (so a family
// enumerator that finds nothing cannot pass), and the representative pair
// reaches the 2× minimum.
func TestShrinkKoFamilies(t *testing.T) {
	f, set := koFormal(t)
	ko := shrinkLevelsFor(LangKo)
	for _, scale := range []int{2, 3} {
		fs := shrinkFontsOf(f, ko[1], scale)
		for _, fam := range koFamilies {
			pairs := koFamilyPairs(set, fam)
			if len(pairs) != fam.pairs {
				t.Errorf("%d× %s: %d pairs in the name set, expected %d", scale, fam.name, len(pairs), fam.pairs)
			}
			minD := 1 << 30
			for _, p := range pairs {
				if d := glyphDistance(fs.Full.Glyphs[p[0]], fs.Full.Glyphs[p[1]]); d < minD {
					minD = d
				}
			}
			bound := fam.min2
			if scale == 3 {
				bound = fam.m3
			}
			if minD < bound {
				t.Errorf("%d× L2 %s: the smallest difference is %d pixels, need %d", scale, fam.name, minD, bound)
			}
			if scale == 2 {
				rr := []rune(fam.rep)
				if d := glyphDistance(fs.Full.Glyphs[rr[0]], fs.Full.Glyphs[rr[1]]); d != minD {
					t.Errorf("2× %s: the representative pair %s differs by %d pixels, the family minimum is %d", fam.name, fam.rep, d, minD)
				}
			}
			t.Logf("%d× L2 %s: %d pairs, smallest difference %d", scale, fam.name, len(pairs), minD)
		}
	}
}

// Pairs that the table of spec 056 draws the same (spec 058 §1) are different
// at both levels and both scales.
func TestShrinkKoRepresentativePairs(t *testing.T) {
	f, _ := koFormal(t)
	for _, scale := range []int{2, 3} {
		for _, sp := range shrinkLevelsFor(LangKo) {
			fs := shrinkFontsOf(f, sp, scale)
			for _, p := range []string{"게계", "네녜", "겍곅", "갠갱", "독톡"} {
				rr := []rune(p)
				if d := glyphDistance(fs.Full.Glyphs[rr[0]], fs.Full.Glyphs[rr[1]]); d == 0 {
					t.Errorf("%d× L%d: %s are drawn the same", scale, sp.Level, p)
				}
			}
		}
	}
	// The negative control: the table of spec 056 draws the pairs of the
	// problem the same (게/계 at 2×, 갠/갱 at 3×).
	old2 := shrinkFontsOf(f, shrinkLevels[1], 2)
	old3 := shrinkFontsOf(f, shrinkLevels[1], 3)
	if glyphDistance(old2.Full.Glyphs['게'], old2.Full.Glyphs['계']) != 0 || glyphDistance(old3.Full.Glyphs['갠'], old3.Full.Glyphs['갱']) != 0 {
		t.Errorf("the L2 of spec 056 was expected to draw 게/계 (2×) and 갠/갱 (3×) the same")
	}
}

// The double stroke of ㅔ stays: 셀 differs from 설 (one stroke) and from 샐 (ㅐ),
// and its bitmaps are frozen at both scales (spec 058 §5.3).
func TestShrinkKoSnapshots(t *testing.T) {
	f, _ := koFormal(t)
	for _, c := range []struct {
		scale int
		w, h  int
		want  string
	}{
		{2, 10, 12, koSel2x}, {3, 15, 16, koSel3x},
	} {
		fs := shrinkFontsOf(f, shrinkLevelsFor(LangKo)[1], c.scale)
		got := glyphArt(fs.Full.Glyphs['셀'], c.w, c.h)
		if fs.Full.W != c.w || fs.Full.H != c.h || got != c.want {
			t.Errorf("%d× 셀 %dx%d:\n%s\nwant\n%s", c.scale, fs.Full.W, fs.Full.H, got, c.want)
		}
		for _, other := range []rune{'설', '샐'} {
			if d := glyphDistance(fs.Full.Glyphs['셀'], fs.Full.Glyphs[other]); d < 2 {
				t.Errorf("%d×: 셀 and %c differ by %d pixels", c.scale, other, d)
			}
		}
	}
}

// The derived glyphs of a fixed probe set at every level and scale of the ko
// table, as a digest (frozen): the representative pairs of the families, the
// pairs of the problem and the names of the examples (spec 058 §5.3).
func TestShrinkKoProbeDigests(t *testing.T) {
	f, _ := koFormal(t)
	var probe []rune
	for _, fam := range koFamilies {
		probe = append(probe, []rune(fam.rep)...)
	}
	for _, p := range []string{"게계", "네녜", "겍곅", "갠갱", "독톡", "셀레스트", "플라비우스", "마리온", "알키메데스"} {
		probe = append(probe, []rune(p)...)
	}
	got := map[string]string{}
	for _, scale := range []int{2, 3} {
		for _, sp := range shrinkLevelsFor(LangKo) {
			got[fmt.Sprintf("L%d/%d×", sp.Level, scale)] = shrinkProbeDigest(shrinkFontsOf(f, sp, scale).Full, probe)
		}
	}
	for k, want := range koProbeDigests {
		if got[k] != want {
			t.Errorf("%s: %s want %s", k, got[k], want)
		}
	}
	if len(koProbeDigests) != len(got) {
		t.Errorf("%d digests, %d frozen", len(got), len(koProbeDigests))
	}
}

// Frozen on the formal ko font with the table of spec 058 §3.3.
const koSel2x = `..........
..........
...#...#.#
...#..##.#
..###.##.#
..#.#..#.#
.#...#.#.#
..........
..#######.
........#.
..#######.
...######.
`

const koSel3x = `...............
...............
.....#....#..#.
.....#....#..#.
.....#....#..#.
....###.###..#.
....###...#..#.
...#..##..#..#.
..#....##.#..#.
..........#..#.
...............
....##########.
.............#.
....#########..
....#..........
.....#########.
`

var koProbeDigests = map[string]string{
	"L1/2×": "ae7571c58f8b417c5c5ba15b3d1ad125d6fa61a04989b1d478d01f2ed5d35d3c",
	"L1/3×": "01e6032ceb0e1acbf8ac8127125825e57244129db5bd25add9b93bfaf075647a",
	"L2/2×": "ea264b3f550cb7d03900c0ae3555baa5940b14a61e74ca2f636ea83e52dbc976",
	"L2/3×": "9596abe34128579a65808bfbfb8e2b1a36b62eb9a619b149db3baa68dec7e2b2",
}
