// Command k2boot 跑《大航海時代 II》的開機收據：片頭 → 防拷 → 標題 → 選主角 →
// 主迴圈，在每一步留比對點，輸出 JSON。
//
// 同一份輸入跑兩次，JSON 應逐位元組相同（`docs/spec/443-koukai2-oracle-adapter`）。
//
// ⚠ 本專案不含任何原版檔案，-root 由玩家自備。
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/wicanr2/dosgolem/apps/koukai2"
	"github.com/wicanr2/dosgolem/oracle"
)

// 片頭 12 個場景的入口（OPEN.EXE，koukai2 `docs/spec/063-opening`）。
var openScenes = []uint32{
	0x15348, 0x155CE, 0x15E7A, 0x1614E, 0x165AF, 0x16A58,
	0x16E0C, 0x17240, 0x17681, 0x17A7E, 0x17CB0, 0x18204,
}

type sceneMark struct {
	Scene int    `json:"scene"`
	Steps uint64 `json:"steps"`
	Ticks uint64 `json:"irq0_ticks"`
}

type dayMark struct {
	Steps    uint64 `json:"steps"`
	Clock    string `json:"clock_ds0532"`
	Saved    string `json:"clock_ds0caa"`
	Position uint16 `json:"word_48dfe"`
}

type exec struct {
	Name  string `json:"name"`
	PSP   uint16 `json:"psp"`
	Exit  uint8  `json:"exit"`
	Ended bool   `json:"ended"`
	TSR   bool   `json:"tsr"`
}

type receipt struct {
	Natural      bool                 `json:"natural_opening"`
	Checkpoints  []koukai2.Checkpoint `json:"checkpoints"`
	Scenes       []sceneMark          `json:"open_scenes"`
	TimerHz      float64              `json:"timer_hz"`
	TimerDivisor uint32               `json:"timer_divisor"`
	Execs        []exec               `json:"execs"`
	Opened       []string             `json:"opened"`
	Missing      []string             `json:"missing"`
	Unimpl       []string             `json:"unimplemented"`
	FirstDay     *dayMark             `json:"first_day"`
	Title        map[string]any       `json:"title"`
	RNG          map[string]any       `json:"rng"`
	SeedWrites   []koukai2.SeedWrite  `json:"seed_writes"`
	Password     map[string]any       `json:"password"`
}

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	scratch := flag.String("scratch", "", "可寫的暫存層")
	natural := flag.Bool("natural", false, "片頭不跳過，量各場景長度；跑到 MAIN.EXE 載入就停")
	out := flag.String("o", "", "JSON 輸出路徑（不給就印到 stdout）")
	shots := flag.String("shots", "", "每個比對點存 PNG 的目錄")
	save := flag.String("save-state", "", "跑到主迴圈後存狀態檔")
	flag.Parse()

	o, err := koukai2.Load(*root)
	if err != nil {
		die(err)
	}
	defer o.Close()
	if *scratch != "" {
		o.SetScratch(*scratch)
	}

	r := receipt{Natural: *natural}
	// 片頭場景入口：等 OPEN.EXE 載入才掛，避免踩到 KOUKAI2.COM 或 FMDRV.COM。
	if err := o.RunUntil(oracle.NewCond("OPEN.EXE 載入", func(o *oracle.Oracle) bool {
		b, ok := koukai2.Current(o)
		return ok && b.Name == "OPEN.EXE"
	}), oracle.Budget(100_000_000)); err != nil {
		die(err)
	}
	open, _ := koukai2.Current(o)
	for i, a := range openScenes {
		i, a := i, a
		o.OnCall(open.Code(a), func(o *oracle.Oracle) {
			if b, ok := koukai2.Current(o); ok && b.Name == "OPEN.EXE" {
				r.Scenes = append(r.Scenes, sceneMark{i + 1, o.Steps(), o.Ticks()})
			}
		})
	}

	if err := koukai2.ToMain(o, !*natural); err != nil {
		die(fmt.Errorf("到 MAIN.EXE：%w（畫面停在 %s）", err, koukai2.Describe(o)))
	}
	main_, ok := koukai2.Current(o)
	if !ok {
		die(fmt.Errorf("MAIN.EXE 不在 EXEC 紀錄"))
	}
	r.Checkpoints = append(r.Checkpoints, koukai2.Capture(o, "main-loaded"))
	rng := koukai2.WatchRNG(o, main_)
	koukai2.WatchSeedWriters(o, main_, &r.SeedWrites)

	cp := func(name string) {
		r.Checkpoints = append(r.Checkpoints, koukai2.Capture(o, name))
		if *shots != "" {
			if err := o.WritePNG(fmt.Sprintf("%s/%02d-%s.png", *shots, len(r.Checkpoints), name)); err != nil {
				die(err)
			}
		}
		if name == "title" {
			// 010：標題畫面時 DS:0CA4..8A18 應等於資料段初值，byte_48525 = 1。
			img, err := os.ReadFile(*root + "/MAIN.EXE")
			if err != nil {
				die(err)
			}
			lo, hi := int(koukai2.DSStateLo), int(koukai2.DSStateHi)
			fileBase := 0x3D350
			mem := o.Bytes(main_.DS(koukai2.DSStateLo), hi-lo)
			diff, first := 0, -1
			var runs [][2]int
			for i := range mem {
				if mem[i] != img[fileBase+lo+i] {
					diff++
					if first < 0 {
						first = lo + i
					}
					if n := len(runs); n > 0 && runs[n-1][1] == lo+i {
						runs[n-1][1] = lo + i + 1
					} else {
						runs = append(runs, [2]int{lo + i, lo + i + 1})
					}
				}
			}
			r.Title = map[string]any{
				"byte_48525":        o.Byte(main_.DS(koukai2.DSDiskFlag)),
				"state_diff_bytes":  diff,
				"state_diff_first":  first,
				"state_diff_ranges": runs,
				"seed_lo":           o.Word(main_.DS(koukai2.DSSeedLo)),
				"seed_hi":           o.Word(main_.DS(koukai2.DSSeedHi)),
			}
		}
	}
	// 063：防拷挑戰。`45DBC` 呼叫 `45DE0(百位, 十位, 個位)`，`[bp+var_A]` 是 r。
	o.OnCall(main_.Code(0x45DBC), func(o *oracle.Oracle) {
		if r.Password == nil {
			r.Password = map[string]any{
				"r":      o.Word(oracle.Addr{Seg: o.Regs().SS, Off: o.BP() - 0xA}),
				"digits": []uint16{o.StackWord(0), o.StackWord(1), o.StackWord(2)},
			}
		}
	})
	// 結果在 `45DCD`，被 `jmp short` 略過，所以只記 465F1 的回傳值。
	o.OnCall(main_.Code(0x45DCA), func(o *oracle.Oracle) {
		if r.Password != nil {
			if _, ok := r.Password["compare_ax"]; !ok {
				r.Password["compare_ax"] = o.AX()
			}
		}
	})
	// 主迴圈第一天：`sub_2B2BA` 第一次進入時的時鐘（010：DS:0CAA..0CAD 初值 15 04 10 18）。
	o.OnCall(main_.Code(koukai2.AddrOneDay), func(o *oracle.Oracle) {
		if r.FirstDay == nil {
			r.FirstDay = &dayMark{
				Steps:    o.Steps(),
				Clock:    hex.EncodeToString(o.Bytes(main_.DS(koukai2.DSClock), 4)),
				Saved:    hex.EncodeToString(o.Bytes(main_.DS(koukai2.DSClockSave), 4)),
				Position: o.Word(main_.DS(0x48DFE - 0x48150)),
			}
		}
	})

	if !*natural {
		if err := koukai2.ToMainLoop(o, cp); err != nil {
			die(fmt.Errorf("開機路徑：%w（%s）", err, koukai2.Describe(o)))
		}
		if *save != "" {
			if err := o.SaveStateFile(*save); err != nil {
				die(err)
			}
		}
	}

	r.TimerHz, r.TimerDivisor = o.TimerHz(), o.TimerDivisor()
	for _, e := range o.ExecLog() {
		r.Execs = append(r.Execs, exec{e.Base, e.PSP, e.Exit, e.Ended, e.TSR})
	}
	r.Opened, r.Missing, r.Unimpl = o.Opened(), o.Missing(), o.Unimplemented()
	ns := make([]string, 0, len(rng.ByN))
	for n, c := range rng.ByN {
		ns = append(ns, fmt.Sprintf("%d:%d", n, c))
	}
	sort.Strings(ns)
	r.RNG = map[string]any{
		"calls": rng.Calls, "first_seed": rng.First, "mismatch": rng.Mismatch, "repeat_fires": rng.Repeats,
		"first_bad": rng.FirstBad, "rand_n_calls": rng.NCalls, "rand_n_by_n": ns,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", " ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		die(err)
	}
	if *out == "" {
		os.Stdout.Write(buf.Bytes())
		return
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		die(err)
	}
	fmt.Printf("%s：%d 個比對點，亂數 %d 次、不符 %d\n", *out, len(r.Checkpoints), rng.Calls, rng.Mismatch)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
