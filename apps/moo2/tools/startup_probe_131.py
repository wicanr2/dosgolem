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
if sys.argv[1:] not in ([], ['--sbb'], ['--sbb-word'], ['--low-entry'], ['--enter'], ['--cmp-word'], ['--xchg'], ['--cmc'], ['--and'], ['--or-memory'], ['--pop-gs']):
    raise SystemExit('usage: startup_probe_131.py [--sbb|--sbb-word|--low-entry|--enter|--cmp-word|--xchg|--cmc|--and|--or-memory|--pop-gs]')
capture_sbb = sys.argv[1:] == ['--sbb']
capture_sbb_word = sys.argv[1:] == ['--sbb-word']
capture_low_entry = sys.argv[1:] == ['--low-entry']
capture_enter = sys.argv[1:] == ['--enter']
capture_cmp_word = sys.argv[1:] == ['--cmp-word']
capture_xchg = sys.argv[1:] == ['--xchg']
capture_cmc = sys.argv[1:] == ['--cmc']
capture_and = sys.argv[1:] == ['--and']
capture_or_memory = sys.argv[1:] == ['--or-memory']
capture_pop_gs = sys.argv[1:] == ['--pop-gs']
mode = 'cmp-word-' if capture_cmp_word else 'enter-' if capture_enter else 'low-entry-' if capture_low_entry else 'sbb-word-' if capture_sbb_word else 'pop-gs-' if capture_pop_gs else 'or-memory-' if capture_or_memory else 'and-' if capture_and else 'cmc-' if capture_cmc else 'xchg-' if capture_xchg else 'sbb-' if capture_sbb else ''
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
