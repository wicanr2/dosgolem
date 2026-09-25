package dos

import "testing"

// TestExit255IsRecordedAsEnded 是 spec-206 的驗收：退出碼 255 不能
// 被當成「還沒結束」（`retro-runtime-study-private#47`）。
//
// WCG 乾淨退出碼就是 255——以前 `Exit == 0xFF` 同時是碼與哨兵，
// `-trace-after-exit` 永不觸發，`memops` 也分不清。
func TestExit255IsRecordedAsEnded(t *testing.T) {
	m, d := newTest(t)
	// 子行程：mov ax,4CFFh; int 21h（離開碼 255）。
	writeChild(t, d, "CHILD.COM", []byte{0xB8, 0xFF, 0x4C, 0xCD, 0x21})
	execChild(m, d, "CHILD.COM")
	for i := 0; i < 200 && len(d.procStack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
		t.Fatal("子程式沒有結束")
	}
	rec := d.ExecLog[len(d.ExecLog)-1]
	if !rec.Ended {
		t.Error("子 exit(255) 之後 Ended 還是 false（哨兵又把 255 當沒結束）")
	}
	if rec.Exit != 0xFF {
		t.Errorf("Exit＝%02X，預期 FF（退出碼 255 本體）", rec.Exit)
	}
	// 再跑一支：PSP 重用時不能誤寫舊紀錄（配對只認還沒結束的那一筆）。
	writeChild(t, d, "CHILD2.COM", []byte{0xB8, 0x07, 0x4C, 0xCD, 0x21})
	execChild(m, d, "CHILD2.COM")
	for i := 0; i < 200 && len(d.procStack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	for _, e := range d.ExecLog {
		if e.Ended {
			n++
		}
	}
	if n != 2 {
		t.Errorf("兩支都結束了，已結束紀錄卻只有 %d 筆：%+v", n, d.ExecLog)
	}
	if got := d.ExecLog[len(d.ExecLog)-1].Exit; got != 0x07 {
		t.Errorf("第二支 Exit＝%02X，預期 07（誤寫了舊紀錄？）", got)
	}
}
