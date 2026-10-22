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
	// Subtitles 是 010 的語音字幕（nil 為不掛）。BeforeStep 在 Labels 之後呼叫它的 Observe，Frame 之後呼叫它的 Frame，
	// SBDMA 事件轉給它（010 §3.12）。
	Subtitles *Subtitles
	// OnSBDMA 在每筆 SB16 DMA 事件呼叫（010 §4.2 前置步驟的事件紀錄）；可為 nil，不得讀記憶體。
	OnSBDMA func(e machine.SBDMAEvent)
	// Flight 是 009 的飛行字串觀察器（nil 為不掛）。BeforeStep 在 Labels 之後呼叫它的 Observe；
	// Frame 依序：OnFrame → Flight.Frame → 更新 LastIndexed → OnFlightFrame（009 §3.11 接點第 2 條）。
	Flight *FlightObserver
	// OnFlightFrame 在 Flight.Frame 與更新 LastIndexed 之後呼叫；steps 是最後一次 BeforeStep 的步數。可為 nil。
	OnFlightFrame func(indexed []byte, steps uint64)

	// afterStep 在 BeforeStep 的三個 Observe 之後呼叫（strike_commander 規格 014 §4.5 N1 的合成點；只供曝光收據）。可為 nil。
	afterStep func(steps uint64)

	steps uint64
	// cur 是這一步的 view。BeforeStep 以 &cur 傳給三個觀察器：StepView 是 16 bytes 的值型別，
	// 直接轉成介面每次都在堆上配置一份（每步三次），指標轉介面不配置（docs/spec/456-sc-observer-view-alloc）。
	// 觀察器不得保留 view（session 238），所以讀到的永遠是本次 BeforeStep 的 v。介面的動態型別因此是
	// *session.StepView，下游不得對它做 session.StepView 的型別斷言（456 §3）。
	cur session.StepView
}

var (
	_ View                  = session.StepView{}
	_ FlightView            = session.StepView{}
	_ SubtitleView          = session.StepView{}
	_ session.StepObserver  = (*SessionObserver)(nil)
	_ session.SBDMAObserver = (*SessionObserver)(nil)
)

// SBDMA 實作 dosgolem 452 的 SBDMAObserver：只要裝了觀察器，每次 Advance 都會安裝 OnSBDMA（452 §2.3），
// 不論 Subtitles 是否為 nil。
func (s *SessionObserver) SBDMA(e machine.SBDMAEvent) {
	if s.OnSBDMA != nil {
		s.OnSBDMA(e)
	}
	if s.Subtitles != nil {
		s.Subtitles.SBDMA(e)
	}
}

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
	s.cur = v
	p := &s.cur
	s.Labels.Observe(p)
	if s.Flight != nil {
		s.Flight.Observe(p)
	}
	if s.Subtitles != nil {
		s.Subtitles.Observe(p)
	}
	if s.afterStep != nil {
		s.afterStep(s.steps)
	}
	return nil
}

func (s *SessionObserver) VideoWrite(machine.VideoWrite) {}

func (s *SessionObserver) Frame(indexed []byte, palette [256][3]uint8) {
	if s.OnFrame != nil {
		s.OnFrame(s.LastIndexed, indexed, s.steps)
	}
	if s.Flight != nil {
		s.Flight.Frame(indexed, s.steps)
	}
	s.LastIndexed, s.LastPalette = indexed, palette
	s.Frames++
	if s.OnFlightFrame != nil {
		s.OnFlightFrame(indexed, s.steps)
	}
	if s.Subtitles != nil {
		s.Subtitles.Frame(indexed, s.steps)
	}
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
	// Fonts 是反鋸齒圖集（strike_commander 規格 011 §3.2、§3.9）：標籤、計分板、圖面、飛行與字幕的中文都以它繪製。
	// Overlay 開時必要，nil 回錯（011 §4.1 第 14 項）。
	Fonts   *TextFonts
	Overlay bool // false：不裝觀察器（同狀態 A/B 的關）
	CursorY int  // 滑鼠游標中心列，負值為未知
	// Inputs、Shots 是腳本事件（002 §2.4）；StopSteps 取腳本的 stop。
	// 觀察器開、關兩次執行都照同一份事件推進；Overlay 關時 shot 只切分推進、不出圖。
	Inputs []ScriptedInput
	Shots  []Shot

	// 010 §3.12：語音字幕。Subtitles 為 false 時與現況相同；為 true 時要 Overlay 開，並給 SpeechIndex。
	// SubtitleE、SubtitleT 是 §3.4、§3.8 的 E 與 T（步數；T 為 0 時取 SubtitleDefaultT）。
	SpeechIndex   *SpeechIndex
	SpeechCatalog map[string]string
	Subtitles     bool
	SubtitleE     uint64
	SubtitleT     uint64
	// SubtitlesOff 讓字幕以 SetEnabled(false) 開始（010 §3.10 的開關；只供驗收）。
	SubtitlesOff bool
	// RecordSBDMA 在 Overlay 開時記下全部 SB16 DMA 事件到 ReceiptResult.SBDMA（010 §4.2 前置步驟）。
	RecordSBDMA bool

	// 009 §3.11 接點第 4 條：FlightCatalogs 為 nil 時不裝 FlightObserver。
	FlightCatalogs *FlightCatalogs

	// expo 非 nil 時另做 strike_commander 規格 014 §4.2 的英文曝光收據（分期 (b)：基線，現行合成）；要 Overlay 開。
	// 結果在 ReceiptResult.Expo。nil 時與現行相同。
	expo *ExpoConfig

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

	// 009 §3.11 接點第 5 條。Digest 在 shot 處理一開始取（Overlay 關也有）；其餘只在裝了 FlightObserver 時有值。
	Digest          session.StateDigest
	FlightVisible   []string
	FlightHidden    map[int][]string
	FlightCovers    map[int][]FlightCover
	FlightReport    map[int][]FlightEntry
	FlightCounts    FlightCounts
	FlightVGAWindow bool
	FlightGen       uint64

	// strike_commander 規格 011 §3.8：各倍率的 arts.tsv 內容；掛字幕時各倍率的字幕區檢查。
	Arts           map[int][]ArtReportEntry
	SubtitleRegion map[int]SubtitleRegionCheck
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

	// 以下是 010 的新增欄位，只在 Overlay 開且對應選項開時有值。
	// Subs、SubSkips 是字幕條與 skip／miss（010 §3.11）；SubShots 與 Shots 平行，是各 shot 的字幕 stamp。
	// shot 與停止點的 Composites 已畫上字幕；Check、CompositeCheck 在畫字幕之前算（只涵蓋 Overlay 自己的 stamp）。
	Subs     []*Subtitle
	SubSkips []SubSkip
	SubShots []SubShot
	// SBDMA 是 RecordSBDMA 開時記下的全部 SB16 DMA 事件（452）。
	SBDMA []machine.SBDMAEvent
	// DOSMouse 是停止時的 DOS 滑鼠狀態（451，010 §4.2 案例 M）。
	DOSMouse session.DOSMouseState

	// 以下是 009 的新增欄位（§3.11 接點第 5 條）。ShotDigests 與 ReceiptConfig.Shots 平行，Overlay 關也填；
	// 其餘只在裝了 FlightObserver 時有值，不併入 Commits、Stamps。
	ShotDigests    []session.StateDigest
	FlightCommits  []FlightCommit
	FlightStamps   [][]CommitStamps
	FlightLog      []Event
	FlightLogDrop  int
	FlightCounts   FlightCounts
	FlightOverlays map[int]*FlightOverlay
	// FlightReport、FlightHidden 是停止點各倍率 Draw 之後的 Report 與 Hidden。
	FlightReport map[int][]FlightEntry
	FlightHidden map[int][]string
	// FlightCUFrames 是 CU 期間的每次回掃：步數與各倍率比對不符的筆數（§3.11 接點第 2 條）。
	FlightCUFrames []FlightCUFrame
	// FlightFirstBlocked 是每次升格後第一個 Frame 各倍率被擋下的筆數（與 FlightCommits 平行）。
	FlightFirstBlocked []map[int]int

	// strike_commander 規格 011 §3.8：停止點的 arts.tsv 內容與字幕區檢查（掛字幕時）。
	StopArts           map[int][]ArtReportEntry
	StopSubtitleRegion map[int]SubtitleRegionCheck

	// Expo 是 strike_commander 規格 014 §4.2 的曝光收據（ReceiptConfig.expo 非 nil 時）。
	Expo *ExpoResult
}

// FlightCUFrame 是 CU 期間的一次回掃（009 §4.2 的「CU 期間比對不符筆數（逐次）」）。
type FlightCUFrame struct {
	Steps    uint64
	Visible  int
	Mismatch map[int]int
	Keys     map[int][]string
}

// SubShot 是一個 shot 的字幕狀態（010 §3.11 的 substamps.tsv）。
type SubShot struct {
	FrameStep uint64
	Enabled   bool
	Blocked   bool
	Key       string            // 畫出的字幕條 key；沒有時為空
	Lines     map[int][]SubLine // 各倍率畫出的行
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
	// 009：飛行字串的觀察器與各倍率 overlay（FlightCatalogs 為 nil 時都不建）。
	var flight *FlightObserver
	fovs := map[int]*FlightOverlay{}
	var expo *expoRun // 規格 014 §4.2（cfg.expo 非 nil 時）
	var err error
	if cfg.Fonts == nil {
		// strike_commander 規格 011 §4.1 第 14 項：沒有圖集不跑。
		return nil, errors.New("strikecommander: 收據缺反鋸齒圖集（Fonts）")
	}
	if cfg.Overlay {
		lo := NewLabelObserver()
		lo.InkCheck, lo.CursorY, lo.noCallerCheck, lo.noBoardCaller = true, cfg.CursorY, cfg.noCallerCheck, cfg.noBoardCaller
		for _, sc := range cfg.Scales {
			ov, err := NewOverlayText(cfg.Fonts, cfg.Catalog, cfg.GfxCatalog, sc)
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
				counts = append(counts, ov.commitStamps(sc)) // 011 §3.9
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
			// 檔別要帶上，artKey 才算得對（008 §3.1）。
			res.Arts = append(res.Arts, ArtEvent{Steps: ev.Steps, File: ev.File, G: ev.G, K: ev.K, X: ev.X, Y: ev.Y, Order: ev.Order, W: ev.W, H: ev.H, Line: ev.Line})
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
		if cfg.FlightCatalogs != nil {
			if err := attachFlight(cfg, res, so, fovs); err != nil {
				return nil, err
			}
			flight = so.Flight
		}
		// 010 §3.12：Subtitles 隨 Owner 建立，不跨 BootFromState。
		if cfg.Subtitles {
			subs, err := NewSubtitlesText(cfg.SpeechIndex, cfg.SpeechCatalog, cfg.Fonts, cfg.SubtitleE, cfg.SubtitleT) // 011 §3.9
			if err != nil {
				return nil, err
			}
			subs.SetEnabled(!cfg.SubtitlesOff)
			so.Subtitles = subs
		}
		if cfg.RecordSBDMA {
			so.OnSBDMA = func(e machine.SBDMAEvent) { res.SBDMA = append(res.SBDMA, e) }
		}
		if cfg.expo != nil {
			// 規格 014 §4.2：曝光收據在既有接線之後串接（不取代），自有實例不碰收據的 overlays、fovs。
			if expo, err = attachExpo(cfg, so, overlays, fovs, &lastMove); err != nil {
				return nil, err
			}
			res.Expo = expo.res
		}
	} else if cfg.Subtitles {
		return nil, errors.New("strikecommander: 字幕要 Overlay 開")
	} else if cfg.expo != nil {
		return nil, errors.New("strikecommander: 曝光收據要 Overlay 開")
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
		// 規格 014 §4.2.1：合成點只切分推進。每段 Advance 前要有一次 Deliver（session 的回合規則）：第一段沿用呼叫端
		// 已交付的回合（空更新或輸入），之後各段交付空更新，同 shot 的切分。合成點 c 在 (steps, target]，同步數的合成點
		// 排在該步的事件之前。expo 為 nil 時只走下面原有的一次 Advance。
		if expo != nil {
			delivered := true
			for {
				c, ok := expo.nextPoint(steps, target)
				if !ok {
					break
				}
				if !delivered {
					if err := deliver(nil); err != nil {
						return err
					}
				}
				r, err := o.Advance(session.InstructionBudget(c - steps))
				if err != nil {
					return fmt.Errorf("strikecommander: Advance（停在第 %d 步）：%w", r.MachineStepsAfter, err)
				}
				if r.MachineStepsAfter != c {
					return fmt.Errorf("strikecommander: 停在第 %d 步，要 %d", r.MachineStepsAfter, c)
				}
				steps, delivered = c, false
				expo.checkpoint(c)
			}
			if target == steps {
				return nil
			}
			if !delivered {
				if err := deliver(nil); err != nil {
					return err
				}
			}
		}
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
			// 009 §3.11 接點第 5 條：Digest 在 shot 處理一開始取，Overlay 關也取。
			d, err := o.Digest()
			if err != nil {
				return res, err
			}
			res.ShotDigests = append(res.ShotDigests, d)
			// 3. shot：只在 Overlay 開時出圖；關時只切分推進，與開的那次相同。
			if !cfg.Overlay {
				continue
			}
			if so.LastIndexed == nil {
				return res, fmt.Errorf("strikecommander: 第 %d 步 shot：還沒有回掃畫格", it.Steps)
			}
			sr := ShotResult{Steps: it.Steps, Name: it.Name, Indexed: append([]byte(nil), so.LastIndexed...), Palette: so.LastPalette, Composites: map[int][]byte{}, Plain: map[int][]byte{}, Check: map[int]CompositeCheck{}, ArtKeys: map[int][]string{}, Hidden: map[int][]string{}, Covers: map[int]map[string][4]int{}, BG: map[int][]BGEntry{}}
			sr.Digest = d
			for _, l := range so.Labels.Visible() {
				sr.Visible = append(sr.Visible, l.Key)
			}
			if flight != nil {
				for _, t := range flight.Visible() {
					sr.FlightVisible = append(sr.FlightVisible, t.Key)
				}
				sr.FlightCounts, sr.FlightVGAWindow, sr.FlightGen = flight.Counts(), flight.VGAWindow(), flight.Generation()
				sr.FlightHidden, sr.FlightCovers, sr.FlightReport = map[int][]string{}, map[int][]FlightCover{}, map[int][]FlightEntry{}
			}
			for _, sc := range cfg.Scales {
				// 009 §3.11 接點第 4 條：Overlay.Draw → fov.Sync、fov.Draw → CheckComposite → 010 的字幕（composeScale，012 §3.8.3）。
				in := scaleIn{Overlay: overlays[sc], LabelsV: so.Labels.Visible(), LabelsG: so.Labels.Generation(), Scale: sc, Frame: so.LastIndexed, Palette: so.LastPalette}
				if fov := fovs[sc]; fov != nil {
					in.Flight, in.FlightV, in.FlightG = fov, flight.Visible(), flight.Generation()
				}
				storeShotScale(&sr, sc, composeScale(in), in.Flight != nil)
			}
			if so.Subtitles != nil {
				ss, region := drawSubtitlesChecked(so.Subtitles, cfg.Scales, sr.Composites)
				res.SubShots = append(res.SubShots, ss)
				sr.SubtitleRegion = region
			}
			res.Shots = append(res.Shots, sr)
		}
	}
	if res.Digest, err = o.Digest(); err != nil {
		return res, err
	}
	if expo != nil {
		expo.finish(res.Digest)
	}
	if cfg.Subtitles || cfg.RecordSBDMA {
		if res.DOSMouse, err = o.DOSMouse(); err != nil {
			return res, err
		}
	}
	if !cfg.Overlay {
		return res, nil
	}
	if so.Subtitles != nil {
		so.Subtitles.Finish()
		res.Subs, res.SubSkips = so.Subtitles.Subs, so.Subtitles.Skips
	}
	res.Log, res.LogDropped = so.Labels.Log()
	res.PartialBlits = so.Labels.PartialBlits()
	if flight != nil {
		res.FlightLog, res.FlightLogDrop = flight.Log()
		res.FlightCounts = flight.Counts()
		res.FlightOverlays = fovs
		res.FlightReport, res.FlightHidden = map[int][]FlightEntry{}, map[int][]string{}
	}
	if so.LastIndexed == nil {
		return res, errors.New("strikecommander: 沒有收到任何回掃畫格")
	}
	res.Frame, res.Palette = so.LastIndexed, so.LastPalette
	res.FrameMatches = sha256.Sum256(res.Frame) == res.Digest.IndexedSHA256
	res.Overlays = overlays
	res.Composites, res.Plain, res.CompositeCheck = map[int][]byte{}, map[int][]byte{}, map[int]CompositeCheck{}
	res.BG = map[int][]BGEntry{}
	for _, sc := range cfg.Scales {
		in := scaleIn{Overlay: overlays[sc], LabelsV: so.Labels.Visible(), LabelsG: so.Labels.Generation(), Scale: sc, Frame: res.Frame, Palette: res.Palette}
		if fov := fovs[sc]; fov != nil {
			in.Flight, in.FlightV, in.FlightG = fov, flight.Visible(), flight.Generation()
		}
		storeStopScale(res, sc, composeScale(in), in.Flight != nil)
	}
	if so.Subtitles != nil {
		_, res.StopSubtitleRegion = drawSubtitlesChecked(so.Subtitles, cfg.Scales, res.Composites)
	}
	return res, nil
}

// drawSubtitlesChecked 同 drawSubtitles；先存各倍率合成圖的複本 pre，畫完以 CheckSubtitleRegion
// 比對（strike_commander 規格 011 §3.9），回各倍率的結果。
func drawSubtitlesChecked(s *Subtitles, scales []int, comps map[int][]byte) (SubShot, map[int]SubtitleRegionCheck) {
	pre := map[int][]byte{}
	for _, sc := range scales {
		pre[sc] = append([]byte(nil), comps[sc]...)
	}
	visible := s.Visible() != nil
	ss := drawSubtitles(s, scales, comps)
	out := map[int]SubtitleRegionCheck{}
	for _, sc := range scales {
		out[sc] = CheckSubtitleRegion(comps[sc], pre[sc], sc, subShotInks(ss, sc), visible)
	}
	return ss, out
}

// drawSubtitles 在各倍率的合成圖（Overlay.Draw 的輸出）上畫字幕（010 §3.7、§3.12），回這一刻的字幕狀態。
func drawSubtitles(s *Subtitles, scales []int, comps map[int][]byte) SubShot {
	ss := SubShot{FrameStep: s.FrameStep(), Enabled: s.Enabled(), Blocked: s.Blocked(), Lines: map[int][]SubLine{}}
	if c := s.Visible(); c != nil {
		ss.Key = c.Key
	}
	for _, sc := range scales {
		ss.Lines[sc] = s.Draw(comps[sc], sc)
	}
	return ss
}

// CheckComposite 逐像素比較合成圖與原版放大圖：差異只能落在遮蓋 stamp（Font == nil）的框，
// 或存讀檔對話框圖面的字帶（004 §3.3）、飛行字串的遮蓋範圍（009 §3.11 接點第 6 條）× 倍率內。
func CheckComposite(comp, plain []byte, scale int, stamps []*xlate.Stamp, art []ArtRect, flight ...FlightCover) CompositeCheck {
	return CheckCompositeText(comp, plain, scale, stamps, art, nil, flight...)
}

// CheckCompositeText 同 CheckComposite，另把照畫的串的文字框（輸出像素，strike_commander 規格 011 §3.5 允許差異區）
// 算進允許區：像素的 1 倍座標落在遮蓋 stamp、圖面或飛行遮蓋範圍內，或輸出像素座標落在某個文字框內，就算允許。
// 它是 CheckCompositeMask(…, nil, …) 的包裝（012 §3.8.3）。
func CheckCompositeText(comp, plain []byte, scale int, stamps []*xlate.Stamp, art []ArtRect, text []xlate.AlphaRect, flight ...FlightCover) CompositeCheck {
	return CheckCompositeMask(comp, plain, scale, stamps, art, text, nil, flight...)
}

// CheckCompositeMask 同 CheckCompositeText，另以 allow（nil 或長度 64000，1 倍像素）放行該像素對應的 s×s 輸出像素
// （strike_commander 規格 012 §3.8.3）：對每個不同的輸出像素先查 allow[(oy/s)×320 + ox/s]，為真就算允許、不再查矩形。
// allow 為 nil 時與 CheckCompositeText 的原行為相同。allow 非 nil 而長度不是 64000 時 panic（呼叫端錯誤）。
func CheckCompositeMask(comp, plain []byte, scale int, stamps []*xlate.Stamp, art []ArtRect, text []xlate.AlphaRect, allow []bool, flight ...FlightCover) CompositeCheck {
	if allow != nil && len(allow) != 64000 {
		panic(fmt.Sprintf("strikecommander: CheckCompositeMask 的 allow 長度 %d，要 64000", len(allow)))
	}
	var c CompositeCheck
	w := 320 * scale
	for i := 0; i+3 < len(comp) && i+3 < len(plain); i += 4 {
		if comp[i] == plain[i] && comp[i+1] == plain[i+1] && comp[i+2] == plain[i+2] && comp[i+3] == plain[i+3] {
			continue
		}
		c.Diffs++
		ox, oy := (i/4)%w, (i/4)/w
		x, y := ox/scale, oy/scale
		if allow != nil {
			if k := y*320 + x; k < len(allow) && allow[k] {
				continue
			}
		}
		in := false
		for _, b := range text {
			if ox >= b.X0 && ox < b.X1 && oy >= b.Y0 && oy < b.Y1 {
				in = true
				break
			}
		}
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
		for _, f := range flight {
			if x >= f.X0 && x < f.X1 && y >= f.Y0 && y < f.Y1 {
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
