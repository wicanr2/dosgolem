package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/host"
)

func newManualSnapshotOwnerFixture(t *testing.T, scale int) (*ManualSnapshotOwner, []ManualPresentationEvent, host.IndexedFrame) {
	t.Helper()
	catalog := manualOverlayCatalog("繁中段落")
	font := manualOverlayFont(catalog)
	owner, err := NewManualSnapshotOwner(loadManualOverlayLayout(t), catalog, font, scale)
	if err != nil {
		t.Fatal(err)
	}
	if font.Name != "" {
		t.Fatal("owner must not name or mutate caller font")
	}
	events := []ManualPresentationEvent{
		{Kind: ManualPresentationBegin, Generation: 7},
		{Kind: ManualPresentationRequest, Generation: 7, Request: manualOverlayRequest(catalog, 7)},
	}
	if n, err := owner.Consume(events); err != nil || n != 2 {
		t.Fatalf("consume=%d err=%v", n, err)
	}
	palette, indexed := manualOverlayPaletteAndFrame()
	return owner, events, host.IndexedFrame{Canvas: host.Canvas{Width: 320, Height: 200}, Indexed: indexed, Palette: palette}
}

func TestManualSnapshotOwnerE1TwoFontsAndFailClosedLifecycle(t *testing.T) {
	text := "RAM繁中"
	catalog := manualOverlayCatalog(text)
	base, _ := manualE1SyntheticFonts(text)
	newOwner := func(t *testing.T) (*ManualSnapshotOwner, []ManualPresentationEvent, host.IndexedFrame) {
		t.Helper()
		owner, err := NewManualSnapshotOwner(loadManualOverlayLayout(t), catalog, base, 3)
		if err != nil {
			t.Fatal(err)
		}
		events := []ManualPresentationEvent{
			{Kind: ManualPresentationBegin, Generation: 7},
			{Kind: ManualPresentationRequest, Generation: 7, Request: manualOverlayRequest(catalog, 7)},
		}
		if n, err := owner.Consume(events); err != nil || n != 2 {
			t.Fatalf("E1 consume=%d err=%v", n, err)
		}
		palette, indexed := manualOverlayPaletteAndFrame()
		return owner, events, host.IndexedFrame{Canvas: host.Canvas{Width: 320, Height: 200}, Indexed: indexed, Palette: palette}
	}
	owner, events, frame := newOwner(t)
	glyphs := owner.overlay.text.Stamps[0].PixelGlyphs
	if len(glyphs) != len([]rune(text)) || glyphs[0].Font != owner.base || glyphs[1].Font != owner.base ||
		glyphs[2].Font != owner.base || glyphs[3].Font != owner.overlay.font {
		t.Fatalf("E1 two-font glyph route changed: %#v", glyphs)
	}
	ticket, err := owner.PrepareFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if shot, err := owner.Snapshot(ticket, 3); err != nil || !shot.Drew || len(shot.RGBA) == 0 {
		t.Fatalf("E1 sealed two-font projection failed: err=%v", err)
	}
	if shot, err := owner.Snapshot(ticket, 2); err == nil || len(shot.RGBA) != 0 {
		t.Fatalf("E1 cross-scale ticket accepted: err=%v", err)
	}
	owner.overlay.e1Plan.lines[0].tokens[0].glyphs[0].x++
	if shot, err := owner.Snapshot(ticket, 3); err == nil || len(shot.RGBA) != 0 {
		t.Fatalf("changed E1 plan accepted: err=%v", err)
	}
	owner.overlay.e1Plan.lines[0].tokens[0].glyphs[0].x--
	if shot, err := owner.Snapshot(ticket, 3); err == nil || len(shot.RGBA) != 0 {
		t.Fatalf("repaired E1 plan revived failed ticket: err=%v", err)
	}

	owner, _, frame = newOwner(t)
	ticket, err = owner.PrepareFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	owner.base.Glyphs['R'][0] ^= 0xff
	if shot, err := owner.Snapshot(ticket, 3); err == nil || len(shot.RGBA) != 0 {
		t.Fatalf("changed base font accepted: err=%v", err)
	}
	owner.base.Glyphs['R'][0] ^= 0xff
	if shot, err := owner.Snapshot(ticket, 3); err == nil || len(shot.RGBA) != 0 {
		t.Fatalf("repaired E1 font revived failed ticket: err=%v", err)
	}

	owner, _, frame = newOwner(t)
	ticket, err = owner.PrepareFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	owner.overlay.e1Base = nil
	if shot, err := owner.Snapshot(ticket, 3); err == nil || len(shot.RGBA) != 0 {
		t.Fatalf("missing E1 base binding accepted: err=%v", err)
	}
	owner.overlay.e1Base = owner.base
	if shot, err := owner.Snapshot(ticket, 3); err == nil || len(shot.RGBA) != 0 {
		t.Fatalf("restored E1 binding revived failed ticket: err=%v", err)
	}

	owner, events, frame = newOwner(t)
	ticket, err = owner.PrepareFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.SetStyle(ManualTextStyle{Background: 10, Foreground: 15}); err != nil {
		t.Fatal(err)
	}
	if owner.overlay.e1Plan != nil {
		t.Fatal("SetStyle left the prior E1 plan visible")
	}
	if shot, err := owner.Snapshot(ticket, 3); err == nil || len(shot.RGBA) != 0 {
		t.Fatalf("pre-SetStyle ticket accepted: err=%v", err)
	}
	events = append(events, ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 8})
	request := manualOverlayRequest(catalog, 8)
	events = append(events, ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 8, Request: request})
	if n, err := owner.Consume(events); err != nil || n != 2 {
		t.Fatalf("new E1 generation after SetStyle failed: consumed=%d err=%v", n, err)
	}
	if shot, err := owner.Snapshot(ticket, 3); err == nil || len(shot.RGBA) != 0 {
		t.Fatalf("new request revived older ticket: err=%v", err)
	}
	if _, err := owner.PrepareFrame(frame); err != nil {
		t.Fatal(err)
	}
}

func TestManualSnapshotOwnerTwoXGoldenBytes(t *testing.T) {
	owner, _, frame := newManualSnapshotOwnerFixture(t, 2)
	ticket, err := owner.PrepareFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	shot, err := owner.Snapshot(ticket, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", ticket.layersHash); got != "ac0039f0c06a31aeb212c6f7128f5cb5d3eca732d1c7ff6d3f9d1c88c8c78b45" {
		t.Fatalf("2× layer Snapshot bytes changed: %s", got)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(shot.RGBA)); got != "0f53dae055734ba39d6677dedfda25d4d23c061c8c21c60b933f54256f108b6a" {
		t.Fatalf("2× RGBA bytes changed: %s", got)
	}
}

func TestManualSnapshotOwnerComposesSealed2x3x(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			owner, _, frame := newManualSnapshotOwnerFixture(t, scale)
			wantName := manualBaseFontIdentity
			if scale == 3 {
				wantName = manualDerivedFontIdentity
			}
			if scale == 2 {
				if got := owner.overlay.text.Stamps[0].Font.Name; got != wantName {
					t.Fatalf("font name before stamp Snapshot=%q want %q", got, wantName)
				}
			} else {
				stamp := owner.overlay.text.Stamps[0]
				if stamp.Font != nil || len(stamp.Text) != 0 || stamp.PixelScale != 3 || len(stamp.PixelGlyphs) == 0 ||
					owner.overlay.e1Plan == nil || len(owner.overlay.text.Stamps) != 14 ||
					owner.overlay.e1Plan.baseName != manualBaseFontIdentity || owner.overlay.e1Plan.derivedName != wantName {
					t.Fatal("E1 physical stamp or two-font plan identity missing")
				}
				for _, glyph := range stamp.PixelGlyphs {
					if glyph.Font != owner.overlay.font || glyph.Font.Name != wantName {
						t.Fatalf("E1 CJK glyph does not use sealed derived font: %#v", glyph)
					}
				}
			}
			ticket, err := owner.PrepareFrame(frame)
			if err != nil {
				t.Fatal(err)
			}
			expected, missing, drew := owner.overlay.Draw(frame.Indexed, frame.Palette)
			if !drew || len(missing) != 0 {
				t.Fatalf("formal draw=%v missing=%q", drew, string(missing))
			}
			// The session owns a copy, not the caller's mutable indexed slice.
			frame.Indexed[0] ^= 0xff
			actual, err := owner.Snapshot(ticket, scale)
			if err != nil {
				t.Fatal(err)
			}
			if !actual.Drew || len(actual.Missing) != 0 || !bytes.Equal(actual.RGBA, expected) {
				t.Fatal("sealed background→text differs from formal presenter")
			}
			if len(actual.RGBA) != 320*200*scale*scale*4 {
				t.Fatalf("RGBA length=%d", len(actual.RGBA))
			}
		})
	}
}

func TestManualSnapshotOwnerRejectsInvalidSourceAndStaleTicket(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			owner, events, frame := newManualSnapshotOwnerFixture(t, scale)
			original := owner.overlay.text.Stamps[0]
			owner.overlay.text.Stamps[0] = nil
			if ticket, err := owner.PrepareFrame(frame); err == nil || ticket.owner != nil {
				t.Fatalf("nil original stamp accepted: ticket=%#v err=%v", ticket, err)
			}
			owner.overlay.text.Stamps[0] = original

			owner, events, frame = newManualSnapshotOwnerFixture(t, scale)
			ticket, err := owner.PrepareFrame(frame)
			if err != nil {
				t.Fatal(err)
			}
			for _, glyph := range owner.overlay.font.Glyphs {
				glyph[0] ^= 0xff // Name and dimensions remain legal.
				break
			}
			if shot, err := owner.Snapshot(ticket, scale); err == nil || len(shot.RGBA) != 0 {
				t.Fatalf("same-name font mutation accepted: err=%v rgba=%d", err, len(shot.RGBA))
			}

			owner, events, frame = newManualSnapshotOwnerFixture(t, scale)
			ticket, err = owner.PrepareFrame(frame)
			if err != nil {
				t.Fatal(err)
			}
			events = append(events, ManualPresentationEvent{Kind: ManualPresentationClear, Generation: 7})
			if n, err := owner.Consume(events); err != nil || n != 1 {
				t.Fatalf("clear consume=%d err=%v", n, err)
			}
			if shot, err := owner.Snapshot(ticket, scale); err == nil || len(shot.RGBA) != 0 {
				t.Fatalf("stale post-Clear ticket accepted: err=%v rgba=%d", err, len(shot.RGBA))
			}

			owner, _, frame = newManualSnapshotOwnerFixture(t, scale)
			ticket, err = owner.PrepareFrame(frame)
			if err != nil {
				t.Fatal(err)
			}
			newTicket, err := owner.PrepareFrame(frame)
			if err != nil {
				t.Fatal(err)
			}
			if shot, err := owner.Snapshot(newTicket, scale); err != nil || len(shot.RGBA) == 0 {
				t.Fatalf("latest frame ticket rejected before stale attempt: err=%v rgba=%d", err, len(shot.RGBA))
			}
			if shot, err := owner.Snapshot(ticket, scale); err == nil || len(shot.RGBA) != 0 {
				t.Fatalf("previous frame ticket accepted: err=%v rgba=%d", err, len(shot.RGBA))
			}
			shot, err := owner.Snapshot(newTicket, scale)
			if scale == 3 {
				if err == nil || len(shot.RGBA) != 0 {
					t.Fatalf("failed E1 Snapshot did not retire current ticket: err=%v rgba=%d", err, len(shot.RGBA))
				}
			} else if err != nil || len(shot.RGBA) == 0 {
				t.Fatalf("2× latest frame ticket changed after stale attempt: err=%v rgba=%d", err, len(shot.RGBA))
			}

			owner, _, frame = newManualSnapshotOwnerFixture(t, scale)
			ticket, err = owner.PrepareFrame(frame)
			if err != nil {
				t.Fatal(err)
			}
			owner.Invalidate()
			if shot, err := owner.Snapshot(ticket, scale); err == nil || len(shot.RGBA) != 0 {
				t.Fatalf("post-Restore/stop ticket accepted: err=%v rgba=%d", err, len(shot.RGBA))
			}
		})
	}
}

func TestManualSnapshotOwnerConsumeErrorRetiresVisiblePrefix(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			catalog := manualOverlayCatalog("繁中段落")
			owner, err := NewManualSnapshotOwner(loadManualOverlayLayout(t), catalog, manualOverlayFont(catalog), scale)
			if err != nil {
				t.Fatal(err)
			}
			events := []ManualPresentationEvent{
				{Kind: ManualPresentationBegin, Generation: 7},
				{Kind: ManualPresentationRequest, Generation: 7, Request: manualOverlayRequest(catalog, 7)},
				{Kind: "invalid", Generation: 7},
			}
			if n, err := owner.Consume(events); err == nil || n != 2 || owner.active {
				t.Fatalf("partial consumer failure: consumed=%d err=%v active=%v", n, err, owner.active)
			}
			palette, indexed := manualOverlayPaletteAndFrame()
			frame := host.IndexedFrame{Canvas: host.Canvas{Width: 320, Height: 200}, Indexed: indexed, Palette: palette}
			if ticket, err := owner.PrepareFrame(frame); err == nil || ticket.owner != nil {
				t.Fatalf("visible prefix rendered after Consume error: ticket=%#v err=%v", ticket, err)
			}

			owner, validEvents, frame := newManualSnapshotOwnerFixture(t, scale)
			ticket, err := owner.PrepareFrame(frame)
			if err != nil {
				t.Fatal(err)
			}
			if n, err := owner.Consume(validEvents[:1]); err == nil || n != 0 || owner.active {
				t.Fatalf("short history did not retire owner: consumed=%d err=%v active=%v", n, err, owner.active)
			}
			if shot, err := owner.Snapshot(ticket, scale); err == nil || len(shot.RGBA) != 0 {
				t.Fatalf("prior ticket accepted after invalid Consume: err=%v rgba=%d", err, len(shot.RGBA))
			}
		})
	}
}

func TestManualSnapshotOwnerRejectsRowIdentityAndPostFrameInkDrift(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			owner, _, frame := newManualSnapshotOwnerFixture(t, scale)
			owner.overlay.text.Stamps[0].Key = "manual.wrong.row"
			if ticket, err := owner.PrepareFrame(frame); err == nil || ticket.owner != nil {
				t.Fatalf("wrong row key accepted: err=%v", err)
			}

			owner, _, frame = newManualSnapshotOwnerFixture(t, scale)
			owner.overlay.text.Stamps[0].Text = []rune("錯")
			if ticket, err := owner.PrepareFrame(frame); err == nil || ticket.owner != nil {
				t.Fatalf("wrong row text accepted: err=%v", err)
			}

			owner, _, frame = newManualSnapshotOwnerFixture(t, scale)
			ticket, err := owner.PrepareFrame(frame)
			if err != nil {
				t.Fatal(err)
			}
			owner.overlay.text.Stamps[0].FG[0] ^= 0xff
			if shot, err := owner.Snapshot(ticket, scale); err == nil || len(shot.RGBA) != 0 {
				t.Fatalf("post-Frame ink/color drift accepted: err=%v rgba=%d", err, len(shot.RGBA))
			}
		})
	}
}
