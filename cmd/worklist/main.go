// worklist 是未完成項的核實與產表工具。
//
// **權威是 `docs/worklist.json`，不是 markdown。** markdown 那一份由
// `render` 產生——在上面打勾，下一次 render 就把它蓋掉了。
//
//	go run ./cmd/worklist verify   # 每一條問一次「這條還成立嗎」
//	go run ./cmd/worklist render   # 產生 docs/worklist.md
//
// verify 的約定是**跑起來為真＝這一條仍然未完成**。為假就是東西做好了
// 而條目沒跟著改——那正是要抓的東西：清單上留著一句自信的「還沒接」，
// 然後有人照它去重做一遍。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Worklist 是 docs/worklist.json 的形狀。
type Worklist struct {
	Schema string            `json:"schema"`
	Note   string            `json:"note"`
	Layers map[string]string `json:"layers"`
	Items  []Item            `json:"items"`

	// Resolved 是做完的條目。**核實不掃它**——它記的是歷史，不是斷言。
	//
	// 放在這裡而不是刪掉，是為了讓「這一條當初錯在哪、被什麼抓到」
	// 留在同一份檔案裡：下一次有人問「HMA 那件事處理了沒」，
	// 答案與證據在一起，不必去翻 git log。
	Resolved []Resolved `json:"resolved,omitempty"`
}

// Resolved 是一條已經做完的未完成項。
type Resolved struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Date     string `json:"date"`
	How      string `json:"how"`
	CaughtBy string `json:"caught_by,omitempty"`

	// EvidenceLevel 是這一條的裁決有多硬：confirmed／強證據／假說／未知。
	// **不填就是沒有人問過這件事**，而那與「確認過了」在報表上長得一樣。
	EvidenceLevel string `json:"evidence_level,omitempty"`
}

// Item 是一條未完成項。
type Item struct {
	ID         string `json:"id"`
	Layer      string `json:"layer"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	BlockedBy  string `json:"blocked_by,omitempty"`
	Acceptance string `json:"acceptance"`
	Verify     Verify `json:"verify"`
}

// Verify 是「這一條還成不成立」的機器訊號。
//
// kind 的語意：
//
//	present  pattern 找得到 → 仍未完成（綁程式碼裡的自承註解）
//	absent   pattern 找不到 → 仍未完成（綁還沒出現的型別或函式名）
//	json_len 某份 JSON 的欄位長度 ≤ max → 仍未完成（綁進度型的清單）
//	manual   一律回「仍未完成」並標出來（真的沒有機器訊號時的誠實選項）
type Verify struct {
	Kind    string   `json:"kind"`
	Paths   []string `json:"paths,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
	Path    string   `json:"path,omitempty"`
	Field   string   `json:"field,omitempty"`
	Max     int      `json:"max,omitempty"`
	Note    string   `json:"note,omitempty"`
}

const worklistPath = "docs/worklist.json"

// scanExts 是 verify 會看的副檔名。
var scanExts = map[string]bool{".go": true, ".sh": true, ".md": true, ".json": true}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法：worklist verify|render")
		os.Exit(2)
	}
	root, err := repoRoot()
	if err != nil {
		die(err)
	}
	wl, err := load(filepath.Join(root, worklistPath))
	if err != nil {
		die(err)
	}
	switch os.Args[1] {
	case "verify":
		os.Exit(verify(root, wl))
	case "render":
		out := filepath.Join(root, "docs/worklist.md")
		if err := os.WriteFile(out, []byte(render(wl)), 0o644); err != nil {
			die(err)
		}
		fmt.Println("寫出", out)
	default:
		fmt.Fprintln(os.Stderr, "不認得的子命令：", os.Args[1])
		os.Exit(2)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "worklist:", err)
	os.Exit(1)
}

// repoRoot 從工作目錄往上找到帶 go.mod 的那一層。
func repoRoot() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("往上找不到 go.mod")
		}
		d = parent
	}
}

func load(path string) (*Worklist, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wl Worklist
	if err := json.Unmarshal(b, &wl); err != nil {
		return nil, fmt.Errorf("%s：%w", path, err)
	}
	if err := check(&wl); err != nil {
		return nil, err
	}
	return &wl, nil
}

// check 擋掉會安靜失效的條目。
//
// **空的 paths 或空的 pattern 會讓 verify 永遠回同一個答案**，而報表上
// 看起來與正常的一模一樣——那比沒有 verify 更糟，它給人「有在看」的假象。
func check(wl *Worklist) error {
	seen := map[string]bool{}
	for _, it := range wl.Items {
		switch {
		case it.ID == "":
			return fmt.Errorf("有條目沒有 id")
		case seen[it.ID]:
			return fmt.Errorf("id 重複：%s", it.ID)
		case wl.Layers[it.Layer] == "":
			return fmt.Errorf("%s：layer %q 不在 layers 裡", it.ID, it.Layer)
		case it.Acceptance == "":
			return fmt.Errorf("%s：沒有 acceptance，說不出怎樣算做完", it.ID)
		}
		seen[it.ID] = true
		v := it.Verify
		switch v.Kind {
		case "present", "absent":
			if v.Pattern == "" || len(v.Paths) == 0 {
				return fmt.Errorf("%s：%s 要有 pattern 與 paths", it.ID, v.Kind)
			}
			if _, err := regexp.Compile(v.Pattern); err != nil {
				return fmt.Errorf("%s：pattern 編不起來：%w", it.ID, err)
			}
		case "json_len":
			if v.Path == "" || v.Field == "" {
				return fmt.Errorf("%s：json_len 要有 path 與 field", it.ID)
			}
		case "manual":
			if v.Note == "" {
				return fmt.Errorf("%s：manual 要在 note 寫明將來接上時綁什麼", it.ID)
			}
		default:
			return fmt.Errorf("%s：不認得的 verify kind %q", it.ID, v.Kind)
		}
	}
	for _, r := range wl.Resolved {
		switch {
		case r.ID == "":
			return fmt.Errorf("resolved 裡有條目沒有 id")
		case seen[r.ID]:
			return fmt.Errorf("%s 同時在 items 與 resolved 裡——做完了就只留一邊", r.ID)
		case r.Date == "" || r.How == "":
			return fmt.Errorf("%s：resolved 要寫 date 與 how", r.ID)
		}
		seen[r.ID] = true
	}
	return nil
}

// hit 在 paths 底下找 pattern，回第一個中的檔案（相對 root）。
//
// ⚠ **測試檔不算。** 測試本來就會提到還沒接上的東西——為了釘住將來的
// 行為，或為了測那個資料結構本身。把 `_test.go` 算進來，`absent` 會
// 因為測試裡有一行呼叫就判成「已經做了」，真缺口就這樣被蓋掉。
func hit(root string, v Verify) (string, error) {
	re, err := regexp.Compile(v.Pattern)
	if err != nil {
		return "", err
	}
	for _, target := range v.Paths {
		full := filepath.Join(root, target)
		err := filepath.Walk(full, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !scanExts[filepath.Ext(p)] {
				return err
			}
			if strings.HasSuffix(p, "_test.go") || strings.HasPrefix(filepath.Base(p), "test_") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if re.Match(b) {
				rel, _ := filepath.Rel(root, p)
				return foundAt(rel)
			}
			return nil
		})
		var f *found
		if err != nil {
			if ok := asFound(err, &f); ok {
				return f.where, nil
			}
			return "", err
		}
	}
	return "", nil
}

// found 是「找到了」這個訊號，借 filepath.Walk 的 error 通道提早收工。
type found struct{ where string }

func (f *found) Error() string { return "找到 " + f.where }

func foundAt(where string) error { return &found{where} }

func asFound(err error, out **found) bool {
	f, ok := err.(*found)
	if ok {
		*out = f
	}
	return ok
}

// stillOpen 回「這一條仍未完成嗎」與判斷的依據。
func stillOpen(root string, it Item) (bool, string, error) {
	v := it.Verify
	switch v.Kind {
	case "manual":
		return true, "要人判：" + v.Note, nil
	case "json_len":
		b, err := os.ReadFile(filepath.Join(root, v.Path))
		if err != nil {
			return false, "", err
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			return false, "", err
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(m[v.Field], &arr); err != nil {
			return false, "", err
		}
		return len(arr) <= v.Max, fmt.Sprintf("%s 的 %s 有 %d 項（門檻 %d）", v.Path, v.Field, len(arr), v.Max), nil
	}
	where, err := hit(root, v)
	if err != nil {
		return false, "", err
	}
	if v.Kind == "present" {
		if where != "" {
			return true, "自承還在 " + where, nil
		}
		return false, "找不到 /" + v.Pattern + "/ 了", nil
	}
	if where != "" {
		return false, "已經出現在 " + where, nil
	}
	return true, "還沒出現：/" + v.Pattern + "/", nil
}

// verify 核實每一條，回 process 的結束碼。
//
// **有條目可能已完成時回非 0**：那是要人回來看的訊號，不是錯誤。
func verify(root string, wl *Worklist) int {
	stale, manual := 0, 0
	for _, it := range wl.Items {
		open, why, err := stillOpen(root, it)
		if err != nil {
			fmt.Printf("%-34s 核實不了       %v\n", it.ID, err)
			stale++
			continue
		}
		status := "仍未完成"
		if !open {
			status, stale = "** 可能已完成 **", stale+1
		} else if it.Verify.Kind == "manual" {
			status, manual = "仍未完成（人判）", manual+1
		}
		fmt.Printf("%-34s %-18s %s\n", it.ID, status, why)
	}
	fmt.Printf("\n%d 條未完成（%d 條要人判），%d 條已解決。\n",
		len(wl.Items), manual, len(wl.Resolved))
	if stale > 0 {
		fmt.Printf("⚠ %d 條的 verify 說東西可能已經做好了——回去看那幾條，做完的從 JSON 移走。\n", stale)
		return 1
	}
	return 0
}

// render 產生 markdown。開頭寫明不要手改。
func render(wl *Worklist) string {
	var b strings.Builder
	b.WriteString("# 未完成項\n\n")
	b.WriteString("<!-- 這一份由 `go run ./cmd/worklist render` 產生，不要手改。 -->\n")
	b.WriteString("<!-- 權威是 docs/worklist.json；在這裡打勾，下一次 render 會蓋掉。 -->\n\n")
	b.WriteString("每一條掛一個 `verify`：**跑起來為真＝這一條仍然未完成**。\n")
	b.WriteString("核實用 `go run ./cmd/worklist verify`。\n")
	for _, kv := range sorted(wl.Layers) {
		layer, desc := kv[0], kv[1]
		items := itemsIn(wl, layer)
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", layer, desc)
		for _, it := range items {
			fmt.Fprintf(&b, "\n### %s\n\n`%s`\n\n%s\n", it.Title, it.ID, it.Body)
			if it.BlockedBy != "" {
				fmt.Fprintf(&b, "\n**卡在**：%s\n", it.BlockedBy)
			}
			fmt.Fprintf(&b, "\n**怎樣算做完**：%s\n", it.Acceptance)
			fmt.Fprintf(&b, "\n**核實**：`%s`", it.Verify.Kind)
			if it.Verify.Pattern != "" {
				fmt.Fprintf(&b, " ／ `%s`", it.Verify.Pattern)
			}
			b.WriteString("\n")
		}
	}
	if len(wl.Resolved) > 0 {
		b.WriteString("\n## 已解決\n\n")
		b.WriteString("核實不掃這一段——它記的是歷史，不是斷言。\n")
		for _, r := range wl.Resolved {
			fmt.Fprintf(&b, "\n### %s（%s）\n\n`%s`\n\n%s\n", r.Title, r.Date, r.ID, r.How)
			if r.CaughtBy != "" {
				fmt.Fprintf(&b, "\n**被什麼抓到**：%s\n", r.CaughtBy)
			}
			if r.EvidenceLevel != "" {
				fmt.Fprintf(&b, "\n**推論等級**：%s\n", r.EvidenceLevel)
			}
		}
	}
	return b.String()
}

// sorted 讓 render 的輸出穩定——map 的走訪順序每次都不同，
// 不排的話 render 出來的檔案每次都在動，git diff 全是雜訊。
func sorted(m map[string]string) [][2]string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	out := make([][2]string, len(keys))
	for i, k := range keys {
		out[i] = [2]string{k, m[k]}
	}
	return out
}

func itemsIn(wl *Worklist, layer string) []Item {
	var out []Item
	for _, it := range wl.Items {
		if it.Layer == layer {
			out = append(out, it)
		}
	}
	return out
}
