"""仅执行本工程编译的 worker/合成 COM DLL；不寻找或加载真实大漠。

窗口 demo 与 smoke 的许可初始化行 LICENSE\t<code>\t<info> 经父私有 stdin 先于其它命令发送，
且只使用公开的 fixture 测试凭据（fixture-license/demo）；真实许可不进入本测试。
提供 --smoke 时，额外用同一合成 fixture 驱动 sdk_smoke 的 typed 绑定/输入/解绑链路；
真实 SDK 的 sdk_smoke 运行只能由维护者显式执行，不属于本测试。
"""
import argparse
import hashlib
import os
from pathlib import Path
import queue
import struct
import subprocess
import tempfile
import threading
import time
import unittest

PARSER = argparse.ArgumentParser()
PARSER.add_argument("--worker", required=True)
PARSER.add_argument("--fixture", required=True)
PARSER.add_argument("--smoke")
PARSER.add_argument("--target")
OPTIONS, REST = PARSER.parse_known_args()
WORKER = str(Path(OPTIONS.worker).resolve())
FIXTURE = Path(OPTIONS.fixture).resolve()
SMOKE = str(Path(OPTIONS.smoke).resolve()) if OPTIONS.smoke else ""
TARGET = str(Path(OPTIONS.target).resolve()) if OPTIONS.target else ""
CLSID = "{9F174D53-4C94-4710-9904-100120023003}"
# 仅是合成 fixture 的公开测试凭据，与任何真实大漠许可无关。
LICENSE_LINE = "LICENSE\tfixture-license\tdemo"
BAD_LICENSE_LINE = "LICENSE\twrong-license\tdemo"


class Child:
    def __init__(self, args, behavior=""):
        env = dict(os.environ, DM_TEST_BEHAVIOR=behavior)
        self.process = subprocess.Popen(
            [WORKER, *args], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT, text=True, encoding="utf-8", env=env,
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
        # 直接写 UTF-8 原始字节：文本模式在 Windows 会把 \n 翻译成 \r\n，
        # 而子进程的许可初始化行合同只接受单个 \n 结尾。
        self.process.stdin.buffer.write(command.encode("utf-8") + b"\n")
        self.process.stdin.buffer.flush()

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
                self.send("stop")
                self.process.wait(timeout=3)
            except (OSError, subprocess.TimeoutExpired):
                self.process.kill()  # 仅清理测试子进程，不作为正常停止成功证据。
                self.process.wait(timeout=3)
        self.reader.join(timeout=1)
        self.process.stdin.close()
        self.process.stdout.close()


def sdk_args(path=FIXTURE, digest=None):
    return ["--sdk-approved", "--dll", str(path), "--sha256",
            digest or hashlib.sha256(path.read_bytes()).hexdigest(),
            "--version", "fixture-1", "--clsid", CLSID]


class WorkerTests(unittest.TestCase):
    def child(self, args, behavior=""):
        child = Child(args, behavior)
        self.addCleanup(child.close)
        return child

    def test_pause_resume_stop_and_repeat_start(self):
        for _ in range(2):
            child = self.child(["--demo"])
            child.until("TICK")
            child.send("pause")
            child.until("ACK pause-requested")
            time.sleep(.15)  # 允许已开始的一步结束，再测暂停稳定性。
            before = child.lines.count("TICK")
            time.sleep(.25)
            self.assertEqual(before, child.lines.count("TICK"))
            child.send("resume")
            child.until("ACK resume-requested")
            child.until("TICK")
            start = time.monotonic()
            child.send("pause")
            child.send("stop")
            self.assertEqual(child.wait(), 0)
            self.assertLess(time.monotonic() - start, 2)
            self.assertEqual(child.lines[-1], "STATE stopped")

    def test_parent_pipe_closed_stops_worker(self):
        child = self.child(["--demo"])
        child.until("TICK")
        child.process.stdin.close()
        self.assertEqual(child.wait(), 0)
        self.assertIn("STATE stopped", child.lines)

    def test_synthetic_com_lifetime(self):
        child = self.child(sdk_args())
        child.until("SDK version-matched")
        child.send("stop")
        self.assertEqual(child.wait(), 0)
        self.assertIn("FIXTURE release-same-thread", child.lines)
        self.assertIn("FIXTURE unload-ready", child.lines)
        self.assertNotIn("FIXTURE leaked", child.lines)

    def test_hash_mismatch_rejected_before_loading(self):
        child = self.child(sdk_args(digest="0" * 64))
        self.assertEqual(child.wait(), 1)
        self.assertTrue(any("SHA-256 mismatch" in x for x in child.lines))
        self.assertFalse(any(x.startswith("FIXTURE") for x in child.lines))

    def test_architecture_rejected_before_loading(self):
        data = bytearray(FIXTURE.read_bytes())
        header = struct.unpack_from("<I", data, 0x3c)[0]
        machine = struct.unpack_from("<H", data, header + 4)[0]
        struct.pack_into("<H", data, header + 4, 0x8664 if machine == 0x14c else 0x14c)
        with tempfile.TemporaryDirectory(prefix="dm-fixture-") as directory:
            altered = Path(directory, "wrong-architecture.dll")
            altered.write_bytes(data)
            child = self.child(sdk_args(altered))
            self.assertEqual(child.wait(), 1)
            self.assertTrue(any("architecture mismatch" in x for x in child.lines))
            self.assertFalse(any(x.startswith("FIXTURE") for x in child.lines))

    def test_com_errors_release_object(self):
        for behavior, message in [("wrong-version", "Ver mismatch"),
                                  ("wrong-type", "BSTR"), ("hresult", "HRESULT")]:
            with self.subTest(behavior=behavior):
                child = self.child(sdk_args(), behavior)
                self.assertEqual(child.wait(), 1)
                self.assertTrue(any(message in x for x in child.lines))
                self.assertIn("FIXTURE release-same-thread", child.lines)
                self.assertIn("FIXTURE unload-ready", child.lines)

    def test_stop_during_blocking_call_waits_for_cleanup(self):
        child = self.child(sdk_args(), "slow")
        child.until("FIXTURE call-blocked")
        child.send("stop")
        child.until("ACK stop-requested")
        self.assertEqual(child.wait(), 0)
        self.assertNotIn("TICK", child.lines)
        self.assertIn("FIXTURE release-same-thread", child.lines)

    def test_invalid_command_stops(self):
        child = self.child(["--demo"])
        child.until("TICK")
        child.send("arbitrary-method")
        self.assertEqual(child.wait(), 2)
        self.assertIn("ERROR invalid command; stopping", child.lines)
        self.assertEqual(child.lines[-1], "STATE stopped")

    def test_sdk_needs_explicit_switch(self):
        child = self.child(sdk_args()[1:])
        self.assertEqual(child.wait(), 2)
        self.assertFalse(any(x.startswith("FIXTURE") for x in child.lines))

    @unittest.skipUnless(TARGET, "requires separately built demo target")
    def test_window_demo_other_process(self):
        child = self.child([*sdk_args(), "--window-demo", "--target", TARGET, "--hidden-target",
                            "--reg-approved", "--license-stdin"])
        child.send(LICENSE_LINE)
        child.until("DEMO pass", timeout=10)
        self.assertEqual(child.wait(), 0)
        self.assertIn("SDK license-registered", child.lines)
        for stage in ("target-ready", "bound", "color-ok", "click-ok", "key-ok", "unbound", "target-closed"):
            self.assertIn("DEMO " + stage, child.lines)
        self.assertEqual(child.lines[-1], "STATE stopped")

    @unittest.skipUnless(TARGET, "requires separately built demo target")
    def test_window_demo_stop_releases_and_closes_target(self):
        child = self.child([*sdk_args(), "--window-demo", "--target", TARGET, "--hidden-target",
                            "--reg-approved", "--license-stdin"])
        child.send(LICENSE_LINE)
        child.until("DEMO bound", timeout=5)
        child.send("stop")
        self.assertEqual(child.wait(), 0)
        self.assertIn("DEMO unbound", child.lines)
        self.assertIn("DEMO target-closed", child.lines)
        self.assertNotIn("DEMO click-ok", child.lines)
        self.assertNotIn("DEMO pass", child.lines)

    @unittest.skipUnless(TARGET, "requires separately built demo target")
    def test_window_demo_binding_error_reports_code_and_closes(self):
        child = self.child([*sdk_args(), "--window-demo", "--target", TARGET, "--hidden-target",
                            "--reg-approved", "--license-stdin"], "bind-fail")
        child.send(LICENSE_LINE)
        self.assertEqual(child.wait(), 1)
        self.assertIn("DEMO target-closed", child.lines)
        self.assertTrue(any("GetLastError=-44" in line for line in child.lines))
        self.assertNotIn("DEMO click-ok", child.lines)

    def test_window_demo_requires_reg_approval(self):
        # 未给 --reg-approved/--license-stdin：窗口 demo 在解析期即被拒绝，不加载、不建目标。
        child = self.child([*sdk_args(), "--window-demo", "--target", "unused.exe"])
        self.assertEqual(child.wait(), 2)
        self.assertTrue(any("reg-approved" in line for line in child.lines))
        self.assertFalse(any(line.startswith("FIXTURE") for line in child.lines))
        self.assertFalse(any(line.startswith("DEMO") for line in child.lines))

    def test_window_demo_reg_failure_stops_before_target(self):
        # 合同：Reg 失败即停；不创建目标进程、不绑定、不进入 stopped 正常收尾。
        child = self.child([*sdk_args(), "--window-demo", "--target", "unused.exe",
                            "--reg-approved", "--license-stdin"])
        child.send(BAD_LICENSE_LINE)
        self.assertEqual(child.wait(), 1)
        self.assertTrue(any("Reg failed; result=4" in line for line in child.lines))
        self.assertFalse(any(line.startswith("DEMO") for line in child.lines))
        self.assertNotIn("FIXTURE licensed", child.lines)
        self.assertNotIn("FIXTURE bound", child.lines)
        self.assertNotIn("STATE stopped", child.lines)

    @unittest.skipUnless(TARGET, "requires separately built demo target")
    def test_window_demo_license_secret_not_echoed(self):
        # 许可凭据只经私有管道进入；全部输出不得出现凭据原文。
        child = self.child([*sdk_args(), "--window-demo", "--target", TARGET, "--hidden-target",
                            "--reg-approved", "--license-stdin"])
        child.send(LICENSE_LINE)
        child.until("DEMO bound", timeout=5)
        child.send("stop")
        self.assertEqual(child.wait(), 0)
        self.assertIn("FIXTURE licensed", child.lines)
        self.assertIn("SDK license-registered", child.lines)
        self.assertFalse(any("fixture-license" in line for line in child.lines))

    def test_license_initialization_rejected(self):
        # 表驱动：许可初始化行合同外的输入一律在加载前拒绝（exit 2），
        # 不进入 STATE loading、不触碰 fixture、不回显许可行原文。
        rows = [
            ("missing-marker", b"NOPE\tfixture-license\tdemo\n", "Missing license initialization marker"),
            ("extra-tab", b"LICENSE\tcode\tinfo\textra\n", "Invalid license initialization fields"),
            ("empty-code", b"LICENSE\t\tdemo\n", "Invalid license field length"),
            ("bad-info-char", b"LICENSE\tfixture-license\tdemo!\n",
             "Invalid license version-info character"),
            ("bad-utf8", b"LICENSE\t\xff\xfe\tdemo\n", "License input must be UTF-8"),
            ("cr-terminated", b"LICENSE\tfixture-license\tdemo\r\n",
             "Invalid license initialization line"),
            ("overlong-line", b"LICENSE\t" + b"a" * 5000 + b"\n",
             "Invalid license initialization line"),
            ("overlong-code", b"LICENSE\t" + b"a" * 513 + b"\tdemo\n",
             "Invalid license field length"),
            ("overlong-info", b"LICENSE\tcode\t" + b"a" * 41 + b"\n",
             "Invalid license field length"),
        ]
        for name, payload, message in rows:
            with self.subTest(case=name):
                child = self.child([*sdk_args(), "--window-demo", "--target", "unused.exe",
                                    "--reg-approved", "--license-stdin"])
                child.process.stdin.buffer.write(payload)
                child.process.stdin.buffer.flush()
                self.assertEqual(child.wait(), 2, child.lines)
                self.assertTrue(any(message in line for line in child.lines), child.lines)
                self.assertFalse(any(line.startswith("STATE loading") for line in child.lines))
                self.assertFalse(any(line.startswith("FIXTURE") for line in child.lines))
                self.assertFalse(any("LICENSE\t" in line for line in child.lines))
                self.assertFalse(any("fixture-license" in line for line in child.lines))

    def test_license_initialization_pipe_closed_bounded(self):
        # 未发送初始化行且父端立即关闭管道：必须立刻有界拒绝，不得空等 5 秒超时。
        child = self.child([*sdk_args(), "--window-demo", "--target", "unused.exe",
                            "--reg-approved", "--license-stdin"])
        child.process.stdin.close()
        start = time.monotonic()
        self.assertEqual(child.wait(), 2)
        self.assertLess(time.monotonic() - start, 3, "pipe-closed rejection must be bounded")
        self.assertTrue(any("License pipe closed before initialization" in line for line in child.lines),
                        child.lines)
        self.assertFalse(any(line.startswith("STATE loading") for line in child.lines))
        self.assertFalse(any(line.startswith("FIXTURE") for line in child.lines))

    def test_window_target_rejected_without_demo_flag(self):
        child = self.child([*sdk_args(), "--target", "unused.exe"])
        self.assertEqual(child.wait(), 2)
        self.assertFalse(any(x.startswith("FIXTURE") for x in child.lines))


class SmokeTests(unittest.TestCase):
    """sdk_smoke + 合成 fixture：typed 接口返回值、异常路径与显式解绑/释放清理。"""

    @staticmethod
    def run_smoke(behavior="", placement=None, drop_approval=False, drop_reg=False,
                  license=LICENSE_LINE):
        base = list(sdk_args())
        # drop_approval 模拟丢失 --sdk-approved；否则两个批准标志紧跟其后固定在最前。
        args = base[1:] if drop_approval else list(base)
        if not drop_reg:
            args = [args[0], "--reg-approved", "--license-stdin", *args[1:]]
        if placement:
            args += ["--placement", placement]
        env = dict(os.environ, DM_TEST_BEHAVIOR=behavior)
        # 二进制管道 + 手动 UTF-8：避免文本模式把 \n 写成 \r\n 破坏许可初始化行合同。
        proc = subprocess.Popen(
            [SMOKE, *args], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT, env=env,
            creationflags=subprocess.CREATE_NO_WINDOW,
        )
        try:
            output, _ = proc.communicate(input=(license + "\n").encode("utf-8"), timeout=30)
            return proc.returncode, [line.decode("utf-8", "replace") for line in output.splitlines()]
        except subprocess.TimeoutExpired:
            proc.kill()  # 仅清理测试子进程，不作为证据。
            output, _ = proc.communicate()
            return "timeout", [line.decode("utf-8", "replace") for line in output.splitlines()]

    def assertLine(self, lines, prefix):
        self.assertTrue(any(x.startswith(prefix) for x in lines),
                        f"Missing {prefix!r}: {lines}")

    def assertNoLine(self, lines, prefix):
        self.assertFalse(any(x.startswith(prefix) for x in lines),
                         f"Unexpected {prefix!r}: {lines}")

    def assert_released_clean(self, lines):
        self.assertIn("FIXTURE release-same-thread", lines)
        self.assertIn("FIXTURE unload-ready", lines)
        self.assertNotIn("FIXTURE leaked", lines)
        self.assertNotIn("FIXTURE action-unbound", lines)

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_synthetic_hidden_pass(self):
        code, lines = self.run_smoke()
        self.assertEqual(code, 0, lines)
        self.assertLine(lines, "SMOKE target-ready placement=hidden")
        self.assertLine(lines, "OBS ver ok version=fixture-1")
        self.assertLine(lines, "OBS license-registered")
        self.assertLine(lines, "OBS bind ok business=1 getbind=")
        self.assertLine(lines, "OBS color match x=50 y=75 color=ff0000 business=0 matched=yes")
        self.assertLine(lines, "OBS color mismatch-probe x=50 y=75 color=00ff00 business=1 matched=no")
        self.assertLine(lines, "OBS click down=1 up=1")
        self.assertLine(lines, "OBS key vk=0x41 down=1 up=1")
        self.assertLine(lines, "OBS unbind ok business=1 getbind=0")
        self.assertLine(lines, "SMOKE released")
        self.assertLine(lines, "SMOKE target-closed")
        self.assertLine(lines, "SMOKE pass")
        self.assert_released_clean(lines)

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_synthetic_offscreen_pass(self):
        code, lines = self.run_smoke(placement="offscreen")
        self.assertEqual(code, 0, lines)
        self.assertLine(lines, "SMOKE target-ready placement=offscreen")
        self.assertLine(lines, "OBS unbind ok business=1 getbind=0")
        self.assertLine(lines, "SMOKE pass")
        self.assert_released_clean(lines)

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_bind_fail_stops_before_any_input(self):
        code, lines = self.run_smoke(behavior="bind-fail")
        self.assertEqual(code, 1, lines)
        self.assertLine(lines, "SMOKE FAIL BindWindow business-failed")
        self.assertNoLine(lines, "OBS click")
        self.assertNoLine(lines, "OBS key")
        self.assert_released_clean(lines)
        self.assertLine(lines, "SMOKE fail")

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_bind_verify_mismatch_still_unbinds(self):
        # 回归：BindWindow 业务成功但 GetBindWindow 核对不一致时，
        # 已生效的绑定仍必须被显式解绑，不得遗漏后靠 exit 0 混过。
        code, lines = self.run_smoke(behavior="bind-verify-mismatch")
        self.assertEqual(code, 1, lines)
        self.assertLine(lines, "SMOKE FAIL BindWindow verification failed")
        self.assertNoLine(lines, "OBS bind ok")
        self.assertNoLine(lines, "OBS click")
        self.assertLine(lines, "SMOKE cleanup-unbind-ok")
        self.assertIn("FIXTURE unbound", lines)
        self.assert_released_clean(lines)
        self.assertLine(lines, "SMOKE fail")

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_action_fail_cleans_up_and_unbinds(self):
        code, lines = self.run_smoke(behavior="action-fail")
        self.assertEqual(code, 1, lines)
        self.assertLine(lines, "SMOKE FAIL MoveTo business-failed")
        self.assertLine(lines, "SMOKE cleanup-unbind-ok")
        self.assertIn("FIXTURE unbound", lines)
        self.assert_released_clean(lines)

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_cmp_mismatch_reports_capture_boundary(self):
        code, lines = self.run_smoke(behavior="cmp-mismatch")
        self.assertEqual(code, 1, lines)
        self.assertLine(lines, "OBS color match x=50 y=75 color=ff0000 business=1 matched=no")
        self.assertLine(lines, "SMOKE FAIL capture-not-verified at hidden placement")
        self.assertLine(lines, "SMOKE cleanup-unbind-ok")
        self.assert_released_clean(lines)

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_unbind_fail_still_releases_without_pass(self):
        code, lines = self.run_smoke(behavior="unbind-fail")
        self.assertEqual(code, 1, lines)
        self.assertLine(lines, "OBS key vk=0x41 down=1 up=1")
        self.assertLine(lines, "SMOKE FAIL UnBindWindow business-failed")
        self.assertLine(lines, "SMOKE cleanup-unbind-failed")
        self.assertNoLine(lines, "OBS unbind ok")
        self.assertNoLine(lines, "SMOKE pass")
        self.assert_released_clean(lines)

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_requires_explicit_approval(self):
        code, lines = self.run_smoke(drop_approval=True)
        self.assertEqual(code, 2, lines)
        self.assertNoLine(lines, "SMOKE target-ready")
        self.assertFalse(any(x.startswith("FIXTURE") for x in lines))

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_requires_reg_approval(self):
        code, lines = self.run_smoke(drop_reg=True)
        self.assertEqual(code, 2, lines)
        self.assertNoLine(lines, "SMOKE target-ready")
        self.assertFalse(any(x.startswith("FIXTURE") for x in lines))

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_reg_failure_stops_before_target(self):
        # 合同：Reg 失败即停；不创建目标窗口、不绑定；对象仍正常释放。
        code, lines = self.run_smoke(license=BAD_LICENSE_LINE)
        self.assertEqual(code, 1, lines)
        self.assertLine(lines, "SMOKE FAIL Reg failed; result=4")
        self.assertNoLine(lines, "SMOKE target-ready")
        self.assertNoLine(lines, "OBS bind ok")
        self.assertNotIn("FIXTURE licensed", lines)
        self.assertNotIn("FIXTURE bound", lines)
        self.assert_released_clean(lines)
        self.assertLine(lines, "SMOKE fail")

    @unittest.skipUnless(SMOKE, "--smoke not provided")
    def test_smoke_license_secret_not_echoed(self):
        code, lines = self.run_smoke()
        self.assertEqual(code, 0, lines)
        self.assertIn("FIXTURE licensed", lines)
        self.assertFalse(any("fixture-license" in x for x in lines))


if __name__ == "__main__":
    unittest.main(argv=[__file__, *REST])
