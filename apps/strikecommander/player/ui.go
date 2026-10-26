package player

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

var UIEnglish = map[string]string{
	"title":                    "Strike Commander",
	"help.title":               "Help and settings",
	"help.keys":                "2/3 Scale  L Language  T Subtitles  H HD",
	"help.keys2":               "F Fullscreen  M Mute  P Screenshot  Q Quit",
	"help.close":               "F11 or Esc: Return to game",
	"help.menu":                "Intro menu: use the mouse on the upper button",
	"help.keyboard":            "The game uses US physical keyboard positions",
	"help.mac":                 "macOS: Fn+function key; Cmd+/ opens help",
	"help.save":                "Saves: ",
	"help.save_warning":        "Deleting the save folder removes all progress",
	"quit.confirm":             "Quit? Progress not saved in the game will be lost",
	"quit.keys":                "Y: Quit; any other key: Cancel",
	"source.title":             "Provide the original Strike Commander",
	"source.guide":             "Original game data is not included. Drop a folder or disc image.",
	"source.paths":             "Use original/ next to the app, the user original/ folder, or -original.",
	"source.examples":          "Linux: ~/.local/share/strikecommander-cht/original",
	"source.windows":           "Windows: %LOCALAPPDATA%\\StrikeCommanderCHT\\original",
	"source.mac":               "macOS: ~/Library/Application Support/StrikeCommanderCHT/original",
	"source.progress":          "Checking original game data...",
	"source.dropped_duplicate": "Duplicate dropped basenames; drop them separately",
	"error.title":              "Unable to start",
	"error.close":              "You can close this window",
	"audio.none":               "No audio output",
	"label.scale":              "Scale",
	"label.language":           "Language",
	"label.subtitles":          "Subtitles",
	"label.hd":                 "HD",
	"label.muted":              "Muted",
	"label.rate":               "Realtime ratio",
	"label.on":                 "On",
	"label.off":                "Off",
}

type UIText struct {
	Strings map[string]string
	Reason  string
}

func LoadUIText(path string, hasRune func(rune) bool) UIText {
	u := UIText{Strings: map[string]string{}}
	f, e := os.Open(path)
	if e != nil {
		u.Reason = e.Error()
		return u
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = 3
	rows, e := r.ReadAll()
	if e != nil || len(rows) == 0 || strings.Join(rows[0], "/") != "key/translation/source" {
		u.Reason = "frontend catalog格式錯誤"
		return u
	}
	for _, row := range rows[1:] {
		if _, ok := u.Strings[row[0]]; ok {
			u.Reason = "frontend catalog重複key：" + row[0]
			return u
		}
		u.Strings[row[0]] = row[1]
		for _, c := range row[1] {
			if hasRune != nil && !hasRune(c) {
				u.Reason = fmt.Sprintf("frontend缺字 U+%04X", c)
				return u
			}
		}
	}
	for k := range UIEnglish {
		if u.Strings[k] == "" {
			u.Reason = "frontend catalog缺key：" + k
			return u
		}
	}
	return u
}
func (u UIText) Text(key string) string {
	if u.Reason == "" && u.Strings[key] != "" {
		return u.Strings[key]
	}
	if s := UIEnglish[key]; s != "" {
		return s
	}
	return key
}
