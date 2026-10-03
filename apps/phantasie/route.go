package phantasie

import (
	"fmt"
	"strconv"
	"strings"
)

// 路線檔（docs/spec/005 §4）：UTF-8 文字，一行一個項目，# 開頭的行是註解。

// RouteKind 是路線項目的種類。
type RouteKind uint8

const (
	RouteKey    RouteKind = iota // 送出一個按鍵
	RouteCheck                   // @check <名稱>
	RouteLang                    // @lang <語言>：切換顯示語言（docs/spec/004 §5、§7 第 3 項）
	RouteAssert                  // @assert-visible-same <檢查點 A> <檢查點 B>：兩個檢查點的可見格集合必須相同（docs/spec/004 §7 第 3 項）
	RouteSnap                    // @snap <名稱> <步數>：鍵都送出後再執行指定步數並等事件結束後擷取（原版沒有讀鍵入口的畫面，例如戰鬥回合）
)

// RouteStep 是路線的一步：按鍵（含它前面的 @wait 讀鍵次數）或檢查點（含 @expect 與 @known-untranslated）。
type RouteStep struct {
	Kind   RouteKind
	Key    string   // RouteKey：鍵名
	Wait   uint64   // RouteKey：先放過幾次讀鍵入口（@wait）
	Name   string   // RouteCheck、RouteSnap：檢查點名稱；RouteLang：語言
	Steps  uint64   // RouteSnap：最後一個鍵送出後再執行的步數
	Other  string   // RouteAssert：第二個檢查點名稱（Name 是第一個）
	Screen bool     // RouteAssert：為真是 @assert-same-screen（vram 與疊字內容都相同），否則是 @assert-visible-same
	Expect []string // RouteCheck、RouteSnap：必須已有疊字的 catalog 鍵
	Known  []string // RouteCheck、RouteSnap：允許未譯的鍵
	Line   int
}

// ParseRoute 解析路線檔。語法錯誤回含行號的錯誤，不猜。
func ParseRoute(text string) ([]RouteStep, error) {
	var steps []RouteStep
	var wait uint64
	haveWait := false
	seen := map[string]bool{}
	for i, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		n := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "@") {
			if strings.ContainsAny(line, " \t") {
				return nil, fmt.Errorf("第 %d 行：一行只能有一個按鍵：%q", n, line)
			}
			steps = append(steps, RouteStep{Kind: RouteKey, Key: line, Wait: wait, Line: n})
			wait, haveWait = 0, false
			continue
		}
		dir, arg, _ := strings.Cut(line, " ")
		arg = strings.TrimSpace(arg)
		switch dir {
		case "@wait":
			v, err := strconv.ParseUint(arg, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("第 %d 行：@wait 需要非負整數：%q", n, arg)
			}
			wait, haveWait = v, true
		case "@check":
			if arg == "" || strings.ContainsAny(arg, " \t") {
				return nil, fmt.Errorf("第 %d 行：@check 需要一個不含空白的名稱：%q", n, arg)
			}
			if haveWait {
				return nil, fmt.Errorf("第 %d 行：@wait 後面必須接按鍵", n)
			}
			if seen[arg] {
				return nil, fmt.Errorf("第 %d 行：檢查點名稱 %q 重複", n, arg)
			}
			seen[arg] = true
			steps = append(steps, RouteStep{Kind: RouteCheck, Name: arg, Line: n})
		case "@snap":
			f := strings.Fields(arg)
			if len(f) != 2 {
				return nil, fmt.Errorf("第 %d 行：@snap 需要名稱與步數：%q", n, arg)
			}
			v, err := strconv.ParseUint(f[1], 10, 63)
			if err != nil {
				return nil, fmt.Errorf("第 %d 行：@snap 的步數需要非負整數：%q", n, f[1])
			}
			if haveWait {
				return nil, fmt.Errorf("第 %d 行：@wait 後面必須接按鍵", n)
			}
			if seen[f[0]] {
				return nil, fmt.Errorf("第 %d 行：檢查點名稱 %q 重複", n, f[0])
			}
			seen[f[0]] = true
			steps = append(steps, RouteStep{Kind: RouteSnap, Name: f[0], Steps: v, Line: n})
		case "@assert-visible-same", "@assert-same-screen":
			f := strings.Fields(arg)
			if len(f) != 2 {
				return nil, fmt.Errorf("第 %d 行：%s 需要兩個檢查點名稱：%q", n, dir, arg)
			}
			for _, name := range f {
				if !seen[name] {
					return nil, fmt.Errorf("第 %d 行：%s 的檢查點 %q 必須先出現", n, dir, name)
				}
			}
			if haveWait {
				return nil, fmt.Errorf("第 %d 行：@wait 後面必須接按鍵", n)
			}
			steps = append(steps, RouteStep{Kind: RouteAssert, Name: f[0], Other: f[1], Screen: dir == "@assert-same-screen", Line: n})
		case "@lang":
			if arg == "" || strings.ContainsAny(arg, " \t") {
				return nil, fmt.Errorf("第 %d 行：@lang 需要一個語言代碼：%q", n, arg)
			}
			if haveWait {
				return nil, fmt.Errorf("第 %d 行：@wait 後面必須接按鍵", n)
			}
			steps = append(steps, RouteStep{Kind: RouteLang, Name: arg, Line: n})
		case "@expect", "@known-untranslated":
			if arg == "" {
				return nil, fmt.Errorf("第 %d 行：%s 需要一個鍵", n, dir)
			}
			if len(steps) == 0 || (steps[len(steps)-1].Kind != RouteCheck && steps[len(steps)-1].Kind != RouteSnap) {
				return nil, fmt.Errorf("第 %d 行：%s 必須緊接在 @check 或 @snap 之後", n, dir)
			}
			c := &steps[len(steps)-1]
			if dir == "@expect" {
				c.Expect = append(c.Expect, arg)
			} else {
				c.Known = append(c.Known, arg)
			}
		default:
			return nil, fmt.Errorf("第 %d 行：未知的指令 %s", n, dir)
		}
	}
	if haveWait {
		return nil, fmt.Errorf("路線結尾的 @wait 後面沒有按鍵")
	}
	return steps, nil
}
