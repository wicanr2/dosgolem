# Buck Rogers 明示倍率的執行期繁中覆繪

狀態：**CONFORMED**

## 範圍

本規格核准把 spec `034-buck-rogers-save-roster-join-runtime-display-requests` 的 typed
`DisplayRequest` 接到 spec `015-buck-rogers-menu-overlay-core` 與 `xlate.Layer`，只在輸出端
presentation snapshot 產生 2×／3× 繁中 RGBA。原版 machine indexed VRAM、CPU、DOS、輸入、
FileOps、保存檔與控制流不得被覆繪資料修改。

本規格不選定產品倍率；所有 overlay 執行必須明示 2 或 3，缺省與其他值失敗即關閉。

## 固定輸入

- menu／roster events 與 translations：沿用 spec 034 的固定 SHA-256。
- menu text-safe rects：
  `a6cc54e8583fc288924edfc4f8923328ffe2b0d62441ff8b7a13c68c2777ff99`。
- roster text-safe rects：
  `705fd25ae720fe0b303957d85c68b4ea244d507850c2c518d8049d0a4d9e9e1b`。
- 合併 56-codepoint 字元清單：
  `c30791cde73e500b8da6aa665fa69fba6a952815329e2b82977c328311e3dbb4`。
- 16×16 GOLEMFNT：
  `064bf0124bf5489b71fc1df590e8be463f196423f189a04de12e936779b1e95c`；來源為既有
  `unifont-13.0.06.hex`，SHA-256
  `1d02f1536c1fda8945c74657cd63191079516306374320365241a300dd4afaf0`。
- 原版正常路徑 checkpoint、BIOS 排程與 #122,400,000 終點沿用 spec 034。

## 已證實幾何

每筆安全矩形由 dispatcher 的 `column*8`、`row*8`、`original_length*8` 與固定 8-pixel 高度
直接導出。功能選單文字不需額外 CJK anchor；種族選單既有縮排仍由原表保留。roster add
prompt 與 loading 共用底列與相同原點，但原文長度分別是 17 與 21 格；後來的輸出在語意上取代同原點舊輸出，不能只依寬度部分重疊。四筆動態姓名沒有 rect entry，不能
形成 stamp。

## DRAFT 待審查

1. 以 machine `SetOnFrame` 的真實垂直回掃回呼驅動 `xlate.Layer.Frame`，不得用重複呼叫偽造
   三幀 invalidation debounce。
2. 每筆新 request 先由完整 event identity 找 rect，以 event 原色號與 request 譯文建立單一
   `MenuOverlayEntry`；`BuildMenuOverlay` 成功後把 stamp 加入同一長存 layer。
3. layer 只畫入由原版 indexed framebuffer 與 palette 轉成的獨立 RGBA buffer；不得回寫
   `m.Indexed()` 或 A0000。
4. 收據需記錄 14 次 overlay action、終態 active stamp keys、scale、missing glyphs、輸出雜湊
   與安全矩形外差異；不得輸出譯文全文。
5. 終態預期只保留 `roster.add_prompt`；四筆 dynamic miss 所在 row 2 不得有任何 overlay rect，
   對應 RGBA 必須逐像素等於同倍率 baseline。

## 驗收

- 2×、3× 各兩份 fresh scratch 正常路徑；每份 18 events、14 requests、4 misses、14 overlay
  actions、零 pending／drop／missing glyph／越界。
- 同倍率雙重重播 RGBA 與 content-safe overlay receipt 逐 byte 相同。
- 終態 active stamp、ink containment、safe-rect outside diff、dynamic-name row preservation 通過。
- events、BIOS keys、FileOps、writes、保存檔及 raw indexed framebuffer 等於 spec 034 baseline。
- 缺 overlay 任一成對輸入、非法倍率、缺字、孤兒 rect、identity 漂移與輸出 buffer 尺寸均
  失敗即關閉。
- 專案測試、dosgolem 全部正式 packages test／vet 與相關 race detector 通過後才可升
  CONFORMED。

## 權利與停止線

字型、原版 checkpoint、scratch、framebuffer 與收據只留使用者本機忽略目錄。正式程式不
內嵌原版素材或翻譯 literal。若真實 frame clock 無法使舊 stamp 正確失效，維持 DRAFT 並補
生命週期證據；不得硬編終態清單或在輸出前直接刪除非預期 stamp。

## READY 審查紀錄（2026-09-21）

- `Machine.SetOnFrame` 在每次真實垂直回掃、`Frames++` 後呼叫；載入 state 後註冊 callback
  不修改序列化 machine state，符合 xlate 三幀 debounce 的正式時鐘契約。
- `Machine.Indexed()` 明文保證回傳複本，`Palette()` 回傳值型別；presentation RGBA 可在呼叫端
  建立，不會回寫 A0000 或 palette。
- menu 安全矩形已擴充並由既有一對一 verifier 通過；roster 兩筆矩形由 runtime identity 的
  row／column／length 機械導出並由新 verifier 鎖定。
- 多 catalog 字型工具拒絕重複 key，以兩份正式 TSV 與固定 Unifont 來源重生 56 字 GOLEMFNT；
  沒有複製譯文權威或未決缺字。
- 行為、幾何、時鐘、輸入、輸出、失敗模式與 same-state oracle 均有 typed 證據；產品倍率仍
  明確保留為使用者待決，不阻擋同時驗證兩個明示倍率。

## 實作回饋與退回 DRAFT（2026-09-21）

首輪 2×／3× 各雙重重播皆穩定產生 18 events、14 requests、4 misses 與 14 overlay actions，
但終態 active keys 仍含舊 `function.menu.normal.add_character_to_team`、`roster.loading` 與目前
`roster.add_prompt`。這證明只靠 frame fingerprint 的三幀 debounce 無法完整涵蓋本路徑快速
清除／部分重疊生命週期；原 READY 前置不足，規格退回 DRAFT。

下一步必須沿用已證實的 `026F:029C` Mode 13h 矩形清除事件，把原版清除矩形傳給 layer
invalidation；不得在終點依 key 硬刪舊 stamp。清除事件與 `xlate.Layer` 的 typed 契約補齊並
以失敗收據重驗前，不得升回 READY 或把首輪 RGBA 當成正式玩家可見成果。

## 第二次實作回饋（2026-09-21）

接入 `026F:029C` 已證實清除矩形後，功能選單舊 stamp 已正確消失，但底列 21 格
`roster.loading` 被後來 17 格 `roster.add_prompt` 部分重疊後仍留四格。終態原版
framebuffer 顯示新 prompt，舊 loading 已不是當下輸出；因此 adapter 應使用通用的
同原點取代契約，不以 event key 或終態清單特判。完成雙倍率重驗前仍維持 DRAFT。

## READY 復審與 CONFORMED 收據（2026-09-21）

`026F:029C` 清除參數、`Layer.Clear` 像素矩形契約與 `Layer.Replace` 同原點輸出契約
均已有正反例測試；adapter 只使用 runtime 位址、堆疊參數與輸出幾何，沒有以 key 特判。
因此契約重新達 READY，並以四份 fresh-scratch 正常路徑驗收後升為 CONFORMED。

2×／3× 各兩次皆為 18 events、14 requests、4 dynamic misses、14 actions，終態只有
`roster.add_prompt`；同倍率 RGBA 逐 byte 相同，raw framebuffer 均維持
`0c43f315…e7d003`。差異只在終態提示安全矩形，動態姓名列零差異；events、
BIOS keys、FileOps、writes 與存檔均等於無覆繪 baseline。產品預設倍率仍未決定。
