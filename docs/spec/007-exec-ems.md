# 007 — EXEC（AX=4B00h）、子程式回傳碼（AH=4Dh）、EMS 最小子集

日期：2026-09-06
狀態：**READY**（§2 EXEC、§3 AH=4D、§4 EMS 查詢、§5 EMMXXXX0 裝置開啟）／
**DRAFT**（§6 刻意不做的部分與已知簡化）
動機：第二個案例《銀河英雄傳說III SP》的 `GIN3.COM`（622-byte COM launcher）
需要這四件才能原生（無 patch）EXEC 進本體 `GIN3PS.EXE`。
證據文件：`~/cht/logh3/docs/re/01-first-contact-gin3com.md`。

本規格全部來自對 `GIN3.COM` 的反組譯 bytes（objdump -m i8086，
`--adjust-vma=0x100`）與 probe 軌跡。**只實作有證據的部分**（§6）。

---

## 1. 證據一覽

| 行為 | 證據（記憶體位址 ＝ 檔案偏移 ＋ 0x100） | 等級 |
|---|---|---|
| launcher 縮記憶體：`AH=4Ah` BX=0x237 段 | `010F b4 4a / 0111 cd 21`；probe 軌跡 #7–#12 | confirmed（軌跡） |
| EMS 偵測先開字元裝置 `EMMXXXX0`（AH=3D AL=00），成功就關掉（AH=3E）；失敗回 FFFF | `01D9–01F0`；probe 原版跑死在這（缺裝置） | confirmed（軌跡＋bytes） |
| 然後 `int 67h AH=40h`，要求 AH 回 0 | `01B8 b4 40 / 01BA cd 67 / 01BC 0a e4` | confirmed（bytes） |
| 再 `int 67h AH=42h`，要求 BX ≥ 8（可用頁數） | `01C0 b4 42 / 01C2 cd 67 / 01C4 83 fb 08` | confirmed（bytes） |
| 滑鼠：`AH=35h AL=33h` 取向量，ES:BX ≠ 0 → `int 33h AX=0` 要回 AX≠0 | `01F1–0211`；patch 版軌跡通過（int 33h AH=00 ×2） | confirmed（軌跡＋bytes） |
| EXEC 參數區（DS:0x32E）：env=0（繼承）、tail 指標、FCB1=DS:0x34E、FCB2=DS:0x35E | `0215–023C`：`c7 04 00 00`、`89 7c 02`、`8c 5c 04`… | confirmed（bytes） |
| `AX=4B00h` DS:DX=檔名、ES:BX=參數區 | `0240 bb 2e 03 / 0243 b8 00 4b / 0246 cd 21`；patch 版軌跡 AH=4B ×3 | confirmed（軌跡＋bytes） |
| EXEC 成功後 `AH=4Dh` 取回碼；AX 整個被比較（`0b c0`、`83 f8 01`）| `024B b4 4d / 024D cd 21`；`016B–0175` | confirmed（軌跡＋bytes） |
| EXEC 鏈順序：`open.exe` → `gin3ps.exe` → （gin3ps 回碼 ≠0 時）`ending.exe` | `015F–017F` DX=0x2F8／0x301／0x30C | confirmed（bytes；patch 版軌跡走完） |
| 命令列尾巴格式：count byte ＋ 文字 ＋ CR（0x346=`06 "-MSDOS"`、0x317=`03 "9 1"`、0x31C=`03 "9 2"`） | 檔案偏移 0x217／0x21C／0x246 | confirmed（bytes） |
| 三個子程式都是未打包 MZ（`4D 5A` 開頭、無 LZEXE/PKLITE 特徵） | GIN3PS.EXE 189,536、OPEN.EXE 52,265、ENDING.EXE 64,677 bytes 檔頭 | confirmed（bytes） |
| 常式 0x188（`AH=63h`／`int 2Fh AX=4F01h`）在 launcher 主流程**沒有呼叫端** | 全檔無 `call 0x188` | confirmed（bytes）→ 不實作 |

## 2. EXEC：`int 21h AX=4B00h`（READY）

語意（照 DOS）：載入並執行子程式，子程式結束後**回到父程式 `int 21h` 的下一道指令**。

- **AL=00**（load-and-execute）是唯一要支援的子功能；其他 AL 記 unimplemented、CF=1。
- DS:DX 檔名照既有 `resolve()`（只認 basename、大小寫不分）。
  找不到 → AX=2、CF=1（與 open 一致）。
- 參數區 ES:BX：`+0` env 段（0 ＝ 子程式 PSP+2Ch 直接指到父程式的環境段）、
  `+2/+4` 命令列尾巴（off/seg）、`+6/+8` FCB1、`+A/+C` FCB2。
- 子程式記憶體：從 DOS 層的 bump 配置器拿 `0x10（PSP）＋ 映像段數` 一段
  （PSP 前留一段假 MCB，比照 `AH=48h` 的 `+1`）。不夠 → AX=8、CF=1。
- 子程式 PSP：照 `initPSP` 同款建在目標段；尾巴（count＋文字＋CR）
  複製到 PSP+80h（上限 127 bytes）；FCB1／FCB2 **原樣各抄 16 bytes**
  到 PSP+5Ch／6Ch（簡化，見 §6）。
- 子程式進入暫存器：CS:IP、SS:SP 照 MZ 檔頭（重定位套用目標載入段），
  DS=ES=子 PSP；其餘通用暫存器清 0（簡化，見 §6）。
- **父程式狀態**：完整的 CPU 暫存器組（含旗標、IP——已指在 `int 21h` 之後）
  推進 exec 堆疊；子程式結束時彈回還原，CF 清 0。
- **記憶體不做快照還原**。隔離靠配置器：子程式載在父程式之上，
  子程式自己的 `AH=4Ah`／`AH=48h` 只動它之上的空間
  （`GIN3.COM` 先把自己縮到 0x237 段，證據見 §1）。
  ⚠ 子程式對視訊記憶體／BDA／IVT 的寫入**保留**——這符合真 DOS
  （畫面不會因為程式結束而消失）；代價是子程式亂寫父程式空間不會被擋（§6）。
- `AH=22h`／`23h`／`24h` 三個向量在 EXEC 期間由 DOS 保管、回來時還原
  （真 DOS 行為）；其他向量子程式改了就算了。
- **handle 繼承**：子程式看得到父程式開著的 handle（共用同一張表——
  真 DOS 也是同一張 JFT 複製）；子程式結束時關掉它在 EXEC 之後才開的 handle。
- 巢狀 EXEC 用堆疊自然支援（子程式再 EXEC 孫程式）。

## 3. `int 21h AH=4Dh`：取子程式回傳碼（READY）

- AL ＝ 最近一次子程式的結束碼（`AH=4Ch` 的 AL），
  AH ＝ 結束方式，**0 ＝ 正常**。
- 證據：`GIN3.COM` 拿整個 AX 比 `or ax,ax`／`cmp ax,1`（`016B–0175`），
  所以正常結束時 AH 必須是 0，不能留垃圾。
- 沒 EXEC 過就呼叫：AL=0、AH=0（簡化，見 §6）。

## 4. EMS 最小子集：`int 67h`（READY，只含查詢）

- **AH=40h**（取狀態）：回 AH=0（硬體正常）。
- **AH=42h**（取頁數）：AH=0；BX ＝ 可用頁數、DX ＝ 總頁數。
  本機宣稱 **8／8 頁**（128 KB）——恰好滿足 launcher 的 `BX ≥ 8`
  （`01C4 83 fb 08`），不多宣稱。
- 其他 EMS 功能（AH=44h 映射等）：記 unimplemented，AH=0x84
  （未定義功能）。**頁框映射不實作**——等 GIN3PS.EXE 真的映射再說。
- 證據等級：launcher 只呼叫 40h／42h（confirmed by bytes）；
  「8 頁是 EMS 驅動存在性的探測下限，不是記憶體需求」是**假說**，
  GIN3PS.EXE 進來之後若做映射再回來補。

## 5. 字元裝置 `EMMXXXX0` 開啟（READY）

- `AH=3Dh` 的 basename 是 `EMMXXXX0`（大小寫不分）→ 回一個裝置 handle，
  不對映任何檔案；`AH=3Eh` 關閉正常。
- 讀寫這個 handle：讀回 0 bytes（EOF）、寫丟棄——**launcher 開完就關**
  （`01E3–01E7`），讀寫語意沒有證據，純防禦。
- 等級：裝置名與「開成功表示 EMS 驅動在」是 confirmed（bytes＋DOS 慣例）；
  讀寫行為是**假說**。

## 6. 刻意不做（DRAFT，等有證據再說）

| 項 | 現況 | 觸發條件 |
|---|---|---|
| EXEC AL=01/03（overlay）等 | 記 unimplemented、CF=1 | 有程式用到 |
| FCB 格式化（DOS 會解析檔名填 drive 等欄位） | 原樣抄 16 bytes | 子程式讀 PSP+5Ch 判參數 |
| 子程式進入時 AX（FCB 有效性旗標）等暫存器 | 清 0 | 有程式依賴 |
| MZ 的 MINALLOC／MAXALLOC 配置語意 | 忽略，配置 = PSP＋映像 | 子程式要額外 heap 卻拿不到 |
| 父程式記憶體保護 | 無（靠配置器隔離） | 觀測到子程式踩父程式 |
| EMS 頁框映射（AH=44h 等）與實體頁框 | 未實作 | GIN3PS.EXE 呼叫 |
| `int 21h AH=63h`、`int 2Fh AX=4F01h` | 不實作（launcher 內無呼叫端，confirmed） | 本體用到 |

## 7. 驗收

1. 單元測試（不需要原版檔）：
   EXEC 一個臨時造的 MZ 子程式（印字串、exit 42）→ 父程式繼續執行、
   `AH=4Dh` 回 AX=0x0042、主控台有子程式輸出；EXEC 不存在檔案
   → CF=1、AX=2、父程式不受影響；int 67h 40h/42h 回 AH=0、BX=8；
   開 `EMMXXXX0` 成功且可關。
2. 整合（缺檔 skip）：原版未修改 `GIN3.COM` 在 probe 下通過 EMS 偵測、
   EXEC 鏈走到 `GIN3PS.EXE`。
3. `tools/go.sh build ./...` 與 `tools/go.sh test ./...` 全綠
   （CPU 語料缺 testdata 時 skip 可接受）。
