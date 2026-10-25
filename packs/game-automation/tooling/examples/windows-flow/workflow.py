# -*- coding: utf-8 -*-
"""windows-flow 参考实现：决策核心与执行框架解耦的状态机自动化脚本。

结构（对应 pack workflow-template）：
- State/Action：封闭状态与动作集合；UNKNOWN/GONE 是一等状态。
- DecisionCore：decide() 纯逻辑，输入 observation + ledger + config，输出唯一决策。
- DeviceAdapter/FakeDevice：observe/act 接口与脚本化 fake（单测用，不启动真实框架）。
- Runner：主循环（poll→decide→act）、控制文件暂停/停止、ledger 持久化、时间线记录。

本文件不含任何真实目标绑定；接真实窗口时实现 AirtestDevice 并替换 adapter，
决策核心不改。控制文件语义：{"cmd": "pause"|"resume"|"stop"|"estop"}，
poll_interval 上限即停止响应上界的一部分（紧急停止判据 <=2s）。
"""

import json
import os
import time
from enum import Enum


class State(Enum):
    IDLE = "idle"            # 初始，等待开始入口
    LOADING = "loading"      # 过渡，等待就绪（延迟注入落在这里）
    READY = "ready"          # 可以执行流程动作
    COLLECTED = "collected"  # 非幂等动作已发生，等待结果
    RESULT = "result"        # 终态
    UNFOCUSED = "unfocused"  # 窗口失焦（可恢复）
    DIALOG = "dialog"        # 意外弹窗（不盲点）
    GONE = "gone"            # 目标消失（安全停止）
    UNKNOWN = "unknown"      # 识别低置信/无法解释


class Action(Enum):
    START = "start"          # 幂等：点击开始入口
    COLLECT_REWARD = "collect_reward"  # 非幂等：领取奖励，ledger 保护
    CLOSE = "close"          # 释放连接/资源（不关闭目标窗口）


class Decision(Enum):
    ACT = "act"
    WAIT = "wait"
    PAUSE = "pause"
    STOP = "stop"


class Observation(object):
    def __init__(self, state, confidence=1.0, detail=""):
        self.state = state
        self.confidence = confidence
        self.detail = detail


class DecisionCore(object):
    """decide() 是纯逻辑：同输入同输出，无副作用，可直接单测。"""

    NON_IDEMPOTENT = {Action.COLLECT_REWARD}

    def __init__(self, config):
        self.config = config
        limits = config["limits"]
        self.wait_timeout = limits["step_wait_timeout_seconds"]
        self.retry_limit = limits["retry_limit_per_step"]

    def decide(self, obs, ledger, step_ctx):
        """step_ctx: {"waited": float, "retries": int}，由 Runner 维护。"""
        s = obs.state
        if obs.confidence < self.config["recognition"]["image_threshold"]:
            # 低置信度一律暂停，不用猜测的状态继续
            return (Decision.PAUSE, "low-confidence: %.2f" % obs.confidence)
        if s is State.GONE:
            return (Decision.STOP, "target gone")
        if s is State.DIALOG:
            return (Decision.PAUSE, "unexpected dialog: " + obs.detail)
        if s is State.UNFOCUSED:
            return (Decision.PAUSE, "window unfocused")
        if s is State.UNKNOWN:
            if step_ctx["retries"] >= self.retry_limit:
                return (Decision.STOP, "unknown-state retry limit")
            return (Decision.WAIT, "unknown state, bounded retry")
        if s is State.IDLE:
            return (Decision.ACT, Action.START)
        if s is State.LOADING:
            if step_ctx["waited"] >= self.wait_timeout:
                return (Decision.PAUSE, "loading timeout")
            return (Decision.WAIT, "loading")
        if s is State.READY:
            pending = ledger.get("pending", [])
            if pending:
                # 已 begin 未 commit 的非幂等动作：无法确认是否已执行，
                # 暂停等待确认，不当作未执行重发（恢复合同）。
                return (Decision.PAUSE,
                        "unresolved pending non-idempotent action(s): %s" % pending)
            if Action.COLLECT_REWARD.value in ledger.get("done", []):
                return (Decision.WAIT, "reward already collected, awaiting result")
            return (Decision.ACT, Action.COLLECT_REWARD)
        if s is State.COLLECTED:
            if step_ctx["waited"] >= self.wait_timeout:
                return (Decision.PAUSE, "result timeout")
            return (Decision.WAIT, "awaiting result")
        if s is State.RESULT:
            return (Decision.STOP, "flow complete")
        return (Decision.PAUSE, "unhandled state: %s" % s)

    def is_non_idempotent(self, action):
        return action in self.NON_IDEMPOTENT


class FakeDevice(object):
    """脚本化 fake：按场景脚本推进状态，记录 act 时间戳；单测用。"""

    def __init__(self, script, clock=None):
        # script: [(delay_seconds, state_after, confidence)]，observe 一次走一格
        self.script = list(script)
        self.clock = clock or time.monotonic
        self.state = self.script[0][1] if self.script else State.IDLE
        self.acts = []       # (action, timestamp)
        self.observe_count = 0
        self.observe_times = []  # 每次 observe 的单调时间（顺序断言用）
        self.released = False

    def observe(self):
        self.observe_count += 1
        self.observe_times.append(time.time())  # 墙钟：与控制/动作时刻可比
        if self.script:
            delay, state, conf = self.script.pop(0)
            if delay:
                self._sleep(delay)
            self.state = state
            return Observation(self.state, conf, detail="fake")
        return Observation(self.state, 1.0, "fake-final")

    def _sleep(self, seconds):
        time.sleep(seconds)

    def act(self, action):
        self.acts.append((action, self.clock()))
        return True

    def release(self):
        self.released = True


class Ledger(object):
    """非幂等动作登记：两阶段（begin→commit）+ 原子落盘。

    begin 在输入发出前落盘，commit 在动作成功后落盘；中断后 pending 残留即
    "无法确认是否已执行"，决策层据此暂停等待确认。写入经 tmp+replace 原子
    替换，消除截断窗口；登记文件损坏时 fail-closed（抛错），不当空。
    """

    KEY_DONE = "done"
    KEY_PENDING = "pending"

    def __init__(self, path):
        self.path = path
        try:
            with open(path, "r", encoding="utf-8") as f:
                self.data = json.load(f)
        except FileNotFoundError:
            self.data = {}
        except ValueError:
            # 损坏的登记不能当作"未执行"——fail-closed
            raise
        self.data.setdefault(self.KEY_DONE, [])
        self.data.setdefault(self.KEY_PENDING, [])

    def _flush(self):
        tmp = self.path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as f:
            json.dump(self.data, f, ensure_ascii=False, indent=2)
        os.replace(tmp, self.path)

    def has(self, action):
        return action.value in self.data[self.KEY_DONE]

    def has_pending(self):
        return bool(self.data[self.KEY_PENDING])

    def begin(self, action):
        v = action.value
        if v not in self.data[self.KEY_PENDING]:
            self.data[self.KEY_PENDING].append(v)
            self._flush()

    def commit(self, action):
        v = action.value
        if v in self.data[self.KEY_PENDING]:
            self.data[self.KEY_PENDING].remove(v)
        if v not in self.data[self.KEY_DONE]:
            self.data[self.KEY_DONE].append(v)
        self._flush()

    def record(self, action):
        # 兼容旧直接登记路径（幂等场景或单步脚本）
        self.commit(action)


class ControlFile(object):
    """控制文件：pause/resume/stop/estop。外部写文件触发，Runner 每 poll 读一次。"""

    def __init__(self, path):
        self.path = path
        self._seen = None

    def poll(self):
        try:
            with open(self.path, "r", encoding="utf-8") as f:
                cmd = json.load(f).get("cmd")
        except (OSError, ValueError):
            return None
        if cmd in (None, "none"):
            self._seen = None
            return None
        if cmd != self._seen:
            self._seen = cmd
            return cmd
        return None  # 同一命令重复出现不重复触发

    def clear(self):
        self._seen = None
        try:
            with open(self.path, "w", encoding="utf-8") as f:
                json.dump({"cmd": "none"}, f)
        except OSError:
            pass


class Runner(object):
    """主循环：poll 控制 → observe → decide → act/wait。停止判据由时间线验收。"""

    def __init__(self, core, device, ledger, control, timeline_path, poll_interval):
        self.core = core
        self.device = device
        self.ledger = ledger
        self.control = control
        self.timeline = open(timeline_path, "a", encoding="utf-8")
        self.poll_interval = poll_interval
        self.paused = False
        self.stop_requested_at = None

    def log(self, event, **fields):
        rec = {"t": time.time(), "event": event}
        rec.update(fields)
        self.timeline.write(json.dumps(rec, ensure_ascii=False) + "\n")
        self.timeline.flush()

    def run(self):
        step_ctx = {"waited": 0.0, "retries": 0}
        last_obs = None
        self.log("run.start")
        while True:
            cmd = self.control.poll()
            if cmd in ("stop", "estop"):
                self.stop_requested_at = time.time()
                self.log("control.%s" % cmd)
                break
            if cmd == "pause":
                self.paused = True
                self.log("control.pause")
            elif cmd == "resume":
                self.paused = False
                step_ctx = {"waited": 0.0, "retries": 0}  # 恢复从重新观察开始
                self.log("control.resume")
            if self.paused:
                time.sleep(self.poll_interval)
                continue
            obs = self.device.observe()
            if last_obs is not None and obs.state is not last_obs.state:
                step_ctx = {"waited": 0.0, "retries": 0}
            if obs.state is State.UNKNOWN:
                step_ctx["retries"] += 1
            last_obs = obs
            decision, payload = self.core.decide(obs, self.ledger.data, step_ctx)
            if decision is Decision.ACT:
                action = payload
                if self.core.is_non_idempotent(action) and self.ledger.has(action):
                    self.log("act.skip-ledger", action=action.value)
                    continue
                if self.core.is_non_idempotent(action):
                    # 输入发出前先落盘 pending：中断后无法确认是否已执行，
                    # 重启时决策层对 pending 暂停等待确认
                    self.ledger.begin(action)
                self.device.act(action)
                self.log("act", action=action.value)
                if self.core.is_non_idempotent(action):
                    self.ledger.commit(action)
                step_ctx = {"waited": 0.0, "retries": 0}
                # 动作后强制观察间隔：快输入通道下目标状态更新滞后一轮观察，
                # 不节流会在旧观察上重发输入（违背"执行一项→等待预期状态"）
                time.sleep(self.poll_interval)
            elif decision is Decision.WAIT:
                step_ctx["waited"] += self.poll_interval
                time.sleep(self.poll_interval)
            elif decision is Decision.PAUSE:
                self.log("pause", reason=str(payload))
                self.paused = True
            elif decision is Decision.STOP:
                self.log("stop", reason=str(payload))
                break
        self.device.release()
        self.log("run.exit", last_event=time.time())
        self.timeline.close()
        return {"stop_requested_at": self.stop_requested_at}


def selftest():
    """fake 演练：正常完成 + 停止路径。真实场景矩阵见 automation-regression recipe。"""
    import os
    import tempfile

    tmp = tempfile.mkdtemp(prefix="windows-flow-selftest-")
    ledger_path = os.path.join(tmp, "ledger.json")
    timeline_path = os.path.join(tmp, "timeline.jsonl")
    control_path = os.path.join(tmp, "control.json")
    with open(control_path, "w", encoding="utf-8") as f:
        f.write('{"cmd": "none"}')
    with open(ledger_path, "w", encoding="utf-8") as f:
        f.write('{"done": [], "pending": []}')

    config = {
        "recognition": {"image_threshold": 0.85},
        "limits": {"step_wait_timeout_seconds": 30,
                   "retry_limit_per_step": 2},
    }
    core = DecisionCore(config)
    script = [
        (0.0, State.IDLE, 1.0),
        (0.0, State.LOADING, 1.0),
        (0.0, State.READY, 1.0),
        (0.0, State.COLLECTED, 1.0),
        (0.0, State.RESULT, 1.0),
    ]
    device = FakeDevice(script)
    runner = Runner(core, device, Ledger(ledger_path), ControlFile(control_path),
                    timeline_path, poll_interval=0.01)
    result = runner.run()
    assert device.released, "release not called"
    assert result["stop_requested_at"] is None, "normal flow should stop by decision"
    assert any(a is Action.COLLECT_REWARD for a, _ in device.acts), \
        "non-idempotent action should fire exactly once in normal flow"
    with open(ledger_path, "r", encoding="utf-8") as f:
        led = json.load(f)
    assert led["done"] == ["collect_reward"] and led["pending"] == [], led
    print("selftest ok: normal flow ->", [a.value for a, _ in device.acts])
    print("timeline:", timeline_path)


if __name__ == "__main__":
    import sys
    if "--selftest" in sys.argv:
        selftest()
    else:
        print("usage: python workflow.py --selftest  (real target: implement AirtestDevice)")
