package phantasie

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/wicanr2/dosgolem/oracle"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestArtCollectionNormalRoutes(t *testing.T) {
	root, dir, project, data, out := os.Getenv("PHANTASIE_ORIGINAL"), os.Getenv("PHANTASIE_HD_DIR"), os.Getenv("PHANTASIE_PROJECT"), os.Getenv("PHANTASIE_DATA"), os.Getenv("PHANTASIE_HD_OUTPUT")
	if root == "" || dir == "" || project == "" || data == "" || out == "" {
		t.Skip("local route inputs absent")
	}
	art, err := LoadTownArt(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	type receipt struct {
		Route, Check, Language, Memory string
		Steps, Reads                   uint64
		VRAM                           uint64
		Changed                        int
	}
	receipts := []receipt{}
	battleChanged := 0
	for _, name := range []string{"town", "guild", "combat"} {
		raw, err := os.ReadFile(filepath.Join(project, "tests/routes", name+".route"))
		if err != nil {
			t.Fatal(err)
		}
		route, err := ParseRoute(string(raw))
		if err != nil {
			t.Fatal("public route invalid")
		}
		s, err := StartSession(SessionOptions{Root: root, TextDir: filepath.Join(data, "text"), FontDir: filepath.Join(data, "font"), Langs: []string{"zh-TW", "zh-CN", "ja", "ko"}, Scratch: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		if len(s.Failed) > 0 {
			t.Fatal(s.Failed)
		}
		// RE029 independently identifies the original draw consumer at image:9D40.
		// Observe its actual normal-route rectangles without writing guest state.
		battleRects := map[image.Rectangle]bool{}
		s.O.OnCall(oracle.Far(s.Img, 0x9D40), func(o *oracle.Oracle) {
			x, y, h, w, mode := int(o.StackWord(1)), int(o.StackWord(2)), int(o.StackWord(3)), int(o.StackWord(4)), o.StackWord(6)
			y = y / 2 * 2
			if x < 0 || y < 0 || w < 1 || h < 1 || x+w > 320 || y+h > 151 {
				return
			}
			r := image.Rect(x, y, x+w, y+h)
			if mode == 0xffff || mode == 2 {
				delete(battleRects, r)
			} else {
				battleRects[r] = true
			}
		})
		for _, st := range route {
			switch st.Kind {
			case RouteKey:
				if st.Wait > 0 {
					s.Gate.PressAfterReads(st.Wait, st.Key)
				} else {
					s.Gate.Press(0, st.Key)
				}
				continue
			case RouteLang:
				if err := s.Ov.SetDisplay(st.Name); err != nil {
					t.Fatal(err)
				}
				continue
			case RouteAssert:
				continue
			}
			cond := oracle.NewCond("art-normal-stop", func(*oracle.Oracle) bool { return s.Gate.Pending() == 0 && s.Gate.Reads > s.Gate.LastSent })
			if st.Kind == RouteSnap {
				cond = oracle.NewCond("art-snap", func(*oracle.Oracle) bool { return s.Gate.Pending() == 0 })
			}
			if err := s.O.RunUntil(cond, oracle.Budget(400_000_000)); err != nil && !cond.Ready(s.O) {
				t.Fatal(err)
			}
			if st.Kind == RouteSnap {
				target := s.O.Steps() + st.Steps
				cond = oracle.NewCond("art-target", func(*oracle.Oracle) bool { return s.O.Steps() >= target })
				if err := s.O.RunUntil(cond, oracle.Budget(st.Steps)); err != nil && !cond.Ready(s.O) {
					t.Fatal(err)
				}
			}
			idx, rgb := s.Frame()
			mem := sha256.Sum256(s.O.Bytes(oracle.Far(0, 0), 1<<20))
			steps, reads, vram := s.O.Steps(), s.Gate.Reads, VramHash(s.O)
			display := s.Ov.Display()
			for _, lang := range []string{"zh-TW", "zh-CN", "en", "ja", "ko"} {
				if err := s.Ov.SetDisplay(lang); err != nil {
					t.Fatal(err)
				}
				idx, rgb = s.Frame()
				orig, hd := make([]byte, 640*400*4), make([]byte, 640*400*4)
				ComposeInto(orig, s.Ov, idx, rgb, 2, nil)
				ComposePresentationInto(hd, s.Ov, idx, rgb, "original", art)
				if !bytes.Equal(orig, hd) {
					t.Fatal("original changed")
				}
				ComposePresentationInto(hd, s.Ov, idx, rgb, "hd", art)
				changed := 0
				townPage := false
				for _, page := range art.pages {
					if page.rect == image.Rect(0, 8, 320, 192) {
						_, townPage = page.Match(idx)
						break
					}
				}
				for i := 0; i < len(hd); i += 4 {
					if bytes.Equal(hd[i:i+4], orig[i:i+4]) {
						continue
					}
					changed++
					pt := image.Pt(i/4%640/2, i/4/640/2)
					allowed := townPage && pt.In(image.Rect(0, 8, 320, 192))
					if name == "combat" {
						for r := range battleRects {
							allowed = allowed || pt.In(r)
						}
					}
					if !allowed {
						for pi, page := range art.pages {
							pr, ok := page.Match(idx)
							t.Logf("page %d match=%v protected=%v", pi, ok, pr)
						}
						matches, ok := art.spriteMatches(idx)
						t.Logf("sprites=%v ok=%v", matches, ok)
						t.Fatalf("HD outside sampled art: %s %s %v", name, st.Name, pt)
					}
				}
				if name == "combat" && !townPage {
					battleChanged += changed
				}
				if mem != sha256.Sum256(s.O.Bytes(oracle.Far(0, 0), 1<<20)) || steps != s.O.Steps() || reads != s.Gate.Reads || vram != VramHash(s.O) {
					t.Fatal("presentation changed original state")
				}
				receipts = append(receipts, receipt{name, st.Name, lang, hex.EncodeToString(mem[:]), steps, reads, vram, changed})
				if lang == "zh-TW" && changed > 0 && (name == "town" && st.Name == "town" || name == "combat" && !townPage) {
					file := filepath.Join(out, name+"-"+st.Name+"-hd.png")
					f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, &image.RGBA{Pix: hd, Stride: 640 * 4, Rect: image.Rect(0, 0, 640, 400)})
					f.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := s.Ov.SetDisplay(display); err != nil {
				t.Fatal(err)
			}
			s.Frame()
		}
		s.Close()
	}
	if battleChanged == 0 {
		t.Fatal("normal combat never displayed HD")
	}
	b, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(out, "normal-route-receipts.json"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d route/language samples; %d changed combat pixels", len(receipts), battleChanged)
}
