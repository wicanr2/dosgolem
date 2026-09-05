package dosgolem_test

import (
	"testing"

	"github.com/wicanr2/dosgolem"
)

// 對外介面的用處是「別的模組接得起來」。這一支就是從**外部**走一次
// 最小流程：造機器、載 .COM、掛服務、跑、讀畫面、找指紋。
//
// 少一個型別別名或常數不會讓內部測試變紅，只會讓使用端編不過——
// 而那時候通常已經在別的 repo 裡了。
func TestPublicSurfaceIsEnoughToRunSomething(t *testing.T) {
	m := dosgolem.New()

	// mov ah,9 / int 21h / int 20h，字串在後面。
	msg := "HI$"
	code := []byte{
		0xB4, 0x09, // mov ah,9
		0xBA, 0x09, 0x01, // mov dx,0109h
		0xCD, 0x21, // int 21h
		0xCD, 0x20, // int 20h
	}
	if err := m.LoadCOM(append(code, msg...)); err != nil {
		t.Fatal(err)
	}
	d := dosgolem.NewDOS(m, t.TempDir())
	d.Install()

	for i := 0; i < 100 && !d.Exited; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if !d.Exited {
		t.Fatal("程式沒有結束")
	}
	if got := string(d.Console); got != "HI" {
		t.Errorf("主控台是 %q，該是 %q", got, "HI")
	}

	// 暫存器索引與旗標常數要接得到。
	if m.CPU.Seg[dosgolem.CS] != dosgolem.PSPSeg {
		t.Errorf("CS = %04X", m.CPU.Seg[dosgolem.CS])
	}
	_ = m.CPU.Flag(dosgolem.IF)

	// 通用的觀測工具。
	if len(m.TextScreen(0)) != 25 {
		t.Error("TextScreen 沒有回 25 列")
	}
	if hits := m.Find([]byte(msg)); len(hits) == 0 {
		t.Error("Find 找不到剛剛載進去的字串")
	}
	if err := m.TypeScan("f"); err != nil {
		t.Fatal(err)
	}
	d.TypeKeys("f")
	if len(m.KeyQueue) == 0 || len(d.Keys) == 0 {
		t.Error("兩條輸入路徑至少有一條沒排進去")
	}
	_ = dosgolem.Key{Scan: 0x21, ASCII: 'f'}

	// 觀測工具也要從外面接得到——第二個案例就是靠它們對拍的。
	var hook dosgolem.WriteHook = func(*dosgolem.Machine, uint32, uint8, uint8) {}
	m.Unwatch(m.WatchWrite(0x500, 0x501, hook))
	m.Unwatch(m.WatchWord(0x500, func(*dosgolem.Machine, uint32, uint16, uint16) {}))
	m.ClearBreak(m.BreakAt(0x1234, 0x5678))
	if why, err := m.RunUntil(nil, 1); err != nil || why != dosgolem.StopBudget {
		t.Errorf("RunUntil 回 %v／%v", why, err)
	}
	_, _ = m.Insn()
}
