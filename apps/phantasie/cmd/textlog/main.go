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
	vram := flag.Bool("vram-callers", false, "統計寫視訊記憶體的呼叫端（映像偏移）")
	ops := flag.Bool("ops", false, "記錄畫面常式的呼叫（反白、整頁存取、視窗…）")
	keys := flag.String("keys", "", "送鍵：步數:鍵名[,...]，鍵名見 oracle.SendKeys（Return、Down、Esc…）")
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
	callers := map[uint16][2]int{}
	if *vram {
		phantasie.CaptureVideoWrites(o, img, func(w phantasie.VideoWrite) {
			c := callers[w.Caller]
			c[0]++
			c[1] += int(w.Count)
			callers[w.Caller] = c
		})
	}
	for _, k := range ks {
		gate.Press(k.step, k.name)
	}
	phantasie.Capture(o, img, func(e phantasie.TextEvent) {
		ov := e.Overlay
		if ov == "" {
			ov = "-"
		}
		fmt.Printf("T step=%d r=%d c=%d caller=%04X ov=%s fmt=%q text=%q\n", e.Step, e.Row, e.Col, e.Caller, ov, e.Format, e.Text)
	})
	if err := o.RunUntil(atLeast(*steps), oracle.Budget(*steps)); err != nil {
		fmt.Fprintln(os.Stderr, "收尾：", err)
	}
	fmt.Printf("# 結束於步 %d，鍵閘已送 %d、尚餘 %d，BIOS 鍵盤佇列待讀 %d、已讀 %d\n", o.Steps(), gate.Gated, gate.Pending(), o.KeysPending(), o.KeysConsumed())
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
