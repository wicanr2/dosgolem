package buckrogers

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func page7MatrixPresenter(t *testing.T, scale int) *RuntimeStoryPage7Overlay {
	t.Helper()
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	text := make(map[string]string, 6)
	for i, r := range []rune("甲乙丙丁戊己") {
		font.Glyphs[r] = make([]byte, 32)
		text[fmt.Sprintf("story.page7.line.%03d", i+1)] = string(r)
	}
	o, err := NewRuntimeStoryPage7Overlay(text, font, scale)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func assertPage7MatrixBaseline(t *testing.T, scale int, w *StoryPage7Watcher) {
	t.Helper()
	if w.Active() || len(w.Events()) != 0 {
		t.Fatalf("watcher 不得 active/events: active=%v events=%d", w.Active(), len(w.Events()))
	}
	o := page7MatrixPresenter(t, scale)
	indexed := make([]byte, 320*200)
	palette := [256][3]uint8{}
	baseline := ScaleIndexedRGBA(indexed, palette, scale)
	got, missing, drew := o.Draw(indexed, palette)
	if len(o.ActiveKeys()) != 0 || drew || len(missing) != 0 || !bytes.Equal(got, baseline) {
		t.Fatalf("%dx 失敗案例必須是零 active、零 draw 且 RGBA=baseline: keys=%v drew=%v missing=%q equal=%v", scale, o.ActiveKeys(), drew, string(missing), bytes.Equal(got, baseline))
	}
}

func emitPage7MatrixGroup(w *StoryPage7Watcher, c *StoryPage7Catalog, raw [][]byte) {
	step := uint64(10)
	for i, line := range raw {
		for j, glyph := range line {
			emit7(w, c.entries[i], glyph, uint8(j+1), step)
			step += 2
		}
	}
}

// This matrix deliberately uses relative, monotonic runtime steps.  The exact
// measured steps remain TSV provenance; legal player Enter timing may vary.
func TestStoryPage7WatcherFailClosedMatrixBothScales(t *testing.T) {
	c, raw := p7(t)
	abiNames := [...]string{"mode", "glyph", "repeat", "background", "foreground/style", "row", "column"}
	type scenario struct {
		name string
		run  func(*StoryPage7Watcher)
	}
	var scenarios []scenario
	for i := 0; i < 7; i++ {
		i := i
		scenarios = append(scenarios,
			scenario{fmt.Sprintf("ABI high %s", abiNames[i]), func(w *StoryPage7Watcher) {
				a := [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}
				a[i] |= 0x100
				w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, a, 10)
			}},
			scenario{fmt.Sprintf("ABI low %s", abiNames[i]), func(w *StoryPage7Watcher) {
				a := [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}
				a[i] = (a[i] + 1) & 0xff
				w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, a, 10)
				if i == 1 {
					w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 11)
					emit7(w, c.entries[0], raw[0][1], 2, 12)
				}
			}},
		)
	}
	scenarios = append(scenarios,
		scenario{"caller", func(w *StoryPage7Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{1, 2}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
		}},
		scenario{"guard", func(w *StoryPage7Watcher) {
			w.ObserveGlyphEntry(Address{1, 2}, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
		}},
		scenario{"RETF previous address", func(w *StoryPage7Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{1, 2}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 11)
		}},
		scenario{"RETF opcode", func(w *StoryPage7Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xcb, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 11)
		}},
		scenario{"RETF caller", func(w *StoryPage7Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{1, 2}, 7, 9+storyGlyphStackDelta, 11)
		}},
		scenario{"SS", func(w *StoryPage7Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 8, 9+storyGlyphStackDelta, 11)
		}},
		scenario{"SP", func(w *StoryPage7Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta+1, 11)
		}},
		scenario{"return step equal", func(w *StoryPage7Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 10)
		}},
		scenario{"return step backward", func(w *StoryPage7Watcher) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}, 10)
			w.ObserveVerifiedGlyphReturn(Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta, 9)
		}},
		scenario{"next entry step equal", func(w *StoryPage7Watcher) {
			emit7(w, c.entries[0], raw[0][0], 1, 10)
			emit7(w, c.entries[0], raw[0][1], 2, 11)
		}},
		scenario{"next entry step backward", func(w *StoryPage7Watcher) {
			emit7(w, c.entries[0], raw[0][0], 1, 10)
			emit7(w, c.entries[0], raw[0][1], 2, 9)
		}},
		scenario{"last glyph hash", func(w *StoryPage7Watcher) {
			emit7(w, c.entries[0], raw[0][0], 1, 10)
			emit7(w, c.entries[0], raw[0][1]^1, 2, 12)
		}},
		scenario{"partial", func(w *StoryPage7Watcher) { emit7(w, c.entries[0], raw[0][0], 1, 10) }},
		scenario{"mixed", func(w *StoryPage7Watcher) {
			emit7(w, c.entries[0], raw[0][0], 1, 10)
			emit7(w, c.entries[1], raw[1][0], 1, 12)
		}},
		scenario{"duplicate", func(w *StoryPage7Watcher) {
			emit7(w, c.entries[0], raw[0][0], 1, 10)
			emit7(w, c.entries[0], raw[0][1], 2, 12)
			emit7(w, c.entries[0], raw[0][0], 1, 14)
		}},
		scenario{"restore discontinuity", func(w *StoryPage7Watcher) {
			emit7(w, c.entries[0], raw[0][0], 1, 10)
			w.ObserveExecutionDiscontinuity()
			emit7(w, c.entries[0], raw[0][1], 2, 12)
		}},
	)

	for _, scale := range []int{2, 3} {
		for _, tc := range scenarios {
			t.Run(fmt.Sprintf("%dx/%s", scale, tc.name), func(t *testing.T) {
				w, err := NewStoryPage7Watcher(c)
				if err != nil {
					t.Fatal(err)
				}
				tc.run(w)
				assertPage7MatrixBaseline(t, scale, w)
			})
		}
	}
}

func TestStoryPage7PrewriteMatrixAndActiveClear(t *testing.T) {
	c, raw := p7(t)
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			for _, tc := range []struct {
				name          string
				at            Address
				es, di, count uint16
			}{
				{"unknown instruction", Address{1, 2}, storyVideoSegment, 136*320 + 8, 1},
				{"unknown segment", storyFillInstruction, 0xb800, 136*320 + 8, 1},
				{"disjoint left", storyFillInstruction, storyVideoSegment, 137 * 320, 8},
				{"disjoint bottom", storyFillInstruction, storyVideoSegment, 184*320 + 8, 1},
			} {
				w, _ := NewStoryPage7Watcher(c)
				emit7(w, c.entries[0], raw[0][0], 1, 10)
				if w.ObserveVideoWrite(tc.at, tc.es, tc.di, tc.count) || w.p == nil {
					t.Fatalf("%s 不得清除 pending", tc.name)
				}
			}

			w, _ := NewStoryPage7Watcher(c)
			emit7(w, c.entries[0], raw[0][0], 1, 10)
			if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 136*320+8, 1) || w.p != nil || w.Active() {
				t.Fatal("相交 prewrite 必須清除 pending")
			}

			w, _ = NewStoryPage7Watcher(c)
			emitPage7MatrixGroup(w, c, raw)
			if !w.Active() {
				t.Fatal("前置條件：完整 group 未 active")
			}
			o := page7MatrixPresenter(t, scale)
			if err := o.Apply(w.Events(), [256][3]uint8{}); err != nil || len(o.ActiveKeys()) != 6 {
				t.Fatalf("前置條件：presenter apply=%v keys=%v", err, o.ActiveKeys())
			}
			if !w.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 136*320+319, 2) {
				t.Fatal("跨列相交 prewrite 未清除 watcher")
			}
			o.Clear()
			indexed := make([]byte, 320*200)
			palette := [256][3]uint8{}
			baseline := ScaleIndexedRGBA(indexed, palette, scale)
			got, missing, drew := o.Draw(indexed, palette)
			if w.Active() || len(w.Events()) != 0 || len(o.ActiveKeys()) != 0 || drew || len(missing) != 0 || !bytes.Equal(got, baseline) {
				t.Fatalf("active→prewrite 必須同步清除：watcher=%v events=%d keys=%v drew=%v equal=%v", w.Active(), len(w.Events()), o.ActiveKeys(), drew, bytes.Equal(got, baseline))
			}
		})
	}
}

func TestStoryPage7ActiveRestoreDiscontinuityClearsOverlay(t *testing.T) {
	c, raw := p7(t)
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			w, _ := NewStoryPage7Watcher(c)
			emitPage7MatrixGroup(w, c, raw)
			o := page7MatrixPresenter(t, scale)
			if err := o.Apply(w.Events(), [256][3]uint8{}); err != nil {
				t.Fatal(err)
			}
			w.ObserveExecutionDiscontinuity()
			o.Clear()
			indexed := make([]byte, 320*200)
			palette := [256][3]uint8{}
			baseline := ScaleIndexedRGBA(indexed, palette, scale)
			got, missing, drew := o.Draw(indexed, palette)
			if w.Active() || len(w.Events()) != 0 || len(o.ActiveKeys()) != 0 || drew || len(missing) != 0 || !bytes.Equal(got, baseline) {
				t.Fatalf("restore/discontinuity 必須清除所有衍生狀態: active=%v events=%d keys=%v drew=%v equal=%v", w.Active(), len(w.Events()), o.ActiveKeys(), drew, bytes.Equal(got, baseline))
			}
		})
	}
}

func TestStoryPage7CatalogAndFontFailClosedMatrix(t *testing.T) {
	events := "event_key\tsequence\toriginal_length\toriginal_sha256\tcaller\tglyph_guard\tbackground\tforeground\trow\tcolumn\tentry_step\tpost_call_step\tevidence_level\tcatalog_status\n"
	text := "key\ttranslation\tsource\n"
	for i, approved := range storyPage7Approved {
		key := fmt.Sprintf("story.page7.line.%03d", i+1)
		events += fmt.Sprintf("%s\t%d\t%d\t%s\t0763:04FF\t0763:026B\t0\t10\t%d\t1\t%d\t%d\tconfirmed\tREADY\n", key, i+1, approved.n, approved.h, 17+i, approved.entry, approved.post)
		text += key + "\t甲\truntime-editorial\n"
	}
	for _, tc := range []struct {
		name, events, translations string
	}{
		{"DRAFT catalog", strings.Replace(events, "READY", "DRAFT", 1), text},
		{"missing translation", events, strings.Replace(text, "story.page7.line.006\t甲\truntime-editorial\n", "", 1)},
		{"translation provenance", events, strings.Replace(text, "runtime-editorial", "manual", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := LoadStoryPage7Catalog("events", []byte(tc.events), "translations", []byte(tc.translations)); err == nil {
				t.Fatal("非 READY catalog/譯文必須失敗即關閉")
			}
		})
	}
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx missing glyph", scale), func(t *testing.T) {
			translations := make(map[string]string, 6)
			for i := 1; i <= 6; i++ {
				translations[fmt.Sprintf("story.page7.line.%03d", i)] = "甲"
			}
			if _, err := NewRuntimeStoryPage7Overlay(translations, &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, scale); err == nil {
				t.Fatal("缺字必須失敗即關閉")
			}
		})
	}
}
