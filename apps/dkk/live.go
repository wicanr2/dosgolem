package dkk

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// StepCPU 是每道指令前攔截點需要的機器狀態（session.StepView 與收據工具都滿足）。
type StepCPU interface {
	CPUView
	Steps() uint64
	CS() uint16
	IP() uint16
	SS() uint16
	SP() uint16
	DS() uint16
}

// Live 把路徑 A、B、C 組在同一個 xlate Layer 上（規格 001–003）。
//
// 攔截點的位址以簽章搜尋，但要等遊戲本體就位才搜：冷開機的第 0 步，記憶體裡的
// START.EXE 還沒自我展開，遮罩簽章會對上一段不是程序本身的位元組（2026-10-15 前端實測：
// 綁在 #0 的位址被執行時參數全不合法）。因此綁定在第一次視訊模式改變之後
// （ModeChange），或由呼叫端在已進入遊戲的狀態檔上呼叫 Arm。失敗就每 bindRetry 步再試；
// 搜尋結果不唯一就不綁定、不猜位址；最後一次失敗原因留在 BindErr。
type Live struct {
	Layer *xlate.Layer
	Font  *xlate.Font
	A     *PathA
	B     *PathB
	C     *PathC

	// Answers 是查驗題答案表（規格 006）；綁定前設定。
	Answers     map[QuizKey]string
	QuizFormat  QuizFormat
	hideAnswers bool
	// Journal 是手札面板（規格 007）；nil 時不啟用。
	Journal *JournalPanel
	// Guide 是攻略面板（規格 008）；nil 時不啟用。
	Guide *GuidePanel
	// Map 是地圖疊層（規格 009）；nil 時不啟用。
	Map *AutoMap
	ds  uint16 // 遊戲資料段（路徑 A 進入時取）
	// Inventory 為 true 時，綁定後建立 Inv 記錄印字路徑盤點（issue #11）。
	Inventory bool
	Inv       *Inventory

	ui      Lookuper
	ecl     *Catalog
	ready   bool
	armed   bool
	BindErr error  // 最近一次綁定失敗的原因；綁定成功後為 nil
	BoundAt uint64 // 綁定成功的步數
	indexed []byte
	pal     [256][3]uint8
}

// NewLive 建立尚未綁定位址的覆繪；ui 是介面字串查表（可為規格 004 的 Translator），
// ecl 可為 nil（不啟用路徑 B）。
func NewLive(font *xlate.Font, ui Lookuper, ecl *Catalog) *Live {
	AddHalfWidthPairs(font)
	AddScaledVariants(font)
	return &Live{Layer: &xlate.Layer{W: 320, H: 200}, Font: font, ui: ui, ecl: ecl}
}

const bindRetry = 1_000_000

// defaultDS 是 START.EXE 1.10 的資料段（docs/re/009、011 實測），路徑 A 進入時以當下 DS 更新。
const defaultDS = 0x0CDE

// mapSampleEvery 是地圖疊層讀記憶體的間隔（規格 009 §2.2）。
const mapSampleEvery = 1_000_000

// Arm 允許下一步起綁定攔截點（從已進入遊戲的狀態檔開始時使用）。
func (l *Live) Arm() { l.armed = true }

func (l *Live) bind(mem CPUView) error {
	var err error
	l.A, l.B, l.C = nil, nil, nil
	l.Layer.Frozen = nil
	if l.A, err = NewPathA(mem, l.ui, l.Font, l.Layer); err != nil {
		return err
	}
	l.A.Answers, l.A.QuizFormat, l.A.HideAnswers = l.Answers, l.QuizFormat, l.hideAnswers
	if l.C, err = NewPathC(mem, l.ui, l.Font, l.Layer); err != nil {
		return err
	}
	if l.ecl != nil {
		if l.B, err = NewPathB(mem, l.Font, l.Layer, l.ecl, l.ui); err != nil {
			return err
		}
		l.B.OnEntry = l.journalEntry
	}
	l.ready = true
	l.ds = defaultDS
	if l.Inventory {
		l.Inv = newInventory(l)
	}
	return nil
}

// BeforeStep 在每道指令前呼叫。
func (l *Live) BeforeStep(v StepCPU) error {
	if !l.ready {
		if !l.armed || l.BindErr != nil && v.Steps()%bindRetry != 0 {
			return nil
		}
		if err := l.bind(v); err != nil {
			l.BindErr = fmt.Errorf("dkk: 綁定攔截點（#%d）：%w", v.Steps(), err)
			return nil
		}
		l.BindErr, l.BoundAt = nil, v.Steps()
	}
	if l.Journal.Open() {
		l.Journal.KeyTaken(v.Read16(bdaKeyHead)) // 規格 007 §2.4 條件 2：先比對再處理進入點
	}
	cs, ip, ss, sp := v.CS(), v.IP(), v.SS(), v.SP()
	if l.Map != nil {
		if linear(cs, ip) == l.A.Entry {
			l.ds = v.DS()
		}
		if l.ds != 0 && v.Steps()%mapSampleEvery == 0 {
			l.Map.Sample(v, l.ds)
		}
	}
	na, nb := len(l.A.Hits), 0
	if l.Inv != nil {
		l.Inv.before(v.Steps(), cs, ip, ss, sp, v)
	}
	l.A.Before(v.Steps(), cs, ip, ss, sp, v)
	l.C.Before(cs, ip, ss, sp, v)
	if l.B != nil {
		nb = len(l.B.Hits)
		l.B.Before(v.Steps(), cs, ip, ss, sp, v.DS(), v)
	}
	if l.Inv != nil {
		if len(l.A.Hits) > na {
			l.Inv.attach("A", l.A.Hits[len(l.A.Hits)-1])
		}
		if l.B != nil && len(l.B.Hits) > nb {
			l.Inv.attach("B", l.B.Hits[len(l.B.Hits)-1])
		}
	}
	return nil
}

// VideoWrite 處理 A000 寫入（offset 相對 A000）。
func (l *Live) VideoWrite(offset uint32) {
	if !l.ready {
		return
	}
	l.C.VideoWrite(offset)
	if l.B != nil {
		l.B.VideoWrite(offset)
	}
}

// SetHideAnswers 切換查驗題答案顯示（規格 006 §2.2）。
func (l *Live) SetHideAnswers(hide bool) {
	l.hideAnswers = hide
	if l.A != nil {
		l.A.SetHideAnswers(hide)
	}
}

// HideAnswers 回目前是否隱藏查驗題答案。
func (l *Live) HideAnswers() bool { return l.hideAnswers }

// bdaKeyHead 是 BIOS 鍵盤緩衝區 head 指標（0040:001A）。
const bdaKeyHead = 0x41A

// journalEntry 是路徑 B 進入時的手札處理（規格 007 §2.2、§2.4 條件 1）。
func (l *Live) journalEntry(text []byte, mem CPUView) {
	if l.Journal == nil {
		return
	}
	l.Journal.CloseAuto()
	if n, ok := JournalEntryOf(text); ok {
		l.Journal.OpenAuto(n, mem.Read16(bdaKeyHead))
	}
}

// ModeChange 在視訊模式改變時呼叫。
func (l *Live) ModeChange() {
	l.Journal.closeAutoSafe()
	if !l.ready {
		l.armed = true
		return
	}
	l.C.Reset()
	if l.B != nil {
		l.B.Reset()
	}
}

// Frame 在每個 retrace 呼叫；palette 須是與 indexed 配對的色盤（machine.FramePalette）。
func (l *Live) Frame(indexed []byte, palette [256][3]uint8) {
	l.indexed = append(l.indexed[:0], indexed...)
	l.pal = palette
	if len(indexed) != 320*200 {
		return
	}
	// 綁定前也跑 Layer.Frame：HD 圖面（hd.go）的標題畫面出現在綁定之前，那時還沒有疊字。
	if l.ready {
		l.A.PrepareFrame(indexed, 320, 200)
		l.C.SetPalette(palette)
		if l.B != nil {
			l.B.SetPalette(palette)
		}
	} else if l.Layer.Watchers() == 0 {
		return
	}
	rgb := make([]uint8, len(indexed)*3)
	for i, c := range indexed {
		rgb[i*3], rgb[i*3+1], rgb[i*3+2] = palette[c][0], palette[c][1], palette[c][2]
	}
	l.Layer.Frame(indexed, rgb)
}

// Compose 回最後一個畫格放大 scale 倍的 RGBA；overlay 為 false 時是原版畫面。
// missing 收字型缺字（可為 nil）。尚無 320×200 畫格時回 nil。
func (l *Live) Compose(scale int, overlay bool, missing func(rune)) []byte {
	if len(l.indexed) != 320*200 || scale <= 0 {
		return nil
	}
	w := 320 * scale
	out := make([]uint8, w*200*scale*4)
	for y := 0; y < 200*scale; y++ {
		row := l.indexed[(y/scale)*320:]
		for x := 0; x < w; x++ {
			c := l.pal[row[x/scale]]
			o := (y*w + x) * 4
			out[o], out[o+1], out[o+2], out[o+3] = c[0], c[1], c[2], 0xFF
		}
	}
	if overlay {
		l.Layer.Draw(out, scale, missing)
		if l.Journal.Open() {
			l.Journal.Draw(out, scale, l.Font, missing)
		}
		if l.Guide.Open() {
			l.Guide.Draw(out, scale, l.Font, missing)
		}
		if l.Map.Open() {
			l.Map.Draw(out, scale, l.Font, missing)
		}
	}
	return out
}

// Indexed 回最後一個畫格的原版色號（收據用）。
func (l *Live) Indexed() []byte { return l.indexed }

// AddScaledVariants 為 16×16 字型登記 3× 變體：最近鄰放成 22×22，置於 24 像素格內偏移 (1,1)，
// 比照拯救地球（apps/buckrogers manualThreeXFont、manualGlyphOffset）。
// 規格見 death-knights-of-krynn-cht docs/spec/010 §2.3；xlate spec 202 §2.4.1。須在半形字對登記之後呼叫。
func AddScaledVariants(font *xlate.Font) {
	if font == nil || font.W != 16 || font.H != 16 {
		return
	}
	font.Scaled = map[int]xlate.ScaledFont{3: {Font: xlate.DeriveNearest(font, 22, 22), X: 1, Y: 1}}
}
