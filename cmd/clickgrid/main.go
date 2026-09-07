// clickgrid 從一份狀態檔展開，逐格點一次，回報哪一格會改變畫面。
//
//	go run ./cmd/clickgrid -state ck-cmd.gz -root /orig \
//	    -area 292,48,256,168 -step 16 -settle 4000000
//
// **每一格都從同一份狀態重新展開**，所以格與格之間互不影響——
// 這是 `probe -sweep` 做不到的（那支是連續點下去，狀態會累積）。
// 有了狀態檔之後這件事才便宜：一格約一秒，而從頭跑到那個畫面要八分鐘。
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

func main() {
	st := flag.String("state", "", "狀態檔（`probe -save-state` 存的，必填）")
	root := flag.String("root", ".", "原版素材目錄")
	area := flag.String("area", "", "掃描範圍 `x,y,寬,高`（畫格座標，必填）")
	step := flag.Int("step", 16, "格距")
	hold := flag.Uint64("hold", 3_000_000, "按住幾道指令（-polls 給了就以 -polls 為準）")
	polls := flag.Int("polls", 0,
		"按住到遊戲讀了幾次滑鼠就放開（0 ＝ 用 -hold）。"+
			"**對話框裡要用這個**：那些畫面每兩千道指令問一次滑鼠，"+
			"固定按住幾百萬道等於連點好幾下，一路穿過好幾層選單")
	settle := flag.Uint64("settle", 6_000_000, "點完再跑幾道指令才看畫面")
	out := flag.String("out", "", "把有變化的那幾格畫面存到這個目錄")
	memSpec := flag.String("mem", "",
		"每一格點完之後把一段線性記憶體寫出來：`<lo>-<hi>:<目錄>`（位址十六進位）。"+
			"檔名是 `x_y.bin`，基準是 `base.bin`。指令的效果要看記憶體差分，"+
			"不是看畫面")
	only := flag.String("only", "", "只點這幾個點：`x,y;x,y…`（給了就不掃格）")
	premove := flag.Int("premove", 0,
		"先把游標移過去、等遊戲讀了幾次滑鼠，才按下去（0 ＝ 移到就按）。"+
			"有些對話框的鈕吃「游標已經在上面」這個狀態，瞬移過去馬上按沒有反應")
	box := flag.Bool("box", false,
		"改印「與基準差幾點、差在哪個矩形」，不印雜湊。"+
			"**畫面會動的時候雜湊沒有用**（框線是跑的彩虹漸層，"+
			"每一格都不一樣），要看差在哪一塊才分得出真的有反應")
	flag.Parse()

	if *st == "" || (*area == "" && *only == "") {
		flag.Usage()
		os.Exit(2)
	}

	pts, err := points(*area, *only, *step)
	if err != nil {
		die(err)
	}

	var memLo, memHi uint32
	memDir := ""
	if *memSpec != "" {
		i := strings.LastIndex(*memSpec, ":")
		if i < 0 {
			die(fmt.Errorf("-mem 要寫成 <lo>-<hi>:<目錄>，收到 %q", *memSpec))
		}
		if _, err := fmt.Sscanf((*memSpec)[:i], "%x-%x", &memLo, &memHi); err != nil {
			die(fmt.Errorf("-mem 的位址解不開：%w", err))
		}
		memDir = (*memSpec)[i+1:]
		if err := os.MkdirAll(memDir, 0o755); err != nil {
			die(err)
		}
	}

	base, err := run(*st, *root, -1, -1, *hold, *settle, *polls, *premove, memLo, memHi, filepath.Join(memDir, "base.bin"), memDir != "")
	if err != nil {
		die(err)
	}
	fmt.Printf("基準畫面 %s（沒點任何地方）\n", base.sum[:16])
	if *out != "" {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			die(err)
		}
		os.WriteFile(filepath.Join(*out, "base.idx"), base.pix, 0o644)
	}

	seen := map[string]string{base.sum: "沒變"}
	n := 0
	for _, p := range pts {
		r, err := run(*st, *root, p.x, p.y, *hold, *settle, *polls, *premove,
			memLo, memHi, filepath.Join(memDir, fmt.Sprintf("%d_%d.bin", p.x, p.y)), memDir != "")
		if err != nil {
			die(err)
		}
		if *box {
			cnt, x0, y0, x1, y1 := diff(base.pix, r.pix)
			if cnt == 0 {
				continue
			}
			n++
			fmt.Printf("(%3d,%3d) → 差 %6d 點，(%d,%d)-(%d,%d)\n", p.x, p.y, cnt, x0, y0, x1, y1)
			continue
		}
		if r.sum == base.sum {
			continue
		}
		n++
		tag, dup := seen[r.sum]
		if !dup {
			tag = fmt.Sprintf("畫面%02d", len(seen))
			seen[r.sum] = tag
			if *out != "" {
				os.WriteFile(filepath.Join(*out, tag+".idx"), r.pix, 0o644)
				os.WriteFile(filepath.Join(*out, tag+".pal"), r.pal, 0o644)
			}
		}
		fmt.Printf("(%3d,%3d) → %s %s\n", p.x, p.y, r.sum[:16], tag)
	}
	fmt.Printf("\n%d 格裡 %d 格有反應，%d 種不同的畫面\n", len(pts), n, len(seen)-1)
}

type shot struct {
	sum      string
	pix, pal []byte
}

// run 展開狀態、點一下、跑一段，回畫面。x < 0 表示不點（基準）。
func run(stPath, root string, x, y int, hold, settle uint64, polls, premove int,
	memLo, memHi uint32, memPath string, wantMem bool) (*shot, error) {
	m := machine.New()
	d := dos.New(m, root)
	d.Install()
	if err := state.Load(stPath, m, d); err != nil {
		return nil, err
	}
	start := m.Steps
	pollsAtPress := 0
	held, waiting := false, false
	if x >= 0 {
		d.Mouse.X, d.Mouse.Y = uint16(x), uint16(y)
		if premove > 0 {
			waiting = true
			pollsAtPress = len(d.Mouse.Polls)
		} else {
			d.Mouse.Buttons = 1
			d.Mouse.Press++
			pollsAtPress = len(d.Mouse.Polls)
			held = true
		}
	}
	for m.Steps < start+hold+settle && !m.CPU.Halted && !d.Exited {
		if waiting && len(d.Mouse.Polls)-pollsAtPress >= premove {
			d.Mouse.Buttons = 1
			d.Mouse.Press++
			pollsAtPress = len(d.Mouse.Polls)
			waiting, held = false, true
		}
		if held {
			done := m.Steps >= start+hold
			if polls > 0 {
				done = len(d.Mouse.Polls)-pollsAtPress >= polls
			}
			if done {
				d.Mouse.Buttons = 0
				d.Mouse.Release++
				held = false
			}
		}
		if err := m.Step(); err != nil {
			return nil, err
		}
	}
	if wantMem {
		if memHi > uint32(len(m.Mem)) {
			memHi = uint32(len(m.Mem))
		}
		if err := os.WriteFile(memPath, m.Mem[memLo:memHi], 0o644); err != nil {
			return nil, err
		}
	}
	pix := m.Indexed()
	sum := fmt.Sprintf("%x", sha256.Sum256(pix))
	flat := make([]byte, 0, 768)
	for _, c := range m.Palette() {
		flat = append(flat, c[0], c[1], c[2])
	}
	return &shot{sum: sum, pix: pix, pal: flat}, nil
}

// diff 回「差幾點」與差異的外接矩形。
func diff(a, b []uint8) (n, x0, y0, x1, y1 int) {
	const w = 640
	x0, y0, x1, y1 = 1<<30, 1<<30, -1, -1
	for i := range a {
		if i >= len(b) || a[i] == b[i] {
			continue
		}
		n++
		x, y := i%w, i/w
		if x < x0 {
			x0 = x
		}
		if y < y0 {
			y0 = y
		}
		if x > x1 {
			x1 = x
		}
		if y > y1 {
			y1 = y
		}
	}
	if n == 0 {
		return 0, 0, 0, 0, 0
	}
	return
}

type pt struct{ x, y int }

func points(area, only string, step int) ([]pt, error) {
	if only != "" {
		var out []pt
		for _, s := range strings.Split(only, ";") {
			var p pt
			if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d,%d", &p.x, &p.y); err != nil {
				return nil, fmt.Errorf("-only 要寫成 x,y;x,y…，收到 %q：%w", s, err)
			}
			out = append(out, p)
		}
		return out, nil
	}
	var x0, y0, w, h int
	if _, err := fmt.Sscanf(area, "%d,%d,%d,%d", &x0, &y0, &w, &h); err != nil {
		return nil, fmt.Errorf("-area 要寫成 x,y,寬,高：%w", err)
	}
	var out []pt
	for y := y0; y < y0+h; y += step {
		for x := x0; x < x0+w; x += step {
			out = append(out, pt{x, y})
		}
	}
	return out, nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
