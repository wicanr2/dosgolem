package dkk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
)

// MenuMasters 是底列選單母串表（規格 003 §2.2.1）：選單模組收整串母串，引擎只畫其中幾個詞，
// 畫面上的詞列是母串的子序列。表內每個母串存逐詞的長度＋雜湊與譯文，不含原文。
type MenuMasters struct {
	masters [][]menuMasterWord
}

type menuMasterWord struct {
	hash string // hashKey(長度, SHA-256)
	zh   string
}

// LoadMenuMasters 讀 text/menu-masters.tsv（master、index、original_length、original_sha256、translation）。
func LoadMenuMasters(b []byte) (*MenuMasters, error) {
	rows, err := readTSV(b, []string{"master", "index", "original_length", "original_sha256", "translation"})
	if err != nil {
		return nil, fmt.Errorf("dkk: 選單母串表：%w", err)
	}
	m := &MenuMasters{}
	at := map[string]int{}
	for i, r := range rows {
		idx, err1 := strconv.Atoi(r[1])
		n, err2 := strconv.Atoi(r[2])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("dkk: 選單母串表第 %d 列格式錯誤", i+2)
		}
		k, ok := at[r[0]]
		if !ok {
			k = len(m.masters)
			at[r[0]] = k
			m.masters = append(m.masters, nil)
		}
		if idx != len(m.masters[k]) {
			return nil, fmt.Errorf("dkk: 選單母串 %s 的詞序不連續（第 %d 列）", r[0], i+2)
		}
		m.masters[k] = append(m.masters[k], menuMasterWord{hash: hashKey(n, r[3]), zh: r[4]})
	}
	return m, nil
}

// Words 回畫面詞列逐詞的譯文：找出以這列為子序列（依序、可跳詞）的母串；
// 所有候選母串對某詞的譯文一致才採用，否則該詞回空字串（顯示原版）。沒有候選回 nil。
func (m *MenuMasters) Words(words []string) []string {
	if m == nil || len(words) == 0 {
		return nil
	}
	hs := make([]string, len(words))
	for i, w := range words {
		sum := sha256.Sum256([]byte(w))
		hs[i] = hashKey(len(w), hex.EncodeToString(sum[:]))
	}
	var out []string
	for _, ms := range m.masters {
		zh := make([]string, 0, len(hs))
		j := 0
		for _, w := range ms {
			if j < len(hs) && w.hash == hs[j] {
				zh = append(zh, w.zh)
				j++
			}
		}
		if j < len(hs) {
			continue
		}
		if out == nil {
			out = zh
			continue
		}
		for i := range out {
			if out[i] != zh[i] {
				out[i] = ""
			}
		}
	}
	return out
}
