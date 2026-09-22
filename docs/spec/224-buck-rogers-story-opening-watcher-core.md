# 224 — Buck Rogers 首屏劇情 watcher 純核心

狀態：**CONFORMED（純核心；非 production runtime）**
日期：2026-09-22
前置：project `docs/spec/010-story-opening-overlay-draft.md`、dosgolem spec 223。

## 目的與邊界

本規格只建立沒有檔案讀取、Oracle install、xlate renderer、命令列 flag、鍵盤或 VRAM 寫入的
`StoryOpeningWatcher`。它接受已完成 far-return guard 的 glyph call metadata，將已證實的五個
identity 收集為 content-safe events，並以已證實 `0CF4:1B3A` 的 `ES:DI`／`CX` span 判斷何時
失效。它是後續 project DRAFT 的可測純核心，不是 runtime hook 或中文覆繪授權。

## 已確認輸入

- 五行均為 `0763:026B`，completed guard return 是 `0763:04FF`，stack delta `0x12`；每行的
  length、SHA-256、caller、mode=1、repeat=1、bg/fg=0/10、row 17–21、column 1 已由 project
  phase105 兩次重播確認。
- 第二頁的首筆實例是 `0CF4:1B3A`、`A000:AB48`、`CX=304`，與 story rectangle
  `[8,320)×[136,176)` 相交；row24 clear 不相交且明確禁止使用。

## typed 契約

`NewStoryOpeningCatalog([]StoryOpeningIdentity)` 只接受恰好五個 sequence 1–5 的 identity；event
key 唯一，雜湊、長度、caller、guard、mode/repeat、色彩與座標皆是 value data。constructor 必須
拒絕 nil、漏列、重複 key／sequence、零長度、零 hash 與不連續的 expected row／column。

`StoryOpeningWatcher`：

1. `ObserveGlyphEntry` 只保留一筆 pending glyph；重疊 entry 丟棄舊 pending。七個 ABI word 的
   **低位 byte** 是原 glyph renderer 的已證實顯示輸入；高位不進 identity。首屏 content-safe
   return-edge trace 以 `high_word_mask` 保存高位存在與否、從不保存原文，並確認低位仍命中
   caller／mode／repeat／色彩／row／column exact identity。
2. `ObserveVerifiedGlyphReturn` 只接受 adapter 已觀測到的實際 far-return control-flow edge，並檢查
   entry step 回參、caller、SS 與 `SP+0x12`。它不接受一般「目前執行到 caller」的通知，避免未返回
   的 pending 在較晚的同位址／同 stack shape 被誤提交。collector 只從 sequence 1 的 exact first
   glyph 開始；後續 glyph／line 的每個欄位必須精確連續。
3. adapter 只要失去對 pending frame 的連續控制流觀測（machine stop、state restore 或未觀測的
   handoff），必須呼叫 `ObserveExecutionDiscontinuity` 清掉 pending／partial。沒有量到可泛化的
   instruction budget，因此禁止用步數逾時代替這個明示、可測的失效事件。
4. 五行都完成 hash 驗證後，才一次 append 五個 `StoryOpeningEvent`；partial line、錯序、hash
   miss 或 guard drift 絕不能外洩局部 event。
5. `ObserveVideoWrite` 只接受 `0CF4:1B3A`、`ES=A000` 與 nonzero `CX`。以至多 40 行的
   row-aware span intersection 判斷 `[8,320)×[136,176)`；相交時移除 active group 和 partial
   candidate，並提高 generation。其他 write 一律不推論失效。

所有 accessors 回 defensive copy；event 不含 glyph bytes、英文文字、答案、原版 pointer 或
translation。

## READY 審查與驗收

這個核心的 inputs、state transition、失敗模式與停止線已完全由 phase105／spec223 定義，且不
採用尚為 DRAFT 的 TSV／字型／rendering 參數，故可標 READY。實作測試必須覆蓋：五行原子完成、
未錨定／partial／錯序／hash／guard／重入失敗、video span 的實例、邊界與不相交條件、
invalidation 後不得由第二頁文字重建、defensive copy。

2026-09-22 已於隔離、無網路 Docker 的 `eob-remake-go:1.26.7-ebiten2.9.9` 完成：

- `go test ./apps/buckrogers -run TestStoryOpening -v`：五行原子完成、七個 word 的高位拒絕、
  verified-return entry step／guard／stack／錯序／hash fail-closed、七個 word 高位非零而低位 exact
  仍可匹配、顯式 execution discontinuity
  不讓 stale pending 晚到提交、已證實的 `A000:AB48`／304-byte second-page span、邊界、不相交、
  第二頁 partial 文字與 defensive copy 全部通過。
- `go test -race ./apps/buckrogers` 與 `go vet ./apps/buckrogers`：通過。
- opt-in `TestStoryOpeningETenSafeRectPrototype` 以 project-local phase107 的實際 16×16
  ETen-derived GOLEMFNT 與 DRAFT 五行量測，而非 EAW 近似：2×／3× 每行墨跡均落於
  `[8,320)×[136,176)` 的各自 8-pixel logical row；3× 最寬第五行墨跡為 328 output pixels，
  小於 936 output-pixel safe width。這是字型／安全矩形證據，未構成 A/B 同狀態畫面收據。

本 CONFORMED 結論只涵蓋純核心。它不代表 project spec010 READY、正式 adapter 已接線，或首屏中文
已顯示；後者仍需完成 spec010 的 TSV／實際 key wiring、原版／中文 A/B 和正常玩家路徑驗收。
尤其 `StoryOpeningVerifiedReturn` 是 future adapter 必須供應的 control-flow-edge 證據契約；本純核心
不自行從「到達 caller」推定 return，也不宣稱目前已有 production edge observer。
