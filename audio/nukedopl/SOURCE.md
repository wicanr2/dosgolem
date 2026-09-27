# audio/nukedopl 來源說明

本目錄是 Nuked OPL3 的 Go 移植，授權 LGPL-2.1-or-later（全文見 `COPYING.LGPL`）。
依據規格 `240-opl-audio-output-draft` §3.4、§3.7。

## 原著作

- 著作權人：Nuke.YKT（2013–2020，Nuked OPL3）、Tony Gies（2026，Nuked-OPL3-fast 修改）
- 上游：Nuked-OPL3 1.8，commit `cfedb09`
- 分支：Nuked-OPL3-fast 1.8-fast.1，commit `fb9afa9`（https://github.com/tgies/Nuked-OPL3-fast）
- 取得處：DOSBox-X 原始樹 `src/hardware/`（本機副本
  `DOSBox-X-MCP-Debugger/dosbox-src`，該副本 HEAD `5fcf624b787e1017273b313de6f9a70f12422102`）

| 原始檔 | SHA-256 |
|---|---|
| `nukedopl.cpp` | `ddc4255e433042afbc97ce0bbd376415db42faa2415c9741c582d1e9b4cdabd8` |
| `nukedopl.h` | `50ed8560b776b90acf663f38e175f84820a61163793ee7774bd9ca6cea947d1c` |
| `nukedopl_wf_rom.h` | `2d92fb305bc8ecf9d95c3bc26e340681f4433062d5db7cbaf57471a65d09bb99` |

`COPYING.LGPL` 取自 Debian bookworm `/usr/share/common-licenses/LGPL-2.1`
（SHA-256 `dc626520dcd53a22f727af3ee42c770e56c97a64fe3adb063799d8ab032fe551`）。

## 移植內容（2026-09-27）

| Go 檔 | 對應 C |
|---|---|
| `opl3.go` | `nukedopl.cpp` 的全部函式與 `nukedopl.h` 的結構 |
| `tables.go` | `nukedopl.cpp` 的 `exprom`、`mt`、`kslrom`、`kslshift`、`eg_incstep`、`ad_slot`、`ch_slot` |
| `wf_rom.go` | `nukedopl_wf_rom.h` 的 `logsin_wf`（逐值轉寫） |

- 固定 `OPL_ENABLE_STEREOEXT=0`、`OPL_QUIRK_CHANNELSAMPLEDELAY=1`；立體聲擴充
  （`0xD0` 暫存器、`panpot_lut`）未移植。
- 對外介面依 DOSBox-X `adlib.cpp`：`New(rate)`、`Reset(rate)`、`WriteRegBuffered`、
  `GenerateStream`（交錯立體聲）；另匯出 `WriteReg`（立即寫入）。
  `OPL3_Generate`、`OPL3_GenerateResampled`、`OPL3_Generate4ChStream` 沒有呼叫端，未匯出，
  其運算分別併入 `generate4Ch`、`generate4ChResampled`、`GenerateStream`。
- 與 C 的結構差異（輸出不變）：
  - C 以 `(uint8_t*)&chip->zeromod` 當「無震音」來源；Go 改用恆為 0 的 `zerotrem` 欄位。
  - 顫音相位增量（`OPL3_PhaseGenerate` 與 `OPL3_ProcessSlot` 快速路徑內相同的兩段）
    抽成 `vibratoInc`；A0/B0 共用的 slot 刷新抽成 `channelUpdateFreq`；
    兩個混音迴圈抽成 `mixSide`。
  - `__builtin_ctz` 改用 `math/bits.TrailingZeros32`。

## 一致性測試資料（`testdata/`）

`.sched` 是排程：`rate <取樣率>`、`end <總框數>`、`<樣本序號> <位址 hex> <值 hex>`。
參考程式在到達樣本序號前 `OPL3_GenerateStream`，再 `OPL3_WriteRegBuffered`；
`.s16.gz` 是其輸出（交錯立體聲 int16 小端，gzip）。Go 測試以同一排程逐樣本比對。

| 排程 | 內容 | 48000 Hz 框數／寫入 | 49716 Hz 框數／寫入 |
|---|---|---|---|
| `tone` | OPL2 模式單音 key-on/off、同一樣本先 off 再 on、回授、WSE 波形、震音／顫音、KSL、NTS、第二組 | 96000／40 | 99432／40 |
| `perc` | `0xBD` 節奏模式五種打擊樂、改 ch7/8 頻率、關閉再開節奏模式 | 96000／98 | 99432／98 |
| `random` | xorshift32 seed `0x2403b0c1`；前段 newm=0、1.5 秒起 newm=1 與 `0x104`（含全 4-op、四種 4-op 演算法、方波同相截幅段）、2.6 秒一次 1100 筆同樣本爆量（溢出 1024 筆寫入緩衝）、3.8 秒關 newm；涵蓋全部暫存器族與無效位址 | 240000／2476 | 248580／2479 |

參考程式以 `gcc:14-bookworm`（g++ 14.3.0，`-O2 -DOPL_ENABLE_STEREOEXT=0
-DOPL_QUIRK_CHANNELSAMPLEDELAY=1`）編譯上表三個原始檔（唯讀掛載，未修改）產生。
參考程式與排程產生器的原始碼不在本儲存庫（位於不進版控的工作區 `nukedopl-ref/`）。

未被測試覆蓋的兩處：`channelSetupAlg` 的 `alg & 0x08` 提早返回（所有呼叫端都不會以
`alg == 0x08` 呼叫，C 版亦同）、包絡計時器 36 位元回捲（需約 2^36 個內部樣本）。
