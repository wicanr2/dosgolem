# 234 — 共用疊字層的實體像素字首計畫

狀態：**READY**（共用 API 契約；實作與《拯救地球》adapter 均未宣稱完成）
日期：2026-09-25
前置：[202 — 轉譯疊字層](202-translation-overlay.md)；《拯救地球》手冊混排輸入仍依該專案規格 005 與 39 段詞界收據審查。

## 1. 範圍與不變條件

在共用 `xlate` 擴充可選的實體像素字首位置，讓 3× 畫布可以精確表達 14px 英文字首 advance。這不是另一個 Buck 專用繪字層，也不改原版 indexed framebuffer、DOS 狀態、現有格子定色／失效／清除規則。

規格 202 的固定格 `Stamp` 不提供可變字距；本規格只增加一條可選的字模繪製分支。沒有新欄位的舊 stamp、舊 JSON 快照及 2× RGBA 必須逐位元保持原樣。`Cells`、`CellW`、`CellH`、`Transparent`、指紋與錨定格仍是唯一清除及存活判準。

## 2. 候選資料契約

- `Stamp.PixelScale int` 與 `Stamp.PixelGlyphs []PixelGlyph` 共同啟用實體像素分支；`PixelScale` 必須等於呼叫 `ValidatePixelGlyphPlan(scale)` 或 `DrawChecked(..., scale, ...)` 所傳入的 `scale`，且 `scale > 0`。一方存在而另一方缺席、空清單、同時給 `Text`，一律拒絕。
- 新型別固定為 `PixelGlyph{Rune rune, Font *Font, SrcX, SrcY, SrcW, SrcH int, X, Y int}`。`SrcX/SrcY` 是來源左上角、`SrcW/SrcH` 是非零寬高，來源半開矩形為 `[SrcX, SrcX+SrcW) × [SrcY, SrcY+SrcH)`；crop 中一個 source pixel 對應最終畫布一個 physical pixel，不再乘倍率。`X/Y` 是最終 RGBA 畫布的**絕對實體像素座標**，不是 stamp 相對或原版 logical 座標。建立與封存時，`Font` 必須能在 registry 以名稱及同一指標找到；還原 JSON 時改以名稱解出字型，再核對快照保存的 canonical 字型 bytes SHA-256，不以跨序列化指標作身分證據。`Font.Glyphs[Rune]` 必須存在且 byte 長度完整，來源 crop 非空並位於 `Font.W/H`。
- 目的地 crop 矩形（不只已著墨點）的完整 bbox 必須位於 `Stamp.Rect() × PixelScale`；**physical stamp 的整個 parent `Stamp.Rect()` 也必須落在 `Layer.W/H` 畫布內**（0 尺寸沿用規格 202 的 320×200），不能只驗 glyph 在 parent 內。字型 `W/H`、`rowBytes=ceil(W/8)`、`glyphBytes=H×rowBytes`、parent rect、畫布倍率與目的座標的加乘皆須先做正值與溢位檢查，不能用可溢位的 `W+7` 或 `H*rowBytes` 直接驗 bitmap 長度。公開介面固定為 `func (l *Layer) ValidatePixelGlyphPlan(scale int) error` 及 `func (l *Layer) DrawChecked(dst []byte, scale int, missing func(rune)) (bool, error)`。`DrawChecked` 於任何 `dst` 寫入前預檢**全層**，正式封存／owner 僅走 checked 路徑。既有 `Draw(... ) bool` 遇到任一實體像素 stamp 時，整層不寫入並回 `false`；沒有實體像素 stamp 時維持原分支及 bytes。不得沿用「逐 stamp 填 BG 後才發現壞 glyph」的既有次序。
- `Draw` 在實體像素分支只覆寫格子背景以後的字模位置；每一個待畫墨點按對應原版邏輯格查 `Transparent`，透明格絕不落墨。部分 `Add`／`Clear`／原版三幀變更及錨定失效維持規格 202 語意。
- Snapshot 對新 glyph 記字型名稱與 canonical bytes SHA-256，另記 rune、crop、目的座標、`PixelScale`；新增 JSON 欄位皆為 optional，舊快照位元組不得增加空欄位。Restore 先完整驗證 list／倍率／`Text` 互斥、字型及來源字模、crop／座標／溢位，再原子替換 layer；缺字型、同名但 bytes 不同、錯 crop／座標時，舊 layer 保持不變。跨倍率投影另由 checked preflight 拒絕。
- canonical 字型雜湊固定為 SHA-256：先寫 ASCII domain `xlate-font-v1\x00`，再依序寫 `Font.W`、`Font.H` 的小端 u32 及 glyph count 的小端 u64；每個 glyph 依 Unicode 碼點升序，寫碼點小端 u32、bitmap 長度小端 u64、完整 bitmap bytes。負尺寸、超過 u32 或 bitmap 長度不等於 `H × ceil(W/8)` 時拒絕，不產生雜湊。不得直接雜湊 `GOLEMFNT` 原檔，因 `ParseFont` 不保留每字來源位元組，也不得依 Go map 迭代順序產生不同雜湊。`Font.Name` 是 registry key，另存於快照，不代替 bytes 身分。
- 已封存的顯示群組須同時綁定所有使用的字型身份及 bytes 指紋；封存前、還原後、投影前均驗。任何變造、跨倍率重用或 stale owner ticket 都不得產生像素。

上述型別、座標基準、1:1 crop、雜湊編碼與 checked 介面是本規格的固定 API 契約；不得把 test-local helper 當成正式實作。

## 3. READY 前驗收矩陣

1. 在 3× 字型上驗 14px 英文字首、22px 中文字模及裁切括號；所有 bbox、逐像素透明格與跨 run 墨跡碰撞須通過。缺字、負 crop、**parent 超出畫布**、字型 `rowBytes/glyphBytes` 加乘溢位與目的座標溢位皆須在全層預檢失敗、零輸出。
2. 原有固定格 `Draw`、Snapshot JSON 與 2× RGBA 的 golden bytes 不變；舊 JSON 能原樣 Restore。
3. 新快照往返後畫面逐位元相同；source／seal 拒絕同名異指標，Restore 拒絕缺 font 或同名異 bytes hash；字型 bytes 變造、無效 crop／座標的 Restore 原子失敗，錯倍率由 checked preflight 拒絕。
4. 部分重疊、部分 Clear、三幀變更、錨定格全失效與重新定色，不漏字模、不讓舊字復活。
5. 共用封存群組的多字型身分、generation／epoch、跨倍率拒絕皆有正反測試；變造後不得投影。這一項只驗共用層，不冒稱遊戲 owner 已完成。

《拯救地球》adapter 另有**獨立 DRAFT 閘門**：全 39 段 immutable 輸入計畫須通過原文空白 source-span round-trip、無行首／行尾可見空白、標點及括號禁則、14px 識別字不拆、墨跡互不碰撞、全部落在安全矩形。其 `ManualSnapshotOwner` 的 layout hash、epoch／ticket、清層／換題／還原／stop 失效，以及首題／換題／返回的 3× E1 同狀態收據、2× 固定 checkpoint 新版回歸，均須在 adapter 規格審查及實作後另行驗收；**不作為本共用 API 升 READY 的前提**。

## 4. 目前證據與停止線

`xlate/pixel_glyph_draft_test.go` 只以 test-local helper 證明基本 14px placement、crop、透明格及部分壞計畫拒絕；正式共用 API、封存與 Snapshot／Restore 尚未實作。**READY 只表示本文件的共用 API 契約經獨立審查，可開始實作；不是 CONFORMED。** Buck tokenizer／owner 與同狀態驗收是另一個 adapter 分支，不能由此規格的狀態冒稱完成。原有 39 段收據的行界空白與 `…` 禁則已在後續 test-local 版修正，仍待正式 immutable plan 接線及獨立驗收。

實作初次獨立審查追加勘誤：只驗 glyph 落在 parent 內仍會讓
parent 背景迴圈跑出畫布；直接計算 `H*rowBytes` 可能溢位。
本規格已明列 parent 必須在畫布內、字型尺寸加乘先安全驗證。
這不推翻 14px 實體字首架構，但現有 production slice 修正前
僅算部分實作，不能升 CONFORMED。

## 5. 2026-09-25 共用核心與封存投影實作進度

本機分支 `8f56d0e` 已加入 `xlate` 的可選 physical glyph、
全層 `DrawChecked` 預檢、舊 `Draw` 零寫入拒絕、optional
Snapshot／原子 Restore 及 canonical 字型 SHA-256；上節
parent 畫布與字型尺寸溢位缺口已修，legacy 2× JSON／RGBA
bytes、14px 位置、跨倍率拒絕與負例有正式定向測試。

本機分支 `1e05ff6` 進一步讓 `presentation.SealedLayerGroup`
在封存前逐一核對 physical glyph 的來源字型指標，使用呼叫者
提供的 canonical registry 建立私有 Snapshot，不修改來源
layer。投影前以 `ValidatePixelGlyphPlan` 驗全部私有層，並用
`DrawChecked` 繪製；錯倍率或錯字型在讀原版影格前拒絕，
不交付部分 RGBA。舊單層 `LayerSnapshotProvider` 明確拒絕
physical glyph，不再靜默略過。無原版素材的雙字型 14px
封存投影與拒絕負例、既有手冊 owner 合成測試、
`go test -race ./presentation ./xlate` 均已通過。

這只證**共用元件**已能安全承載像素字首；Buck 手冊
immutable layout、14 行 owner 身分、正式 E1 同狀態對拍、
生命週期完整矩陣與 Linux 玩家 session 仍未完成。另由本機
`5fe9707` 的無原版素材測試證實：實體 glyph 在部分 Clear 後
透明格不再著墨，原版格連續三幀變動後整筆移除且無殘字；
這尚未涵蓋部分 Add／錨定格與真實手冊 owner。規格維持 READY，
不以局部綠燈改稱 CONFORMED。

本機 `2702c56` 再補正式合成回歸：physical glyph 的部分
`Add` 僅遮中間邏輯格、錨定格全部失效時整筆清除且無殘字，
sealed group 的 generation／epoch 被改動時在讀影格前拒絕。
主代理獨立重跑 `go test -race ./xlate ./presentation -count=1`
通過。這仍不代替正式手冊 adapter 的 39 段 plan、實際字型
及原版同狀態收據，規格維持 READY。
