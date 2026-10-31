// sweep 在 DOS 原版上照一份巡檢路線走（`docs/spec/198` §3.5）。
//
//	DOSGOLEM_ORIG=<原版目錄> DOSGOLEM_EXTRA_MOUNT=<輸出>:/out \
//	    tools/go.sh run ./apps/dm/cmd/sweep \
//	    -exe /orig/dm.exe -root /orig -route /out/map2-save.json -out /out/run
//
// 從原版附的 `DMSAVE.DAT` 讀檔出發，每個檢查點寫一份 `party.json` 與
// `frame.png`，最後寫總表 `receipt.json`。哪一步的 `expect` 對不上就當場停，
// 那一步的取樣寫到 `fail-NNNN/`，回非零碼。
//
// ⚠ **巡檢跑出來的不是 parity 證據**（§1）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/apps/dm"
)

type fileHash struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

type checkpoint struct {
	Step        int    `json:"step"`
	GameTime    int    `json:"game_time"`
	Dir         string `json:"dir"`
	PartySHA256 string `json:"party_sha256"`
	FrameSHA256 string `json:"frame_sha256"`
}

type receipt struct {
	Schema      int          `json:"schema"`
	Runner      string       `json:"runner"`
	Route       fileHash     `json:"route"`
	Save        fileHash     `json:"save"`
	SaveAfter   string       `json:"save_sha256_after,omitempty"`
	Shell       fileHash     `json:"shell"`
	Engine      fileHash     `json:"engine"`
	Map         int          `json:"map"`
	StepsTotal  int          `json:"steps_total"`
	StepsDone   int          `json:"steps_done"`
	Checkpoints []checkpoint `json:"checkpoints"`
	Result      string       `json:"result"` // ok／expect_mismatch／error
	Error       string       `json:"error,omitempty"`
	Failure     *checkpoint  `json:"failure,omitempty"`
}

func hashFile(path string) (fileHash, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fileHash{}, err
	}
	sum := sha256.Sum256(raw)
	return fileHash{Name: filepath.Base(path), SHA256: hex.EncodeToString(sum[:])}, nil
}

func main() {
	exe := flag.String("exe", "", "dm.exe 路徑（玩家自備）")
	root := flag.String("root", "", "原版資料目錄，要有路線指定的存檔")
	routePath := flag.String("route", "", "巡檢路線 JSON（remake 的 dmtool sweep route 產生）")
	out := flag.String("out", "", "輸出目錄")
	flag.Uint64Var(&dm.RouteHover, "hover", dm.RouteHover, "每一步移游標之後、按下之前跑幾道指令")
	flag.Parse()
	if *exe == "" || *root == "" || *routePath == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	code, err := run(*exe, *root, *routePath, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sweep:", err)
	}
	os.Exit(code)
}

func run(exe, root, routePath, out string) (int, error) {
	r, err := dm.ReadRoute(routePath)
	if err != nil {
		return 2, err
	}
	rec := receipt{Schema: 1, Runner: "dosgolem", Map: r.Map, StepsTotal: len(r.Steps),
		Checkpoints: []checkpoint{}}
	if rec.Route, err = hashFile(routePath); err != nil {
		return 2, err
	}
	// ⚠ 收據只記 basename 與雜湊，不記私有絕對路徑（§3.5）。
	savePath := filepath.Join(root, r.Save.Name)
	if rec.Save, err = hashFile(savePath); err != nil {
		return 2, fmt.Errorf("讀存檔：%w", err)
	}
	if rec.Save.SHA256 != r.Save.SHA256 {
		return 2, fmt.Errorf("存檔 %s 的 SHA-256 是 %s，路線要的是 %s——起點不同，兩側比不起來",
			r.Save.Name, rec.Save.SHA256, r.Save.SHA256)
	}
	if rec.Shell, err = hashFile(exe); err != nil {
		return 2, err
	}
	if rec.Engine, err = hashFile(filepath.Join(filepath.Dir(exe), dm.EngineName)); err != nil {
		return 2, err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return 2, err
	}

	o, err := dm.Load(exe, root)
	if err != nil {
		return 2, err
	}
	defer o.Close()

	want := map[int]bool{}
	for _, k := range r.Checkpoints {
		want[k] = true
	}
	var last dm.StepResult
	write := func(dir string, s dm.StepResult) (checkpoint, error) {
		cp := checkpoint{Step: s.Step, GameTime: s.Party.GameTime, Dir: dir}
		full := filepath.Join(out, dir)
		if err := os.MkdirAll(full, 0o755); err != nil {
			return cp, err
		}
		raw, err := json.MarshalIndent(s.Party, "", "  ")
		if err != nil {
			return cp, err
		}
		raw = append(raw, '\n')
		if err := os.WriteFile(filepath.Join(full, "party.json"), raw, 0o644); err != nil {
			return cp, err
		}
		sum := sha256.Sum256(raw)
		cp.PartySHA256 = hex.EncodeToString(sum[:])
		frame := filepath.Join(full, "frame.png")
		if err := o.WritePNG(frame); err != nil {
			return cp, err
		}
		h, err := hashFile(frame)
		if err != nil {
			return cp, err
		}
		cp.FrameSHA256 = h.SHA256
		return cp, nil
	}

	// steps.jsonl 每一步一行：位置、面向與遊戲刻。
	trace, err := os.Create(filepath.Join(out, "steps.jsonl"))
	if err != nil {
		return 2, err
	}
	defer trace.Close()
	enc := json.NewEncoder(trace)

	b, c, err := dm.Resume(o)
	if err == nil {
		err = b.RunRoute(c, r, func(s dm.StepResult) error {
			last = s
			rec.StepsDone = s.Step
			if err := enc.Encode(map[string]any{"step": s.Step, "game_time": s.Party.GameTime,
				"x": s.Party.X, "y": s.Party.Y, "facing": s.Party.Facing}); err != nil {
				return err
			}
			if !want[s.Step] {
				return nil
			}
			cp, err := write(fmt.Sprintf("cp-%04d", s.Step), s)
			if err != nil {
				return err
			}
			rec.Checkpoints = append(rec.Checkpoints, cp)
			return nil
		})
	}

	rec.Result = "ok"
	var mismatch *dm.ExpectError
	switch {
	case errors.As(err, &mismatch):
		rec.Result, rec.Error = "expect_mismatch", err.Error()
		// expect 不符時那一步已經算「走了」，但沒有通過。
		rec.StepsDone = mismatch.Step - 1
		if rec.StepsDone < 0 {
			rec.StepsDone = 0
		}
		if cp, werr := write(fmt.Sprintf("fail-%04d", last.Step), last); werr == nil {
			rec.Failure = &cp
		}
	case err != nil:
		rec.Result, rec.Error = "error", err.Error()
	}

	// ⚠ 讀檔只讀不寫（§3.6）。存檔被改到的話這一輪的結果不算數。
	if after, herr := hashFile(savePath); herr == nil {
		rec.SaveAfter = after.SHA256
		if after.SHA256 != rec.Save.SHA256 && rec.Result == "ok" {
			rec.Result, rec.Error = "error", "跑完之後存檔的 SHA-256 變了"
		}
	}

	raw, merr := json.MarshalIndent(rec, "", "  ")
	if merr != nil {
		return 1, merr
	}
	if werr := os.WriteFile(filepath.Join(out, "receipt.json"), append(raw, '\n'), 0o644); werr != nil {
		return 1, werr
	}
	fmt.Printf("%s：%d／%d 步，%d 個檢查點，結束於遊戲刻 %d\n",
		rec.Result, rec.StepsDone, rec.StepsTotal, len(rec.Checkpoints), last.Party.GameTime)
	if rec.Result != "ok" {
		return 1, errors.New(rec.Error)
	}
	return 0, nil
}
