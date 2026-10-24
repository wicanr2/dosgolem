package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/audio/sbpcm"
	"github.com/wicanr2/dosgolem/internal/machine"
)

type sbPCMOptions struct {
	wav, raw *string
	sps      *uint64
	linear   *bool
}

func sbPCMFlags(fs *flag.FlagSet) sbPCMOptions {
	return sbPCMOptions{
		wav:    fs.String("dump-sbpcm-wav", "", "只傾印 SB16 PCM 為 48 kHz、16 位元單聲道 WAV（規格 462）"),
		raw:    fs.String("dump-sbpcm-raw", "", "每筆 start 的實體 DMA 樣本傾印目錄（規格 462）"),
		sps:    fs.Uint64("sbpcm-sps", 11550000, "SBPCM 傾印每秒步數"),
		linear: fs.Bool("sbpcm-linear-check", false, "比較線性 DMA 位址與實體取樣，只記差異次數"),
	}
}

type sbPCMDump struct {
	source                                             *sbpcm.Source
	m                                                  *machine.Machine
	file                                               *os.File
	w                                                  *bufio.Writer
	raw                                                string
	sps, first, next, lastFlush, count, linearMismatch uint64
	started, linear                                    bool
	err                                                error
	lastDMA                                            machine.SBDMAEvent
	previousDMA                                        func(machine.SBDMAEvent)
}

func openSBPCM(m *machine.Machine, o sbPCMOptions) (*sbPCMDump, error) {
	if *o.wav == "" && *o.raw == "" && !*o.linear {
		return nil, nil
	}
	if !m.SBDMA8() {
		return nil, fmt.Errorf("SBPCM 傾印需要 -sb-dma8")
	}
	if *o.sps == 0 {
		return nil, fmt.Errorf("-sbpcm-sps 必須大於 0")
	}
	d := &sbPCMDump{m: m, source: sbpcm.New(*o.sps), sps: *o.sps, raw: *o.raw, linear: *o.linear}
	if d.raw != "" {
		if err := os.MkdirAll(d.raw, 0755); err != nil {
			return nil, err
		}
	}
	if *o.wav != "" {
		f, err := os.Create(*o.wav)
		if err != nil {
			return nil, err
		}
		d.file, d.w = f, bufio.NewWriterSize(f, 64*1024)
		if _, err := f.Write(sbPCMWAVHeader(0)); err != nil {
			f.Close()
			return nil, err
		}
	}
	// 線性診斷取剛送出的 SBDMA 定位資訊，保留既有 TSV 回呼與
	// nil 回呼時的 SBDMALog 路徑，不新增客體 API 或改寫收據。
	if d.linear && m.OnSBDMA != nil {
		d.previousDMA = m.OnSBDMA
		m.OnSBDMA = func(e machine.SBDMAEvent) { d.previousDMA(e); d.lastDMA = e }
	}
	m.ObserveSBPCM(d.apply)
	return d, nil
}

func (d *sbPCMDump) sampleOf(step uint64) uint64 { return step * 48000 / d.sps }
func (d *sbPCMDump) apply(e machine.SBPCMEvent) {
	if d.err != nil {
		return
	}
	if e.Kind == "start" {
		if !d.started {
			d.first, d.next, d.started = d.sampleOf(e.Start), d.sampleOf(e.Start), true
		}
		if d.raw != "" {
			name := fmt.Sprintf("%d-%.16s.u8", e.Step, e.SHA256)
			d.err = os.WriteFile(filepath.Join(d.raw, name), e.Data, 0644)
		}
		if d.linear && e.Command != 0x80 {
			b := make([]byte, e.Length)
			loc := d.lastDMA
			if d.previousDMA == nil && len(d.m.SBDMALog) > 0 {
				loc = d.m.SBDMALog[len(d.m.SBDMALog)-1]
			}
			for i := range b {
				b[i] = d.m.Peek8(uint32(loc.Page)<<16 | uint32(loc.Addr+uint16(i)))
			}
			if !bytes.Equal(b, e.Data) {
				d.linearMismatch++
			}
		}
	}
	d.source.Apply(e)
}

// flush 在有界回合後寫入；不在每個 CPU 步合成，也不累存整段 WAV。
func (d *sbPCMDump) flush(step uint64, final bool) {
	if d == nil || d.err != nil || (!final && step-d.lastFlush < 192500) {
		return
	}
	d.lastFlush = step
	end := d.sampleOf(step)
	if d.started && d.w != nil {
		var buf [8192]byte
		for d.next < end {
			n := 0
			for n < len(buf) && d.next < end {
				binary.LittleEndian.PutUint16(buf[n:], uint16(int16(d.source.Sample(d.next))))
				n += 2
				d.next++
				d.count++
			}
			if _, d.err = d.w.Write(buf[:n]); d.err != nil {
				return
			}
		}
	}
	d.source.Prune(end)
}

func sbPCMWAVHeader(dataBytes uint32) []byte {
	b := make([]byte, 44)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], 36+dataBytes)
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], 48000)
	binary.LittleEndian.PutUint32(b[28:], 96000)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], dataBytes)
	return b
}

func (d *sbPCMDump) close(step uint64, report io.Writer) error {
	if d == nil {
		return nil
	}
	d.flush(step, true)
	d.m.ObserveSBPCM(nil)
	if d.previousDMA != nil {
		d.m.OnSBDMA = d.previousDMA
	}
	if d.file != nil {
		if err := d.w.Flush(); d.err == nil {
			d.err = err
		}
		if d.count > (uint64(^uint32(0))-36)/2 {
			if d.err == nil {
				d.err = fmt.Errorf("SBPCM WAV 超過 RIFF 長度上限")
			}
		} else if d.err == nil {
			_, d.err = d.file.WriteAt(sbPCMWAVHeader(uint32(d.count*2)), 0)
		}
		if err := d.file.Close(); d.err == nil {
			d.err = err
		}
	}
	st := d.source.Stats()
	fmt.Fprintf(report, "SBPCM firstSample=%d endStep=%d voices=%d adpcmSkipped=%d cycleClockSkipped=%d completeMismatch=%d orphanEvents=%d SBPCMMutated=%d",
		d.first, step, st.Voices, st.ADPCMSkipped, st.CycleClockSkipped, st.CompleteMismatch, st.OrphanEvents, d.m.SBPCMMutated())
	if d.linear {
		fmt.Fprintf(report, " linearMismatch=%d", d.linearMismatch)
	}
	fmt.Fprintln(report)
	return d.err
}
