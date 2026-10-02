"""重播固定 MOO2 1.31 的 DOSBox-X 輔助啟動與 DPMI 快照。

容器內需要 /tmp/game/ORION2.EXE、可寫 /shots、DISPLAY 與外層 Xvfb trap。
原版檔案及輸出的完整終端／記憶體資料不可加入公開版控。
--check-xor-al-immediate-spec-backlinks 只驗證34 ib與原版RET／MOV消費回填，不啟動DOSBox-X。
--check-irq7-passdown-spec-backlinks 只驗證IRQ7轉送／返回及較早音訊邊界回填，不啟動DOSBox-X。
--check-shared-device-clock-spec-backlinks 只驗證共用DMA時計及首block／IRQ7邊界回填，不啟動DOSBox-X。
--check-xchg-ax-word-spec-backlinks 只驗證 word XCHG及完整ROR消費回填，不啟動DOSBox-X。
--check-xor-word-immediate-spec-backlinks 只驗證 word XOR與原版堆疊消費回填，不啟動DOSBox-X。
--check-adc-dword-register-spec-backlinks 只驗證 dword ADC真實消費回填，不啟動DOSBox-X。
--check-shl-dword-one-spec-backlinks 只驗證 dword 單位SHL回填，不啟動DOSBox-X。
--check-c1-single-shift-overflow-spec-backlinks 只驗證 C1單位OF契約回填，不啟動DOSBox-X。
--check-ror-dword-immediate-spec-backlinks 只驗證 dword ROR 規格回填，不啟動 DOSBox-X。
--check-sb16-c6-spec-backlinks 只驗證 SB16 C6h 規格回填，不啟動 DOSBox-X。
--check-or-dword-memory-spec-backlinks 只驗證 dword 記憶體 OR 規格回填，不啟動 DOSBox-X。
--check-or-byte-memory-spec-backlinks 只驗證 byte 記憶體 OR 規格回填，不啟動 DOSBox-X。
--check-shl-byte-cl-spec-backlinks 只驗證 byte SHL／CL 規格回填，不啟動 DOSBox-X。
--check-neg-byte-spec-backlinks 只驗證 byte NEG 規格回填，不啟動 DOSBox-X。
--check-rol-dword-immediate-spec-backlinks 只驗證 ROL dword 規格回填，不啟動 DOSBox-X。
--check-test-dword-immediate-spec-backlinks 只驗證 TEST dword 規格回填，不啟動 DOSBox-X。
--check-xor-byte-immediate-spec-backlinks 只驗證 XOR byte 規格回填，不啟動 DOSBox-X。
--check-add-byte-memory-spec-backlinks 只驗證 ADD byte 規格回填，不啟動 DOSBox-X。
--check-sub-byte-memory-spec-backlinks 只驗證 SUB byte 規格回填，不啟動 DOSBox-X。
"""

import hashlib
import json
import os
import pathlib
import pty
import select
import subprocess
import termios
import fcntl
import struct
import time
import re
import sys
import threading

def validate_mouse_sensitivity_resolution(spec_dir):
    """規格 254 的原始定位與規格 230 解析回填必須同時存在。"""
    current = (spec_dir / '254-moo2-protected-mouse-sensitivity-settings.md').read_text()
    older = (spec_dir / '230-moo2-protected-mouse-zero-sensitivity.md').read_text()
    required = ('0180:0038031B', '001Ah', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('滑鼠敏感度新規格缺原始定位或可實作狀態')
    if '非零拒絕邊界已由規格 254 取代' not in older or '254-moo2-protected-mouse-sensitivity-settings.md' not in older:
        raise RuntimeError('滑鼠敏感度舊規格缺勘誤回填')

spec_dir = pathlib.Path(__file__).resolve().parents[3] / 'docs' / 'spec'



def validate_cmp_word_imm16_resolution(spec_dir):
    """313原始CMP與兩方向JL消費須回填全部較早相同停點。"""
    current=(spec_dir/'313-cpu386-cmp-word-register-imm16.md').read_text()
    required=('0x14E3DE','66 81 F9 D4 00','0x14E3E3','0F 8C 45 FF FF FF',
              '0x14E32E','0x14E3E9','48919460','48919797','48992578',
              'CF1／PF1／AF1／ZF0／SF1／OF0','CF0／PF1／AF0／ZF1／SF0／OF0',
              'flags293h→297h','flags293h→246h','SS0188:002BDBA4',
              'total=212 sample_groups=3 boundary212_observed=true',
              'mouse_started=0','mouse_started=1','0x24C31B','AX0014h','61027457',
              '9ecb69d4db8d3e563ecf0437aa6c94ef20bf81cae8616054f3fcb50429380785',
              '024cb32aacccb303f6b58054e277c5a14e989ca79d426f8a25ef02790c40f895',
              '1d9d785ac3189d58aab71527bb80f8335655a4d024e28e72b395f6926241e958',
              '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(v in current for v in required) or not re.search(r'^狀態：\*\*CONFORMED',current,re.M):
        raise RuntimeError('word CMP iw缺原始來源／六旗標／兩方向JL／完整窗口與正式收據')
    for name in ('309-sb16-pause-resume-dma8.md','310-moo2-dos-calendar-date.md','311-cpu386-sub-word-register-imm16.md','312-cpu386-add-word-memory-source.md'):
        older=(spec_dir/name).read_text()
        if 'word CMP完整立即值停點已由規格 313 接通' not in older or '313-cpu386-cmp-word-register-imm16.md' not in older:
            raise RuntimeError('word CMP iw較早拒絕缺回填')

if sys.argv[1:]==['--check-cmp-word-imm16-spec-backlinks']:
    validate_cmp_word_imm16_resolution(spec_dir)
    print('word CMP iw／六旗標／JL兩方向與較早四文件回填通過')
    raise SystemExit(0)

def validate_add_word_memory_source_resolution(spec_dir):
    """312真正word來源／寫回與較早相同停點必須一起保存。"""
    current=(spec_dir/'312-cpu386-add-word-memory-source.md').read_text()
    required=('0x210C7E','66 03 05 A4 BE 29 00','DS0188:0029BEA4',
              'EAX0000000F→00000011','flags202h→216h',
              'CF0／PF1／AF1／ZF0／SF0／OF0','0x210C85','66 A3 A2 BE 29 00',
              'DS0188:0029BEA2','0F00000002000200→0F00110002000200',
              '0x210C8B','0x210C90','58965328',
              'mouse_started=0','mouse_completed=0','mouse_started=1','mouse_completed=1',
              '0x14E3DE','66 81 F9 D4 00','Loading Master of Orion II',
              '35b61654a285c65c619fee17cd49c86720e2fc0def4d405c06552bdfe04d4fc9',
              '32274bc6d48f28dad1567c74ee515e3a7867f4a8c12b2764c830c79abe6ebb14',
              'd2d1475f15c94cccd43c012f98427571475c444548decff883f34a73aa7de904',
              '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(v in current for v in required) or not re.search(r'^狀態：\*\*CONFORMED',current,re.M):
        raise RuntimeError('word ADD來源缺原始定位／flags／寫回／受控初態與正式收據')
    for name in ('309-sb16-pause-resume-dma8.md','310-moo2-dos-calendar-date.md','311-cpu386-sub-word-register-imm16.md'):
        older=(spec_dir/name).read_text()
        if 'word ADD記憶體來源停點已由規格 312 接通' not in older or '312-cpu386-add-word-memory-source.md' not in older:
            raise RuntimeError('word ADD來源較早拒絕缺解析回填')

if sys.argv[1:]==['--check-add-word-memory-source-spec-backlinks']:
    validate_add_word_memory_source_resolution(spec_dir)
    print('word ADD來源／六旗標／真正寫回與較早三文件回填通過')
    raise SystemExit(0)

def validate_dos_calendar_resolution(spec_dir):
    """310明示日曆與原始返回／caller須回填309。"""
    current=(spec_dir/'310-moo2-dos-calendar-date.md').read_text()
    required=('0x240A32','0x240A96','SetCalendarEpoch',
              'DOSGOLEM_MOO2_CALENDAR_EPOCH=1996-01-01','1980','2099',
              '58553364','58553400','01600101','SS:002BDB90','SS:002BDB8C',
              'configured=false','AH2C','RNG','platform-spec approximation',
              '950f0e6690aad7f7546e2dcdcfb8ddd6aeeb4b398aa001c14a483b359d5ba1f8',
              '09b176610ba23b5f6b221564659a3666ed7bd8d2589b2f984a86e3e66ab4bc79',
              '5f736d9fa2d296bc138ba0782212e0b775e78d6a258fb569c1d2dcb57eed2589',
              '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(v in current for v in required) or not re.search(r'^狀態：\*\*CONFORMED',current,re.M):
        raise RuntimeError('日期服務缺明示初態／原始返回／寫回／拒絕／收據與限定範圍')
    older=(spec_dir/'309-sb16-pause-resume-dma8.md').read_text()
    if 'DOS AH2Ah日期停點已由規格 310 接通' not in older or '310-moo2-dos-calendar-date.md' not in older:
        raise RuntimeError('日期服務的309未知缺回填')

if sys.argv[1:]==['--check-dos-calendar-spec-backlinks']:
    validate_dos_calendar_resolution(spec_dir)
    print('明示日曆／日期返回／原始寫回與309回填通過')
    raise SystemExit(0)


def validate_sub_word_imm16_resolution(spec_dir):
    """311原始SUB與完整消費須回填310。"""
    current=(spec_dir/'311-cpu386-sub-word-register-imm16.md').read_text()
    required=('0x240A34','0x240A98','66 81 E9 6C 07','000007CC→00000060',
              'CF0／PF1／AF0／ZF0／SF0／OF0','高16非零',
              'MOV CH,AL','01600101','0x240A41','0x240AA5',
              'SS:002BDB90','SS:002BDB8C','01 01 60 01',
              '兩時計58554306','0x210C7E','AH2C','RNG',
              '950f0e6690aad7f7546e2dcdcfb8ddd6aeeb4b398aa001c14a483b359d5ba1f8',
              '09b176610ba23b5f6b221564659a3666ed7bd8d2589b2f984a86e3e66ab4bc79',
              '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(v in current for v in required) or not re.search(r'^狀態：\*\*CONFORMED',current,re.M):
        raise RuntimeError('word SUB iw缺原始定位／六旗標／完整consumer／寫回／收據與限定範圍')
    older=(spec_dir/'310-moo2-dos-calendar-date.md').read_text()
    if 'word SUB完整立即值停點已由規格 311 接通' not in older or '311-cpu386-sub-word-register-imm16.md' not in older:
        raise RuntimeError('word SUB iw的310拒絕缺回填')

if sys.argv[1:]==['--check-sub-word-imm16-spec-backlinks']:
    validate_sub_word_imm16_resolution(spec_dir)
    print('word SUB iw／六旗標／完整consumer與310回填通過')
    raise SystemExit(0)

def validate_dma8_control_resolution(spec_dir):
    """309正常D0／D4與原版返回須回填較早DSP未知。"""
    current=(spec_dir/'309-sb16-pause-resume-dma8.md').read_text()
    required=('0x217AD8','1201:05DA','OUT022C=D0','1201:0682',
              '102步','450096µs','461100','58507105',
              '0x217ADF','0x2454B0','0x2454E7','0x240A32',
              'hardware-spec approximation','idle','主選單',
              'a33b5da6a93a996cd1cf6653455c39605fc4aaad9e539cf0cefcfce5d6abe28a',
              '07a99506c2c6789cabf3ada4d365431cf977aa37e5509114302bd28708faf25e',
              '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(v in current for v in required) or not re.search(r'^狀態：\*\*CONFORMED',current,re.M):
        raise RuntimeError('DMA8控制缺原始定位／D0保持／真正IRQ7返回／D4 caller／收據與限定範圍')
    for number in (303,305,306,307):
        names=list(spec_dir.glob(str(number)+'-*.md'))
        if len(names)!=1:raise RuntimeError('DMA8控制舊規格不唯一')
        text=names[0].read_text()
        if 'D0 DMA暫停停點已由規格 309 接通' not in text or '309-sb16-pause-resume-dma8.md' not in text:
            raise RuntimeError('DMA8控制舊未知缺原始返回回填')

if sys.argv[1:]==['--check-dma8-control-spec-backlinks']:
    validate_dma8_control_resolution(spec_dir)
    print('D0／D4／原版返回與舊DSP邊界回填通過')
    raise SystemExit(0)

def validate_hardware_keyboard_resolution(spec_dir):
    """307正常controller輸入與原版IRQ1／caller須回填。"""
    current=(spec_dir/'307-moo2-protected-keyboard-irq1.md').read_text()
    required=('0x239833','AH2509','8:21C4D8','48000000','01／81','97／77',
              '8:21C573','OUT022C=D0','0x215880','0x215882','0x215885',
              'hardware-spec approximation','完整鍵盤','主選單',
              '9a8aad65b88d1748431eb8ddfb17733a7a342cf255f4f0147db6a4b49a957753',
              '430349cf9d921862d40f73bce9d2f0e65bb9a36a571ed4b51072ba1688801e59',
              '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(v in current for v in required) or not re.search(r'^狀態：\*\*CONFORMED',current,re.M):
        raise RuntimeError('正常IRQ1缺向量／controller輸入／原始返回／caller／收據與限定範圍')
    for number in (303,305,306):
        names=list(spec_dir.glob(str(number)+'-*.md'))
        if len(names)!=1:raise RuntimeError('IRQ1舊規格不唯一')
        text=names[0].read_text()
        if '正常Esc IRQ1已由規格 307 接線' not in text or '307-moo2-protected-keyboard-irq1.md' not in text:
            raise RuntimeError('IRQ1舊未知缺有限範圍回填')

if sys.argv[1:]==['--check-hardware-keyboard-spec-backlinks']:
    validate_hardware_keyboard_resolution(spec_dir)
    print('正常controller／IRQ1與舊觀測回填通過')
    raise SystemExit(0)

def validate_far_call_indirect_resolution(spec_dir):
    """308原始FF1D／遠指標／寫回及CB消費須回填307診斷。"""
    current=(spec_dir/'308-cpu386-call-far-indirect-absolute.md').read_text()
    required=('0x21C4EE','FF 1D DC 42 2A 00','09 60 32 00 08 01',
              'F4 C4 21 00 08 00 00 00','flags12h','既有CB','同RPL',
              'padding','完整IRQ1',
              'bced307f87cc1c3afb96409fcfaf247627b583ab50f80f2761c1256de3d1ca43',
              '089b1a4ad44da3302dadedaaa610e269778b58b34cb9a8e63d3336566e6166b1',
              '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(v in current for v in required) or not re.search(r'^狀態：\*\*CONFORMED',current,re.M):
        raise RuntimeError('遠CALL缺原始定位／指標／框架／CB消費／收據與限定範圍')
    older=(spec_dir/'307-moo2-protected-keyboard-irq1.md').read_text()
    if 'FF 1D遠呼叫停點已由規格 308 接通' not in older or '308-cpu386-call-far-indirect-absolute.md' not in older:
        raise RuntimeError('遠CALL的307診斷停點缺回填')

if sys.argv[1:]==['--check-far-call-indirect-spec-backlinks']:
    validate_far_call_indirect_resolution(spec_dir)
    print('遠CALL／CB消費與307舊停點回填通過')
    raise SystemExit(0)

def validate_xor_al_immediate_resolution(spec_dir):
    """306裸34、原始AL與真實返回消費須回填305停點。"""
    current = (spec_dir / '306-cpu386-xor-al-imm8.md').read_text()
    required = ('0x247BE1', '34 01 C3', '297h→246h', '202h→202h',
                'AF未定義', '工具近似', '0x231ADF', '0x231AE2', 'MOV ESI,EAX',
                'DF 1A 23 00', '五定義旗標', '正常鍵盤', 'IRQ7 started386／completed386',
                '7a984c68b925623589aa6dae45973ef8431b98b72df65523dbed65eaeb485720',
                'd4526b29dc51b057566604e4538c0adb4c379b06ad005a589e38d59a2a84506e',
                '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*CONFORMED', current, re.M):
        raise RuntimeError('XOR AL立即值缺原始定位／旗標／真正返回消費／收據或限定範圍')
    older = (spec_dir / '305-moo2-irq7-real-mode-passdown.md').read_text()
    if 'XOR AL立即值停點已由規格 306 接通' not in older or '306-cpu386-xor-al-imm8.md' not in older:
        raise RuntimeError('XOR AL立即值的305舊停點缺回填')

if sys.argv[1:] == ['--check-xor-al-immediate-spec-backlinks']:
    validate_xor_al_immediate_resolution(spec_dir)
    print('XOR AL立即值／RET／MOV消費與305舊停點回填通過')
    raise SystemExit(0)

def validate_irq7_passdown_resolution(spec_dir):
    """305實際IVT、來源確認／返回與限定範圍須同步回填。"""
    current = (spec_dir / '305-moo2-irq7-real-mode-passdown.md').read_text()
    required = ('0x257FC9', '1201:0682', '0x12692', 'OUT0020=20', 'IN022E=00',
                'IRET實模式1201:0729', '73條', '44032151=44032078+73',
                'PCM29175／DMACompletions14／信用60600', 'IRQ7 started14／completed14',
                '0x247BE1', '34 01 C3', '正常鍵盤IRQ1', '平台近似',
                '47169dd3acc1613268ffa4a3cef6121c7b3a00009bd7b7388795a16d57fefbdf',
                'c4e9db17c3b29249867db6eb41eac9bc2744cd5f09e0e21ba122d12718e3f8a1',
                '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*CONFORMED', current, re.M):
        raise RuntimeError('IRQ7轉送缺實際IVT／來源確認／返回／收據或限定範圍')
    for number in (241, 293, 294, 295, 296, 297, 298, 300, 301, 302, 303, 304):
        names = list(spec_dir.glob(str(number) + '-*.md'))
        if len(names) != 1:
            raise RuntimeError('IRQ7較早音訊規格缺檔或定位不唯一')
        older = names[0].read_text()
        if '保護模式IRQ7轉送由規格 305 接線' not in older or '305-moo2-irq7-real-mode-passdown.md' not in older:
            raise RuntimeError('IRQ7較早音訊邊界缺回填')

if sys.argv[1:] == ['--check-irq7-passdown-spec-backlinks']:
    validate_irq7_passdown_resolution(spec_dir)
    print('IRQ7實際轉送／返回／限定範圍與十二份舊規格回填通過')
    raise SystemExit(0)

def validate_shared_device_clock_resolution(spec_dir):
    """304共用時計、真正首block與未完成IRQ7邊界須同步回填。"""
    current = (spec_dir / '304-le-shared-device-clock.md').read_text()
    required = ('0x2454AE', 'C6 20 FF 07', '43985659', '44032078',
                '46419×44100', 'DMACompletions1／PCMBytes2048', 'DMA8SampleCredit4000',
                '88ed1a04cb43fe65827d1cd9ef6d24a736108730b1ce6315d4d3ca79b6a0d140',
                '1201:0682', '0x12692', 'DOS保護模式0F=0000:00000000',
                'DSPIRQPending／PICPending=true', 'IRQ7Deliveries仍1',
                '不重跑未變平台／CPU測試', '保護模式IRQ7派送、連續PCM及人耳仍未知',
                'ec4abf4f565e6cf1ea3ec1b210e414b459d00a6ac8e80da0307e3c323dcb3de0',
                'd89716bc0d1482670ea1eb5cf1709475ef31bbeff0930cb86af34ef407369fda',
                '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('共用裝置時間缺原版定位／首block／真正PCM／pending／未知邊界或可實作狀態')
    for number in (241, 293, 294, 295, 296, 297, 298, 300, 301, 302, 303):
        names = list(spec_dir.glob(str(number) + '-*.md'))
        if len(names) != 1:
            raise RuntimeError('共用時計舊規格缺檔或定位不唯一')
        older = names[0].read_text()
        if '保護模式裝置時計缺口由規格 304 接線' not in older or '304-le-shared-device-clock.md' not in older:
            raise RuntimeError('共用時計較早音訊邊界缺回填')

if sys.argv[1:] == ['--check-shared-device-clock-spec-backlinks']:
    validate_shared_device_clock_resolution(spec_dir)
    print('共用裝置時間／真正首block／IRQ7邊界與十一份舊規格回填通過')
    raise SystemExit(0)

def validate_pit_count_latch_resolution(spec_dir):
    """282 的計數契約與原有未知讀取／鎖存停點回填必須並存。"""
    current = (spec_dir / '282-pit0-mode2-count-latch.md').read_text()
    required = ('0x239B3A', 'E6 43 EB 00 E4 40', '5966', '264×1000000', '5681', '1631h', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('PIT latch 缺原始定位／計數契約或可實作狀態')
    for name, marker in (
        ('281-moo2-protected-irq0-end-chain.md', '計數鎖存停點已由規格 282 接通'),
        ('276-pit0-mode2-shared-clock.md', '模式 2 計數鎖存由規格 282 擴充'),
    ):
        older = (spec_dir / name).read_text()
        if marker not in older or '282-pit0-mode2-count-latch.md' not in older:
            raise RuntimeError('PIT 舊停點／拒絕範圍缺後續回填')

if sys.argv[1:] == ['--check-pit-count-latch-spec-backlinks']:
    validate_pit_count_latch_resolution(spec_dir)
    print('PIT latch 原始定位／契約與後續回填通過')
    raise SystemExit(0)

def validate_xchg_ax_word_resolution(spec_dir):
    """302 的兩高word／全部旗標保持及真正ROR消費須回填舊停點。"""
    current = (spec_dir / '302-cpu386-xchg-ax-word-register.md').read_text()
    required = ('0x256171', '66 93', '完整EAX=0A0A0A0Ah', '完整EBX=2E0A0A2Eh',
                'ROR完整EBX=2E2E0A0Ah', 'flags206h', '全部旗標保持',
                'step_limit=50000000 eip=0x22FCD2', 'started7789／completed7789',
                '72e5ee4954b0b4616e32b3b3844b6ee8836b210c0746cd4b8a20c0b9b5113326',
                'fe3472fa0d7766a761b3b5f7cc5fab8cc22a296a16d0e947e3a093a62826be87',
                'ae21d6b88b831f10addae20471effd45d0ce0e163de3d9067d0840829f4f44e9',
                '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*CONFORMED', current, re.M):
        raise RuntimeError('word XCHG缺定位／兩高word與全部旗標保持／真正ROR消費或上限收據')
    for name in ('293-cpu386-xor-dword-register-imm8.md',
                 '294-cpu386-bsf-dword-register.md',
                 '295-cpu386-or-dword-memory-register.md',
                 '296-sb16-c6-auto-init-dma.md',
                 '297-cpu386-ror-dword-register-imm8.md',
                 '298-cpu386-shl-dword-register-one.md',
                 '299-cpu386-c1-dword-single-shift-overflow.md',
                 '300-cpu386-adc-dword-register.md',
                 '301-cpu386-xor-word-register-imm8.md'):
        older = (spec_dir / name).read_text()
        if 'word XCHG停點已由規格 302 接通' not in older or '302-cpu386-xchg-ax-word-register.md' not in older:
            raise RuntimeError('word XCHG舊停點缺後續回填')

if sys.argv[1:] == ['--check-xchg-ax-word-spec-backlinks']:
    validate_xchg_ax_word_resolution(spec_dir)
    print('word XCHG／完整ROR消費與50M上限及舊停點回填通過')
    raise SystemExit(0)

def validate_xor_word_immediate_resolution(spec_dir):
    """301 的五旗標／AF模型與真正PUSH寫入須回填舊word XOR停點。"""
    current = (spec_dir / '301-cpu386-xor-word-register-imm8.md').read_text()
    required = ('0x24678C', '66 83 F7 01', '完整EDI=00000001h', 'flags2',
                'SS:002723FC dword=1', 'SS:002723F8 dword=00325048h',
                'AF未定義', 'started6228／completed6228',
                '37e637105219c22d92065f7c173b716667f43f91c525cc5a5bdcd2722bd6d39b',
                'ccd184dd64aa984b77e21b6624fca6a7407c95b938dfa959203af32ef0901c13',
                '339eb37ec3be286d6e42b95e7f87d8913ad9423cbf381052a77993fc3025b27b',
                '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*CONFORMED', current, re.M):
        raise RuntimeError('word XOR缺定位／五旗標／AF模型／真正兩個stack dword寫入或IRQ0返回')
    for name in ('293-cpu386-xor-dword-register-imm8.md',
                 '294-cpu386-bsf-dword-register.md',
                 '295-cpu386-or-dword-memory-register.md',
                 '296-sb16-c6-auto-init-dma.md',
                 '297-cpu386-ror-dword-register-imm8.md',
                 '298-cpu386-shl-dword-register-one.md',
                 '299-cpu386-c1-dword-single-shift-overflow.md',
                 '300-cpu386-adc-dword-register.md'):
        older = (spec_dir / name).read_text()
        if 'word XOR立即數停點已由規格 301 接通' not in older or '301-cpu386-xor-word-register-imm8.md' not in older:
            raise RuntimeError('word XOR舊停點缺後續回填')

if sys.argv[1:] == ['--check-xor-word-immediate-spec-backlinks']:
    validate_xor_word_immediate_resolution(spec_dir)
    print('word XOR／真正兩個stack dword寫入與IRQ0返回及舊停點回填通過')
    raise SystemExit(0)

def validate_adc_dword_register_resolution(spec_dir):
    """300 的六定義旗標與真正索引ADD消費須回填舊IRQ0停點。"""
    current = (spec_dir / '300-cpu386-adc-dword-register.md').read_text()
    required = ('0x25179F', '13 ED', 'flags847h', 'EBP=1', 'flags2',
                'DS:00272D44 dword=2', '完整ESI=0071E1D2h', '六算術旗標皆定義',
                'b7407d43ddaf3eb8d20732acd64f2c7cfb3933924615b29cf7e7e32c7c99c021',
                '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('ADC缺完整原始定位／六旗標／真正索引ADD來源與結果')
    for name in ('293-cpu386-xor-dword-register-imm8.md',
                 '294-cpu386-bsf-dword-register.md',
                 '295-cpu386-or-dword-memory-register.md',
                 '296-sb16-c6-auto-init-dma.md',
                 '297-cpu386-ror-dword-register-imm8.md',
                 '298-cpu386-shl-dword-register-one.md',
                 '299-cpu386-c1-dword-single-shift-overflow.md'):
        older = (spec_dir / name).read_text()
        if 'dword ADC停點已由規格 300 接通' not in older or '300-cpu386-adc-dword-register.md' not in older:
            raise RuntimeError('ADC舊IRQ0停點缺後續回填')

if sys.argv[1:] == ['--check-adc-dword-register-spec-backlinks']:
    validate_adc_dword_register_resolution(spec_dir)
    print('ADC六旗標／真正索引ADD消費與舊停點回填通過')
    raise SystemExit(0)

def validate_shl_dword_one_resolution(spec_dir):
    """298 的IRQ0完整前後態與真正兩個dword寫回須回填舊停點。"""
    current = (spec_dir / '298-cpu386-shl-dword-register-one.md').read_text()
    required = ('0x2520B7', 'D1 E0 D1 E3', 'flags46h', '0x2520CB', '0x2520D0',
                'DS:00272D40／DS:00272D44真實寫回', 'AF未定義',
                '7cb0cade0c7b66adc37e01d458f9f22a1a57e2112afa03e62c91417d3a8a2f7c',
                '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('SHL缺完整原始狀態／旗標／真實寫回／未定義模型')
    for name in ('293-cpu386-xor-dword-register-imm8.md',
                 '294-cpu386-bsf-dword-register.md',
                 '295-cpu386-or-dword-memory-register.md',
                 '296-sb16-c6-auto-init-dma.md',
                 '297-cpu386-ror-dword-register-imm8.md'):
        older = (spec_dir / name).read_text()
        if 'dword SHL單位移停點已由規格 298 接通' not in older or '298-cpu386-shl-dword-register-one.md' not in older:
            raise RuntimeError('SHL舊停點缺後續回填')

if sys.argv[1:] == ['--check-shl-dword-one-spec-backlinks']:
    validate_shl_dword_one_resolution(spec_dir)
    print('SHL完整狀態／旗標／真實寫回與舊停點回填通過')
    raise SystemExit(0)

def validate_c1_single_shift_overflow_resolution(spec_dir):
    """299 的CPU反例與公開契約範圍不能包裝成原版動態旗標驗收。"""
    current = (spec_dir / '299-cpu386-c1-dword-single-shift-overflow.md').read_text()
    required = ('C1 E0 01', 'flags603h', 'flagsE03h', '計數0全部保持',
                'C1多位OF目前清除', '沒有MOO2自然OF=1同狀態收據',
                '7c672d84df30bdd041903bc7812b6bb839f000de01222b8af640520eaba52b33',
                '7cb0cade0c7b66adc37e01d458f9f22a1a57e2112afa03e62c91417d3a8a2f7c')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('C1單位OF缺反例／定義與未定義旗標模型／原版限制')
    for name in ('186-fd2-platform-gap-continuation.md', '298-cpu386-shl-dword-register-one.md'):
        older = (spec_dir / name).read_text()
        if '裸C1單位移OF契約由規格 299 補齊' not in older or '299-cpu386-c1-dword-single-shift-overflow.md' not in older:
            raise RuntimeError('C1單位OF缺舊契約或回歸發現回填')

if sys.argv[1:] == ['--check-c1-single-shift-overflow-spec-backlinks']:
    validate_c1_single_shift_overflow_resolution(spec_dir)
    print('C1單位OF公開契約／原版限制與回歸發現回填通過')
    raise SystemExit(0)

def validate_ror_dword_immediate_resolution(spec_dir):
    """297 的全部計數／旗標／真正MOV消費須回填舊範圍與停點。"""
    current = (spec_dir / '297-cpu386-ror-dword-register-imm8.md').read_text()
    required = ('0x257662', 'C1 CA 10', '0AFF0AFFh', 'CF=0', '0x25766A',
                'MOV AX,DX真實消費', '完整EAX=00340AFFh／0A0A0AFFh',
                '多位OF未定義', '零計數全部保持',
                '35ad5bdc11d470d670e044ff4b1894476056930793ade3df2dec7218ca91ca30',
                '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('ROR缺完整初態／計數／旗標邊界／MOV消費或可實作狀態')
    for name, marker in (
        ('222-cpu386-ror-r32-imm8.md', 'dword ROR立即數範圍由規格 297 擴充'),
        ('288-cpu386-rol-dword-register-imm8.md', 'dword ROR立即數範圍由規格 297 擴充'),
        ('293-cpu386-xor-dword-register-imm8.md', 'dword ROR停點已由規格 297 接通'),
        ('294-cpu386-bsf-dword-register.md', 'dword ROR停點已由規格 297 接通'),
        ('295-cpu386-or-dword-memory-register.md', 'dword ROR停點已由規格 297 接通'),
        ('296-sb16-c6-auto-init-dma.md', 'dword ROR停點已由規格 297 接通'),
    ):
        older = (spec_dir / name).read_text()
        if marker not in older or '297-cpu386-ror-dword-register-imm8.md' not in older:
            raise RuntimeError('ROR舊範圍或停點缺後續回填')

if sys.argv[1:] == ['--check-ror-dword-immediate-spec-backlinks']:
    validate_ror_dword_immediate_resolution(spec_dir)
    print('ROR全部計數／旗標／MOV消費與舊範圍回填通過')
    raise SystemExit(0)

def validate_sb16_c6_resolution(spec_dir):
    """296 的命令／返回／成功caller及音訊邊界須回填293／294／295。"""
    current = (spec_dir / '296-sb16-c6-auto-init-dma.md').read_text()
    required = ('0x2454AE', '1201:05D9', 'C6 20 FF 07', 'Returned=true', '333步',
                '0x2454B3', '0x2454B6', '0x2454E7', '46440µs', 'TimeConstant',
                '926100', 'PCMBytes=0', '保護模式連續PCM／IRQ7仍未知',
                'hardware-spec approximation',
                '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('C6h缺原始定位／命令／返回／caller／條件時鐘或音訊邊界')
    for name in ('293-cpu386-xor-dword-register-imm8.md',
                 '294-cpu386-bsf-dword-register.md',
                 '295-cpu386-or-dword-memory-register.md'):
        older = (spec_dir / name).read_text()
        if 'SB16 C6h 停點已由規格 296 接通' not in older or '296-sb16-c6-auto-init-dma.md' not in older:
            raise RuntimeError('C6h舊停點缺後續回填')

if sys.argv[1:] == ['--check-sb16-c6-spec-backlinks']:
    validate_sb16_c6_resolution(spec_dir)
    print('C6h命令／返回／成功caller與音訊邊界回填通過')
    raise SystemExit(0)

def validate_or_dword_memory_resolution(spec_dir):
    """295 的完整OR寫回／真正MOV消費及錯誤模型，須回填293／294停點。"""
    current = (spec_dir / '295-cpu386-or-dword-memory-register.md').read_text()
    required = ('0x23C36B', '09 86 84 03 00 00', 'DS:00325864 dword=00002040h', '0x23C371', '0x23B693', '完整EAX=2040h', 'AF未定義', '逐byte Bus錯誤模型', '647de0701fcdf4c582d561ebc5b8b54b2cb34ff6484b73d9908135e5a17ac1ed', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('OR dword缺完整初態／寫回／真正消費／未定義與錯誤模型或可實作狀態')
    for name in ('293-cpu386-xor-dword-register-imm8.md', '294-cpu386-bsf-dword-register.md'):
        older = (spec_dir / name).read_text()
        if '記憶體 dword OR 停點已由規格 295 接通' not in older or '295-cpu386-or-dword-memory-register.md' not in older:
            raise RuntimeError('OR dword舊停點缺後續回填')

if sys.argv[1:] == ['--check-or-dword-memory-spec-backlinks']:
    validate_or_dword_memory_resolution(spec_dir)
    print('OR dword完整寫回／真正消費與後續回填通過')
    raise SystemExit(0)

def validate_bsf_dword_resolution(spec_dir):
    """294 的完整BSF來源／索引／定義ZF與真實儲存，須回填293缺件。"""
    current = (spec_dir / '294-cpu386-bsf-dword-register.md').read_text()
    required = ('0x2548A2', '0F BC D0', '完整 EDX=Ah', '0x2548A5', '0x2548A7', '0x2548AE', 'DS:002726D0 word=074Ah', '五個未定義旗標保留', '工具模型', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('BSF缺完整來源／索引／真實消費／未定義邊界或可實作狀態')
    older = (spec_dir / '293-cpu386-xor-dword-register-imm8.md').read_text()
    if 'BSF 停點已由規格 294 接通' not in older or '294-cpu386-bsf-dword-register.md' not in older:
        raise RuntimeError('BSF舊停點缺後續回填')

if sys.argv[1:] == ['--check-bsf-dword-spec-backlinks']:
    validate_bsf_dword_resolution(spec_dir)
    print('BSF完整來源／索引／定義ZF／儲存與後續回填通過')
    raise SystemExit(0)

def validate_xor_dword_immediate_resolution(spec_dir):
    """293 的完整符號延伸XOR／AF邊界，須回填292的停點。"""
    current = (spec_dir / '293-cpu386-xor-dword-register-imm8.md').read_text()
    required = ('0x25489C', '83 F0 FF', '0x25489F', '完整 EAX=400h', 'flags=206h', '0x2548A2', 'AF未定義', '工具近似', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('XOR dword缺完整初態／後態／AF邊界或可實作狀態')
    older = (spec_dir / '292-cpu386-repe-scasd.md').read_text()
    if 'dword XOR／imm8 停點已由規格 293 接通' not in older or '293-cpu386-xor-dword-register-imm8.md' not in older:
        raise RuntimeError('XOR dword舊停點缺後續回填')

if sys.argv[1:] == ['--check-xor-dword-imm8-spec-backlinks']:
    validate_xor_dword_immediate_resolution(spec_dir)
    print('XOR dword完整初態／符號延伸／AF與後續回填通過')
    raise SystemExit(0)

def validate_repe_scasd_resolution(spec_dir):
    """292 的掃描／六旗標／真實讀取消費，須和 291 停點回填並存。"""
    current = (spec_dir / '292-cpu386-repe-scasd.md').read_text()
    required = ('0x25488F', 'F3 AF', '0x254891', '完整 ECX=7C5h', 'EDI=6BBD4Ch', 'flags=206h', '0x254894', '0x254896', 'FFFFFBFFh', '六算術旗標全部定義', '單次 Step', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('REPE SCASD 缺完整原始初態／後態／六旗標／模型邊界或可實作狀態')
    older = (spec_dir / '291-cpu386-or-byte-memory-register.md').read_text()
    if 'REPE SCASD 停點已由規格 292 接通' not in older or '292-cpu386-repe-scasd.md' not in older:
        raise RuntimeError('REPE SCASD 舊停點缺後續回填')

if sys.argv[1:] == ['--check-repe-scasd-spec-backlinks']:
    validate_repe_scasd_resolution(spec_dir)
    print('REPE SCASD 完整初態／六旗標／真實讀取與後續回填通過')
    raise SystemExit(0)

def validate_or_byte_memory_resolution(spec_dir):
    """291 的 byte 寫回／AF 邊界，須和 290 的 consumer 缺件回填並存。"""
    current = (spec_dir / '291-cpu386-or-byte-memory-register.md').read_text()
    required = ('0x254A06', '08 2C 17', '0x254A09', 'DS:006BBC60 byte=04h', '0x254A0C', 'AF 未定義', '工具近似', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('byte 記憶體 OR 缺原始初態／寫回／AF 邊界或可實作狀態')
    older = (spec_dir / '290-cpu386-shl-byte-register-cl.md').read_text()
    if 'byte 記憶體 OR 停點已由規格 291 接通' not in older or '291-cpu386-or-byte-memory-register.md' not in older:
        raise RuntimeError('byte 記憶體 OR 舊停點缺後續回填')

if sys.argv[1:] == ['--check-or-byte-memory-spec-backlinks']:
    validate_or_byte_memory_resolution(spec_dir)
    print('byte 記憶體 OR 原始初態／寫回／AF 邊界與後續回填通過')
    raise SystemExit(0)

def validate_shl_byte_cl_resolution(spec_dir):
    """290 的完整 CH／CL 與未定義旗標邊界，須和 289 停點回填並存。"""
    current = (spec_dir / '290-cpu386-shl-byte-register-cl.md').read_text()
    required = ('0x254A04', 'D2 E5', '0x254A06', '完整 ECX=402h', 'CL=2h 保持', 'AF 未定義', 'CF 未定義', '工具近似', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('byte SHL／CL 缺原始初態／後態／未定義邊界或可實作狀態')
    older = (spec_dir / '289-cpu386-neg-byte-register.md').read_text()
    if 'byte SHL／CL 停點已由規格 290 接通' not in older or '290-cpu386-shl-byte-register-cl.md' not in older:
        raise RuntimeError('byte SHL／CL 舊停點缺後續回填')

if sys.argv[1:] == ['--check-shl-byte-cl-spec-backlinks']:
    validate_shl_byte_cl_resolution(spec_dir)
    print('byte SHL／CL 原始初態／完整後態／未定義邊界與後續回填通過')
    raise SystemExit(0)

def validate_neg_byte_resolution(spec_dir):
    """289 的完整 byte NEG／MOV 消費與 288 的停點回填必須並存。"""
    current = (spec_dir / '289-cpu386-neg-byte-register.md').read_text()
    required = ('0x2545EF', 'F6 D9', '0x2545F1', '完整 ECX=FFFFFF0Ah', 'flags=217h', '0x2545F3', '六旗標均定義', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('byte NEG 缺原始初態／後態／六旗標或可實作狀態')
    older = (spec_dir / '288-cpu386-rol-dword-register-imm8.md').read_text()
    if 'byte NEG 停點已由規格 289 接通' not in older or '289-cpu386-neg-byte-register.md' not in older:
        raise RuntimeError('byte NEG 舊停點缺後續回填')

if sys.argv[1:] == ['--check-neg-byte-spec-backlinks']:
    validate_neg_byte_resolution(spec_dir)
    print('byte NEG 原始初態／完整後態／六旗標與後續回填通過')
    raise SystemExit(0)

def validate_rol_dword_immediate_resolution(spec_dir):
    """288 的完整 ROL／未定義 OF 邊界與 287 的停點回填必須並存。"""
    current = (spec_dir / '288-cpu386-rol-dword-register-imm8.md').read_text()
    required = ('0x254510', 'C1 C0 08', '0x254513', '完整 EAX=2h', '計數大於1的 OF 未定義', '工具近似', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('ROL dword 缺原始初態／後態／OF 邊界或可實作狀態')
    older = (spec_dir / '287-cpu386-test-dword-register-imm32.md').read_text()
    if 'dword ROL 停點已由規格 288 接通' not in older or '288-cpu386-rol-dword-register-imm8.md' not in older:
        raise RuntimeError('ROL dword 舊停點缺後續回填')

if sys.argv[1:] == ['--check-rol-dword-immediate-spec-backlinks']:
    validate_rol_dword_immediate_resolution(spec_dir)
    print('ROL dword 原始初態／後態／OF 邊界與後續回填通過')
    raise SystemExit(0)

def validate_test_dword_immediate_resolution(spec_dir):
    """287 的完整 TEST／JNZ 消費與 286 的停點回填必須並存。"""
    current = (spec_dir / '287-cpu386-test-dword-register-imm32.md').read_text()
    required = ('0x254499', 'F7 C1 00 00 00 80', '0x25449F', '0x2544A1', '第 20,651,439 步第一 JNZ 不跳', 'flags=246h', 'AF 列為未定義', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('TEST dword 缺原始初態／第一分支／AF 邊界或可實作狀態')
    older = (spec_dir / '286-cpu386-xor-byte-register-imm8.md').read_text()
    if 'dword 暫存器 TEST 停點已由規格 287 接通' not in older or '287-cpu386-test-dword-register-imm32.md' not in older:
        raise RuntimeError('TEST dword 舊停點缺後續回填')

if sys.argv[1:] == ['--check-test-dword-immediate-spec-backlinks']:
    validate_test_dword_immediate_resolution(spec_dir)
    print('TEST dword 原始初態／第一分支／AF 邊界與後續回填通過')
    raise SystemExit(0)

def validate_xor_byte_immediate_resolution(spec_dir):
    """286 的原始 CL／後態與 285 的停點回填必須並存。"""
    current = (spec_dir / '286-cpu386-xor-byte-register-imm8.md').read_text()
    required = ('0x254275', '80 F1 FF', '0x254278', '6BBCF9h→6BBC06h', 'AF 未定義', '工具模型', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('XOR byte 缺原始 CL／後態／AF 邊界或可實作狀態')
    older = (spec_dir / '285-cpu386-add-byte-memory-imm8.md').read_text()
    if 'byte 暫存器 XOR 停點已由規格 286 接通' not in older or '286-cpu386-xor-byte-register-imm8.md' not in older:
        raise RuntimeError('XOR byte 舊停點缺後續回填')

if sys.argv[1:] == ['--check-xor-byte-immediate-spec-backlinks']:
    validate_xor_byte_immediate_resolution(spec_dir)
    print('XOR byte 原始 CL／後態／AF 邊界與後續回填通過')
    raise SystemExit(0)

def validate_add_byte_memory_resolution(spec_dir):
    """285 的原始 byte／後態與 284 的 CPU 停點回填必須並存。"""
    current = (spec_dir / '285-cpu386-add-byte-memory-imm8.md').read_text()
    required = ('0x25425F', '80 05 C0 26 27 00 18', 'DS:002726C0', 'byte=03h', 'byte=1Bh', '0x254266', 'flags=206h', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('ADD byte 缺原始定位／目的 byte／後態或可實作狀態')
    older = (spec_dir / '284-cpu386-sub-byte-memory-imm8.md').read_text()
    if 'byte 記憶體 ADD 停點已由規格 285 接通' not in older or '285-cpu386-add-byte-memory-imm8.md' not in older:
        raise RuntimeError('ADD byte 舊停點缺後續回填')

if sys.argv[1:] == ['--check-add-byte-memory-spec-backlinks']:
    validate_add_byte_memory_resolution(spec_dir)
    print('ADD byte 原始定位／目的 byte／後態與後續回填通過')
    raise SystemExit(0)

def validate_sub_byte_memory_resolution(spec_dir):
    """284 的原始 byte／後態與 283 的 CPU 停點回填必須並存。"""
    current = (spec_dir / '284-cpu386-sub-byte-memory-imm8.md').read_text()
    required = ('0x254249', '80 2D C0 26 27 00 08', 'DS:002726C0', 'byte=16h', 'byte=0Eh', '0x254250', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('SUB byte 缺原始定位／目的 byte／後態或可實作狀態')
    older = (spec_dir / '283-cpu386-in-al-imm8.md').read_text()
    if 'byte 記憶體 SUB 停點已由規格 284 接通' not in older or '284-cpu386-sub-byte-memory-imm8.md' not in older:
        raise RuntimeError('SUB byte 舊停點缺後續回填')

if sys.argv[1:] == ['--check-sub-byte-memory-spec-backlinks']:
    validate_sub_byte_memory_resolution(spec_dir)
    print('SUB byte 原始定位／目的 byte／後態與後續回填通過')
    raise SystemExit(0)

def validate_in_al_immediate_resolution(spec_dir):
    """283 的 byte 立即埠語意與 282 的 CPU 停點回填必須並存。"""
    current = (spec_dir / '283-cpu386-in-al-imm8.md').read_text()
    required = ('0x239B3E', '0x239B42', 'E4 40 88 C4 E4 40', '補零', 'AL=31h', 'AL=16h', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('E4 缺原始定位／兩次讀取或可實作狀態')
    older = (spec_dir / '282-pit0-mode2-count-latch.md').read_text()
    if '立即埠輸入缺件已由規格 283 接通' not in older or '283-cpu386-in-al-imm8.md' not in older:
        raise RuntimeError('E4 舊停點缺後續回填')

if sys.argv[1:] == ['--check-in-al-immediate-spec-backlinks']:
    validate_in_al_immediate_resolution(spec_dir)
    print('E4 原始定位／兩次讀取與後續回填通過')
    raise SystemExit(0)

def validate_irq0_end_chain_resolution(spec_dir):
    """281 的原始框架與平台收據、舊結束鏈停點回填必須並存。"""
    current = (spec_dir / '281-moo2-protected-irq0-end-chain.md').read_text()
    required = ('0180:00378E9C', '0080:00000D49', '0080:00000F16', '0108:00326008', '16 0F 00 00 80 00 00 00 46 00 00 00', '41 27 02 00 00→42 27 02 00 00', '1,595', '01 00 00 00', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('結束鏈缺原始框架／BDA／自然收據或可實作狀態')
    for name, marker in (
        ('280-cpu386-far-ret32.md', '預設核心鏈停點已由規格 281 接通'),
        ('277-moo2-dos4gw-protected-irq0.md', '預設 DOS08h 結束鏈已由規格 281 接通'),
    ):
        older = (spec_dir / name).read_text()
        if marker not in older or '281-moo2-protected-irq0-end-chain.md' not in older:
            raise RuntimeError('舊結束鏈停點缺後續回填')

if sys.argv[1:] == ['--check-irq0-end-chain-spec-backlinks']:
    validate_irq0_end_chain_resolution(spec_dir)
    print('結束鏈原始框架／BDA與後續回填通過')
    raise SystemExit(0)

def validate_irq0_end_chain_boundary(before, after, bda_before, bda_after):
    """只驗證黑箱邊界的通用暫存器／資料段、堆疊消費及 BIOS tick。"""
    if after[2:13] != before[2:13] or int(after[14], 16) != int(before[14], 16) + 12:
        raise RuntimeError('結束鏈邊界暫存器／資料段或框架消費不符')
    tick = (int.from_bytes(bda_before[:4], 'little') + 1) & 0xffffffff
    midnight = bda_before[4]
    if tick >= 0x1800b0:
        tick = 0
        midnight = (midnight + 1) & 0xff
    if bda_after != tick.to_bytes(4, 'little') + bytes([midnight]):
        raise RuntimeError('結束鏈 BDA 計數／午夜回繞不符')

def validate_far_ret_resolution(spec_dir):
    """280 的原始框架與 279／277 的後續停點回填必須並存。"""
    current = (spec_dir / '280-cpu386-far-ret32.md').read_text()
    required = ('0180:00378E9C', '0x244E9C', '49 0D 00 00 80 00 00 00', '0080:00000D49', '0108:00326008', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('CB 規格缺原始定位／框架或可實作狀態')
    for name, marker in (
        ('279-cpu386-cs-absolute-es-load.md', 'CB 遠返回停點已由規格 280 接通'),
        ('277-moo2-dos4gw-protected-irq0.md', 'CB 缺件已由規格 280 接通，預設核心鏈仍未建模'),
    ):
        older = (spec_dir / name).read_text()
        if marker not in older or '280-cpu386-far-ret32.md' not in older:
            raise RuntimeError('CB 舊停點／返回範圍缺後續回填')

if sys.argv[1:] == ['--check-far-ret-spec-backlinks']:
    validate_far_ret_resolution(spec_dir)
    print('CB 原始框架與後續回填通過')
    raise SystemExit(0)

def validate_cs_es_load_resolution(spec_dir):
    """279 的 ES 定位與 278／256 的範圍回填必須並存。"""
    current = (spec_dir / '279-cpu386-cs-absolute-es-load.md').read_text()
    required = ('0180:00378DBA', '00378DC2', '0x244DBA', '66 2E 8E 05 06 1A 27 00', '0039FA06', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('CS ES 規格缺原始定位或可實作狀態')
    for name, marker in (
        ('278-cpu386-cs-memory-cmp-imm8.md', 'CS word MOV ES 停點已由規格 279 接通'),
        ('256-cpu386-cs-absolute-ds-load.md', 'ES 形狀由規格 279 擴充'),
    ):
        older = (spec_dir / name).read_text()
        if marker not in older or '279-cpu386-cs-absolute-es-load.md' not in older:
            raise RuntimeError('CS ES 舊停點／範圍缺後續回填')

if sys.argv[1:] == ['--check-cs-es-load-spec-backlinks']:
    validate_cs_es_load_resolution(spec_dir)
    print('CS ES 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_cs_memory_cmp_resolution(spec_dir):
    """278 的原始比較定位與 277 的舊 CPU 停點回填必須並存。"""
    current = (spec_dir / '278-cpu386-cs-memory-cmp-imm8.md').read_text()
    required = ('0180:00378D9A', '0180:00378DA2', '0x244D9A', '2E 83 3D FE 19 27 00 00', '0039F9FE', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('CS CMP 規格缺原始定位或可實作狀態')
    older = (spec_dir / '277-moo2-dos4gw-protected-irq0.md').read_text()
    if 'CS 記憶體比較停點已由規格 278 接通' not in older or '278-cpu386-cs-memory-cmp-imm8.md' not in older:
        raise RuntimeError('CS CMP 舊停點缺後續回填')

if sys.argv[1:] == ['--check-cs-memory-cmp-spec-backlinks']:
    validate_cs_memory_cmp_resolution(spec_dir)
    print('CS CMP 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_protected_irq0_resolution(spec_dir):
    """277 的原始向量／框架與 276 舊派送缺口回填必須並存。"""
    current = (spec_dir / '277-moo2-dos4gw-protected-irq0.md').read_text()
    required = ('0180:0037901B', '0180:00379048', '0180:00378D9A', '0x24501B', '0x245048', '0x244D9A', 'CD 21', '00D0:00006838', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('IRQ0 規格缺原始向量／框架或可實作狀態')
    older = (spec_dir / '276-pit0-mode2-shared-clock.md').read_text()
    if 'DOS08h 派送缺口已由規格 277 接線（原版返回仍未閉合）' not in older or '277-moo2-dos4gw-protected-irq0.md' not in older:
        raise RuntimeError('IRQ0 舊派送缺口缺後續回填')

if sys.argv[1:] == ['--check-protected-irq0-spec-backlinks']:
    validate_protected_irq0_resolution(spec_dir)
    print('IRQ0 原始向量／框架與後續回填通過')
    raise SystemExit(0)

def validate_pit_mode2_resolution(spec_dir):
    """固定 PIT 模式 2 定位、舊停點與模式 3 邊界訂正必須同時存在。"""
    current = (spec_dir / '276-pit0-mode2-shared-clock.md').read_text()
    required = ('0180:0036DAE8', '0x239AE8', 'E6 43', '34h', '5966', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('PIT 模式 2 規格缺固定原始定位或可實作狀態')
    for name, marker in (
        ('275-moo2-protected-vtd-entry-query.md', 'PIT 模式 2 停點已由規格 276 接通'),
        ('186-fd2-platform-gap-continuation.md', 'PIT 模式 2 與非法重載邊界由規格 276 補齊'),
    ):
        older = (spec_dir / name).read_text()
        if marker not in older or '276-pit0-mode2-shared-clock.md' not in older:
            raise RuntimeError('PIT 既有規格缺後續回填')

if sys.argv[1:] == ['--check-pit-mode2-spec-backlinks']:
    validate_pit_mode2_resolution(spec_dir)
    print('PIT 模式 2 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_vtd_entry_resolution(spec_dir):
    """固定 VTD 查詢定位與舊停點回填必須同時存在。"""
    current = (spec_dir / '275-moo2-protected-vtd-entry-query.md').read_text()
    older = (spec_dir / '274-cpu386-xor-word-register.md').read_text()
    required = ('0180:0036DA47', '0x239A47', 'CD 2F', '0005h', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('VTD 查詢規格缺固定原始定位或可實作狀態')
    if 'VTD 空入口停點已由規格 275 接通' not in older or '275-moo2-protected-vtd-entry-query.md' not in older:
        raise RuntimeError('VTD 舊停點缺後續回填')

if sys.argv[1:] == ['--check-vtd-entry-spec-backlinks']:
    validate_vtd_entry_resolution(spec_dir)
    print('VTD 查詢原始定位與後續回填通過')
    raise SystemExit(0)

def validate_xor_word_register_resolution(spec_dir):
    """固定 word XOR 定位與舊停點回填必須同時存在。"""
    current = (spec_dir / '274-cpu386-xor-word-register.md').read_text()
    older = (spec_dir / '273-cpu386-short-sign-branches.md').read_text()
    required = ('0180:0036DA42', '0x239A42', '66 31 FF', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('word XOR 規格缺固定原始定位或可實作狀態')
    if 'word XOR 停點已由規格 274 接通' not in older or '274-cpu386-xor-word-register.md' not in older:
        raise RuntimeError('word XOR 舊停點缺後續回填')

if sys.argv[1:] == ['--check-xor-word-register-spec-backlinks']:
    validate_xor_word_register_resolution(spec_dir)
    print('word XOR 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_short_sign_branches_resolution(spec_dir):
    """固定短符號分支定位與舊停點回填必須同時存在。"""
    current = (spec_dir / '273-cpu386-short-sign-branches.md').read_text()
    older = (spec_dir / '272-cpu386-inc-word-memory.md').read_text()
    required = ('0180:003502D6', '0x21C2D6', '78 06', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('短符號分支規格缺固定原始定位或可實作狀態')
    if '短 JS 停點已由規格 273 接通' not in older or '273-cpu386-short-sign-branches.md' not in older:
        raise RuntimeError('短 JS 舊停點缺後續回填')

if sys.argv[1:] == ['--check-short-sign-branches-spec-backlinks']:
    validate_short_sign_branches_resolution(spec_dir)
    print('短符號分支原始定位與後續回填通過')
    raise SystemExit(0)

def validate_inc_word_memory_resolution(spec_dir):
    """固定 INC 指令定位與舊停點回填必須同時存在。"""
    current = (spec_dir / '272-cpu386-inc-word-memory.md').read_text()
    older = (spec_dir / '271-cpu386-cmp-word-destination.md').read_text()
    required = ('0180:00350A3D', '0x21CA3D', '66 FF 40 04', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('word INC 規格缺固定原始定位或可實作狀態')
    if 'word INC 停點已由規格 272 接通' not in older or '272-cpu386-inc-word-memory.md' not in older:
        raise RuntimeError('word INC 舊停點缺後續回填')

if sys.argv[1:] == ['--check-inc-word-memory-spec-backlinks']:
    validate_inc_word_memory_resolution(spec_dir)
    print('word INC 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_cmp_word_destination_resolution(spec_dir):
    """固定 CMP 指令定位與舊停點回填必須同時存在。"""
    current = (spec_dir / '271-cpu386-cmp-word-destination.md').read_text()
    older = (spec_dir / '270-cpu386-cmp-memory-register.md').read_text()
    required = ('0180:0035CDCE', '0x228DCE', '66 39 07', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('word CMP 規格缺固定原始定位或可實作狀態')
    if 'word CMP 停點已由規格 271 接通' not in older or '271-cpu386-cmp-word-destination.md' not in older:
        raise RuntimeError('word CMP 舊停點缺後續回填')

if sys.argv[1:] == ['--check-cmp-word-destination-spec-backlinks']:
    validate_cmp_word_destination_resolution(spec_dir)
    print('word CMP 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_cmp_memory_register_resolution(spec_dir):
    """固定 CMP 指令定位與舊停點回填必須同時存在。"""
    current = (spec_dir / '270-cpu386-cmp-memory-register.md').read_text()
    older = (spec_dir / '269-cpu386-imul-dword-register.md').read_text()
    required = ('0180:00368C9A', '0x234C9A', '39 0D', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('記憶體 CMP 規格缺固定原始定位或可實作狀態')
    if '記憶體 CMP 停點已由規格 270 接通' not in older or '270-cpu386-cmp-memory-register.md' not in older:
        raise RuntimeError('記憶體 CMP 舊停點缺後續回填')

if sys.argv[1:] == ['--check-cmp-memory-register-spec-backlinks']:
    validate_cmp_memory_register_resolution(spec_dir)
    print('記憶體 CMP 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_imul_dword_register_resolution(spec_dir):
    """固定 IMUL 指令定位與舊停點回填必須同時存在。"""
    current = (spec_dir / '269-cpu386-imul-dword-register.md').read_text()
    older = (spec_dir / '268-cpu386-imul-word-register.md').read_text()
    required = ('0180:00368B5F', '0x234B5F', 'F7 EB', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('dword IMUL 規格缺固定原始定位或可實作狀態')
    if 'dword IMUL 停點已由規格 269 接通' not in older or '269-cpu386-imul-dword-register.md' not in older:
        raise RuntimeError('dword IMUL 舊停點缺後續回填')

if sys.argv[1:] == ['--check-imul-dword-register-spec-backlinks']:
    validate_imul_dword_register_resolution(spec_dir)
    print('dword IMUL 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_imul_word_register_resolution(spec_dir):
    """固定 IMUL 指令定位與舊停點回填必須同時存在。"""
    current = (spec_dir / '268-cpu386-imul-word-register.md').read_text()
    older = (spec_dir / '267-cpu386-add-word-register-immediate.md').read_text()
    required = ('0180:00368B43', '0x234B43', '66 F7 EB', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('word IMUL 規格缺固定原始定位或可實作狀態')
    if 'word IMUL 停點已由規格 268 接通' not in older or '268-cpu386-imul-word-register.md' not in older:
        raise RuntimeError('word IMUL 舊停點缺後續回填')

if sys.argv[1:] == ['--check-imul-word-register-spec-backlinks']:
    validate_imul_word_register_resolution(spec_dir)
    print('word IMUL 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_add_word_register_resolution(spec_dir):
    """固定 ADD 指令定位與舊停點回填必須同時存在。"""
    current = (spec_dir / '267-cpu386-add-word-register-immediate.md').read_text()
    older = (spec_dir / '266-moo2-vbe-display-start.md').read_text()
    required = ('0180:00368B10', '0x234B10', '66 83 C3 18', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('word ADD 規格缺固定原始定位或可實作狀態')
    if 'word ADD 停點已由規格 267 接通' not in older or '267-cpu386-add-word-register-immediate.md' not in older:
        raise RuntimeError('word ADD 舊停點缺後續回填')

if sys.argv[1:] == ['--check-add-word-register-spec-backlinks']:
    validate_add_word_register_resolution(spec_dir)
    print('word ADD 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_vbe_display_start_resolution(spec_dir):
    """非零起點的固定定位、舊停點及歷史零座標邊界須一起回填。"""
    current = (spec_dir / '266-moo2-vbe-display-start.md').read_text()
    required = ('0180:0035CCA7', '0x228CA7', 'CD 10 61 FC C3', '4F07h', 'hardware-spec approximation', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('VBE 顯示起點規格缺固定原始定位或可實作狀態')
    for name, marker in [('265-moo2-vbe-window-control.md', '非零顯示停點已由規格 266 接通'), ('237-moo2-vbe-zero-display-start.md', '非零起點已由規格 266 接通')]:
        older = (spec_dir / name).read_text()
        if marker not in older or '266-moo2-vbe-display-start.md' not in older:
            raise RuntimeError('VBE 顯示起點舊範圍缺後續回填：' + name)

if sys.argv[1:] == ['--check-vbe-display-start-spec-backlinks']:
    validate_vbe_display_start_resolution(spec_dir)
    print('VBE 顯示起點原始定位與後續回填通過')
    raise SystemExit(0)

def validate_vbe_window_control_resolution(spec_dir):
    """固定映像的 VBE 視窗定位與舊停點必須一起回填。"""
    current = (spec_dir / '265-moo2-vbe-window-control.md').read_text()
    older = (spec_dir / '264-cpu386-div-byte-register.md').read_text()
    required = ('0180:0035CC54', '0x228C54', 'CD 10', '4F05h', 'hardware-spec approximation', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('VBE 視窗規格缺固定原始定位或可實作狀態')
    if 'VBE 視窗停點已由規格 265 接通' not in older or '265-moo2-vbe-window-control.md' not in older:
        raise RuntimeError('VBE 視窗舊停點缺後續回填')

if sys.argv[1:] == ['--check-vbe-window-spec-backlinks']:
    validate_vbe_window_control_resolution(spec_dir)
    print('VBE 視窗原始定位與後續回填通過')
    raise SystemExit(0)

def validate_div_byte_register_resolution(spec_dir):
    """固定映像的 byte DIV 原始定位與舊停點必須同時回填。"""
    current = (spec_dir / '264-cpu386-div-byte-register.md').read_text()
    older = (spec_dir / '263-vga-dac-pel-mask.md').read_text()
    required = ('0180:00356CCB', '0x222CCB', 'F6 F3', '46h → 6h', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('byte DIV 規格缺固定原始定位或可實作狀態')
    if 'byte DIV 停點已由規格 264 接通' not in older or '264-cpu386-div-byte-register.md' not in older:
        raise RuntimeError('byte DIV 舊停點缺後續回填')

if sys.argv[1:] == ['--check-div-byte-register-spec-backlinks']:
    validate_div_byte_register_resolution(spec_dir)
    print('byte DIV 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_vga_pel_mask_resolution(spec_dir):
    """固定映像的 VGA 原始定位與舊停點必須同時回填。"""
    current = (spec_dir / '263-vga-dac-pel-mask.md').read_text()
    older = (spec_dir / '262-cpu386-test-word-register.md').read_text()
    required = ('0180:00356C9E', '0x222C9E', '03C6h', 'EE E8 9D FE FF FF', 'hardware-spec approximation', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('VGA 像素遮罩規格缺固定原始定位或可實作狀態')
    if 'VGA 像素遮罩停點已由規格 263 接通' not in older or '263-vga-dac-pel-mask.md' not in older:
        raise RuntimeError('VGA 像素遮罩舊停點缺後續回填')

if sys.argv[1:] == ['--check-vga-pel-mask-spec-backlinks']:
    validate_vga_pel_mask_resolution(spec_dir)
    print('VGA 像素遮罩原始定位與後續回填通過')
    raise SystemExit(0)

def validate_test_word_register_resolution(spec_dir):
    """固定映像的 word TEST 原始定位與舊停點必須同時回填。"""
    current = (spec_dir / '262-cpu386-test-word-register.md').read_text()
    older = (spec_dir / '261-moo2-dos-findfirst-current-directory.md').read_text()
    required = ('0180:00248F43', '0x114F43', '66 85 C0', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('word TEST 規格缺固定原始定位或可實作狀態')
    if 'word TEST 停點已由規格 262 接通' not in older or '262-cpu386-test-word-register.md' not in older:
        raise RuntimeError('word TEST 舊停點缺後續回填')

if sys.argv[1:] == ['--check-test-word-register-spec-backlinks']:
    validate_test_word_register_resolution(spec_dir)
    print('word TEST 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_find_current_directory_resolution(spec_dir):
    """目前目錄搜尋證據與規格 260 的舊停點須保持回填關係。"""
    current = (spec_dir / '261-moo2-dos-findfirst-current-directory.md').read_text()
    older = (spec_dir / '260-cpu386-sar-stack-dword-immediate.md').read_text()
    required = ('0180:0035DA59', '0x229A59', '2E 5C 73 69 6D 74 65 78 2E 6C 62 78 00', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('目前目錄搜尋規格缺固定原始定位或可實作狀態')
    if '目前目錄前綴已由規格 261 接通' not in older or '261-moo2-dos-findfirst-current-directory.md' not in older:
        raise RuntimeError('目前目錄搜尋舊停點缺後續回填')

if sys.argv[1:] == ['--check-find-current-directory-spec-backlinks']:
    validate_find_current_directory_resolution(spec_dir)
    print('目前目錄搜尋原始定位與後續回填通過')
    raise SystemExit(0)

def validate_sar_stack_resolution(spec_dir):
    """固定 1.31 映像的 CPU 停點須回填到規格 259。"""
    current = (spec_dir / '260-cpu386-sar-stack-dword-immediate.md').read_text()
    older = (spec_dir / '259-dpmi-free-memory-information.md').read_text()
    required = ('0180:00334E5A', '0x200E5A', 'C1 7D F4 04', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('堆疊 SAR 規格缺原始定位或可實作狀態')
    if '堆疊 SAR 停點已由規格 260 接通' not in older or '260-cpu386-sar-stack-dword-immediate.md' not in older:
        raise RuntimeError('堆疊 SAR 舊停點缺後續回填')

if sys.argv[1:] == ['--check-sar-stack-spec-backlinks']:
    validate_sar_stack_resolution(spec_dir)
    print('堆疊 SAR 原始定位與後續回填通過')
    raise SystemExit(0)

def validate_free_memory_resolution(spec_dir):
    """以固定映像與兩個位址基準驗證規格 259 回填規格 258。"""
    current = (spec_dir / '259-dpmi-free-memory-information.md').read_text()
    older = (spec_dir / '258-moo2-windows-version-absence.md').read_text()
    required = ('0180:00380315', '0x24C315', '0500h', '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f')
    if not all(value in current for value in required) or not re.search(r'^狀態：\*\*(READY|CONFORMED)', current, re.M):
        raise RuntimeError('DPMI 記憶體新規格缺固定原始定位或可實作狀態')
    if '0500h 平台契約已由規格 259 補齊' not in older or '259-dpmi-free-memory-information.md' not in older:
        raise RuntimeError('DPMI 記憶體舊停點缺後續回填')

if sys.argv[1:] == ['--check-free-memory-spec-backlinks']:
    validate_free_memory_resolution(spec_dir)
    print('DPMI 記憶體原始定位與後續回填通過')
    raise SystemExit(0)

if sys.argv[1:] == ['--check-mouse-spec-backlinks']:
    validate_mouse_sensitivity_resolution(spec_dir)
    print('滑鼠規格原始定位與勘誤回填通過')
    raise SystemExit(0)

root = pathlib.Path('/shots')
capture_irq0_end_chain = sys.argv[1:] == ['--irq0-end-chain']
capture_far_ret = capture_irq0_end_chain or sys.argv[1:] == ['--far-ret']
capture_cs_word_es_load = sys.argv[1:] == ['--cs-word-es-load']
capture_cs_memory_cmp = capture_cs_word_es_load or sys.argv[1:] == ['--cs-memory-cmp']
capture_dos_timer_vector = capture_far_ret or capture_cs_memory_cmp or sys.argv[1:] == ['--dos-timer-vector']
capture_pit_mode2 = sys.argv[1:] == ['--pit-mode2']
capture_vtd_entry = sys.argv[1:] == ['--vtd-entry']
capture_xor_word_register = sys.argv[1:] == ['--xor-word-register']
capture_short_sign_branches = sys.argv[1:] == ['--short-sign-branches']
capture_inc_word_memory = sys.argv[1:] == ['--inc-word-memory']
capture_cmp_word_destination = sys.argv[1:] == ['--cmp-word-destination']
capture_cmp_memory_register = sys.argv[1:] == ['--cmp-memory-register']
capture_imul_dword_register = sys.argv[1:] == ['--imul-dword-register']
capture_imul_word_register = sys.argv[1:] == ['--imul-word-register']
capture_add_word_register = sys.argv[1:] == ['--add-word-register']
capture_vbe_display_start = sys.argv[1:] == ['--vbe-display-start']
capture_vbe_window_control = sys.argv[1:] == ['--vbe-window-control']
capture_div_byte_register = sys.argv[1:] == ['--div-byte-register']
capture_vga_pel_mask = sys.argv[1:] == ['--vga-pel-mask']
capture_test_word_register = sys.argv[1:] == ['--test-word-register']
capture_find_current_directory = sys.argv[1:] == ['--find-current-directory']
capture_sar_stack_memory = sys.argv[1:] == ['--sar-stack-memory']
capture_free_memory = sys.argv[1:] == ['--free-memory']
capture_windows_version = sys.argv[1:] == ['--windows-version']
capture_mouse_sequence = sys.argv[1:] == ['--mouse-sequence']
capture_mouse_sensitivity = sys.argv[1:] == ['--mouse-sensitivity']
capture_mouse_set_sensitivity = sys.argv[1:] == ['--mouse-set-sensitivity']
capture_mouse_set_position = sys.argv[1:] == ['--mouse-set-position']
capture_mouse_callback_event = sys.argv[1:] == ['--mouse-callback-event']
capture_mouse_callback = sys.argv[1:] == ['--mouse-callback'] or capture_mouse_callback_event
if capture_mouse_set_sensitivity:
    validate_mouse_sensitivity_resolution(spec_dir)
capture_mouse_horizontal_range = sys.argv[1:] == ['--mouse-horizontal-range']
capture_mouse_vertical_range = sys.argv[1:] == ['--mouse-vertical-range']
capture_mouse_reset = sys.argv[1:] == ['--mouse-reset'] or capture_mouse_sensitivity or capture_mouse_set_sensitivity or capture_mouse_horizontal_range or capture_mouse_vertical_range or capture_mouse_callback or capture_mouse_set_position
capture_or_register_imm8 = sys.argv[1:] == ['--or-register-imm8']
capture_xor_register_imm32 = sys.argv[1:] == ['--xor-register-imm32']
if not (capture_dos_timer_vector or capture_pit_mode2 or capture_vtd_entry or capture_xor_word_register or capture_short_sign_branches or capture_inc_word_memory or capture_cmp_word_destination or capture_cmp_memory_register or capture_imul_dword_register or capture_imul_word_register or capture_add_word_register or capture_vbe_display_start or capture_vbe_window_control or capture_div_byte_register or capture_vga_pel_mask or capture_test_word_register or capture_find_current_directory or capture_sar_stack_memory or capture_free_memory or capture_windows_version or capture_mouse_sequence or capture_mouse_reset or capture_or_register_imm8 or capture_xor_register_imm32) and sys.argv[1:] not in ([], ['--sbb'], ['--sbb-word'], ['--add-al-imm8'], ['--low-entry'], ['--enter'], ['--cmp-word'], ['--cmp-byte'], ['--lea-cs'], ['--startup-value'], ['--mouse-query'], ['--mouse-function-21'], ['--mouse-function-1a'], ['--video-mode-03'], ['--full-data-test-word'], ['--dos-memory-0100'], ['--real-video-0300'], ['--real-video-4f01'], ['--video-display-4f07'], ['--video-mode-4f02'], ['--es-store'], ['--or-al-ah'], ['--ror-imm8'], ['--es-byte-load'], ['--es-byte-load-ev'], ['--test-word'], ['--dta'], ['--dta-find'], ['--dta-find-present'], ['--empty-mox-cmp'], ['--xchg'], ['--cmc'], ['--and'], ['--or-memory'], ['--pop-gs']):
    raise SystemExit('usage: startup_probe_131.py [--check-pit-count-latch-spec-backlinks|--check-in-al-immediate-spec-backlinks|--irq0-end-chain|--check-irq0-end-chain-spec-backlinks|--check-far-ret-spec-backlinks|--far-ret|--check-cs-es-load-spec-backlinks|--cs-word-es-load|--check-cs-memory-cmp-spec-backlinks|--cs-memory-cmp|--check-protected-irq0-spec-backlinks|--dos-timer-vector|--check-pit-mode2-spec-backlinks|--pit-mode2|--check-vtd-entry-spec-backlinks|--vtd-entry|--check-xor-word-register-spec-backlinks|--xor-word-register|--check-short-sign-branches-spec-backlinks|--short-sign-branches|--check-inc-word-memory-spec-backlinks|--inc-word-memory|--check-cmp-word-destination-spec-backlinks|--cmp-word-destination|--check-cmp-memory-register-spec-backlinks|--cmp-memory-register|--check-imul-dword-register-spec-backlinks|--imul-dword-register|--check-imul-word-register-spec-backlinks|--imul-word-register|--add-word-register|--check-add-word-register-spec-backlinks|--vbe-display-start|--check-vbe-display-start-spec-backlinks|--vbe-window-control|--check-vbe-window-spec-backlinks|--div-byte-register|--check-div-byte-register-spec-backlinks|--vga-pel-mask|--check-vga-pel-mask-spec-backlinks|--test-word-register|--check-test-word-register-spec-backlinks|--find-current-directory|--check-find-current-directory-spec-backlinks|--sbb|--sbb-word|--add-al-imm8|--or-register-imm8|--xor-register-imm32|--mouse-reset|--mouse-sensitivity|--mouse-sequence|--mouse-horizontal-range|--mouse-vertical-range|--mouse-set-sensitivity|--mouse-set-position|--mouse-callback|--mouse-callback-event|--windows-version|--sar-stack-memory|--check-sar-stack-spec-backlinks|--free-memory|--check-free-memory-spec-backlinks|--check-mouse-spec-backlinks|--low-entry|--enter|--cmp-word|--cmp-byte|--lea-cs|--startup-value|--mouse-query|--mouse-function-21|--mouse-function-1a|--video-mode-03|--full-data-test-word|--dos-memory-0100|--real-video-0300|--real-video-4f01|--video-display-4f07|--video-mode-4f02|--empty-mox-cmp|--es-store|--or-al-ah|--ror-imm8|--es-byte-load|--es-byte-load-ev|--test-word|--dta|--dta-find|--dta-find-present|--xchg|--cmc|--and|--or-memory|--pop-gs]')
capture_sbb = sys.argv[1:] == ['--sbb']
capture_sbb_word = sys.argv[1:] == ['--sbb-word']
capture_add_al_imm8 = sys.argv[1:] == ['--add-al-imm8']
capture_low_entry = sys.argv[1:] == ['--low-entry']
capture_enter = sys.argv[1:] == ['--enter']
capture_cmp_word = sys.argv[1:] == ['--cmp-word']
capture_cmp_byte = sys.argv[1:] == ['--cmp-byte']
capture_lea_cs = sys.argv[1:] == ['--lea-cs']
capture_startup_value = sys.argv[1:] == ['--startup-value']
capture_mouse_query = sys.argv[1:] == ['--mouse-query']
capture_mouse_function_21 = sys.argv[1:] == ['--mouse-function-21']
capture_mouse_function_1a = sys.argv[1:] == ['--mouse-function-1a']
capture_video_mode_03 = sys.argv[1:] == ['--video-mode-03']
capture_full_data_test_word = sys.argv[1:] == ['--full-data-test-word']
capture_dos_memory_0100 = sys.argv[1:] == ['--dos-memory-0100']
capture_real_video_0300 = sys.argv[1:] == ['--real-video-0300']
capture_real_video_4f01 = sys.argv[1:] == ['--real-video-4f01']
capture_video_display_4f07 = sys.argv[1:] == ['--video-display-4f07']
capture_video_mode_4f02 = sys.argv[1:] == ['--video-mode-4f02']
capture_es_store = sys.argv[1:] == ['--es-store']
capture_or_al_ah = sys.argv[1:] == ['--or-al-ah']
capture_ror_imm8 = sys.argv[1:] == ['--ror-imm8']
capture_es_byte_load = sys.argv[1:] == ['--es-byte-load']
capture_es_byte_load_ev = sys.argv[1:] == ['--es-byte-load-ev']
capture_test_word = sys.argv[1:] == ['--test-word']
capture_dta = sys.argv[1:] in (['--dta'], ['--dta-find'], ['--dta-find-present'])
capture_dta_find = sys.argv[1:] in (['--dta-find'], ['--dta-find-present'])
capture_dta_find_present = sys.argv[1:] == ['--dta-find-present']
capture_empty_mox_cmp = sys.argv[1:] == ['--empty-mox-cmp']
capture_dta = capture_dta or capture_empty_mox_cmp
capture_dta_find = capture_dta_find or capture_empty_mox_cmp
capture_dta_find_present = capture_dta_find_present or capture_empty_mox_cmp
capture_xchg = sys.argv[1:] == ['--xchg']
capture_cmc = sys.argv[1:] == ['--cmc']
capture_and = sys.argv[1:] == ['--and']
capture_or_memory = sys.argv[1:] == ['--or-memory']
capture_pop_gs = sys.argv[1:] == ['--pop-gs']
mode = 'add-al-imm8-' if capture_add_al_imm8 else 'video-mode-4f02-' if capture_video_mode_4f02 else 'real-video-4f01-' if capture_real_video_4f01 else 'video-display-4f07-' if capture_video_display_4f07 else 'real-video-0300-' if capture_real_video_0300 else 'dos-memory-0100-' if capture_dos_memory_0100 else 'full-data-test-word-' if capture_full_data_test_word else 'empty-mox-cmp-' if capture_empty_mox_cmp else 'video-mode-03-' if capture_video_mode_03 else 'mouse-function-1a-' if capture_mouse_function_1a else 'mouse-function-21-' if capture_mouse_function_21 else 'es-store-' if capture_es_store else 'mouse-query-' if capture_mouse_query else 'startup-value-' if capture_startup_value else 'dta-find-present-' if capture_dta_find_present else 'dta-find-' if capture_dta_find else 'dta-' if capture_dta else 'test-word-' if capture_test_word else 'lea-cs-' if capture_lea_cs else 'or-al-ah-' if capture_or_al_ah else 'ror-imm8-' if capture_ror_imm8 else 'es-byte-load-ev-' if capture_es_byte_load_ev else 'es-byte-load-' if capture_es_byte_load else 'cmp-byte-' if capture_cmp_byte else 'cmp-word-' if capture_cmp_word else 'enter-' if capture_enter else 'low-entry-' if capture_low_entry else 'sbb-word-' if capture_sbb_word else 'pop-gs-' if capture_pop_gs else 'or-memory-' if capture_or_memory else 'and-' if capture_and else 'cmc-' if capture_cmc else 'xchg-' if capture_xchg else 'sbb-' if capture_sbb else ''
exe = pathlib.Path('/tmp/game/ORION2.EXE')
if capture_dos_timer_vector:
    mode = 'irq0-end-chain-' if capture_irq0_end_chain else 'far-ret-' if capture_far_ret else 'cs-word-es-load-' if capture_cs_word_es_load else 'cs-memory-cmp-' if capture_cs_memory_cmp else 'dos-timer-vector-'
if capture_pit_mode2:
    mode = 'pit-mode2-'
if capture_vtd_entry:
    mode = 'vtd-entry-'
if capture_xor_word_register:
    mode = 'xor-word-register-'
elif capture_short_sign_branches:
    mode = 'short-sign-branches-'
elif capture_inc_word_memory:
    mode = 'inc-word-memory-'
elif capture_cmp_word_destination:
    mode = 'cmp-word-destination-'
elif capture_cmp_memory_register:
    mode = 'cmp-memory-register-'
elif capture_imul_dword_register:
    mode = 'imul-dword-register-'
elif capture_imul_word_register:
    mode = 'imul-word-register-'
elif capture_add_word_register:
    mode = 'add-word-register-'
elif capture_vbe_display_start:
    mode = 'vbe-display-start-'
elif capture_vbe_window_control:
    mode = 'vbe-window-control-'
if capture_div_byte_register:
    mode = 'div-byte-register-'
if capture_vga_pel_mask:
    mode = 'vga-pel-mask-'
if capture_test_word_register:
    mode = 'test-word-register-'
if capture_find_current_directory:
    mode = 'find-current-directory-'
if capture_sar_stack_memory:
    mode = 'sar-stack-memory-'
if capture_free_memory:
    mode = 'free-memory-'
if capture_windows_version:
    mode = 'windows-version-'
if capture_mouse_sequence:
    mode = 'mouse-sequence-'
if capture_mouse_reset:
    mode = 'mouse-sensitivity-' if capture_mouse_sensitivity else 'mouse-reset-'
    if capture_mouse_set_sensitivity:
        mode = 'mouse-set-sensitivity-'
    if capture_mouse_set_position:
        mode = 'mouse-set-position-'
    if capture_mouse_callback:
        mode = 'mouse-callback-event-' if capture_mouse_callback_event else 'mouse-callback-'
    if capture_mouse_horizontal_range:
        mode = 'mouse-horizontal-range-'
    if capture_mouse_vertical_range:
        mode = 'mouse-vertical-range-'
if capture_or_register_imm8:
    mode = 'or-register-imm8-'
if capture_xor_register_imm32:
    mode = 'xor-register-imm32-'
expected_sha256 = '4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f'
actual_sha256 = hashlib.sha256(exe.read_bytes()).hexdigest()
if actual_sha256 != expected_sha256:
    raise RuntimeError('MOO2 1.31 EXE SHA-256 不符：' + actual_sha256)
master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 50, 160, 0, 0))
proc = subprocess.Popen(
    ['dosbox-x', '-fastlaunch', '-conf', str(pathlib.Path(__file__).with_name('dosbox.conf')),
     '-c', 'mount c /tmp/game', '-c', 'c:', '-c', 'debugbox ORION2.EXE'],
    stdin=slave, stdout=slave, stderr=slave,
    env=dict(os.environ, TERM='xterm'),
)
os.close(slave)
commands = []
records = {
    'input_sha256': actual_sha256,
    'address_space': 'DOSBox-X CS:EIP',
    'register_order': 'CS EIP EAX EBX ECX EDX DS ES FS GS SS ESP EFLAGS',
}
if capture_dta_find_present:
    fixture = pathlib.Path('/tmp/game/MOX.SET')
    if not fixture.is_file() or fixture.stat().st_size != 0:
        raise RuntimeError('MOO2 成功分支要求明示的空 MOX.SET 合成輸入')
    if int(fixture.stat().st_mtime) != 820454400:
        raise RuntimeError('MOO2 空 MOX.SET 的修改時間必須固定為 1996-01-01 00:00:00 UTC')
    records['controlled_fixture'] = {
        'name': 'MOX.SET', 'size': 0, 'sha256': hashlib.sha256(b'').hexdigest(),
        'mtime_epoch_seconds': 820454400,
    }
if capture_dos_timer_vector or capture_pit_mode2 or capture_vtd_entry or capture_xor_word_register or capture_short_sign_branches or capture_inc_word_memory or capture_cmp_word_destination or capture_cmp_memory_register or capture_imul_dword_register or capture_imul_word_register or capture_add_word_register or capture_vbe_display_start or capture_vbe_window_control or capture_div_byte_register or capture_vga_pel_mask or capture_test_word_register or capture_find_current_directory or capture_sar_stack_memory or capture_free_memory or capture_windows_version or capture_full_data_test_word or capture_dos_memory_0100 or capture_real_video_0300 or capture_real_video_4f01 or capture_video_display_4f07 or capture_video_mode_4f02 or capture_add_al_imm8 or capture_or_register_imm8 or capture_xor_register_imm32 or capture_mouse_reset or capture_mouse_sequence:
    fixture = pathlib.Path('/tmp/game/MOX.SET')
    if not fixture.is_file() or hashlib.sha256(fixture.read_bytes()).hexdigest() != 'bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80':
        raise RuntimeError('MOO2 完整資料對拍要求固定正版 MOX.SET')
    records['controlled_fixture'] = {'name': 'MOX.SET', 'size': fixture.stat().st_size, 'sha256': hashlib.sha256(fixture.read_bytes()).hexdigest()}
if capture_xchg:
    records['xchg_register_order'] = 'CS EIP EAX EBX ECX EDX EDI DS ES FS GS SS ESP EFLAGS'

def drain(seconds):
    deadline = time.monotonic() + seconds
    captured = bytearray()
    while time.monotonic() < deadline:
        if select.select([master], [], [], 0.1)[0]:
            data = os.read(master, 65536)
            if not data:
                break
            captured.extend(data)
            output.write(data)
            output.flush()
    return bytes(captured)

def cmd(value, seconds=0.5):
    commands.append(value)
    os.write(master, (value + '\n').encode())
    return drain(seconds)

def registers(data):
    text = data.decode('latin1')
    clean = re.sub(r'\x1b\[[0-9;?]*[A-Za-z]|\x1b\([A-Za-z0-9]', '', text)
    matches = re.findall(r"EV of '[^']+' is:\s*([0-9a-f ]+)", clean)
    return [match.split() for match in matches]

with (root / (mode + 'terminal.raw')).open('wb') as output:
    try:
        drain(3)
        cmd('BPINT 21 30')
        found = False
        for _ in range(16):
            cmd('RUN', 5)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES FS GS SS ESP EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:4] == ['180', '333fb5', '3000', '50484152']), None)
            if match:
                records['dos_version_before'] = match
                found = True
                break
        if not found:
            raise RuntimeError('MOO2 AH=30h caller not reached')
        cmd('BPDEL *')
        cmd('BP 0180:00333FB7')
        cmd('RUN', 6)
        snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES FS GS SS ESP EFLAGS', 0.8))
        match = next((value for value in snapshots if value[:2] == ['180', '333fb7']), None)
        if not match:
            raise RuntimeError('MOO2 AH=30h return breakpoint not reached: ' + repr(snapshots))
        records['dos_version_after'] = match
        cmd('BPDEL *')
        cmd('BPINT 21 FF 00')
        found = False
        for _ in range(12):
            cmd('RUN', 5)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES FS GS SS ESP EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:3] == ['180', '334054', 'ff00']), None)
            if match:
                records['dos4g_before'] = match
                found = True
                break
        if not found:
            raise RuntimeError('MOO2 AX=FF00h caller not reached')
        cmd('BPDEL *')
        cmd('BP 0180:00334056')
        cmd('RUN', 6)
        snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES FS GS SS ESP EFLAGS', 0.8))
        match = next((value for value in snapshots if value[:2] == ['180', '334056']), None)
        if not match:
            raise RuntimeError('MOO2 AX=FF00h return breakpoint not reached: ' + repr(snapshots))
        records['dos4g_after'] = match
        cmd('BPDEL *')
        cmd('BP 0180:00334072')
        cmd('RUN', 6)
        snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES FS GS SS ESP EFLAGS', 0.8))
        match = next((value for value in snapshots if value[:2] == ['180', '334072']), None)
        if not match:
            raise RuntimeError('MOO2 DPMI AX=0006h return breakpoint not reached: ' + repr(snapshots))
        records['dpmi_base_after'] = match
        cmd('BPDEL *')
        cmd('BP 0180:00334079')
        cmd('RUN', 6)
        snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES FS GS SS ESP EFLAGS', 0.8))
        match = next((value for value in snapshots if value[:2] == ['180', '334079']), None)
        if not match:
            raise RuntimeError('MOO2 zero-base branch breakpoint not reached: ' + repr(snapshots))
        records['dpmi_zero_base_branch'] = match
        if capture_find_current_directory:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['find_current_directory_register_order'] = order
            records['find_current_directory_events'] = []
            dump = pathlib.Path('MEMDUMP.BIN')
            def memory(address, count):
                dump.unlink(missing_ok=True)
                cmd(f'MEMDUMPBIN {address} {count:X}', 1)
                if not dump.is_file() or dump.stat().st_size != count:
                    raise RuntimeError('DOS 搜尋記憶體擷取長度不符：' + address)
                return dump.read_bytes()
            matched = False
            for _ in range(12):
                cmd('BPDEL *')
                cmd('BPINT 21 1A')
                cmd('RUN', 8)
                snapshots = registers(cmd('EV ' + order, 0.8))
                dta_call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '35da53'] and (int(v[2],16)>>8)&255 == 0x1a), None)
                if not dta_call:
                    raise RuntimeError('目前目錄搜尋 DTA 設定未命中：' + repr(snapshots))
                destination = f'{int(dta_call[9],16):04X}:{int(dta_call[5],16):08X}'
                cmd('BPDEL *')
                cmd('BP 0180:0035DA59')
                cmd('RUN', 6)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '35da59'] and (int(v[2],16)>>8)&255 == 0x4e), None)
                if not call:
                    raise RuntimeError('目前目錄搜尋入口未命中：' + repr(snapshots))
                pattern_address = f'{int(call[9],16):04X}:{int(call[5],16):08X}'
                raw_pattern = memory(pattern_address, 128)
                pattern = raw_pattern.split(b'\0', 1)[0]
                if len(pattern) == 128:
                    raise RuntimeError('搜尋字串缺結尾')
                event = {'dta_call': dta_call, 'call': call, 'pattern_address': pattern_address,
                         'pattern_hex': pattern.hex(), 'dta_address': destination,
                         'dta_before_hex': memory(destination, 43).hex()}
                records['find_current_directory_events'].append(event)
                if pattern == b'.\\simtex.lbx':
                    raw_code = memory('0180:0035DA59', 16)
                    if raw_code != bytes.fromhex('cd 21 e8 ac 65 01 00 89 da e8 21 00 00 00 59 c3'):
                        raise RuntimeError('目前目錄搜尋原始 bytes 不符')
                    records['find_current_directory_bytes_hex'] = raw_code.hex()
                cmd('BPDEL *')
                cmd('BP 0180:0035DA5B')
                cmd('RUN', 6)
                snapshots = registers(cmd('EV ' + order, 0.8))
                returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '35da5b']), None)
                if not returned:
                    raise RuntimeError('目前目錄搜尋返回未命中')
                event['return'] = returned
                event['dta_after_hex'] = memory(destination, 43).hex()
                if pattern == b'.\\simtex.lbx':
                    cmd('BPDEL *')
                    log = pathlib.Path('LOGCPU.TXT')
                    log.unlink(missing_ok=True)
                    cmd('LOG 40', 7)
                    if not log.is_file():
                        raise RuntimeError('目前目錄搜尋 caller 紀錄未產生')
                    data = log.read_bytes()
                    lines = data.decode('latin1').splitlines()
                    if len(lines) != 64 or not lines[0].startswith('0180:0035DA5B'):
                        raise RuntimeError('目前目錄搜尋 caller 序列不符')
                    records['find_current_directory_caller_lines'] = lines
                    records['find_current_directory_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
                    (root / 'find-current-directory-caller-logcpu.txt').write_bytes(data)
                    matched = True
                    break
            if not matched:
                raise RuntimeError('有界搜尋未命中目前目錄前綴')
        if capture_vbe_display_start:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['vbe_display_start_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:0035CCA7')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '35cca7']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 VBE 顯示起點控制 候選未命中：' + repr(snapshots))
            if [int(x,16) for x in call[2:6]] != [0x4f07,0,0,0x200]:
                raise RuntimeError('非零顯示起點輸入不符')
            records['vbe_display_start_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:0035CCA7 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes()[:12] != bytes.fromhex('cd 10 61 fc c3 00 00 00 00 60 66 8b'):
                raise RuntimeError('MOO2 VBE 顯示起點控制 原始 bytes 不符')
            records['vbe_display_start_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:0035CCA9')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '35cca9']), None)
            if not returned:
                raise RuntimeError('MOO2 VBE 顯示起點控制 下一指令未命中')
            records['vbe_display_start_after'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 3', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 VBE 顯示起點控制 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:0035CCA9'):
                raise RuntimeError('MOO2 VBE 顯示起點控制 消費端起點不符')
            records['vbe_display_start_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'vbe-display-start-caller-logcpu.txt').write_bytes(data)
        if capture_add_word_register:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['add_word_register_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:00368B10')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '368b10']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 word ADD 候選未命中：' + repr(snapshots))
            records['add_word_register_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00368B10 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes() != bytes.fromhex('66 83 c3 18 81 fb e0 01 00 00 7c 13 33 db 66 bb'):
                raise RuntimeError('MOO2 word ADD 原始 bytes 不符')
            records['add_word_register_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00368B14')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '368b14']), None)
            if not returned:
                raise RuntimeError('MOO2 word ADD 下一指令未命中')
            records['add_word_register_after'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 word ADD 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:00368B14'):
                raise RuntimeError('MOO2 word ADD 消費端起點不符')
            records['add_word_register_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'add-word-register-caller-logcpu.txt').write_bytes(data)
        if capture_imul_word_register:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['imul_word_register_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:00368B43')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '368b43']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 word IMUL 候選未命中：' + repr(snapshots))
            records['imul_word_register_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00368B43 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes() != bytes.fromhex('66 f7 eb a3 7a ed 39 00 33 db 33 c9 33 c0 33 d2'):
                raise RuntimeError('MOO2 word IMUL 原始 bytes 不符')
            records['imul_word_register_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00368B46')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '368b46']), None)
            if not returned:
                raise RuntimeError('MOO2 word IMUL 下一指令未命中')
            records['imul_word_register_after'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 word IMUL 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:00368B46'):
                raise RuntimeError('MOO2 word IMUL 消費端起點不符')
            records['imul_word_register_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'imul-word-register-caller-logcpu.txt').write_bytes(data)
        if capture_imul_dword_register:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['imul_dword_register_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:00368B5F')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '368b5f']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 dword IMUL 候選未命中：' + repr(snapshots))
            records['imul_dword_register_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00368B5F 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes() != bytes.fromhex('f7 eb 33 db 33 d2 66 8b 1d 42 1a 3d 00 03 c3 a3'):
                raise RuntimeError('MOO2 dword IMUL 原始 bytes 不符')
            records['imul_dword_register_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00368B61')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '368b61']), None)
            if not returned:
                raise RuntimeError('MOO2 dword IMUL 下一指令未命中')
            records['imul_dword_register_after'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 dword IMUL 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:00368B61'):
                raise RuntimeError('MOO2 dword IMUL 消費端起點不符')
            records['imul_dword_register_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'imul-dword-register-caller-logcpu.txt').write_bytes(data)
        if capture_cmp_memory_register:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['cmp_memory_register_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:00368C9A')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '368c9a']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 CMP 記憶體 候選未命中：' + repr(snapshots))
            records['cmp_memory_register_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00368C9A 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes() != bytes.fromhex('39 0d 9b ed 39 00 0f 8d 01 02 00 00 80 3d 94 ed'):
                raise RuntimeError('MOO2 CMP 記憶體 原始 bytes 不符')
            records['cmp_memory_register_bytes_hex'] = dump.read_bytes().hex()
            destination = int.from_bytes(dump.read_bytes()[2:6], 'little')
            records['cmp_memory_destination_offset_hex'] = format(destination, 'x')
            dump.unlink()
            cmd('MEMDUMPBIN ' + call[9] + ':' + format(destination, '08X') + ' 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('MOO2 CMP 記憶體輸入未擷取')
            records['cmp_memory_destination_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00368CA0')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '368ca0']), None)
            if not returned:
                raise RuntimeError('MOO2 CMP 記憶體 下一指令未命中')
            records['cmp_memory_register_after'] = returned
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN ' + returned[9] + ':' + format(destination, '08X') + ' 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('MOO2 CMP 記憶體比較後資料未擷取')
            records['cmp_memory_destination_after_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 CMP 記憶體 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:00368CA0'):
                raise RuntimeError('MOO2 CMP 記憶體 消費端起點不符')
            records['cmp_memory_register_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'cmp-memory-register-caller-logcpu.txt').write_bytes(data)
            snapshots = registers(cmd('EV ' + order, 0.8))
            consumed = next((v for v in reversed(snapshots) if len(v) == 16 and v[0] == '180'), None)
            if not consumed or consumed[1] not in ('368ca6', '368ea7'):
                raise RuntimeError('MOO2 CMP 記憶體分支後未擷取')
            records['cmp_memory_consumer_after'] = consumed
        if capture_cmp_word_destination:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['cmp_word_destination_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:0035CDCE')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '35cdce']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 word CMP 候選未命中：' + repr(snapshots))
            records['cmp_word_destination_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:0035CDCE 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes() != bytes.fromhex('66 39 07 7c 03 66 89 07 83 c7 02 66 39 1f 7f 03'):
                raise RuntimeError('MOO2 word CMP 原始 bytes 不符')
            records['cmp_word_destination_bytes_hex'] = dump.read_bytes().hex()
            destination = int(call[7], 16)
            records['cmp_word_memory_offset_hex'] = format(destination, 'x')
            dump.unlink()
            cmd('MEMDUMPBIN ' + call[9] + ':' + format(destination, '08X') + ' 2', 1)
            if not dump.is_file() or dump.stat().st_size != 2:
                raise RuntimeError('MOO2 word CMP輸入未擷取')
            records['cmp_word_memory_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:0035CDD1')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '35cdd1']), None)
            if not returned:
                raise RuntimeError('MOO2 word CMP 下一指令未命中')
            records['cmp_word_destination_after'] = returned
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN ' + returned[9] + ':' + format(destination, '08X') + ' 2', 1)
            if not dump.is_file() or dump.stat().st_size != 2:
                raise RuntimeError('MOO2 word CMP比較後資料未擷取')
            records['cmp_word_memory_after_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 word CMP 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:0035CDD1'):
                raise RuntimeError('MOO2 word CMP 消費端起點不符')
            records['cmp_word_destination_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'cmp-word-destination-caller-logcpu.txt').write_bytes(data)
            snapshots = registers(cmd('EV ' + order, 0.8))
            consumed = next((v for v in reversed(snapshots) if len(v) == 16 and v[0] == '180'), None)
            if not consumed or consumed[1] not in ('35cdd3', '35cdd6'):
                raise RuntimeError('MOO2 word CMP分支後未擷取')
            records['cmp_word_consumer_after'] = consumed
        if capture_dos_timer_vector:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['dos_timer_vector_order'] = order
            def timer_snapshot(name, expected_ip):
                snapshots = registers(cmd('EV ' + order, 0.8))
                value = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', expected_ip]), None)
                if not value:
                    raise RuntimeError('MOO2 向量有限樣本位置不符：' + name + repr(snapshots))
                records[name] = value
                return value
            def timer_run_to(name, address):
                cmd('BPDEL *')
                cmd('BP 0180:' + address)
                value = None
                for _ in range(6):
                    cmd('RUN', 12)
                    snapshots = registers(cmd('EV ' + order, 0.8))
                    value = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', address.lower().lstrip('0')]), None)
                    if value:
                        break
                if not value:
                    raise RuntimeError('MOO2 向量候選未命中：' + name + repr(snapshots))
                records[name] = value
                return value
            def timer_memory(name, selector, offset, count):
                dump = pathlib.Path('MEMDUMP.BIN')
                dump.unlink(missing_ok=True)
                cmd('MEMDUMPBIN ' + selector + ':' + offset + ' ' + format(count, 'X'), 1)
                if not dump.is_file() or dump.stat().st_size != count:
                    raise RuntimeError('MOO2 向量有限資料未擷取：' + name)
                records[name] = dump.read_bytes().hex()
                return dump.read_bytes()
            def timer_one(name, before, after):
                cmd('BPDEL *')
                log = pathlib.Path('LOGCPU.TXT')
                log.unlink(missing_ok=True)
                cmd('LOG 2', 5)
                if not log.is_file() or not log.read_bytes().decode('latin1').startswith('0180:' + before):
                    raise RuntimeError('MOO2 向量有限服務起點不符')
                data = log.read_bytes()
                (root / ((mode if capture_far_ret or capture_cs_memory_cmp else '') + name + '-logcpu.txt')).write_bytes(data)
                records[name + '_log_sha256'] = hashlib.sha256(data).hexdigest()
                timer_run_to(name + '_after', after)
            get = timer_run_to('timer_get_before', '0037901B')
            if int(get[2], 16) & 0xffff != 0x3508 or timer_memory('timer_get_bytes', '0180', '0037901B', 24)[:2] != bytes.fromhex('cd 21'):
                raise RuntimeError('MOO2 AH=3508h 呼叫形狀不符')
            timer_one('timer-get', '0037901B', '0037901D')
            setting = timer_run_to('timer_set_before', '00379048')
            if int(setting[2], 16) & 0xffff != 0x2508 or setting[9] != '180' or setting[5] != '378d9a' or timer_memory('timer_set_bytes', '0180', '00379048', 24)[:2] != bytes.fromhex('cd 21'):
                raise RuntimeError('MOO2 AH=2508h 呼叫形狀不符')
            timer_one('timer-set', '00379048', '0037904A')
            if capture_far_ret:
                before = timer_run_to('far_ret_before', '00378E9C')
                if timer_memory('far_ret_instruction_bytes', '0180', '00378E9C', 1) != bytes([0xcb]):
                    raise RuntimeError('CB 遠返回候選原始位元組不符')
                ss, esp = int(before[13], 16), int(before[14], 16)
                frame_size = 20 if capture_irq0_end_chain else 8
                frame = timer_memory('far_ret_frame_before', format(ss, '04X'), format(esp, '08X'), frame_size)
                target_ip, selector_slot = struct.unpack('<II', frame[:8])
                target_cs = selector_slot & 0xffff
                if target_cs & 3 != int(before[0], 16) & 3:
                    raise RuntimeError('CB 樣本不是同權限返回')
                records['far_ret_target'] = {'cs': target_cs, 'eip': target_ip, 'selector_slot': selector_slot}
                cmd('BPDEL *')
                log = pathlib.Path('LOGCPU.TXT'); log.unlink(missing_ok=True)
                cmd('LOG 2', 5)
                if not log.is_file() or not log.read_bytes().decode('latin1').startswith('0180:00378E9C'):
                    raise RuntimeError('CB 有限單指令起點不符')
                data = log.read_bytes()
                (root / (mode + 'logcpu.txt')).write_bytes(data)
                records['far_ret_log_sha256'] = hashlib.sha256(data).hexdigest()
                snapshots = registers(cmd('EV ' + order, 0.8))
                returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == [format(target_cs, 'x'), format(target_ip, 'x')]), None)
                if not returned or int(returned[14], 16) != esp + 8:
                    raise RuntimeError('CB 返回位置或堆疊消費不符：' + repr(snapshots))
                if returned[2:14] != before[2:14] or returned[15] != before[15]:
                    raise RuntimeError('CB 改變了其他暫存器／段或旗標')
                records['far_ret_after'] = returned
                if timer_memory('far_ret_frame_after', format(ss, '04X'), format(esp, '08X'), frame_size) != frame:
                    raise RuntimeError('CB 改寫了原始返回框架')
                if capture_irq0_end_chain:
                    return_ip, return_selector, return_flags = struct.unpack('<III', frame[8:])
                    return_cs = return_selector & 0xffff
                    if not return_cs or return_cs & 3 != target_cs & 3:
                        raise RuntimeError('結束鏈返回框架不是同權限的非空 selector')
                    records['irq0_end_chain_frame'] = frame[8:].hex()
                    records['irq0_end_chain_target'] = {'cs': return_cs, 'eip': return_ip, 'flags': return_flags}
                    bda_before = timer_memory('irq0_end_chain_bda_before', '0188', '0000046C', 5)
                    cmd('BPDEL *')
                    cmd(f'BP {return_cs:04X}:{return_ip:08X}')
                    # 核心只黑箱自然執行到既有框架給出的返回點，不擷取內部指令。
                    cmd('RUN', 12)
                    snapshots = registers(cmd('EV ' + order, 0.8))
                    resumed = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == [format(return_cs, 'x'), format(return_ip, 'x')]), None)
                    if not resumed:
                        raise RuntimeError('結束鏈框架返回點未命中：' + repr(snapshots))
                    records['irq0_end_chain_after'] = resumed
                    bda_after = timer_memory('irq0_end_chain_bda_after', '0188', '0000046C', 5)
                    validate_irq0_end_chain_boundary(returned, resumed, bda_before, bda_after)
                    timer_memory('irq0_end_chain_frame_after', format(ss, '04X'), format(esp + 8, '08X'), 12)
            else:
                timer_run_to('timer_mode2_before', '0036DAE8')
                timer_memory('timer_wait_source_before', '0188', '0039F148', 4)
                cmd('BPDEL *')
                log = pathlib.Path('LOGCPU.TXT'); log.unlink(missing_ok=True)
                cmd('LOG 8', 5)
                if not log.is_file() or not log.read_bytes().decode('latin1').startswith('0180:0036DAE8'):
                    raise RuntimeError('MOO2 模式 2 呼叫序列未擷取')
                data = log.read_bytes(); (root / ((mode if capture_cs_memory_cmp else '') + 'timer-mode2-logcpu.txt')).write_bytes(data)
                records['timer_mode2_log_sha256'] = hashlib.sha256(data).hexdigest()
                timer_snapshot('timer_mode2_after', '36daf6')
                entry = timer_run_to('timer_irq_entry', '00378D9A')
                timer_memory('timer_irq_entry_bytes', '0180', '00378D9A', 16)
                timer_memory('timer_irq_stack', entry[13].zfill(4), entry[14].zfill(8), 24)
                timer_memory('timer_wait_source_at_irq', '0188', '0039F148', 4)
                if capture_cs_memory_cmp:
                    if records['timer_irq_entry_bytes'][:16] != '2e833dfef9390000':
                        raise RuntimeError('CS CMP 原始位元組不符')
                    timer_memory('cs_cmp_before_bytes', '0180', '0039F9FE', 4)
                    cmd('BPDEL *')
                    log = pathlib.Path('LOGCPU.TXT'); log.unlink(missing_ok=True)
                    cmd('LOG 2', 5)
                    if not log.is_file() or not log.read_bytes().decode('latin1').startswith('0180:00378D9A'):
                        raise RuntimeError('CS CMP 有限單指令起點不符')
                    data = log.read_bytes()
                    (root / ('cs-word-es-load-cmp-logcpu.txt' if capture_cs_word_es_load else 'cs-memory-cmp-logcpu.txt')).write_bytes(data)
                    records['cs_cmp_log_sha256'] = hashlib.sha256(data).hexdigest()
                    timer_snapshot('cs_cmp_after', '378da2')
                    timer_memory('cs_cmp_after_bytes', '0180', '0039F9FE', 4)
                if capture_cs_word_es_load:
                    timer_run_to('cs_es_before', '00378DBA')
                    if timer_memory('cs_es_instruction_bytes', '0180', '00378DBA', 8) != bytes.fromhex('66 2e 8e 05 06 fa 39 00'):
                        raise RuntimeError('CS word ES 載入原始位元組不符')
                    timer_memory('cs_es_source_before', '0180', '0039FA06', 4)
                    cmd('BPDEL *')
                    log = pathlib.Path('LOGCPU.TXT'); log.unlink(missing_ok=True)
                    cmd('LOG 2', 5)
                    if not log.is_file() or not log.read_bytes().decode('latin1').startswith('0180:00378DBA'):
                        raise RuntimeError('CS word ES 載入有限起點不符')
                    data = log.read_bytes()
                    (root / 'cs-word-es-load-logcpu.txt').write_bytes(data)
                    records['cs_es_log_sha256'] = hashlib.sha256(data).hexdigest()
                    timer_snapshot('cs_es_after', '378dc2')
                    timer_memory('cs_es_source_after', '0180', '0039FA06', 4)
                timer_run_to('timer_wait_exit', '0036DB0B')
                timer_memory('timer_wait_source_after', '0188', '0039F148', 4)
        if capture_pit_mode2:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['pit_mode2_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:0036DAE8')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '36dae8']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 PIT 模式 2 候選未命中：' + repr(snapshots))
            records['pit_mode2_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:0036DAE8 20', 1)
            if not dump.is_file() or dump.stat().st_size != 32 or dump.read_bytes()[:14] != bytes.fromhex('e6 43 eb 00 88 d8 e6 40 eb 00 88 f8 e6 40'):
                raise RuntimeError('MOO2 PIT 模式 2 原始 bytes 不符')
            records['pit_mode2_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 8', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 PIT 三筆 OUT 未產生')
            data = log.read_bytes()
            lines = data.decode('latin1').splitlines()
            if len(lines) != 8 or not lines[0].startswith('0180:0036DAE8') or not lines[-1].startswith('0180:0036DAF6'):
                raise RuntimeError('MOO2 PIT 有限設定指令序列不符')
            records['pit_mode2_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'pit-mode2-logcpu.txt').write_bytes(data)
            snapshots = registers(cmd('EV ' + order, 0.8))
            configured = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '36daf6']), None)
            if not configured:
                raise RuntimeError('MOO2 PIT 三筆 OUT 後狀態未擷取')
            records['pit_mode2_configured'] = configured
            log.unlink()
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 PIT 第一個後續讀取未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:0036DAF6'):
                raise RuntimeError('MOO2 PIT 後續讀取起點不符')
            records['pit_mode2_consumer_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'pit-mode2-caller-logcpu.txt').write_bytes(data)
            snapshots = registers(cmd('EV ' + order, 0.8))
            consumed = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '36dafb']), None)
            if not consumed:
                raise RuntimeError('MOO2 PIT 後續讀取狀態未擷取')
            records['pit_mode2_consumer_after'] = consumed
        if capture_vtd_entry:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['vtd_entry_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:0036DA47')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '36da47']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 VTD 入口查詢候選未命中：' + repr(snapshots))
            records['vtd_entry_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            def sample_memory(address, count):
                dump.unlink(missing_ok=True)
                cmd('MEMDUMPBIN ' + address + ' ' + format(count, 'X'), 1)
                if not dump.is_file() or dump.stat().st_size != count:
                    raise RuntimeError('MOO2 VTD 原始記憶體未擷取')
                return dump.read_bytes()
            data = sample_memory('0180:0036DA47', 40)
            if data[:16] != bytes.fromhex('cd 2f 66 89 3d 22 9c 3d 00 66 c7 05 24 9c 3d 00'):
                raise RuntimeError('MOO2 VTD 原始 bytes 不符')
            records['vtd_entry_bytes_hex'] = data.hex()
            records['vtd_entry_memory_before_hex'] = sample_memory('0188:003D9C22', 6).hex()
            cmd('BPDEL *')
            cmd('BP 0180:0036DA49')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '36da49']), None)
            if not returned:
                raise RuntimeError('MOO2 VTD 返回位址未命中')
            records['vtd_entry_after'] = returned
            records['vtd_entry_memory_after_call_hex'] = sample_memory('0188:003D9C22', 6).hex()
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 8', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 VTD 入口保存未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:0036DA49'):
                raise RuntimeError('MOO2 VTD 保存起點不符')
            records['vtd_entry_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'vtd-entry-caller-logcpu.txt').write_bytes(data)
            snapshots = registers(cmd('EV ' + order, 0.8))
            consumed = next((v for v in reversed(snapshots) if len(v) == 16 and v[0] == '180'), None)
            if not consumed:
                raise RuntimeError('MOO2 VTD 保存後狀態未擷取')
            records['vtd_entry_consumer_after'] = consumed
            records['vtd_entry_memory_consumed_hex'] = sample_memory('0188:003D9C22', 6).hex()
        if capture_xor_word_register:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['xor_word_register_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:0036DA42')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '36da42']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 word XOR 候選未命中：' + repr(snapshots))
            records['xor_word_register_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:0036DA42 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes() != bytes.fromhex('66 31 ff 8e c7 cd 2f 66 89 3d 22 9c 3d 00 66 c7'):
                raise RuntimeError('MOO2 word XOR 原始 bytes 不符')
            records['xor_word_register_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:0036DA45')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '36da45']), None)
            if not returned:
                raise RuntimeError('MOO2 word XOR 下一 MOV ES 未命中')
            records['xor_word_register_after'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 word XOR 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:0036DA45'):
                raise RuntimeError('MOO2 word XOR 消費端起點不符')
            records['xor_word_register_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'xor-word-register-caller-logcpu.txt').write_bytes(data)
            snapshots = registers(cmd('EV ' + order, 0.8))
            consumed = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '36da47']), None)
            if not consumed or int(consumed[10], 16) != (int(returned[7], 16) & 0xffff):
                raise RuntimeError('MOO2 word XOR 的 MOV ES 未擷取或 selector 不符')
            records['xor_word_consumer_after'] = consumed
        if capture_short_sign_branches:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['short_sign_branches_order'] = order
            for name, address, expected_bytes, taken, fallthrough, sign_value in (
                ('js', '003502D6', '78 06 2b c2 79 02 eb ec 5e 61 c3 68 24 00 00 00', '3502de', '3502d8', True),
                ('jns', '003502DA', '79 02 eb ec 5e 61 c3 68', '3502de', '3502dc', False),
            ):
                cmd('BPDEL *')
                cmd('BP 0180:' + address)
                call = None
                for _ in range(6):
                    cmd('RUN', 12)
                    snapshots = registers(cmd('EV ' + order, 0.8))
                    call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', address.lower().lstrip('0')]), None)
                    if call:
                        break
                if not call:
                    raise RuntimeError('MOO2 短符號分支候選未命中：' + name + repr(snapshots))
                records[name + '_call'] = call
                dump = pathlib.Path('MEMDUMP.BIN')
                dump.unlink(missing_ok=True)
                size = len(bytes.fromhex(expected_bytes))
                cmd('MEMDUMPBIN 0180:' + address + ' ' + format(size, 'X'), 1)
                if not dump.is_file() or dump.read_bytes() != bytes.fromhex(expected_bytes):
                    raise RuntimeError('MOO2 短符號分支原始 bytes 不符：' + name)
                records[name + '_bytes_hex'] = dump.read_bytes().hex()
                cmd('BPDEL *')
                log = pathlib.Path('LOGCPU.TXT')
                log.unlink(missing_ok=True)
                cmd('LOG 2', 5)
                if not log.is_file():
                    raise RuntimeError('MOO2 短符號分支 LOG 未產生：' + name)
                data = log.read_bytes()
                if not data.decode('latin1').startswith('0180:' + address):
                    raise RuntimeError('MOO2 短符號分支 LOG 起點不符：' + name)
                records[name + '_log_sha256'] = hashlib.sha256(data).hexdigest()
                (root / ('short-sign-branches-' + name + '-logcpu.txt')).write_bytes(data)
                snapshots = registers(cmd('EV ' + order, 0.8))
                returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[0] == '180'), None)
                branch = bool(int(call[15], 16) & 0x80) == sign_value
                expected = taken if branch else fallthrough
                if not returned or returned[1] != expected or returned[2:] != call[2:]:
                    raise RuntimeError('MOO2 短符號分支後狀態或目的不符：' + name + repr(returned))
                records[name + '_after'] = returned
                records[name + '_taken'] = branch
        if capture_inc_word_memory:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['inc_word_memory_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:00350A3D')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '350a3d']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 word INC 候選未命中：' + repr(snapshots))
            records['inc_word_memory_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00350A3D 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes() != bytes.fromhex('66 ff 40 04 a1 a8 22 3d 00 8a 40 0b 24 20 25 ff'):
                raise RuntimeError('MOO2 word INC 原始 bytes 不符')
            records['inc_word_memory_bytes_hex'] = dump.read_bytes().hex()
            destination = (int(call[2], 16) + 4) & 0xffffffff
            records['inc_word_destination_offset_hex'] = format(destination, 'x')
            dump.unlink()
            cmd('MEMDUMPBIN ' + call[9] + ':' + format(destination, '08X') + ' 2', 1)
            if not dump.is_file() or dump.stat().st_size != 2:
                raise RuntimeError('MOO2 word INC輸入未擷取')
            records['inc_word_destination_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00350A41')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '350a41']), None)
            if not returned:
                raise RuntimeError('MOO2 word INC 下一指令未命中')
            records['inc_word_memory_after'] = returned
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN ' + returned[9] + ':' + format(destination, '08X') + ' 2', 1)
            if not dump.is_file() or dump.stat().st_size != 2:
                raise RuntimeError('MOO2 word INC遞增後資料未擷取')
            records['inc_word_destination_after_bytes_hex'] = dump.read_bytes().hex()
            source = int.from_bytes(bytes.fromhex(records['inc_word_memory_bytes_hex'])[5:9], 'little')
            records['inc_word_consumer_source_offset_hex'] = format(source, 'x')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN ' + returned[9] + ':' + format(source, '08X') + ' 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('MOO2 word INC 的 A1 輸入未擷取')
            records['inc_word_consumer_source_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 word INC 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:00350A41'):
                raise RuntimeError('MOO2 word INC 消費端起點不符')
            records['inc_word_memory_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'inc-word-memory-caller-logcpu.txt').write_bytes(data)
            snapshots = registers(cmd('EV ' + order, 0.8))
            consumed = next((v for v in reversed(snapshots) if len(v) == 16 and v[0] == '180'), None)
            if not consumed or consumed[1] != '350a46':
                raise RuntimeError('MOO2 word INC 的 A1 執行後未擷取')
            records['inc_word_consumer_after'] = consumed
            if int(consumed[2], 16) != int.from_bytes(bytes.fromhex(records['inc_word_consumer_source_bytes_hex']), 'little') or consumed[15] != returned[15]:
                raise RuntimeError('MOO2 word INC 的 A1 結果或旗標不符')
        if capture_vbe_window_control:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['vbe_window_control_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:0035CC54')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '35cc54']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 VBE 視窗控制 候選未命中：' + repr(snapshots))
            records['vbe_window_control_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:0035CC54 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes() != bytes.fromhex('cd 10 61 c3 60 25 ff ff 00 00 33 d2 bb 00 00 00'):
                raise RuntimeError('MOO2 VBE 視窗控制 原始 bytes 不符')
            records['vbe_window_control_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:0035CC56')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '35cc56']), None)
            if not returned:
                raise RuntimeError('MOO2 VBE 視窗控制 下一指令未命中')
            records['vbe_window_control_after'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 VBE 視窗控制 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:0035CC56'):
                raise RuntimeError('MOO2 VBE 視窗控制 消費端起點不符')
            records['vbe_window_control_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'vbe-window-control-caller-logcpu.txt').write_bytes(data)
        if capture_div_byte_register:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['div_byte_register_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:00356CCB')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '356ccb']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 byte DIV 候選未命中：' + repr(snapshots))
            records['div_byte_register_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00356CCB 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or dump.read_bytes() != bytes.fromhex('f6 f3 ee ac f6 e7 b3 64 f6 f3 ee ac f6 e7 b3 64'):
                raise RuntimeError('MOO2 byte DIV 原始 bytes 不符')
            records['div_byte_register_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00356CCD')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '356ccd']), None)
            if not returned:
                raise RuntimeError('MOO2 byte DIV 下一指令未命中')
            records['div_byte_register_after'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 byte DIV 第一個消費端未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:00356CCD'):
                raise RuntimeError('MOO2 byte DIV 消費端起點不符')
            records['div_byte_register_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'div-byte-register-caller-logcpu.txt').write_bytes(data)
        if capture_vga_pel_mask:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['vga_pel_mask_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:00356C9E')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '356c9e']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 VGA 像素遮罩候選未命中：' + repr(snapshots))
            if int(call[5],16) & 0xffff != 0x3c6 or int(call[2],16) & 0xff != 0xff:
                raise RuntimeError('MOO2 VGA 像素遮罩輸入不符')
            records['vga_pel_mask_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00356C9E 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or not dump.read_bytes().startswith(bytes.fromhex('ee e8 9d fe ff ff 66 ba c8 03')):
                raise RuntimeError('MOO2 VGA 像素遮罩原始 bytes 不符')
            records['vga_pel_mask_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00356C9F')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '356c9f']), None)
            if not returned:
                raise RuntimeError('MOO2 VGA 像素遮罩下一指令未命中')
            records['vga_pel_mask_after'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 VGA 像素遮罩後續紀錄未產生')
            data = log.read_bytes()
            if not data.decode('latin1').startswith('0180:00356C9F'):
                raise RuntimeError('MOO2 VGA 像素遮罩後續紀錄起點不符')
            records['vga_pel_mask_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'vga-pel-mask-caller-logcpu.txt').write_bytes(data)
        if capture_test_word_register:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['test_word_register_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:00248F43')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '248f43']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 word TEST 候選未命中：' + repr(snapshots))
            records['test_word_register_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00248F43 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or not dump.read_bytes().startswith(bytes.fromhex('66 85 c0 0f 85 f5 00 00 00')):
                raise RuntimeError('MOO2 word TEST 原始 bytes 不符')
            records['test_word_register_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00248F46')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '248f46']), None)
            if not returned:
                raise RuntimeError('MOO2 word TEST 下一指令未命中')
            records['test_word_register_after'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 word TEST 第一個分支紀錄未產生')
            data = log.read_bytes()
            lines = data.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:00248F46'):
                raise RuntimeError('MOO2 word TEST 分支紀錄起點不符')
            records['test_word_register_branch_lines'] = lines
            records['test_word_register_branch_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'test-word-register-caller-logcpu.txt').write_bytes(data)
        if capture_windows_version:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['windows_version_register_order'] = order
            cmd('BPDEL *')
            cmd('BPINT 2F 16 0A')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '34b888'] and int(v[2],16) & 0xffff == 0x160a), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 Windows 版本查詢入口未命中: ' + repr(snapshots))
            records['windows_version_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:0034B888 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or not dump.read_bytes().startswith(bytes.fromhex('cd 2f 83 f8 00 75 0d')):
                raise RuntimeError('MOO2 Windows 版本查詢原始指令不符')
            records['windows_version_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:0034B88A')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '34b88a']), None)
            if not returned:
                raise RuntimeError('MOO2 Windows 版本查詢返回未命中: ' + repr(snapshots))
            records['windows_version_return'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 40', 7)
            if not log.is_file():
                raise RuntimeError('MOO2 Windows 版本查詢 caller 紀錄未產生')
            data = log.read_bytes()
            lines = data.decode('latin1').splitlines()
            if len(lines) != 64 or not lines[0].startswith('0180:0034B88A'):
                raise RuntimeError('MOO2 Windows 版本查詢 caller 序列不符')
            records['windows_version_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            records['windows_version_caller_lines'] = lines
            (root / 'windows-version-caller-logcpu.txt').write_bytes(data)
        if capture_sar_stack_memory:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['sar_stack_memory_register_order'] = order
            cmd('BPDEL *')
            cmd('BP 0180:00334E5A')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '334e5a']), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 堆疊 SAR 指令入口未命中: ' + repr(snapshots))
            records['sar_stack_memory_call'] = call
            destination = f'{int(call[13],16):04X}:{(int(call[8],16)-12)&0xffffffff:08X}'
            records['sar_stack_memory_destination'] = destination
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN ' + destination + ' 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('堆疊 SAR 前 dword 未取得')
            records['sar_stack_memory_value_before_hex'] = dump.read_bytes().hex()
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00334E5A 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or not dump.read_bytes().startswith(bytes.fromhex('c1 7d f4 04')):
                raise RuntimeError('MOO2 堆疊 SAR 指令原始指令不符')
            records['sar_stack_memory_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00334E5E')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '334e5e']), None)
            if not returned:
                raise RuntimeError('MOO2 堆疊 SAR 指令返回未命中: ' + repr(snapshots))
            records['sar_stack_memory_return'] = returned
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN ' + destination + ' 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('堆疊 SAR 後 dword 未取得')
            records['sar_stack_memory_value_after_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 40', 7)
            if not log.is_file():
                raise RuntimeError('MOO2 堆疊 SAR 指令 caller 紀錄未產生')
            data = log.read_bytes()
            lines = data.decode('latin1').splitlines()
            if len(lines) != 64 or not lines[0].startswith('0180:00334E5E'):
                raise RuntimeError('MOO2 堆疊 SAR 指令 caller 序列不符')
            records['sar_stack_memory_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            records['sar_stack_memory_caller_lines'] = lines
            (root / 'sar-stack-memory-caller-logcpu.txt').write_bytes(data)
        if capture_free_memory:
            order = 'CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS'
            records['free_memory_register_order'] = order
            cmd('BPDEL *')
            cmd('BPINT 31 05 00')
            call = None
            for _ in range(6):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV ' + order, 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '380315'] and int(v[2],16) & 0xffff == 0x0500), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 DPMI 記憶體資訊查詢入口未命中: ' + repr(snapshots))
            records['free_memory_call'] = call
            destination = f'{int(call[10],16):04X}:{int(call[7],16):08X}'
            records['free_memory_destination'] = destination
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN ' + destination + ' 30', 1)
            if not dump.is_file() or dump.stat().st_size != 48:
                raise RuntimeError('DPMI 查詢前 48-byte buffer 未取得')
            records['free_memory_buffer_before_hex'] = dump.read_bytes().hex()
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:00380315 10', 1)
            if not dump.is_file() or dump.stat().st_size != 16 or not dump.read_bytes().startswith(bytes.fromhex('cd 31 c3')):
                raise RuntimeError('MOO2 DPMI 記憶體資訊查詢原始指令不符')
            records['free_memory_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            cmd('BP 0180:00380317')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '380317']), None)
            if not returned:
                raise RuntimeError('MOO2 DPMI 記憶體資訊查詢返回未命中: ' + repr(snapshots))
            records['free_memory_return'] = returned
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN ' + destination + ' 30', 1)
            if not dump.is_file() or dump.stat().st_size != 48:
                raise RuntimeError('DPMI 查詢後 48-byte buffer 未取得')
            records['free_memory_buffer_after_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 40', 7)
            if not log.is_file():
                raise RuntimeError('MOO2 DPMI 記憶體資訊查詢 caller 紀錄未產生')
            data = log.read_bytes()
            lines = data.decode('latin1').splitlines()
            if len(lines) != 64 or not lines[0].startswith('0180:00380317'):
                raise RuntimeError('MOO2 DPMI 記憶體資訊查詢 caller 序列不符')
            records['free_memory_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            records['free_memory_caller_lines'] = lines
            (root / 'free-memory-caller-logcpu.txt').write_bytes(data)
            cmd('BPDEL *')
            cmd('BP 0180:00334FD2')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + order, 0.8))
            consumer = next((v for v in reversed(snapshots) if len(v) == 16 and v[:2] == ['180', '334fd2']), None)
            if not consumer:
                raise RuntimeError('DPMI 查詢結果的外層 caller 未命中')
            records['free_memory_consumer_start'] = consumer
            cmd('BPDEL *')
            log.unlink(missing_ok=True)
            cmd('LOG 40', 7)
            data = log.read_bytes()
            lines = data.decode('latin1').splitlines()
            if len(lines) != 64 or not lines[0].startswith('0180:00334FD2'):
                raise RuntimeError('DPMI 查詢 consumer 序列不符')
            records['free_memory_consumer_log_sha256'] = hashlib.sha256(data).hexdigest()
            records['free_memory_consumer_lines'] = lines
            (root / 'free-memory-consumer-logcpu.txt').write_bytes(data)
        if capture_mouse_sequence:
            records['mouse_sequence_register_order'] = 'CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS'
            records['mouse_sequence'] = []
            for _ in range(10):
                cmd('BPDEL *')
                cmd('BPINT 33')
                cmd('RUN', 12)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 13 and v[:2] == ['180', '38031b']), None)
                if not call:
                    raise RuntimeError('MOO2 滑鼠序列入口未取得: ' + repr(snapshots))
                cmd('BPDEL *')
                cmd('BP 0180:0038031D')
                cmd('RUN', 8)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                returned = next((v for v in reversed(snapshots) if len(v) == 13 and v[:2] == ['180', '38031d']), None)
                if not returned:
                    raise RuntimeError('MOO2 滑鼠序列返回未取得: ' + repr(snapshots))
                records['mouse_sequence'].append({'call': call, 'return': returned})
                if int(call[2], 16) & 0xffff == 0x1b:
                    break
            else:
                raise RuntimeError('MOO2 滑鼠序列十筆內未到敏感度查詢')
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 40', 7)
            data = log.read_bytes()
            records['mouse_sequence_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            (root / 'mouse-sequence-caller-logcpu.txt').write_bytes(data)
        if capture_mouse_reset:
            function = 0x1b if capture_mouse_sensitivity else 0
            record_key = 'mouse_sensitivity' if capture_mouse_sensitivity else 'mouse_reset'
            if capture_mouse_set_sensitivity:
                function, record_key = 0x1a, 'mouse_set_sensitivity'
            if capture_mouse_set_position:
                function, record_key = 4, 'mouse_set_position'
            if capture_mouse_callback:
                function, record_key = 0x0c, 'mouse_callback'
            if capture_mouse_horizontal_range:
                function, record_key = 7, 'mouse_horizontal_range'
            if capture_mouse_vertical_range:
                function, record_key = 8, 'mouse_vertical_range'
            records[record_key + '_register_order'] = 'CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS'
            cmd('BPDEL *')
            cmd(f'BPINT 33 00 {function:02X}')
            call = None
            for _ in range(8):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                call = next((v for v in reversed(snapshots) if len(v) == 13 and v[:2] == ['180', '38031b'] and int(v[2], 16) & 0xffff == function), None)
                if call:
                    break
            if not call:
                raise RuntimeError('MOO2 滑鼠指定功能 未命中: ' + repr(snapshots))
            records[record_key + '_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd('MEMDUMPBIN 0180:0038031B 3', 1)
            if not dump.is_file() or dump.read_bytes() != bytes.fromhex('cd 33 c3'):
                raise RuntimeError('MOO2 滑鼠服務原始指令不符')
            records[record_key + '_bytes_hex'] = dump.read_bytes().hex()
            if capture_mouse_callback:
                callback_selector, callback_offset = int(call[9], 16), int(call[5], 16)
                records['mouse_callback_target'] = {'address_space': 'DOSBox-X ES:EDX', 'selector': callback_selector, 'offset': callback_offset}
                dump.unlink(missing_ok=True)
                cmd(f'MEMDUMPBIN {callback_selector:04X}:{callback_offset:08X} 40', 1)
                if not dump.is_file() or dump.stat().st_size != 64:
                    raise RuntimeError('MOO2 滑鼠回呼入口擷取失敗')
                callback_bytes = dump.read_bytes()
                records['mouse_callback_target_sha256'] = hashlib.sha256(callback_bytes).hexdigest()
                (root / (mode + 'target.bin')).write_bytes(callback_bytes)
            cmd('BPDEL *')
            cmd('BP 0180:0038031D')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
            returned = next((v for v in reversed(snapshots) if len(v) == 13 and v[:2] == ['180', '38031d']), None)
            if not returned:
                raise RuntimeError('MOO2 滑鼠服務返回未取得: ' + repr(snapshots))
            records[record_key + '_return'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 40', 7)
            if not log.is_file():
                raise RuntimeError('MOO2 滑鼠服務消費端 LOG 未產生')
            data = log.read_bytes()
            lines = data.decode('latin1').splitlines()
            if len(lines) != 64 or not lines[0].startswith('0180:0038031D'):
                raise RuntimeError('MOO2 滑鼠服務消費端序列不符')
            records[record_key + '_caller_log_sha256'] = hashlib.sha256(data).hexdigest()
            records[record_key + '_caller_lines'] = lines
            (root / (mode + 'caller-logcpu.txt')).write_bytes(data)
            if capture_mouse_callback_event:
                # 只送一般 X11 輸入，不改原版指令、狀態或回呼參數。
                command = ['xdotool', 'search', '--onlyvisible', '--class', 'dosbox']
                found = subprocess.run(command, check=True, capture_output=True, text=True, timeout=5)
                windows = found.stdout.split()
                if len(windows) != 1:
                    raise RuntimeError('MOO2 滑鼠事件要求唯一 DOSBox-X 視窗: ' + repr(windows))
                records['mouse_callback_input_window'] = windows[0]
                subprocess.run(['xdotool', 'windowfocus', windows[0]], check=True, timeout=5)
                cmd('BPDEL *')
                cmd(f'BP {callback_selector:04X}:{callback_offset:08X}')
                input_errors = []
                def inject_move():
                    try:
                        time.sleep(1)
                        subprocess.run(['xdotool', 'mousemove_relative', '--', '8', '6'], check=True, timeout=5)
                    except Exception as error:
                        input_errors.append(repr(error))
                worker = threading.Thread(target=inject_move)
                worker.start()
                cmd('RUN', 12)
                worker.join(timeout=8)
                if worker.is_alive() or input_errors:
                    raise RuntimeError('MOO2 滑鼠輸入送出失敗: ' + repr(input_errors))
                records['mouse_callback_input'] = {'method': 'X11 xdotool mousemove_relative', 'relative_x': 8, 'relative_y': 6}
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                entered = next((v for v in reversed(snapshots) if len(v) == 13 and int(v[0],16) == callback_selector and int(v[1],16) == callback_offset), None)
                if not entered:
                    raise RuntimeError('MOO2 一般移動後回呼入口未命中: ' + repr(snapshots))
                records['mouse_callback_event_entry'] = entered
                dump.unlink(missing_ok=True)
                cmd(f'MEMDUMPBIN {int(entered[10],16):04X}:{int(entered[11],16):08X} 40', 1)
                if not dump.is_file() or dump.stat().st_size != 64:
                    raise RuntimeError('MOO2 回呼堆疊擷取失敗')
                stack = dump.read_bytes()
                records['mouse_callback_event_stack_hex'] = stack.hex()
                (root / 'mouse-callback-event-stack.bin').write_bytes(stack)
                cmd('BPDEL *')
                log.unlink(missing_ok=True)
                cmd('LOG 100', 10)
                if not log.is_file():
                    raise RuntimeError('MOO2 回呼執行紀錄未產生')
                data = log.read_bytes()
                records['mouse_callback_event_log_sha256'] = hashlib.sha256(data).hexdigest()
                (root / 'mouse-callback-event-logcpu.txt').write_bytes(data)
                cmd('BP 0180:003477EC')
                cmd('RUN', 12)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                ending = next((v for v in reversed(snapshots) if len(v) == 13 and v[:2] == ['180', '3477ec']), None)
                if not ending:
                    raise RuntimeError('MOO2 回呼收尾入口未命中: ' + repr(snapshots))
                records['mouse_callback_event_ending'] = ending
                dump.unlink(missing_ok=True)
                cmd('MEMDUMPBIN 0180:003477EC 20', 1)
                if not dump.is_file() or dump.stat().st_size != 32:
                    raise RuntimeError('MOO2 回呼收尾指令擷取失敗')
                records['mouse_callback_event_ending_bytes_hex'] = dump.read_bytes().hex()
                cmd('BPDEL *')
                log.unlink(missing_ok=True)
                cmd('LOG 20', 7)
                if not log.is_file():
                    raise RuntimeError('MOO2 回呼返回紀錄未產生')
                data = log.read_bytes()
                records['mouse_callback_event_return_log_sha256'] = hashlib.sha256(data).hexdigest()
                (root / 'mouse-callback-event-return-logcpu.txt').write_bytes(data)
        if capture_or_register_imm8 or capture_xor_register_imm32:
            offset = 0x38621e if capture_or_register_imm8 else 0x384722
            expected_bytes = bytes.fromhex('83 c8 10' if capture_or_register_imm8 else '81 f2 00 80 00 00')
            record_key = 'or_register_imm8' if capture_or_register_imm8 else 'xor_register_imm32'
            filename = 'or-register-imm8' if capture_or_register_imm8 else 'xor-register-imm32'
            cmd('BPDEL *')
            cmd(f'BP 0180:{offset:08X}')
            hit = None
            for _ in range(3):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                hit = next((value for value in reversed(snapshots) if len(value) == 13 and value[:2] == ['180', f'{offset:x}']), None)
                if hit:
                    break
            if hit is None:
                raise RuntimeError('MOO2 ' + filename + ' 候選未命中')
            records[record_key + '_call'] = hit
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN 0180:{offset:08X} {len(expected_bytes):X}', 1)
            if not dump.is_file() or dump.read_bytes() != expected_bytes:
                raise RuntimeError('MOO2 ' + filename + ' 原始 bytes 不符')
            records[record_key + '_bytes_hex'] = dump.read_bytes().hex()
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 ' + filename + ' LOG 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith(f'0180:{offset:08X}') or not lines[1].startswith(f'0180:{offset + len(expected_bytes):08X}'):
                raise RuntimeError('MOO2 ' + filename + ' LOG 指令序列不符')
            records[record_key + '_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records[record_key + '_log_before'] = lines[0]
            records[record_key + '_log_after'] = lines[1]
            (root / (filename + '-logcpu.txt')).write_bytes(log_bytes)
            if capture_xor_register_imm32:
                snapshots = registers(cmd('EV CS EIP EDX EFLAGS', 0.8))
                records['xor_register_imm32_after_ev'] = snapshots[-1] if snapshots else None
                log.unlink(missing_ok=True)
                cmd('LOG 3', 5)
                if not log.is_file():
                    raise RuntimeError('MOO2 XOR 後續 LOG 未產生')
                continuation = log.read_bytes()
                records['xor_register_imm32_continuation_sha256'] = hashlib.sha256(continuation).hexdigest()
                records['xor_register_imm32_continuation_lines'] = continuation.decode('latin1').splitlines()
                (root / 'xor-register-imm32-continuation-logcpu.txt').write_bytes(continuation)
        if capture_add_al_imm8:
            cmd('BPDEL *')
            cmd('BP 0180:0038425F')
            hit = None
            for _ in range(3):
                cmd('RUN', 12)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                hit = next((value for value in reversed(snapshots) if len(value) == 13 and value[:2] == ['180', '38425f']), None)
                if hit:
                    break
            if hit is None:
                records['add_al_imm8_status'] = '原版在有界執行內未命中候選 CS:EIP'
            else:
                records['add_al_imm8_call'] = hit
                cmd('BPDEL *')
                log = pathlib.Path('LOGCPU.TXT')
                log.unlink(missing_ok=True)
                cmd('LOG 2', 5)
                if not log.is_file():
                    raise RuntimeError('MOO2 ADD AL,imm8 LOG 未產生')
                log_bytes = log.read_bytes()
                records['add_al_imm8_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
                (root / 'add-al-imm8-logcpu.txt').write_bytes(log_bytes)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                records['add_al_imm8_after_log'] = snapshots[-1] if snapshots else None
        if capture_video_mode_4f02:
            cmd('BPDEL *')
            cmd('BPINT 10 4F 02')
            cmd('RUN', 12)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
            call = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[2], 16) & 0xffff == 0x4f02), None)
            if not call:
                raise RuntimeError('MOO2 原版 INT 10h/AX=4F02h 未命中: ' + repr(snapshots))
            records['video_mode_4f02_call'] = call
            cmd('BPDEL *')
            cs, next_eip = int(call[0], 16), int(call[1], 16) + 2
            cmd(f'BP {cs:04X}:{next_eip:08X}')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
            returned = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[0], 16) == cs and int(value[1], 16) == next_eip), None)
            if not returned:
                raise RuntimeError('MOO2 INT 10h/AX=4F02h 返回未命中: ' + repr(snapshots))
            records['video_mode_4f02_return'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 80', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 4F02h caller LOG 未產生')
            log_bytes = log.read_bytes()
            records['video_mode_4f02_caller_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            (root / 'video-mode-4f02-caller-logcpu.txt').write_bytes(log_bytes)
        if capture_video_display_4f07:
            cmd('BPDEL *')
            cmd('BPINT 10 4F 07')
            cmd('RUN', 12)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
            call = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[2], 16) & 0xffff == 0x4f07), None)
            if not call:
                raise RuntimeError('MOO2 原版 INT 10h/AX=4F07h 未命中: ' + repr(snapshots))
            records['video_display_4f07_call'] = call
            cmd('BPDEL *')
            cs, next_eip = int(call[0], 16), int(call[1], 16) + 2
            cmd(f'BP {cs:04X}:{next_eip:08X}')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
            returned = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[0], 16) == cs and int(value[1], 16) == next_eip), None)
            if not returned:
                raise RuntimeError('MOO2 INT 10h/AX=4F07h 返回未命中: ' + repr(snapshots))
            records['video_display_4f07_return'] = returned
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 20', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 4F07h caller LOG 未產生')
            log_bytes = log.read_bytes()
            records['video_display_4f07_caller_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            (root / 'video-display-4f07-caller-logcpu.txt').write_bytes(log_bytes)
        if capture_real_video_4f01:
            records['real_video_4f01_events'] = []
            for index in range(12):
                cmd('BPDEL *')
                cmd('BPINT 31 03 00')
                cmd('RUN', 12)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                call = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[2], 16) & 0xffff == 0x0300 and int(value[3], 16) & 0xff == 0x10), None)
                if not call:
                    raise RuntimeError('MOO2 原版第 %d 筆 DPMI 0300h/INT 10h 未命中: %r' % (index + 1, snapshots))
                event = {'call': call, 'address_space': 'DOSBox-X CS:EIP'}
                records['real_video_4f01_events'].append(event)
                dump = pathlib.Path('MEMDUMP.BIN')
                dump.unlink(missing_ok=True)
                es, edi = int(call[9], 16), int(call[7], 16)
                cmd(f'MEMDUMPBIN {es:04X}:{edi:08X} 32', 1)
                if not dump.is_file() or dump.stat().st_size != 50:
                    raise RuntimeError('MOO2 VBE DPMI 輸入封包擷取失敗')
                packet_before = dump.read_bytes()
                event['packet_before_hex'] = packet_before.hex()
                ax = struct.unpack_from('<H', packet_before, 28)[0]
                if ax not in (0x4f00, 0x4f01) or (index == 0 and ax != 0x4f00):
                    raise RuntimeError('MOO2 VBE 呼叫序與預期不同: %04X' % ax)
                event['vbe_ax'] = ax
                if ax == 0x4f01:
                    segment = struct.unpack_from('<H', packet_before, 34)[0]
                    offset = struct.unpack_from('<H', packet_before, 0)[0]
                    linear = segment * 16 + offset
                    event['buffer_address'] = {'real_mode_segment': segment, 'real_mode_offset': offset, 'linear': linear, 'debugger_flat_selector': es}
                    if linear >= 0xa0000:
                        raise RuntimeError('MOO2 4F01h 緩衝區不在 DOS 傳統記憶體')
                    dump.unlink(missing_ok=True)
                    cmd(f'MEMDUMPBIN {es:04X}:{linear:08X} 100', 1)
                    if not dump.is_file() or dump.stat().st_size != 256:
                        raise RuntimeError('MOO2 4F01h 輸入緩衝擷取失敗')
                    data = dump.read_bytes()
                    event['buffer_before_sha256'] = hashlib.sha256(data).hexdigest()
                    (root / 'real-video-4f01-buffer-before.bin').write_bytes(data)
                cmd('BPDEL *')
                cs, next_eip = int(call[0], 16), int(call[1], 16) + 2
                cmd(f'BP {cs:04X}:{next_eip:08X}')
                cmd('RUN', 8)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                returned = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[0], 16) == cs and int(value[1], 16) == next_eip), None)
                if not returned:
                    raise RuntimeError('MOO2 VBE DPMI 返回未命中: ' + repr(snapshots))
                event['return'] = returned
                dump.unlink(missing_ok=True)
                es, edi = int(returned[9], 16), int(returned[7], 16)
                cmd(f'MEMDUMPBIN {es:04X}:{edi:08X} 32', 1)
                if not dump.is_file() or dump.stat().st_size != 50:
                    raise RuntimeError('MOO2 VBE DPMI 輸出封包擷取失敗')
                event['packet_after_hex'] = dump.read_bytes().hex()
                if ax == 0x4f01:
                    dump.unlink(missing_ok=True)
                    cmd(f'MEMDUMPBIN {es:04X}:{linear:08X} 100', 1)
                    if not dump.is_file() or dump.stat().st_size != 256:
                        raise RuntimeError('MOO2 4F01h 輸出緩衝擷取失敗')
                    data = dump.read_bytes()
                    event['buffer_after_sha256'] = hashlib.sha256(data).hexdigest()
                    (root / 'real-video-4f01-buffer-after.bin').write_bytes(data)
                    cmd('BPDEL *')
                    log = pathlib.Path('LOGCPU.TXT')
                    log.unlink(missing_ok=True)
                    cmd('LOG 80', 5)
                    if not log.is_file():
                        raise RuntimeError('MOO2 4F01h caller LOG 未產生')
                    data = log.read_bytes()
                    event['caller_log_sha256'] = hashlib.sha256(data).hexdigest()
                    (root / 'real-video-4f01-caller-logcpu.txt').write_bytes(data)
                    break
            else:
                raise RuntimeError('MOO2 前 12 筆 VBE 查詢未出現 4F01h')
        if capture_real_video_0300:
            cmd('BPDEL *')
            cmd('BPINT 31 03 00')
            cmd('RUN', 12)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
            call = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[2], 16) & 0xffff == 0x0300 and int(value[3], 16) & 0xff == 0x10), None)
            if not call:
                raise RuntimeError('MOO2 原版 DPMI 0300h/INT 10h 未命中: ' + repr(snapshots))
            records['real_video_0300_call'] = call
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            es, edi = int(call[9], 16), int(call[7], 16)
            cmd(f'MEMDUMPBIN {es:04X}:{edi:08X} 32', 1)
            if not dump.is_file() or dump.stat().st_size != 50:
                raise RuntimeError('MOO2 DPMI 0300h 輸入封包擷取失敗')
            packet_before = dump.read_bytes()
            records['real_video_0300_packet_before_hex'] = packet_before.hex()
            video_segment = struct.unpack_from('<H', packet_before, 34)[0]
            video_offset = struct.unpack_from('<H', packet_before, 0)[0]
            video_linear = video_segment * 16 + video_offset
            if video_linear >= 0xa0000:
                raise RuntimeError('MOO2 VESA 目標緩衝區不在 DOS 傳統記憶體')
            records['real_video_0300_buffer_address'] = {'real_mode_segment': video_segment, 'real_mode_offset': video_offset, 'linear': video_linear, 'debugger_flat_selector': es}
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {es:04X}:{video_linear:08X} 200', 1)
            if not dump.is_file() or dump.stat().st_size != 512:
                raise RuntimeError('MOO2 VESA 控制器資訊輸入緩衝擷取失敗')
            records['real_video_0300_buffer_before_sha256'] = hashlib.sha256(dump.read_bytes()).hexdigest()
            (root / 'real-video-0300-buffer-before.bin').write_bytes(dump.read_bytes())
            cmd('BPDEL *')
            cs, next_eip = int(call[0], 16), int(call[1], 16) + 2
            cmd(f'BP {cs:04X}:{next_eip:08X}')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
            returned = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[0], 16) == cs and int(value[1], 16) == next_eip), None)
            if not returned:
                raise RuntimeError('MOO2 DPMI 0300h 返回未命中: ' + repr(snapshots))
            records['real_video_0300_return'] = returned
            dump.unlink(missing_ok=True)
            es, edi = int(returned[9], 16), int(returned[7], 16)
            cmd(f'MEMDUMPBIN {es:04X}:{edi:08X} 32', 1)
            if not dump.is_file() or dump.stat().st_size != 50:
                raise RuntimeError('MOO2 DPMI 0300h 返回封包擷取失敗')
            packet_after = dump.read_bytes()
            records['real_video_0300_packet_after_hex'] = packet_after.hex()
            if (struct.unpack_from('<H', packet_after, 34)[0], struct.unpack_from('<H', packet_after, 0)[0]) != (video_segment, video_offset):
                raise RuntimeError('MOO2 VESA 輸出緩衝位址改變')
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {es:04X}:{video_linear:08X} 200', 1)
            if not dump.is_file() or dump.stat().st_size != 512:
                raise RuntimeError('MOO2 VESA 控制器資訊輸出緩衝擷取失敗')
            records['real_video_0300_buffer_after_sha256'] = hashlib.sha256(dump.read_bytes()).hexdigest()
            (root / 'real-video-0300-buffer-after.bin').write_bytes(dump.read_bytes())
            for label, linear in (('mode-list', 0xc0100), ('oem-string', 0xc016c)):
                dump.unlink(missing_ok=True)
                cmd(f'MEMDUMPBIN {es:04X}:{linear:08X} 80', 1)
                if not dump.is_file() or dump.stat().st_size != 128:
                    raise RuntimeError('MOO2 VESA ' + label + ' ROM 資料擷取失敗')
                content = dump.read_bytes()
                records['real_video_0300_' + label.replace('-', '_') + '_sha256'] = hashlib.sha256(content).hexdigest()
                (root / ('real-video-0300-' + label + '.bin')).write_bytes(content)
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 80', 5)
            if not log.is_file():
                raise RuntimeError('MOO2 VESA 呼叫端 LOG 未產生')
            log_bytes = log.read_bytes()
            records['real_video_0300_caller_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            (root / 'real-video-0300-caller-logcpu.txt').write_bytes(log_bytes)
        if capture_dos_memory_0100:
            records['dos_memory_0100_events'] = []
            cmd('BPDEL *')
            for _ in range(2):
                cmd('BPINT 31 01 00')
                cmd('RUN', 12)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                call = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[2], 16) & 0xffff == 0x0100), None)
                if not call:
                    records['dos_memory_0100_status'] = '有界執行未命中或 EV 無回應'
                    (root / 'dos-memory-0100-registers.json').write_text(json.dumps(records, indent=2))
                    raise RuntimeError('MOO2 原版 DPMI 0100h 入口未取得: ' + repr(snapshots))
                event = {'call': call, 'address_space': 'DOSBox-X CS:EIP'}
                records['dos_memory_0100_events'].append(event)
                cmd('BPDEL *')
                cs, next_eip = int(call[0], 16), int(call[1], 16) + 2
                cmd(f'BP {cs:04X}:{next_eip:08X}')
                cmd('RUN', 8)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                returned = next((value for value in reversed(snapshots) if len(value) == 13 and int(value[0], 16) == cs and int(value[1], 16) == next_eip), None)
                if not returned:
                    records['dos_memory_0100_status'] = '返回斷點未命中或 EV 無回應'
                    (root / 'dos-memory-0100-registers.json').write_text(json.dumps(records, indent=2))
                    raise RuntimeError('MOO2 原版 DPMI 0100h 返回未取得: ' + repr(snapshots))
                event['return'] = returned
                cmd('BPDEL *')
        if capture_full_data_test_word:
            cmd('BPDEL *')
            cmd('BP 0180:00375A21')
            cmd('BPINT 21 4C')
            cmd('RUN', 12)
            shot = root / 'full-data-test-word-screen.png'
            capture = subprocess.run(
                ['import', '-display', os.environ['DISPLAY'], '-window', 'root', str(shot)],
                capture_output=True, timeout=15, check=False,
            )
            records['full_data_screen'] = {
                'file': shot.name,
                'capture_returncode': capture.returncode,
                'sha256': hashlib.sha256(shot.read_bytes()).hexdigest() if shot.is_file() else None,
            }
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES SS ESP EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '375a21']), None)
            if not match:
                records['full_data_test_word_status'] = '有界執行未命中，不能推定原版未執行此指令'
                (root / 'full-data-test-word-registers.json').write_text(json.dumps(records, indent=2))
                raise RuntimeError('MOO2 完整資料 TEST word 停點未命中；可能先行退出: ' + repr(snapshots))
            records['full_data_test_word_before'] = match
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            if not log.is_file():
                raise RuntimeError('MOO2 TEST word 同次 LOG 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:00375A21') or not lines[1].startswith('0180:00375A2A'):
                raise RuntimeError('MOO2 TEST word 連續指令序列不符: ' + repr(lines))
            (root / 'full-data-test-word-logcpu.txt').write_bytes(log_bytes)
            records['full_data_test_word_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records['full_data_test_word_source'] = '尚未用同次指令 bytes 確認重定位來源位址'
        if capture_sbb:
            cmd('BPDEL *')
            cmd('BP 0180:003759EF')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES FS GS SS ESP EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '3759ef']), None)
            if not match:
                raise RuntimeError('MOO2 19 C0 前斷點未命中: ' + repr(snapshots))
            records['sbb_before'] = match
            cmd('BPDEL *')
            cmd('BP 0180:003759F1')
            cmd('RUN', 5)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES FS GS SS ESP EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '3759f1']), None)
            if not match:
                raise RuntimeError('MOO2 19 C0 後斷點未命中: ' + repr(snapshots))
            records['sbb_after'] = match
        if capture_sbb_word:
            cmd('BPDEL *')
            cmd('BP 0180:00377E84')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + records['register_order'], 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '377e84']), None)
            if not match:
                raise RuntimeError('MOO2 66 19 C0 前斷點未命中: ' + repr(snapshots))
            records['sbb_word_ev_before_address'] = match[:2]
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            snapshots = registers(cmd('EV ' + records['register_order'], 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '377e87']), None)
            if not match:
                raise RuntimeError('MOO2 66 19 C0 連續 LOG 後未到下一指令: ' + repr(snapshots))
            records['sbb_word_ev_after_address'] = match[:2]
            if not log.is_file():
                raise RuntimeError('MOO2 66 19 C0 LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:00377E84') or not lines[1].startswith('0180:00377E87'):
                raise RuntimeError('MOO2 66 19 C0 同次 LOG 指令序列不符: ' + repr(lines))
            (root / 'sbb-word-logcpu.txt').write_bytes(log_bytes)
            records['sbb_word_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records['sbb_word_log_before'] = lines[0]
            records['sbb_word_log_after'] = lines[1]
        if capture_low_entry:
            cmd('BPDEL *')
            cmd('BP 0180:00362F5C')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + records['register_order'], 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '362f5c']), None)
            if not match:
                raise RuntimeError('MOO2 低位址跳轉前斷點未命中: ' + repr(snapshots))
            records['low_entry_ev_before'] = match
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 20', 7)
            if not log.is_file():
                raise RuntimeError('MOO2 低位址跳轉 LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if not lines or not lines[0].startswith('0180:00362F5C'):
                raise RuntimeError('MOO2 低位址跳轉 LOG 起點不符: ' + repr(lines[:2]))
            (root / 'low-entry-logcpu.txt').write_bytes(log_bytes)
            records['low_entry_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records['low_entry_log_count'] = len(lines)
            records['low_entry_log_first'] = lines[0]
            records['low_entry_log_last'] = lines[-1]
        if capture_enter:
            cmd('BPDEL *')
            cmd('BP 0180:0023405B')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EBP ESP SS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '23405b']), None)
            if not match or len(match) != 6:
                raise RuntimeError('MOO2 C8 前斷點未命中: ' + repr(snapshots))
            esp, ss = int(match[3], 16), int(match[4], 16)
            if esp < 4:
                raise RuntimeError('MOO2 ENTER 堆疊不足')
            frame = esp - 4
            records['enter_before'] = match
            records['enter_frame_address'] = {'segment': f'{ss:04X}', 'offset': f'{frame:08X}'}
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ss:04X}:{frame:08X} 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('MOO2 ENTER 前堆疊擷取失敗')
            before = dump.read_bytes()
            (root / 'enter-stack-before.bin').write_bytes(before)
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            snapshots = registers(cmd('EV CS EIP EBP ESP SS EFLAGS', 0.8))
            after_match = next((value for value in snapshots if value[:2] == ['180', '23405f']), None)
            if not after_match:
                raise RuntimeError('MOO2 ENTER 後未到下一指令: ' + repr(snapshots))
            records['enter_after'] = after_match
            if not log.is_file():
                raise RuntimeError('MOO2 ENTER LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:0023405B') or not lines[1].startswith('0180:0023405F'):
                raise RuntimeError('MOO2 ENTER 同次 LOG 指令序列不符: ' + repr(lines))
            (root / 'enter-logcpu.txt').write_bytes(log_bytes)
            records['enter_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ss:04X}:{frame:08X} 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('MOO2 ENTER 後堆疊擷取失敗')
            after = dump.read_bytes()
            (root / 'enter-stack-after.bin').write_bytes(after)
            records['enter_stack_before_hex'] = before.hex()
            records['enter_stack_after_hex'] = after.hex()
        if capture_cmp_word:
            cmd('BPDEL *')
            cmd('BP 0180:002349FF')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP ECX EBP SS ESP EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '2349ff']), None)
            if not match or len(match) != 7:
                raise RuntimeError('MOO2 66 3B 4D CE 前斷點未命中: ' + repr(snapshots))
            ebp, ss = int(match[3], 16), int(match[4], 16)
            if ebp < 0x32:
                raise RuntimeError('MOO2 CMP EBP 位移下溢')
            offset = ebp - 0x32
            records['cmp_word_before'] = match
            records['cmp_word_memory_address'] = {'segment': f'{ss:04X}', 'offset': f'{offset:08X}'}
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ss:04X}:{offset:08X} 2', 1)
            if not dump.is_file() or dump.stat().st_size != 2:
                raise RuntimeError('MOO2 CMP 前記憶體擷取失敗')
            before = dump.read_bytes()
            (root / 'cmp-word-before.bin').write_bytes(before)
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            snapshots = registers(cmd('EV CS EIP ECX EBP SS ESP EFLAGS', 0.8))
            after_match = next((value for value in snapshots if value[:2] == ['180', '234a03']), None)
            if not after_match:
                raise RuntimeError('MOO2 CMP 連續 LOG 後未到下一指令: ' + repr(snapshots))
            records['cmp_word_after'] = after_match
            if not log.is_file():
                raise RuntimeError('MOO2 CMP LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:002349FF') or not lines[1].startswith('0180:00234A03'):
                raise RuntimeError('MOO2 CMP 同次 LOG 指令序列不符: ' + repr(lines))
            (root / 'cmp-word-logcpu.txt').write_bytes(log_bytes)
            records['cmp_word_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ss:04X}:{offset:08X} 2', 1)
            if not dump.is_file() or dump.stat().st_size != 2:
                raise RuntimeError('MOO2 CMP 後記憶體擷取失敗')
            after = dump.read_bytes()
            (root / 'cmp-word-after.bin').write_bytes(after)
            records['cmp_word_memory_before_hex'] = before.hex()
            records['cmp_word_memory_after_hex'] = after.hex()
        if capture_cmp_byte:
            cmd('BPDEL *')
            cmd('BP 0180:0036C224')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX EDX DS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '36c224']), None)
            if not match or len(match) != 6:
                raise RuntimeError('MOO2 38 10 候選前斷點未命中: ' + repr(snapshots))
            eax, ds = int(match[2], 16), int(match[4], 16)
            records['cmp_byte_before'] = match
            records['cmp_byte_memory_address'] = {'segment': f'{ds:04X}', 'offset': f'{eax:08X}'}
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ds:04X}:{eax:08X} 1', 1)
            if not dump.is_file() or dump.stat().st_size != 1:
                raise RuntimeError('MOO2 CMP byte 前記憶體擷取失敗')
            before = dump.read_bytes()
            (root / 'cmp-byte-before.bin').write_bytes(before)
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            snapshots = registers(cmd('EV CS EIP EAX EDX DS EFLAGS', 0.8))
            after_match = next((value for value in snapshots if value[:2] == ['180', '36c226']), None)
            if not after_match:
                raise RuntimeError('MOO2 CMP byte 後未到下一指令: ' + repr(snapshots))
            records['cmp_byte_after'] = after_match
            if not log.is_file():
                raise RuntimeError('MOO2 CMP byte LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:0036C224') or not lines[1].startswith('0180:0036C226'):
                raise RuntimeError('MOO2 CMP byte 同次 LOG 指令序列不符: ' + repr(lines))
            (root / 'cmp-byte-logcpu.txt').write_bytes(log_bytes)
            records['cmp_byte_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ds:04X}:{eax:08X} 1', 1)
            if not dump.is_file() or dump.stat().st_size != 1:
                raise RuntimeError('MOO2 CMP byte 後記憶體擷取失敗')
            after = dump.read_bytes()
            (root / 'cmp-byte-after.bin').write_bytes(after)
            records['cmp_byte_memory_before_hex'] = before.hex()
            records['cmp_byte_memory_after_hex'] = after.hex()
        if capture_lea_cs:
            cmd('BPDEL *')
            cmd('BP 0180:003801DF')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX ESI CS DS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '3801df']), None)
            if not match or len(match) != 7:
                raise RuntimeError('MOO2 2E 8D 候選斷點未命中: ' + repr(snapshots))
            records['lea_cs_ev_candidate'] = match
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            if not log.is_file():
                raise RuntimeError('MOO2 2E 8D LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:003801DF') or not lines[1].startswith('0180:003801E6'):
                raise RuntimeError('MOO2 2E 8D 連續 LOG 指令序列不符: ' + repr(lines))
            (root / 'lea-cs-logcpu.txt').write_bytes(log_bytes)
            records['lea_cs_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records['lea_cs_log_before'] = lines[0]
            records['lea_cs_log_after'] = lines[1]
        if capture_startup_value or capture_mouse_query or capture_mouse_function_21 or capture_mouse_function_1a or capture_video_mode_03 or capture_es_store:
            cmd('BPDEL *')
            cmd('BP 0180:00348105')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX ESI DS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '348105']), None)
            if not match:
                raise RuntimeError('MOO2 啟動值候選斷點未命中: ' + repr(snapshots))
            records['startup_value_ev_candidate'] = match
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 80', 8)
            if not log.is_file():
                raise RuntimeError('MOO2 啟動值 LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 128 or not lines[0].startswith('0180:00348105'):
                raise RuntimeError('MOO2 啟動值 LOG 指令序列不符: ' + repr(lines[:2]))
            first_lea = next((line for line in lines if line.startswith('0180:003801DF')), None)
            if not first_lea or ' EAX:00000033 ' not in first_lea or ' ESI:00000099 ' not in first_lea:
                raise RuntimeError('MOO2 第一次 LEA 輸入不符: ' + repr(first_lea))
            (root / (mode + 'logcpu.txt')).write_bytes(log_bytes)
            records['startup_value_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records['startup_value_first_lea'] = first_lea
            if capture_mouse_query or capture_mouse_function_21 or capture_mouse_function_1a or capture_video_mode_03 or capture_es_store:
                mouse_call = next((line for line in lines if line.startswith('0180:0038031B')), None)
                if not mouse_call or ' EAX:00000003 ' not in mouse_call:
                    raise RuntimeError('MOO2 滑鼠位置查詢輸入不符: ' + repr(mouse_call))
                records['mouse_query_log_before'] = mouse_call
                cmd('BP 0180:0038031D')
                cmd('RUN', 8)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                match = next((value for value in snapshots if value[:2] == ['180', '38031d']), None)
                if not match:
                    raise RuntimeError('MOO2 滑鼠查詢返回斷點未命中: ' + repr(snapshots))
                records['mouse_query_after'] = match
                if capture_mouse_function_21 or capture_mouse_function_1a or capture_video_mode_03:
                    prior_prefix = 'mouse-function-1a-prior-21-' if capture_mouse_function_1a else 'video-mode-03-prior-21-' if capture_video_mode_03 else 'mouse-function-21-'
                    cmd('BPDEL *')
                    cmd('BP 0180:0038031B')
                    cmd('RUN', 8)
                    snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                    match = next((value for value in snapshots if value[:2] == ['180', '38031b'] and int(value[2], 16) & 0xffff == 0x21), None)
                    if not match:
                        raise RuntimeError('MOO2 第二次滑鼠呼叫 AX=21h 未命中: ' + repr(snapshots))
                    records['mouse_function_21_ev_candidate'] = match
                    cmd('BPDEL *')
                    log.unlink(missing_ok=True)
                    cmd('LOG 200', 12)
                    if not log.is_file():
                        raise RuntimeError('MOO2 第二次滑鼠連續 LOGCPU.TXT 未產生')
                    return_log = log.read_bytes()
                    return_lines = return_log.decode('latin1').splitlines()
                    if len(return_lines) != 512 or not return_lines[0].startswith('0180:0038031B') or ' EAX:00000021 ' not in return_lines[0]:
                        raise RuntimeError('MOO2 第二次滑鼠連續指令起點不符: ' + repr(return_lines[:2]))
                    (root / (prior_prefix + 'call-logcpu.txt')).write_bytes(return_log)
                    records['mouse_function_21_call_log_sha256'] = hashlib.sha256(return_log).hexdigest()
                    cmd('BP 0180:0038031D')
                    cmd('RUN', 8)
                    cmd('BPDEL *')
                    log.unlink(missing_ok=True)
                    cmd('LOG 20', 6)
                    if not log.is_file():
                        raise RuntimeError('MOO2 第二次滑鼠返回 LOGCPU.TXT 未產生')
                    return_log = log.read_bytes()
                    return_lines = return_log.decode('latin1').splitlines()
                    if len(return_lines) != 32 or not return_lines[0].startswith('0180:0038031D'):
                        raise RuntimeError('MOO2 第二次滑鼠返回指令序列不符: ' + repr(return_lines[:2]))
                    records['mouse_function_21_return_log_line'] = return_lines[0]
                    (root / (prior_prefix + 'return-logcpu.txt')).write_bytes(return_log)
                    records['mouse_function_21_return_log_sha256'] = hashlib.sha256(return_log).hexdigest()
                    if capture_mouse_function_1a or capture_video_mode_03:
                        cmd('BP 0180:0038031B')
                        cmd('RUN', 8)
                        snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                        match = next((value for value in snapshots if value[:2] == ['180', '38031b'] and int(value[2], 16) & 0xffff == 0x1a), None)
                        if not match:
                            raise RuntimeError('MOO2 第三次滑鼠呼叫 AX=1Ah 未命中: ' + repr(snapshots))
                        records['mouse_function_1a_ev_candidate'] = match
                        cmd('BPDEL *')
                        log.unlink(missing_ok=True)
                        cmd('LOG 200', 12)
                        if not log.is_file():
                            raise RuntimeError('MOO2 AX=1Ah 連續 LOGCPU.TXT 未產生')
                        call_log = log.read_bytes()
                        call_lines = call_log.decode('latin1').splitlines()
                        if len(call_lines) != 512 or not call_lines[0].startswith('0180:0038031B') or ' EAX:0000001A ' not in call_lines[0]:
                            raise RuntimeError('MOO2 AX=1Ah 連續指令起點不符: ' + repr(call_lines[:2]))
                        (root / 'mouse-function-1a-call-logcpu.txt').write_bytes(call_log)
                        records['mouse_function_1a_call_log_sha256'] = hashlib.sha256(call_log).hexdigest()
                        cmd('BP 0180:0038031D')
                        cmd('RUN', 8)
                        cmd('BPDEL *')
                        log.unlink(missing_ok=True)
                        cmd('LOG 20', 6)
                        if not log.is_file():
                            raise RuntimeError('MOO2 AX=1Ah 返回 LOGCPU.TXT 未產生')
                        return_log = log.read_bytes()
                        return_lines = return_log.decode('latin1').splitlines()
                        if len(return_lines) != 32 or not return_lines[0].startswith('0180:0038031D'):
                            raise RuntimeError('MOO2 AX=1Ah 返回指令序列不符: ' + repr(return_lines[:2]))
                        records['mouse_function_1a_return_log_line'] = return_lines[0]
                        (root / 'mouse-function-1a-return-logcpu.txt').write_bytes(return_log)
                        records['mouse_function_1a_return_log_sha256'] = hashlib.sha256(return_log).hexdigest()
                        if capture_video_mode_03:
                            cmd('BP 0180:003802B2')
                            cmd('RUN', 8)
                            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                            match = next((value for value in snapshots if value[:2] == ['180', '3802b2']), None)
                            if not match:
                                raise RuntimeError('MOO2 INT 10h 模式設定呼叫未命中: ' + repr(snapshots))
                            records['video_mode_03_call'] = match
                            cmd('BPDEL *')
                            log.unlink(missing_ok=True)
                            cmd('LOG 80', 8)
                            call_log = log.read_bytes()
                            call_lines = call_log.decode('latin1').splitlines()
                            if len(call_lines) != 128 or not call_lines[0].startswith('0180:003802B2'):
                                raise RuntimeError('MOO2 INT 10h 指令序列不符: ' + repr(call_lines[:2]))
                            (root / 'video-mode-03-call-logcpu.txt').write_bytes(call_log)
                            records['video_mode_03_call_log_sha256'] = hashlib.sha256(call_log).hexdigest()
                            cmd('BP 0180:003802B4')
                            cmd('RUN', 8)
                            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX ESI EDI DS ES SS ESP EFLAGS', 0.8))
                            match = next((value for value in snapshots if value[:2] == ['180', '3802b4']), None)
                            if not match:
                                raise RuntimeError('MOO2 INT 10h 返回未命中: ' + repr(snapshots))
                            records['video_mode_03_return'] = match
                            cmd('BPDEL *')
                            log.unlink(missing_ok=True)
                            cmd('LOG 20', 6)
                            return_log = log.read_bytes()
                            return_lines = return_log.decode('latin1').splitlines()
                            if len(return_lines) != 32 or not return_lines[0].startswith('0180:003802B4'):
                                raise RuntimeError('MOO2 INT 10h 返回序列不符: ' + repr(return_lines[:2]))
                            (root / 'video-mode-03-return-logcpu.txt').write_bytes(return_log)
                            records['video_mode_03_return_log_sha256'] = hashlib.sha256(return_log).hexdigest()
                if capture_mouse_query:
                    cmd('BPDEL *')
                    log.unlink(missing_ok=True)
                    cmd('LOG 20', 6)
                    if not log.is_file():
                        raise RuntimeError('MOO2 滑鼠查詢後續 LOGCPU.TXT 未產生')
                    return_log = log.read_bytes()
                    return_lines = return_log.decode('latin1').splitlines()
                    if len(return_lines) != 32 or not return_lines[0].startswith('0180:0038031D'):
                        raise RuntimeError('MOO2 滑鼠查詢後續指令序列不符: ' + repr(return_lines[:2]))
                    (root / 'mouse-query-return-logcpu.txt').write_bytes(return_log)
                    records['mouse_query_return_log_sha256'] = hashlib.sha256(return_log).hexdigest()
                if capture_es_store:
                    cmd('BPDEL *')
                    cmd('BP 0180:003801D6')
                    cmd('RUN', 8)
                    snapshots = registers(cmd('EV CS EIP EBX ES DS EFLAGS', 0.8))
                    match = next((value for value in snapshots if value[:2] == ['180', '3801d6']), None)
                    if not match or len(match) != 6:
                        raise RuntimeError('MOO2 ES store 斷點未命中: ' + repr(snapshots))
                    records['es_store_ev_candidate'] = match
                    ebx, ds = int(match[2], 16), int(match[4], 16)
                    dump = pathlib.Path('MEMDUMP.BIN')
                    dump.unlink(missing_ok=True)
                    cmd(f'MEMDUMPBIN {ds:04X}:{ebx:08X} 2', 1)
                    if not dump.is_file() or dump.stat().st_size != 2:
                        raise RuntimeError('MOO2 ES store 前記憶體擷取失敗')
                    before = dump.read_bytes()
                    (root / 'es-store-before.bin').write_bytes(before)
                    cmd('BPDEL *')
                    log.unlink(missing_ok=True)
                    cmd('LOG 2', 6)
                    if not log.is_file():
                        raise RuntimeError('MOO2 ES store LOGCPU.TXT 未產生')
                    store_log = log.read_bytes()
                    store_lines = store_log.decode('latin1').splitlines()
                    if len(store_lines) != 2 or not store_lines[0].startswith('0180:003801D6') or not store_lines[1].startswith('0180:003801D9'):
                        raise RuntimeError('MOO2 ES store 指令序列不符: ' + repr(store_lines))
                    (root / 'es-store-logcpu.txt').write_bytes(store_log)
                    dump.unlink(missing_ok=True)
                    cmd(f'MEMDUMPBIN {ds:04X}:{ebx:08X} 2', 1)
                    if not dump.is_file() or dump.stat().st_size != 2:
                        raise RuntimeError('MOO2 ES store 後記憶體擷取失敗')
                    after = dump.read_bytes()
                    (root / 'es-store-after.bin').write_bytes(after)
                    records['es_store_before_hex'] = before.hex()
                    records['es_store_after_hex'] = after.hex()
                    records['es_store_log_sha256'] = hashlib.sha256(store_log).hexdigest()
        if capture_or_al_ah:
            cmd('BPDEL *')
            cmd('BP 0180:0036B01B')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '36b01b']), None)
            if not match or len(match) != 4:
                raise RuntimeError('MOO2 08 E0 候選斷點未命中: ' + repr(snapshots))
            records['or_al_ah_ev_candidate'] = match
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            if not log.is_file():
                raise RuntimeError('MOO2 08 E0 LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:0036B01B') or not lines[1].startswith('0180:0036B01D'):
                raise RuntimeError('MOO2 08 E0 連續 LOG 指令序列不符: ' + repr(lines))
            (root / 'or-al-ah-logcpu.txt').write_bytes(log_bytes)
            records['or_al_ah_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records['or_al_ah_log_before'] = lines[0]
            records['or_al_ah_log_after'] = lines[1]
        if capture_ror_imm8:
            cmd('BPDEL *')
            cmd('BP 0180:0036C22D')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EDX EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '36c22d']), None)
            if not match or len(match) != 4:
                raise RuntimeError('MOO2 C1 CA 08 候選斷點未命中: ' + repr(snapshots))
            records['ror_imm8_ev_candidate'] = match
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            if not log.is_file():
                raise RuntimeError('MOO2 C1 CA 08 LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:0036C22D') or not lines[1].startswith('0180:0036C230'):
                raise RuntimeError('MOO2 C1 CA 08 連續 LOG 指令序列不符: ' + repr(lines))
            (root / 'ror-imm8-logcpu.txt').write_bytes(log_bytes)
            records['ror_imm8_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records['ror_imm8_log_before'] = lines[0]
            records['ror_imm8_log_after'] = lines[1]
        if capture_es_byte_load:
            cmd('BPDEL *')
            cmd('BP 0180:0036A903')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EBX ESI ES DS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '36a903']), None)
            if not match or len(match) != 7:
                raise RuntimeError('MOO2 ES byte 載入候選前斷點未命中: ' + repr(snapshots))
            esi, es = int(match[3], 16), int(match[4], 16)
            records['es_byte_load_ev_candidate_before'] = match
            records['es_byte_load_memory_address'] = {'segment': f'{es:04X}', 'offset': f'{esi:08X}'}
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {es:04X}:{esi:08X} 1', 1)
            if not dump.is_file() or dump.stat().st_size != 1:
                raise RuntimeError('MOO2 ES byte 載入前記憶體擷取失敗')
            before = dump.read_bytes()
            (root / 'es-byte-load-before.bin').write_bytes(before)
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            snapshots = registers(cmd('EV CS EIP EBX ESI ES DS EFLAGS', 0.8))
            after_match = next((value for value in snapshots if value[:2] == ['180', '36a906']), None)
            if not after_match:
                raise RuntimeError('MOO2 ES byte 載入後未到下一指令: ' + repr(snapshots))
            records['es_byte_load_ev_after_log'] = after_match
            if not log.is_file():
                raise RuntimeError('MOO2 ES byte 載入 LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:0036A903') or not lines[1].startswith('0180:0036A906'):
                raise RuntimeError('MOO2 ES byte 載入同次 LOG 指令序列不符: ' + repr(lines))
            before_ebx = re.search(r' EBX:([0-9A-F]{8}) ', lines[0])
            after_ebx = re.search(r' EBX:([0-9A-F]{8}) ', lines[1])
            if not before_ebx or not after_ebx:
                raise RuntimeError('MOO2 ES byte LOG 暫存器欄缺失')
            records['es_byte_load_log_before_ebx'] = before_ebx.group(1)
            records['es_byte_load_log_after_ebx'] = after_ebx.group(1)
            records['es_byte_load_ev_log_before_ebx_conflict'] = int(match[2], 16) != int(before_ebx.group(1), 16)
            records['es_byte_load_sampling_limit'] = '候選 EV 前斷點與 LOG 首行 EBX 不同；不得將兩者串成同一次指令的前後態。'
            (root / 'es-byte-load-logcpu.txt').write_bytes(log_bytes)
            records['es_byte_load_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {es:04X}:{esi:08X} 1', 1)
            if not dump.is_file() or dump.stat().st_size != 1:
                raise RuntimeError('MOO2 ES byte 載入後記憶體擷取失敗')
            after = dump.read_bytes()
            (root / 'es-byte-load-after.bin').write_bytes(after)
            records['es_byte_load_memory_before_hex'] = before.hex()
            records['es_byte_load_memory_after_hex'] = after.hex()
        if capture_es_byte_load_ev:
            cmd('BPDEL *')
            cmd('BP 0180:0036A903')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EBX ESI ES DS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '36a903']), None)
            if not match or len(match) != 7:
                raise RuntimeError('MOO2 ES byte 載入 EV 前斷點未命中: ' + repr(snapshots))
            records['es_byte_load_ev_before'] = match
            cmd('BPDEL *')
            cmd('BP 0180:0036A906')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EBX ESI ES DS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '36a906']), None)
            if not match or len(match) != 7:
                raise RuntimeError('MOO2 ES byte 載入 EV 後斷點未命中: ' + repr(snapshots))
            records['es_byte_load_ev_after'] = match
        if capture_test_word:
            cmd('BPDEL *')
            cmd('BP 0180:0034A570')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP EAX EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '34a570']), None)
            if not match or len(match) != 4:
                raise RuntimeError('MOO2 66 A9 89 CF 候選前斷點未命中: ' + repr(snapshots))
            records['test_word_before'] = match
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            snapshots = registers(cmd('EV CS EIP EAX EFLAGS', 0.8))
            after_match = next((value for value in snapshots if value[:2] == ['180', '34a574']), None)
            if not after_match:
                raise RuntimeError('MOO2 66 A9 89 CF 連續 LOG 後未到下一指令: ' + repr(snapshots))
            records['test_word_after'] = after_match
            if not log.is_file():
                raise RuntimeError('MOO2 TEST LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:0034A570') or not lines[1].startswith('0180:0034A574'):
                raise RuntimeError('MOO2 TEST 同次 LOG 指令序列不符: ' + repr(lines))
            (root / 'test-word-logcpu.txt').write_bytes(log_bytes)
            records['test_word_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
        if capture_dta:
            cmd('BPDEL *')
            cmd('BP 0180:0035DA53')
            cmd('RUN', 10)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES SS ESP EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '35da53']), None)
            if not match:
                raise RuntimeError('MOO2 AH=1Ah 候選呼叫位址未命中: ' + repr(snapshots))
            records['dta_before'] = match
            cmd('BPDEL *')
            cmd('BP 0180:0035DA55')
            cmd('RUN', 6)
            snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES SS ESP EFLAGS', 0.8))
            after_match = next((value for value in snapshots if value[:2] == ['180', '35da55']), None)
            if not after_match:
                raise RuntimeError('MOO2 AH=1Ah 返回位址未命中: ' + repr(snapshots))
            records['dta_after'] = after_match
            if capture_dta_find:
                dta_seg, dta_off = int(match[6], 16), int(match[5], 16)
                records['dta_address'] = {'segment': f'{dta_seg:04X}', 'offset': f'{dta_off:08X}'}
                cmd('BPDEL *')
                cmd('BPINT 21 4E')
                found = None
                for _ in range(12):
                    cmd('RUN', 6)
                    snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES SS ESP EFLAGS', 0.8))
                    found = next((value for value in snapshots if value[0] == '180' and int(value[2], 16) >> 8 & 0xff == 0x4e), None)
                    if found:
                        break
                if not found:
                    raise RuntimeError('MOO2 AH=4Eh 首次搜尋未命中: ' + repr(snapshots))
                records['find_first_before'] = found
                dump = pathlib.Path('MEMDUMP.BIN')
                pattern_seg, pattern_off = int(found[7], 16), int(found[5], 16)
                records['find_first_pattern_address'] = {'segment': f'{pattern_seg:04X}', 'offset': f'{pattern_off:08X}'}
                dump.unlink(missing_ok=True)
                cmd(f'MEMDUMPBIN {pattern_seg:04X}:{pattern_off:08X} 80', 1)
                if not dump.is_file() or dump.stat().st_size != 128:
                    raise RuntimeError('MOO2 AH=4Eh 搜尋字串擷取失敗')
                pattern = dump.read_bytes().split(b'\x00', 1)[0]
                if len(pattern) == 128:
                    raise RuntimeError('MOO2 AH=4Eh 搜尋字串超出 128 bytes')
                (root / (mode + 'pattern.bin')).write_bytes(pattern + b'\x00')
                records['find_first_pattern_hex'] = pattern.hex()
                dump.unlink(missing_ok=True)
                cmd(f'MEMDUMPBIN {dta_seg:04X}:{dta_off:08X} 2B', 1)
                if not dump.is_file() or dump.stat().st_size != 43:
                    raise RuntimeError('MOO2 AH=4Eh 前 DTA 擷取失敗')
                before = dump.read_bytes()
                (root / (mode + 'before.bin')).write_bytes(before)
                cmd('BPDEL *')
                return_eip = int(found[1], 16) + 2
                cmd(f'BP 0180:{return_eip:08X}')
                cmd('RUN', 6)
                snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES SS ESP EFLAGS', 0.8))
                after_find = next((value for value in snapshots if value[:2] == ['180', f'{return_eip:x}']), None)
                if not after_find:
                    raise RuntimeError('MOO2 AH=4Eh 返回位址未命中: ' + repr(snapshots))
                records['find_first_after'] = after_find
                dump.unlink(missing_ok=True)
                cmd(f'MEMDUMPBIN {dta_seg:04X}:{dta_off:08X} 2B', 1)
                if not dump.is_file() or dump.stat().st_size != 43:
                    raise RuntimeError('MOO2 AH=4Eh 後 DTA 擷取失敗')
                after = dump.read_bytes()
                (root / (mode + 'after.bin')).write_bytes(after)
                records['dta_changed'] = before != after
                log = pathlib.Path('LOGCPU.TXT')
                log.unlink(missing_ok=True)
                cmd('BPDEL *')
                cmd('LOG 24', 6)
                if not log.is_file():
                    raise RuntimeError('MOO2 AH=4Eh 返回後 LOGCPU.TXT 未產生')
                log_bytes = log.read_bytes()
                lines = log_bytes.decode('latin1').splitlines()
                if not lines or not lines[0].startswith(f'0180:{return_eip:08X}'):
                    raise RuntimeError('MOO2 AH=4Eh 返回後指令起點不符: ' + repr(lines[:2]))
                (root / (mode + 'next-logcpu.txt')).write_bytes(log_bytes)
                records['find_first_next_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
                if capture_empty_mox_cmp:
                    cmd('BP 0180:002340CF')
                    cmd('RUN', 12)
                    snapshots = registers(cmd('EV CS EIP EAX EBX ECX EDX DS ES SS ESP EFLAGS', 0.8))
                    match = next((value for value in snapshots if value[:2] == ['180', '2340cf']), None)
                    if not match:
                        raise RuntimeError('MOO2 空 MOX.SET 比較停點未命中: ' + repr(snapshots))
                    records['empty_mox_cmp_before'] = match
                    cmd('BPDEL *')
                    log.unlink(missing_ok=True)
                    cmd('LOG 2', 6)
                    if not log.is_file():
                        raise RuntimeError('MOO2 空 MOX.SET 比較 LOG 未產生')
                    call_log = log.read_bytes()
                    call_lines = call_log.decode('latin1').splitlines()
                    if len(call_lines) != 2 or not call_lines[0].startswith('0180:002340CF'):
                        raise RuntimeError('MOO2 空 MOX.SET 比較序列不符: ' + repr(call_lines))
                    (root / 'empty-mox-cmp-logcpu.txt').write_bytes(call_log)
                    records['empty_mox_cmp_log_sha256'] = hashlib.sha256(call_log).hexdigest()
        if capture_xchg:
            for name, location in [('xchg_before', '37571d'), ('xchg_after', '37571f')]:
                cmd('BPDEL *')
                cmd('BP 0180:00' + location.upper())
                cmd('RUN', 8)
                snapshots = registers(cmd('EV ' + records['xchg_register_order'], 0.8))
                match = next((value for value in snapshots if value[:2] == ['180', location]), None)
                if not match:
                    raise RuntimeError('MOO2 87 FA 斷點未命中: ' + name + ' ' + repr(snapshots))
                records[name] = match
        if capture_cmc:
            for name, location in [('cmc_before', '3755ca'), ('cmc_after', '3755cb')]:
                cmd('BPDEL *')
                cmd('BP 0180:00' + location.upper())
                cmd('RUN', 8)
                snapshots = registers(cmd('EV ' + records['register_order'], 0.8))
                match = next((value for value in snapshots if value[:2] == ['180', location]), None)
                if not match:
                    raise RuntimeError('MOO2 F5 斷點未命中: ' + name + ' ' + repr(snapshots))
                records[name] = match
        if capture_and:
            cmd('BPDEL *')
            cmd('BP 0180:003755CD')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV ' + records['register_order'], 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '3755cd']), None)
            if not match:
                raise RuntimeError('MOO2 21 C8 前斷點未命中: ' + repr(snapshots))
            records['and_ev_before_address'] = match[:2]
            cmd('BPDEL *')
            cmd('LOG 2', 6)
            snapshots = registers(cmd('EV ' + records['register_order'], 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '3755cf']), None)
            if not match:
                raise RuntimeError('MOO2 21 C8 連續 LOG 後未到下一指令: ' + repr(snapshots))
            records['and_ev_after_address'] = match[:2]
            log = pathlib.Path('LOGCPU.TXT')
            if not log.is_file():
                raise RuntimeError('DOSBox-X LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            (root / 'and-logcpu.txt').write_bytes(log_bytes)
            records['and_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:003755CD  and  eax,ecx') or not lines[1].startswith('0180:003755CF  add  eax,edx'):
                raise RuntimeError('21 C8 同次 LOG 指令序列不符: ' + repr(lines))
            records['and_log_before'] = lines[0]
            records['and_log_after'] = lines[1]
        if capture_or_memory:
            cmd('BPDEL *')
            cmd('BP 0180:00375648')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP ESI DS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '375648']), None)
            if not match or len(match) != 5:
                raise RuntimeError('MOO2 83 0E 01 前斷點未命中: ' + repr(snapshots))
            esi, ds = int(match[2], 16), int(match[3], 16)
            records['or_memory_address'] = {'segment': f'{ds:04X}', 'offset': f'{esi:08X}'}
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ds:04X}:{esi:08X} 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('MOO2 83 0E 01 前記憶體擷取失敗')
            before = dump.read_bytes()
            (root / 'or-memory-before.bin').write_bytes(before)
            cmd('BPDEL *')
            cmd('LOG 2', 6)
            snapshots = registers(cmd('EV CS EIP ESI DS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '37564b']), None)
            if not match:
                raise RuntimeError('MOO2 83 0E 01 連續 LOG 後未到下一指令: ' + repr(snapshots))
            log = pathlib.Path('LOGCPU.TXT')
            if not log.is_file():
                raise RuntimeError('MOO2 83 0E 01 LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:00375648') or not lines[1].startswith('0180:0037564B'):
                raise RuntimeError('MOO2 83 0E 01 同次 LOG 指令序列不符: ' + repr(lines))
            logged_esi = re.search(r' ESI:([0-9A-F]{8}) ', lines[0])
            logged_ds = re.search(r' DS:([0-9A-F]{4}) ', lines[0])
            if not logged_esi or not logged_ds or int(logged_esi.group(1), 16) != esi or int(logged_ds.group(1), 16) != ds:
                raise RuntimeError('MOO2 83 0E 01 EV 地址與 LOG 不一致')
            (root / 'or-memory-logcpu.txt').write_bytes(log_bytes)
            records['or_memory_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records['or_memory_log_before'] = lines[0]
            records['or_memory_log_after'] = lines[1]
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ds:04X}:{esi:08X} 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('MOO2 83 0E 01 後記憶體擷取失敗')
            after = dump.read_bytes()
            (root / 'or-memory-after.bin').write_bytes(after)
            records['or_memory_before_hex'] = before.hex()
            records['or_memory_after_hex'] = after.hex()
        if capture_pop_gs:
            cmd('BPDEL *')
            cmd('BP 0180:00360C50')
            cmd('RUN', 8)
            snapshots = registers(cmd('EV CS EIP GS ESP SS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '360c50']), None)
            if not match or len(match) != 6:
                raise RuntimeError('MOO2 0F A9 前斷點未命中: ' + repr(snapshots))
            esp, ss = int(match[3], 16), int(match[4], 16)
            records['pop_gs_stack_address'] = {'segment': f'{ss:04X}', 'offset': f'{esp:08X}'}
            dump = pathlib.Path('MEMDUMP.BIN')
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ss:04X}:{esp:08X} 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('MOO2 0F A9 前堆疊擷取失敗')
            before = dump.read_bytes()
            (root / 'pop-gs-stack-before.bin').write_bytes(before)
            cmd('BPDEL *')
            log = pathlib.Path('LOGCPU.TXT')
            log.unlink(missing_ok=True)
            cmd('LOG 2', 6)
            snapshots = registers(cmd('EV CS EIP GS ESP SS EFLAGS', 0.8))
            match = next((value for value in snapshots if value[:2] == ['180', '360c52']), None)
            if not match:
                raise RuntimeError('MOO2 0F A9 連續 LOG 後未到下一指令: ' + repr(snapshots))
            if not log.is_file():
                raise RuntimeError('MOO2 0F A9 LOGCPU.TXT 未產生')
            log_bytes = log.read_bytes()
            lines = log_bytes.decode('latin1').splitlines()
            if len(lines) != 2 or not lines[0].startswith('0180:00360C50') or not lines[1].startswith('0180:00360C52'):
                raise RuntimeError('MOO2 0F A9 同次 LOG 指令序列不符: ' + repr(lines))
            logged_esp = re.search(r' ESP:([0-9A-F]{8}) ', lines[0])
            logged_ss = re.search(r' SS:([0-9A-F]{4}) ', lines[0])
            if not logged_esp or not logged_ss or int(logged_esp.group(1), 16) != esp or int(logged_ss.group(1), 16) != ss:
                raise RuntimeError('MOO2 0F A9 EV 堆疊地址與 LOG 不一致')
            (root / 'pop-gs-logcpu.txt').write_bytes(log_bytes)
            records['pop_gs_log_sha256'] = hashlib.sha256(log_bytes).hexdigest()
            records['pop_gs_log_before'] = lines[0]
            records['pop_gs_log_after'] = lines[1]
            dump.unlink(missing_ok=True)
            cmd(f'MEMDUMPBIN {ss:04X}:{esp:08X} 4', 1)
            if not dump.is_file() or dump.stat().st_size != 4:
                raise RuntimeError('MOO2 0F A9 後堆疊擷取失敗')
            after = dump.read_bytes()
            (root / 'pop-gs-stack-after.bin').write_bytes(after)
            records['pop_gs_stack_before_hex'] = before.hex()
            records['pop_gs_stack_after_hex'] = after.hex()
        (root / (mode + 'registers.json' if mode else 'startup-registers.json')).write_text(json.dumps(records, indent=2))
    finally:
        (root / (mode + 'commands.json')).write_text(json.dumps(commands, indent=2))
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
