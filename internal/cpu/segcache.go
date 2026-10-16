package cpu

// 分段模式的狀態（docs/spec/434-legacy-core-segmented-mode §2）。
//
// 分段模式＝ CR0.PE=1、VM=0 時既有核心執行 16 位元保護模式碼。快取的索引與既有常數相同
// （ES、CS、SS、DS、FS、GS）。segmented 與 cache 只經下列方法讀寫，供 Machine 使用。
//
// 不變式（由 442 §2.1 修訂）：segmented=true ⇒ profile 開啟、VM=0、pmCPL=0，且（PE=1）或（PE=0、PG=0，
// 442 的殘留快取實模式）⇒ 432 的 xlat=true ⇒ Code==nil（由 Machine 的 setter 與 syncCodeFastPath 維持，
// 這裡不檢查）。

// SegCache 是段描述元快取（433 §3.2 的型別與編碼，434 §2 由 internal/machine 移來）。
// Attr：bits 0–7 為 386 描述元的 access byte；bits 8–11 恆 0；bit 12 AVL；bit 13 恆 0；
// bit 14 D/B；bit 15 G。Limit 是已依 G 展開的有效界限。LDTR、TR 的屬性用同一編碼。
//
// 「不可用」的快取：Attr 的 P 位元（bit 7）為 0，且 Base、Limit 為 0（零值即不可用）。
type SegCache struct {
	Base, Limit uint32
	Attr        uint16
}

// Segmented 回目前是否在分段模式。
func (c *CPU) Segmented() bool { return c.segmented }

// SetSegmented 開關分段模式。只由 Machine 呼叫（進入分段模式、Restore、LoadState、profile 切換）。
// 442 §2.7：同時清除離開檢查旗標。
func (c *CPU) SetSegmented(on bool) { c.segmented, c.leavePending = on, false }

// SegCache 回段暫存器 i（ES、CS、SS、DS、FS、GS）的快取。
func (c *CPU) SegCache(i int) SegCache { return c.cache[i] }

// SetSegCache 寫段暫存器 i 的快取。442 §2.7：同時清除離開檢查旗標。
func (c *CPU) SetSegCache(i int, sc SegCache) { c.cache[i], c.leavePending = sc, false }
