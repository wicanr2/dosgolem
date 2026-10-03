// Command textlog 跑原版的啟動鏈，記錄每一次繪字（欄、列、已格式化字串、呼叫端），
// 並在指定步數送鍵。證據探針：只讀，不改原版狀態。
//
//	textlog -root <原版目錄> -steps 30000000 -keys 'step:Return,step:Down'
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/phantasie"
	"github.com/wicanr2/dosgolem/oracle"
)

type keyAt struct {
	step uint64
	name string
}

func parseKeys(s string) ([]keyAt, error) {
	var out []keyAt
	for _, it := range strings.Split(s, ",") {
		if it = strings.TrimSpace(it); it == "" {
			continue
		}
		i := strings.Index(it, ":")
		if i < 0 {
			return nil, fmt.Errorf("要寫成 步數:鍵名：%q", it)
		}
		n, err := strconv.ParseUint(it[:i], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("步數看不懂：%q", it)
		}
		out = append(out, keyAt{n, it[i+1:]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].step < out[j].step })
	return out, nil
}

func main() {
	root := flag.String("root", "", "原版目錄（必填）")
	bat := flag.String("bat", "", "啟動批次檔名；空字串＝目錄內唯一的 .BAT")
	steps := flag.Uint64("steps", 30_000_000, "跑到第幾道指令（絕對步數）")
	pngPath := flag.String("png", "", "結束時把畫面存成 PNG（CGA 模式 4）")
	pal := flag.String("palette", "1h", "PNG 用的 CGA 調色盤：0、0h、1、1h")
	pages := flag.Bool("pages", false, "結束時印出頁面緩衝區變數 DS:5BAA、5BAC、5BAE 與 INT 61h 向量")
	spf := flag.Bool("sprintf", false, "記錄 sprintf 呼叫，並標出以 sprintf 緩衝區當格式字串的繪字事件")
	int10 := flag.Bool("int10", false, "記錄 INT 10h 呼叫（int86 入口的輸入暫存器）")
	vram := flag.Bool("vram-callers", false, "統計寫視訊記憶體的呼叫端（映像偏移）")
	ops := flag.Bool("ops", false, "記錄畫面常式的呼叫（反白、整頁存取、視窗…）")
	keys := flag.String("keys", "", "送鍵：步數:鍵名[,...]，鍵名見 oracle.SendKeys（Return、Down、Esc…）")
	route := flag.String("route", "", "按鍵路線檔：以空白分隔的鍵名，每次讀鍵送一個（單一字元走字元鍵）；與 -keys 並用時先送路線")
	args := flag.Bool("args", false, "每筆繪字附上 8 個格式字串之後的堆疊字組")
	flag.Parse()
	if *root == "" {
		flag.Usage()
		os.Exit(2)
	}
	ks, err := parseKeys(*keys)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	o, names, err := phantasie.Launch(*root, *bat)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer o.Close()
	fmt.Printf("# 啟動鏈：%s\n", strings.Join(names, " → "))

	// 先跑到最後一支程式載入，才知道映像段。
	want := len(names)
	atLeast := func(n uint64) oracle.Cond {
		return oracle.NewCond(fmt.Sprintf("步數 %d", n), func(o *oracle.Oracle) bool { return o.Steps() >= n })
	}
	loaded := oracle.NewCond("鏈載入完成", func(o *oracle.Oracle) bool { return len(o.ExecLog()) >= want-1 })
	if err := o.RunUntil(loaded, oracle.Budget(*steps)); err != nil {
		fmt.Fprintln(os.Stderr, "等鏈載入：", err)
		os.Exit(1)
	}
	img := phantasie.ImageSeg(o)
	fmt.Printf("# 映像段 %04X（步 %d）\n", img, o.Steps())
	gate := phantasie.NewKeyGate(o, img)
	if *ops {
		phantasie.CaptureOps(o, img, func(e phantasie.OpEvent) {
			fmt.Printf("O step=%d %s caller=%04X args=%04X\n", e.Step, e.Name, e.Caller, e.Args)
		})
	}
	if *int10 {
		phantasie.CaptureInt86(o, img, func(e phantasie.Int86Event) {
			if e.IntNo == 0x10 {
				fmt.Printf("B step=%d caller=%04X int10 ax=%04X bx=%04X cx=%04X dx=%04X\n", e.Step, e.Caller, e.AX, e.BX, e.CX, e.DX)
			}
		})
	}
	dests := map[uint16]string{}
	if *spf {
		phantasie.CaptureSprintf(o, img, func(e phantasie.SprintfEvent) {
			dests[e.Dest] = string(e.Fmt)
			fmt.Printf("S step=%d caller=%04X dest=%04X fmt=%q args=%04X\n", e.Step, e.Caller, e.Dest, e.Fmt, e.Args)
		})
	}
	callers := map[uint16][2]int{}
	if *vram {
		phantasie.CaptureVideoWrites(o, img, func(w phantasie.VideoWrite) {
			c := callers[w.Caller]
			c[0]++
			c[1] += int(w.Count)
			callers[w.Caller] = c
		})
	}
	if *route != "" {
		b, err := os.ReadFile(*route)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, line := range strings.Split(string(b), "\n") {
			if i := strings.Index(line, "#"); i >= 0 {
				line = line[:i]
			}
			for _, tok := range strings.Fields(line) {
				gate.Press(0, tok)
			}
		}
	}
	for _, k := range ks {
		gate.Press(k.step, k.name)
	}
	phantasie.Capture(o, img, func(e phantasie.TextEvent) {
		ov := e.Overlay
		if ov == "" {
			ov = "-"
		}
		if *spf {
			if f, ok := dests[e.FmtPtr]; ok {
				fmt.Printf("# composed-fmt %q\n", f)
			}
			for _, a := range e.Args {
				if f, ok := dests[a]; ok && a != 0 {
					fmt.Printf("# composed-arg %04X %q\n", a, f)
				}
			}
		}
		if *args {
			fmt.Printf("T step=%d r=%d c=%d caller=%04X ov=%s fmtptr=%04X fmt=%q text=%q args=%04X\n", e.Step, e.Row, e.Col, e.Caller, ov, e.FmtPtr, e.Format, e.Text, e.Args)
			return
		}
		fmt.Printf("T step=%d r=%d c=%d caller=%04X ov=%s fmt=%q text=%q\n", e.Step, e.Row, e.Col, e.Caller, ov, e.Format, e.Text)
	})
	if err := o.RunUntil(atLeast(*steps), oracle.Budget(*steps)); err != nil {
		fmt.Fprintln(os.Stderr, "收尾：", err)
	}
	fmt.Printf("# 結束於步 %d，鍵閘已送 %d、尚餘 %d，BIOS 鍵盤佇列待讀 %d、已讀 %d\n", o.Steps(), gate.Gated, gate.Pending(), o.KeysPending(), o.KeysConsumed())
	if *pages {
		dg := img + phantasie.DGroupParas
		for _, off := range []uint16{0x5BAA, 0x5BAC, 0x5BAE} {
			fmt.Printf("# DS:%04X = %04X\n", off, o.Word(oracle.Far(dg, off)))
		}
		fmt.Printf("# IVT 61h = %04X:%04X\n", o.Word(oracle.Far(0, 0x61*4+2)), o.Word(oracle.Far(0, 0x61*4)))
	}
	fmt.Printf("# 開過的檔：%s\n", strings.Join(o.Opened(), " "))
	if *vram {
		var ks []int
		for k := range callers {
			ks = append(ks, int(k))
		}
		sort.Ints(ks)
		for _, k := range ks {
			c := callers[uint16(k)]
			fmt.Printf("V caller=%04X 次數=%d 位元組=%d\n", k, c[0], c[1])
		}
	}
	for _, f := range o.FileOps() {
		fmt.Printf("# 檔案 %+v\n", f)
	}
	if *pngPath != "" {
		if err := writePNG(*pngPath, o.CGA4(), *pal, 2); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
