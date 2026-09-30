package buckrogers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// Spec 040 §3.1 (Buck repo): language codes and file names.  Every family
// keeps its language-independent files (events, safe rectangles, layout,
// callers, header-columns structure, the original side of name data) as
// shared files; each language has text/<family>.<lang>.tsv whose keys are a
// subset of the shared events.

const (
	// LangZhTW is the reference language: its files must match the events
	// exactly and a failure keeps the runtime from starting.
	LangZhTW = "zh-TW"
	// LangEn is the original English: no overlay at all (spec 040 §3.4).
	LangEn = "en"
	// LangZhCN is Simplified Chinese (Buck repo spec 041): its files are
	// generated from zh-TW by OpenCC and its player names are the zh-TW
	// transliteration mapped character by character.
	LangZhCN = "zh-CN"
	// LangJa is Japanese (Buck repo spec 042): machine-assisted translation
	// from the English original; its layout profile adds kinsoku.
	LangJa = "ja"
	// LangKo is Korean (Buck repo spec 043): machine-assisted translation
	// from the English original; its layout profile wraps at word level.
	LangKo = "ko"
	// LangTest is the test-only fake language of spec 040 §5.3.  It never
	// appears in the F4 cycle or a release.
	LangTest = "zz"
)

// LangCycle is the F4 order of spec 040 §1 (zz is not in it).
var LangCycle = []string{"zh-TW", "zh-CN", "en", "ja", "ko"}

// KnownLang reports whether code is a language the runtime understands
// (the F4 languages plus the test language).
func KnownLang(code string) bool {
	if code == LangTest {
		return true
	}
	for _, c := range LangCycle {
		if c == code {
			return true
		}
	}
	return false
}

// LangFile is the file name of one family's text for one language.
func LangFile(family, lang string) string { return family + "." + lang + ".tsv" }

// LangFontPath is the spec 040 §3.4 font of a language under a project root.
func LangFontPath(root, lang string) string {
	return filepath.Join(root, "font", "buckrogers-"+lang+".golemfnt")
}

var langTextHeader = []string{"key", "translation", "source"}

// readTSVAllowEmpty is readTSV that accepts a header-only file (spec 040:
// a language may leave a family untranslated with zero rows).
func readTSVAllowEmpty(name string, data []byte, header []string) ([][]string, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s: 不是有效 UTF-8", name)
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return nil, fmt.Errorf("%s: 不接受 BOM", name)
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = '\t'
	r.FieldsPerRecord = len(header)
	gotHeader, err := r.Read()
	if err != nil || !equalStrings(gotHeader, header) {
		return nil, fmt.Errorf("%s: 標頭不符", name)
	}
	var rows [][]string
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, value := range row {
			if value == "" {
				return nil, fmt.Errorf("%s: 欄位不得為空", name)
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// langTexts reads a non-reference language file: unique, non-empty keys
// that all belong to allowed (nil: any key).  Missing keys are allowed.
func langTexts(name string, data []byte, allowed map[string]bool) (map[string]string, error) {
	out := map[string]string{}
	if data == nil {
		return out, nil
	}
	rows, err := readTSVAllowEmpty(name, data, langTextHeader)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if _, dup := out[r[0]]; dup {
			return nil, fmt.Errorf("%s: 重複文字鍵 %q", name, r[0])
		}
		if allowed != nil && !allowed[r[0]] {
			return nil, fmt.Errorf("%s: 孤兒文字鍵 %q", name, r[0])
		}
		out[r[0]] = r[1]
	}
	return out, nil
}

// readLangFile reads a language file from dir; a missing file is (nil, nil).
func readLangFile(dir, family, lang string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(dir, LangFile(family, lang)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return b, err
}

// headerOnly is an empty language file (used where a loader needs bytes).
func headerOnly() []byte { return []byte("key\ttranslation\tsource\n") }

// firstColumnKeys lists the first column of a TSV's data rows (the event
// keys of an events file), whatever its other columns.
func firstColumnKeys(name string, data []byte) (map[string]bool, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("%s: 無法讀取事件鍵", name)
	}
	out := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) == 0 || row[0] == "" {
			return nil, fmt.Errorf("%s: 事件鍵不得為空", name)
		}
		out[row[0]] = true
	}
	return out, nil
}

// readLangTSV reads a language file: zh-TW keeps the established checks
// (at least one row); another language may have a header-only file.
func readLangTSV(name string, data []byte, header []string, lang string) ([][]string, error) {
	if lang == LangZhTW || lang == "" {
		return readTSV(name, data, header)
	}
	return readTSVAllowEmpty(name, data, header)
}
