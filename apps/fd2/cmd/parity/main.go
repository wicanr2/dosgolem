// Command parity produces a traceable FD2 original/remake image comparison.
package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	fd2parity "github.com/wicanr2/dosgolem/apps/fd2/parity"
)

const (
	wantEXESize   = 357074
	wantEXEMD5    = "b97caf2239a27a896069d03549d96e1e"
	wantEXESHA256 = "222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f"
)

type fileIdentity struct {
	Path   string `json:"path"`
	Size   int    `json:"size"`
	MD5    string `json:"md5"`
	SHA256 string `json:"sha256"`
}

type report struct {
	Schema                int              `json:"schema"`
	Game                  string           `json:"game"`
	Scenario              string           `json:"scenario"`
	State                 string           `json:"state"`
	Executable            fileIdentity     `json:"executable"`
	InputSHA256           string           `json:"input_sha256"`
	DosgolemCommit        string           `json:"dosgolem_commit"`
	RemakeCommit          string           `json:"remake_commit"`
	OriginalCapture       string           `json:"original_capture"`
	OriginalCaptureSHA256 string           `json:"original_capture_sha256"`
	RemakeCapture         string           `json:"remake_capture"`
	RemakeCaptureSHA256   string           `json:"remake_capture_sha256"`
	RemakeNormalization   string           `json:"remake_normalization"`
	Comparison            fd2parity.Result `json:"comparison"`
}

func main() {
	exe := flag.String("exe", "", "固定版本 FD2.EXE（必填）")
	original := flag.String("original", "", "原版 320x200 PNG（必填）")
	remake := flag.String("remake", "", "重製版 320x200 PNG（必填）")
	input := flag.String("input", "", "宣告式輸入 JSON（必填）")
	dosgolemCommit := flag.String("dosgolem-commit", "", "dosgolem Git commit（必填）")
	remakeCommit := flag.String("remake-commit", "", "重製版 Git commit（必填）")
	reportPath := flag.String("report", "", "輸出 JSON 路徑（必填）")
	diffPath := flag.String("diff", "", "輸出差異 PNG 路徑（必填）")
	flag.Parse()
	if *exe == "" || *original == "" || *remake == "" || *input == "" ||
		*dosgolemCommit == "" || *remakeCommit == "" ||
		*reportPath == "" || *diffPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	identity, err := identify(*exe)
	if err != nil {
		die(err)
	}
	if identity.Size != wantEXESize || identity.MD5 != wantEXEMD5 || identity.SHA256 != wantEXESHA256 {
		die(fmt.Errorf("FD2.EXE 身分不符：size=%d md5=%s sha256=%s", identity.Size, identity.MD5, identity.SHA256))
	}
	inputSHA, err := hashFile(*input)
	if err != nil {
		die(err)
	}
	scenario, err := fd2parity.LoadScenario(*input)
	if err != nil {
		die(err)
	}
	originalSHA, err := hashFile(*original)
	if err != nil {
		die(err)
	}
	remakeSHA, err := hashFile(*remake)
	if err != nil {
		die(err)
	}
	a, err := loadPNG(*original)
	if err != nil {
		die(err)
	}
	b, err := loadPNG(*remake)
	if err != nil {
		die(err)
	}
	b, normalization, err := fd2parity.NormalizeRemake(b)
	if err != nil {
		die(err)
	}
	comparison, diff, err := fd2parity.Compare(a, b)
	if err != nil {
		die(err)
	}
	if err := writePNG(*diffPath, diff); err != nil {
		die(err)
	}
	r := report{
		Schema: 1, Game: "fd2", Scenario: scenario.Name, State: scenario.State,
		Executable: identity, InputSHA256: inputSHA,
		DosgolemCommit: *dosgolemCommit, RemakeCommit: *remakeCommit,
		OriginalCapture:       filepath.Base(*original),
		OriginalCaptureSHA256: originalSHA,
		RemakeCapture:         filepath.Base(*remake),
		RemakeCaptureSHA256:   remakeSHA,
		RemakeNormalization:   normalization,
		Comparison:            comparison,
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		die(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(*reportPath, data, 0o644); err != nil {
		die(err)
	}
	fmt.Printf("相同像素 %.3f%%，RGB 平均絕對誤差 %.3f\n", comparison.EqualRatio*100, comparison.MeanAbsRGB)
}

func identify(path string) (fileIdentity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return fileIdentity{}, err
	}
	m := md5.Sum(b)
	s := sha256.Sum256(b)
	return fileIdentity{Path: filepath.Base(path), Size: len(b), MD5: hex.EncodeToString(m[:]), SHA256: hex.EncodeToString(s[:])}, nil
}

func hashFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func writePNG(path string, im image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, im); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "fd2 parity:", err)
	os.Exit(1)
}
