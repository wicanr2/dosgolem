//go:build frontendlinked

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/apps/strikecommander/player"
	"github.com/wicanr2/dosgolem/session"
)

// R8 identity is physical CurPSP+10h, not the snapshot's static ProgramPath.
// Sources: 464 READY, 016a section 8 and the frozen physical return witness.
// Prefix=256; SCCD samples resident image+0100h, others sample MZ entry (64).
// Only one complete byte match is accepted, never a filename guess.
type frontendProgram struct {
	name        string
	image       []byte
	entry       uint32
	relocations []uint32
}

// The sampling profile is separate from the original MZ entry metadata.
func frontendProgramSampleOffset(p frontendProgram) uint32 {
	if p.name == "SCCD.EXE" {
		return 0x100
	}
	return p.entry
}

func parseFrontendProgram(name string, raw []byte) (frontendProgram, error) {
	p := frontendProgram{name: name}
	bad := func() (frontendProgram, error) {
		return frontendProgram{}, fmt.Errorf("016 R8 %s 的 MZ 範圍無效", name)
	}
	if len(raw) < 28 || string(raw[:2]) != "MZ" {
		return bad()
	}
	u16 := func(at int) uint32 { return uint32(binary.LittleEndian.Uint16(raw[at:])) }
	tail, pages, relocs, header := u16(2), u16(4), u16(6), u16(8)*16
	if pages == 0 || tail > 511 {
		return bad()
	}
	end := pages * 512
	if tail != 0 {
		end -= 512 - tail
	}
	if header < 28 || header > end || uint64(end) > uint64(len(raw)) || u16(24)+relocs*4 > header {
		return bad()
	}
	p.entry = u16(22)*16 + u16(20)
	if end-header < 256 || uint64(p.entry)+64 > uint64(end-header) || uint64(frontendProgramSampleOffset(p))+64 > uint64(end-header) {
		return bad()
	}
	p.image = append([]byte(nil), raw[header:end]...)
	for i := uint32(0); i < relocs; i++ {
		at := int(u16(24) + i*4)
		off := uint32(binary.LittleEndian.Uint16(raw[at:])) + uint32(binary.LittleEndian.Uint16(raw[at+2:]))*16
		if uint64(off)+2 > uint64(len(p.image)) {
			return bad()
		}
		p.relocations = append(p.relocations, off)
	}
	return p, nil
}

func loadFrontendPrograms(root string) ([]frontendProgram, error) {
	var out []frontendProgram
	for _, name := range []string{"SCCD.EXE", "MENU.EXE", "OPTTEST.EXE", "MKTERR.EXE", "STRIKE.EXE", "TOPTTST1.EXE"} {
		var spec *player.FileSpec
		for i := range player.RequiredFiles {
			if player.RequiredFiles[i].Name == name {
				spec = &player.RequiredFiles[i]
				break
			}
		}
		if spec == nil {
			return nil, fmt.Errorf("016 R8 缺少 %s 固定身分", name)
		}
		path := filepath.Join(root, name)
		st, e := os.Stat(path)
		if e != nil {
			return nil, e
		}
		if !st.Mode().IsRegular() || st.Size() != spec.Size {
			return nil, fmt.Errorf("016 R8 %s 檔案形態或大小不符", name)
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			return nil, e
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != spec.SHA256 {
			return nil, fmt.Errorf("016 R8 %s SHA-256 不符", name)
		}
		p, e := parseFrontendProgram(name, raw)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, nil
}

type frontendBootReader interface {
	ReadBootPSP([]session.BootPSPRange) (session.BootPSPReceipt, error)
}

func relocatedFrontendProgram(p frontendProgram, psp uint16) ([]byte, error) {
	if psp == 0 || psp > 0xffef {
		return nil, fmt.Errorf("016 R8 PSP 無法表示 MZ 載入段")
	}
	image := append([]byte(nil), p.image...)
	for _, off := range p.relocations {
		if uint64(off)+2 > uint64(len(image)) {
			return nil, fmt.Errorf("016 R8 %s 重定位範圍無效", p.name)
		}
		binary.LittleEndian.PutUint16(image[off:], binary.LittleEndian.Uint16(image[off:])+psp+0x10)
	}
	return image, nil
}

func identifyFrontendBootProgram(owner frontendBootReader, library []frontendProgram) (string, error) {
	return identifyFrontendProgram(owner.ReadBootPSP, library)
}

func identifyFrontendProgram(read func([]session.BootPSPRange) (session.BootPSPReceipt, error), library []frontendProgram) (string, error) {
	if len(library) == 0 || len(library) > 6 {
		return "", fmt.Errorf("016 R8 候選數量無效")
	}
	ranges := make([]session.BootPSPRange, 0, 2*len(library))
	for _, p := range library {
		sample := frontendProgramSampleOffset(p)
		if len(p.image) < 256 || uint64(p.entry)+64 > uint64(len(p.image)) || p.entry > ^uint32(0)-0x100 || uint64(sample)+64 > uint64(len(p.image)) || sample > ^uint32(0)-0x100 {
			return "", fmt.Errorf("016 R8 %s 候選範圍無效", p.name)
		}
		ranges = append(ranges, session.BootPSPRange{Offset: 0x100, Dst: make([]byte, 256)}, session.BootPSPRange{Offset: 0x100 + sample, Dst: make([]byte, 64)})
	}
	receipt, e := read(ranges)
	if e != nil {
		return "", fmt.Errorf("016 R8 bootstrap: %w", e)
	}
	var matches []string
	for i, p := range library {
		image, e := relocatedFrontendProgram(p, receipt.PSP)
		if e != nil {
			return "", e
		}
		sample := frontendProgramSampleOffset(p)
		if bytes.Equal(image[:256], ranges[2*i].Dst) && bytes.Equal(image[sample:sample+64], ranges[2*i+1].Dst) {
			matches = append(matches, p.name)
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("016 R8 程式身分命中 %d 筆，必須唯一", len(matches))
	}
	return matches[0], nil
}
