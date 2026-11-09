"""windows-flow 定位蒸馏与合成弹窗 fixture 的直接测试；不调用模型、不开窗口、不注入系统输入。

- LocatorPolicyTests：DistilledLocator 策略（缓存 key、失配回退、歧义/低纹理/越界
  安全失败、无文件 I/O、checkpoint 异常传播），stub matcher，零第三方依赖。
- ImagePipelineTests / EndToEndPopupTests：真实图像管线（numpy+opencv）与端到端
  workflow（复用原 Runner），依赖缺失时整类 skip 并注明原因。
"""
import importlib.util
import json
import tempfile
import threading
import time
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
EXAMPLE = ROOT / "packs/game-automation/tooling/examples/windows-flow"

_LOADED = {}


def _load(name):
    if name not in _LOADED:
        spec = importlib.util.spec_from_file_location(
            "steamai_wf_" + name, str(EXAMPLE / (name + ".py")))
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        _LOADED[name] = module
    return _LOADED[name]


SD = _load("synth_device")   # 复用原 workflow，不建第二套 Runner
WF = SD.workflow
LZ = SD.localize

try:
    import cv2  # noqa: F401
    import numpy  # noqa: F401
    HAVE_IMAGING = True
except ImportError:
    HAVE_IMAGING = False

SKIP_IMAGING = "numpy/opencv 不可用：图像管线与端到端测试跳过（策略单测零依赖仍运行）"
# key 形状与 SynthDevice.key_for 一致：(实例 token, action, 目标名, 状态)
KEY = ("tok", "collect_reward", "reward_button", "ready")


def noop_checkpoint():
    pass


class StubMatcher(object):
    """测试接缝：只在单测里替代 CvMatcher；真实图像匹配只有 CvMatcher 一个实现。"""

    def __init__(self, score=0.99, runner_up=0.1, top_left=(20, 30), texture=99.0):
        self.score, self.runner_up, self.top_left, self.texture = score, runner_up, top_left, texture
        self.match_calls = []

    def size(self, screen):
        return screen["size"]

    def template_size(self, template):
        return (40, 40)

    def crop(self, screen, bbox):
        return {"screen": screen["id"], "bbox": bbox}

    def texture_score(self, template):
        return self.texture

    def match(self, template, screen):
        self.match_calls.append((template, screen))
        return LZ.Match(self.top_left, self.score, self.runner_up)


class StubCallback(object):
    def __init__(self, bbox=(50, 60, 40, 40)):
        self.bbox = bbox
        self.calls = []

    def __call__(self, key, screen, checkpoint):
        self.calls.append(key)
        return self.bbox


class LocatorPolicyTests(unittest.TestCase):
    def make(self, matcher=None, callback=None, **kwargs):
        matcher = matcher or StubMatcher()
        callback = callback or StubCallback()
        return LZ.DistilledLocator(callback=callback, matcher=matcher, **kwargs), callback, matcher

    def test_callback_once_then_distilled_hit_without_second_callback(self):
        locator, callback, matcher = self.make()
        screen = {"id": "s1", "size": (640, 480)}
        first = locator.locate(KEY, screen, noop_checkpoint)
        self.assertEqual(first.source, "callback")
        self.assertEqual(first.point, (70, 80))
        second = locator.locate(KEY, screen, noop_checkpoint)
        self.assertEqual(second.source, "distilled")
        self.assertEqual(second.point, (40, 50))  # top_left(20,30) + 模板半宽高
        self.assertEqual(len(callback.calls), 1)
        self.assertEqual(len(matcher.match_calls), 1)

    def test_float_bbox_accepted_as_integers(self):
        locator, _, _ = self.make(callback=StubCallback((50.0, 60.0, 40.0, 40.0)))
        result = locator.locate(KEY, {"id": "s", "size": (640, 480)}, noop_checkpoint)
        self.assertEqual((result.point, result.bbox), ((70, 80), (50, 60, 40, 40)))

    def test_low_score_falls_back_to_callback_and_updates_template(self):
        matcher = StubMatcher(score=0.2)
        locator, callback, _ = self.make(matcher=matcher)
        screen = {"id": "s", "size": (640, 480)}
        locator.locate(KEY, screen, noop_checkpoint)  # 首次：callback
        fallen = locator.locate(KEY, screen, noop_checkpoint)
        self.assertEqual(fallen.source, "fallback")
        self.assertEqual(len(callback.calls), 2)
        matcher.score = 0.99
        again = locator.locate(KEY, screen, noop_checkpoint)
        self.assertEqual(again.source, "distilled")  # 失配后模板已更新

    def test_frame_size_change_invalidates_cache(self):
        locator, callback, _ = self.make()
        small = {"id": "a", "size": (640, 480)}
        large = {"id": "b", "size": (800, 600)}
        locator.locate(KEY, small, noop_checkpoint)
        resized = locator.locate(KEY, large, noop_checkpoint)
        self.assertEqual(resized.source, "fallback")
        self.assertEqual(len(callback.calls), 2)
        self.assertEqual(locator.locate(KEY, large, noop_checkpoint).source, "distilled")

    def test_ambiguous_is_safe_failure_without_callback_fallback(self):
        matcher = StubMatcher(score=0.99, runner_up=0.95)
        locator, callback, _ = self.make(matcher=matcher)
        screen = {"id": "s", "size": (640, 480)}
        locator.locate(KEY, screen, noop_checkpoint)
        with self.assertRaises(LZ.LocateFailed) as ctx:
            locator.locate(KEY, screen, noop_checkpoint)
        self.assertEqual(ctx.exception.reason, "ambiguous")
        self.assertEqual(len(callback.calls), 1)  # 歧义不回调刷结果

    def test_key_discipline_action_state_identity_do_not_cross_use(self):
        locator, callback, _ = self.make()
        screen = {"id": "s", "size": (640, 480)}
        other_state = ("collect_reward", "reward_button", "loading")
        other_target = ("start", "start_button", "ready")
        locator.locate(KEY, screen, noop_checkpoint)
        locator.locate(other_state, screen, noop_checkpoint)
        locator.locate(other_target, screen, noop_checkpoint)
        self.assertEqual(len(callback.calls), 3)
        self.assertEqual(locator.cached_keys(), sorted([KEY, other_state, other_target]))
        # 同 key 复用缓存：不新增回调
        locator.locate(KEY, screen, noop_checkpoint)
        self.assertEqual(len(callback.calls), 3)

    def test_invalid_callback_bbox_rejected_and_nothing_cached(self):
        variants = [
            ((50, 60, 40), "callback_bbox_malformed"),
            ("oops", "callback_bbox_malformed"),
            (["10", "10", "12", "12"], "callback_bbox_malformed"),
            ((True, 60, 40, 40), "callback_bbox_malformed"),
            ((float("nan"), 60, 40, 40), "callback_bbox_malformed"),
            ((float("inf"), 60, 40, 40), "callback_bbox_malformed"),
            ((-5, 60, 40, 40), "callback_bbox_out_of_bounds"),
            ((10, 10, 4, 40), "callback_bbox_too_small"),
            ((600, 60, 80, 40), "callback_bbox_out_of_bounds"),
            ((50, 470, 40, 40), "callback_bbox_out_of_bounds"),
        ]
        for bbox, reason in variants:
            with self.subTest(bbox=bbox):
                locator, _, matcher = self.make(callback=StubCallback(bbox))
                # 不合格的 bbox 必须在写缓存之前以 LocateFailed 拒绝
                # （不是 TypeError 逃逸，也不留半成品缓存）
                with self.assertRaises(LZ.LocateFailed) as ctx:
                    locator.locate(KEY, {"id": "s", "size": (640, 480)}, noop_checkpoint)
                self.assertEqual(ctx.exception.reason, reason)
                self.assertEqual(locator.cached_keys(), [])
                self.assertEqual(matcher.match_calls, [])

    def test_low_texture_template_rejected_and_not_cached(self):
        matcher = StubMatcher(texture=3.0)
        locator, _, _ = self.make(matcher=matcher)
        with self.assertRaises(LZ.LocateFailed) as ctx:
            locator.locate(KEY, {"id": "s", "size": (640, 480)}, noop_checkpoint)
        self.assertEqual(ctx.exception.reason, "low_texture")
        self.assertEqual(locator.cached_keys(), [])

    def test_locator_performs_no_file_io(self):
        locator, _, _ = self.make()
        screen = {"id": "s", "size": (640, 480)}
        with mock.patch("builtins.open", side_effect=AssertionError("定位缓存只存内存")):
            locator.locate(KEY, screen, noop_checkpoint)
            locator.locate(KEY, screen, noop_checkpoint)

    def test_checkpoint_exception_propagates_unwrapped(self):
        class Cancel(Exception):
            pass

        counter = {"n": 0}

        def checkpoint():
            counter["n"] += 1
            if counter["n"] >= 2:
                raise Cancel()

        class PollingCallback(object):
            def __call__(self, key, screen, checkpoint):
                checkpoint()
                checkpoint()  # 第二次触发 Cancel
                return (50, 60, 40, 40)

        locator = LZ.DistilledLocator(callback=PollingCallback(), matcher=StubMatcher())
        with self.assertRaises(Cancel):
            locator.locate(KEY, {"id": "s", "size": (640, 480)}, checkpoint)
        self.assertEqual(locator.cached_keys(), [])  # 不留半成品模板


@unittest.skipUnless(HAVE_IMAGING, SKIP_IMAGING)
class ImagePipelineTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.SynthTarget = _load("synth_target").SynthTarget

    def make(self, **target_kw):
        target = self.SynthTarget(loading_seconds=0.0, result_seconds=0.0, **target_kw)
        callback = SD.ScriptedLocateCallback(target, dwell_seconds=0.0)
        locator = LZ.DistilledLocator(callback=callback, matcher=LZ.CvMatcher())
        return target, callback, locator

    def ready_screen(self, target):
        x, y, w, h = target.target_bbox("start_button")
        self.assertEqual(target.click(x + w // 2, y + h // 2), "start")
        return target.capture()  # loading 0s：首次 tick 即 ready

    def test_distilled_full_image_match_tracks_moved_target(self):
        target, callback, locator = self.make()
        screen = self.ready_screen(target)
        first = locator.locate(KEY, screen, noop_checkpoint)
        self.assertEqual(first.source, "callback")
        bx, by, bw, bh = target.target_bbox("reward_button")
        center = (bx + bw // 2, by + bh // 2)
        self.assertTrue(abs(first.point[0] - center[0]) <= 1
                        and abs(first.point[1] - center[1]) <= 1, first)
        target.set_target_pos("reward_button", 100, 200)  # 目标位移
        second = locator.locate(KEY, target.capture(), noop_checkpoint)
        self.assertEqual(second.source, "distilled")
        self.assertEqual(callback.calls[KEY], 1)  # 位移后仍不回调
        self.assertTrue(abs(second.point[0] - 122) <= 2
                        and abs(second.point[1] - 218) <= 2, second)
        self.assertGreaterEqual(second.score, 0.85)

    def test_low_texture_background_bbox_rejected_then_recovers(self):
        target, callback, locator = self.make()
        screen = self.ready_screen(target)
        callback.override(KEY, (10, 10, 32, 32))  # 纯背景角：纹理不足
        with self.assertRaises(LZ.LocateFailed) as ctx:
            locator.locate(KEY, screen, noop_checkpoint)
        self.assertEqual(ctx.exception.reason, "low_texture")
        self.assertEqual(locator.cached_keys(), [])
        callback.clear_override(KEY)
        self.assertEqual(locator.locate(KEY, screen, noop_checkpoint).source, "callback")

    def test_out_of_bounds_bbox_rejected(self):
        target, callback, locator = self.make()
        screen = self.ready_screen(target)
        for bbox in ((470, 300, 44, 36), (10, -5, 44, 36)):
            with self.subTest(bbox=bbox):
                callback.override(KEY, bbox)
                with self.assertRaises(LZ.LocateFailed) as ctx:
                    locator.locate(KEY, screen, noop_checkpoint)
                self.assertEqual(ctx.exception.reason, "callback_bbox_out_of_bounds")
                self.assertEqual(locator.cached_keys(), [])
        callback.clear_override(KEY)

    def test_duplicate_target_is_ambiguous_safe_failure(self):
        target, callback, locator = self.make()
        screen = self.ready_screen(target)
        locator.locate(KEY, screen, noop_checkpoint)  # 缓存模板
        bx, by, bw, bh = target.target_bbox("reward_button")
        duplicate = screen.copy()
        duplicate[220:220 + bh, 60:60 + bw] = screen[by:by + bh, bx:bx + bw]
        with self.assertRaises(LZ.LocateFailed) as ctx:
            locator.locate(KEY, duplicate, noop_checkpoint)
        self.assertEqual(ctx.exception.reason, "ambiguous")
        self.assertEqual(callback.calls[KEY], 1)  # 歧义不回退刷结果

    def test_solid_color_region_rejected_as_low_texture(self):
        # 回归：全元素 std 会把纯色块（通道差大，如纯红）误当纹理；
        # 空间 std 必须拒绝任何纯色区域。
        import numpy
        target, callback, locator = self.make()
        solid = numpy.zeros((320, 480, 3), numpy.uint8)
        solid[:, :] = (0, 0, 255)  # 纯红（通道序不影响结论：空间上无变化）
        callback.override(KEY, (100, 100, 24, 24))
        with self.assertRaises(LZ.LocateFailed) as ctx:
            locator.locate(KEY, solid, noop_checkpoint)
        self.assertEqual(ctx.exception.reason, "low_texture")
        self.assertEqual(locator.cached_keys(), [])

    def test_popup_render_matches_flag_and_close_path(self):
        # 弹窗显隐必须反映在帧内容上（自报 state 不掩盖图像错误）
        target = self.SynthTarget()
        center = (target.height // 2, target.width // 2)
        target.show_popup()
        frame = target.capture()
        self.assertEqual(tuple(frame[center]), (90, 90, 90), "弹窗激活必须渲染")
        target.close_popup()
        frame = target.capture()
        self.assertNotEqual(tuple(frame[center]), (90, 90, 90), "关闭后必须消失")

    def test_popup_autoclose_reflected_in_frame(self):
        target = self.SynthTarget(popup_close_after_ms=120)
        center = (target.height // 2, target.width // 2)
        target.show_popup()
        self.assertEqual(tuple(target.capture()[center]), (90, 90, 90))
        time.sleep(0.2)
        frame = target.capture()
        self.assertFalse(target.popup_active)
        self.assertNotEqual(tuple(frame[center]), (90, 90, 90), "到时自动关闭必须渲染")

    def test_popup_close_after_ms_contract(self):
        for value in (-1, -0.5, "soon", True):
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    self.SynthTarget(popup_close_after_ms=value)
        never = self.SynthTarget(popup_close_after_ms=0)  # 0 = 永久不关
        never.show_popup()
        time.sleep(0.15)
        frame = never.capture()
        self.assertTrue(never.popup_active, "0 必须等同于不自动关闭")
        self.assertEqual(tuple(frame[never.height // 2, never.width // 2]), (90, 90, 90))

    def test_set_target_pos_rejects_out_of_frame(self):
        # 按钮出画直接拒绝：不产生残缺渲染（不复现历史"位移出截图"缺陷题）
        target, _, _ = self.make()
        for x, y in [(-5, 100), (100, -5), (470, 100), (100, 310)]:
            with self.subTest(pos=(x, y)):
                with self.assertRaises(ValueError):
                    target.set_target_pos("reward_button", x, y)
        self.assertEqual(target.positions["reward_button"],
                         tuple(self.SynthTarget().positions["reward_button"]))

    def test_instance_token_isolates_shared_locator(self):
        # 同名按钮的不同目标实例不得共享缓存：key 携带实例 token
        target_a, callback, locator = self.make()
        device_a = SD.SynthDevice(target_a, locator)
        first = device_a.prepare(WF.Action.START, noop_checkpoint)
        self.assertEqual(first.source, "callback")
        target_b = self.SynthTarget(loading_seconds=0.0, result_seconds=0.0)
        device_b = SD.SynthDevice(target_b, locator)
        second = device_b.prepare(WF.Action.START, noop_checkpoint)
        self.assertEqual(second.source, "callback", "不同实例不得蒸馏命中")
        self.assertNotEqual(device_a.key_for(WF.Action.START),
                            device_b.key_for(WF.Action.START))

    def test_stale_prepared_input_refused_before_click(self):
        # prepare 之后目标变化：发输入前复核失败——不点击、保留 pending 语义
        for mutate in ("popup", "move"):
            with self.subTest(mutate=mutate):
                target, _, locator = self.make()
                device = SD.SynthDevice(target, locator)
                prepared = device.prepare(WF.Action.START, noop_checkpoint)
                if mutate == "popup":
                    target.show_popup()
                else:
                    target.set_target_pos("start_button", 200, 40)
                with self.assertRaises(WF.InputUnconfirmed):
                    device.act(WF.Action.START, prepared)
                self.assertEqual(len(target.clicks), 0, "过期凭证不得发出点击")

    def test_callback_mutating_target_does_not_smuggle_stale_point(self):
        # 回归（终审复现）：定位回调收到旧 frame 后移动目标、再返回旧 frame 的
        # 合法 bbox——prepare 冻结旧 frame 的 revision，act 复核必须拒绝，
        # 不得把定位期间的新 revision 贴到旧位置上发出点击。
        class MutatingCallback(object):
            def __init__(self, target):
                self.target = target
                self.calls = 0

            def __call__(self, key, screen, checkpoint):
                self.calls += 1
                checkpoint()
                name = key[2]
                old = self.target.target_bbox(name)
                self.target.set_target_pos(name, 120, 160)  # 回调期间位移（画内）
                return old  # 旧 frame 的合法 bbox

        target, _, _ = self.make()
        device = SD.SynthDevice(
            target,
            LZ.DistilledLocator(callback=MutatingCallback(target),
                                matcher=LZ.CvMatcher()))
        prepared = device.prepare(WF.Action.START, noop_checkpoint)
        self.assertEqual(prepared.source, "callback")
        with self.assertRaises(WF.InputUnconfirmed):
            device.act(WF.Action.START, prepared)
        self.assertEqual(len(target.clicks), 0, "定位期间目标变化：不得发出点击")

    def test_act_ticks_time_based_change_before_compare(self):
        # 回归：act 必须先推进目标时间（_tick）再比较凭证——到期的弹窗自动
        # 关闭要在比较前生效，不能等 click 内部才发现时序变化
        now = [0.0]
        target = self.SynthTarget(popup_close_after_ms=100, clock=lambda: now[0])
        callback = SD.ScriptedLocateCallback(target, dwell_seconds=0.0)
        locator = LZ.DistilledLocator(callback=callback, matcher=LZ.CvMatcher())
        device = SD.SynthDevice(target, locator)
        target.show_popup()
        prepared = SD.Prepared(point=(240, 160), bbox=(220, 140, 40, 40),
                               source="callback", state="dialog",
                               revision=target.revision)
        now[0] = 0.2  # 弹窗到期：只有 _tick 会应用关闭
        with self.assertRaises(WF.InputUnconfirmed):
            device.act(WF.Action.COLLECT_REWARD, prepared)
        self.assertFalse(target.popup_active, "act 前置 capture 应已推进时间")
        self.assertEqual(len(target.clicks), 0)


@unittest.skipUnless(HAVE_IMAGING, SKIP_IMAGING)
class EndToEndPopupTests(unittest.TestCase):
    """端到端走原 workflow.Runner：合成通路，无真实 LLM/GUI。"""

    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="steamai-distill-e2e-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.control_path = self.root / "control.json"
        self.command("none")
        self.config = {"recognition": {"image_threshold": 0.85},
                       "limits": {"step_wait_timeout_seconds": 3, "retry_limit_per_step": 2}}
        self.SynthTarget = _load("synth_target").SynthTarget

    def command(self, value):
        temporary = self.root / "control.tmp"
        temporary.write_text(json.dumps({"cmd": value}), encoding="utf-8")
        temporary.replace(self.control_path)

    def make_device(self, **target_kw):
        target = self.SynthTarget(loading_seconds=0.08, result_seconds=0.08, **target_kw)
        callback = SD.ScriptedLocateCallback(target, dwell_seconds=0.02)
        locator = LZ.DistilledLocator(callback=callback, matcher=LZ.CvMatcher())
        return target, callback, SD.SynthDevice(target, locator)

    def runner_for(self, device, tag):
        return WF.Runner(
            WF.DecisionCore(self.config), device,
            WF.Ledger(str(self.root / ("ledger-%s.json" % tag))),
            WF.ControlFile(str(self.control_path)),
            str(self.root / ("timeline-%s.jsonl" % tag)), 0.01)

    def wait_until(self, predicate, timeout=5.0):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if predicate():
                return True
            time.sleep(0.005)
        return False

    def test_permanent_popup_pauses_without_blind_click_or_auto_recovery(self):
        target, _, device = self.make_device(
            popup_close_after_ms=None, popup_trigger_state="loading")
        runner = self.runner_for(device, "perm")
        flags = {}

        def user_side():
            flags["paused_with_popup"] = self.wait_until(
                lambda: runner.paused and target.popup_active)
            time.sleep(0.3)  # 默认永久：无自动关闭、无自动恢复
            flags["still_paused"] = runner.paused
            flags["popup_still_active"] = target.popup_active
            flags["no_collect_while_paused"] = not any(
                a is WF.Action.COLLECT_REWARD for a, _, _ in device.acts)
            self.command("stop")

        thread = threading.Thread(target=user_side)
        thread.start()
        result = runner.run()
        thread.join(2)
        self.assertTrue(flags.get("paused_with_popup"))
        self.assertTrue(flags.get("still_paused"), "默认永久弹窗不得自动恢复")
        self.assertTrue(flags.get("popup_still_active"), "默认永久弹窗不得自动关闭")
        self.assertTrue(flags.get("no_collect_while_paused"))
        self.assertIsNotNone(result["stop_requested_at"])
        self.assertEqual(target.blind_clicks, 0)
        self.assertEqual(runner.ledger.data, {"done": [], "pending": []})
        self.assertTrue(device.released)

    def test_autoclose_popup_needs_explicit_resume_then_claims_once_per_run(self):
        target, callback, device = self.make_device(
            popup_close_after_ms=250, popup_trigger_state="loading")
        still_paused_after_close = []

        def user_side(runner):
            seen = self.wait_until(lambda: runner.paused and target.popup_active)
            closed = seen and self.wait_until(lambda: not target.popup_active, timeout=2)
            observed = device.observations
            closed = closed and self.wait_until(
                lambda: device.observations >= observed + 2, timeout=2)
            still_paused_after_close.append(bool(closed and runner.paused))
            self.command("resume")

        # run 1：首次授权定位（两次 callback），弹窗自动关闭后显式 resume 完成流程
        runner1 = self.runner_for(device, "run1")
        thread = threading.Thread(target=user_side, args=(runner1,))
        thread.start()
        result1 = runner1.run()
        thread.join(2)
        self.assertIsNone(result1["stop_requested_at"])
        self.assertEqual(runner1.ledger.data, {"done": ["collect_reward"], "pending": []})
        self.assertEqual([hit for _, _, hit in device.acts].count("reward"), 1)
        self.assertEqual([source for _, source, _ in device.locations],
                         ["callback", "callback"])
        self.assertTrue(still_paused_after_close[0], "弹窗自动关闭不得触发自动恢复")

        # run 2：同进程同目标实例，按钮位移——跨 run 只复用内存缓存（蒸馏命中），
        # 回调不再触发；仍需一次显式 resume，且非幂等动作再次恰好一次
        target.reset()
        target.set_target_pos("reward_button", 120, 180)
        self.command("none")
        clicks_before = len(target.clicks)
        runner2 = self.runner_for(device, "run2")
        thread = threading.Thread(target=user_side, args=(runner2,))
        thread.start()
        result2 = runner2.run()
        thread.join(2)
        self.assertIsNone(result2["stop_requested_at"])
        self.assertEqual(runner2.ledger.data, {"done": ["collect_reward"], "pending": []})
        new_reward_hits = [c.hit for c in target.clicks[clicks_before:]].count("reward")
        self.assertEqual(new_reward_hits, 1)
        self.assertEqual([source for _, source, _ in device.locations],
                         ["callback", "callback", "distilled", "distilled"])
        self.assertEqual(sum(callback.calls.values()), 2, "位移后不得再触发授权回调")
        point = device.locations[-1][2]
        self.assertTrue(abs(point[0] - 142) <= 2 and abs(point[1] - 198) <= 2, point)
        self.assertEqual(target.blind_clicks, 0)
        self.assertTrue(device.released)

    def test_stop_during_authorized_callback_interrupts_without_side_effects(self):
        target, callback, device = self.make_device()  # 无弹窗
        runner = self.runner_for(device, "stop")
        wrote = []

        def user_side():
            self.assertTrue(callback.entered.wait(2), "授权回调应已进入")
            wrote.append(time.time())
            self.command("estop")

        thread = threading.Thread(target=user_side)
        thread.start()
        result = runner.run()
        thread.join(2)
        self.assertIsNotNone(result["stop_requested_at"])
        self.assertLess(result["stop_requested_at"] - wrote[0], 2.0,
                        "停止响应须 ≤2s（回调内 checkpoint 轮询）")
        self.assertEqual(device.acts, [])
        self.assertEqual(device.locator.cached_keys(), [], "中断不得留下半成品模板")
        self.assertEqual(target.blind_clicks, 0)
        self.assertTrue(device.released)


if __name__ == "__main__":
    unittest.main()
