# 213 — Buck Rogers 技能配置底部操作列字元事件

狀態：DRAFT  
日期：2026-09-21

## 目標

為 `apps/buckrogers` 定義可將底部逐字 glyph 呼叫收旂成 content-safe 操作標籤事件的
watcher，供後續繁體中文輸出端覆繪使用。不修改原版記憶體、鍵盤輸入、技能點或
選單控制流。

## 已證實契約

- 原版逐字進入 runtime `0763:026B`，最終由 `0763:1809` 輸出 8×8 glyph。
- 參數可取得 glyph、repeat、background、foreground、row 與 column；只使用低位 byte。
- 標籤皆在 row 24；職業頁為 17 字元，技術頁為 27 字元。
- 焦點 caller 是 `37F1:0337`；一般快捷鍵首字是 `37F1:0391`，其餘字是
  `37F1:03CE`。caller 只是 guarded identity，不是可重命名的語意事實。
- 八個畫面配置的長度、SHA-256、字格座標與色彩來自主專案
  `text/skill-action-bar-events.tsv`。

## DRAFT watcher

1. 只在 Buck Rogers adapter 與已證實技能畫面錨定啟用。
2. 以同列、連續 column 收集字元；遇到座標斷裂、呼叫端不合法、repeat 不為 1
   或畫面錨定失效時，丟棄候選事件。
3. 完成候選後只輸出長度、SHA-256、座標、色彩與 variant，不把原文加入日誌或收據。
4. 只接受 exact catalog hit；任一字元、順序、座標、色彩或 caller 漂移都維持 miss。
5. 原版照常繪製；watcher 本身不清畫面、不覆繪、不改操作結果。

## READY 前置

- 主專案清冊 verifier 與正反向單元測試通過。
- watcher 純核心能重建職業三標籤與技術五標籤，並拒絕部分序列、串接跨列、
  未知 caller 與非 exact 輸入。
- base 及所有 Right 焦點路徑都產生與基準數量相符的事件，而 framebuffer 逐 byte
  與無 watcher 對照組一致。
- disabled 畫法目前是 `unknown`；不將一般狀態自動命名為「可用」，也不擴張
  catalog。若未來有正常輸入證據，另以新 identity 審核。

## CONFORMED 條件

READY 實作、負向測試、決定性 runtime 收據與原版 framebuffer 零差異全數通過後，
才可升為 CONFORMED。繁中 overlay 是後續獨立規格，不屬本檔完成聲明。
