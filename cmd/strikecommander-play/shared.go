package main

import (
	"flag"
	"github.com/wicanr2/dosgolem/session"
	"os"
	"path/filepath"
	"strings"
)

type originals []string

func (o *originals) String() string     { return strings.Join(*o, ",") }
func (o *originals) Set(s string) error { *o = append(*o, s); return nil }

type options struct {
	rebase                                        originals
	script, record, replay                        string
	hd, subtitles, fullscreen, muted, pauseOnBlur bool
	explicit                                      map[string]bool
	original                                      originals
	reimport, verify, headless                    bool
	uiSmoke                                       int
	scale                                         int
	lang                                          string
	wav, state                                    string
	steps                                         uint64
}

func parse() options {
	var o options
	flag.Var(&o.original, "original", "原版目錄或光碟映像，可重複")
	flag.BoolVar(&o.reimport, "reimport", false, "重建原版快取")
	flag.BoolVar(&o.verify, "verify", false, "完整核對快取")
	flag.IntVar(&o.scale, "scale", 0, "輸出倍率2或3")
	flag.StringVar(&o.lang, "lang", "", "zh-TW或en")
	flag.BoolVar(&o.headless, "headless", false, "驗收模式，不開視窗或播放裝置")
	flag.StringVar(&o.wav, "wav", "", "headless混音WAV輸出")
	flag.StringVar(&o.state, "state", "", "headless初始state")
	flag.Uint64Var(&o.steps, "steps", 0, "headless固定步數")
	flag.IntVar(&o.uiSmoke, "ui-smoke", 0, "只驗引導頁視窗啟動，指定Update數後結束，不開遊戲")
	flag.StringVar(&o.script, "script", "", "headless原版步數腳本")
	flag.StringVar(&o.record, "record", "", "錄製推進回合JSONL")
	flag.StringVar(&o.replay, "replay", "", "headless重播已錄推進回合")
	flag.BoolVar(&o.hd, "hd", true, "HD開關")
	flag.BoolVar(&o.subtitles, "subtitles", true, "語音字幕")
	flag.BoolVar(&o.fullscreen, "fullscreen", false, "全螢幕")
	flag.BoolVar(&o.muted, "muted", false, "靜音")
	flag.BoolVar(&o.pauseOnBlur, "pause-on-blur", true, "失焦暫停")
	flag.Var(&o.rebase, "rebase-from", "headless old scratch prefix; repeatable")
	flag.Parse()
	o.explicit = map[string]bool{}
	flag.Visit(func(f *flag.Flag) { o.explicit[f.Name] = true })
	return o
}
func resource(rel string) string {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	wd, _ := os.Getwd()
	for _, p := range []string{filepath.Join(dir, rel), filepath.Join(dir, "..", "Resources", rel), filepath.Join(wd, rel)} {
		if st, e := os.Stat(p); e == nil && !st.IsDir() {
			return p
		}
	}
	return filepath.Join(dir, rel)
}
func resourceDir(rel string) string {
	return filepath.Dir(resource(filepath.Join(rel, ".resource-marker")))
}
func bundleDir() string {
	if a := os.Getenv("APPIMAGE"); a != "" {
		return filepath.Dir(a)
	}
	p, _ := os.Executable()
	if q, e := filepath.EvalSymlinks(p); e == nil {
		p = q
	}
	dir := filepath.Dir(p)
	if i := strings.Index(dir, ".app"+string(filepath.Separator)+"Contents"); i >= 0 {
		return filepath.Dir(dir[:i+4])
	}
	return dir
}

// runtimeGame is implemented by the actual app bridge, never a renderer stub.
// CopyPrepared fills one independently owned frontend frame after Prepare.
type runtimeGame interface {
	Owner() *session.Owner
	MenuState() (period, visible bool)
	NotifyStopped() error
	Prepare(scale int, lang string, hd, subtitles bool, version uint64, dst []byte) error
	Close() error
}
