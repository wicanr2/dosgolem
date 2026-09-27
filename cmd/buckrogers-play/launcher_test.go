package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeOriginals(t *testing.T, dir string, names map[string]string) {
	t.Helper()
	old := requiredOriginals
	t.Cleanup(func() { requiredOriginals = old })
	requiredOriginals = nil
	for _, canon := range []string{"START.EXE", "GAME.OVR"} {
		body := []byte("synthetic " + canon)
		sum := sha256.Sum256(body)
		requiredOriginals = append(requiredOriginals, struct{ name, sha string }{canon, hex.EncodeToString(sum[:])})
		if err := os.WriteFile(filepath.Join(dir, names[canon]), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "OTHER.DAX"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindFold(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "start.exe"), nil, 0o644)
	if n, err := findFold(d, "START.EXE"); err != nil || n != "start.exe" {
		t.Fatalf("%q %v", n, err)
	}
	os.WriteFile(filepath.Join(d, "Start.Exe"), nil, 0o644)
	if _, err := findFold(d, "START.EXE"); err == nil {
		t.Fatal("重複應報錯")
	}
	os.Mkdir(filepath.Join(d, "GAME.OVR"), 0o755)
	if _, err := findFold(d, "game.ovr"); err == nil {
		t.Fatal("目錄不應當成檔案")
	}
}

func TestPrepareGameImportThenReuse(t *testing.T) {
	orig, data := t.TempDir(), t.TempDir()
	fakeOriginals(t, orig, map[string]string{"START.EXE": "start.exe", "GAME.OVR": "Game.Ovr"})
	stale := filepath.Join(data, "game.importing-1")
	os.MkdirAll(stale, 0o755)
	game, exe, err := prepareGame(orig, data)
	if err != nil || exe != "start.exe" || game != filepath.Join(data, "game") {
		t.Fatalf("%s %s %v", game, exe, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("殘留的匯入目錄沒清掉")
	}
	// 遊戲寫入的檔案在第二次啟動時保留。
	marker := filepath.Join(game, "SAVE1.SAV")
	os.WriteFile(marker, []byte("s"), 0o644)
	if _, _, err := prepareGame(orig, data); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("第二次啟動覆蓋了遊戲樹")
	}
	// 竄改後失敗並指向 game/。
	os.WriteFile(filepath.Join(game, "Game.Ovr"), []byte("tampered"), 0o644)
	if _, _, err := prepareGame(orig, data); err == nil || !strings.Contains(err.Error(), game) {
		t.Fatalf("竄改未偵測：%v", err)
	}
}

func TestPrepareGameMissingOriginal(t *testing.T) {
	data := t.TempDir()
	_, _, err := prepareGame(filepath.Join(t.TempDir(), "none"), data)
	if err == nil || !strings.Contains(err.Error(), "自備") {
		t.Fatalf("%v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "game")); !os.IsNotExist(err) {
		t.Fatal("失敗時不應建立 game/")
	}
}

func TestPackageDirAppImage(t *testing.T) {
	t.Setenv("APPIMAGE", "/opt/x/BuckRogersCHT.AppImage")
	if packageDir() != "/opt/x" {
		t.Fatal(packageDir())
	}
}
