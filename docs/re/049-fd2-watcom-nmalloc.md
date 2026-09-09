# 049 — FD2 的 Watcom `_nmalloc`

日期：2026-09-06  
證據等級：函式身分、邊界、直接呼叫者與主要失敗路徑為**已證實**  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`  
位址空間：IDA 載入 LE 映像後的 32 位元線性位址

IDA 對固定雜湊原檔套用 `Watcom v9-*1.5 32bit DOS runtime` 與
`Watcom v9-*1.5 32bit common runtime` 的 FLIRT 簽章，將
`0x36D26..0x36D98` 識別為原始執行期名稱 `_nmalloc`。這項名稱由簽章與函式
邊界共同支持，不是 dosgolem 為了導覽而建立的猜測性改名。

直接交叉參照如下：

- `0x36D1C`：同一段 Watcom runtime 的呼叫者；
- `0x46910`：`0x468F8..0x4693D` 內的呼叫；
- `0x4CC4C`、`0x4CC6B`：FD2 第三個啟動回呼 `0x4CBFD..0x4CCE1` 的兩次呼叫。

已證實的控制流：參數零時直接回傳零；非零參數先呼叫 `__MemAllocator`，失敗時
依序嘗試 `__ExpandDGROUP` 與 `__nmemneed`，可重試配置，最後以 EAX 回傳近指標。
`0x4CC4C` 的第一個實際參數為 1，返回位址是 `0x4CC51`；呼叫者負責移除參數，
符合 32 位元 cdecl 堆疊形狀。

受控匯出雜湊：函式報告
`ede1675b1ad71cecd50cae4ec6631b50f6f422f3e784ab6499bcd247f9407f68`；
Hex-Rays 報告
`00c00e7a4518c79da5733ad08adfe9ec9b10f0d5b80d6eff32c02ba47b4c8cb6`。
暫存 `.i64` 只供本機回查，不納入公開儲存庫。

## 實作邊界

dosgolem 不必逐指令重做 Watcom 內部 heap metadata；它需要提供可界定、可失敗、
可重現的近堆配置服務，忠實維持 cdecl 呼叫／返回、非零可寫平坦指標、互不重疊
配置及耗盡回傳零。任何未登錄入口仍交給 CPU 正常解碼並失敗即關閉。
