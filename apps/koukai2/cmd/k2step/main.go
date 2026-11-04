// Command k2step 逐步操作《大航海時代 II》：從進入點或狀態檔開始，依腳本送輸入、
// 以指令數推進時間，存狀態、畫面與記憶體。
//
// 所有時間都是指令數，同一份腳本、同一個起點永遠得到同一個結果。
//
// 動作（逗號分隔）：
//
//	run:N          跑 N 道指令
//	key:名字       送一個 BIOS 鍵（Return、Esc、Up、Down…，或單一可列印字元）
//	type:文字      送一段可列印文字
//	move:X:Y       移動滑鼠（遊戲座標）
//	click:X:Y      移動、等 hover、按左鍵、放開
//	rclick         原地按右鍵
//	main           跑到 MAIN.EXE 被載入
//	poll:N         跑到滑鼠再被輪詢 N 次
//	at:IDA         跑到 IDA 線性位址（MAIN.EXE）
//	shot:路徑      存 PNG
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/koukai2"
	"github.com/wicanr2/dosgolem/oracle"
)

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	scratch := flag.String("scratch", "", "可寫的暫存層")
	loadState := flag.String("load-state", "", "從這個狀態檔接著跑")
	saveState := flag.String("save-state", "", "跑完存狀態")
	shot := flag.String("shot", "", "跑完存 PNG")
	do := flag.String("do", "", "動作腳本")
	dump := flag.String("dump", "", "跑完傾印 MAIN.EXE DS 範圍：`起hex-迄hex:路徑`，分號分隔")
	budget := flag.Uint64("budget", 2_000_000_000, "條件動作的指令數上限")
	flag.Parse()

	o, err := koukai2.Load(*root)
	if err != nil {
		die(err)
	}
	defer o.Close()
	if *scratch != "" {
		o.SetScratch(*scratch)
	}
	if *loadState != "" {
		if err := o.LoadStateFile(*loadState); err != nil {
			die(err)
		}
	}
	for _, a := range strings.Split(*do, ",") {
		if a == "" {
			continue
		}
		if err := act(o, a, *budget); err != nil {
			fmt.Println(koukai2.Describe(o))
			die(fmt.Errorf("動作 %q：%w", a, err))
		}
	}
	if *dump != "" {
		b, ok := koukai2.Current(o)
		if !ok {
			die(fmt.Errorf("沒有執行中的程式"))
		}
		for _, spec := range strings.Split(*dump, ";") {
			rng, path, _ := strings.Cut(spec, ":")
			lo, hi, _ := strings.Cut(rng, "-")
			l, _ := strconv.ParseUint(lo, 16, 32)
			h, _ := strconv.ParseUint(hi, 16, 32)
			data := o.Bytes(b.DS(uint16(l)), int(h-l))
			if err := os.WriteFile(path, data, 0o644); err != nil {
				die(err)
			}
			fmt.Printf("DS:%04X..%04X sha256 %x → %s\n", l, h, sha256.Sum256(data), path)
		}
	}
	if *shot != "" {
		if err := o.WritePNG(*shot); err != nil {
			die(err)
		}
	}
	if *saveState != "" {
		if err := o.SaveStateFile(*saveState); err != nil {
			die(err)
		}
	}
	w, h, px := o.Screen()
	polls, _ := o.MouseActivity()
	fmt.Printf("步數 %d  %s\n模式 %02Xh %dx%d 畫面 sha256 %x\n滑鼠輪詢 %d  讀走鍵 %d  待讀 %d\n",
		o.Steps(), koukai2.Describe(o), o.VideoMode(), w, h, sha256.Sum256(px),
		polls, o.KeysConsumed(), o.KeysPending())
	for _, r := range o.ExecLog() {
		fmt.Printf("  EXEC %-10s PSP=%04X exit=%d ended=%v tsr=%v\n", r.Base, r.PSP, r.Exit, r.Ended, r.TSR)
	}
	if u := o.Unimplemented(); len(u) > 0 {
		fmt.Printf("未實作：%v\n", u)
	}
	if m := o.Missing(); len(m) > 0 {
		fmt.Printf("缺檔：%v\n", m)
	}
}

func act(o *oracle.Oracle, a string, budget uint64) error {
	f := strings.Split(a, ":")
	num := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 0, 64) }
	switch f[0] {
	case "run":
		n, err := num(1)
		if err != nil {
			return err
		}
		return o.Run(n)
	case "key":
		return o.SendKeys(f[1])
	case "type":
		return o.TypeKeys(f[1])
	case "move":
		x, _ := num(1)
		y, _ := num(2)
		return o.MoveMouse(int(x), int(y))
	case "click":
		x, _ := num(1)
		y, _ := num(2)
		err := o.Click(int(x), int(y), oracle.NoCursorWait())
		if _, none := err.(*oracle.NoResponseError); none {
			fmt.Printf("  %v\n", err)
			return nil
		}
		return err
	case "rclick":
		err := o.Press(oracle.Right())
		if _, none := err.(*oracle.NoResponseError); none {
			fmt.Printf("  %v\n", err)
			return nil
		}
		return err
	case "main":
		return o.RunUntil(oracle.NewCond("MAIN.EXE 載入", koukai2.IsMain), oracle.Budget(budget))
	case "poll":
		n, err := num(1)
		if err != nil {
			return err
		}
		return o.RunUntil(oracle.MousePolled(int(n)), oracle.Budget(budget))
	case "at":
		lin, err := strconv.ParseUint(f[1], 16, 32)
		if err != nil {
			return err
		}
		b, ok := koukai2.Current(o)
		if !ok {
			return fmt.Errorf("沒有執行中的程式")
		}
		target := b.Code(uint32(lin)).Linear()
		return o.RunUntil(oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool {
			return o.IP().Linear() == target
		}), oracle.Budget(budget))
	case "shot":
		return o.WritePNG(f[1])
	}
	return fmt.Errorf("不認得的動作")
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
