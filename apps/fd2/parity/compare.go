// Package parity compares fixed-state FD2 original and remake captures.
package parity

import (
	"fmt"
	"image"
	"image/color"
)

const (
	Width  = 320
	Height = 200
)

type Box struct {
	MinX int `json:"min_x"`
	MinY int `json:"min_y"`
	MaxX int `json:"max_x"`
	MaxY int `json:"max_y"`
}

type Result struct {
	EqualPixels int     `json:"equal_pixels"`
	TotalPixels int     `json:"total_pixels"`
	EqualRatio  float64 `json:"equal_ratio"`
	MeanAbsRGB  float64 `json:"mean_abs_rgb"`
	DiffBox     *Box    `json:"diff_box,omitempty"`
}

func Compare(original, remake image.Image) (Result, *image.RGBA, error) {
	want := image.Rect(0, 0, Width, Height)
	if original.Bounds() != want {
		return Result{}, nil, fmt.Errorf("原版畫面尺寸為 %v，預期 %v", original.Bounds(), want)
	}
	if remake.Bounds() != want {
		return Result{}, nil, fmt.Errorf("重製畫面尺寸為 %v，預期 %v", remake.Bounds(), want)
	}

	diff := image.NewRGBA(want)
	result := Result{TotalPixels: Width * Height}
	var sum uint64
	var box Box
	hasDiff := false
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			r0, g0, b0, _ := original.At(x, y).RGBA()
			r1, g1, b1, _ := remake.At(x, y).RGBA()
			r0, g0, b0, r1, g1, b1 = r0>>8, g0>>8, b0>>8, r1>>8, g1>>8, b1>>8
			if r0 == r1 && g0 == g1 && b0 == b1 {
				result.EqualPixels++
				diff.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
				continue
			}
			sum += abs(r0, r1) + abs(g0, g1) + abs(b0, b1)
			diff.SetRGBA(x, y, color.RGBA{255, 0, 255, 255})
			if !hasDiff {
				box = Box{x, y, x, y}
				hasDiff = true
			} else {
				box.MinX, box.MaxX = min(box.MinX, x), max(box.MaxX, x)
				box.MinY, box.MaxY = min(box.MinY, y), max(box.MaxY, y)
			}
		}
	}
	result.EqualRatio = float64(result.EqualPixels) / float64(result.TotalPixels)
	result.MeanAbsRGB = float64(sum) / float64(result.TotalPixels*3)
	if hasDiff {
		result.DiffBox = &box
	}
	return result, diff, nil
}

func abs(a, b uint32) uint64 {
	if a > b {
		return uint64(a - b)
	}
	return uint64(b - a)
}
