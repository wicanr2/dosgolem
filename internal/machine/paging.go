package machine

import "fmt"

// 分頁與共用 TLB（docs/spec/432-linear-memory-and-paging §5、§10 第 1、5、7、9 項）。
//
// 有效分頁＝轉換開啟且 CR0.PG=1（m.paging，DOSBox-X `cpu/cpu.cpp:2629-2632`）。兩層 4 KB
// 頁表；頁目錄基底取 CR3 & 0xFFFFF000。PDE bit 7（PS）、PCD、PWT 在 386 忽略；不支援 4 MB 頁
// 與 WP。頁表走訪以 pageRead32／pageWrite32 存取實體位址並套 A20 遮罩（刻意與 DOSBox-X 不同，
// 見規格 §5：差異只在 A20 關且頁表位於 1 MB 以上時出現）。走訪位址超出 Ext 時讀到
// FFFFFFFFh，即 P=1 的垃圾項（同 DOSBox-X `mem.h:263`）。

const (
	pteP = 1 << 0 // present
	pteW = 1 << 1 // R/W
	pteU = 1 << 2 // U/S
	pteA = 1 << 5 // accessed
	pteD = 1 << 6 // dirty

	// TLB 條目的權限等級：PDE 與 PTE 的 U/S、R/W 化簡成四級
	// （監督者讀、監督者讀寫、使用者讀、使用者讀寫），同 DOSBox-X `cpu/paging.cpp:150-172` 的
	// translate_array：U 位元＝兩層 U/S 都為 1，W 位元＝兩層 R/W 都為 1。
	tlbU = 1 << 0
	tlbW = 1 << 1
)

// tlbEntry 是一頁的快取轉換。gen 與 Machine.tlbGen 相同才有效（0 代表從未填過）。
type tlbEntry struct {
	gen     uint32
	phys    uint32 // 實體頁基底（低 12 位為 0），尚未套 A20
	pteAddr uint32 // PTE 的實體位址（已套 A20）
	level   uint8
	dirty   bool // PTE 的 dirty 已寫回（或填入時就已是 1）
}

// tlbBlock 是 4 MB 範圍的 1024 筆條目（懶配置）。
type tlbBlock [1024]tlbEntry

// PageFaultRecord 是一次分頁錯誤的紀錄（§5.3）。本規格不遞送 #PF（S6），只記錄。
type PageFaultRecord struct {
	// Linear 是出錯的那個 byte 的線性位址。
	Linear uint32
	// PDE、PTE 是出錯時讀到的項目（PDE 的 P=0 時 PTE 填 0）。PDE 是 accessed 已寫回之後的值。
	PDE, PTE uint32
	// Write 是寫入存取；User 是使用者身分（V86 或 CPL=3）。
	Write, User bool
	// Steps 是產生時的 m.Steps。tick、keyTick 在 m.Steps++ 之前執行，它們產生的紀錄因此
	// 帶上一步的序號（已知的序號語意）。
	Steps uint64
}

func (r PageFaultRecord) String() string {
	op := "讀"
	if r.Write {
		op = "寫"
	}
	who := "監督者"
	if r.User {
		who = "使用者"
	}
	return fmt.Sprintf("分頁錯誤（%s%s）線性 %08X PDE=%08X PTE=%08X step=%d", who, op, r.Linear, r.PDE, r.PTE, r.Steps)
}

// PageFaultError 是 Machine.Step 在該步產生分頁錯誤紀錄時回傳的錯誤（§10 第 1 項）。
// 與 CPU 的錯誤（含控制流哨兵）並存時 Err 非 nil，Unwrap 回傳它，所以 errors.Is、
// errors.As 兩種都可用；哨兵不遞送。
type PageFaultError struct {
	Record PageFaultRecord
	Err    error
}

func (e *PageFaultError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%v（同一步的%s）", e.Err, e.Record)
	}
	return e.Record.String()
}

func (e *PageFaultError) Unwrap() error { return e.Err }

// PageFault 回目前的錯誤紀錄（Machine.Step 的第一個動作會把殘留的紀錄移到 LastPageFault）。
// Step 以外的存取（oracle 的 Byte／SetByte、probe 的 dump 等）產生的紀錄由這裡查詢，
// 不影響下一個 Step。
func (m *Machine) PageFault() (PageFaultRecord, bool) { return m.pf, m.pfOK }

// LastPageFault 回上一筆被移出 PageFault 的紀錄。
func (m *Machine) LastPageFault() (PageFaultRecord, bool) { return m.pfLast, m.pfLastOK }

// notePageFault 記一筆紀錄。PageFault 非空時不覆蓋（同一步多次錯誤只保留第一筆，
// Step 以外連續產生多筆也一樣）。
func (m *Machine) notePageFault(lin, pde, pte uint32, write, user bool) {
	if m.pfOK {
		return
	}
	m.pf = PageFaultRecord{Linear: lin, PDE: pde, PTE: pte, Write: write, User: user, Steps: m.Steps}
	m.pfOK = true
}

// clearPageFaults 清空兩個紀錄（Restore、LoadState）。
func (m *Machine) clearPageFaults() {
	m.pf, m.pfOK = PageFaultRecord{}, false
	m.pfLast, m.pfLastOK = PageFaultRecord{}, false
}

// stepFault 把該步的分頁紀錄併入 Step 的回傳值（§5.3、§10 第 1 項）：回 *PageFaultError，
// CPU 的錯誤（含哨兵）以 %w 包住。紀錄留在 PageFault()，下一個 Step 才移走。
func (m *Machine) stepFault(cpuErr error) error {
	return &PageFaultError{Record: m.pf, Err: cpuErr}
}

// tlbClear 整份清除 TLB（世代號溢位時）：丟掉所有區塊，世代回到 1。
// 微快取一併清掉：它的條目以世代號判定有效，世代回到 1 之後舊條目的 gen 可能碰巧又等於 1。
func (m *Machine) tlbClear() {
	m.tlb = nil
	m.tlbGen = 1
	m.utlbR = [utlbSize]utlbEntry{}
	m.utlbW = [utlbSize]utlbEntry{}
}

// 微快取（docs/spec/455-fetch-page-cache §3.1：只省 xlatPage 的查表，不改任何可觀察行為）。
//
// readLin／writeLin 在分頁開啟時每個 byte 都要經 xlatPage：兩層指標（tlb → 區塊 → 條目）、
// 世代比對與權限判斷。微快取是 utlbSize 筆直接對映的條目，讀、寫各一份，記下「這一頁在這個世代、
// 以這個身分（user）成功轉換過，實體頁是什麼」。
//
// 等價性：
//   - 只在 xlatPage 成功回來之後填入。成功表示共用 TLB 在目前世代有這一頁的條目，且以同一 user 身分
//     通過權限檢查；之後同世代、同身分再查同一頁，xlatPage 必定走 TLB 命中分支、權限照樣通過。
//   - 寫入份只在 xlatPage(write=true) 成功之後填入，那時條目的 dirty 已寫回（命中分支的 tlbMarkDirty
//     或 walk 填入時 pteD 已設），之後 xlatPage 的寫入命中不再有副作用。
//   - 讀取命中不會寫頁表（xlatPage 讀取命中沒有副作用），所以讀取份不必看 dirty。
//   - 失敗路徑（頁不存在、權限錯誤）不填，照舊每次經 xlatPage 記錄錯誤。
//
// 失效條件：條目的 gen 與 m.tlbGen 不同即無效。tlbGen 在 invalidateTLB 遞增，涵蓋 SetCR0（PE、PG 改變）、
// SetCR3、SetA20、SetProtectedModeProfile、applyPM（Restore、LoadState）；共用 TLB 條目只在世代改變後
// 才會被 walk 改寫，所以同世代的微快取條目與共用 TLB 一致。tlbGen 溢位時 tlbClear 清空微快取。
// CPL、VM 改變不必作廢：user 是條目的鍵。A20 套在轉換之後（readLin 的 a20 遮罩不進微快取），
// 而 SetA20 本來就作廢 TLB。
const utlbSize = 64

type utlbEntry struct {
	page uint32 // 線性頁號（a >> 12）
	phys uint32 // 實體頁基底（尚未套 A20）
	gen  uint32 // 填入時的 m.tlbGen；0 永不有效（tlbGen 從 1 起算）
	user bool
}

// xlatRead 是 readLin 的分頁轉換：微快取命中就直接回，否則走 xlatPage 並在成功時填入。
func (m *Machine) xlatRead(a uint32, user bool) (uint32, bool) {
	p := a >> 12
	e := &m.utlbR[p&(utlbSize-1)]
	if e.page == p && e.gen == m.tlbGen && e.user == user {
		return e.phys | a&0xFFF, true
	}
	phys, ok := m.xlatPage(a, false, user)
	if ok {
		*e = utlbEntry{page: p, phys: phys &^ 0xFFF, gen: m.tlbGen, user: user}
	}
	return phys, ok
}

// xlatWrite 是 writeLin 的分頁轉換，同 xlatRead。
func (m *Machine) xlatWrite(a uint32, user bool) (uint32, bool) {
	p := a >> 12
	e := &m.utlbW[p&(utlbSize-1)]
	if e.page == p && e.gen == m.tlbGen && e.user == user {
		return e.phys | a&0xFFF, true
	}
	phys, ok := m.xlatPage(a, true, user)
	if ok {
		*e = utlbEntry{page: p, phys: phys &^ 0xFFF, gen: m.tlbGen, user: user}
	}
	return phys, ok
}

// a20Mask 對實體位址套 A20：關閉時清掉 bit 20。
func (m *Machine) a20Mask(p uint32) uint32 {
	if !m.a20 {
		p &^= 0x100000
	}
	return p
}

// xlatPage 把線性位址轉成（尚未套 A20 的）實體位址。user 為真時檢查 U/S、R/W
// （§5.3：PDE 與 PTE 任一的 U/S 為 0 即錯誤；寫入時任一的 R/W 為 0 即錯誤）。
// 失敗（頁不存在或權限錯誤）時記下錯誤紀錄並回 false。
//
// 按需填入：某頁第一次經轉換時才走頁表（惰性 TLB，同 DOSBox-X `cpu/paging.cpp:779-905`）。
// 條目不逐出，所以「舊 PTE 仍被使用」的行為不取決於碰撞。權限在查表時依 user 比對，
// CPL 改變不必作廢。
func (m *Machine) xlatPage(a uint32, write, user bool) (uint32, bool) {
	if t := m.tlb; t != nil {
		if blk := t[a>>22]; blk != nil {
			if e := &blk[(a>>12)&0x3FF]; e.gen == m.tlbGen {
				if user && (e.level&tlbU == 0 || write && e.level&tlbW == 0) {
					// TLB 命中而權限錯誤：紀錄的 PTE 由條目記住的 PTE 位址重讀，PDE 由 CR3 與線性位址
					// 重算位址後讀取（皆經 pageRead32，無副作用）。
					pde := m.pageRead32(m.a20Mask(m.cr3&0xFFFFF000 + (a>>22)<<2))
					m.notePageFault(a, pde, m.pageRead32(e.pteAddr), write, user)
					return 0, false
				}
				if write && !e.dirty {
					m.tlbMarkDirty(e)
				}
				return e.phys | a&0xFFF, true
			}
		}
	}
	return m.walk(a, write, user)
}

// tlbMarkDirty：讀入的條目之後被寫入時，以條目記住的 PTE 位址讀改寫 dirty 位元，
// 不重走權限；PTE 的實體頁若已改變，仍用條目的舊實體頁（保持舊轉換語意，不像 DOSBox-X
// `paging.cpp:360-361` 那樣中止）。dirty 已寫回後不再改頁表。
func (m *Machine) tlbMarkDirty(e *tlbEntry) {
	if pte := m.pageRead32(e.pteAddr); pte&pteD == 0 {
		m.pageWrite32(e.pteAddr, pte|pteD)
	}
	e.dirty = true
}

// walk 走頁表，順序跟隨 DOSBox-X（`cpu/paging.cpp:837-844`）：先讀 PDE、PDE 的 P=0 即錯誤；
// 再讀 PTE；然後寫 PDE 的 accessed（早於 PTE 的存在與權限檢查）；最後做 PTE 的存在與權限檢查；
// 通過後才寫 PTE 的 accessed（寫入時連同 dirty）。權限錯誤時不改 PTE。頁目錄自我映射時結果
// 因此與 DOSBox-X 相同（PTE 是 PDE 寫回 accessed 之前讀的）。
// 不採 DOSBox-X 在權限錯誤時重讀頁表、屬性變了就清 TLB 重試的 hack_check（本規格採真機語意）。
// P=0 與權限錯誤的走訪不填 TLB（沒有負向快取）。
func (m *Machine) walk(a uint32, write, user bool) (uint32, bool) {
	pdeAddr := m.a20Mask(m.cr3&0xFFFFF000 + (a>>22)<<2)
	pde := m.pageRead32(pdeAddr)
	if pde&pteP == 0 {
		m.notePageFault(a, pde, 0, write, user)
		return 0, false
	}
	pteAddr := m.a20Mask(pde&0xFFFFF000 + (a>>12&0x3FF)<<2)
	pte := m.pageRead32(pteAddr)
	if pde&pteA == 0 {
		pde |= pteA
		m.pageWrite32(pdeAddr, pde)
	}
	if pte&pteP == 0 {
		m.notePageFault(a, pde, pte, write, user)
		return 0, false
	}
	var level uint8
	if pde&pte&pteU != 0 {
		level |= tlbU
	}
	if pde&pte&pteW != 0 {
		level |= tlbW
	}
	if user && (level&tlbU == 0 || write && level&tlbW == 0) {
		m.notePageFault(a, pde, pte, write, user)
		return 0, false
	}
	old := pte
	pte |= pteA
	if write {
		pte |= pteD
	}
	if pte != old {
		m.pageWrite32(pteAddr, pte)
	}
	if m.tlb == nil {
		m.tlb = new([1024]*tlbBlock)
	}
	blk := m.tlb[a>>22]
	if blk == nil {
		blk = new(tlbBlock)
		m.tlb[a>>22] = blk
	}
	blk[(a>>12)&0x3FF] = tlbEntry{
		gen: m.tlbGen, phys: pte & 0xFFFFF000, pteAddr: pteAddr, level: level, dirty: pte&pteD != 0,
	}
	return pte&0xFFFFF000 | a&0xFFF, true
}

// peekXlatPage 是觀測用的轉換（§6）：TLB 條目世代符合就用（看到 CPU 此刻會看到的，
// 含舊轉換）；不符就無副作用地走頁表，不填 TLB、不寫 accessed／dirty、不產生錯誤紀錄。
// 不檢查 U/S、R/W（監督者身分），只有頁不存在時回 false。
func (m *Machine) peekXlatPage(a uint32) (uint32, bool) {
	if t := m.tlb; t != nil {
		if blk := t[a>>22]; blk != nil {
			if e := &blk[(a>>12)&0x3FF]; e.gen == m.tlbGen {
				return e.phys | a&0xFFF, true
			}
		}
	}
	pde := m.pageRead32(m.a20Mask(m.cr3&0xFFFFF000 + (a>>22)<<2))
	if pde&pteP == 0 {
		return 0, false
	}
	pte := m.pageRead32(m.a20Mask(pde&0xFFFFF000 + (a>>12&0x3FF)<<2))
	if pte&pteP == 0 {
		return 0, false
	}
	return pte&0xFFFFF000 | a&0xFFF, true
}
