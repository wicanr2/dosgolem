// Command strikecommander-census 是 OPTTEST 文字路徑的動態普查工具（strike_commander M1）。
//
// 它從 probe 狀態檔啟動 session，依腳本在指定步數送滑鼠移動、按下、放開，並在整行輸出
// puts（RE 011 §2）入口記下字串、輸出物件的游標與 BP 鏈上的呼叫端。只讀原版狀態；
// 結果是研究證據，不是覆繪規格。
//
//	strikecommander-census -root DIR -state FILE -state-sha256 HEX -script FILE -out DIR
//
// 腳本每列一個指令，座標是 DOS 邏輯像素，步數是絕對步數：
//
//	move STEP X Y
//	down STEP X Y
//	up   STEP X Y
//	grid FROM X0 Y0 X1 Y1 DX DY DWELL   從 FROM 起每 DWELL 步移到下一格（列優先）
//	shot STEP NAME                      存最後一次回掃的畫格（NAME.png）
//	trace FROM TO                       FROM 到 TO 之間依序記下每次 puts 與 blit（RE 012 的 3C3C:0000）入口，寫進 trace.tsv
//	dump STEP SEG:OFF LEN NAME          在 STEP 之後的第一步把 SEG:OFF 起 LEN bytes 存成 NAME.bin（十六進位位址與長度）
//	stop STEP
package main

import (
	"bufio"
	"encoding/hex"
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

	"github.com/wicanr2/dosgolem/apps/strikecommander"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

// putsSig 是 OPTTEST 整行輸出的前 16 bytes（RE 011 §2，檔案位移 3F36Bh）。
var putsSig = []byte{0x55, 0x8B, 0xEC, 0x83, 0xEC, 0x06, 0x56, 0x57, 0x8B, 0x76, 0x06, 0x8B, 0x7E, 0x08, 0xFF, 0x74}

// blitSegOff 是 blit（RE 012 §2，3C3C:0000）相對 base 的段偏移。
const blitSegOff = 0x39FB

// putsSegOff 是 puts 相對載入段 base 的段偏移（3B83 − base；base = 3B83 − 3942h）。
const putsSegOff, putsIP = 0x3942, 0x034B

type event struct {
	step uint64
	kind string // move、down、up、shot、stop
	x, y int
	name string
}

type callKey struct {
	chain string
	text  string
}

type callStat struct {
	count       int
	first, last uint64
	cx, cy      uint16
	obj, font   uint16
}

// dumpReq 是待做的記憶體傾印；BeforeStep 在步數到了之後的第一步讀取。
type dumpReq struct {
	step     uint64
	seg, off uint16
	n        int
	name     string
	data     []byte
}

type observer struct {
	base      uint16
	baseKnown bool
	sigOK     bool
	stats     map[callKey]*callStat
	order     []callKey
	frame     []byte
	palette   [256][3]uint8
	calls     int
	dumps     []*dumpReq
	traceFrom uint64
	traceTo   uint64
	trace     []string
}

func (o *observer) VideoWrite(machine.VideoWrite) {}

func (o *observer) Frame(indexed []byte, palette [256][3]uint8) {
	o.frame, o.palette = indexed, palette
}

func rd16(v session.StepView, seg, off uint16) uint16 {
	a := uint32(seg)<<4 + uint32(off)
	return uint16(v.Read8(a)) | uint16(v.Read8(a+1))<<8
}

func (o *observer) BeforeStep(v session.StepView) error {
	for _, d := range o.dumps {
		if d.data == nil && v.Steps() >= d.step {
			d.data = make([]byte, d.n)
			base := uint32(d.seg)<<4 + uint32(d.off)
			for i := range d.data {
				d.data[i] = v.Read8(base + uint32(i))
			}
		}
	}
	if o.baseKnown && v.IP() == 0 && v.CS() == o.base+blitSegOff && v.Steps() >= o.traceFrom && v.Steps() < o.traceTo {
		o.trace = append(o.trace, fmt.Sprintf("%d\tblit\t\t", v.Steps()))
	}
	if v.IP() != putsIP {
		return nil
	}
	cs := v.CS()
	if !o.baseKnown {
		if cs < putsSegOff {
			return nil
		}
		ok := true
		for i, b := range putsSig {
			if v.Read8(uint32(cs)<<4+putsIP+uint32(i)) != b {
				ok = false
				break
			}
		}
		if !ok {
			return nil
		}
		o.base, o.baseKnown, o.sigOK = cs-putsSegOff, true, true
	}
	if cs != o.base+putsSegOff {
		return nil
	}
	o.calls++
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	retIP, retCS := rd16(v, ss, sp), rd16(v, ss, sp+2)
	obj, str := rd16(v, ss, sp+4), rd16(v, ss, sp+6)
	chain := []string{o.rel(retCS, retIP)}
	bp := v.BP()
	for d := 0; d < 4 && bp != 0; d++ {
		ip, seg := rd16(v, ss, bp+2), rd16(v, ss, bp+4)
		chain = append(chain, o.rel(seg, ip))
		nbp := rd16(v, ss, bp)
		if nbp <= bp {
			break
		}
		bp = nbp
	}
	var sb strings.Builder
	for i := uint16(0); i < 160; i++ {
		c := v.Read8(uint32(ds)<<4 + uint32(str+i))
		if c == 0 {
			break
		}
		switch {
		case c == '\\':
			sb.WriteString(`\\`)
		case c >= 0x20 && c < 0x7F:
			sb.WriteByte(c)
		default:
			fmt.Fprintf(&sb, `\x%02X`, c)
		}
	}
	k := callKey{strings.Join(chain, "<"), sb.String()}
	if v.Steps() >= o.traceFrom && v.Steps() < o.traceTo {
		o.trace = append(o.trace, fmt.Sprintf("%d\tputs\t%s\t%s", v.Steps(), chain[0], k.text))
	}
	st := o.stats[k]
	if st == nil {
		st = &callStat{first: v.Steps(), cx: rd16(v, ds, obj+0x10), cy: rd16(v, ds, obj+0x12), obj: obj, font: rd16(v, ds, obj+0x14)}
		o.stats[k] = st
		o.order = append(o.order, k)
	}
	st.count++
	st.last = v.Steps()
	return nil
}

// rel 把段寫成相對 base 的段偏移，例如 0CF2:082A 在 base 0241 時是 +0AB1:082A。
func (o *observer) rel(seg, off uint16) string {
	if seg >= o.base && seg-o.base < 0x8000 {
		return fmt.Sprintf("+%04X:%04X", seg-o.base, off)
	}
	return fmt.Sprintf("%04X:%04X", seg, off)
}

func parseScript(path string) ([]event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var evs []event
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fs := strings.Fields(line)
		num := func(i int) (uint64, error) {
			if i >= len(fs) {
				return 0, fmt.Errorf("第 %d 列欄位不足", n)
			}
			return strconv.ParseUint(fs[i], 10, 64)
		}
		switch fs[0] {
		case "move", "down", "up":
			s, e1 := num(1)
			x, e2 := num(2)
			y, e3 := num(3)
			if e1 != nil || e2 != nil || e3 != nil {
				return nil, fmt.Errorf("第 %d 列：%q", n, line)
			}
			evs = append(evs, event{step: s, kind: fs[0], x: int(x), y: int(y)})
		case "grid":
			var a [8]uint64
			for i := range a {
				if a[i], err = num(i + 1); err != nil {
					return nil, err
				}
			}
			from, x0, y0, x1, y1, dx, dy, dwell := a[0], a[1], a[2], a[3], a[4], a[5], a[6], a[7]
			if dx == 0 || dy == 0 || dwell == 0 {
				return nil, fmt.Errorf("第 %d 列：grid 的間距與停留步數不得為 0", n)
			}
			s := from
			for y := y0; y <= y1; y += dy {
				for x := x0; x <= x1; x += dx {
					evs = append(evs, event{step: s, kind: "move", x: int(x), y: int(y)})
					s += dwell
				}
			}
		case "shot":
			s, err := num(1)
			if err != nil || len(fs) < 3 {
				return nil, fmt.Errorf("第 %d 列：%q", n, line)
			}
			evs = append(evs, event{step: s, kind: "shot", name: fs[2]})
		case "trace":
			a, e1 := num(1)
			b, e2 := num(2)
			if e1 != nil || e2 != nil || b <= a {
				return nil, fmt.Errorf("第 %d 列：%q", n, line)
			}
			evs = append(evs, event{step: a, kind: "trace", name: strconv.FormatUint(b, 10)})
		case "dump":
			s, err := num(1)
			if err != nil || len(fs) < 5 {
				return nil, fmt.Errorf("第 %d 列：%q", n, line)
			}
			so := strings.SplitN(fs[2], ":", 2)
			if len(so) != 2 {
				return nil, fmt.Errorf("第 %d 列：位址要 SEG:OFF", n)
			}
			sg, e1 := strconv.ParseUint(so[0], 16, 16)
			of, e2 := strconv.ParseUint(so[1], 16, 16)
			ln, e3 := strconv.ParseUint(fs[3], 16, 32)
			if e1 != nil || e2 != nil || e3 != nil || ln == 0 || ln > 0x100000 {
				return nil, fmt.Errorf("第 %d 列：%q", n, line)
			}
			evs = append(evs, event{step: s, kind: "dump", x: int(sg), y: int(of), name: fs[4] + ":" + strconv.FormatUint(ln, 10)})
		case "stop":
			s, err := num(1)
			if err != nil {
				return nil, fmt.Errorf("第 %d 列：%q", n, line)
			}
			evs = append(evs, event{step: s, kind: "stop"})
		default:
			return nil, fmt.Errorf("第 %d 列：不認得的指令 %q", n, fs[0])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].step < evs[j].step })
	return evs, nil
}

func writePNG(path string, idx []byte, pal [256][3]uint8) error {
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for i := 0; i < 320*200 && i < len(idx); i++ {
		c := pal[idx[i]]
		img.Set(i%320, i/320, color.RGBA{c[0], c[1], c[2], 255})
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func main() {
	root := flag.String("root", "", "SB16 設定樹（唯讀來源，複製到 -work 下當 SaveRoot）")
	statePath := flag.String("state", "", "probe -save-state 狀態檔")
	stateSHA := flag.String("state-sha256", "", "狀態檔 SHA-256")
	rebase := flag.String("rebase-from", "/scratch", "狀態檔開檔路徑的舊前綴，逗號分隔")
	script := flag.String("script", "", "腳本檔")
	out := flag.String("out", "", "輸出目錄")
	work := flag.String("work", os.TempDir(), "放 SaveRoot 複本的目錄")
	flag.Parse()
	if err := run(*root, *statePath, *stateSHA, *rebase, *script, *out, *work); err != nil {
		fmt.Fprintln(os.Stderr, "strikecommander-census：", err)
		os.Exit(1)
	}
}

func run(root, statePath, stateSHA, rebase, script, out, work string) error {
	if root == "" || statePath == "" || stateSHA == "" || script == "" || out == "" {
		return fmt.Errorf("-root、-state、-state-sha256、-script、-out 都要給")
	}
	evs, err := parseScript(script)
	if err != nil {
		return err
	}
	shaBytes, err := hex.DecodeString(stateSHA)
	if err != nil || len(shaBytes) != 32 {
		return fmt.Errorf("-state-sha256 要 64 個十六進位字")
	}
	var sha [32]byte
	copy(sha[:], shaBytes)
	state, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	saveRoot, err := os.MkdirTemp(work, "census-root-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(saveRoot)
	if err := strikecommander.CopyTree(root, saveRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	obs := &observer{stats: map[callKey]*callStat{}}
	for _, e := range evs {
		if e.kind == "trace" {
			obs.traceFrom = e.step
			obs.traceTo, _ = strconv.ParseUint(e.name, 10, 64)
		}
		if e.kind == "dump" {
			i := strings.LastIndex(e.name, ":")
			n, _ := strconv.Atoi(e.name[i+1:])
			obs.dumps = append(obs.dumps, &dumpReq{step: e.step, seg: uint16(e.x), off: uint16(e.y), n: n, name: e.name[:i]})
		}
	}
	o, err := session.New(strikecommander.SessionConfig(obs))
	if err != nil {
		return err
	}
	defer o.Close()
	var rebaseFrom []string
	for _, p := range strings.Split(rebase, ",") {
		if p = strings.TrimSpace(p); p != "" {
			rebaseFrom = append(rebaseFrom, p)
		}
	}
	boot, err := o.BootFromState(session.StateInput{State: state, ExpectedStateSHA256: sha, SaveRoot: saveRoot, RebaseFrom: rebaseFrom})
	if err != nil {
		return err
	}
	fmt.Printf("狀態檔 %s，啟動步數 %d\n", statePath, boot.Steps)
	steps := boot.Steps
	deliver := func(u session.CapturedUpdate) (session.InputReceipt, error) {
		v, err := o.View()
		if err != nil {
			return session.InputReceipt{}, err
		}
		u.StartedPanel, u.Layout, u.SourceGeneration = v.Panel, v.Layout, v.SourceGeneration
		return o.Deliver(u)
	}
	advanceTo := func(target uint64) error {
		if target <= steps {
			return nil
		}
		if _, err := deliver(session.CapturedUpdate{}); err != nil {
			return err
		}
		r, err := o.Advance(session.InstructionBudget(target - steps))
		steps = r.MachineStepsAfter
		if err != nil {
			return fmt.Errorf("Advance 停在第 %d 步：%w", steps, err)
		}
		return nil
	}
	pointer := func(kind host.MouseEventKind, x, y int) host.MouseEvent {
		v, _ := o.View()
		l := v.Layout
		return host.MouseEvent{Kind: kind, X: x * int(l.Scale), Y: l.ChromeHeight + y*int(l.Scale), Target: host.MouseTargetCanvas}
	}
	stopped := false
	for _, e := range evs {
		if e.step < steps {
			return fmt.Errorf("事件步數 %d 早於目前第 %d 步", e.step, steps)
		}
		if err := advanceTo(e.step); err != nil {
			return err
		}
		switch e.kind {
		case "move":
			mv := pointer(host.MouseEventMove, e.x, e.y)
			_, err = deliver(session.CapturedUpdate{PointerMoves: []host.MouseEvent{mv}})
		case "down":
			d := pointer(host.MouseEventDown, e.x, e.y)
			_, err = deliver(session.CapturedUpdate{PointerDown: &d})
		case "up":
			u := pointer(host.MouseEventUp, e.x, e.y)
			_, err = deliver(session.CapturedUpdate{PointerUp: &u})
		case "shot":
			if obs.frame == nil {
				fmt.Printf("第 %d 步 shot %s：還沒有回掃畫格\n", steps, e.name)
				continue
			}
			err = writePNG(filepath.Join(out, e.name+".png"), obs.frame, obs.palette)
		case "dump", "trace":
			// 由觀察器在之後的步讀取或記錄。
		case "stop":
			stopped = true
		}
		if err != nil {
			return fmt.Errorf("第 %d 步 %s：%w", e.step, e.kind, err)
		}
		if stopped {
			break
		}
		if e.kind == "move" || e.kind == "down" || e.kind == "up" {
			// 交付後要 Advance 才能再 Deliver；推進 1 步，維持事件之間的步數。
			r, err := o.Advance(1)
			steps = r.MachineStepsAfter
			if err != nil {
				return fmt.Errorf("Advance 停在第 %d 步：%w", steps, err)
			}
		}
	}
	for _, d := range obs.dumps {
		if d.data == nil {
			fmt.Printf("dump %s：第 %d 步之後沒有再執行，未傾印\n", d.name, d.step)
			continue
		}
		if err := os.WriteFile(filepath.Join(out, d.name+".bin"), d.data, 0o644); err != nil {
			return err
		}
	}
	if len(obs.trace) > 0 {
		tr := "step\tkind\tcaller\ttext\n" + strings.Join(obs.trace, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(out, "trace.tsv"), []byte(tr), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("停在第 %d 步；puts 呼叫 %d 次，base %04X（簽章 %v），不同（呼叫鏈、字串）%d 組\n",
		steps, obs.calls, obs.base, obs.sigOK, len(obs.order))
	f, err := os.Create(filepath.Join(out, "census.tsv"))
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "chain\ttext\tcount\tfirst\tlast\tcursor_x\tcursor_y\tobj\tfont")
	for _, k := range obs.order {
		s := obs.stats[k]
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%04X\t%04X\n", k.chain, k.text, s.count, s.first, s.last, s.cx, s.cy, s.obj, s.font)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
