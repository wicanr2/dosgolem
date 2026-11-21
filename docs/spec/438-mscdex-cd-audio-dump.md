# 438 — MSCDEX 的 CD 音軌倒成 WAV

狀態：**READY**
日期：2026-10-18
前置：[`434-mscdex-minimal`](434-mscdex-minimal.md)
案例：三國志V（san5）的 CD 音樂（標題選單的第 21 軌等）

---

## 1. 為什麼要做

`434-mscdex-minimal` 只回播放狀態，不出聲。三國志V 的背景音樂有一部分是 CD 音軌：標題選單送
PLAY AUDIO 播第 21 軌，播完再送一次（量測：`wicanr2/san5-remake` 的 `docs/re/013-main-to-title-menu.md`）。
要聽到遊戲當下播的是哪一段，就要把播放紀錄對上光碟映像的音軌資料。

## 2. 語意

- 曲目表每行可以多一欄：`<軌號> audio|data <LBA> <映像位移>`。映像位移是這一軌第一個扇區在光碟映像檔裡的
  位元組位移，映像每扇區 2352 bytes（raw，例如 `.mdf`、`.bin`）。三欄的舊格式仍接受，那一軌不能倒音訊。
- MSCDEX 記下每段播放 `CDPlay{FromStep, ToStep, Start, Length, SPS}`：
  - PLAY AUDIO（`84h`）先結束還在播的那段，再開新的一段；
  - STOP（`85h`）在播放中時、SEEK（`83h`）結束目前這段（`ToStep` = 當時的指令數）；
  - RESUME（`88h`）從暫停的位置開新的一段。
  - 播滿長度自然結束時 `ToStep` 留 0，長度本身就是終點。
- `CDROM.WriteWAV(path, image, endStep, sps)`：輸出 44.1 kHz、16 位元、立體聲的 PCM WAV，
  時間軸從指令 0 到 `endStep`，樣本位置 = 指令數 ÷ `sps` × 44100（捨去）。
  每段播放從 `FromStep` 的樣本位置起，依序複製映像中 LBA = `Start` 起的扇區（每扇區 588 個樣本），
  長度取「`Length` 扇區」與「`ToStep − FromStep` 換算的樣本數（用該段的 `SPS`）」較小者；
  遇到資料軌或沒有映像位移的軌就停。其餘位置是 0（靜音）。CD-DA 的取樣格式與 WAV 相同，不重取樣。
- `probe -cdrom D:<曲目表> -cdrom-image <映像> -dump-cd-wav <輸出>`：跑完寫 WAV，並列出每段播放。

## 3. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 即時播放 | 本規格只倒檔；混音與即時播放見 `440-mixer-cd-audio`、`441-play-frontend` | DOSBox-X 即時混音 | 已由 440、441 處理 |
| 聲道與音量（IOCTL OUTPUT 03h） | 不套用 | DOSBox-X 套用 | 三國志V 未觀測到改音量 |
| 狀態檔之前的播放 | `-load-state` 之後只記得到新的播放 | 狀態檔不存 MSCDEX（`434` §4） | 同 `434` |

## 4. 驗收

1. `internal/dos/mscdex_contract_test.go` 的 `TestMSCDEXWriteWAVCopiesPlayedSectors`：自造映像的音軌扇區照時刻出現在 WAV，
   STOP 之後是靜音，播放之前是靜音，資料長度約為 `endStep` 換算的秒數。
2. 負對照：不理會 `ToStep`，第 1 條必須失敗（2026-10-18 手動確認後還原）。
3. 上層收據：三國志V 從 100,330,000 道指令的狀態跑到 6 億道，WAV 裡第一次播第 21 軌的位置（145.16 秒）起 30 秒，
   與標準版 `.mdf` 第 21 軌開頭 30 秒逐位元組相同。
