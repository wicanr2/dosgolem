# 440 — 混音器加入 CD 音軌，probe 輸出混音 WAV

狀態：**READY**
日期：2026-10-18
前置：[`240-opl-audio-output-draft`](240-opl-audio-output-draft.md)、[`438-mscdex-cd-audio-dump`](438-mscdex-cd-audio-dump.md)
案例：三國志V（san5）

---

## 1. 為什麼要做

三國志V 的聲音只有兩路（量測：`wicanr2/san5-remake` 的 `docs/re/014-san5-audio.md`）：

- 背景音樂走 CD 音軌。22 首曲目都對到光碟音軌，遊戲經 MSCDEX 送 PLAY AUDIO。
- 音效走 FM。BGMSEQ 的音效命令交給 FMOPL2，寫 OPL2 埠 `388h`、`389h`。

`audio/mixer` 已經合成 OPL 與 PC 喇叭，`438` 只把 CD 音軌倒成獨立的 WAV。要同時聽到音樂與音效，
CD 音軌要進同一個混音器，和 OPL 用同一條步數時間軸。

## 2. 語意

### 2.1 混音器的 CD 來源

- `Mixer.AttachCD(cd *dos.CDROM, image io.ReaderAt)`：混音器從 `cd.Plays` 讀播放紀錄，從 `image`（raw，每扇區 2352 bytes）
  讀音軌資料。沒有呼叫時混音器行為與以前相同。
- 時間軸：輸出樣本序號 n 對應步數的方式不變（`sampleOf(step) = step × 48000 ÷ stepsPerSecond`，捨去）。
  一段播放 p 的起點 `n0 = sampleOf(p.FromStep)`。
- 終點取兩者較小：`p.ToStep` 不為 0 時的 `sampleOf(p.ToStep)`（不含），以及播滿長度的位置
  （CD 樣本總數 `p.Length × 588` 換算成輸出樣本，見下一條）。`CDPlay.SPS` 不使用，時間基準一律是混音器的
  `stepsPerSecond`。
- 重取樣：輸出樣本 n（`n0 ≤ n` 且未到終點）在這段播放裡的 CD 樣本位置是 `(n − n0) × 44100 ÷ 48000`。
  整數部分 k、分數部分 f（以 48000 為分母的整數），值 = `s[k] + (s[k+1] − s[k]) × f ÷ 48000`，
  左右聲道各算一次。`k + 1` 超出這段播放的 CD 樣本數時用 `s[k]`。
  播滿長度的終點就是 CD 位置 k 到達 `p.Length × 588` 的第一個輸出樣本。
- CD 樣本 `s[k]` 讀自 LBA `p.Start + k ÷ 588` 那個扇區的第 `k mod 588` 個立體聲樣本
  （16 位元有號、小端、左聲道在前）。扇區位移用 `438` 的 `sectorOffset`；遇到資料軌或沒有映像位移的軌，
  這段播放從那裡起是靜音。讀映像失敗也當靜音，並記一次錯誤（`Mixer.CDErrors()`）。
- 播放紀錄重疊時（下一段的 `FromStep` 早於上一段的終點；MSCDEX 開新段前一定先結束舊段，所以不該發生），
  取起點較晚的那一段。
- 混音：每個輸出樣本 = OPL ÷ 32768 + 喇叭 + CD ÷ 32768，三路增益都是 1，最後夾在 [−1, 1]。
- 位置由絕對樣本序號直接算出，不依賴 `Render` 被呼叫幾次。`Render` 的區間不連續時（第一次或跳號），
  CD 照樣從正確的位置接上。

### 2.2 `probe -dump-mix-wav <輸出>`

- 開了這個旗標，probe 掛上混音器：OPL 寫入、喇叭與 PIT 通道 2 交給混音器，同時照舊追加到 `m.OPL`、`m.Speaker`，
  其他倒檔旗標的結果不變。給了 `-cdrom` 與 `-cdrom-image` 就再接 CD 來源。
- `stepsPerSecond` 用 `machine.StepsPerSecond()`。
- 主迴圈每跑 1,000,000 道指令呼叫一次 `Render`，跑完再補最後一段，樣本直接寫進檔案。輸出 48 kHz、16 位元、
  立體聲 PCM WAV，時間軸從這次執行的第一道指令起（`-load-state` 時從狀態檔的步數起）。
- 結束時印出秒數、CD 段數、讀映像錯誤數與夾值樣本數。

## 3. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 重取樣 | 線性內插 44.1→48 kHz | DOSBox-X 混音器自己的重取樣 | 刻意不照做：取樣值不是對拍目標，聽感接近即可 |
| 聲道與音量（IOCTL OUTPUT 03h） | 不套用，增益固定 1 | DOSBox-X 套用 | 同 `438`，三國志V 未觀測到改音量 |
| `-load-state` 之後的 OPL | 合成器從空白暫存器開始，狀態檔之前載入的音色聽不到 | 狀態檔不存 OPL 暫存器 | remake-owned。到期條件：狀態檔加入 OPL 暫存器時，混音器開始前先把暫存器寫進合成器 |
| 播放紀錄重疊 | 取起點較晚的一段 | MSCDEX 不會重疊 | 防禦性規則 |

## 4. 驗收

1. `audio/mixer/mixer_cd_test.go`：
   - `TestCDInterpolatesAndStops`：自造映像（第 j 個樣本的值是 7j mod 30000，斜率大於 1 才驗得到內插的分數部分）、手寫三段播放（第三段落在資料軌）。播放前與段落之間是 0；
     抽查的輸出樣本等於 §2.1 的內插式；第一段在 `ToStep` 停，第二段在播滿長度時停。
   - `TestCDPositionIgnoresRenderSplit`：同一份紀錄一次 `Render` 與切成多段 `Render` 的輸出逐樣本相同。
   - `TestCDSilentOnDataTrack`：播放落在資料軌時是 0。
2. 負對照：把 CD 位置改成從每次 `Render` 的起點起算（不從播放起點的絕對序號算），第 1 條的第二個測試必須失敗；
   把內插的分數部分拿掉，第一個測試必須失敗（2026-10-18 手動確認後還原）。
3. 上層收據（三國志V，從開機跑，不用狀態檔）：混音 WAV 中只有 CD 出聲的一段，與標準版 `.mdf` 對應音軌
   以同一條內插式獨立算出的值相差不超過 1；點選標題選單之後，CD 之外有 OPL 的能量。
