package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"os"
)

func read(p string) (image.Image, string) {
	b, e := os.ReadFile(p)
	if e != nil {
		panic(e)
	}
	f, e := os.Open(p)
	if e != nil {
		panic(e)
	}
	defer f.Close()
	im, _, e := image.Decode(f)
	if e != nil {
		panic(e)
	}
	return im, fmt.Sprintf("%x", sha256.Sum256(b))
}
func main() {
	if len(os.Args) != 3 {
		panic("輸入DOSBox視窗PNG與dosgolem原生PNG")
	}
	a, ah := read(os.Args[1])
	b, bh := read(os.Args[2])
	if a.Bounds().Dx() != 640 || a.Bounds().Dy() != 417 || b.Bounds().Dx() != 320 || b.Bounds().Dy() != 200 {
		panic("固定2倍顯示／17列視窗功能列尺寸不符")
	}
	different, scaledMismatch, maxDelta := 0, 0, 0
	rows := make([]int, 200)
	columns := make([]int, 320)
	bounds := []int{320, 200, -1, -1}
	rawA, rawB := make([]byte, 0, 192000), make([]byte, 0, 192000)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			ar, ag, ab, _ := a.At(x*2, y*2+17).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			ac := [3]uint32{ar >> 8, ag >> 8, ab >> 8}
			bc := [3]uint32{br >> 8, bg >> 8, bb >> 8}
			if ac != bc {
				different++
				rows[y]++
				columns[x]++
				if x < bounds[0] {
					bounds[0] = x
				}
				if y < bounds[1] {
					bounds[1] = y
				}
				if x > bounds[2] {
					bounds[2] = x
				}
				if y > bounds[3] {
					bounds[3] = y
				}
			}
			for j := 0; j < 3; j++ {
				rawA = append(rawA, byte(ac[j]))
				rawB = append(rawB, byte(bc[j]))
				d := int(ac[j]) - int(bc[j])
				if d < 0 {
					d = -d
				}
				if d > maxDelta {
					maxDelta = d
				}
			}
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					r, g, b, _ := a.At(x*2+dx, y*2+17+dy).RGBA()
					if [3]uint32{r >> 8, g >> 8, b >> 8} != ac {
						scaledMismatch++
					}
				}
			}
		}
	}
	r := map[string]any{"different_bounds_xy_inclusive": bounds, "different_by_row": rows, "different_by_column": columns, "classification": "DOSBox輔助對應畫面；非完整機器同狀態", "dosbox_window": os.Args[1], "dosbox_sha256": ah, "dosgolem_native": os.Args[2], "dosgolem_sha256": bh, "comparison_pixels": 64000, "different_pixels": different, "maximum_channel_delta": maxDelta, "nonuniform_scaled_pixels": scaledMismatch, "normalization": "DOSBox完整640x400遊戲內容：視窗y=17，2x整數顯示；只排除17列外部功能列，遊戲內容無遮罩", "dosbox_RGB_sha256": fmt.Sprintf("%x", sha256.Sum256(rawA)), "dosgolem_RGB_sha256": fmt.Sprintf("%x", sha256.Sum256(rawB))}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	if err := e.Encode(r); err != nil {
		panic(err)
	}
}
