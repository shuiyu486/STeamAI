# -*- coding: utf-8 -*-
"""自建 dm_demo_target 子进程的设备（Windows + ctypes）：只驱动自己启动的那一个目标窗口。

信任边界：
- 启动前要求调用者在命令行独立给出 SHA-256（--exe-sha256），按文件字节计算并比对；
  缺失、格式不对、不是普通文件、不一致 → 拒绝启动，一个进程都不创建。位置或文件名不能
  证明是哪份程序，本模块不据此放行；也不要从 exe 旁边的文件读哈希。
- 只使用子进程 stdout 第一行 `TARGET ready <hwnd> <pid>` 给出的 hwnd，并核对 pid 等于本子
  进程 pid、hwnd 所属进程也等于该 pid；不接收外部 hwnd，不查找/枚举已有窗口。
- 画面只从这个 hwnd 读取：先 PrintWindow(PW_CLIENTONLY) 让目标自绘，再从同一 hwnd 的窗口 DC
  BitBlt 读呈现表面，两路一致才算有画面；不截屏、不取桌面 DC。输入只向这个 hwnd PostMessage；
  不 SendInput、不 SetForegroundWindow、不装钩子。
- 默认把窗口显示到虚拟屏幕之外（目标自身带 WS_EX_NOACTIVATE|WS_EX_TOOLWINDOW，这里只
  SW_SHOWNOACTIVATE），不占前台、不进任务栏；显示后核对不在任何显示器内且不是前台窗口，
  否则立即收尾并报错。--hidden 窗口无法从外部取像素（PrintWindow、窗口 DC 都给空帧；用户态
  跨进程 WM_PRINTCLIENT 传过去的 HDC 在目标进程里无效），画面一律记 unknown，不伪造颜色。

DPI：VS2019 链接器默认清单带 dpiAware，目标按系统 DPI 渲染；Python 默认 DPI-unaware，
看到的客户区和发出的坐标都会被虚拟化缩放。几何、截图、PostMessage 全部在目标窗口的
DPI 感知上下文里完成。
"""

import collections
import contextlib
import ctypes
import hashlib
import os
import queue
import subprocess
import sys
import threading
import time
from ctypes import wintypes

_HERE = os.path.dirname(os.path.abspath(__file__))
if _HERE not in sys.path:
    sys.path.insert(0, _HERE)

import localize  # noqa: E402  DistilledLocator/LocateFailed
import refdiff  # noqa: E402  picture_fingerprint（画面区域均值）
import workflow  # noqa: E402  State/Observation/InputUnconfirmed

WM_LBUTTONDOWN, WM_LBUTTONUP = 0x201, 0x202
SW_SHOWNOACTIVATE = 4
PW_CLIENTONLY = 1
SM_XVIRTUALSCREEN, SM_YVIRTUALSCREEN, SM_CXVIRTUALSCREEN, SM_CYVIRTUALSCREEN = 76, 77, 78, 79
SWP_FLAGS = 0x0001 | 0x0004 | 0x0010 | 0x0200  # NOSIZE|NOZORDER|NOACTIVATE|NOOWNERZORDER
SRCCOPY_CAPTUREBLT = 0x00CC0020 | 0x40000000
OFFSCREEN_MARGIN = 64  # 与 tests/sdk_smoke.cpp 的 offscreen 放置一致：虚拟屏幕右边界之外 64px


class TargetError(Exception):
    """目标进程/窗口不符合预期（协议、归属、前台、放置），一律停止并收尾。"""


class ExeNotVerified(TargetError):
    """调用者没有给出可核对的 SHA-256，或与文件字节不一致：拒绝启动。"""


Prepared = collections.namedtuple(
    "Prepared", ["point", "bbox", "source", "score", "page", "binding_rect"])


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_exe(path, expected):
    """信任闸门：expected 是调用者独立给出的 64 位十六进制；通过才返回实际哈希。"""
    if (not isinstance(expected, str) or len(expected) != 64
            or any(c not in "0123456789abcdefABCDEF" for c in expected)):
        raise ExeNotVerified(
            "拒绝启动：--exe-sha256 必须是调用者独立给出的 64 位十六进制 SHA-256"
            "（位置或文件名不能证明是哪份程序）")
    if not os.path.isfile(path):
        raise ExeNotVerified("拒绝启动：不是普通文件：%s" % path)
    actual = sha256_file(path)
    if actual != expected.lower():
        raise ExeNotVerified("拒绝启动：SHA-256 不一致（expected %s, actual %s）"
                             % (expected.lower(), actual))
    return actual


_U = _G = None


def _libs():
    global _U, _G
    if _U is not None:
        return _U, _G
    u = ctypes.WinDLL("user32", use_last_error=True)
    g = ctypes.WinDLL("gdi32", use_last_error=True)
    u.IsWindow.argtypes = [wintypes.HWND]; u.IsWindow.restype = wintypes.BOOL
    u.IsWindowVisible.argtypes = [wintypes.HWND]; u.IsWindowVisible.restype = wintypes.BOOL
    u.GetClientRect.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.RECT)]
    u.GetWindowRect.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.RECT)]
    u.GetWindowThreadProcessId.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.DWORD)]
    u.GetWindowThreadProcessId.restype = wintypes.DWORD
    u.GetForegroundWindow.restype = wintypes.HWND
    u.GetDpiForWindow.argtypes = [wintypes.HWND]; u.GetDpiForWindow.restype = wintypes.UINT
    u.GetSystemMetrics.argtypes = [ctypes.c_int]; u.GetSystemMetrics.restype = ctypes.c_int
    u.GetWindowDpiAwarenessContext.argtypes = [wintypes.HWND]
    u.GetWindowDpiAwarenessContext.restype = wintypes.HANDLE
    u.SetThreadDpiAwarenessContext.argtypes = [wintypes.HANDLE]
    u.SetThreadDpiAwarenessContext.restype = wintypes.HANDLE
    u.SetWindowPos.argtypes = [wintypes.HWND, wintypes.HWND, ctypes.c_int, ctypes.c_int,
                               ctypes.c_int, ctypes.c_int, wintypes.UINT]
    u.ShowWindowAsync.argtypes = [wintypes.HWND, ctypes.c_int]
    u.PostMessageW.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM]
    u.PostMessageW.restype = wintypes.BOOL
    u.GetDC.argtypes = [wintypes.HWND]; u.GetDC.restype = wintypes.HDC
    u.ReleaseDC.argtypes = [wintypes.HWND, wintypes.HDC]
    u.PrintWindow.argtypes = [wintypes.HWND, wintypes.HDC, wintypes.UINT]
    u.PrintWindow.restype = wintypes.BOOL
    g.CreateCompatibleDC.restype = wintypes.HDC
    g.CreateCompatibleDC.argtypes = [wintypes.HDC]
    g.DeleteDC.argtypes = [wintypes.HDC]
    g.SelectObject.argtypes = [wintypes.HDC, wintypes.HGDIOBJ]
    g.SelectObject.restype = wintypes.HGDIOBJ
    g.DeleteObject.argtypes = [wintypes.HGDIOBJ]
    g.BitBlt.argtypes = [wintypes.HDC, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_int,
                         wintypes.HDC, ctypes.c_int, ctypes.c_int, wintypes.DWORD]
    g.BitBlt.restype = wintypes.BOOL
    g.CreateDIBSection.argtypes = [wintypes.HDC, ctypes.c_void_p, wintypes.UINT, ctypes.POINTER(ctypes.c_void_p),
                                   wintypes.HANDLE, wintypes.DWORD]
    g.CreateDIBSection.restype = wintypes.HBITMAP
    _U, _G = u, g
    return u, g


class _BMIH(ctypes.Structure):
    _fields_ = [("biSize", wintypes.DWORD), ("biWidth", wintypes.LONG), ("biHeight", wintypes.LONG),
                ("biPlanes", wintypes.WORD), ("biBitCount", wintypes.WORD),
                ("biCompression", wintypes.DWORD), ("biSizeImage", wintypes.DWORD),
                ("biXPelsPerMeter", wintypes.LONG), ("biYPelsPerMeter", wintypes.LONG),
                ("biClrUsed", wintypes.DWORD), ("biClrImportant", wintypes.DWORD)]


class _BMI(ctypes.Structure):
    _fields_ = [("bmiHeader", _BMIH), ("bmiColors", wintypes.DWORD * 3)]


class DemoTarget(object):
    """按 SHA-256 核对后启动一个 dm_demo_target（--hidden --pages N），只操作它的窗口。"""

    def __init__(self, exe, exe_sha256, pages=3, placement="offscreen", ready_timeout=5.0):
        if placement not in ("offscreen", "hidden"):
            raise ValueError("placement 必须是 offscreen 或 hidden")
        self.exe = os.path.abspath(exe)
        self.exe_sha256 = exe_sha256
        self.pages = int(pages)
        self.placement = placement
        self.ready_timeout = ready_timeout
        self.process = None
        self.hwnd = None
        self.pid = None
        self.lines = []           # 目标 stdout/stderr 行（有界），供证据留存
        self.closed_by = None
        self.exit_code = None
        self.capture_error = ""   # 最近一次 capture() 返回 None 的原因（证据里如实记录）
        self._pending = queue.Queue()

    # -- 生命周期 -----------------------------------------------------------
    def start(self):
        # 信任闸门在任何进程创建之前；失败时什么都没启动。
        self.sha256 = verify_exe(self.exe, self.exe_sha256)
        self.process = subprocess.Popen(
            [self.exe, "--hidden", "--pages", str(self.pages)],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
            text=True, encoding="utf-8", creationflags=subprocess.CREATE_NO_WINDOW)
        threading.Thread(target=self._read, daemon=True).start()
        line = self._read_line(self.ready_timeout)
        parsed = self._parse_ready(line)
        if not parsed:
            self.close()
            raise TargetError("目标没有给出可解析的 ready 行：%r；输出：%r" % (line, self.lines[-5:]))
        hwnd, pid = parsed
        if pid != self.process.pid:
            self.close()
            raise TargetError("ready 行的 pid %d 不等于本子进程 pid %d，拒绝操作" % (pid, self.process.pid))
        u, _ = _libs()
        owner = wintypes.DWORD(0)
        u.GetWindowThreadProcessId(hwnd, ctypes.byref(owner))
        if owner.value != self.process.pid:
            self.close()
            raise TargetError("hwnd %d 不属于本子进程（owner pid %d），拒绝操作" % (hwnd, owner.value))
        self.hwnd, self.pid = hwnd, self.process.pid
        if self.placement == "offscreen":
            self._place_offscreen()
        binding = self.binding()
        if binding["foreground"]:
            self.close()
            raise TargetError("目标窗口成了前台窗口，违反无前台约束，已收尾")
        return self

    def _read(self):
        try:
            for raw in self.process.stdout:
                line = raw.rstrip()
                self.lines.append(line)
                del self.lines[:-200]
                self._pending.put(line)
        except (OSError, ValueError):
            return  # 父进程收尾时关闭了管道

    def _read_line(self, timeout):
        try:
            return self._pending.get(timeout=max(0.01, timeout))
        except queue.Empty:
            return None

    @staticmethod
    def _parse_ready(line):
        if not line:
            return None
        parts = line.split(" ")
        if len(parts) != 4 or parts[:2] != ["TARGET", "ready"]:
            return None
        try:
            return int(parts[2]), int(parts[3])
        except ValueError:
            return None

    def query(self, command, prefix, timeout=2.0):
        if self.process is None or self.process.poll() is not None:
            raise TargetError("目标进程已退出（code %r），不能查询 %s" % (self.process.poll() if self.process else None, command))
        try:
            self.process.stdin.write(command + "\n")
            self.process.stdin.flush()
        except (OSError, ValueError) as exc:
            raise TargetError("写目标 stdin 失败：%s" % exc)
        deadline = time.monotonic() + timeout
        while True:
            left = deadline - time.monotonic()
            if left <= 0:
                raise TargetError("等待 %r 超时；目标输出：%r" % (prefix, self.lines[-5:]))
            try:
                line = self._pending.get(timeout=left)
            except queue.Empty:
                line = None
            if line is None:
                if self.process.poll() is not None:
                    raise TargetError("目标在应答 %r 前退出（code %r）；输出：%r" % (prefix, self.process.returncode, self.lines[-5:]))
                continue
            if line.startswith(prefix):
                return line

    def snapshot(self):
        """目标自报：当前页 + 输入计数 [moves, down, up, keyDown, keyUp]。"""
        page = self.query("page", "TARGET page")
        stats = self.query("stats", "TARGET stats")
        page_parts = page.split(" ")
        stat_parts = stats.split(" ")
        if len(page_parts) != 3 or len(stat_parts) != 7:
            raise TargetError("协议行形状不对：%r %r" % (page, stats))
        return {"page": int(page_parts[2]), "stats": [int(p) for p in stat_parts[2:]]}

    def alive(self):
        if self.process is None or self.process.poll() is not None:
            return False
        u, _ = _libs()
        return bool(u.IsWindow(self.hwnd))

    # -- 几何 / DPI -----------------------------------------------------------
    @contextlib.contextmanager
    def dpi_context(self):
        u, _ = _libs()
        previous = u.SetThreadDpiAwarenessContext(u.GetWindowDpiAwarenessContext(self.hwnd))
        try:
            yield
        finally:
            u.SetThreadDpiAwarenessContext(previous)

    def binding(self):
        u, _ = _libs()
        with self.dpi_context():
            rect, client = wintypes.RECT(), wintypes.RECT()
            u.GetWindowRect(self.hwnd, ctypes.byref(rect))
            u.GetClientRect(self.hwnd, ctypes.byref(client))
            return {"hwnd": int(self.hwnd), "pid": self.pid,
                    "rect": [rect.left, rect.top, rect.right, rect.bottom],
                    "client": [client.right, client.bottom],
                    "dpi": int(u.GetDpiForWindow(self.hwnd)),
                    "visible": bool(u.IsWindowVisible(self.hwnd)),
                    "foreground": int(u.GetForegroundWindow() or 0) == int(self.hwnd),
                    "placement": self.placement}

    def _place_offscreen(self):
        u, _ = _libs()
        with self.dpi_context():
            x = u.GetSystemMetrics(SM_XVIRTUALSCREEN) + u.GetSystemMetrics(SM_CXVIRTUALSCREEN) + OFFSCREEN_MARGIN
            y = u.GetSystemMetrics(SM_YVIRTUALSCREEN) + OFFSCREEN_MARGIN
            if not u.SetWindowPos(self.hwnd, None, x, y, 0, 0, SWP_FLAGS):
                self.close()
                raise TargetError("SetWindowPos 失败：%d" % ctypes.get_last_error())
            u.ShowWindowAsync(self.hwnd, SW_SHOWNOACTIVATE)
            deadline = time.monotonic() + 1.0
            while not u.IsWindowVisible(self.hwnd) and time.monotonic() < deadline:
                time.sleep(0.01)
        self._check_offscreen()

    def _check_offscreen(self):
        u, _ = _libs()
        with self.dpi_context():
            vx, vy = u.GetSystemMetrics(SM_XVIRTUALSCREEN), u.GetSystemMetrics(SM_YVIRTUALSCREEN)
            vw, vh = u.GetSystemMetrics(SM_CXVIRTUALSCREEN), u.GetSystemMetrics(SM_CYVIRTUALSCREEN)
        b = self.binding()
        left, top, right, bottom = b["rect"]
        on_screen = left < vx + vw and right > vx and top < vy + vh and bottom > vy
        if on_screen or not b["visible"]:
            self.close()
            raise TargetError("目标窗口没有落在所有显示器之外（rect=%r, virtual=%r, visible=%r），已收尾" % (b["rect"], (vx, vy, vw, vh), b["visible"]))

    def move(self, dy=120):
        """位移扰动：仍在虚拟屏幕之外换一个位置，不激活。只动本子进程窗口。"""
        u, _ = _libs()
        with self.dpi_context():
            rect = wintypes.RECT()
            u.GetWindowRect(self.hwnd, ctypes.byref(rect))
            if not u.SetWindowPos(self.hwnd, None, rect.left, rect.top + int(dy), 0, 0, SWP_FLAGS):
                raise TargetError("SetWindowPos(move) 失败：%d" % ctypes.get_last_error())
        self._check_offscreen()
        return self.binding()["rect"]

    # -- 画面 / 输入 ----------------------------------------------------------
    def capture(self):
        """客户区画面 → RGB ndarray；读不到像素返回 None，原因写在 self.capture_error。

        屏外窗口的可见区为空，收不到有效的 WM_PAINT，呈现表面一直是空白（实测全白）。这里先用
        PrintWindow(PW_CLIENTONLY) 让目标按 WM_PRINTCLIENT 自绘到 DIB，再从同一 hwnd 的窗口 DC
        BitBlt 读呈现表面；两路像素必须一致才算有画面，否则记 unknown。隐藏窗口两路都读不到像素
        （实测全黑）→ None；不处理 WM_PRINTCLIENT 的窗口会给空帧 → None。不截屏、不取桌面 DC。
        """
        self.capture_error = ""
        u, g = _libs()
        with self.dpi_context():
            if not u.IsWindowVisible(self.hwnd):
                self.capture_error = "hidden: 隐藏窗口没有可读像素"
                return None
            client = wintypes.RECT()
            u.GetClientRect(self.hwnd, ctypes.byref(client))
            width, height = client.right, client.bottom
            if width <= 0 or height <= 0:
                self.capture_error = "empty-client"
                return None
            bmi = _BMI()
            bmi.bmiHeader.biSize = ctypes.sizeof(_BMIH)
            bmi.bmiHeader.biWidth = width
            bmi.bmiHeader.biHeight = -height
            bmi.bmiHeader.biPlanes = 1
            bmi.bmiHeader.biBitCount = 32
            bits = ctypes.c_void_p()
            hdc = g.CreateCompatibleDC(None)
            bitmap = g.CreateDIBSection(hdc, ctypes.byref(bmi), 0, ctypes.byref(bits), None, 0)
            if not bitmap:
                g.DeleteDC(hdc)
                self.capture_error = "CreateDIBSection 失败"
                return None
            previous = g.SelectObject(hdc, bitmap)
            ctypes.memset(bits.value, 0, width * height * 4)
            printed = bool(u.PrintWindow(self.hwnd, hdc, PW_CLIENTONLY))
            g.GdiFlush()
            printed_data = ctypes.string_at(bits.value, width * height * 4) if printed else None
            window_dc = u.GetDC(self.hwnd)
            try:
                ok = bool(window_dc) and bool(g.BitBlt(hdc, 0, 0, width, height, window_dc, 0, 0, SRCCOPY_CAPTUREBLT))
                data = ctypes.string_at(bits.value, width * height * 4) if ok else None
            finally:
                if window_dc:
                    u.ReleaseDC(self.hwnd, window_dc)
                g.SelectObject(hdc, previous)
                g.DeleteObject(bitmap)
                g.DeleteDC(hdc)
        if printed_data is None:
            self.capture_error = "PrintWindow(PW_CLIENTONLY) 失败：%d" % ctypes.get_last_error()
            return None
        if data is None:
            self.capture_error = "窗口 DC BitBlt 失败"
            return None
        import numpy
        shape = (height, width, 4)
        printed_frame = numpy.frombuffer(printed_data, numpy.uint8).reshape(shape)
        frame = numpy.frombuffer(data, numpy.uint8).reshape(shape)
        if not printed_frame.any():
            self.capture_error = "PrintWindow 给空帧（目标不处理 WM_PRINTCLIENT 或表面不可读）"
            return None
        if not numpy.array_equal(printed_frame, frame):
            self.capture_error = "PrintWindow 自绘与窗口表面不一致"
            return None
        return frame[:, :, :3][:, :, ::-1].copy()  # BGRA → RGB

    def post_click(self, x, y):
        """只向本子进程窗口 PostMessage 一次按下/抬起（客户区坐标，目标 DPI 上下文）。"""
        u, _ = _libs()
        lparam = (int(y) << 16) | (int(x) & 0xFFFF)
        with self.dpi_context():
            down = u.PostMessageW(self.hwnd, WM_LBUTTONDOWN, 1, lparam)
            up = u.PostMessageW(self.hwnd, WM_LBUTTONUP, 0, lparam)
        return bool(down and up)

    def close(self):
        if self.process is None:
            return
        if self.process.poll() is None:
            try:
                self.process.stdin.write("close\n")
                self.process.stdin.flush()
                self.process.wait(timeout=3)
                self.closed_by = "close"
            except (OSError, ValueError, subprocess.TimeoutExpired):
                self.process.kill()  # 只清理本子进程；不作为正常停止成功的证据。
                self.process.wait(timeout=3)
                self.closed_by = "kill"
        else:
            self.closed_by = "exited"
        self.exit_code = self.process.returncode
        for stream in (self.process.stdin, self.process.stdout):
            try:
                stream.close()
            except (OSError, ValueError):
                pass

    def make_device(self, candidate, controls, regions):
        """按候选参数造设备：点击偏移在发输入时叠加（定位证据保持原样），坏 bbox 只注入指定控件。"""
        return DemoTargetDevice(self, controls, regions,
                                click_offset=candidate.get("click_offset") or (0, 0),
                                effect_budget_ms=candidate.get("effect_budget_ms", 1000.0),
                                locate_dwell_ms=candidate.get("locate_dwell_ms", 0.0),
                                bad_bbox_for=candidate.get("bad_bbox") or {})

    def describe(self):
        return {"kind": "demo_target", "exe": self.exe, "exe_sha256": getattr(self, "sha256", None),
                "pid": self.pid, "hwnd": self.hwnd, "pages": self.pages, "placement": self.placement,
                "closed_by": self.closed_by, "exit_code": self.exit_code,
                "target_lines": [l for l in self.lines if not l.startswith("TARGET ready")][-20:]}


class LayoutCallback(object):
    """授权定位回调的脚本化替身：dwell 秒可中断等待后返回布局表里的控件 bbox；可按控件注入坏 bbox。"""

    def __init__(self, controls, dwell_seconds=0.0, bad_bbox_for=None, clock=None):
        self.controls = dict(controls)
        self.dwell = float(dwell_seconds or 0.0)
        self.bad = dict(bad_bbox_for or {})
        self.clock = clock or time.monotonic
        self.calls = collections.Counter()

    def __call__(self, key, screen, checkpoint):
        self.calls[key[2]] += 1
        deadline = self.clock() + self.dwell
        while self.clock() < deadline:
            checkpoint()
            time.sleep(0.005)
        checkpoint()
        name = key[2]  # key = (实例 token, action, 目标名, 状态)
        if name in self.bad:
            return tuple(self.bad[name])
        if name not in self.controls:
            raise localize.LocateFailed("callback_unknown_control", repr(name))
        return tuple(self.controls[name])


class DemoTargetDevice(object):
    """Runner 设备接口：observe/prepare/act/release。证据经 probe 字典交给 TracingDevice。"""

    def __init__(self, target, controls, regions, click_offset=(0, 0), effect_budget_ms=1000.0,
                 locate_dwell_ms=0.0, bad_bbox_for=None, matcher=None):
        from localize import CvMatcher, DistilledLocator
        self.target = target
        self.regions = dict(regions or {})
        self.click_offset = (int(click_offset[0]), int(click_offset[1]))
        self.effect_budget_ms = float(effect_budget_ms)
        self.token = "demotarget-%d" % target.pid
        self.callback = LayoutCallback(controls, dwell_seconds=locate_dwell_ms / 1000.0, bad_bbox_for=bad_bbox_for)
        self.locator = DistilledLocator(callback=self.callback, matcher=matcher or CvMatcher())
        self.released = False

    def observe(self, checkpoint):
        checkpoint()
        if not self.target.alive():
            return workflow.Observation(workflow.State.GONE, 1.0, "demo_target gone")
        if self.target.binding()["foreground"]:
            raise TargetError("目标窗口成了前台窗口，停止")
        snapshot = self.target.snapshot()
        observation = workflow.Observation(workflow.State.READY, 1.0, "demo_target")
        observation.snapshot = snapshot
        return observation

    def snapshot(self):
        try:
            return self.target.snapshot() if self.target.alive() else None
        except TargetError:
            return None

    def binding(self):
        return self.target.binding() if self.target.alive() else None

    def picture(self):
        return refdiff.picture_fingerprint(self.target.capture() if self.target.alive() else None, self.regions)

    def move(self, spec):
        """位移扰动：仍在虚拟屏幕之外换一个位置（默认下移 120px），不激活。只动本子进程窗口。"""
        return {"rect": self.target.move(int((spec or {}).get("dy", 120)))}

    def describe_locate(self, prepared):
        return {"point": list(prepared.point), "bbox": list(prepared.bbox), "source": prepared.source,
                "score": None if prepared.score is None else round(float(prepared.score), 4)}

    def prepare(self, action, checkpoint):
        checkpoint()
        frame = self.target.capture()
        if frame is None:
            raise localize.LocateFailed("capture-failed", "窗口画面不可读（%s）" % (self.target.capture_error or "unknown"))
        snapshot = self.target.snapshot()
        key = (self.token, action.value, action.target, snapshot["page"])
        located = self.locator.locate(key, frame, checkpoint)
        sw, sh = self.locator.matcher.size(frame)
        x, y = located.point
        if not (0 <= x < sw and 0 <= y < sh):
            raise localize.LocateFailed("point_out_of_screen", "point=%r screen=%r" % (located.point, (sw, sh)))
        return Prepared(located.point, located.bbox, located.source, located.score, snapshot["page"],
                        self.target.binding()["rect"])

    def act(self, action, prepared, probe):
        """发输入前复核绑定与状态（变了就拒绝、不发）；发后以目标自报计数判送达、以 expect 判效果。"""
        probe.update({"sent": False, "kind": "post-message", "point": None, "delivered": None,
                      "clicks_delta": None, "reason_code": None, "reason": None})
        binding = self.target.binding()
        if binding["rect"] != list(prepared.binding_rect):
            probe["reason_code"] = "stale-binding"
            probe["reason"] = "stale prepared location: window rect %r->%r" % (list(prepared.binding_rect), binding["rect"])
            raise workflow.InputUnconfirmed(probe["reason"])
        before = self.target.snapshot()
        if before["page"] != prepared.page:
            probe["reason_code"] = "stale-state"
            probe["reason"] = "stale prepared location: page %d->%d" % (prepared.page, before["page"])
            raise workflow.InputUnconfirmed(probe["reason"])
        x = prepared.point[0] + self.click_offset[0]
        y = prepared.point[1] + self.click_offset[1]
        probe["point"] = [x, y]
        sent = self.target.post_click(x, y)
        probe["sent"] = bool(sent)
        if not sent:
            probe["reason_code"] = "post-failed"
            probe["reason"] = "PostMessage 失败"
            return False
        deadline = time.monotonic() + self.effect_budget_ms / 1000.0
        while True:
            # 目标先排空窗口消息再应答管道命令：应答到达时，先发出的点击已被处理（或不存在）。
            snapshot = self.target.snapshot()
            delivered = snapshot["stats"][1] > before["stats"][1]
            if delivered:
                probe["delivered"] = True
                probe["clicks_delta"] = snapshot["stats"][1] - before["stats"][1]
            if delivered and refdiff.state_matches(snapshot, getattr(action, "step", {}).get("expect")) is True:
                probe["reason_code"] = "confirmed"
                return True
            if time.monotonic() >= deadline:
                probe["delivered"] = bool(delivered)  # 预算用完仍未计数：这条消息没到
                if delivered:
                    changed = snapshot["page"] != before["page"]
                    probe["reason_code"] = "wrong-state" if changed else "no-effect"
                    probe["reason"] = "click counted; page %d -> %d, expected %r" % (
                        before["page"], snapshot["page"], action.step.get("expect"))
                else:
                    probe["reason_code"] = "not-delivered"
                    probe["reason"] = "click not counted within budget"
                return False
            time.sleep(0.002)

    def release(self):
        self.released = True
