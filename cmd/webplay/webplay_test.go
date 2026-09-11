package main

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func testSession(t *testing.T) *session {
	t.Helper()
	m := machine.New()
	d := dos.New(m, t.TempDir())
	d.Install()
	return &session{m: m, d: d, input: "bios", realtime: true}
}

// 按鍵對照（`docs/spec/200-webplay` §5）：方向鍵沒有 ASCII，字母帶 key 的字元，Enter 是 0Dh。
func TestBrowserKey(t *testing.T) {
	for _, tc := range []struct {
		code, key   string
		scan, ascii uint8
	}{
		{"ArrowLeft", "ArrowLeft", 0x4B, 0},
		{"ArrowUp", "ArrowUp", 0x48, 0},
		{"KeyX", "x", 0x2D, 'x'},
		{"KeyP", "P", 0x19, 'P'},
		{"Space", " ", 0x39, ' '},
		{"Enter", "Enter", 0x1C, 0x0D},
		{"Escape", "Escape", 0x01, 0x1B},
	} {
		k, ok := browserKey(tc.code, tc.key)
		if !ok || k.Scan != tc.scan || k.ASCII != tc.ascii {
			t.Errorf("%s：得到 %02X/%02X ok=%v，預期 %02X/%02X", tc.code, k.Scan, k.ASCII, ok, tc.scan, tc.ascii)
		}
	}
	if _, ok := browserKey("MetaLeft", "Meta"); ok {
		t.Error("不在表裡的鍵應該回 false")
	}
}

// /frame 回目前模式大小的 PNG；帶上同一個雜湊時回 204。
func TestFrameEndpoint(t *testing.T) {
	s := testSession(t)
	s.m.SetVideoMode(0x12)
	srv := httptest.NewServer(s.routes())
	defer srv.Close()

	r, err := http.Get(srv.URL + "/frame")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r.Body)
	r.Body.Close()
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatalf("不是 PNG：%v", err)
	}
	if b := img.Bounds(); b.Dx() != 640 || b.Dy() != 480 {
		t.Fatalf("mode 12h 的畫面是 %dx%d，預期 640x480", b.Dx(), b.Dy())
	}
	tag := r.Header.Get("X-Frame")
	r2, err := http.Get(srv.URL + "/frame?have=" + tag)
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusNoContent {
		t.Fatalf("同一畫面帶 have 應回 204，得到 %d", r2.StatusCode)
	}
}

// /key 的按下排進 BIOS 佇列；放開不送（-input bios）。
func TestKeyEndpointQueuesBIOSKey(t *testing.T) {
	s := testSession(t)
	srv := httptest.NewServer(s.routes())
	defer srv.Close()
	before := s.d.KeysPending()
	for _, body := range []string{
		`{"code":"ArrowLeft","key":"ArrowLeft","down":true}`,
		`{"code":"ArrowLeft","key":"ArrowLeft","down":false}`,
	} {
		r, err := http.Post(srv.URL+"/key", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
	}
	if got := s.d.KeysPending() - before; got != 1 {
		t.Fatalf("BIOS 佇列多了 %d 個鍵，預期 1", got)
	}
}

// 調降 IRQ0Base 之後，目前分頻下的計時器間隔照比例變小（`docs/spec/200-webplay` §4）。
func TestLowerIRQ0BaseShortensInterval(t *testing.T) {
	s := testSession(t)
	before := s.m.IRQ0Every
	s.m.IRQ0Base /= 2
	s.m.RecalcIRQ0()
	if s.m.IRQ0Every >= before || s.m.IRQ0Every < before/2-1 {
		t.Fatalf("IRQ0Every 從 %d 變成 %d，預期約為一半", before, s.m.IRQ0Every)
	}
}
