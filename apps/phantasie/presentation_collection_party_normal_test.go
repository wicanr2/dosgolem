package phantasie

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// Local sampled normal-route evidence; independent PNG mask audit is external.
func TestPartyHDNormalRoutes(t *testing.T) {
	root, dir, project, data, out := os.Getenv("PHANTASIE_ORIGINAL"), os.Getenv("PHANTASIE_PARTY_HD_DIR"), os.Getenv("PHANTASIE_PROJECT"), os.Getenv("PHANTASIE_DATA"), os.Getenv("PHANTASIE_PARTY_OUTPUT")
	if root == "" || dir == "" || project == "" || data == "" || out == "" {
		t.Skip("local party HD normal-route inputs absent")
	}
	art, err := LoadTownArt(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(project, "tests/routes/combat.route"))
	if err != nil {
		t.Fatal(err)
	}
	route, err := ParseRoute(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	s, err := StartSession(SessionOptions{Root: root, TextDir: filepath.Join(data, "text"), FontDir: filepath.Join(data, "font"), Langs: []string{"zh-TW", "zh-CN", "ja", "ko"}, Scratch: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if len(s.Failed) > 0 {
		t.Fatal(s.Failed)
	}
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
	type receipt struct {
		Check, Language, Memory     string
		Steps, Reads                uint64
		VRAM                        uint64
		MonsterRects                [][4]int
		OriginalPNG, HDPNG, Indexed string
	}
	receipts := []receipt{}
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
		cond := oracle.NewCond("party-normal-stop", func(*oracle.Oracle) bool { return s.Gate.Pending() == 0 && s.Gate.Reads > s.Gate.LastSent })
		if st.Kind == RouteSnap {
			cond = oracle.NewCond("party-snap", func(*oracle.Oracle) bool { return s.Gate.Pending() == 0 })
		}
		if err := s.O.RunUntil(cond, oracle.Budget(400_000_000)); err != nil && !cond.Ready(s.O) {
			t.Fatal(err)
		}
		if st.Kind == RouteSnap {
			target := s.O.Steps() + st.Steps
			cond = oracle.NewCond("party-target", func(*oracle.Oracle) bool { return s.O.Steps() >= target })
			if err := s.O.RunUntil(cond, oracle.Budget(st.Steps)); err != nil && !cond.Ready(s.O) {
				t.Fatal(err)
			}
		}
		mem := sha256.Sum256(s.O.Bytes(oracle.Far(0, 0), 1<<20))
		steps, reads, vram := s.O.Steps(), s.Gate.Reads, VramHash(s.O)
		display := s.Ov.Display()
		for _, lang := range []string{"zh-TW", "zh-CN", "en", "ja", "ko"} {
			if err := s.Ov.SetDisplay(lang); err != nil {
				t.Fatal(err)
			}
			idx, rgb := s.Frame()
			orig, hd := make([]byte, 640*400*4), make([]byte, 640*400*4)
			ComposeInto(orig, s.Ov, idx, rgb, 2, nil)
			ComposePresentationInto(hd, s.Ov, idx, rgb, "original", art)
			if !bytes.Equal(orig, hd) {
				t.Fatal("original changed")
			}
			ComposePresentationInto(hd, s.Ov, idx, rgb, "hd", art)
			if mem != sha256.Sum256(s.O.Bytes(oracle.Far(0, 0), 1<<20)) || steps != s.O.Steps() || reads != s.Gate.Reads || vram != VramHash(s.O) {
				t.Fatal("presentation changed original state")
			}
			r := receipt{Check: st.Name, Language: lang, Memory: hex.EncodeToString(mem[:]), Steps: steps, Reads: reads, VRAM: vram, MonsterRects: [][4]int{}}
			for rect := range battleRects {
				r.MonsterRects = append(r.MonsterRects, [4]int{rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy()})
			}
			if st.Name == "combat-bar" || st.Name == "combat-spell-bob" || st.Name == "round3" || st.Name == "after-round" {
				base := st.Name + "-" + lang
				r.OriginalPNG, r.HDPNG, r.Indexed = base+"-original.png", base+"-hd.png", base+"-indexed.bin"
				for file, pix := range map[string][]byte{r.OriginalPNG: orig, r.HDPNG: hd} {
					f, err := os.OpenFile(filepath.Join(out, file), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, &image.RGBA{Pix: pix, Stride: 640 * 4, Rect: image.Rect(0, 0, 640, 400)})
					f.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(out, r.Indexed), idx, 0644); err != nil {
					t.Fatal(err)
				}
			}
			receipts = append(receipts, r)
		}
		if err := s.Ov.SetDisplay(display); err != nil {
			t.Fatal(err)
		}
		s.Frame()
	}
	b, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "normal-route-receipts.json"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d combat/language state samples", len(receipts))
}
