"""直接验证分发的 windows-flow；不调用模型、窗口或系统输入。"""
import json
import os
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import threading
import types
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "packs/game-automation/tooling/examples/windows-flow/workflow.py"
FLOW = types.SimpleNamespace(**runpy.run_path(str(SCRIPT), run_name="steamai_windows_flow"))


class Device:
    def __init__(self):
        self.state = FLOW.State.READY
        self.inputs = []
        self.preparations = 0
        self.observations = 0
        self.released = False

    def observe(self, checkpoint):
        checkpoint()
        self.observations += 1
        return FLOW.Observation(self.state)

    def prepare(self, action, checkpoint):
        checkpoint()
        self.preparations += 1
        return self.preparations

    def act(self, action, prepared):
        self.inputs.append((action, prepared))
        self.state = FLOW.State.RESULT
        return True

    def release(self):
        self.released = True


class RunnerTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="steamai-flow-test-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.control_path = self.root / "control.json"
        self.command("none")
        self.config = {
            "recognition": {"image_threshold": 0.85},
            "limits": {"step_wait_timeout_seconds": 1, "retry_limit_per_step": 2},
        }

    def command(self, value):
        temporary = self.root / "control.tmp"
        temporary.write_text(json.dumps({"cmd": value}), encoding="utf-8")
        temporary.replace(self.control_path)

    def runner(self, device):
        return FLOW.Runner(
            FLOW.DecisionCore(self.config), device,
            FLOW.Ledger(str(self.root / "ledger.json")),
            FLOW.ControlFile(str(self.control_path)),
            str(self.root / "timeline.jsonl"), poll_interval=0.01,
        )

    def events(self):
        return [json.loads(line) for line in
                (self.root / "timeline.jsonl").read_text(encoding="utf-8").splitlines()]

    def assert_released(self, runner, device):
        self.assertTrue(device.released)
        self.assertTrue(runner.timeline.closed)

    def test_normal_flow_commits_once_and_releases(self):
        device = Device()
        runner = self.runner(device)
        runner.run()
        self.assertEqual(device.inputs, [(FLOW.Action.COLLECT_REWARD, 1)])
        self.assertEqual(runner.ledger.data, {"done": ["collect_reward"], "pending": []})
        self.assert_released(runner, device)

    def test_stop_before_observe_never_prepares(self):
        self.command("estop")
        device = Device()
        runner = self.runner(device)
        result = runner.run()
        self.assertIsNotNone(result["stop_requested_at"])
        self.assertEqual(device.observations, 0)
        self.assertEqual(device.inputs, [])
        self.assert_released(runner, device)

    def test_stop_written_during_observe_blocks_action(self):
        device = Device()

        def observe(checkpoint):
            self.command("stop")
            return FLOW.Observation(FLOW.State.READY)

        device.observe = observe
        runner = self.runner(device)
        runner.run()
        self.assertEqual(device.preparations, 0)
        self.assertEqual(device.inputs, [])
        self.assert_released(runner, device)

    def test_stop_during_prepare_even_without_callback_blocks_input(self):
        device = Device()

        def prepare(action, checkpoint):
            self.command("estop")
            return "stale-location"

        device.prepare = prepare
        runner = self.runner(device)
        runner.run()
        self.assertEqual(device.inputs, [])
        self.assertEqual(runner.ledger.data["pending"], [])
        self.assert_released(runner, device)

    def test_cooperative_prepare_stops_while_still_waiting(self):
        device = Device()
        entered = threading.Event()
        finished = threading.Event()
        errors = []

        def prepare(action, checkpoint):
            entered.set()
            while not finished.wait(0.01):
                checkpoint()
            return "must-not-use"

        device.prepare = prepare
        runner = self.runner(device)

        def run():
            try:
                runner.run()
            except Exception as exc:
                errors.append(exc)
            finally:
                finished.set()

        thread = threading.Thread(target=run)
        thread.start()
        try:
            self.assertTrue(entered.wait(2), errors)
            self.command("estop")
            self.assertTrue(finished.wait(1), "停止必须在准备完成之前生效")
        finally:
            finished.set()
            thread.join(2)
        self.assertFalse(thread.is_alive())
        self.assertEqual(errors, [])
        self.assertEqual(device.inputs, [])
        self.assert_released(runner, device)

    def test_prepare_cancellation_reaps_owned_subprocess(self):
        device = Device()
        children = []

        def prepare(action, checkpoint):
            process = subprocess.Popen(
                [sys.executable, "-c", "import time; time.sleep(30)"],
                stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0,
            )
            children.append(process)
            try:
                self.command("estop")
                while process.poll() is None:
                    checkpoint()
                    try:
                        process.wait(timeout=0.01)
                    except subprocess.TimeoutExpired:
                        pass
            finally:
                if process.poll() is None:
                    process.terminate()
                process.wait(timeout=2)
            return "cancelled-result"

        device.prepare = prepare
        runner = self.runner(device)
        runner.run()
        self.assertIsNotNone(children[0].poll())
        self.assertEqual(device.inputs, [])
        self.assert_released(runner, device)

    def test_stop_after_pending_is_not_committed(self):
        device = Device()
        runner = self.runner(device)
        begin = runner.ledger.begin

        def begin_then_stop(action):
            begin(action)
            self.command("estop")

        runner.ledger.begin = begin_then_stop
        runner.run()
        self.assertEqual(device.inputs, [])
        self.assertEqual(runner.ledger.data, {"done": [], "pending": ["collect_reward"]})
        self.assert_released(runner, device)

    def test_pause_during_prepare_discards_plan_and_resume_reobserves(self):
        device = Device()
        original_observe = device.observe
        original_prepare = device.prepare
        paused_observations = []
        prepare_observations = []
        runner = self.runner(device)

        def prepare(action, checkpoint):
            prepared = original_prepare(action, checkpoint)
            prepare_observations.append(device.observations)
            if prepared == 1:
                self.command("pause")
                checkpoint()
            return prepared

        def observe(checkpoint):
            result = original_observe(checkpoint)
            if runner.paused:
                paused_observations.append(device.observations)
                self.assertEqual(device.inputs, [])
                self.command("resume")
            return result

        device.prepare = prepare
        device.observe = observe
        runner.run()
        self.assertEqual(device.inputs, [(FLOW.Action.COLLECT_REWARD, 2)])
        self.assertEqual(len(paused_observations), 1)
        self.assertGreater(prepare_observations[1], paused_observations[0])
        self.assert_released(runner, device)

    def test_prepare_exception_releases_without_pending(self):
        device = Device()

        def prepare(action, checkpoint):
            raise RuntimeError("locator failed")

        device.prepare = prepare
        runner = self.runner(device)
        with self.assertRaisesRegex(RuntimeError, "locator failed"):
            runner.run()
        self.assertEqual(device.inputs, [])
        self.assertEqual(runner.ledger.data["pending"], [])
        self.assert_released(runner, device)

    def test_failed_input_keeps_pending_and_releases(self):
        device = Device()
        runner = self.runner(device)
        runner.ledger.data = {"done": [], "pending": []}

        def act(action, prepared):
            raise RuntimeError("input result unknown")

        device.act = act
        with self.assertRaises(RuntimeError):
            runner.run()
        self.assertEqual(runner.ledger.data, {"done": [], "pending": ["collect_reward"]})
        self.assert_released(runner, device)

    def test_pending_restart_pauses_without_input(self):
        device = Device()
        runner = self.runner(device)
        runner.ledger.begin(FLOW.Action.COLLECT_REWARD)
        original_observe = device.observe

        def observe(checkpoint):
            result = original_observe(checkpoint)
            if runner.paused:
                self.command("stop")
            return result

        device.observe = observe
        runner.run()
        self.assertEqual(device.inputs, [])
        self.assertEqual(runner.ledger.data, {"done": [], "pending": ["collect_reward"]})
        self.assert_released(runner, device)

    def test_poll_interval_rejects_unbounded_or_busy_loop(self):
        for interval in (0, -0.01, 0.201, float("nan")):
            with self.subTest(interval=interval), self.assertRaises(ValueError):
                FLOW.Runner(
                    FLOW.DecisionCore(self.config), Device(),
                    FLOW.Ledger(str(self.root / "ledger.json")),
                    FLOW.ControlFile(str(self.control_path)),
                    str(self.root / "timeline.jsonl"), interval,
                )


if __name__ == "__main__":
    unittest.main()
