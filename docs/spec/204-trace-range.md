# 204 — trace-range（任意步號區間的全暫存器追蹤）

狀態：**READY**（只定旗標與輸出格式，不碰實作；觸發案例見 §2；實作另開條目）
日期：2026-09-24
前置：[`203-trace-operands-and-steps`](203-trace-operands-and-steps.md)
 （行格式：絕對步號＋CS:IP＋位元組＋暫存器）

---

## 1. 規則

`-trace-after-exit` 只管子行程結束後，`-trace-tail` 只管最後 N 條——
要看中間（例如 chained compile 的子行程初始化窗）目前無旗標可用。

- 新旗標 `-trace-range lo-hi`（十進位絕對步號，起止含首尾；`lo > hi` 視為
  用法錯誤 exit 2）。步號與 `-watch` 的 `步 N` 同一座鐘（`m.Steps` 1-based，
  見 203 §2 的對齊結論）。
- 行格式與 `-trace-after-exit` 完全一致（含 `步` 欄），只是觸發條件改成
  「步號落在區間內」。與 `-trace-tail`、`-trace-after-exit` 可併用。
- 輸出量警告：每步一行，一千步約數百 KB；旗標 help 註明「區間先抓小
  （如懷疑區間前後各一千步），太大先切段」。效能不承諾（本來就慢），
  確定性照舊（同輸入同輸出）。

## 2. 觸發案例（Watcom 6.5 E142，`retro-runtime-study-private#32` R46）

`SI＝0x16B6` 在 732390–732397 步之間出現（7 步窗），既有旗標看不到：
`-trace-tail` 搆不到中間，`-watch` 只看得到寫入看不到暫存器寫入者。
有了區間追蹤，直接讀出定值指令即可閉合 index 選擇器懸案。

## 3. 範圍外

- 條件斷點／運算式觸發：不做。只要固定區間。
- 輸出過濾（只印 CALL／只印某段）：不做。用 grep。
