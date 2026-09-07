# 010 — DOS/V 最小集（DBCS 與 DOSJP 的落腳處）

狀態：**READY**（§2 有量測證據；§3 是邊界宣告）
日期：2026-09-06
前置：[`008`](008-tsr-resident.md)（TSR）、[`009`](009-exec.md)（監督佇列）

---

## 1. 定位

源平合戰的啟動鏈第一支是 **DOSJP.COM**（「Dos-J Plus Ver. 1.1s」，
KOEI 附的軟體 DOS/V 字型驅動）：它裝 int 10h／15h／21h 的常駐 handler，
提供日文環境服務（DBCS 向量、字型存取），之後 GRPDRV／MAIN 才認得出
「這是 DOS/V 機器」。

DOSJP 自己也需要 DOS/V 的入口已經存在（它偵測「是否已經裝過」與
DBCS 環境）：`int 21h AX=6300h`。

**原則：原版驅動能跑就跑原版驅動**（字型資料 JIS.FNT／FONT.DAT 都是
原版檔）；機器層只補「一台 DOS/V 機器本來就該有的」服務。
DOSJP 依賴什麼就量測什麼，不照手冊列整個 DOS/V API。

## 2. `int 21h AH=63h`（DBCS 前導位元組表）

量測證據（2026-09-06，cmd/probe，dosgolem-yuan aec0222 之前）：

- DOSJP.COM 依序呼叫 `AH=49h`（釋放環境區塊）→ `AX=6300h`，
  然後進入掃表迴圈（CS:IP ＝ 0100:04A8 附近打轉，300 萬道不停）。
  收據：`yuan/workplace/boot-20260906-02/dosjp-before.txt`。
- 反組譯（DOSJP.COM 檔案位移 0x2C4–0x2D9）：`mov ax,6300h / int 21h`
  之後 `or [si],0 / jnz …`——**它掃的是 DS:SI 指的 DBCS 表**，
  我們沒填 DS:SI，表當然掃不完。

實作（AL=00h，get DBCS vector table）：

- 回 `DS:SI` → 一張 Shift-JIS 前導位元組表：`81 9F E0 FC 00 00`
  （前導範圍 81h–9Fh 與 E0h–FCh，雙 0 結束）。表放在 StubSeg 的
  固定位移，與中斷 stub 同一區。
- AL=01h（set table）收下、真的把表內容抄回 StubSeg 那格，
  之後 AL=00h 回的是抄過的內容（TSR 會指到自己的表再叫我們換）。
- 其他 AL：記一筆（`004` §1.3）。

## 2.1 `int 21h AH=38h`（取國別資訊）與 `int 16h AH=13h`

DOS/V 程式用 `AH=38h` 判斷「這是不是日文環境」。回不出來它會走另一條
路徑，而那條路徑上什麼都不會說。實作回國碼 **81（日本）**與一張 34 bytes
的表（日期格式 2 ＝ 年月日、24 小時制、貨幣符號 `5Ch`），表放 StubSeg
的 `+80h`，`DS:DX` 指過去。

`int 16h AH=13h`（DOS/V 鍵盤擴充狀態）回 `AL=0`——收下，表示沒有特殊狀態。

## 3. 邊界宣告：這份規格**不做**什麼

- **不做完整 DOS/V API 表面**（int 10h 的 DOS/V 擴充、int 21h
  AH=63h AL=02h 之後的子功能）。理由：那些服務在真機上由 DOSJP
  這支原版驅動提供——**讓原版自己提供，比照手冊重寫一份忠實**。
  只有 DOSJP 落腳失敗（需要 386 指令或 XMS，量測後另立規格）時
  才回頭考慮機器層接手。
- **不做字形渲染**。OPEN/MAIN 的字型來自原版 JIS.FNT／FONT.DAT，
  誰讀它們（DOSJP 的 int 10h handler 或 GRPDRV 自己）由量測決定，
  不是這份規格的事。

## 4. 後續量測清單（做完一項收一項進規格）

1. AH=63h 落地後 DOSJP 跑到哪：預期 `AH=4Ah` 縮區塊 → `AH=48h`
   要 0x45E8 段（286 KB，JIS.FNT 的份量）→ 開 JIS.FNT（有 `-F:`
   參數時可能是 FONT.DAT）→ int 2Fh AX=4300h（XMS 偵測）。
2. DOSJP 常駐 handler 裡的 0x66 前綴（80386 operand-size）指令：
   若真的執行到，CPU 層要加 386 子集——那會是另一份規格（011），
   語料仍是 8086 準則（`cpu.New()` 預設不變，386 走 Model 旗標）。
3. XMS（int 2Fh AH=43h ＋ driver entry 的 AH=09h/0Bh/0Ah）：
   Config.5 有 `DEVICE=HIMEM.SYS`，DOSJP 設計上把字型放 XMS；
   需要時同樣另立規格。
