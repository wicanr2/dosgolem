package main

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

// docs/spec/433-real-mode-system-instructions §6、§8 第 6 項：-pm-profile。

func synthPMState(t *testing.T, pm bool) string {
	t.Helper()
	m := machine.New()
	if err := configureMem(m, 4096, false); err != nil {
		t.Fatal(err)
	}
	if err := configurePM(m, pm); err != nil {
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

func TestConfigurePMRequiresExtendedMemory(t *testing.T) {
	m := machine.New()
	if err := configureMem(m, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := configurePM(m, true); err == nil {
		t.Error("-pm-profile 與 -ext-mem-kb 0 同給：應回錯誤")
	}
	if err := configurePM(m, false); err != nil {
		t.Errorf("沒給 -pm-profile：不應回錯誤：%v", err)
	}
}

func TestConfigurePMAgainstStateFiles(t *testing.T) {
	on := synthPMState(t, true)
	off := synthPMState(t, false)
	for _, tc := range []struct {
		name string
		path string
		pm   bool
		fail bool
	}{
		{"profile 開的狀態檔、給 -pm-profile", on, true, false},
		{"profile 開的狀態檔、漏給 -pm-profile", on, false, true},
		{"profile 關的狀態檔、多給 -pm-profile", off, true, true},
		{"profile 關的狀態檔、不給", off, false, false},
	} {
		m := machine.New()
		if err := configureMem(m, 4096, false); err != nil {
			t.Fatal(err)
		}
		if err := configurePM(m, tc.pm); err != nil {
			t.Fatal(err)
		}
		d := dos.New(m, "/anchor")
		d.Install()
		err := state.Load(tc.path, m, d)
		if tc.fail && err == nil {
			t.Errorf("%s：應回錯誤", tc.name)
		}
		if !tc.fail && err != nil {
			t.Errorf("%s：不應回錯誤：%v", tc.name, err)
		}
	}
}

func TestConfigurePMSysInstalled(t *testing.T) {
	m := machine.New()
	if err := configureMem(m, 4096, false); err != nil {
		t.Fatal(err)
	}
	if err := configurePM(m, true); err != nil {
		t.Fatal(err)
	}
	if m.CPU.Sys == nil || !m.ProtectedModeProfile() {
		t.Error("-pm-profile 之後 Sys 應裝上")
	}
	if err := m.SetProtectedModeProfile(false); err != nil {
		t.Fatal(err)
	}
	if m.CPU.Sys != nil {
		t.Error("SetProtectedModeProfile(false) 後 Sys 應為 nil")
	}
}
