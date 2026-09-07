package dos

import (
	"sort"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// EMS（LIM/EMS 4.0 的 `int 67h`）。規格：`docs/spec/008`（頁配置與映射）
// 與 `docs/spec/014`（偵測路徑與 4.0 功能）。
//
// 1994 年前後的 DOS 遊戲把放不進 640 KB 的資料放這裡：源平合戰的
// `Opendat.gp` 有 853 KB、`Enddat.gp` 有 1,013 KB。**偵測不到 EMS 的話
// 程式不會報錯，它只是不去讀那些檔**——開場停在 logo，看起來像卡住。
//
// 機器模型：page frame 在 machine.EMSFrameSeg（4 個實體頁 × 16 KB）。
// 頁內容存在這裡的 Go 記憶體（不佔客體的 1 MB），映射時複製進頁框、
// 換映射／釋放前寫回——**複製式分頁**，語意等價的前提是「先映射再存取」
// （`docs/spec/008` §3）。
//
// 進得來的路有兩條，兩條都要留：
//
//  1. **`int 67h` 向量指向 EMSSeg 的驅動 header**，位移 0Ah 是簽章
//     `EMMXXXX0`，位移 0 是一段 `int F6h` trampoline。程式偵測 EMS 的
//     標準做法就是讀向量段、比對那 8 個位元組（`docs/spec/014` §3.1）。
//  2. **直接 `int 67h`**：向量還指著 StubSeg 時（測試與沒有驅動 header
//     的機器設定）由 handle 直接分派。
//
// 兩條最後都走同一個 emsCall。

const (
	// emsPageSize 是一頁 16 KB。
	emsPageSize = 16 * 1024
	// emsPhysPages 是 page frame 裡的實體頁數。
	emsPhysPages = 4
	// emsTotalPages 是宣告的總頁數（512 × 16 KB ＝ 8 MB）。
	//
	// ⚠ 這個數字**兩條分支給過不同的值**：logh3 給 8（恰好滿足 GIN3
	// launcher 的 `cmp bx,8` 探測下限），源平合戰那邊給 512（它真的要放
	// 兩份接近 1 MB 的資料檔）。512 同時滿足兩者——探測下限是下限，
	// 不是上限——所以合併之後取 512。原本「8 頁」在 `docs/spec/007` §4
	// 標的就是**假說**（「探測下限而非記憶體需求」），不是量到的事實。
	emsTotalPages = 512
)

// emsHandle 是一個 EMS handle 擁有的邏輯頁。
type emsHandle struct {
	pages [][]byte
}

// emsMapping 是一個實體頁目前映著誰。
type emsMapping struct {
	h    uint16
	page uint16
}

// ems 是 EMS 的狀態。
type ems struct {
	handles map[uint16]*emsHandle
	next    uint16

	// frame[p] 是實體頁 p 目前映著誰，nil ＝ 空。
	// saved 是 AH=47h 存下來的那一份。
	frame, saved [emsPhysPages]*emsMapping
}

func newEMS() *ems {
	return &ems{handles: map[uint16]*emsHandle{}, next: 1}
}

// used 回傳目前配出去的總頁數。
func (e *ems) used() int {
	n := 0
	for _, h := range e.handles {
		n += len(h.pages)
	}
	return n
}

// emsState 取狀態，沒有就先建一份。
func (d *DOS) emsState() *ems {
	if d.ems == nil {
		d.ems = newEMS()
	}
	return d.ems
}

// emsFrameAddr 是實體頁 p 在 1 MB 空間裡的位址。
func emsFrameAddr(p int) uint32 {
	return uint32(machine.EMSFrameSeg)*16 + uint32(p)*emsPageSize
}

// emsFlush 把實體頁 p 的內容寫回它映著的邏輯頁。
//
// ⚠ **不寫回的話「換頁再換回來」會拿到舊資料**，而且完全不報錯——
// 遊戲的資料看起來就是隨機少了一塊。
func (d *DOS) emsFlush(p int) {
	e := d.emsState()
	m := e.frame[p]
	if m == nil {
		return
	}
	h, ok := e.handles[m.h]
	if !ok || int(m.page) >= len(h.pages) {
		return
	}
	base := emsFrameAddr(p)
	for i := 0; i < emsPageSize; i++ {
		h.pages[m.page][i] = d.M.Read8(base + uint32(i))
	}
}

// emsLoad 把邏輯頁的內容搬進實體頁 p。
func (d *DOS) emsLoad(p int, handle, page uint16) {
	e := d.emsState()
	h := e.handles[handle]
	// 走 WriteBytes 而不是直接動 M.Mem：寫入監看要看得到這一筆
	// （`oracle.WatchLinear` 監的就是「誰把這個位址寫成這樣」）。
	d.M.WriteBytes(emsFrameAddr(p), h.pages[page])
	e.frame[p] = &emsMapping{h: handle, page: page}
}

// emsCall 是 `int 67h` 的分派（直接進來或經 `int F6h` trampoline）。
//
// **回傳慣例是 AH ＝ 0 成功、非 0 是錯誤碼**，不是 DOS 的 CF。
func (d *DOS) emsCall(c *cpu.CPU) {
	e := d.emsState()
	switch ah(c) {
	case 0x40: // Get Status
		setAH(c, 0)

	case 0x41: // Get Page Frame Segment
		c.R[cpu.BX] = machine.EMSFrameSeg
		setAH(c, 0)

	case 0x42: // Get Page Counts：BX ＝ 未配置、DX ＝ 總數
		c.R[cpu.BX] = uint16(emsTotalPages - e.used())
		c.R[cpu.DX] = emsTotalPages
		setAH(c, 0)

	case 0x43: // Allocate Pages：BX ＝ 頁數 → DX ＝ handle
		d.emsAlloc(c)

	case 0x44: // Map Handle Page：AL ＝ 實體頁、BX ＝ 邏輯頁、DX ＝ handle
		d.emsMap(c)

	case 0x45: // Release Handle
		d.emsRelease(c)

	case 0x46: // Get Version
		setAL(c, 0x40) // 4.0
		setAH(c, 0)

	case 0x47: // Save Page Map
		e.saved = e.frame
		setAH(c, 0)

	case 0x48: // Restore Page Map
		for p := 0; p < emsPhysPages; p++ {
			d.emsFlush(p)
		}
		for p := 0; p < emsPhysPages; p++ {
			m := e.saved[p]
			if m == nil {
				e.frame[p] = nil
				continue
			}
			if h, ok := e.handles[m.h]; ok && int(m.page) < len(h.pages) {
				d.emsLoad(p, m.h, m.page)
			}
		}
		setAH(c, 0)

	case 0x4E: // Get／Set Page Map（EMS 4.0）
		// AL=00 存到 ES:DI、AL=01 從 DS:SI 取回、AL=02 兩者一起做、
		// AL=03 回這份對映表要幾個位元組。
		//
		// **這是比 `47h`／`48h` 新的那一套**：中斷處理常式借用 page frame
		// 時用它把整份對映表搬進自己的緩衝區。只做 `47h`／`48h` 的話，
		// 用 4.0 寫法的程式會拿到 84h（沒有這個功能）然後**改走不借用
		// page frame 的慢路徑**——行為與原版不同，而且不會報錯。
		d.emsPageMap(c)

	case 0x4B: // Get Handle Count
		c.R[cpu.BX] = uint16(len(e.handles))
		setAH(c, 0)

	case 0x4C: // Get Pages for Handle
		h, ok := e.handles[c.R[cpu.DX]]
		if !ok {
			setAH(c, 0x83)
			return
		}
		c.R[cpu.BX] = uint16(len(h.pages))
		setAH(c, 0)

	case 0x50: // Map／Unmap Multiple Pages（EMS 4.0）
		// AL=00：DS:SI 是「邏輯頁, 實體頁」的陣列；AL=01：陣列裡放的是
		// 段位址而不是實體頁號。CX ＝ 幾組。
		//
		// **一次換好幾頁是原子的**：中間失敗要保持原狀，不然程式以為
		// 換好了，實際上只換了前半——它讀到的是兩份資料拼起來的東西。
		d.emsMapMultiple(c)

	case 0x51: // Reallocate Pages：BX ＝ 新的頁數、DX ＝ handle
		d.emsRealloc(c)

	case 0x58: // Get Mappable Physical Address Array
		// AL=00：把陣列寫到 ES:DI（每項 ＝ 段 ＋ 實體頁號）；AL=01：只回項數。
		// **這是 EMS 4.0 程式問「page frame 長什麼樣」的方式**，
		// 回 84h（沒這功能）它就當成不能用。
		if al(c) == 0x00 {
			dst := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.DI])
			for p := 0; p < emsPhysPages; p++ {
				d.M.Write16(dst+uint32(p)*4, machine.EMSFrameSeg+uint16(p)*(emsPageSize/16))
				d.M.Write16(dst+uint32(p)*4+2, uint16(p))
			}
		}
		c.R[cpu.CX] = emsPhysPages
		setAH(c, 0)

	default:
		d.note(0x67, ah(c), al(c))
		setAH(c, 0x84) // 沒有這個功能
	}
}

// int67 是直接 `int 67h` 的入口（向量還指著 StubSeg 時）。
func (d *DOS) int67(c *cpu.CPU) { d.emsCall(c) }

// emsAlloc 是 AH=43h。
//
// 三個錯誤碼分得開才有診斷價值（LIM 4.0 §狀態碼）：
//
//   - 87h：要的頁數**比整台機器有的還多**（e.g. 要 65535 頁）
//   - 88h：機器有那麼多，但**現在沒空**（e.g. 剩 5 頁卻要 6 頁）
//   - 87h：要 0 頁
//
// ⚠ 最後一條**與 LIM 手冊不符**（手冊寫 89h ＝ zero pages requested）。
// 這裡照兩條來源分支的既有行為留 87h：沒有任何一支被觀測的程式走過
// 這條路，改成 89h 是憑手冊改行為而不是憑證據。**這是假說待驗**，
// 真的遇到有程式要 0 頁再回來量。
func (d *DOS) emsAlloc(c *cpu.CPU) {
	e := d.emsState()
	want := int(c.R[cpu.BX])
	avail := emsTotalPages - e.used()
	switch {
	case want == 0:
		setAH(c, 0x87)
	case want > emsTotalPages:
		setAH(c, 0x87)
	case want > avail:
		setAH(c, 0x88)
	default:
		h := &emsHandle{pages: make([][]byte, want)}
		for i := range h.pages {
			h.pages[i] = make([]byte, emsPageSize)
		}
		id := e.next
		e.next++
		e.handles[id] = h
		c.R[cpu.DX] = id
		setAH(c, 0)
		d.EMSOps = append(d.EMSOps, EMSOp{Step: d.M.Steps, Fn: 0x43, Handle: id, Pages: want})
	}
}

// emsMap 是 AH=44h。BX=0FFFFh 是解除映射（EMS 4.0）。
func (d *DOS) emsMap(c *cpu.CPU) {
	e := d.emsState()
	p := int(al(c))
	if p >= emsPhysPages {
		setAH(c, 0x8B) // 實體頁超範圍
		return
	}
	h, ok := e.handles[c.R[cpu.DX]]
	if !ok {
		setAH(c, 0x83) // handle 無效
		d.EMSOps = append(d.EMSOps, EMSOp{Step: d.M.Steps, Fn: 0x44,
			Handle: c.R[cpu.DX], Logical: c.R[cpu.BX], Phys: uint8(p), Status: 0x83})
		return
	}
	d.emsFlush(p)
	if c.R[cpu.BX] == 0xFFFF { // 解除映射
		e.frame[p] = nil
		setAH(c, 0)
		return
	}
	if int(c.R[cpu.BX]) >= len(h.pages) {
		setAH(c, 0x8A) // 邏輯頁超範圍
		d.EMSOps = append(d.EMSOps, EMSOp{Step: d.M.Steps, Fn: 0x44,
			Handle: c.R[cpu.DX], Logical: c.R[cpu.BX], Phys: uint8(p), Status: 0x8A})
		return
	}
	d.emsLoad(p, c.R[cpu.DX], c.R[cpu.BX])
	setAH(c, 0)
	d.EMSOps = append(d.EMSOps, EMSOp{Step: d.M.Steps, Fn: 0x44,
		Handle: c.R[cpu.DX], Logical: c.R[cpu.BX], Phys: uint8(p)})
}

// emsRelease 是 AH=45h。釋放前把還映著的頁寫回。
func (d *DOS) emsRelease(c *cpu.CPU) {
	e := d.emsState()
	id := c.R[cpu.DX]
	if _, ok := e.handles[id]; !ok || id == 0 {
		setAH(c, 0x83)
		return
	}
	for p := 0; p < emsPhysPages; p++ {
		if m := e.frame[p]; m != nil && m.h == id {
			d.emsFlush(p)
			e.frame[p] = nil
		}
	}
	delete(e.handles, id)
	setAH(c, 0)
	d.EMSOps = append(d.EMSOps, EMSOp{Step: d.M.Steps, Fn: 0x45, Handle: id})
}

// EMSPage 是一頁 EMS 的內容與身分。**搜尋記憶體時不能漏掉它**：
// EMS 是資料倉庫，遊戲把字型、圖庫這類大東西放在裡面，只有當下映射進
// 頁框的那幾頁會出現在 1 MB 位址空間裡。只掃主記憶體的話，
// 「找不到」與「不存在」就分不開了。
type EMSPage struct {
	Handle uint16
	Page   int
	Data   []byte
}

// EMSPages 回目前所有 EMS 邏輯頁（含沒有映射進頁框的）。
//
// ⚠ 已經映射進頁框的那幾頁，**客體對頁框的寫入還沒寫回來**——
// 要看最新內容得先 EMSFlushAll。
func (d *DOS) EMSPages() []EMSPage {
	if d.ems == nil {
		return nil
	}
	var out []EMSPage
	for h, hd := range d.ems.handles {
		for i, p := range hd.pages {
			out = append(out, EMSPage{Handle: h, Page: i, Data: p})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Handle != out[j].Handle {
			return out[i].Handle < out[j].Handle
		}
		return out[i].Page < out[j].Page
	})
	return out
}

// EMSFlushAll 把所有映射中的實體頁寫回邏輯頁。
//
// 在 EMSPages 之前叫，拿到的才是「現在」而不是「上次換頁時」。
func (d *DOS) EMSFlushAll() {
	if d.ems == nil {
		return
	}
	for p := 0; p < emsPhysPages; p++ {
		d.emsFlush(p)
	}
}

// emsPageMap 是 `AH=4Eh`：整份對映表的存／取。
//
// 表的內容是我們自己的格式（每個實體頁 4 bytes：handle ＋ 邏輯頁），
// 程式只會把它原封不動存起來再交回來——**規格明講它是 opaque 的**。
// 唯一的要求是「取回來之後畫面上的資料要跟存的時候一樣」。
func (d *DOS) emsPageMap(c *cpu.CPU) {
	e := d.emsState()
	const entry = 4
	switch al(c) {
	case 0x00, 0x02: // 存（02 是先存再取）
		dst := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.DI])
		for p := 0; p < emsPhysPages; p++ {
			h, page := uint16(0xFFFF), uint16(0xFFFF)
			if m := e.frame[p]; m != nil {
				h, page = m.h, m.page
			}
			d.M.Write16(dst+uint32(p*entry), h)
			d.M.Write16(dst+uint32(p*entry)+2, page)
		}
		if al(c) == 0x00 {
			setAH(c, 0)
			return
		}
		fallthrough
	case 0x01: // 取回
		src := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.SI])
		// **先把現在映著的寫回去**，再照表重映——順序反了的話，
		// 被換掉的那一頁的修改會留在 page frame 裡然後被寫進新的那一頁。
		for p := 0; p < emsPhysPages; p++ {
			d.emsFlush(p)
		}
		for p := 0; p < emsPhysPages; p++ {
			h := d.M.Read16(src + uint32(p*entry))
			page := d.M.Read16(src + uint32(p*entry) + 2)
			if h == 0xFFFF {
				e.frame[p] = nil
				continue
			}
			hh, ok := e.handles[h]
			if !ok || int(page) >= len(hh.pages) {
				setAH(c, 0xA3) // 對映表的內容壞了
				return
			}
			d.emsLoad(p, h, page)
		}
		setAH(c, 0)
	case 0x03: // 問這份表要幾個位元組
		setAL(c, emsPhysPages*entry)
		setAH(c, 0)
	default:
		d.note(0x67, 0x4E, al(c))
		setAH(c, 0x8F) // 無效的子功能
	}
}

// emsMapMultiple 是 `AH=50h`：一次換好幾頁。
func (d *DOS) emsMapMultiple(c *cpu.CPU) {
	e := d.emsState()
	h, ok := e.handles[c.R[cpu.DX]]
	if !ok {
		setAH(c, 0x83)
		return
	}
	n := int(c.R[cpu.CX])
	src := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.SI])
	type pair struct{ logical, phys uint16 }
	list := make([]pair, 0, n)

	// **先全部檢查再全部套用。** 中間失敗就回原狀：程式以為換好了、
	// 實際只換了前半的話，它讀到的是兩份資料拼起來的東西，
	// 而那看起來像資料檔壞了。
	for i := 0; i < n; i++ {
		logical := d.M.Read16(src + uint32(i*4))
		phys := d.M.Read16(src + uint32(i*4) + 2)
		if al(c) == 0x01 { // 陣列裡放的是段位址
			seg := phys
			if seg < machine.EMSFrameSeg {
				setAH(c, 0x8B)
				return
			}
			phys = (seg - machine.EMSFrameSeg) / (emsPageSize / 16)
		}
		if int(phys) >= emsPhysPages {
			setAH(c, 0x8B) // 實體頁超範圍
			return
		}
		if logical != 0xFFFF && int(logical) >= len(h.pages) {
			setAH(c, 0x8A) // 邏輯頁超範圍
			return
		}
		list = append(list, pair{logical, phys})
	}
	for _, p := range list {
		d.emsFlush(int(p.phys))
		if p.logical == 0xFFFF {
			e.frame[p.phys] = nil
			continue
		}
		d.emsLoad(int(p.phys), c.R[cpu.DX], p.logical)
	}
	setAH(c, 0)
	d.EMSOps = append(d.EMSOps, EMSOp{Step: d.M.Steps, Fn: 0x50,
		Handle: c.R[cpu.DX], Pages: n})
}

// emsRealloc 是 `AH=51h`：改變一個 handle 的頁數。
//
// **縮小要保留前面那些頁的內容**：程式縮小之後仍然會讀它們。
func (d *DOS) emsRealloc(c *cpu.CPU) {
	e := d.emsState()
	h, ok := e.handles[c.R[cpu.DX]]
	if !ok {
		setAH(c, 0x83)
		return
	}
	want := int(c.R[cpu.BX])
	avail := emsTotalPages - e.used() + len(h.pages)
	if want > avail {
		c.R[cpu.BX] = uint16(len(h.pages))
		setAH(c, 0x88) // 現在沒有那麼多
		return
	}
	// 縮小之前要把還映著、而且落在被砍掉範圍裡的實體頁先寫回並解除映射。
	for p := 0; p < emsPhysPages; p++ {
		if m := e.frame[p]; m != nil && m.h == c.R[cpu.DX] && int(m.page) >= want {
			d.emsFlush(p)
			e.frame[p] = nil
		}
	}
	for len(h.pages) < want {
		h.pages = append(h.pages, make([]byte, emsPageSize))
	}
	if want < len(h.pages) {
		h.pages = h.pages[:want]
	}
	setAH(c, 0)
	d.EMSOps = append(d.EMSOps, EMSOp{Step: d.M.Steps, Fn: 0x51,
		Handle: c.R[cpu.DX], Pages: want})
}
