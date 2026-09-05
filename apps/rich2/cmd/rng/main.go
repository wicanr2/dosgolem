// Command rng 用 dosgolem 直接量原版 BASIC runtime 的 `RND`。
//
// `rich2/docs/re/050` 已經靜態解出演算法並標 confirmed：
//
//	xₙ₊₁ = (xₙ × 0x43FD43FD + 0xC39EC3) mod 2²⁴
//	RND  = xₙ₊₁ / 2²⁴
//
// 這支在同一個 Go 行程裡重驗，並回答 `rich2/WORKLIST.md` P1.1 要的兩件事：
// `RANDOMIZE TIMER` 之後的初始狀態是多少，以及誰在消耗亂數、各消耗幾次。
//
// 演算法與攔截都在 oracle/rich2；這裡只負責印。
//
// ⚠ **本專案不含任何原版檔案。**
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/wicanr2/dosgolem/apps/rich2"
	"github.com/wicanr2/dosgolem/oracle"
)

// LCG 的三個常數在原版 DGROUP 裡的位置（`rich2/docs/re/050` §2）。
// **每次都讀出來對一次**——位址錯了會讀到看起來很合理的別的值。
var constants = []struct {
	off  uint16
	want string
	note string
}{
	{0x279A, "43FD", "乘數"},
	{0x279E, "9EC3", "加數（高位元組 C3 由同一個 word 再用一次）"},
}

// 新局初始化那三個 RND 各自的係數（反組譯 1B61A–1B695 讀到的運算元）。
var newGameCoef = []struct {
	off  uint16
	note string
}{
	{0x1C6E, "① 乘數（1B625 fmul）→ 陣列 15F2h"},
	{0x1CD0, "② 乘數（1B64F fmul）→ 陣列 10EAh"},
	{0x1C8E, "② 加數（1B65B fadd）"},
	{0x1DAE, "③ 乘數（1B67E fmul）→ 陣列 1620h"},
	{0x1C8A, "③ 減數（1B683 fsub）"},
}

func main() {
	exe := flag.String("exe", "", "RUN_full.EXE（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	stop := flag.String("stop", "board", "量到哪裡：password 或 board")
	limit := flag.Int("limit", 20, "印出前幾次呼叫")
	flag.Parse()
	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}

	o, err := oracle.Load(*exe, *root)
	if err != nil {
		die(err)
	}
	defer o.Close()

	tr := rich2.TraceRND(o)
	switch *stop {
	case "password":
		err = o.RunUntil(oracle.PasswordScreen())
	case "board":
		err = rich2.ToBoard(o)
	default:
		die(fmt.Errorf("-stop 只能是 password 或 board"))
	}
	if err != nil {
		die(err)
	}

	fmt.Printf("跑了 %d 道指令\n", o.Steps())
	fmt.Printf("RANDOMIZE ×%d", len(tr.RandomizeAt))
	if len(tr.RandomizeAt) > 0 {
		fmt.Printf("（第一次在第 %d 道）", tr.RandomizeAt[0])
	}
	fmt.Println()

	fmt.Println("\nLCG 常數（讀原版的 DGROUP）：")
	for _, c := range constants {
		got := fmt.Sprintf("%04X", o.Word(o.DS(c.off)))
		mark := "✓"
		if got != c.want {
			mark = "✗"
		}
		fmt.Printf("  %s ds:%04X = %s（預期 %s）%s\n", mark, c.off, got, c.want, c.note)
	}
	fmt.Printf("  除數 ds:27A2 = %v（＝ 2²⁴）\n", o.Float(o.DS(0x27A2)))

	fmt.Println("\n新局初始化的係數：")
	for _, c := range newGameCoef {
		fmt.Printf("  ds:%04X = %-8v %s\n", c.off, o.Float(o.DS(c.off)), c.note)
	}

	fmt.Printf("\nRND ×%d　初始狀態 %06X　最終狀態 %06X\n",
		len(tr.Calls), tr.InitialState, rich2.RNDState(o))
	if err := tr.Verify(o); err != nil {
		fmt.Println("✗", err)
		os.Exit(1)
	}
	fmt.Println("✓ 每一次的狀態轉移都符合 x' = (x × 43FD43FD + C39EC3) mod 2²⁴")

	by := tr.ByCaller(o)
	keys := make([]uint32, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return by[keys[i]] > by[keys[j]] })
	fmt.Printf("\n呼叫端（%d 個）：\n", len(keys))
	for _, k := range keys {
		fmt.Printf("  IDA %05X ×%d\n", k, by[k])
	}

	if *limit > 0 {
		fmt.Printf("\n前 %d 次：\n", *limit)
		for i, c := range tr.Calls {
			if i >= *limit {
				break
			}
			fmt.Printf("  #%-3d 第 %10d 道　%06X → %06X　RND ＝ %.9f　"+
				"呼叫端 IDA %05X\n",
				i, c.Step, c.State, c.Next(), c.Value(), o.ToIDA(c.Caller))
		}
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "rng:", err)
	os.Exit(1)
}
