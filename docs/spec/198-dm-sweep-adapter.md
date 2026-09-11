# 198 — DM 巡檢 adapter（`apps/dm`）

狀態：**READY**
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
| `PartyMapX` / `PartyMapY` | `ds:0x3C94` / `ds:0x3CE0` | ⚠ 兩個**不相鄰** |
| `PartyDirection` | `ds:0x3C92` | 0＝北 1＝東 2＝南 3＝西 |
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
    X, Y, Facing       int          // 面向 0＝北 1＝東 2＝南 3＝西
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

#### 負重上限怎麼換掉

`sub_1E063` 用 `oracle.StubValue` 直接換回傳值。三個前提都查過了：

| 前提 | 實際 | 依據 |
|---|---|---|
| 呼叫慣例 | **cdecl far**：`retf` 沒有立即數，參數由呼叫端 `add sp, 6` 清掉 | `0x1e0db` 的 `retf`；呼叫端 `sub_1E0DC` 的 `pop cx / pop cx` |
| 參數 | **一個 far 指標**（`arg_0 = dword ptr 6`，指向 `CHAMPION`） | `0x1e068` 的 `les bx, [bp+arg_0]` |
| 副作用 | **沒有**。整棵呼叫樹（`sub_1DF30`／`sub_1B4FD`／`sub_1089C`／`sub_108D7`／`sub_13DC1`／`sub_1B4CE`／`sub_137D1`）一個記憶體寫入都沒有，全是查詢與算術 | 全檔反組譯逐行掃過，`[bp±n]` 以外的寫入 0 筆 |

`oracle.Stub` 的邊界（`oracle/run.go` 的說明）正好對上：它只支援
「呼叫端清參數、被呼叫者只 `retf`」的 far 常式，而 `F309` 就是。

⚠ **回傳值只有 `AX` 是 16 位元的。** `Stub` 把 `fn` 的 `uint32` 放進 `DX:AX`，
而 `F309` 的呼叫端只收 `AX`——給它 `1<<20` 的話 `AX` ＝ `0`，
**全隊反而變成永遠超重**，效果正好相反。

**定案值：`10000`**（單位是十分之一公斤，＝ 1000 公斤）。兩個條件：

- **遠大於任何真實負重**。正常上限是 `力量×8 + 100` 再按體力與傷勢打折，
  力量頂天也只到九百多。
- **`值×5` 也要放得進 16 位元**（`10000×5 ＝ 50000 < 65536`）。
  `F310_GetMovementTicks`（`sub_1E0DC`）拿 `負重×8` 跟 `上限×5` 比，
  決定走一步僵直 2 刻還是 3 刻。⚠ 這一段 DOS 版走的是 32 位元 helper
  （`sub_10985`／`sub_1089C`），**所以不會溢位**——限制來自 `AX`，不是這裡。
  留這個條件是因為 remake 那一側同一條算式用的是 Go 的 `int`，
  兩側要得出同一個答案。

**remake 那一側用同一個數值**（`internal/game/sweep.go` 的 `SweepMaximumLoad`，
由一支斷言盯著 `×5 < 65536`）。不同值的話 `MovementTicks` 與
`damage.go` 裡 `上限 >> 4` 那些算式會在兩側得出不同結果，檢查點的遊戲刻就對不起來。

### 3.3 檢查點與停法

**檢查點的鍵是路線的步序，不是 `GameTime`。**

⚠ 這一條推翻了「原版側跑到同樣的刻數停下來」那個想法：兩側每一步花幾刻
**正是要比對的東西**，拿它當對齊的鍵等於把結論當前提。`GameTime`
是兩側共用的**時間單位**（規格 73 §7 第 5 條），在檢查點上是**被比對的量**。

一個檢查點的取樣流程：

1. 送出第 `k` 步的輸入。
2. `RunUntil` 到隊伍座標等於路線裡那一步的 `expect`。
   走不到就是差異，`BudgetError` 會把它變成錯誤——**不要靜靜地回來**。
3. 再跑到**下一個刻的邊界**取樣：`ReadParty` ＋ 畫面。

停在刻的邊界要用 `OnCall`，不要在 `Cond` 裡讀記憶體：

```go
type Clock struct{ tick uint32 }

func (c *Clock) Attach(o *oracle.Oracle) {
    o.OnCall(o.IDA(0x10D94), func(o *oracle.Oracle) {
        c.tick = gameTime(o) // 這一道還沒跑，讀到的是「這一刻的」值
        SweepRestore(o)      // §3.2 的寫回掛在同一個點
    })
}

func AtTickAfter(c *Clock, n uint32) oracle.Cond {
    return oracle.NewCond(fmt.Sprintf("GameTime≥%d", n),
        func(*oracle.Oracle) bool { return c.tick >= n })
}
```

⚠ **`RunUntil` 在每道指令執行前都會呼叫 `Cond.ready`**（`oracle/run.go`）。
一刻約 2.54 M 道指令，直接在條件裡讀兩個 word 等於做幾億次記憶體存取。
`OnCall` 只在那一個位址觸發，條件只讀一個 Go 變數。

⚠ **`OnCall` 觸發時 `add word_38B54, 1` 還沒執行**，所以讀到的是遞增**前**的值。
停在「`GameTime` ＝ N」的語意是「第 N 刻結束、第 N+1 刻要開始的那一瞬間」——
與 remake 側 `sweepRestore()` 的位置（`Tick()` 裡 `GameTime++` 之前）同一個語意。

⚠ **預算要自己算。** `oracle.DefaultBudget` 是 1 億道指令，只夠約 **39 刻**。
跑 K 刻要 `Budget(K × 4_000_000)` 這個量級，寧可寬一點——
跑不完會回 `BudgetError`，那是錯誤不是靜默。

畫面在刻邊界時是完整的：主迴圈 `sub_10C94` 的順序是
`sub_33C74` → `sub_3089E`（畫地城視圖）→ `sub_27783`／`sub_23DCB`／`sub_1E967`
→ `0x10D94`（`GameTime++`），繪製排在遞增之前（**強證據**：從呼叫順序讀出來的，
翻頁時機未實測）。

### 3.4 巡檢路線

路線是**一份 JSON，兩側讀同一份**。remake 那一側產生（它有地城解碼與
可達性的圖模型，`internal/assets/map2sweep_test.go`），原版這一側只消費。

```json
{
  "map": 2,
  "start": { "x": 25, "y": 21, "facing": 0 },
  "steps": [
    { "dir": "N", "expect": { "x": 25, "y": 20 } },
    { "dir": "E", "expect": { "x": 27, "y": 4 }, "teleport": true }
  ],
  "checkpoints": [0, 40, 120],
  "unreachable": [[19,8],[19,9],[2,11],[2,12],[8,19],[8,20],[3,28]]
}
```

- **`dir` 是絕對方向**（`N`／`E`／`S`／`W`），不是「前進／後退」。
  面向全程固定為 `start.facing`，用四顆移動鈕（前進、後退、左移、右移）走，
  **不轉向**。這樣「絕對方向 → 按鈕」是一個常數映射，兩側一定一致；
  轉向要花刻，混進來就分不清差異來自移動還是轉向。
  要看四面的檢查點另外加轉向指令，那是後續切片。
- **每一步都帶 `expect`**：走完之後隊伍應該在哪一格。對不上就當場停，
  不要跑完整條路線才發現早就岔開了。
- **`teleport` 標記**該步踩上的是會傳走隊伍的同層傳送格，`expect`
  是傳送**之後**的座標。第 2 層有 8 個這種邊，**避開它們就走不遍**
  （只算相鄰移動的話最大連通塊只有 273／485）。
- **`unreachable` 是明寫的漏掉格**：那 7 格只能經由跨層傳送走過去，
  而踩上去隊伍就被送到第 1 層。路線接受漏掉，不假裝走遍。
  覆蓋率 **478／485（98.6%）**。

⚠ 產生器要用 `dmtool dungeon trigger` 驗一次。`dmtool walk`
**不跑感應器**，門在它裡面打不開（2026-09-10 在第 0 層踩過）。

### 3.5 CLI

`apps/dm/cmd/sweep` 讀玩家提供的 EXE／資料根與一份路線 JSON（§3.4），
寫進指定輸出目錄：每個檢查點一份 `party.json`（§3.1 的結構）＋ `frame.png`
＋總表 `receipt.json`。路線裡每一步的 `expect` 對不上就當場中止並回非零碼。

收據只記輸入檔的 basename 與 SHA-256、步數、artifact 雜湊；
**不得記錄私有絕對路徑，也不得嵌入原版 bytes**。

## 4. 驗證與停止線

1. **純測試**：結構解碼（給一段合成的 319 位元組緩衝，欄位要落在對的偏移）。
2. **真實資料測試**（由 `DM_ORACLE_EXE`／`DM_ORACLE_ROOT` 選配）：
   - `ReadParty` 從既有檢查點展開讀出 `"ELIJA"` 與 `GameTime` ＝ 458。
   - 巡檢開著時跑 N 刻，生命力／食物／水不下降、中毒數維持 0。
   - **反對照**：巡檢關著時同一段跑下來，食物與水要確實下降。
     ⚠ 少了反對照就分不出「擋住了」與「這個情境本來就不會發生」。
3. **路線與停法**（同樣由私有資料選配）：
   - 吃一份只有兩三步的小路線，每一步的 `expect` 都對得上。
   - **反對照**：故意把某一步的 `expect` 改錯，跑起來要**失敗**。
     少了這一條就分不出「對上了」與「根本沒在檢查」。
   - 停在刻的邊界時 `GameTime` 是遞增**前**的值（§3.3），用連續兩個檢查點的
     差值驗證。
4. **負重 stub**：開著時 `F309` 回 `10000`；`F310`（`sub_1E0DC`）
   走的是「沒超載」那一支。**反對照**：關掉 stub，同一個負重下 `F310`
   走超載那一支。
5. 缺少私有資料只能明確 skip，**不能偽造 pass**。

## 5. 原本的未解項（2026-09-11 全數收斂，規格升 READY）

1. ~~**刻的邊界掛在哪。**~~ IDA `0x10D94` ＝ 執行時 `070a:0144`，見 §3.2。
   用 `-watch` 監看 `GameTime` 那 4 個位元組反追到的（`GameTime` 每刻都變，
   所以不受「監看只記值改變」那個限制影響）。
2. ~~**`sub_1E063` 的 stub 方式。**~~ 見 §3.2「負重上限怎麼換掉」：
   cdecl far、**一個** far 指標參數、整棵呼叫樹零副作用，`StubValue` 直接適用；
   值定案為 `10000`，兩側同值。
3. ~~**檢查點的定義。**~~ 見 §3.3：**鍵是步序不是 `GameTime`**，
   停法是「走到 `expect` → 再跑到下一個刻的邊界」，刻邊界用 `OnCall` 掛。
4. ~~**巡檢路線。**~~ 見 §3.4：一份 JSON 兩側共用，絕對方向的步序、
   每步帶 `expect`、傳送格明標，覆蓋 478／485 並明寫漏掉的 7 格。

## 6. 實作時踩到的三件事

**方向按鈕的滑鼠熱區**原本列為未解，其實早就有了：六顆鈕的座標在
dungeon_master 專案的 `tools/dosgolem-shots.sh`（在 DOS 原版上實跑過），
指令編號與範圍在它的 `docs/spec/73` §5（`x 234–318`、`y 125–167` 那一塊）。
已經抄進 `apps/dm/boot.go` 的 `moveHot`。

⚠ **選單項目的可點區域是圖示不是文字**：`ENTER` 是那顆綠寶石，
點在 `ENTER` 那幾個字上事件全都送到、回呼也跑了、畫面毫無變化。
**`-click-premove` 對 DM 無效**，它等的是滑鼠輪詢次數，而 DM 全程只輪詢 3 次。

**`fires` 是多程式碼段的。** 35 個 `CODE` 段（`apps/dm/segs.go` 的 `codeSegs`，
由 dungeon_master 專案 `tools/dm_segs.py` 從 IDA 匯出）：主迴圈 `sub_10C94`
在 `seg001`、`F309` 在 `seg010`。

⚠ **要跳進去執行（`oracle.Call`）就得給對的 `CS`。** `oracle.IDA()` 回的是
**正規化**的段（線性 >> 4，偏移 0–15），機器碼的位置是對的，
但常式裡第一個 `call near ptr` 就跳到別的地方——**而且不會報錯**。
實測過一次：跑滿預算，停在 `seg028` 的繪圖常式裡。`apps/dm` 因此提供
[`Bridge.Code`]，讀寫與 `OnCall`／`Stub` 仍用只看線性位址的 `Bridge.IDA`。

段表同時是 §2 那張表的正對照：`dseg` 的基底是 IDA `0x34ED0`，
正是 DS 偏移換算用的那個數；而 `(0x34ED0 − 0x10000) / 16 ＝ 0x24ED`
就是 DGROUP 相對映像起點的節數，不必靠兩個實測值相減。

**`int 21h AH=3Bh`（切目錄）與 `dmset` 開不到都是正常的。** 前者在 dosgolem
有實作但刻意記一筆（目錄是假的，開檔一律相對 root）；後者是 `selector`
存設定用的檔，第一次跑本來就沒有——主控台那三個問題正是因此才會問。
判準不是「報告是空的」，而是**除了它們以外沒有開不到的檔**。
