// Command step 是「像玩家一樣」逐步操作：從狀態檔（或程式進入點）開始，
// 依動作腳本推進機器時間，存回狀態檔與畫面（`docs/spec/201`）。
//
// 現有的兩條路都不合適：probe 的 -press／-hold 以「第幾道指令」排程，要先
// 知道步數；前端（Ebiten）要常駐行程，代理的每次工具呼叫之間保留不住它。
// 這一支是給代理（或測試腳本）用的：看一眼畫面、決定按什麼、按多久、
// 等多久，再看下一眼——每一步是一次獨立的指令。
//
// ⚠ 本專案不含任何原版檔案，-exe 與 -root 都由玩家自備。
//
//	go run ./cmd/step -exe PW_UNP.EXE -root RICH2 -cycles 750 \
//	    -do "tap:Up:150,wait:500" -save-state a.state -shot a.png -scale 3
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/oracle"
)

func main() {
	exe := flag.String("exe", "", "要跑的執行檔（必填；MZ 或 .COM，看檔頭 magic 自動判斷）")
	root := flag.String("root", "", "原版素材目錄（必填）")
	loadState := flag.String("load-state", "", "從這個狀態檔接著跑（不給就從程式進入點開始）")
	cyclesSpec := flag.String("cycles", "750", "機器速度：DOSBox 相容的每毫秒 cycles（`docs/spec/198`），"+
		"或 xt（240）、at8（750）、at12（1510）")
	doSpec := flag.String("do", "", "動作腳本，逗號分隔（`docs/spec/201` §2.1；必填）")
	saveState := flag.String("save-state", "", "跑完把整台機器存成這個檔（不給就不存）")
	shot := flag.String("shot", "", "跑完把畫面以 nearest 放大存成 PNG（不給就不存）")
	scale := flag.Int("scale", 1, "-shot 的 nearest 放大倍率")
	scratch := flag.String("scratch", "", "可寫的暫存層目錄，程式存的檔落在這裡（`docs/spec/009`），"+
		"原版目錄永遠不動")
	flag.Parse()

	if *exe == "" || *root == "" || *doSpec == "" {
		flag.Usage()
		os.Exit(2)
	}

	perMs, err := parseCycles(*cyclesSpec)
	if err != nil {
		die(err)
	}

	res, err := runStep(stepConfig{
		exe: *exe, root: *root, loadState: *loadState, perMs: perMs,
		do: *doSpec, saveState: *saveState, shot: *shot, scale: *scale,
		scratch: *scratch,
	})
	if err != nil {
		die(err)
	}

	// 摘要（`docs/spec/201` §2.3）：步數、cycles 是絕對值（方便串下一步的
	// -load-state）；機器毫秒是**這一步**（-do 那段腳本）花掉的機器時間。
	fmt.Printf("步數 %d，cycles %d（本步 %d，機器時間 %.1f ms），視訊模式 %02Xh，按著的鍵 %v\n",
		res.steps, res.cycles, res.deltaCycles, res.ms, res.videoMode, res.held)
	if res.exited {
		fmt.Fprintln(os.Stderr, "程式在這一步裡自己結束了（int 21h AH=4Ch）")
	}
}

// stepConfig 是一次 step 呼叫的設定，拆出旗標解析方便測試用合成程式直接呼叫
// （`docs/spec/201` §3 第 5 項）。
type stepConfig struct {
	exe, root string
	loadState string
	perMs     uint64
	do        string
	saveState string
	shot      string
	scale     int
	scratch   string
}

// stepResult 是一次 step 呼叫的摘要。
type stepResult struct {
	steps, cycles uint64
	deltaCycles   uint64 // 這一步（RunActions 那段）跑掉的 cycles
	ms            float64
	videoMode     uint8
	held          []uint8
	exited        bool
}

// runStep 跑完一步：載入（＋可選的狀態檔）、設定速度、跑動作腳本、
// 存狀態與畫面、量出摘要。
func runStep(cfg stepConfig) (*stepResult, error) {
	acts, err := oracle.ParseActions(cfg.do)
	if err != nil {
		return nil, fmt.Errorf("-do 看不懂：%w", err)
	}

	o, err := oracle.Load(cfg.exe, cfg.root)
	if err != nil {
		return nil, err
	}
	defer o.Close()

	if cfg.scratch != "" {
		o.SetScratch(cfg.scratch)
	}
	if cfg.loadState != "" {
		if err := o.LoadStateFile(cfg.loadState); err != nil {
			return nil, fmt.Errorf("-load-state 讀不了：%w", err)
		}
	}
	// 機器速度放在載入狀態檔之後：狀態檔會還原先前的速度設定，
	// 命令列給的 -cycles 才是這一步真正要用的（與 cmd/probe 的順序一致）。
	o.SetDOSBoxCycles(cfg.perMs)

	startCycles := o.Cycles()
	exited := false
	if err := o.RunActions(acts, 0, nil); err != nil {
		if _, ok := err.(*oracle.ExitError); ok {
			exited = true
		} else {
			return nil, fmt.Errorf("動作跑到一半出錯：%w", err)
		}
	}

	if cfg.saveState != "" {
		if err := o.SaveStateFile(cfg.saveState); err != nil {
			return nil, fmt.Errorf("-save-state 存不了：%w", err)
		}
	}
	if cfg.shot != "" {
		if err := writeShotPNG(o, cfg.shot, cfg.scale); err != nil {
			return nil, fmt.Errorf("-shot 存不了：%w", err)
		}
	}

	delta := o.Cycles() - startCycles
	return &stepResult{
		steps: o.Steps(), cycles: o.Cycles(), deltaCycles: delta,
		ms:        float64(delta) / float64(cfg.perMs),
		videoMode: o.VideoMode(), held: o.HeldKeys(), exited: exited,
	}, nil
}

// writeShotPNG 把目前畫面（`ScreenRGB`）以 nearest 放大 scale 倍存成 PNG
// （`docs/spec/201` §2.3）。scale < 1 當成 1（不放大）。
func writeShotPNG(o *oracle.Oracle, path string, scale int) error {
	if scale < 1 {
		scale = 1
	}
	w, h, rgb := o.ScreenRGB()
	img := image.NewRGBA(image.Rect(0, 0, w*scale, h*scale))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			c := color.RGBA{R: rgb[i], G: rgb[i+1], B: rgb[i+2], A: 255}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.SetRGBA(x*scale+dx, y*scale+dy, c)
				}
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// parseCycles 解析 -cycles：每毫秒 cycles 的正整數，或 xt／at8／at12。
//
// 與 `cmd/probe` 的 `parseCycles` 同義——那一支在 `package main`，模組間
// import 不到，只好在這裡照抄一份。`internal/machine` 那邊的 CyclesXT／
// AT8／AT12 常數改了，兩邊都要跟著動。
func parseCycles(spec string) (uint64, error) {
	switch strings.ToLower(strings.TrimSpace(spec)) {
	case "xt":
		return machine.CyclesXT, nil
	case "at8":
		return machine.CyclesAT8, nil
	case "at12":
		return machine.CyclesAT12, nil
	}
	v, err := strconv.ParseUint(strings.TrimSpace(spec), 10, 64)
	if err != nil || v == 0 {
		return 0, fmt.Errorf("-cycles 看不懂：%q（要正整數或 xt／at8／at12）", spec)
	}
	return v, nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
