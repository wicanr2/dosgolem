package strikecommander

// All slices are caller-pre-reserved scratch valid only for this one Draw.
// There is no renderer-owned byte alias or lookup by another CU/palette key.
type hdFlightBackground struct {
	buf                     []byte
	rgb, qualified, applied []byte
}

func (o *FlightOverlay) drawHD(out, indexed []byte, palette [256][3]byte, hd *hdFlightBackground) {
	o.hdBackground = hd
	defer func() { o.hdBackground = nil }()
	o.Draw(out, indexed, palette)
}
func labelBackgroundHD(out, indexed []byte, palette [256][3]byte, scale int, items []bgItem, hd *hdFlightBackground) []byte {
	bg := append([]byte(nil), out...)
	idx := append([]byte(nil), indexed...)
	W := 320 * scale
	for _, it := range items {
		for y := it.y0; y < it.y0+it.rows; y++ {
			if y < 0 || y >= 200 {
				continue
			}
			row := (y - it.y0) * 320
			for x := max(it.x0, 0); x < min(it.x1, 320); x++ {
				bf, af := it.before[row+x], it.after[row+x]
				p := y*320 + x
				if bf == af || idx[p] != af {
					continue
				}
				cur, nc := palette[idx[p]], palette[bf]
				for j := 0; j < scale; j++ {
					for i := 0; i < scale; i++ {
						q := (y*scale+j)*W + x*scale + i
						at := 4 * q
						if bg[at] != cur[0] || bg[at+1] != cur[1] || bg[at+2] != cur[2] {
							continue
						}
						if len(hd.buf) == 64000 && bf == hd.buf[p] && hd.qualified[q] != 0 {
							copy(bg[at:at+4], hd.rgb[at:at+4])
							hd.applied[q] = 1
						} else {
							bg[at], bg[at+1], bg[at+2] = nc[0], nc[1], nc[2]
						}
					}
				}
				idx[p] = bf
			}
		}
	}
	return bg
}
