package dos

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sort"
)

// 存檔／讀檔：DOS 這一層的狀態。與 `machine.SaveState` 合起來才是一台
// 完整的機器——**光有記憶體與 CPU 不夠**，開著的檔、配出去的記憶體區塊、
// EMS 的映射、XMS 的 EMB 內容都在 Go 這一端，漏掉任何一項，還原之後
// 程式讀到的是別的東西而不會有任何錯誤。
//
// 診斷用的紀錄（`Opened`／`Reads`／`Allocs`／`EMSOps`／`ExecLog`…）
// **不存**：它們是這一趟的觀測紀錄，不是機器狀態。

// v3 加了 XMS 的 HMA 擁有權與區域 A20 計數（`docs/spec/191-bios-rom-tail` §4）；
// v2 仍然讀，缺的兩項讀成零值（HMA 沒人拿、計數 0），與 v2 執行器讀回來相同。
const (
	dosStateMagic      = "DOSGOLEM-D"
	dosStateVersion    = 3
	dosStateVersionMin = 2
)

type handleState struct {
	H    uint16
	Name string
	Path string
	Off  int64
	Size int64
	// PSP 是開這個檔的行程；子行程結束時要靠它決定關哪些。
	PSP uint16
}

// embState 是一塊 XMS EMB。存成依 handle 排序的 slice：gob 編 map 的順序隨機，
// 兩次一模一樣的執行會存出不同 bytes（與 machine 的埠統計同一類問題）。
type embState struct {
	H    uint16
	Data []byte
}

// xmsBlockState 是 kb > 0 時一塊實體化 XMS 區塊的線上版本
// （docs/spec/430-physical-memory-and-xms-unification §3）。
// ⚠ 欄位與型別在「只加欄位」的 commit 定案，之後不得再改，
// 否則 gob 的型別描述會第二次改變。
type xmsBlockState struct {
	H        uint16
	BasePage int // 相對 Ext 起點（實體 0x100000）的 4 KB 頁號；0 頁區塊為 0
	Pages    int
	KB       int
	Locks    int
}

type emsHandleState struct {
	H     uint16
	Pages [][]byte
}

type emsMapState struct {
	Used bool
	H    uint16
	Page uint16
}

// blockState 是 memBlock 的線上版本（memBlock 的欄位不匯出，gob 看不到）。
type blockState struct {
	Seg, Size uint16
	Free      bool
}

type procState struct {
	R       [8]uint16
	Seg     [4]uint16
	IP, Fl  uint16
	PSP     uint16
	FreeSeg uint16
	// IVT 是 DOS 保管的 22h/23h/24h。**漏掉不會報錯**：讀檔之後
	// 子行程結束時 Ctrl-Break 向量會被還原成 0。
	IVT [3][2]uint16
}

// mouseExtra 是狀態檔裡滑鼠的設定部分（事件常式、範圍、最後按放位置）。
// 另開一個型別是因為 Mouse 本身帶著不存的觀測紀錄。
type mouseExtra struct {
	Handler struct {
		Seg, Off, Mask uint16
		Set            bool
	}
	PressAt, ReleaseAt     [3][2]uint16
	MinX, MaxX, MinY, MaxY uint16
}

type dosState struct {
	Magic   string
	Version int

	Drive uint8
	Dir   string
	// RootDir 是 Root 在 DOS 樹上的位置（`docs/spec/198`）。舊檔沒有它：
	// 讀進來是空字串時沿用目前這一台的設定。
	RootDir string
	Now     Time
	Mouse   Mouse
	// MouseExtra 是 Mouse 以外要留的滑鼠設定（`docs/spec/196-mouse-handler-survives-state-file`）。
	// **漏掉 Handler 的話讀檔之後注入的輸入不會呼叫事件常式**，
	// 靠它畫游標的遊戲看起來像滑鼠沒接上。舊檔讀進來是零值。
	MouseExtra mouseExtra

	// Root 是原版素材的目錄。**一定要存**：讀檔之後遊戲還會再開檔，
	// 少了它 `resolve` 會拿預設的 `.` 去找，之後每一次開檔都失敗——
	// 而且**不會有任何錯誤**，只有緩衝區裡留著填充值，
	// 遊戲照樣把它當資料用（見 `docs/spec/004` §4.18）。
	Root string

	Handles []handleState
	FreeSeg uint16

	// Arena 是配置器的區塊表（`docs/spec/009`）。
	// **漏掉的話讀檔之後配置器是空的**，第一次 AH=48h 會從 freeSeg
	// 重新切，把已經發出去的緩衝區再發一次。
	Arena []blockState

	Stack    []procState
	CurPSP   uint16
	LastExit uint16
	Queue    []Queued

	// EMB 只留著讀舊檔：同名欄位換型別的話 gob 解不開。存檔時是 nil，gob 不寫它。
	EMB     map[uint16][]byte
	EMBList []embState
	NextEMB uint16
	// XMSBlocks：430，kb > 0 時的實體化區塊，依 handle 排序。
	XMSBlocks []xmsBlockState `json:",omitempty"`
	// HMAOwned／A20Local：v3 起才有。
	HMAOwned bool
	A20Local int

	EMSHandles []emsHandleState
	EMSNext    uint16
	EMSFrame   [4]emsMapState
	EMSSaved   [4]emsMapState

	Exited   bool
	ExitCode uint8
}

// SaveState 把 DOS 這一層寫出去。
func (d *DOS) SaveState(w io.Writer) error {
	s := dosState{
		Magic: dosStateMagic, Version: dosStateVersion,
		Drive: d.Drive, Dir: d.Dir, RootDir: d.RootDir, Now: d.Now, Root: d.Root,
		FreeSeg: d.freeSeg,

		CurPSP:   d.curPSP,
		LastExit: d.lastExit,
		Queue:    append([]Queued(nil), d.queue...),
		NextEMB:  d.nextEMB,
		HMAOwned: d.hmaOwned,
		A20Local: d.a20Local,
		Exited:   d.Exited,
		ExitCode: d.ExitCode,
	}
	// 滑鼠驅動的狀態要整份留（`docs/spec/196-mouse-handler-survives-state-file`、
	// `docs/spec/197-mouse-handler-in-saved-state`、logh3 `docs/spec/004` §4.18）：
	// 座標與鍵之外，`AX=000Ch` 的事件常式、`AX=7`／`AX=8` 的座標範圍、
	// `AX=5`／`AX=6` 用的最後按放座標與 `AX=0Bh` 的相對位移。
	// 觀測紀錄（Polls／Sets／Calls／Events／PressQ／PressReads）不留。
	//
	// ⚠ **事件常式掉了的症狀是「展開之後點擊全部沒反應」，而畫面
	// 逐像素相同。** 記憶體、CPU、開著的檔、配出去的區塊全都還原了，
	// 唯一沒還原的東西剛好不影響畫面，只影響之後的輸入——
	// `clickgrid` 掃出來的「這一格沒反應」與「這一區真的沒有熱區」
	// 長得一模一樣。
	//
	// mickey 也存：不中斷地跑下去，下一次 `AX=0Bh` 本來就會拿到這段累計；
	// 歸零的話展開後與從頭跑不再一致。
	//
	// 同一份設定寫兩處：Mouse 本身（logh3、fix/dm-boot 的格式）與 MouseExtra
	// （stubseg 分支的格式），三種舊狀態檔都讀得回來。
	s.Mouse = Mouse{X: d.Mouse.X, Y: d.Mouse.Y, Buttons: d.Mouse.Buttons,
		Press: d.Mouse.Press, Release: d.Mouse.Release, XScale: d.Mouse.XScale,
		PressAt: d.Mouse.PressAt, ReleaseAt: d.Mouse.ReleaseAt,
		MickeyX: d.Mouse.MickeyX, MickeyY: d.Mouse.MickeyY,
		MinX: d.Mouse.MinX, MaxX: d.Mouse.MaxX, MinY: d.Mouse.MinY, MaxY: d.Mouse.MaxY,
		Handler: d.Mouse.Handler}
	s.MouseExtra = mouseExtra{Handler: d.Mouse.Handler, PressAt: d.Mouse.PressAt, ReleaseAt: d.Mouse.ReleaseAt,
		MinX: d.Mouse.MinX, MaxX: d.Mouse.MaxX, MinY: d.Mouse.MinY, MaxY: d.Mouse.MaxY}

	for _, b := range d.arena {
		s.Arena = append(s.Arena, blockState{Seg: b.seg, Size: b.size, Free: b.free})
	}
	for h, fh := range d.handles {
		off, err := fh.f.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("dos: 取不到 %s 的讀寫位置：%w", fh.name, err)
		}
		s.Handles = append(s.Handles, handleState{H: h, Name: fh.name, Path: fh.path, Off: off, Size: fh.size, PSP: fh.psp})
	}
	sort.Slice(s.Handles, func(i, j int) bool { return s.Handles[i].H < s.Handles[j].H })
	if d.M.Ext != nil {
		// kb > 0：區塊內容在 Machine.Ext，這裡只存配置表（規格 430 §3）；不寫 EMBList。
		s.XMSBlocks = d.xmsBlocksState()
	} else {
		for k, v := range d.emb {
			s.EMBList = append(s.EMBList, embState{H: k, Data: append([]byte(nil), v...)})
		}
		sort.Slice(s.EMBList, func(i, j int) bool { return s.EMBList[i].H < s.EMBList[j].H })
	}
	for _, f := range d.procStack {
		s.Stack = append(s.Stack, procState{R: f.r, Seg: f.seg, IP: f.ip, Fl: f.fl,
			PSP: f.psp, FreeSeg: f.freeSeg, IVT: f.ivt})
	}
	if d.ems != nil {
		s.EMSNext = d.ems.next
		for h, eh := range d.ems.handles {
			e := emsHandleState{H: h}
			for _, p := range eh.pages {
				e.Pages = append(e.Pages, append([]byte(nil), p...))
			}
			s.EMSHandles = append(s.EMSHandles, e)
		}
		sort.Slice(s.EMSHandles, func(i, j int) bool { return s.EMSHandles[i].H < s.EMSHandles[j].H })
		for i := 0; i < 4; i++ {
			if m := d.ems.frame[i]; m != nil {
				s.EMSFrame[i] = emsMapState{Used: true, H: m.h, Page: m.page}
			}
			if m := d.ems.saved[i]; m != nil {
				s.EMSSaved[i] = emsMapState{Used: true, H: m.h, Page: m.page}
			}
		}
	}
	return gob.NewEncoder(w).Encode(&s)
}

// LoadState 把 DOS 這一層倒回存檔的狀態。**開著的檔案會重新開一次
// 並 seek 回原位**——存檔裡只有路徑與位置，不含檔案內容（原版素材唯讀）。
func (d *DOS) LoadState(r io.Reader) error {
	var s dosState
	if err := gob.NewDecoder(r).Decode(&s); err != nil {
		return fmt.Errorf("dos: 讀不開狀態檔：%w", err)
	}
	if s.Magic != dosStateMagic || s.Version < dosStateVersionMin || s.Version > dosStateVersion {
		return fmt.Errorf("dos: 狀態檔不認得（%q v%d）", s.Magic, s.Version)
	}
	// 430：XMS 內容與這台機器不相容時，在任何寫入之前失敗。
	if err := d.checkXMSState(&s); err != nil {
		return err
	}
	for _, h := range d.handles {
		h.f.Close()
	}
	d.Drive, d.Dir, d.Now = s.Drive, s.Dir, s.Now
	if s.RootDir != "" {
		d.RootDir = s.RootDir
	}
	// **舊的狀態檔沒有 Root**，那時就沿用目前這一台的設定，
	// 不要把它清成空字串。
	if s.Root != "" {
		d.Root = s.Root
	}
	d.Mouse.X, d.Mouse.Y, d.Mouse.Buttons = s.Mouse.X, s.Mouse.Y, s.Mouse.Buttons
	d.Mouse.Press, d.Mouse.Release, d.Mouse.XScale = s.Mouse.Press, s.Mouse.Release, s.Mouse.XScale
	// 先讀 Mouse 本身（logh3／fix/dm-boot 格式）；MouseExtra 有值就以它為準（stubseg 格式）。
	d.Mouse.Handler = s.Mouse.Handler
	d.Mouse.PressAt, d.Mouse.ReleaseAt = s.Mouse.PressAt, s.Mouse.ReleaseAt
	d.Mouse.MickeyX, d.Mouse.MickeyY = s.Mouse.MickeyX, s.Mouse.MickeyY
	d.Mouse.MinX, d.Mouse.MaxX, d.Mouse.MinY, d.Mouse.MaxY = s.Mouse.MinX, s.Mouse.MaxX, s.Mouse.MinY, s.Mouse.MaxY
	if x := s.MouseExtra; x != (mouseExtra{}) {
		d.Mouse.Handler, d.Mouse.PressAt, d.Mouse.ReleaseAt = x.Handler, x.PressAt, x.ReleaseAt
		d.Mouse.MinX, d.Mouse.MaxX, d.Mouse.MinY, d.Mouse.MaxY = x.MinX, x.MaxX, x.MinY, x.MaxY
	}
	d.freeSeg = s.FreeSeg
	d.curPSP, d.lastExit = s.CurPSP, s.LastExit
	d.queue = append([]Queued(nil), s.Queue...)
	d.arena = nil
	for _, b := range s.Arena {
		d.arena = append(d.arena, memBlock{seg: b.Seg, size: b.Size, free: b.Free})
	}
	d.Exited, d.ExitCode = s.Exited, s.ExitCode

	d.handles = map[uint16]*handle{}
	for _, hs := range s.Handles {
		f, err := os.Open(hs.Path)
		if err != nil {
			return fmt.Errorf("dos: 還原時開不了 %s：%w", hs.Path, err)
		}
		if _, err := f.Seek(hs.Off, io.SeekStart); err != nil {
			f.Close()
			return fmt.Errorf("dos: 還原時 seek 不了 %s：%w", hs.Path, err)
		}
		d.handles[hs.H] = &handle{name: hs.Name, path: hs.Path, f: f, size: hs.Size, psp: hs.PSP, refs: 1}
	}
	d.emb = map[uint16][]byte{}
	for k, v := range s.EMB { // 舊檔
		d.emb[k] = append([]byte(nil), v...)
	}
	for _, e := range s.EMBList {
		d.emb[e.H] = append([]byte(nil), e.Data...)
	}
	d.xmsBlocks = nil
	for _, b := range s.XMSBlocks {
		if d.xmsBlocks == nil {
			d.xmsBlocks = map[uint16]*xmsBlock{}
		}
		d.xmsBlocks[b.H] = &xmsBlock{base: b.BasePage, pages: b.Pages, kb: b.KB, locks: b.Locks}
	}
	d.nextEMB = s.NextEMB
	d.hmaOwned, d.a20Local = s.HMAOwned, s.A20Local
	d.procStack = nil
	for _, f := range s.Stack {
		d.procStack = append(d.procStack, procFrame{r: f.R, seg: f.Seg, ip: f.IP, fl: f.Fl,
			psp: f.PSP, freeSeg: f.FreeSeg, ivt: f.IVT})
	}
	d.ems = &ems{handles: map[uint16]*emsHandle{}, next: s.EMSNext}
	for _, e := range s.EMSHandles {
		eh := &emsHandle{}
		for _, p := range e.Pages {
			eh.pages = append(eh.pages, append([]byte(nil), p...))
		}
		d.ems.handles[e.H] = eh
	}
	for i := 0; i < 4; i++ {
		if s.EMSFrame[i].Used {
			d.ems.frame[i] = &emsMapping{h: s.EMSFrame[i].H, page: s.EMSFrame[i].Page}
		}
		if s.EMSSaved[i].Used {
			d.ems.saved[i] = &emsMapping{h: s.EMSSaved[i].H, page: s.EMSSaved[i].Page}
		}
	}
	return nil
}
