// Package dosfile 是 DOS 檔案服務的語意核心。
//
// 它**只認識「handle 對到一個開著的檔」與 DOS 的錯誤碼**——不認識暫存器
// 有幾個位元、不認識記憶體長什麼樣、不認識暫存層或原版目錄。
//
// 為什麼要獨立成一包：同一組語意目前有兩個前端在用。16 位元那條走
// `internal/dos`（`cpu.CPU` ＋ 1 MB 位址空間 ＋ 暫存層），32 位元那條走
// `internal/machine` 的 LE 執行環境（`cpu386.CPU` ＋ 平坦位址空間 ＋ 唯讀
// 檔案提供者）。**兩邊的政策不同，語意必須相同**：位移是有號的、
// EOF 不是錯誤、origin 只有 0／1／2、讀不滿不算失敗。這幾條各寫一份的話，
// 兩邊會慢慢分岔，而分岔的症狀是「同一個檔在 16 位元程式裡讀得到、
// 在 32 位元程式裡少了最後一段」——沒有人會懷疑是 seek 的號誌不同。
package dosfile

import "io"

// DOS 的錯誤碼（`AH=59h` 回的就是這些）。
//
// 放在這裡是為了**只有一份**：兩個前端各自寫魔術數字的話，同一種失敗
// 會在兩邊回不同的碼，而呼叫端是照碼決定要重試還是放棄的。
const (
	ErrInvalidFunction = 1
	ErrFileNotFound    = 2
	ErrPathNotFound    = 3
	ErrTooManyOpen     = 4
	ErrAccessDenied    = 5
	ErrInvalidHandle   = 6
	ErrFileExists      = 80
)

// File 是一個開著的檔。字元裝置（`EMMXXXX0`、`CON`）不是 File，
// 兩個前端都用 nil 表示——見 Read／Seek 的說明。
type File interface {
	io.ReadSeeker
}

// Read 照 `AH=3Fh` 的語意讀進 buf，回「讀到幾個」與 DOS 錯誤碼（0 ＝ 成功）。
//
// ⚠ **EOF 不是錯誤。** 讀到檔尾要回 0 個位元組加成功，程式靠這個判斷
// 「檔案讀完了」。回失敗的話它會走錯誤路徑，而那多半是「檔案損毀」的訊息。
//
// ⚠ **讀不滿也不是錯誤。** 要求 512 拿到 300 是合法的；程式看的是回傳值，
// 不是有沒有拿滿。
//
// f 為 nil ＝ 字元裝置，回 0 個位元組加成功（＝ 立刻 EOF）。
func Read(f File, buf []byte) (int, uint16) {
	if f == nil || len(buf) == 0 {
		return 0, 0
	}
	n, err := f.Read(buf)
	if n < 0 {
		n = 0
	}
	if err != nil && err != io.EOF {
		return n, ErrAccessDenied
	}
	return n, 0
}

// Seek 照 `AH=42h` 的語意移動檔案指標，回新位置與 DOS 錯誤碼。
//
// ⚠ **CX:DX 是有號的。** 從檔尾往回退 100 個位元組傳的是 −100，
// 而它在暫存器裡長得像 0xFFFFFF9C。當成無號的話會 seek 到 4 GB 之外，
// 接下來的讀取回 0 個位元組——看起來像「檔案是空的」。呼叫端傳進來的
// off 已經是有號值（`int32`），這裡不再猜。
//
// ⚠ **origin 只有 0（檔頭）／1（目前）／2（檔尾）。** 別的值要失敗；
// 放行的話 io 套件會把它當成 0，而程式以為自己從檔尾往回找。
//
// f 為 nil ＝ 字元裝置，不能 seek。
func Seek(f File, off int32, origin uint8) (uint32, uint16) {
	if f == nil || origin > 2 {
		return 0, ErrInvalidFunction
	}
	old, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, ErrInvalidFunction
	}
	pos, err := f.Seek(int64(off), int(origin))
	if err != nil || pos < 0 || uint64(pos) > uint64(^uint32(0)) {
		// **失敗要退回原位。** 有些實作在失敗之後把指標留在半途，
		// 而呼叫端只看 CF——它會從一個它沒指定過的位置繼續讀。
		_, _ = f.Seek(old, io.SeekStart)
		return 0, ErrInvalidFunction
	}
	return uint32(pos), 0
}

// SignedOffset 把 `CX:DX` 這一對暫存器組成 `AH=42h` 用的有號位移。
func SignedOffset(cx, dx uint16) int32 {
	return int32(uint32(cx)<<16 | uint32(dx))
}

// Table 是 handle 表，給**不需要暫存層與寫入政策**的前端用（目前是 32 位元
// 那條）。16 位元那條有自己的 handle 結構（要帶 PSP、寫入權、參考計數），
// 用上面那幾支自由函式共用語意就好——政策不同的東西硬共用只會讓兩邊
// 都變複雜。
type Table struct {
	files map[uint16]io.ReadSeekCloser
	names map[uint16]string
	next  uint16
}

// FirstHandle 是第一個可以配出去的號碼。0–4 是 DOS 開好的標準 handle
// （stdin／stdout／stderr／stdaux／stdprn），配出去會蓋掉它們。
const FirstHandle = 5

// NewTable 造一張空的 handle 表。
func NewTable() *Table {
	return &Table{files: map[uint16]io.ReadSeekCloser{},
		names: map[uint16]string{}, next: FirstHandle}
}

func (t *Table) init() {
	if t.files == nil {
		t.files = map[uint16]io.ReadSeekCloser{}
		t.names = map[uint16]string{}
	}
	if t.next < FirstHandle {
		t.next = FirstHandle
	}
}

// Add 把一個開好的檔放進表裡，回 handle 與 DOS 錯誤碼。
func (t *Table) Add(f io.ReadSeekCloser, name string) (uint16, uint16) {
	t.init()
	if t.next == 0xFFFF {
		return 0, ErrTooManyOpen
	}
	h := t.next
	t.next++
	t.files[h] = f
	t.names[h] = name
	return h, 0
}

// Get 取出 handle 對到的檔。
func (t *Table) Get(h uint16) (io.ReadSeekCloser, bool) {
	f, ok := t.files[h]
	return f, ok
}

// Name 回 handle 開的時候用的名字（觀測用；找不到回空字串）。
func (t *Table) Name(h uint16) string { return t.names[h] }

// Has 回 handle 在不在表裡。
func (t *Table) Has(h uint16) bool {
	_, ok := t.files[h]
	return ok
}

// Read 讀進 buf。
func (t *Table) Read(h uint16, buf []byte) (int, uint16) {
	f, ok := t.files[h]
	if !ok {
		return 0, ErrInvalidHandle
	}
	return Read(f, buf)
}

// Seek 移動檔案指標。
func (t *Table) Seek(h uint16, off int32, origin uint8) (uint32, uint16) {
	f, ok := t.files[h]
	if !ok {
		return 0, ErrInvalidHandle
	}
	return Seek(f, off, origin)
}

// Close 關掉一個 handle。
//
// **關過的號碼不重用**：重用的話，程式關掉之後又拿舊號碼去讀，
// 讀到的是別人的檔——而它不會報錯。
func (t *Table) Close(h uint16) uint16 {
	f, ok := t.files[h]
	if !ok {
		return ErrInvalidHandle
	}
	delete(t.files, h)
	delete(t.names, h)
	if err := f.Close(); err != nil {
		return ErrAccessDenied
	}
	return 0
}

// CloseAll 關掉所有 handle，回第一個錯誤。
func (t *Table) CloseAll() error {
	var first error
	for h, f := range t.files {
		if err := f.Close(); err != nil && first == nil {
			first = err
		}
		delete(t.files, h)
		delete(t.names, h)
	}
	return first
}
