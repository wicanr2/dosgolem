package cpu

// SysBus 是既有核心存取系統暫存器的介面（docs/spec/433-real-mode-system-instructions §3.1）。
//
// 由 machine.Machine 實作，只在保護模式 profile 開啟時裝在 CPU.Sys；nil 時行為完全不變。
// 所有寫入都經這裡，由 Machine 的方法重算衍生欄位（431 §4）。
type SysBus interface {
	// CR 回 CRn 的原始值；mov r32,cr0 的 |7FFFFFF0h 與 CR3 的遮罩在指令處理函式做。
	// 不存在的 CRn（CR1、CR4 起）回錯誤。
	CR(n int) (uint32, error)
	SetCR(n int, v uint32) error
	// DR 讀寫 DR0–DR7（DR4、DR5 是 DR6、DR7 的別名）。
	DR(n int) uint32
	SetDR(n int, v uint32)
	GDTR() (base uint32, limit uint16)
	SetGDTR(base uint32, limit uint16)
	IDTR() (base uint32, limit uint16)
	SetIDTR(base uint32, limit uint16)
	// User 回目前是否為使用者模式（432 的 cplUser）。實模式恆為 false。
	User() bool
	// IVTMismatch 是 433 §2 的 IDTR 守門條件：profile 開啟、PE=0 而 IDTR 基底不為 0。
	IVTMismatch() bool
}
