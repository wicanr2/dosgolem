// Command solvepw 用窮舉過原版的防拷密碼畫面。
//
// 防拷每題問「地圖上留白的那一區在說明書上是什麼顏色」，答案在紙本說明書上
// ——程式裡沒有。所以這裡**從快照展開四個變體**，看哪一個被接受。
//
// 這件事在 DOSBox 那條線上是昂貴的（每個變體都要重開一次遊戲、25 秒開機
// ＋ 每步 2.2 秒的 sleep），所以 rich2 改成拿說明書掃描圖做影像比對
// （`rich2/tools/dosbox_play.py` 的 analyse／IoU）。在這裡快照是 1 毫秒，
// 窮舉比影像比對簡單也可靠。
//
// ⚠ **本專案不含任何原版檔案。**
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

// 四個色塊的中心（`rich2/docs/re/005`「色塊的精確位置」實測）。
var swatches = []struct {
	name string
	y    int
}{{"綠色", 125}, {"黃色", 143}, {"紅色", 161}, {"藍色", 179}}

const swatchX = 102

func main() {
	exe := flag.String("exe", "", "RUN_full.EXE（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	rounds := flag.Int("rounds", 3, "最多答幾題")
	debug := flag.String("debug-dir", "", "把每個變體的畫面存成 PNG 到這個目錄")
	reverse := flag.Bool("reverse", false, "反序試顏色——**用來驗證判定是真的**："+
		"如果不管先試哪個顏色都第一個就過，那是判定沒生效，不是運氣好")
	wrongX := flag.Int("wrong-x", -1, "不點留白區，改點這個座標（負對照）")
	wrongY := flag.Int("wrong-y", -1, "同上")
	shot := flag.String("shot", "", "過關後把畫面色號寫到這個檔")
	after := flag.Uint64("after", 0, "過關後再跑幾道指令（看它走到哪）")
	keys := flag.String("keys", "", "過關並跑完 -after 之後送這串鍵，"+
		"每個字元之間再跑 -key-gap 道指令")
	keyGap := flag.Uint64("key-gap", 20_000_000, "兩個鍵之間跑幾道指令")
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

	if *reverse {
		for i, j := 0, len(swatches)-1; i < j; i, j = i+1, j-1 {
			swatches[i], swatches[j] = swatches[j], swatches[i]
		}
	}

	if err := o.RunUntil(oracle.PasswordScreen()); err != nil {
		die(err)
	}
	fmt.Printf("防拷畫面（跑了 %d 道指令）\n", o.Steps())

	for round := 1; round <= *rounds; round++ {
		blank := findBlank(o.Indexed(), o.Palette())
		if blank.n == 0 {
			fmt.Println("找不到地圖上的留白區——可能已經不在防拷畫面了")
			break
		}
		fmt.Printf("\n第 %d 題：留白區色號 %d，中心 (%d,%d)，%d 個像素\n",
			round, blank.colour, blank.cx, blank.cy, blank.n)

		before := o.Save()
		beforeScreen := append([]uint8(nil), o.Indexed()...)
		solved := false
		for _, sw := range swatches {
			o.Restore(before)
			// 兩步作答，中間不要移開游標——選色是 hover-based
			// （`rich2/tools/dosbox_play.py`）。
			if err := o.Click(swatchX, sw.y); err != nil {
				fmt.Printf("  %s：選色沒反應（%v）\n", sw.name, err)
				continue
			}
			// 點留白區「沒有立即回饋」不代表沒收到——答錯的訊息要一會兒
			// 才畫出來。所以這裡只記一筆，不跳過。
			tx, ty := blank.cx, blank.cy
			if *wrongX >= 0 {
				tx, ty = *wrongX, *wrongY
			}
			noResp := ""
			if err := o.Click(tx, ty); err != nil {
				noResp = "（點下去當下畫面沒動）"
			}
			// 讓回饋畫完
			if err := o.Run(20_000_000); err != nil {
				die(err)
			}
			if *debug != "" {
				_ = o.WritePNG(fmt.Sprintf("%s/r%d-%s.png", *debug, round, sw.name))
			}
			d := diff(beforeScreen, o.Indexed())
			after := findBlank(o.Indexed(), o.Palette())
			fmt.Printf("  %s：畫面變了 %d 點%s；留白區 ", sw.name, d, noResp)

			// ⚠ **判準不能用「畫面變化量」。**
			//
			// 答對會進下一題，而下一題的佈局完全相同——只有留白的縣市與
			// 標題「密碼圖之X」不一樣。那大概是幾百點，與「答錯多印一行
			// 訊息」同一個量級。第一版拿 20,000 點當門檻，四個顏色全被判
			// 成「不接受」，而其中一個其實是對的。
			//
			// 可靠的判準是**留白區本身**：換題了它會跑到別的縣市，
			// 過關了它會消失（不再是防拷畫面）。
			switch {
			case after.n == 0:
				fmt.Println("消失了  ← 過關")
				solved = true
			case after.cx != blank.cx || after.cy != blank.cy:
				fmt.Printf("移到 (%d,%d)  ← 接受，進下一題\n", after.cx, after.cy)
				solved = true
			default:
				fmt.Println("沒動")
			}
			if solved {
				break
			}
		}
		if !solved {
			fmt.Println("四個顏色都不被接受——判準或座標要重看")
			os.Exit(1)
		}
	}

	if *after > 0 {
		before := len(o.Opened())
		if err := o.Run(*after); err != nil {
			fmt.Println("過關後繼續跑：", err)
		}
		if n := len(o.Opened()); n > before {
			fmt.Printf("\n過關後又開了：%v\n", o.Opened()[before:])
		}
	}

	for i, k := range []byte(*keys) {
		before := len(o.Opened())
		o.Type(string(k))
		if err := o.Run(*keyGap); err != nil {
			fmt.Printf("送第 %d 個鍵之後：%v\n", i, err)
			break
		}
		note := ""
		if n := len(o.Opened()); n > before {
			note = fmt.Sprintf("，開了 %v", o.Opened()[before:])
		}
		fmt.Printf("送鍵 %q：還有 %d 個鍵沒被讀走%s\n", k, o.Pending(), note)
	}

	fmt.Printf("\n過關後：跑了 %d 道指令，開了 %v\n", o.Steps(), o.Opened())
	if *shot != "" {
		if err := os.WriteFile(*shot, o.Indexed(), 0o644); err != nil {
			die(err)
		}
		if err := o.WritePNG(strings.TrimSuffix(*shot, ".bin") + ".png"); err != nil {
			die(err)
		}
		fmt.Println("寫出", *shot, "與同名 PNG")
	}
}

type blankRegion struct {
	cx, cy, n int
	colour    uint8
}

// findBlank 找地圖上的留白區。
//
// ⚠ **判準是調色盤，不是色號。**
//
// 台灣地圖上每個縣市都有自己的色號（實測 192–206），而**調色盤把它們全部
// 設成同一個綠 RGB(0,130,0)**，只有這一題要問的那一區設成白色。
// 換句話說「哪一區被留白」是調色盤的狀態，不是像素的狀態——
// 直接找色號 15（一般的白）會一個點都找不到，而且不會有錯誤訊息。
//
// 地圖在畫面左側；標題文字也是白的，所以要避開上緣。
func findBlank(px []uint8, pal [256][3]uint8) blankRegion {
	var sx, sy, n int
	var colour uint8
	for y := 25; y < 195; y++ {
		for x := 5; x < 92; x++ {
			c := px[y*oracle.Width+x]
			if pal[c][0] > 240 && pal[c][1] > 240 && pal[c][2] > 240 {
				sx += x
				sy += y
				n++
				colour = c
			}
		}
	}
	if n == 0 {
		return blankRegion{}
	}
	return blankRegion{sx / n, sy / n, n, colour}
}

func diff(a, b []uint8) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "solvepw:", err)
	os.Exit(1)
}
