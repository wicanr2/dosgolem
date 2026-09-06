# 008 — EMS 頁配置與映射（int 67h AH=41h/43h/44h/45h）、檔案屬性（int 21h AH=43h）

日期：2026-09-06
狀態：**READY**（§2 AH=41h/43h、§3 AH=44h、§4 int 21h AH=43h AL=00）／
**DRAFT**（§3 的 AH=45h——已實作但尚未被任何程式呼叫；§5 EGA planar）
動機：`GIN3PS.EXE`（logh3 本體）在 dosgolem 裡的停點是 EMS 配置
（`~/cht/logh3/docs/re/02` §2）。
前置：[`007`](007-exec-ems.md)（EMS 查詢子集 40h/42h、EMMXXXX0）。

> **審查更新（2026-09-06）**：實作後跑原版 GIN3.COM 鏈，
> `int 67h AH=44h` 實際被呼叫 **152 次**（probe 軌跡）→ §3 的 44h 升格 READY。
> AH=45h 全程 0 次呼叫——實作與測試保留（45h 是 43h 的對稱面，
> 拿掉會讓「頁池耗盡」變成安靜的狀態錯誤），但有呼叫證據前不算驗收項目。

---

## 1. 證據一覽

| 行為 | 證據 | 等級 |
|---|---|---|
| GIN3PS.EXE 在鏈內呼叫 `int 67h AH=41h`（取頁框段）×1、`AH=43h`（配置頁）×1，之後安靜 exit 0 | probe 鏈內軌跡（re/02 §1：unimplemented 清單 `int 67h AH=41 AL=00`、`AH=43 AL=00`）| confirmed（軌跡） |
| 實作 41h/43h/44h 之後：43h ×1 成功、`AH=44h` ×152、無任何 EMS 錯誤路徑被觸發 | 2026-09-06 鏈內軌跡（re/03 §1）| confirmed（軌跡） |
| launcher 只要求 ≥8 頁可用（AH=42h，BX≥8） | GIN3.COM bytes `01C4 83 fb 08`（spec 007 §1）| confirmed |
| GIN3PS.EXE 的 int 67h 呼叫點在 TINYPROG 殼的壓縮資料區內，**靜態反組譯取不出呼叫序列** | objdump 全檔只有 1 個 `cd 67`，位於亂碼資料中（檔案偏移 0x1a5a8 一帶）| confirmed（嘗試記錄） |
| 43h（配置）之後必然要 44h（映射）才有意義——EMS 的頁不映射就摸不到 | LIM EMS 3.2 介面語意 | 強證據（待軌跡確認） |
| GIN3PS 單跑會 `TINYPROG says, "Bad program file!"` exit 255；鏈內才走正常路徑 | re/02 §2 | confirmed（軌跡） |
| int 21h AH=43h AL=00（取檔案屬性）在鏈內被叫 5 次（OPEN.EXE）＋1 次（GIN3PS），目前回垃圾 CX | re/02 §4 #2；probe unimplemented 清單 | confirmed（軌跡） |

**方法論註記**：本體是 TINYPROG 殼，反組譯無法先驗取證，呼叫序列的證據
一律以 probe 軌跡為準；每實作一批就重跑一次看下一個 unimplemented。

## 2. int 67h AH=41h／43h（READY）

EMS 機器模型（通用語意，無程式專屬位址）：

- **頁框段 ＝ 0xE000**，4 個實體頁 × 16 KB（0xE0000–0xEFFFF）。
  機器記憶體是平坦 1 MB，這一段在 MemTop 之上、VGA/BIOS 區之外，沒有人用。
- **頁池 ＝ 8 頁（128 KB）**，與 spec 007 §4 的 AH=42h 宣稱一致。
  EMS 頁的內容存在 DOS 層（`[]byte`，每頁 16 KB），**不佔機器記憶體**；
  映射時才複製進頁框（見 §3）。
- **AH=41h**：AH=0，BX ＝ 頁框段（0xE000）。
- **AH=43h**（配置 BX 頁）：
  - BX ＝ 0 → AH=0x87（要 0 頁是錯誤）；
  - BX ＞ 可用頁數 → AH=0x88（頁不夠）；
  - 否則 AH=0、DX ＝ handle（從 1 起編），頁池扣掉 BX 頁。
- **AH=48h** 是 EMS 4.0 的同名配置（語意同 43h）——**尚未被呼叫**，
  有軌跡再加（§6）。

## 3. int 67h AH=44h／45h（44h READY——鏈內軌跡 ×152；45h 已實作待證據）

複製式分頁（copy-banking）：映射＝把頁內容複製進頁框；
換映射／釋放前把頁框**寫回**原頁。語意等價於真分頁，前提是程式
「先映射再存取」——這正是 EMS 的用法；違反前提會在軌跡裡看到
（映射前摸頁框 → 讀到上一頁的內容，行為錯但可觀測）。

- **AH=44h**（映射）：AL ＝ 實體頁（0–3）、BX ＝ 邏輯頁、DX ＝ handle。
  handle 不存在 → AH=0x83；邏輯頁超出 → AH=0x8A；實體頁 >3 → AH=0x8B。
  成功：舊映射寫回、新頁複製進框、AH=0。
- **AH=45h**（釋放）：DX ＝ handle。無效 → AH=0x83。
  把該 handle 佔著的實體頁全部寫回並解除映射，頁池加回，AH=0。

升格條件：probe 軌跡確認 GIN3PS 呼叫 44h（與預期的 45h）。

## 4. int 21h AH=43h AL=00：取檔案屬性（READY）

- AL=00：DS:DX 檔名照 `resolve()`；找到 → CX ＝ 0x20（archive，普通檔）、
  CF=0；找不到 → AX=2、CF=1（與 open 一致）。
- **AL=01（設屬性）不實作**：記 unimplemented 但 CF 清 0（素材唯讀，
  假裝成功並記錄，與 `AH=40h` 寫檔的 Wrote 清單同一原則——看得見的假）。

## 5. EGA mode 10h planar（DRAFT——不屬本批）

現況：機器記憶體是平坦 1 MB，寫 A0000 直接落 Mem，**沒有 sequencer
map mask（3C5:02）／graphics controller（3CE/3CF）的寫路徑語意**；
3CE/3CF、3C4/3C5 的埠值有記（`m.Ports`），但 Mem 裡的 bytes 是
「所有 plane 混在一起」的超集，拼不回四個 plane。

所以要 dump 640×350×16 色不是工具面的活，是渲染語意：
寫路徑要過 map mask＋latch（mode 10h 常用 write mode 0/1），
讀回要過 read plane select。**等開頭動畫要逐點對拍時另開 spec**；
本批已加 probe 的 `-dump-linear`（把 A0000 的 64 KB raw bytes 倒出去）
當偵錯手段，不宣稱它是畫面。

## 6. 刻意不做

| 項 | 觸發條件 |
|---|---|
| AH=48h（EMS 4.0 配置別名）| 軌跡出現（截至 2026-09-06 鏈內 0 次）|
| AH=46h（取版本）、4Eh/4Fh（mapping context）等 | 軌跡出現 |
| EMS 頁框放 0xE000 以外的段 | 有程式檢查特定段址 |
| EGA/VGA planar 寫路徑語意 | 開頭動畫對拍時另開 spec |

## 7. 驗收

1. 單元測試：41h 回 0xE000；43h 配置/超額/0 頁三路；44h 映射後
   頁框讀寫＝頁內容、換映射寫回不漏；45h 釋放後頁池回升、
   再用同 handle → AH=0x83；int 21h AH=43h 找到/找不到兩路。
2. 整合（缺檔 skip）：原版 GIN3.COM 鏈內，GIN3PS.EXE 的 EMS 配置
   通過、本體推進超過上一輪停點（re/02 §2）。
3. `tools/go.sh build ./...` 與 `test ./...` 全綠。
