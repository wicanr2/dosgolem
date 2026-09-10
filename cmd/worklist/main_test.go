package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 這些測試釘住的是**核實機制自己會不會安靜地失效**。
//
// 只驗「它報仍未完成」證明不了任何事：空的目錄、寫錯的路徑、永遠為真的
// pattern，印出來的好消息一模一樣。所以每一條都往兩個方向各驗一次——
// 訊號在的時候報未完成，把訊號拿掉之後**要開口**。

// fixture 造一棵小樹：root/go.mod ＋ 指定的檔案。
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if files == nil {
		files = map[string]string{}
	}
	files["go.mod"] = "module fixture\n"
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func presentItem(pattern string) Item {
	return Item{
		ID: "x", Layer: "l", Acceptance: "做完就好",
		Verify: Verify{Kind: "present", Paths: []string{"src"}, Pattern: pattern},
	}
}

func absentItem(pattern string) Item {
	return Item{
		ID: "x", Layer: "l", Acceptance: "做完就好",
		Verify: Verify{Kind: "absent", Paths: []string{"src"}, Pattern: pattern},
	}
}

// TestPresentOpensWhenTheAdmissionIsGone 是 present 的正反對照。
//
// 綁自承註解的條目，**註解被刪掉就要開口**——那正是「該回頭看這一條」
// 的時刻。不開口的話，清單會在東西做好之後繼續說「還沒接」。
func TestPresentOpensWhenTheAdmissionIsGone(t *testing.T) {
	withAdmission := fixture(t, map[string]string{
		"src/a.go": "package a\n\n// 這裡到不了 1 MB 之上\n",
	})
	open, why, err := stillOpen(withAdmission, presentItem("到不了 1 MB 之上"))
	if err != nil {
		t.Fatal(err)
	}
	if !open {
		t.Fatalf("自承還在卻說做完了：%s", why)
	}

	// 反方向：把自承拿掉。
	without := fixture(t, map[string]string{"src/a.go": "package a\n"})
	open, why, err = stillOpen(without, presentItem("到不了 1 MB 之上"))
	if err != nil {
		t.Fatal(err)
	}
	if open {
		t.Fatalf("自承已經不在了，還說仍未完成：%s", why)
	}
}

// TestAbsentOpensUntilTheThingAppears 是 absent 的正反對照。
func TestAbsentOpensUntilTheThingAppears(t *testing.T) {
	without := fixture(t, map[string]string{"src/a.go": "package a\n"})
	open, _, err := stillOpen(without, absentItem(`func Linear\(`))
	if err != nil {
		t.Fatal(err)
	}
	if !open {
		t.Fatal("東西還沒出現卻說做完了")
	}

	with := fixture(t, map[string]string{
		"src/a.go": "package a\n\nfunc Linear(seg, off uint16) uint32 { return 0 }\n",
	})
	open, why, err := stillOpen(with, absentItem(`func Linear\(`))
	if err != nil {
		t.Fatal(err)
	}
	if open {
		t.Fatalf("東西已經出現了，還說仍未完成：%s", why)
	}
}

// TestVerifyIgnoresTestFiles 釘住「測試檔不算」。
//
// 測試本來就會提到還沒接上的東西——為了釘住將來的行為。把測試檔算進來，
// absent 會因為測試裡有一行呼叫就判成「已經做了」，**真缺口就這樣被蓋掉**，
// 而且蓋得無聲無息。
func TestVerifyIgnoresTestFiles(t *testing.T) {
	root := fixture(t, map[string]string{
		"src/a_test.go": "package a\n\nfunc TestX(t *testing.T) { Linear(0, 0) }\n",
	})
	open, why, err := stillOpen(root, absentItem(`Linear\(`))
	if err != nil {
		t.Fatal(err)
	}
	if !open {
		t.Fatalf("只有測試檔提到就判成做完了：%s", why)
	}
}

// TestManualAlwaysOpens 釘住「沉默不等於通過」。
func TestManualAlwaysOpens(t *testing.T) {
	root := fixture(t, nil)
	it := Item{ID: "x", Layer: "l", Acceptance: "a",
		Verify: Verify{Kind: "manual", Note: "接上時綁那個型別名"}}
	open, why, err := stillOpen(root, it)
	if err != nil {
		t.Fatal(err)
	}
	if !open || !strings.Contains(why, "要人判") {
		t.Fatalf("manual 沒有回仍未完成並標出來：open=%v why=%q", open, why)
	}
}

// TestCheckRejectsSilentlyBrokenItems 擋掉會安靜失效的條目。
//
// **空的 paths、空的 pattern、編不起來的 regexp** 都會讓 verify 永遠回
// 同一個答案，而報表上與正常的一模一樣。那比沒有 verify 更糟：
// 它給人「有在看」的假象。
func TestCheckRejectsSilentlyBrokenItems(t *testing.T) {
	layers := map[string]string{"l": "測試用"}
	bad := []struct {
		name string
		item Item
	}{
		{"沒有 paths", Item{ID: "a", Layer: "l", Acceptance: "a",
			Verify: Verify{Kind: "present", Pattern: "x"}}},
		{"沒有 pattern", Item{ID: "a", Layer: "l", Acceptance: "a",
			Verify: Verify{Kind: "absent", Paths: []string{"src"}}}},
		{"pattern 編不起來", Item{ID: "a", Layer: "l", Acceptance: "a",
			Verify: Verify{Kind: "present", Paths: []string{"src"}, Pattern: "("}}},
		{"manual 沒寫將來綁什麼", Item{ID: "a", Layer: "l", Acceptance: "a",
			Verify: Verify{Kind: "manual"}}},
		{"沒有 acceptance", Item{ID: "a", Layer: "l",
			Verify: Verify{Kind: "manual", Note: "n"}}},
		{"layer 不在表裡", Item{ID: "a", Layer: "沒這層", Acceptance: "a",
			Verify: Verify{Kind: "manual", Note: "n"}}},
		{"不認得的 kind", Item{ID: "a", Layer: "l", Acceptance: "a",
			Verify: Verify{Kind: "怎麼看"}}},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			if err := check(&Worklist{Layers: layers, Items: []Item{c.item}}); err == nil {
				t.Error("這種條目應該被擋下來")
			}
		})
	}
	if err := check(&Worklist{Layers: layers, Items: []Item{
		{ID: "same", Layer: "l", Acceptance: "a", Verify: Verify{Kind: "manual", Note: "n"}},
		{ID: "same", Layer: "l", Acceptance: "a", Verify: Verify{Kind: "manual", Note: "n"}},
	}}); err == nil {
		t.Error("id 重複應該被擋下來")
	}
}

// TestRealWorklistIsWellFormed 讓 repo 裡那一份也過同一關。
func TestRealWorklistIsWellFormed(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	wl, err := load(filepath.Join(root, worklistPath))
	if err != nil {
		t.Fatal(err)
	}
	// **空清單有兩種原因，要分得開**：路徑錯／檔案讀不到（那時 resolved
	// 也會是空的），與真的全部做完了（resolved 裡有東西）。只檢查 items
	// 的話，前者會被讀成後者——而那正是這份工具要防的那種安靜失效。
	if len(wl.Items)+len(wl.Resolved) == 0 {
		t.Fatal("items 與 resolved 都是空的——是不是路徑錯了")
	}
	// 每一條的 pattern 都要真的在這棵樹上有明確的落點：
	// present 找得到、absent 找不到。**兩個方向的錯都會安靜地過去**，
	// 所以在這裡當場對現在的樹問一次。
	for _, it := range wl.Items {
		open, why, err := stillOpen(root, it)
		if err != nil {
			t.Errorf("%s：核實不了：%v", it.ID, err)
			continue
		}
		if !open {
			t.Errorf("%s：verify 說它可能已經做完了（%s）——做完就從 JSON 移走", it.ID, why)
		}
	}
}

// TestRenderIsStable 釘住 render 的輸出不隨 map 走訪順序漂移。
//
// 漂移的話每次 render 出來的檔案都在動，git diff 全是雜訊，
// 真正的斷言改動就藏在裡面看不見了。
func TestRenderIsStable(t *testing.T) {
	wl := &Worklist{
		Layers: map[string]string{"b": "第二層", "a": "第一層", "c": "第三層"},
		Items: []Item{
			{ID: "x", Layer: "a", Title: "甲", Acceptance: "a", Verify: Verify{Kind: "manual", Note: "n"}},
			{ID: "y", Layer: "b", Title: "乙", Acceptance: "a", Verify: Verify{Kind: "manual", Note: "n"}},
			{ID: "z", Layer: "c", Title: "丙", Acceptance: "a", Verify: Verify{Kind: "manual", Note: "n"}},
		},
	}
	first := render(wl)
	for i := 0; i < 20; i++ {
		if got := render(wl); got != first {
			t.Fatal("render 的輸出會漂移")
		}
	}
	if ia, ib := strings.Index(first, "## a"), strings.Index(first, "## b"); ia > ib {
		t.Error("層沒有照名字排序")
	}
	if !strings.Contains(first, "不要手改") {
		t.Error("產生的 markdown 沒寫明不要手改")
	}
}

// TestResolvedIsNotVerified 釘住「已解決的不再被核實」。
//
// resolved 記的是歷史：那些 pattern 早就不在樹上了，拿去核實只會得到
// 一整排「可能已完成」的雜訊，把真正該看的那一條淹掉。
func TestResolvedIsNotVerified(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	wl, err := load(filepath.Join(root, worklistPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(wl.Resolved) == 0 {
		t.Skip("目前沒有已解決的條目")
	}
	ids := map[string]bool{}
	for _, it := range wl.Items {
		ids[it.ID] = true
	}
	for _, r := range wl.Resolved {
		if ids[r.ID] {
			t.Errorf("%s 同時在 items 與 resolved 裡", r.ID)
		}
	}
	if md := render(wl); !strings.Contains(md, "已解決") {
		t.Error("render 沒把已解決那一段列出來")
	}
}

// TestCheckRejectsResolvedWithoutEvidence 擋掉沒寫清楚的 resolved。
func TestCheckRejectsResolvedWithoutEvidence(t *testing.T) {
	layers := map[string]string{"l": "測試用"}
	for name, r := range map[string]Resolved{
		"沒有 date": {ID: "a", How: "改好了"},
		"沒有 how":  {ID: "a", Date: "2026-09-10"},
		"沒有 id":   {Date: "2026-09-10", How: "改好了"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := check(&Worklist{Layers: layers, Resolved: []Resolved{r}}); err == nil {
				t.Error("這種 resolved 應該被擋下來")
			}
		})
	}
	// 同一個 id 不能兩邊都在——那表示做完了卻沒從 items 移走。
	if err := check(&Worklist{
		Layers:   layers,
		Items:    []Item{{ID: "dup", Layer: "l", Acceptance: "a", Verify: Verify{Kind: "manual", Note: "n"}}},
		Resolved: []Resolved{{ID: "dup", Date: "2026-09-10", How: "改好了"}},
	}); err == nil {
		t.Error("id 同時在 items 與 resolved 應該被擋下來")
	}
}
