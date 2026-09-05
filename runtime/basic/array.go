package basic

import "github.com/wicanr2/dosgolem/oracle"

// 編譯後 BASIC 的陣列。
//
// **描述子的解讀是編譯器的版面，不是通用 DOS 的**（`docs/spec/006` §2.1）：
// 前兩個 word 是 `(位移, 段)`、索引是列主序。換一個 Turbo Pascal 編的程式，
// 這個解讀直接錯，而且**不會報錯**，只會讀出一片看起來像資料的東西。

// Dim 是一個維度：下界與元素數。
//
// `1..6` 寫成 `Dim{Lo: 1, N: 6}`，`0..59` 寫成 `Dim{Lo: 0, N: 60}`。
// 維度取自 `rich2/docs/re/014` §2 的 DIM 對照表。
type Dim struct{ Lo, N int }

// Array 是一個 BASIC 陣列的檢視。
type Array struct {
	o     *oracle.Oracle
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
func NewArray(o *oracle.Oracle, descriptor uint16, dims []Dim, width int) *Array {
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
		panic("basic: 索引個數與維度不符")
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
	return int16(a.o.Word(oracle.Phys(a.addr(idx...))))
}

// Int32 讀一格 32 位元有號值。
func (a *Array) Int32(idx ...int) int32 {
	at := a.addr(idx...)
	return int32(uint32(a.o.Word(oracle.Phys(at))) |
		uint32(a.o.Word(oracle.Phys(at+2)))<<16)
}

// Float32 讀一格單精度浮點。
//
// **編譯後的 BASIC 在這裡用 IEEE 754，不是 MBF**——浮點指令被 Microsoft
// 的浮點模擬器換成 `INT 34h`–`INT 3Dh`，那個模擬器算的是 IEEE。
// 判準是拿已知的常數對：`ds:1B5Ah` 讀出來剛好是 `1.0`。
func (a *Array) Float32(idx ...int) float32 {
	return a.o.Float(oracle.Phys(a.addr(idx...)))
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

// Bytes 讀一格的原始位元組。
//
// 給**定長字串表**用：編譯後的 BASIC 把固定長度的字串陣列存成連續的
// 位元組區塊（大富翁2 的 `17ECh` 是每格 20 bytes），沒有長度前綴也沒有
// 結束符，尾端用空白補滿。**所以要自己 trim**，不能當 C 字串讀。
func (a *Array) Bytes(idx ...int) []byte {
	base := a.addr(idx...)
	out := make([]byte, a.Width)
	for i := range out {
		out[i] = a.o.Byte(oracle.Phys(base + uint32(i)))
	}
	return out
}
