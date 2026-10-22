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

// SubtitleMeta 是 subsmeta.tsv 的內容（010 §3.11、011 §3.8）：結構表與譯文的 SHA-256、E、T，
// 以及 2、3 倍圖集的名稱、字級與 SHA-256。
type SubtitleMeta struct {
	IndexSHA256, CatalogSHA256 string
	E, T                       uint64
	TextName                   [2]string
	TextPx                     [2]int
	TextSHA256                 [2]string
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// SubsMetaTSV 編 subsmeta.tsv：item、value 兩欄。
func SubsMetaTSV(meta SubtitleMeta) []byte {
	return []byte(fmt.Sprintf("item\tvalue\nspeech_index_sha256\t%s\nspeech_catalog_sha256\t%s\nE\t%d\nT\t%d\n"+
		"font_name_s2\t%s\nfont_name_s3\t%s\nfont_px_s2\t%d\nfont_px_s3\t%d\nfont_sha256_s2\t%s\nfont_sha256_s3\t%s\n",
		meta.IndexSHA256, meta.CatalogSHA256, meta.E, meta.T, meta.TextName[0], meta.TextName[1], meta.TextPx[0], meta.TextPx[1],
		meta.TextSHA256[0], meta.TextSHA256[1]))
}

// SubStampsTSV 編一個 shot 某倍率的 substamps.tsv（011 §3.8）：每行一列（Text 與 display 是該行字串，N() 後）；
// 沒有畫時只有一列，Text 空、其餘欄 -。
func SubStampsTSV(s SubShot, scale int) []byte {
	var b strings.Builder
	b.WriteString("frame_step\tenabled\tblocked\tText\tpx\ttx_out\tbaseline\tink_x0\tink_y0\tink_x1\tink_y1\tdisplay\n")
	head := fmt.Sprintf("%d\t%t\t%t", s.FrameStep, s.Enabled, s.Blocked)
	if len(s.Lines[scale]) == 0 {
		b.WriteString(head + "\t" + strings.Repeat("\t-", 8) + "\n")
	}
	for _, l := range s.Lines[scale] {
		ink := l.Ink
		if ink.Empty() {
			ink.X0, ink.Y0, ink.X1, ink.Y1 = 0, 0, 0, 0
		}
		fmt.Fprintf(&b, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", head, string(l.Text),
			l.Px, l.TxOut, l.Baseline, ink.X0, ink.Y0, ink.X1, ink.Y1, string(l.Text))
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
