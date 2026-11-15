"""仅驱动本工程编译的 MFC 控制台隐藏自测与 dm_worker/合成 DLL；不加载真实大漠，不显示窗口。

--window-demo-smoke 隐藏验收入口经隔离桌面 + 私有 stdin 许可行用合成 DLL 走
绑定自建测试窗口端到端，并验证缺明确许可参数/未知参数/非法许可行时在创建
窗口前即拒绝。真实 DLL 由维护者另行显式调用，注册码绝不进入日志或结果文件。
"""
import argparse
import hashlib
from pathlib import Path
import subprocess
import tempfile
import unittest

PARSER = argparse.ArgumentParser()
PARSER.add_argument("--console", required=True)
PARSER.add_argument("--worker", required=True)
PARSER.add_argument("--fixture", required=True)
OPTIONS, REST = PARSER.parse_known_args()
CONSOLE = str(Path(OPTIONS.console).resolve())
WORKER = str(Path(OPTIONS.worker).resolve())
FIXTURE = str(Path(OPTIONS.fixture).resolve())
TARGET = str(Path(WORKER).with_name("dm_demo_target.exe"))

FIXTURE_CLSID = "{9F174D53-4C94-4710-9904-100120023003}"
FIXTURE_VERSION = "fixture-1"
FIXTURE_LICENSE = b"LICENSE\tfixture-license\tdemo\n"

EXPECTED_PASSES = [
    "PASS start-demo",
    "PASS pause-stable",
    "PASS resume",
    "PASS stop-clean",
    "PASS repeat-start",
    "PASS start-failure",
    "PASS approval-revocation",
    "PASS sdk-unapproved-rejected",
    "PASS sdk-synthetic",
    "PASS stop-deadline-honest",
    "PASS close-while-running",
]

WINDOW_DEMO_MARKERS = [
    "DEMO target-ready",
    "DEMO bound",
    "DEMO color-ok",
    "DEMO click-ok",
    "DEMO key-ok",
    "DEMO unbound",
    "DEMO target-closed",
    "DEMO pass",
]


def run_console(arguments, timeout, feed=None):
    stream = {} if feed is not None else {"stdin": subprocess.DEVNULL}
    return subprocess.run(
        [CONSOLE, *arguments],
        input=feed,
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout,
        creationflags=subprocess.CREATE_NO_WINDOW, **stream)


class MfcSmokeTests(unittest.TestCase):
    def test_ui_smoke_hidden_window(self):
        with tempfile.TemporaryDirectory(prefix="dm-mfc-smoke-") as directory:
            result = Path(directory, "result.txt")
            completed = run_console(
                ["--ui-smoke", "--worker", WORKER, "--fixture", FIXTURE,
                 "--result-file", str(result)], 150)
            output = completed.stdout.decode("utf-8", errors="replace")
            self.assertEqual(completed.returncode, 0, output)
            self.assertTrue(result.is_file(), "result file missing")
            text = result.read_text(encoding="utf-8")
            for marker in EXPECTED_PASSES:
                self.assertIn(marker, text)
            self.assertIn("SMOKE OK", text)
            self.assertNotIn("FAIL", text)

    @unittest.skipUnless(Path(TARGET).is_file(), "dm_demo_target.exe 未随本构建生成")
    def test_window_demo_smoke_synthetic_e2e(self):
        digest = hashlib.sha256(Path(FIXTURE).read_bytes()).hexdigest()
        with tempfile.TemporaryDirectory(prefix="dm-mfc-wdemo-") as directory:
            result = Path(directory, "result.txt")
            completed = run_console(
                ["--window-demo-smoke", "--sdk-approved", "--reg-approved",
                 "--license-stdin", "--dll", FIXTURE,
                 "--sha256", digest, "--version", FIXTURE_VERSION, "--clsid", FIXTURE_CLSID,
                 "--worker", WORKER, "--result-file", str(result)],
                150, feed=FIXTURE_LICENSE)
            output = completed.stdout.decode("utf-8", errors="replace")
            self.assertEqual(completed.returncode, 0, output)
            self.assertTrue(result.is_file(), "result file missing")
            text = result.read_text(encoding="utf-8")
            self.assertIn("PASS window-demo-e2e", text)
            self.assertIn("SDK license-registered", text)
            for marker in WINDOW_DEMO_MARKERS:
                self.assertIn(marker, text)
            self.assertIn("STATE stopped", text)
            self.assertIn("SMOKE OK", text)
            self.assertNotIn("FAIL", text)

    def test_window_demo_smoke_requires_explicit_approval(self):
        digest = hashlib.sha256(Path(FIXTURE).read_bytes()).hexdigest()
        with tempfile.TemporaryDirectory(prefix="dm-mfc-wdemo-rej-") as directory:
            result = Path(directory, "result.txt")
            full = ["--window-demo-smoke", "--sdk-approved", "--reg-approved",
                    "--license-stdin", "--dll", FIXTURE, "--sha256", digest,
                    "--version", FIXTURE_VERSION, "--clsid", FIXTURE_CLSID,
                    "--result-file", str(result)]
            cases = [
                ("missing --sdk-approved", [a for a in full if a != "--sdk-approved"]),
                ("missing --reg-approved", [a for a in full if a != "--reg-approved"]),
                ("missing --license-stdin", [a for a in full if a != "--license-stdin"]),
                ("missing --clsid",
                 ["--window-demo-smoke", "--sdk-approved", "--reg-approved",
                  "--license-stdin", "--dll", FIXTURE, "--sha256", digest,
                  "--version", FIXTURE_VERSION, "--result-file", str(result)]),
                ("unknown argument", full + ["--evil"]),
                ("fixture fallback not allowed", full + ["--fixture", FIXTURE]),
                ("value slot swallows smoke flag",
                 ["--dll", "--window-demo-smoke", "--sdk-approved", "--reg-approved",
                  "--license-stdin", "--sha256", digest, "--version", FIXTURE_VERSION,
                  "--clsid", FIXTURE_CLSID, "--result-file", str(result)]),
                ("missing value at end",
                 ["--ui-smoke", "--result-file", str(result), "--worker"]),
                ("arguments without smoke mode",
                 ["--worker", WORKER, "--result-file", str(result)]),
            ]
            for name, arguments in cases:
                with self.subTest(name):
                    completed = run_console(arguments, 60)
                    self.assertNotEqual(completed.returncode, 0,
                                        "缺许可参数必须以非零退出码拒绝")
                    self.assertTrue(result.is_file(), "result file missing")
                    text = result.read_text(encoding="utf-8")
                    self.assertIn("FAIL", text)
                    self.assertIn("SMOKE FAILED", text)

    def test_window_demo_smoke_rejects_bad_license_line(self):
        digest = hashlib.sha256(Path(FIXTURE).read_bytes()).hexdigest()
        with tempfile.TemporaryDirectory(prefix="dm-mfc-wdemo-lic-") as directory:
            result = Path(directory, "result.txt")
            arguments = ["--window-demo-smoke", "--sdk-approved", "--reg-approved",
                         "--license-stdin", "--dll", FIXTURE, "--sha256", digest,
                         "--version", FIXTURE_VERSION, "--clsid", FIXTURE_CLSID,
                         "--result-file", str(result)]
            for name, feed in [("garbage line", b"JUNK\n"),
                               ("marker only", b"LICENSE\t\n"),
                               ("no code", b"LICENSE\t\tdemo\n")]:
                with self.subTest(name):
                    completed = run_console(arguments, 60, feed=feed)
                    self.assertNotEqual(completed.returncode, 0,
                                        "非法许可行必须以非零退出码拒绝")
                    self.assertTrue(result.is_file(), "result file missing")
                    text = result.read_text(encoding="utf-8")
                    self.assertIn("FAIL", text)
                    self.assertIn("license input", text)
                    self.assertNotIn("fixture-license", text)
                    self.assertIn("SMOKE FAILED", text)


if __name__ == "__main__":
    unittest.main(argv=[__file__, *REST])
