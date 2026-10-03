# 規格索引

`docs/spec/` 是 dosgolem 的規格目錄。SDD 的規則沒變：**只有標 `READY`
的規格可以動手**，反組譯／量測 → 規格 → 才寫程式。

## 引用規格時要連號碼帶檔名

規格數以目錄現況為準，而編號**不是唯一鍵**：十條分支各自從 007 開始編，
合併之後同一個號碼底下有好幾份不同主題的文件。

| 號碼 | 底下有幾份 | 檔名 |
|---:|---:|---|
| `004` | 2 | `004-dos-bios-services.md`、`004-machine.md` |
| `007` | 6 | `007-com-loader-and-keyboard.md`、`007-dos-exec-overlay.md`、`007-ega-mode-0dh-planar-vram.md`、`007-exec-ems.md`、`007-linear-executable-intake.md`、`007-vga-planar.md` |
| `008` | 7 | `008-bios-keyboard-injection.md`、`008-blocking-console-input.md`、`008-ems-paging.md`、`008-eob1-adapter.md`、`008-flat-386-entry.md`、`008-tsr-resident.md`、`008-wolong-services.md` |
| `009` | 8 | `009-bios-time-of-day.md`、`009-exec.md`、`009-fd2-dos4g-install-check.md`、`009-keyboard-irq1.md`、`009-memory-allocator.md`、`009-mouse-event-handler.md`、`009-planar-vga.md`、`009-scratch-writes.md` |
| `010` | 6 | `010-dosv.md`、`010-eob1-title-menu.md`、`010-exec-memory-reclaim-and-planar-write-modes.md`、`010-fd2-segment-bootstrap.md`、`010-input-verbs.md`、`010-overlay-loading.md` |
| `011` | 5 | `011-bios-palette-and-vector-stubs.md`、`011-ega-mode10h.md`、`011-eob1-new-party-entry.md`、`011-fd2-es-environment-cell.md`、`011-xms.md` |
| `012` | 5 | `012-cpu-386-subset.md`、`012-eob1-first-character-race.md`、`012-fd2-parity-capture.md`、`012-input-keyboard-irq-and-mouse-scale.md`、`012-mcb-chain.md` |
| `013` | 5 | `013-eob1-first-character-class.md`、`013-fd2-load-es-selector.md`、`013-file-handles.md`、`013-mouse-event-callback.md`、`013-vga-planar.md` |
| `014` | 5 | `014-dos4gw-flat-descriptors.md`、`014-ems.md`、`014-eob1-first-character-alignment.md`、`014-hardware-keyboard.md`、`014-poke-state-not-luck.md` |
| `015` | 4 | `015-bytecode-observation.md`、`015-eob1-first-character-stats.md`、`015-fd2-store-environment-word.md`、`015-outermost-cmdline.md` |
| `016` | 3 | `016-dos4gw-protected-push.md`、`016-eob1-first-character-review.md`、`016-right-mouse-button.md` |
| `017` | 2 | `017-dos4gw-selector-load-validation.md`、`017-mouse-callback.md` |
| `018` | 2 | `018-cpu386-command-tail-prelude.md`、`018-eob1-first-character-name.md` |
| `019` | 2 | `019-dos4gw-psp-command-tail.md`、`019-eob1-first-character-alfa.md` |
| `020` | 2 | `020-cpu386-repe-scasb.md`、`020-eob1-second-character-race.md` |
| `021` | 2 | `021-cpu386-lea-disp8.md`、`021-eob1-second-character-review.md` |
| `022` | 2 | `022-dos4gw-selector-swap-jz.md`、`022-eob1-second-character-beta.md` |
| `023` | 2 | `023-cpu386-buffer-stack-finalize.md`、`023-eob1-third-character-review.md` |
| `024` | 2 | `024-dos4gw-environment-selector-load.md`、`024-eob1-third-character-gamma.md` |
| `025` | 2 | `025-dos4gw-environment-block.md`、`025-eob1-fourth-character-review.md` |
| `026` | 2 | `026-cpu386-environment-prefix-test.md`、`026-eob1-fourth-character-delta.md` |
| `027` | 2 | `027-cpu386-environment-byte-scan.md`、`027-dos-dta-find-first.md` |
| `028` | 2 | `028-cpu386-pop-ds.md`、`028-eob1-play-level1-entry.md` |
| `029` | 2 | `029-cpu386-startup-buffer-clear.md`、`029-eob1-level1-first-move.md` |
| `030` | 2 | `030-cpu386-startup-buffer-tail.md`、`030-eob1-level1-first-pickup.md` |
| `031` | 2 | `031-cpu386-near-call.md`、`031-eob1-level1-first-drop.md` |
| `032` | 2 | `032-cpu386-push-es.md`、`032-eob1-level1-camp-lifecycle.md` |
| `033` | 2 | `033-cpu386-register-cmp-jae.md`、`033-eob1-level1-memorize-spell.md` |

所以：**引用寫 `docs/spec/009-memory-allocator`，不要只寫 `docs/spec/009`**。
程式碼註解裡既有的「`docs/spec/NNN`」是各分支寫的，指的是那條分支自己的
那一份——照上表對回去（用寫那一行的子系統判斷），不要照號碼硬套。

> 沒有重編號是刻意的取捨：註解裡幾百處「`docs/spec/NNN`」在合併之後每一處
> 都有 2–7 個候選，重編號要逐處判斷它原本指哪一份。判錯的結果是一個
> **看起來對、其實指到別份規格**的指標，比號碼重複更難發現。

## 依案例分組

### 共同基礎（master）（14 份）

- `001-scope-and-mvp.md`
- `002-cpu-8086.md`
- `003-machine-and-loader.md`
- `004-dos-bios-services.md`
- `005-oracle-api.md`
- `006-layering.md`
- `187-frame-clock.md`
- `188-pit-divisor-decoding.md`
- `189-a20-gate-and-hma-addressing.md`
- `190-irq0-calibration.md`
- `191-two-clocks-relationship.md`
- `192-crtc-effects.md`
- `193-crtc-timing.md`
- `194-stubseg-font-collision-and-program-path.md`

### 銀河英雄傳說 3（logh3）（10 份）

- `007-exec-ems.md`
- `008-ems-paging.md`
- `009-planar-vga.md`
- `010-exec-memory-reclaim-and-planar-write-modes.md`
- `011-bios-palette-and-vector-stubs.md`
- `012-input-keyboard-irq-and-mouse-scale.md`
- `013-mouse-event-callback.md`
- `014-poke-state-not-luck.md`
- `015-outermost-cmdline.md`
- `016-right-mouse-button.md`

### 源平合戰（yuan-genpei）（9 份）

- `004-machine.md`
- `008-tsr-resident.md`
- `009-exec.md`
- `010-dosv.md`
- `011-xms.md`
- `012-cpu-386-subset.md`
- `013-vga-planar.md`
- `014-ems.md`
- `015-bytecode-observation.md`

### 三國演義（san1）（7 份）

- `008-blocking-console-input.md`
- `009-memory-allocator.md`
- `010-overlay-loading.md`
- `011-ega-mode10h.md`
- `012-mcb-chain.md`
- `013-file-handles.md`
- `014-hardware-keyboard.md`

### 臥龍傳（wolong）（4 份）

- `007-vga-planar.md`
- `008-wolong-services.md`
- `009-mouse-event-handler.md`
- `010-input-verbs.md`

### Eye of the Beholder（eob1）（28 份）

- `007-dos-exec-overlay.md`
- `008-eob1-adapter.md`
- `009-bios-time-of-day.md`
- `009-keyboard-irq1.md`
- `010-eob1-title-menu.md`
- `011-eob1-new-party-entry.md`
- `012-eob1-first-character-race.md`
- `013-eob1-first-character-class.md`
- `014-eob1-first-character-alignment.md`
- `015-eob1-first-character-stats.md`
- `016-eob1-first-character-review.md`
- `017-mouse-callback.md`
- `018-eob1-first-character-name.md`
- `019-eob1-first-character-alfa.md`
- `020-eob1-second-character-race.md`
- `021-eob1-second-character-review.md`
- `022-eob1-second-character-beta.md`
- `023-eob1-third-character-review.md`
- `024-eob1-third-character-gamma.md`
- `025-eob1-fourth-character-review.md`
- `026-eob1-fourth-character-delta.md`
- `027-dos-dta-find-first.md`
- `028-eob1-play-level1-entry.md`
- `029-eob1-level1-first-move.md`
- `030-eob1-level1-first-pickup.md`
- `031-eob1-level1-first-drop.md`
- `032-eob1-level1-camp-lifecycle.md`
- `033-eob1-level1-memorize-spell.md`

### Pool of Radiance（por）（3 份）

- `007-ega-mode-0dh-planar-vram.md`
- `008-bios-keyboard-injection.md`
- `009-scratch-writes.md`

### UCSD p-System（psys）（1 份）

- `007-com-loader-and-keyboard.md`

### 風之谷 2／DOS4GW（fd2）（177 份）

- `007-linear-executable-intake.md`
- `008-flat-386-entry.md`
- `009-fd2-dos4g-install-check.md`
- `010-fd2-segment-bootstrap.md`
- `011-fd2-es-environment-cell.md`
- `012-fd2-parity-capture.md`
- `013-fd2-load-es-selector.md`
- `014-dos4gw-flat-descriptors.md`
- `015-fd2-store-environment-word.md`
- `016-dos4gw-protected-push.md`
- `017-dos4gw-selector-load-validation.md`
- `018-cpu386-command-tail-prelude.md`
- `019-dos4gw-psp-command-tail.md`
- `020-cpu386-repe-scasb.md`
- `021-cpu386-lea-disp8.md`
- `022-dos4gw-selector-swap-jz.md`
- `023-cpu386-buffer-stack-finalize.md`
- `024-dos4gw-environment-selector-load.md`
- `025-dos4gw-environment-block.md`
- `026-cpu386-environment-prefix-test.md`
- `027-cpu386-environment-byte-scan.md`
- `028-cpu386-pop-ds.md`
- `029-cpu386-startup-buffer-clear.md`
- `030-cpu386-startup-buffer-tail.md`
- `031-cpu386-near-call.md`
- `032-cpu386-push-es.md`
- `033-cpu386-register-cmp-jae.md`
- `034-cpu386-byte-record-scan.md`
- `035-cpu386-callback-pointer-gate.md`
- `036-cpu386-segment-transfer-indirect-call.md`
- `037-cpu386-near-jump.md`
- `038-cpu386-x87-init-control.md`
- `039-cpu386-x87-control-return.md`
- `040-cpu386-record-status-write.md`
- `041-cpu386-absolute-byte-cmp.md`
- `042-cpu386-register-byte-xor.md`
- `043-fd2-second-callback-x87-store.md`
- `044-fd2-second-callback-x87-class-gate.md`
- `045-cpu386-movzx-absolute-word.md`
- `046-fd2-control-baseline-dispatch.md`
- `047-cpu386-x87-self-test.md`
- `048-fd2-second-callback-class-result.md`
- `049-fd2-second-callback-record-state.md`
- `050-fd2-third-callback-selection.md`
- `051-cpu386-push-pop-fs.md`
- `052-cpu386-group83-sub-cmp.md`
- `053-cpu386-lfs-absolute.md`
- `054-cpu386-stack-mov-xor.md`
- `055-cpu386-environment-es-byte-gate.md`
- `056-cpu386-sub-stack-memory.md`
- `057-watcom-near-heap-runtime.md`
- `058-cpu386-register-test.md`
- `059-cpu386-register-shl.md`
- `060-cpu386-register-add.md`
- `061-fd2-environment-copy.md`
- `062-cpu386-mov-eax-moffs.md`
- `063-cpu386-sib-immediate-store.md`
- `064-cpu386-push-sign-extended-byte.md`
- `065-watcom-memset-runtime.md`
- `066-cpu386-leave.md`
- `067-cpu386-null-data-selector.md`
- `068-cpu386-absolute-byte-and-or.md`
- `069-cpu386-cmp-base-disp8.md`
- `070-cpu386-store-base-disp8.md`
- `071-cpu386-load-register-absolute.md`
- `072-cpu386-store-register-indirect.md`
- `073-cpu386-store-immediate-absolute.md`
- `074-watcom-init-argv-runtime.md`
- `075-dos-current-time-deterministic.md`
- `076-cpu386-register-cmp.md`
- `077-cpu386-register-byte-cmp.md`
- `078-cpu386-sub-register-absolute.md`
- `079-cpu386-sub-register.md`
- `080-cpu386-push-absolute-dword.md`
- `081-dos4gw-capability-matrix.md`
- `082-cpu386-push-immediate-dword.md`
- `083-cpu386-xchg-register-stack-disp8.md`
- `084-cpu386-neg-register.md`
- `085-cpu386-cmp-register-absolute.md`
- `086-cpu386-jbe-short.md`
- `087-cpu386-load-register-stack-disp8.md`
- `088-cpu386-ret-immediate.md`
- `089-cpu386-store-immediate-stack-sib.md`
- `090-cpu386-store-register-stack-disp8.md`
- `091-cpu386-and-eax-immediate.md`
- `092-cpu386-and-register-immediate.md`
- `093-cpu386-lea-stack-disp8.md`
- `094-watcom-int386-dpmi-lock.md`
- `095-cpu386-cmp-stack-disp8-immediate.md`
- `096-cpu386-setz-register-byte.md`
- `097-cpu386-add-register-stack-disp8.md`
- `098-cpu386-push-base-disp8-dword.md`
- `099-cpu386-repne-scasb.md`
- `100-cpu386-not-register.md`
- `101-cpu386-mov-absolute-indexed-sib.md`
- `102-cpu386-mov-to-absolute-indexed-sib.md`
- `103-cpu386-dec-absolute-dword.md`
- `104-cpu386-cmp-register-imm8.md`
- `105-cpu386-jl-short.md`
- `106-cpu386-pushfd.md`
- `107-cpu386-cli.md`
- `108-cpu386-store-segment-absolute-word.md`
- `109-cpu386-load-segment-absolute-word.md`
- `110-dpmi-get-real-mode-interrupt-vector.md`
- `111-cpu386-register-mov16.md`
- `112-dos386-interrupt-vectors.md`
- `113-cpu386-store-segment-register16.md`
- `114-cpu386-load-segment-register16.md`
- `115-cpu386-inc-absolute-dword.md`
- `116-cpu386-store-register-base-disp32.md`
- `117-cpu386-store-immediate-base-disp32.md`
- `118-cpu386-cmp-base-disp32-immediate.md`
- `119-cpu386-short-jb.md`
- `120-cpu386-load-register-base-disp32.md`
- `121-cpu386-cmp-base-disp8-immediate32.md`
- `122-cpu386-port-output.md`
- `123-cpu386-test-byte-base-disp8.md`
- `124-cpu386-popfd.md`
- `125-cpu386-sub-register-immediate32.md`
- `126-cpu386-lea-stack-disp32.md`
- `127-cpu386-mov-stack-disp32-read.md`
- `128-cpu386-cmp-register-immediate32.md`
- `129-cpu386-and-byte-base-disp8.md`
- `130-cpu386-movzx-byte-esi.md`
- `131-cpu386-jg-short.md`
- `132-cpu386-or-al-immediate8.md`
- `133-cpu386-or-register8-immediate8.md`
- `134-cpu386-or-base-disp8-register.md`
- `135-cpu386-movzx-byte-eax.md`
- `136-cpu386-mov-byte-base-disp8-register.md`
- `137-cpu386-or-register8-base-disp8.md`
- `138-cpu386-mov-immediate-base-disp8.md`
- `139-read-only-dos-file-provider.md`
- `140-fd2-le-dos-open-readonly.md`
- `141-cpu386-rcl-ror-one.md`
- `142-cpu386-movzx-register-word.md`
- `143-cpu386-test-byte-sib-disp8.md`
- `144-cpu386-or-byte-sib-disp8.md`
- `145-cpu386-mov-register-word-store.md`
- `146-fd2-le-dos-ioctl-device-info.md`
- `147-cpu386-test-register-byte.md`
- `148-cpu386-setnz-register-byte.md`
- `149-cpu386-movzx-register-byte.md`
- `150-cpu386-load-dword-scaled-index.md`
- `151-cpu386-store-dword-scaled-index.md`
- `152-cpu386-short-jle.md`
- `153-cpu386-dec-dword-base-disp8.md`
- `154-cpu386-short-jge.md`
- `155-cpu386-or-byte-base-disp8.md`
- `156-cpu386-push-base-dword.md`
- `157-fd2-le-dos-read.md`
- `158-cpu386-inc-dword-base.md`
- `159-cpu386-load-register-base.md`
- `160-cpu386-store-register-byte-base.md`
- `161-cpu386-load-register-byte-base.md`
- `162-cpu386-load-register-byte-sib-disp32.md`
- `163-cpu386-inc-register-byte.md`
- `164-cpu386-test-byte-base-disp32.md`
- `165-cpu386-store-register-byte-sib-disp32.md`
- `166-cpu386-store-register-stack-disp32.md`
- `167-cpu386-store-immediate-byte-base.md`
- `168-cpu386-cmp-byte-base.md`
- `169-cpu386-add-byte-register.md`
- `170-cpu386-test-byte-register.md`
- `171-cpu386-store-dword-stack-base.md`
- `172-cpu386-store-immediate-stack-disp8.md`
- `173-cpu386-lea-edi-ebp.md`
- `174-cpu386-cmp-register-stack-disp8.md`
- `175-cpu386-movzx-byte-ebx-disp32.md`
- `176-cpu386-load-al-edi-ebp.md`
- `177-cpu386-load-eax-stack-base.md`
- `178-cpu386-imul-eax-stack-disp8.md`
- `179-cpu386-store-ax-stack-disp32.md`
- `180-cpu386-neg-stack-disp8.md`
- `181-cpu386-compare-ebx-ss-ebp-disp8.md`
- `182-cpu386-compare-edx-ds-eax-disp8.md`
- `183-fd2-le-dos-seek.md`
- `184-mvp-scope-review.md`
- `185-keyboard-trace-and-keypad-names.md`


- [185 — SS 段覆寫的16位元 MOV 記憶體寫入](185-cpu386-ss-word-store.md)：CONFORMED。

- [186 — FD2平台缺口持續驗證](186-fd2-platform-gap-continuation.md)：原版標題與BIOS單次方向鍵已驗證，正常START進王宮；目前CPU缺口、音訊與輸入限制統一見186檔首。

- [189 — A20 閘門與 HMA 的定址](189-a20-gate-and-hma-addressing.md)：READY。位址遮罩從 CPU 移到匯流排——`段:偏移` 到得了 1 MB 之上，A20 關著時才環繞。

- [190 — 計時器間隔的標定](190-irq0-calibration.md)：READY。165,000 是 17,000 分頻的一刻，不是 65,536 的基準；更正 `004` §5 的公式基準。

- [191 — 兩個時鐘的關係](191-two-clocks-relationship.md)：READY。比值隨指令混合走（3.25～7.40），不是常數；不強制對齊，改成把關係做成看得到的觀測。

- [192 — CRTC 的效果](192-crtc-effects.md)：READY。列距（offset）與分割畫面（line compare）；少了它們畫面會斜成平行四邊形或狀態列跟著捲走。

- [193 — 時序暫存器](193-crtc-timing.md)：READY。從 CRTC 算掃描位置，`3DA` 回報真的回掃狀態；**預設不啟用**，既有對拍收據建立在行為模型上。
- [194 — StubSeg 字型 stub 撞號與程式路徑](194-stubseg-font-collision-and-program-path.md)：READY。字型 stub 從 0x20／0x24 移到 0x410／0x414，不再蓋掉向量 8、9 的預設 stub；`oracle.Options.ProgramPath` 與 `probe -program-path` 可指定 argv[0]。
- [195 — 明示偏移的 LE 載入探針](195-moo2-explicit-le-offset.md)：CONFORMED，僅限明示標頭解析及零基址載入；MOO2 資料頁勘誤見 197。
- [196 — MOO2 入口的 16 位元絕對位址比較](196-cpu386-moo2-word-cmp-absolute.md)：CONFORMED，僅限通用 `66 3B 15` 指令形狀；它不是 MOO2 真入口，見 197。
- [197 — 內嵌 MZ 的 LE 資料頁基址](197-bound-mz-le-file-base.md)：CONFORMED。MOO2 第二層 MZ `0x26654` 的 LE 資料頁偏移加上該層基址，真入口恢復 `EB 76 WATCOM`。
- [198 — MOO2 啟動鏈的 16 位暫存器 OR](198-cpu386-or-word-register.md)：CONFORMED。只補 `66 09 /r`、`mod=11` 的低 16 位 OR 與旗標；FD2 服務層測跑只作診斷。
- [199 — MOO2 啟動鏈的段覆寫指令形狀](199-cpu386-moo2-segment-prefixes.md)：CONFORMED。補 ES 覆寫的絕對位址 segment word store，以及無記憶體運算元的 `3E BA`；FD2 服務層測跑只作診斷。
- [200 — MOO2 暫定保護模式 DOS 服務入口](200-moo2-provisional-protected-dos.md)：CONFORMED，僅限合成環境與獨立入口；原先借用的 FD2 回傳已由 201 訂正。
- [201 — MOO2 1.31 的 DOS/4G 啟動回傳](201-moo2-dos4g-startup-returns.md)：CONFORMED，固定 DOSBox-X 輔助基準另涵蓋 DPMI `AX=0006h` 的零基底分支；PSP／環境與正式玩家路徑仍未知。
- [202 — MOO2 啟動鏈的 8 位暫存器 SUB](202-cpu386-moo2-sub-byte-register.md)：CONFORMED，僅限 `28 /r` 的暫存器形狀；合成環境診斷至下個通用段載入缺口。
- [203 — MOO2 啟動鏈的 ES 覆寫段載入](203-cpu386-es-word-segment-load.md)：CONFORMED。`26 66 8E 1D` 的 16 位 selector 絕對位址讀取，保留載入與段界限檢查。
- [204 — 暫存器立即數 MOV 前的 DS 前綴](204-cpu386-ds-prefix-register-immediate.md)：CONFORMED。`3E B8..BF` 不含記憶體運算元，前綴不改立即數或旗標。
- [205 — ES 覆寫的 byte 暫存器／記憶體比較](205-cpu386-es-byte-compare-register-memory.md)：CONFORMED。`26 3A /r` 從 ES 讀取來源 byte，依既有 `sub8` 更新旗標。
- [206 — 32 位堆疊的 PUSH GS](206-cpu386-push-gs.md)：CONFORMED，僅限所列 32 位堆疊形狀；合成環境診斷抵達下一個通用 CPU 缺口。
- [207 — 16 位暫存器與符號延伸立即數 AND](207-cpu386-and-word-register-immediate.md)：CONFORMED。`66 83 /4` 的暫存器形狀；舊 `AH=4Ah` 停點已勘誤為未綁定 DPMI 的診斷錯誤。
- [208 — 保護模式 SBB r/m32,r32 的暫存器形狀](208-cpu386-sbb-rm32-register.md)：CONFORMED。固定原版命中 `19 C0`；通用暫存器形狀越過此處，下一停點為 `87 FA`。
- [209 — 保護模式 XCHG 暫存器與暫存器](209-cpu386-xchg-register-register.md)：CONFORMED。固定原版命中 `87 FA`；通用指令越過此處，下一停點為 `F5`。
- [210 — 保護模式 CMC 進位旗標反轉](210-cpu386-cmc.md)：CONFORMED。固定原版 `F5` 命中，`EFLAGS=0202h → 0203h`；下一停點為 `21 C8`。
- [211 — 保護模式 AND r/m32,r32 暫存器形狀](211-cpu386-and-rm32-register.md)：CONFORMED。同次原版指令 LOG 核對 `21 C8`；舊 EV 暫存器快照已撤回，下一停點為 `83 0E 01`。
- [212 — 保護模式 83 /1 記憶體 OR 立即數](212-cpu386-or-rm32-imm8-memory.md)：CONFORMED。原版同次 LOG 加記憶體前後擷取核對 `90h → 91h`；下一停點為 `0F A9`。
- [213 — 32 位堆疊的 POP GS](213-cpu386-pop-gs.md)：CONFORMED，限通用指令及 MOO2 `0020h → GS` 許可。原版同次 LOG 核對 ESP 加 4；合成診斷至下一停點 `0x153E84`。
- [214 — 16 位暫存器目的的 SBB r/m16,r16](214-cpu386-sbb-rm16-register.md)：CONFORMED，限 `66 19 /r mod=11`。固定原版同次 LOG 核對 CF=0、AX 清零；合成診斷至 `0x1005B` 的 `C8` 停點。
- [215 — 32 位堆疊的 ENTER 巢狀層級 0](215-cpu386-enter-level-zero.md)：CONFORMED。原版核對 EBP／ESP 與堆疊寫入；無額外讀取，合成診斷至 `0x109FF` 的 `66 3B 4D CE` 停點。
- [216 — 16 位暫存器與 32 位位址記憶體 CMP](216-cpu386-cmp-word-register-memory.md)：CONFORMED。原版同次 LOG 核對 SS 記憶體與旗標；合成診斷至 `0x126570` 的 `66 A9 89 CF` 停點。
- [217 — 16 位累加器的 TEST 立即數](217-cpu386-test-ax-imm16.md)：CONFORMED。原版同次 LOG 核對 `66 A9 89 CF` 的 EAX 與旗標；合成診斷至 `0x139A53` 的 DOS 服務停點。
- [218 — 受保護模式 DOS 設定 DTA 指標](218-protected-dos-set-dta.md)：CONFORMED。原版 `AH=1Ah` 返回與下一次 `AH=4Eh` 已擷取；合成診斷至 `0x139A59`。
- [219 — 受保護模式 DOS 的精確檔名首次搜尋](219-protected-dos-findfirst-exact.md)：CONFORMED。缺檔與固定空檔兩種原版輔助收據；合成診斷至 `0x148224`。
- [220 — 無前綴 byte 記憶體目的 CMP](220-cpu386-cmp-rm8-register.md)：CONFORMED。`38 10` 原版同次 LOG 已擷取；合成診斷至 `0x146903`。
- [221 — ES 覆寫的 byte 記憶體來源 MOV](221-cpu386-mov-r8-es-memory.md)：CONFORMED。`26 8A 1E` 原版同次 LOG 已擷取；合成診斷至 `0x14822D`。
- [222 — 32 位暫存器 ROR 立即數 8](222-cpu386-ror-r32-imm8.md)：CONFORMED。原版零輸入 LOG 與 dosgolem 原檔整合測試通過；合成診斷至 `0x14701B`；立即數範圍由297擴充。
- [223 — 8 位元暫存器目的的 OR r/m8,r8](223-cpu386-or-rm8-register.md)：CONFORMED。原版 `08 E0` 的 AH 零輸入 LOG 與 dosgolem 原檔整合測試通過；合成診斷至 `0x15C1DF`。
- [224 — LEA 的段前綴不參與有效位址](224-cpu386-lea-segment-prefix.md)：CONFORMED。原版 `2E 8D 86` 與 dosgolem 各自狀態的 CPU 收據通過；後續連續記錄已訂正單點斷點的呼叫次數，對應來源 EAX／ESI 一致；合成診斷至 `0x15C1E7`。
- [225 — DS:[EBX] 的 word 載入 ES](225-cpu386-mov-es-ds-ebx.md)：CONFORMED。固定原檔第 5808 步越過 `8E 03`，合成診斷至 `INT 33h`；不宣稱滑鼠服務或玩家路徑完成。
- [226 — MOO2 保護模式滑鼠位置查詢](226-moo2-protected-mouse-query.md)：CONFORMED。固定原檔 `AX=3` 回傳與 record 消費端通過；合成診斷至 `8F 47 14`，尚無 GUI 玩家對拍。
- [227 — POP dword 至 DS:[EDI+disp8]](227-cpu386-pop-edi-disp8.md)：CONFORMED。固定原檔越過 `8F 47 14`，合成診斷至 `66 8C 03`；原版非零來源未獨立擷取。
- [228 — 將 ES 的 word 寫入 DS:[EBX]](228-cpu386-mov-es-to-ds-ebx-word.md)：CONFORMED。原版同次 LOG 與前後記憶體、固定原檔整合測試已核對；非相等搬移依 CPU 契約及合成測試，下一停點為 `INT 33h/AX=21h`。
- [229 — MOO2 保護模式滑鼠軟體重設](229-moo2-protected-mouse-software-reset.md)：CONFORMED。固定原版同次呼叫與返回、caller 消費端及 dosgolem 原檔整合測試通過；內部滑鼠狀態依 DOSBox-X 契約標示近似，下一停點為 `INT 33h/AX=1Ah`。
- [230 — MOO2 保護模式滑鼠零敏感度設定](230-moo2-protected-mouse-zero-sensitivity.md)：CONFORMED（歷史零值切片）。原版同次樣本及整合測試有效；非零拒絕邊界已由 [254-moo2-protected-mouse-sensitivity-settings.md](254-moo2-protected-mouse-sensitivity-settings.md) 取代，移動係數仍未建模。
- [231 — MOO2 保護模式視訊模式 03h 啟動呼叫](231-moo2-protected-video-mode-03.md)：CONFORMED。固定原版與 dosgolem 的受限模式呼叫已核對；合成環境後續因缺 `MOX.SET` 以代碼 1 結束。
- [232 — 16 位元記憶體與立即數比較](232-cpu386-cmp-rm16-imm16.md)：CONFORMED。空 `MOX.SET` 的原版 LOG、dosgolem 固定原檔與合成測試通過；完整本機資料下一停點是 `66 F7 /0`。
- [233 — 16 位元記憶體與立即數 TEST](233-cpu386-test-rm16-imm16.md)：CONFORMED。CPU 測試與正版資料診斷已越過第 288215 步；原版同狀態斷點仍未命中。合成記憶體退出已定位至 DOS 低位游標與線性配置不重用，需另立規格。
- [234 — DPMI 線性區塊釋放與重用](234-dpmi-linear-block-reuse.md)：CONFORMED。首次適配與控制代號測試、固定原檔全套測試通過；完整資料走至第 3,947,961 步，低位 DOS 配置仍失敗，尚無玩家路徑對拍。
- [235 — LE 映像與 DOS 低位記憶體隔離](235-le-image-and-dos-arena-separation.md)：CONFORMED，限明示高位 LE 映射。固定原檔及正版資料自然完成 513／176 段落兩筆配置；段號不與 DOSBox-X 逐值比較，無玩家畫面對拍。
- [236 — MOO2 實模式 VBE 控制器資訊呼叫](236-moo2-real-mode-vbe-controller-info.md)：CONFORMED。受限 BIOS callback、緩衝與拒絕測試通過；原檔自行前進至直接保護模式 `INT 10h/AX=4F07h`。
- [237 — MOO2 VBE 顯示起點歸零](237-moo2-vbe-zero-display-start.md)：CONFORMED。原版 `4F07h` 零座標返回、caller 記錄及 dosgolem 自生越過受限視訊服務；畫面輸出另驗。
- [238 — MOO2 查詢 VBE 模式 0101h](238-moo2-vbe-mode-0101-info.md)：CONFORMED。原版兩次 `4F00h` 後的 `4F01h` 固定模式與 256 位元組緩衝已核對；dosgolem 自生下一停點為直接 `INT 10h/AX=4F02h`。
- [239 — MOO2 設定 VBE 模式 0101h](239-moo2-vbe-set-mode-0101.md)：CONFORMED。原版直接 `4F02h` 的返回與 caller 已擷取；dosgolem 自生前進至第 1,151,730 步的 opcode `04h` 停點，無畫面對拍。
- [240 — 以立即數加到 AL](240-cpu386-add-al-immediate.md)：CONFORMED。`04 20` 原版同指令形狀的暫存器／旗標與 dosgolem 自生越過停點均已核對；下一停點為 `INT 66h` 底層 DSP 埠。
- [241 — MOO2 保護／實模式共用平台埠](241-moo2-shared-platform-ports.md)：CONFORMED。原檔 `INT 66h` 越過 DSP 重設埠；正式共用接線後仍在第二 DMA 埠 `00D4h` 拒絕，尚非音訊或畫面對拍。
- [242 — 第二組 DMA 控制器單通道遮罩](242-secondary-dma-single-mask.md)：CONFORMED。原檔越過 `D4h/05h`，實模式下一停點為 `D8h/00h`；限平台遮罩狀態，尚非音訊或畫面對拍。
- [243 — 第二組 DMA 控制器暫存器程式設定](243-secondary-dma-register-programming.md)：CONFORMED。原檔越過 `D8h/00h`；第二控制器獨立暫存器已測試，下一停點為頁埠 `8Bh`。
- [244 — PC AT 第二組 DMA 頁暫存器](244-secondary-dma-page-registers.md)：CONFORMED。原檔越過 `8Bh/00h`，下一停點為 DSP `022Ch/B0h`；尚非 16 位元音效或畫面對拍。
- [245 — MOO2 SB16 `B0h` 單次 16 位元 DMA](245-sb16-b0-single-cycle-probe.md)：CONFORMED。原檔自然越過 `B0 30 00 00`；音訊完成時間僅為硬體規格近似，無波形或聽感對拍。
- [246 — MOO2 啟動時安裝 BIOS 資料區](246-moo2-bios-data-attach.md)：CONFORMED。重用既有 DOS/4GW BDA 安裝契約，原檔越過誤讀 `0006h`，下一停點為 DSP `0225h`。
- [247 — SB16 左右數位語音音量混音器暫存器](247-sb16-voice-volume-mixer.md)：CONFORMED。原檔越過索引 `32h` 讀取，實模式返回；下一停點為高位 LE 線性 `0x25221E` 的 `83 C8 10`。
- [248 — 32 位元暫存器 OR 符號擴展立即數](248-cpu386-or-register-imm8.md)：CONFORMED。原版同次 raw bytes、EAX 與旗標已核對；dosgolem 自生下一停點為 `81 F2 00 80 00 00`。
- [249 — 32 位元暫存器 XOR 完整立即數](249-cpu386-xor-register-imm32.md)：CONFORMED。原版 raw bytes、EDX 與實際 EFLAGS 已核對；全套測試通過，尾端 BIOS tick 等待由規格 250 接線解決。
- [250 — MOO2 啟動時安裝既有 BIOS 時鐘](250-moo2-bios-clock-attach.md)：CONFORMED。原檔自然離開 `046Ch` 等待；下一停點為 `INT 33h/AX=0000h`，無正常玩家畫面。
- [251 — MOO2 保護模式滑鼠驅動重設](251-moo2-protected-mouse-driver-reset.md)：CONFORMED。原版返回與 record 消費端已核對；自然越過重設，下一筆服務為 `AX=001Bh`。
- [252 — MOO2 保護模式滑鼠敏感度查詢](252-moo2-protected-mouse-sensitivity-query.md)：CONFORMED。正式兩側前兩筆序列與三個 50 返回已核對；自然下一停點為水平範圍設定 `AX=0007h`。
- [253 — MOO2 保護模式滑鼠座標範圍](253-moo2-protected-mouse-coordinate-ranges.md)：CONFORMED。兩軸範圍與受控輸入已驗；自然下一停點為非零敏感度設定 `AX=001Ah`。
- [254 — MOO2 保護模式滑鼠非零敏感度設定](254-moo2-protected-mouse-sensitivity-settings.md)：CONFORMED。設定／查詢按平台契約驗收，規格 230 非零拒絕邊界已回填；自然下一停點為回呼設定 `AX=000Ch`，移動速度仍未知。
- [255 — MOO2 保護模式滑鼠回呼](255-moo2-protected-mouse-callback.md)：READY。受控派送、狀態恢復及原檔早期遠返回已驗；完整座標／游標消費支線同狀態仍待驗。
- [256 — 從 CS 絕對位址載入 DS](256-cpu386-cs-absolute-ds-load.md)：CONFORMED。窄指令與原檔回呼自然越過停點已驗。
- [257 — MOO2 保護模式滑鼠座標設定](257-moo2-protected-mouse-position-setting.md)：CONFORMED。受控位置狀態及兩條原檔路徑已驗；下一自然停點為 `INT 2Fh/AX=160Ah`。
- [258 — MOO2 在 DOS 環境查詢 Windows 版本](258-moo2-windows-version-absence.md)：CONFORMED。未安裝 Windows 的限定查詢已驗；兩條原檔路徑下一停點為 `INT 31h/AX=0500h`。
- [259 — DPMI 可用記憶體資訊](259-dpmi-free-memory-information.md)：CONFORMED。配置器一致資訊與兩條原檔路徑已驗；下一停點為記憶體移位 `C1 7D F4 04`。
- [260 — 堆疊 dword 的立即數算術右移](260-cpu386-sar-stack-dword-immediate.md)：CONFORMED。值及定義旗標已核對，AF 差異明示；當時的搜尋停點已由規格 261 接通。
- [261 — MOO2 DOS 搜尋的目前目錄前綴](261-moo2-dos-findfirst-current-directory.md)：CONFORMED。原版缺檔返回已核對，DTA 保留區差異明示；當時的 TEST 停點已由規格 262 接通。
- [262 — 16 位元暫存器間的 TEST](262-cpu386-test-word-register.md)：CONFORMED。暫存器配對、定義旗標與原版第一個分支已核對；當時 VGA 埠停點已由規格 263 接通。
- [263 — VGA DAC 像素遮罩](263-vga-dac-pel-mask.md)：CONFORMED。標準遮罩的硬體規格近似、色彩消費端及兩種保存路徑已驗；當時 byte DIV 停點已由規格 264 接通。
- [264 — byte 暫存器的無號除法](264-cpu386-div-byte-register.md)：CONFORMED。商餘、別名保存與第一個 OUT 已驗，未定義旗標／例外近似明示；當時的 VBE 視窗停點已由規格 265 接通。
- [265 — MOO2 VBE 顯存視窗控制](265-moo2-vbe-window-control.md)：CONFORMED（硬體規格近似）。原版固定使用點、共享顯存映射與消費端已驗，兩條自然路徑實際切換 5–9 並寫入；當時非零顯示停點已由規格 266 接通。
- [266 — MOO2 非零 VBE 顯示起點](266-moo2-vbe-display-start.md)：CONFORMED（硬體規格近似）。原版同次返回與起點消費已驗，當時 word ADD 停點已由規格 267 接通；當次黑圖不代表正常玩家畫面。
- [267 — 16 位元暫存器的帶符號立即值加法](267-cpu386-add-word-register-immediate.md)：CONFORMED。八目的、立即值／旗標與原版 CMP 已驗；JL target 由相同架構測試／Intel 條件支持，原版分支後未擷取；word IMUL 停點已由規格 268 接通。
- [268 — 單運算元的 16 位元有號暫存器乘法](268-cpu386-imul-word-register.md)：CONFORMED，word 有號乘積／CF／OF 與 A3 消費已驗；未定義 ZF 差異明示，dword IMUL 停點已由規格 269 接通。
- [269 — 單運算元的 32 位元有號暫存器乘法](269-cpu386-imul-dword-register.md)：CONFORMED，dword 完整有號積／CF／OF 與 XOR 消費已驗；記憶體 CMP 停點已由規格 270 接通，黑圖仍非正常玩家畫面。
- [270 — 32 位元記憶體目的與暫存器的比較](270-cpu386-cmp-memory-register.md)：CONFORMED，dword 記憶體目的、六旗標與實際 JGE 不取分支已驗；word CMP 停點已由規格 271 接通。
- [271 — 16 位元目的與暫存器的比較](271-cpu386-cmp-word-destination.md)：CONFORMED，word 目的、寬度／六旗標與實際 JL 不取分支已驗；word INC 停點已由規格 272 接通；當次黑圖不代表正常玩家畫面。
- [272 — 16 位元記憶體的遞增](272-cpu386-inc-word-memory.md)：CONFORMED。word 記憶體 INC、CF／五旗標與下一 A1 已驗；自然路徑顯示 Simtex 標誌；短 JS 停點已由規格 273 接通，仍未進入主選單。
- [273 — 短距離負號與非負號分支](273-cpu386-short-sign-branches.md)：CONFORMED。JS／JNS 的完整條件／位移／保持狀態及有限原版分支已驗；步數上限擷取已修正，word XOR 停點已由規格 274 接通，仍未進入主選單。
- [274 — 16 位元暫存器間的 XOR](274-cpu386-xor-word-register.md)：CONFORMED（有限 CPU／下一段載入）。原版 word XOR 高半部／定義旗標與 MOV ES 消費、全套及兩條自然路徑已驗；VTD 空入口停點已由規格 275 接通。
- [275 — MOO2 未安裝 VTD 的裝置入口查詢](275-moo2-protected-vtd-entry-query.md)：CONFORMED（限定未安裝 VTD 的平台查詢）。完整返回保持、原版 record 保存／讀取及兩條自然路徑已驗；PIT 模式 2 停點已由規格 276 接通。
- [276 — PIT 通道 0 的模式 2 設定與共享週期](276-pit0-mode2-shared-clock.md)：CONFORMED（限定設定與週期近似）。模式 2／3 設定已驗；277／281 已閉合原版返回與模式 2 等待，count latch 已由 282／283 接通。
- [277 — MOO2 DOS/4GW 保護模式向量與 IRQ0 派送](277-moo2-dos4gw-protected-irq0.md)：CONFORMED（受限向量／IRQ0／模式 2 等待）。281 兩個自然排程自行完成 1,595 次返回、等待值變化與退出；完整核心／玩家路徑未驗。
- [278 — CS 記憶體目的與帶符號 imm8 的比較](278-cpu386-cs-memory-cmp-imm8.md)：CONFORMED（有限 CPU 比較）。原版樣本與全部 CPU／固定 EXE 回歸通過；後續返回／等待範圍見 277／281。
- [279 — 從 CS 絕對 word 位址載入 ES](279-cpu386-cs-absolute-es-load.md)：CONFORMED（限定 CPU／既有 selector 模型）。有限原版與全部 CPU／固定 EXE 回歸通過；後續 CB／結束鏈見 280／281。
- [280 — 32 位元同權限遠返回 CB](280-cpu386-far-ret32.md)：CONFORMED（限定同權限 CPU 返回）。有限原版框架及回歸通過；預設核心鏈停點已由 281 接通。
- [281 — MOO2 保護模式 IRQ0 的預設結束鏈](281-moo2-protected-irq0-end-chain.md)：CONFORMED（受限預設入口／BIOS tick／EOI）。1,595 次自然返回及模式 2 等待閉合；PIT 鎖存停點已由 282／283 接通，玩家路徑未驗。
- [282 — PIT 通道 0 模式 2 的計數鎖存](282-pit0-mode2-count-latch.md)：CONFORMED（受限模式 2 latch／低高讀取）。共享時鐘／凍結與拒絕測試通過；283 已讓原版自然消費兩次 IN，硬體時序仍近似。
- [283 — 從立即埠編號輸入 byte 至 AL](283-cpu386-in-al-imm8.md)：CONFORMED（裸 E4／明示平台 I/O）。全部 CPU／固定 EXE 回歸及兩個自然排程已驗，讀取凍結 PIT count；SUB 後續停點由 284 接通，主選單未驗。
- [284 — byte 記憶體目的與 imm8 的 SUB](284-cpu386-sub-byte-memory-imm8.md)：CONFORMED。原版自然 byte 16h→0Eh／0Dh→05h、完整 R／段與六旗標已驗；ADD 後續停點由 285 接通。

- [285 — byte 記憶體目的與 imm8 的 ADD](285-cpu386-add-byte-memory-imm8.md)：CONFORMED。原版自然 byte=03h→1Bh／flags=297h→206h 及完整 R／段已驗；XOR 後續停點由 286 接通。
- [286 — byte 暫存器目的與 imm8 的 XOR](286-cpu386-xor-byte-register-imm8.md)：CONFORMED。全部 CPU／固定 EXE 及三筆自然 CL／定義旗標後態已驗；AF 保留工具近似，TEST 後續停點由 287 接通。
- [287 — dword 暫存器與 imm32 的 TEST](287-cpu386-test-dword-register-imm32.md)：CONFORMED。全部 CPU／固定 EXE、自然完整 R／段／定義旗標與第一 JNZ 不跳已驗；ROL 後續停點由 288 接通。

- [288：dword 暫存器的立即數左循環移位](288-cpu386-rol-dword-register-imm8.md)：CONFORMED。全部 CPU／固定 EXE 與三筆自然完整後態／CF 已驗；多位 OF 保留只屬工具近似，NEG 後續停點由 289 接通。

- [289：byte 暫存器取負](289-cpu386-neg-byte-register.md)：CONFORMED。全部 CPU／固定 EXE、三筆自然完整後態／六旗標與 MOV 消費已驗；SHL／CL 與 OR 後續停點由 290／291 接通。

- [290：以 CL 計數左移 byte 暫存器](290-cpu386-shl-byte-register-cl.md)：CONFORMED。八 byte 目的／全部 CL 及別名／定義旗標已驗，未定義近似明示；291 已閉合真實 OR consumer。

- [291：記憶體 byte 目的與暫存器的 OR](291-cpu386-or-byte-memory-register.md)：CONFORMED。全部 CPU／固定 EXE 與三組自然 SHL→OR→ADD 完整資料鏈通過；AF 清除為工具近似，REPE SCASD 後續停點由292接通。

- [292：REPE 掃描 dword 與 EAX 比較](292-cpu386-repe-scasd.md)：CONFORMED。全部 CPU／固定 EXE 與兩自然掃描59次／SUB／MOV 完整資料鏈通過；六旗標全定義，IRQ／Error 模型界限明示，XOR／imm8後續停點由293接通。

- [293：dword 暫存器與符號延伸 imm8 的 XOR](293-cpu386-xor-dword-register-imm8.md)：CONFORMED。全部CPU／固定EXE與兩自然符號延伸後態已驗，AF清除近似明示；294閉合真實BSF consumer。

- [294：dword 暫存器的向前位元掃描](294-cpu386-bsf-dword-register.md)：CONFORMED。全部CPU／固定EXE與兩自然XOR→BSF→ADD→word MOV完整資料鏈通過；ZF定義與未定義模型分開驗，09記憶體dword OR停點已由295接通。

- [295：記憶體 dword 目的與暫存器的 OR](295-cpu386-or-dword-memory-register.md)：CONFORMED。全部CPU／固定EXE、兩自然40h→2040h寫回及MOV EAX的完整dword真實消費已驗；AF／逐byte錯誤模型明示，C6h停點由296接通，CPU證據範圍不擴張。

- [296：SB16 C6h 的 8 位元自動初始化 DMA](296-sb16-c6-auto-init-dma.md)：CONFORMED。原版C6 20 FF 07接受／333步返回與MOV／CMP／JZ成功分支、條件DMA時鐘／block與ring已驗；305已驗14次IRQ7／PCM傳輸，完整音訊與人耳仍未知，ROR立即數停點由297接通，保留原有音訊邊界。

- [297：dword 暫存器的立即數右循環移位](297-cpu386-ror-dword-register-imm8.md)：CONFORMED。全部CPU／固定EXE與兩自然兩組完整ROR／CF／MOV AX,DX消費已驗；多位OF模型明示，IRQ0內D1 SHL停點由298接通。

- [298：dword 暫存器的單位左移](298-cpu386-shl-dword-register-one.md)：CONFORMED。全部CPU／固定EXE與兩自然完整SHL／TEST／JZ及兩個dword寫回已驗，AF模型明示；後續ADC停點由300接通，完整IRQ0返回未知。

- [299：C1 dword 單位移的溢位旗標](299-cpu386-c1-dword-single-shift-overflow.md)：CONFORMED。限定公開CPU契約的C1單位OF窄修正，獨立反例／全部CPU與固定EXE通過；未有MOO2自然OF=1同狀態收據，未定義AF／多位OF模型保持。

- [300：dword 暫存器的帶進位加法](300-cpu386-adc-dword-register.md)：CONFORMED。全部CPU／固定EXE與兩自然三組完整ADC／六旗標、索引0／1的dword真實ADD消費已驗；word XOR停點由301接通並驗此IRQ0返回；XCHG停點由302接通；自然到50M上限與星空片段，正常玩家路徑未知。

- [301：word 暫存器與符號延伸 imm8 的 XOR](301-cpu386-xor-word-register-imm8.md)：CONFORMED。全部CPU／固定EXE與兩自然完整XOR／PUSH EDI、PUSH EAX的兩個dword真實寫入已驗；IRQ0返回，XCHG停點由302接通；自然到50M上限與星空片段，正常玩家路徑仍未驗收。

- [302：AX 與 word 暫存器的短編碼交換](302-cpu386-xchg-ax-word-register.md)：CONFORMED。全部CPU／固定EXE、兩自然三組完整交換／高16位及全部旗標保持與下一ROR完整消費已驗；自然到50M上限無CPU拒絕，已見星空片段，主選單／正常玩家路徑未驗。

- [303：晚期啟動的平台狀態與正常輸入入口](303-moo2-late-startup-platform-observation.md)：DRAFT。90b9f4a基線兩自然各11快照確認時計1375未前進；時間缺口由304接線並驗首block，IRQ7由305閉合14次轉送；Esc IRQ1由307接線，其他鍵盤／等待因果與正常玩家路徑仍未知。

- [304：兩種 CPU 模式共用的裝置時間](304-le-shared-device-clock.md)：CONFORMED。共用時計／首block已驗，實際IVT1201:0682的IRQ7由305閉合14次轉送；完整音訊與人耳仍未知。

- [305：MOO2 保護模式 IRQ7 的實模式轉送](305-moo2-irq7-real-mode-passdown.md)：CONFORMED。機器層／固定EXE全套及兩自然14次原版73步、EOI／22E／IRET返回與PCM29175已驗，XOR AL,1由306接通；正常玩家流程仍未知。

- [306：AL 與立即 byte 的 XOR](306-cpu386-xor-al-imm8.md)：CONFORMED。全部CPU／固定EXE與兩自然三組AL／旗標、RET及MOV ESI,EAX消費已驗；50M無拒絕、386次IRQ7返回與星空片段，正常Esc IRQ1由307接線，完整鍵盤／玩家流程仍未知。

- [307：MOO2 硬體鍵盤與保護模式 IRQ1](307-moo2-protected-keyboard-irq1.md)：CONFORMED。限定正常controller Esc 01／81、97／77步原版IRQ1返回及caller續行；後續D0／D4由309接線，完整鍵盤／主選單仍未知。

- [308：dword 指標的間接遠呼叫](308-cpu386-call-far-indirect-absolute.md)：CONFORMED。裸FF 1D真實六byte指標／8byte寫回與CB消費、全套與兩自然診斷已驗；完整CPU權限／gate未建模。

- [309：DSP 8 位元 DMA 暫停與恢復](309-sb16-pause-resume-dma8.md)：CONFORMED。正常Esc後真正D0／102步原版IRQ7返回、450096µs暫停來源保持與D4／成功caller／全套已驗；idle／硬體時序／人耳及完整玩家路徑仍未知。

- [310：DOS AH2Ah 與可重播日曆](310-moo2-dos-calendar-date.md)：CONFORMED。明示epoch／兩正常日期與原始caller寫回、未設定拒絕與全套已驗；AH2C／RNG／真實系統日期及玩家流程未知。

- [311：word SUB 完整立即值](311-cpu386-sub-word-register-imm16.md)：CONFORMED。八目的／全word／六旗標／全套與兩正常各兩筆SUB、MOV／SHL／word MOV及兩次堆疊寫回已驗；memory與完整CPU未擴張。

- [312：word ADD 的記憶體來源](312-cpu386-add-word-memory-source.md)：CONFORMED。word記憶體來源／獨立六旗標／全部測試與兩正常ADD、真正兩byte寫回及四caller已驗；已見Loading畫面，主選單未知。

- [313：word CMP 完整立即值](313-cpu386-cmp-word-register-imm16.md)：CONFORMED。八來源／全word／全套、兩正常三組CMP旗標及JL跳轉／不跳轉已驗；已見標題背景，AX0014h續見314，完整主選單未知。

- [314：保護模式滑鼠回呼交換](314-moo2-protected-mouse-callback-exchange.md)：CONFORMED。24次正常交換／三次C3返回、全套與三流程通過；兩組到50M，主選單部分滑入，完整操作未驗。

- [315：主選單滑入階段觀測](315-moo2-menu-slide-observation.md)：CONFORMED。三原版29張唯讀快照／全部基線不突變，已證實畫面持續改變；完整操作未驗，保持50M上限。

- [316：明示硬體Esc排程](316-moo2-configured-hardware-escape-schedule.md)：CONFORMED。11設定閘門／四原版、46M六按鈕文字已見／48M逐列保持；點擊未驗，保持50M上限。

- [317：第40換頁唯讀觀測](317-moo2-menu-display40-observation.md)：CONFORMED。第49882420步／63906833µs六按鈕全見；316全部舊列保持，剩117580步，點擊未驗。

- [318：NEW GAME正常點擊](318-moo2-new-game-normal-click.md)：CONFORMED。兩真正CB／完整R、六段、flags恢復；模式關閉基線保持，設定畫面與按鍵consumer未驗。

- [319：NEW GAME按鍵消費觀測](319-moo2-new-game-button-consumer.md)：CONFORMED。限定兩CB後word0100／0000與318全部列保持；讀取覆蓋無正對照，consumer仍未知。

- [320：讀取掛勾正對照](320-moo2-button-read-hook-control.md)：CONFORMED。兩原版word MOV讀500／229座標，掛勾匹配／319全部列保持；後續事件消費見321，320原始限定收據保留。

- [321：主選單滑鼠事件消費](321-moo2-menu-mouse-event-consumer.md)：CONFORMED。7正常讀取／28續行／2CB window，放開後讀取並清事件；320其他列與終圖保持，設定畫面仍未知。

- [322：44M Esc正常續行](322-moo2-earlier-escape-new-game-continuation.md)：CONFORMED。八CLI／兩44M正常流程，真實save?.gam搜尋在47995790拒絕；保持46M獨立基線，設定畫面未知。

- [323：DOS問號首次搜尋](323-moo2-dos-findfirst-question-pattern.md)：CONFORMED。真實save?.gam缺檔／EAX12h／CF與12步消費通過，兩原版流程及固定EXE完整測試通過；byte ADD後續見324，設定畫面未知。

- [324：byte ADD目的](324-cpu386-add-byte-register-memory.md)：CONFORMED。1暫存器／7記憶體ADD與24續行、JGE兩方向與全套通過；記憶體NEG後續見325，設定畫面未知。

- [325：32位記憶體NEG](325-cpu386-neg-dword-memory.md)：CONFORMED。3正常NEG寫回／9步MOV、CMP與JGE不跳、全套與舊4123列保持；同50M流程到上限無新CPU拒絕，後段觀測見326。

- [326：NEW GAME後段進度](326-moo2-post-click-progress-observation.md)：CONFORMED。六時點VBE與像素保持／R、堆疊、RAM有變，完整兩325流程不突變；來源與最小消費見327，後續玩家路徑未知。

- [327：後段實際來源與最小消費](327-moo2-post-click-source-consumer.md)：CONFORMED。六組02h／80h／82h來源與分支、兩次原值FDh寫回，90步獨立核對；跨次RAM／70步未驗；後段發布觀測見328，不改輸入或CPU。

- [328：目的RAM與VBE發布監測](328-moo2-post-click-publish-monitor.md)：CONFORMED。後段500000步、兩目標讀回0／寫3與5、VBE提交0，源與目的正對照通過；獨立較晚畫面見329，正常開局與完整renderer未知。

- [329：正常NEW GAME的100M有界續行](329-moo2-bounded-new-game-continuation.md)：CONFORMED。14 CLI／50M兩基線與100M前綴保持；六快照與較晚致謝文字變化已見、無新CPU拒絕，NEW GAME激活未知。

- [330：正常事件返回與上層判定](330-moo2-event-return-caller.md)：CONFORMED。兩組32步／三RET／非零JNE與CALL邊界、329三流程及72PNG保持；NEW GAME指令仍未知。

- [331：非零事件分支與正常CALL返回](331-moo2-button-branch-call-return.md)：CONFORMED。96 caller步、五自然返回與第一筆x500>25跳過，原來源／舊基線保持；其餘命中及CALL返回見332／333，caller最終RET與NEW GAME未知。

- [332：同輪其餘範圍與caller返回](332-moo2-button-tail-return.md)：限定CONFORMED。同輪命中全畫面index8；313步來源限制明示，後續CALL209325正常返回已由333接通，NEW GAME仍未知。

- [333：全畫面命中後的原表與正常返回](333-moo2-menu-table-return.md)：限定CONFORMED。47990733正常返回；9筆表更換7筆表，第2筆範圍對應選單位置，NEW GAME尚未實際觸發。

- [334：正式選單就緒後的正常新遊戲點擊](334-moo2-ready-menu-normal-click.md)：限定CONFORMED。正常額外點擊實際選中index2；歷史F2 66 AF拒絕與黑屏由335解除，舊三基線保持。

- [335：REPNE SCASW與正式選單後續](335-cpu386-repne-scasw.md)：限定CONFORMED。原F2 66 AF掃描與單筆MOV／全套／三基線保持，正常設定頁已見，ACCEPT由336接通，完整開局未驗。

- [336：原設定頁按鈕表與ACCEPT正常輸入](336-moo2-setup-accept-normal-click.md)：限定CONFORMED。正常ACCEPT實際index15寫回，SELECT RACE已見；四舊基線與102PNG保持，第7筆正常選族與名稱頁由337接通，完整開局未驗。

- [337：原選族頁的人類正常選擇](337-moo2-race-humans-normal-click.md)：限定CONFORMED。正常index7寫回與Enter Ruler Name已見，五舊基線與132PNG保持；名稱正常確認與旗幟頁由338接通，完整開局未驗。

- [338：統治者名稱頁的正常確認](338-moo2-ruler-name-normal-confirmation.md)：限定CONFORMED。正常名稱放開與旗幟頁已見，六舊基線與162PNG保持；共享store未命中／持久名稱writer及完整開局未知。

- [339：旗幟頁正常紅色輸入與按鍵查詢](339-moo2-banner-red-normal-click.md)：限定CONFORMED。完整99M原表／pressed查詢／正常放開已驗，120M仍停旗幟頁；原GUI按鍵返回由340接通，選色消費、持久旗色與完整開局未知。

- [340：旗幟首次pressed查詢後的原正常GUI消費](340-moo2-banner-pressed-consumer.md)：限定CONFORMED。192原步已證實339第二次查詢與GUI返回0；340原RET返回1才正常放開，120M仍旗幟頁；後段按鍵返回由341接通，原選色未驗。

- [341：旗幟GUI按鍵返回後的原正常分支](341-moo2-banner-after-gui-return.md)：限定CONFORMED。兩個獨立192步／原20E165返回1與正常放開已驗；當輪184694的85 82拒絕並全黑；該CPU缺口由342接通，正式writer與完整開局未驗。

- [342：記憶體dword與暫存器的TEST](342-cpu386-test-dword-memory.md)：限定CONFORMED。通用唯讀TEST／原SETNE及RET／正常宇宙生成圖已驗，全套通過；當輪17D536的0F9E拒絕由343接通，後續SETG由344接通，120M完整開局未知。

- [343：byte暫存器的SETLE](343-cpu386-setle-byte-register.md)：限定CONFORMED。原SETLE AL0與下一SS byte真實write／固定EXE全套通過；當輪17D5A0的0F9F拒絕由344接通，120M完整開局未知。

- [344：完整標準SETcc暫存器條件](344-cpu386-setcc-byte-register.md)：限定CONFORMED。標準16個暫存器條件與原SETG／MOV兩步、固定EXE全套已驗；同120M無新CPU拒絕仍宇宙生成圖，完整開局未知。

- [345：宇宙生成迴圈的有界進度與正常返回](345-moo2-universe-loop-progress.md)：限定CONFORMED。576原步／兩RET已驗、8503原列與32PNG保持，原120M第三例pending；其較晚RET與160M續行由346接通，完整生成未知。

- [346：固定160M正常宇宙生成續行](346-moo2-universe-160m-normal-continuation.md)：限定CONFORMED。120M完整同狀態／原基準保持，第三RET120083995；160M已進入Placing home worlds...，原文字來源與caller由347接通，完整開局未驗。

- [347：配置母星文字來源與原正常消費](347-moo2-home-worlds-text-source.md)：限定CONFORMED。原索引242／161查詢、NUL複製與caller16B985已驗，10523原列／36PNG及關閉8M啟動基準保持，完整開局未知。

- [348：配置母星進度函式返回與上層原分支](348-moo2-home-worlds-return.md)：限定CONFORMED。153214295原RET／caller word0000與JNZ不跳已驗，10530原列／36PNG及關閉8M保持；母星配置與完整開局未驗。

- [349：後續母星生成呼叫的原入口與等待狀態](349-moo2-generation-entry.md)：限定CONFORMED。153878499原CALL／框架／三直接返回與首距離返回已驗，外層160M pending；10542原列／36PNG及關閉8M保持，完整開局未知。

- [350：後續生成的外層迭代上限與重繪邊界](350-moo2-generation-iteration-bound.md)：限定CONFORMED。首四原SI迭代／signed界限36及JL已驗，10556原列／36PNG與關閉8M保持；原出口／RET已由352回填；完整開局未知。

- [351：後續生成的外層出口、後段與返回邊界](351-moo2-generation-completion-boundary.md)：限定CONFORMED。原SI1..26連續、耗時與160M截斷已驗，10570原列／36PNG與關閉8M保持；出口／後段／RET已由352回填；完整開局未知。

- [352：固定180M正常續行與160M同狀態核對](352-moo2-generation-180m-continuation.md)：限定CONFORMED。原160M同狀態保持，35迭代／出口／AL0與真正RET已驗；163795435原word記憶體AND停止已由353接通；完整開局未驗。

- [353：word記憶體AND與完整立即值](353-cpu386-and-word-memory-imm16.md)：限定CONFORMED。原word0000／AND／SHL／OR三步、10723原前綴／36PNG、CPU全套與關閉8M保持；164321317原memory byte XCHG已由354接通，完整開局未知。

- [354：byte記憶體與暫存器XCHG](354-cpu386-xchg-byte-memory-register.md)：限定CONFORMED。原DS0E／ALFF交換與STOSB兩步、10742原前綴／35frames、CPU全套／8M保持；164560803原byte ADD memory來源已由355接通，完整開局未知。

- [355：byte記憶體來源ADD](355-cpu386-add-byte-memory-source.md)：限定CONFORMED。原四來源ADD與目的ADD零結果／五步旗標、10759原前綴／35frames、全套／8M保持；164561579原0F94 memory目的已由356接通，完整開局未知。

- [356：標準SETcc的byte記憶體目的](356-cpu386-setcc-byte-memory.md)：限定CONFORMED。原SETE SS41→01與下一JMP／flags保持、10764原前綴／35frames、全套／8M通過；164567987原word IMUL已由357接通，後續NEG與完整開局見357。

- [357：立即值IMUL的word目的](357-cpu386-imul-word-immediate.md)：CONFORMED，限定word IMUL／原兩MOV零寫／正常續行，非完整開局。原1CF90A word0000×5／DI低word寫回與兩MOV零寫已驗，10767正常前綴／35frames／全套通過；原164568139 word NEG已由358接通；原SUB／POP／RET已由359接通，原CWD／SUB／SAR已由360接通，原memory ROR／A1已由361接通，存檔錯誤與完整開局見361。

- [358：word暫存器與記憶體NEG](358-cpu386-neg-word.md)：限定CONFORMED。原NEG word0000與下一EB09／六flags、10770正常前綴／35frames／全套保持；第三CMP數值未驗。原164610300在1D0944的word SUB與三POP／RET已由359接通，完整開局未知。

- [359：word暫存器來源SUB](359-cpu386-sub-word-register-source.md)：限定CONFORMED。原SUB word0003-AX0／六flags與三POP／RET、10775正常前綴／35frames／全套保持；原164984957於1D2A33的CWD與SUB／SAR已由360接通，新memory ROR與完整開局未知。

- [360：word CWD符號延伸](360-cpu386-cwd-word.md)：限定CONFORMED。原CWD AX1／DX0與下一SUB／SAR、10789正常前綴／35frames／全套保持；原168496272 memory ROR與A1已由361接通；同180M終圖存檔權限錯誤，完整開局未驗。

- [361：dword記憶體ROR與立即數](361-cpu386-ror-dword-memory-imm8.md)：限定CONFORMED。原ROR000B1818→18000B18／CF0與A1真正load、10949正常前綴／35frames／全套通過；同180M無CPU拒絕，終圖存檔Permission denied，完整開局未驗。

- [362：正常開局DOS存檔權限邊界](362-moo2-save-permission-boundary.md)，CONFORMED限定唯讀拒絕診斷，覆蓋層玩家驗收DRAFT。
