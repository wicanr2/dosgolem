package machine_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 規格 461 §5 第 6 條：實模式遊戲的開機路徑（不送輸入、不存讀），之後把 PortLog 的 40h、43h 重播給
// 獨立的 pit 值與 pitWrite（PIT461Report），列出所有不同之處。
//
//	DOSGOLEM_PIT461_SAMPLES="名稱|exe|root|步數;名稱|exe|root|步數"
//
// 原版素材由玩家自備；沒設就略過（略過不等於通過）。
func TestPIT461SampleBoot(t *testing.T) {
	spec := os.Getenv("DOSGOLEM_PIT461_SAMPLES")
	if spec == "" {
		t.Skip("DOSGOLEM_PIT461_SAMPLES 未設定")
	}
	for _, item := range strings.Split(spec, ";") {
		f := strings.Split(item, "|")
		if len(f) != 4 {
			t.Fatalf("樣本格式錯：%q", item)
		}
		name, exe, root := f[0], f[1], f[2]
		n, err := strconv.ParseUint(f[3], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) {
			img, err := os.ReadFile(exe)
			if err != nil {
				t.Fatal(err)
			}
			m := machine.New()
			if err := m.LoadEXE(img); err != nil {
				t.Fatal(err)
			}
			d := dos.New(m, root)
			d.Install()
			defer d.Close()
			stop := "步數用完"
			for m.Steps < n {
				if d.Exited {
					stop = "程式結束"
					break
				}
				if m.CPU.Halted {
					stop = "HLT"
					break
				}
				if err := m.Step(); err != nil {
					stop = "錯誤：" + err.Error()
					break
				}
			}
			writes, diffs, final := machine.PIT461Report(t, name, m.PortLog)
			t.Logf("PIT461BOOT name=%s steps=%d stop=%s PITDiv=%d PITDivisor=%d programmed=%t replayWrites=%d diffs=%d",
				name, m.Steps, stop, m.PITDiv, m.PITDivisor(), m.PITProgrammed(), writes, diffs)
			if final != m.PITDiv {
				t.Errorf("重播的 PITDiv=%d 與原機器 %d 不同：PortLog 不完整或解讀者不同", final, m.PITDiv)
			}
			if writes == 0 {
				t.Errorf("%s 沒有 40h／43h 寫入，抽樣無效", name)
			}
		})
	}
}
