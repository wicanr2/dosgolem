// Package state 把一台跑到一半的機器存成檔案，讓下一支行程從那裡接著跑。
//
// **為什麼要有它**：要觀測的東西常常在幾億道指令之後。每問一個問題就
// 從頭跑一次，一輪要好幾分鐘，實驗只能用猜的。存一次、之後每次從那裡
// 展開，一輪是秒級的。
//
// 檔案是 gzip 過的兩段 gob：機器（`machine.SaveState`）與 DOS
// （`dos.SaveState`）。**兩段都要**——記憶體與 CPU 只是一半，
// 開著的檔、配出去的區塊、EMS 映射、XMS 的 EMB 都在 Go 這一端。
//
// 每一段前面有 8 bytes 的長度。**不能把兩個 gob 直接接在一起**：
// `gob.Decoder` 會預讀，第一段解完就已經吃掉第二段的開頭，
// 症狀是第二段報「unknown type id or corrupted data」。
package state

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// section 把一段寫成「8 bytes 長度 ＋ 內容」。
func section(w io.Writer, fill func(io.Writer) error) error {
	var buf bytes.Buffer
	if err := fill(&buf); err != nil {
		return err
	}
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(buf.Len()))
	if _, err := w.Write(n[:]); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// readSection 回一個只讀那一段的 reader。
func readSection(r io.Reader) (io.Reader, error) {
	var n [8]byte
	if _, err := io.ReadFull(r, n[:]); err != nil {
		return nil, fmt.Errorf("state: 讀不到段長：%w", err)
	}
	return io.LimitReader(r, int64(binary.LittleEndian.Uint64(n[:]))), nil
}

// Save 把機器與 DOS 一起寫到 path。
func Save(path string, m *machine.Machine, d *dos.DOS) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 1<<20)
	zw := gzip.NewWriter(bw)
	if err := section(zw, m.SaveState); err != nil {
		return err
	}
	if err := section(zw, d.SaveState); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return bw.Flush()
}

// Load 把 path 的狀態倒進機器與 DOS。
//
// 呼叫之前機器要建好、`dos.Install()` 要跑過——中斷處理常式掛在 Go 這端，
// 不在存檔裡。
func Load(path string, m *machine.Machine, d *dos.DOS) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(bufio.NewReaderSize(f, 1<<20))
	if err != nil {
		return fmt.Errorf("state: %s 不是 gzip：%w", path, err)
	}
	defer zr.Close()
	ms, err := readSection(zr)
	if err != nil {
		return err
	}
	if err := m.LoadState(ms); err != nil {
		return err
	}
	// 機器那一段可能沒被讀完（gob 只取它要的），剩下的丟掉才對得到下一段。
	if _, err := io.Copy(io.Discard, ms); err != nil {
		return err
	}
	ds, err := readSection(zr)
	if err != nil {
		return err
	}
	return d.LoadState(ds)
}
