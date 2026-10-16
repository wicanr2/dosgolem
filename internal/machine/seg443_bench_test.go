package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// BenchmarkSegmentedRepMovsd32 是 docs/spec/443-segmented-32bit-code-and-stack §3 第 15 項：分段模式、分頁開（恆等頁表，
// 低 1 MB）、D=1 碼段的 rep movsd，一次 16000 個 dword（JEMM CU 的整段複製）。ns/op 是一次整段複製的時間
// （驗收門檻 5 ms 以下）；ns/dword 是每次迭代的時間。不做時間斷言，數值記入收據。
func BenchmarkSegmentedRepMovsd32(b *testing.B) {
	const (
		codeBase = 0x30000
		src      = 0x40000
		dst      = 0x50000
		n        = 16000
	)
	m, _ := newPagedProg(b, []byte{0x90}, fPWU)
	if err := m.enterSegmented(); err != nil {
		b.Fatal(err)
	}
	putDesc(m, segGDT, 0x28, codeBase, 0xFFFF, 0x9A, 0x4)
	m.SetGDTR(segGDT, segGDTLim)
	copy(m.Mem[codeBase:], []byte{0xF3, 0xA5}) // rep movsd（D=1：ESI、EDI、ECX、dword）
	setSel(m, cpu.CS, 0x28)
	flat := SegCache{Base: 0, Limit: 0xFFFFFFFF, Attr: 0x8093}
	m.CPU.SetSegCache(cpu.DS, flat)
	m.CPU.SetSegCache(cpu.ES, flat)
	for i := 0; i < 4*n; i++ {
		m.Mem[src+i] = byte(i*13 + i>>8)
	}
	if !m.CPU.Code32() || !m.paging {
		b.Fatal("前提：D=1 碼段、分頁開")
	}
	run := func() {
		m.CPU.IP = 0
		setReg32(m, cpu.CX, n)
		setReg32(m, cpu.SI, src)
		setReg32(m, cpu.DI, dst)
		if err := m.Step(); err != nil {
			b.Fatal(err)
		}
	}
	run()
	if reg32(m.CPU, cpu.CX) != 0 || m.CPU.IP != 2 || !bytes.Equal(m.Mem[dst:dst+4*n], m.Mem[src:src+4*n]) {
		b.Fatal("rep movsd 沒有完成整段複製")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run()
	}
	b.StopTimer()
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/n, "ns/dword")
}
