# -*- coding: utf-8 -*-
"""参照/候选两份观察的首差比对（refdiff）：纯逻辑、零 I/O；只有 picture_fingerprint 需要 numpy。

输入是 workbench.py 产出的 windows-flow-trace/1 观察文件（参照 = 旧脚本/基线的那次观察，
候选 = 改动后的那次观察）。比较的核心是"外部行为是否达成/保留"：目标自报状态、画面、
输入是否送达、步骤结果、终态。可换的实现细节（点击坐标、定位 bbox/来源/分数、输入种类、
各相时长、窗口位置）单独列为"实现差异"，不影响外部行为结论。

证据顺序（每个尝试）：pre.state → input.sent → input.delivered → input.discarded →
post.state → post.picture → effect → result，再到下一步。
- 任一维度缺证据（None/缺失）即 unknown：既不算相同也不算不同。结论：same（外部行为相同、
  两份都确有目标达成、声明的保留项有证据且无冲突）/ differs（外部行为不同）/ unknown（没有
  差异但关键证据缺失）/ not-achieved（两份一致但参照或候选没有确有目标达成——同一份坏轨迹
  自己比自己不是"参照已达标"）/ keep-violated（声明必须保留的项在其声明的证据上不一致）。
  两份空数组、两个设备自报 confirmed 都不算证据：步骤是否达成按 effect 重算——目标自报
  状态满足 expect 且画面确实变化；缺任一项即 unknown，不能被自报漂绿。
- 尝试次数/步骤个数不同是后验现象：先比公共前缀，次数差异排在公共前缀的真实差异之后。
- 慢步骤：逐步总时长与 wait/locate/effect 三相归因；暂停/位移时给出最后一次确认的边界。
- 可比前提：两份步骤表的 label/target/when/expect 逐项一致，否则 incomparable（标签相同但目标或期待
  不同，行为一致也不是 same）。人类输出在首差处列出两份的原始事实（目标/期待/定位/落点/送达/前后态）
  与定位点-落点关系，再给可选的下一次检查；这些是事实和检查项，不是根因或修复答案。
"""

import math

TOL_PICTURE = 8        # 区域均值每通道容差：同色块重绘抖动远小于真实换页
TOL_SCORE = 0.02
SLOW_MIN_MS = 20.0     # 候选比参照至少慢这么多且超过 SLOW_RATIO 倍才算慢步骤
SLOW_RATIO = 1.5

BINDING_COMPARED = ("rect", "client", "dpi", "placement", "visible", "foreground", "screen")
BINDING_IDENTITY = ("hwnd", "pid", "instance")  # 记录但不比较：每次启动/每个实例都是新值
STEP_DEFINITION = ("label", "target", "when", "expect")  # 步骤表定义：任一项不同就不是同一份步骤表，不可比
EVIDENCED, NOT_EVIDENCED = "evidenced", "not-evidenced"


def compare_binding(a, b):
    """绑定比较：只比双方出现的几何/放置键，不比身份键；返回 'same'|'diff'|'unknown'。"""
    if not isinstance(a, dict) or not isinstance(b, dict):
        return "unknown"
    keys = sorted((set(a) | set(b)) - set(BINDING_IDENTITY))
    if not keys:
        return "unknown"
    return _worst([compare_value(a.get(k), b.get(k)) for k in keys])


def _num(value):
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def compare_value(a, b, tol=0.0, *, definition=False):
    """'same' | 'diff' | 'unknown'。空值不是观察证据；步骤定义则比较字段和允许的空值。"""
    if a is None or b is None:
        return ("same" if a is b else "diff") if definition else "unknown"
    if isinstance(a, bool) or isinstance(b, bool):
        return "same" if a is b else "diff"  # JSON 的开关不是数值 0/1
    if _num(a) and _num(b):
        if not (math.isfinite(a) and math.isfinite(b)):
            return "diff"
        return "same" if abs(a - b) <= tol else "diff"
    if isinstance(a, (list, tuple)) and isinstance(b, (list, tuple)):
        if len(a) != len(b):
            return "diff"
        if definition and not a:
            return "same"
        return _worst([compare_value(x, y, tol, definition=definition) for x, y in zip(a, b)])
    if isinstance(a, dict) and isinstance(b, dict):
        if definition:
            if a.keys() != b.keys():
                return "diff"
            if not a:
                return "same"
        return _worst([compare_value(a.get(k), b.get(k), tol, definition=definition) for k in sorted(set(a) | set(b))])
    return "same" if a == b else "diff"


def _worst(statuses):
    if not statuses:
        return "unknown"  # 没有可比较的内容就不是证据
    if "diff" in statuses:
        return "diff"
    if "unknown" in statuses:
        return "unknown"
    return "same"


def picture_fingerprint(frame, regions):
    """观察侧的画面证据：区域均值 {name: [r,g,b]}（RGB 帧）。帧为 None 或区域越界 → 该项 None
    （unknown）。这是本模块唯一需要 numpy 的函数，延迟导入；其余函数零依赖。"""
    if frame is None or not regions:
        return None
    import numpy
    height, width = frame.shape[:2]
    out = {}
    for name, box in regions.items():
        x, y, w, h = [int(v) for v in box]
        if x < 0 or y < 0 or w <= 0 or h <= 0 or x + w > width or y + h > height:
            out[name] = None
            continue
        out[name] = [int(v) for v in frame[y:y + h, x:x + w].reshape(-1, 3).mean(axis=0).round()]
    return out


def state_matches(state, expect):
    """用观察核对约定的状态字段：True/False；缺证据为 None，不能作为前置条件或完成证明。"""
    if not isinstance(state, dict) or not isinstance(expect, dict) or not expect:
        return None
    status = _worst([compare_value(state.get(k), v) for k, v in expect.items()])
    return None if status == "unknown" else status == "same"


def effect_evidence(attempt, expect):
    """只有本次有效输入确实送达、目标状态满足 expect 且画面变化，才有该步达成的证据。"""
    attempt = attempt if isinstance(attempt, dict) else {}
    inp = attempt.get("input")
    if not isinstance(inp, dict):
        return None
    if inp.get("discarded") or inp.get("sent") is False or inp.get("delivered") is False:
        return NOT_EVIDENCED
    if inp.get("sent") is not True or inp.get("delivered") is not True:
        return None
    pre = attempt.get("pre") or {}
    post = attempt.get("post") or {}
    state_ok = state_matches(post.get("state"), expect)
    change = compare_value(pre.get("picture"), post.get("picture"), TOL_PICTURE)
    if state_ok is None or change == "unknown":
        return None
    return EVIDENCED if (state_ok and change == "diff") else NOT_EVIDENCED


def _steps(trace):
    return trace.get("steps") or []


def _last_attempt(step):
    attempts = step.get("attempts") or []
    return attempts[-1] if attempts and isinstance(attempts[-1], dict) else None


def trace_outcome(trace):
    """整次观察的实际结果（不是期望，也不是设备自报）：confirmed 要求每一步都有外部达成
    证据；任一步证据缺失 → unknown；否则取最后一个未确认步骤的结果码。"""
    terminal = (trace.get("terminal") or {}).get("kind")
    steps = _steps(trace)
    if not steps:
        return "unknown"
    if terminal == "flow-complete" and all(s.get("result") == "confirmed" for s in steps):
        evidence = [effect_evidence(_last_attempt(s), s.get("expect")) for s in steps]
        final_ok = state_matches((trace.get("post_final") or {}).get("state"), steps[-1].get("expect"))
        if NOT_EVIDENCED in evidence or final_ok is False:
            return "unconfirmed"
        if None in evidence or final_ok is None:
            return "unknown"
        return "confirmed"
    for step in reversed(steps):
        result = step.get("result")
        if result in ("confirmed", "not-reached", None):
            continue
        if result == "discarded":
            return "paused" if terminal == "paused" else "stopped"
        return result
    if terminal in ("input-unconfirmed", "locate-failed", "stopped", "paused", "error"):
        return {"input-unconfirmed": "unknown", "locate-failed": "locate-failed",
                "stopped": "stopped", "paused": "paused", "error": "error"}[terminal]
    return "unknown"


def trace_unknowns(trace):
    """一份观察里关键证据的缺失清单（路径列表），供 run/observe 输出与 matrix 记录。"""
    out = []
    if not _steps(trace):
        out.append("steps")
    if not trace.get("terminal") or not (trace.get("terminal") or {}).get("kind"):
        out.append("terminal")
    if not isinstance((trace.get("post_final") or {}).get("state"), dict):
        out.append("post_final.state")
    for index, step in enumerate(_steps(trace)):
        path = "steps[%d](%s)" % (index, step.get("label"))
        attempts = step.get("attempts") or []
        if not attempts:
            out.append(path + ".attempts")
        if not step.get("result"):
            out.append(path + ".result")
        for j, attempt in enumerate(attempts):
            apath = "%s.attempts[%d]" % (path, j)
            inp = attempt.get("input")
            discarded = isinstance(inp, dict) and bool(inp.get("discarded"))
            for phase in ("pre", "post"):
                if discarded and phase == "post":
                    continue  # 被丢弃的尝试没有发输入，本来就没有后置状态
                block = attempt.get(phase) or {}
                if not isinstance(block.get("state"), dict):
                    out.append(apath + "." + phase + ".state")
                if not isinstance(block.get("picture"), dict):
                    out.append(apath + "." + phase + ".picture")
            if not isinstance(inp, dict):
                out.append(apath + ".input")
            elif not discarded and inp.get("sent") is True and inp.get("delivered") is None:
                out.append(apath + ".input.delivered")
        if effect_evidence(_last_attempt(step), step.get("expect")) is None and step.get("result") == "confirmed":
            out.append(path + ".effect")
    return out


def slow_steps(ref, cand, min_ms=SLOW_MIN_MS, ratio=SLOW_RATIO):
    out, unknown = [], []
    for sa, sb in zip(_steps(ref), _steps(cand)):
        da, db = sa.get("duration_ms"), sb.get("duration_ms")
        if not (_num(da) and _num(db)):
            unknown.append({"step": sa.get("label"), "note": "步骤时长缺证据"})
            continue
        if db < da + min_ms or (da > 0 and db < da * ratio):
            continue
        phases = {}
        for name in ("wait", "locate", "effect"):
            pa, pb = _phase_sum(sa, name), _phase_sum(sb, name)
            if _num(pa) and _num(pb):
                phases[name] = (pa, pb)
        if not phases:
            unknown.append({"step": sa.get("label"), "note": "慢步骤无法归因：分相时长缺证据"})
            continue  # 不猜是哪一相慢
        name = max(phases, key=lambda k: phases[k][1] - phases[k][0])
        out.append({"step": sa.get("label"), "ref_ms": round(da, 1), "cand_ms": round(db, 1),
                    "phase": name, "phase_ref_ms": round(phases[name][0], 1),
                    "phase_cand_ms": round(phases[name][1], 1)})
    return out, unknown


def _phase_sum(step, name):
    values = []
    for attempt in step.get("attempts") or []:
        inp = attempt.get("input") if isinstance(attempt.get("input"), dict) else {}
        if name == "effect" and (inp.get("discarded") or inp.get("sent") is not True):
            continue  # 没发输入的尝试没有"输入到效果"时长，不计入也不算缺证据
        values.append(attempt.get({"wait": "wait_ms", "locate": "locate_ms", "effect": "effect_ms"}[name]))
    if not values or any(not _num(v) for v in values):
        return None
    return sum(values)


RULES_TEXT = {
    "start-state-differs": "两份观察的起点（应用状态）不同，后面的差异不能直接归因于改动；先把起点对齐（同一初始页、同一窗口位置/尺寸）再比。",
    "locate-failed": "候选在定位阶段安全失败，没有坐标可用；按失败类别处理：ambiguous=模板有两个近分候选要收窄区域；low_texture/out_of_bounds=布局表或截图尺寸不对；capture-failed=截图通道无像素。",
    "discarded-by-pause": "输入在发出前被丢弃（暂停/恢复/停止），不是目标故障；按边界里的最后一次确认继续，恢复后对同一步骤重新定位再发。",
    "stale-refusal": "定位后窗口/目标变了，输入没有发出（发前复核拒绝）；确认这次位移/状态变化是有意注入的扰动，恢复后重新定位，不要用旧坐标点击。",
    "not-sent": "候选没有发出输入；先看该尝试的 input.reason 与控制事件，区分是暂停丢弃还是复核拒绝。",
    "delivery-unknown": "输入已发出，但目标输入计数没有取到，无法判断消息是否到达；下一步先读目标输入计数/日志并与发送时刻对齐，再决定改定位还是改等待。",
    "not-delivered": "输入已发出但目标输入计数没有变化（消息没到）；下一步检查目标是否处理了这条消息（窗口句柄、消息过滤），这不是等待问题。",
    "effect-late": "消息已到达目标，效果在等待预算用完之后才出现（放弃后再查已到）；下一步只放宽效果等待预算，用同一输入复验，不改点击点。",
    "delivered-no-effect": "消息已到达，但预期状态没有出现；按位置关系选检查：定位或落点与参照不同先查位置（见上一项），位置一致先查目标侧处理，不要先加等待。",
    "wrong-state": "消息已到达，目标却变到了另一个状态；比较参照的控件框、实际点击点和目标自报状态，先分清点错控件还是目标处理不同。",
    "effect-unknown": "目标是否达成仍未知。先从目标当前状态或已有观察补证据；已发且结果未明的动作不重放。",
    "effect-not-evidenced": "目标自报已确认但外部证据不支持（状态满足而画面没变，或反之）；下一步核对画面来源与区域、目标自报与真实呈现是否一致，核对不了就保持未知。",
    "post-state-differs": "输入相同但后置状态不同；对比两份的 post.state 与画面，确认是目标侧行为变化还是起点不同。",
    "picture-differs": "目标自报状态相同但画面不同；核对画面来源与区域（截图通道、DPI、遮挡），核对不了就保持未知。",
    "result-differs": "步骤结果不同但前面的证据一致；查看 terminal 与 boundary，看候选是否在到达该步骤前停止。",
    "attempt-missing": "候选在这一步少/多一次尝试；看该步的结果与边界，判断是提前结束还是多了一次丢弃后的重试。",
    "step-missing": "候选在到达这一步之前就结束了；先看候选的 terminal 与边界（最后一次确认）。",
    "extra-step": "候选多出参照没有的步骤；先确认两份观察用的是同一份场景（步骤表），否则不可比。",
    "terminal-differs": "各步证据一致但终态不同；核对 terminal 与 post_final。",
    "slow-only": "外部结果相同，只有时长不同；按归因的那一相处理：wait=目标前置条件慢，locate=定位慢，effect=输入到效果慢。",
    "unclassified": "差异不属于本工作台已列出的类别；保留原始差异，不要猜根因。",
}


def _attempt_of(step, index):
    attempts = (step or {}).get("attempts") or []
    if not attempts:
        return None
    if index is None or index < 0 or index >= len(attempts):
        return attempts[-1]
    return attempts[index]


def classify(first, ref, cand):
    """根据首差所在步骤/尝试/阶段给出故障类别、依据与下一步。只读，不改观察。"""
    if first is None:
        return None
    index, phase, key = first.get("step_index"), first.get("phase"), first.get("key")
    if phase == "start":
        return _rule("start-state-differs", first)
    if phase == "terminal":
        return _rule("terminal-differs", first, terminal={"ref": ref.get("terminal"), "cand": cand.get("terminal")})
    if phase == "steps":
        return _rule("step-missing" if key == "missing-in-cand" else "extra-step", first,
                     terminal={"ref": ref.get("terminal"), "cand": cand.get("terminal")})
    if index is None or index >= len(_steps(cand)):
        return _rule("step-missing", first, terminal=cand.get("terminal"))
    cstep = _steps(cand)[index]
    c = _attempt_of(cstep, first.get("attempt"))
    evidence = {"step": cstep.get("label"), "phase": phase, "key": key,
                "cand_result": cstep.get("result"), "terminal": (cand.get("terminal") or {}).get("kind")}
    if phase == "attempts":
        return _rule("attempt-missing", first, evidence)
    if c is None:
        return _rule("step-missing", first, evidence, note="候选该步没有尝试记录")
    inp = c.get("input") if isinstance(c, dict) else None
    post = c.get("post") if isinstance(c, dict) else {}
    locate = c.get("locate") if isinstance(c, dict) else None
    if phase == "pre" and key == "state":
        return _rule("start-state-differs", first, evidence)
    if phase == "result" and isinstance(locate, dict) and "error" in locate:
        return _rule("locate-failed", first, dict(evidence, reason=locate.get("error")))
    if phase in ("input", "result", "attempts"):
        if not isinstance(inp, dict):
            return _rule("effect-unknown", first, evidence, note="候选该尝试没有输入记录")
        if inp.get("sent") is not False and inp.get("sent") is not True:
            return _rule("effect-unknown", first, evidence, note="没有发送与否的证据")
        if inp.get("sent") is False:
            if inp.get("discarded"):
                return _rule("discarded-by-pause", first, dict(evidence, discarded=inp.get("discarded")))
            if inp.get("reason_code") in ("stale-binding", "stale-state"):
                return _rule("stale-refusal", first, dict(evidence, reason_code=inp.get("reason_code"), reason=inp.get("reason")))
            return _rule("not-sent", first, dict(evidence, reason=inp.get("reason")))
        if inp.get("delivered") is None:
            return _rule("delivery-unknown", first, evidence)
        if inp.get("delivered") is False:
            return _rule("not-delivered", first, evidence)
    if phase == "effect" or phase in ("input", "result", "post"):
        ev = effect_evidence(c, cstep.get("expect"))
        if ev is None:
            return _rule("effect-unknown", first, dict(evidence, post_state=post.get("state") if isinstance(post, dict) else None, expect=cstep.get("expect")))
        if ev == NOT_EVIDENCED:
            final = cand.get("post_final") or {}
            if not isinstance(inp, dict) or inp.get("delivered") is not True:
                return _rule("not-delivered" if isinstance(inp, dict) and inp.get("delivered") is False else "delivery-unknown", first, evidence)
            expect = cstep.get("expect") or {}
            before = (c.get("pre") or {}).get("state") or {}
            after = (post or {}).get("state") or {}
            if state_matches(after, expect) is True:
                return _rule("effect-not-evidenced", first, dict(evidence, post_state=after, expect=expect))
            if state_matches(final.get("state"), expect) is True:
                return _rule("effect-late", first, dict(evidence, effect_budget_ms=cstep.get("effect_budget_ms"), final_state=final.get("state")))
            # 输入计数本来就会变化，只用场景定义的目标字段判断是否去了错误状态。
            wrong = any(k in before and k in after and before[k] != after[k] for k in expect)
            return _rule("wrong-state" if wrong else "delivered-no-effect", first,
                         dict(evidence, point=inp.get("point"), locate_point=(locate or {}).get("point"), post_state=after, expect=expect))
    if phase == "post" and key == "state":
        return _rule("post-state-differs", first, evidence)
    if phase == "post" and key == "picture":
        return _rule("picture-differs", first, evidence)
    if phase == "result":
        return _rule("result-differs", first, evidence)
    return _rule("unclassified", first, evidence)


def _rule(code, first, evidence=None, **extra):
    payload = dict(evidence or {})
    payload.update(extra)
    payload["first"] = "%s/%s/%s" % (first.get("step"), first.get("phase"), first.get("key"))
    return {"fault": code, "evidence": payload, "next": RULES_TEXT[code]}


# 位置关系对"下一步查什么"有意义的故障类别；其它类别（没发输入、定位失败、步骤缺失等）按类别规则处理
POSITION_FAULTS = ("delivered-no-effect", "wrong-state", "effect-unknown", "effect-not-evidenced", "post-state-differs",
                   "stale-refusal", "not-sent", "discarded-by-pause")  # 后三类明示"没发输入，无落点可比"
POSITION_TEXT = {
    "locate-differs": "定位本身不同（两份定位点不一样）",
    "point-shifted": "定位后输入位置变了（定位点相同，落点不同）",
    "point-same": "位置一致（定位点与落点两份相同）",
    "locate-unknown": "定位证据不全（有一份没有定位点，先看 locate.error）",
    "point-unknown": "落点证据不全（有一份没有落点）",
    "not-sent": "候选没有发出输入，无落点可比",
    "send-unknown": "是否发出输入未知，不能确认实际落点",
}
POSITION_CHECKS = {
    "locate-differs": "两份定位点不同：先查定位为何不同（回调 bbox / 模板匹配分数 / 窗口位置 pre.binding.rect），确认前不要改点击点",
    "point-shifted": "定位点相同、落点不同：对照两份记录里定位之后的偏移或坐标换算，先核对转换，不重发输入",
    "send-unknown": "先从已有记录或目标侧计数核对是否发过；不重放结果未明的输入",
    "point-same": "位置一致：问题不在落点，查目标是否收到并处理了这条消息（句柄归属、消息过滤、目标侧状态/DPI），不要先改落点或加等待",
    "locate-unknown": "有一份没有定位点：先补齐两份的 locate 记录（定位失败看 locate.error）",
    "point-unknown": "有一份没有落点：先补齐两份的 input.point",
    "not-sent": "候选没有发出输入：没有落点可比，按下面的类别查（暂停丢弃 / 复核拒绝 / 定位失败）",
}


def focus_of(first, ref, cand):
    """首差所在步骤/尝试两份的原始事实摘录（只摘录、不判断）：步骤目标/期待/结果、定位、输入、
    前态/后态、期待是否满足、画面是否变化。start/terminal/步骤数等非尝试级首差 → None。"""
    if not first:
        return None
    index = first.get("step_index")
    steps_ref, steps_cand = _steps(ref), _steps(cand)
    if index is None or index >= len(steps_ref) or index >= len(steps_cand):
        return None
    attempt = first.get("attempt")
    return {"step_index": index, "attempt": attempt,
            "ref": _side(steps_ref[index], _attempt_of(steps_ref[index], attempt)),
            "cand": _side(steps_cand[index], _attempt_of(steps_cand[index], attempt))}


def _side(step, attempt):
    attempt = attempt if isinstance(attempt, dict) else {}
    pre = attempt.get("pre") if isinstance(attempt.get("pre"), dict) else {}
    post = attempt.get("post") if isinstance(attempt.get("post"), dict) else {}
    inp = attempt.get("input") if isinstance(attempt.get("input"), dict) else {}
    loc = attempt.get("locate") if isinstance(attempt.get("locate"), dict) else {}
    expect = step.get("expect")
    return {"target": step.get("target"), "expect": expect, "result": step.get("result"),
            "locate": {k: loc.get(k) for k in ("point", "bbox", "source", "error")},
            "input": {k: inp.get(k) for k in ("sent", "delivered", "discarded", "point", "reason_code")},
            "pre_state": pre.get("state"), "post_state": post.get("state"),
            "expect_met": state_matches(post.get("state"), expect),
            "picture_changed": compare_value(pre.get("picture"), post.get("picture"), TOL_PICTURE)}


def position_relation(focus):
    """定位点/落点两份的已证关系（事实，不是根因）：locate-differs | point-shifted | point-same |
    locate-unknown | point-unknown | not-sent。"""
    if not focus:
        return None
    ref, cand = focus["ref"], focus["cand"]
    if cand["input"].get("sent") is False:
        return "not-sent"
    if cand["input"].get("sent") is not True or ref["input"].get("sent") is not True:
        return "send-unknown"
    for field, unknown, different in (("locate", "locate-unknown", "locate-differs"),
                                      ("input", "point-unknown", "point-shifted")):
        points = (ref[field].get("point"), cand[field].get("point"))
        if not all(isinstance(p, (list, tuple)) and len(p) == 2 and
                   all(_num(x) and math.isfinite(x) for x in p) for p in points):
            return unknown
        if compare_value(*points) == "diff":
            return different
    return "point-same"


def next_checks(classification, position):
    """负责人可选的下一次检查（是检查项，不是修复答案）：位置相关故障先给按位置关系的那一项，
    再给类别规则；其它故障只有类别规则。"""
    fault = classification["fault"]
    checks = []
    if position:
        if position == "point-same" and fault in ("effect-unknown", "effect-not-evidenced"):
            checks.append("位置一致但效果未知/尚未可证：从已有画面或目标自报补证，不改落点，不重放输入")
        elif position == "point-same" and fault == "delivered-no-effect":
            checks.append("位置一致、消息已到但无效果：查目标是否把这条消息当成有效输入（句柄归属、消息过滤、目标侧命中判定/DPI），不要先改落点或加等待")
        elif position == "point-same":
            checks.append("位置一致但后态不同：对照两份的前态与目标侧处理（是否同一起点、目标是否换了响应），不要先改落点")
        else:
            checks.append(POSITION_CHECKS[position])
    checks.append(classification["next"])
    return checks


def resolve_path(trace, path):
    """取证据路径的值：timings.wait_ms / terminal.kind / steps[*].locate.source / steps[*].result。"""
    if not isinstance(path, str) or not path:
        return None
    parts = path.split(".")
    node = trace
    for i, part in enumerate(parts):
        if part.endswith("[*]"):
            if part != "[*]":
                if not isinstance(node, dict) or part[:-3] not in node:
                    return None
                node = node[part[:-3]]
            if not isinstance(node, list):
                return None
            rest = ".".join(parts[i + 1:])
            return [resolve_path(item, rest) for item in node]
        if isinstance(node, list):
            if part == "length":
                node = len(node)
                continue
            return None
        if not isinstance(node, dict) or part not in node:
            return None
        node = node[part]
    return node


def _name(trace):
    scenario = (trace.get("scenario") or {}).get("name")
    variant = (trace.get("variant") or {}).get("id")
    return "%s/%s" % (scenario, variant)


def check_behaviors(ref, cand):
    """声明保留/替换的行为与实测是否一致。条目形如
    {"behavior": "...", "evidence": "steps[*].locate.source"}：keep 要求该证据相同，
    replace 要求该证据不同；证据缺失或为空列表标 evidence-unknown，不替声明背书。"""
    behaviors = (cand.get("variant") or {}).get("behaviors") or {}
    out = []
    for kind in ("keep", "replace"):
        for entry in behaviors.get(kind) or []:
            if not isinstance(entry, dict) or "evidence" not in entry:
                out.append({"kind": kind, "behavior": entry, "flag": "evidence-unknown"})
                continue
            path = entry["evidence"]
            va, vb = resolve_path(ref, path), resolve_path(cand, path)
            empty = (isinstance(va, list) and not va) or (isinstance(vb, list) and not vb)
            status = "unknown" if empty else compare_value(va, vb, TOL_PICTURE if path.endswith("_ms") else 0.0)
            if status == "unknown":
                flag = "evidence-unknown"
            elif kind == "keep" and status == "diff":
                flag = "declared-keep-but-differs"
            elif kind == "replace" and status == "same":
                flag = "declared-replace-but-same"
            else:
                flag = "consistent"
            out.append({"kind": kind, "behavior": entry.get("behavior", path), "evidence": path,
                        "ref": va, "cand": vb, "flag": flag})
    return out


def boundary_of(trace):
    """暂停/位移/未确认时的边界：最后一次确认到哪、有没有已发出未确认的输入。"""
    steps = _steps(trace)
    last = None
    for step in steps:
        if step.get("result") == "confirmed" and effect_evidence(_last_attempt(step), step.get("expect")) == EVIDENCED:
            last = step
    last_post = (_last_attempt(last) or {}).get("post") if last else None
    unconfirmed = None
    refused = None
    discarded = 0
    for step in steps:
        for attempt in step.get("attempts") or []:
            inp = attempt.get("input") if isinstance(attempt, dict) else None
            if not isinstance(inp, dict):
                continue
            if inp.get("discarded"):
                discarded += 1
            elif inp.get("sent") is True and effect_evidence(attempt, step.get("expect")) != EVIDENCED:
                unconfirmed = {"step": step.get("label"), "delivered": inp.get("delivered"), "point": inp.get("point")}
            elif inp.get("sent") is False:
                refused = {"step": step.get("label"), "reason_code": inp.get("reason_code")}
    return {"last_confirmed_step": last.get("label") if last else None,
            "last_confirmed_state": (last_post or {}).get("state"),
            "last_confirmed_picture": (last_post or {}).get("picture"),
            "unconfirmed_input": unconfirmed, "refused_input": refused,
            "discarded_attempts": discarded,
            "terminal": trace.get("terminal")}


def _attempt_summary(attempt):
    """差异记录里只放尝试的要点，不放整条证据。"""
    if not isinstance(attempt, dict):
        return attempt
    inp = attempt.get("input") if isinstance(attempt.get("input"), dict) else {}
    post = attempt.get("post") if isinstance(attempt.get("post"), dict) else {}
    return {"sent": inp.get("sent"), "delivered": inp.get("delivered"), "discarded": inp.get("discarded") or False,
            "post_state": post.get("state"), "confirmed": post.get("confirmed")}


def _diff(step_index, step, attempt, phase, key, ref, cand, kind):
    return {"step_index": step_index, "step": step, "attempt": attempt, "phase": phase, "key": key,
            "ref": ref, "cand": cand, "kind": kind}


def _cmp(collect_diff, collect_unknown, step_index, step, attempt, phase, key, va, vb, tol=0.0, kind="external"):
    status = compare_value(va, vb, tol)
    if status == "diff":
        collect_diff.append(_diff(step_index, step, attempt, phase, key, va, vb, kind))
    elif status == "unknown":
        collect_unknown.append({"step_index": step_index, "step": step, "attempt": attempt, "phase": phase, "key": key, "kind": kind})
    return status


def compare_attempt(index, step, j, a, b, expect):
    """比较一次尝试。返回 (external_diffs, external_unknowns, other_diffs)；other 为实现/环境差异。"""
    ext_d, ext_u, other_d, other_u = [], [], [], []
    if a is None or b is None:
        return ([_diff(index, step, j, "attempts", "missing", a is not None, b is not None, "external")],
                [], [], [])
    pre_a, pre_b = a.get("pre") or {}, b.get("pre") or {}
    post_a, post_b = a.get("post") or {}, b.get("post") or {}
    inp_a = a.get("input") if isinstance(a.get("input"), dict) else {}
    inp_b = b.get("input") if isinstance(b.get("input"), dict) else {}
    _cmp(ext_d, ext_u, index, step, j, "pre", "state", pre_a.get("state"), pre_b.get("state"))
    _cmp(ext_d, ext_u, index, step, j, "pre", "picture", pre_a.get("picture"), pre_b.get("picture"), TOL_PICTURE)
    ba, bb = pre_a.get("binding"), pre_b.get("binding")
    status = compare_binding(ba, bb)
    if status == "diff":
        other_d.append(_diff(index, step, j, "pre", "binding", ba, bb, "environment"))
    elif status == "unknown":
        other_u.append({"step_index": index, "step": step, "attempt": j, "phase": "pre", "key": "binding", "kind": "environment"})
    for key in ("sent", "discarded"):
        if key == "discarded":
            # discarded：有输入记录时 None 即"未丢弃"（确定事实），没有记录才是缺证据
            va = (inp_a.get("discarded") or False) if isinstance(a.get("input"), dict) else None
            vb = (inp_b.get("discarded") or False) if isinstance(b.get("input"), dict) else None
        else:
            va, vb = inp_a.get(key), inp_b.get(key)
        _cmp(ext_d, ext_u, index, step, j, "input", key, va, vb)
    discarded_any = bool(inp_a.get("discarded") or inp_b.get("discarded"))
    if not discarded_any:
        # 被丢弃的尝试没有发输入：送达/坐标/后置状态都不适用，input.sent/discarded 的差异已记录在案
        _cmp(ext_d, ext_u, index, step, j, "input", "delivered", inp_a.get("delivered"), inp_b.get("delivered"))
        for key in ("point", "kind", "clicks_delta"):
            _cmp(other_d, other_u, index, step, j, "input", key, inp_a.get(key), inp_b.get(key), kind="implementation")
    loc_a, loc_b = a.get("locate"), b.get("locate")
    if isinstance(loc_a, dict) and "error" in loc_a or isinstance(loc_b, dict) and "error" in loc_b:
        _cmp(other_d, other_u, index, step, j, "locate", "error",
             loc_a.get("error") if isinstance(loc_a, dict) else None,
             loc_b.get("error") if isinstance(loc_b, dict) else None, kind="implementation")
    else:
        for key in ("point", "bbox", "source", "score"):
            va = loc_a.get(key) if isinstance(loc_a, dict) else None
            vb = loc_b.get(key) if isinstance(loc_b, dict) else None
            if key == "score" and va is None and vb is None and "callback" in (loc_a.get("source") if isinstance(loc_a, dict) else None, loc_b.get("source") if isinstance(loc_b, dict) else None):
                continue  # 授权回调定位没有分数：不适用，不是缺证据
            _cmp(other_d, other_u, index, step, j, "locate", key, va, vb, TOL_SCORE if key == "score" else 0.0,
                 kind="implementation")
    if not discarded_any:
        # 被丢弃的尝试没有发输入，也就没有后置状态可比：input.sent/discarded 的差异已记录在案
        _cmp(ext_d, ext_u, index, step, j, "post", "state", post_a.get("state"), post_b.get("state"))
        _cmp(ext_d, ext_u, index, step, j, "post", "picture", post_a.get("picture"), post_b.get("picture"), TOL_PICTURE)
        _cmp(ext_d, ext_u, index, step, j, "effect", "evidence", effect_evidence(a, expect), effect_evidence(b, expect))
    return ext_d, ext_u, other_d, other_u


def compare_step(index, sa, sb):
    """返回 (external_diffs, external_unknowns, other_diffs, other_unknowns)。公共前缀优先，
    尝试次数差异排在公共前缀差异之后。"""
    ext_d, ext_u, other_d, other_u = [], [], [], []
    label = sa.get("label")
    aa, bb = sa.get("attempts") or [], sb.get("attempts") or []
    if not aa and not bb:
        # 两份都没有尝试记录：这一步没有任何行为证据，不能判相同
        ext_u.append({"step_index": index, "step": label, "attempt": None, "phase": "attempts", "key": "none", "kind": "external"})
    common = min(len(aa), len(bb))
    for j in range(common):
        d, u, od, ou = compare_attempt(index, label, j, aa[j], bb[j], sa.get("expect"))
        ext_d.extend(d); ext_u.extend(u); other_d.extend(od); other_u.extend(ou)
    if len(aa) != len(bb):
        ext_d.append(_diff(index, label, None, "attempts", "count", len(aa), len(bb), "external"))
    for j in range(common, max(len(aa), len(bb))):
        side = "ref" if len(aa) > len(bb) else "cand"
        ext_d.append(_diff(index, label, j, "attempts", "extra-in-" + side,
                           _attempt_summary(aa[j]) if j < len(aa) else None,
                           _attempt_summary(bb[j]) if j < len(bb) else None, "external"))
    _cmp(ext_d, ext_u, index, label, None, "result", "result", sa.get("result"), sb.get("result"))
    return ext_d, ext_u, other_d, other_u


def compare_traces(ref, cand):
    """两份观察的完整比对报告（dict）。"""
    report = {"schema": "windows-flow-compare/1", "ref": _name(ref), "cand": _name(cand),
              "comparable": True, "not_compared": {"pre.binding": list(BINDING_IDENTITY)}}
    steps_ref, steps_cand = _steps(ref), _steps(cand)
    common = min(len(steps_ref), len(steps_cand))
    for i in range(common):
        # 标签相同但目标/前置条件/期待不同 = 两份跑的不是同一份步骤表：行为一致也不能判 same
        mismatch = [k for k in STEP_DEFINITION if
                    compare_value(steps_ref[i].get(k), steps_cand[i].get(k), definition=True) != "same"]
        if mismatch:
            shown = "; ".join("%s: %s vs %s" % (k, _fmt(steps_ref[i].get(k)), _fmt(steps_cand[i].get(k))) for k in mismatch)
            report.update(comparable=False, first=None, differences=[], unknown=[], other_differences=[],
                          verdict="incomparable",
                          reason="第 %d 步的步骤定义不同（%s），不能按步比较" % (i, shown))
            report.update(classification=None, focus=None, position=None, slow=[], slow_unknown=[], behaviors=[],
                          boundary={"ref": boundary_of(ref), "cand": boundary_of(cand)},
                          outcome={"ref": trace_outcome(ref), "cand": trace_outcome(cand)},
                          evidence={"ref": step_evidence(ref), "cand": step_evidence(cand)})
            return report
    ext_d, ext_u, other_d, other_u = [], [], [], []
    if not steps_ref and not steps_cand:
        # 两份都没有步骤：没有任何可比较的行为证据，不能判相同
        ext_u.append({"step_index": None, "step": "steps", "attempt": None, "phase": "steps", "key": "empty", "kind": "external"})
    start_ref, start_cand = ref.get("start") or {}, cand.get("start") or {}
    for key in ("state", "picture"):
        _cmp(ext_d, ext_u, None, "start", None, "start", key, start_ref.get(key), start_cand.get(key),
             TOL_PICTURE if key == "picture" else 0.0)
    ba, bb = start_ref.get("binding"), start_cand.get("binding")
    status = compare_binding(ba, bb)
    if status == "diff":
        other_d.append(_diff(None, "start", None, "start", "binding", ba, bb, "environment"))
    elif status == "unknown":
        other_u.append({"step_index": None, "step": "start", "attempt": None, "phase": "start", "key": "binding", "kind": "environment"})
    for i in range(common):
        d, u, od, ou = compare_step(i, steps_ref[i], steps_cand[i])
        ext_d.extend(d); ext_u.extend(u); other_d.extend(od); other_u.extend(ou)
    if len(steps_ref) != len(steps_cand):
        # 步骤数不同：公共前缀的真实差异已排在前，缺失的步骤记在后
        longer, side = (steps_ref, "ref") if len(steps_ref) > len(steps_cand) else (steps_cand, "cand")
        for i in range(common, len(longer)):
            ext_d.append(_diff(i, longer[i].get("label"), None, "steps", "missing-in-" + ("cand" if side == "ref" else "ref"),
                               longer[i].get("result") if side == "ref" else None,
                               longer[i].get("result") if side == "cand" else None, "external"))
    term_ref, term_cand = ref.get("terminal") or {}, cand.get("terminal") or {}
    _cmp(ext_d, ext_u, None, "terminal", None, "terminal", "kind", term_ref.get("kind"), term_cand.get("kind"))
    final_ref, final_cand = (ref.get("post_final") or {}).get("state"), (cand.get("post_final") or {}).get("state")
    if final_ref is None and final_cand is None:
        ext_u.append({"step_index": None, "step": "post_final", "attempt": None, "phase": "terminal", "key": "post_final.state", "kind": "external"})
    else:
        _cmp(ext_d, ext_u, None, "post_final", None, "terminal", "post_final.state", final_ref, final_cand)
    slow, slow_unknown = slow_steps(ref, cand)
    first = ext_d[0] if ext_d else None
    classification = classify(first, ref, cand) if first else None
    if first is None and slow:
        classification = {"fault": "slow-only", "evidence": {"slow": slow[0]}, "next": RULES_TEXT["slow-only"]}
    focus = focus_of(first, ref, cand)
    position = position_relation(focus) if classification and classification["fault"] in POSITION_FAULTS else None
    behaviors = check_behaviors(ref, cand)
    outcome_ref, outcome_cand = trace_outcome(ref), trace_outcome(cand)
    achieved_ref, achieved_cand = outcome_ref == "confirmed", outcome_cand == "confirmed"
    keep_flags = [b.get("flag") for b in behaviors if b.get("kind") == "keep"]
    # 退出成功（same）的门槛：无外部差异、无关键证据缺失、两份都确有目标达成、保留项有证据且不冲突。
    # 两份完全相同的坏轨迹不是"参照已达标"：任一份有"未达成"的正面证据就明报 not-achieved；
    # 达成与否本身缺证据（缺终态/关键证据）报 unknown。
    if ext_d:
        verdict = "differs"
    elif achieved_ref and achieved_cand:
        if ext_u or "evidence-unknown" in keep_flags:
            verdict = "unknown"
        elif "declared-keep-but-differs" in keep_flags:
            verdict = "keep-violated"
        else:
            verdict = "same"
    elif "unknown" in (outcome_ref, outcome_cand):
        verdict = "unknown"
    else:
        verdict = "not-achieved"
    report.update(first=first, differences=ext_d, unknown=ext_u, other_differences=other_d, other_unknown=other_u,
                  verdict=verdict, known_same=verdict == "same", classification=classification,
                  focus=focus, position=position,
                  slow=slow, slow_unknown=slow_unknown, behaviors=behaviors,
                  boundary={"ref": boundary_of(ref), "cand": boundary_of(cand)},
                  outcome={"ref": outcome_ref, "cand": outcome_cand},
                  achieved={"ref": achieved_ref, "cand": achieved_cand},
                  evidence={"ref": step_evidence(ref), "cand": step_evidence(cand)})
    return report


def step_evidence(trace):
    """逐步：设备自报结果 vs 外部达成证据（evidenced / not-evidenced / unknown）。"""
    out = []
    for step in _steps(trace):
        attempt = _last_attempt(step)
        out.append({"step": step.get("label"), "self_report": step.get("result"),
                    "external": effect_evidence(attempt, step.get("expect")),
                    "delivered": (attempt or {}).get("input", {}).get("delivered") if isinstance((attempt or {}).get("input"), dict) else None})
    return out


def _fmt(value):
    if value is None:
        return "unknown"
    if isinstance(value, dict):
        return "{" + ", ".join("%s=%s" % (k, _fmt(v)) for k, v in sorted(value.items())) + "}"
    if isinstance(value, list):
        return "[" + ", ".join(_fmt(v) for v in value) + "]"
    if _num(value):
        return "%.1f" % value if isinstance(value, float) else str(value)
    return str(value)


def _idx(index):
    return "-" if index is None else str(index)


def _delivery_text(inp):
    if inp.get("discarded"):
        return "丢弃(%s)" % inp["discarded"]
    if inp.get("sent") is False:
        return "未发(%s)" % (inp.get("reason_code") or "记录为未发送")
    if inp.get("sent") is not True:
        return "发送未知"
    return {True: "已发/已到", False: "已发/未到"}.get(inp.get("delivered"), "已发/送达未知")


def _locate_text(loc):
    if loc.get("error") is not None:
        return "失败(%s)" % loc["error"]
    return "point %s bbox %s src %s" % (_fmt(loc.get("point")), _fmt(loc.get("bbox")), _fmt(loc.get("source")))


def _effect_text(side):
    met = {True: "满足期待", False: "不满足期待", None: "期待无法核对"}[side.get("expect_met")]
    picture = {"diff": "画面变了", "same": "画面没变", "unknown": "画面未知"}[side.get("picture_changed")]
    return "%s -> %s（%s，%s）" % (_fmt(side.get("pre_state")), _fmt(side.get("post_state")), met, picture)


def render_report(report):
    """比对报告的可读文本（中文）：结论 → 实际结果 → 首差处两份的已证事实（目标/期待/定位/落点/
    送达/前后态）→ 位置关系 → 判断/可选的下一次检查 → 逐步自报/外部证据 → 其它外部差异 →
    证据未知 → 实现/环境差异 → 慢 → 边界 → 声明核对。事实与检查项都来自原始数据，不给修复答案。"""
    lines = ["比较: A=%s  B=%s" % (report["ref"], report["cand"])]
    if not report.get("comparable"):
        lines.append("不可比: " + report.get("reason", ""))
        return "\n".join(lines)
    verdict_text = {
        "same": "外部行为相同（两份都确有目标达成，保留项有证据且无冲突）",
        "differs": "外部行为有差异",
        "unknown": "未知：没有已知差异，但关键证据缺失",
        "not-achieved": "未达标：两份外部行为一致，但参照或候选没有确有目标达成（不是'参照已达标'）",
        "keep-violated": "保留行为冲突：外部行为一致，但声明的保留项在声明的证据上不一致",
    }
    lines.append("结论: " + verdict_text.get(report.get("verdict"), report.get("verdict")))
    lines.append("实际结果: A=%s  B=%s" % (report["outcome"]["ref"], report["outcome"]["cand"]))
    achieved = report.get("achieved") or {}
    lines.append("目标达成(外部证据): A=%s  B=%s" % ("是" if achieved.get("ref") else "否", "是" if achieved.get("cand") else "否"))
    first = report.get("first")
    if first:
        lines.append("首差: 步骤=%s 尝试=%s 阶段=%s 维度=%s" % (
            _fmt(first.get("step")), _idx(first.get("attempt")), first.get("phase"), first.get("key")))
        lines.append("  A=%s" % _fmt(first.get("ref")))
        lines.append("  B=%s" % _fmt(first.get("cand")))
    classification = report.get("classification")
    fault = classification["fault"] if classification else None
    focus = report.get("focus")
    if focus:
        ref, cand = focus["ref"], focus["cand"]
        lines.append("首差处已证事实(A=参照 B=候选，原始数据):")
        if ref.get("target") is not None or cand.get("target") is not None:
            lines.append("  步骤目标: A=%s  B=%s" % (_fmt(ref.get("target")), _fmt(cand.get("target"))))
        lines.append("  期待: %s" % _fmt(ref.get("expect")))  # 两份期待不同会在对齐时判不可比
        lines.append("  定位: A=%s  |  B=%s" % (_locate_text(ref["locate"]), _locate_text(cand["locate"])))
        lines.append("  落点: A=%s  |  B=%s" % (_fmt(ref["input"].get("point")), _fmt(cand["input"].get("point"))))
        lines.append("  送达: A=%s  |  B=%s" % (_delivery_text(ref["input"]), _delivery_text(cand["input"])))
        lines.append("  前态->后态: A=%s  |  B=%s" % (_effect_text(ref), _effect_text(cand)))
    position = report.get("position")
    if position:
        lines.append("位置关系(已证事实，不是根因): " + POSITION_TEXT[position])
    if classification:
        lines.append("判断: %s（类别，不是根因）" % fault)
        for key in ("reason", "reason_code", "discarded", "effect_budget_ms", "final_state", "terminal", "note"):
            if key in classification["evidence"]:
                lines.append("  %s=%s" % (key, _fmt(classification["evidence"][key])))
        lines.append("下一步(选一项检查，改一处后用同一输入复验):")
        for n, check in enumerate(next_checks(classification, position), 1):
            lines.append("  %d) %s" % (n, check))
    ev_a, ev_b = report["evidence"]["ref"], report["evidence"]["cand"]
    lines.append("逐步(自报/外部证据/送达):")
    for i in range(max(len(ev_a), len(ev_b))):
        a = ev_a[i] if i < len(ev_a) else {}
        b = ev_b[i] if i < len(ev_b) else {}
        lines.append("  %s  A=%s/%s/%s  B=%s/%s/%s" % (
            _fmt(a.get("step") or b.get("step")), a.get("self_report"), a.get("external"), _fmt(a.get("delivered")),
            b.get("self_report"), b.get("external"), _fmt(b.get("delivered"))))
    if len(report.get("differences") or []) > 1:
        lines.append("其它外部差异(%d):" % (len(report["differences"]) - 1))
        for diff in report["differences"][1:12]:
            lines.append("  %s/%s/%s A=%s B=%s" % (_fmt(diff.get("step")), _idx(diff.get("attempt")), diff.get("phase") + "." + str(diff.get("key")),
                                                 _fmt(diff.get("ref")), _fmt(diff.get("cand"))))
    unknown = report.get("unknown") or []
    if unknown:
        lines.append("证据未知(%d，不计为相同):" % len(unknown))
        for item in unknown[:16]:
            lines.append("  %s/%s/%s" % (_fmt(item.get("step")), _idx(item.get("attempt")), item.get("phase") + "." + str(item.get("key"))))
    other = report.get("other_differences") or []
    if other or report.get("other_unknown"):
        lines.append("实现/环境差异(%d，不影响外部行为结论；未知 %d):" % (len(other), len(report.get("other_unknown") or [])))
        for diff in other[:10]:
            lines.append("  [%s] %s/%s/%s A=%s B=%s" % (diff.get("kind"), _fmt(diff.get("step")), _idx(diff.get("attempt")),
                                                      diff.get("phase") + "." + str(diff.get("key")), _fmt(diff.get("ref")), _fmt(diff.get("cand"))))
        for item in (report.get("other_unknown") or [])[:6]:
            lines.append("  [unknown] %s/%s/%s" % (_fmt(item.get("step")), _idx(item.get("attempt")), item.get("phase") + "." + str(item.get("key"))))
    slow = report.get("slow") or []
    lines.append("慢步骤: " + ("无" if not slow else ""))
    for item in slow:
        lines.append("  %s A=%.1fms B=%.1fms 归因=%s（A=%.1fms B=%.1fms）" % (
            item["step"], item["ref_ms"], item["cand_ms"], item["phase"], item["phase_ref_ms"], item["phase_cand_ms"]))
    for item in report.get("slow_unknown") or []:
        lines.append("  %s: %s" % (item.get("step"), item.get("note")))
    lines.append("边界(最后一次确认):")
    for side in ("ref", "cand"):
        b = (report.get("boundary") or {}).get(side) or {}
        lines.append("  %s: 步骤=%s 状态=%s 已发未确认=%s 复核拒绝=%s 丢弃尝试=%s 终止=%s" % (
            "A" if side == "ref" else "B", _fmt(b.get("last_confirmed_step")), _fmt(b.get("last_confirmed_state")),
            _fmt(b.get("unconfirmed_input")), _fmt(b.get("refused_input")), b.get("discarded_attempts"),
            (b.get("terminal") or {}).get("kind")))
    flagged = [x for x in report.get("behaviors") or [] if x.get("flag") != "consistent"]
    if report.get("behaviors"):
        lines.append("声明核对: 共 %d 条，不一致/未知 %d 条" % (len(report["behaviors"]), len(flagged)))
        for item in flagged:
            lines.append("  [%s] %s -> %s (证据 %s: A=%s B=%s)" % (
                item.get("flag"), item.get("kind"), item.get("behavior"), item.get("evidence"), _fmt(item.get("ref")), _fmt(item.get("cand"))))
    lines.append("不比较: " + ", ".join("%s（每次启动都不同，只记录）" % k for k in report.get("not_compared", {}).get("pre.binding", [])))
    return "\n".join(lines)
