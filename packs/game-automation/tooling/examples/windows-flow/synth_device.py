# -*- coding: utf-8 -*-
"""SynthDevice：把纯合成目标（synth_target）与定位蒸馏（localize）接到原 Runner。

复用 workflow.py 的 DecisionCore/Runner/Ledger/ControlFile，不复制第二套执行框架。
- observe：推进合成目标时间并自报状态（合成简化：状态机输入来源，非识别能力）。
- prepare：截图后立即冻结该 frame 对应的 state/revision/key 再定位；
  返回的 Prepared 携带冻结凭证（不把定位期间产生的新 revision 贴到旧 frame 上）。
- act：发输入前先推进目标时间（_tick/capture）再与冻结凭证比较——定位过期即
  InputUnconfirmed，不点击、保留 pending、不重试（不靠点击落点事后判定零误点）；
  点击后未命中预期控件同样未确认。合成目标是单线程的，revision 复核是精确的；
  真实 UI 中"复核通过"与"点击落地"之间仍非原子事务，最终以目标端确认为准。
- 定位 key = (实例 token, action, 目标名, 状态)：目标实例/动作/状态变更都不
  跨用缓存；帧尺寸变化由 DistilledLocator 自行失效。

ScriptedLocateCallback 是授权定位回调的脚本化替身（延时等待 + 返回目标真值 bbox，
支持注入坏 bbox 做负例）：模拟"首次授权（慢）定位"，不调用任何模型；
真实场景替换为 headless 视觉模型调用（见 ../../integrations/airtest.md 定位分层）。
"""

import collections
import os
import sys
import threading
import time

_HERE = os.path.dirname(os.path.abspath(__file__))
if _HERE not in sys.path:
    sys.path.insert(0, _HERE)

import workflow  # noqa: E402  原样复用：State/Action/DecisionCore/Runner/Ledger/ControlFile
import localize  # noqa: E402  定位蒸馏：DistilledLocator/LocateFailed

STATE_MAP = {
    "idle": workflow.State.IDLE,
    "loading": workflow.State.LOADING,
    "ready": workflow.State.READY,
    "collected": workflow.State.COLLECTED,
    "result": workflow.State.RESULT,
    "dialog": workflow.State.DIALOG,
}
ACTION_TARGETS = {
    workflow.Action.START: "start_button",
    workflow.Action.COLLECT_REWARD: "reward_button",
}
ACTION_HITS = {"start_button": "start", "reward_button": "reward"}

# 定位凭证：除坐标外携带定位所依据的目标状态与 revision；act 发输入前复核，
# 过期即 InputUnconfirmed（不点击、保留 pending）——不靠点击落点事后判定零误点。
Prepared = collections.namedtuple(
    "Prepared", ["point", "bbox", "source", "state", "revision"])


class ScriptedLocateCallback(object):
    """授权定位回调替身：dwell 秒可中断等待后返回目标真值 bbox（或注入坏 bbox）。"""

    def __init__(self, target, dwell_seconds=0.0, clock=None):
        self.target = target
        self.dwell = dwell_seconds
        self.clock = clock or time.monotonic
        self.calls = collections.Counter()
        self.entered = threading.Event()
        self._overrides = {}

    def override(self, target_key, bbox):
        self._overrides[target_key] = bbox

    def clear_override(self, target_key):
        self._overrides.pop(target_key, None)

    def __call__(self, key, screen, checkpoint):
        self.entered.set()
        self.calls[key] += 1
        deadline = self.clock() + self.dwell
        while self.clock() < deadline:
            checkpoint()
            time.sleep(0.005)
        checkpoint()
        if key in self._overrides:
            return self._overrides[key]
        return self.target.target_bbox(key[2])  # key = (实例 token, action, 目标名, 状态)


class SynthDevice(object):
    def __init__(self, target, locator, instance_token=None):
        self.target = target
        self.locator = locator
        # 实例 token：同名按钮的不同目标实例不共享缓存（token 默认进程内唯一）
        self.instance_token = instance_token or ("synthtarget-%d" % id(target))
        self.observations = 0
        self.locations = []  # (key, source, point)：定位来源时间线（断言用）
        self.acts = []       # (action, point, hit)
        self.released = False

    def key_for(self, action):
        return (self.instance_token, action.value,
                ACTION_TARGETS[action], self.target.state)

    def observe(self, checkpoint):
        checkpoint()
        self.target.capture()  # 推进合成目标内部时间（状态迁移/弹窗到时关闭）
        self.observations += 1
        return workflow.Observation(STATE_MAP[self.target.state], 1.0, "synth")

    def prepare(self, action, checkpoint):
        checkpoint()
        # 先截图，随后立即冻结该 frame 对应的 state/revision/key（单线程下
        # capture 与读取之间无并发）。定位回调期间目标可能变化——冻结值
        # 属于旧 frame，绝不把定位之后的新 revision 贴到旧位置上；
        # 变化由 act 的复核拒绝。
        screen = self.target.capture()
        state = self.target.state
        revision = self.target.revision
        key = (self.instance_token, action.value, ACTION_TARGETS[action], state)
        located = self.locator.locate(key, screen, checkpoint)
        # 坐标校验不因蒸馏命中豁免：定位点必须落在本次截图范围内
        sw, sh = self.locator.matcher.size(screen)
        x, y = located.point
        if not (0 <= x < sw and 0 <= y < sh):
            raise localize.LocateFailed("point_out_of_screen",
                                        "point=%r screen=%r" % (located.point, (sw, sh)))
        self.locations.append((key, located.source, located.point))
        return Prepared(located.point, located.bbox, located.source,
                        state, revision)

    def act(self, action, prepared):
        # 输入前复核：先推进目标时间（_tick/capture），再与冻结凭证比较——
        # 不能拿未 tick 的内存值比较、把时序变化留到 click 内部才发现。
        # 定位所依据的状态与 revision 未变才发输入；过期即未确认
        # （不点击、保留 pending），不靠点击落点事后判定零误点
        self.target.capture()
        if prepared.state != self.target.state \
                or prepared.revision != self.target.revision:
            raise workflow.InputUnconfirmed(
                "stale prepared location: state %r->%r revision %d->%d"
                % (prepared.state, self.target.state,
                   prepared.revision, self.target.revision))
        x, y = prepared.point
        hit = self.target.click(x, y)
        self.acts.append((action, (x, y), hit))
        if hit != ACTION_HITS[ACTION_TARGETS[action]]:
            # 输入已发出但未命中预期控件：结果未确认，抛 InputUnconfirmed
            # 保留 pending、不重试，交上层暂停等待确认
            raise workflow.InputUnconfirmed(
                "click hit %r, expected %r" % (hit, ACTION_HITS[ACTION_TARGETS[action]]))
        return True

    def release(self):
        self.released = True


def demo():
    """合成通路演示：正常领取 + loading 弹窗自动关闭 + 显式 resume 后接续完成。

    全程无真实窗口、无系统输入、无模型调用；依赖 numpy/opencv（缺失时报错退出）。
    """
    import json
    import tempfile

    try:
        from localize import CvMatcher
        CvMatcher()
    except ImportError as exc:
        raise SystemExit("demo 需要 numpy 与 opencv-python（airtest 既有依赖）：%s" % exc)
    from synth_target import SynthTarget

    tmp = tempfile.mkdtemp(prefix="windows-flow-demo-")
    control_path = os.path.join(tmp, "control.json")
    with open(control_path, "w", encoding="utf-8") as f:
        f.write('{"cmd": "none"}')

    config = {"recognition": {"image_threshold": 0.85},
              "limits": {"step_wait_timeout_seconds": 5, "retry_limit_per_step": 2}}
    target = SynthTarget(popup_close_after_ms=400, popup_trigger_state="loading")
    callback = ScriptedLocateCallback(target, dwell_seconds=0.05)
    locator = localize.DistilledLocator(callback=callback, matcher=CvMatcher())
    device = SynthDevice(target, locator)
    ledger = workflow.Ledger(os.path.join(tmp, "ledger.json"))
    timeline = os.path.join(tmp, "timeline.jsonl")
    runner = workflow.Runner(workflow.DecisionCore(config), device, ledger,
                             workflow.ControlFile(control_path), timeline, 0.02)

    def write(cmd):
        tmpf = control_path + ".tmp"
        with open(tmpf, "w", encoding="utf-8") as f:
            f.write(json.dumps({"cmd": cmd}))
        os.replace(tmpf, control_path)

    finished = threading.Event()

    def user_side():
        # 模拟用户：弹窗自动关闭后仍保持暂停（关闭不自动恢复）→ 显式 resume。
        # 总体 deadline 兜底：到时仍未进入预期状态（或线程异常）就写 stop，
        # 保证主循环可退出；主侧 finally 用 finished 令本线程立即退出并 join。
        deadline = time.monotonic() + 15
        try:
            while not finished.is_set() and time.monotonic() < deadline:
                if runner.paused and not target.popup_active:
                    break
                time.sleep(0.005)
            observed = device.observations
            while (not finished.is_set() and time.monotonic() < deadline
                   and device.observations < observed + 2):
                time.sleep(0.005)
            if finished.is_set():
                return
            if runner.paused:
                write("resume")
            else:
                write("stop")  # 超时未进入预期状态：兜底停止
        except Exception:
            write("stop")  # 线程异常也解除主循环
            raise

    thread = threading.Thread(target=user_side)
    thread.start()
    try:
        result = runner.run()
    finally:
        finished.set()
        write("stop")  # run 异常时解除仍在观察循环的 runner；正常结束后无副作用
        thread.join(2)

    with open(timeline, "r", encoding="utf-8") as f:
        events = [json.loads(line)["event"] for line in f.read().splitlines()]
    print("timeline:", " ".join(events))
    print("locations:", [(k[2], src, pt) for k, src, pt in device.locations])
    print("acts:", [(a.value, pt, hit) for a, pt, hit in device.acts])
    print("blind_clicks:", target.blind_clicks, "callback_calls:", dict(callback.calls))
    assert target.blind_clicks == 0, "不得盲点弹窗"
    assert [hit for _, _, hit in device.acts].count("reward") == 1, "非幂等动作恰好一次"
    assert result["stop_requested_at"] is None, "正常流程由决策停止"
    print("demo ok（时间线/ledger 留存备查，可手动删除）:", tmp)


if __name__ == "__main__":
    demo()
