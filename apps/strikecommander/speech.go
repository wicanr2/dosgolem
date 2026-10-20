package strikecommander

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// 語音字幕的結構表 text/speech.index.tsv（strike_commander 規格 010 §3.5）：UTF-8、LF、第一列為欄名，
// 每列 9 欄：key、seg_len、end（十六進位）、op、speaker、speaker_en、voc、samples（可空）、symbols（可空，逗號分隔）。

var speechIndexHeader = []string{"key", "seg_len", "end", "op", "speaker", "speaker_en", "voc", "samples", "symbols"}

// SpeechLine 是結構表的一列。
type SpeechLine struct {
	Key        string
	Seg, Idx   int
	SegLen     uint32
	End        uint32
	Op         string
	Speaker    string
	SpeakerEN  string
	Voc        string
	Samples    uint64 // S(k)；HasSamples 為假時語音不在 VOCLIST.TRE
	HasSamples bool
	Symbols    []string
}

// SpeechIndex 是解析後的結構表。
type SpeechIndex struct {
	Lines  []SpeechLine
	byKey  map[string]int
	segLen map[int]uint32
	byEnd  map[int]map[uint32]int
}

// Line 回 key 的列。
func (x *SpeechIndex) Line(key string) (*SpeechLine, bool) {
	i, ok := x.byKey[key]
	if !ok {
		return nil, false
	}
	return &x.Lines[i], true
}

// SegLen 回段號 g 的解壓長度（表內沒有該段時 ok 為假）。
func (x *SpeechIndex) SegLen(g int) (uint32, bool) {
	n, ok := x.segLen[g]
	return n, ok
}

// LineAtEnd 回段 g 內句尾等於 rel 的帶語音句（§3.3 條件 g）。
func (x *SpeechIndex) LineAtEnd(g int, rel uint32) (*SpeechLine, bool) {
	i, ok := x.byEnd[g][rel]
	if !ok {
		return nil, false
	}
	return &x.Lines[i], true
}

// Speakers 回 speaker 欄的相異值。
func (x *SpeechIndex) Speakers() map[string]bool {
	out := map[string]bool{}
	for _, l := range x.Lines {
		out[l.Speaker] = true
	}
	return out
}

// parseSpeechKey 解 conv/<三位>/<兩位>。
func parseSpeechKey(k string) (seg, idx int, ok bool) {
	if len(k) != len("conv/000/00") || !strings.HasPrefix(k, "conv/") || k[8] != '/' {
		return 0, 0, false
	}
	g, e1 := strconv.Atoi(k[5:8])
	i, e2 := strconv.Atoi(k[9:11])
	if e1 != nil || e2 != nil || g < 0 || i < 0 {
		return 0, 0, false
	}
	return g, i, true
}

// splitTSVLines 把 UTF-8、LF 的 TSV 切成列；最後一列必須以 LF 結尾。
func splitTSVLines(b []byte, what string) ([]string, error) {
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("strikecommander: %s 不是合法 UTF-8", what)
	}
	s := string(b)
	if !strings.HasSuffix(s, "\n") {
		return nil, fmt.Errorf("strikecommander: %s 最後一列沒有 LF", what)
	}
	if strings.Contains(s, "\r") {
		return nil, fmt.Errorf("strikecommander: %s 含 CR", what)
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n"), nil
}

// ParseSpeechIndex 解析並驗證結構表：欄名、每列 9 欄、key 格式與唯一、依段號與句序遞增、數值欄可解析。
func ParseSpeechIndex(b []byte) (*SpeechIndex, error) {
	lines, err := splitTSVLines(b, "結構表")
	if err != nil {
		return nil, err
	}
	if lines[0] != strings.Join(speechIndexHeader, "\t") {
		return nil, fmt.Errorf("strikecommander: 結構表第 1 列欄名不符")
	}
	x := &SpeechIndex{byKey: map[string]int{}, segLen: map[int]uint32{}, byEnd: map[int]map[uint32]int{}}
	prevSeg, prevIdx := -1, -1
	for n, ln := range lines[1:] {
		row := n + 2
		c := strings.Split(ln, "\t")
		if len(c) != 9 {
			return nil, fmt.Errorf("strikecommander: 結構表第 %d 列欄數 %d，要 9", row, len(c))
		}
		g, i, ok := parseSpeechKey(c[0])
		if !ok {
			return nil, fmt.Errorf("strikecommander: 結構表第 %d 列 key %q 格式不對", row, c[0])
		}
		if g < prevSeg || (g == prevSeg && i <= prevIdx) {
			return nil, fmt.Errorf("strikecommander: 結構表第 %d 列 key %q 沒有依段號、句序遞增", row, c[0])
		}
		prevSeg, prevIdx = g, i
		segLen, e1 := strconv.ParseUint(c[1], 10, 32)
		end, e2 := strconv.ParseUint(c[2], 16, 32)
		if e1 != nil || e2 != nil || c[2] != strings.ToUpper(c[2]) || strings.HasPrefix(c[2], "0") && c[2] != "0" {
			return nil, fmt.Errorf("strikecommander: 結構表第 %d 列段長或句尾格式不對", row)
		}
		switch c[3] {
		case "02", "03", "04", "0A":
		default:
			return nil, fmt.Errorf("strikecommander: 結構表第 %d 列 op %q 不對", row, c[3])
		}
		l := SpeechLine{Key: c[0], Seg: g, Idx: i, SegLen: uint32(segLen), End: uint32(end), Op: c[3], Speaker: c[4], SpeakerEN: c[5], Voc: c[6]}
		if c[7] != "" {
			s, err := strconv.ParseUint(c[7], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("strikecommander: 結構表第 %d 列 samples 格式不對", row)
			}
			l.Samples, l.HasSamples = s, true
		}
		if c[8] != "" {
			l.Symbols = strings.Split(c[8], ",")
			for _, s := range l.Symbols {
				if len(s) != 2 || s[0] != '$' || !strings.ContainsRune(subSymbols, rune(s[1])) {
					return nil, fmt.Errorf("strikecommander: 結構表第 %d 列 symbols %q 不對", row, c[8])
				}
			}
		}
		if l.Speaker == "" || l.SpeakerEN == "" || l.Voc == "" {
			return nil, fmt.Errorf("strikecommander: 結構表第 %d 列說話人或語音是空的", row)
		}
		if old, ok := x.segLen[g]; ok && old != l.SegLen {
			return nil, fmt.Errorf("strikecommander: 結構表第 %d 列段 %d 的段長 %d 與前列 %d 不同", row, g, l.SegLen, old)
		}
		x.segLen[g] = l.SegLen
		if x.byEnd[g] == nil {
			x.byEnd[g] = map[uint32]int{}
		}
		if _, dup := x.byEnd[g][l.End]; dup {
			return nil, fmt.Errorf("strikecommander: 結構表第 %d 列段 %d 句尾 %X 重複", row, g, l.End)
		}
		x.byEnd[g][l.End] = len(x.Lines)
		x.byKey[l.Key] = len(x.Lines)
		x.Lines = append(x.Lines, l)
	}
	return x, nil
}
