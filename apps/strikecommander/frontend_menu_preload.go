package strikecommander

import "crypto/sha256"

type PresentMenuPreload struct {
	Enabled     bool
	TRE, Reason string
	ButtonsSHA  [32]byte
}

func (p *Presenter) loadPresetMenu(root string) {
	if root == "" {
		return
	}
	m, e := ledgerReadMainMenu(root)
	if e != nil {
		p.counts.PlatePreloadMissing++
		p.menuPreload.Reason = e.Error()
		return
	}
	p.presetMenuButtons = m.buttons
	p.menuPreload = PresentMenuPreload{Enabled: true, TRE: m.tre, ButtonsSHA: sha256.Sum256(m.buttons)}
}
func (p *Presenter) MenuPreload() PresentMenuPreload { return p.menuPreload }

// scaleFrontendInto avoids an additional retained RGBA during preparation.
func scaleFrontendInto(dst, indexed []byte, palette [256][3]byte, scale int) {
	W := 320 * scale
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			c := palette[indexed[y*320+x]]
			for j := 0; j < scale; j++ {
				for i := 0; i < scale; i++ {
					at := 4 * ((y*scale+j)*W + x*scale + i)
					dst[at], dst[at+1], dst[at+2], dst[at+3] = c[0], c[1], c[2], 255
				}
			}
		}
	}
}
func (p *Presenter) composeImageInto(img *presentImage, scale int, subs bool, dst []byte) {
	scaleFrontendInto(dst, img.indexed, img.palette, scale)
	ov := p.ovs[scale]
	ov.Sync(img.labels, img.labelGen)
	ov.drawOverlayInto(dst, img.indexed, img.palette)
	if fov := p.fovs[scale]; fov != nil {
		fov.Sync(img.flight, img.presGen)
		fov.Draw(dst, img.indexed, img.palette)
	}
	if subs && p.subs != nil && img.subPtr != nil && !img.subBlocked && img.sub.Text != "" {
		v := img.sub
		p.subs.drawText(&v, dst, scale)
	}
}
