# -*- coding: utf-8 -*-
"""定位蒸馏（localization distillation）参考实现：授权定位回调 + 运行内内存模板缓存。

流程（对应 integrations/airtest.md「定位蒸馏」条）：
- 首次定位走授权回调（慢、权威）；回调返回 bbox 经校验（越界/过小/低纹理拒绝）
  后裁剪为模板，只存内存 dict，不落盘——进程重启即失效，符合"重启不带状态"。
- 同 key（目标身份 + 状态）重复定位先本地全图模板匹配（毫秒级，可跟踪窗口/
  目标位移）；低分失配自动回退授权回调并更新模板。
- 歧义（全图存在与最优近分的第二峰值）一律安全失败：不猜坐标、不回退刷结果。

硬合同：
- 目标身份、动作或状态变更使用不同 key，不得跨用缓存；截图帧尺寸变化
  （窗口缩放/分辨率变化）同样失效缓存，回退回调重建；缓存仅内存，无文件 I/O。
- 蒸馏命中只是"目标看起来还在"：坐标校验与点击后状态复核不因此豁免；
  目标端测试校验是测试 oracle，不构成真实点击前可靠性。
- callback 与 checkpoint 抛出的异常（含停止请求）一律原样传播：不吞错、
  不缓存半成品模板。
- 图像操作全部经注入式 matcher（crop/texture/match/size）；唯一真实实现是
  CvMatcher（cv2.matchTemplate，与 aircv find_template 同一 OpenCV 原语），
  策略层可用 stub matcher 零依赖单测。
"""

import math
from collections import namedtuple


Match = namedtuple("Match", ["top_left", "score", "runner_up"])
LocateResult = namedtuple("LocateResult", ["point", "bbox", "score", "source"])
# source: "callback"=首次授权定位; "fallback"=失配后回退; "distilled"=本地模板命中


class LocateFailed(Exception):
    """安全失败：拿不到可信坐标就不返回坐标。reason 说明失败类别。"""

    def __init__(self, reason, detail=""):
        super(LocateFailed, self).__init__("%s: %s" % (reason, detail))
        self.reason = reason
        self.detail = detail


class CvMatcher(object):
    """真实图像 matcher：numpy ndarray 截图，cv2.matchTemplate 全图匹配。

    runner_up 为抑制最优峰邻域后的全图次峰值，用于歧义判定；模板大于截图
    （窗口收缩等）按失配处理，交给回调回退。
    """

    SUPPRESS_MARGIN = 8  # 峰值抑制边距（像素）：与半模板宽一起构成对称邻域

    def __init__(self):
        import cv2  # noqa: F401  延迟导入：缺失时仅图像测试 skip
        import numpy  # noqa: F401
        self.cv2 = cv2
        self.numpy = numpy

    def size(self, screen):
        return (int(screen.shape[1]), int(screen.shape[0]))  # (w, h)

    def template_size(self, template):
        return (int(template.shape[1]), int(template.shape[0]))

    def crop(self, screen, bbox):
        x, y, w, h = bbox
        return screen[y:y + h, x:x + w].copy()

    def texture_score(self, template):
        # 空间纹理：逐通道沿空间轴的 std 取最大。不能用全元素 std——纯色块
        # 的通道间差异（如纯红 BGR）会被误当纹理，令退化模板混入缓存。
        return float(self.numpy.max(self.numpy.std(template, axis=(0, 1))))

    def match(self, template, screen):
        th, tw = template.shape[:2]
        sh, sw = screen.shape[:2]
        if th > sh or tw > sw:
            return Match(None, -1.0, -1.0)  # 无法整幅容纳：按失配回退
        res = self.cv2.matchTemplate(screen, template, self.cv2.TM_CCOEFF_NORMED)
        _, best, _, loc = self.cv2.minMaxLoc(res)
        # 对称近峰邻域抑制：半模板宽 + 边距，绕最优峰中心对称。抑制过宽
        # （整模板宽+边距）会把紧邻的独立候选一并抹掉、漏报歧义。
        rx = tw // 2 + self.SUPPRESS_MARGIN
        ry = th // 2 + self.SUPPRESS_MARGIN
        res2 = res.copy()
        x0 = max(0, loc[0] - rx)
        y0 = max(0, loc[1] - ry)
        x1 = min(res.shape[1], loc[0] + rx + 1)
        y1 = min(res.shape[0], loc[1] + ry + 1)
        res2[y0:y1, x0:x1] = -1.0
        _, second, _, _ = self.cv2.minMaxLoc(res2)
        return Match(loc, float(best), float(second))


class DistilledLocator(object):
    """定位缓存本体。回调合同：callback(target_key, screen, checkpoint) -> bbox
    (x, y, w, h)；回调内部耗时等待必须反复调用 checkpoint 响应停止。"""

    def __init__(self, callback, matcher, threshold=0.85, ambiguity_margin=0.08,
                 min_template_px=8, min_texture_std=12.0):
        self.callback = callback
        self.matcher = matcher
        self.threshold = threshold
        self.ambiguity_margin = ambiguity_margin
        self.min_template_px = min_template_px
        self.min_texture_std = min_texture_std
        # 仅内存：{key: (template, frame_size)}；frame_size 变化即失效重建
        self._templates = {}

    def cached_keys(self):
        return sorted(self._templates.keys())

    def invalidate(self, target_key=None):
        if target_key is None:
            self._templates.clear()
        else:
            self._templates.pop(target_key, None)

    def locate(self, target_key, screen, checkpoint):
        checkpoint()
        entry = self._templates.get(target_key)
        if entry is not None:
            template, frame_size = entry
            if self.matcher.size(screen) == frame_size:
                match = self.matcher.match(template, screen)
                # 本地匹配是单次同步 OpenCV 调用：停止在调用前后 checkpoint 生效，
                # 不承诺 native 调用期间可取消；调用时长受截图尺寸约束（有界）。
                checkpoint()
                if match.score >= self.threshold:
                    if match.runner_up >= match.score - self.ambiguity_margin:
                        # 两个近分候选：模板无法判别，安全失败（失配才回退，歧义不回退）
                        raise LocateFailed("ambiguous",
                                           "score=%.3f runner_up=%.3f key=%r"
                                           % (match.score, match.runner_up, target_key))
                    return self._result_from_match(match, template, "distilled")
            # 低分失配或帧尺寸变化：外观/尺寸/分辨率变化 → 回退授权回调并更新模板
        return self._locate_via_callback(
            target_key, screen, checkpoint,
            source="fallback" if entry is not None else "callback")

    def _locate_via_callback(self, target_key, screen, checkpoint, source):
        checkpoint()
        bbox = self.callback(target_key, screen, checkpoint)  # 异常原样传播
        checkpoint()
        # 严格校验 + 统一 normalize：任何不合格 bbox 在写缓存之前抛出，
        # 不留半成品；返回值与缓存使用同一 normalize 结果。
        x, y, w, h = self._normalize_bbox(bbox)
        self._admit(target_key, screen, (x, y, w, h))
        return LocateResult((x + w // 2, y + h // 2), (x, y, w, h), None, source)

    def _normalize_bbox(self, bbox):
        """bbox 必须是 4 个有限实数（bool/字符串/NaN/inf/长度≠4 一律 malformed）；
        有限小数四舍五入取整（亚像素级，无实质偏移）。"""
        malformed = LocateFailed("callback_bbox_malformed", repr(bbox))
        try:
            values = list(bbox)
        except TypeError:
            raise malformed
        if len(values) != 4:
            raise malformed
        for value in values:
            if isinstance(value, bool) or not isinstance(value, (int, float)):
                raise malformed
            if not math.isfinite(value):
                raise malformed
        x, y, w, h = (int(round(v)) for v in values)
        if x < 0 or y < 0:
            raise LocateFailed("callback_bbox_out_of_bounds", repr(bbox))
        if w < self.min_template_px or h < self.min_template_px:
            raise LocateFailed("callback_bbox_too_small",
                               "w=%d h=%d min=%d" % (w, h, self.min_template_px))
        return (x, y, w, h)

    def _admit(self, target_key, screen, bbox):
        """模板准入（bbox 已 normalize）：合法且空间纹理足够才裁剪入缓存。"""
        x, y, w, h = bbox
        sw, sh = self.matcher.size(screen)
        if x + w > sw or y + h > sh:
            raise LocateFailed("callback_bbox_out_of_bounds", repr(bbox))
        template = self.matcher.crop(screen, (x, y, w, h))
        score = self.matcher.texture_score(template)
        if score < self.min_texture_std:
            raise LocateFailed("low_texture",
                               "std=%.1f min=%.1f key=%r"
                               % (score, self.min_texture_std, target_key))
        self._templates[target_key] = (template, (sw, sh))
        return template

    def _result_from_match(self, match, template, source):
        tx, ty = match.top_left
        tw, th = self.matcher.template_size(template)
        return LocateResult((tx + tw // 2, ty + th // 2),
                            (tx, ty, tw, th), match.score, source)
