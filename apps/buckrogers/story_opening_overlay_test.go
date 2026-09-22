package buckrogers

import (
	"github.com/wicanr2/dosgolem/xlate"
	"testing"
)

func TestRuntimeStoryOpeningOverlayAtomicAndClear(t *testing.T) {
	f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for _, r := range []rune("甲乙丙丁戊") {
		f.Glyphs[r] = make([]byte, 32)
		f.Glyphs[r][0] = 0x80
	}
	text := map[string]string{}
	ev := make([]StoryOpeningEvent, 5)
	for i := range ev {
		k := string(rune('a' + i))
		text[k] = string([]rune("甲乙丙丁戊")[i])
		ev[i] = StoryOpeningEvent{EventKey: k, Row: uint8(17 + i), Column: 1}
	}
	o, e := NewRuntimeStoryOpeningOverlay(text, f, 3)
	if e != nil {
		t.Fatal(e)
	}
	layer := o.PresentationLayer()
	if layer == nil {
		t.Fatal("presentation layer nil")
	}
	if e = o.Apply(ev, [256][3]uint8{}); e != nil {
		t.Fatal(e)
	}
	out, miss, d := o.Draw(make([]byte, 320*200), [256][3]uint8{})
	if !d || len(miss) != 0 || len(out) != 320*200*9*4 {
		t.Fatal("draw")
	}
	o.Clear()
	if o.PresentationLayer() != layer {
		t.Fatal("Clear replaced presentation layer")
	}
	if len(layer.Stamps) != 0 {
		t.Fatal("Clear retained story stamps")
	}
	_, _, d = o.Draw(make([]byte, 320*200), [256][3]uint8{})
	if d {
		t.Fatal("clear")
	}
}
