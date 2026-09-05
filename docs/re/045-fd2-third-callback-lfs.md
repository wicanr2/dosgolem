# 045 — FD2 第三次回呼載入 FS 遠指標

日期：2026-09-06  
證據等級：**已證實**（固定雜湊原始位元組、writer 後執行期內容與 consumer）  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：dosgolem `feat/fd2-parity`，基線提交 `a6a0d4d`  
位址空間：dosgolem 載入 LE 映像後的 32 位元線性位址  
平台前提：Intel 80386 `LFS r32,m16:32` 公開指令契約

第三回呼的全域閘門不跳轉後，consumer 為：

```text
0x4CC10  0F 85 C3000000       jne 0x4CCD9
0x4CC16  0F B4 05 34280500    lfs eax,fword ptr [0x52834]
0x4CC1D  89 45 FC             mov [ebp-4],eax
```

固定重定位映像初始 `[0x52834..0x52839]` 為全零；由 LE 入口自然執行到
`0x4CC16` 時內容為 `00 00 00 00 30 00`，證明前置 writer 已把 selector
`0x0030` 寫入，而 offset 保持 0。`LFS` 因此應得到 EAX=0、FS=`0x0030`。

既有 RE 016／017 已證實同一 selector `0x0030` 是具邊界 environment backing，
先前只登錄 DS 載入目的地。本次直接 `LFS` consumer 證明同一 selector 亦須允許
載入 FS；這只擴充 selector load gate，不偽造一般 flat descriptor。

本證據確認 writer 後的資料形狀與直接 consumer；該遠指標的高層用途仍為
**未知**，不得僅因 FS 指向 selector `0x30` 就命名其資料區。
