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
	// OnFrame 在每次回掃、更新 LastIndexed 之前呼叫（005 §4.4 的量測）：old 是上一次回掃的畫格（可為 nil），
	// cur 是這次的；steps 是最後一次 BeforeStep 的步數。可為 nil。兩者都不得修改。
	OnFrame func(old, cur []byte, steps uint64)

	steps uint64
}

var (
	_ View                 = session.StepView{}
	_ session.StepObserver = (*SessionObserver)(nil)
)

// AttachOverlays 把存讀檔對話框的繪圖事件與全螢幕 blit 入口的清除送到每個倍率的 Overlay（004 §3.1、§3.3）。
// 每次呼叫取代前一次的接線。
func (s *SessionObserver) AttachOverlays(ovs ...*Overlay) {
	s.Labels.OnArt = func(ev ArtEvent) {
		for _, ov := range ovs {
			ov.AddDialogArt(ev)
		}
	}
	s.Labels.OnClearArt = func(uint64) {
		for _, ov := range ovs {
			ov.ClearArt()
		}
	}
}

func (s *SessionObserver) BeforeStep(v session.StepView) error {
	s.steps = v.Steps()
	s.Labels.Observe(v)
	return nil
}

func (s *SessionObserver) VideoWrite(machine.VideoWrite) {}

func (s *SessionObserver) Frame(indexed []byte, palette [256][3]uint8) {
	if s.OnFrame != nil {
		s.OnFrame(s.LastIndexed, indexed, s.steps)
	}
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
	GfxCatalog  map[string]string // 圖形字 catalog（text/menu_gfx，004 §3.6）；nil 時對話框不產生圖面
	Font        *LabelFont
	Overlay     bool // false：不裝觀察器（同狀態 A/B 的關）
	CursorY     int  // 滑鼠游標中心列，負值為未知
	// Inputs、Shots 是腳本事件（002 §2.4）；StopSteps 取腳本的 stop。
	// 觀察器開、關兩次執行都照同一份事件推進；Overlay 關時 shot 只切分推進、不出圖。
	Inputs []ScriptedInput
	Shots  []Shot

	noCallerCheck bool // 001 §3.3 的負對照
	exactOnly     bool // 002 §3.2 案例 D 的負對照：解析只留第 1 步
	noBoardCaller bool // 003 §4.2 案例 E 的負對照：只關掉計分板呼叫點（003 §3.1 條件 2）
}

// CommitStamps 是一次提交後某倍率的 stamp 數與文字 stamp 的文字（依 stamp 順序）。
// Kinds 與 Texts 平行，是各文字 stamp 的種類（003 §3.4）。
type CommitStamps struct {
	Scale  int
	Stamps int
	Texts  []string
	Kinds  []Kind
}

// ShotResult 是腳本一個 shot 的結果：當時各倍率的合成圖、原版放大圖與比較。
type ShotResult struct {
	Steps      uint64
	Name       string
	Indexed    []byte // 出圖用的回掃畫格（320×200 色號）與色盤，供離線比對（003 §4.2）
	Palette    [256][3]uint8
	Composites map[int][]byte
	Plain      map[int][]byte
	Check      map[int]CompositeCheck
	// ArtKeys 是 shot 時各倍率的存讀檔對話框圖面 Key（004 §3.3）。
	ArtKeys map[int][]string
	// Hidden 是各倍率 Draw 因比對不符而不畫的標籤 Key（005 §4.2）；Visible 是 shot 時可見集合的 Key。
	Hidden  map[int][]string
	Visible []string
	// Covers 是 shot 時各倍率遮蓋 stamp 的矩形 [x0, y0, x1, y1)（1×），以 stamp Key 索引（005 §4.2 的比對框）。
	Covers map[int]map[string][4]int
	// BG 是 shot 時各倍率 Draw 之後的 BGReport（006 §3.3）。
	BG map[int][]BGEntry
}

// FrameMeasure 是 005 §4.4 的量測：每次回掃時以各倍率 Overlay 的 Match 統計「可見集合非空但有標籤不符」的回掃數。
// 兩種取樣並列（勘誤候選，見 strike_commander workplace/i005/report.md）：
//   - Cur：Match(這次回掃的畫格)，即回掃之後前端會畫的那一張；
//   - Old：Match(上一次回掃的畫格，LastIndexed)，即回掃之前前端一直在畫的那一張。
//
// 分類依序判定：(1) 上一次回掃早於上次提交（Old 的畫格比 Ref 舊；對 Cur 是「兩次回掃之間有提交」）；
// (2) 腳本最後一次 move 的游標 11×11 矩形與不符標籤的比對框相交；(3) 其他。
type FrameMeasure struct {
	Frames int // 可見集合非空的回掃數
	Cur    map[int]*MeasureCount
	Old    map[int]*MeasureCount
}

// MeasureCount 是一個倍率、一種取樣的分類次數；Others 逐筆列出 (3)。
type MeasureCount struct {
	Mismatch int
	Class    [3]int
	Others   []MeasureEvent
}

// MeasureEvent 是一筆 (3) 類：回掃步數與不符的 Key。
type MeasureEvent struct {
	Steps uint64
	Keys  []string
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
	Shots        []ShotResult // 只在 Overlay 開時有值，與 ReceiptConfig.Shots 平行
	// Arts 是被接受的存讀檔對話框繪圖事件（004 §3.1），依先後；只記中繼資料，不含解碼陣列。
	Arts []ArtEvent
	// ArtClears 是圖面清除（全螢幕 blit 入口）且清除前有圖面的步數。
	ArtClears []uint64

	// 以下只在 Overlay 開時有值。
	Frame          []byte // 停止時最後一次回掃的畫格
	Palette        [256][3]uint8
	FrameMatches   bool // Frame 的雜湊等於 Digest.IndexedSHA256
	Overlays       map[int]*Overlay
	Composites     map[int][]byte
	Plain          map[int][]byte
	CompositeCheck map[int]CompositeCheck
	Measure        FrameMeasure // 005 §4.4
	// BG 是停止時各倍率 Draw 之後的 BGReport（006 §3.3）。
	BG map[int][]BGEntry
}

// cursorHits 回游標 11×11 矩形 [x−5, x+6) × [y−5, y+6) 是否與 keys 任一筆的比對框相交（005 §4.4 分類 2）。
func cursorHits(ov *Overlay, keys []string, p *Point) bool {
	if p == nil {
		return false
	}
	for _, k := range keys {
		x0, y0, x1, y1, ok := ov.MatchBox(k)
		if ok && p.X-5 < x1 && x0 < p.X+6 && p.Y-5 < y1 && y0 < p.Y+6 {
			return true
		}
	}
	return false
}

// CompositeCheck 是合成圖與原版畫格放大後的逐像素比較。
type CompositeCheck struct {
	Diffs   int
	Outside int // 落在遮蓋 stamp 框（× 倍率）外的差異像素
}

// RunReceipt 從狀態檔啟動 session，依腳本事件推進到 StopSteps，記錄每次提交與合成圖。
// 回合節奏同 cmd/strikecommander-census（002 §2.4）：
//  1. 推進到事件步數：重新 View，交付空更新，Advance 到該步；
//  2. 輸入事件：重新 View，交付對應的輸入，Advance(1)；
//  3. shot：Sync(Visible(), Generation()) 後以最後一次回掃的畫格出各倍率合成圖與原版放大圖。
//
// 每次 Advance 後核對 MachineStepsAfter 等於目標；早停時回錯誤。沒有事件時只推進到 StopSteps。
func RunReceipt(cfg ReceiptConfig) (*ReceiptResult, error) {
	res := &ReceiptResult{}
	var so *SessionObserver
	var obs session.StepObserver
	overlays := map[int]*Overlay{}
	var lastMove *Point // 腳本最後一次 move 的 DOS 座標（005 §4.4 分類 2）
	if cfg.Overlay {
		if cfg.Font == nil {
			return nil, errors.New("strikecommander: 收據缺字型")
		}
		lo := NewLabelObserver()
		lo.InkCheck, lo.CursorY, lo.noCallerCheck, lo.noBoardCaller = true, cfg.CursorY, cfg.noCallerCheck, cfg.noBoardCaller
		for _, sc := range cfg.Scales {
			ov, err := NewOverlay(cfg.Font, cfg.Catalog, cfg.GfxCatalog, sc)
			if err != nil {
				return nil, err
			}
			ov.exactOnly = cfg.exactOnly
			overlays[sc] = ov
		}
		var lastCommit, prevFrame uint64
		lo.OnCommit = func(c Commit) {
			lastCommit = c.BlitReturn
			// 保存的複本中 label 去掉前後列（006 §3.1 第 4 條，省記憶體）；board 保留。
			// 另建 Labels 切片，Overlay 仍收到完整的 c.Labels。
			kept := c
			kept.Labels = append([]Label(nil), c.Labels...)
			for i := range kept.Labels {
				if kept.Labels[i].Kind == KindLabel {
					kept.Labels[i].Before, kept.Labels[i].After = nil, nil
				}
			}
			res.Commits = append(res.Commits, kept)
			var counts []CommitStamps
			for _, sc := range cfg.Scales {
				ov := overlays[sc]
				ov.Sync(c.Labels, c.Generation)
				cs := CommitStamps{Scale: sc, Stamps: len(ov.Stamps())}
				for _, s := range ov.Stamps() {
					if s.Font != nil {
						cs.Texts = append(cs.Texts, string(s.Text))
						cs.Kinds = append(cs.Kinds, ov.StampKind(s))
					}
				}
				counts = append(counts, cs)
			}
			res.Stamps = append(res.Stamps, counts)
		}
		so = &SessionObserver{Labels: lo}
		var ovs []*Overlay
		for _, sc := range cfg.Scales {
			ovs = append(ovs, overlays[sc])
		}
		so.AttachOverlays(ovs...)
		forward, clear := lo.OnArt, lo.OnClearArt
		lo.OnArt = func(ev ArtEvent) {
			res.Arts = append(res.Arts, ArtEvent{Steps: ev.Steps, G: ev.G, K: ev.K, X: ev.X, Y: ev.Y, Order: ev.Order, W: ev.W, H: ev.H})
			forward(ev)
		}
		lo.OnClearArt = func(steps uint64) {
			if len(ovs) != 0 && len(ovs[0].Art()) != 0 {
				res.ArtClears = append(res.ArtClears, steps)
			}
			clear(steps)
		}
		// 005 §4.4 的量測。簽章層與 §3.1 第 5 條清空時不經 OnCommit，所以先以 Visible()、Generation() Sync。
		res.Measure = FrameMeasure{Cur: map[int]*MeasureCount{}, Old: map[int]*MeasureCount{}}
		for _, sc := range cfg.Scales {
			res.Measure.Cur[sc], res.Measure.Old[sc] = &MeasureCount{}, &MeasureCount{}
		}
		classify := func(mc *MeasureCount, ov *Overlay, bad []string, steps uint64) {
			if len(bad) == 0 {
				return
			}
			mc.Mismatch++
			switch {
			case prevFrame < lastCommit:
				mc.Class[0]++
			case cursorHits(ov, bad, lastMove):
				mc.Class[1]++
			default:
				mc.Class[2]++
				mc.Others = append(mc.Others, MeasureEvent{Steps: steps, Keys: bad})
			}
		}
		so.OnFrame = func(old, cur []byte, steps uint64) {
			defer func() { prevFrame = steps }()
			vis := lo.Visible()
			if len(vis) == 0 {
				return
			}
			res.Measure.Frames++
			for _, sc := range cfg.Scales {
				ov := overlays[sc]
				ov.Sync(vis, lo.Generation())
				classify(res.Measure.Cur[sc], ov, ov.Match(cur), steps)
				if old != nil {
					classify(res.Measure.Old[sc], ov, ov.Match(old), steps)
				}
			}
		}
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
	evs, items := receiptEvents(cfg.Inputs, cfg.Shots, cfg.StopSteps)
	if err := validateScript(evs, true, cfg.StopSteps, boot.Steps); err != nil {
		return nil, err
	}
	steps := boot.Steps
	// deliver 以當次 View 交付；mk 依當次 Layout 填入輸入（nil 為空更新）。
	deliver := func(mk func(l host.MouseLayout, u *session.CapturedUpdate)) error {
		v, err := o.View()
		if err != nil {
			return err
		}
		u := session.CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration}
		if mk != nil {
			mk(v.Layout, &u)
		}
		_, err = o.Deliver(u)
		return err
	}
	advance := func(target uint64) error {
		r, err := o.Advance(session.InstructionBudget(target - steps))
		if err != nil {
			return fmt.Errorf("strikecommander: Advance（停在第 %d 步）：%w", r.MachineStepsAfter, err)
		}
		if r.MachineStepsAfter != target {
			return fmt.Errorf("strikecommander: 停在第 %d 步，要 %d", r.MachineStepsAfter, target)
		}
		steps = target
		return nil
	}
	for i, e := range evs {
		// 1. 推進到事件步數（stop 也是）。
		if err := deliver(nil); err != nil {
			return res, err
		}
		if err := advance(e.steps); err != nil {
			return res, err
		}
		if e.kind == "stop" {
			break
		}
		switch it := items[i].(type) {
		case ScriptedInput:
			if it.Kind == "kdown" || it.Kind == "kup" {
				// 2a. 鍵盤事件（007 §3.8）：硬體掃描碼經 IRQ1，kup 為放開；之後同滑鼠推進 1 步。
				err := deliver(func(_ host.MouseLayout, u *session.CapturedUpdate) {
					u.ScanEvents = []session.ScanEvent{{Scan: it.Scan, Break: it.Kind == "kup"}}
				})
				if err != nil {
					return res, fmt.Errorf("strikecommander: 第 %d 步 %s：%w", it.Steps, it.Kind, err)
				}
				if err := advance(steps + 1); err != nil {
					return res, err
				}
				continue
			}
			// 2. 輸入事件：DOS 邏輯像素換成 host 座標。
			var kind host.MouseEventKind
			switch it.Kind {
			case "move":
				kind = host.MouseEventMove
			case "down":
				kind = host.MouseEventDown
			case "up":
				kind = host.MouseEventUp
			default:
				return res, fmt.Errorf("strikecommander: 第 %d 步的輸入 %q 不支援", it.Steps, it.Kind)
			}
			err := deliver(func(l host.MouseLayout, u *session.CapturedUpdate) {
				ev := host.MouseEvent{Kind: kind, X: it.X * int(l.Scale), Y: l.ChromeHeight + it.Y*int(l.Scale), Target: host.MouseTargetCanvas}
				switch kind {
				case host.MouseEventMove:
					u.PointerMoves = []host.MouseEvent{ev}
				case host.MouseEventDown:
					u.PointerDown = &ev
				case host.MouseEventUp:
					u.PointerUp = &ev
				}
			})
			if err != nil {
				return res, fmt.Errorf("strikecommander: 第 %d 步 %s：%w", it.Steps, it.Kind, err)
			}
			if kind == host.MouseEventMove {
				lastMove = &Point{it.X, it.Y}
			}
			if err := advance(steps + 1); err != nil {
				return res, err
			}
		case Shot:
			// 3. shot：只在 Overlay 開時出圖；關時只切分推進，與開的那次相同。
			if !cfg.Overlay {
				continue
			}
			if so.LastIndexed == nil {
				return res, fmt.Errorf("strikecommander: 第 %d 步 shot：還沒有回掃畫格", it.Steps)
			}
			sr := ShotResult{Steps: it.Steps, Name: it.Name, Indexed: append([]byte(nil), so.LastIndexed...), Palette: so.LastPalette, Composites: map[int][]byte{}, Plain: map[int][]byte{}, Check: map[int]CompositeCheck{}, ArtKeys: map[int][]string{}, Hidden: map[int][]string{}, Covers: map[int]map[string][4]int{}, BG: map[int][]BGEntry{}}
			for _, l := range so.Labels.Visible() {
				sr.Visible = append(sr.Visible, l.Key)
			}
			for _, sc := range cfg.Scales {
				ov := overlays[sc]
				ov.Sync(so.Labels.Visible(), so.Labels.Generation())
				comp, _ := ov.Draw(so.LastIndexed, so.LastPalette)
				plain := ScaleIndexedRGBA(so.LastIndexed, so.LastPalette, sc)
				sr.Composites[sc], sr.Plain[sc] = comp, plain
				sr.Check[sc] = CheckComposite(comp, plain, sc, ov.Stamps(), ov.Art())
				sr.ArtKeys[sc] = ov.ArtKeys()
				sr.Hidden[sc] = append([]string(nil), ov.Hidden...)
				sr.BG[sc] = ov.BGReport()
				sr.Covers[sc] = map[string][4]int{}
				for _, s := range ov.Stamps() {
					if s.Font == nil {
						sr.Covers[sc][s.Key] = [4]int{s.X, s.Y, s.X + s.Cells*s.CellW, s.Y + s.CellH}
					}
				}
			}
			res.Shots = append(res.Shots, sr)
		}
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
	res.BG = map[int][]BGEntry{}
	for _, sc := range cfg.Scales {
		ov := overlays[sc]
		ov.Sync(so.Labels.Visible(), so.Labels.Generation())
		comp, _ := ov.Draw(res.Frame, res.Palette)
		res.BG[sc] = ov.BGReport()
		plain := ScaleIndexedRGBA(res.Frame, res.Palette, sc)
		res.Composites[sc], res.Plain[sc] = comp, plain
		res.CompositeCheck[sc] = CheckComposite(comp, plain, sc, ov.Stamps(), ov.Art())
	}
	return res, nil
}

// CheckComposite 逐像素比較合成圖與原版放大圖：差異只能落在遮蓋 stamp（Font == nil）的框，
// 或存讀檔對話框圖面的字帶（004 §3.3）× 倍率內。
func CheckComposite(comp, plain []byte, scale int, stamps []*xlate.Stamp, art []ArtRect) CompositeCheck {
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
		for _, a := range art {
			if x >= a.X && x < a.X+a.W && y >= a.Y && y < a.Y+a.H {
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
