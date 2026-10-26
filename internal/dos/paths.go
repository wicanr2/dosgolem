package dos

import (
	"os"
	"path/filepath"
	"strings"
)

// 相對子目錄解析（`docs/spec/198-dos-relative-directories`）。
//
// 背景：`resolve` 原本**只認 basename**——遊戲組出 `A:\<垃圾>\DATA.PAK` 這種
// 多磁片殘留路徑，目錄部分對不上硬碟安裝版。那條規則照舊，但它與「資料夾裡
// 真的有子目錄」衝突：AESOP 的 `SAVEGAME\ITEMS_00.BIN`、`CHARGEN\CHGEN.EXE`
// 都是真實子目錄，只看 basename 會找不到，而程式把「找不到」當成要從 `.TXT`
// 翻譯，翻譯又失敗，開機就以離開碼 1 結束。
//
// 規則：路徑帶目錄元件（含目前目錄 `Dir` 的前綴）、**而且那串目錄在 Root 或
// Scratch 裡真的存在**，才走這一條；否則維持 basename 規則。junk 路徑因此
// 不受影響。

// RootDir 與 Dir 的關係：`Dir` 是 DOS 的目前目錄（`AH=47h` 回的那個，
// 從磁碟機根起算、不含磁碟機與開頭的 `\`）；`RootDir` 是 `Root` 這個主機
// 目錄在 DOS 樹上的位置（`C:\EOB3` → `EOB3`）。**兩者預設相同**（舊行為：
// 程式在自己的目錄裡跑，相對路徑就是 Root 底下），`chdir` 之後 `Dir` 離開
// `RootDir`，相對路徑才跟著換目錄。

// relComponents 把 DOS 路徑換成「從 Root 起算的目錄元件」與檔名。
//
// 去掉磁碟機字母；`\` 開頭是絕對路徑，否則接在目前目錄 `Dir` 後面；
// `.` 丟掉、`..` 往上退一層（退過根目錄就停在根目錄，真 DOS 同）。
// 目錄元件一律大寫。再把 `RootDir` 的前綴去掉；不在 `RootDir` 底下的路徑
// （junk 路徑、別的目錄）原樣保留，之後多半對不上真實目錄而退回 basename 規則。
func (d *DOS) relComponents(name string) (dirs []string, file string) {
	s := strings.ReplaceAll(name, "/", `\`)
	if len(s) >= 2 && s[1] == ':' {
		s = s[2:]
	}
	segs := strings.Split(s, `\`)
	file = segs[len(segs)-1]
	full := d.applySegments(strings.HasPrefix(s, `\`), segs[:len(segs)-1])
	return d.underRoot(full), file
}

// underRoot 去掉 RootDir 前綴，得到「從 Root 起算」的元件。
func (d *DOS) underRoot(full []string) []string {
	root := splitPath(strings.ToUpper(d.RootDir))
	if len(full) < len(root) {
		return full
	}
	for i, r := range root {
		if full[i] != r {
			return full
		}
	}
	return full[len(root):]
}

// joinDir 把「目錄路徑」（chdir）換成**從磁碟機根起算**的目錄元件。
func (d *DOS) joinDir(path string) []string {
	s := strings.ReplaceAll(path, "/", `\`)
	if len(s) >= 2 && s[1] == ':' {
		s = s[2:]
	}
	return d.applySegments(strings.HasPrefix(s, `\`), strings.Split(s, `\`))
}

func (d *DOS) applySegments(abs bool, segs []string) []string {
	var parts []string
	if !abs {
		parts = splitPath(strings.ToUpper(d.Dir))
	}
	for _, seg := range segs {
		switch seg {
		case "", ".":
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, strings.ToUpper(seg))
		}
	}
	return parts
}

// walkDirs 在 layer 底下逐層找子目錄（大小寫不分，找不到再用 8.3 截斷過的名字比）。
// 找到回實際路徑，任何一層不存在回 ""。dirs 為空回 layer 本身。
func walkDirs(layer string, dirs []string) string {
	cur := layer
	for _, want := range dirs {
		entries, err := os.ReadDir(cur)
		if err != nil {
			return ""
		}
		next := ""
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if strings.EqualFold(e.Name(), want) ||
				strings.EqualFold(dosName(e.Name()), dosName(want)) {
				next = filepath.Join(cur, e.Name())
				break
			}
		}
		if next == "" {
			return ""
		}
		cur = next
	}
	return cur
}

// dirsExist 回報這串目錄在 Root 或 Scratch 裡是否真的存在。dirs 為空時恆為假——
// 沒有目錄元件的路徑走原本的 basename 規則。
func (d *DOS) dirsExist(dirs []string) bool {
	if len(dirs) == 0 {
		return false
	}
	return (d.Root != "" && walkDirs(d.Root, dirs) != "") ||
		(d.Scratch != "" && walkDirs(d.Scratch, dirs) != "")
}

// resolveDir 是帶真實子目錄的路徑解析：暫存層蓋過原版目錄。
// 目錄不存在或檔案不在其中回 ""，由呼叫端退回 basename 規則。
func (d *DOS) resolveDir(name string) string {
	dirs, file := d.relComponents(name)
	if len(dirs) == 0 || file == "" {
		return ""
	}
	for _, layer := range []string{d.Scratch, d.Root} {
		if layer == "" {
			continue
		}
		dir := walkDirs(layer, dirs)
		if dir == "" {
			continue
		}
		if p := lookupDOS(dir, file); p != "" {
			return p
		}
	}
	return ""
}

// scratchPath 回這個名字在暫存層的寫入位置，並確保它的目錄存在。
//
// 路徑帶的目錄在 Root／Scratch 真的存在時，寫進暫存層的同名子目錄
// （`SAVEGAME\X.BIN` → `scratch/SAVEGAME/X.BIN`）；否則維持舊規則，
// 攤平成 `scratch/<basename>`。讀（`resolveDir`）與寫（這裡）用同一套判準，
// 所以寫進去的檔下一次一定讀得回來。
func (d *DOS) scratchPath(name string) (string, error) {
	if d.protectedRoots {
		return d.protectedTarget(name, false, true)
	}
	dirs, file := d.relComponents(name)
	dir := d.Scratch
	if d.dirsExist(dirs) {
		dir = filepath.Join(append([]string{d.Scratch}, dirs...)...)
	} else {
		file = baseName(name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, file), nil
}

// dirExists 回報 path 是不是一個存在的目錄（根目錄 `\` 恆為真）。
func (d *DOS) dirExists(path string) bool {
	rel := d.underRoot(d.joinDir(path))
	if len(rel) == 0 {
		return true // 磁碟機根目錄，或 RootDir 本身（Root 當然存在）
	}
	return d.dirsExist(rel)
}

// scratchDir 回「目錄名字在暫存層的位置」（mkdir／rmdir 用）：父目錄真的存在時
// 用同名的巢狀位置，否則照舊攤平成大寫 basename。
func (d *DOS) scratchDir(name string) string {
	rel := d.underRoot(d.joinDir(name))
	if len(rel) > 1 && d.dirsExist(rel[:len(rel)-1]) {
		return filepath.Join(append([]string{d.Scratch}, rel...)...)
	}
	return filepath.Join(d.Scratch, strings.ToUpper(baseName(name)))
}
