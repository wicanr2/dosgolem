// state-compare 比較兩個 dosgolem 狀態檔的完整持久化語意。
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

type digest struct {
	Machine string `json:"machine_sha256"`
	DOS     string `json:"dos_sha256"`
}

type result struct {
	Left  digest `json:"left"`
	Right digest `json:"right"`
	Equal bool   `json:"equal"`
}

func readSection(r io.Reader) ([]byte, error) {
	var n [8]byte
	if _, err := io.ReadFull(r, n[:]); err != nil {
		return nil, fmt.Errorf("讀不到段長：%w", err)
	}
	length := binary.LittleEndian.Uint64(n[:])
	if length > 1<<30 {
		return nil, fmt.Errorf("段長過大：%d", length)
	}
	b := make([]byte, int(length))
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("讀不到完整段：%w", err)
	}
	return b, nil
}

func readDigest(path string) (digest, error) {
	f, err := os.Open(path)
	if err != nil {
		return digest{}, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return digest{}, fmt.Errorf("%s: 不是 gzip 狀態檔：%w", path, err)
	}
	defer zr.Close()
	machineSection, err := readSection(zr)
	if err != nil {
		return digest{}, fmt.Errorf("%s: machine section：%w", path, err)
	}
	dosSection, err := readSection(zr)
	if err != nil {
		return digest{}, fmt.Errorf("%s: DOS section：%w", path, err)
	}
	machineDigest, err := machine.StateDigest(bytes.NewReader(machineSection))
	if err != nil {
		return digest{}, fmt.Errorf("%s: %w", path, err)
	}
	dosDigest, err := dos.StateDigest(bytes.NewReader(dosSection))
	if err != nil {
		return digest{}, fmt.Errorf("%s: %w", path, err)
	}
	return digest{Machine: hex.EncodeToString(machineDigest[:]), DOS: hex.EncodeToString(dosDigest[:])}, nil
}

func main() {
	leftPath := flag.String("left", "", "左側 .state")
	rightPath := flag.String("right", "", "右側 .state")
	flag.Parse()
	if *leftPath == "" || *rightPath == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "用法：state-compare -left PATH -right PATH")
		os.Exit(2)
	}
	left, err := readDigest(*leftPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	right, err := readDigest(*rightPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	output, err := json.Marshal(result{Left: left, Right: right, Equal: left == right})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println(string(output))
	if left != right {
		os.Exit(1)
	}
}
