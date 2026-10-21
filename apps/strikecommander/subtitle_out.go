package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
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

// SubtitleMeta 是 subsmeta.tsv 的內容（010 §3.11）。第 4 版加字幕字型的名稱與尺寸（SC-SUB10、10、10），
// FontSHA256 是 SC-SUB10 檔的雜湊，不再記 16×16 字型。
type SubtitleMeta struct {
	FontName                               string
	FontW, FontH                           int
	FontSHA256, IndexSHA256, CatalogSHA256 string
	E, T                                   uint64
}

// NewSubtitleMeta 由字幕字型與三個輸入檔的位元組組 SubtitleMeta：名稱與尺寸取自解析後的 xlate.Font，不另寫字面值。
func NewSubtitleMeta(font *SubtitleFont, fontB, indexB, catalogB []byte, e, t uint64) SubtitleMeta {
	m := SubtitleMeta{FontSHA256: sha256Hex(fontB), IndexSHA256: sha256Hex(indexB), CatalogSHA256: sha256Hex(catalogB), E: e, T: t}
	if font != nil && font.Font != nil {
		m.FontName, m.FontW, m.FontH = font.Font.Name, font.Font.W, font.Font.H
	}
	return m
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// SubsMetaTSV 編 subsmeta.tsv：item、value 兩欄（010 §3.11 第 4 版的列名）。
func SubsMetaTSV(meta SubtitleMeta) []byte {
	return []byte(fmt.Sprintf("item\tvalue\nfont_name\t%s\nfont_w\t%d\nfont_h\t%d\nfont_sha256\t%s\nspeech_index_sha256\t%s\nspeech_catalog_sha256\t%s\nE\t%d\nT\t%d\n",
		meta.FontName, meta.FontW, meta.FontH, meta.FontSHA256, meta.IndexSHA256, meta.CatalogSHA256, meta.E, meta.T))
}

// SubStampsTSV 編一個 shot 某倍率的 substamps.tsv：每行 stamp 一列；沒有畫時只有一列，stamp 欄位留空。
func SubStampsTSV(s SubShot, scale int) []byte {
	var b strings.Builder
	b.WriteString("frame_step\tenabled\tblocked\tX\tY\tCells\tCellW\tCellH\tGlyphX\tGlyphY\tGlyphScale\tText\n")
	head := fmt.Sprintf("%d\t%t\t%t", s.FrameStep, s.Enabled, s.Blocked)
	if len(s.Lines[scale]) == 0 {
		b.WriteString(head + "\t\t\t\t\t\t\t\t\t\n")
	}
	for _, l := range s.Lines[scale] {
		fmt.Fprintf(&b, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", head, l.X, l.Y, l.Cells, l.CellW, l.CellH, l.GlyphX, l.GlyphY, l.GlyphScale, string(l.Text))
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
	if err := put("subsmeta.tsv", SubsMetaTSV(meta)); err != nil {
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
