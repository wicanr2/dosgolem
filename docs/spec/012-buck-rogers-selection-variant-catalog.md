# Buck Rogers 種族反白 variant catalog

狀態：**CONFORMED**

本規格只核准把已由正常 Down／Up 路徑證實的三個新 selection display identity 加入既有
`MenuCatalog` 正式輸入。它不改 resolver 演算法、不繪圖、不送輸入，也不修改原版狀態。

## 證據輸入

- 既有九 identity `menu-events.tsv` SHA-256：
  `38bc0fa693fa5ee4bed3209548a0dc67c1270b28657f0801e42645b70fd370e9`。
- 四事件 lifecycle `race-selection-events.tsv` SHA-256：
  `08d58c956ecf3ca2f6ba9c2dc4cfab9654ebfac94e7b41f26baacbf9ba0dd213`。
- 繁中 catalog SHA-256：
  `ca3319830adb7b34a048b498d8d0466b38b8fb6f418e5244a3a467e77ea68077`。
- logical rectangle SHA-256：
  `2f1d4ff874c3976c0e26ca18450edcbb6cb177302bdacd5b2610ce76503fac00`。
- 動態位址一律為 dosgolem runtime `segment:offset`；四筆事件與 framebuffer 證據見專案
  `docs/re/phase-30-race-selection-highlight-lifecycle.md`。

## 新增 identity

正式 `menu-events.tsv` 在既有 sequence 1–9 後追加下列三個唯一 identity：

| sequence | event key | text key | len／SHA-256 | caller | bg/fg | row,col |
| ---: | --- | --- | --- | --- | --- | --- |
| 10 | `race.selection.normal.terran` | `race.terran` | 6／`237c59a3…345f419` | `37F1:1856` | 0/10 | 3,3 |
| 11 | `race.selection.selected.martian` | `race.martian` | 7／`72d567c5…442c96a` | `37F1:175D` | 15/0 | 4,3 |
| 12 | `race.selection.normal.martian` | `race.martian` | 7／`72d567c5…442c96a` | `37F1:1856` | 0/10 | 4,3 |

Up 的第四筆 selected Terran 必須解析既有 sequence 9 `race.heading.terran`；不得加入第二個
相同 identity。既有 event key 雖早期以 heading 命名，仍是穩定外部識別字，不因後續解出
selection 語意而破壞性改名。

所有新增 rows 沿用 `docs/spec/009-buck-rogers-menu-display-request.md` 的完整 exact identity、
TSV schema、唯一性與失敗即關閉規則。共用 text key 是明示設計：normal／selected variant
只改原版畫面 style，不改玩家看到的繁中名稱。

## lifecycle 對照

`race-selection-events.tsv` 增加 `event_key` 欄，四筆依序必須對應：

1. Down：`race.selection.normal.terran`
2. Down：`race.selection.selected.martian`
3. Up：`race.selection.normal.martian`
4. Up：既有 `race.heading.terran`

每一列必須以完整 identity 反查正式 inventory，不能只用 text key、role、色號、座標或順序。

## 舊收據相容性

Phase 27／29 的 Enter-only 收據仍必須恰有九筆，並逐筆對應 inventory sequence 1–9；verifier
不得因 inventory 擴充而要求 12 筆，也不得接受少於或多於九筆。正式 catalog 載入則必須看見
全部 12 個唯一 identity。

## 真實執行驗收

以 spec 011 的相同 Enter→Down→Up 固定排程、#100,600,000 終點，啟用正式 catalog：

- 13 個完成事件依序產生 13 個 `DisplayRequest`；
- request 1–9 對應 inventory 1–9；request 10–13 對應上述四步 lifecycle；
- 零 recorder drop、零 pending、零 catalog miss；
- request 收據只含 event key、text key 與譯文字數，不含英／中文全文；
- 任一新 identity 欄位或 lifecycle event key 被修改時，載入或 verifier 失敗。

全部正式 packages test／vet、相關 race detector 與專案 verifier 通過後，本規格可標為
CONFORMED。玩家可見 renderer 與 2×／3× 決策仍不在本規格範圍。

## 符合性紀錄（2026-09-20）

- 正式 `menu-events.tsv` 已擴充為 12 個唯一 identity，SHA-256 為
  `973a6a1e247e7d9e16518a1a66266f340666d32e785f3f6f6652a890e830da2e`。
- `race-selection-events.tsv` 已加入逐事件 `event_key`，SHA-256 為
  `055f77591bcc4fc238138c59e837be4aeeae625c7f9970105242374773176828`；
  `menu-text-safe-rects.tsv` SHA-256 為
  `3e37efcae4222ffb9ea11b19ea93dc5b016e11df974253d58da797365f6c982f`。
- 同一 Enter→Down→Up 固定排程兩次重生的 JSON 收據逐位元相同，SHA-256 均為
  `eb619b596f312aa61a2e5ea335c3c57157a0cd7cb2cf6350e03aaaaf03748b2f`；13 個完成事件依序
  產生 13 個 request，且 recorder drop、pending 與 catalog miss 全為零。
- 舊九事件／九 request 收據仍只能對應 inventory 前九筆；新增的三筆 variant 或四步 lifecycle
  任一欄位遭修改時，新 verifier 均失敗即關閉。
- 專案 41 項 Python 測試與四組正式收據 verifier 通過；dosgolem 排除既有非正式
  `workplace/` 草稿後，全部正式 packages 的 test／vet 與 `apps/buckrogers` race detector
  通過。
- 此符合性只涵蓋 catalog 與 runtime request，不包含字型載入、畫面清除、繁中繪製或倍率
  決策。
