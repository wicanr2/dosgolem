# 075 — DOS 目前時間的決定性服務

狀態：**CONFORMED**  
日期：2026-09-06  
依據：DOS `INT 21h/AH=2Ch` 公開介面；FD2 消費端見
[`RE 054`](../re/054-fd2-watcom-delay-init-time.md)。

- 完成既有兩個固定啟動 service 後，接受 `INT 21h/AH=2Ch`。
- 回傳 CH=hour、CL=minute、DH=second、DL=hundredths；目前 hour、minute、
  hundredths 固定為零，second 從零開始，每次查詢後遞增並於 60 回繞。
- 保留 ECX／EDX 高 16 位；非 21h、錯誤順序或其他 AH 維持拒絕。
- 這是為決定性對拍採用的硬體規格近似，不是逐週期時間模擬。
- 固定 FD2 必須由 LE 入口自然越過 `__delay_init`，且不得陷入等待秒值改變的迴圈。

2026-09-06：三次時間查詢回傳 0、1、2 秒；固定原版校準值為 1，無等待迴圈。
