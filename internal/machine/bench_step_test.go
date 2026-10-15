package machine

import "testing"

// 執行速度基準（docs/spec/431-hybrid-cpu-protected-mode §7、
// docs/spec/432-linear-memory-and-paging §9 第 12 項）。
//
// 程式以 bytes 現場組出，不讀原版檔。迴圈有記憶體讀、讀改寫、push/pop、
// 暫存器運算與條件跳躍各一，每圈 8 道指令；DS 資料落在 0800h–08FFh，
// 不碰程式本身與 PSP。
//
// IRQ0 關閉（IRQ0Every=0）：沒有掛 internal/dos 時，int 1Ch 向量指到的預設 stub 是
// `CD 1C`，沒有人替它 IRET，第一次計時器中斷就會無限遞迴。幀間隔維持預設，
// tick 的入口判斷照常計入。
//
// 基準要在行為 commit 之前先提交：432 的實作要拿修改前的版本編出
// 測試程式，才能在同一個容器內交錯跑新舊（ns/op 就是每道指令的奈秒）。

// stepLoopCOM 是 BenchmarkStepRealMode 的合成迴圈。
//
//	0100  BB 00 08        mov bx,0800h
//	0103  8B 07           mov ax,[bx]       ; 迴圈起點
//	0105  05 03 00        add ax,3
//	0108  89 07           mov [bx],ax
//	010A  50              push ax
//	010B  58              pop ax
//	010C  43              inc bx
//	010D  81 E3 FF 08     and bx,08FFh      ; bx 落在 0800h–08FFh
//	0111  EB F0           jmp short 0103h
var stepLoopCOM = []byte{
	0xBB, 0x00, 0x08,
	0x8B, 0x07,
	0x05, 0x03, 0x00,
	0x89, 0x07,
	0x50,
	0x58,
	0x43,
	0x81, 0xE3, 0xFF, 0x08,
	0xEB, 0xF0,
}

// BenchmarkStepRealMode 以 Machine.Step 跑合成的實模式迴圈。
// ns/op 是每道指令的奈秒；Msteps/s 是每秒百萬道指令。
func BenchmarkStepRealMode(b *testing.B) {
	m := New()
	m.IRQ0Every = 0
	if err := m.LoadCOM(stepLoopCOM); err != nil {
		b.Fatal(err)
	}
	// 暖機：讓第一次 tick 之前的路徑都走過，並確認迴圈沒有飛掉。
	for i := 0; i < 1000; i++ {
		if err := m.Step(); err != nil {
			b.Fatal(err)
		}
	}
	// 迴圈在跑：DS:0800h 起的資料已被改寫。
	if m.Mem[uint32(PSPSeg)*16+0x800] == 0 {
		b.Fatal("合成迴圈沒有寫到資料區，可能飛掉了")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := m.Step(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds()/1e6, "Msteps/s")
}
