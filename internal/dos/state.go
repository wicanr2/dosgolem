package dos

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
)

// 存檔／讀檔：DOS 這一層的狀態。與 `machine.SaveState` 合起來才是一台
// 完整的機器——**光有記憶體與 CPU 不夠**，開著的檔、配出去的記憶體區塊、
// EMS 的映射、XMS 的 EMB 內容都在 Go 這一端，漏掉任何一項，還原之後
// 程式讀到的是別的東西而不會有任何錯誤。
//
// 診斷用的紀錄（`Opened`／`Reads`／`Allocs`／`EMSOps`／`ExecLog`…）
// **不存**：它們是這一趟的觀測紀錄，不是機器狀態。

const (
	dosStateMagic   = "DOSGOLEM-D"
	dosStateVersion = 1
)

type handleState struct {
	H        uint16
	Name     string
	Path     string
	Off      int64
	Size     int64
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

// holeState 是 memHole 的線上版本（memHole 的欄位不匯出，gob 看不到）。
type holeState struct{ Seg, Size uint16 }

type procState struct {
	R       [8]uint16
	Seg     [4]uint16
	IP, Fl  uint16
	PSP     uint16
	FreeSeg uint16
}

type dosState struct {
	Magic   string
	Version int

	Drive uint8
	Dir   string
	Now   Time
	Mouse Mouse

	Handles    []handleState
	NextHandle uint16
	FreeSeg    uint16

	Blocks map[uint16]uint16
	Holes  []holeState

	Stack    []procState
	CurPSP   uint16
	LastExit uint16
	Queue    []Queued

	EMB     map[uint16][]byte
	NextEMB uint16

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
		Drive: d.Drive, Dir: d.Dir, Now: d.Now,
		NextHandle: d.nextHandle, FreeSeg: d.freeSeg,
		Blocks:   map[uint16]uint16{},

		CurPSP:   d.curPSP,
		LastExit: d.lastExit,
		Queue:    append([]Queued(nil), d.queue...),
		EMB:      map[uint16][]byte{},
		NextEMB:  d.nextEMB,
		Exited:   d.Exited,
		ExitCode: d.ExitCode,
	}
	// 滑鼠的座標與按鍵要留，觀測紀錄不留。
	s.Mouse = Mouse{X: d.Mouse.X, Y: d.Mouse.Y, Buttons: d.Mouse.Buttons,
		Press: d.Mouse.Press, Release: d.Mouse.Release, XScale: d.Mouse.XScale}

	for _, h := range d.holes {
		s.Holes = append(s.Holes, holeState{Seg: h.seg, Size: h.size})
	}
	for h, fh := range d.handles {
		off, err := fh.f.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("dos: 取不到 %s 的讀寫位置：%w", fh.name, err)
		}
		s.Handles = append(s.Handles, handleState{H: h, Name: fh.name, Path: fh.path, Off: off, Size: fh.size})
	}
	for k, v := range d.blocks {
		s.Blocks[k] = v
	}
	for k, v := range d.emb {
		s.EMB[k] = append([]byte(nil), v...)
	}
	for _, f := range d.stack {
		s.Stack = append(s.Stack, procState{R: f.r, Seg: f.seg, IP: f.ip, Fl: f.fl, PSP: f.psp, FreeSeg: f.freeSeg})
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
	if s.Magic != dosStateMagic || s.Version != dosStateVersion {
		return fmt.Errorf("dos: 狀態檔不認得（%q v%d）", s.Magic, s.Version)
	}
	for _, h := range d.handles {
		h.f.Close()
	}
	d.Drive, d.Dir, d.Now = s.Drive, s.Dir, s.Now
	d.Mouse.X, d.Mouse.Y, d.Mouse.Buttons = s.Mouse.X, s.Mouse.Y, s.Mouse.Buttons
	d.Mouse.Press, d.Mouse.Release, d.Mouse.XScale = s.Mouse.Press, s.Mouse.Release, s.Mouse.XScale
	d.nextHandle, d.freeSeg = s.NextHandle, s.FreeSeg
	d.curPSP, d.lastExit = s.CurPSP, s.LastExit
	d.queue = append([]Queued(nil), s.Queue...)
	d.holes = nil
	for _, h := range s.Holes {
		d.holes = append(d.holes, memHole{seg: h.Seg, size: h.Size})
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
		d.handles[hs.H] = &handle{name: hs.Name, path: hs.Path, f: f, size: hs.Size}
	}
	d.blocks = map[uint16]uint16{}
	for k, v := range s.Blocks {
		d.blocks[k] = v
	}
	d.emb = map[uint16][]byte{}
	for k, v := range s.EMB {
		d.emb[k] = append([]byte(nil), v...)
	}
	d.nextEMB = s.NextEMB
	d.stack = nil
	for _, f := range s.Stack {
		d.stack = append(d.stack, procFrame{r: f.R, seg: f.Seg, ip: f.IP, fl: f.Fl, psp: f.PSP, freeSeg: f.FreeSeg})
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
