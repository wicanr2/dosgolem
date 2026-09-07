# 143 — CPU386 SIB＋disp8 byte TEST

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 101`](../re/101-fd2-iomode-tests-file-state.md)

## 範圍

- 擴充 opcode `F6 /0`，支援固定原版需要的 ModRM `44h` 與 SIB `03h`：
  `TEST byte ptr [ebx+eax+disp8],imm8`。
- disp8 以有號值加入 EBX 與 EAX；以 DS selector 讀取一個 byte。
- 以既有 `setLogicFlags8(value & imm)` 更新 CF／PF／AF／ZF／SF／OF。

## 失敗即關閉

- 只接受這個已證實的 SIB shape；其他 SIB、scale、index、base、記憶體形狀
  與 prefix 維持拒絕。
- 位址超出 DS descriptor 時不得讀取或假裝成功。

## 驗收

- CPU 單元測試驗證命中、零結果、有號 disp8、來源不變與越界失敗。
- 固定原版由 LE entry 自然執行並越過 `0x46375`；後續阻塞不得仍是該
  `F6 44 03 01 40`。

本規格不宣告 `0x4637D` 的 OR 或 `isatty` 已完成。

驗收收據（2026-09-06）：`TestByteTESTAtSIBDisp8` 通過命中、零結果、
有號 disp8、來源不變、越界與未授權 SIB 拒絕；`TestFD2TestsIOModeFlag`
由固定原版 LE entry 自然執行 `0x46375`，抵達 `0x4637A`，並核對 ZF 與
原始 byte 的 bit `0x40` 一致且沒有修改來源。後續有界探針的下一阻塞為
`0x4637D` 的 `80 4C 03 01 40`。
