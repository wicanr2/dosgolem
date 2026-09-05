# 005 — FD2 穩定標題畫面對拍

日期：2026-09-05
狀態：**DOSBox 輔助基準（dosbox-bootstrap／near-state）**

## 輸入與工具

- 原版：固定雜湊 `FD2.EXE`，DOSBox 擷取映像
  `fd2-dosbox-screenshot-local:latest`，`cycles=fixed 12000`。
- 時間線：`wait:10;repeat:8,Escape,500;wait:2;shot:title-original`。
- dosgolem：`ce69513` 的 `tools/fd2/capture.sh`、具型別場景契約與
  `apps/fd2/cmd/parity`。
- 重製：`fd2_re` `1178c8a0` 的既有執行期標題收據
  `title-remake-runtime.png`，明示以最近鄰從 `640×400` 正規化到
  `320×200`。

原版完整擷取視窗固定為 `1024×768`，遊戲原生畫布在左上角 `320×200`。
擷取器先驗證完整視窗尺寸，再裁切畫布；這不是任意縮放。

## 結果

64,000／64,000 個 RGB 像素完全相同，RGB 平均絕對誤差為 0；機器可讀摘要見
[`fd2-title-parity.json`](../evidence/fd2-title-parity.json)。原版與重製 PNG 不進
dosgolem 版控，避免散布原版素材。

證據仍標為 `near-state`，因重製畫面是既有執行期收據，這一輪沒有用同一份
宣告式輸入腳本重新驅動重製程式。像素結果證實穩定標題的視覺輸出一致，不能
外推為開場動畫、輸入時間或其他場景已一致。

2026-09-06 勘誤：本圖由 DOSBox 產生，只可協助補足 dosgolem；在 dosgolem
自行執行至同一畫面並重生收據前，不得稱為正式原版／重製對拍完成。

## 勘誤

第一次誤用了 `fd2_re/docs/figures/title.png`；該檔是帶洋紅遮罩的研究素材，不是
重製執行期畫面，所得 0% 相符率已作廢。正式比較改用
`title-remake-runtime.png`。這項錯誤保留在本紀錄，避免後續把研究圖再次當成
執行期收據。
