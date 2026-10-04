package phantasie

import (
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// 指標種類分類表（docs/spec/003 §5.1）。區間證據在 docs/re/009；表的內容在 regions.tsv（內嵌）。

//go:embed regions.tsv
var regionsTSV []byte

const regionsHeader = "kind\tstart\tstride\tcount\tnote"

// regionRow 是表中的一列。stride 為 0 時是區間 [start, start+count)；
// 大於 0 時是 count 筆記錄，第 i 筆起點是 start+stride*i。
type regionRow struct {
	kind    Kind
	overlay string // 空字串表示常駐；否則是 overlay 名稱必須含有的小寫字樣（如 "ov1"）
	start   int
	stride  int
	count   int
}

func (r regionRow) contains(p int) bool {
	if r.stride == 0 {
		return p >= r.start && p < r.start+r.count
	}
	d := p - r.start
	return d >= 0 && d%r.stride == 0 && d/r.stride < r.count
}

// RegionTable 是已解析的分類表。建立後唯讀，可並行使用。
type RegionTable struct {
	rows []regionRow // 依判定優先序排好
	tags []string    // 表內出現過的 overlay 字樣
}

// DefaultRegions 載入內嵌的 regions.tsv。
func DefaultRegions() (*RegionTable, error) {
	return ParseRegions(regionsTSV)
}

// regRank 是同一指標落在多個列時的優先序（小者先）。固定在程式內，與檔內列序無關。
func regRank(k Kind) int {
	switch k {
	case KindBuffer:
		return 0
	case KindStatic:
		return 1
	case KindMonster:
		return 2
	case KindTown:
		return 3
	}
	return 4
}

func regKindByName(name string) (Kind, bool) {
	switch name {
	case "static":
		return KindStatic, true
	case "buffer":
		return KindBuffer, true
	case "monster":
		return KindMonster, true
	case "town":
		return KindTown, true
	}
	return KindOther, false
}

func regParseHex(s string, limit int) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("數字為空")
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return 0, fmt.Errorf("%q 不是十六進位數字（不帶 0x）", s)
		}
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil || v > uint64(limit) {
		return 0, fmt.Errorf("%q 超出上限 %X", s, limit)
	}
	return int(v), nil
}

// ParseRegions 解析分類表：第一個非註解、非空白的列是欄名 kind start stride count note，其後每列五欄。
// 錯誤（欄名不符、欄數、未知 kind、壞的 overlay 字樣、非十六進位、count 為 0、超出 16 位元位址空間、
// 沒有任何資料列）回 *TSVError 並指出行號。
func ParseRegions(data []byte) (*RegionTable, error) {
	var rows []regionRow
	seenHeader := false
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		n := i + 1
		if strings.Contains(line, "\r") {
			return nil, &TSVError{n, "含 CR（要 LF）"}
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !seenHeader {
			if line != regionsHeader {
				return nil, &TSVError{n, "欄名要是 kind、start、stride、count、note"}
			}
			seenHeader = true
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 5 {
			return nil, &TSVError{n, fmt.Sprintf("欄數是 %d，要 5", len(cols))}
		}
		name, tag, hasTag := strings.Cut(cols[0], "@")
		kind, ok := regKindByName(name)
		if !ok {
			return nil, &TSVError{n, fmt.Sprintf("未知的 kind %q", name)}
		}
		if hasTag && !regValidTag(tag) {
			return nil, &TSVError{n, fmt.Sprintf("overlay 字樣 %q 要是小寫英數字", tag)}
		}
		start, err := regParseHex(cols[1], 0xFFFF)
		if err != nil {
			return nil, &TSVError{n, "start：" + err.Error()}
		}
		stride, err := regParseHex(cols[2], 0xFFFF)
		if err != nil {
			return nil, &TSVError{n, "stride：" + err.Error()}
		}
		count, err := regParseHex(cols[3], 0x10000)
		if err != nil {
			return nil, &TSVError{n, "count：" + err.Error()}
		}
		if count == 0 {
			return nil, &TSVError{n, "count 不可為 0"}
		}
		if stride == 0 && start+count > 0x10000 {
			return nil, &TSVError{n, "區間超出 16 位元位址空間"}
		}
		if stride > 0 && start+stride*(count-1) > 0xFFFF {
			return nil, &TSVError{n, "記錄陣列超出 16 位元位址空間"}
		}
		rows = append(rows, regionRow{kind: kind, overlay: tag, start: start, stride: stride, count: count})
	}
	if !seenHeader {
		return nil, &TSVError{len(lines), "缺少欄名列"}
	}
	if len(rows) == 0 {
		return nil, &TSVError{len(lines), "沒有任何資料列"}
	}
	sort.SliceStable(rows, func(i, j int) bool { return regRank(rows[i].kind) < regRank(rows[j].kind) })
	t := &RegionTable{rows: rows}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.overlay != "" && !seen[r.overlay] {
			seen[r.overlay] = true
			t.tags = append(t.tags, r.overlay)
		}
	}
	return t, nil
}

func regValidTag(tag string) bool {
	if tag == "" {
		return false
	}
	for _, c := range tag {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// activeOverlay 由 overlay 名稱決定目前生效的字樣：名稱（不分大小寫）恰含表內一個字樣才生效，
// 空字串、不認得、同時含多個都回空字串（預設拒絕：不判 overlay 資料區）。
func (t *RegionTable) activeOverlay(overlay string) string {
	if overlay == "" {
		return ""
	}
	lower := strings.ToLower(overlay)
	found := ""
	for _, tag := range t.tags {
		if strings.Contains(lower, tag) {
			if found != "" {
				return ""
			}
			found = tag
		}
	}
	return found
}

// Classify 依 003 §5.1 分類指標。ptr 是 DS 偏移，sp 是事件 A 時的 SP，overlay 是 L 記錄的目前載入 overlay 名稱
// （如 "ov1.test"，不分大小寫）。預設拒絕：不在表內回 KindOther。
//
// 優先序：堆疊（ptr >= sp，SS=DS，堆疊在高位址）→ buffer → static（含目前 overlay 的資料區）→ monster → town。
// sp 為 0 視為沒有堆疊資訊（未擷取），不套用堆疊規則，否則 ptr >= 0 會把所有指標都判成 buffer。
func (t *RegionTable) Classify(ptr, sp uint16, overlay string) Kind {
	if sp != 0 && ptr >= sp {
		return KindBuffer
	}
	tag := t.activeOverlay(overlay)
	p := int(ptr)
	for _, r := range t.rows {
		if r.overlay != "" && r.overlay != tag {
			continue
		}
		if r.contains(p) {
			return r.kind
		}
	}
	return KindOther
}
