// Package phantasie 是一款 DOS 角色扮演遊戲（原版英文名 Phantasie）的遊戲專屬層：
// 啟動鏈、繪字呼叫的辨識位址與覆繪所需的觀測。
// 原版執行檔與資料由使用者提供，套件本身不含任何遊戲內容。
//
// 位址是映像偏移（CS = 映像段 = 最後一支 EXE 的 PSP + 10h），證據在使用者的私有研究紀錄。
package phantasie

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

// ParseBatch 取出批次檔依序要執行的程式名。
// 略過空行、`echo`、`@echo`、`rem`、`::` 註解與標籤；其餘每行的第一個詞就是程式名。
func ParseBatch(text string) []string {
	var names []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		f := strings.Fields(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "@")))
		if len(f) == 0 {
			continue
		}
		switch w := strings.ToLower(f[0]); {
		case w == "echo", w == "rem", strings.HasPrefix(w, "::"), strings.HasPrefix(w, ":"):
			continue
		default:
			names = append(names, f[0])
		}
	}
	return names
}

// FindBatch 在 root 找唯一一個 .BAT 檔（不分大小寫）。找不到或不只一個都回錯，不猜。
func FindBatch(root string) (string, error) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var found []string
	for _, e := range ents {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".bat") {
			found = append(found, e.Name())
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("%s 內的 .BAT 檔有 %d 個，要剛好 1 個（可用旗標指定）", root, len(found))
	}
	return found[0], nil
}

// resolve 以不分大小寫解析 root 內的檔名。
func resolve(root, name string) (string, error) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, e := range ents {
		if !e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(root, e.Name()), nil
		}
	}
	// 批次檔裡的程式名可以省略副檔名：依 DOS 的順序試 .COM、.EXE。
	if filepath.Ext(name) == "" {
		for _, ext := range []string{".com", ".exe"} {
			if p, err := resolve(root, name+ext); err == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("%s 內找不到 %s", root, name)
}

// Launch 照批次檔的順序啟動：第一支載入，其餘排進監督佇列（常駐程式結束後依序接續）。
// 回傳的 Oracle 尚未執行任何指令。
func Launch(root, bat string) (*oracle.Oracle, []string, error) {
	if bat == "" {
		b, err := FindBatch(root)
		if err != nil {
			return nil, nil, err
		}
		bat = b
	}
	raw, err := os.ReadFile(filepath.Join(root, bat))
	if err != nil {
		return nil, nil, err
	}
	names := ParseBatch(string(raw))
	if len(names) == 0 {
		return nil, nil, fmt.Errorf("%s 內沒有任何程式", bat)
	}
	first, err := resolve(root, names[0])
	if err != nil {
		return nil, nil, err
	}
	o, err := oracle.LoadProgram(first, root, "")
	if err != nil {
		return nil, nil, err
	}
	for _, n := range names[1:] {
		p, err := resolve(root, n)
		if err != nil {
			o.Close()
			return nil, nil, err
		}
		o.Enqueue(filepath.Base(p), "")
	}
	return o, names, nil
}
