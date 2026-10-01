package translit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Buck repo spec 044 §3.1 (zh 不變的護欄): every pure-letter word of the full
// CMU dictionary, each gender, through Transliterate; the digest of the
// result lines must not change when the front end is exported or shared.
// The test needs BUCKROGERS_CHT_ROOT (the Buck repo, for text/); without it
// the test skips with its reason, it never passes silently.
//
// zhFullDictDigest is the digest of this package at the commit before spec
// 044 touched it (the line format below is part of the digest).  Do not edit.
const zhFullDictDigest = "eb7e6c07c51ed26339e17d199c45f82df8afc50df8acf2c4df42b77fe685cb26"

func TestZhFullDictionaryDigest(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定：全字典摘要未檢查")
	}
	tr, err := Load(filepath.Join(root, "text"))
	if err != nil {
		t.Fatal(err)
	}
	var words []string
	for w := range tr.cmu {
		if isLowerLetters(w) {
			words = append(words, strings.ToUpper(w))
		}
	}
	sort.Strings(words)
	h := sha256.New()
	rows := 0
	for _, w := range words {
		for _, g := range []Gender{GenderUnknown, Male, Female} {
			zh, tier, ok := tr.Transliterate(w, g)
			fmt.Fprintf(h, "%s\t%d\t%s\t%s\t%v\n", w, g, zh, tier, ok)
			rows++
		}
	}
	got := hex.EncodeToString(h.Sum(nil))
	t.Logf("全字典：%d 詞、%d 列，摘要 %s", len(words), rows, got)
	if got != zhFullDictDigest {
		t.Errorf("摘要 %s，應為 %s", got, zhFullDictDigest)
	}
}

func isLowerLetters(w string) bool {
	if w == "" {
		return false
	}
	for i := 0; i < len(w); i++ {
		if w[i] < 'a' || w[i] > 'z' {
			return false
		}
	}
	return true
}
