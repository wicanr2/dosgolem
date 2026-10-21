# 453：xlate 反鋸齒圖集文字（AlphaFont）

狀態：**READY**（第 4 版：依陸空戰將規格 011 三輪審查修訂，2026-10-21 轉 READY），未實作
日期：2026-10-21
程式位置以 dosgolem-sc 分支 `strike-commander` `00be973` 為準。推論等級：已證實、強推論、假說、未知。

## 1. 為什麼放通用層

現行 `xlate` 只有 1 位元點陣字：`Font` 每字 `H × ⌈W/8⌉` bytes、MSB 在左（`xlate/font.go:17–26`），`drawGlyphs` 逐點以 `set` 寫入前景色，沒有混色（`xlate/layer.go:796–851`）。反鋸齒字需要 8 位元 alpha 遮罩與混色公式。

判準（`dosgolem/docs/spec/006`）：換一支 binary 之後這段程式碼還成立嗎。下列三項與遊戲無關，所以放 `xlate`：

- 圖集檔的格式與解析。
- 以 alpha 遮罩把一串字畫進 RGBA 緩衝（混色公式、字串遮罩的合成、裁切、外接框）。
- 字串寬度的計算。

遊戲專屬的部分（字級、版面、外框色、哪一條輸出路徑用哪個字級）留在 `apps/<app>/`。

範圍限制：本規格只新增獨立的型別與函式。`Stamp`、`Layer`、`drawGlyphs`、`Snapshot`／`Restore`、204 圖面、234 物理像素路徑都不改，既有遊戲（buckrogers、phantasie 等）的行為不變。

## 2. 圖集格式 `GOLEMAAF` 第 1 版

一個檔是一種字型的一個字級。全部 little-endian。

**檔頭**（56 bytes）：

| 位移 | 型別 | 欄位 | 條件 |
|---|---|---|---|
| 0 | 8 bytes | magic `GOLEMAAF` | 相同 |
| 8 | u16 | version | 1 |
| 10 | u16 | px：字級（像素） | 1 到 255 |
| 12 | i16 | ascent：em 框上緣在基線上方的像素 | 0 < ascent ≤ px |
| 14 | i16 | descent：em 框下緣在基線下方的像素 | ascent + descent = px |
| 16 | u16 | flags：bit 0 有外框遮罩，其餘位元為 0 | |
| 18 | u16 | stroke：外框半徑（像素）；flags bit 0 為 0 時必須是 0 | |
| 20 | u32 | count：字數 | ≥ 1 |
| 24 | 32 bytes | source：來源字型檔的 SHA-256 | 不解讀，原樣保留 |

**每字**，依碼點嚴格遞增排列：

| 型別 | 欄位 | 條件 |
|---|---|---|
| u32 | 碼點 | ≤ `10FFFF`，不在 `D800`–`DFFF` |
| i16 | adv：前進寬（像素） | ≥ 0 |
| i16、i16 | fx、fy：填色遮罩左上角相對筆位（筆位在基線上；y 向下為正） | |
| u16、u16 | fw、fh：填色遮罩寬高 | 任一可為 0（空白字常見 `fw > 0、fh = 0`） |
| fw × fh bytes | 填色 alpha，列優先 | |
| 只在 flags bit 0：i16、i16、u16、u16、sw × sh bytes | 外框遮罩：sx、sy、sw、sh 與 alpha | 同上 |

- 外框遮罩是實心描邊（描邊加上字形內部；Pillow `stroke_filled=True`），不是中空環。格式本身不檢查這點，由產生圖集的工具保證（規格 011 §3.2.3）。
- 遮罩長度 `fw × fh`、`sw × sh` 以 64 位元整數計算再與剩餘長度比較，不得溢位。
- 解析嚴格：每讀一個欄位或遮罩之前先檢查剩餘長度，長度不足才配置之前就回錯（不以宣告的 count、寬高預先配置超出輸入長度的記憶體）；尾端多餘 bytes、碼點未遞增或重複、條件不符，一律回錯，不回部分結果。
- 遮罩是 8 位元覆蓋率（0 透明、255 完全覆蓋），不含顏色。
- 格式不含字距調整與字形替換。字串的字形位置只由逐字前進寬決定（§3.2）。

## 3. API（`xlate/alpha.go`，新檔）

```go
type AlphaGlyph struct {
	Adv            int
	FX, FY, FW, FH int
	Fill           []byte // len = FW × FH，列優先
	SX, SY, SW, SH int    // HasStroke 為假時全為 0
	Stroke         []byte // HasStroke 為假時為 nil
}

type AlphaFont struct {
	Name                string // 呼叫端標記，不在檔案內（同 Font.Name）
	Px, Ascent, Descent int
	HasStroke           bool
	StrokeRadius        int
	Source              [32]byte
	Glyphs              map[rune]*AlphaGlyph
}

func ParseAlphaFont(b []byte) (*AlphaFont, error)
func LoadAlphaFont(path string) (*AlphaFont, error)

// Advance 回字串的前進寬總和；missing 依出現順序列出全部缺字（重複出現的字照列），缺字的前進寬記 0。
func (f *AlphaFont) Advance(rs []rune) (w int, missing []rune)

type AlphaLayer int

const (
	AlphaFill AlphaLayer = iota
	AlphaStroke
)

// AlphaRect 是半開矩形 [X0, X1) × [Y0, Y1)，輸出像素。X0 >= X1 或 Y0 >= Y1 為空。
type AlphaRect struct{ X0, Y0, X1, Y1 int }

// StringMask 算出字串在 layer 的合成遮罩（§3.2）。
// 回傳 m 的排列：寬 = box.X1 − box.X0、高 = box.Y1 − box.Y0、列優先，m[(y−box.Y0)*寬 + (x−box.X0)]。
// box 是裁到 clip 之後 alpha > 0 的外接框；字串沒有任何 alpha > 0 時 box 為空、m 為 nil。
// 有缺字時 m 為 nil、box 為空，missing 列出全部缺字。
// layer 為 AlphaStroke 而字型沒有外框層時 m 為 nil、box 為空、noStroke 為真（不畫，呼叫端記錄）。
func StringMask(f *AlphaFont, rs []rune, x, baseline int, layer AlphaLayer, clip AlphaRect) (m []byte, box AlphaRect, missing []rune, noStroke bool)

// BlendMask 把遮罩以顏色 c 混進 RGBA 緩衝 dst（寬 W、高 H 像素，H 由參數給，不由 len(dst) 推算）。
// len(dst) < 4·W·H，或 len(m) ≠ (box 寬)·(box 高)（空 box 時 m 必須為空）時完全不寫入。
// 只寫 box ∩ [0,W)×[0,H) 內 alpha > 0 的像素。
func BlendMask(dst []byte, W, H int, m []byte, box AlphaRect, c [3]uint8)

// DrawAlphaText = StringMask + BlendMask。回傳的 ink 是「alpha > 0 且在 dst（[0,W)×[0,H)）內」
// 的像素的緊外接框；沒有寫入任何像素時回空框（四欄皆 0）。其餘回傳值同 StringMask。
func DrawAlphaText(dst []byte, W, H int, f *AlphaFont, rs []rune, x, baseline int, layer AlphaLayer, c [3]uint8, clip AlphaRect) (ink AlphaRect, missing []rune, noStroke bool)
```

### 3.1 混色公式

每個 alpha `a` > 0 的像素、每個色版：

```
dst = (c × a + dst × (255 − a) + 127) / 255     （整數除法）
```

`a = 0` 不寫（像素維持原值，含 alpha 位元組）；`a > 0` 時 alpha 位元組寫 255。

依據：255 是奇數，`x / 255` 不會剛好是 `.5`，`(x + 127) / 255` 就是最接近的整數，與 Pillow `Paste.c` 的 `DIV255` 相同。規格 011 §2 在 Pillow 11.3.0 比對 `Image.paste(color, mask)` 與本式，不符 0（本草稿 65536 組、審查 B 全部 256³ 組，已證實）。

### 3.2 字串遮罩

1. 缺字檢查：先以 `Advance` 檢查整串；有缺字就回 missing（全部缺字），不算遮罩。
2. 外框層：`layer == AlphaStroke` 而 `HasStroke` 為假，回 `noStroke = true`，不算遮罩。
3. 筆位 `px = x`。依序對每個字 `g`：遮罩左上角 `(px + g.FX, baseline + g.FY)`（外框層用 `SX`、`SY`、`SW`、`SH`、`Stroke`）；`px += g.Adv`。
4. 字串遮罩以**疊合式**逐字合成：遮罩初值全 0；每個字的每個像素 `a`，該位置 `m′ = (a × 255 + m × (255 − a) + 127) / 255`。
5. 裁到 `clip`，再縮成 alpha > 0 的外接框 `box`（§3 的排列）。

依據：Pillow 整串遮罩合成相鄰字形用的就是第 4 步的疊合式。以疊合式逐字合成，6 個字級、263 句、填色層與外框層對 Pillow 整串遮罩全部 0 不符；改用最大值合成，填色層每字級 1 到 8 句、外框層 216 到 259 句不符（規格 011 §2，審查 A 實測）。再經 §3.1 混色、且多串時先畫全部外框層再畫全部填色層，與 Pillow 兩趟 `ImageDraw.text` 的 RGB 逐位元組相同（規格 011 §2，6 個字級各 90 組 0 不符，已證實）。

### 3.3 不做的事

- 不做執行期向量光柵化（不連結 FreeType）。圖集由各專案的工具預先烘製，執行期只讀檔與整數運算，所以 Linux、Windows、macOS 的輸出逐位元組相同（強推論：整數運算、無浮點；規格 011 §4.1 第 12 項以固定輸入的雜湊測試把關）。
- 不做字距、連字、雙向文字、換行。換行由呼叫端以 `Advance` 自行實作。
- 不做 gamma 修正；混色在 sRGB 數值上直接做，與 Pillow 相同。

## 4. 驗收（`xlate` 單元測試，合成圖集）

測試在 Go 內以程式組出圖集 bytes，不讀任何字型檔。

1. 解析：最小合法檔（1 字、無外框）；有外框；`fw > 0、fh = 0` 的空白字；外框 `sw × 2` 全 0 的空白字。各錯誤條件各一例回錯：magic、version 2、`ascent + descent ≠ px`、flags 未定義位元、無外框但 stroke ≠ 0、碼點未遞增、碼點重複、代理對碼點、長度少 1、尾端多 1 byte、count 與實際不符、宣告寬高乘積大於剩餘長度（不得先配置）。
2. `Advance`：前進寬相加；缺字回 missing（同一缺字出現兩次就列兩次）、該字記 0。
3. 混色：`a = 0` 不改（`dst` 的 alpha 位元組初值設為 0，不是 255，才分辨得出是否被改寫）；`a = 255` 等於 `c`；`a = 128`、底 `(0,0,0)`、`c = (255,255,255)` 得 128；字色 `c = 255`、全部 65536 組 (底, a) 與式子逐值相同（測試內獨立寫出；`c = 255` 時去掉 +127 的式子有 31770 組不同，審查 B 第 3 輪實算，A1 因此必失敗）。
4. 字串合成：兩字遮罩在同一像素重疊，兩字 alpha 都是 3，字色 255、底 128（單一色版）：逐字各自混進 dst 得 130，疊合式得 131（字串遮罩 6），最大值得 129。斷言結果為 131，並在測試內另算 130、129 確認三者不同。
5. 外框層：`AlphaStroke` 取 `S*` 欄位與 `Stroke`（合成圖集的外框遮罩與填色遮罩形狀、位置都不同，誤取 `F*`／`Fill` 會得到不同像素）；`HasStroke` 為假時 `noStroke` 為真、`dst` 不變。
6. 裁切：clip 切掉遮罩一部分時，clip 外的 `dst` 不變；`box` 在 clip 內，且等於 alpha > 0 的外接框（不大於它）。clip 與 `dst` 範圍不交時不寫入、`ink` 為空框。字串一部分落在 `dst` 外時，`ink` 只含 `dst` 內的像素。`BlendMask` 的 `len(m)` 與 box 不符、`len(dst)` 不足時 `dst` 不變。
7. 缺字：字串含兩個不同缺字時 `dst` 完全不變，missing 依序列出兩個。
8. 回歸：`go test ./xlate/...` 既有測試全部照舊通過；`Stamp`、`Layer` 的公開簽名不變，以編譯期賦值斷言（例 `var _ func(*Layer, []uint8, int, func(rune)) bool = (*Layer).Draw`，`DrawWithBackground`、`ParseFont` 同法）確認。

### 4.1 負對照

| 編號 | 突變 | 必須失敗 |
|---|---|---|
| A1 | 混色改為 `(c·a + d·(255−a)) / 255`（無 +127） | 第 3 項 |
| A2 | 字串合成改為最大值 | 第 4 項 |
| A2a | 字串合成改為逐字各自混進 dst | 第 4 項 |
| A3 | `a = 0` 也寫入（alpha 位元組改 255） | 第 3 項 |
| A4 | 解析不檢查碼點遞增 | 第 1 項 |
| A5 | 不裁到 clip | 第 6 項 |
| A6 | 缺字時照畫其餘字，或 missing 只列第一個 | 第 7 項 |
| A7 | `box` 不縮到 alpha > 0 的外接框（直接回裁後的整個遮罩範圍） | 第 6 項 |
| A8 | 外框層誤用填色欄位（`F*`、`Fill`） | 第 5 項 |
| A9 | `BlendMask` 不檢查 `len(m)` | 第 6 項 |
| A10 | `ink` 不與 `dst` 取交集 | 第 6 項 |

## 5. 未知

- 其他 dosgolem 遊戲是否改用本 API，不在本規格；`Stamp` 的 1 位元路徑繼續存在。
- 若日後要把 alpha 字接進 `Layer`（例如 Snapshot），需另立規格。
