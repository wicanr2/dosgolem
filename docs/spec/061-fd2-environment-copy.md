# 061 — FD2 啟動 environment 複製

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 050`](../re/050-fd2-third-callback-allocation-consumer.md)

固定雜湊 FD2 的第三個已選回呼必須由 LE 入口自然完成兩次 Watcom `_nmalloc`、
建立 environment 指標表、寫入終止空指標、設定 `dword_53800`，再由
`sub_4CBFD` 返回 `0x45DD3`。最小 environment 沒有變數字串，因此：

- `dword_537FC = 0x634D8`；
- `dword_53800 = 0x634DC`；
- `0x634D8` 的終止指標為零；
- 第一段字串緩衝位於 `0x634D4`，不得與指標表重疊。

探針與整合測試不得直接設定 EIP 或注入回呼結果；未支援的其他 environment
形狀仍保持失敗即關閉。

2026-09-06：固定原版由 LE 入口執行至第三回呼返回，以上四項記憶體結果與兩次
DOS service 呼叫均通過整合測試。
