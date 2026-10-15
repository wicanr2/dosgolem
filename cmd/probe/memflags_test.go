package main

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

// docs/spec/430-physical-memory-and-xms-unification §5 第 10 項：
// configureMem 之後 state.Load，以合成狀態檔驗證 -ext-mem-kb 與 -no-ems 必須與狀態檔一致。

func synthState(t *testing.T, kb int, noEMS bool) string {
	t.Helper()
	m := machine.New()
	if err := configureMem(m, kb, noEMS); err != nil {
		t.Fatal(err)
	}
	if err := m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, "/anchor")
	d.Install()
	p := filepath.Join(t.TempDir(), "s.gz")
	if err := state.Save(p, m, d); err != nil {
		t.Fatal(err)
	}
	return p
}

func loadWith(t *testing.T, path string, kb int, noEMS bool) error {
	t.Helper()
	m := machine.New()
	if err := configureMem(m, kb, noEMS); err != nil {
		return err
	}
	d := dos.New(m, "/anchor")
	d.Install()
	return state.Load(path, m, d)
}

func TestConfigureMemAgainstStateFiles(t *testing.T) {
	plain := synthState(t, 0, false)
	ext := synthState(t, 8192, false)
	noEMS := synthState(t, 0, true)
	both := synthState(t, 8192, true)

	cases := []struct {
		name  string
		path  string
		kb    int
		noEMS bool
		fail  bool
	}{
		{"kb>0 狀態檔、旗標一致", ext, 8192, false, false},
		{"kb>0 狀態檔、kb 不同", ext, 4096, false, true},
		{"kb>0 狀態檔、漏給 -ext-mem-kb", ext, 0, false, true},
		{"kb==0 狀態檔、多給 -ext-mem-kb", plain, 8192, false, true},
		{"kb==0 狀態檔、一致", plain, 0, false, false},
		{"no-ems 狀態檔、一致", noEMS, 0, true, false},
		{"no-ems 狀態檔、漏給 -no-ems", noEMS, 0, false, true},
		{"一般狀態檔、多給 -no-ems", plain, 0, true, true},
		{"kb>0＋no-ems、一致", both, 8192, true, false},
		{"kb>0＋no-ems、漏給 -no-ems", both, 8192, false, true},
		{"kb>0＋no-ems、漏給 -ext-mem-kb", both, 0, true, true},
	}
	for _, tc := range cases {
		err := loadWith(t, tc.path, tc.kb, tc.noEMS)
		if tc.fail && err == nil {
			t.Errorf("%s：應回錯誤", tc.name)
		}
		if !tc.fail && err != nil {
			t.Errorf("%s：不應回錯誤：%v", tc.name, err)
		}
	}
}

func TestConfigureMemRejectsBadValues(t *testing.T) {
	for _, kb := range []int{1, 4095, 4097, 65536, -4} {
		if err := configureMem(machine.New(), kb, false); err == nil {
			t.Errorf("kb ＝ %d 應回錯誤（範圍由 SetExtendedMemoryKB 檢查）", kb)
		}
	}
	m := machine.New()
	if err := configureMem(m, 0, false); err != nil || m.ExtendedMemoryKB() != 0 || !m.EMSPresent() {
		t.Errorf("預設旗標應不改變機器：%v", err)
	}
	m = machine.New()
	if err := configureMem(m, 4096, true); err != nil || m.ExtendedMemoryKB() != 4096 || m.EMSPresent() {
		t.Errorf("兩者同給：%v", err)
	}
}
