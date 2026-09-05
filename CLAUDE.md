# dosgolem — 給 Claude 的專案規則

## 這是什麼

**無頭、決定性、可以當 Go 套件 import 的 DOS 執行器**，為程式化觀測而寫。
第一個案例是《大富翁2》`RUN_full.EXE` 與 [`rich2`](https://github.com/wicanr2/rich2)
的對拍（背景與評估：`rich2/docs/spec/082-parity-oracle-emulator.md`），
但**目標不是只跑那一支**——分層與判準見 `docs/spec/006`。

## 動手前

1. 讀 `docs/spec/001-scope-and-mvp.md`（範圍與 MVP）。
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
