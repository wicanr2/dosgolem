# dosoracle — 給 Claude 的專案規則

## 這是什麼

**只跑得動《大富翁2》`RUN_full.EXE` 的無頭 DOS 執行器**，
為 [`rich2`](https://github.com/wicanr2/rich2) 的對拍而寫。
背景與評估：`rich2/docs/spec/082-parity-oracle-emulator.md`。

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
internal/dos/      DOS 與 BIOS 服務（未建立）
internal/machine/  記憶體、載入器、PIT、VGA（未建立）
oracle/            對外的 Go API（未建立）
docs/spec/         規格，標 DRAFT／READY
tools/             docker 包裝與語料抓取
testdata/          測試語料（gitignore）
```

分層的理由：**CPU 要能在沒有 DOS、沒有原版素材的情況下獨立驗收到底**。
那是這條路唯一一段可以自己證明自己對的部分，不要讓它依賴上層。
