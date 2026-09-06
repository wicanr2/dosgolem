// Command shots 跑一段鍵序，每一步存一張畫面：色號陣列（對拍用）與
// PNG（給人看）。給「remake 的畫面與原版對不對得上」這類問題用。
//
// **走位靠程式自己的兩個訊號**——「讀走了幾個鍵」與「畫面連續多少道指令
// 沒動」——不靠 sleep。這是 dosgolem 存在的理由：外面那一套只能拿牆上的
// 時鐘猜，而猜錯與猜對在畫面上長得一樣（`docs/spec/005`）。
//
// 這支是通用的，不含任何一支程式的位址；鍵序由 `-keys` 給。
//
//	tools/go.sh run ./cmd/shots -exe /orig/start.exe -root /orig \
//	  -out /src/workplace/shots -keys "rep:9:Space,Return,Return,c"
//
// 色號輸出（`.idx`，320×200 一格一個位元組）才是對拍的依據：PNG 經過
// 調色盤，而調色盤可能在動（`docs/spec/005` §3.3）。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

type shotInfo struct {
	Index   int    `json:"index"`
	Label   string `json:"label"`
	Step    uint64 `json:"step"`
	NonZero int    `json:"non_zero"`
	Digest  string `json:"sha256"`
	Path    string `json:"path"`
}

func main() {
	exe := flag.String("exe", "", "MZ 執行檔")
	root := flag.String("root", ".", "素材目錄（唯讀）")
	scratch := flag.String("scratch", "", "可寫的暫存層；程式存檔會落在這裡（docs/spec/009）")
	budget := flag.Uint64("budget", 60_000_000, "每一步最多跑幾道指令")
	idle := flag.Uint64("idle", 3_000_000, "畫面連續這麼多道指令沒動就算穩定")
	out := flag.String("out", "", "輸出目錄")
	script := flag.String("keys", "", "鍵序，逗號分隔：Return、Space、c、type:HERO、rep:18:Space")
	flag.Parse()

	img, err := os.ReadFile(*exe)
	if err != nil {
		fmt.Println("讀不到執行檔：", err)
		os.Exit(1)
	}
	m := machine.New()
	if err := m.LoadEXE(img); err != nil {
		fmt.Println("載入失敗：", err)
		os.Exit(1)
	}
	d := dos.New(m, *root)
	d.Scratch = *scratch
	d.Install()

	calls := map[string]int{}
	inner := m.CPU.IntHook
	m.CPU.IntHook = func(c *cpu.CPU, n uint8) bool {
		calls[fmt.Sprintf("int %02Xh AH=%02X", n, c.R[cpu.AX]>>8)]++
		return inner(c, n)
	}

	if *out != "" {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	var shots []shotInfo
	save := func(label string, frame []uint8) {
		if *out == "" {
			return
		}
		name := fmt.Sprintf("%02d-%s", len(shots), label)
		if err := os.WriteFile(filepath.Join(*out, name+".idx"), frame, 0o644); err != nil {
			fmt.Println("寫色號失敗：", err)
			return
		}
		if err := writePNG(filepath.Join(*out, name+".png"), frame); err != nil {
			fmt.Println("寫 PNG 失敗：", err)
			return
		}
		nz := 0
		for _, v := range frame {
			if v != 0 {
				nz++
			}
		}
		sum := sha256.Sum256(frame)
		shots = append(shots, shotInfo{len(shots), label, m.Steps, nz, fmt.Sprintf("%x", sum), name + ".png"})
		fmt.Printf("  → %s：step %d 非零 %d\n", name, m.Steps, nz)
	}

	// settle 跑到畫面連續 idle 道指令沒變為止，或用完 budget。
	// 回傳「有沒有真的靜下來」——**逾時與靜止在畫面上一模一樣**，
	// 不分開的話一張拍到一半的圖會被當成穩定畫面收進清冊。
	settle := func() ([]uint8, bool) {
		last := m.IndexedEGA()
		lastChange := m.Steps
		deadline := m.Steps + *budget
		for m.Steps < deadline {
			for i := 0; i < 100_000; i++ {
				if err := m.Step(); err != nil {
					fmt.Println("  停下：", err)
					return m.IndexedEGA(), false
				}
			}
			frame := m.IndexedEGA()
			if !same(last, frame) {
				last, lastChange = frame, m.Steps
				continue
			}
			if m.Steps-lastChange >= *idle {
				return frame, true
			}
		}
		return m.IndexedEGA(), false
	}

	fmt.Println("開機到第一個穩定畫面…")
	frame, ok := settle()
	if !ok {
		fmt.Println("  ⚠ 畫面沒有靜下來（用完 budget）")
	}
	save("boot", frame)

	for _, item := range parseScript(*script) {
		before := d.KeysConsumed
		switch {
		case strings.HasPrefix(item, "type:"):
			text := strings.TrimPrefix(item, "type:")
			if !d.PushText(text) {
				fmt.Printf("鍵盤表沒有 %q 裡的某個字元\n", text)
				os.Exit(1)
			}
		default:
			if !d.PushKeyNamed(item) {
				if !d.PushText(item) {
					fmt.Printf("不認得按鍵 %q\n", item)
					os.Exit(1)
				}
			}
		}
		want := len(d.Keys)
		fmt.Printf("送 %s（佇列 %d）\n", item, want)
		frame, ok := settle()
		if !ok {
			fmt.Println("  ⚠ 畫面沒有靜下來")
		}
		if d.KeysConsumed == before {
			fmt.Printf("  ⚠ 程式一個鍵都沒讀走（佇列還有 %d）\n", len(d.Keys))
		}
		save(strings.NewReplacer(":", "-", " ", "_").Replace(item), frame)
	}

	fmt.Printf("\n跑了 %d 道指令，CS:IP=%04X:%04X，鍵讀走 %d、還剩 %d\n",
		m.Steps, m.CPU.Seg[cpu.CS], m.CPU.IP, d.KeysConsumed, len(d.Keys))
	keys := make([]string, 0, len(calls))
	for k := range calls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println("服務呼叫統計：")
	for _, k := range keys {
		fmt.Printf("  %-18s %d\n", k, calls[k])
	}
	fmt.Println("開過的檔：", len(d.Opened))

	// 顯示相關埠的寫入統計。**「這支程式沒用到圖形控制器」這種結論會過期**
	// ——量的時候程式還沒畫到那一段而已，所以每一輪都重新數。
	video := map[uint16]string{
		0x3C4: "序列器索引", 0x3C5: "序列器資料",
		0x3CE: "圖控索引", 0x3CF: "圖控資料",
		0x3C0: "屬性", 0x3C2: "雜項輸出",
		0x3D4: "CRTC 索引", 0x3D5: "CRTC 資料",
	}
	portCounts := map[uint16]int{}
	portValues := map[uint16]map[uint8]int{}
	for _, w := range m.PortLog {
		if _, ok := video[w.Port]; !ok {
			continue
		}
		portCounts[w.Port]++
		if portValues[w.Port] == nil {
			portValues[w.Port] = map[uint8]int{}
		}
		portValues[w.Port][w.Val]++
	}
	ports := make([]int, 0, len(portCounts))
	for p := range portCounts {
		ports = append(ports, int(p))
	}
	sort.Ints(ports)
	fmt.Println("顯示相關埠的寫入：")
	for _, p := range ports {
		port := uint16(p)
		vals := make([]int, 0, len(portValues[port]))
		for v := range portValues[port] {
			vals = append(vals, int(v))
		}
		sort.Ints(vals)
		fmt.Printf("  %03X %-12s %d 次，值 %v\n", p, video[port], portCounts[port], vals)
	}

	if *out != "" {
		manifest, _ := json.MarshalIndent(shots, "", "  ")
		os.WriteFile(filepath.Join(*out, "shots.json"), append(manifest, '\n'), 0o644)
	}
}

// parseScript 把鍵序展開。`rep:18:Space` 是同一個鍵按 18 次。
func parseScript(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "rep:") {
			parts := strings.SplitN(item, ":", 3)
			if len(parts) == 3 {
				n, err := strconv.Atoi(parts[1])
				if err == nil {
					for i := 0; i < n; i++ {
						out = append(out, parts[2])
					}
					continue
				}
			}
		}
		out = append(out, item)
	}
	return out
}

func same(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func writePNG(path string, frame []uint8) error {
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, ega16[frame[y*320+x]&0x0F])
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

var ega16 = [16]color.RGBA{
	{0, 0, 0, 255}, {0, 0, 170, 255}, {0, 170, 0, 255}, {0, 170, 170, 255},
	{170, 0, 0, 255}, {170, 0, 170, 255}, {170, 85, 0, 255}, {170, 170, 170, 255},
	{85, 85, 85, 255}, {85, 85, 255, 255}, {85, 255, 85, 255}, {85, 255, 255, 255},
	{255, 85, 85, 255}, {255, 85, 255, 255}, {255, 255, 85, 255}, {255, 255, 255, 255},
}
