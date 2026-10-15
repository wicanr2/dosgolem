// Package repolint 放「整個儲存庫的一致性」檢查：編譯與 vet 都會過、
// 只有跑起來或有人去讀才會發現的錯。
//
// 這些檢查都是合併多條分支時實際踩到的（WORKLOG 2026-10-15）。
package repolint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// root 是儲存庫根目錄（本檔在 internal/repolint）。
const root = "../.."

// 同一個執行檔裡兩處 `flag.String("scratch", ...)` 編得過、vet 也過，
// 一執行就 panic「flag redefined」。三條分支各自加了 `-scratch` 時就是這樣。
func TestCommandFlagsAreUnique(t *testing.T) {
	defs := map[string]map[string][]string{} // 目錄 → 旗標名 → 位置
	for _, top := range []string{"cmd", "apps"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, e os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			if f.Name.Name != "main" {
				return nil
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "flag" {
					return true
				}
				// flag.String("名字", ...)；flag.Var(&v, "名字", ...) 的名字在第二個參數。
				arg := 0
				switch sel.Sel.Name {
				case "Var", "TextVar":
					arg = 1
				case "Bool", "Int", "Int64", "Uint", "Uint64", "String", "Float64", "Duration", "Func", "BoolFunc":
				default:
					return true
				}
				if len(call.Args) <= arg {
					return true
				}
				lit, ok := call.Args[arg].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				name, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				dir := filepath.Dir(path)
				if defs[dir] == nil {
					defs[dir] = map[string][]string{}
				}
				defs[dir][name] = append(defs[dir][name], fset.Position(lit.Pos()).String())
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(defs) == 0 {
		t.Fatal("一個執行檔都沒找到，路徑可能錯了")
	}
	for dir, names := range defs {
		for name, at := range names {
			if len(at) > 1 {
				t.Errorf("%s：旗標 -%s 定義了 %d 次：%s", dir, name, len(at), strings.Join(at, "、"))
			}
		}
	}
}

// 每份規格都要在 `docs/spec/000-index.md` 裡出現。
//
// 引用規格要連號碼帶檔名，而同一個號碼底下有好幾份；索引漏列的那一份，
// 讀者就只能靠 grep 才知道它存在。合併時發現 17 份是原分支就漏登的。
func TestSpecIndexListsEverySpec(t *testing.T) {
	dir := filepath.Join(root, "docs", "spec")
	idx, err := os.ReadFile(filepath.Join(dir, "000-index.md"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".md") || n == "000-index.md" {
			continue
		}
		if !strings.Contains(string(idx), n) {
			missing = append(missing, n)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("索引沒列到 %d 份規格：%s", len(missing), strings.Join(missing, "、"))
	}
}

// findCPUAddrCalls 回 src 裡所有 `cpu.Addr(` 呼叫的位置。
func findCPUAddrCalls(filename string, src any) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	var at []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Addr" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "cpu" {
			at = append(at, fset.Position(call.Pos()).String())
		}
		return true
	})
	return at, nil
}

// internal/dos 的非測試檔不得呼叫 `cpu.Addr(`（docs/spec/432-linear-memory-and-paging §4）。
//
// `cpu.Addr` 把 段:偏移 遮成 20 位元。轉換開啟且 A20 開時，客體的 `FFFF:0010` 起是線性
// 0x100000 起，服務層卻會讀到 0 起——編得過、既有測試也過（轉換關閉時兩者相等），
// 只有 JEMM 場景才錯。服務層的客體指標一律用 `Machine.SvcAddr`。
// 沒有白名單：診斷回溯也用 SvcAddr。
func TestDOSServiceDoesNotCallCPUAddr(t *testing.T) {
	dir := filepath.Join(root, "internal", "dos")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		checked++
		at, err := findCPUAddrCalls(filepath.Join(dir, n), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range at {
			t.Errorf("%s：internal/dos 不得呼叫 cpu.Addr，改用 d.M.SvcAddr", p)
		}
	}
	if checked == 0 {
		t.Fatal("internal/dos 一個非測試檔都沒找到，路徑可能錯了")
	}
}

// 規則本身要抓得到：新增一個 `cpu.Addr(` 呼叫就失敗。
func TestCPUAddrRuleCatchesNewCall(t *testing.T) {
	const bad = `package dos

import "github.com/wicanr2/dosgolem/internal/cpu"

func f(c *cpu.CPU) uint32 { return cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX]) }
`
	at, err := findCPUAddrCalls("bad.go", bad)
	if err != nil {
		t.Fatal(err)
	}
	if len(at) != 1 {
		t.Fatalf("找到 %d 處 cpu.Addr 呼叫，應為 1：%v", len(at), at)
	}
	const good = `package dos

func f(d *DOS) uint32 { return d.M.SvcAddr(1, 2) }
`
	at, err = findCPUAddrCalls("good.go", good)
	if err != nil {
		t.Fatal(err)
	}
	if len(at) != 0 {
		t.Fatalf("SvcAddr 不該被抓：%v", at)
	}
}
