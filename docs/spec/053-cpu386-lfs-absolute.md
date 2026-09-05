# 053 — 386 絕對位址 LFS

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 045`](../re/045-fd2-third-callback-lfs.md)

- 支援無前綴 `0F B4 /r` 的 `mod=0, r/m=5, disp32` 形式。
- 經 DS 描述子由 disp32 讀取連續的 32 位元 offset 與 16 位元 selector；兩者
  都成功且 selector 通過 FS load gate 後，才原子更新目的暫存器與 FS。
- 任一讀取或 selector 驗證失敗時不得部分更新；其他定址及前綴維持失敗即關閉。
- 固定 FD2 從 LE 入口自然抵達 `0x4CC1D`，驗證 EAX=0、FS=`0x0030`、
  ESP=`0x5567C`。
- 固定 FD2 host 對既有 environment selector `0x0030` 增加 FS 載入目的地；
  backing 與界限沿用 spec 025，不建立一般 flat descriptor。

驗收包含獨立成功與拒絕部分更新測試、固定雜湊整合測試及探針收據。

2026-09-06：成功／原子拒絕單元測試、host selector gate 測試、固定雜湊整合
測試、探針及全套 Go 回歸通過；探針於 473 步抵達 `0x4CC1D`，EAX=0、
FS=`0x0030`、ESP=`0x5567C`。
