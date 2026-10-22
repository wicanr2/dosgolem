package machine

import "testing"

// docs/spec/458 §3.3 的 BenchmarkVideoWriteObserved：裝空回呼，連續寫 A0000 視窗（mode 13h 鏈結）。
// 只用 2436db8 已有的 API，同一個檔案也在 2436db8 上編譯，改動前後交錯比對每筆 ns。
func BenchmarkVideoWriteObserved(b *testing.B) {
	m := New()
	m.SetVideoMode(0x13)
	m.ObserveVideoWrites(func(VideoWrite) {})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Write8(0xA0000+uint32(i)&0xFFFF, uint8(i))
	}
}
