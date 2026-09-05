# 074 — Watcom 無參數 `__Init_Argv` 執行期服務

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 053`](../re/053-fd2-watcom-init-argv.md)

- 服務只處理明確登錄入口及明確配置的輸入／輸出全域位址。
- command line 首 byte 必須為零，program name 必須是記憶體內有界 NUL 字串；
  非空 command line 或無界字串回報錯誤，不猜 Watcom quoting。
- 使用同一 Watcom near heap 配置 1 byte 空字串加兩個 dword 指標；argv 表從
  `base+1` 開始，寫入 program pointer 與 NULL。
- 內部／公開 argc 均寫 1，內部／公開 argv 均寫 `base+1`；EAX 回傳 argv。
- 採無參數 near call 返回：EIP 取 `[SS:ESP]`，ESP 增加 4。
- 任一讀寫、配置或驗證失敗不得偽造成功。

固定 FD2 登錄入口 `0x46114` 與 RE 053 的六個位址；整合測試必須由 LE 入口
自然抵達並執行，不直接寫入 argc／argv。

2026-09-06：空 command line、非空拒絕、argc／argv 四個全域、兩個指標槽、
cdecl 返回與固定原版 LE 入口整合測試通過。
