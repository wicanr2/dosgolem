package phantasie

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// 期望值都是 docs/re/009 與 docs/spec/003 §5.1 的字面位址與手算結果，不由被測函式推導。

type regCase struct {
	name    string
	ptr, sp uint16
	overlay string
	want    Kind
}

func regDefault(t *testing.T) *RegionTable {
	t.Helper()
	tb, err := DefaultRegions()
	if err != nil {
		t.Fatalf("DefaultRegions: %v", err)
	}
	return tb
}

func regRun(t *testing.T, tb *RegionTable, cases []regCase) {
	t.Helper()
	for _, c := range cases {
		if got := tb.Classify(c.ptr, c.sp, c.overlay); got != c.want {
			t.Errorf("%s：Classify(%04X, sp=%04X, %q) = %v，要 %v", c.name, c.ptr, c.sp, c.overlay, got, c.want)
		}
	}
}

const regSP = 0xFF00 // 一般狀況的 SP：遠高於所有表內區間

func TestRegionsDefaultLoads(t *testing.T) {
	tb := regDefault(t)
	if len(tb.rows) != 8 {
		t.Errorf("內嵌表應有 8 列（三筆 buffer：行緩衝區、位置描述區、buffer@ov2 位置訊息行；static、static@ov1、static@ov2、monster、town），得到 %d", len(tb.rows))
	}
	// 列序依種類優先序（buffer 在 static 之前）穩定排序，buffer@ov2 先出現。
	if strings.Join(tb.tags, ",") != "ov2,ov1" {
		t.Errorf("overlay 字樣 = %v，要 ov2、ov1", tb.tags)
	}
}

func TestRegionsStatic(t *testing.T) {
	regRun(t, regDefault(t), []regCase{
		{"靜態區起點", 0x0000, regSP, "", KindStatic},
		{"實測最大字串偏移", 0x321B, regSP, "", KindStatic},
		{"靜態區末位元組", 0x3243, regSP, "", KindStatic},
		{"靜態區界外（FONT 緩衝區起點）", 0x3244, regSP, "", KindOther},
		{"靜態區界外之後", 0x3245, regSP, "", KindOther},
		{"載入 ov1 時靜態區照常有效", 0x0100, regSP, "ov1.test", KindStatic},
		{"載入 ov2 時靜態區照常有效", 0x3243, regSP, "ov2.test", KindStatic},
	})
}

func TestRegionsOverlayData(t *testing.T) {
	regRun(t, regDefault(t), []regCase{
		// ov1：[B8F0, C45E)
		{"ov1 區前一個位元組", 0xB8EF, regSP, "ov1.test", KindOther},
		{"ov1 區起點", 0xB8F0, regSP, "ov1.test", KindStatic},
		{"ov1 區中段（超過 ov2 區尾）", 0xC400, regSP, "ov1.test", KindStatic},
		{"ov1 區末位元組", 0xC45D, regSP, "ov1.test", KindStatic},
		{"ov1 區尾是界外", 0xC45E, regSP, "ov1.test", KindOther},
		{"ov1 區尾之後", 0xC45F, regSP, "ov1.test", KindOther},
		// ov2：[B8F0, C400)
		{"ov2 區前一個位元組", 0xB8EF, regSP, "ov2.test", KindOther},
		{"ov2 區起點", 0xB8F0, regSP, "ov2.test", KindStatic},
		{"ov2 區末位元組", 0xC3FF, regSP, "ov2.test", KindStatic},
		{"ov2 區尾起是位置訊息行緩衝區（buffer@ov2，80 bytes）", 0xC400, regSP, "ov2.test", KindBuffer},
		{"位置訊息行緩衝區末位元組", 0xC44F, regSP, "ov2.test", KindBuffer},
		{"位置訊息行緩衝區界外", 0xC450, regSP, "ov2.test", KindOther},
		{"ov1 下 C400 仍是靜態區（不是 buffer）", 0xC400, regSP, "ov1.test", KindStatic},
		{"沒有載入 overlay 時 C400 不是 buffer", 0xC400, regSP, "", KindOther},
		{"ov1 的末位元組在 ov2 下是界外", 0xC45D, regSP, "ov2.test", KindOther},
		// overlay 名稱的比對
		{"大寫", 0xB8F0, regSP, "OV1.TEST", KindStatic},
		{"含路徑", 0xB8F0, regSP, `C:\PHANTASI\Ov2.test`, KindStatic},
		{"沒有載入 overlay 不判", 0xB8F0, regSP, "", KindOther},
		{"不認得的 overlay 不判", 0xB8F0, regSP, "ov3.ovr", KindOther},
		{"同時含 ov1 與 ov2（有歧義）不判", 0xB8F0, regSP, "ov1ov2.test", KindOther},
		{"沒有載入 overlay 時區尾內也不判", 0xC000, regSP, "", KindOther},
	})
}

func TestRegionsLineBuffer(t *testing.T) {
	regRun(t, regDefault(t), []regCase{
		{"行緩衝區前一個位元組", 0x638D, regSP, "", KindOther},
		{"行緩衝區起點", 0x638E, regSP, "", KindBuffer},
		{"行緩衝區 80 bytes 內", 0x63A0, regSP, "", KindBuffer},
		{"行緩衝區末位元組", 0x63DD, regSP, "", KindBuffer},
		{"行緩衝區尾是界外", 0x63DE, regSP, "", KindOther},
		{"與 overlay 無關", 0x638E, regSP, "ov1.test", KindBuffer},
	})
}

func TestRegionsStack(t *testing.T) {
	regRun(t, regDefault(t), []regCase{
		{"ptr = sp 是堆疊", 0xFF00, 0xFF00, "", KindBuffer},
		{"ptr = sp - 1 不是", 0xFEFF, 0xFF00, "", KindOther},
		{"堆疊頂端", 0xFFFF, 0xFF00, "", KindBuffer},
		{"動態記錄的 FF8B", 0xFF8B, 0xFF7E, "", KindBuffer},
		{"動態記錄的 sp 以下一格", 0xFF7D, 0xFF7E, "", KindOther},
		// 堆疊優先於其他區間
		{"堆疊優先於怪物區間", 0x7522, 0x7000, "", KindBuffer},
		{"堆疊優先於城鎮區間", 0x8071, 0x8000, "", KindBuffer},
		{"堆疊優先於靜態區", 0x0100, 0x0100, "", KindBuffer},
		{"sp 之下的怪物仍是怪物", 0x7522, 0x7523, "", KindMonster},
		// sp 為 0：沒有堆疊資訊，不套用堆疊規則（否則全部指標都是 buffer）
		{"sp = 0 不判堆疊", 0xFF8B, 0, "", KindOther},
		{"sp = 0 時靜態區照常", 0x0000, 0, "", KindStatic},
		{"sp = 0 時行緩衝區照常", 0x638E, 0, "", KindBuffer},
	})
}

func TestRegionsMonster(t *testing.T) {
	regRun(t, regDefault(t), []regCase{
		{"第 0 筆", 0x7522, regSP, "", KindMonster},
		{"第 1 筆", 0x755B, regSP, "", KindMonster},
		{"第 2 筆", 0x7594, regSP, "", KindMonster},
		{"第 3 筆是 TWNS.INT 緩衝區起點，界外", 0x75CD, regSP, "", KindOther},
		{"不在記錄起點", 0x7523, regSP, "", KindOther},
		{"記錄 1 起點前一個位元組", 0x755A, regSP, "", KindOther},
		{"記錄 1 起點後一個位元組", 0x755C, regSP, "", KindOther},
		{"第 0 筆前一個位元組", 0x7521, regSP, "", KindOther},
		{"與 overlay 無關", 0x755B, regSP, "ov2.test", KindMonster},
	})
}

func TestRegionsTown(t *testing.T) {
	// 8071h + 0123h × i（i = 0 至 11），手算並以十進位核對：
	// 32881 + 291 × i，i = 11 為 36082 = 8CF2h；i = 12 為 36373 = 8E15h。
	starts := []uint16{0x8071, 0x8194, 0x82B7, 0x83DA, 0x84FD, 0x8620, 0x8743, 0x8866, 0x8989, 0x8AAC, 0x8BCF, 0x8CF2}
	var cases []regCase
	for i, s := range starts {
		cases = append(cases,
			regCase{"城鎮起點 i=" + strconv.Itoa(i), s, regSP, "", KindTown},
			regCase{"城鎮起點後一個位元組 i=" + strconv.Itoa(i), s + 1, regSP, "", KindOther},
			regCase{"城鎮起點前一個位元組 i=" + strconv.Itoa(i), s - 1, regSP, "", KindOther},
		)
	}
	cases = append(cases,
		regCase{"i=12 是界外", 0x8E15, regSP, "", KindOther},
		regCase{"與 overlay 無關", 0x8194, regSP, "ov1.test", KindTown},
	)
	regRun(t, regDefault(t), cases)
}

func TestRegionsOther(t *testing.T) {
	var cases []regCase
	for _, ov := range []string{"", "ov1.test", "ov2.test"} {
		cases = append(cases,
			regCase{"玩家記錄內的名字欄", 0x5CBE, regSP, ov, KindOther},
			regCase{"玩家記錄陣列起點", 0x5BB0, regSP, ov, KindOther},
			regCase{"名冊 8E0C", 0x8E0C, regSP, ov, KindOther},
			regCase{"名冊 8F26", 0x8F26, regSP, ov, KindOther},
			regCase{"名冊 A648", 0xA648, regSP, ov, KindOther},
			regCase{"名冊 A762", 0xA762, regSP, ov, KindOther},
			regCase{"名冊 A87C", 0xA87C, regSP, ov, KindOther},
			// 位置描述區 [C94D, CA65)：OUT*.DAT 讀進 DS:C6FA，偏移 253h 起 7 筆 40 bytes（docs/spec/003 §12 第 6 項）。
			regCase{"位置描述 C99D", 0xC99D, regSP, ov, KindBuffer},
			regCase{"位置描述 C9C5", 0xC9C5, regSP, ov, KindBuffer},
			regCase{"位置描述 CA3D", 0xCA3D, regSP, ov, KindBuffer},
			regCase{"位置描述區起點 C94D", 0xC94D, regSP, ov, KindBuffer},
			regCase{"位置描述區前一格 C94C", 0xC94C, regSP, ov, KindOther},
			regCase{"位置描述區末位元組 CA64", 0xCA64, regSP, ov, KindBuffer},
			regCase{"位置描述區界外 CA65", 0xCA65, regSP, ov, KindOther},
			regCase{"空指標區之後的未知位址", 0x4000, regSP, ov, KindOther},
		)
	}
	regRun(t, regDefault(t), cases)
}

func TestRegionsParseLegal(t *testing.T) {
	src := "# 註解\n\n" +
		"kind\tstart\tstride\tcount\tnote\n" +
		"# 中間的註解\n" +
		"static\t0\t0\ta\t十六進位 a 個位元組\n" +
		"town@ov9\t100\t20\t3\t三筆\n" +
		"buffer\tFFF0\t0\t10\t到位址空間末端\n"
	tb, err := ParseRegions([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	regRun(t, tb, []regCase{
		{"static 內", 0x0009, 0, "", KindStatic},
		{"static 界外", 0x000A, 0, "", KindOther},
		{"ov9 的第 0 筆", 0x100, 0, "ov9.ovr", KindTown},
		{"ov9 的第 1 筆", 0x120, 0, "ov9.ovr", KindTown},
		{"ov9 的第 2 筆", 0x140, 0, "ov9.ovr", KindTown},
		{"ov9 的第 3 筆界外", 0x160, 0, "ov9.ovr", KindOther},
		{"ov9 不在起點", 0x101, 0, "ov9.ovr", KindOther},
		{"沒載入 ov9", 0x100, 0, "", KindOther},
		{"載入別的 overlay", 0x100, 0, "ov1.test", KindOther},
		{"區間到 FFFF（含）", 0xFFFF, 0, "", KindBuffer},
		{"區間起點", 0xFFF0, 0, "", KindBuffer},
		{"區間前一個位元組", 0xFFEF, 0, "", KindOther},
	})
	// 十六進位大小寫皆可。
	if _, err := ParseRegions([]byte("kind\tstart\tstride\tcount\tnote\nstatic\tabcd\t0\tAB\t\n")); err != nil {
		t.Errorf("大小寫混用的十六進位：%v", err)
	}
	// 記錄陣列最後一筆剛好到 FFFF 也合法。
	if _, err := ParseRegions([]byte("kind\tstart\tstride\tcount\tnote\ntown\tFF00\tFF\t2\tx\n")); err != nil {
		t.Errorf("陣列末筆起點 FFFF：%v", err)
	}
}

// 優先序固定在程式內，與檔內列序無關。
func TestRegionsPriorityIndependentOfFileOrder(t *testing.T) {
	h := "kind\tstart\tstride\tcount\tnote\n"
	a := h + "static\t0\t0\t100\t-\nbuffer\t50\t0\t10\t-\n"
	b := h + "buffer\t50\t0\t10\t-\nstatic\t0\t0\t100\t-\n"
	for i, src := range []string{a, b} {
		tb, err := ParseRegions([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		regRun(t, tb, []regCase{
			{"重疊處取 buffer（檔案順序 " + strconv.Itoa(i) + "）", 0x55, 0, "", KindBuffer},
			{"只在 static 內", 0x70, 0, "", KindStatic},
			{"buffer 末位元組", 0x5F, 0, "", KindBuffer},
			{"buffer 界外回 static", 0x60, 0, "", KindStatic},
		})
	}
}

func TestRegionsParseErrors(t *testing.T) {
	h := "kind\tstart\tstride\tcount\tnote\n"
	cases := []struct {
		name string
		data string
		line int
	}{
		{"空資料", "", 1},
		{"只有註解", "# a\n# b\n", 3},
		{"只有欄名列", h, 2},
		{"欄名不符", "kind\tstart\tstride\tcount\n", 1},
		{"註解之後欄名不符", "# a\nkind\tstart\n", 2},
		{"欄數 4", h + "static\t0\t0\t10\n", 2},
		{"欄數 6", h + "static\t0\t0\t10\tn\tx\n", 2},
		{"未知 kind", h + "static\t0\t0\t10\tn\nstack\t0\t0\t10\tn\n", 3},
		{"kind 是 other", h + "other\t0\t0\t10\tn\n", 2},
		{"空的 overlay 字樣", h + "static@\t0\t0\t10\tn\n", 2},
		{"大寫的 overlay 字樣", h + "static@OV1\t0\t0\t10\tn\n", 2},
		{"start 帶 0x", h + "static\t0x10\t0\t10\tn\n", 2},
		{"start 非十六進位", h + "static\tzz\t0\t10\tn\n", 2},
		{"start 為空", h + "static\t\t0\t10\tn\n", 2},
		{"start 帶負號", h + "static\t-1\t0\t10\tn\n", 2},
		{"start 帶正號", h + "static\t+1\t0\t10\tn\n", 2},
		{"start 帶底線", h + "static\t1_0\t0\t10\tn\n", 2},
		{"stride 非十六進位", h + "town\t0\tx\t10\tn\n", 2},
		{"count 非十六進位", h + "static\t0\t0\tg\tn\n", 2},
		{"count 為 0", h + "static\t0\t0\t0\tn\n", 2},
		{"start 超過 FFFF", h + "static\t10000\t0\t1\tn\n", 2},
		{"count 超過 10000", h + "static\t0\t0\t10001\tn\n", 2},
		{"區間超出位址空間", h + "static\tFFF0\t0\t20\tn\n", 2},
		{"陣列超出位址空間", h + "town\tFF00\t100\t2\tn\n", 2},
		{"含 CR", "kind\tstart\tstride\tcount\tnote\r\nstatic\t0\t0\t1\tn\n", 1},
	}
	for _, c := range cases {
		tb, err := ParseRegions([]byte(c.data))
		if err == nil {
			t.Errorf("%s：應回 error", c.name)
			continue
		}
		var te *TSVError
		if !errors.As(err, &te) {
			t.Errorf("%s：error 不是 *TSVError：%v", c.name, err)
			continue
		}
		if te.Line != c.line {
			t.Errorf("%s：行號 %d，要 %d（%v）", c.name, te.Line, c.line, err)
		}
		if tb != nil {
			t.Errorf("%s：出錯時表應為 nil", c.name)
		}
	}
	// 邊界合法：區間剛好到 10000（不含）、count 剛好 10000。
	if _, err := ParseRegions([]byte(h + "static\tFFF0\t0\t10\tn\n")); err != nil {
		t.Errorf("區間尾端剛好 10000：%v", err)
	}
	if _, err := ParseRegions([]byte(h + "static\t0\t0\t10000\tn\n")); err != nil {
		t.Errorf("count 10000：%v", err)
	}
}
