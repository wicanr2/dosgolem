package machine

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestFD2NaturalDriverInitialization(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, services := fixedFD2Machine(t)
	if err := InstallDOS4GWBIOSData(m); err != nil {
		t.Fatal(err)
	}
	ports := NewLEOPLPorts()
	m.CPU.PortIn = ports.In8
	m.CPU.PortOut = ports.Out8
	services.DPMI.RealModeIO = ports
	for step := 0; step < 100000; step++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("第%d步：%v；實模式=%+v", step, err, services.DPMI.RealModeLast)
		}
		tr := services.DPMI.RealModeLast
		if tr == nil || !tr.Returned {
			continue
		}
		input, err := hex.DecodeString(strings.ReplaceAll(tr.InputPacket, " ", ""))
		if err != nil {
			t.Fatal(err)
		}
		if len(input) != 50 || input[28] != 2 || input[29] != 5 {
			continue
		}
		output, err := hex.DecodeString(strings.ReplaceAll(tr.OutputPacket, " ", ""))
		if err != nil {
			t.Fatal(err)
		}
		if len(output) != 50 || output[28] != 1 || output[29] != 0 {
			t.Fatal("原版驅動初始化未成功")
		}
		if ports.Reads[0x3da] == 0 {
			continue
		}
		if ports.Reads[0x228] == 0 || ports.Writes[0x228] == 0 {
			t.Fatal("原版未走完音源及回掃查詢")
		}
		if len(services.DPMI.Unimplemented) != 0 {
			t.Fatal("路徑含未實作服務")
		}
		t.Logf("原版入口自然執行%d步，實模式初始化返回；尚非遊戲畫面或玩家路徑驗收", step+1)
		return
	}
	t.Fatal("有界執行未完成驅動初始化")
}

func TestFD2NaturalDigitalDriverInitialization(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, services := fixedFD2Machine(t)
	if err := InstallDOS4GWBIOSData(m); err != nil {
		t.Fatal(err)
	}
	ports := NewLEOPLPorts()
	m.CPU.PortIn = ports.In8
	m.CPU.PortOut = ports.Out8
	services.DPMI.RealModeIO = ports
	for step := 0; step < 100000; step++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("第%d步：%v", step, err)
		}
		tr := services.DPMI.RealModeLast
		if tr == nil || !tr.Returned || ports.DMACompletions == 0 || ports.IRQ7Deliveries == 0 {
			continue
		}
		input, err := hex.DecodeString(strings.ReplaceAll(tr.InputPacket, " ", ""))
		if err != nil {
			t.Fatal(err)
		}
		if len(input) != 50 || input[28] != 4 || input[29] != 3 {
			continue
		}
		output, err := hex.DecodeString(strings.ReplaceAll(tr.OutputPacket, " ", ""))
		if err != nil {
			t.Fatal(err)
		}
		if len(output) != 50 || output[28] != 5 || output[29] != 0x84 {
			t.Fatal("數位驅動回傳值回歸")
		}
		if len(services.DPMI.Unimplemented) != 0 || len(ports.PCM) == 0 {
			t.Fatal("未完成資料傳輸或含未實作服務")
		}
		t.Logf("原版入口%d步；DMA完成%d、IRQ7派送%d；時序近似，AX8405僅執行器回歸，尚非DOSBox同狀態驗收", step+1, ports.DMACompletions, ports.IRQ7Deliveries)
		return
	}
	t.Fatal("有界執行未完成數位音效驅動初始化")
}

func TestFD2NaturalMode13Initialization(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT未設定")
	}
	m, services := fixedFD2Machine(t)
	if err := InstallDOS4GWBIOSData(m); err != nil {
		t.Fatal(err)
	}
	p := NewLEOPLPorts()
	m.CPU.PortIn = p.In8
	m.CPU.PortOut = p.Out8
	services.DPMI.RealModeIO = p
	if !InstallLEVideo(m, p) {
		t.Fatal("視訊裝置")
	}
	for step := 0; step < 200000; step++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("第%d步：%v", step, err)
		}
		if m.Video.Mode != 0x13 {
			continue
		}
		if !p.PIT0.Loaded || p.DMACompletions == 0 || p.IRQ7Deliveries == 0 || len(services.DPMI.Unimplemented) != 0 {
			t.Fatal("初始化依賴未完成")
		}
		if len(m.Video.Indexed()) != 64000 {
			t.Fatal("模式13色號介面")
		}
		t.Logf("原版入口%d步真正設定mode13；仍為空白初始化，不是遊戲畫面", step+1)
		return
	}
	t.Fatal("有界執行未達mode13")
}

func TestFD2NaturalIntroClockWait(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT未設定")
	}
	m, services := fixedFD2Machine(t)
	if err := InstallDOS4GWBIOSData(m); err != nil {
		t.Fatal(err)
	}
	p := NewLEOPLPorts()
	m.CPU.PortIn = p.In8
	m.CPU.PortOut = p.Out8
	services.DPMI.RealModeIO = p
	if !InstallLEVideo(m, p) || !InstallLEBIOSClock(m, p) {
		t.Fatal("開場平台")
	}
	waiting := false
	for step := 0; step < 3500000; step++ {
		// 僅觀測原版對應DOSBox的同一程式位置，未改EIP或等待結果。
		if m.CPU.EIP == 0x17add {
			waiting = true
		}
		if waiting && m.CPU.EIP == 0x17adf {
			if p.BIOSClock.Deliveries == 0 || p.Writes[0x3c9] < 768 || len(services.DPMI.Unimplemented) != 0 {
				t.Fatal("等待退出依賴")
			}
			visible := false
			for _, v := range m.Video.Indexed() {
				if v != 0 {
					visible = true
					break
				}
			}
			if !visible {
				t.Fatal("原版開場未有色號")
			}
			t.Logf("原版入口%d步退出BIOS等待，IRQ0=%d；局部開場回歸，非完整操作對拍", step, p.BIOSClock.Deliveries)
			return
		}
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("第%d步：%v", step, err)
		}
	}
	t.Fatal("原版開場未退出時鐘等待")
}
