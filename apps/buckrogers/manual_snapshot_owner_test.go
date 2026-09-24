package buckrogers

import (
	"bytes"
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

func TestManualSnapshotOwnerComposesSealed2x3x(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			owner, _, frame := newManualSnapshotOwnerFixture(t, scale)
			wantName := manualBaseFontIdentity
			if scale == 3 {
				wantName = manualDerivedFontIdentity
			}
			if got := owner.overlay.text.Stamps[0].Font.Name; got != wantName {
				t.Fatalf("font name before stamp Snapshot=%q want %q", got, wantName)
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
			if shot, err := owner.Snapshot(ticket, scale); err == nil || len(shot.RGBA) != 0 {
				t.Fatalf("previous frame ticket accepted: err=%v rgba=%d", err, len(shot.RGBA))
			}
			if shot, err := owner.Snapshot(newTicket, scale); err != nil || len(shot.RGBA) == 0 {
				t.Fatalf("latest frame ticket rejected: err=%v rgba=%d", err, len(shot.RGBA))
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
