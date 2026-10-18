package strikecommander

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
	"github.com/wicanr2/dosgolem/xlate"
)

// SessionObserver 把 LabelObserver 接到 session.StepObserver，並保留最後一次回掃的畫格複本。
// 它不保留 StepView，也不寫入任何原版狀態。
type SessionObserver struct {
	Labels      *LabelObserver
	LastIndexed []byte
	LastPalette [256][3]uint8
	Frames      int
}

var (
	_ View                 = session.StepView{}
	_ session.StepObserver = (*SessionObserver)(nil)
)

func (s *SessionObserver) BeforeStep(v session.StepView) error {
	s.Labels.Observe(v)
	return nil
}

func (s *SessionObserver) VideoWrite(machine.VideoWrite) {}

func (s *SessionObserver) Frame(indexed []byte, palette [256][3]uint8) {
	s.LastIndexed, s.LastPalette = indexed, palette
	s.Frames++
}

// SessionConfig 是 dosgolem 規格 448 §3.2 的 Config（KbdIntercept 開），觀察器由呼叫端給（nil 為關）。
func SessionConfig(obs session.StepObserver) session.Config {
	const s = 2
	return session.Config{
		InitialScale: host.OutputScale2,
		InitialLayout: host.MouseLayout{Epoch: 1, Scale: host.OutputScale2, ChromeHeight: 18 * s,
			Canvas: host.Canvas{Width: 320, Height: 200}, FrameWidth: 320 * s, FrameHeight: 18*s + 200*s},
		Observer:             obs,
		ProgramPath:          `C:\SCCD.EXE`,
		DOSFiles:             30,
		ExtendedMemoryKB:     8192,
		NoEMS:                true,
		ProtectedModeProfile: true,
		ExecEnvCopy:          true,
		ExecReclaim:          true,
		Mode13Phase:          true,
		SoundBlaster:         true,
		AdLib:                true,
		SBDMA8:               true,
		KbdIntercept:         true,
	}
}

// ReceiptConfig 是收據執行（001 §2.8）的輸入。
type ReceiptConfig struct {
	SaveRoot    string // SB16 設定樹的可寫複本
	State       []byte
	StateSHA256 [32]byte
	RebaseFrom  []string
	StopSteps   uint64 // 跑到這一步（絕對步數）
	Scales      []int  // 合成圖的倍率，例如 2、3
	Catalog     map[string]string
	Font        *LabelFont
	Overlay     bool // false：不裝觀察器（同狀態 A/B 的關）
	CursorY     int  // 滑鼠游標中心列，負值為未知

	noCallerCheck bool // §3.3 的負對照
}

// CommitStamps 是一次提交後各倍率的 stamp 數。
type CommitStamps struct {
	Scale  int
	Stamps int
}

// ReceiptResult 是收據執行的結果。
type ReceiptResult struct {
	Boot    session.BootReceipt
	Digest  session.StateDigest
	Commits []Commit
	// Stamps 與 Commits 平行：每次提交後各倍率 overlay 的 stamp 數。
	Stamps       [][]CommitStamps
	Log          []Event
	LogDropped   int
	PartialBlits int

	// 以下只在 Overlay 開時有值。
	Frame          []byte // 停止時最後一次回掃的畫格
	Palette        [256][3]uint8
	FrameMatches   bool // Frame 的雜湊等於 Digest.IndexedSHA256
	Overlays       map[int]*Overlay
	Composites     map[int][]byte
	Plain          map[int][]byte
	CompositeCheck map[int]CompositeCheck
}

// CompositeCheck 是合成圖與原版畫格放大後的逐像素比較。
type CompositeCheck struct {
	Diffs   int
	Outside int // 落在遮蓋 stamp 框（× 倍率）外的差異像素
}

// RunReceipt 從狀態檔啟動 session，跑到 StopSteps，記錄每次提交與合成圖。
func RunReceipt(cfg ReceiptConfig) (*ReceiptResult, error) {
	res := &ReceiptResult{}
	var so *SessionObserver
	var obs session.StepObserver
	overlays := map[int]*Overlay{}
	if cfg.Overlay {
		if cfg.Font == nil {
			return nil, errors.New("strikecommander: 收據缺字型")
		}
		lo := NewLabelObserver()
		lo.InkCheck, lo.CursorY, lo.noCallerCheck = true, cfg.CursorY, cfg.noCallerCheck
		for _, sc := range cfg.Scales {
			ov, err := NewOverlay(cfg.Font, cfg.Catalog, sc)
			if err != nil {
				return nil, err
			}
			overlays[sc] = ov
		}
		lo.OnCommit = func(c Commit) {
			res.Commits = append(res.Commits, c)
			var counts []CommitStamps
			for _, sc := range cfg.Scales {
				overlays[sc].Sync(c.Labels, c.Generation)
				counts = append(counts, CommitStamps{sc, len(overlays[sc].Stamps())})
			}
			res.Stamps = append(res.Stamps, counts)
		}
		so = &SessionObserver{Labels: lo}
		obs = so
	}
	o, err := session.New(SessionConfig(obs))
	if err != nil {
		return nil, err
	}
	defer o.Close()
	boot, err := o.BootFromState(session.StateInput{State: cfg.State, ExpectedStateSHA256: cfg.StateSHA256, SaveRoot: cfg.SaveRoot, RebaseFrom: cfg.RebaseFrom})
	if err != nil {
		return nil, err
	}
	res.Boot = boot
	if cfg.StopSteps <= boot.Steps {
		return nil, fmt.Errorf("strikecommander: 停止步數 %d 不大於狀態檔步數 %d", cfg.StopSteps, boot.Steps)
	}
	v, err := o.View()
	if err != nil {
		return nil, err
	}
	if _, err := o.Deliver(session.CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration}); err != nil {
		return nil, err
	}
	r, err := o.Advance(session.InstructionBudget(cfg.StopSteps - boot.Steps))
	if err != nil {
		return res, fmt.Errorf("strikecommander: Advance（停在第 %d 步）：%w", r.MachineStepsAfter, err)
	}
	if r.MachineStepsAfter != cfg.StopSteps {
		return res, fmt.Errorf("strikecommander: 停在第 %d 步，要 %d", r.MachineStepsAfter, cfg.StopSteps)
	}
	if res.Digest, err = o.Digest(); err != nil {
		return res, err
	}
	if !cfg.Overlay {
		return res, nil
	}
	res.Log, res.LogDropped = so.Labels.Log()
	res.PartialBlits = so.Labels.PartialBlits()
	if so.LastIndexed == nil {
		return res, errors.New("strikecommander: 沒有收到任何回掃畫格")
	}
	res.Frame, res.Palette = so.LastIndexed, so.LastPalette
	res.FrameMatches = sha256.Sum256(res.Frame) == res.Digest.IndexedSHA256
	res.Overlays = overlays
	res.Composites, res.Plain, res.CompositeCheck = map[int][]byte{}, map[int][]byte{}, map[int]CompositeCheck{}
	for _, sc := range cfg.Scales {
		ov := overlays[sc]
		ov.Sync(so.Labels.Visible(), so.Labels.Generation())
		comp, _ := ov.Draw(res.Frame, res.Palette)
		plain := ScaleIndexedRGBA(res.Frame, res.Palette, sc)
		res.Composites[sc], res.Plain[sc] = comp, plain
		res.CompositeCheck[sc] = CheckComposite(comp, plain, sc, ov.Stamps())
	}
	return res, nil
}

// CheckComposite 逐像素比較合成圖與原版放大圖：差異只能落在遮蓋 stamp（Font == nil）的框 × 倍率內。
func CheckComposite(comp, plain []byte, scale int, stamps []*xlate.Stamp) CompositeCheck {
	var c CompositeCheck
	w := 320 * scale
	for i := 0; i+3 < len(comp) && i+3 < len(plain); i += 4 {
		if comp[i] == plain[i] && comp[i+1] == plain[i+1] && comp[i+2] == plain[i+2] && comp[i+3] == plain[i+3] {
			continue
		}
		c.Diffs++
		x, y := (i/4)%w/scale, (i/4)/w/scale
		in := false
		for _, s := range stamps {
			if s.Font == nil && x >= s.X && x < s.X+s.Cells*s.CellW && y >= s.Y && y < s.Y+s.CellH {
				in = true
				break
			}
		}
		if !in {
			c.Outside++
		}
	}
	return c
}

// CopyTree 把 src 實體複製到 dst（不用硬連結）；遊戲會寫入 SaveRoot，每次執行各用一份。
func CopyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		switch {
		case info.IsDir():
			return os.MkdirAll(out, 0o755)
		case !info.Mode().IsRegular():
			return fmt.Errorf("複製樹內有非一般檔案：%s", p)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, in); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	})
}
