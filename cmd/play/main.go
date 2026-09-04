// Command play 在棋盤上操作原版：走頂端按鈕列、擲骰、買地。
//
// 座標取自 `rich2/tools/dosbox_query.py` 的實測值
// （`rich2/docs/spec/063` §111：MOVE (125,9)、QUERY (167,9)）。
//
// ⚠ **本專案不含任何原版檔案。**
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/oracle/rich2"
)

func main() {
	exe := flag.String("exe", "", "RUN_full.EXE（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	x := flag.Int("x", -1, "要點的 X（−1 ＝ 不點。頂端按鈕列：前進 125、查詢 167）")
	y := flag.Int("y", 9, "要點的 Y")
	after := flag.Uint64("after", 40_000_000, "點完再跑幾道指令")
	roll := flag.Bool("roll", false, "點「前進」擲骰並等棋子走完")
	key := flag.String("key", "", "送這串鍵，一個一個送")
	keyGap := flag.Uint64("key-gap", 20_000_000, "兩個鍵之間跑幾道指令")
	shot := flag.String("shot", "", "把畫面存成 PNG")
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
	fmt.Printf("在棋盤上（%d 道指令，已抽 %d 次亂數）\n", o.Steps(), len(tr.Calls))
	dump(o, "點之前")

	before := o.Save()
	beforeRND := len(tr.Calls)
	beforeScreen := append([]uint8(nil), o.Indexed()...)

	if *roll {
		p0, r0 := o.MouseActivity()
		from, to, err := rich2.Roll(o, rich2.RollBudget(*after))
		p1, r1 := o.MouseActivity()
		fmt.Printf("\n擲骰：格號 %d → %d（第 %d 道）　"+
			"期間輪詢 %d 次、按著 %d 次\n", from, to, o.Steps(), p1-p0, r1-r0)
		if err != nil {
			fmt.Println("  ", err)
		}
	} else if *x >= 0 {
		fmt.Printf("\n點 (%d,%d)……\n", *x, *y)
		if err := o.Click(*x, *y); err != nil {
			fmt.Println("  ", err)
		}
		if err := o.Run(*after); err != nil {
			fmt.Println("  ", err)
		}
	}
	// **一個一個送。** 一次把整串排進佇列的話，`INKEY$` 的輪詢迴圈會在
	// 同一輪把它們全部讀走，等於同時按下——選單根本來不及反應。
	for _, k := range []byte(*key) {
		o.Type(string(k))
		if err := o.Run(*keyGap); err != nil {
			fmt.Println("  送鍵期間：", err)
			break
		}
		fmt.Printf("送鍵 %q：還有 %d 個沒被讀走　格號 %d\n",
			k, o.Pending(), rich2.Tile(o))
	}
	diff := 0
	for i, v := range o.Indexed() {
		if v != beforeScreen[i] {
			diff++
		}
	}
	fmt.Printf("畫面變了 %d 點，期間抽了 %d 次亂數\n",
		diff, len(tr.Calls)-beforeRND)
	for _, c := range tr.Calls[beforeRND:] {
		fmt.Printf("  第 %d 道　%06X → %06X　RND ＝ %.6f　呼叫端 IDA %05X\n",
			c.Step, c.State, c.Next(), c.Value(), o.ToIDA(c.Caller))
	}
	dump(o, "點之後")

	fmt.Println("\nint 33h 的功能號使用次數：")
	calls := o.MouseCalls()
	keys := make([]int, 0, len(calls))
	for k := range calls {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Printf("  AX=%04X ×%d%s\n", k, calls[uint16(k)], mouseFn(uint16(k)))
	}

	if x, y, step, ok := o.LastPressedPoll(); ok {
		fmt.Printf("最後一次「回報按著」：(%d,%d) 在第 %d 道\n", x, y, step)
	} else {
		fmt.Println("**遊戲從來沒有讀到按著**")
	}
	fmt.Println("程式自己設游標位置（AX=4）：")
	for _, st := range o.MouseSets() {
		fmt.Printf("  第 %10d 道 → (%d,%d)\n", st.Step, st.X, st.Y)
	}

	// 狀態有沒有變——比畫面可靠。
	changed := o.SearchChanged(before)
	fmt.Printf("\n記憶體變了 %d 個位元組\n", len(changed))
	inArrays(o, changed)

	if *shot != "" {
		if err := o.WritePNG(*shot); err != nil {
			die(err)
		}
		fmt.Println("寫出", *shot)
	}
}

func dump(o *oracle.Oracle, tag string) {
	fmt.Printf("%s：玩家 %d　格號 %d　方向 %d　現金 %d\n",
		tag, rich2.Turn(o), rich2.Tile(o), rich2.Direction(o), rich2.Cash(o, 1))
}

// inArrays 把變動的位元組歸類到已知的陣列裡。
//
// **這是「這個動作改了什麼」最直接的答案**，比從畫面反推可靠。
func inArrays(o *oracle.Oracle, changed []uint32) {
	type box struct {
		name string
		arr  *oracle.Array
	}
	boxes := []box{
		{"11A2h 玩家金錢", rich2.Money(o)},
		{"1146h 玩家狀態", rich2.PlayerState(o)},
		{"1174h 土地表", rich2.Land(o)},
		{"122Ch 棋盤", rich2.Board(o)},
	}
	// 把變動的位址反查成索引，並收斂成「哪一格的哪一欄」。
	cells := map[string]map[string]bool{}
	for _, c := range changed {
		for _, b := range boxes {
			if idx, ok := b.arr.Index(c); ok {
				if cells[b.name] == nil {
					cells[b.name] = map[string]bool{}
				}
				cells[b.name][fmt.Sprint(idx)] = true
			}
		}
	}
	if len(cells) == 0 {
		fmt.Println("  已知的四個陣列都沒動")
		return
	}
	for _, b := range boxes {
		set := cells[b.name]
		if len(set) == 0 {
			continue
		}
		keys := make([]string, 0, len(set))
		for k := range set {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) > 12 {
			keys = append(keys[:12], fmt.Sprintf("…另外 %d 格", len(set)-12))
		}
		fmt.Printf("  %s：%v\n", b.name, keys)
		// 印出現值——「哪一格變了」配上「變成什麼」才推得動語意。
		for _, k := range keys {
			var i, j int
			if _, err := fmt.Sscanf(k, "[%d %d]", &i, &j); err != nil {
				continue
			}
			var v int
			if b.arr.Width == 4 {
				v = int(b.arr.Int32(i, j))
			} else {
				v = int(b.arr.Int16(i, j))
			}
			fmt.Printf("      (%d, %d) ＝ %d\n", i, j, v)
		}
	}
}

func mouseFn(ax uint16) string {
	switch ax {
	case 0:
		return "　重設並偵測"
	case 1:
		return "　顯示游標"
	case 2:
		return "　隱藏游標"
	case 3:
		return "　取位置與鍵狀態"
	case 4:
		return "　設位置"
	case 5:
		return "　按下的統計"
	case 6:
		return "　放開的統計"
	case 7, 8:
		return "　設範圍"
	}
	return ""
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "play:", err)
	os.Exit(1)
}
