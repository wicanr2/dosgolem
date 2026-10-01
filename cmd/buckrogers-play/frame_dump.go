package main

// 自動模式畫格輸出（Buck 規格 049）：每隔 N 格把合成畫格寫成 PNG，供外部工具合成影片。
// 不改遊戲與覆繪：只在 finishFrame 開頭多呼叫一次 Compose 並寫檔。

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

const (
	frameEveryUnset   = -1 // -frame-every 的預設：未給
	frameEveryDefault = 2  // 主機 60 Hz 的一半，30 fps
	frameEveryMax     = 60
)

// checkFrameDump 驗證 -frame-dir／-frame-every，回傳實際的間隔（沒開輸出時為 0）。
func checkFrameDump(frames int, dir string, every int, script map[int][]scriptAction) (int, error) {
	if dir == "" {
		if every != frameEveryUnset {
			return 0, errors.New("-frame-every 須搭配 -frame-dir")
		}
		return 0, nil
	}
	if frames <= 0 {
		return 0, errors.New("-frame-dir 須搭配 -frames（大於 0 的自動模式）")
	}
	if every == frameEveryUnset {
		every = frameEveryDefault
	}
	if every < 1 || every > frameEveryMax {
		return 0, fmt.Errorf("-frame-every 須在 1–%d，得 %d", frameEveryMax, every)
	}
	for f, acts := range script {
		for _, a := range acts {
			if a.scale {
				return 0, fmt.Errorf("-frame-dir 不接受含 scale 動作的腳本（第 %d 格）：輸出尺寸會中途改變", f)
			}
		}
	}
	return every, nil
}

// frameDumper 依間隔輸出畫格；first、last 是第一張與最後一張的畫格編號。
type frameDumper struct {
	dir         string
	every       int
	count       int
	first, last int
}

// newFrameDumper 建立輸出目錄；目錄須不存在或為空，避免新舊畫格混在一起。
func newFrameDumper(dir string, every int) (*frameDumper, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(ents) > 0 {
		return nil, fmt.Errorf("-frame-dir %s 須不存在或為空", dir)
	}
	return &frameDumper{dir: dir, every: every}, nil
}

func frameDumpName(frame int) string { return fmt.Sprintf("frame-%08d.png", frame) }

// frameEncode 可由測試替換，用來模擬編碼中途失敗。
var frameEncode = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode

// maybeWrite 在 frame 是間隔的倍數時呼叫 compose 並寫檔；不到時不呼叫 compose。
// compose 回 ok=false（尚無畫格）時略過，不報錯、不佔編號。
func (d *frameDumper) maybeWrite(frame, scale int, compose func() ([]byte, bool, error)) error {
	if frame%d.every != 0 {
		return nil
	}
	rgba, ok, err := compose()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return d.write(frame, 320*scale, 200*scale, rgba)
}

// write 以 O_EXCL 建檔寫 PNG；編碼或關閉失敗時刪除這個半成品（不刪已存在的檔）。
func (d *frameDumper) write(frame, w, h int, rgba []byte) (err error) {
	if len(rgba) != 4*w*h {
		return fmt.Errorf("畫格 %d 的像素 %d 位元組，與 %d×%d 不符", frame, len(rgba), w, h)
	}
	path := filepath.Join(d.dir, frameDumpName(frame))
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(path)
		}
	}()
	img := &image.NRGBA{Pix: rgba, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	err = frameEncode(f, img)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if d.count == 0 {
		d.first = frame
	}
	d.last = frame
	d.count++
	return nil
}

func (d *frameDumper) summary() string {
	return fmt.Sprintf("buckrogers-play: frame-dump dir=%s every=%d count=%d first=%d last=%d", d.dir, d.every, d.count, d.first, d.last)
}
