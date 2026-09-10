package machine

// loadCGACompatiblePalette 實作 0Dh/0Eh 的 BIOS 預設色盤。
// 位元公式及屬性色號映射見 docs/spec/194-ega-bios-default-palette.md。
func (m *Machine) loadCGACompatiblePalette() {
	m.DAC = [768]uint8{}
	for i := 0; i < 64; i++ {
		intensity := uint8((i>>4)&1) * 21
		r := uint8((i>>2)&1)*42 + intensity
		g := uint8((i>>1)&1)*42 + intensity
		b := uint8(i&1)*42 + intensity
		if i&0x17 == 6 {
			g = 21
		}
		m.DAC[i*3], m.DAC[i*3+1], m.DAC[i*3+2] = r, g, b
	}
	for i := 0; i < 16; i++ {
		m.VGA.ac[i] = uint8(i&7) | uint8(i&8)<<1
	}
	m.VGA.ac[0x10], m.VGA.ac[0x12], m.VGA.ac[0x14] = 1, 15, 0
}
