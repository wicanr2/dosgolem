// Command shot 是《臥龍傳》的對拍取樣器：開機、依腳本操作、存一張
// 640×400 的 PNG，直接拿去跟原版的 DOSBox-X 擷取逐點比。
//
//	go run ./apps/wolong/cmd/shot -exe KI.EXE -root <原版目錄> \
//	    -script "wait;click:320,179;wait" -out shot.png
//
// 腳本的每一步用 `;` 隔開：
//
//	wait              畫面停住為止
//	steps:N           再跑 N 道指令
//	click:X,Y         在遊戲座標 (X,Y) 點左鍵（0–639 × 0–399）
//	move:X,Y          只移動游標
//	shot:NAME         當場存一張 <-dir>/NAME.png
//
// `shot` 讓一次執行產出整條時間軸的每一格——**與 DOSBox-X 那邊的
// `capture-metadata.txt` 是同一種寫法**，舊腳本可以照抄過來
// （y 記得加 `-dosbox-y`）。
//
// ⚠ **座標是遊戲座標，不是 DOSBox-X 的視窗座標。**
// 舊腳本的 y 要先除以 1.2（`-dosbox-y` 幫你換）。
//
// ⚠ **本專案不含任何原版檔案**，`-exe` 與 `-root` 都由玩家自備。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/wolong"
	"github.com/wicanr2/dosgolem/oracle"
)

func main() {
	exe := flag.String("exe", "", "KI.EXE 的路徑（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	script := flag.String("script", "wait", "操作腳本，用 ; 隔開")
	out := flag.String("out", "shot.png", "最後一張輸出的 PNG（腳本沒有 shot 步驟時才用）")
	dir := flag.String("dir", ".", "shot:NAME 步驟寫到哪個目錄")
	full := flag.Bool("full", false, "存完整的 640×480，不裁成 640×400")
	dosboxY := flag.Bool("dosbox-y", false, "腳本裡的 y 是 DOSBox-X 的視窗座標，幫忙換算")
	budget := flag.Uint64("budget", 40_000_000, "每一步的指令數上限")
	flag.Parse()

	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}
	o, err := wolong.Load(*exe, *root)
	if err != nil {
		die(err)
	}
	defer o.Close()

	shots := 0
	for i, step := range strings.Split(*script, ";") {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		if err := run(o, step, *dosboxY, *budget, *dir, *full); err != nil {
			die(fmt.Errorf("第 %d 步「%s」：%w", i+1, step, err))
		}
		if strings.HasPrefix(step, "shot:") {
			shots++
		}
		fmt.Printf("第 %d 步「%s」完成，累計 %d 道指令\n", i+1, step, o.Steps())
	}
	if shots > 0 {
		// 腳本自己存過了就不要再存一張沒有名字的——**同一個狀態出現兩份
		// 檔案，之後沒有人分得出哪一份是驗收依據。**
		report(o)
		return
	}

	if *full {
		err = o.WritePNG(*out)
	} else {
		err = wolong.Shot(o, *out)
	}
	if err != nil {
		die(err)
	}
	w, h, _ := o.Screen()
	fmt.Printf("寫出 %s（畫面 %d×%d）\n", *out, w, h)
	report(o)
}

// report 印收工前一定要看的東西。
//
// **「跑得動」與「跑得動但行為不對」的差別在這裡**：沒實作的服務宣告成功，
// 該填的緩衝區沒填就是垃圾，症狀出現在很後面而且完全不指向這裡。
func report(o *oracle.Oracle) {
	if rep := o.Unimplemented(); len(rep) > 0 {
		fmt.Printf("沒實作的服務（%d 種）：%s\n", len(rep), strings.Join(rep, "、"))
	}
}

func run(o *oracle.Oracle, step string, dosboxY bool, budget uint64,
	dir string, full bool) error {
	verb, arg, _ := strings.Cut(step, ":")
	switch verb {
	case "wait":
		return o.RunUntil(wolong.Booted(), oracle.Budget(budget))
	case "shot":
		path := filepath.Join(dir, arg+".png")
		if full {
			return o.WritePNG(path)
		}
		return wolong.Shot(o, path)
	case "steps":
		n, err := strconv.ParseUint(arg, 10, 64)
		if err != nil {
			return err
		}
		return o.Run(n)
	case "click", "move":
		x, y, err := point(arg, dosboxY)
		if err != nil {
			return err
		}
		if verb == "move" {
			o.MoveMouse(x, y)
			return nil
		}
		return o.Click(x, y)
	}
	return fmt.Errorf("看不懂的動作")
}

func point(arg string, dosboxY bool) (int, int, error) {
	xs, ys, ok := strings.Cut(arg, ",")
	if !ok {
		return 0, 0, fmt.Errorf("座標要寫成 X,Y")
	}
	x, err := strconv.Atoi(strings.TrimSpace(xs))
	if err != nil {
		return 0, 0, err
	}
	y, err := strconv.Atoi(strings.TrimSpace(ys))
	if err != nil {
		return 0, 0, err
	}
	if dosboxY {
		y = wolong.FromDOSBoxY(y)
	}
	return x, y, nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "shot:", err)
	os.Exit(1)
}
