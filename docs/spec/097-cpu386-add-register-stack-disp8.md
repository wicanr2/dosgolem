# 097 — 386 暫存器加上 stack disp8 dword

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 061`](../re/061-fd2-dpmi-lock-sized-region.md)

- 擴充無前綴 opcode `03`，只接受 `mod=1`、SIB
  `scale=0/index=none/base=ESP` 的 `ADD r32,[SS:ESP+disp8]`。
- 使用有符號 disp8 與 SS descriptor；成功讀取後寫回目的暫存器，更新完整
  32 位 addition 旗標。讀取失敗不得修改目的或旗標。
- 其他 `03` 形狀與前綴失敗即關閉。
- 固定 FD2 在 `0x3631A` 以 `03 44 24 08` 建立 DPMI 鎖定終點。

驗收：單元測試覆蓋一般加法、carry 與越界；固定雜湊 FD2 必須由 LE entry
自然完成 AIL 的八個定長資料區鎖定。

驗收收據（2026-09-06）：`TestAddRegisterStackDisp8` 與固定雜湊
`TestFD2CompletesAILDPMILocksWhenProvided` 通過；後者由 LE entry 自然執行到
`0x378DE`，並確認 AIL 初始化閘門 `dword_527E4=1`。
