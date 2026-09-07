# dosgolem

*[English](README_en.md)*

**無頭、決定性、可以當 Go 套件 import 的 DOS 執行器**——為**程式化觀測**而生，
不是為了給人看畫面。你問「原版跑到這一步時這個變數是多少」，它當場回答。

第一個案例是《大富翁2》（大宇資訊，1993，DOS）的 remake
[`rich2`](https://github.com/wicanr2/rich2)。那個案例把機器層與觀測層推到
能用的程度；接第二個程式時要寫的只有最上面一層，見
[分層](#分層哪些是通用的哪些是某個程式專屬的)。

## 起源：DOSBox-X 不好跟 AI agent 配合

DOSBox-X 是為「人坐在前面玩」設計的——畫面出在 X 上、輸入靠鍵盤事件、
要觀察就得截圖。人玩的時候這樣剛好。

AI agent（Claude Code、Codex）在 remake 專案裡做的是另一件事：**對拍**，
把 remake 的畫面與狀態逐項比對原版。對拍需要的是**問得到、量得到、可重播**，
不是看得到。「這一格的租金算成多少」隔著截圖只能從像素反推；
「跑到收租常式再停下來」隔著 X 只能睡兩秒再猜。

dosgolem 是為這個場景寫的 DOS 執行器：讓「原版怎麼做」可以在 `go test` 裡
直接量，而不是隔著 X 看畫面。

## 名字

dosgolem ＝ DOS 魔像。golem 是被造出來替你做事的自動人偶，
正好是這個工具在 AI 工作流裡的位置；而 "go" 就寫在名字裡。

## 現在這條路的成本

`rich2` 目前的對拍是 docker ＋ Xvfb ＋ DOSBox ＋ xdotool ＋ 截圖，
累積了 54 支腳本、6,116 行，每一支各自重寫一次
「送鍵 → sleep → 截圖 → 判像素」。

| 量 | 值 |
|---|---|
| 一次 `docker exec` 往返 | 0.058 秒 |
| 送一個鍵 | 1 次 exec ＋ 0.35 秒 sleep |
| 取一張截圖 | 2 次 exec ＋ 0.7 秒 sleep |
| 自走腳本每前進一步 | 2.2 秒 sleep |

瓶頸不是 IPC。0.058 秒的往返旁邊掛著 0.35 到 2.2 秒的等待，比例 6 到 38 倍。
那些 sleep 存在的唯一理由是：**主機沒辦法問模擬器「你跑完這一幀了嗎」，
只能拿牆上的時鐘猜。** 那同時是慢的原因與不穩的原因。

派生出來的四個成本：

- 判準只能讀像素。「選單框裡有青字」會在棋盤的青綠色建築上誤中。
- 走到罕見畫面很貴。150 回合擲骰一次都沒踩到法院，最後改存檔碰運氣。
- 相位對不齊。調色盤 240–249／250–254 在輪轉，單張截圖比對會得出假結論。
- 亂數序列對不上。原版 BASIC 的 `RANDOMIZE` 與 remake 的 RNG 沒有可重播對應。

重寫 DOSBox-X 不能解決這件事。它有 943,452 行 C++、4,501 個原始檔、479 MB，
而問題出在「只能從外面看畫面」，不在模擬器本身。它的除錯器又是 ncurses
互動介面，不是可程式化的 API，改它也還是得自己再包一層。

完整評估：[`rich2/docs/spec/082`](https://github.com/wicanr2/rich2/blob/master/docs/spec/082-parity-oracle-emulator.md)。

### DOSBox-X 留著

它是時序的參考實作，也是交叉 oracle。dosgolem 畫出來的每一張，
都要拿 DOSBox-X 的索引截圖驗過才算數——否則就是拿自己驗自己。

## 為什麼做得起來

三個量出來的事實：

1. `RUN_full.EXE` 主程式區 52,892 bytes 只用到 **62 個助憶碼**，
   而且是 8086 加上兩個 80186 指令——`PUSH imm16` 與 `PUSH imm8`，
   主程式區 3,345 次、全檔 5,280 次。保護模式、分頁、32 位元運算元一概用不到。
2. **不需要 x87**：全檔 876 個 `INT 34h–3Dh`，浮點走 Microsoft 浮點模擬器，
   而那個模擬器連結在 binary 裡面，執行期只跑整數指令。
3. 系統服務面很窄，而且 `rich2` 那邊用 unicorn 已經走過一遍、跑到防拷畫面。

## 它長什麼樣

```go
o, _ := oracle.Load(exe, root)        // 原版由玩家自備
o.RunUntil(oracle.PasswordScreen)     // 跑到防拷畫面，跑不到會回錯誤
o.Click(102, 125)                     // 點色塊，對齊指令數不是 sleep

snap := o.Save()                      // 1 毫秒的快照
o.Restore(snap)                       // 從同一個狀態展開下一個變體

v := o.Word(o.DS(0x1BE))              // 直接讀原版的變數
shot := o.Indexed()                   // 320×200 色號，不經過 X
                                      // （平面模式用 o.Screen()）
```

`o.DS(0x1BE)` 與 `o.IDA(0x25BF6)` 吃的就是 `rich2` 那邊 RE 筆記裡的位址，
不必再換算一次。判準因此從「像素」變成**原版自己的呼叫參數**——
`OnCall` 攔住繪製常式，等於讓原版說出「我在 (154,54) 用色 60 印了第 229 則」。

對拍也就變成 CI 跑得動的 `go test`：程序內跑到防拷畫面 1.8 秒；
連 docker 啟動一起算，從冷啟動打完三題防拷 5.3 秒。
DOSBox 那條線光容器啟動加開機就 25 秒，之後每前進一步再 2.2 秒。

介面定案在 [`docs/spec/005`](docs/spec/005-oracle-api.md)（READY）。

## 要走到哪裡

終點是**讓 AI agent 能自己驗證 remake 對不對**，而且驗證的依據是原版本身，
不是人事後看截圖判斷。

具體是三件事：

1. **問得到。** agent 在 `go test` 裡問「原版走到這一步時，這個變數是多少、
   畫面上這一格是什麼色號、它用什麼參數呼叫了繪製常式」，當場拿到答案。
   現在做到了讀變數、讀畫面、攔呼叫。
2. **可重播。** 同一組輸入永遠得到同一個畫面。時鐘是指令數不是牆上的時間，
   亂數種子與原版的固定種子版對齊——這是 MVP-B 能逐點 100% 的前提。
3. **走得到。** 要驗一個罕見畫面（法院、破產、某張卡片），agent 得能走到那裡。
   快照讓「從同一個狀態展開多個變體」變成 1 毫秒的事；
   防拷三題現在就是這樣自動打完的。

做完這三件，`rich2` 那 54 支 DOSBox 腳本、6,116 行可以收斂成一組宣告式的對拍表，
而且跑在 CI 裡。

**已經驗證過的範圍仍然只有那一個 binary。** 別的 DOS 程式跑不跑得動，
取決於它用到哪些指令與 DOS 服務——機器層是照規格寫的，但**只有被跑過的
路徑算數**。接新程式的第一件事是跑 `cmd/probe`：它會把「用到而還沒實作的
服務」列出來（`o.Unimplemented()`），那份清單就是待辦。

## 分層：哪些是通用的，哪些是某個程式專屬的

接第二個程式的時候會發現，這裡面的東西不是同一個性質。四層：

| 層 | 是什麼 | 換一個程式要不要改 |
|---|---|---|
| **機器** `internal/cpu`／`internal/dos`／`internal/machine` | 8086 ＋ 80186、DOS 與 BIOS 服務、VGA、PIT、滑鼠 | **不用**。缺的服務就補，補完誰都受惠 |
| **觀測** `oracle/` | `Load`／`RunUntil`／`Click`／`Save`／`Restore`／`Search`／`Indexed`／`OnCall` | **不用** |
| **runtime** `runtime/basic/` | 編譯器與執行期的慣例：MS BASIC 的 `RND`（LCG）、陣列描述子 | **同一個編譯器就複用**；換一個（Turbo Pascal、Clipper、VB p-code）就在 `runtime/` 下新增一包 |
| **程式** `apps/rich2/`／`apps/wolong/` | 那一支程式自己的位址、按鈕座標、流程 | **一定要自己寫** |

第三層是接第二個程式時才看得出來的那一層，也是「下一個案例不必 fork」的關鍵。

拆的時候有一條分得特別細，值得記：**演算法通用，位址不通用。**
`runtime/basic` 有 LCG 的公式與追蹤，但 `RND` 的進入點與狀態變數偏移
放在 `basic.Config` 裡由呼叫端給——runtime 是連結進去的，
同一份 MS BASIC 落在不同 binary 會在不同位址。拿別的程式的值套過來
不會報錯，只會一次都攔不到。

反過來，`Array` 原本放在 `oracle/`（通用層）**是錯的**：通用的是 `o.Word`，
而「描述子前兩個 word 是 `(位移, 段)`、索引是列主序」是 BASIC 編譯器的版面。
換一個 Turbo Pascal 編的程式那個解讀直接錯，**而且不會報錯**，
只會讀出一片看起來像資料的東西。它現在在 `runtime/basic`。

過程與判準見 [`docs/spec/006`](docs/spec/006-layering.md)。

**接一個新程式要寫的是第四層**，前三層照用。第四層的形狀可以照抄
`apps/rich2/`：

```
位址常數      DescPlayer = 0x1146    陣列描述子、單一變數
狀態讀取      Cash(o, player)        把位址包成有名字的東西
流程函式      ToBoard(o)             從冷啟動走到某個畫面
攔截點        WatchDispatch(o)       掛 OnCall，讓程式自己說它在做什麼
```

最後一項是最省力的一個：與其想辦法「走到卡片格」，不如攔住**分派器**，
一路走下去，它踩到的時候自己會說。

### 第二個案例實際花了多少：《臥龍傳》

[`wolong_cht`](https://github.com/wicanr2/wolong_cht) 是 NEO･GETEN《臥龍傳》
（松崗 DOS/V 中文版，1995）的 remake。那一支是 mode 12h、640×480、
16 色四平面，與第一個案例（mode 13h 線性 256 色）完全不同。

`KI.EXE` **不必改任何程式碼**就跑得動 330 萬道指令、開得了 14 個資料檔。
要補的只有兩件機器層的事：

- **VGA 16 色平面模式**（[`docs/spec/007`](docs/spec/007-vga-planar.md)）
- **DOS/V 字型服務 `INT 15h AH=50h`**（[`docs/spec/008`](docs/spec/008-wolong-services.md)）

加上 `apps/wolong/`（畫面幾何、開機流程、座標換算）約 200 行。
結果是開機 → 選劇本 → 選君主 → 主畫面**五格畫面對 DOSBox-X 的原版擷取
每一區都是 0 個不同像素**，整條鏈 0.99 秒（DOSBox-X 那邊同一條時間軸的
`wait` 加起來是 146 秒）。

四個「不會報錯」的坑記在
[`docs/findings/003`](docs/findings/003-wolong-boots-and-matches.md)——
其中兩個（`Click` 拿 mode 13h 的緩衝區判斷畫面有沒有變、
`int 33h AX=5` 的 `BX` 是輸入）是**第一個案例結構上踩不到**的，
接第二個程式才會浮出來。

## 現況

| | 里程碑 | 狀態 |
|---|---|---|
| MVP-A | 8086 整數指令核心，SingleStepTests/8088 v2 全綠 | **323／323 檔綠**，一項已知差距見下 |
| MVP-B | 跑到防拷畫面，與 DOSBox-X 索引截圖逐點相同 | **64,000／64,000 ＝ 100%** |
| M2 | 輸入與時序（鍵盤／滑鼠／PIT）| 滑鼠、鍵盤、PIT 都接了；**冷啟動連走十四回合全自動**（擲骰、買地、租金、退出銀行那種畫面）。週期精確未做 |
| M3 | 儀器層：breakpoint／watchpoint／call trace／RND 記錄／savestate | `OnCall`、`Caller`、快照、**RND 追蹤**可用；breakpoint 與 watchpoint 未做 |
| M4 | Go API（`oracle` 套件）| **可用**，[`docs/spec/005`](docs/spec/005-oracle-api.md) READY |
| M5 | 迴歸：重跑 `rich2` 既有的 parity 收據 | 未開始 |
| M6 | **分層**：把 runtime 與程式專屬拆開，讓第二個案例不必 fork | **做完**：`runtime/basic`／`apps/rich2`，規格 [`docs/spec/006`](docs/spec/006-layering.md) |
| M7 | **第二個案例**：《臥龍傳》（松崗 DOS/V，1995）| **做完**：VGA 16 色平面模式（[`007`](docs/spec/007-vga-planar.md)）＋ DOS/V 字型服務（[`008`](docs/spec/008-wolong-services.md)）＋ `apps/wolong`。開機到主畫面**五格 × 五區全部 0 px**，見 [`docs/findings/003`](docs/findings/003-wolong-boots-and-matches.md) |

規格在 [`docs/spec/`](docs/spec/)，標 `DRAFT` 或 `READY`；
只有 READY 的可以動手。

### 怎麼接進自己的專案

用 Go workspace 接**本機副本**，不要在 `go.mod` 裡 `replace`
（`rich2` 就是這樣接的）：

```sh
# rich2/go.work（gitignore，只在容器裡成立）
go 1.24.0
use .
use /dosgolem
```

`rich2/tools/go.sh` 偵測到旁邊有 dosgolem 就唯讀掛載進去。對拍測試放在
`-tags oracle` 之下，所以沒有這份掛載的人跑 `go test ./...` 會跳過，不會失敗。
**這個做法與遊戲無關**，任何要拿 dosgolem 當 oracle 的專案都適用。

⚠ **不要在 `go.mod` 加 `require` 指向不存在的版本。** 那會讓每一個 package
都去抓 proxy，而建置容器是 `--network none`——結果是整個專案編不過，
錯誤訊息還指向一堆與這件事無關的檔案。

下面這幾類是 `rich2` 實際接出來的東西，列在這裡是因為**形狀可以照抄**——
換一個程式，比的東西不同，但「兩條路徑算同一件事，逐項比」這個做法一樣。

第一個接起來的對拍是調色盤：拿原版執行期的 VGA DAC 對 remake 解碼 `256.PAT`
的結果。256 色裡有 30 色不同，而且落點是有意義的——`192`–`206` 是台灣地圖的
十五個縣市（`256.PAT` 裡是藍色漸層，防拷畫面把它們全改成同一個綠，
只留要問的那格改成白），`240`–`254` 是循環動畫。
**判準因此是「差異落在哪」，不是「差異有幾個」。**

第二個是亂數。原版 BASIC 的 `RANDOMIZE TIMER` 與 remake 的 seed 一直沒有
對應關係，`rich2` 的同路徑對拍因此卡著。在這裡掛上 `RND` 的攔截直接量：
`int 21h AH=2Ch` 回 0 時**初始狀態是 `000000`**，所以 remake 的 `seed = 0`
就是原版固定種子版的起點——從冷啟動到棋盤的 216 次抽取，兩邊的狀態、
浮點值與 `INT(RND×6)` 逐次相同。

`Caller()` 順便答出「誰在抽」：新局初始化 150 次（迴圈 50 圈、每圈三抽）、
開場動畫 62 次、防拷 4 次。**序列本身是決定性的，兩邊會岔開的永遠是
「誰在什麼時候抽了幾次」。**

第三類是**資料表**。`Array` 解 BASIC 的陣列描述子（前兩個 word 是
`(位移, 段)`，索引是**列主序**），所以 rich2 的 DIM 對照表上那些陣列都讀得到。
於是「解碼器對不對」有了最硬的判準——同一份資料兩條路徑：

| | remake 走 | 原版走 | 結果 |
|---|---|---|---|
| 角色表 | `SAVE_7.DSK` → 容器 → 解壓區段 0 | 執行期的 `11A2h` | 360 格 **0 格不同** |
| 土地數值表 | 同上 → `ParseLandTables` | 執行期的 `1174h` | 144 格 **0 格不同** |
| 棋盤陣列 | 同上 → `ParseBoard` | 執行期的 `122Ch` | 5,660 格 **0 格不同** |
| 座標 → 格號 | `Board.SquareAt` 掃棋盤資料 | 執行期算出來的 `11FEh` | 108 個在用的格 **0 個對不上** |
| 買地扣款 | 地價表第 0 張 | 走一步買下來，實際扣的錢 | 付 2200，**在表裡** |
| **移動（終點）** | `Board.Exits` 走 N 步的可達集合 | 走一步，讀起點、骰子點數、終點 | 五步 **全部命中** |
| **移動（方向）** | `Board.Walk` 吃原版抽到的方向序列 | 攔 `11A32`／`11A87` 收方向序列 | 五步 **同一個終點**，而且序列**每一步都剛好用完**（連重抽幾次都對上）|
| **移動（逐格）** | 棋盤的鄰接表 | `MoveTrace.Trail` ——原版踩過的每一格 | 五步**每一段都是一條邊** |

| 租金 | `RentBase(街, 等級表)` | 走到別人的地上，實際被收的錢 | 三筆 **逐筆相同** |

行為 parity 繞開了「原版走一步消耗 28–74 次亂數而 remake 只抽一次」那個
差距——直接讀點數本身（`ds:1B0h`）。

**驗過什麼、沒驗什麼的完整清單**在 rich2 的
[`docs/playtest/054`](https://github.com/wicanr2/rich2/blob/master/docs/playtest/054-dosgolem-parity-matrix.md)。
沒驗的同樣列在那裡——沒驗 ≠ 錯，但也不能當成驗過。

最後一列比前兩列更進一步：資料表對得上只證明**解碼**對，
那一列證明**那個數字真的是遊戲拿來扣錢的那一個**。整趟 15 秒，
從冷啟動走到棋盤、點「前進」、回答買地對話框，全自動。

解壓錯一個位元組不會報錯，只會讓多數數值碰巧是對的。以前只能拿
「開局現金 25000 對得上畫面」驗兩格。

MVP-B 的驗收可以重跑（素材由玩家自備）：

```sh
# 在 rich2 那邊產 oracle：DOSBox 的 Ctrl+F5 索引截圖
tools/pyx.sh tools/dosbox_pw_indexed.py 2

# 在這邊比對
DOSGOLEM_ORIG=~/cht/rich2/workplace tools/parity.sh <oracle.png>
```

`RUN_full.EXE` 需要的不是純 8086——主程式區有 3,345 個 80186 的 `PUSH imm`。
`machine.New()` 因此用 `Model80186`；`cpu.New()` 維持 8086，語料驗收走那一條
（細節與那個「不會報錯」的陷阱見 [`docs/spec/002`](docs/spec/002-cpu-8086.md) §1.1）。

CPU 用 [SingleStepTests/8088](https://github.com/SingleStepTests/8088) v2 驗收：
**323 個 opcode 檔、每檔 10,000 筆、共 323 萬筆**，
含每一個匯流排週期的暫存器與記憶體前後狀態。目前唯一沒解的是
**`IDIV` 溢位時推上堆疊的旗標**（[`docs/spec/002`](docs/spec/002-cpu-8086.md) §3.4）。
它以「上限計數」盯著：每一輪照跑照數，超過上限就紅，所以擋得住退步。

### 四條與 Intel 手冊衝突的行為

SingleStepTests 是硬體產生的，與手冊衝突時以它為準。這一路反推出四條：

1. `AAA`／`AAS` 的 `AL + 6` **進位不會跑進 `AH`**——手冊寫的 `AX += 106h`
   在 `AL ≥ FAh` 時會讓 `AH` 多加一次。
2. `DAA`／`DAS` 第二段的條件**不是** `old_AL > 99h OR old_CF`：
   實機在 `old_AL` 落在 9Ah–9Fh、而且進來時 `AF` 已經是 1 的那六個值上不做調整。
3. `D0`–`D3` 的 `/6` 在 8086 **不是 `SHL` 的別名**，是未公開的 `SETMO`
   （把目的地整個設成 1）；186 以上才變成別名。
4. 移位／旋轉的 `OF` **每一圈都重算**，不是只有「移 1 位」時才有定義。
   `D2.3`（`RCR` by `CL`）的 `flags-mask` 是 `FFFF`——一個位元都沒遮。

前兩條都用整份語料（各 10,000 筆）驗過，各 0 筆不合。
照手冊寫這幾處，程式跑得動，但會安靜地跑錯——這就是拿硬體語料當判準、
不照抄手冊的理由。

## 怎麼跑

建置與測試一律走 docker，不裝到系統環境：

```sh
tools/go.sh build ./...
tools/go.sh test ./...

tools/fetch_cputests.sh                  # 抓 SingleStepTests/8088 v2（761 MB）
tools/go.sh test ./internal/cpu -run TestSingleStep
```

測試語料**不進版控**，也不隨本儲存庫散布——它有自己的授權。
沒抓語料時 CPU 測試會 skip，不會假裝通過。

## 不含原版素材

本儲存庫**沒有** `RUN.EXE`、`.PIX`、`.PAK` 或任何《大富翁2》的檔案。
要跑原版得自己有一份合法的。缺檔的測試一律 skip，
**不用自製代用品**——安靜的替代品會讓「還沒做完」看起來像做完了。

## 授權

採 **RRSAL-1.0**（復古重製 source-available 授權條款 1.0），
全文見 [`LICENSE`](LICENSE)。非商業使用、修改與再散布免費且不必事先同意；
商業使用要另洽 `wicanr2@gmail.com`。
