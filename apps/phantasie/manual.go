package phantasie

import (
	"fmt"
	"strconv"
	"strings"
)

// 本機手冊提示（專案規格 006）。答案不內嵌，只讀取外部 TSV。
// 位址為執行期 image segment 內的返回偏移；FmtPtr 為 DGROUP 偏移。
func manualKey(rec *EventRecord) string {
	if rec == nil || rec.Overlay != "ov2" || rec.FmtKind != KindStatic || rec.Composed != nil || rec.Col != 0 {
		return ""
	}
	switch {
	case rec.Caller == 0xB795 && rec.FmtPtr == 0xC1BC && rec.Row == 10 && rec.Format == "What is the item number of a%c":
		if len(rec.ArgStrs) == 0 && (rec.Args[0] == ' ' || rec.Args[0] == 'n') && rec.Text == fmt.Sprintf(rec.Format, rec.Args[0]) {
			return "prompt:item"
		}
	case rec.Caller == 0xB7B4 && rec.FmtPtr == 0xC1DB && rec.Row == 11 && rec.Format == "%s? (page 15,16)":
		if len(rec.ArgStrs) != 1 {
			return ""
		}
		a := rec.ArgStrs[0]
		if a.Kind == KindStatic && a.Piece == nil && a.Ptr == rec.Args[0] && a.Content != "" && printable(a.Content) && rec.Text == fmt.Sprintf(rec.Format, a.Content) {
			return "item:" + a.Content
		}
	case rec.Caller == 0xB848 && rec.FmtPtr == 0xC1F3 && rec.Row == 10 && rec.Format == "What is the name":
		if len(rec.ArgStrs) == 0 && rec.Text == rec.Format {
			return "prompt:spell"
		}
	case rec.Caller == 0xB861 && rec.FmtPtr == 0xC204 && rec.Row == 11 && rec.Format == "of spell %d? (back cover)":
		if len(rec.ArgStrs) == 0 && rec.Args[0] >= 1 && rec.Args[0] <= 54 && rec.Text == fmt.Sprintf(rec.Format, rec.Args[0]) {
			return "spell:" + strconv.Itoa(int(rec.Args[0]))
		}
	}
	return ""
}

func validManualKey(key string) bool {
	if key == "prompt:item" || key == "prompt:spell" {
		return true
	}
	if name, ok := strings.CutPrefix(key, "item:"); ok {
		return name != "" && printable(name)
	}
	if number, ok := strings.CutPrefix(key, "spell:"); ok {
		n, err := strconv.Atoi(number)
		return err == nil && n >= 1 && n <= 54 && strconv.Itoa(n) == number
	}
	return false
}

func parseManualCatalog(data []byte) (map[string]string, error) {
	rows, err := ParseCatalogTSV(data)
	if err != nil {
		return nil, err
	}
	for key, value := range rows {
		if !validManualKey(key) {
			return nil, fmt.Errorf("手冊表含不支援的鍵")
		}
		if value == "" {
			return nil, fmt.Errorf("手冊表含空譯文")
		}
		for _, c := range value {
			if c < 0x20 || c == 0x7F || c == CenterMark {
				return nil, fmt.Errorf("手冊表含控制字元或置中標記")
			}
		}
	}
	return rows, nil
}

func (c *Catalog) Manual(key string) (string, bool) {
	v, ok := c.manual[key]
	return v, ok
}

func (r *Resolver) manualHint(rec *EventRecord) (Result, bool) {
	key := manualKey(rec)
	cat, ok := r.Cat.(interface{ Manual(string) (string, bool) })
	if key == "" || !ok || r.Wide == nil || r.Font == nil || r.Font.W != 16 || r.Font.H != 16 {
		return Result{}, false
	}
	text, ok := cat.Manual(key)
	if !ok || text == "" {
		return Result{}, false
	}
	width := 0
	for _, c := range text {
		if c < 0x20 || c == 0x7F || c == CenterMark || len(r.Font.Glyphs[c]) != 32 {
			return Result{}, false
		}
		width += hw(c, r.Wide)
	}
	if width > availH(rec.Col, len(rec.Text)) {
		return Result{}, false
	}
	return Result{Zh: []rune(text), Key: key, Hits: []string{key}, OK: true}, true
}
