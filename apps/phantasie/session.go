package phantasie

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// SessionOptions 是 StartSession 的參數。
type SessionOptions struct {
	Root, Bat        string   // 原版目錄與啟動批次檔（空字串＝目錄內唯一的 .BAT）
	TextDir, FontDir string   // text/ 與字型目錄（<lang>.golemfnt）
	Langs            []string // 要載入的語言；第一個是初始顯示語言；"en" 不需載入（關閉疊字）
	Scratch          string   // 可寫狀態目錄（存檔寫在這裡，不寫原版目錄）；空字串＝不設
	ChainBudget      uint64   // 等啟動鏈載入完成的步數上限；0 取預設
}

// Session 是一個執行中的原版加疊字層（docs/spec/005 §3）。
type Session struct {
	O        *oracle.Oracle
	Ov       *Overlay
	Hk       *Hooks
	Gate     *KeyGate
	Img      uint16
	DG       uint16
	Failed   []string // 載入失敗而停用的語言與原因
	Warnings []string // 可選本機資料的診斷，不停用一般語言。
}

// StartSession 啟動原版（照 .BAT 的順序）、載入語言、安裝唯讀鉤子與鍵閘。回傳的 Session 尚未跑進遊戲本體
// 以外的任何指令（只跑到啟動鏈載入完成）。
func StartSession(opt SessionOptions) (*Session, error) {
	o, names, err := Launch(opt.Root, opt.Bat)
	if err != nil {
		return nil, err
	}
	if opt.Scratch != "" {
		o.SetScratch(opt.Scratch)
	}
	budget := opt.ChainBudget
	if budget == 0 {
		budget = 400_000_000
	}
	chain := oracle.NewCond("鏈載入完成", func(o *oracle.Oracle) bool { return len(o.ExecLog()) >= len(names)-1 })
	if err := o.RunUntil(chain, oracle.Budget(budget)); err != nil {
		o.Close()
		return nil, fmt.Errorf("等啟動鏈載入：%w", err)
	}
	img := ImageSeg(o)
	s := &Session{O: o, Img: img, DG: img + DGroupParas}
	s.Ov = NewOverlay()
	first := ""
	for _, name := range opt.Langs {
		if name == "en" {
			continue
		}
		l := LoadLanguage(name, opt.TextDir, opt.FontDir)
		if l.ManualErr != "" {
			s.Warnings = append(s.Warnings, name+"："+l.ManualErr)
		}
		if !l.Enabled {
			s.Failed = append(s.Failed, fmt.Sprintf("%s：%s", name, l.Err))
			continue
		}
		s.Ov.AddLanguage(l)
		if first == "" {
			first = name
		}
	}
	initial := first
	if len(opt.Langs) > 0 && opt.Langs[0] == "en" {
		initial = "en"
	}
	if initial == "" {
		initial = "en"
	}
	if err := s.Ov.SetDisplay(initial); err != nil {
		o.Close()
		return nil, err
	}
	regions, err := DefaultRegions()
	if err != nil {
		o.Close()
		return nil, err
	}
	s.Hk = InstallHooks(o, img, s.Ov, regions)
	s.Gate = NewKeyGate(o, img)
	return s, nil
}

// Close 釋放原版執行環境。
func (s *Session) Close() { s.O.Close() }

// Frame 取目前畫面並對疊字層做一次 Frame 與 recolor。
func (s *Session) Frame() (indexed, rgb []uint8) {
	indexed, rgb = FrameBuffers(s.O)
	s.Ov.Frame(indexed, rgb)
	return indexed, rgb
}

// DisplayCycle 回語言切換鍵的循環順序（zh-TW、zh-CN、en、ja、ko 中已啟用者；en 永遠可用）。
func (s *Session) DisplayCycle() []string {
	var out []string
	for _, n := range []string{"zh-TW", "zh-CN", "en", "ja", "ko"} {
		if n == "en" {
			out = append(out, n)
			continue
		}
		if l := s.Ov.Language(n); l != nil && l.Enabled {
			out = append(out, n)
		}
	}
	return out
}

// NextDisplay 切換到循環中的下一個語言，回傳新的顯示語言。
func (s *Session) NextDisplay() (string, error) {
	cyc := s.DisplayCycle()
	cur := s.Ov.Display()
	next := cyc[0]
	for i, n := range cyc {
		if n == cur {
			next = cyc[(i+1)%len(cyc)]
			break
		}
	}
	return next, s.Ov.SetDisplay(next)
}
