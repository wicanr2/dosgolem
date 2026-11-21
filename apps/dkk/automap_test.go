package dkk

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// plantGeo 在合成記憶體放一塊 GEO 本體與 DS:C06A 指標、DS:C073 位置，回本體雜湊。
func plantGeo(m fakeMem, ds uint16, seg uint16, x, y, dir byte) string {
	body := make([]byte, geoBodySize)
	body[3*16+2] = 0x10       // (2,3) 北牆
	body[0x100+3*16+2] = 0x01 // (2,3) 西牆
	copy(m[linear(seg, 0):], body)
	base := linear(ds, 0)
	m.put16(base+geoPtrOff, 0)
	m.put16(base+geoPtrOff+2, seg)
	m[base+partyXOff], m[base+partyXOff+1], m[base+partyXOff+2] = x, y, dir
	s := sha256.Sum256(body)
	return hex.EncodeToString(s[:])
}

func TestAutoMapSample(t *testing.T) {
	m := make(fakeMem, 0x100000)
	sum := plantGeo(m, 0x0CDE, 0x4C28, 2, 3, 4)
	var visits []string
	a := &AutoMap{Guide: &MapGuide{ByHash: map[string]string{sum: "1/24"}, Titles: map[string]string{"1/24": "蓋加斯屯墾區"},
		Points: map[string][]MapPoint{"1/24": {{N: 9, X: 2, Y: 3, Label: "銀行", Summary: "唯一的銀行"}, {N: 11, X: 6, Y: 9, Label: "神殿"}}}},
		Format:  MapFormat{"未知", "無資料", "{x},{y} 面向{dir}", "北東南西", "V 全圖", "V 只看走過", "{page}/{pages}"},
		OnVisit: func(k string, x, y int) { visits = append(visits, k) }}
	a.Sample(m, 0x0CDE)
	if a.Key != "1/24" || a.X != 2 || a.Y != 3 || a.Dir != 2 {
		t.Fatalf("取樣：key=%q (%d,%d) dir=%d", a.Key, a.X, a.Y, a.Dir)
	}
	if w := a.walls[3][2]; w[0] != 1 || w[3] != 1 || w[1] != 0 || w[2] != 0 {
		t.Errorf("牆面：%v", w)
	}
	if len(visits) != 1 {
		t.Errorf("第一次走到要通知一次：%v", visits)
	}
	a.Sample(m, 0x0CDE)
	if len(visits) != 1 {
		t.Errorf("同一格不重複通知：%v", visits)
	}
	// 只看走過：銀行那格走過、神殿沒有
	if p := a.VisiblePoints(); len(p) != 1 || p[0].N != 9 {
		t.Errorf("可見點：%+v", p)
	}
	a.open = true
	a.ToggleReveal()
	if p := a.VisiblePoints(); len(p) != 2 {
		t.Errorf("全圖：%+v", p)
	}
	// 同一緩衝區小改動（開門）沿用地圖鍵；換緩衝區又對不到則未知
	m[linear(0x4C28, 0)+5] = 0xFF
	a.Sample(m, 0x0CDE)
	if a.Key != "1/24" {
		t.Errorf("同緩衝區小改動應沿用：%q", a.Key)
	}
	base := linear(0x0CDE, 0)
	m.put16(base+geoPtrOff+2, 0x5000)
	a.Sample(m, 0x0CDE)
	if a.Key != "" {
		t.Errorf("換緩衝區對不到應為未知：%q", a.Key)
	}
}

func TestVisitedFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "v.txt")
	if err := AppendVisited(p, "1/24", 6, 9); err != nil {
		t.Fatal(err)
	}
	if err := AppendVisited(p, "1/24", 7, 5); err != nil {
		t.Fatal(err)
	}
	v, err := ReadVisited(p)
	if err != nil || len(v["1/24"]) != 2 || !v["1/24"][cellPos{7, 5}] {
		t.Fatalf("讀回：%v %v", v, err)
	}
	if err := os.WriteFile(p, []byte("bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadVisited(p); err == nil {
		t.Error("壞行要回錯")
	}
}

func TestLoadMapGuideRejectsUnknownMap(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "map-geo.tsv"), []byte("map\tpayload_sha256\ttitle\n1/24\tab\t蓋加斯\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "map-guide.zh-TW.tsv"), []byte("map\tn\tx\ty\tlabel\tsummary\tsource\n9/99\t1\t0\t0\tx\ty\tz\n"), 0o644)
	if _, err := LoadMapGuideDir(dir); err == nil {
		t.Error("地圖不在 map-geo.tsv 要回錯")
	}
}

// TestMapGuideTable 檢查正式地圖資料（規格 009 §3 靜態）：可載入、每點在 0..15、每塊有點的地圖都有名稱。
func TestMapGuideTable(t *testing.T) {
	dir := os.Getenv("DOSGOLEM_DKK_TEXT_DIR")
	if dir == "" {
		t.Skip("DOSGOLEM_DKK_TEXT_DIR 未設定")
	}
	g, err := LoadMapGuideDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if g == nil || len(g.Points) == 0 {
		t.Fatal("沒有攻略點")
	}
	for k, ps := range g.Points {
		if g.Titles[k] == "" {
			t.Errorf("地圖 %s 有 %d 點但沒有名稱", k, len(ps))
		}
		for _, p := range ps {
			if p.Label == "" || len([]rune(p.Label)) > 10 {
				t.Errorf("%s %d 名稱長度不合：%q", k, p.N, p.Label)
			}
		}
	}
}
