package phantasie

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRouteLegal(t *testing.T) {
	src := "# 註解\n" +
		"\n" +
		"Return\r\n" +
		"@wait 3\n" +
		"Up\n" +
		"  A  \n" +
		"@check title\n" +
		"@expect - continue saved game\n" +
		"@expect Do you want:\n" +
		"@known-untranslated SOME KEY\n" +
		"@lang zh-CN\n" +
		"@check next\n"
	got, err := ParseRoute(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []RouteStep{
		{Kind: RouteKey, Key: "Return", Line: 3},
		{Kind: RouteKey, Key: "Up", Wait: 3, Line: 5},
		{Kind: RouteKey, Key: "A", Line: 6},
		{Kind: RouteCheck, Name: "title", Line: 7,
			Expect: []string{"- continue saved game", "Do you want:"}, Known: []string{"SOME KEY"}},
		{Kind: RouteLang, Name: "zh-CN", Line: 11},
		{Kind: RouteCheck, Name: "next", Line: 12},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("解析結果不同：\n得 %+v\n要 %+v", got, want)
	}
}

// @snap：鍵送出後再執行指定步數擷取（沒有讀鍵入口的畫面，例如戰鬥回合）；@expect 與 @known-untranslated 可接在 @snap 之後。
func TestParseRouteSnap(t *testing.T) {
	got, err := ParseRoute("Return\n@snap round 1500\n@expect HITS\n@known-untranslated X\n@check after\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []RouteStep{
		{Kind: RouteKey, Key: "Return", Line: 1},
		{Kind: RouteSnap, Name: "round", Steps: 1500, Line: 2, Expect: []string{"HITS"}, Known: []string{"X"}},
		{Kind: RouteCheck, Name: "after", Line: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("解析結果不同：\n得 %+v\n要 %+v", got, want)
	}
}

func TestParseRouteErrors(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"一行兩個鍵", "Return Esc\n", "第 1 行：一行只能有一個按鍵"},
		{"wait 不是數字", "@wait x\nReturn\n", "第 1 行：@wait 需要非負整數"},
		{"wait 負數", "@wait -1\nReturn\n", "第 1 行：@wait 需要非負整數"},
		{"wait 後接檢查點", "@wait 2\n@check a\n", "第 2 行：@wait 後面必須接按鍵"},
		{"wait 後接語言", "@wait 2\n@lang zh-CN\n", "第 2 行：@wait 後面必須接按鍵"},
		{"路線結尾的 wait", "Return\n@wait 2\n", "路線結尾的 @wait 後面沒有按鍵"},
		{"check 沒有名稱", "@check\n", "第 1 行：@check 需要一個不含空白的名稱"},
		{"check 名稱含空白", "@check a b\n", "第 1 行：@check 需要一個不含空白的名稱"},
		{"expect 沒有鍵", "@check a\n@expect\n", "第 2 行：@expect 需要一個鍵"},
		{"expect 前面不是 check", "Return\n@expect X\n", "第 2 行：@expect 必須緊接在 @check 或 @snap 之後"},
		{"known 在開頭", "@known-untranslated X\n", "第 1 行：@known-untranslated 必須緊接在 @check 或 @snap 之後"},
		{"lang 沒有語言", "@lang\n", "第 1 行：@lang 需要一個語言代碼"},
		{"未知指令", "@frobnicate 1\n", "第 1 行：未知的指令 @frobnicate"},
		{"snap 沒有步數", "@snap a\n", "第 1 行：@snap 需要名稱與步數"},
		{"snap 步數不是數字", "@snap a x\n", "第 1 行：@snap 的步數需要非負整數"},
		{"snap 步數負數", "@snap a -5\n", "第 1 行：@snap 的步數需要非負整數"},
		{"snap 名稱與 check 重複", "@check a\n@snap a 10\n", "第 2 行：檢查點名稱 \"a\" 重複"},
		{"wait 後接 snap", "@wait 2\n@snap a 10\n", "第 2 行：@wait 後面必須接按鍵"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRoute(c.src)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("錯誤 %v，期望含 %q", err, c.want)
			}
		})
	}
}

// 路線檔不含指令的輸入是合法的空路線。
func TestParseRouteEmpty(t *testing.T) {
	got, err := ParseRoute("# 只有註解\n\n")
	if err != nil || len(got) != 0 {
		t.Fatalf("得 %v, %v，期望空路線", got, err)
	}
}
