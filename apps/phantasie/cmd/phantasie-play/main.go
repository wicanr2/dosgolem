// Command phantasie-play 是《幽靈戰士》繁體中文化的互動前端（docs/spec/005 §3）：
// 以 dosgolem 執行原版，疊字層覆繪成目標語言，只用鍵盤操作。
//
//	phantasie-play -root <原版目錄> -text text -font <字型目錄> -lang zh-TW
//
// F11 全螢幕、F12 依序切換語言（zh-TW、zh-CN、en、ja、ko 中已啟用者）。這兩個鍵不送給原版。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/dosgolem/apps/phantasie"
)

const maxQueued = 16

// namedKeys 把主機鍵對應到 oracle.SendKeys 的鍵名（docs/spec/005 §3）。
var namedKeys = map[ebiten.Key]string{
	ebiten.KeyEnter: "Return", ebiten.KeyNumpadEnter: "Return", ebiten.KeyEscape: "Esc",
	ebiten.KeyBackspace: "Backspace", ebiten.KeyTab: "Tab", ebiten.KeySpace: "Space",
	ebiten.KeyArrowUp: "Up", ebiten.KeyArrowDown: "Down", ebiten.KeyArrowLeft: "Left", ebiten.KeyArrowRight: "Right",
	ebiten.KeyF1: "F1", ebiten.KeyF2: "F2", ebiten.KeyF3: "F3", ebiten.KeyF4: "F4", ebiten.KeyF5: "F5",
	ebiten.KeyF6: "F6", ebiten.KeyF7: "F7", ebiten.KeyF8: "F8", ebiten.KeyF9: "F9", ebiten.KeyF10: "F10",
}

type game struct {
	s        *phantasie.Session
	stepsPer uint64
	rgba     []uint8
	img      *ebiten.Image
	baseName string
	banner   time.Time
	err      error
	full     bool
}

func (g *game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		g.full = !g.full
		ebiten.SetFullscreen(g.full)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		name, err := g.s.NextDisplay()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			ebiten.SetWindowTitle(g.baseName + "（" + name + "）")
			g.banner = time.Now().Add(time.Second)
		}
	}
	if !g.banner.IsZero() && time.Now().After(g.banner) {
		ebiten.SetWindowTitle(g.baseName)
		g.banner = time.Time{}
	}
	for k, name := range namedKeys {
		if inpututil.IsKeyJustPressed(k) && g.s.Gate.Pending() < maxQueued {
			g.s.Gate.Press(0, name)
		}
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 0x20 && r < 0x7F && r != ' ' && g.s.Gate.Pending() < maxQueued {
			g.s.Gate.Press(0, string(r))
		}
	}
	if g.err == nil {
		if err := g.s.O.Run(g.stepsPer); err != nil {
			g.err = err
			fmt.Fprintln(os.Stderr, "原版結束或出錯：", err)
		}
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	idx, rgb := g.s.Frame()
	phantasie.ComposeInto(g.rgba, g.s.Ov, idx, rgb, 2, nil)
	g.img.WritePixels(g.rgba)
	screen.Fill(color.Black)
	screen.DrawImage(g.img, nil)
}

func (g *game) Layout(int, int) (int, int) { return 640, 400 }

func main() {
	root := flag.String("root", "", "原版目錄（唯讀；必填）")
	bat := flag.String("bat", "", "啟動批次檔名；空字串＝目錄內唯一的 .BAT")
	state := flag.String("state", "", "可寫狀態目錄（存檔放這裡）；預設 $XDG_DATA_HOME/phantasie-cht")
	lang := flag.String("lang", "zh-TW", "初始顯示語言：zh-TW、zh-CN、en、ja、ko")
	textDir := flag.String("text", "text", "譯文目錄")
	fontDir := flag.String("font", "", "字型目錄（<lang>.golemfnt；必填）")
	zoom := flag.Int("zoom", 1, "視窗倍率（1 或 2；視窗為 640x400 的倍數）")
	ips := flag.Uint64("ips", 6_000_000, "每秒執行的原版指令數")
	flag.Parse()
	if *root == "" || *fontDir == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *zoom != 1 && *zoom != 2 {
		fmt.Fprintln(os.Stderr, "-zoom 只能是 1 或 2")
		os.Exit(2)
	}
	dir := *state
	if dir == "" {
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			base = filepath.Join(home, ".local", "share")
		}
		dir = filepath.Join(base, "phantasie-cht")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	langs := []string{*lang}
	for _, n := range []string{"zh-TW", "zh-CN", "ja", "ko"} {
		if n != *lang {
			langs = append(langs, n)
		}
	}
	s, err := phantasie.StartSession(phantasie.SessionOptions{
		Root: *root, Bat: *bat, TextDir: *textDir, FontDir: *fontDir, Langs: langs, Scratch: dir,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer s.Close()
	for _, f := range s.Failed {
		fmt.Fprintln(os.Stderr, "語言停用：", f)
	}
	g := &game{s: s, stepsPer: *ips / 60, rgba: make([]uint8, 640*400*4), baseName: "幽靈戰士（Phantasie）繁體中文化"}
	g.img = ebiten.NewImage(640, 400)
	ebiten.SetWindowTitle(g.baseName)
	ebiten.SetWindowSize(640**zoom, 400**zoom)
	ebiten.SetTPS(60)
	if err := ebiten.RunGame(g); err != nil && !strings.Contains(err.Error(), "game ended") {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
