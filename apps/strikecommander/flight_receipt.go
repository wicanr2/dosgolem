package strikecommander

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/session"
)

// 飛行字串的收據接線與寫檔（strike_commander 規格 009 §3.11 接點第 2、4、7 條）。

// attachFlight 建 FlightObserver 與各倍率的 FlightOverlay，接到 so（Flight、OnFlightFrame）。
// 升格在 Frame 回呼內發生：OnCommit 設「這次 Frame 有升格」旗標，OnFlightFrame 讀後清掉；有升格時每個倍率
// Sync 後 Match，不符筆數記入「升格後第一個 Frame 被擋下」；「CU 進行中」的 Frame 也 Match，記入「CU 期間比對不符」。
// 兩個計數以最大倍率的結果回填 FlightCounts，各倍率的筆數另記在 res。
func attachFlight(cfg ReceiptConfig, res *ReceiptResult, so *SessionObserver, fovs map[int]*FlightOverlay) error {
	cats := *cfg.FlightCatalogs
	flight := NewFlightObserver(func(k Kind, s string) (string, int, bool) { return ResolveFlight(k, s, cats) })
	maxSc := 0
	for _, sc := range cfg.Scales {
		fov, err := NewFlightOverlayText(cfg.Fonts, cats, sc) // strike_commander 規格 011 §3.9
		if err != nil {
			return err
		}
		fovs[sc] = fov
		maxSc = max(maxSc, sc)
	}
	promoted := false
	flight.OnCommit = func(c FlightCommit) {
		promoted = true
		res.FlightCommits = append(res.FlightCommits, c)
		var counts []CommitStamps
		for _, sc := range cfg.Scales {
			fov := fovs[sc]
			fov.Sync(flight.Visible(), flight.Generation())
			counts = append(counts, fov.commitStamps(sc)) // 011 §3.9
		}
		res.FlightStamps = append(res.FlightStamps, counts)
	}
	so.Flight = flight
	so.OnFlightFrame = func(indexed []byte, steps uint64) {
		if promoted {
			promoted = false
			blocked := map[int]int{}
			for _, sc := range cfg.Scales {
				fov := fovs[sc]
				fov.Sync(flight.Visible(), flight.Generation())
				blocked[sc] = len(fov.Match(indexed))
			}
			flight.NoteFirstFrameBlocked(blocked[maxSc])
			res.FlightFirstBlocked = append(res.FlightFirstBlocked, blocked)
		}
		if in, _ := flight.CUState(); in {
			cf := FlightCUFrame{Steps: steps, Visible: len(flight.visible), Mismatch: map[int]int{}, Keys: map[int][]string{}}
			for _, sc := range cfg.Scales {
				fov := fovs[sc]
				fov.Sync(flight.Visible(), flight.Generation())
				bad := fov.Match(indexed)
				cf.Mismatch[sc], cf.Keys[sc] = len(bad), bad
			}
			flight.NoteCUFrameMismatch(cf.Mismatch[maxSc])
			res.FlightCUFrames = append(res.FlightCUFrames, cf)
		}
	}
	return nil
}

// flightTSVHeader 是 flight.tsv 的欄名（009 §3.11、011 §3.8）。
var flightTSVHeader = []string{
	"key", "catkey", "kind", "k", "x0", "xs", "x1", "y", "b0", "bh", "cx0", "cx1", "text", "fg", "outline",
	"hidden",
	"before_off", "before_len", "after_off", "after_len", "ref_off", "ref_len",
	"display", "px", "tx_out", "baseline", "clip_x0", "clip_y0", "clip_x1", "clip_y1",
	"ink_x0", "ink_y0", "ink_x1", "ink_y1", "e0", "e1",
}

// FlightTSV 把一份 Report 編成 flight.tsv 與 flight-rows.bin 的內容。
func FlightTSV(entries []FlightEntry) (tsv, rows []byte) {
	var t strings.Builder
	var r bytes.Buffer
	t.WriteString(strings.Join(flightTSVHeader, "\t") + "\n")
	put := func(b []byte) (int, int) {
		if b == nil {
			return 0, 0
		}
		off := r.Len()
		r.Write(b)
		return off, len(b)
	}
	b01 := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	for _, e := range entries {
		xs := make([]string, len(e.Xs))
		for i, x := range e.Xs {
			xs[i] = fmt.Sprint(x)
		}
		bo, bl := put(e.Before)
		ao, al := put(e.After)
		ro, rl := put(e.Ref)
		ink := e.Ink
		if ink.Empty() {
			ink.X0, ink.Y0, ink.X1, ink.Y1 = 0, 0, 0, 0
		}
		extra := fmt.Sprintf("%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d", e.Display, e.Px, e.TxOut, e.Baseline,
			e.Clip.X0, e.Clip.Y0, e.Clip.X1, e.Clip.Y1, ink.X0, ink.Y0, ink.X1, ink.Y1, e.E0, e.E1)
		fmt.Fprintf(&t, "%s\t%s\t%s\t%d\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n",
			e.Key, e.CatKey, e.Kind, e.K, e.TextX0, strings.Join(xs, ","), e.TextX1, e.Y, e.B0, e.BH, e.X0, e.X1,
			e.Text, e.FG, b01(e.Outline), b01(e.Hidden),
			bo, bl, ao, al, ro, rl, extra)
	}
	return []byte(t.String()), r.Bytes()
}

// DigestText 是 <前綴>-digest.txt 的內容：StateDigest 每個欄位一列。
func DigestText(d session.StateDigest) []byte {
	return []byte(fmt.Sprintf("Steps\t%d\nMemorySHA256\t%x\nCPUSHA256\t%x\nIndexedSHA256\t%x\nPaletteSHA256\t%x\nDOSExited\t%t\nAdLib\t%t\nClockPercent\t%d\nExtSHA256\t%s\nIRQ0Clamped\t%d\n",
		d.Steps, d.MemorySHA256, d.CPUSHA256, d.IndexedSHA256, d.PaletteSHA256, d.DOSExited, d.AdLib, d.ClockPercent, d.ExtSHA256, d.IRQ0Clamped))
}

// WriteFlightFiles 寫一個 shot 或停止點的飛行收據（§3.11 接點第 7 條）：<前綴>-s<倍率>-flight.tsv、
// <前綴>-s<倍率>-flight-rows.bin 與 <前綴>-digest.txt。frame.idx、palette.bin 由 WriteReceiptFiles 寫。
func WriteFlightFiles(dir, prefix string, scales []int, report map[int][]FlightEntry, d session.StateDigest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, sc := range scales {
		tsv, rows := FlightTSV(report[sc])
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-s%d-flight.tsv", prefix, sc)), tsv, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-s%d-flight-rows.bin", prefix, sc)), rows, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, prefix+"-digest.txt"), DigestText(d), 0o644)
}

// FlightCommitsTSV 是每次升格一列：CU 入口步數、Frame 步數、generation、每筆（kind:key@X0,Y;Xs）。
func FlightCommitsTSV(cs []FlightCommit) []byte {
	var b strings.Builder
	b.WriteString("cu_steps\tframe_steps\tgen\ttexts\n")
	for _, c := range cs {
		var parts []string
		for _, t := range c.Texts {
			xs := make([]string, len(t.Xs))
			for i, x := range t.Xs {
				xs[i] = fmt.Sprint(x)
			}
			parts = append(parts, fmt.Sprintf("%s|%s|%d|%d|%d|%s", t.Kind, t.Key, t.X0, t.Y, t.X1, strings.Join(xs, ",")))
		}
		fmt.Fprintf(&b, "%d\t%d\t%d\t%s\n", c.CUSteps, c.FrameSteps, c.Generation, strings.Join(parts, ";"))
	}
	return []byte(b.String())
}
