package translit

import (
	"path/filepath"
	"sync"
)

// Buck repo spec 044 §3.1: the front end that zh shares with the Japanese and
// Korean transliterators (xlate/translitjk).  These are wrappers: the code
// they call is the one zh has always used, so nothing about the zh output
// changes (zh_fulldict_test.go holds the digest).

// Phone is one ARPAbet phoneme; Stress is -1 for a consonant.
type Phone struct {
	Sym    string
	Stress int
	Vowel  bool
}

func toPhones(in []ph) []Phone {
	out := make([]Phone, len(in))
	for i, p := range in {
		out[i] = Phone{p.sym, p.stress, p.vowel}
	}
	return out
}

func fromPhones(in []Phone) []ph {
	out := make([]ph, len(in))
	for i, p := range in {
		out[i] = ph{p.Sym, p.Stress, p.Vowel}
	}
	return out
}

// ParsePron parses the fields of one CMUdict pronunciation (stress digits on
// the vowels); false when a field is not an ARPAbet phoneme.
func ParsePron(pron []string) ([]Phone, bool) {
	p, ok := parseARPAbet(pron)
	if !ok {
		return nil, false
	}
	return toPhones(p), true
}

// SpellingPhones is the spelling fallback of spec 037 §3.2 (tier 3): letters
// to approximate phonemes.  letters is lower case, without apostrophes and
// hyphens; anything else is cleaned out first.
func SpellingPhones(letters string) []Phone {
	b := make([]byte, 0, len(letters))
	for i := 0; i < len(letters); i++ {
		c := letters[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c >= 'a' && c <= 'z' {
			b = append(b, c)
		}
	}
	return toPhones(spellingPhones(string(b)))
}

// AlignVowels pairs every vowel phoneme with the group of vowel letters it
// is spelled with (spec 044 §3.3 F2); nil when the two sides do not align.
func AlignVowels(letters string, phones []Phone) []string {
	return alignVowels(letters, fromPhones(phones))
}

// cmuEntry is one cached dictionary: its own Once, the table and the error.
type cmuEntry struct {
	once sync.Once
	m    map[string][]string
	err  error
}

var (
	cmuMu    sync.Mutex
	cmuCache = map[string]*cmuEntry{}
)

// SharedCMU returns the CMU pronunciation dictionary of path, parsed once per
// process and shared by every transliterator (zh-TW, zh-CN, ja, ko).  The
// table is read only; a caller must not write to it.  A parse error is not
// cached: the next call parses again.
func SharedCMU(path string) (map[string][]string, error) {
	key, err := filepath.Abs(path)
	if err != nil {
		key = path
	}
	cmuMu.Lock()
	e := cmuCache[key]
	if e == nil {
		e = &cmuEntry{}
		cmuCache[key] = e
	}
	cmuMu.Unlock()
	e.once.Do(func() { e.m, e.err = parseCMU(path) })
	if e.err != nil {
		cmuMu.Lock()
		if cmuCache[key] == e {
			delete(cmuCache, key)
		}
		cmuMu.Unlock()
		return nil, e.err
	}
	return e.m, nil
}
