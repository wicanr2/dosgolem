// Command logo 從EOB1 START1.EXE正常啟動器擷取Westwood標誌收據。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/apps/eob1"
	"github.com/wicanr2/dosgolem/oracle"
)

type receipt struct {
	Executable       string   `json:"executable"`
	ExecutableSHA256 string   `json:"executable_sha256"`
	Steps            uint64   `json:"steps"`
	IndexedSHA256    string   `json:"indexed_sha256"`
	PaletteSHA256    string   `json:"palette_sha256"`
	Opened           []string `json:"opened"`
	Unimplemented    []string `json:"unimplemented"`
}

func main() {
	exe := flag.String("exe", "", "玩家自備的START1.EXE（必填）")
	root := flag.String("root", "", "玩家自備的EOB1資料目錄（必填）")
	out := flag.String("out", "", "本機研究artifact輸出目錄（必填）")
	flag.Parse()
	if *exe == "" || *root == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*exe, *root, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(exe, root, out string) error {
	image, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	o, err := oracle.Load(exe, root)
	if err != nil {
		return err
	}
	defer o.Close()
	if err := eob1.ToWestwoodLogo(o); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	indexed := o.Indexed()
	pal := o.Palette()
	palette := make([]byte, 0, 768)
	for _, rgb := range pal {
		palette = append(palette, rgb[:]...)
	}
	if err := os.WriteFile(filepath.Join(out, "indexed.bin"), indexed, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "palette.rgb"), palette, 0o644); err != nil {
		return err
	}
	if err := o.WritePNG(filepath.Join(out, "frame.png")); err != nil {
		return err
	}
	r := receipt{
		Executable: filepath.Base(exe), ExecutableSHA256: digest(image), Steps: o.Steps(),
		IndexedSHA256: digest(indexed), PaletteSHA256: digest(palette),
		Opened: o.Opened(), Unimplemented: o.Unimplemented(),
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(out, "receipt.json"), data, 0o644)
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
