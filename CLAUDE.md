# dosgolem — 給 Claude 的專案規則

## 這是什麼

**無頭、決定性、可以當 Go 套件 import 的 DOS 執行器**，為程式化觀測而寫。
第一個案例是《大富翁2》`RUN_full.EXE` 與 [`rich2`](https://github.com/wicanr2/rich2)
的對拍（背景與評估：`rich2/docs/spec/082-parity-oracle-emulator.md`），
但**目標不是只跑那一支**——分層與判準見 `docs/spec/006`。

## 動手前

1. 讀 `docs/spec/001-scope-and-mvp.md`（範圍與 MVP）與 `docs/spec/000-index.md`
   （規格索引）。**引用規格要連號碼帶檔名**（`009-memory-allocator`，不是 `009`）
   ——十條分支各自從 007 開始編號，同一個號碼底下有最多八份不同主題的文件。
2. **SDD：spec 齊了才實作。只有標 `READY` 的規格可以動手。**
   反組譯／量測 → 規格 → 才寫程式。
3. 通用規則（含硬規則、docker 邊界）在 `rich2/CLAUDE.md`，這裡不重抄。

## `[HARD]` 硬規則

- **不得散布原版素材。** 本儲存庫不含 `RUN.EXE`、`.PIX`、`.PAK` 或任何原版檔案。
  需要原版的測試**缺檔就 skip**，不用自製代用品——安靜的替代品會讓
  「還沒做完」看起來像做完了。
- **建置與測試一律走 docker**（`tools/go.sh`），不裝到系統環境。
  只清理自己建立的 container；禁止任何 `docker image/system/volume/builder prune`
  或 `rmi`。
- **git 身分一律 `wicanr2@gmail.com`。** 進 repo 先看 `git config user.email`，
  再跑一次 `git log --format=%ae | sort -u` 看歷史。
- **測試語料不進版控**（761 MB）。用 `tools/fetch_cputests.sh` 抓到 `testdata/`。
- **CPU 的驗收判準是「全部通過」，不是「大部分通過」**（`docs/spec/002` §5）。
  CPU 的錯不會報錯，只會讓上層在幾百萬個指令之後畫錯一個像素。
- **推論標籤要誠實**：confirmed／強證據／假說／未知。
- **把步數換算成時間時，時間基準有兩個，別拿錯。**
  `machine.StepsPerSecond()` 是常數，它成立的前提是「IRQ0 間隔跟著 PIT
  分頻走」；`Machine.StepsPerSecondNow()` 是 `IRQ0Every × PITHz`，
  呼叫端用 `-tick` 釘死間隔（`IRQ0Pinned`）時前提不成立，兩者可以差四倍。
  要秒數（倒 VGM、寫 WAV、算某一段花了多久）一律用後者。
  **拿錯的症狀是東西照樣產得出來，只是整條時間軸被縮放過**，
  沒有任何一個環節會報錯。理由與已知的不一致見
  `docs/spec/190-opl-vgm-dump` §3（`SpeakerWAV` 刻意還沒跟著改）。
- **寫死在程式裡的常數也算「機制」。** 沒有名字的行為最容易在改寫時掉，
  因為沒有旗標、沒有測試、沒有規格會替它說話。改寫任何送輸入的路徑之前，
  先問「舊版在這裡多做了什麼」——`-click-move-lead`（`docs/spec/004`
  §4.20.1）就是這樣掉的：舊 `probe` 裡一個 `const moveLead = 200_000`，
  改寫命令列時整個不見，40 份收據因此重跑出另一個畫面，而那個畫面
  **完全正常**，被當成上層專案的缺口追了一整輪。
- **為了重跑既有收據而存在的機制，不要當成冗餘拿掉。** 收據是證據不是
  程式碼：它記的是「當時的輸入長什麼樣」，那組數字與當時跑出來的畫面
  綁在一起。換掉判準會得到**另一個畫面**，於是那份收據不再是原本那一份，
  而**畫面照樣畫得出來，沒有測試會自己開口**。要換的唯一辦法是全部重錄，
  而重錄之前得先有一個跑得動舊收據的執行器。目前有兩處，都附了理由與
  釘住優先序的測試，改之前先讀：
  - `cmd/probe` 的 `-clicks` 第五欄（逐次的按住指令數）。看起來與
    `docs/spec/004` §4.5「用輪詢次數不要用指令數」矛盾，其實分工不同
    ——§4.5 是給新腳本的建議，第五欄是給舊收據的。分工與優先序寫在
    §4.5.1，`TestHoldForPrefersTheMoreSpecific` 釘住。
  - `machine.IRQ0Pinned`（呼叫端明講的計時器間隔壓過 PIT 的重算）。
    看起來與 `docs/spec/016` §3.1 的分頻換算重複，其實是讓兩者並存
    ——不釘住的話，程式一寫 PIT 就把呼叫端設的值換掉，而旗標看起來
    完全對得上。理由寫在 §3.1 與該欄位的註解。

## 從語料反推規則時

SingleStepTests 是**硬體產生的**，它與手冊衝突時**以它為準**——
已經踩到兩次（`docs/spec/002` §3.3 的 DAA／DAS 條件、AAA／AAS 的進位）。

反推出來的規則要：

1. 寫進 `docs/spec/`，**講清楚它是從語料反推的、手冊怎麼寫是錯的**；
2. 在程式碼註解裡標出是哪一個檔、幾筆資料支持；
3. 用整份語料驗過（不是幾個樣本）。

## 分層

```
internal/cpu/      CPU 核心。不認識 DOS、不認識畫面、不認識檔案
internal/dosfile/  DOS 檔案語意（handle 表、錯誤碼、seek／read 的號誌）。
                   不認識暫存器寬度、不認識記憶體——16 與 32 位元兩條路共用
internal/dos/      DOS 與 BIOS 服務
internal/machine/  記憶體、載入器、PIT、VGA、滑鼠
oracle/            對外的 Go API：Load／RunUntil／Click／Save／Search／OnCall
runtime/basic/     編譯後 MS BASIC 程式的共用支援（LCG 的 RND、陣列描述子）
apps/rich2/        大富翁2 專屬：位址、狀態、流程、攔截點；工具在 apps/rich2/cmd/
cmd/probe/         通用探針（吃 exe 路徑，不認識任何程式）
docs/spec/         規格，標 DRAFT／READY
tools/             docker 包裝與語料抓取
testdata/          測試語料（gitignore）
```

分層的理由有兩個。

**下層要能自己證明自己對**：CPU 要能在沒有 DOS、沒有原版素材的情況下
獨立驗收到底，那是這條路唯一一段可以自己證明自己對的部分，
不要讓它依賴上層。

**上層要能換掉**：接第二個程式時只寫 `apps/<程式>/`，前面三層照用。
判準是一句話——**「換一支 binary 之後，這段程式碼還成立嗎？」**
還成立就往下放，不成立就留在 `apps/`。
⚠ 這裡最容易錯的是**位址**：演算法（LCG 的公式）通用，
但 `RND` 的進入點是 runtime 連結進去之後的位址，**per-binary**，
所以收在 `basic.Config` 由 `apps/` 給。完整判準與案例見 `docs/spec/006`。
