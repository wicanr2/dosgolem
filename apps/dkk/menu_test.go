package dkk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func menuTable(masters map[string][][2]string) []byte {
	var b strings.Builder
	b.WriteString("master\tindex\toriginal_length\toriginal_sha256\ttranslation\n")
	for _, k := range []string{"m1", "m2", "m3"} {
		for i, w := range masters[k] {
			sum := sha256.Sum256([]byte(w[0]))
			fmt.Fprintf(&b, "%s\t%d\t%d\t%s\t%s\n", k, i, len(w[0]), hex.EncodeToString(sum[:]), w[1])
		}
	}
	return []byte(b.String())
}

// TestMenuMastersSubsequence 檢查規格 003 §2.2.1：子序列比對、候選一致才採用、沒有候選回 nil。
func TestMenuMastersSubsequence(t *testing.T) {
	m, err := LoadMenuMasters(menuTable(map[string][][2]string{
		"m1": {{"Ready", "備妥"}, {"Use", "使用"}, {"Trade", "交易"}, {"Drop", "丟棄"}, {"Exit", "離開"}},
		"m2": {{"Order", "排列"}, {"Drop", "丟棄隊員"}, {"Exit", "離開"}},
		"m3": {{"Items", "物品"}, {"Trade", "交易"}, {"Drop", "丟棄"}, {"Exit", "離開"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Words([]string{"Use", "Drop", "Exit"}); strings.Join(got, ",") != "使用,丟棄,離開" {
		t.Errorf("子序列：%q", got)
	}
	// Drop Exit 是三個母串的子序列：Drop 譯文不一致回空，Exit 一致
	if got := m.Words([]string{"Drop", "Exit"}); strings.Join(got, ",") != ",離開" {
		t.Errorf("候選不一致：%q", got)
	}
	if got := m.Words([]string{"Exit", "Drop"}); got != nil {
		t.Errorf("順序不符應回 nil：%q", got)
	}
	if got := m.Words([]string{"Fly"}); got != nil {
		t.Errorf("不在表中應回 nil：%q", got)
	}
}

func TestMenuMastersRejectsGap(t *testing.T) {
	b := []byte("master\tindex\toriginal_length\toriginal_sha256\ttranslation\nm\t1\t1\tab\t甲\n")
	if _, err := LoadMenuMasters(b); err == nil {
		t.Error("詞序從 1 開始應回錯")
	}
}
