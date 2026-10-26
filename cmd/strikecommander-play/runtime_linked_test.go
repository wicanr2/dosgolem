//go:build frontendlinked

package main

import (
	"os"
	"path/filepath"
	"testing"

	sc "github.com/wicanr2/dosgolem/apps/strikecommander"
	"github.com/wicanr2/dosgolem/apps/strikecommander/player"
	"github.com/wicanr2/dosgolem/audio/mixer"
)

func TestFrontendRuntimeAssetsPrepare(t *testing.T) {
	root, assets, fontRoot := os.Getenv("SC_FRONTEND_EXE_ROOT"), os.Getenv("SC_FRONTEND_RESOURCE_ROOT"), os.Getenv("SC_FRONTEND_FONT_ROOT")
	if root == "" || assets == "" || fontRoot == "" {
		t.Skip("explicit real immutable assets required; no session, guest or GUI")
	}
	t.Chdir(assets)
	fonts, e := sc.LoadTextFonts(fontRoot, filepath.Join(assets, "font/zh-TW.cjkb.expected.tsv"))
	if e != nil {
		t.Fatal(e)
	}
	app, e := loadRuntimeAssets(root, mixer.New(player.StepsPerSecond), fonts)
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	if e = app.InitializeMenu(sc.FrontendMenuBootstrap{Boot: sc.FrontendMenuFromState, Program: sc.FrontendMenuOther}); e != nil {
		t.Fatal(e)
	}
	if !app.MenuPreload().Enabled {
		t.Fatal("R6 failed", app.MenuPreload())
	}
	r := linkedRuntime{app: app}
	version := uint64(0)
	for _, scale := range []int{2, 3} {
		for _, lang := range []string{"en", "zh-TW"} {
			for _, hd := range []bool{false, true} {
				version++
				dst := make([]byte, 320*scale*200*scale*4)
				if e := r.Prepare(scale, lang, hd, true, version, dst); e != nil {
					t.Fatal(e)
				}
				// No Frame or Session has been supplied: the real R2 result is a
				// black safety fallback, independently owned by this caller.
				for i, v := range dst {
					want := byte(0)
					if i%4 == 3 {
						want = 255
					}
					if v != want {
						t.Fatal("unexpected no-image safety pixel", i, v)
					}
				}
				dst[0] = 111
				if e := r.Prepare(scale, lang, hd, true, version, dst); e != nil || dst[0] != 0 {
					t.Fatal("lease/caller ownership", e)
				}
			}
		}
	}
	if e := r.Prepare(2, "en", false, false, 1, make([]byte, 4)); e == nil {
		t.Fatal("CopyRGBA length error hidden")
	}
	for _, name := range []string{"MENU.EXE", "SCCD.EXE"} {
		if _, e := frontendMenuBootstrap(true, name); e == nil {
			t.Fatal("unsupported state MENU/SCCD bootstrap")
		}
	}
	if _, e := loadRuntimeAssets(root, nil, fonts); e == nil {
		t.Fatal("missing audio collector hidden")
	}
	t.Log("real font/catalog/R6 constructor and two-lease Prepare/Compose/copy/release; no VM or GUI")
}

func TestR8VerifiedBootstrapAndClassMapping(t *testing.T) {
	if b, e := frontendMenuBootstrap(false, "SCCD.EXE"); e != nil || b.Boot != sc.FrontendMenuColdOriginal || b.Program != sc.FrontendMenuSCCD {
		t.Fatal("cold classified SCCD", b, e)
	}
	for _, name := range []string{"OPTTEST.EXE", "MKTERR.EXE", "STRIKE.EXE", "TOPTTST1.EXE"} {
		if b, e := frontendMenuBootstrap(true, name); e != nil || b.Boot != sc.FrontendMenuFromState || b.Program != sc.FrontendMenuOther {
			t.Fatal("verified Other state", b, e)
		}
		if _, e := frontendMenuBootstrap(false, name); e == nil {
			t.Fatal("cold non-SCCD accepted")
		}
	}
	for _, name := range []string{"MENU.EXE", "SCCD.EXE", "unknown.exe", "menu.exe"} {
		if _, e := frontendMenuBootstrap(true, name); e == nil {
			t.Fatal("unsupported state bootstrap", name)
		}
	}
	if _, e := frontendMenuProgram("unknown.exe"); e == nil {
		t.Fatal("arbitrary name classified Other")
	}
	// No Session is created by these bootstrap/asset tests.
}
