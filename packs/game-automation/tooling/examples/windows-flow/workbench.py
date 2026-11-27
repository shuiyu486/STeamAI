# -*- coding: utf-8 -*-
"""参照-候选工作台（workbench）：同一场景分别跑参照与候选，逐步记录外部证据（应用状态/绑定/
画面/识别/输入/后置状态），比对首差、给出能区分故障的下一步，并保留"没有证据=未知"。

四个命令（退出码分开两件事：run/observe 的 0 = 工作流达成；compare 的 0 = 外部行为相同且两份都
确有目标达成、声明的保留项有证据且无冲突（两份相同的坏轨迹是 not-achieved，缺终态/关键证据是
unknown，都不是 0）；matrix 的 0 = 每项实际结果都符合 plan 的声明，不等于工作流成功；1 = 未达成/
有差异/关键证据缺失/保留项冲突/不符合声明；2 = 用法/基础设施错误，含 SHA-256 不一致拒绝启动、
不安全 id、产物已存在）：
  run      --scenario S.json [--variant V] --out T.json
           合成目标：numpy 内存画面，无窗口、无子进程、无系统输入。
  observe  --scenario S.json --exe PATH --exe-sha256 HEX [--placement offscreen|hidden]
           [--variant V] --out T.json
           自建 dm_demo_target 子进程窗口。SHA-256 由调用者独立给出，启动前按文件字节核对，
           不一致/缺失就不启动（位置或文件名不能证明是哪份程序）；只操作这个子进程自己的窗口。
  compare  A.json B.json [--out R.json]
           两份观察的首差/未知/慢/边界；只认外部行为，可换实现（坐标、等待）列为实现差异，
           但场景/变体声明必须保留的行为（含声明为证据的坐标）不一致时不得判相同。
  matrix   --plan P.json --out DIR [--exe PATH --exe-sha256 HEX]
           一次跑完基线/坏候选/修后/失效拒绝/暂停恢复，每项记录实际结果并与声明核对；启动任何
           目标前先预检 id（短、唯一、无路径分隔）、产物不出 out、不覆盖已有文件。
所有 --out/--out 目录下的产物都不覆盖已有文件，compare --out 不能指回输入。

场景文件只写"做什么"（步骤、前置 when、预期 expect、控件布局、画面区域）；variant 写"这次
自动化怎么做"（候选参数、扰动、声明保留/替换的行为及其证据路径、期望结果）。产品不按
case 写死：区域、控件、步骤都来自场景文件。

执行框架原样复用 workflow.Runner/Ledger/ControlFile（暂停/停止/非幂等登记），这里只换成按
步骤表决策的 ScenarioCore（等待 when → ACT → 由设备以目标自报与画面确认效果）并包一层记录
设备。不截屏、不取桌面 DC、不 SendInput、不装钩子、不调用模型。
"""

import sys

sys.dont_write_bytecode = True  # 必须在导入同目录模块之前：Fresh source closure 拒绝 pack 内 __pycache__

import argparse  # noqa: E402
import hashlib  # noqa: E402
import json  # noqa: E402
import os  # noqa: E402
import re  # noqa: E402
import shutil  # noqa: E402
import tempfile  # noqa: E402
import threading  # noqa: E402
import time  # noqa: E402

_HERE = os.path.dirname(os.path.abspath(__file__))
if _HERE not in sys.path:
    sys.path.insert(0, _HERE)

import localize  # noqa: E402
import refdiff  # noqa: E402
import synth_device  # noqa: E402
import workflow  # noqa: E402

SCHEMA_TRACE = "windows-flow-trace/1"
SCHEMA_SCENARIO = "windows-flow-scenario/1"
SCHEMA_PLAN = "windows-flow-plan/1"
DEFAULT_CANDIDATE = {"locate_dwell_ms": 0.0, "click_offset": [0, 0], "effect_budget_ms": 1000.0, "bad_bbox": {}}
FINAL_SETTLE_SECONDS = 0.5  # 主循环结束后再等多久取终态：区分"效果迟到"（终态满足 expect）与"没效果"
DEFAULT_CONFIG = {"wait_timeout_seconds": 5.0, "retry_limit": 3, "image_threshold": 0.85}
STEP_KEYS = ("label", "target", "when", "expect")


class UsageError(Exception):
    """用法/场景文件/计划文件错误：退出码 2。"""


class InfrastructureError(Exception):
    """目标无法启动/核对（含 SHA-256 不一致）：退出码 2，没有产生观察。"""


def _wall():
    return time.time()


def _perf():
    return time.perf_counter()


def _ms(seconds):
    return None if seconds is None else round(seconds * 1000.0, 2)


def sha256_text(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def load_json_file(path):
    try:
        with open(path, "r", encoding="utf-8") as handle:
            text = handle.read()
    except (OSError, UnicodeDecodeError) as exc:
        raise UsageError("读不了 %s：%s" % (path, exc))
    try:
        return json.loads(text), text
    except ValueError as exc:
        raise UsageError("%s 不是合法 JSON：%s" % (path, exc))


# -- 场景 / 变体 -----------------------------------------------------------------

def validate_scenario(scenario, path):
    if not isinstance(scenario, dict) or scenario.get("schema") != SCHEMA_SCENARIO:
        raise UsageError("%s：schema 必须是 %s" % (path, SCHEMA_SCENARIO))
    if not isinstance(scenario.get("name"), str) or not scenario["name"]:
        raise UsageError("%s：缺 name" % path)
    target = scenario.get("target")
    if not isinstance(target, dict) or target.get("kind") not in ("synth", "demo_target"):
        raise UsageError("%s：target.kind 必须是 synth 或 demo_target" % path)
    if target["kind"] == "demo_target":
        if not isinstance(target.get("pages"), int) or not 2 <= target["pages"] <= 8:
            raise UsageError("%s：demo_target 的 target.pages 必须是 2..8" % path)
        if not isinstance(scenario.get("controls"), dict) or not scenario["controls"]:
            raise UsageError("%s：demo_target 需要控件布局表 controls（客户区 bbox）" % path)
    steps = scenario.get("steps")
    if not isinstance(steps, list) or not steps:
        raise UsageError("%s：steps 必须是非空列表" % path)
    labels = set()
    for index, step in enumerate(steps):
        if not isinstance(step, dict) or not isinstance(step.get("label"), str) or not step["label"]:
            raise UsageError("%s：steps[%d] 缺 label" % (path, index))
        if step["label"] in labels:
            raise UsageError("%s：steps 标签重复 %r" % (path, step["label"]))
        labels.add(step["label"])
        if not isinstance(step.get("target"), str) or not step["target"]:
            raise UsageError("%s：steps[%d] 缺 target（控件/区域名）" % (path, index))
        for key in ("when", "expect"):
            value = step.get(key)
            if value is not None and not isinstance(value, dict):
                raise UsageError("%s：steps[%d].%s 必须是对象" % (path, index, key))
        if not step.get("expect"):
            raise UsageError("%s：steps[%d] 缺 expect：没有预期就没有可核对的达成证据" % (path, index))
        if target["kind"] == "demo_target" and step["target"] not in scenario["controls"]:
            raise UsageError("%s：steps[%d].target %r 不在 controls 里" % (path, index, step["target"]))
    regions = scenario.get("regions")
    if regions is not None and (not isinstance(regions, dict) or
                                any(not (isinstance(v, list) and len(v) == 4) for v in regions.values())):
        raise UsageError("%s：regions 必须是 {名称: [x,y,w,h]}" % path)
    return scenario


def load_scenario(path):
    scenario, text = load_json_file(path)
    validate_scenario(scenario, path)
    return scenario, sha256_text(text)


def validate_variant(variant):
    if not isinstance(variant, dict):
        raise UsageError("variant 必须是 JSON 对象")
    variant.setdefault("id", "variant")
    for key in ("candidate", "disturbance", "behaviors"):
        if key in variant and variant[key] is not None and not isinstance(variant[key], dict):
            raise UsageError("variant.%s 必须是对象" % key)
    disturbance = variant.get("disturbance")
    if disturbance:
        if disturbance.get("kind") not in ("pause", "move") or not disturbance.get("step"):
            raise UsageError("disturbance 需要 kind=pause|move 与 step")
        if disturbance["kind"] == "pause":
            disturbance.setdefault("resume_after_ms", 200)
    if variant.get("expect_result") not in (None, "confirmed", "unconfirmed", "refused", "locate-failed", "stopped", "paused", "unknown", "not-sent"):
        raise UsageError("expect_result 必须是 confirmed/unconfirmed/refused/locate-failed/stopped/paused/unknown/not-sent")
    return variant


def load_variant(spec, base_dir=None):
    """--variant：None → 默认；JSON 文件路径；或内联 JSON 对象。"""
    if not spec:
        return {"id": "default"}
    candidate_path = spec if os.path.isabs(spec) else os.path.join(base_dir or "", spec)
    if os.path.isfile(candidate_path):
        variant, _ = load_json_file(candidate_path)
    else:
        try:
            variant = json.loads(spec)
        except ValueError as exc:
            raise UsageError("--variant 既不是文件也不是合法 JSON：%s" % exc)
    return validate_variant(variant)


def variant_from_run(spec):
    """matrix 的一项 run → variant（id/candidate/disturbance/behaviors/expect_*，可带 variant 子对象）。"""
    variant = {"id": spec.get("id") or "run"}
    for key in ("candidate", "disturbance", "behaviors", "expect_result", "expect_fault"):
        if key in spec:
            variant[key] = spec[key]
    nested = spec.get("variant")
    if isinstance(nested, dict):
        for key, value in nested.items():
            if isinstance(variant.get(key), dict) and isinstance(value, dict):
                variant[key] = dict(variant[key], **value)
            else:
                variant[key] = value
    return validate_variant(variant)


# -- 决策核心 ----------------------------------------------------------------------

class StepAction(object):
    """步骤动作：value 形如 '0:to-page-2'，供 Runner 日志与 ledger 登记。"""

    def __init__(self, index, step):
        self.index = index
        self.step = step
        self.value = "%d:%s" % (index, step["label"])
        self.target = step["target"]
        self.expected_hit = step.get("hit")

    def __repr__(self):
        return "StepAction(%s)" % self.value


class ScenarioCore(object):
    """按步骤表决策：等待目标自报状态满足 when → ACT。所有步骤按非幂等登记（ledger 保护）。"""

    def __init__(self, steps, config):
        config = dict(DEFAULT_CONFIG, **(config or {}))
        self.actions = [StepAction(i, s) for i, s in enumerate(steps)]
        self.index = 0
        self.threshold = float(config["image_threshold"])
        self.wait_timeout = float(config["wait_timeout_seconds"])
        self.retry_limit = int(config["retry_limit"])

    def current(self):
        return self.actions[self.index] if self.index < len(self.actions) else None

    def is_non_idempotent(self, action):
        return True

    def decide(self, obs, ledger, step_ctx):
        decision = workflow.Decision
        if obs.confidence < self.threshold:
            return (decision.PAUSE, "low-confidence: %.2f" % obs.confidence)
        if obs.state is workflow.State.GONE:
            return (decision.STOP, "target gone")
        done = ledger.get("done", [])
        while self.index < len(self.actions) and self.actions[self.index].value in done:
            self.index += 1
        if self.index >= len(self.actions):
            return (decision.STOP, "flow complete")
        action = self.actions[self.index]
        if action.value in ledger.get("pending", []):
            return (decision.PAUSE, "unresolved pending step %s" % action.value)
        snapshot = getattr(obs, "snapshot", None)
        if not isinstance(snapshot, dict):
            step_ctx["retries"] += 1
            if step_ctx["retries"] >= self.retry_limit:
                return (decision.PAUSE, "no state evidence for step %s" % action.value)
            return (decision.WAIT, "state evidence missing, bounded retry")
        when = action.step.get("when")
        matches = refdiff.precondition_matches(snapshot, when)
        if matches is None:
            return (decision.PAUSE, "state evidence missing for %r in step %s" % (when, action.value))
        if not matches:
            if step_ctx["waited"] >= self.wait_timeout:
                return (decision.PAUSE, "precondition %r not reached (state=%r) for step %s" % (when, snapshot, action.value))
            return (decision.WAIT, "waiting %r for step %s" % (when, action.value))
        return (decision.ACT, action)


# -- 记录设备与扰动 ----------------------------------------------------------------

class Recorder(object):
    def __init__(self, steps, effect_budget_ms):
        self.steps = []
        for index, step in enumerate(steps):
            self.steps.append({"label": step["label"], "index": index, "target": step["target"],
                               "when": step.get("when"), "expect": step.get("expect"),
                               "effect_budget_ms": effect_budget_ms, "attempts": [],
                               "result": None, "duration_ms": None})
        self.current = None

    def begin_attempt(self, action):
        attempt = {"pre": None, "wait_ms": None, "locate": None, "locate_ms": None,
                   "input": None, "post": None, "effect_ms": None}
        self.steps[action.index]["attempts"].append(attempt)
        self.current = attempt
        return attempt


class Disturber(threading.Thread):
    """扰动线程：在某一步"定位完成、发输入之前"落地一次暂停（resume_after_ms 后恢复）或位移。
    prepare_done 是阻塞屏障：等主扰动（写入 pause 或完成位移）落地后才放行 prepare，保证
    扰动落在发输入复核之前，而不是靠停留时长碰运气；resume 不阻塞，稍后由本线程写入。"""

    def __init__(self, spec, write_control, mover):
        super(Disturber, self).__init__(daemon=True)
        self.spec = spec
        self.write = write_control
        self.mover = mover
        self.entered = threading.Event()
        self.done = threading.Event()
        self.stop = threading.Event()
        self.fired = False
        self.applied = None

    def prepare_done(self, label):
        if self.spec.get("step") != label or self.fired:
            return
        self.fired = True
        self.entered.set()
        self.done.wait(5.0)

    def run(self):
        deadline = _wall() + 60.0
        while not self.entered.is_set() and not self.stop.is_set() and _wall() < deadline:
            self.entered.wait(0.05)
        if not self.entered.is_set() or self.stop.is_set():
            self.applied = {"step": self.spec.get("step"), "kind": self.spec.get("kind"), "applied": False,
                            "note": "该步骤没有走到发输入前，扰动未发生"}
            return
        record = {"step": self.spec.get("step"), "kind": self.spec.get("kind"), "applied": True, "at": _wall()}
        try:
            if self.spec["kind"] == "pause":
                self.write("pause")
                self.applied = record  # 主扰动已落地：放行 prepare，让 Runner 的 checkpoint 看见 pause
                self.done.set()
                time.sleep(float(self.spec.get("resume_after_ms", 200)) / 1000.0)
                self.write("resume")
                record["detail"] = {"resume_after_ms": self.spec.get("resume_after_ms", 200), "resumed_at": _wall()}
                return
            record["detail"] = self.mover(self.spec)
        except Exception as exc:  # 扰动失败也是实际结果，记录后让主流程继续
            record.update(applied=False, error="%s: %s" % (type(exc).__name__, exc))
        self.applied = record
        self.done.set()


class Watchdog(threading.Thread):
    def __init__(self, write_control, deadline_seconds):
        super(Watchdog, self).__init__(daemon=True)
        self.write = write_control
        self.deadline = float(deadline_seconds)
        self.stop = threading.Event()
        self.timed_out = False

    def run(self):
        if self.stop.wait(self.deadline):
            return
        self.timed_out = True
        self.write("stop")


class TracingDevice(object):
    """包一层内层设备，把每一步的证据写进 Recorder；不改 Runner 合同。"""

    def __init__(self, inner, core, recorder, disturber=None):
        self.inner = inner
        self.core = core
        self.rec = recorder
        self.disturber = disturber
        self._step_started = None
        self.released = False

    def observe(self, checkpoint):
        obs = self.inner.observe(checkpoint)
        if self._step_started is None:
            self._step_started = _perf()
        if not hasattr(obs, "snapshot"):
            obs.snapshot = _safe(self.inner.snapshot)
        return obs

    def prepare(self, action, checkpoint):
        attempt = self.rec.begin_attempt(action)
        attempt["pre"] = {"t": _wall(), "state": _safe(self.inner.snapshot),
                          "binding": _safe(self.inner.binding), "picture": _safe(self.inner.picture)}
        attempt["wait_ms"] = _ms(None if self._step_started is None else _perf() - self._step_started)
        self._step_started = None
        started = _perf()
        try:
            prepared = self.inner.prepare(action, checkpoint)
        except localize.LocateFailed as exc:
            attempt["locate"] = {"error": exc.reason, "detail": exc.detail}
            attempt["locate_ms"] = _ms(_perf() - started)
            raise
        attempt["locate_ms"] = _ms(_perf() - started)
        attempt["locate"] = self.inner.describe_locate(prepared)
        if self.disturber is not None:
            self.disturber.prepare_done(action.step["label"])
        return prepared

    def act(self, action, prepared):
        attempt = self.rec.current
        probe = {}
        started = _perf()
        try:
            when = action.step.get("when")
            if when:
                for state in (attempt["pre"]["state"], _safe(self.inner.snapshot)):
                    if refdiff.precondition_matches(state, when) is not True:
                        probe.update(sent=False, reason_code="stale-state", reason="输入前无法确认前置条件 %r：%r" % (when, state))
                        raise workflow.InputUnconfirmed(probe["reason"])
            ok = self.inner.act(action, prepared, probe)
        except workflow.InputUnconfirmed as exc:
            attempt["input"] = _input_record(probe, exc)
            attempt["post"] = {"t": _wall(), "state": _safe(self.inner.snapshot), "picture": _safe(self.inner.picture),
                               "confirmed": False, "reason_code": probe.get("reason_code") or "unconfirmed",
                               "reason": str(exc)}
            attempt["effect_ms"] = _ms(_perf() - started)
            raise
        attempt["input"] = _input_record(probe)
        attempt["post"] = {"t": _wall(), "state": _safe(self.inner.snapshot), "picture": _safe(self.inner.picture),
                           "confirmed": bool(ok), "reason_code": probe.get("reason_code"), "reason": probe.get("reason")}
        attempt["effect_ms"] = _ms(_perf() - started)
        return ok is True and refdiff.effect_evidence(attempt, action.step) == refdiff.EVIDENCED

    def release(self):
        self.released = True
        self.inner.release()


def _input_record(probe, exc=None):
    return {"sent": probe.get("sent"), "kind": probe.get("kind"), "point": probe.get("point"),
            "delivered": probe.get("delivered"), "clicks_delta": probe.get("clicks_delta"),
            "hit": probe.get("hit"), "discarded": None,
            "reason_code": probe.get("reason_code"), "reason": probe.get("reason") or (str(exc) if exc else None)}


def _safe(fn):
    try:
        return fn()
    except Exception as exc:
        return None


# -- 合成内层设备 ------------------------------------------------------------------

class BadBboxCallback(synth_device.ScriptedLocateCallback):
    """授权定位回调替身 + 可按目标名注入坏 bbox（模拟识别错误候选）。"""

    def __init__(self, target, dwell_seconds, bad_bbox_for):
        super(BadBboxCallback, self).__init__(target, dwell_seconds=dwell_seconds)
        self.bad = dict(bad_bbox_for or {})

    def __call__(self, key, screen, checkpoint):
        bbox = super(BadBboxCallback, self).__call__(key, screen, checkpoint)
        return tuple(self.bad[key[2]]) if key[2] in self.bad else bbox


class SynthInner(object):
    def __init__(self, target, locator, regions, click_offset, effect_budget_ms=1000.0):
        self.target = target
        self.device = synth_device.SynthDevice(target, locator)
        self.regions = dict(regions or {})
        self.click_offset = (int(click_offset[0]), int(click_offset[1]))
        self.effect_budget_ms = float(effect_budget_ms)

    def observe(self, checkpoint):
        return self.device.observe(checkpoint)

    def snapshot(self):
        return {"state": self.target.state, "revision": self.target.revision}

    def binding(self):
        return self.device.binding()

    def picture(self):
        # 合成目标的契约：截图推进内部时间（loading→ready 等到时迁移只在截图时发生）。
        return refdiff.picture_fingerprint(self.target.capture(), self.regions)

    def describe_locate(self, prepared):
        return {"point": list(prepared.point), "bbox": list(prepared.bbox), "source": prepared.source, "score": None}

    def prepare(self, action, checkpoint):
        return self.device.prepare(action, checkpoint)

    def act(self, action, prepared, probe):
        point = (prepared.point[0] + self.click_offset[0], prepared.point[1] + self.click_offset[1])
        before = len(self.device.acts)
        probe.update({"sent": False, "kind": "synth-click", "point": None, "delivered": None,
                      "clicks_delta": None, "hit": None, "reason_code": None, "reason": None})
        try:
            self.device.act(action, prepared._replace(point=point))
        except workflow.InputUnconfirmed as exc:
            sent = len(self.device.acts) > before
            probe.update(sent=sent, point=list(point) if sent else None, delivered=True if sent else None,
                         clicks_delta=1 if sent else None, hit=self.device.acts[-1][2] if sent else None,
                         reason_code="hit-mismatch" if sent else "stale-state", reason=str(exc))
            raise
        probe.update(sent=True, point=list(point), delivered=True, clicks_delta=1, hit=self.device.acts[-1][2])
        # 命中只说明点到了按钮；效果按场景 expect 在预算内等目标自报状态（与真实窗口设备同一口径）。
        expect = getattr(action, "step", {}).get("expect")
        deadline = time.monotonic() + self.effect_budget_ms / 1000.0
        while True:
            self.target.capture()  # 合成目标的截图推进内部时间（状态到时迁移）
            state = {"state": self.target.state}
            if refdiff.state_matches(state, expect) is True:
                probe["reason_code"] = "confirmed"
                return True
            if time.monotonic() >= deadline:
                probe["reason_code"] = "no-effect" if prepared.state == state["state"] else "wrong-state"
                probe["reason"] = "click hit; state %s -> %s, expected %r" % (prepared.state, state["state"], expect)
                raise workflow.InputUnconfirmed(probe["reason"])
            time.sleep(0.001)

    def move(self, spec):
        self.target.set_target_pos(spec["target"], int(spec["to"][0]), int(spec["to"][1]))
        return {"target": spec["target"], "to": [int(spec["to"][0]), int(spec["to"][1])]}

    def release(self):
        self.device.release()


# -- 轨迹整理 ----------------------------------------------------------------------

def _step_result(step):
    attempts = step["attempts"]
    if not attempts:
        return "not-reached"
    last = attempts[-1]
    locate = last.get("locate")
    if isinstance(locate, dict) and locate.get("error"):
        return "locate-failed"
    inp = last.get("input")
    if not isinstance(inp, dict):
        return "unknown"
    if inp.get("discarded"):
        return "discarded"
    if inp.get("sent") is True:
        return "confirmed" if (last.get("post") or {}).get("confirmed") is True else "unconfirmed"
    if inp.get("sent") is False:
        return "refused" if inp.get("reason_code") in ("stale-binding", "stale-state") else "not-sent"
    return "unknown"


def finalize(scenario, variant, recorder, events, terminal, post_final, start_record, target_info,
             ledger_data, disturbance):
    last_control = None
    by_value = {"%d:%s" % (s["index"], s["label"]): s for s in recorder.steps}
    for event in events:
        name = event.get("event")
        if name.startswith("control."):
            last_control = name.split(".", 1)[1]
        elif name == "pause":
            last_control = "decision-pause"
        elif name in ("act.discard-prepared", "act.discard-pending"):
            step = by_value.get(event.get("action"))
            if step:
                for attempt in reversed(step["attempts"]):
                    if attempt.get("input") is None:
                        attempt["input"] = {"sent": False, "kind": None, "point": None, "delivered": None,
                                            "clicks_delta": None, "hit": None,
                                            "discarded": last_control or "discard", "reason_code": None,
                                            "reason": name}
                        break
    for step in recorder.steps:
        step["result"] = _step_result(step)
        first, last = step["attempts"][0] if step["attempts"] else None, step["attempts"][-1] if step["attempts"] else None
        if first and last and (first.get("pre") or {}).get("t") and (last.get("post") or {}).get("t"):
            step["duration_ms"] = round(((last["post"]["t"] - first["pre"]["t"]) * 1000.0), 2)
    start_t = next((e["t"] for e in events if e.get("event") == "run.start"), None)
    exit_t = next((e["t"] for e in reversed(events) if e.get("event") == "run.exit"), None)

    def phase_sum(key):
        values = []
        for step in recorder.steps:
            for attempt in step["attempts"]:
                inp = attempt.get("input") if isinstance(attempt.get("input"), dict) else {}
                if key == "effect_ms" and (inp.get("discarded") or inp.get("sent") is not True):
                    continue  # 没发输入的尝试没有"输入到效果"时长
                values.append(attempt.get(key))
        return round(sum(values), 2) if values and all(isinstance(v, (int, float)) for v in values) else None

    trace = {
        "schema": SCHEMA_TRACE,
        "scenario": {"name": scenario["name"], "sha256": None, "target_kind": scenario["target"]["kind"],
                     "steps": [s["label"] for s in recorder.steps]},
        "variant": {k: variant.get(k) for k in ("id", "candidate", "disturbance", "behaviors", "expect_result", "expect_fault")},
        "target": target_info,
        "start": start_record,
        "steps": recorder.steps,
        "post_final": post_final,
        "terminal": terminal,
        "control": [{"t": e.get("t"), "event": e.get("event"), "reason": e.get("reason")}
                    for e in events if e.get("event", "").startswith(("control.", "pause", "stop"))],
        "disturbances": [disturbance] if disturbance else [],
        "ledger": ledger_data,
        "timings": {"total_ms": round((exit_t - start_t) * 1000.0, 2) if start_t and exit_t else None,
                    "wait_ms": phase_sum("wait_ms"), "locate_ms": phase_sum("locate_ms"), "effect_ms": phase_sum("effect_ms")},
        "events": [e.get("event") for e in events][-400:],
    }
    trace["unknown"] = refdiff.trace_unknowns(trace)
    trace["outcome"] = refdiff.trace_outcome(trace)
    trace["boundary"] = refdiff.boundary_of(trace)
    return trace


def render_trace(trace):
    lines = ["场景: %s  变体: %s  目标: %s" % (
        trace["scenario"]["name"], trace["variant"]["id"],
        json.dumps({k: v for k, v in (trace.get("target") or {}).items() if k in ("kind", "placement", "pid", "pages", "closed_by", "exit_code", "exe_sha256")},
                   ensure_ascii=False))]
    lines.append("启动: 状态=%s 绑定=%s" % (_short(trace["start"].get("state")), _short(trace["start"].get("binding"))))
    for step in trace["steps"]:
        attempts = step["attempts"]
        if not attempts:
            lines.append("  [%d] %s: %s" % (step["index"], step["label"], step["result"]))
            continue
        for j, attempt in enumerate(attempts):
            inp, loc = attempt.get("input") or {}, attempt.get("locate") or {}
            lines.append("  [%d] %s%s: %s  wait=%s locate=%s effect=%s  输入 sent=%s delivered=%s point=%s%s  定位=%s" % (
                step["index"], step["label"], "" if len(attempts) == 1 else "(尝试%d)" % j, step["result"] if j == len(attempts) - 1 else "-",
                attempt.get("wait_ms"), attempt.get("locate_ms"), attempt.get("effect_ms"),
                inp.get("sent"), inp.get("delivered"), inp.get("point"),
                (" discarded=%s" % inp.get("discarded")) if inp.get("discarded") else "",
                ("%s %s" % (loc.get("source"), loc.get("point"))) if "error" not in loc else ("失败 %s" % loc.get("error"))))
    lines.append("结束: %s — %s  步骤=%s" % ((trace["terminal"] or {}).get("kind"), (trace["terminal"] or {}).get("reason"), (trace["terminal"] or {}).get("step")))
    for item in trace.get("disturbances") or []:
        lines.append("扰动: %s" % json.dumps(item, ensure_ascii=False))
    b = trace.get("boundary") or {}
    lines.append("最后确认: 步骤=%s 状态=%s 已发未确认=%s 复核拒绝=%s 丢弃尝试=%s" % (
        _short(b.get("last_confirmed_step")), _short(b.get("last_confirmed_state")), _short(b.get("unconfirmed_input")),
        _short(b.get("refused_input")), b.get("discarded_attempts")))
    unknown = trace.get("unknown") or []
    lines.append("证据未知(%d): %s" % (len(unknown), ", ".join(unknown[:10]) + (" ..." if len(unknown) > 10 else "")))
    lines.append("实际结果: %s  耗时: total=%s wait=%s locate=%s effect=%s" % (
        trace.get("outcome"), *(trace["timings"].get(k) for k in ("total_ms", "wait_ms", "locate_ms", "effect_ms"))))
    return "\n".join(lines)


def _short(value):
    text = json.dumps(value, ensure_ascii=False)
    return text if len(text) <= 160 else text[:157] + "..."


# -- 执行 ----------------------------------------------------------------------------

def make_synth_target(spec):
    from synth_target import SynthTarget

    kwargs = {}
    for key in ("loading_seconds", "result_seconds", "popup_trigger_state", "popup_close_after_ms"):
        if key in spec:
            kwargs[key] = spec[key]
    return SynthTarget(**kwargs)


def _write_json_atomic(path, data, *, replace=False):
    # 对外的观察和报告只新增；只有私有工作目录里的控制文件需要覆盖更新。
    fd, tmp = tempfile.mkstemp(dir=os.path.dirname(os.path.abspath(path)))
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(data, handle, ensure_ascii=False, indent=1)
        if replace:
            os.replace(tmp, path)
        elif os.name == "nt":
            os.rename(tmp, path)  # Windows rename 不替换已存在的文件
        else:
            os.link(tmp, path)
    except FileExistsError as exc:
        raise UsageError("拒绝写入：已存在 %s（不覆盖已有产物）" % path) from exc
    finally:
        try:
            os.unlink(tmp)
        except FileNotFoundError:
            pass  # rename/replace 已经移走暂存文件


def _refuse_existing(path, inputs=()):
    """产物不覆盖已有文件（保护已留存的观察/报告），也不能指回任何输入文件。"""
    absolute = os.path.abspath(path)
    if any(os.path.abspath(i) == absolute for i in inputs):
        raise UsageError("拒绝写入：%s 是本次的输入文件" % path)
    if os.path.lexists(absolute):
        raise UsageError("拒绝写入：已存在 %s（不覆盖已有产物，请换一个输出路径）" % path)
    parent = os.path.dirname(absolute)
    if parent and not os.path.isdir(parent):
        raise UsageError("输出目录不存在：%s" % parent)
    return absolute


SAFE_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,47}$")


def _safe_id(run_id):
    """plan 的 run id 只允许短的唯一安全标识：不能带路径分隔/盘符/绝对路径，也不能是 . 或 .."""
    if not isinstance(run_id, str) or not SAFE_ID.fullmatch(run_id) or \
            re.fullmatch(r"CON|PRN|AUX|NUL|(?:COM|LPT)[1-9]", run_id.split(".", 1)[0], re.IGNORECASE):
        raise UsageError("plan.runs 的 id 必须是短的安全标识（字母数字开头，仅含 . _ -，不超过 48 字符）：%r" % (run_id,))
    return run_id


def _matrix_pairs(plan, ids):
    """compares 每项是 [ref, cand] 或 {"ref","cand","expect_fault"}（同一 run 对不同参照可以有不同期望）。"""
    items = list(plan.get("compares") or [])
    if not items and "baseline" in ids:
        items = [["baseline", run_id] for run_id in ids if run_id != "baseline"]
    pairs = []
    for item in items:
        spec = dict(item) if isinstance(item, dict) else {}
        pair = (spec.get("ref"), spec.get("cand")) if spec else tuple(item)
        if not (isinstance(pair, tuple) and len(pair) == 2 and pair[0] in ids and pair[1] in ids):
            raise UsageError("plan.compares 的每一项必须是两个已声明 run id：%r" % (item,))
        pairs.append((pair[0], pair[1], spec))
    return pairs


def _preflight_matrix(plan, out_dir):
    """启动任何目标之前做完的预检：id 安全且唯一、所有产物路径都在 out 目录内、没有同名产物。"""
    runs = plan.get("runs") or []
    ids = []
    for spec in runs:
        run_id = _safe_id(spec.get("id") if isinstance(spec, dict) else None)
        if run_id in ids:
            raise UsageError("plan.runs 的 id 重复：%r" % run_id)
        ids.append(run_id)
    pairs = _matrix_pairs(plan, ids)
    names = ["%s.trace.json" % run_id for run_id in ids]
    names += ["%s~~%s.compare.json" % pair[:2] for pair in pairs]
    names.append("summary.json")
    seen = set()
    for name in names:
        path = os.path.abspath(os.path.join(out_dir, name))
        if os.path.dirname(path) != os.path.abspath(out_dir) or name.casefold() in seen:
            raise UsageError("产物路径越出输出目录或重名：%s" % path)
        seen.add(name.casefold())  # Windows 会把仅大小写不同的名字当成同一份产物
        if os.path.lexists(path):  # 输出目录此时可以还不存在；已存在的同名产物一律不覆盖
            raise UsageError("拒绝写入：已存在 %s（不覆盖已有产物，请换一个输出目录）" % path)
    return ids, pairs


def execute(scenario, scenario_sha, variant, demo_factory=None, deadline_seconds=30.0, poll_interval=0.02,
            keep_workdir=False):
    candidate = dict(DEFAULT_CANDIDATE)
    candidate.update(scenario.get("candidate") or {})
    candidate.update(variant.get("candidate") or {})
    steps = scenario["steps"]
    core = ScenarioCore(steps, scenario.get("config"))
    recorder = Recorder(steps, candidate["effect_budget_ms"])
    work = tempfile.mkdtemp(prefix="windows-flow-workbench-")
    control_path = os.path.join(work, "control.json")
    ledger_path = os.path.join(work, "ledger.json")
    timeline_path = os.path.join(work, "timeline.jsonl")
    _write_json_atomic(control_path, {"cmd": "none"})
    _write_json_atomic(ledger_path, {"done": [], "pending": []})

    target_info = {"kind": scenario["target"]["kind"]}
    demo = None
    if scenario["target"]["kind"] == "synth":
        target = make_synth_target(scenario["target"])
        callback = BadBboxCallback(target, candidate["locate_dwell_ms"] / 1000.0, candidate["bad_bbox"])
        inner = SynthInner(target, localize.DistilledLocator(callback=callback, matcher=localize.CvMatcher()),
                           scenario.get("regions") or {}, candidate["click_offset"], candidate["effect_budget_ms"])
        target_info.update(loading_seconds=scenario["target"].get("loading_seconds"), closed_by="in-process")

    def write(cmd):
        _write_json_atomic(control_path, {"cmd": cmd}, replace=True)

    def closer():
        if demo is not None:
            demo.close()
            target_info.update(demo.describe())

    if scenario["target"]["kind"] == "demo_target":
        if demo_factory is None:
            raise UsageError("demo_target 场景需要 --exe 与 --exe-sha256")
        demo = demo_factory().start()  # SHA 核对失败在这里抛 ExeNotVerified：尚未创建任何进程
        inner = demo.make_device(candidate, scenario.get("controls") or {}, scenario.get("regions") or {})
        target_info.update(demo.describe())
    disturbance = variant.get("disturbance") or None
    disturber = Disturber(disturbance, write, inner.move) if disturbance else None
    ledger = workflow.Ledger(ledger_path)
    device = TracingDevice(inner, core, recorder, disturber)
    if disturber is not None:
        disturber.start()
    watchdog = Watchdog(write, deadline_seconds)
    watchdog.start()
    runner = workflow.Runner(core, device, ledger, workflow.ControlFile(control_path), timeline_path, poll_interval)
    start_record = {"t": _wall(), "state": _safe(inner.snapshot), "binding": _safe(inner.binding), "picture": _safe(inner.picture)}

    def current_label():
        action = core.current()
        return action.step["label"] if action else None

    terminal = None
    try:
        result = runner.run()
        if watchdog.timed_out:
            terminal = {"kind": "stopped", "reason": "deadline reached, control stop written", "step": current_label()}
        elif result.get("stop_requested_at") is not None:
            terminal = {"kind": "stopped", "reason": "control stop/estop", "step": current_label()}
        elif core.index >= len(core.actions):
            terminal = {"kind": "flow-complete", "reason": "all steps done", "step": None}
        else:
            terminal = {"kind": "paused", "reason": "run loop ended before completion", "step": current_label()}
    except workflow.InputUnconfirmed as exc:
        terminal = {"kind": "input-unconfirmed", "reason": str(exc), "step": current_label()}
    except localize.LocateFailed as exc:
        terminal = {"kind": "locate-failed", "reason": "%s: %s" % (exc.reason, exc.detail), "step": current_label()}
    except workflow.StopRequested:
        terminal = {"kind": "stopped", "reason": "StopRequested escaped runner", "step": current_label()}
    except Exception as exc:
        terminal = {"kind": "error", "reason": "%s: %s" % (type(exc).__name__, exc), "step": current_label()}
    finally:
        watchdog.stop.set()
        write("stop")  # 主循环已结束，无副作用；解除可能仍在等待的扰动线程
        if disturber is not None:
            disturber.stop.set()
            disturber.join(2.0)
        settle_deadline = time.monotonic() + FINAL_SETTLE_SECONDS
        while time.monotonic() < settle_deadline:
            time.sleep(0.05)
            _safe(inner.picture)  # 给迟到的效果留固定时间（合成目标靠截图推进时间），再取终态
        post_final = {"t": _wall(), "state": _safe(inner.snapshot), "picture": _safe(inner.picture),
                      "settle_ms": FINAL_SETTLE_SECONDS * 1000.0}
        device.release()
        try:
            closer()
        except Exception as exc:  # 收尾失败记进目标信息，不丢观察
            target_info["close_error"] = "%s: %s" % (type(exc).__name__, exc)
    events = []
    try:
        with open(timeline_path, "r", encoding="utf-8") as handle:
            events = [json.loads(line) for line in handle.read().splitlines() if line.strip()]
    except (OSError, ValueError):
        events = []
    if terminal["kind"] == "paused":
        pauses = [e for e in events if e.get("event") == "pause"]
        if pauses:
            terminal["reason"] = "paused: %s; %s" % (pauses[-1].get("reason"), terminal["reason"])
    disturbance = disturber.applied if disturber is not None else None
    if disturbance is None and disturber is not None:
        disturbance = {"step": variant["disturbance"].get("step"), "kind": variant["disturbance"].get("kind"),
                       "applied": False, "note": "扰动线程未报告结果"}
    with open(ledger_path, "r", encoding="utf-8") as handle:
        ledger_data = json.load(handle)
    trace = finalize(scenario, variant, recorder, events, terminal, post_final, start_record, target_info,
                     ledger_data, disturbance)
    trace["scenario"]["sha256"] = scenario_sha
    if scenario["target"]["kind"] == "synth":
        target_info.update(closed_by="in-process")
    if not keep_workdir:
        shutil.rmtree(work, ignore_errors=True)
    else:
        trace["workdir"] = work
    return trace


# -- 命令 ------------------------------------------------------------------------------

def _declared_match(trace, variant):
    expected = variant.get("expect_result")
    return None if expected is None else trace["outcome"] == expected


def cmd_run(args, demo_factory=None):
    scenario, scenario_sha = load_scenario(args.scenario)
    variant = load_variant(args.variant, os.path.dirname(os.path.abspath(args.scenario)))
    if scenario["target"]["kind"] != "synth" and demo_factory is None:
        raise UsageError("run 只跑合成场景；真实窗口场景请用 observe")
    inputs = [args.scenario] + ([args.variant] if args.variant and os.path.isfile(args.variant) else [])
    _refuse_existing(args.out, inputs)
    trace = execute(scenario, scenario_sha, variant, demo_factory=demo_factory, deadline_seconds=args.deadline_seconds,
                    keep_workdir=args.keep_workdir)
    trace["declared_result_match"] = _declared_match(trace, variant)
    _write_json_atomic(args.out, trace)
    print(render_trace(trace))
    print("观察文件:", os.path.abspath(args.out))
    return _print_run_exit(trace, variant)


def _print_run_exit(trace, variant):
    """run/observe 退出码只表示工作流是否达成：0=全部步骤有外部达成证据；1=未达成/未知/停止。
    expect_result 只是测试声明，单独打印，不混进退出码。"""
    expected = variant.get("expect_result")
    if expected is not None:
        print("符合声明(%s): %s   （退出码看的是工作流是否达成，不是是否符合声明）" % (
            expected, "是" if trace["outcome"] == expected else "否"))
    print("工作流结果: %s → 退出码 %d" % (trace["outcome"], 0 if trace["outcome"] == "confirmed" else 1))
    return 0 if trace["outcome"] == "confirmed" else 1


def _demo_factory(args, scenario):
    import win_target
    placement = getattr(args, "placement", "offscreen")
    exe_sha256 = getattr(args, "exe_sha256", None)
    exe = getattr(args, "exe", None)

    class _Factory(object):
        def __call__(self):
            if not exe:
                raise InfrastructureError("拒绝启动：需要 --exe 与 --exe-sha256（位置或文件名不能证明是哪份程序）")
            return win_target.DemoTarget(exe, exe_sha256, pages=scenario["target"]["pages"], placement=placement)
    return _Factory()


def _as_infrastructure(exc):
    """目标启动/核对失败（含 SHA-256 不一致）→ 退出码 2；其余异常原样抛出。"""
    import win_target
    if isinstance(exc, win_target.TargetError):
        return InfrastructureError(str(exc))
    return exc


def win_target_errors():
    import win_target
    return (win_target.TargetError, OSError)


def cmd_observe(args):
    scenario, scenario_sha = load_scenario(args.scenario)
    variant = load_variant(args.variant, os.path.dirname(os.path.abspath(args.scenario)))
    if scenario["target"]["kind"] != "demo_target":
        raise UsageError("observe 只跑 demo_target 场景；合成场景请用 run")
    inputs = [args.scenario] + ([args.variant] if args.variant and os.path.isfile(args.variant) else [])
    _refuse_existing(args.out, inputs)
    factory = _demo_factory(args, scenario)
    try:
        trace = execute(scenario, scenario_sha, variant, demo_factory=factory, deadline_seconds=args.deadline_seconds,
                        keep_workdir=args.keep_workdir)
    except win_target_errors() as exc:
        raise _as_infrastructure(exc)
    trace["declared_result_match"] = _declared_match(trace, variant)
    _write_json_atomic(args.out, trace)
    print(render_trace(trace))
    print("观察文件:", os.path.abspath(args.out))
    return _print_run_exit(trace, variant)


def cmd_compare(args):
    ref, _ = load_json_file(args.a)
    cand, _ = load_json_file(args.b)
    for trace, path in ((ref, args.a), (cand, args.b)):
        if not isinstance(trace, dict) or trace.get("schema") != SCHEMA_TRACE or not isinstance(trace.get("steps"), list):
            raise UsageError("%s 不是 %s 观察文件" % (path, SCHEMA_TRACE))
    if args.out:
        _refuse_existing(args.out, [args.a, args.b])
    report = refdiff.compare_traces(ref, cand)
    if args.out:
        _write_json_atomic(args.out, report)
    print(refdiff.render_report(report))
    if not report["comparable"]:
        return 2
    return 0 if report["verdict"] == "same" else 1


def cmd_matrix(args):
    plan, _ = load_json_file(args.plan)
    if not isinstance(plan, dict) or plan.get("schema") != SCHEMA_PLAN or not isinstance(plan.get("runs"), list) or not plan["runs"]:
        raise UsageError("%s：schema 必须是 %s 且 runs 非空" % (args.plan, SCHEMA_PLAN))
    plan_dir = os.path.dirname(os.path.abspath(args.plan))
    scenario_path = plan.get("scenario")
    if not scenario_path:
        raise UsageError("plan 缺 scenario（相对 plan 文件）")
    if not os.path.isabs(scenario_path):
        scenario_path = os.path.join(plan_dir, scenario_path)
    scenario, scenario_sha = load_scenario(scenario_path)
    out_dir = os.path.abspath(args.out)
    if os.path.exists(out_dir) and not os.path.isdir(out_dir):
        raise UsageError("输出路径不是目录：%s" % out_dir)
    # 预检全部做完才允许启动任何目标：id 安全唯一、产物不出目录、不覆盖已有文件
    ids, pairs = _preflight_matrix(plan, out_dir)
    variants = {run_id: variant_from_run(spec) for run_id, spec in zip(ids, plan["runs"])}
    os.makedirs(out_dir, exist_ok=True)
    factory = _demo_factory(args, scenario) if scenario["target"]["kind"] == "demo_target" else None
    rows, traces, failures = [], {}, []
    for run_id in ids:
        variant = variants[run_id]
        try:
            trace = execute(scenario, scenario_sha, variant, demo_factory=factory, deadline_seconds=args.deadline_seconds)
        except win_target_errors() as exc:
            converted = _as_infrastructure(exc)
            if isinstance(converted, InfrastructureError):
                failures.append({"id": run_id, "error": str(converted)})
                continue
            raise
        trace["declared_result_match"] = _declared_match(trace, variant)
        path = os.path.join(out_dir, "%s.trace.json" % run_id)
        _write_json_atomic(path, trace)
        traces[run_id] = trace
        rows.append({"id": run_id, "outcome": trace["outcome"], "expected": variant.get("expect_result"),
                     "result_match": _declared_match(trace, variant),
                     "terminal": trace["terminal"]["kind"], "unknown_count": len(trace.get("unknown") or []),
                     "total_ms": trace["timings"]["total_ms"], "trace": path})
    compares = []
    for pair in pairs:
        if pair[0] not in traces or pair[1] not in traces:
            continue  # 启动失败的 run 没有观察可比
        report = refdiff.compare_traces(traces[pair[0]], traces[pair[1]])
        path = os.path.join(out_dir, "%s~~%s.compare.json" % pair[:2])
        _write_json_atomic(path, report)
        expected_fault = pair[2].get("expect_fault") or variants[pair[1]].get("expect_fault")
        fault = (report.get("classification") or {}).get("fault")
        fault_match = None
        if expected_fault == "same":
            fault_match = report["verdict"] == "same"
        elif expected_fault is not None:
            fault_match = fault == expected_fault
        compares.append({"ref": pair[0], "cand": pair[1], "verdict": report["verdict"], "comparable": report["comparable"],
                         "first": report.get("first"), "fault": fault, "expected_fault": expected_fault,
                         "fault_match": fault_match, "report": path})
    confirmed = [r["id"] for r in rows if r["outcome"] == "confirmed"]
    summary = {"schema": "windows-flow-matrix/1", "scenario": scenario["name"], "runs": rows, "compares": compares,
               "failures": failures,
               "all_as_declared": not failures and all(r["result_match"] in (None, True) for r in rows)
               and all(c["fault_match"] in (None, True) for c in compares),
               "workflow_confirmed": confirmed,
               "workflow_not_confirmed": [r["id"] for r in rows if r["outcome"] != "confirmed"]}
    _write_json_atomic(os.path.join(out_dir, "summary.json"), summary)
    print("矩阵: 场景=%s  输出=%s" % (scenario["name"], out_dir))
    print("%-16s %-14s %-14s %-8s %-18s %s" % ("run", "实际结果", "声明", "符合声明", "终止", "耗时ms"))
    for row in rows:
        print("%-16s %-14s %-14s %-8s %-18s %s" % (
            row["id"], row["outcome"], row["expected"],
            "-" if row["result_match"] is None else ("是" if row["result_match"] else "否"), row["terminal"], row["total_ms"]))
    print("%-16s %-16s %-10s %-26s %-24s %s" % ("比较(A)", "B", "结论", "首差", "判断", "声明判断"))
    for c in compares:
        first = c["first"] or {}
        print("%-16s %-16s %-10s %-26s %-24s %s%s" % (c["ref"], c["cand"], c["verdict"],
                                                     "%s/%s/%s" % (first.get("step"), first.get("phase"), first.get("key")),
                                                     c["fault"], c["expected_fault"], "" if c["fault_match"] in (None, True) else "  <-- 不符"))
    for failure in failures:
        print("启动失败: %s: %s" % (failure["id"], failure["error"]))
    print("工作流达成(外部证据齐全): %s；未达成: %s" % (", ".join(confirmed) or "无", ", ".join(summary["workflow_not_confirmed"]) or "无"))
    print("全部符合声明: %s（退出码 0 只表示每项实际结果都符合 plan 的声明，不表示工作流成功）" % summary["all_as_declared"])
    if failures:
        return 2
    return 0 if summary["all_as_declared"] else 1


def build_parser():
    parser = argparse.ArgumentParser(
        prog="workbench.py",
        description="参照-候选工作台：同一场景的两份观察按外部证据比首差，缺证据记未知。",
        epilog="退出码：run/observe 0=工作流达成（每步有外部证据）；compare 0=外部行为相同且两份都确有目标达成、"
               "声明的保留项有证据且无冲突（两份相同的坏轨迹是 not-achieved，缺终态/关键证据是 unknown，都不是 0）；"
               "matrix 0=每项实际结果都符合 plan 声明（不是工作流成功）；1=未达成/有差异/证据缺失/保留项冲突/不符合声明；"
               "2=用法或基础设施错误（含 SHA-256 不一致拒绝启动、不安全 id、产物已存在）。")
    sub = parser.add_subparsers(dest="command")
    for name, help_text in (("run", "合成目标场景"), ("observe", "自建 dm_demo_target 窗口场景")):
        p = sub.add_parser(name, help=help_text)
        p.add_argument("--scenario", required=True, help="场景 JSON（windows-flow-scenario/1）")
        p.add_argument("--variant", help="变体：JSON 文件路径或内联 JSON（候选参数/扰动/声明/期望）")
        p.add_argument("--out", required=True, help="观察文件输出路径")
        p.add_argument("--deadline-seconds", type=float, default=30.0)
        p.add_argument("--keep-workdir", action="store_true", help="保留控制文件/ledger/时间线临时目录")
        if name == "observe":
            p.add_argument("--exe", help="dm_demo_target.exe 路径；启动前按 --exe-sha256 核对文件字节")
            p.add_argument("--exe-sha256", help="调用者独立给出的 64 位十六进制 SHA-256；缺失或不一致就不启动")
            p.add_argument("--placement", choices=("offscreen", "hidden"), default="offscreen",
                           help="offscreen=显示到所有显示器之外（可取像素）；hidden=不显示（画面证据为未知）")
    p = sub.add_parser("compare", help="比较两份观察")
    p.add_argument("a", help="参照观察文件")
    p.add_argument("b", help="候选观察文件")
    p.add_argument("--out", help="比对报告 JSON 输出路径")
    p = sub.add_parser("matrix", help="按计划一次跑完并逐项核对")
    p.add_argument("--plan", required=True, help="计划 JSON（windows-flow-plan/1）")
    p.add_argument("--out", required=True, help="输出目录（每项一个 trace，每个比较一个 report，另有 summary.json）")
    p.add_argument("--exe")
    p.add_argument("--exe-sha256")
    p.add_argument("--placement", choices=("offscreen", "hidden"), default="offscreen")
    p.add_argument("--deadline-seconds", type=float, default=30.0)
    return parser


def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)
    if not args.command:
        parser.print_help()
        return 2
    try:
        if args.command == "run":
            return cmd_run(args)
        if args.command == "observe":
            return cmd_observe(args)
        if args.command == "compare":
            return cmd_compare(args)
        if args.command == "matrix":
            return cmd_matrix(args)
    except UsageError as exc:
        print("用法错误: %s" % exc)
        return 2
    except InfrastructureError as exc:
        print("基础设施错误: %s" % exc)
        return 2
    except ImportError as exc:
        print("缺少依赖（numpy/opencv）: %s" % exc)
        return 2
    return 2


if __name__ == "__main__":
    if not sys.stdout.isatty():
        sys.stdout.reconfigure(encoding="utf-8")
    sys.exit(main())
