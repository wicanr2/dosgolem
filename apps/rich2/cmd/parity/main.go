// Command parity 是 MVP-B 的驗收：把 `RUN_full.EXE` 跑到防拷密碼畫面，
// 拿 `0xA0000` 的 320×200 色號陣列與原版的索引截圖**逐點**比對
// （`docs/spec/001` §4）。
//
// 「逐點」是刻意的。相符率 99% 看起來很好，實際上那是「整塊東西畫錯位置」
// ——第一次跑就是 99.22%，差的 499 點是一整個 17×27 的滑鼠游標。
//
// oracle 怎麼來：`rich2/tools/dosbox_pw_indexed.py`（DOSBox 的 Ctrl+F5，
// 存的是索引 PNG，像素值就是執行期色號）。
//
// ⚠ **兩邊要對齊三件事**，缺一個就永遠不會 100%：
//
//  1. **亂數種子**。防拷每次問不同的題（密碼圖之一／二／三、留白哪一區）。
//     原版那邊用固定種子版（`rich2/tools/patch_seed.py` 把 TIMER 改成回 0），
//     我們這邊讓 `int 21h AH=2Ch` 回 CX=DX=0，效果相同。
//  2. **滑鼠位置**。游標是遊戲自己畫的 17×27 小手，位置就是畫面內容。
//     DOSBox 的滑鼠會蓋掉程式的 `AX=4`，所以兩邊都要移到同一個座標。
//  3. **移動的時機**。程式在進防拷畫面時用 `AX=4` 設一次游標位置；
//     在那之前移動會被它蓋掉（實測差異一點都沒少）。
//
// ⚠ **本專案不含任何原版檔案**，`-exe`／`-root`／`-oracle` 都由玩家自備。
//
//	go run ./cmd/parity -exe RUN_full.EXE -root RICH2 -oracle pw.png
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func main() {
	exe := flag.String("exe", "", "要跑的 MZ 執行檔（必填）")
	root := flag.String("root", ".", "原版素材目錄")
	oracle := flag.String("oracle", "", "原版的索引 PNG（必填）")
	steps := flag.Uint64("steps", 50_000_000, "最多執行幾道指令")
	mouseX := flag.Int("mouse-x", 160, "滑鼠要移到的像素 X")
	mouseY := flag.Int("mouse-y", 100, "滑鼠要移到的像素 Y")
	mouseAt := flag.Uint64("mouse-at", 45_000_000, "第幾道指令時移動滑鼠")
	verbose := flag.Bool("v", false, "不合時畫出差異遮罩")
	flag.Parse()

	if *exe == "" || *oracle == "" {
		flag.Usage()
		os.Exit(2)
	}

	want, err := loadIndexed(*oracle)
	if err != nil {
		die(err)
	}

	img, err := os.ReadFile(*exe)
	if err != nil {
		die(err)
	}
	m := machine.New()
	if err := m.LoadEXE(img); err != nil {
		die(err)
	}
	d := dos.New(m, *root)
	d.Install()

	for m.Steps < *steps && !m.CPU.Halted && !d.Exited {
		if m.Steps == *mouseAt {
			d.Mouse.X, d.Mouse.Y = uint16(*mouseX), uint16(*mouseY)
		}
		if a := cpu.Addr(m.CPU.Seg[cpu.CS], m.CPU.IP); a >= machine.VideoSeg*16 {
			die(fmt.Errorf("跑出可用記憶體：CS:IP ＝ %04X:%04X",
				m.CPU.Seg[cpu.CS], m.CPU.IP))
		}
		if err := m.Step(); err != nil {
			die(err)
		}
	}

	got := m.Indexed()
	os.Exit(compare(got, want, *verbose))
}

// compare 逐點比對，回傳 process exit code。
func compare(got, want []uint8, verbose bool) int {
	var diff []int
	for i := range got {
		if got[i] != want[i] {
			diff = append(diff, i)
		}
	}
	total := len(got)
	fmt.Printf("逐點相符 %d/%d = %.3f%%\n", total-len(diff), total,
		float64(total-len(diff))*100/float64(total))
	if len(diff) == 0 {
		fmt.Println("✓ 逐點完全相同")
		return 0
	}

	x0, y0 := diff[0]%machine.VideoWidth, diff[0]/machine.VideoWidth
	x1, y1 := x0, y0
	for _, i := range diff {
		x, y := i%machine.VideoWidth, i/machine.VideoWidth
		x0, x1 = min(x0, x), max(x1, x)
		y0, y1 = min(y0, y), max(y1, y)
	}
	fmt.Printf("✗ 不合 %d 點，範圍 x %d–%d  y %d–%d（%d×%d）\n",
		len(diff), x0, x1, y0, y1, x1-x0+1, y1-y0+1)

	if verbose {
		fmt.Println("差異遮罩（X ＝ 不同）：")
		set := map[int]bool{}
		for _, i := range diff {
			set[i] = true
		}
		for y := y0; y <= y1 && y-y0 < 60; y++ {
			line := make([]byte, 0, x1-x0+1)
			for x := x0; x <= x1 && x-x0 < 100; x++ {
				if set[y*machine.VideoWidth+x] {
					line = append(line, 'X')
				} else {
					line = append(line, '.')
				}
			}
			fmt.Printf("  %3d %s\n", y, line)
		}
	}
	return 1
}

// loadIndexed 讀原版的索引 PNG。
//
// **一定要是 `image.Paletted`**——`Ctrl+F5` 存的索引 PNG 的像素值就是執行期
// 色號。被轉成 RGB 的截圖（`import`／`xwd`）不能用：色號只能用最近色反查，
// 近似色多的地方會跳（`rich2/docs/playtest/019` §3）。
func loadIndexed(path string) ([]uint8, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	p, ok := im.(*image.Paletted)
	if !ok {
		return nil, fmt.Errorf("%s 不是索引 PNG（是 %T）——"+
			"要用 DOSBox 的 Ctrl+F5，不能用 import／xwd 的 RGB 截圖", path, im)
	}
	b := p.Bounds()
	if b.Dx() != machine.VideoWidth || b.Dy() != machine.VideoHigh {
		return nil, fmt.Errorf("%s 是 %d×%d，預期 %d×%d",
			path, b.Dx(), b.Dy(), machine.VideoWidth, machine.VideoHigh)
	}
	out := make([]uint8, machine.VideoWidth*machine.VideoHigh)
	for y := 0; y < b.Dy(); y++ {
		copy(out[y*machine.VideoWidth:], p.Pix[y*p.Stride:y*p.Stride+b.Dx()])
	}
	return out, nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "parity:", err)
	os.Exit(1)
}
