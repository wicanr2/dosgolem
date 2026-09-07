# 024 — FD2 第一個啟動 callee prologue

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、dosgolem stack receipt）

沿用 `023` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x45D9A  56                 push esi
0x45D9B  57                 push edi
0x45D9C  53                 push ebx
0x45D9D  06                 push es
0x45D9E  BE B0390500        mov esi,0x539B0
0x45DA3  BF E0390500        mov edi,0x539E0
0x45DA8  8B DF              mov ebx,edi
0x45DAA  B0 FF              mov al,0xFF
0x45DAC  3B F7              cmp esi,edi
```

進入時 SS:ESP=`0x160:0x556AC`。三個通用 register push 後，`PUSH ES` 將 flat
selector `0x160` 的零擴展 32-bit stack cell 寫至 `0x5569C`，再設定表格範圍與
AL sentinel。執行停在第一次表格範圍比較前；callee 的高層用途仍未知。

`PUSH ES` 的四 byte stack slot 是 32-bit protected-mode 平台契約；本 receipt
證實 FD2 在此路徑消費該形狀，不聲稱 DOS/4GW 私有語意。
