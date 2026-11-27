"""dm_demo_target 最小管道协议测试：只以 --hidden + 父 stdin 管道驱动本工程编译的
目标 EXE，不开启可见窗口、不用真实鼠标键盘、不 PrintWindow、不加载任何 SDK。

覆盖：ready 身份（exact HWND/PID 同一性且永不可见）、stats 计数（仅 VK_A）、
close / stdin EOF 正常退出、无效命令与 --hidden 无管道的 fail-closed 拒绝、
--pages 页签模式（只有页签内的按下切页、其它区域只计数、非法参数创建窗口前退出 2）。
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
USER32.GetClientRect.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.RECT)]
USER32.PostMessageW.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM]
USER32.PostMessageW.restype = wintypes.BOOL
# Python 进程默认 DPI-unaware：读目标几何会被虚拟化缩放（144 DPI 下 200 -> 133），而且从
# unaware 线程 PostMessage 的鼠标坐标也会被系统按 DPI 放大（实测 99 -> 148）。几何断言与
# 合成点击都切到目标窗口自己的 DPI 感知上下文再做。
USER32.GetWindowDpiAwarenessContext.argtypes = [wintypes.HWND]
USER32.GetWindowDpiAwarenessContext.restype = wintypes.HANDLE
USER32.SetThreadDpiAwarenessContext.argtypes = [wintypes.HANDLE]
USER32.SetThreadDpiAwarenessContext.restype = wintypes.HANDLE

WM_KEYDOWN, WM_KEYUP, WM_MOUSEMOVE, WM_LBUTTONDOWN, WM_LBUTTONUP = 0x100, 0x101, 0x200, 0x201, 0x202
# --pages 3 的布局：客户区 200x（150 色块 + 28 页签 + 40 状态条），页签宽 200/3，页签高 28。
TAB_2, TAB_3, CONTENT = (99, 14), (165, 14), (10, 100)


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


def parse_page(line):
    parts = line.split(" ")
    if len(parts) != 3 or parts[:2] != ["TARGET", "page"]:
        return None
    try:
        return int(parts[2])  # 当前页（1 起）
    except ValueError:
        return None


class DemoTarget:
    def __init__(self, *arguments):
        self.process = subprocess.Popen(
            [TARGET, "--hidden", *arguments], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
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
    def target(self, *arguments):
        child = DemoTarget(*arguments)
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

    @staticmethod
    def in_target_dpi(hwnd, action):
        previous = USER32.SetThreadDpiAwarenessContext(USER32.GetWindowDpiAwarenessContext(hwnd))
        try:
            return action()
        finally:
            USER32.SetThreadDpiAwarenessContext(previous)

    @classmethod
    def click(cls, hwnd, x, y):
        def post():
            lparam = (y << 16) | (x & 0xFFFF)
            assert USER32.PostMessageW(hwnd, WM_LBUTTONDOWN, 1, lparam)
            assert USER32.PostMessageW(hwnd, WM_LBUTTONUP, 0, lparam)
        cls.in_target_dpi(hwnd, post)
        time.sleep(.1)  # 目标先排空窗口消息再应答管道：下一条 page/stats 应答已包含这次点击。

    @staticmethod
    def page(child):
        child.send("page")
        return parse_page(child.until("TARGET page"))

    @staticmethod
    def stats(child):
        child.send("stats")
        return parse_stats(child.until("TARGET stats"))

    @classmethod
    def client(cls, hwnd):
        def read():
            rect = wintypes.RECT()
            USER32.GetClientRect(hwnd, ctypes.byref(rect))
            return (rect.right, rect.bottom)
        return cls.in_target_dpi(hwnd, read)

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

    def test_classic_mode_page_is_always_one(self):
        child = self.target()
        hwnd = self.ready(child)
        self.assertEqual(self.client(hwnd), (200, 190))
        self.assertEqual(self.page(child), 1)
        self.click(hwnd, *TAB_2)  # 经典布局这里只是红色内容区：只计数，不切页。
        self.assertEqual(self.page(child), 1)
        self.assertEqual(self.stats(child), [0, 1, 1, 0, 0])

    def test_pages_switch_only_by_tab_press(self):
        child = self.target("--pages", "3")
        hwnd = self.ready(child)
        self.assertEqual(self.client(hwnd), (200, 218))
        self.assertEqual(self.page(child), 1)
        self.click(hwnd, *TAB_2)
        self.assertEqual(self.page(child), 2)
        self.click(hwnd, *TAB_3)
        self.assertEqual(self.page(child), 3)
        self.click(hwnd, *TAB_3)  # 同一页重复按下不变。
        self.assertEqual(self.page(child), 3)
        self.click(hwnd, *CONTENT)  # 页签区之外只计数。
        self.assertEqual(self.page(child), 3)
        self.click(hwnd, *TAB_2)
        self.assertEqual(self.page(child), 2)
        self.assertEqual(self.stats(child), [0, 5, 5, 0, 0])

    def test_pages_argument_rejected_before_window(self):
        for arguments in (["--pages"], ["--pages", "1"], ["--pages", "9"], ["--pages", "x"],
                          ["--pages", "100"], ["--pages", "-3"], ["--pages", "3", "--pages", "3"]):
            with self.subTest(arguments=arguments):
                result = subprocess.run(
                    [TARGET, "--hidden", *arguments], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE, text=True, encoding="utf-8", timeout=10,
                    creationflags=subprocess.CREATE_NO_WINDOW,
                )
                self.assertEqual(result.returncode, 2)
                self.assertNotIn("ready", result.stdout)
                self.assertTrue(result.stderr.startswith("ERROR"), result.stderr)
        # 管道模式下 8 页上限可用；页签更窄但布局仍是 200 宽。
        child = self.target("--pages", "8")
        self.assertEqual(self.client(self.ready(child)), (200, 218))
        self.assertEqual(self.page(child), 1)


if __name__ == "__main__":
    unittest.main(argv=[__file__, *REST])
