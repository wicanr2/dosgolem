// Command state 在棋盤上讀原版的遊戲狀態。
//
// 用途是把對拍的判準從像素換成變數。rich2 的 RE 筆記定出了陣列的**描述子
// 位址**（`docs/re/014` §2）與部分欄位語意（`docs/re/013`）；這裡把它們讀出來。
//
// ⚠ **本專案不含任何原版檔案。**
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/oracle/rich2"
)

func main() {
	exe := flag.String("exe", "", "RUN_full.EXE（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	find := flag.Int("find", 0, "順便搜尋這個 32 位元值在哪（0 ＝ 不搜）")
	square := flag.Int("square", -1, "印出棋盤陣列 122Ch 這一格的 20 個欄位")
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
	if err := rich2.ToBoard(o); err != nil {
		die(err)
	}
	fmt.Printf("在棋盤上（%d 道指令）\n\n", o.Steps())

	fmt.Printf("目前玩家 %d　格號 %d　方向 %d\n",
		rich2.Turn(o), rich2.Tile(o), rich2.Direction(o))

	fmt.Println("\n玩家（11A2h，1..6 × 0..59，4B，列主序）：")
	fmt.Println("  槽    現金      存款")
	for i := 1; i <= rich2.MaxPlayers; i++ {
		fmt.Printf("  %d  %9d %9d\n", i, rich2.Cash(o, i), rich2.Deposit(o, i))
	}
	fmt.Printf("  有錢的槽：%v\n", rich2.ActivePlayers(o))

	fmt.Println("\n陣列（描述子 → 資料基底，大小對照 docs/re/014 §2）：")
	for _, a := range []struct {
		name string
		arr  *oracle.Array
		want int
	}{
		{"11A2h 玩家金錢", rich2.Money(o), 1440},
		{"1146h 玩家狀態", rich2.PlayerState(o), 360},
		{"1174h 土地表", rich2.Land(o), 900},
		{"122Ch 棋盤", rich2.Board(o), 11320},
	} {
		mark := "✓"
		if a.arr.Size() != a.want {
			mark = "✗"
		}
		fmt.Printf("  %s %-16s 基底 %05X　%d bytes（DIM 表 %d）\n",
			mark, a.name, a.arr.Base, a.arr.Size(), a.want)
	}

	if *square >= 0 {
		bd := rich2.Board(o)
		fmt.Printf("\n122Ch(%d, 0..19)：\n ", *square)
		for j := 0; j < 20; j++ {
			fmt.Printf("%6d", bd.Int16(*square, j))
			if j%10 == 9 {
				fmt.Printf("\n ")
			}
		}
		fmt.Println("\n  欄 4–7 是四個方向的出口（`rich2/docs/re/014` §4c：" +
			"棋盤[格][候選方向+3]）")
	}

	if *find != 0 {
		hits := o.SearchDWord(uint32(*find))
		fmt.Printf("\n搜尋 32 位元的 %d：%d 個命中\n", *find, len(hits))
		for i, h := range hits {
			if i >= 20 {
				fmt.Printf("  …另外 %d 個\n", len(hits)-20)
				break
			}
			fmt.Printf("  %05X%s\n", h, locate(o, h))
		}
	}
}

// locate 說明一個線性位址落在哪個已知陣列裡。
func locate(o *oracle.Oracle, addr uint32) string {
	for _, a := range []struct {
		name string
		arr  *oracle.Array
	}{
		{"11A2h 玩家金錢", rich2.Money(o)},
		{"1146h 玩家狀態", rich2.PlayerState(o)},
		{"1174h 土地表", rich2.Land(o)},
		{"122Ch 棋盤", rich2.Board(o)},
	} {
		if addr >= a.arr.Base && addr < a.arr.Base+uint32(a.arr.Size()) {
			return fmt.Sprintf("　← %s 基底 +%d", a.name, addr-a.arr.Base)
		}
	}
	return ""
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "state:", err)
	os.Exit(1)
}
