package player

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Settings struct {
	Version     int      `json:"version"`
	Scale       int      `json:"scale"`
	Fullscreen  bool     `json:"fullscreen"`
	Lang        string   `json:"lang"`
	Subtitles   bool     `json:"subtitles"`
	HD          bool     `json:"hd"`
	Muted       bool     `json:"muted"`
	PauseOnBlur bool     `json:"pause_on_blur"`
	Original    []string `json:"original"`
}

func DefaultSettings(hd bool) Settings {
	return Settings{Version: 1, Scale: 3, Lang: "zh-TW", Subtitles: true, HD: hd, PauseOnBlur: true}
}
func LoadSettings(p string, hd bool) (Settings, bool, error) {
	s := DefaultSettings(hd)
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return s, false, nil
	}
	if e != nil {
		return s, false, e
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(b, &raw); e != nil {
		return DefaultSettings(hd), false, e
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return DefaultSettings(hd), false, e
	}
	if s.Version != 1 || (s.Scale != 2 && s.Scale != 3) || (s.Lang != "zh-TW" && s.Lang != "en") {
		return DefaultSettings(hd), false, fmt.Errorf("settings.json 格式或版本不符，使用預設值")
	}
	_, scaleSet := raw["scale"]
	return s, scaleSet, nil
}
func SaveSettings(p string, s Settings) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return AtomicWrite(p, append(b, '\n'), 0600)
}
func AtomicWrite(p string, b []byte, mode os.FileMode) error {
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".settings-*.tmp")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, p)
}
func InitialDisplay(w, h int, explicitScale bool, settings Settings) Settings {
	if explicitScale {
		return settings
	}
	if w-32 >= 960 && h-96 >= 600 {
		settings.Scale = 3
	} else if w-32 >= 640 && h-96 >= 400 {
		settings.Scale = 2
	} else {
		settings.Scale = 3
		settings.Fullscreen = true
	}
	return settings
}
