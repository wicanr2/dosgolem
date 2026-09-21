# 218 — Buck Rogers 手冊正式字型子集輸入契約

狀態：**DRAFT**  
日期：2026-09-21  
前置：[`202-translation-overlay.md`](202-translation-overlay.md)、
[`217-buck-rogers-manual-multiline-presenter-core.md`](217-buck-rogers-manual-multiline-presenter-core.md)、
專案 `font/README.md`、規格 005 與第八十八階段字型稽核。

## 目的與邊界

本規格界定手冊 presenter 所需 16×16 GOLEMFNT 子集的**本機輸入與驗證**，而非採用任何特定
第三方字型、更不是公開散布的授權判定。它只處理正式繁中 catalog 的 glyph coverage、可重生
輸入、雜湊、實際授權告知、GOLEMFNT 格式回讀與失敗即關閉；不得改動 DOS、VRAM、輸入、答案、
存檔、host UI 或 `RuntimeManualOverlay` 的 lifecycle。

第三方字型原始檔、完整授權文字與生成的 GOLEMFNT 一律位於專案被忽略的 `workplace/`，不得加入
dosgolem 或專案 Git、GitHub Issue、Release 或可散布測試語料。此規格的 future manifest 只能保存
檔名、版本、SHA-256、授權檔 SHA-256、通知要求與 coverage 結果，不可保存字模本體。

## 已證實輸出與格式契約

- 專案正式 `manual.zh-TW.tsv` 已決定性重生 691 個 Unicode 碼點；字元清單 SHA-256 為
  `dc656f0729ac3c02abe691d463e62454d1505fbe4d8122aa6056822332a6667f`。這是 glyph 集合，
  不是原版或手冊全文。
- spec 217 的 `RuntimeManualOverlay` 只接受 16×16 `xlate.Font`，每 rune 必有 32-byte glyph；缺字
  時 constructor 整體失敗，沒有 partial display。
- spec 202 的 `GOLEMFNT` 是 `GOLEMFNT` magic、little-endian u16 width／height、u32 glyph count，
  接著每 glyph 為 u32 code point、u8 source tag、32-byte bitmap。`xlate.LoadFont` 會拒絕 magic 或
  檔案長度不符；它故意丟棄 source tag，故 format 本身不能證明字型來源、版本或授權。
- 既有 `tools/catalog_font.py` 只接受 Unifont `.hex`／`.hex.gz` 的 8×16 或16×16字模，將 8×16
  置中為 16×16，並對缺字、重複 code point、非十六進位與其他尺寸失敗即關閉。

## 第八十八階段盤點（已證實）

2026-09-21 在 Docker、唯讀 `workplace/` 下，以 `*.hex`、`*.hex.gz`、`*.bdf`、`*.pcf`、`*.ttf`、
`*.otf`、`COPYING*`、`LICENSE*`、`OFL.txt` 搜尋。除了 dosgolem 自己的 `LICENSE` 之外，沒有可供
手冊字型建立的原始字型檔或其實際授權文字。

已有十份被忽略的 `.golemfnt` 產物，皆為 16×16，但 glyph count 分別是 24、24、56、77、74、5、47、
81、8、15；最大 81，低於 691。它們也沒有可回查的原始輸入或對應授權檔，不能合併、擴張或當成
正式候選。這是「候選缺席」的已證實結果，不是某個字型缺少特定字的推測。

## DRAFT manifest 與驗收契約

將來只有在使用者提供或明確授權取得一份候選後，才可於 `workplace/phase88/input/` 保留原始檔與
完整授權文字，並建立 content-free manifest。manifest 至少包含：

1. source filename、發行版本／日期、source SHA-256、format、16×16 conversion rule；
2. 實際授權檔 filename／SHA-256、必要 attribution／embedding notice，及「本機驗證」與「可散布」
   分開的枚舉狀態；
3. 691-code-point list SHA-256、found／missing count、output SHA-256、GOLEMFNT width／height／count；
4. 任何缺輸入、缺授權告知、雜湊漂移、缺 glyph、重複 glyph、非法尺寸、header／length 漂移的
   fail-closed 結果。

僅在實際 candidate、完整告知與 691／691 回讀都符合後，本規格才可由 DRAFT 升為 READY，並只授權
本機子集 build。是否把含該字型的產物放入可散布包，仍是獨立的權利／發行決策；不得由 READY 或
本機 build 推定。

## 目前停止線

目前沒有可核對候選，故禁止新增 source manifest、GOLEMFNT、runtime flag 或 presenter 接線。既有
fixture font 與舊的未追蹤子集只可維持測試／歷史輸出，不能填補此缺口。此 DRAFT 不構成法律意見，
也不宣稱 GNU Unifont 或任何其他字型已被本專案採用。

第九十二階段已在專案根目錄的 `tools/catalog_font.py` 完成 `validate-candidate` 候選審查工具：它可於
使用者提供本機輸入後檢查 strict manifest、source／license SHA-256、既有 Unifont parser coverage 與
本機驗證／發行未定狀態，不寫 GOLEMFNT。這只縮小未來輸入驗證的機械缺口，沒有建立 manifest instance、
沒有 candidate source／完整授權文字，也不改變本 spec 的 DRAFT、停止線或採用／散布決策。
