package geometry

import (
	"image/color"
	"math/big"
)

func (t *Target) wire(a, b pointBig, c color.RGBA) {
	// Exact Q8/256, geometric viewport [0,w]x[0,h], left/right/top/bottom.
	x0 := new(big.Rat).SetFrac(a.x, big.NewInt(256))
	y0 := new(big.Rat).SetFrac(a.y, big.NewInt(256))
	x1 := new(big.Rat).SetFrac(b.x, big.NewInt(256))
	y1 := new(big.Rat).SetFrac(b.y, big.NewInt(256))
	for side := 0; side < 4; side++ {
		av, bv := x0, x1
		limit := int64(0)
		if side >= 2 {
			av, bv = y0, y1
		}
		if side == 1 {
			limit = int64(t.frame.Width)
		}
		if side == 3 {
			limit = int64(t.frame.Height)
		}
		bound := big.NewRat(limit, 1)
		ai, bi := av.Cmp(bound) >= 0, bv.Cmp(bound) >= 0
		if side%2 == 1 {
			ai, bi = av.Cmp(bound) <= 0, bv.Cmp(bound) <= 0
		}
		if !ai && !bi {
			return
		}
		if ai == bi {
			continue
		}
		ratio := new(big.Rat).Quo(new(big.Rat).Sub(bound, av), new(big.Rat).Sub(bv, av))
		ix := new(big.Rat).Add(x0, new(big.Rat).Mul(ratio, new(big.Rat).Sub(x1, x0)))
		iy := new(big.Rat).Add(y0, new(big.Rat).Mul(ratio, new(big.Rat).Sub(y1, y0)))
		if !ai {
			x0, y0 = ix, iy
		} else {
			x1, y1 = ix, iy
		}
	}
	index := func(v *big.Rat, limit int) int {
		q := roundRat(new(big.Rat).Sub(v, big.NewRat(1, 2)))
		return max(0, min(limit-1, int(q.Int64())))
	}
	ax, ay, bx, by := index(x0, t.frame.Width), index(y0, t.frame.Height), index(x1, t.frame.Width), index(y1, t.frame.Height)
	if ax > bx || (ax == bx && ay > by) {
		ax, bx = bx, ax
		ay, by = by, ay
	}
	dx, dy := bx-ax, by-ay
	sx, sy := 1, 1
	if dx < 0 {
		dx = -dx
		sx = -1
	}
	if dy < 0 {
		dy = -dy
		sy = -1
	}
	dy = -dy
	err := dx + dy
	for {
		t.frame.Wire.SetRGBA(ax, ay, c)
		if ax == bx && ay == by {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			ax += sx
		}
		if e2 <= dx {
			err += dx
			ay += sy
		}
	}
}
