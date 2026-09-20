package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const menuEventFixture = "event_key\tsequence\ttext_key\toriginal_length\toriginal_sha256\tcaller\tbackground\tforeground\trow\tcolumn\n" +
	"race.option.terran\t1\trace.terran\t8\t28f962ee4f76bcaea2e7d195fccb11782ecb4aea3c8f78e342118d69968e5d69\t37F1:15BD\t0\t10\t3\t1\n" +
	"race.heading.terran\t2\trace.terran\t6\t237c59a3b965d0787ce08d5afcd4ec80017f7de4b6d84021f2207f865345f419\t37F1:175D\t15\t0\t3\t3\n"

const menuTextFixture = "key\ttranslation\tsource\n" +
	"race.terran\t地球人\tmanual-and-runtime\n"

func fixtureMenuEvent(t *testing.T) TextEvent {
	t.Helper()
	h, err := menuHash("28f962ee4f76bcaea2e7d195fccb11782ecb4aea3c8f78e342118d69968e5d69")
	if err != nil {
		t.Fatal(err)
	}
	return TextEvent{EntryStep: 10, PostCallStep: 20, Caller: Address{0x37F1, 0x15BD},
		OriginalLength: 8, OriginalSHA256: h, Background: 0, Foreground: 10, Row: 3, Column: 1}
}

func TestMenuCatalogExactResolveAndSharedTextKey(t *testing.T) {
	c, err := LoadMenuCatalog([]byte(menuEventFixture), []byte(menuTextFixture))
	if err != nil {
		t.Fatal(err)
	}
	r, ok := c.Resolve(fixtureMenuEvent(t))
	if !ok || r.Generation != 0 || r.EventKey != "race.option.terran" || r.TextKey != "race.terran" || r.Translation != "地球人" {
		t.Fatalf("Resolve = %#v, %v", r, ok)
	}
}

func TestGenderCatalogReusesExactResolver(t *testing.T) {
	c, err := LoadGenderCatalog([]byte(menuEventFixture), []byte(menuTextFixture))
	if err != nil {
		t.Fatal(err)
	}
	request, ok := c.Resolve(fixtureMenuEvent(t))
	if !ok || request.EventKey != "race.option.terran" || request.Translation != "地球人" {
		t.Fatalf("Resolve = %#v, %v", request, ok)
	}
}

func TestClassCatalogReusesExactResolver(t *testing.T) {
	c, err := LoadClassCatalog([]byte(menuEventFixture), []byte(menuTextFixture))
	if err != nil {
		t.Fatal(err)
	}
	request, ok := c.Resolve(fixtureMenuEvent(t))
	if !ok || request.EventKey != "race.option.terran" || request.Translation != "地球人" {
		t.Fatalf("Resolve = %#v, %v", request, ok)
	}
}

func TestMergeMenuCatalogsAndRejectIdentityCollision(t *testing.T) {
	first, err := LoadMenuCatalog([]byte(menuEventFixture), []byte(menuTextFixture))
	if err != nil {
		t.Fatal(err)
	}
	secondEvents := strings.ReplaceAll(menuEventFixture, "37F1:15BD", "37F1:15BE")
	secondEvents = strings.ReplaceAll(secondEvents, "37F1:175D", "37F1:175E")
	second, err := LoadGenderCatalog([]byte(secondEvents), []byte(menuTextFixture))
	if err != nil {
		t.Fatal(err)
	}
	merged, err := MergeMenuCatalogs(first, second)
	if err != nil || len(merged.byIdentity) != 4 {
		t.Fatalf("merged=%#v err=%v", merged, err)
	}
	if _, err := MergeMenuCatalogs(first, first); err == nil {
		t.Fatal("相同 identity 合併必須失敗")
	}
	if empty, err := MergeMenuCatalogs(nil); err != nil || empty != nil {
		t.Fatalf("空合併 = %#v, %v", empty, err)
	}
}

func TestMenuCatalogResolveFailsClosed(t *testing.T) {
	c, err := LoadMenuCatalog([]byte(menuEventFixture), []byte(menuTextFixture))
	if err != nil {
		t.Fatal(err)
	}
	base := fixtureMenuEvent(t)
	cases := []TextEvent{base, base, base, base, base, base, base, base}
	cases[0].PostCallStep = cases[0].EntryStep
	cases[1].OriginalLength++
	cases[2].OriginalSHA256[0]++
	cases[3].Caller.Offset++
	cases[4].Background++
	cases[5].Foreground++
	cases[6].Row++
	cases[7].Column++
	for i, event := range cases {
		if got, ok := c.Resolve(event); ok {
			t.Errorf("case %d 不應命中：%#v", i, got)
		}
	}
}

func TestMenuCatalogRejectsMalformedInputs(t *testing.T) {
	line := strings.Split(menuEventFixture, "\n")[1]
	tests := []struct{ name, events, texts string }{
		{"跳號", strings.Replace(menuEventFixture, "\t2\trace.terran", "\t3\trace.terran", 1), menuTextFixture},
		{"大寫雜湊", strings.Replace(menuEventFixture, "28f9", "28F9", 1), menuTextFixture},
		{"小寫位址", strings.Replace(menuEventFixture, "37F1", "37f1", 1), menuTextFixture},
		{"數值越界", strings.Replace(menuEventFixture, "\t8\t28f9", "\t256\t28f9", 1), menuTextFixture},
		{"重複事件鍵", menuEventFixture + strings.Replace(line, "\t1\t", "\t3\t", 1) + "\n", menuTextFixture},
		{"重複 identity", menuEventFixture + strings.Replace(strings.Replace(line, "race.option.terran", "race.option.copy", 1), "\t1\t", "\t3\t", 1) + "\n", menuTextFixture},
		{"缺翻譯", strings.Replace(menuEventFixture, "race.terran", "race.unknown", 1), menuTextFixture},
		{"孤兒翻譯", menuEventFixture, menuTextFixture + "race.orphan\t孤兒\ttest\n"},
		{"重複翻譯鍵", menuEventFixture, menuTextFixture + "race.terran\t重複\ttest\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadMenuCatalog([]byte(tc.events), []byte(tc.texts)); err == nil {
				t.Fatal("應拒絕無效輸入")
			}
		})
	}
}

func TestFormalProjectMenuCatalogAndNineEvents(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, "text", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	events, texts := read("menu-events.tsv"), read("menu.zh-TW.tsv")
	eventsSum := sha256.Sum256(events)
	if got := hex.EncodeToString(eventsSum[:]); got != "973a6a1e247e7d9e16518a1a66266f340666d32e785f3f6f6652a890e830da2e" {
		t.Fatalf("menu-events.tsv SHA-256 = %s", got)
	}
	textsSum := sha256.Sum256(texts)
	if got := hex.EncodeToString(textsSum[:]); got != "ca3319830adb7b34a048b498d8d0466b38b8fb6f418e5244a3a467e77ea68077" {
		t.Fatalf("menu.zh-TW.tsv SHA-256 = %s", got)
	}
	c, err := LoadMenuCatalog(events, texts)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := readTSV("menu-events.tsv", events, menuEventHeader)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Fatalf("事件數 = %d，要 12", len(rows))
	}
	for i, row := range rows {
		h, _ := menuHash(row[4])
		caller, _ := menuAddress(row[5])
		length, _ := menuByte(row[3])
		bg, _ := menuByte(row[6])
		fg, _ := menuByte(row[7])
		y, _ := menuByte(row[8])
		x, _ := menuByte(row[9])
		event := TextEvent{EntryStep: uint64(100 + i*2), PostCallStep: uint64(101 + i*2), Caller: caller,
			OriginalLength: length, OriginalSHA256: h, Background: bg, Foreground: fg, Row: y, Column: x}
		if request, ok := c.Resolve(event); !ok || request.EventKey != row[0] || request.TextKey != row[2] {
			t.Fatalf("事件 %d 解析 = %#v, %v", i+1, request, ok)
		}
	}
}

func TestFormalProjectGenderCatalog(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, "text", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	events, texts := read("gender-events.tsv"), read("gender.zh-TW.tsv")
	for name, fixture := range map[string]struct {
		data []byte
		want string
	}{
		"gender-events.tsv": {events, "a8c96c8edc393c26717a5d05bc46fc031d25a8d0c269f3fe1c6b617ee0dda297"},
		"gender.zh-TW.tsv":  {texts, "8fd64b9a15fc94aff73a8f7d06ff100b406763b09ac562989a82344eeeac6857"},
	} {
		sum := sha256.Sum256(fixture.data)
		if got := hex.EncodeToString(sum[:]); got != fixture.want {
			t.Fatalf("%s SHA-256 = %s", name, got)
		}
	}
	gender, err := LoadGenderCatalog(events, texts)
	if err != nil {
		t.Fatal(err)
	}
	menu, err := LoadMenuCatalog(read("menu-events.tsv"), read("menu.zh-TW.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	merged, err := MergeMenuCatalogs(menu, gender)
	if err != nil || len(gender.byIdentity) != 7 || len(merged.byIdentity) != 19 {
		t.Fatalf("gender=%d merged=%d err=%v", len(gender.byIdentity), len(merged.byIdentity), err)
	}
}

func TestFormalPhase27ReceiptResolvesNineRequests(t *testing.T) {
	root, receiptPath := os.Getenv("BUCKROGERS_CHT_ROOT"), os.Getenv("BUCKROGERS_MENU_RECEIPT")
	if root == "" || receiptPath == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 或 BUCKROGERS_MENU_RECEIPT 未設定")
	}
	events, err := os.ReadFile(filepath.Join(root, "text", "menu-events.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	texts, err := os.ReadFile(filepath.Join(root, "text", "menu.zh-TW.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadMenuCatalog(events, texts)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Events []struct {
			EntryStep      uint64  `json:"entry_step"`
			PostCallStep   uint64  `json:"post_call_step"`
			Caller         Address `json:"caller"`
			OriginalLength uint8   `json:"original_length"`
			OriginalSHA256 string  `json:"original_sha256"`
			Background     uint8   `json:"background"`
			Foreground     uint8   `json:"foreground"`
			Row            uint8   `json:"row"`
			Column         uint8   `json:"column"`
		} `json:"events"`
	}
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt.Events) != 9 {
		t.Fatalf("收據事件數 = %d，要 9", len(receipt.Events))
	}
	for i, raw := range receipt.Events {
		h, err := menuHash(raw.OriginalSHA256)
		if err != nil {
			t.Fatalf("事件 %d 雜湊：%v", i+1, err)
		}
		event := TextEvent{raw.EntryStep, raw.PostCallStep, raw.Caller, raw.OriginalLength, h,
			raw.Background, raw.Foreground, raw.Row, raw.Column}
		request, ok := c.Resolve(event)
		if !ok {
			t.Fatalf("收據事件 %d 未解析", i+1)
		}
		if request.Generation != 0 || request.EventKey == "" || request.TextKey == "" || request.Translation == "" {
			t.Fatalf("收據事件 %d 請求不完整：%#v", i+1, request)
		}
	}
}
