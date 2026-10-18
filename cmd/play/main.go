// Command play 即時跑一支 DOS 程式：畫面、鍵盤、滑鼠與聲音（`docs/spec/441-play-frontend`）。
//
// 不認識任何特定程式。聲音經 audio/mixer：OPL、PC 喇叭，給了 -cdrom 與 -cdrom-image 再加 CD 音軌
// （`docs/spec/440-mixer-cd-audio`）。**這是給人玩的外殼**：輸入時機來自人，重跑不會得到相同結果；
// 決定性的觀測照舊用 cmd/probe。
//
//	play -prog /orig/SAN5.COM -root /orig -scratch /tmp/san5 -adlib \
//	     -cdrom D:san5.toc -cdrom-image san5.mdf
package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// tps 是主機每秒畫格數。
const tps = 60

type game struct {
	m   *machine.Machine
	d   *dos.DOS
	mix *mixer.Mixer
	sps uint64 // 每秒步數（混音器同一個值）

	ring    *mixer.Ring // nil：自動模式，沒有播放裝置
	player  *audio.Player
	samples []float32
	wav     []byte // -wav：已轉成 16 位元的樣本
	wavPath string

	input  string
	frames int // 自動模式的畫格數；0 ＝ 互動模式
	frame  int
	shot   string

	img      *ebiten.Image
	rgba     []byte
	w, h     int
	mx, my   int
	pressed  [2]bool
	caps     bool
	keysDown []ebiten.Key
	runErr   error
}

// frameTarget 回第 f 格（從 0 起）結束時的目標步數：整數除法，不累積誤差（§2.2）。
func frameTarget(f int, sps uint64) uint64 {
	return uint64(f+1) * sps / tps
}

// mouseInside 回游標是否落在 w×h 的畫面內。
func mouseInside(x, y, w, h int) bool {
	return x >= 0 && y >= 0 && x < w && y < h
}

func (g *game) Update() error {
	if g.frames == 0 {
		g.hostInput()
	}
	before := g.m.Steps
	target := frameTarget(g.frame, g.sps)
	for g.m.Steps < target && !g.d.Exited && !g.m.CPU.Halted {
		if err := g.m.Step(); err != nil {
			g.runErr = err
			break
		}
	}
	g.samples = g.mix.Render(g.samples[:0], before, g.m.Steps)
	if g.ring != nil {
		g.ring.Push(g.samples)
	}
	if g.wavPath != "" {
		g.wav = appendPCM16(g.wav, g.samples)
	}
	g.frame++
	if g.runErr != nil || g.d.Exited || g.m.CPU.Halted || (g.frames > 0 && g.frame >= g.frames) {
		return ebiten.Termination
	}
	return nil
}

// hostInput 把這一格的滑鼠與鍵盤送進 DOS（§2.4）。
func (g *game) hostInput() {
	x, y := ebiten.CursorPosition() // 已是 Layout 的畫面座標
	if mouseInside(x, y, g.w, g.h) && (x != g.mx || y != g.my) {
		g.d.MoveMouse(x, y)
		g.mx, g.my = x, y
	}
	for i, b := range []ebiten.MouseButton{ebiten.MouseButtonLeft, ebiten.MouseButtonRight} {
		if inpututil.IsMouseButtonJustPressed(b) && !g.pressed[i] && mouseInside(x, y, g.w, g.h) {
			g.d.PressMouse(i)
			g.pressed[i] = true
		}
		if inpututil.IsMouseButtonJustReleased(b) && g.pressed[i] {
			g.d.ReleaseMouse(i)
			g.pressed[i] = false
		}
	}
	mod := modifiers{
		shift: ebiten.IsKeyPressed(ebiten.KeyShift),
		ctrl:  ebiten.IsKeyPressed(ebiten.KeyControl),
		alt:   ebiten.IsKeyPressed(ebiten.KeyAlt),
	}
	g.keysDown = inpututil.AppendJustPressedKeys(g.keysDown[:0])
	for _, k := range g.keysDown {
		if k == ebiten.KeyCapsLock {
			g.caps = !g.caps
			continue
		}
		mod.caps = g.caps
		key, ok := hostKey(keyCode(k), mod)
		if !ok {
			continue
		}
		if g.input == "bios" || g.input == "both" {
			g.d.PushKey(key)
		}
		if g.input == "hw" || g.input == "both" {
			g.m.QueueScan(key.Scan)
		}
	}
	if g.input == "hw" || g.input == "both" {
		g.keysDown = inpututil.AppendJustReleasedKeys(g.keysDown[:0])
		for _, k := range g.keysDown {
			if scan, ok := scanByCode[keyCode(k)]; ok {
				g.m.QueueScan(scan | 0x80)
			}
		}
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	w, h, rgb := screenRGB(g.m)
	if g.img == nil || w != g.w || h != g.h {
		g.img = ebiten.NewImage(w, h)
		g.rgba = make([]byte, w*h*4)
		g.w, g.h = w, h
	}
	for i := 0; i < w*h; i++ {
		g.rgba[i*4], g.rgba[i*4+1], g.rgba[i*4+2], g.rgba[i*4+3] = rgb[i*3], rgb[i*3+1], rgb[i*3+2], 255
	}
	g.img.WritePixels(g.rgba)
	screen.DrawImage(g.img, nil)
}

func (g *game) Layout(int, int) (int, int) {
	if g.w == 0 {
		return 640, 480
	}
	return g.w, g.h
}

// appendPCM16 把 [−1, 1] 的樣本 ×32768 四捨五入再夾，附加成 16 位元小端（同 probe -dump-mix-wav）。
func appendPCM16(dst []byte, s []float32) []byte {
	for _, v := range s {
		x := math.Max(-32768, math.Min(32767, math.Round(float64(v)*32768)))
		dst = binary.LittleEndian.AppendUint16(dst, uint16(int16(x)))
	}
	return dst
}

// wavFile 包上 48 kHz 立體聲 16 位元的 RIFF 檔頭。
func wavFile(pcm []byte) []byte {
	b := make([]byte, 0, 44+len(pcm))
	b = append(b, "RIFF"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(36+len(pcm)))
	b = append(b, "WAVEfmt "...)
	b = binary.LittleEndian.AppendUint32(b, 16)
	b = binary.LittleEndian.AppendUint16(b, 1) // PCM
	b = binary.LittleEndian.AppendUint16(b, 2)
	b = binary.LittleEndian.AppendUint32(b, mixer.SampleRate)
	b = binary.LittleEndian.AppendUint32(b, mixer.SampleRate*4)
	b = binary.LittleEndian.AppendUint16(b, 4)
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(pcm)))
	return append(b, pcm...)
}

func main() {
	prog := flag.String("prog", "", "要跑的 .COM 或 .EXE")
	root := flag.String("root", "", "DOS 看得到的目錄（唯讀）")
	dir := flag.String("dir", "", "DOS 的目前目錄（root 底下的子目錄）")
	scratch := flag.String("scratch", "", "可寫的暫存層（`docs/spec/009-scratch-writes`）；三國志V 會寫 SWAPFILE.S5，要給")
	args := flag.String("args", "", "命令列參數，寫進 PSP 的 80h")
	cpuModel := flag.String("cpu", "386", "CPU 機型：8086、186 或 386")
	programPath := flag.String("program-path", "", "環境區塊裡的程式全路徑（預設 C:\\ 加執行檔名）")
	adlib := flag.Bool("adlib", false, "讓 AdLib（OPL2，埠 388h）偵測存在")
	cdrom := flag.String("cdrom", "", "掛一台 MSCDEX 光碟機：`D:<曲目表檔>`（`docs/spec/434`、`438`）")
	cdromImage := flag.String("cdrom-image", "", "光碟映像（每扇區 2352 bytes 的 raw）；配 -cdrom 才有 CD 音軌")
	input := flag.String("input", "bios", "按鍵送哪條路：bios（int 16h 佇列）、hw（IRQ1 掃描碼）、both")
	clock := flag.Int("clock", 100, "速度百分比：每秒步數 = machine.StepsPerSecond() × clock ÷ 100")
	scale := flag.Int("scale", 1, "視窗起始倍率（以 640×480 為準）")
	frames := flag.Int("frames", 0, "自動模式：跑幾格後結束，不讀輸入、不開播放裝置（0 ＝ 互動模式）")
	wavPath := flag.String("wav", "", "把混音樣本寫成 48 kHz 16 位元立體聲 WAV")
	shot := flag.String("shot", "", "結束時把最後一格存成 PNG")
	flag.Parse()
	if *prog == "" || *root == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *input != "bios" && *input != "hw" && *input != "both" {
		die(fmt.Errorf("-input 只接受 bios、hw、both：%q", *input))
	}
	if *clock <= 0 || *scale <= 0 {
		die(errors.New("-clock 與 -scale 要大於 0"))
	}

	m, d, err := newMachine(*prog, *root, *dir, *scratch, *args, *cpuModel, *programPath, *adlib, *cdrom)
	if err != nil {
		die(err)
	}
	sps := uint64(machine.StepsPerSecond()) * uint64(*clock) / 100
	mix := mixer.New(sps)
	m.ObserveOPLWrites(mix.OPLWrite)
	m.ObserveSpeaker(mix.SpeakerSample)
	m.ObservePITChannel2(mix.PITChannel2)
	if *cdromImage != "" {
		if d.CD == nil {
			die(errors.New("-cdrom-image 要配 -cdrom"))
		}
		img, err := os.Open(*cdromImage)
		if err != nil {
			die(err)
		}
		defer img.Close()
		mix.AttachCD(d.CD, img)
	}

	g := &game{m: m, d: d, mix: mix, sps: sps, input: *input, frames: *frames, shot: *shot, wavPath: *wavPath}
	if *frames == 0 {
		g.ring = mixer.NewRing(200)
		player, err := audio.NewContext(mixer.SampleRate).NewPlayerF32(g.ring)
		if err != nil {
			die(err)
		}
		player.SetBufferSize(50 * time.Millisecond)
		player.Play()
		g.player = player
	}
	ebiten.SetWindowTitle("dosgolem play — " + filepath.Base(*prog))
	ebiten.SetWindowSize(640**scale, 480**scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(tps)
	started := time.Now()
	if err := ebiten.RunGame(g); err != nil {
		die(err)
	}
	wall := time.Since(started).Seconds()
	report := fmt.Sprintf("play: frames=%d steps=%d wall=%.1fs fps=%.1f cd_errors=%d clipped=%d",
		g.frame, m.Steps, wall, float64(g.frame)/wall, mix.CDErrors(), mix.Clipped())
	if g.ring != nil {
		u, dr := g.ring.Stats()
		report += fmt.Sprintf(" audio_underruns=%d audio_dropped_frames=%d", u, dr)
	}
	if d.Exited {
		report += fmt.Sprintf(" exit=%d", d.ExitCode)
	}
	fmt.Fprintln(os.Stderr, report)
	if g.wavPath != "" {
		if err := os.WriteFile(g.wavPath, wavFile(g.wav), 0o644); err != nil {
			die(err)
		}
	}
	if g.shot != "" {
		if err := writePNG(g.shot, m); err != nil {
			die(err)
		}
	}
	if g.runErr != nil {
		die(g.runErr)
	}
}

// newMachine 建機器與 DOS（同 cmd/webplay，加上 probe 的 -program-path、-adlib、-cdrom）。
func newMachine(prog, root, dir, scratch, args, model, programPath string, adlib bool, cdrom string) (*machine.Machine, *dos.DOS, error) {
	data, err := os.ReadFile(prog)
	if err != nil {
		return nil, nil, err
	}
	m := machine.New()
	m.ProgramPath = programPath
	if m.ProgramPath == "" {
		m.ProgramPath = `C:\` + strings.ToUpper(filepath.Base(prog))
	}
	if adlib {
		m.SetAdLib(true)
	}
	if len(data) >= 2 && data[0] == 'M' && data[1] == 'Z' {
		err = m.LoadEXE(data)
	} else {
		err = m.LoadCOM(data)
	}
	if err != nil {
		return nil, nil, err
	}
	d := dos.New(m, root)
	d.Dir = dir
	d.Scratch = scratch
	d.Install()
	switch model {
	case "386":
	case "186":
		m.CPU.Model = cpu.Model80186
		m.CPU.SetFlags(m.CPU.Flags)
	case "8086":
		m.CPU.Model = cpu.Model8086
		m.CPU.SetFlags(m.CPU.Flags)
	default:
		return nil, nil, fmt.Errorf("-cpu 只接受 8086、186 或 386：%q", model)
	}
	if args != "" {
		if len(args) > 125 {
			args = args[:125]
		}
		psp := uint32(machine.PSPSeg) * 16
		line := " " + args + "\r"
		m.Write8(psp+0x80, uint8(len(line)-1))
		m.WriteBytes(psp+0x81, []byte(line))
	}
	if cdrom != "" {
		letter, path, ok := strings.Cut(cdrom, ":")
		if !ok || len(letter) != 1 {
			return nil, nil, fmt.Errorf("-cdrom 要寫成 `D:<曲目表檔>`：%q", cdrom)
		}
		toc, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		cd, err := dos.ParseCDTOC(strings.ToUpper(letter)[0]-'A', string(toc))
		if err != nil {
			return nil, nil, err
		}
		d.CD = cd
	}
	return m, d, nil
}

func writePNG(path string, m *machine.Machine) error {
	w, h, rgb := screenRGB(m)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = rgb[i*3], rgb[i*3+1], rgb[i*3+2], 255
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "play:", err)
	os.Exit(1)
}
