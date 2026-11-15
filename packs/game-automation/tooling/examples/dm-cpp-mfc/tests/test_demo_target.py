"""dm_demo_target 最小管道协议测试：只以 --hidden + 父 stdin 管道驱动本工程编译的
目标 EXE，不开启可见窗口、不用真实鼠标键盘、不 PrintWindow、不加载任何 SDK。

覆盖：ready 身份（exact HWND/PID 同一性且永不可见）、stats 计数（仅 VK_A）、
close / stdin EOF 正常退出、无效命令与 --hidden 无管道的 fail-closed 拒绝。
合成输入一律 PostMessage 到协议返回的本子进程自有 HWND。
"""
import argparse
import ctypes
import os
from pathlib import Path
import queue
import subprocess
import threading
import time
import unittest
from ctypes import wintypes

PARSER = argparse.ArgumentParser()
PARSER.add_argument("--target", required=True)
OPTIONS, REST = PARSER.parse_known_args()
TARGET = str(Path(OPTIONS.target).resolve())

USER32 = ctypes.WinDLL("user32", use_last_error=True)
USER32.IsWindow.argtypes = [wintypes.HWND]
USER32.IsWindow.restype = wintypes.BOOL
USER32.IsWindowVisible.argtypes = [wintypes.HWND]
USER32.IsWindowVisible.restype = wintypes.BOOL
USER32.GetWindowThreadProcessId.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.DWORD)]
USER32.GetWindowThreadProcessId.restype = wintypes.DWORD
USER32.PostMessageW.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM]
USER32.PostMessageW.restype = wintypes.BOOL

WM_KEYDOWN, WM_KEYUP, WM_MOUSEMOVE, WM_LBUTTONDOWN, WM_LBUTTONUP = 0x100, 0x101, 0x200, 0x201, 0x202


def parse_ready(line):
    parts = line.split(" ")
    if len(parts) != 4 or parts[:2] != ["TARGET", "ready"]:
        return None
    try:
        return int(parts[2]), int(parts[3])  # hwnd, pid
    except ValueError:
        return None


def parse_stats(line):
    parts = line.split(" ")
    if len(parts) != 7 or parts[:2] != ["TARGET", "stats"]:
        return None
    try:
        return [int(p) for p in parts[2:]]  # moves, clickDown, clickUp, keyDown, keyUp
    except ValueError:
        return None


class DemoTarget:
    def __init__(self):
        self.process = subprocess.Popen(
            [TARGET, "--hidden"], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT, text=True, encoding="utf-8",
            creationflags=subprocess.CREATE_NO_WINDOW,
        )
        self.lines = []
        self.pending = queue.Queue()
        self.reader = threading.Thread(target=self._read, daemon=True)
        self.reader.start()

    def _read(self):
        for raw in self.process.stdout:
            line = raw.rstrip()
            self.lines.append(line)
            self.pending.put(line)

    def send(self, command):
        self.process.stdin.write(command + "\n")
        self.process.stdin.flush()

    def until(self, prefix, timeout=3):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            try:
                line = self.pending.get(timeout=max(.01, deadline - time.monotonic()))
            except queue.Empty:
                break
            if line.startswith(prefix):
                return line
        raise AssertionError(f"Missing {prefix!r}: {self.lines}")

    def wait(self):
        code = self.process.wait(timeout=3)
        self.reader.join(timeout=1)
        return code

    def close(self):
        if self.process.poll() is None:
            try:
                self.send("close")
                self.process.wait(timeout=3)
            except (OSError, subprocess.TimeoutExpired):
                self.process.kill()  # 仅清理测试子进程，不作为正常停止成功证据。
                self.process.wait(timeout=3)
        self.reader.join(timeout=1)
        self.process.stdin.close()
        self.process.stdout.close()


class DemoTargetTests(unittest.TestCase):
    def target(self):
        child = DemoTarget()
        self.addCleanup(child.close)
        return child

    def ready(self, child):
        hwnd, pid = parse_ready(child.until("TARGET ready")) or (0, 0)
        self.assertGreater(hwnd, 0)
        self.assertEqual(pid, child.process.pid)
        owner = wintypes.DWORD(0)
        USER32.GetWindowThreadProcessId(hwnd, ctypes.byref(owner))
        self.assertEqual(owner.value, pid)
        return hwnd

    def test_ready_identity_and_hidden(self):
        child = self.target()
        hwnd = self.ready(child)
        time.sleep(.2)  # ready 之后仍必须保持永不显示。
        self.assertFalse(USER32.IsWindowVisible(hwnd))

    def test_stats_counts_only_vk_a(self):
        child = self.target()
        hwnd = self.ready(child)
        child.send("stats")
        self.assertEqual(parse_stats(child.until("TARGET stats")), [0, 0, 0, 0, 0])
        for message, wparam in [(WM_MOUSEMOVE, 0), (WM_LBUTTONDOWN, 0), (WM_LBUTTONUP, 0),
                                (WM_KEYDOWN, 0x41), (WM_KEYUP, 0x41),
                                (WM_KEYDOWN, 0x42), (WM_KEYUP, 0x42)]:
            self.assertTrue(USER32.PostMessageW(hwnd, message, wparam, 0))
        time.sleep(.15)
        child.send("stats")
        self.assertEqual(parse_stats(child.until("TARGET stats")), [1, 1, 1, 1, 1])

    def test_close_exits_zero_and_destroys_window(self):
        child = self.target()
        hwnd = self.ready(child)
        child.send("close")
        self.assertEqual(child.wait(), 0)
        self.assertFalse(USER32.IsWindow(hwnd))

    def test_stdin_eof_exits_zero(self):
        child = self.target()
        self.ready(child)
        child.process.stdin.close()
        self.assertEqual(child.wait(), 0)

    def test_invalid_command_stops_with_error(self):
        child = self.target()
        self.ready(child)
        child.send("arbitrary-command")
        self.assertEqual(child.wait(), 2)
        self.assertTrue(any(line.startswith("ERROR invalid command") for line in child.lines))
        # stdout 侧只允许出现过唯一一行 TARGET 行（启动 ready），错误细节走 stderr。
        target_lines = [line for line in child.lines if line.startswith("TARGET")]
        self.assertEqual(len(target_lines), 1)
        self.assertTrue(target_lines[0].startswith("TARGET ready"))

    def test_overlong_single_write_stops(self):
        # 回归：单次写入超过 64 字节的非法行必须按协议错误退出 2，
        # 不得超出读缓冲或阻塞等待读满请求字节数。
        child = self.target()
        self.ready(child)
        child.process.stdin.write("x" * 100 + "\n")
        child.process.stdin.flush()
        self.assertEqual(child.wait(), 2)
        self.assertTrue(any(line.startswith("ERROR command too long") for line in child.lines))

    def test_hidden_requires_parent_pipe(self):
        result = subprocess.run(
            [TARGET, "--hidden"], stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, text=True, encoding="utf-8", timeout=10,
            creationflags=subprocess.CREATE_NO_WINDOW,
        )
        self.assertEqual(result.returncode, 2)
        self.assertNotIn("ready", result.stdout)
        self.assertTrue(result.stderr.startswith("ERROR"))


if __name__ == "__main__":
    unittest.main(argv=[__file__, *REST])
