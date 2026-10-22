package strikecommander

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

// 圖形字的繪圖事件：存讀檔對話框（strike_commander 規格 004 §3.1、§3.2）、主選單與 MENU 片頭選單（規格 008 §3.1）。
// shape 繪製常式 +3A97:0000（檔案位移 40570h）在入口時，參數在 [SP+4] 起：
// 描述元近指標、x、y、圖檔遠指標（偏移、段）、條目號（RE 015 §3；dis-3A97.txt 的 [bp+6] 起）。
// MENU.EXE 的 +1BA2:0002 是同一份程式碼（16 bytes 相同，RE 018 §1.2、§5.3），參數堆疊相同。
const (
	segShape = 0x3A97

	// ipMenuShape 是 MENU.EXE shape 繪製常式入口的 IP（008 §3.1 第 1 條）。
	ipMenuShape = 0x0002

	saveLoadSize  = 42014
	saveLoadGroup = 21 // 每組條目數；組基底 0、21、42（RE 015 §2）

	// 008 §2 的子條目長度（首 u32）。執行期 p 指向子條目（RE 018 §1.1；TOMENU 是 LZW 解壓後）。
	mainMenuButtonsSize = 15606 // MAINMENU.PAK 條目 0：10 張 211×15 按鈕
	mainMenuPlateSize   = 8593  // MAINMENU.PAK 條目 1：233×104 底板
	tomenuButtonsSize   = 10314 // TOMENU.PAK 條目 1：4 張按鈕
)

// saveLoadSHA256 是 SAVELOAD.SHP 整檔的 SHA-256（RE 015 §1）。
var saveLoadSHA256 = mustHex("617f7270097913ea2c9841035fd201d4196d084e2408cc593eaf056d7f97ef6a")

// 008 §2 子條目的 SHA-256。
var (
	mainMenuButtonsSHA256 = mustHex("3729dfc27900368bbe27e7fb1e7deaa2e16e5666476fbbf715b719139208d0af")
	mainMenuPlateSHA256   = mustHex("abebe6c16aee0408fa27e93881f3a8ade463745c68ce3a1bdabcedef1210d380")
	tomenuButtonsSHA256   = mustHex("13dfa8ecdd986cf8b298aea6d64fccf62ed61a956f4fba476e689597b4f0e37a")
)

// sigShapeEntry 是 +3A97:0000 起 16 bytes 的入口簽章（RE 015 §4 簽章表）；MENU 的 +1BA2:0002 起相同（RE 018 §5.3）。
var sigShapeEntry = []int{0xC8, 0x52, 0x01, 0x00, 0x56, 0x57, 0x1E, 0xFC, 0x16, 0x07, 0x1E, 0x0F, 0xA9, 0x8B, 0x76, 0x06}

func mustHex(s string) [32]byte {
	var out [32]byte
	if _, err := hex.Decode(out[:], []byte(s)); err != nil {
		panic(err)
	}
	return out
}

// ArtFile 是繪圖事件的檔別（008 §3.1）。零值是 SAVELOAD.SHP。
type ArtFile int

const (
	ArtSaveLoad        ArtFile = iota // SAVELOAD.SHP（004、007）
	ArtMainMenuButtons                // MAINMENU.PAK 條目 0：主選單按鈕
	ArtMainMenuPlate                  // MAINMENU.PAK 條目 1：主選單底板（含五行灰字）
	ArtTomenuButtons                  // TOMENU.PAK 條目 1：MENU 片頭選單按鈕
)

func (f ArtFile) String() string {
	switch f {
	case ArtSaveLoad:
		return "SAVELOAD"
	case ArtMainMenuButtons:
		return "MAINMENU 按鈕"
	case ArtMainMenuPlate:
		return "MAINMENU 底板"
	case ArtTomenuButtons:
		return "TOMENU 按鈕"
	}
	return "未知"
}

// ArtEvent 是一次被接受的繪圖事件（004 §3.1、008 §3.1）。陣列都是複本。
//   - Entry 是條目 e 的解碼陣列，尺寸 W×H。
//   - Base0：SAVELOAD 是同一 k 的 0 組條目（條目 k）；主選單與片頭選單是 Entry 本身（沒有組別）。
//   - Buttons：MAINMENU 按鈕事件是整個按鈕檔（規則 P 要 k 與 k+5，008 §3.3）；底板列是建列時的按鈕檔（§3.4）。
//   - Line：底板列的行號 n（0 到 4，§3.4）。底板事件本身與其他檔為 0。
type ArtEvent struct {
	Steps   uint64
	File    ArtFile
	G, K    int
	X, Y    int
	Order   int
	W, H    int
	Entry   []uint8
	Base0   []uint8
	Buttons []byte
	Line    int
}

// shapeEntry 處理 IP == 0 時的 shape 繪製常式入口（004 §3.1）。依序：
//  1. CS:0000 起 16 bytes 等於入口簽章；base 已知時另要求 CS == base + 3A97h，
//     未知時以 CS − 3A97h 為候選，以 001 的 puts 簽章與其餘簽章驗證後採用；
//  2. 之後同 shapeDraw。
func (o *LabelObserver) shapeEntry(v View) {
	cs := v.CS()
	if o.baseKnown && o.baseOK {
		if cs != o.base+segShape {
			return
		}
		if !match(v, cs, 0, sigShapeEntry, 0) {
			return
		}
	} else {
		if !match(v, cs, 0, sigShapeEntry, 0) {
			return
		}
		cand := cs - segShape
		if why := o.checkBase(v, cand); why != "" {
			if !o.artBadCS[cs] {
				if o.artBadCS == nil {
					o.artBadCS = map[uint16]bool{}
				}
				o.artBadCS[cs] = true
				o.note(v.Steps(), "art", "shape 入口 %04X:0000 的候選 base %04X 驗證失敗：%s", cs, cand, why)
			}
			return
		}
		if o.baseKnown && cand != o.base {
			// 定位到不同的 base（005 §3.1 第 5 條）。
			o.dropVisible(v.Steps(), "shape 入口定位到不同的 base %04X（原 %04X），可見集合清空", cand, o.base)
		}
		o.base, o.baseKnown, o.baseOK = cand, true, true
		o.note(v.Steps(), "art", "由 shape 入口 %04X:0000 定位 base %04X", cs, cand)
	}
	o.shapeDraw(v)
}

// menuShapeEntry 處理 IP == 0002 時 MENU.EXE 的 shape 繪製常式入口（008 §3.1 第 1、2 條）：
// CS:0002 起 16 bytes 等於入口簽章才接 shapeDraw。不經 CS − 3A97h 與 base 檢查，也不學 base。
func (o *LabelObserver) menuShapeEntry(v View) {
	if !match(v, v.CS(), ipMenuShape, sigShapeEntry, 0) {
		return
	}
	o.shapeDraw(v)
}

// shapeDraw 是兩個入口共用的後半（004 §3.1、008 §3.1 第 3 到 5 條）：
//  1. 描述元 DS:d 的段是 FFFFh、矩形全螢幕；
//  2. 圖檔 p 起 u32 等於四個長度之一（先比長度，不符不讀整段），整段 SHA-256 相符（每次都重算），決定檔別；
//  3. 依檔別產生事件（saveLoadEvent、menuEvent）。
func (o *LabelObserver) shapeDraw(v View) {
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	d := rd16(v, ss, sp+4)
	x := int(int16(rd16(v, ss, sp+6)))
	y := int(int16(rd16(v, ss, sp+8)))
	pOff, pSeg := rd16(v, ss, sp+0x0A), rd16(v, ss, sp+0x0C)
	e := int(rd16(v, ss, sp+0x0E))
	if o.OnCursor != nil && o.baseKnown && o.baseOK && rd16(v, ss, sp) == ipCursorRet && rd16(v, ss, sp+2) == o.base+segCursor && rd16(v, ds, d) == 0xFFFF {
		o.OnCursor(v.Steps(), x, y) // 規格 014 §4.2.2 的游標分類（只供曝光收據）
	}
	if rd16(v, ds, d) != 0xFFFF || readRect(v, ds, d) != fullScreen {
		return // 裁切重畫（FCE5 等）、畫外緩衝與游標的 DB2E 都不收
	}
	base := lin(pSeg, pOff)
	var kind ArtFile
	var want [32]byte
	n := readLin32(v, base)
	switch n {
	case saveLoadSize:
		kind, want = ArtSaveLoad, saveLoadSHA256
	case mainMenuButtonsSize:
		kind, want = ArtMainMenuButtons, mainMenuButtonsSHA256
	case mainMenuPlateSize:
		kind, want = ArtMainMenuPlate, mainMenuPlateSHA256
	case tomenuButtonsSize:
		kind, want = ArtTomenuButtons, tomenuButtonsSHA256
	default:
		return
	}
	file := make([]byte, n)
	for i := range file {
		file[i] = v.Read8(base + uint32(i))
	}
	if sha256.Sum256(file) != want {
		return
	}
	if o.OnShapeArt != nil {
		o.OnShapeArt(v.Steps(), kind, file, e, x, y) // 規格 014 §4.2.2（只供曝光收據）
	}
	if kind == ArtSaveLoad {
		o.saveLoadEvent(v.Steps(), file, e, x, y)
		return
	}
	o.menuEvent(v.Steps(), kind, file, e, x, y)
}

// saveLoadEvent 是 SAVELOAD.SHP 的事件（004 §3.1 第 6 條、§3.2）：
//   - g = e / 21、k = e mod 21：g 為 0、1 且 k 屬於 0 到 9 或 15 到 20（確認框，007 §3.1）才產生事件；
//     k 屬於 10 到 14（箭頭、滑塊）不產生也不記錄；g == 2 記 skip；
//   - 解碼條目 e 與條目 k，尺寸要等於 §2.2 的元素尺寸。
func (o *LabelObserver) saveLoadEvent(steps uint64, file []byte, e, x, y int) {
	g, k := e/saveLoadGroup, e%saveLoadGroup
	switch {
	case g == 2:
		o.note(steps, "art", "SAVELOAD.SHP 條目 %d（第 2 組）未觀察，略過", e)
		return
	case g >= 3:
		return
	case k >= 10 && k <= 14:
		return
	}
	el, _, _ := elementFor(k)
	ent, err := decodeEntry(file, e)
	if err != nil {
		o.note(steps, "art", "SAVELOAD.SHP 條目 %d 解碼失敗：%v", e, err)
		return
	}
	b0, err := decodeEntry(file, k)
	if err != nil {
		o.note(steps, "art", "SAVELOAD.SHP 條目 %d 解碼失敗：%v", k, err)
		return
	}
	if ent.W != el.w || ent.H != el.h || b0.W != el.w || b0.H != el.h {
		o.note(steps, "art", "SAVELOAD.SHP 條目 %d 尺寸 %d×%d（0 組 %d×%d），要 %d×%d，略過", e, ent.W, ent.H, b0.W, b0.H, el.w, el.h)
		return
	}
	o.artOrder++
	if o.OnArt != nil {
		o.OnArt(ArtEvent{Steps: steps, G: g, K: k, X: x, Y: y, Order: o.artOrder, W: ent.W, H: ent.H, Entry: ent.Pix, Base0: b0.Pix})
	}
}

// menuEvent 是主選單與片頭選單的事件（008 §3.1 第 5 條）：MAINMENU 按鈕檔 k 0 到 9、底板檔只有條目 0、
// TOMENU 按鈕檔 k 0 到 3；條目超出不產生也不記錄；解碼後的尺寸要等於 §3.2，不符記一筆 art 紀錄。
// MAINMENU 按鈕事件另帶整個按鈕檔的複本（file 本身就是這次讀出的複本）。
func (o *LabelObserver) menuEvent(steps uint64, kind ArtFile, file []byte, e, x, y int) {
	w, h, ok := menuElementSize(kind, e)
	if !ok {
		return
	}
	ent, err := decodeEntry(file, e)
	if err != nil {
		o.note(steps, "art", "%s 條目 %d 解碼失敗：%v", kind, e, err)
		return
	}
	if ent.W != w || ent.H != h {
		o.note(steps, "art", "%s 條目 %d 尺寸 %d×%d，要 %d×%d，略過", kind, e, ent.W, ent.H, w, h)
		return
	}
	ev := ArtEvent{Steps: steps, File: kind, K: e, X: x, Y: y, W: w, H: h, Entry: ent.Pix, Base0: ent.Pix}
	if kind == ArtMainMenuButtons {
		ev.Buttons = file
	}
	o.artOrder++
	ev.Order = o.artOrder
	if o.OnArt != nil {
		o.OnArt(ev)
	}
}

// readLin32 讀線性位址 a 起的 u32。
func readLin32(v View, a uint32) uint32 {
	var b [4]byte
	for i := range b {
		b[i] = v.Read8(a + uint32(i))
	}
	return binary.LittleEndian.Uint32(b[:])
}
