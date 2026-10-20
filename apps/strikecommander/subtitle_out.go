package strikecommander

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// 010 §3.11 的收據檔：
//   - <前綴>-subs.tsv、<前綴>-subskips.tsv、<前綴>-subsmeta.tsv；
//   - 每個 shot 另寫 <shot 前綴>-s<倍率>-substamps.tsv（shot 的 frame.idx、palette.bin、PNG 照 WriteShotFiles）；
//   - <前綴>-sbdma.tsv：RecordSBDMA 記下的全部事件，欄位同 probe 的 -sb-dma-log（452 §2.6）。

// SubtitleMeta 是 subsmeta.tsv 的內容。
type SubtitleMeta struct {
	FontSHA256, IndexSHA256, CatalogSHA256 string
	E, T                                   uint64
}

// SubStampsTSV 編一個 shot 某倍率的 substamps.tsv：每行 stamp 一列；沒有畫時只有一列，stamp 欄位留空。
func SubStampsTSV(s SubShot, scale int) []byte {
	var b strings.Builder
	b.WriteString("frame_step\tenabled\tblocked\tX\tY\tCells\tCellW\tCellH\tGlyphX\tGlyphY\tText\n")
	head := fmt.Sprintf("%d\t%t\t%t", s.FrameStep, s.Enabled, s.Blocked)
	if len(s.Lines[scale]) == 0 {
		b.WriteString(head + "\t\t\t\t\t\t\t\t\n")
	}
	for _, l := range s.Lines[scale] {
		fmt.Fprintf(&b, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", head, l.X, l.Y, l.Cells, l.CellW, l.CellH, l.GlyphX, l.GlyphY, string(l.Text))
	}
	return []byte(b.String())
}

// SBDMATSV 編事件清單，欄位與 probe 的 -sb-dma-log 相同（machine.SBDMAEventFields）。
func SBDMATSV(evs []machine.SBDMAEvent) []byte {
	var b strings.Builder
	b.WriteString(strings.Join(machine.SBDMAEventFields, "\t") + "\n")
	for _, e := range evs {
		b.WriteString(strings.Join(e.Fields(), "\t") + "\n")
	}
	return []byte(b.String())
}

// WriteSubtitleReceipt 在 dir 寫 010 §3.11 的收據檔，前綴 prefix；shots 與 r.Shots、r.SubShots 平行時另寫各 shot 的檔。
func WriteSubtitleReceipt(dir, prefix string, r *ReceiptResult, meta SubtitleMeta, scales []int) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	put := func(name string, b []byte) error { return os.WriteFile(filepath.Join(dir, prefix+"-"+name), b, 0o644) }
	if err := put("subs.tsv", SubsTSV(r.Subs)); err != nil {
		return err
	}
	if err := put("subskips.tsv", SubSkipsTSV(r.SubSkips)); err != nil {
		return err
	}
	m := fmt.Sprintf("item\tvalue\nfont_sha256\t%s\nspeech_index_sha256\t%s\nspeech_catalog_sha256\t%s\nE\t%d\nT\t%d\n",
		meta.FontSHA256, meta.IndexSHA256, meta.CatalogSHA256, meta.E, meta.T)
	if err := put("subsmeta.tsv", []byte(m)); err != nil {
		return err
	}
	if r.SBDMA != nil {
		if err := put("sbdma.tsv", SBDMATSV(r.SBDMA)); err != nil {
			return err
		}
	}
	for i := range r.Shots {
		s := &r.Shots[i]
		sp := ShotPrefix(s)
		if err := WriteShotFiles(dir, sp, s, scales); err != nil {
			return err
		}
		if i >= len(r.SubShots) {
			continue
		}
		for _, sc := range scales {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-s%d-substamps.tsv", sp, sc)), SubStampsTSV(r.SubShots[i], sc), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
