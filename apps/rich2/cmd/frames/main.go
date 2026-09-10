// Command frames 把**一回合的移動**逐幀 dump 成索引 PNG ＋ 一份狀態清單。
//
// 用途是與 remake 的同編號幀逐點比對。幀的定義見 `docs/spec/187`：
// 一幀 ＝ `rich2.FrameTicks`（4）個計時器刻，出處是
// `rich2/docs/spec/059`「每個像素幀的等待」（confirmed）。
//
//	frames -exe RUN_full.EXE -root .../RICH2 -out workplace/frames
//
// ⚠ **本專案不含任何原版檔案**，素材由玩家自備。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/apps/rich2"
	"github.com/wicanr2/dosgolem/oracle"
)

// frameRec 是一幀的狀態。座標與格號用來對齊 remake 的同一幀——
// **光比 PNG 不夠**：兩邊的幀號對齊錯位時，畫面差異看起來會像繪製 bug。
type frameRec struct {
	Frame  int    `json:"frame"`
	Screen uint64 `json:"screen"` // VGA 垂直回掃次數
	Tick   uint64 `json:"tick"`
	Step   uint64 `json:"step"`
	Player int    `json:"player"`
	Square int    `json:"square"`
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Tile   int    `json:"tile"`
}

func main() {
	exe := flag.String("exe", "", "RUN_full.EXE（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	out := flag.String("out", "workplace/frames", "輸出目錄")
	maxFrames := flag.Int("max", 1200, "最多 dump 幾幀")
	after := flag.Uint64("after", 400,
		"棋子開始動之後再錄幾個螢幕幀（走完一整步要這一段）")
	budget := flag.Uint64("budget", 150_000_000, "等移動的指令預算")
	flag.Parse()
	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*exe, *root, *out, *maxFrames, *budget, *after); err != nil {
		fmt.Fprintln(os.Stderr, "錯誤：", err)
		os.Exit(1)
	}
}

func run(exe, root, out string, maxFrames int, budget, after uint64) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
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
	fmt.Printf("起點：玩家 %d 在格 %d（tick %d）\n", player, from, o.Ticks())

	var recs []frameRec
	n := 0
	// **錄影用螢幕幀**（VGA 垂直回掃），不是動畫幀：
	// 回掃是顯示硬體的事實，逐幀存下來就是螢幕實際輸出的序列
	// （`docs/spec/187` §3）。原版一個像素幀 ≈ 4 次刷新。
	o.OnFrame(func(o *oracle.Oracle) {
		if n >= maxFrames {
			return
		}
		row, col := rich2.MapCoord(o, player)
		recs = append(recs, frameRec{
			Frame: n, Screen: o.Frames(), Tick: o.Ticks(), Step: o.Steps(), Player: player,
			Square: rich2.Position(o, player), Row: row, Col: col,
			Tile: rich2.Tile(o),
		})
		// ⚠ **這裡的畫面是上一幀畫完的結果**（`docs/spec/187` §4）：
		// 回呼點在送中斷之前。
		if err := o.WritePNG(filepath.Join(out, fmt.Sprintf("frame-%04d.png", n))); err != nil {
			fmt.Fprintln(os.Stderr, "寫 PNG：", err)
		}
		n++
	})
	stop := func() { o.OnFrame(nil) }
	defer stop()

	// 點「前進」，然後等到棋子真的動了或回合推進。
	if err := o.Click(rich2.BtnMoveX, rich2.BtnY); err != nil {
		return fmt.Errorf("點前進：%w", err)
	}
	moved := oracle.NewCond("玩家位置改變", func(o *oracle.Oracle) bool {
		return rich2.Position(o, player) != from || rich2.Turn(o) != player
	})
	if err := o.RunUntil(moved, oracle.Budget(budget)); err != nil {
		return fmt.Errorf("等棋子動：%w", err)
	}
	// ⚠ **「動了」只是第一格。** 走完一整步還要好幾格，每格 4／6／8 個
	// 像素幀、每個像素幀 4 次螢幕刷新——停在這裡錄到的全是擲骰動畫。
	if after > 0 {
		if err := o.RunUntil(oracle.AtFrame(o.Frames()+after),
			oracle.Budget(budget)); err != nil {
			return fmt.Errorf("錄移動尾段：%w", err)
		}
	}
	stop()

	to := rich2.Position(o, player)
	fmt.Printf("終點：格 %d　骰 %d　共 %d 個螢幕幀（tick %d、刷新 %d）\n",
		to, rich2.Steps(o), len(recs), o.Ticks(), o.Frames())

	f, err := os.Create(filepath.Join(out, "frames.json"))
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"player": player, "from": from, "to": to,
		"dice": rich2.Steps(o), "frameTicks": rich2.FrameTicks,
		"screenFrames": o.Frames(),
		"frames":       recs,
	})
}
