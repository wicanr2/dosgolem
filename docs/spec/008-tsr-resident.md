# 008 — TSR 常駐與行程模型

狀態：**READY**（量測證據見 §4）
日期：2026-09-06
前置：[`003`](003-machine-and-loader.md) §1／§3、[`004`](004-dos-bios-services.md) §2

---

## 1. 為什麼需要

第二個案例（KOEI《源平合戰》DOS/V 版）的啟動鏈是

```
DOSJP -F:font.dat -TJ → Genpei.com（殼）→ EXEC → FMDRV.COM → GRPDRV.EXE
                    → OPEN.EXE → MAIN.EXE → END.EXE
```

其中 FMDRV.COM 與 GRPDRV.EXE 是 **TSR**：用 `int 21h AH=31h` 常駐，
讓 MAIN.EXE 透過它們裝的中斷向量呼叫驅動。現行機器層只有「一台機器
一支程式」，`AH=31h` 未實作，TSR 一落下去就被當成結束。

判準（`006` §2）：「換一支 binary 之後這段程式碼還成立嗎？」
TSR 與行程是 DOS 本身的行為，成立 → 收在機器層（`internal/machine` 的
載入器與 `internal/dos` 的服務），不進 `apps/`。

## 2. 行程模型

`internal/dos` 維護一個**行程疊**（LIFO），一格存一個行程的：

- 全部通用暫存器、段暫存器、IP、FLAGS（父行程的完整 CPU 上下文）；
- 自己的 PSP 段。

「目前行程」是疊頂；機器剛載入的第一支程式是疊底，它的「父行程」
欄位指向自己（與現行 `initPSP` 一致）。

### 2.1 記憶體：bump 配置 + LIFO 回收

沿用現行的 bump 配置器（`AH=48h` 的 `freeSeg`），加一條：

> **子行程結束時，`freeSeg` 收回該子行程的 PSP 前一格（它的 MCB 段）。**
> TSR（`AH=31h`）例外：只收回 `PSP + DX` 之後的部分。

LIFO 回收對「嚴格父子巢狀」永遠正確（EXEC 鏈就是巢狀）。
TSR 之後再載入的程式自然落在常駐區之上，**不會覆蓋常駐區**。

⚠ 這不是通用 DOS 的 MCB 鏈管理——不做合併、不處理亂序釋放。
`AH=49h` 維持現況（收下、不回收）。真的踩到再說，先記一筆。

### 2.2 中斷向量：TSR 裝的向量由 CPU 直接執行

既有規則已經是對的，這裡只是重申它的推論：

- TSR 用 `AH=25h` 裝的向量（FMDRV／GRPDRV／DOSJP 都這麼做）會真的
  寫進向量表；之後該中斷 **服務層不攔**（`dos.handle` 的既有規則：
  向量不再指向 StubSeg 就放行），CPU 跳進常駐程式碼執行。
- 常駐程式碼結束於 `IRET`，回到呼叫端——**不需要任何排程**，
  因為 TSR 的服務是被呼叫的，不是自己跑的。

## 3. `int 21h AH=31h`（TSR 並結束）

- 輸入：`AL` ＝ 回傳碼、`DX` ＝ 從 PSP 起保留幾段（paragraph）。
- 行為：
  1. 記下回傳碼（給 `AH=4Dh`，見 `009`）。
  2. `DX > 0`：`freeSeg = max(freeSeg, PSP + DX)`，常駐區保留；
     `DX == 0`：與 `AH=4Ch` 相同，全部回收。
  3. 之後與 `AH=4Ch` 一樣：行程疊不空就彈回父行程；空了就照
     `009` §4 的監督佇列決定下一支程式或停機。
- `PSP + DX` 超出 `MemTop` 時夾到 `MemTop`，不當錯誤
  （真 DOS 也只看區塊能不能切）。

## 4. 量測證據（2026-09-06，cmd/probe，dosgolem-yuan 86b1815）

- **Fmdrv.com**（4,774 bytes）：`AH=35h`×3、`AH=25h`×2（裝兩個向量）、
  寫 OPL2 埠 388/389 與 PIT 埠 40/43，然後 `AH=31h AL=01` ——
  未實作所以直接返回，程式落進 `int 20h` 結束。
  收據：`yuan/workplace/boot-20260906-02/fmdrv-before.txt`。
- **Grpdrv.exe**（21,875 bytes，MZ）：`AH=25h`×2、`AH=49h`×2，然後
  `AH=31h AL=00`；未實作返回後它自己走 `AH=4Ch` 離開。
  收據：`yuan/workplace/boot-20260906-02/grpdrv-before.txt`。
- **Genpei.com**（殼）：`AX=4B00h` 四次（ES:BX 指向同一個參數區塊
  0534h/0574h），每次之後 `AH=4Dh` 取回傳碼；並用自己的
  `int 65h`／`66h` 向量在 EXEC 前後切換（自行 save/restore，
  不需要我們介入）。出處：`yuan/workplace/boot-20260906-01/genpei-com.dis`
  的 6F9h–761h。
- **DOSJP.COM**：`AH=49h` → `AX=6300h`（DBCS 向量）之後卡在掃表迴圈
  （`AH=63h` 是 `010` 的範圍）。

## 5. 不做的事

- 不做多工／排程。TSR 只在被中斷呼叫時執行。
- 不做真的 MCB 鏈走查（`003` §4 的未定案維持：先不建真鏈）。
  每支程式照樣在自己的 PSP−1 寫一格假 MCB，與現行 `initMCB` 同形。
- 不做 `AH=50h`／`51h`（set/get PSP）以外的行程 API；`AH=26h`
  （建 PSP）不做，記一筆。
