package oracle

// 編譯後 BASIC 的陣列。
//
// rich2 的 RE 筆記記的是**描述子位址**（`ds:11A2h` 那種），資料落在哪要
// 從描述子解。這一層把那個解讀收在一個地方。

// Phys 把 20 位元線性位址包成 Addr。
func Phys(linear uint32) Addr {
	return Addr{Seg: uint16(linear >> 4), Off: uint16(linear & 0xF)}
}

// Dim 是一個維度：下界與元素數。
//
// `1..6` 寫成 `Dim{Lo: 1, N: 6}`，`0..59` 寫成 `Dim{Lo: 0, N: 60}`。
// 維度取自 `rich2/docs/re/014` §2 的 DIM 對照表。
type Dim struct{ Lo, N int }

// Array 是一個 BASIC 陣列的檢視。
type Array struct {
	o     *Oracle
	Base  uint32 // 資料的線性位址
	Dims  []Dim
	Width int // 元素寬度（bytes）
}

// Array 從描述子位址開一個陣列檢視。
//
// **描述子的前兩個 word 是 `(位移, 段)`**，資料基底就是它們組出來的線性位址。
// 這是拿開局現金 25000 搜出來的：4 個命中全部落在描述子指向的 1,440 bytes 內
// （`11A2h` 是 `1..6 × 0..59` × 4B）。
//
// 維度與寬度要呼叫端給——描述子裡也有，但那部分的欄位語意還沒定，
// 而 `rich2/docs/re/014` §2 已經有完整的對照表，用已知的比猜的可靠。
func (o *Oracle) Array(descriptor uint16, dims []Dim, width int) *Array {
	off := o.Word(o.DS(descriptor))
	seg := o.Word(o.DS(descriptor + 2))
	return &Array{o: o, Base: uint32(seg)*16 + uint32(off), Dims: dims, Width: width}
}

// Size 是整個陣列的位元組數。
func (a *Array) Size() int {
	n := a.Width
	for _, d := range a.Dims {
		n *= d.N
	}
	return n
}

// addr 算一格的線性位址。
//
// ⚠ **BASIC 是列主序**（column-major）：`A(i,j)` ＝ base + ((j−lo₂)×n₁ + (i−lo₁)) × 寬度。
//
// 判準是實測的落點：現金（欄 0）與存款（欄 1）開局都是 25000。
// 行主序的話兩者相鄰（+0 與 +4），列主序則差一整個第一維（+0 與 +24）。
// **實測是後者**——而搞錯的話讀到的是別的玩家的別的欄位，值看起來完全合理。
func (a *Array) addr(idx ...int) uint32 {
	if len(idx) != len(a.Dims) {
		panic("oracle: 索引個數與維度不符")
	}
	stride, off := 1, 0
	for k, d := range a.Dims {
		off += (idx[k] - d.Lo) * stride
		stride *= d.N
	}
	return a.Base + uint32(off*a.Width)
}

// Int16 讀一格 16 位元有號值。
func (a *Array) Int16(idx ...int) int16 {
	return int16(a.o.m.Read16(a.addr(idx...)))
}

// Int32 讀一格 32 位元有號值。
func (a *Array) Int32(idx ...int) int32 {
	at := a.addr(idx...)
	return int32(uint32(a.o.m.Read16(at)) | uint32(a.o.m.Read16(at+2))<<16)
}

// Index 把線性位址反查成索引。
//
// **這是「這個動作改了哪一格」的答案**：拿快照差分出來的位址丟進來，
// 直接得到 `(格號, 欄位)`，不必再從畫面反推。
func (a *Array) Index(linear uint32) ([]int, bool) {
	if linear < a.Base || linear >= a.Base+uint32(a.Size()) {
		return nil, false
	}
	off := int(linear-a.Base) / a.Width
	idx := make([]int, len(a.Dims))
	for k, d := range a.Dims {
		idx[k] = off%d.N + d.Lo
		off /= d.N
	}
	return idx, true
}

// InRange 檢查索引在界內。**越界讀不會報錯**，只會讀到鄰居的資料。
func (a *Array) InRange(idx ...int) bool {
	if len(idx) != len(a.Dims) {
		return false
	}
	for k, d := range a.Dims {
		if idx[k] < d.Lo || idx[k] >= d.Lo+d.N {
			return false
		}
	}
	return true
}
