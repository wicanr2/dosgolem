// Command movetrace 逐**像素幀**印出移動動畫的內部狀態。
//
// 用途：回答「原版每一幀把棋子畫在哪一個地圖格」。棋盤表 `122Ch` 欄 0/1 是
// 那一格的錨點，**不是棋子站的格**（路口格佔不只一個地圖格），所以拿棋盤表
// 的座標差去插值會在「兩軸都變」的那一步走出斜線——原版走的是單軸。
//
//	movetrace -exe RUN_full.EXE -root .../RICH2
//
// ⚠ **本專案不含任何原版檔案**，素材由玩家自備。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/apps/rich2"
	"github.com/wicanr2/dosgolem/oracle"
)

// 這三個 DGROUP 純量是繪製當下用的工作變數（`rich2/docs/re/014` §4）：
// 兩個地圖索引 ＋ 方向。玩家狀態陣列 `1146h` 的座標欄要整步走完才寫回，
// 逐幀要看的是這裡。
const (
	varMapRow = 0x10D8 // 地圖的第一索引
	varMapCol = 0x10DA // 地圖的第二索引
)

func main() {
	exe := flag.String("exe", "", "RUN_full.EXE（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	max := flag.Int("max", 120, "最多印幾個像素幀")
	budget := flag.Uint64("budget", 150_000_000, "等移動的指令預算")
	flag.Parse()
	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*exe, *root, *max, *budget); err != nil {
		fmt.Fprintln(os.Stderr, "錯誤：", err)
		os.Exit(1)
	}
}

func run(exe, root string, max int, budget uint64) error {
	o, err := rich2.Load(exe, root)
	if err != nil {
		return err
	}
	defer o.Close()
	if err := rich2.ToBoard(o); err != nil {
		return fmt.Errorf("進棋盤：%w", err)
	}
	player := rich2.Turn(o)
	from := rich2.Position(o, player)
	fmt.Printf("起點：玩家 %d 在格 %d　方向 %d　亂數狀態 %06X\n",
		player, from, rich2.Direction(o), rich2.RNDState(o))
	for i := 1; i <= rich2.MaxPlayers; i++ {
		if rich2.Position(o, i) == 0 {
			continue
		}
		row, col := rich2.MapCoord(o, i)
		fmt.Printf("  玩家 %d 格 %d 地圖(列 %d, 行 %d) 現金 %d\n",
			i, rich2.Position(o, i), row, col, rich2.Cash(o, i))
	}
	fmt.Println()
	fmt.Println("像素幀  螢幕幀  格號  方向  地圖列  地圖行")

	n := 0
	stop := rich2.EachFrame(o, func(o *oracle.Oracle) {
		if n >= max {
			return
		}
		fmt.Printf("%6d  %6d  %4d  %4d  %6d  %6d\n",
			n, o.Frames(), rich2.Tile(o), rich2.Direction(o),
			int(int16(o.Word(o.DS(varMapRow)))), int(int16(o.Word(o.DS(varMapCol)))))
		n++
	})
	defer stop()

	tr := rich2.TraceRND(o)
	base := len(tr.Calls)
	if err := o.Click(rich2.BtnMoveX, rich2.BtnY); err != nil {
		return fmt.Errorf("點前進：%w", err)
	}
	moved := oracle.NewCond("玩家位置改變", func(o *oracle.Oracle) bool {
		return rich2.Position(o, player) != from || rich2.Turn(o) != player
	})
	if err := o.RunUntil(moved, oracle.Budget(budget)); err != nil {
		return fmt.Errorf("等棋子動：%w", err)
	}
	stop()
	fmt.Printf("\n這一步消耗亂數 %d 次：\n", len(tr.Calls)-base)
	for i, c := range tr.Calls[base:] {
		fmt.Printf("  #%-3d %06X → %06X  RND %.9f  呼叫端 IDA %X\n",
			i, c.State, c.Next(), c.Value(), o.ToIDA(c.Caller))
	}
	fmt.Printf("\n終點：格 %d　骰 %d　方向 %d　亂數狀態 %06X\n",
		rich2.Position(o, player), rich2.Steps(o), rich2.Direction(o), rich2.RNDState(o))
	return nil
}
