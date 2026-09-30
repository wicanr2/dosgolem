package buckrogers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Spec 039 §3.4「欄名列」(Buck repo): a table header laid out by the
// original with spaces, whose data columns below are printed by the
// original, keeps each Chinese column name at the original column.  Only
// the rows of text/header-columns.tsv are anchored; the file holds keys and
// cell offsets, never the English.

// HeaderColumnsFile is the formal file name in the Buck Rogers text directory.
const HeaderColumnsFile = "header-columns.tsv"

var headerColumnsHeader = []string{"family", "key", "cols", "evidence"}

// Header families: "menu" keys are menu-family event keys, "dispatcher"
// keys are engine fragment keys without the ".uc" suffix.
const (
	headerFamilyMenu       = "menu"
	headerFamilyDispatcher = "dispatcher"
)

// HeaderColumns is the parsed white list.  cols are the cells, relative to
// the original text's first cell, where each column name starts.
type HeaderColumns struct {
	menu       map[string][]int
	dispatcher map[string][]int
}

// LoadHeaderColumns parses text/header-columns.tsv.  It checks the file's
// own structure; the translation checks need the catalogs (ValidateMenu,
// ValidateDispatcher).
func LoadHeaderColumns(name string, data []byte) (*HeaderColumns, error) {
	rows, err := readTSV(name, data, headerColumnsHeader)
	if err != nil {
		return nil, err
	}
	h := &HeaderColumns{menu: map[string][]int{}, dispatcher: map[string][]int{}}
	for _, r := range rows {
		family, key := r[0], r[1]
		var dst map[string][]int
		switch family {
		case headerFamilyMenu:
			dst = h.menu
		case headerFamilyDispatcher:
			if strings.HasSuffix(key, ".uc") {
				return nil, fmt.Errorf("buckrogers: %s: %s 須寫去掉 .uc 的片段鍵", name, key)
			}
			dst = h.dispatcher
		default:
			return nil, fmt.Errorf("buckrogers: %s: %s 的 family %q 無效", name, key, family)
		}
		if _, dup := dst[key]; dup {
			return nil, fmt.Errorf("buckrogers: %s: 重複鍵 %s", name, key)
		}
		cols, err := parseHeaderCols(r[2])
		if err != nil {
			return nil, fmt.Errorf("buckrogers: %s: %s：%w", name, key, err)
		}
		dst[key] = cols
	}
	return h, nil
}

// parseHeaderCols reads "2,8,14": non-negative, strictly increasing.
func parseHeaderCols(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	cols := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 39 {
			return nil, fmt.Errorf("cols %q 無效", s)
		}
		if i > 0 && n <= cols[i-1] {
			return nil, fmt.Errorf("cols %q 必須嚴格遞增", s)
		}
		cols[i] = n
	}
	return cols, nil
}

// anchorColumns lays zh out as header columns (spec 039 §3.4 欄名列): zh
// is cut at U+0020 into one token per column; token i starts at half unit
// 2·cols[i] after §3.1 padding, and the row is padded to total units.  The
// separating spaces are not drawn.  A token count other than len(cols), an
// empty token (leading, trailing or repeated spaces), a column that leaves
// less than one unit before the next, or a last column past total fail.
func anchorColumns(zh []rune, cols []int, total int) ([]rune, error) {
	tokens := strings.Split(string(zh), " ")
	if len(cols) == 0 || len(tokens) != len(cols) {
		return nil, fmt.Errorf("欄名 %d 個，cols %d 項", len(tokens), len(cols))
	}
	var out []rune
	u := 0
	for i, tok := range tokens {
		if tok == "" {
			return nil, fmt.Errorf("欄名有空 token（連續、前導或結尾空白）")
		}
		if i > 0 && cols[i] <= cols[i-1] {
			return nil, fmt.Errorf("cols 必須嚴格遞增")
		}
		start, tu := 2*cols[i], stringUnits(tok)
		if i+1 < len(cols) {
			if tu+1 > 2*(cols[i+1]-cols[i]) {
				return nil, fmt.Errorf("第 %d 欄 %d 單位，與下一欄之間不足 1 單位", i, tu)
			}
		} else if start+tu > total {
			return nil, fmt.Errorf("末欄止於 %d 單位，超過 %d", start+tu, total)
		}
		out = appendPadding(out, u, start)
		out = append(out, []rune(tok)...)
		u = start + tu
	}
	return appendPadding(out, u, total), nil
}

// tokenStarts lists the cells where each space-separated word of original
// starts, relative to its first cell (spec 039 §3.4 欄名列 rule 4).
func tokenStarts(original []byte) []int {
	var out []int
	for i, c := range original {
		if c != ' ' && (i == 0 || original[i-1] == ' ') {
			out = append(out, i)
		}
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedHeaderKeys(m map[string][]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// MenuColumns returns the anchor cells of a menu event key (nil: not listed).
func (h *HeaderColumns) MenuColumns(eventKey string) []int {
	if h == nil {
		return nil
	}
	return h.menu[eventKey]
}

// HasDispatcher reports whether the list has dispatcher rows.
func (h *HeaderColumns) HasDispatcher() bool { return h != nil && len(h.dispatcher) != 0 }

// ValidateMenu applies the load check to the menu rows: every key is an
// event of the merged menu catalog (so it has a translation), and every
// identity of that event lays out within its original length × 2 units.
func (h *HeaderColumns) ValidateMenu(c *MenuCatalog) error {
	if h == nil {
		return nil
	}
	if c == nil {
		return fmt.Errorf("buckrogers: %s 需要選單 catalog", HeaderColumnsFile)
	}
	for _, key := range sortedHeaderKeys(h.menu) {
		found := false
		for id, e := range c.byIdentity {
			if e.eventKey != key {
				continue
			}
			found = true
			if _, err := anchorColumns([]rune(e.translation), h.menu[key], 2*int(id.length)); err != nil {
				return fmt.Errorf("buckrogers: %s: %s：%w", HeaderColumnsFile, key, err)
			}
		}
		if !found {
			return fmt.Errorf("buckrogers: %s: %s 不在選單 events／catalog", HeaderColumnsFile, key)
		}
	}
	return nil
}

// ValidateDispatcher applies the load check to the dispatcher rows: the key
// (and any ".uc" variant) is an engine fragment event with a translation,
// laid out within the original length × 2 units.
func (h *HeaderColumns) ValidateDispatcher(c *EngineTextCatalog) error {
	if !h.HasDispatcher() {
		return nil
	}
	if c == nil {
		return fmt.Errorf("buckrogers: %s 有 dispatcher 列，但沒有引擎片段 catalog", HeaderColumnsFile)
	}
	for _, key := range sortedHeaderKeys(h.dispatcher) {
		zh := c.fragText[key]
		if zh == "" {
			return fmt.Errorf("buckrogers: %s: %s 不在引擎片段譯文", HeaderColumnsFile, key)
		}
		found := false
		for id, k := range c.frag {
			if k != key && k != key+".uc" {
				continue
			}
			found = found || k == key
			if _, err := anchorColumns([]rune(zh), h.dispatcher[key], 2*id.n); err != nil {
				return fmt.Errorf("buckrogers: %s: %s：%w", HeaderColumnsFile, k, err)
			}
		}
		if !found {
			return fmt.Errorf("buckrogers: %s: %s 不在引擎片段 events", HeaderColumnsFile, key)
		}
	}
	return nil
}

// dispatcherColumns returns the anchor cells for a dispatcher original when
// its whole text is a listed fragment (".uc" folded).
func (h *HeaderColumns) dispatcherColumns(c *EngineTextCatalog, original string) ([]int, bool) {
	if !h.HasDispatcher() || c == nil {
		return nil, false
	}
	k, ok := c.frag[engineIDOf(original)]
	if !ok {
		return nil, false
	}
	cols, ok := h.dispatcher[strings.TrimSuffix(k, ".uc")]
	return cols, ok
}
