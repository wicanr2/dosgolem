package buckrogers

import (
	"fmt"
	"strconv"

	"github.com/wicanr2/dosgolem/xlate"
)

var overlayRectHeader = []string{
	"event_key", "x", "y", "width", "height", "draw_x", "draw_y",
	"capacity_cells", "line_count", "overflow_policy",
}

type overlayRect struct {
	x, y, width, height, drawX, drawY, capacity, lines int
	overflow                                           string
}

// MenuOverlayRects is an immutable event-key to proven text-safe rectangle map.
type MenuOverlayRects struct {
	byEvent  map[string]overlayRect
	extended map[string]bool
}

func LoadMenuOverlayRects(name string, data []byte) (*MenuOverlayRects, error) {
	rows, err := readTSV(name, data, overlayRectHeader)
	if err != nil {
		return nil, err
	}
	out := &MenuOverlayRects{byEvent: make(map[string]overlayRect, len(rows)), extended: map[string]bool{}}
	for _, row := range rows {
		if row[0] == "" {
			return nil, fmt.Errorf("%s: event_key 不得為空", name)
		}
		if _, exists := out.byEvent[row[0]]; exists {
			return nil, fmt.Errorf("%s: 重複 event_key %q", name, row[0])
		}
		values := [8]int{}
		for i := range values {
			n, err := strconv.Atoi(row[i+1])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("%s: %s 的幾何欄無效", name, row[0])
			}
			values[i] = n
		}
		out.byEvent[row[0]] = overlayRect{values[0], values[1], values[2], values[3],
			values[4], values[5], values[6], values[7], row[9]}
	}
	if len(out.byEvent) == 0 {
		return nil, fmt.Errorf("%s: 安全矩形不得為空", name)
	}
	return out, nil
}

// LoadCharacterSheetOverlayRects allows only the two evidence-backed label
// rectangles whose right edge extends to (but never across) the dynamic value
// beginning at text column 35. All other events remain exact-width.
func LoadCharacterSheetOverlayRects(name string, data []byte) (*MenuOverlayRects, error) {
	out, err := LoadMenuOverlayRects(name, data)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"character.sheet.label.ac", "character.sheet.label.thac0"} {
		if _, ok := out.byEvent[key]; !ok {
			return nil, fmt.Errorf("%s: 缺少可擴張事件 %q", name, key)
		}
		out.extended[key] = true
	}
	return out, nil
}

func MergeMenuOverlayRects(catalogs ...*MenuOverlayRects) (*MenuOverlayRects, error) {
	out := &MenuOverlayRects{byEvent: map[string]overlayRect{}, extended: map[string]bool{}}
	for _, catalog := range catalogs {
		if catalog == nil {
			continue
		}
		for key, rect := range catalog.byEvent {
			if _, exists := out.byEvent[key]; exists {
				return nil, fmt.Errorf("安全矩形合併：重複 event_key %q", key)
			}
			out.byEvent[key] = rect
			out.extended[key] = catalog.extended[key]
		}
	}
	if len(out.byEvent) == 0 {
		return nil, fmt.Errorf("安全矩形合併：不得為空")
	}
	return out, nil
}

// RuntimeMenuOverlay owns only presentation state. It never writes machine VRAM.
type RuntimeMenuOverlay struct {
	layer   *xlate.Layer
	rects   *MenuOverlayRects
	font    *xlate.Font
	scale   int
	actions []MenuOverlayGeometry
}

func NewRuntimeMenuOverlay(rects *MenuOverlayRects, font *xlate.Font, scale int) (*RuntimeMenuOverlay, error) {
	if rects == nil || len(rects.byEvent) == 0 {
		return nil, fmt.Errorf("buckrogers: runtime 覆繪缺少安全矩形")
	}
	if font == nil || font.W != 16 || font.H != 16 {
		return nil, fmt.Errorf("buckrogers: runtime 覆繪字型必須是 16x16")
	}
	if scale != 2 && scale != 3 {
		return nil, fmt.Errorf("buckrogers: runtime 覆繪倍率必須明示為 2 或 3")
	}
	return &RuntimeMenuOverlay{layer: &xlate.Layer{W: 320, H: 200}, rects: rects, font: font, scale: scale}, nil
}

func (o *RuntimeMenuOverlay) Apply(event TextEvent, request DisplayRequest, palette [256][3]uint8) error {
	r, ok := o.rects.byEvent[request.EventKey]
	if !ok {
		return fmt.Errorf("buckrogers: %s 沒有安全矩形", request.EventKey)
	}
	exactWidth := int(event.OriginalLength) * 8
	validWidth := r.width == exactWidth
	if o.rects.extended[request.EventKey] {
		validWidth = r.width >= exactWidth && r.x+r.width == 35*8
	}
	if r.x != int(event.Column)*8 || r.y != int(event.Row)*8 || !validWidth || r.height != 8 {
		return fmt.Errorf("buckrogers: %s 安全矩形與 runtime event 幾何不符", request.EventKey)
	}
	overlay, err := BuildMenuOverlay([]MenuOverlayEntry{{
		EventKey: request.EventKey, TextKey: request.TextKey, Translation: request.Translation,
		Background: event.Background, Foreground: event.Foreground,
		X: r.x, Y: r.y, Width: r.width, Height: r.height, DrawX: r.drawX, DrawY: r.drawY,
		Capacity: r.capacity, LineCount: r.lines, Overflow: r.overflow,
	}}, o.font, palette, o.scale)
	if err != nil {
		return err
	}
	stamp := overlay.Layer.Stamps[0]
	stamp.State = xlate.Pending
	o.layer.Replace(stamp)
	o.actions = append(o.actions, overlay.Events[0])
	return nil
}

func logicalRGB(indexed []byte, palette [256][3]uint8) []byte {
	rgb := make([]byte, len(indexed)*3)
	for i, c := range indexed {
		copy(rgb[i*3:i*3+3], palette[c][:])
	}
	return rgb
}

func (o *RuntimeMenuOverlay) Frame(indexed []byte, palette [256][3]uint8) {
	o.layer.Frame(indexed, logicalRGB(indexed, palette))
}

// ClearTextCells 套用 026F:029C 已證實的包含端點文字格清除參數。
func (o *RuntimeMenuOverlay) ClearTextCells(bottom, right, top, left uint8) error {
	if bottom < top || right < left || bottom >= 25 || right >= 40 {
		return fmt.Errorf("buckrogers: 清除矩形無效 bottom=%d right=%d top=%d left=%d", bottom, right, top, left)
	}
	o.layer.Clear(int(left)*8, int(top)*8, (int(right)+1)*8, (int(bottom)+1)*8)
	return nil
}

func (o *RuntimeMenuOverlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune, bool) {
	rgba := ScaleIndexedRGBA(indexed, palette, o.scale)
	missing := []rune{}
	drew := o.layer.Draw(rgba, o.scale, func(r rune) { missing = append(missing, r) })
	return rgba, missing, drew
}

// ScaleIndexedRGBA creates the presentation baseline used for same-frame
// containment checks. It does not mutate the indexed framebuffer or palette.
func ScaleIndexedRGBA(indexed []byte, palette [256][3]uint8, scale int) []byte {
	rgba := make([]byte, 320*scale*200*scale*4)
	w := 320 * scale
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			c := palette[indexed[y*320+x]]
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					i := ((y*scale+dy)*w + x*scale + dx) * 4
					rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = c[0], c[1], c[2], 255
				}
			}
		}
	}
	return rgba
}

func (o *RuntimeMenuOverlay) Actions() []MenuOverlayGeometry {
	return append([]MenuOverlayGeometry(nil), o.actions...)
}

func (o *RuntimeMenuOverlay) ActiveKeys() []string {
	out := make([]string, len(o.layer.Stamps))
	for i, stamp := range o.layer.Stamps {
		out[i] = stamp.Key
	}
	return out
}
