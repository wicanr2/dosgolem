# 048 — FD2 第三次回呼的第一個配置型呼叫

日期：2026-09-06  
證據等級：指令、資料流、呼叫位址與 Watcom `_nmalloc` 身分為**已證實**

輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：dosgolem `feat/fd2-parity`，基線提交 `e9a8910`  
位址空間：dosgolem 載入 LE 映像後的 32 位元線性位址

最小 environment 的首 byte 為零並跳至 `0x4CC41` 後：

```text
0x4CC41  2B 45 FC           sub eax,[ebp-4]
0x4CC44  75 05              jnz 0x4CC4B
0x4CC46  B8 01000000        mov eax,1
0x4CC4B  50                 push eax
0x4CC4C  E8 D5A0FEFF        call 0x36D26
0x4CC51  89 C7              mov edi,eax
```

固定前置 EAX=0、`[EBP-4]=0`，所以 subtraction 得零、`JNZ` 不跳轉，再把
EAX 設成 1 作為堆疊參數並呼叫 `0x36D26`。後續立即保存回傳 EAX，並另有零值
檢查與第二次同函式呼叫。IDA Pro 9.4 的 Watcom 執行期簽章、函式邊界與直接
呼叫關係已進一步證實它是 `_nmalloc`；完整證據見
[`RE 049`](049-fd2-watcom-nmalloc.md)。本頁保留早期定位鏈，但不再沿用
「配置型 helper」的舊推論。
