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
	hold := flag.Uint64("hold", 3_000_000, "按住幾道指令")
	settle := flag.Uint64("settle", 6_000_000, "點完再跑幾道指令才看畫面")
	out := flag.String("out", "", "把有變化的那幾格畫面存到這個目錄")
	only := flag.String("only", "", "只點這幾個點：`x,y;x,y…`（給了就不掃格）")
	flag.Parse()

	if *st == "" || (*area == "" && *only == "") {
		flag.Usage()
		os.Exit(2)
	}

	pts, err := points(*area, *only, *step)
	if err != nil {
		die(err)
	}

	base, err := run(*st, *root, -1, -1, *hold, *settle)
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
		r, err := run(*st, *root, p.x, p.y, *hold, *settle)
		if err != nil {
			die(err)
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
func run(stPath, root string, x, y int, hold, settle uint64) (*shot, error) {
	m := machine.New()
	d := dos.New(m, root)
	d.Install()
	if err := state.Load(stPath, m, d); err != nil {
		return nil, err
	}
	start := m.Steps
	if x >= 0 {
		d.Mouse.X, d.Mouse.Y = uint16(x), uint16(y)
		d.Mouse.Buttons = 1
		d.Mouse.Press++
	}
	for m.Steps < start+hold+settle && !m.CPU.Halted && !d.Exited {
		if x >= 0 && m.Steps == start+hold {
			d.Mouse.Buttons = 0
			d.Mouse.Release++
		}
		if err := m.Step(); err != nil {
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
