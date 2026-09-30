"""重播固定 MOO2 1.31 的 DOSBox-X 輔助啟動與 DPMI 快照。

容器內需要 /tmp/game/ORION2.EXE、可寫 /shots、DISPLAY 與外層 Xvfb trap。
原版檔案及輸出的完整終端／記憶體資料不可加入公開版控。
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

root = pathlib.Path('/shots')
if sys.argv[1:] not in ([], ['--sbb'], ['--sbb-word'], ['--low-entry'], ['--enter'], ['--cmp-word'], ['--cmp-byte'], ['--lea-cs'], ['--or-al-ah'], ['--ror-imm8'], ['--es-byte-load'], ['--es-byte-load-ev'], ['--test-word'], ['--dta'], ['--dta-find'], ['--dta-find-present'], ['--xchg'], ['--cmc'], ['--and'], ['--or-memory'], ['--pop-gs']):
    raise SystemExit('usage: startup_probe_131.py [--sbb|--sbb-word|--low-entry|--enter|--cmp-word|--cmp-byte|--lea-cs|--or-al-ah|--ror-imm8|--es-byte-load|--es-byte-load-ev|--test-word|--dta|--dta-find|--dta-find-present|--xchg|--cmc|--and|--or-memory|--pop-gs]')
capture_sbb = sys.argv[1:] == ['--sbb']
capture_sbb_word = sys.argv[1:] == ['--sbb-word']
capture_low_entry = sys.argv[1:] == ['--low-entry']
capture_enter = sys.argv[1:] == ['--enter']
capture_cmp_word = sys.argv[1:] == ['--cmp-word']
capture_cmp_byte = sys.argv[1:] == ['--cmp-byte']
capture_lea_cs = sys.argv[1:] == ['--lea-cs']
capture_or_al_ah = sys.argv[1:] == ['--or-al-ah']
capture_ror_imm8 = sys.argv[1:] == ['--ror-imm8']
capture_es_byte_load = sys.argv[1:] == ['--es-byte-load']
capture_es_byte_load_ev = sys.argv[1:] == ['--es-byte-load-ev']
capture_test_word = sys.argv[1:] == ['--test-word']
capture_dta = sys.argv[1:] in (['--dta'], ['--dta-find'], ['--dta-find-present'])
capture_dta_find = sys.argv[1:] in (['--dta-find'], ['--dta-find-present'])
capture_dta_find_present = sys.argv[1:] == ['--dta-find-present']
capture_xchg = sys.argv[1:] == ['--xchg']
capture_cmc = sys.argv[1:] == ['--cmc']
capture_and = sys.argv[1:] == ['--and']
capture_or_memory = sys.argv[1:] == ['--or-memory']
capture_pop_gs = sys.argv[1:] == ['--pop-gs']
mode = 'dta-find-present-' if capture_dta_find_present else 'dta-find-' if capture_dta_find else 'dta-' if capture_dta else 'test-word-' if capture_test_word else 'lea-cs-' if capture_lea_cs else 'or-al-ah-' if capture_or_al_ah else 'ror-imm8-' if capture_ror_imm8 else 'es-byte-load-ev-' if capture_es_byte_load_ev else 'es-byte-load-' if capture_es_byte_load else 'cmp-byte-' if capture_cmp_byte else 'cmp-word-' if capture_cmp_word else 'enter-' if capture_enter else 'low-entry-' if capture_low_entry else 'sbb-word-' if capture_sbb_word else 'pop-gs-' if capture_pop_gs else 'or-memory-' if capture_or_memory else 'and-' if capture_and else 'cmc-' if capture_cmc else 'xchg-' if capture_xchg else 'sbb-' if capture_sbb else ''
exe = pathlib.Path('/tmp/game/ORION2.EXE')
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
