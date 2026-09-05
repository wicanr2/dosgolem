// Command dice 找「骰子點數存在哪一個變數」。
//
// 為什麼要找它：原版走一步消耗數十次亂數，因為擲骰動畫三幀、**每一幀都
// 真的擲**（`rich2/docs/re/155`）；remake 只在決定點數時抽一次。
// 兩邊要重播同一局，得先知道那些抽取裡哪一次決定了實際的點數——
// 而第一步是找到「點數」這個值本身。
//
// 做法是**純觀察**：走一步，拿快照差分，留下值落在 1..12 的 word，
// 走幾步取交集。
//
// ⚠ **不要改亂數狀態來做這件事。** 試過：改了狀態遊戲會走進完全不同的
// 路徑（三個種子裡有三個直接換頁，畫面非零像素從 57,000 掉到 24,714），
// 因為那個狀態在擲骰之前就被別的地方用掉了。
//
// ⚠ **本專案不含任何原版檔案。**
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/runtime/basic"
	"github.com/wicanr2/dosgolem/apps/rich2"
)

func main() {
	exe := flag.String("exe", "", "RUN_full.EXE（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	steps := flag.Int("steps", 4, "走幾步")
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
	if err := rich2.ToBoard(o); err != nil {
		die(err)
	}

	var cand map[uint32]bool
	fmt.Println("步　格號　　花費　亂數　變過且值在 1..12 的 word（交集後）")
	for i := 1; i <= *steps; i++ {
		if err := rich2.WaitTurn(o, 1, 0); err != nil {
			die(err)
		}
		if err := rich2.WaitReady(o); err != nil {
			die(err)
		}
		before := o.Save()
		rndBefore := len(tr.Calls)
		cashBefore := rich2.Cash(o, 1)

		from, to, err := rich2.Roll(o)
		if err != nil {
			fmt.Printf("%2d  第 %d 步：%v\n", i, i, err)
			break
		}
		if rich2.Turn(o) == 1 {
			if err := rich2.Answer(o, true, 0); err != nil {
				die(err)
			}
		}

		hits := map[uint32]bool{}
		for _, a := range o.SearchChanged(before) {
			if a%2 != 0 {
				continue // 只看對齊的 word
			}
			if v := o.Word(oracle.Phys(a)); v >= 1 && v <= 12 {
				hits[a] = true
			}
		}
		if cand == nil {
			cand = hits
		} else {
			for a := range cand {
				if !hits[a] {
					delete(cand, a)
				}
			}
		}
		fmt.Printf("%2d  %3d → %3d　%6d　%3d　本步 %d 個，交集後 %d 個\n",
			i, from, to, cashBefore-rich2.Cash(o, 1),
			len(tr.Calls)-rndBefore, len(hits), len(cand))
	}

	fmt.Printf("\n每一步都變、而且值都落在 1..12 的位址：%d 個\n", len(cand))
	keys := make([]uint32, 0, len(cand))
	for a := range cand {
		keys = append(keys, a)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for i, a := range keys {
		if i >= 24 {
			fmt.Printf("  …另外 %d 個\n", len(keys)-24)
			break
		}
		fmt.Printf("  %05X ＝ %2d%s\n", a, o.Word(oracle.Phys(a)), locate(o, a))
	}
}

// locate 說明位址落在哪個已知陣列或 DGROUP 的哪個偏移。
func locate(o *oracle.Oracle, addr uint32) string {
	for _, b := range []struct {
		name string
		arr  *basic.Array
	}{
		{"11A2h 玩家金錢", rich2.Money(o)},
		{"1146h 玩家狀態", rich2.PlayerState(o)},
		{"1174h 土地表", rich2.Land(o)},
		{"122Ch 棋盤", rich2.Board(o)},
		{"11FEh 座標表", rich2.Coord(o)},
	} {
		if idx, ok := b.arr.Index(addr); ok {
			return fmt.Sprintf("　← %s %v", b.name, idx)
		}
	}
	// DGROUP 裡的單一變數：算回 ds: 偏移，那是 rich2 筆記用的形式。
	base := uint32(0x32F9) * 16
	if addr >= base && addr < base+0x10000 {
		return fmt.Sprintf("　← ds:%04X", addr-base)
	}
	return ""
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "dice:", err)
	os.Exit(1)
}
