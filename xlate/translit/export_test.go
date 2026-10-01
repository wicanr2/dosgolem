package translit

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestSharedCMUParsesOnceAndSharesTheTable(t *testing.T) {
	path := filepath.Join("testdata", "text", "cmudict", "cmudict.dict")
	a, err := SharedCMU(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	got := make([]map[string][]string, 8)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m, err := SharedCMU(path)
			if err != nil {
				t.Error(err)
			}
			got[i] = m
		}(i)
	}
	wg.Wait()
	for i, m := range got {
		if reflect.ValueOf(m).Pointer() != reflect.ValueOf(a).Pointer() {
			t.Errorf("第 %d 次取得的對照表不是同一份", i)
		}
	}
	// A relative and an absolute spelling of the path are the same key.
	abs, _ := filepath.Abs(path)
	if m, _ := SharedCMU(abs); reflect.ValueOf(m).Pointer() != reflect.ValueOf(a).Pointer() {
		t.Error("相對與絕對路徑應為同一份")
	}
	// Two zh transliterators share the table.
	t1, t2 := load(t), load(t)
	if reflect.ValueOf(t1.cmu).Pointer() != reflect.ValueOf(t2.cmu).Pointer() {
		t.Error("兩個 Transliterator 應共用詞典")
	}
}

func TestSharedCMUErrorIsNotCached(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cmudict.dict")
	if _, err := SharedCMU(path); err == nil {
		t.Fatal("缺檔應回錯誤")
	}
	if err := os.WriteFile(path, []byte("hello HH AH0 L OW1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := SharedCMU(path)
	if err != nil || len(m) != 1 {
		t.Fatalf("檔案補上後應成功：%v %v", m, err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if m2, err := SharedCMU(path); err != nil || len(m2) != 1 {
		t.Errorf("成功值載入後唯讀、不重新解析：%v %v", m2, err)
	}
}

func TestExportedFrontEndMatchesInternal(t *testing.T) {
	ps, ok := ParsePron([]string{"HH", "AH0", "L", "OW1"})
	if !ok || len(ps) != 4 || !ps[1].Vowel || ps[1].Stress != 0 || ps[0].Stress != -1 {
		t.Errorf("ParsePron = %+v %v", ps, ok)
	}
	if _, ok := ParsePron([]string{"XX1"}); ok {
		t.Error("非音素應失敗")
	}
	sp := SpellingPhones("Jane-")
	want := toPhones(spellingPhones("jane"))
	if !reflect.DeepEqual(sp, want) {
		t.Errorf("SpellingPhones = %+v，應為 %+v", sp, want)
	}
	al := AlignVowels("michael", mustPhones(t, "M", "AY1", "K", "AH0", "L"))
	if len(al) != 2 || al[0] != "i" || al[1] != "ae" {
		t.Errorf("AlignVowels(michael) = %v", al)
	}
	if AlignVowels("xyz", mustPhones(t, "AH0")) != nil {
		// no vowel letter group for one vowel phoneme: nil
	}
}

func mustPhones(t *testing.T, pron ...string) []Phone {
	t.Helper()
	p, ok := ParsePron(pron)
	if !ok {
		t.Fatalf("ParsePron(%v)", pron)
	}
	return p
}
