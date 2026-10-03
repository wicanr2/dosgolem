// Command overlay-prototype 是疊字層的證據原型（不是實作）：
// 跑原版啟動鏈，用 xlate 在 2× 畫面上蓋繁體中文，存成 PNG。
// 只讀原版狀態；用來驗證「攔截點加疊字層」的做法可行，之後才依規格實作。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/phantasie"
	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/xlate"
)

// cga 模式 4 調色盤 1（高亮度）。
var pal = [4][3]uint8{{0, 0, 0}, {85, 255, 255}, {255, 85, 255}, {255, 255, 255}}

func loadCatalog(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		c := strings.Split(sc.Text(), "\t")
		if len(c) >= 2 {
			m[c[0]] = c[1]
		}
	}
	return m, sc.Err()
}

func wide(r rune) bool { return r >= 0x2E80 || (r >= 0xFF00 && r <= 0xFFEF) }

// stamps 把一行譯文拆成「全形一格 8 像素、半形一格 4 像素」的幾筆疊字，並用空白補滿原文寬度。
func stamps(font *xlate.Font, key string, col, row, origCells int, tr string) []*xlate.Stamp {
	var out []*xlate.Stamp
	x := col * 8
	runes := []rune(tr)
	for i := 0; i < len(runes); {
		j := i
		for j < len(runes) && wide(runes[j]) == wide(runes[i]) {
			j++
		}
		cw := 4
		if wide(runes[i]) {
			cw = 8
		}
		out = append(out, &xlate.Stamp{Key: key, X: x, Y: row * 8, Cells: j - i, CellW: cw, CellH: 8,
			Font: font, GlyphScale: 1, Text: runes[i:j], State: xlate.Pending})
		x += (j - i) * cw
		i = j
	}
	if pad := col*8 + origCells*8 - x; pad > 0 {
		out = append(out, &xlate.Stamp{Key: key, X: x, Y: row * 8, Cells: pad / 4, CellW: 4, CellH: 8,
			Font: font, GlyphScale: 1, Text: []rune(strings.Repeat(" ", pad/4)), State: xlate.Pending})
	}
	return out
}

func rgbFrame(idx []uint8) []uint8 {
	rgb := make([]uint8, 320*200*3)
	for i, v := range idx {
		c := pal[v&3]
		rgb[3*i], rgb[3*i+1], rgb[3*i+2] = c[0], c[1], c[2]
	}
	return rgb
}

func compose(layer *xlate.Layer, idx []uint8, scale int, missing func(rune)) *image.RGBA {
	W, H := 320*scale, 200*scale
	dst := make([]uint8, W*H*4)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c := pal[idx[(y/scale)*320+x/scale]&3]
			o := (y*W + x) * 4
			dst[o], dst[o+1], dst[o+2], dst[o+3] = c[0], c[1], c[2], 255
		}
	}
	layer.Draw(dst, scale, missing)
	return &image.RGBA{Pix: dst, Stride: W * 4, Rect: image.Rect(0, 0, W, H)}
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

type shot struct {
	step uint64
	path string
}

func main() {
	root := flag.String("root", "", "原版目錄")
	fontPath := flag.String("font", "", "GOLEMFNT 字型檔")
	catPath := flag.String("catalog", "", "譯文 TSV（key、translation、source）")
	steps := flag.Uint64("steps", 12_000_000, "跑到第幾道指令")
	keys := flag.String("keys", "", "送鍵：步數:鍵名[,...]")
	shots := flag.String("shots", "", "存圖：步數:路徑[,...]")
	every := flag.Uint64("frame-every", 20_000, "每幾道指令呼叫一次 Layer.Frame")
	scale := flag.Int("scale", 2, "放大倍率")
	flag.Parse()
	if *root == "" || *fontPath == "" || *catPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	font, err := xlate.LoadFont(*fontPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	font.Name = "full16"
	cat, err := loadCatalog(*catPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	o, names, err := phantasie.Launch(*root, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer o.Close()
	run := func(n uint64) {
		cond := oracle.NewCond("步數", func(o *oracle.Oracle) bool { return o.Steps() >= n })
		if err := o.RunUntil(cond, oracle.Budget(*steps+1)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	err = o.RunUntil(oracle.NewCond("鏈載入", func(o *oracle.Oracle) bool { return len(o.ExecLog()) >= len(names)-1 }), oracle.Budget(*steps))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	img := phantasie.ImageSeg(o)
	layer := &xlate.Layer{W: 320, H: 200}
	gate := phantasie.NewKeyGate(o, img)
	for _, it := range strings.Split(*keys, ",") {
		if i := strings.Index(it, ":"); i > 0 {
			n, _ := strconv.ParseUint(it[:i], 10, 64)
			gate.Press(n, it[i+1:])
		}
	}
	var shotList []shot
	for _, it := range strings.Split(*shots, ",") {
		if i := strings.Index(it, ":"); i > 0 {
			n, _ := strconv.ParseUint(it[:i], 10, 64)
			shotList = append(shotList, shot{n, it[i+1:]})
		}
	}
	sort.Slice(shotList, func(i, j int) bool { return shotList[i].step < shotList[j].step })
	missingSeen := map[rune]bool{}
	phantasie.Capture(o, img, func(e phantasie.TextEvent) {
		tr, ok := cat[string(e.Text)]
		if !ok {
			return
		}
		for _, s := range stamps(font, string(e.Text), e.Col, e.Row, len(e.Text), tr) {
			layer.Add(s)
		}
	})
	next := 0
	for o.Steps() < *steps {
		run(o.Steps() + *every)
		idx := o.CGA4()
		layer.Frame(idx, rgbFrame(idx))
		for next < len(shotList) && o.Steps() >= shotList[next].step {
			if err := writePNG(shotList[next].path, compose(layer, idx, *scale, func(r rune) { missingSeen[r] = true })); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Printf("# 步 %d 存 %s（疊字 %d 筆）\n", o.Steps(), shotList[next].path, len(layer.Stamps))
			next++
		}
	}
	fmt.Printf("# 結束於步 %d，疊字 %d 筆，缺字 %d\n", o.Steps(), len(layer.Stamps), len(missingSeen))
}
