package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Spec 029 (Buck repo): engine messages are fixed fragments joined with
// names and numbers at run time.  A string is split into fragments, item
// names and plain slots; the Chinese is the fragments' translations joined
// in order (or a template when word order must change).  Only hashes of the
// original text are loaded; the original bytes seen at run time are hashed
// piecewise and never stored.

type engineID struct {
	n   int
	sum [32]byte
}

func engineIDOf(s string) engineID { return engineID{len(s), sha256.Sum256([]byte(s))} }

type EngineTextCatalog struct {
	coordDir  map[byte]string     // spec 029 §2.8: N/E/S/W -> one Chinese character
	frag      map[engineID]string // fragment id -> key
	fragText  map[string]string   // key -> Chinese
	fragLens  []int
	item      map[engineID]string
	itemText  map[string]string
	itemLens  []int
	phrase    map[engineID]string // whole item name -> Chinese
	templates map[string]string   // signature -> Chinese with {0}..{n}
	monster   map[engineID]string // monster name -> Chinese
}

func loadHashRows(name string, data []byte) (map[engineID]string, []int, error) {
	rows, err := readTSV(name, data, []string{"event_key", "original_length", "original_sha256"})
	if err != nil {
		return nil, nil, err
	}
	out := map[engineID]string{}
	lens := map[int]bool{}
	for _, r := range rows {
		n, err := strconv.Atoi(r[1])
		b, herr := hex.DecodeString(r[2])
		if err != nil || n <= 0 || herr != nil || len(b) != 32 {
			return nil, nil, fmt.Errorf("buckrogers: %s 列 %s 無效", name, r[0])
		}
		var id engineID
		id.n = n
		copy(id.sum[:], b)
		if _, dup := out[id]; dup {
			return nil, nil, fmt.Errorf("buckrogers: %s 雜湊重複 %s", name, r[0])
		}
		out[id] = r[0]
		lens[n] = true
	}
	var ls []int
	for n := range lens {
		ls = append(ls, n)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ls)))
	return out, ls, nil
}

func loadTextRows(name string, data []byte, keys map[string]bool) (map[string]string, error) {
	out := map[string]string{}
	if data == nil {
		return out, nil
	}
	rows, err := readTSV(name, data, []string{"key", "translation", "source"})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if keys != nil && !keys[r[0]] || r[1] == "" || out[r[0]] != "" {
			return nil, fmt.Errorf("buckrogers: %s 列 %s 無效", name, r[0])
		}
		out[r[0]] = r[1]
	}
	return out, nil
}

// EngineTextFiles holds the raw catalog files; nil optional files are empty.
type EngineTextFiles struct {
	FragmentEvents, FragmentText []byte
	ItemEvents, ItemText         []byte
	PhraseEvents, PhraseText     []byte
	TemplateEvents, TemplateText []byte
	MonsterEvents, MonsterText   []byte
}

func LoadEngineTextCatalog(f EngineTextFiles) (*EngineTextCatalog, error) {
	c := &EngineTextCatalog{phrase: map[engineID]string{}, templates: map[string]string{}, monster: map[engineID]string{}}
	var err error
	if c.frag, c.fragLens, err = loadHashRows("engine-fragment-events.tsv", f.FragmentEvents); err != nil {
		return nil, err
	}
	if c.item, c.itemLens, err = loadHashRows("item-word-events.tsv", f.ItemEvents); err != nil {
		return nil, err
	}
	keys := func(m map[engineID]string) map[string]bool {
		out := map[string]bool{}
		for _, k := range m {
			out[k] = true
		}
		return out
	}
	if c.fragText, err = loadTextRows("engine-fragment.zh-TW.tsv", f.FragmentText, keys(c.frag)); err != nil {
		return nil, err
	}
	if c.itemText, err = loadTextRows("item-word.zh-TW.tsv", f.ItemText, keys(c.item)); err != nil {
		return nil, err
	}
	if f.PhraseEvents != nil {
		ids, _, err := loadHashRows("item-phrase-events.tsv", f.PhraseEvents)
		if err != nil {
			return nil, err
		}
		text, err := loadTextRows("item-phrase.zh-TW.tsv", f.PhraseText, keys(ids))
		if err != nil {
			return nil, err
		}
		for id, k := range ids {
			if t := text[k]; t != "" {
				c.phrase[id] = t
			}
		}
	}
	if f.MonsterEvents != nil {
		ids, _, err := loadHashRows("monster-name-events.tsv", f.MonsterEvents)
		if err != nil {
			return nil, err
		}
		text, err := loadTextRows("monster-name.zh-TW.tsv", f.MonsterText, keys(ids))
		if err != nil {
			return nil, err
		}
		for id, k := range ids {
			if t := text[k]; t != "" {
				c.monster[id] = t
			}
		}
	}
	if f.TemplateEvents != nil {
		rows, err := readTSV("engine-template-events.tsv", f.TemplateEvents, []string{"event_key", "signature"})
		if err != nil {
			return nil, err
		}
		sigOf := map[string]string{}
		for _, r := range rows {
			sigOf[r[0]] = r[1]
		}
		text, err := loadTextRows("engine-template.zh-TW.tsv", f.TemplateText, nil)
		if err != nil {
			return nil, err
		}
		for k, t := range text {
			if sig, ok := sigOf[k]; ok {
				c.templates[sig] = t
			}
		}
	}
	return c, nil
}

// EnginePart is one piece of a decomposed string.
type EnginePart struct {
	Kind byte   // 'F' fragment, 'I' item name, '_' plain slot
	Text string // original text (only kept for the current call)
	Key  string // fragment key for 'F'
}

func engineAlpha(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }

func engineBoundary(s string, p int) bool {
	return p == 0 || p == len(s) || !(engineAlpha(s[p-1]) && engineAlpha(s[p]))
}

var engineLower = regexp.MustCompile(`[a-z]`)

func (c *EngineTextCatalog) fragKey(s string, i, j int) (string, bool) {
	t := s[i:j]
	k, ok := c.frag[engineIDOf(t)]
	if !ok || len(t) < 3 || !engineLower.MatchString(t) || !engineBoundary(s, i) || !engineBoundary(s, j) {
		return "", false
	}
	return k, true
}

func (c *EngineTextCatalog) itemUnit(s string, q int) int {
	for _, n := range c.itemLens {
		if q+n <= len(s) && engineBoundary(s, q+n) {
			if _, ok := c.item[engineIDOf(s[q:q+n])]; ok {
				return n
			}
		}
	}
	return 0
}

var engineAmmo = regexp.MustCompile(`^ \(\d`)

func (c *EngineTextCatalog) itemRuns(s string) [][2]int {
	var runs [][2]int
	for p := 0; p < len(s); {
		if !engineBoundary(s, p) {
			p++
			continue
		}
		q, n := p, 0
		for {
			u := c.itemUnit(s, q)
			if u == 0 {
				break
			}
			q += u
			n++
			if q+1 < len(s) && s[q] == ' ' && engineBoundary(s, q+1) && c.itemUnit(s, q+1) > 0 {
				q++
				continue
			}
			break
		}
		if n >= 2 || n == 1 && engineAmmo.MatchString(s[q:]) {
			runs = append(runs, [2]int{p, q})
			p = q
		} else {
			p++
		}
	}
	return runs
}

// Decompose implements spec 029 §2.2.
func (c *EngineTextCatalog) Decompose(s string) []EnginePart {
	if k, ok := c.frag[engineIDOf(s)]; ok && s != "" {
		return []EnginePart{{Kind: 'F', Text: s, Key: k}}
	}
	var out []EnginePart
	last := 0
	for _, r := range c.itemRuns(s) {
		out = append(out, c.segment(s[last:r[0]])...)
		out = append(out, EnginePart{Kind: 'I', Text: s[r[0]:r[1]]})
		last = r[1]
	}
	out = append(out, c.segment(s[last:])...)
	var merged []EnginePart
	for _, p := range out {
		if p.Kind == '_' && len(merged) > 0 && merged[len(merged)-1].Kind == '_' {
			merged[len(merged)-1].Text += p.Text
			continue
		}
		merged = append(merged, p)
	}
	return merged
}

// segment: maximum fragment coverage, then fewest pieces (tie on the
// earliest better cost, matching the Python reference).
func (c *EngineTextCatalog) segment(s string) []EnginePart {
	n := len(s)
	if n == 0 {
		return nil
	}
	type cell struct {
		cov, pieces int
		from        int
		frag        bool
		key         string
		set         bool
	}
	best := make([]cell, n+1)
	best[0] = cell{set: true}
	for i := 0; i < n; i++ {
		if !best[i].set {
			continue
		}
		for j := i + 1; j <= n; j++ {
			k, isf := c.fragKey(s, i, j)
			cov := best[i].cov
			if isf {
				cov += j - i
			}
			cand := cell{cov: cov, pieces: best[i].pieces + 1, from: i, frag: isf, key: k, set: true}
			b := best[j]
			if !b.set || cand.cov > b.cov || cand.cov == b.cov && cand.pieces < b.pieces {
				best[j] = cand
			}
		}
	}
	var out []EnginePart
	for j := n; j > 0; {
		b := best[j]
		kind := byte('_')
		if b.frag {
			kind = 'F'
		}
		out = append(out, EnginePart{Kind: kind, Text: s[b.from:j], Key: b.key})
		j = b.from
	}
	for i, k := 0, len(out)-1; i < k; i, k = i+1, k-1 {
		out[i], out[k] = out[k], out[i]
	}
	return out
}

// Signature joins fragment keys, 'I' and '_'.
func EngineSignature(parts []EnginePart) string {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteByte('|')
		}
		if p.Kind == 'F' {
			b.WriteString(p.Key)
		} else {
			b.WriteByte(p.Kind)
		}
	}
	return b.String()
}

func (c *EngineTextCatalog) itemChinese(t string) (string, bool) {
	if z, ok := c.phrase[engineIDOf(t)]; ok {
		return z, true
	}
	var b strings.Builder
	for q := 0; q < len(t); {
		if t[q] == ' ' {
			q++
			continue
		}
		u := c.itemUnit(t, q)
		if u == 0 {
			return "", false
		}
		z := c.itemText[strings.TrimSuffix(c.item[engineIDOf(t[q:q+u])], ".uc")]
		if z == "" {
			return "", false
		}
		b.WriteString(z)
		q += u
	}
	return b.String(), true
}

// monsterSlot translates a plain slot that is exactly a monster name,
// keeping its surrounding spaces and trailing punctuation.
func (c *EngineTextCatalog) monsterSlot(t string) (string, bool) {
	core := strings.TrimSpace(t)
	lead := t[:strings.Index(t, core)]
	trail := t[len(lead)+len(core):]
	punct := ""
	for len(core) > 0 && strings.ContainsRune(".,!?", rune(core[len(core)-1])) {
		punct = core[len(core)-1:] + punct
		core = core[:len(core)-1]
	}
	if core == "" {
		return "", false
	}
	z, ok := c.monster[engineIDOf(core)]
	if !ok {
		return "", false
	}
	if punct == "." {
		punct = "。"
	}
	return lead + z + punct + trail, true
}

// matchTemplate finds a template whose signature equals the parts, where a
// '*' element matches any part.  It returns the indexes of the parts the
// template's {0}.. placeholders refer to: every element that is not a
// fragment key ('_', 'I' or '*').
func (c *EngineTextCatalog) matchTemplate(parts []EnginePart) (string, []int, bool) {
	sig := strings.Split(EngineSignature(parts), "|")
	patterns := make([]string, 0, len(c.templates))
	for p := range c.templates {
		patterns = append(patterns, p)
	}
	// Exact signatures before wildcard ones, then lexical: deterministic.
	sort.Slice(patterns, func(i, j int) bool {
		wi, wj := strings.Contains(patterns[i], "*"), strings.Contains(patterns[j], "*")
		if wi != wj {
			return !wi
		}
		return patterns[i] < patterns[j]
	})
	for _, pattern := range patterns {
		t := c.templates[pattern]
		pat := strings.Split(pattern, "|")
		if len(pat) != len(sig) {
			continue
		}
		var slots []int
		ok := true
		for i, e := range pat {
			switch {
			case e == "*" || e == "_" || e == "I":
				if e != "*" && e != sig[i] {
					ok = false
				}
				slots = append(slots, i)
			case e != sig[i]:
				ok = false
			}
			if !ok {
				break
			}
		}
		if ok {
			return t, slots, true
		}
	}
	return "", nil, false
}

func engineAlnum(c byte) bool { return engineAlpha(c) || c >= '0' && c <= '9' }

// Translate returns the Chinese for s, or false (spec 029 §2.4).
func (c *EngineTextCatalog) Translate(s string) (string, bool) {
	if c == nil || s == "" {
		return "", false
	}
	// Spec 029 §2.8: the exploration coordinate line swaps only the compass
	// letter; any caller, since the format is unmistakable.
	if c.coordDir != nil && engineCoordLine.MatchString(s) {
		i := strings.IndexByte(s, ' ') + 1
		return s[:i] + c.coordDir[s[i]] + s[i+1:], true
	}
	// Spec 029 §2.6: a table row keeps its last column where it was.
	if m := engineTableRow.FindStringSubmatch(s); m != nil {
		front, ok := c.translateLine(m[1])
		n, last := len([]rune(s)), len(m[3])
		if !ok || len([]rune(front))+1+last > n {
			return "", false
		}
		return front + strings.Repeat(" ", n-len([]rune(front))-last) + m[3], true
	}
	return c.translateLine(s)
}

var engineCoordLine = regexp.MustCompile(`^[0-9]{1,2},[0-9]{1,2} [NESW] [0-9]{2}:[0-9]{2}$`)

// LoadCoordinateText reads text/coordinate-line.zh-TW.tsv: all four
// compass keys, each translated to exactly one character.
func (c *EngineTextCatalog) LoadCoordinateText(data []byte) error {
	rows, err := readTSV("coordinate-line.zh-TW.tsv", data, []string{"key", "translation", "source"})
	if err != nil {
		return err
	}
	dir := map[byte]string{}
	for _, r := range rows {
		k := strings.TrimPrefix(r[0], "coord.dir.")
		if len(k) != 1 || !strings.Contains("NESW", k) || len([]rune(r[1])) != 1 {
			return fmt.Errorf("buckrogers: 座標列譯文 %s 無效", r[0])
		}
		dir[k[0]] = r[1]
	}
	if len(dir) != 4 {
		return fmt.Errorf("buckrogers: 座標列譯文需有 N、E、S、W 四個方位")
	}
	c.coordDir = dir
	return nil
}

// engineTableRow: text, two or more spaces, and a last column of digits
// and , / ( ) %.
var engineTableRow = regexp.MustCompile(`^(.*[^ ])( {2,})([0-9,/()%]*[0-9][0-9,/()%]*)$`)

func (c *EngineTextCatalog) translateLine(s string) (string, bool) {
	if z, ok := c.monsterSlot(s); ok {
		return z, true
	}
	parts := c.Decompose(s)
	hasFixed := false
	zh := make([]string, len(parts))
	for i, p := range parts {
		switch p.Kind {
		case 'F':
			hasFixed = true
			z := c.fragText[strings.TrimSuffix(p.Key, ".uc")]
			if z == "" {
				if _, _, tmpl := c.matchTemplate(parts); !tmpl {
					return "", false
				}
			}
			zh[i] = z
		case 'I':
			hasFixed = true
			z, ok := c.itemChinese(p.Text)
			if !ok {
				return "", false
			}
			zh[i] = z
		default:
			if z, ok := c.monsterSlot(p.Text); ok {
				zh[i] = z
			} else {
				zh[i] = p.Text
			}
		}
	}
	if !hasFixed {
		return "", false
	}
	if t, slots, ok := c.matchTemplate(parts); ok {
		for i, v := range slots {
			t = strings.ReplaceAll(t, "{"+strconv.Itoa(i)+"}", strings.TrimSpace(zh[v]))
		}
		return t, true
	}
	var b strings.Builder
	for i, p := range parts {
		z := zh[i]
		if p.Kind == 'F' {
			if strings.HasPrefix(p.Text, " ") && i > 0 && parts[i-1].Kind == '_' && len(zh[i-1]) > 0 && engineAlnum(zh[i-1][len(zh[i-1])-1]) {
				z = " " + z
			}
			if strings.HasSuffix(p.Text, " ") && i+1 < len(parts) && parts[i+1].Kind == '_' && len(zh[i+1]) > 0 && engineAlnum(zh[i+1][0]) {
				z += " "
			}
		}
		b.WriteString(z)
	}
	return b.String(), true
}
