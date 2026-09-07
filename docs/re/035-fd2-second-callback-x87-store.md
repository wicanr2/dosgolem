# 035 — FD2 第二個啟動回呼保存 x87 控制字

日期：2026-09-06  
證據等級：**已證實**（固定雜湊原始位元組、自然執行路徑與記憶體結果）  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：dosgolem `feat/fd2-parity`，基線提交 `cee9142`  
位址空間：dosgolem 載入 LE 映像後的 32 位元線性位址

第二個回呼清除 BL 後執行：

```text
0x460E1  88 1D F5270500  mov byte ptr [0x527F5],bl
0x460E7  2B C0           sub eax,eax
0x460E9  50              push eax
0x460EA  DB E3           fninit
0x460EC  D9 3C 24        fnstcw word ptr [esp]
0x460EF  58              pop eax
```

固定輸入自然抵達 `0x460EF` 時，`[0x527F5]=0`、EAX=0、ESP=`0x55690`，
堆疊頂端與 CPU x87 控制字皆為 `0x037F`。這證明 dosgolem 已能承載此段啟動
能力檢查；尚未執行的 `pop eax` 與後續分類分支不屬於本證據。

「這段用於 x87 能力分類」為**強推論**；`0x527F5` 的高層名稱仍為**未知**。
