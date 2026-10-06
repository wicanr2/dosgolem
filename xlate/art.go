package xlate

import (
	"fmt"
	"sort"
)

// product 在尺寸乘法前防止整數溢位（spec 204 §2.1）。
func product(values ...int) (int, bool) {
	n := 1
	maxInt := int(^uint(0) >> 1)
	for _, v := range values {
		if v <= 0 || n > maxInt/v {
			return 0, false
		}
		n *= v
	}
	return n, true
}

func (l *Layer) validateArt(s *Stamp) error {
	if s == nil || !s.Art || s.Key == "" {
		return fmt.Errorf("xlate: 圖面必須有 Art 及非空 Key")
	}
	w, ok := product(s.Cells, s.CellW)
	if !ok || s.CellH <= 0 || s.X < 0 || s.Y < 0 || w > l.width() || s.CellH > l.height() ||
		s.X > l.width()-w || s.Y > l.height()-s.CellH {
		return fmt.Errorf("xlate: 圖面 %s 的矩形或格數不合法", s.Key)
	}
	n, ok := product(w, s.CellH)
	if !ok || len(s.Reference) != n {
		return fmt.Errorf("xlate: 圖面 %s 的原版基準長度不符", s.Key)
	}
	n, ok = product(w, s.CellH, s.PixScale, s.PixScale, 4)
	if !ok || len(s.Pix) != n {
		return fmt.Errorf("xlate: 圖面 %s 的倍率或 RGBA 長度不符", s.Key)
	}
	return nil
}

func cloneArt(s *Stamp) *Stamp {
	c := *s
	c.Pix = append([]uint8(nil), s.Pix...)
	c.Reference = append([]uint8(nil), s.Reference...)
	c.Transparent = append([]bool(nil), s.Transparent...)
	c.artHidden = make([]bool, s.Cells)
	c.artOpaque = make([]bool, s.Cells)
	rowW := s.Cells * s.CellW * s.PixScale
	for cell := 0; cell < s.Cells; cell++ {
		opaque := true
		for y := 0; y < s.CellH*s.PixScale && opaque; y++ {
			for x := cell * s.CellW * s.PixScale; x < (cell+1)*s.CellW*s.PixScale; x++ {
				if c.Pix[4*(y*rowW+x)+3] != 255 {
					opaque = false
					break
				}
			}
		}
		c.artOpaque[cell] = opaque
	}
	c.State = Printing // 有效 Frame 比對全部格後才顯示
	c.scaleWarned = false
	c.hashes, c.misses, c.anchors = nil, nil, nil
	return &c
}

func (l *Layer) sortArt() {
	sort.SliceStable(l.Art, func(i, j int) bool {
		if l.Art[i].Order != l.Art[j].Order {
			return l.Art[i].Order < l.Art[j].Order
		}
		return l.Art[i].Key < l.Art[j].Key
	})
}

// AddArt 登記不可變資產與原版基準；同 Key 替換，重疊的不同 Key 保留。
func (l *Layer) AddArt(s *Stamp) error {
	if err := l.validateArt(s); err != nil {
		return err
	}
	copy := cloneArt(s)
	for i, old := range l.Art {
		if old.Key == s.Key {
			l.Art[i] = copy
			l.sortArt()
			return nil
		}
	}
	l.Art = append(l.Art, copy)
	l.sortArt()
	return nil
}

// DropArt 明確移除一列，不影響文字或其他圖面。
func (l *Layer) DropArt(key string) {
	keep := l.Art[:0]
	for _, s := range l.Art {
		if s.Key == key {
			l.drop(s, "explicit")
		} else {
			keep = append(keep, s)
		}
	}
	l.Art = keep
}

func (l *Layer) ClearArt() {
	for _, s := range l.Art {
		l.drop(s, "clear")
	}
	l.Art = nil
	l.artOwners = nil
}

// frameArt 逐格完整比較，上一幀的遮格絕不成為本幀的永久狀態。
func (l *Layer) frameArt(indexed []uint8) {
	w := l.width()
	for _, s := range l.Art {
		if s.Owner != "" && !l.artOwners[s.Owner] {
			s.State = Printing
			continue
		}
		if l.validateArt(s) != nil {
			s.State = Printing
			continue
		}
		if len(s.artHidden) != s.Cells {
			s.artHidden = make([]bool, s.Cells)
		}
		rowW := s.Cells * s.CellW
		for cell := 0; cell < s.Cells; cell++ {
			hidden := s.transparent(cell)
			for y := 0; y < s.CellH && !hidden; y++ {
				for x := cell * s.CellW; x < (cell+1)*s.CellW; x++ {
					if indexed[(s.Y+y)*w+s.X+x] != s.Reference[y*rowW+x] {
						hidden = true
						break
					}
				}
			}
			s.artHidden[cell] = hidden
		}
		s.State = Shown
	}
}

// over 合成非預乘 RGBA；透明目標仍保留來源 RGB，不能預乘兩次。
func over(dst, src []uint8) {
	a := int(src[3])
	if a == 0 {
		return
	}
	if a == 255 {
		copy(dst, src)
		return
	}
	da := int(dst[3])
	den := a*255 + da*(255-a)
	for c := 0; c < 3; c++ {
		dst[c] = uint8((int(src[c])*a*255 + int(dst[c])*da*(255-a) + den/2) / den)
	}
	dst[3] = uint8((den + 127) / 255)
}

func (l *Layer) drawArt(dst []uint8, scale int) bool {
	drew := false
	W := l.width() * scale
	for _, s := range l.Art {
		if s.State != Shown {
			continue
		}
		if s.PixScale != scale {
			if !s.scaleWarned {
				l.drop(s, "scale")
				s.scaleWarned = true
			}
			continue
		}
		s.scaleWarned = false
		if l.validateArt(s) != nil || len(s.artHidden) != s.Cells || len(s.artOpaque) != s.Cells {
			continue
		}
		rowW := s.Cells * s.CellW * scale
		cellW := s.CellW * scale
		for y := 0; y < s.CellH*scale; y++ {
			for cell := 0; cell < s.Cells; {
				if s.artHidden[cell] {
					cell++
					continue
				}
				if s.artOpaque[cell] {
					end := cell + 1
					for end < s.Cells && !s.artHidden[end] && s.artOpaque[end] {
						end++
					}
					i := 4 * (y*rowW + cell*cellW)
					j := 4 * ((s.Y*scale+y)*W + s.X*scale + cell*cellW)
					n := (end - cell) * cellW * 4
					copy(dst[j:j+n], s.Pix[i:i+n])
					drew = true
					cell = end
					continue
				}
				for x := cell * cellW; x < (cell+1)*cellW; x++ {
					i := 4 * (y*rowW + x)
					if s.Pix[i+3] == 0 {
						continue
					}
					j := 4 * ((s.Y*scale+y)*W + s.X*scale + x)
					over(dst[j:j+4], s.Pix[i:i+4])
					drew = true
				}
				cell++
			}
		}
	}
	return drew
}

func artKeySet(keys []string) (map[string]bool, bool) {
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		if k == "" || set[k] {
			return nil, false
		}
		set[k] = true
	}
	return set, len(set) > 0
}

func (l *Layer) artComplete(owner string, keys map[string]bool) bool {
	seen := make(map[string]bool, len(keys))
	for _, s := range l.Art {
		if s.Owner != owner {
			continue
		}
		if !keys[s.Key] || seen[s.Key] {
			return false
		}
		seen[s.Key] = true
	}
	return len(seen) == len(keys)
}

func (l *Layer) checkArtWatcher(wa *Watcher, indexed []uint8, w, h int) {
	l.artOwners[wa.Key] = false
	keys, ok := artKeySet(wa.ArtKeys)
	if !ok || wa.Key == "" || wa.Make == nil {
		l.drop(&Stamp{Key: wa.Key, Art: true}, "art")
		return
	}
	if !wa.match(indexed, w, h) {
		return
	}
	if l.artComplete(wa.Key, keys) {
		l.artOwners[wa.Key] = true
		return
	}
	rows := wa.Make()
	prepared := make([]*Stamp, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, s := range rows {
		if err := l.validateArt(s); err != nil {
			if s == nil {
				s = &Stamp{Key: wa.Key, Art: true}
			}
			l.drop(s, "art")
			return
		}
		if !keys[s.Key] || seen[s.Key] {
			l.drop(s, "art")
			return
		}
		for _, old := range l.Art {
			if old.Key == s.Key && old.Owner != wa.Key {
				l.drop(s, "art")
				return
			}
		}
		seen[s.Key] = true
		c := cloneArt(s)
		c.Owner = wa.Key
		prepared = append(prepared, c)
	}
	if len(seen) != len(keys) {
		l.drop(&Stamp{Key: wa.Key, Art: true}, "art")
		return
	}
	keep := l.Art[:0]
	for _, s := range l.Art {
		if s.Owner != wa.Key {
			keep = append(keep, s)
		}
	}
	l.Art = append(keep, prepared...)
	l.sortArt()
	l.artOwners[wa.Key] = true
}

// artSnapshot 只有中繼資料。還原後必須從已知資產與基準重新登記。
type artSnapshot struct {
	Key      string `json:"key"`
	Owner    string `json:"owner,omitempty"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Cells    int    `json:"cells"`
	CellW    int    `json:"cell_w"`
	CellH    int    `json:"cell_h"`
	PixScale int    `json:"pix_scale"`
	Order    int    `json:"order"`
}
