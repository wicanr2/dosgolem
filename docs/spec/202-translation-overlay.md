# 202 — 轉譯疊字層（`xlate` 套件）

狀態：**READY**
日期：2026-09-17
前置：[`200-hook-register-write.md`](200-hook-register-write.md)、[`201-step-actions.md`](201-step-actions.md)
來源：`psychic_war_cht` `docs/spec/008`、`009` 的雛形（原本在該 repo `apps/psychicwar/overlay`，驗收見其 `docs/re/018`）

---

## 1. 問題

中文化要在原版畫字的那一刻，把一行英文換成中文。原版照常執行、照常畫英文，轉譯層在放大畫布上的同一個位置蓋上中文。
這套機制與遊戲無關：**排版、定色、判斷疊字是否還有效、捲動、畫字**換一款遊戲照樣成立；
「哪個位址是印字常式、字串來源是哪個檔哪個偏移」才是遊戲專屬的，由呼叫端提供。

## 2. 規格（套件 `github.com/wicanr2/dosgolem/xlate`）

### 2.1 字型

- `type Font struct { W, H int; Glyphs map[rune][]byte; Name string }`：每字 H 列、每列 `(W+7)/8` bytes，MSB 在左。`Name` 由呼叫端設定，快照以它記字型（`LoadFont` 回傳空字串）。
- `LoadFont(path)`：檔案格式 `GOLEMFNT`（8 bytes）、u16 W、u16 H、u32 字數，之後每字 u32 碼點 ＋ u8 來源（呼叫端自訂）＋ 字模。
  長度對不上、magic 不對回錯。每字的來源位元組讀過即丟，不保存。

### 2.2 排版

- `Layout(text string, widths []int) ([][]rune, error)`：一個字元一格，依序填滿各行，`\n` 強制換行；放不下回 `ErrTooLong` 與截掉後的結果。

### 2.3 疊字

`type Stamp struct`：

| 欄位 | 意義 |
|---|---|
| `Key` | 呼叫端的識別（例：文本檔 key） |
| `X, Y` | 左上角，原版像素 |
| `Cells`、`CellW`、`CellH` | 格數與每格原版像素 |
| `Font *Font`、`GlyphX, GlyphY` | 字型與字模在放大後的格內的偏移（放大後像素） |
| `Text []rune` | 這一行要畫的字元 |
| `Transparent []bool` | 透明格：不填背景、不畫字、不列入定色與指紋（長度可短於 `Cells`，缺的當 false） |
| `State` | `Printing`（原版還在畫）、`Pending`（等定色）、`Shown` |
| `Transparent` 會被 `Frame` 與 `Add` 就地改寫 | 被別的東西蓋住的格子自動標成透明 |
| `FG, BG` | 定色後的 RGB |

`type Layer struct { Stamps []*Stamp; OnDrop func(*Stamp, string); W, H int }`（W、H 是原版畫面大小，0 當 320×200）：

- `Add(s)`：舊疊字被 s 的矩形**整個蓋住**才移除（原因 `overlap`）；只蓋到一部分時，被蓋到的格子標成透明
  （不畫、不列入指紋），狀態回到 `Pending` 讓剩下的格子重新定色；全部格子都透明時才移除。
  原版會在一個框旁邊開另一個框（輸入檔名的面板蓋住訊息框右半），整筆移除會讓沒被蓋到的那半露出原版英文。
- `Scroll(x0, y0, x1, y1, dy)`：左緣與右緣都在 `[x0, x1]`、上緣在 `[y0, y1)` 的疊字 Y ＋dy；移出框的移除（`scroll`）。橫向只超出一部分的疊字（例如另一套字型、位置剛好與框重疊的區塊）不搬。
- `Frame(indexed, rgb []uint8)`：`Pending` 的取矩形內非透明格的色號，出現最多是背景、第二多是前景（只有一種時兩者相同），
  RGB 取該色號在矩形內第一次出現的位置，**每一格各記一個 FNV-1a 64 位元指紋**，轉 `Shown`；
  `Shown` 的**逐格**比對：某一格連續 3 次不同就把那一格標成透明（中間恢復一次就重新計數），
  全部格子都透明時整筆移除（`changed`）。
  逐格而不是逐行的理由：原版會在一行的一部分上面畫別的東西（輸入面板蓋住訊息框右半、狀態欄的值改寫標籤旁邊），
  整行失效會讓沒被蓋到的部分露出原版英文。
  - **錨定格**：定色時該格的原版像素不只一種色號（＝那一格有原文的墨跡）。定色時一起記下來。
    錨定格**全部**失效時整筆移除（`anchors`），不等其餘格子。
    理由：疊字存在的前提是「原版那段文字還在畫面上」，而這件事只有壓在墨跡上的格子看得出來。
    中文比原文寬時，多出來的格子壓在純色背景上，指紋永遠不變——原版把整段文字清掉之後，
    這些格子仍然「沒變」，於是整行中文瓦解成幾個留在畫面上的孤字。
    （實例：道具頁翻頁後藍色面板已清空，畫面上仍留著「上」「經」兩個字，
    因為「上限」「經驗」的錨定格都失效了，而第一格壓在純藍背景上。）
    沒有任何錨定格的疊字（整塊都是純色）照舊逐格判斷。
- `Draw(dst []uint8, scale int, missing func(rune)) bool`：dst 是放大後 RGBA（寬 W×scale）。每筆 `Shown`：非透明格填背景色；每格的字模以前景色畫在 `(格左上 ＋ GlyphX, 格左上 ＋ GlyphY)`；
  字模每個點畫成 `k×k`，k 由呼叫端透過 `GlyphScale` 決定（§2.4）。字型沒有的字呼叫 `missing`；半形與全形空白不畫。
  `GlyphX`／`GlyphY` 是放大後像素（呼叫端自己乘上 scale 的比例）；超出格子寬或高的點都不畫。
- `Stamp.Owner string`：建立這一筆的 watcher 的 Key（`203-baked-text-watchers`），一般疊字是空字串；快照要保存。
- `Frozen func(*Stamp) bool`（可為 nil）：回 true 的疊字在這次 `Frame` 不定色、不檢查指紋、失效計數不變。原版正在逐步搬動那一塊畫面時（訊息框捲動），中間狀態不能拿來判斷失效。
- `Snapshot() ([]byte, error)`、`Restore([]byte, fonts map[string]*Font) error`：疊字層存成 JSON（字型以 `Font.Name` 記），給逐步操作（規格 `201`）跨步保留。
  指紋與連續不同次數也一起存，否則還原後第一次 `Frame` 會誤判成畫面變了。`OnDrop` 不存（是呼叫端的 hook）。快照裡的字型名稱在 `fonts` 找不到時整批回錯。

### 2.4 字模放大倍率

`Stamp.GlyphScale int`：字模一個點在放大後畫布上佔幾個像素。0 表示 `scale / 3`（原本的 24 點字型配 8 像素字格、放大 3 倍）。
呼叫端保證「格寬 × scale ≥ GlyphX ＋ 字寬 × GlyphScale」；超出的點不畫。

### 2.5 逐字迴圈的換行判斷

`type LineTracker struct`：`Hit(lin uint32, remain int, useRemain bool) (newLine bool)`。
印字常式常以迴圈頭逐字命中；位址等於上一次 ＋1（`useRemain` 時另外要 remain 等於上一次 −1）就是同一行，否則是新的一行。

## 3. 驗收

1. 從 `psychic_war_cht` 搬來的測試全部通過：排版（7 字、18 字、32 字過長、`\n`）、捲動（右緣超出框的不搬）、重疊（整個蓋住才移除、部分重疊只遮格子）、3 幀逐格失效與恢復計數、定色、重疊移除、畫字與缺字回呼。
2. `GlyphScale` 與 `GlyphX／GlyphY`：16×15 字型、6×7 字格、scale 3、GlyphScale 1、偏移 (1,3)：字模左上點畫在格左上 ＋(1,3)；一格右緣之外不畫。
3. `LoadFont`：寫一個 2 字的 16×15 字型再讀回，字模相同；magic 錯、長度錯回錯。
4. `Snapshot`／`Restore` 往返後 `Draw` 出的 RGBA 逐位元組相同。
5. `LineTracker`：位址連續＋remain 遞減是同一行；remain 不遞減（`useRemain`）是新行；位址跳號是新行。
6. 透明格：透明格的色號不影響定色；只改透明格不會失效，改非透明格 3 次後失效（反向對照）；`Draw` 不碰透明格的像素。
7. `Frozen`：凍結期間指紋不同也不累計；解除後內容相同就繼續顯示。反向對照：不凍結時 3 次後失效。

## 4. 不做

- 印字常式的位址、字串來源換算、文本檔格式（遊戲專屬）。
- 可變寬字型、字距調整、雙向文字。
