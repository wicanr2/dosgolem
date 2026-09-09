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
	// varWalkTick 是行走循環的全域計數器（`rich2/docs/re/121` §3–4）：
	// 移動迴圈每畫一幀 `1DDA0` 就 +1，相位 ＝ `(tick AND 7) / 2`。
	varWalkTick = 0x101C
)

func main() {
	exe := flag.String("exe", "", "RUN_full.EXE（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	max := flag.Int("max", 120, "最多印幾個像素幀")
	budget := flag.Uint64("budget", 150_000_000, "等移動的指令預算")
	answer := flag.String("answer", "", "移動之後回答買地問答：yes／no")
	poll := flag.Bool("poll", false, "量等輸入時的輪詢節拍與游標擦畫節奏（慢）")
	flag.Parse()
	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*exe, *root, *max, *budget, *answer, *poll); err != nil {
		fmt.Fprintln(os.Stderr, "錯誤：", err)
		os.Exit(1)
	}
}

func run(exe, root string, max int, budget uint64, answer string, poll bool) error {
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
	fmt.Println("像素幀  螢幕幀  格號  方向  地圖列  地圖行  101Ch  相位")

	n := 0
	stop := rich2.EachFrame(o, func(o *oracle.Oracle) {
		if n >= max {
			return
		}
		tick := int(int16(o.Word(o.DS(varWalkTick))))
		fmt.Printf("%6d  %6d  %4d  %4d  %6d  %6d  %5d  %4d\n",
			n, o.Frames(), rich2.Tile(o), rich2.Direction(o),
			int(int16(o.Word(o.DS(varMapRow)))), int(int16(o.Word(o.DS(varMapCol)))),
			tick, (tick&7)/2)
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
	tick := int(int16(o.Word(o.DS(varWalkTick))))
	to := rich2.Position(o, player)
	fmt.Printf("\n終點：格 %d　骰 %d　方向 %d　亂數狀態 %06X　101Ch %d（相位 %d）\n",
		to, rich2.Steps(o), rich2.Direction(o), rich2.RNDState(o), tick, (tick&7)/2)
	if poll {
		// 等輸入時遊戲每跑一圈就問一次滑鼠；相鄰兩筆的距離就是那個迴圈的週期。
		// 畫面上那隻小手是**遊戲自己畫的**（`rich2/docs/re/182` §4.1：全檔沒有
		// 呼叫 `INT 33h AX=1`），所以它一閃一閃的節拍就是這個迴圈。
		pollBase := o.Steps()
		if err := o.Run(30_000_000); err != nil {
			return fmt.Errorf("量輪詢節拍：%w", err)
		}
		if st := o.MousePollSteps(); len(st) > 1 {
			var gaps []uint64
			for i := 1; i < len(st); i++ {
				if st[i] > pollBase {
					gaps = append(gaps, st[i]-st[i-1])
				}
			}
			if len(gaps) > 0 {
				var sum uint64
				min, max := gaps[0], gaps[0]
				for _, g := range gaps {
					sum += g
					if g < min {
						min = g
					}
					if g > max {
						max = g
					}
				}
				avg := sum / uint64(len(gaps))
				fmt.Printf("\n等輸入的輪詢節拍：%d 次，平均 %d 道指令"+
					"（最短 %d、最長 %d）＝ %.2f 個計時器刻\n",
					len(gaps), avg, min, max, float64(avg)/165_000)
			}
		}
		// 細粒度取樣：每 N 道指令看一次游標那一塊，量它亮／暗各持續多久。
		// 螢幕幀（70 Hz）的解析度看不出「一幀之內畫了又擦」。
		{
			const grain = 3_000
			grab := func() []uint8 {
				w, _, px := o.Screen()
				out := make([]uint8, 0, 25*16)
				for y := 93; y < 118; y++ {
					out = append(out, px[y*w+252:y*w+268]...)
				}
				return out
			}
			ref := grab()
			changes, runs, cur := 0, []int{}, 0
			lastDiff := 0
			for i := 0; i < 4_000; i++ {
				if err := o.Run(grain); err != nil {
					break
				}
				cu := grab()
				d := 0
				for j := range cu {
					if cu[j] != ref[j] {
						d++
					}
				}
				v := 0
				if d > 30 {
					v = 1
				}
				if i > 0 && v != lastDiff {
					changes++
					runs = append(runs, cur)
					cur = 0
				}
				lastDiff, cur = v, cur+1
			}
			n := len(runs)
			if n > 24 {
				n = 24
			}
			fmt.Printf("\n游標區塊變動：%d 次切換；前幾段長度（單位 %d 道指令）：%v\n",
				changes, grain, runs[:n])
		}
	}
	if answer == "" {
		return nil
	}
	// 回答買地問答，然後等地權真的寫進棋盤欄 12（地主）——那是可比對的
	// 檢查點，不是「跑了幾道指令」。
	before := len(tr.Calls)
	// ⚠ **畫完不等於開始收輸入**（`rich2.WaitReady` 的註解）：買地對話框
	// 剛畫出來時遊戲還沒輪詢，這時送鍵等於沒送。
	if err := rich2.WaitReady(o); err != nil {
		return fmt.Errorf("等對話框收輸入：%w", err)
	}
	if err := rich2.Answer(o, answer == "yes", 5_000_000); err != nil {
		return fmt.Errorf("回答：%w", err)
	}
	owned := oracle.NewCond("地權寫入", func(o *oracle.Oracle) bool {
		return int(rich2.Board(o).Int16(to, 12)) != 0 || rich2.Turn(o) != player
	})
	if err := o.RunUntil(owned, oracle.Budget(budget)); err != nil {
		return fmt.Errorf("等地權：%w", err)
	}
	bd := rich2.Board(o)
	fmt.Printf("\n回答 %s 之後：格 %d 地主 %d 等級 %d　玩家 %d 現金 %d　"+
		"目前玩家 %d　亂數狀態 %06X（這一段又抽 %d 次）\n",
		answer, to, bd.Int16(to, 12), bd.Int16(to, 15), player, rich2.Cash(o, player),
		rich2.Turn(o), rich2.RNDState(o), len(tr.Calls)-before)
	return nil
}
