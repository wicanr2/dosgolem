# 191 — BIOS ROM 尾巴、ROM 唯讀、狀態檔的 A20；`-watch` 多段與 `-regs-at` 不去重

狀態：**READY**
日期：2026-09-17
前置：[`189-a20-gate-and-hma-addressing.md`](189-a20-gate-and-hma-addressing.md)（A20 與 HMA）、
[`018-all-writes-through-write8.md`](018-all-writes-through-write8.md)（寫入路徑）、
[`011-xms.md`](011-xms.md)（HMA 擁有權與 A20 計數）。
動機：《銀河英雄傳說III SP》戰略階段艦隊全滅之後轉跑 `ENDING.EXE`
（`~/cht/logh3/docs/re/334`、logh3 issue #73）；觀測旗標的兩個洞（logh3 issue #54）。

---

## 1. 證據

| 事實 | 證據 | 等級 |
|---|---|---|
| 這台機器 `FFFF0`–`FFFFF` 全是 0，而且可寫 | `New()` 只建 BDA、IVT 與 `StubSeg` 的 stub；`Write8` 對 `F0000` 以上沒有分支 | confirmed（程式碼） |
| GIN3 把 `FFFF:FFFF` 當物件指標，`2376:02DE` 從 `FFFF1` 讀 `AH` | logh3 `fcx-a20-rom` 的逐道軌跡；本規格普查 `fcx-romsurvey-phzt4` 讀取監看：`FFFF5`–`FFFFA` ← `2376:0206`／`0218`／`022D`／`023E`，`FFFF1` ← `2376:02E2` | confirmed（軌跡） |
| `AH = 0` 時 `2376:00B2` 的迴圈把資料段寫壞，計時器向量被改，最後 `EXEC ENDING.EXE` | logh3 `docs/re/334` §一 | confirmed（軌跡） |
| 只把那 16 bytes poke 成 DOSBox 的值，回合照常走完 | logh3 `fcx-a20-rom`（跑滿 11 億道、主控台空、沒開 `ending.exe`） | confirmed（實驗） |
| DOSBox-X 的 ROM 頁寫入：記 log、不改內容 | DOSBox-X `src/hardware/memory.cpp` 的 `ROMPageHandler::writeb/w/d` | confirmed（原始碼，只讀） |
| DOSBox-X 開機寫 `FFFF0 = EA`、`FFFF1 = RealOff(重置位置)`、`FFFF3 = RealSeg`、`FFFF5 = "01/01/92"`、`FFFFE = FC`（非 Tandy／PCjr／MCGA）、`FFFFF = 55`（非 Tandy） | DOSBox-X `src/ints/bios.cpp` 的 `write_FFFF_signature`；重置位置在 PC 架構是 `F000:E05B` | confirmed（原始碼，只讀） |
| js-dos bundle 用的是 vanilla DOSBox 還是 DOSBox-X 後端 | bundle 的 `jsdos.json` 沒寫 | unknown |
| `-watch A-B,C-D` 只看 `A-B` | `cmd/probe` 用 `Sscanf("%x-%x")` 解，逗號後面被丟掉，不報錯 | confirmed（程式碼） |
| `-regs-at` 漏掉 `fcx-assault-ast` 的第一擊 | 該批 `20B4:17B8`／`17C6`／`0EB5`／`18AF` 的 SI 從 `0000` 換到 `0004` 的那一筆都不在報告裡；probe 寫死「DS:SI 比上一筆往前 1–16 就跳過」（blit 去重） | confirmed（報告＋程式碼） |
| `-regs-at` 在 `-regs-from` 下 0 命中（issue #54 第二例） | logh3 的 12 份帶 `-regs-at 3765:006C` 的報告都有命中，重現不出來 | unknown |

## 2. ROM 尾巴

開機時（`machine.New`）`FFFF0`–`FFFFF` 是：

| 位址 | 內容 | 語意 |
|---|---|---|
| `FFFF0` | `EA 5B E0 00 F0` | 處理器重置進入點 `jmp far F000:E05B`（PC/AT 的 POST 位置） |
| `FFFF5` | `30 31 2F 30 31 2F 39 32` | BIOS 日期 `"01/01/92"`（`MM/DD/YY`） |
| `FFFFD` | `00` | |
| `FFFFE` | `FC` | 機型位元組：AT |
| `FFFFF` | `55` | 簽章 |

- 這台機器沒有 BIOS 程式碼，`F000:E05B` 不會被執行（沒有重置）；那 5 bytes 是給**讀它的程式**的。
- `FFFFF = 55` 不是 PC/AT 的規定，是 DOSBox 自己的位元組。取它的理由：本專案的 oracle 用途是
  「重現原版執行環境」，而那個環境是 DOSBox。真機 BIOS 在那一格各有各的值。
- 機型位元組 `FC` 對應 `machine=svga_s3`（DOSBox 的非 Tandy／PCjr／MCGA 機型都是 `FC`）。
  要模擬別的機型時才需要改成可設定，現在不做。
- `F0000`–`FFFEF` 維持 0。沒有證據說有程式讀那一段；有的話另開規格。

## 3. ROM 唯讀

`Write8` 對 `F0000`–`FFFFF`（A20 環繞之後的線性位址）一律不寫，`ROMWrites` 加一，
最前面 20 筆記進 `ROMWriteLog`（步數、位址、值、`OpAddr`）。`WriteBytes`／`Write16` 走 `Write8`，同樣適用。

- **忽略要看得見**：真機上寫 ROM 是無聲的，但寫 ROM 的程式通常是指標飛了。probe 的報告在
  `ROMWrites > 0` 時印出次數與前幾筆。
- 監看（`WatchWrites`／`WatchWrite`）看不到 ROM 寫入——值沒有變。要找是誰寫，看 `ROMWriteLog`。
- 普查（§6）的結果決定了唯讀的範圍可以是整個 `F0000`–`FFFFF`。

## 4. 狀態檔與快照

- 機器狀態檔版本 `3`：多存 `A20` 與 `HMA`。DOS 狀態檔版本 `3`：多存 `HMAOwned` 與 `A20Local`（`011-xms`）。
- **`2` 仍然讀**：缺的欄位讀成零值（A20 關、HMA 全 0、HMA 沒人拿、計數 0）。那正是 v2 執行器讀 v2 檔
  得到的結果，所以舊檢查點展開的行為不變。只有「存檔那一刻 A20 開著」的 v2 檔會錯，那種檔要從開機重錄；
  `Machine.LegacyState` 記下這次讀的是舊檔，probe 印一行警告。
- 不拒讀 v2 的理由：上層專案有上千份收據掛在檢查點鏈上，拒讀等於整條鏈從開機重錄，而除了 A20 之外
  內容完全相容。
- **ROM 不是狀態**：`LoadState` 在倒回記憶體之後重填 ROM 尾巴。舊版存下的那一段是 0，照舊倒回去的話
  改了執行器也沒用。
- 記憶體內快照（`Snapshot`／`Restore`）一併帶 A20 與 HMA，還原後重算取指令快路徑（`syncCodeFastPath`）。

## 5. probe 旗標（logh3 issue #54）

- `-watch`：逗號分隔的 `<lo>-<hi>` 或單一 `<位址>`（十六進位），**每一段都監看**；解不出來的那一段報錯。
  每段各掛一個 `WatchWrite`，不再佔用只留一個回呼的 `WatchWrites`——後者會被 `-watch-video` 蓋掉。
  報告語意不變：只列值有變的寫入。
- `-regs-at`：預設記下每一次命中（上限 `-regs-max`）。舊版寫死的「DS:SI 往前 1–16 就跳過」改成選用的
  `-regs-skip-blit`：它只對 blit 迴圈成立，一般函式的參數指標常常只差幾個 byte。
- 這兩項只改報告，不改執行；收據的畫面不受影響（§6 以重跑確認）。

## 6. 普查與收據

改唯讀之前，以前一版（`b5485eb`）跑 logh3 四條長收據，監看 `F0000`–`FFFFF` 的寫入與讀取：

| 批次 | 內容 | 寫入（值有變） | 讀取 |
|---|---|---|---|
| `fcx-romsurvey-turnnext` | 冷開機、主選單到 12 個月的回合（22.5 億道） | 0 | 0 |
| `fcx-romsurvey-month3` | 冷開機、月底處理（22.5 億道） | 0 | 0 |
| `fcx-romsurvey-phzt4` | 費沙第四回合全滅 | 0 | 7 筆，全在崩潰路徑（§1） |
| `fcx-romsurvey-engend` | 交戰演出結束 | 0 | 0 |

兩條冷開機的畫面與原收據 `turnnext`／`watchmonth3` 相同（加監看不改執行）。值沒有變的寫入（寫 0 進全 0 的 ROM）
監看看不到，但那種寫入在唯讀前後結果相同，不影響判斷。結論：GIN3 正常路徑不讀也不寫這一段，唯讀範圍取整個
`F0000`–`FFFFF`。其他專案的程式沒有普查；它們若寫 ROM，報告會印出來（§3）。

## 7. 驗收

- 單元測試：`TestROMTailAtBoot`、`TestROMTailThroughFFFFPointer`、`TestROMWritesIgnored`、
  `TestStateKeepsA20AndHMA`、`TestLegacyStateRefillsROM`、`TestSnapshotKeepsA20`（`internal/machine`）；
  `TestParseWatchRangesKeepsEveryRange`、`TestParseWatchRangesRejectsJunk`、`TestRegsAtKeepsNearbyPointers`（`cmd/probe`）。
- 既有測試全過。這次沒動 CPU 層；SingleStepTests 的語料不在工作副本時照規則 skip。
- logh3：全部收據在這一版重跑，畫面有變的逐條說明原因（logh3 `docs/re/336`）；
  `fcmd-occ-phz-t4` 不再轉跑 `ENDING.EXE`。
