# -*- coding: utf-8 -*-
"""纯合成无窗口目标：numpy 内存屏幕 + 带纹理按钮 + 可关闭弹窗。

修复历史评测 fixture 的缺陷（弹窗无关闭路径，popup 后永久 DIALOG，任务不可达）：
- popup_close_after_ms：None/0/省略 = 永久不自动关闭；正数 = 到时自动关闭；
  负数/非数值/bool 构造即 ValueError。
- 弹窗关闭只是目标侧事实变化，不触发任何 workflow 恢复——恢复只能来自显式
  resume（Runner 语义，本文件不接触控制文件）。
- close_popup() 提供显式关闭路径（模拟用户/外部关闭），同样不自动恢复。

合成边界（如实声明，不冒充能力）：
- 本文件不打开任何真实窗口、不注入任何系统输入；截图 = 内存数组拷贝。
- state 由目标自报：真实游戏也知道自己状态，这只代表状态机输入来源，
  不代表识别能力；识别/定位链在被测的 adapter 侧。
- click() 的命中判定是目标端测试 oracle：用于断言脚本行为（点没点中预期控件、
  有没有盲点弹窗），不构成真实点击可靠性证明，也不替代点击后状态复核。
"""

import time
from collections import namedtuple

import numpy


Click = namedtuple("Click", ["x", "y", "hit", "t"])

BUTTONS = {  # 名称 -> (宽, 高, 基色, 条纹色, 边框色)
    "start_button": (44, 36, (40, 120, 40), (80, 200, 80), (20, 60, 20)),
    "reward_button": (44, 36, (140, 90, 20), (230, 160, 40), (90, 50, 10)),
}
DEFAULT_POSITIONS = {"start_button": (40, 140), "reward_button": (280, 60)}


class SynthTarget(object):
    """状态：idle →(点中 start)→ loading →(到时)→ ready →(点中 reward)→
    collected →(到时)→ result。弹窗激活期间对外状态为 dialog。"""

    def __init__(self, screen_size=(480, 320), clock=None,
                 loading_seconds=0.15, result_seconds=0.12,
                 popup_close_after_ms=None, popup_trigger_state=None):
        # popup_trigger_state：进入该状态时自动弹一次窗（每 run 一次，reset 重挂），
        # 用于确定性注入"意外弹窗"。
        # popup_close_after_ms 契约：None 或 0 = 永久不自动关闭；正数 = 到时自动
        # 关闭；负数/非数值/bool 一律 ValueError（拒绝静默歧义）。
        if popup_close_after_ms is not None:
            if isinstance(popup_close_after_ms, bool) \
                    or not isinstance(popup_close_after_ms, (int, float)) \
                    or popup_close_after_ms < 0:
                raise ValueError(
                    "popup_close_after_ms 必须是 None/0（不关）或正数（毫秒），got %r"
                    % (popup_close_after_ms,))
            if popup_close_after_ms == 0:
                popup_close_after_ms = None
        self.width, self.height = screen_size
        self.clock = clock or time.monotonic
        self.loading_seconds = loading_seconds
        self.result_seconds = result_seconds
        self.popup_close_after_ms = popup_close_after_ms
        self.popup_trigger_state = popup_trigger_state
        self.positions = dict(DEFAULT_POSITIONS)
        self.clicks = []
        self.blind_clicks = 0  # 点在弹窗上的输入次数（脚本违规信号）
        self.revision = 0      # 可观察变化计数：adapter 在发输入前复核，定位过期即失败
        self.reset()

    # -- 生命周期 -----------------------------------------------------------
    def reset(self):
        """回到 idle 并重挂弹窗触发；保留按钮位置、点击历史与配置。"""
        self._state = "idle"
        self._state_since = self.clock()
        self._popup_active = False
        self._popup_shown_at = None
        self._popup_armed = self.popup_trigger_state is not None
        self._render()

    @property
    def state(self):
        return "dialog" if self._popup_active else self._state

    @property
    def popup_active(self):
        self._tick()
        return self._popup_active

    # -- 观察/截图 ----------------------------------------------------------
    def capture(self):
        """截图 = 当前渲染的拷贝；同时推进内部时间（状态迁移/弹窗到时关闭）。"""
        self._tick()
        return self._screen.copy()

    def target_bbox(self, name):
        """目标侧真值 bbox：授权定位回调（模拟"看图返回 bbox"）的数据源。"""
        x, y = self.positions[name]
        w, h = BUTTONS[name][:2]
        return (x, y, w, h)

    def set_target_pos(self, name, x, y):
        """移动按钮；整按钮必须落在画幅内——出画直接拒绝（不产生残缺渲染）。"""
        w, h = BUTTONS[name][:2]
        x, y = int(x), int(y)
        if x < 0 or y < 0 or x + w > self.width or y + h > self.height:
            raise ValueError("按钮 %s 出画：(%d,%d)+%dx%d 超出 %dx%d"
                             % (name, x, y, w, h, self.width, self.height))
        self.positions[name] = (x, y)
        self._dirty()

    # -- 弹窗 ---------------------------------------------------------------
    def show_popup(self):
        if not self._popup_active:
            self._popup_active = True
            self._popup_shown_at = self.clock()
            self._dirty()

    def close_popup(self):
        """显式关闭（模拟用户/外部动作）；不改变 workflow 暂停状态。"""
        if self._popup_active:
            self._popup_active = False
            self._dirty()

    # -- 输入 ---------------------------------------------------------------
    def click(self, x, y):
        """单次点击输入。返回命中："start"/"reward"/"popup"/"background"。"""
        self._tick()
        hit = "background"
        if self._popup_active:
            px, py, pw, ph = self._popup_rect()
            if px <= x < px + pw and py <= y < py + ph:
                self.blind_clicks += 1
                hit = "popup"
        elif self._state == "idle" and self._inside("start_button", x, y):
            self._set_state("loading")
            hit = "start"
        elif self._state == "ready" and self._inside("reward_button", x, y):
            self._set_state("collected")
            hit = "reward"
        self.clicks.append(Click(int(x), int(y), hit, self.clock()))
        return hit

    # -- 内部 ---------------------------------------------------------------
    def _popup_rect(self):
        return (self.width // 2 - 80, self.height // 2 - 60, 160, 120)

    def _dirty(self):
        """任何可观察变化（状态/弹窗/位移）都推进 revision 并重渲染，
        保证 capture() 的帧内容与 state/popup_active 一致。"""
        self.revision += 1
        self._render()

    def _inside(self, name, x, y):
        bx, by, bw, bh = self.target_bbox(name)
        return bx <= x < bx + bw and by <= y < by + bh

    def _set_state(self, state):
        self._state = state
        self._state_since = self.clock()
        if self._popup_armed and state == self.popup_trigger_state:
            self._popup_armed = False
            self.show_popup()
        self._dirty()

    def _tick(self):
        now = self.clock()
        if self._popup_active and self.popup_close_after_ms is not None:
            if now - self._popup_shown_at >= self.popup_close_after_ms / 1000.0:
                self._popup_active = False  # 到时自动关闭：静默，不触碰恢复语义
                self._dirty()
        if self._state == "loading" and now - self._state_since >= self.loading_seconds:
            self._set_state("ready")
        elif self._state == "collected" and now - self._state_since >= self.result_seconds:
            self._set_state("result")

    def _render(self):
        screen = numpy.full((self.height, self.width, 3), 30, numpy.uint8)
        ys, xs = numpy.mgrid[0:self.height, 0:self.width]
        noise = ((ys * 7 + xs * 13) % 17).astype(numpy.uint8)
        screen += noise[..., None]
        if self._state == "idle":
            self._blit(screen, "start_button")
        elif self._state == "loading":
            screen[150:170, 60:420] = (70, 70, 90)
            screen[150:170, 60:60 + 180] = (200, 200, 60)
        elif self._state == "ready":
            self._blit(screen, "reward_button")
        elif self._state == "collected":
            screen[140:180, 160:320] = (60, 170, 170)
        elif self._state == "result":
            screen[120:200, 140:340] = (50, 80, 200)
            screen[150:170, 160:320] = (240, 240, 240)
        if self._popup_active:
            px, py, pw, ph = self._popup_rect()
            screen[py:py + ph, px:px + pw] = (110, 110, 110)
            screen[py + 3:py + ph - 3, px + 3:px + pw - 3] = (90, 90, 90)
            screen[py + 8:py + 24, px + 8:px + pw - 8] = (30, 30, 30)
        self._screen = screen

    def _blit(self, screen, name):
        w, h, base, stripe, border = BUTTONS[name]
        x, y = self.positions[name]
        patch = numpy.zeros((h, w, 3), numpy.uint8)
        yy, xx = numpy.mgrid[0:h, 0:w]
        patch[((yy + xx) // 5) % 2 == 0] = stripe
        patch[((yy + xx) // 5) % 2 == 1] = base
        patch[:2, :] = border
        patch[-2:, :] = border
        patch[:, :2] = border
        patch[:, -2:] = border
        patch[h // 2 - 3:h // 2 + 3, w // 2 - 3:w // 2 + 3] = (255, 255, 255)
        screen[y:y + h, x:x + w] = patch
