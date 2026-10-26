//go:build frontendlinked

package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	sc "github.com/wicanr2/dosgolem/apps/strikecommander"
	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/session"
)

// linkedRuntime owns the real app assembly and its sealed session. No HD
// producer is configured: an HD request uses the app's published fixed fallback.
type linkedRuntime struct {
	owner   *session.Owner
	app     *sc.FrontendRuntime
	stopped bool
}

func loadRuntimeAssets(root string, mix *mixer.Mixer, fonts *sc.TextFonts) (*sc.FrontendRuntime, error) {
	if mix == nil || fonts == nil {
		return nil, fmt.Errorf("016 Runtime 缺少混音器或已驗證字型")
	}
	h := sha256.New()
	catalogs := make(map[string]map[string]string)
	for _, family := range []string{"opttest_labels", "menu_gfx", "strike_hud", "strike_mfd", "strike_plaque", "speech", "frontend"} {
		path := resource(filepath.Join("text", family+".zh-TW.tsv"))
		raw, e := os.ReadFile(path)
		if e != nil {
			return nil, fmt.Errorf("016 catalog %s: %w", family, e)
		}
		h.Write(raw)
		catalogs[family], e = sc.LoadCatalog(path)
		if e != nil {
			return nil, fmt.Errorf("016 catalog %s: %w", family, e)
		}
	}
	raw, e := os.ReadFile(resource("font/zh-TW.cjkb.expected.tsv"))
	if e != nil {
		return nil, e
	}
	h.Write(raw)
	raw, e = os.ReadFile(resource("text/speech.index.tsv"))
	if e != nil {
		return nil, e
	}
	h.Write(raw)
	index, e := sc.ParseSpeechIndex(raw)
	if e != nil {
		return nil, e
	}
	var identity [32]byte
	copy(identity[:], h.Sum(nil))
	return sc.NewFrontendRuntime(sc.FrontendRuntimeConfig{
		Fonts: fonts, Catalog: catalogs["opttest_labels"], GfxCatalog: catalogs["menu_gfx"],
		FlightCatalogs: &sc.FlightCatalogs{HUD: catalogs["strike_hud"], MFD: catalogs["strike_mfd"], Plaque: catalogs["strike_plaque"]},
		SpeechIndex:    index, SpeechCatalog: catalogs["speech"], FontCatalogSHA: identity,
		CacheRoot: root, Subtitles: true, Sparse: false, Audio: mix, PCM: mix,
		Menu: &sc.FrontendMenuConfig{},
	})
}

func openRuntime(originalRoot, saveRoot string, mix *mixer.Mixer, fonts *sc.TextFonts, stateFile string, rebase []string) (runtimeGame, error) {
	library, e := loadFrontendPrograms(originalRoot)
	if e != nil {
		return nil, e
	}
	app, e := loadRuntimeAssets(originalRoot, mix, fonts)
	if e != nil {
		return nil, e
	}
	observer := &frontendPlayerObserver{FrontendRuntime: app, program: frontendProgramTracker{library: library}}
	owner, e := bootFrontendSession(observer, originalRoot, saveRoot, stateFile, rebase)
	if e != nil {
		app.Close()
		return nil, e
	}
	name, e := identifyFrontendBootProgram(owner, library)
	if e == nil {
		var bootstrap sc.FrontendMenuBootstrap
		bootstrap, e = frontendMenuBootstrap(stateFile != "", name)
		if e == nil {
			e = app.InitializeMenu(bootstrap)
		}
	}
	if e != nil {
		owner.Close()
		app.Close()
		return nil, e
	}
	observer.program.name = name
	return &linkedRuntime{owner: owner, app: app}, nil
}

func (r *linkedRuntime) Owner() *session.Owner             { return r.owner }
func (r *linkedRuntime) MenuState() (period, visible bool) { return r.app.MenuState() }
func (r *linkedRuntime) NotifyStopped() error {
	if r.stopped {
		return nil
	}
	if e := r.app.ObserveMenuProgram(sc.FrontendMenuStopped); e != nil {
		return e
	}
	r.stopped = true
	return nil
}

func frontendMenuProgram(name string) (sc.FrontendMenuProgram, error) {
	switch name {
	case "SCCD.EXE":
		return sc.FrontendMenuSCCD, nil
	case "MENU.EXE":
		return sc.FrontendMenuMENU, nil
	case "OPTTEST.EXE", "MKTERR.EXE", "STRIKE.EXE", "TOPTTST1.EXE":
		return sc.FrontendMenuOther, nil
	default:
		return 0, fmt.Errorf("016a 未驗證的程式分類 %q", name)
	}
}

func frontendMenuBootstrap(fromState bool, name string) (sc.FrontendMenuBootstrap, error) {
	p, e := frontendMenuProgram(name)
	if e != nil {
		return sc.FrontendMenuBootstrap{}, e
	}
	if fromState && p == sc.FrontendMenuOther {
		return sc.FrontendMenuBootstrap{Boot: sc.FrontendMenuFromState, Program: p}, nil
	}
	if !fromState && p == sc.FrontendMenuSCCD {
		return sc.FrontendMenuBootstrap{Boot: sc.FrontendMenuColdOriginal, Program: p}, nil
	}
	return sc.FrontendMenuBootstrap{}, fmt.Errorf("016a 不支援此啟動組合：fromState=%t，已驗程式=%s", fromState, name)
}

func (r *linkedRuntime) Prepare(scale int, lang string, hd, subtitles bool, version uint64, dst []byte) error {
	req := sc.FrontendPrepareRequest{Scale: scale, Language: lang, HD: hd, Subtitles: subtitles, SettingsVersion: version}
	prepared, e := r.app.Prepare(req)
	if e != nil {
		return e
	}
	defer prepared.Release()
	composed, ok := r.app.Compose(req)
	if !ok {
		return fmt.Errorf("016 Runtime Compose: NotPrepared")
	}
	defer composed.Release()
	want, e := prepared.Info()
	if e != nil {
		return e
	}
	got, e := composed.Info()
	if e != nil {
		return e
	}
	if got != want {
		return fmt.Errorf("016 Runtime Prepare/Compose 身分不同")
	}
	return composed.CopyRGBA(dst)
}

func (r *linkedRuntime) Close() error {
	e := r.owner.Close()
	r.app.Close()
	return e
}
