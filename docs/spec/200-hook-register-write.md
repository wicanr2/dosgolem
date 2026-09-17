# 200 — 在執行掛鉤裡改暫存器

狀態：**READY**
日期：2026-09-17
前置：[`199-live-session-api.md`](199-live-session-api.md)（即時執行）；`OnCall`（`oracle/run.go`）

---

## 1. 問題

轉譯層（例：`psychic_war_cht` 的中文疊字）要在原版畫字的那一刻決定「這個字要不要照畫」。
`OnCall` 只能讀：暫存器用 `Regs()`、記憶體用 `Byte`／`Word`。要讓原版改畫別的東西（例如把字元換成空白、只保留游標前進），
得在掛鉤裡改暫存器。現有的寫入路徑只有 `CallNear`／`FarCall`，它們會推返回位址、改 CS:IP，不能在掛鉤裡用。

改記憶體（`SetByte`）不夠：字元常放在暫存器裡傳（AL），原版的字串資料也不該被改——同一則字串之後可能還要照原文印。

## 2. 規格

- `(*Oracle).SetRegs(r CallRegs)`：把 `r` 裡 `Set*` 為 true 的暫存器寫進 CPU（AX、BX、CX、DX、SI、DI、BP、DS、ES），其他暫存器、CS:IP、旗標、堆疊都不動。
- `CallNear` 改用同一個套用函式，行為不變。
- 在 `OnCall` 的掛鉤裡呼叫時，改動對**同一步接著執行的那道指令**生效（掛鉤在執行該位址的指令之前觸發）。

⚠ 位址與「改成什麼」一律由呼叫端（遊戲專屬的 repo）決定，本 repo 不含任何遊戲的位址。

## 3. 驗收

1. 合成 EXE：`mov al,41h; mov [0100h],al; jmp $`。在第二道指令的位址掛 `OnCall`，掛鉤裡 `SetRegs(CallRegs{AX: 0x20, SetAX: true})`；
   跑完 `[0100h]` 是 20h。反向對照：掛鉤不改時是 41h。
2. `SetRegs` 只寫 `Set*` 為 true 的暫存器：設 BX 時 AX、CX、DX、SI、DI、BP、DS、ES、CS、IP、SP、旗標都不變。
3. `CallNear` 既有測試全部通過。

## 4. 不做

- 在掛鉤裡跳過指令或改 CS:IP（需要時另寫規格）。
- 旗標、SS:SP 的寫入。
