package main

// 語言選擇、設定檔與說明頁的語言列（Buck repo 規格 040 §3.4）。
// 設定檔只存 {"lang": "<代碼>"}；讀寫失敗不致命。自動模式（-frames>0）
// 與開發模式（-save）不讀也不寫。

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/xlate"
)

const settingsName = "settings.json"

// frontendKnownLang 是前端認得的語言：F4 循環的語言。測試用的 zz 只在收據工具。
func frontendKnownLang(code string) bool {
	for _, c := range buckrogers.LangCycle {
		if c == code {
			return true
		}
	}
	return false
}

type settingsFile struct {
	Lang string `json:"lang"`
}

// readSettingsLang 回設定檔的語言；檔案不存在回 ""。
func readSettingsLang(dir string) (string, error) {
	path := filepath.Join(dir, settingsName)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var s settingsFile
	if err := json.Unmarshal(b, &s); err != nil {
		return "", fmt.Errorf("%s：%w", path, err)
	}
	return s.Lang, nil
}

// writeSettingsLang 先寫暫存檔再改名，寫到一半失敗不會留下半個設定檔。
func writeSettingsLang(dir, lang string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(settingsFile{Lang: lang})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "settings-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(append(b, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(dir, settingsName))
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// startLang 依 -lang ＞ 設定檔 ＞ zh-TW 決定起始語言。flagLang 須已通過
// frontendKnownLang（不認得是用法錯誤，由呼叫端處理）。已知但未啟用、或設定檔
// 給出不認得的代碼時退為 zh-TW，並回一則錯誤說明。
func startLang(flagLang, saved string, enabled func(string) bool) (string, error) {
	cand, from := flagLang, "-lang"
	if cand == "" {
		cand, from = saved, settingsName
	}
	switch {
	case cand == "":
		return buckrogers.LangZhTW, nil
	case !frontendKnownLang(cand):
		return buckrogers.LangZhTW, fmt.Errorf("%s 的語言代碼 %q 不認得，改用 %s", from, cand, buckrogers.LangZhTW)
	case !enabled(cand):
		return buckrogers.LangZhTW, fmt.Errorf("%s 指定的語言 %s 未啟用，改用 %s", from, cand, buckrogers.LangZhTW)
	}
	return cand, nil
}

// langSettings 是互動模式的設定檔位置；dir 為空表示不讀不寫。
type langSettings struct{ dir string }

func newLangSettings(frames int, save, data string) langSettings {
	if frames > 0 || save != "" {
		return langSettings{}
	}
	return langSettings{dir: data}
}

func (s langSettings) read() (string, error) {
	if s.dir == "" {
		return "", nil
	}
	return readSettingsLang(s.dir)
}

func (s langSettings) write(lang string) error {
	if s.dir == "" {
		return nil
	}
	return writeSettingsLang(s.dir, lang)
}

// 說明頁的語言列（text/host-ui.zh-TW.tsv，source frontend-help）：
// help.* 裡的 {lang} 換成目前語言名稱，{off} 換成未啟用語言與原因摘要
// （沒有未啟用語言時整列不顯示）。語言名稱與原因摘要是 lang.* 列。
const (
	helpColumns   = 38 // 與 apps/buckrogers 說明頁的每行上限相同
	langOffFiles  = "lang.off.files"
	langOffFont   = "lang.off.font"
	langOffLoad   = "lang.off.load"
	langTagCur    = "{lang}"
	langTagOff    = "{off}"
	langKeyPrefix = "lang."
)

// langUI 是語言列的文字；缺字模的名稱改顯示語言代碼。
type langUI struct {
	text map[string]string
}

// loadLangUI 讀 host-ui.zh-TW.tsv 的 lang.* 列。每個 F4 語言的名稱與三種原因摘要
// 都要有；字型缺字模的項目改用代碼（名稱）或略過（原因），並回報在 warn。
func loadLangUI(textDir string, font *xlate.Font) (langUI, []string, error) {
	name := "host-ui.zh-TW.tsv"
	data, err := os.ReadFile(filepath.Join(textDir, name))
	if err != nil {
		return langUI{}, nil, err
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err == nil && len(rows) == 0 {
		err = errors.New("空檔")
	}
	if err != nil {
		return langUI{}, nil, fmt.Errorf("%s：%w", name, err)
	}
	ui := langUI{text: map[string]string{}}
	var warn []string
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 3 || !strings.HasPrefix(row[0], langKeyPrefix) {
			continue
		}
		if row[2] != "frontend-help" {
			return langUI{}, nil, fmt.Errorf("%s：%s 的 source 須為 frontend-help", name, row[0])
		}
		seen[row[0]] = true
		if ch, ok := missingGlyph(font, row[1]); !ok {
			warn = append(warn, fmt.Sprintf("%s：%s 缺字模 %q，說明頁改顯示代碼", name, row[0], ch))
			continue
		}
		ui.text[row[0]] = row[1]
	}
	need := []string{langOffFiles, langOffFont, langOffLoad}
	for _, c := range buckrogers.LangCycle {
		need = append(need, langKeyPrefix+c)
	}
	for _, k := range need {
		if !seen[k] {
			return langUI{}, nil, fmt.Errorf("%s：缺 %s", name, k)
		}
	}
	return ui, warn, nil
}

func missingGlyph(font *xlate.Font, s string) (rune, bool) {
	if font == nil {
		return 0, true
	}
	for _, ch := range s {
		if _, ok := font.Glyphs[ch]; !ok && ch != ' ' {
			return ch, false
		}
	}
	return 0, true
}

// name 回語言的顯示名稱；沒有（或缺字模）就用代碼。
func (u langUI) name(code string) string {
	if t, ok := u.text[langKeyPrefix+code]; ok {
		return t
	}
	return code
}

// reason 把 LiveRuntime 的停用原因歸成三種摘要（完整原因在 stderr）。
func (u langUI) reason(why string) string {
	key := langOffLoad
	switch {
	case strings.HasPrefix(why, "缺語言檔"), why == "未載入":
		key = langOffFiles
	case why == "缺字型", strings.HasPrefix(why, "字型："):
		key = langOffFont
	}
	return u.text[key]
}

// offSummary 列出未啟用語言，同一原因的語言併成一組：
// 「簡體中文、日文、韓文（缺語言檔）」。
func (u langUI) offSummary(langs []buckrogers.LangStatus) string {
	var order []string
	groups := map[string][]string{}
	for _, l := range langs {
		if l.Enabled {
			continue
		}
		why := u.reason(l.Reason)
		if _, ok := groups[why]; !ok {
			order = append(order, why)
		}
		groups[why] = append(groups[why], u.name(l.Code))
	}
	var parts []string
	for _, why := range order {
		p := strings.Join(groups[why], "、")
		if why != "" {
			p += "（" + why + "）"
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "；")
}

// helpLines 代入說明頁樣板列：{lang} 為目前語言，{off} 為未啟用語言摘要。
func (u langUI) helpLines(lines []string, cur string, langs []buckrogers.LangStatus) []string {
	out := make([]string, 0, len(lines))
	off := u.offSummary(langs)
	for _, l := range lines {
		if strings.Contains(l, langTagOff) {
			if off == "" {
				continue
			}
			l = strings.ReplaceAll(l, langTagOff, off)
		}
		l = strings.ReplaceAll(l, langTagCur, u.name(cur))
		if rs := []rune(l); len(rs) > helpColumns {
			l = string(rs[:helpColumns])
		}
		out = append(out, l)
	}
	return out
}
