package phantasie

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/xlate"
)

// LoadLanguage 載入一個語言通道（docs/spec/004 §4）：catalog（ui、prose 與共用的保護清單）、字型與寬度表。
// 任一項失敗只停用該語言（Enabled=false，Err 記原因），其他語言不受影響。字型檔名為 <fontDir>/<name>.golemfnt，
// 字型名稱為 full16-<name>（001 §7 要求非空）。
func LoadLanguage(name, textDir, fontDir string) *Language {
	l := &Language{Name: name}
	cat, err := LoadCatalog(
		filepath.Join(textDir, "ui."+name+".tsv"),
		filepath.Join(textDir, "prose."+name+".tsv"),
		filepath.Join(textDir, "protected.tsv"))
	if err != nil {
		l.Err = fmt.Sprintf("catalog：%v", err)
		return l
	}
	manual, err := os.ReadFile(filepath.Join(textDir, "manual."+name+".tsv"))
	if err == nil {
		cat.manual, err = parseManualCatalog(manual)
	}
	if err != nil && !os.IsNotExist(err) {
		l.ManualErr = fmt.Sprintf("手冊提示停用：%v", err)
	}
	data, err := os.ReadFile(filepath.Join(fontDir, name+".golemfnt"))
	if err != nil {
		l.Err = fmt.Sprintf("字型：%v", err)
		return l
	}
	font, err := xlate.ParseFont(data)
	if err != nil {
		l.Err = fmt.Sprintf("字型：%v", err)
		return l
	}
	font.Name = "full16-" + name
	wt, err := ParseFontWide(data)
	if err != nil {
		l.Err = fmt.Sprintf("字型寬度表：%v", err)
		return l
	}
	l.Cat, l.Font, l.Wide, l.Enabled = cat, font, wt.Func(), true
	return l
}
