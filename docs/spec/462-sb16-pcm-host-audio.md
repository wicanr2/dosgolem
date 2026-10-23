# 462：SB16 8 位元數位音效輸出到主機音訊（`SBPCM`）

狀態：**READY**（第 2 版，2026-10-23；兩位獨立審查皆無阻擋，應改依下表處置，審查紀錄在陸空戰將 repo `workplace/sb462/review462-*.md`），未實作。
日期：2026-10-23
編號：462。dosgolem-sc `strike-commander` 分支的 `docs/spec/` 目前最大號 461（`461-pit-divisor-after-load`）。
前置：[`006-layering`](006-layering.md)、[`240-opl-audio-output-draft`](240-opl-audio-output-draft.md)（`AudioObserver`、`audio/mixer`、`Ring`）、[`247-sb16-voice-volume-mixer`](247-sb16-voice-volume-mixer.md)、
[`444-legacy-sb16-dma8-irq7`](444-legacy-sb16-dma8-irq7.md) §2.4、§2.5、§2.8、§2.10、[`445-legacy-sb16-dma8-pause-resume`](445-legacy-sb16-dma8-pause-resume.md)、
[`446-legacy-sb16-idle-control-and-busy-cycle`](446-legacy-sb16-idle-control-and-busy-cycle.md)、[`452-session-sbdma-observer`](452-session-sbdma-observer.md)、[`190-opl-vgm-dump`](190-opl-vgm-dump.md) §3（時間基準）
上游需求：strike_commander 規格 016（`workplace/fe016/spec016-draft.md`）§8.2、§10 R3

dosgolem-sc 的程式位置以 `strike-commander` 分支 `395b679`（`395b67995edcff713f6849337726833cb2139f24`）為準。DOSBox-X 對照原始碼為 `~/cht/DOSBox-X-MCP-Debugger/dosbox-src/`。推論等級：已證實、強推論、假說、未知。

## 第一輪審查處置

審查原文：`workplace/sb462/review462-A.md`（甲）、`review462-B.md`（乙）。兩份都無阻擋，應改全部採納。

| 編號 | 處置 | 位置 |
|---|---|---|
| 甲 1 | 續播第一個樣本的來源序號改為 `T − 1` 或 `T`（`T = resume.Transferred`） | §4.1 第 4 項 |
| 甲 2 | 新增 64 KiB 環繞單測（`Addr = FFF0h`、`Length = 32`）與「位址進位到頁」負對照 N10 | §4.1 第 2 項、§4.4 |
| 甲 3 | Q1 定案不門控；§4.1 第 1 項加 `D1h` 之前與 `D3h` 之後各一次傳輸；N5 限 machine 層；G4 只留「有聲」 | §3.4、§4.1、§4.2、§4.4 |
| 甲 4 | N9 改成 reflect 斷言 | §4.4 |
| 甲 5 | N2 的必敗欄改為單元測試與 G2；N4 寫明要暫加 `RateNum` 欄位 | §4.4 |
| 甲 6 | N3 不再寫成可能等價：§4.1 第 2 項必敗。probe 另加線性讀取比對計數，在遊戲上量 JEMM 是否恆等對映 | §3.7、§4.2 G1、§4.4 |
| 甲 7 | 新增 G6：session 加 `Mixer`、每 192,500 步 `Render`，AdLib 關閉時逐位元等於 G2 ÷ 32768，記削波樣本數；寫明 `resume` 只有單元測試覆蓋 | §4.2 |
| 甲 8 | G2 的期望長度寫成到 `sampleOf(停止時 m.Steps)`（不含） | §4.2 |
| 甲 9 | 新增計數 `orphanEvents`，§4.1 第 8 項與 §3.7 對齊；`SBPCMMutated` 加測試 | §3.4、§3.7、§4.1 |
| 甲 10 | 暫態欄位在 `sb8Apply`（`LoadState`）與 `Restore` 路徑顯式歸零 | §3.2 第 6 條 |
| 甲 11 | 016 的修訂列成行號清單給呼叫端，不是本規格 READY 條件 | §6 |
| 甲 建議 | 偵測傳輸的 IVT 位元組記入 G4 收據；§3.9 註明只在 `Clock = steps` 成立；登記 `000-index`；Q3 補一句；`num` 用有號比較 | §4.2、§3.9、§4.3、§5、§3.4 |
| 乙 1 | `Data` 經暫態欄位 `m.sb8pcmData` 從取樣點帶到發送點，`sb8Event` 簽名不變；`complete` 只比雜湊；`SBPCMMutated` 匯出；沒有 `start` 雜湊時不比不計 | §3.2 |
| 乙 2 | `audio/sbpcm.Source` 改為 `Apply(e)` 到達即記區段、`Sample(j)` 純函式；寫明「同回合的事件先於 `Render`」的前提。混音器不新增 `evSBPCM` | §3.4、§3.5 |
| 乙 3 | 寫明記憶體最壞情況；空區段立即丟 `Data`；`Prune` 修剪已結束的聲部 | §3.4 |
| 乙 4 | 內插用 `int64`；寫明 `num` 的寬度與商不超過 `N` 的理由 | §3.4 |
| 乙 5 | G2、G3 的 Python 輸入：`start` 列的 `Step` 即 `Start`，`resume` 由 `Due − Δ` 推回；偵測傳輸不比 VOCLIST，只比 raw 傾印的 SHA；probe 在 stderr 印第一個樣本序號與結束步數；路徑寫全 | §3.7、§4.2 |
| 乙 6 | Q1 改寫為「016 第 2 版沒有此規定，本規格定案」；列 016 要改的行；016 A11 改用 Source 單獨輸出；`ring.go` 引用改為 `Read` 的實際行號 | §5、§6、§3.6 |
| 乙 建議 | 混音器單測補 SBPCM 路徑與夾限；黃金 SHA 在 `395b679` 先取；probe 邊跑邊寫 WAV；`Source.Reset()`；實體位址單測的設定順序；§4.3 分機器側與混音器側；Q6 寫明未量 | §4.1、§3.7、§3.4、§4.3、§5 |
| 呼叫端裁決 | Q1 維持不門控，寫明偵測傳輸可能有咔嗒聲，人工驗收聽開場；Q5 不淡出，016 人工驗收加跳句爆音項 | §3.4、§5、§6 |

用語：

| 用語 | 意義 |
|---|---|
| 傳輸 | 444 的一次 DSP 單週期傳輸（`14h`、ADPCM、`80h` 靜音），從 `start` 到 `complete` 或 `cancel` |
| 事件步 | 事件的 `Step`，定義照 452 §2.4 的表 |
| SPS | 混音器的「每秒步數」，`mixer.New` 的參數。前端 016 取 11,550,000 |
| 輸出樣本序號 | 48 kHz 的絕對樣本序號；步數 `s` 對應 `sampleOf(s) = ⌊s × 48000 ÷ SPS⌋`（`audio/mixer/mixer.go:82-86`） |
| 來源 | 本規格新增的 `audio/sbpcm.Source`：把傳輸排成 48 kHz 樣本的合成器 |
| 收據 | strike_commander `workplace/` 下 133 份 probe `-sb-dma-log` 的 `sbdma*.tsv`，與 `workplace/rerec459/scratch/` 的 71 份狀態檔 |

---

## 1. 為什麼要做

`audio/` 只有 `mixer` 與 `nukedopl`。混音器合成 OPL、PC 喇叭與 PIT 通道 2（`audio/mixer/mixer.go:1-7、29-33`），SB16 的數位樣本沒有任何路徑送到主機（已證實：程式碼）。《Strike Commander》的語音與音效都走 SB16 8 位元 DMA（RE 007 §1），所以前端目前聽得到音樂，聽不到語音。

樣本不能在觀察器層讀：

- `StepView.ReadBlock` 讀線性位址，經頁表轉換（`session/observer.go:149-158`）。
- DMA 讀實體位址：`sb8SHA` 以 `peekPhysRaw` 讀 `Page<<16 | ((Addr+i) & FFFFh)`（`internal/machine/sb_dma8.go:893-903`）。
- JEMM 開分頁後兩者可能不同（452 §3.1 第 2 項已點出）。

所以取樣放在 `machine` 內，用與 `sb8SHA` 相同的讀法。混音屬硬體行為，換一支 binary 仍成立，依 006 的判準放通用層。

## 2. 證據：陸空戰將用到的 SB16 功能

### 2.1 盤點方法

| 來源 | 做法 | 工具與輸出（`/home/anr2/cht/strike_commander/workplace/sb462/`） |
|---|---|---|
| 133 份 `sbdma*.tsv` | 依 `Kind`、`Command`、`Format`、取樣率、時間常數、`DMAMode`、`SpeakerOn`、`CPUMode`、`Clock` 分組計數；`start` 與下一筆同 `Phys`、`Length` 的 `complete` 比 `SHA256` | `scripts/inv.sh`、`scripts/inv2.sh`；`out/inv.txt`、`out/inv2.txt` |
| 71 份狀態檔（冷開機 7 億步到 80 億步） | 在容器內的複本解 gob，印 `220h–22Fh` 的最後寫入值（`PortList`）、讀取次數（`PortInList`）與 `sbDMA8State` 的 DSP、混音器欄位 | `scripts/zz_sb462_ports_test.go`、`scripts/ports.sh`；`out/ports.txt` |
| 程式碼 | `sb_dsp.go`、`sb_dma8.go`、`machine.go:1178-1306` | 唯讀 |
| DOSBox-X | `src/hardware/sblaster.cpp`、`src/hardware/mixer.cpp` | 唯讀 |

收據涵蓋冷開機、MENU、OPTTEST 過場與營房、任務簡報（RE 022 的 C11、T10、H12、K1、K2）、訓練任務飛行。STRIKE 戰役飛行、`MKTERR.EXE`、`TOPTTST1.EXE` 沒有收據。

### 2.2 結果

| 項目 | 觀察 | 來源 | 等級 |
|---|---|---|---|
| 傳輸命令 | 全部 `start` 都是 `14h`（`pcm8`），共 878 列（多份收據重疊，非 878 次不同播放；不同 `SHA256` 108 個）；沒有 ADPCM、沒有 `80h` 靜音 | `out/inv.txt:4-24、110-111` | 已證實（收據範圍內） |
| 自動初始化 | 71 份狀態檔的 `Auto8Commands` 全為 0；444 沒有接 `StartAutoDMA`、`StartSB16AutoDMA`，用到 `1Ch` 或 `C6h` 會失敗即關閉（`sb_dma8.go:137-144`、`sb_dsp.go:146、230`） | `out/ports.txt`；程式碼 | 已證實（收據範圍內） |
| 16 位元 | 同上，`B0h` 接不上 `Start16DMA` 會失敗即關閉 | `sb_dsp.go:176` | 已證實（收據範圍內） |
| DMA 模式 | `DMAMode` 全為 `48h`（單週期、讀、遞增） | `out/inv.txt` | 已證實 |
| 取樣率 | 全部經時間常數 `40h` 設定，`TCKnown` 全為 1，沒有用 `41h`。`start` 列中 166（1,000,000÷90 ≈ 11,111 Hz）786 列，173（÷83 ≈ 12,048 Hz）84 列，194（÷62 ≈ 16,129 Hz）8 列 | `out/inv.txt:4-24` | 已證實 |
| 時鐘 | `Clock` 全為 `steps` | 同上 | 已證實 |
| CPU 模式 | `start`、`pause`、`cancel` 都在 V86；`start` 的 CS:IP 只有 `B5C5:0255`（849）、`B4E5:0255`（29），`pause` 只有 `B5C5:03F9` | `out/inv.txt:67-71` | 已證實 |
| DMA 頁 | `Page` 為 09h（781）、08h（23）、07h（8）、00h（66） | `out/inv.txt:72-76` | 已證實 |
| 長度 | 最長 64,755 bytes；跨 64 KiB 頁的句子拆成兩次 `start`，兩段間隔 378 到 408 步 | `out/inv2.txt:21`；RE 022 §1、規格 010 §3.4 | 已證實 |
| 偵測傳輸 | 66 列長 4 bytes、`Page` 00h、`Addr` 0000h（實體位址 0 到 3，即 IVT[0]），`SpeakerOn` 都是 0；其他 `start` 的 `SpeakerOn` 都是 1 | `out/inv2.txt:3-5` | 已證實 |
| 傳輸期間緩衝不變 | `start` 與對應 `complete` 的 `SHA256` 730 對全同、0 對不同 | `out/inv2.txt:1-2` | 已證實（收據範圍內） |
| 暫停與恢復 | `pause` 138 列；其中 134 列的下一筆事件是同一 CS:IP 新 `start` 造成的隱式取消，其餘 4 列在收據結尾。`resume` 0 列 | `out/inv2.txt:6-8` | 已證實 |
| 暫停時的進度 | `Transferred ÷ Length` 分布在 0 到 0.9 各段 | `out/inv2.txt:9-17` | 已證實 |
| 混音器 | 71 份狀態檔沒有任何 `224h`、`225h` 的寫入或讀取；`VoiceVolumeKnown` 全為假 | `out/ports.txt` | 已證實（收據範圍內） |
| 其他 DSP 埠 | 寫入只見 `226h`、`22Ch`；讀取 `226h`、`22Ah`、`22Ch`、`22Eh` | `out/ports.txt` | 已證實 |
| 用到的 DSP 命令 | 444 的 DSP 子集遇到未實作的命令會失敗即關閉（`machine.go:1301-1305`），收據全程沒有停點，所以用到的命令落在已實作的集合內（`14h`、`40h`、`41h`、`48h`、`D0h`、`D1h`、`D3h`、`D4h`、`D8h`、`E1h` 與 ADPCM、`80h`）。`22Ch` 的最後寫入值在 66 億步之後是 `D1h` | `sb_dsp.go:91-298`；`out/ports.txt` | 子集關係已證實；逐條命令只確認 `14h`、`40h`、`D0h`（事件）與 `D1h`（最後寫入值），其餘未知 |
| 樣本內容 | RE 022 的 51 句，DMA 的 PCM 雜湊等於 `VOCLIST.TRE` 條目樣本的雜湊；拆頁的 3 句兩段各對上條目的前後切片 | RE 022 §1；`workplace/rerec459/i022/timing-*.tsv` 的 `dma_sha_match` 欄 | 已證實 |
| 步數與秒 | 一次播完的 48 句，`(Due − Step) ÷ VOC 秒數` 都是 11,580,932 步／秒 | RE 022 開頭；`timing-*.tsv` 的 `steps_per_s` | 已證實 |

沒有收據的範圍：戰役飛行中的 `SPEECH.PAK`、過場 `MID*VOC.PAK`、`MKTERR.EXE`、資料片。這些若用 ADPCM 或自動初始化，444 會失敗即關閉，不會靜靜漏音。

### 2.3 DOSBox-X 對照

| 行為 | DOSBox-X | 位置 |
|---|---|---|
| 8 位元無號樣本 | `AddSamples_m8` 以 `uint8_t` 無號模板讀入，等於 `(b − 128) × 256` 的 16 位元值 | `src/hardware/mixer.cpp:613-615`；`sblaster.cpp:938` |
| 時間常數 | `256000000 ÷ (65536 − (tc << 8))`，等於 1,000,000 ÷ (256 − tc) | `sblaster.cpp:2129-2131` |
| 取樣時點 | 混音器每次要樣本時才從 DMA 讀，邊播邊讀 | `sblaster.cpp:883-941` |
| 喇叭開關 | `DSP_SetSpeaker` 對 `SBT_16` 直接返回，不關聲道；`C6h` 一帶註解「SB16 always has speaker enabled」 | `sblaster.cpp:762-767、2287` |
| 混音器重設值 | `dac`、`master` 都設 31 | `sblaster.cpp:1877-1887` |
| 音量換算 | SB16：`count = 31 − v`，`dB = 2·count`（`count > 20` 時再減 1），增益 `10^(−dB/20)`；31 為 1.0 | `sblaster.cpp:1844-1859` |
| `30h`–`33h` | 寫入取 `val >> 3` | `sblaster.cpp:2777-2807` |
| `D0h` | 停止送樣本，`mode = MODE_DMA_PAUSE` | `sblaster.cpp:2298-2308` |

## 3. 規則

### 3.1 範圍

| 在範圍 | 不在範圍（另立規格） |
|---|---|
| 444 的 `14h` 8 位元單聲道 PCM，經 445 的暫停、恢復與取消 | ADPCM 解碼（`74h`–`77h`、`16h`、`17h`）：事件照送，來源不出聲並計數 |
| `80h` 靜音：該區間輸出 0 | 自動初始化、16 位元、立體聲、錄音（444 §2.10 本來就不支援） |
| 時鐘為步數（`Clock = steps`）的傳輸 | `CycleClock`（`Clock = cycles`）：來源不出聲並計數 |
| 混音進 `audio/mixer`，48 kHz | 傳輸期間緩衝被改寫的逐段取樣（§3.2 第 5 條只偵測） |
| probe 的離線傾印（§3.7） | 混音器 `30h`、`31h`、`34h` 以後的索引（dosgolem 沒有建模，寫入會失敗即關閉） |
| | 喇叭濾波、類比輸出特性 |

### 3.2 machine：取樣與 `SBPCMEvent`

1. 新增型別與掛載方法，放 `internal/machine/sb_pcm.go`：

```go
// SBPCMEvent 是 docs/spec/462 的音訊事件。不進狀態檔、不進 SBDMALog、不影響執行。
type SBPCMEvent struct {
	Kind        string // start、pause、resume、cancel、complete
	Step        uint64 // 同 SBDMAEvent.Step（452 §2.4）
	Clock       string // steps 或 cycles（傳輸的時鐘單位）
	Command     uint8
	Format      string // sbFormat 的名稱
	Length      uint32 // DMA bytes（靜音為取樣數）
	Samples     uint64
	Start, Due  uint64 // 傳輸的開始與到期時刻（傳輸時鐘單位；resume 後為平移後的值）
	Transferred uint32 // pause、resume、cancel、complete 的已傳數；start 為 0
	SHA256      string // 只在 start；與同一步 SBDMAEvent 的 SHA256 相同
	Data        []byte // 只在 Command ≠ 80h 的 start；新配置，所有權交給回呼
}

// ObserveSBPCM 掛上音訊事件觀測器；nil 關閉（同 ObserveOPLWrites）。
func (m *Machine) ObserveSBPCM(fn func(SBPCMEvent))

// SBPCMMutated 回 §3.2 第 5 條的診斷計數。
func (m *Machine) SBPCMMutated() uint64
```

2. **產生點**：與 `sb8Log` 同一個呼叫點、同一步、緊接在 `sb8Log` 之後，只對五種 Kind：`start`（`sb8Begin`，`sb_dma8.go:265-266`）、`complete`（`sb8Poll`，404-406）、`cancel`（`sb8CancelBody`，443-445）、`pause`（`sb8Control`，480-482）、`resume`（`sb8Resume`，516-518）。`irq7`、`eoi`、`defaulted`、`control-idle` 不送。回呼為 nil 時這幾處只多一次 nil 判斷。
3. **取樣與 `Data` 的路徑**：`sb8SHA` 改成 `sb8Capture`，一次讀取同時得到 bytes 與雜湊，讀法不變（`peekPhysRaw`，`Page<<16 | ((Addr+i) & FFFFh)`，`i < Length`，位址在 64 KiB 內環繞，不進位到頁）。`sb8Event` 的簽名不變。
   - `start` 且回呼非 nil 時，`sb8Event` 把這份新配置的切片存進暫態欄位 `m.sb8pcmData`，雜湊存進 `m.sb8pcmStartSHA`。發送 `start` 的 `SBPCMEvent` 時把 `m.sb8pcmData` 放進 `Data`，隨即設 nil，不複製。
   - 回呼為 nil 時不存，行為與現況相同（配置一次、算一次雜湊）。
   - `SBDMAEvent.SHA256` 的值不變。
4. **`Start`、`Due`**：`start` 事件取 `s.start`；`resume` 事件取平移後的 `s.start`、`s.due`（`sb8Resume` 已加 d）；其他取當下值。來源不必自己重算平移。
5. **改寫偵測**：`complete` 時 `sb8Event` 已重算一次 `SHA256`（`sb_dma8.go:878-880`）。回呼非 nil、`m.sb8pcmStartSHA` 非空且與之不同時，`SBPCMMutated` 加一。`m.sb8pcmStartSHA` 為空（例如 `BootFromState` 後沒有看到 `start`）時不比、不計。比完清空。來源照 `start` 的 `Data` 播，不重取。收據顯示本遊戲 730 對全同（§2.2）。
6. **暫態欄位**：`m.sb8pcmData`、`m.sb8pcmStartSHA`、`SBPCMMutated` 的計數不進狀態檔、不進 `Digest`。`sb8Apply`（`LoadState` 的 sb8 路徑，`sb_dma8.go:1018`）與 `Restore`（`snapshot.go:233`）顯式歸零。
7. 回呼不得改變機器狀態。它拿不到 `Machine`，只拿到值與自己的切片。
8. 不新增狀態檔欄位，不改 `sbDMA8StateV`（1）與 `stateVersionSB8`（4），不改 `SBDMAEvent` 與 `SBDMAEventFields`，不改 `SBDMALog`。
9. 為什麼不把 bytes 放進 `SBDMAEvent`：`SBDMALog` 上限 4096 筆，每筆最多 64 KiB，最壞約 256 MiB；452 §2.2 的「以值傳遞」遇到切片會變成共用底層陣列。另開事件型別讓 444、452 的收據與測試都不動。

### 3.3 session：`SBPCMObserver`

```go
// SBPCMObserver 是 StepObserver 的選配擴充（docs/spec/462）。
type SBPCMObserver interface {
	SBPCM(e machine.SBPCMEvent)
}
```

- 放 `session/observer.go`，安裝與移除比照 452 §2.3：`runObserved` 以型別斷言判斷一次，實作時 `o.machine.ObserveSBPCM(func(e) { guard("SBPCM", ...) })`，`defer` 設回 nil；沒實作就不碰。
- 與 `SBDMAObserver` 互不依賴。同一個觀察器可以兩個都實作：同一步內先收 `SBDMA`，再收 `SBPCM`（§3.2 第 2 條的次序）。
- `Config.SBDMA8` 為假時沒有事件，`New` 不回錯誤（與 452 §4 第 1 項一致）。
- 時序、回呼內不能用 `StepView` 等規則照 452 §2.4。

### 3.4 來源：把傳輸排成樣本（`audio/sbpcm`）

**API**：

```go
type Source struct{ /* 聲部清單與計數 */ }

func New(sps uint64) *Source
func (s *Source) Apply(e machine.SBPCMEvent) // 事件到達即記下聲部與區段
func (s *Source) Sample(j uint64) int32      // 輸出樣本 j 的值；純函式，不改狀態
func (s *Source) Prune(j uint64)             // 丟掉所有區段都在 j 之前結束的聲部
func (s *Source) Reset()                     // 清空聲部（不清計數），換場景或重開機用
func (s *Source) Stats() Stats               // 聲部數、adpcmSkipped、cycleClockSkipped、completeMismatch、orphanEvents
```

**前提**：一個回合內的事件都在該回合的 `Render(before, after)` 之前 `Apply` 完。session 的回合是 `Advance` 之後才 `Render`（`cmd/buckrogers-play/main.go:137-150` 的次序），成立。因此已輸出的樣本 `j < sampleOf(after)` 不會被後到的事件改變：後到事件的步數不小於 `after`，`sampleOf` 不減。

**時間放置**：以混音器的步數換算為準，放置用 `Start`、`Due`，不用 `RateNum ÷ RateDen`。

- `sb8Delta` 已把到期時刻上整到整數步（`sb_dma8.go:270-318`），`Due` 是機器認定的結束點。規格 010 判「播完」用 `complete.Step == Due`（010 §3.4），音訊用同一個訖點，字幕與聲音的訖點相同。
- 傳輸的 `N = Length` 個來源樣本均勻填滿步數區間 `[Start, Due)`。來源樣本 `k` 的時刻是 `Start + k·Δ ÷ N`，`Δ = Due − Start`。這與機器的進度 `k = ⌊(now − Start)·N ÷ Δ⌋`（`sb8Progress`，`sb_dma8.go:366-383`）一致。

**聲部**：每個 `start`（`Command = 14h`、`Clock = steps`）建一個聲部，記 `Data`、`N`、`Δ`、目前的 `Start`、播放區段清單。第一個區段 `[a, b) = [Start, Due)`。

| 事件 | `Apply` 的處理 |
|---|---|
| `start`（`14h`） | 新聲部。前一個聲部若仍有未截止的區段，以本事件步截止（實際上 444 會先送 `cancel`） |
| `start`（`80h`） | 不建聲部（輸出 0） |
| `start`（ADPCM） | 不建聲部，`adpcmSkipped` 加一 |
| 任一事件 `Clock = cycles` | 不處理，`cycleClockSkipped` 加一 |
| `pause` | 目前區段的訖 `b = e.Step` |
| `resume` | 新區段 `[e.Step, e.Due)`，聲部的 `Start = e.Start`（已平移） |
| `cancel` | 若未暫停，目前區段的訖 `b = e.Step`；已暫停則不變。聲部結束 |
| `complete` | `e.Step ≠ b` 時 `completeMismatch` 加一。聲部結束 |
| 找不到對應聲部的 `pause`、`resume`、`cancel`、`complete` | 忽略，`orphanEvents` 加一（`BootFromState` 後載入當下在播的傳輸，同 010 §3.4 末條） |

**輸出樣本 j 的值**（整數，16 位元刻度）：聲部在 j 有聲，當且僅當某個區段滿足 `sampleOf(a) ≤ j < sampleOf(b)`。這與 `Mixer.Render` 套用事件的條件 `sampleOf(e.step) ≤ next` 相同（`mixer.go:152`）。有聲時：

```
d    = j·SPS − Start·48000                    // 有號比較；d < 0 時取 0
num  = d · N                                   // 128 位元乘積（bits.Mul64）
den  = 48000 · Δ
k    = min(⌊num ÷ den⌋, N − 1)                 // bits.Div64
f    = ⌊(num mod den) · 65536 ÷ den⌋            // 0..65535
k1   = min(k + 1, N − 1)
x0   = int64(Data[k]) − 128,  x1 = int64(Data[k1]) − 128
v    = (x0·(65536 − f) + x1·f) · 256 >> 16     // int64，算術右移（向負無限大）
```

- 寬度：區段內 `j < sampleOf(b) ≤ b·48000 ÷ SPS`，所以 `j·SPS ≤ b·48000`；`b − Start ≤ Δ`（`resume` 後 `Start` 已平移，`Due − Start = Δ`）。因此 `d ≤ 48000·Δ = den`，`num ≤ den·N`，商不超過 `N`，`bits.Div64` 的高位小於 `den` 的前提成立。`(num mod den)·65536` 也用 128 位元算。
- `v` 在 −32768 到 32512 之間，等於 DOSBox-X `(b − 128) << 8` 的刻度再做線性內插。全程整數，跨平台逐位元組相同。浮點只在交給混音器時出現（§3.5）。
- 拆頁的第二段是另一個聲部；兩段之間 378 到 408 步的空檔對應 0 到 2 個輸出樣本，輸出 0。第一段最後一個樣本不跨聲部內插。
- 同一時刻最多一個聲部有聲（444 不允許重疊傳輸）。`Sample(j)` 回該聲部的 `v`，沒有就回 0。

**記憶體**：

- 區段為空（`sampleOf(a) == sampleOf(b)`，例如同一步 `start` 又 `cancel`）的聲部在聲部結束時立即丟 `Data`。
- `Render` 每回合結尾呼叫 `Prune(end)`，丟掉已結束且所有區段都在 `end` 之前的聲部。
- 正常情形最多兩個聲部持有 `Data`（播放中一個、同回合剛結束一個），每個不超過 64 KiB。
- 病態最壞：一個回合（192,500 步）內反覆 `start` 與 `cancel`。區段非空至少要 `SPS ÷ 48000 ≈ 240` 步，所以一回合最多約 800 個非空聲部，約 51 MiB，`Prune` 後釋放。

**喇叭開關**：`SpeakerOn` 不影響輸出，照 DOSBox-X 的 SB16 行為（§2.3）。受影響的只有 4 bytes 的偵測傳輸：約 0.36 ms，內容是實體位址 0 到 3 的 IVT[0] 位元組，與中心值 80h 差得多時會聽到一聲咔嗒。真機 SB16 會不會同樣出聲，以 DOSBox-X 為準，未另查 Creative 文件（強推論）。

**音量**：本版固定增益 1.0（DOSBox-X 混音器重設值 31，`calc_vol(31) = 1.0`）。本遊戲從不寫混音器（§2.2），不影響陸空戰將的輸出。

**與轉回合無關**：`Sample(j)` 只由已 `Apply` 的事件與 `j` 決定，與 `Render` 的切分無關。同一段步數一次 `Render` 或切成多次，輸出逐樣本相同（§4.1 第 6 項）。

### 3.5 混音器

- `mixer.Mixer` 內嵌一個 `*sbpcm.Source`，實作 `SBPCM(e machine.SBPCMEvent)`：到達即呼叫 `Source.Apply(e)`。不新增 `evSBPCM`，不進 `events` 佇列。
- `Render` 迴圈每個輸出樣本取 `v = Source.Sample(m.next)`。`v ≠ 0` 時 `L = clamp(oplL/32768 + spk + float64(v)/32768)`，`R` 同理（單聲道，左右相同）；`v == 0` 時照現有算式不做加法，讓沒有 SB 樣本時的輸出與現況逐位元組相同。迴圈結束呼叫 `Source.Prune(end)`。
- 與 OPL 相加後才夾 ±1，與 DOSBox-X 兩個 1.0 聲道直接相加相同，大聲時可能削波（強推論：DOSBox-X 混音器最後同樣夾到 16 位元）。
- `SPS` 沿用 `mixer.New` 的參數，不另設。前端 016 用 11,550,000；機器在 120 Hz 計時器下的步數是每秒 11,580,932（§2.2）。所以聲音比真機低約 0.27%（約 4.6 cent），與 OPL、畫面同一個時基、同方向，不互相漂移。不另補償。

### 3.6 即時節奏

- 樣本只隨模擬步數產生，與牆鐘無關。即時倍率小於 1 時，每牆鐘秒產生的樣本不足一秒份，`Ring` 斷流、淡出 2 ms、回到預填、再淡入（`audio/mixer/ring.go:61-103` 的 `Read`）。音高不變，語音會斷續。這是前端 016 D12 的做法，本規格不改 `Ring`。
- 不做時間伸縮、不依牆鐘丟或補樣本。
- 前端暫停（說明頁）時沒有 `Render`，`Ring` 自然耗盡淡出。

### 3.7 probe 的離線傾印

供驗收用，放 `cmd/probe/sbpcm.go`。只用 `sbpcm.Source`，不經 `mixer`。

| 旗標 | 內容 |
|---|---|
| `-dump-sbpcm-wav <檔名>` | 只含來源的 48 kHz、16 位元、單聲道 WAV。樣本就是 §3.4 的整數 `v`，不經浮點、不混 OPL。範圍從第一個 `start` 的 `sampleOf(Start)` 到 `sampleOf(執行結束時 m.Steps)`（不含）。執行中邊跑邊寫，結束時補 RIFF 長度 |
| `-sbpcm-sps <n>` | 傾印用的 SPS，預設 11,550,000（與前端 016 相同）。期望值腳本要用同一個數 |
| `-dump-sbpcm-raw <目錄>` | 每次 `start` 的 `Data` 存成 `<Step>-<SHA256 前 16 碼>.u8` |
| `-sbpcm-linear-check` | 每次 `start` 另以線性位址（`Peek8`，經頁表）讀同樣的 `Length` bytes，與 `Data` 比較，不同的次數計入 `linearMismatch`。只供診斷，不影響輸出 |

結束時 stderr 印一列：第一個樣本的絕對序號、結束時 `m.Steps`、聲部數、`adpcmSkipped`、`cycleClockSkipped`、`completeMismatch`、`orphanEvents`、`SBPCMMutated`、`linearMismatch`（有開時）。

### 3.8 決定性

- 掛不掛 `SBPCMObserver`、掛不掛混音器與 `Ring`，同一狀態、同一輸入、同一停止步的 `Digest`、`MachineDigest` 與 `SaveState` 的 bytes 都相同。
- 取樣只讀 `Mem`／`Ext`（`peekPhysRaw`，`internal/machine/physical.go:35-46`），不經頁表、不碰 TLB 與快取。`-sbpcm-linear-check` 的 `Peek8` 同樣唯讀。
- `SBDMALog` 的內容與有無本觀察器無關（本規格不碰 `OnSBDMA`）。

### 3.9 與 452、規格 010 的關係

| 項目 | 452 `SBDMAObserver` | 本規格 `SBPCMObserver` |
|---|---|---|
| 用途 | 字幕觸發與起訖（010）、收據 | 聲音 |
| 事件 | 九種 Kind，全部轉發 | 五種 Kind，加 `Start`、`Data` |
| 安裝條件 | 觀察器實作該介面 | 同左，彼此獨立 |
| 起點 | `start.Step` | `sampleOf(Start)`，`Start == start.Step`（`sb8Begin` 在步數時鐘下取 `m.Steps`） |
| 訖點 | `Due`、`pause.Step`、`cancel.Step`（010 §3.4） | 同左 |
| 條目辨識 | `SHA256` 對 VOCLIST 條目表 | 不需要；`SHA256` 與 452 事件相同，可交叉核對 |

- 上表的「同起同訖」只在 `Clock = steps` 成立。`Clock = cycles` 時來源不出聲（§3.1）。
- 字幕與聲音在模擬時間上同起同訖。前端畫面與聲音的牆鐘差，等於 `Ring` 預填（100 ms）加播放器緩衝（50 ms）的延遲：字幕比語音早出現約 0.1 到 0.15 秒（強推論，未量）。
- `BootFromState` 後載入當下在播的那句，字幕與聲音都沒有（兩邊都缺 `start`）。

## 4. 驗收

### 4.1 單元測試（`internal/machine`、`audio/sbpcm`、`audio/mixer`、`session`）

1. **事件**：沿用 452 §5 的六段合成 `.COM`（不開分頁），加一段已知圖樣（0、1、2、…、255 循環）的緩衝，並加兩次 `14h` 傳輸：一次在 DSP 重設之後、`D1h` 之前，一次在 `D3h` 之後。`SBPCM` 事件序列的 Kind、Step、Start、Due、Transferred 與 `SBDMA` 事件對應列相同；`start` 的 `Data` 等於圖樣，`sha256(Data) == SHA256`，兩次 `SpeakerOn` 為假的傳輸同樣有 `Data`。`80h` 的 `start` 沒有 `Data`；`irq7`、`eoi`、`defaulted`、`control-idle` 不出現。
2. **實體位址與環繞**：`internal/machine` 層的單測，不用 `.COM`。設定順序：`SetSoundBlaster(true)`、`SetSBDMA8(true)` 在載入程式之前；沿用 `paging_test.go` 的頁表建法開分頁，把 DMA 緩衝所在的線性頁對映到不同的實體頁，兩邊放不同內容；以 `Out8` 直接寫 8237 與 `22Ch` 啟動傳輸。
   - `Data` 等於實體頁內容，不等於線性讀取。
   - 環繞：`Addr = FFF0h`、`Length = 32`。`Data` 前 16 bytes 取 `Page<<16 | FFF0h..FFFFh`，後 16 bytes 取 `Page<<16 | 0000h..000Fh`，不進位到下一頁。
3. **公式**：`audio/sbpcm` 以手算的小例子（N=4、Δ=10、SPS=48000×5）逐樣本比對 §3.4 的 `k`、`f`、`v`，含 `d < 0` 取 0、最後一個樣本保持、`v` 的負數右移。
4. **暫停與恢復**：`pause` 在第 k 個樣本附近，輸出在 `sampleOf(pause.Step)` 起為 0；`resume` 之後的第一個有聲輸出樣本，其來源序號 `k` 為 `T − 1` 或 `T`（`T = resume.Transferred`，第一個輸出樣本的時刻可能略早於恢復點的整數步）。遊戲收據沒有 `resume`（§2.2），`resume` 只有本項與第 1 項覆蓋。
5. **DSP 重設的取消**：未暫停時 `226h←1`，輸出在 `sampleOf(cancel.Step)` 截斷。
6. **轉回合無關**：同一段事件與步數區間，一次 `Render` 與每 1、7、192,500 步切一次的輸出逐樣本相同。
7. **混音器**：
   - 回歸：沒有 `SBPCM` 事件時，現有 `mixer_test.go` 全過；以固定 OPL 事件序列產生的 `Render` 輸出 SHA-256 等於在 `395b679` 先取的黃金值。
   - SBPCM 路徑：只有 SB 事件時，輸出等於 `float32(v / 32768)`；OPL 與 SB 同時滿幅時夾到 ±1。
8. **計數**：ADPCM、`Clock = cycles`、`complete` 步不符、沒有對應聲部的 `pause` 各觸發一次，`adpcmSkipped`、`cycleClockSkipped`、`completeMismatch`、`orphanEvents` 各為 1，且無聲。`SBPCMMutated`：傳輸中由測試直接改寫緩衝一個 byte，`complete` 後計數為 1；`LoadState`、`Restore` 後歸零；沒有 `start` 就到 `complete`（從存檔接續）時不計。
9. **觀察器**：`SBPCMObserver` 的安裝、移除、panic 鎖存、`SBDMA8` 為假時不呼叫，照 452 §3.1 第 4 到 6、9 項同法。

### 4.2 遊戲上（strike_commander 主代理執行；`-mode13-phase`）

期望值由獨立腳本產生：Docker 內的 Python，**不 import dosgolem 程式**，依 §3.4 的公式自寫。輸入：

- 同次執行的 `-sb-dma-log` 收據。`start` 列的 `Step` 即 `Start`，`Δ = Due − Step`；`resume` 列的 `Start = Due − Δ`（`Δ` 取同一傳輸 `start` 列）；`pause`、`cancel` 列取 `Step`。
- 樣本 bytes：`/home/anr2/cht/strike_commander/workplace/root/VOCLIST.TRE`（光碟版，即收據執行用的 `/orig` 樹，RE 007 §2），以 `/home/anr2/cht/strike_commander/workplace/i022/tools/voctiming.py` 的條目切法取得條目樣本，拆頁兩段取前後切片。
- 偵測傳輸（4 bytes，`Page` 00h）不在 VOCLIST。它的 bytes 取 `-dump-sbpcm-raw` 的檔，只驗 SHA-256 等於收據；期望 WAV 用這份 bytes。

獨立性的邊界：`Step`、`Due` 只能來自機器，沒有別的來源；獨立的是樣本 bytes（原版資料）與公式的重寫。`Due − Step` 本身已由 RE 022 另證等於 VOC 秒數 × 11,580,932（`timing-*.tsv` 的 `steps_per_s`）。

| 編號 | 案例與起點 | 比對 | 期望 |
|---|---|---|---|
| G1 | RE 022 的 T10、H12、C11（狀態檔與腳本同 `workplace/rerec459/` 的重錄配方），加 `-sbpcm-linear-check` | `-dump-sbpcm-raw` 每個檔 | SHA-256 等於收據 `start` 列的 `SHA256`；bytes 等於 VOCLIST 條目樣本（拆頁兩段各等於前後切片）。`linearMismatch` 照實記錄：為 0 表示 JEMM 對這些頁恆等對映（量測，不是通過條件） |
| G2 | 同 G1 | `-dump-sbpcm-wav` 對期望 WAV | 逐樣本相同。期望 WAV 的第 0 個樣本對齊 probe stderr 印出的第一個樣本序號（應等於收據第一個 `start` 列的 `sampleOf(Step)`，兩者也要相同），長度到 `sampleOf(stderr 印出的結束 m.Steps)`（不含），SPS 用 11,550,000。拆頁句照收據的兩列 `start` 分別建聲部，不用 `timing-*.tsv` 合併後的一列 |
| G3 | K1（播放中點擊）、K2（按 Esc） | 同 G2，收據含 `pause`、`cancel` 列 | 逐樣本相同；`pause` 之後到下一個 `start` 之前全為 0 |
| G4 | 冷開機到 MENU 第一次出現（10.1 億步） | 4 bytes 偵測傳輸 | 有 4 個來源樣本的聲音（不門控）。收據記下該次的 4 個 IVT[0] 位元組 |
| G5 | T10 | 有無 `SBPCMObserver`、有無混音器 | 停止步的 `Digest`、`MachineDigest`、`SaveState` bytes 相同；`sbdma.tsv` 與重錄收據逐列相同 |
| G6 | T10、K1，`AdLib` 關閉 | session 加 `mixer.Mixer`，每 192,500 步 `Advance` 一次、`Render` 一次（前端的回合切法），收集左聲道 | 每個樣本逐位元等於 `float32(G2 的 v ÷ 32768)`；記錄削波樣本數（AdLib 關閉時應為 0）。前提：喇叭與 PIT2 在該段沒有輸出，否則先記錄差異來源再判 |

### 4.3 其他程式不受影響

機器側：

- 沒開 `-sb-dma8` 的程式不產生 `SBPCM` 事件，`sb8` 相關程式碼不執行。
- 開 `-sb-dma8` 但不掛 `SBPCMObserver`：444 到 446、452 的單元測試，448 到 451、459 的長測試，strike_commander 規格 001 到 010 的長測試全過；probe `-sb-dma-log` 的收據與重錄前逐列相同（`SHA256` 欄驗證 `sb8Capture` 重構沒有改值）。

混音器側：

- `cmd/buckrogers-play -wav` 以同一腳本、同一步數跑改動前後，WAV 逐位元組相同（沒有 SB 事件時 `Render` 不做加法，§3.5）。
- §4.1 第 7 項的黃金 SHA。

共同：

- 改動前後 `go test -count=1 ./...` 的套件結果相同（ebiten 套件在無顯示器容器裡改動前就失敗，照 452 §5 的記法）。
- `000-index.md` 登記本規格（`internal/repolint` 會檢查）。

### 4.4 負對照（實作子代理逐列執行）

| 編號 | 突變 | 必須失敗 |
|---|---|---|
| N1 | 來源忽略 `pause` | §4.1 第 4 項；G3 |
| N2 | 丟掉拆頁的第二段（第二個 `start` 不建聲部） | 單元測試（§4.1 第 1 項加一組兩段傳輸）、G2（H12 的拆頁句）。G1 是機器側傾印，不受影響 |
| N3 | 取樣改用線性讀取（經頁表） | §4.1 第 2 項 |
| N4 | 放置改用 `RateNum ÷ RateDen` 換算長度（突變時暫加 `RateNum`、`RateDen` 到 `SBPCMEvent`） | G2（拉伸差約 0.27%） |
| N5 | machine 在 `SpeakerOn` 為假時不送 `Data`（門控只能在 machine 層做，`SBPCMEvent` 沒有 `SpeakerOn`） | §4.1 第 1 項（`D1h` 前、`D3h` 後兩次傳輸）、G4 |
| N6 | 回呼內寫一個 byte 到 `Mem` | G5（`Digest` 不同） |
| N7 | `Render` 開頭清空聲部 | §4.1 第 6 項 |
| N8 | 內插改成取最近樣本 | §4.1 第 3 項、G2 |
| N9 | 把 bytes 加進 `SBDMAEvent` | §4.1 第 1 項加 reflect 斷言：`reflect.TypeOf(SBDMAEvent{}).NumField() == len(SBDMAEventFields)`，且沒有切片型別的欄位 |
| N10 | 取樣位址進位到頁（`Page<<16 + Addr + i`） | §4.1 第 2 項的環繞 |

證明為等價突變的列，在實作收據記錄處置，不退回 DRAFT。

## 5. 決定與待決

| 編號 | 事項 | 決定 |
|---|---|---|
| Q1 | `SpeakerOn` 是否門控 | **定案：不門控**，照 DOSBox-X 的 SB16（`sblaster.cpp:762-767`）。016 第 2 版沒有相關規定，由本規格定案。偵測傳輸可能有一聲咔嗒（§3.4），016 人工驗收聽開場（§6） |
| Q2 | 混音器 `32h`、`33h` 被寫過時是否套 `calc_vol`；247 的讀回預設 `C0h`（24/31）若拿來算增益是 −14 dB，與 DOSBox-X 重設值 31（增益 1.0）不一致 | 待決。本版固定 1.0；247 的讀值模型不改。等有程式真的寫混音器再另立規格，並一併處理 `30h`、`31h` |
| Q3 | `BootFromState` 載入當下在播的傳輸要不要補播 | 待決，建議不補，與 010 一致。補播需要在啟動時從 `sbDMA8State` 取出進行中的傳輸並讀實體記憶體，送一筆 `inflight` 事件；這筆事件的格式與時點另立規格 |
| Q4 | 字幕比語音早約 0.1 到 0.15 秒（`Ring` 預填與播放器緩衝） | 待決，建議不延後；016 人工驗收時聽看 |
| Q5 | 停在 `pause` 時是否淡出以免爆音 | **定案：不淡出**，照硬體與 DOSBox-X（`D0h` 直接停送樣本）。016 人工驗收加跳句爆音項（§6） |
| Q6 | 內插法 | 定案：線性。DOSBox-X 混音器的重取樣方式與實際波形未量測，本版不追求與 DOSBox-X 逐樣本相同 |
| Q7 | ADPCM 解碼與自動初始化 | 待決。本遊戲收據沒有；戰役飛行（`SPEECH.PAK`）、過場、`MKTERR`、資料片的 SB 模式未量。量到再另立規格 |
| Q8 | 傳輸期間緩衝被改寫時的逐段取樣 | 待決。本遊戲 730 對 0 差；先只計數 `SBPCMMutated`，其他程式出現時另立規格 |

## 6. 給呼叫端：016 要修訂的行

本規格不改 016。下列行號依 `workplace/fe016/spec016-draft.md` 第 2 版（2026-10-23 13:57 的檔）。

| 016 行 | 現況 | 要改成 |
|---|---|---|
| 30 | R3「另由 dosgolem 規格處理」 | 指名 dosgolem-sc 規格 462 |
| 499 | SB16 事件經 `SBDMAObserver` 轉發，不含樣本 | 補一列：樣本經 462 的 `SBPCMObserver`（`SBPCMEvent.Data`）送達 |
| 505 | `Ring` 的行號引用 | 對齊 `ring.go` 的實際行號（`Read` 在 61-103） |
| 507 | 「SB16 PCM 輸出另由 dosgolem 規格處理」 | 指名 462 |
| 511 | 觀察器實作 `AudioObserver` 與 `SBDMAObserver` | 另實作 `SBPCMObserver`，轉給 `mixer.Mixer.SBPCM` |
| 512-517 | 對 R3 的接點要求 | 改寫為引用 462：取樣在 machine（462 §3.2）；放置用 `Start`、`Due`（§3.4）；不門控 `SpeakerOn`（§5 Q1）；第 516 行「單週期以外是否支援」已由 462 §2.2 回答：收據範圍內沒有；第 517 行改為「462 實作前」 |
| 543 | R1 觀察器組裝 | 回傳的觀察器另實作 `SBPCMObserver` |
| 545 | R3「dosgolem 規格（另案）」 | 指名 462 |
| 574 | A11「`-wav` 輸出在語音起點有非零樣本，長度符合 `Samples ÷ 取樣率`」 | 改用 462 的驗收（§4.2 G2、G6）：以 `sbpcm.Source` 單獨輸出比對；長度以 `sampleOf(Due) − sampleOf(Start)` 計，不用 `Samples ÷ 取樣率` |
| 587 | 人工驗收「開場到主選單」 | 加：MENU 偵測時是否聽到一聲短咔嗒（462 §3.4、Q1），記錄有無 |
| 589 | 人工驗收「任務簡報」 | 去掉「R3 完成後」；加：點擊或 Esc 跳句時的爆音是否刺耳（462 Q5）；字幕與語音的先後差是否明顯（462 Q4） |

## 附錄 A：版本紀錄

| 版 | 日期 | 內容 |
|---|---|---|
| 1 | 2026-10-23 | 初稿 |
| 2 | 2026-10-23 | 依第一輪審查甲、乙修訂（開頭處置表）；標 READY 候選；編號定為 462 |
