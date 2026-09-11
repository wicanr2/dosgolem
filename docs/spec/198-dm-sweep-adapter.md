# 198 — DM 巡檢 adapter（`apps/dm`）

狀態：**DRAFT**
日期：2026-09-11

## 1. 目的與範圍

《Dungeon Master》的 remake 專案（`~/cht/dungeon_master`）要拿**全層巡檢**當對拍載具：
兩側走遍同一層、人員存活、物品全拿，逐檢查點比對。remake 那一側已經做好了
（`internal/game/sweep.go`，四項各有測試與反對照）；**這一份規格是原版那一側**。

本切片只提供四件事：

1. 讀出隊伍與勇士狀態（座標、面向、遊戲刻、生命力／食物／水、物品欄）。
2. 在**刻的邊界**把生命力、食物、水寫回，並清掉中毒事件數。
3. 把負重上限那一支換成固定回傳值。
4. 把上面三件事包成可重跑的 CLI，輸出結構化收據。

**不宣稱**：完整流程自動化、存檔支援、怪物行為觀測、跨層巡檢。

⚠ **[HARD] 巡檢跑出來的不是 parity 證據**，它證明的是「這一層的畫面、物品與感應器
在兩側對得上」。這一條與 remake 規格 73 §6 同義，兩邊都要寫。

## 2. 證據

四個攔截點與 `CHAMPION` 結構由 remake 專案反組譯 `fires` 取得，
逐行對過 ReDMCSB 的原始碼（`~/cht/dungeon_master/docs/re/73-dos-champion-intercepts.md`，
推論等級**已確認**）。這裡只抄結論，不重複推導：

| 項目 | 原版函式 | `fires` |
|---|---|---|
| 傷害 | `F321_AddPendingDamageAndWounds_GetDamage` | `sub_1EAB3` |
| 中毒 | `F322_CHAMPION_Poison` | `sub_1EC7D` |
| 飢餓與口渴 | `F331_ApplyTimeEffects` 第二段 | `sub_1F232` 的 `loc_1F445` |
| 負重上限 | `F309_CHAMPION_GetMaximumLoad` | `sub_1E063` |

全域與結構（DS 偏移；DS 基底的線性位址是 `0x2B320`）：

| | 偏移 | 備註 |
|---|---|---|
| `Party.Champions` | `ds:0x3592` | 一筆 `0x13F` ＝ 319 位元組 |
| `GameTime` | `ds:0x3C84` | 4 位元組 |
| `PartyMapIndex` | `ds:0x3C8A` | |
| `CurrentHealth` | `+0x34` | 勇士結構內 |
| `PoisonEventCount` | `+0x2A` | |
| `Food` / `Water` | `+0x42` / `+0x44` | |
| `Slots[30]` | `+0xD3` | 一格 2 位元組 |
| `MaximumHealth` | `+0x36` | 寫回的目標值 |

刻的邊界：**IDA `0x10D94` ＝ 執行時 `070a:0144`**（主迴圈裡的 `add word_38B54, 1`）。

實測（`hired.state` 展開）：`Champions[0].Name` ＝ `"ELIJA"`、`Title` ＝ `"LION OF …"`、
`GameTime` ＝ 458、`PartyMapIndex` ＝ 0。

⚠ **`Statistics[7][3]` 在 x86 沒有補齊位元組**，所以 `Skills` 從奇數位址 `+0x5B` 起算。
照 68000 的對齊習慣推會整段偏一。

## 3. 契約

### 3.1 狀態讀取

```go
type Champion struct {
    Name, Title            string
    CurrentHealth, MaxHealth int
    Food, Water            int
    PoisonEventCount       int
    Slots                  [30]uint16
}
type Party struct {
    MapIndex, GameTime int
    Champions          []Champion  // 只含 MaxHealth > 0 的
}
func ReadParty(*oracle.Oracle) (Party, error)
```

### 3.2 巡檢介入

**做法是「每刻寫回」不是「攔截函式」**（remake 規格 73 §3 定案）。理由：

- `F331` 帶 `_COPYPROTECTIONF` 後綴，是被防拷邏輯穿插過的函式，**不得整支 stub**；
  而且它同時做體力回復與法術倒數，擋掉會改變別的規則。
- 兩側的介入點都變成「一刻的邊界」這個雙方都有的概念，對得起來。

負重是例外：它不是狀態而是每次算出來的上限，只能換回傳值。

```go
// SweepRestore 掛在刻的邊界，每次把生命力／食物／水寫回、中毒清零。
func SweepRestore(*oracle.Oracle) error
// SweepMaximumLoad 把 sub_1E063 換成固定回傳值。
func SweepMaximumLoad(*oracle.Oracle, value int) error
```

**刻的邊界是 IDA `0x10D94` ＝ 執行時 `070a:0144`**（主迴圈 `sub_10C94` 內）：

```
0x10d94  add  word_38B54, 1      ; GameTime++ 低位
0x10d99  adc  word_38B56, 0      ; 進位到高位
0x10d9e  test word_38B54, 1FFh   ; GameTime & 0x1FF
```

**寫回掛在這一道之前**，與 remake 那一側對齊——remake 的 `sweepRestore()` 在
`Tick()` 裡的位置是 `ApplyPendingDamage` 之後、`GameTime++` 之前
（`internal/game/clock.go`）。

⚠ 監看看到的 IP 是 `070a:0149`（`adc` 那一道），因為**監看記的是寫入完成後的 IP**。
查反組譯時要往前看一道，不然會停在進位指令上。

實測（`game.state` 展開跑 20M 指令）：`GameTime` 逐刻 `0x22 → 0x23 → 0x24 …`，
每刻約 2.54M 道指令。

⚠ **傷勢（`Wounds`）與受傷音效照樣發生**，寫回不擋它們。那是刻意的：
兩側都會發生，比較得出來。

### 3.3 CLI

`apps/dm/cmd/sweep` 只讀玩家提供的 EXE／資料根，寫進指定輸出目錄：
每個檢查點一份 `party.json`（§3.1 的結構）＋ `frame.png`＋總表 `receipt.json`。

收據只記輸入檔的 basename 與 SHA-256、步數、artifact 雜湊；
**不得記錄私有絕對路徑，也不得嵌入原版 bytes**。

## 4. 驗證與停止線

1. **純測試**：結構解碼（給一段合成的 319 位元組緩衝，欄位要落在對的偏移）。
2. **真實資料測試**（由 `DM_ORACLE_EXE`／`DM_ORACLE_ROOT` 選配）：
   - `ReadParty` 從既有檢查點展開讀出 `"ELIJA"` 與 `GameTime` ＝ 458。
   - 巡檢開著時跑 N 刻，生命力／食物／水不下降、中毒數維持 0。
   - **反對照**：巡檢關著時同一段跑下來，食物與水要確實下降。
     ⚠ 少了反對照就分不出「擋住了」與「這個情境本來就不會發生」。
3. 缺少私有資料只能明確 skip，**不能偽造 pass**。

## 5. 未解（要從 DRAFT 升到 READY 之前要補）

1. ~~**刻的邊界掛在哪。**~~ **已解決 2026-09-11**：IDA `0x10D94` ＝
   執行時 `070a:0144`，見 §3.2。用 `-watch` 監看 `GameTime` 那 4 個位元組反追到的
   （`GameTime` 每刻都變，所以不受「監看只記值改變」那個限制影響）。
2. **`sub_1E063` 的 stub 方式。** `oracle` 的 `Stub`／`StubValue` 對 far call
   的返回與堆疊平衡要確認一次；`F309` 是 `retf` 且有兩個參數。
3. **檢查點的定義。** remake 那一側用 `GameTime` 對齊（規格 73 §7 第 5 條），
   原版側要在同樣的刻數停下來取樣——停的方式（跑到某個 `GameTime` 值）要定案。
4. **巡檢路線。** remake 那一側已經確認第 2 層可達 478／485（98.6%），
   路線本身還沒產生；兩側要走同一條。
