// Package dkk 是《Death Knights of Krynn》（DOS 1.10）的繁中覆繪 adapter。
//
// 本套件只放這款遊戲專屬的東西：印字程序的位元組簽章、參數版面、譯文查表。
// 通用的疊字、字型與畫面處理在 xlate；機器在 internal/machine。
// 規格與證據在 death-knights-of-krynn-cht 儲存庫的 docs/spec、docs/re。
package dkk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Catalog 以原字串的「長度＋SHA-256」查譯文（規格 001 §2.2）。
// 原文不進 catalog；events 檔只有 key、長度、雜湊。
type Catalog struct {
	byHash map[string]Entry
}

// Entry 是一筆命中的譯文。
type Entry struct {
	Key         string
	Translation string
}

func hashKey(n int, sum string) string { return strconv.Itoa(n) + ":" + sum }

// LoadCatalog 讀 events（event_key、original_length、original_sha256、sources）與
// 譯文（key、translation、source）兩份 TSV。只有兩邊都有的 key 才收。
func LoadCatalog(events, translations []byte) (*Catalog, error) {
	trans, err := readTSV(translations, []string{"key", "translation"})
	if err != nil {
		return nil, fmt.Errorf("dkk: 譯文 TSV：%w", err)
	}
	zh := make(map[string]string, len(trans))
	for _, r := range trans {
		if _, dup := zh[r[0]]; dup {
			return nil, fmt.Errorf("dkk: 譯文 key 重複：%s", r[0])
		}
		if !utf8.ValidString(r[1]) || r[1] == "" {
			return nil, fmt.Errorf("dkk: 譯文 %s 為空或不是 UTF-8", r[0])
		}
		zh[r[0]] = r[1]
	}
	ev, err := readTSV(events, []string{"event_key", "original_length", "original_sha256"})
	if err != nil {
		return nil, fmt.Errorf("dkk: events TSV：%w", err)
	}
	c := &Catalog{byHash: make(map[string]Entry, len(ev))}
	for _, r := range ev {
		n, err := strconv.Atoi(r[1])
		if err != nil || n <= 0 || n > 255 {
			return nil, fmt.Errorf("dkk: events %s 長度 %q 不合法", r[0], r[1])
		}
		if len(r[2]) != 64 {
			return nil, fmt.Errorf("dkk: events %s 雜湊長度不對", r[0])
		}
		t, ok := zh[r[0]]
		if !ok {
			continue
		}
		hk := hashKey(n, r[2])
		if prev, dup := c.byHash[hk]; dup {
			return nil, fmt.Errorf("dkk: 雜湊重複：%s 與 %s", prev.Key, r[0])
		}
		c.byHash[hk] = Entry{Key: r[0], Translation: t}
	}
	return c, nil
}

// Lookup 以原字串 bytes 查譯文。
func (c *Catalog) Lookup(original []byte) (Entry, bool) {
	if c == nil || c.byHash == nil {
		return Entry{}, false
	}
	sum := sha256.Sum256(original)
	e, ok := c.byHash[hashKey(len(original), hex.EncodeToString(sum[:]))]
	return e, ok
}

// Len 回收錄的譯文數。
func (c *Catalog) Len() int { return len(c.byHash) }

// readTSV 以 tab 切欄、換行切列，不處理引號：譯文與原文常含 `"`，
// encoding/csv 會把它當成引號欄而吞掉換行（2026-10-15 實測：869 則只讀到 505 則）。
func readTSV(b []byte, header []string) ([][]string, error) {
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("空檔")
	}
	head := strings.Split(strings.TrimRight(lines[0], "\r"), "\t")
	for i, h := range header {
		if i >= len(head) || head[i] != h {
			return nil, fmt.Errorf("表頭應以 %v 開頭，得到 %v", header, head)
		}
	}
	var rows [][]string
	for n, l := range lines[1:] {
		rec := strings.Split(strings.TrimRight(l, "\r"), "\t")
		if len(rec) < len(header) {
			return nil, fmt.Errorf("第 %d 列欄數不足", n+2)
		}
		rows = append(rows, rec)
	}
	return rows, nil
}
