package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	p "github.com/wicanr2/dosgolem/apps/phantasie"
	"github.com/wicanr2/dosgolem/oracle"
)

type captureOptions struct {
	Route, Dir, Bundle, Root, Text, Font, Art string
	Every, Hold                               int
	IPS                                       uint64
	Showcase                                  bool
}
type capturePoint struct {
	Name                                 string `json:"name"`
	Frame                                int    `json:"frame"`
	Steps, Reads                         uint64
	Memory, VRAM, Content, Visible, RGBA string
	Language, Theme                      string
}
type captureFrame struct {
	Frame                    int    `json:"frame"`
	File                     string `json:"file"`
	SHA256                   string `json:"sha256"`
	Section, Language, Theme string
}
type captureSegment struct {
	Name            string `json:"name"`
	Start, End      int
	Language, Theme string
}
type captureReport struct {
	Schema                     int               `json:"schema"`
	FPS                        int               `json:"virtual_fps"`
	IPS                        uint64            `json:"ips"`
	RouteSHA256                string            `json:"route_sha256"`
	Scope                      string            `json:"scope"`
	Inputs                     map[string]string `json:"inputs"`
	Bundle                     map[string]string `json:"bundle,omitempty"`
	InitialMemory, InitialVRAM string
	StateInitiallyEmpty        bool
	TotalFrames                int              `json:"total_frames"`
	Points                     []capturePoint   `json:"points"`
	Frames                     []captureFrame   `json:"frames"`
	Segments                   []captureSegment `json:"segments"`
}

func captureHash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func captureNoLinks(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for q := abs; ; q = filepath.Dir(q) {
		info, err := os.Lstat(q)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("擷取路徑含符號連結")
		}
		if q == filepath.Dir(q) {
			break
		}
	}
	return nil
}
func captureNewDir(path string, inputs ...string) error {
	if path == "" {
		return fmt.Errorf("擷取須明示全新輸出目錄")
	}
	if err := captureNoLinks(path); err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(abs); !os.IsNotExist(err) {
		return fmt.Errorf("擷取目錄已存在或不可讀")
	}
	if info, err := os.Stat(filepath.Dir(abs)); err != nil || !info.IsDir() {
		return fmt.Errorf("擷取父目錄不存在")
	}
	for _, input := range inputs {
		if input == "" {
			continue
		}
		src, err := filepath.Abs(input)
		if err != nil {
			return err
		}
		if abs == src || strings.HasPrefix(abs, src+string(os.PathSeparator)) || strings.HasPrefix(src, abs+string(os.PathSeparator)) {
			return fmt.Errorf("擷取輸出與輸入重疊")
		}
	}
	return os.Mkdir(abs, 0755)
}
func captureFileHash(path string) (string, error) {
	if err := captureNoLinks(path); err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 64*1024*1024 {
		return "", fmt.Errorf("擷取輸入型態或大小不符")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
func captureInputs(o captureOptions) (map[string]string, error) {
	hashes := map[string]string{}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	hashes["backend"], err = captureFileHash(exe)
	if err != nil {
		return nil, err
	}
	for label, dir := range map[string]string{"root": o.Root, "text": o.Text, "font": o.Font, "art": o.Art} {
		if dir == "" {
			continue
		}
		if err := captureNoLinks(dir); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			hashes[label+"/"+name], err = captureFileHash(filepath.Join(dir, name))
			if err != nil {
				return nil, err
			}
		}
	}
	return hashes, nil
}
func captureBundle(path string, o captureOptions, inputs map[string]string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	if err := captureNoLinks(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > 1024*1024 {
		return nil, fmt.Errorf("錄影清冊過大")
	}
	var b struct {
		Schema              int    `json:"schema"`
		Version             string `json:"version"`
		Engine              string `json:"engine_commit"`
		Backend, Text, Font string
		Art                 string `json:"art,omitempty"`
		LocalOriginal       string `json:"local_original,omitempty"`
		Assets              []struct {
			Name   string
			Bytes  int64
			SHA256 string
		}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&b); err != nil {
		return nil, err
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("錄影清冊有額外JSON")
	}
	if b.Schema != 1 || !regexp.MustCompile(`^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$`).MatchString(b.Version) || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(b.Engine) {
		return nil, fmt.Errorf("錄影清冊版本不符")
	}
	if _, err := time.Parse("20060102", b.Version[len(b.Version)-8:]); err != nil {
		return nil, fmt.Errorf("錄影清冊日期無效")
	}
	base := filepath.Dir(path)
	// macOS bundle lives in Resources; its paths are relative to Contents.
	if strings.HasPrefix(b.Backend, "MacOS/") {
		base = filepath.Dir(base)
	}
	safe := func(name string) bool {
		return name != "" && name != "." && name != ".." && !filepath.IsAbs(name) && !strings.ContainsAny(name, "\\:\x00") && filepath.ToSlash(filepath.Clean(name)) == name && !strings.HasPrefix(name, "../")
	}
	if !safe(b.Backend) || (b.LocalOriginal != "" && !safe(b.LocalOriginal)) {
		return nil, fmt.Errorf("錄影清冊程式或原版路徑無效")
	}
	records := map[string]bool{}
	seen := map[string]bool{}
	for _, a := range b.Assets {
		if !safe(a.Name) || seen[strings.ToLower(a.Name)] || a.Bytes < 0 || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a.SHA256) {
			return nil, fmt.Errorf("錄影資產清冊無效")
		}
		seen[strings.ToLower(a.Name)] = true
		records[a.Name] = true
		file := filepath.Join(base, filepath.FromSlash(a.Name))
		h, err := captureFileHash(file)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		if info.Size() != a.Bytes || h != a.SHA256 {
			return nil, fmt.Errorf("錄影資產內容不符")
		}
	}
	needed := []string{b.Backend, b.Text + "/protected.tsv"}
	for _, lang := range []string{"zh-TW", "zh-CN", "ja", "ko"} {
		needed = append(needed, b.Font+"/"+lang+".golemfnt")
		for _, family := range []string{"ui", "prose", "manual"} {
			needed = append(needed, b.Text+"/"+family+"."+lang+".tsv")
		}
	}
	if b.Art != "" {
		if b.LocalOriginal == "" {
			return nil, fmt.Errorf("錄影HD只能本機包")
		}
		if !safe(b.Art) {
			return nil, fmt.Errorf("錄影 HD 路徑無效")
		}
		profile, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(b.Art), "profile.json"))
		if err != nil || len(profile) > 65536 {
			return nil, fmt.Errorf("錄影 HD profile 不可讀")
		}
		names, err := p.ArtAssetNames(profile)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			needed = append(needed, b.Art+"/"+name)
		}
	}
	for _, name := range needed {
		if !records[name] {
			return nil, fmt.Errorf("錄影缺必要資產")
		}
	}
	if b.LocalOriginal == "" {
		return nil, fmt.Errorf("正式錄影須使用本機完整版")
	}
	for label, dir := range map[string]string{"text": b.Text, "font": b.Font, "art": b.Art, "root": b.LocalOriginal} {
		actual := map[string]string{"text": o.Text, "font": o.Font, "art": o.Art, "root": o.Root}[label]
		if dir == "" && actual == "" {
			continue
		}
		if !safe(dir) {
			return nil, fmt.Errorf("錄影資料路徑無效")
		}
		want, err := filepath.Abs(filepath.Join(base, filepath.FromSlash(dir)))
		if err != nil {
			return nil, err
		}
		have, err := filepath.Abs(actual)
		if err != nil {
			return nil, err
		}
		if want != have {
			return nil, fmt.Errorf("錄影不是包內的資料路徑")
		}
	}
	hash, err := captureFileHash(filepath.Join(base, filepath.FromSlash(b.Backend)))
	if err != nil {
		return nil, err
	}
	if hash != inputs["backend"] {
		return nil, fmt.Errorf("錄影後端不是此包的程式")
	}
	return map[string]string{"version": b.Version, "engine_commit": b.Engine, "sha256": captureHash(data)}, nil
}

func recordGameplay(s *p.Session, theme string, art *p.TownArt, o captureOptions) (result error) {
	// The shared parser and input gate can include the offending key in their
	// errors. Private routes must never expose that text through diagnostics.
	stage := "路線解析"
	defer func() {
		if result != nil {
			result = fmt.Errorf("錄影未完成：%s失敗", stage)
		}
	}()
	raw, err := os.ReadFile(o.Route)
	if err != nil {
		return err
	}
	if len(raw) > 1024*1024 {
		return fmt.Errorf("錄影路線過大")
	}
	route, err := p.ParseRoute(string(raw))
	if err != nil {
		return err
	}
	stage = "素材核對"
	inputs, err := captureInputs(o)
	if err != nil {
		return err
	}
	bundle, err := captureBundle(o.Bundle, o, inputs)
	if err != nil {
		return err
	}
	report := captureReport{Schema: 1, FPS: 60, IPS: o.IPS, RouteSHA256: captureHash(raw), Scope: "research", Inputs: inputs, Bundle: bundle, StateInitiallyEmpty: true}
	if bundle != nil {
		report.Scope = "local package gameplay capture"
	}
	stage = "路線執行或停點驗證"
	memory := func() string { return captureHash(s.O.Bytes(oracle.Far(0, 0), 1<<20)) }
	report.InitialMemory, report.InitialVRAM = memory(), fmt.Sprintf("%016x", p.VramHash(s.O))
	rgba := make([]byte, 640*400*4)
	frame, lastWritten := 0, -1
	section := "transition"
	writeFrame := func(force bool) error {
		if frame == lastWritten || (!force && frame%o.Every != 0) {
			return nil
		}
		var b bytes.Buffer
		if err := png.Encode(&b, &image.RGBA{Pix: rgba, Stride: 640 * 4, Rect: image.Rect(0, 0, 640, 400)}); err != nil {
			return err
		}
		name := fmt.Sprintf("frame-%06d.png", frame)
		if o.Dir != "" {
			f, err := os.OpenFile(filepath.Join(o.Dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				return err
			}
			_, err = f.Write(b.Bytes())
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		report.Frames = append(report.Frames, captureFrame{frame, name, captureHash(b.Bytes()), section, s.Ov.Display(), theme})
		lastWritten = frame
		return nil
	}
	render := func() error {
		if frame >= 120000 {
			return fmt.Errorf("錄影超過有界格數")
		}
		idx, rgb := s.Frame()
		p.ComposePresentationInto(rgba, s.Ov, idx, rgb, theme, art)
		err := writeFrame(false)
		frame++
		return err
	}
	hold := func(name string, n int) error {
		section = name
		start := frame
		for i := 0; i < n; i++ {
			if err := render(); err != nil {
				return err
			}
		}
		report.Segments = append(report.Segments, captureSegment{name, start, frame, s.Ov.Display(), theme})
		return nil
	}
	run := func(cond oracle.Cond, limit uint64) error {
		section = "transition"
		for !cond.Ready(s.O) {
			budget := o.IPS / 60
			if limit != 0 && limit-s.O.Steps() < budget {
				budget = limit - s.O.Steps()
			}
			err := s.O.RunUntil(cond, oracle.Budget(budget))
			if e := render(); e != nil {
				return e
			}
			if err != nil {
				var wait *oracle.InputWaitError
				var be *oracle.BudgetError
				if errors.As(err, &wait) && cond.Ready(s.O) {
					return nil
				}
				if !errors.As(err, &be) {
					return err
				}
			}
			if s.O.Steps() > 4_000_000_000 || frame > 120000 {
				return fmt.Errorf("錄影超過有界步數或格數")
			}
		}
		return nil
	}
	points := map[string]capturePoint{}
	showcased := false
	var queuedKeys int
	for _, st := range route {
		switch st.Kind {
		case p.RouteKey:
			queuedKeys++
			if st.Wait > 0 {
				s.Gate.PressAfterReads(st.Wait, st.Key)
			} else {
				s.Gate.Press(0, st.Key)
			}
			continue
		case p.RouteLang:
			if err := s.Ov.SetDisplay(st.Name); err != nil {
				return err
			}
			continue
		case p.RouteAssert:
			a, b := points[st.Name], points[st.Other]
			if a.Visible != b.Visible || (st.Screen && (a.VRAM != b.VRAM || a.Content != b.Content)) {
				return fmt.Errorf("錄影路線斷言失敗")
			}
			continue
		}
		cond := oracle.NewCond("record-check", func(*oracle.Oracle) bool { return s.Gate.Pending() == 0 && s.Gate.Reads > s.Gate.LastSent })
		if st.Kind == p.RouteSnap {
			cond = oracle.NewCond("record-snap", func(*oracle.Oracle) bool { return s.Gate.Pending() == 0 })
		}
		if err := run(cond, 0); err != nil {
			return err
		}
		if st.Kind == p.RouteSnap {
			target := s.O.Steps() + st.Steps
			if err := run(oracle.NewCond("snap-target", func(*oracle.Oracle) bool { return s.O.Steps() >= target }), target); err != nil {
				return err
			}
		}
		idx, rgb := s.Frame()
		p.ComposePresentationInto(rgba, s.Ov, idx, rgb, theme, art)
		if s.Hk.Failed() || !s.Hk.Armed() || s.Gate.Err != "" || s.Gate.Gated != queuedKeys || s.Ov.C.Get("protected") != 0 || s.Ov.C.Get("unpaired") != 0 {
			return fmt.Errorf("錄影停點完整性失敗")
		}
		if s.Ov.Display() != "en" {
			shown := map[string]bool{}
			for _, k := range s.Ov.KeysShown() {
				shown[k] = true
			}
			for _, k := range st.Expect {
				if !shown[k] {
					return fmt.Errorf("錄影停點缺期望文字")
				}
			}
			known := map[string]bool{}
			for _, k := range st.Known {
				known[k] = true
			}
			for _, k := range append(s.Ov.C.KeySet("untranslated"), s.Ov.C.KeySet("untranslated_args")...) {
				if !known[k] {
					return fmt.Errorf("錄影停點有未核對文字")
				}
			}
			if !s.Hk.Busy() {
				ex, strict := s.Ov.AuditEvents(idx)
				if ex != 0 || strict != 0 || s.Ov.AuditStale(idx) != 0 {
					return fmt.Errorf("錄影停點覆繪稽核失敗")
				}
			}
		}
		point := capturePoint{st.Name, frame, s.O.Steps(), s.Gate.Reads, memory(), fmt.Sprintf("%016x", p.VramHash(s.O)), fmt.Sprintf("%016x", s.Ov.ContentHash()), fmt.Sprintf("%016x", s.Ov.VisibleHash()), captureHash(rgba), s.Ov.Display(), theme}
		report.Points = append(report.Points, point)
		points[st.Name] = point
		if err := hold(st.Name, o.Hold); err != nil {
			return err
		}
		if o.Showcase && !showcased && st.Name == "town" {
			savedTheme, savedLang := theme, s.Ov.Display()
			beforeMem, beforeReads := memory(), s.Gate.Reads
			for _, name := range []string{"original", "amber", "hd"} {
				if name == "hd" && art == nil {
					continue
				}
				theme = name
				if err := hold("show-theme-"+name, 90); err != nil {
					return err
				}
			}
			for range s.DisplayCycle() {
				name, err := s.NextDisplay()
				if err != nil {
					return err
				}
				if err := hold("show-language-"+name, 60); err != nil {
					return err
				}
			}
			theme = savedTheme
			if err := s.Ov.SetDisplay(savedLang); err != nil {
				return err
			}
			if memory() != beforeMem || s.Gate.Reads != beforeReads {
				return fmt.Errorf("展示改變原版")
			}
			showcased = true
		}
	}
	if s.Gate.Pending() != 0 || s.Gate.Gated != queuedKeys {
		return fmt.Errorf("錄影路線末尾仍有未送出按鍵")
	}
	if frame == 0 {
		return fmt.Errorf("錄影沒有畫格")
	}
	// The final canonical composition is an explicit virtual frame after all
	// display restorations, so sparse output cannot relabel an earlier image.
	if err := hold("final", 1); err != nil {
		return err
	}
	frame--
	if err := writeFrame(true); err != nil {
		return err
	}
	frame++
	report.TotalFrames = frame
	stage = "收據寫入"
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if o.Dir == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	f, err := os.OpenFile(filepath.Join(o.Dir, "capture.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
