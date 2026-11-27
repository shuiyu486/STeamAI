"""windows-flow 参照-候选工作台（refdiff / workbench / win_target）的直接测试。

不调用模型、不占前台、不注入系统输入：
- RefdiffTests：手造两份观察的首差比对（嵌套分歧、删掉正确证据、前缀不齐、空数组/自报漂绿、
  同一份坏轨迹自比不绿、缺终态不绿、保留项声明在坐标上要尊重），纯逻辑，零第三方依赖。
- MatrixSafetyTests / TrustGateTests：plan id 安全唯一、产物不覆盖、compare --out 不指回输入、
  --exe 的 SHA-256 闸门（纯函数与 CLI 退出码；用错误 SHA 和合成场景证明拒绝发生在启动之前）。
- SynthEndToEndTests：合成目标经 CLI run/compare/matrix 的端到端（numpy+opencv），依赖缺失整类 skip。
- DemoTargetTests：自建 dm_demo_target 的真实屏外窗口；只有显式给出 STEAMAI_DEMO_TARGET 与
  STEAMAI_DEMO_TARGET_SHA256 才运行，默认 skip。
"""
import copy
import hashlib
import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
EXAMPLE = ROOT / "packs/game-automation/tooling/examples/windows-flow"
WORKBENCH = str(EXAMPLE / "workbench.py")
_LOADED = {}


def _load(name):
    if name not in _LOADED:
        # 加载产品示例时禁写字节码，避免在 pack 目录留下 __pycache__（Fresh source closure 拒绝）
        with mock.patch.object(sys, "dont_write_bytecode", True):
            spec = importlib.util.spec_from_file_location(
                "steamai_wb_" + name, str(EXAMPLE / (name + ".py")))
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
        _LOADED[name] = module
    return _LOADED[name]


RD = _load("refdiff")
WB = _load("workbench")
WT = _load("win_target")

try:
    import cv2  # noqa: F401
    import numpy  # noqa: F401
    HAVE_IMAGING = True
except ImportError:
    HAVE_IMAGING = False

SKIP_IMAGING = "numpy/opencv 不可用：合成端到端跳过（首差比对与安全拒绝单测零依赖仍运行）"
DEMO_EXE = os.environ.get("STEAMAI_DEMO_TARGET")
DEMO_SHA = os.environ.get("STEAMAI_DEMO_TARGET_SHA256")
SKIP_DEMO = ("需要显式给出 STEAMAI_DEMO_TARGET（自建 dm_demo_target.exe）与 STEAMAI_DEMO_TARGET_SHA256"
             "（调用者独立核对的哈希）才运行真实窗口测试")
ENV = dict(os.environ, PYTHONIOENCODING="utf-8", PYTHONDONTWRITEBYTECODE="1")


def cli(*args, timeout=180):
    """产品 CLI 的唯一入口：子进程、禁写字节码，返回 (rc, stdout)。"""
    proc = subprocess.run([sys.executable, "-B", WORKBENCH] + [str(a) for a in args], env=ENV,
                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, encoding="utf-8",
                          timeout=timeout)
    return proc.returncode, proc.stdout


def sha256_file(path):
    with open(path, "rb") as handle:
        return hashlib.sha256(handle.read()).hexdigest()


# -- 手造观察（windows-flow-trace/1） -------------------------------------------------------
P1 = {"tab1": [255, 255, 0], "tab2": [200, 200, 200]}
P2 = {"tab1": [200, 200, 200], "tab2": [255, 255, 0]}
P3 = {"tab1": [200, 200, 200], "tab2": [200, 200, 200]}


def attempt(page_before, page_after, pic_before, pic_after, sent=True, delivered=True, discarded=None,
            reason_code="confirmed"):
    return {
        "pre": {"t": 1.0, "state": {"page": page_before, "stats": [0, 0, 0, 0, 0]}, "picture": pic_before,
                "binding": {"rect": [1, 2, 3, 4], "client": [200, 218], "hwnd": 1, "pid": 2}},
        "wait_ms": 1.0, "locate_ms": 1.0, "effect_ms": 1.0,
        "locate": {"point": [99, 14], "bbox": [68, 2, 62, 24], "source": "callback", "score": None},
        "input": {"sent": sent, "kind": "post-message", "point": [99, 14], "delivered": delivered,
                  "clicks_delta": 1, "discarded": discarded, "reason_code": reason_code, "reason": None},
        "post": {"t": 2.0, "state": {"page": page_after, "stats": [0, 1, 1, 0, 0]}, "picture": pic_after,
                 "confirmed": sent and delivered and page_after != page_before, "reason_code": reason_code,
                 "reason": None},
    }


def trace(step_attempts, labels=("to-page-2", "to-page-3")):
    steps = []
    for i, (label, atts) in enumerate(zip(labels, step_attempts)):
        if not atts:
            steps.append({"label": label, "index": i, "expect": {"page": i + 2}, "attempts": [],
                          "result": "not-reached", "duration_ms": None})
            continue
        last = atts[-1]
        if last["input"]["discarded"]:
            res = "discarded"
        elif last["post"]["confirmed"]:
            res = "confirmed"
        elif last["input"]["sent"] is False:
            res = "refused"
        else:
            res = "unconfirmed"
        steps.append({"label": label, "index": i, "expect": {"page": i + 2}, "attempts": atts,
                      "result": res, "duration_ms": 10.0})
    return {"schema": "windows-flow-trace/1", "scenario": {"name": "s"}, "variant": {"id": "v"},
            "start": {"state": {"page": 1, "stats": [0, 0, 0, 0, 0]}, "picture": P1},
            "steps": steps,
            "post_final": {"state": {"page": 3, "stats": [0, 2, 2, 0, 0]}, "picture": P3},
            "terminal": {"kind": "flow-complete", "reason": "", "step": None}, "timings": {"total_ms": 20}}


def good_trace():
    return trace([[attempt(1, 2, P1, P2)], [attempt(2, 3, P2, P3)]])


def bad_trace():
    """点击收到但页签没变：第二步 delivered=True、状态不变、终止 input-unconfirmed。"""
    bad = copy.deepcopy(good_trace())
    bad["steps"][1]["attempts"][0] = attempt(2, 2, P2, P2)
    bad["steps"][1]["result"] = "unconfirmed"
    bad["terminal"] = {"kind": "input-unconfirmed", "reason": "click hit", "step": "to-page-3"}
    bad["post_final"]["state"] = {"page": 2, "stats": [0, 2, 2, 0, 0]}
    bad["post_final"]["picture"] = P2
    return bad


def declare(cand, kind, behavior, evidence):
    cand.setdefault("variant", {})["behaviors"] = {kind: [{"behavior": behavior, "evidence": evidence}]}
    return cand


class RefdiffTests(unittest.TestCase):
    def test_nested_post_state_difference_is_first_diff(self):
        report = RD.compare_traces(good_trace(), bad_trace())
        self.assertEqual(report["verdict"], "differs")
        self.assertEqual((report["first"]["step"], report["first"]["attempt"], report["first"]["phase"], report["first"]["key"]),
                         ("to-page-3", 0, "post", "state"))
        self.assertEqual(report["classification"]["fault"], "delivered-no-effect")
        self.assertIn("下一步", RD.render_report(report))
        self.assertEqual(report["achieved"], {"ref": True, "cand": False})

    def test_wrong_page_is_not_no_effect(self):
        cand = bad_trace()
        step = cand["steps"][1]
        step["attempts"][0] = attempt(2, 1, P2, P1)
        cand["post_final"]["state"]["page"] = 1
        report = RD.compare_traces(good_trace(), cand)
        self.assertEqual(report["classification"]["fault"], "wrong-state")
        self.assertFalse(report["achieved"]["cand"])

    def test_missing_picture_evidence_is_unknown_not_same(self):
        cand = good_trace()
        cand["steps"][1]["attempts"][0]["post"]["picture"] = None
        report = RD.compare_traces(good_trace(), cand)
        self.assertEqual(report["verdict"], "unknown")
        self.assertIsNone(report["first"])
        self.assertTrue(any(u["phase"] == "effect" for u in report["unknown"]))
        self.assertEqual(report["outcome"]["cand"], "unknown")

    def test_prefix_before_attempt_count(self):
        ref = good_trace()
        cand = copy.deepcopy(ref)
        cand["steps"][1]["attempts"] = [attempt(2, 2, P2, P2, sent=False, delivered=None, discarded="pause",
                                                 reason_code=None), attempt(2, 3, P2, P3)]
        report = RD.compare_traces(ref, cand)
        self.assertEqual((report["first"]["phase"], report["first"]["attempt"]), ("input", 0))
        self.assertEqual(report["classification"]["fault"], "discarded-by-pause")
        cand = copy.deepcopy(ref)
        cand["steps"][1]["attempts"].append(attempt(2, 3, P2, P3, sent=False, delivered=None, discarded="stop",
                                                    reason_code=None))
        report = RD.compare_traces(ref, cand)
        self.assertEqual((report["first"]["phase"], report["first"]["key"]), ("attempts", "count"))
        cand = copy.deepcopy(ref)
        cand["steps"] = cand["steps"][:1]
        cand["terminal"] = {"kind": "input-unconfirmed", "reason": "x", "step": None}
        report = RD.compare_traces(ref, cand)
        self.assertEqual((report["first"]["phase"], report["classification"]["fault"]), ("steps", "step-missing"))

    def test_empty_attempts_and_steps_are_unknown(self):
        empty = trace([[], []])
        report = RD.compare_traces(copy.deepcopy(empty), copy.deepcopy(empty))
        self.assertEqual(report["verdict"], "unknown")
        self.assertEqual(report["outcome"]["ref"], "unknown")
        empty["steps"] = []
        self.assertEqual(RD.compare_traces(empty, copy.deepcopy(empty))["verdict"], "unknown")

    def test_self_reported_confirmed_cannot_go_green(self):
        a, b = good_trace(), good_trace()
        for t in (a, b):
            for step in t["steps"]:
                self.assertTrue(step["attempts"][0]["post"]["confirmed"])  # 设备自报确认
                step["attempts"][0]["pre"]["picture"] = None
                step["attempts"][0]["post"]["picture"] = None
        report = RD.compare_traces(a, b)
        self.assertEqual(report["verdict"], "unknown")
        self.assertEqual((report["outcome"]["ref"], report["outcome"]["cand"]), ("unknown", "unknown"))

    def test_resolve_path_wildcards(self):
        good = good_trace()
        self.assertEqual(RD.resolve_path(good, "steps[*].attempts[*].locate.source"), [["callback"], ["callback"]])
        self.assertEqual(RD.resolve_path(good, "steps[*].result"), ["confirmed", "confirmed"])
        self.assertEqual(RD.resolve_path(trace([[], []]), "steps[*].result"), ["not-reached", "not-reached"])
        no_steps = good_trace()
        no_steps["steps"] = []
        self.assertEqual(RD.resolve_path(no_steps, "steps[*].result"), [])

    def test_behaviors_flags(self):
        ref, cand = good_trace(), good_trace()
        path = "steps[*].attempts[*].locate.source"
        cand["steps"][0]["attempts"][0]["locate"]["source"] = "distilled"
        declare(cand, "keep", "定位来源", path)
        cand["variant"]["behaviors"]["replace"] = [{"behavior": "r", "evidence": path}]
        self.assertEqual([b["flag"] for b in RD.check_behaviors(ref, cand)],
                         ["declared-keep-but-differs", "consistent"])
        empty = trace([[], []])
        declare(empty, "keep", "b", path)
        self.assertEqual(RD.check_behaviors(trace([[], []]), empty)[0]["flag"], "evidence-unknown")

    def test_implementation_change_is_allowed_without_declaration(self):
        cand = good_trace()
        cand["steps"][0]["attempts"][0]["input"]["point"] = [100, 15]
        cand["steps"][0]["attempts"][0]["locate"]["point"] = [100, 15]
        report = RD.compare_traces(good_trace(), cand)
        self.assertEqual(report["verdict"], "same")
        self.assertEqual(len(report["other_differences"]), 2)

    # -- 协调方第五点：同一份坏轨迹自比不能绿 ---------------------------------------------
    def test_identical_bad_trace_is_not_achieved(self):
        bad = bad_trace()
        report = RD.compare_traces(bad, copy.deepcopy(bad))
        self.assertEqual(report["verdict"], "not-achieved")
        self.assertEqual(report["achieved"], {"ref": False, "cand": False})
        self.assertEqual((report["differences"], report["unknown"]), ([], []))
        self.assertFalse(report["known_same"])
        self.assertIn("未达标", RD.render_report(report))

    def test_no_input_no_credit_for_a_changed_page(self):
        for field, value, verdict in (("sent", False, "not-achieved"), ("delivered", False, "not-achieved"),
                                      ("discarded", "pause", "not-achieved"), ("sent", 1, "unknown"),
                                      ("delivered", None, "unknown")):
            with self.subTest(field=field, value=value):
                t = good_trace()
                t["steps"][-1]["attempts"][0]["input"][field] = value
                report = RD.compare_traces(t, copy.deepcopy(t))
                self.assertEqual(report["verdict"], verdict)
                self.assertEqual(report["boundary"]["cand"]["last_confirmed_step"], "to-page-2")

    def test_last_attempt_and_final_state_cannot_borrow_earlier_proof(self):
        t = good_trace()
        t["steps"][-1]["attempts"].append({"pre": t["steps"][-1]["attempts"][0]["pre"]})
        report = RD.compare_traces(t, copy.deepcopy(t))
        self.assertEqual((report["outcome"]["cand"], report["boundary"]["cand"]["last_confirmed_step"]),
                         ("unknown", "to-page-2"))
        for state, verdict in (({"page": 1}, "not-achieved"), ({"page": None}, "unknown"), ({}, "unknown")):
            t = good_trace()
            t["post_final"]["state"] = state
            self.assertEqual(RD.compare_traces(t, copy.deepcopy(t))["verdict"], verdict, state)

    def test_boolean_state_is_not_a_count_and_null_is_no_evidence(self):
        ref = good_trace()
        last = ref["steps"][-1]
        last["expect"] = {"indicators": [True]}
        last["attempts"][0]["post"]["state"]["indicators"] = [True]
        ref["post_final"]["state"]["indicators"] = [True]
        cand = copy.deepcopy(ref)
        cand["steps"][-1]["expect"]["indicators"] = [1]
        self.assertEqual(RD.compare_traces(ref, cand)["verdict"], "incomparable")
        last["attempts"][0]["post"]["state"]["indicators"] = [1]
        self.assertEqual(RD.compare_traces(ref, copy.deepcopy(ref))["verdict"], "not-achieved")
        last["expect"]["indicators"] = [None]
        last["attempts"][0]["post"]["state"]["indicators"] = [None]
        ref["post_final"]["state"]["indicators"] = [None]
        self.assertEqual(RD.compare_traces(ref, copy.deepcopy(ref))["verdict"], "unknown")
        self.assertEqual(RD.compare_value(True, 1), "diff")
        self.assertEqual(RD.compare_value(1, 1.0), "same")

    def test_good_trace_vs_itself_is_same(self):
        report = RD.compare_traces(good_trace(), good_trace())
        self.assertEqual((report["verdict"], report["achieved"]), ("same", {"ref": True, "cand": True}))

    def test_missing_final_state_or_terminal_is_unknown(self):
        for mutate in (lambda t: t.update(post_final={}),
                       lambda t: t["post_final"].update(state=None),
                       lambda t: t.update(terminal=None)):
            t = good_trace()
            mutate(t)
            report = RD.compare_traces(t, copy.deepcopy(t))
            self.assertEqual(report["verdict"], "unknown")
            self.assertFalse(report["known_same"])

    def test_declared_keep_on_coordinates_is_honored(self):
        point = "steps[*].attempts[*].input.point"
        cand = good_trace()
        cand["steps"][0]["attempts"][0]["input"]["point"] = [100, 15]
        declare(cand, "keep", "tab 点击坐标固定", point)
        report = RD.compare_traces(good_trace(), cand)
        self.assertEqual((report["verdict"], report["behaviors"][0]["flag"]), ("keep-violated", "declared-keep-but-differs"))
        same = declare(good_trace(), "keep", "tab 点击坐标固定", point)
        report = RD.compare_traces(good_trace(), same)
        self.assertEqual((report["verdict"], report["behaviors"][0]["flag"]), ("same", "consistent"))

    def test_declared_keep_with_missing_evidence_is_unknown(self):
        cand = good_trace()
        for step in cand["steps"]:
            step["attempts"][0]["post"]["picture"] = None
        declare(cand, "keep", "画面", "steps[*].attempts[*].post.picture")
        self.assertEqual(RD.compare_traces(good_trace(), cand)["verdict"], "unknown")

    def test_declared_replace_that_did_not_change_does_not_block(self):
        cand = declare(good_trace(), "replace", "坐标换", "steps[*].attempts[*].input.point")
        report = RD.compare_traces(good_trace(), cand)
        self.assertEqual((report["verdict"], report["behaviors"][0]["flag"]), ("same", "declared-replace-but-same"))

    # -- 负责人拿到输出就能选下一次检查：首差处两份原始事实 + 位置关系 + 可选检查 -------------------
    @staticmethod
    def with_target(trace_, target="tab"):
        for step in trace_["steps"]:
            step["target"] = target
        return trace_

    def test_same_label_but_different_step_definition_is_not_comparable(self):
        """false green 反例：标签相同但 target/when/expect 不同 = 不是同一份步骤表，行为一致也不能判 same。"""
        ref = self.with_target(good_trace())
        for step in ref["steps"]:
            step["when"] = {"state": "idle"}
        for key, other in (("target", "other"), ("when", {"state": "x"}), ("when", {"state": "idle", "mode": 2}),
                           ("target", None), ("expect", {"page": 2})):
            cand = copy.deepcopy(ref)
            cand["steps"][1][key] = other
            report = RD.compare_traces(ref, cand)
            self.assertEqual((report["verdict"], report["comparable"], report.get("known_same")), ("incomparable", False, None), key)
            self.assertIn(key, report["reason"])
            self.assertIn("不可比", RD.render_report(report))
        self.assertEqual(RD.compare_traces(ref, copy.deepcopy(ref))["verdict"], "same")  # 对照：定义一致仍是 same

    def test_landing_point_shift_and_locate_difference_are_named(self):
        ref = self.with_target(good_trace())
        cand = self.with_target(bad_trace())
        cand["steps"][1]["attempts"][0]["input"]["point"] = [99, 74]
        report = RD.compare_traces(ref, cand)
        self.assertEqual((report["classification"]["fault"], report["position"]), ("delivered-no-effect", "point-shifted"))
        text = RD.render_report(report)
        self.assertIn("定位后输入位置变了", text)
        for raw in ("步骤目标: A=tab  B=tab", "期待: {page=3}", "point [99, 14]", "[99, 74]", "已发/已到",
                    "不满足期待", "画面没变", "先核对转换，不重发输入", "下一步"):
            self.assertIn(raw, text, raw)
        cand["steps"][1]["attempts"][0]["locate"]["point"] = [120, 14]
        cand["steps"][1]["attempts"][0]["input"]["point"] = [120, 14]
        report = RD.compare_traces(ref, cand)
        self.assertEqual(report["position"], "locate-differs")
        self.assertIn("定位本身不同", RD.render_report(report))

    def test_missing_input_is_not_a_confirmed_refusal_or_a_position(self):
        ref, cand = good_trace(), bad_trace()
        a = cand["steps"][1]["attempts"][0]
        a["input"].pop("sent")
        report = RD.compare_traces(ref, cand)
        self.assertEqual(report["position"], "send-unknown")
        self.assertIn("发送未知", RD.render_report(report))
        a["input"]["sent"] = True
        for field, which in (("locate", "locate-unknown"), ("input", "point-unknown")):
            good = a[field]["point"]
            a[field]["point"] = []
            self.assertEqual(RD.compare_traces(ref, cand)["position"], which)
            a[field]["point"] = good

    def test_consistent_position_redirects_to_target_side_or_evidence(self):
        ref, cand = good_trace(), bad_trace()
        report = RD.compare_traces(ref, cand)
        self.assertEqual(report["position"], "point-same")
        text = RD.render_report(report)
        self.assertIn("位置一致、消息已到但无效果", text)
        self.assertIn("不要先改落点或加等待", text)
        self.assertNotIn("定位本身不同", text)
        cand = good_trace()  # 位置一致但后置画面缺证据、结果码不同 → 首差落在 result
        cand["steps"][1]["attempts"][0]["post"]["picture"] = None
        cand["steps"][1]["result"] = "unconfirmed"
        report = RD.compare_traces(ref, cand)
        self.assertEqual((report["classification"]["fault"], report["position"]), ("effect-unknown", "point-same"))
        self.assertIn("位置一致但效果未知/尚未可证", RD.render_report(report))
        cand = good_trace()  # 复核拒绝：没发输入，没有落点可比
        cand["steps"][1]["attempts"][0] = attempt(2, 3, P2, P3, sent=False, delivered=None, reason_code="stale-binding")
        cand["steps"][1]["attempts"][0]["input"]["point"] = None
        cand["steps"][1]["result"] = "refused"
        report = RD.compare_traces(ref, cand)
        self.assertEqual((report["classification"]["fault"], report["position"]), ("stale-refusal", "not-sent"))
        self.assertIn("无落点可比", RD.render_report(report))


# -- 安全拒绝（协调方第二、四点） -------------------------------------------------------------
class MatrixSafetyTests(unittest.TestCase):
    def plan(self, ids, compares=None):
        return {"schema": "windows-flow-plan/1", "scenario": "scenario.example.json",
                "runs": [{"id": i, "expect_result": "confirmed"} for i in ids], "compares": compares or []}

    def test_unsafe_or_duplicate_ids_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            for ids in (["a/b"], ["a\\b"], [".."], ["."], ["C:\\x"], ["/abs"], ["x:y"], ["", "ok"], ["a" * 60],
                        ["ok", "ok"], ["A", "a"], ["ok\n"], ["NUL"], ["con.part"]):
                with self.assertRaises(WB.UsageError, msg=repr(ids)):
                    WB._preflight_matrix(self.plan(ids), os.path.join(tmp, "out"))
            self.assertFalse(os.path.exists(os.path.join(tmp, "out")))

    def test_existing_artifact_refused_and_unchanged(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "out")
            os.makedirs(out)
            keeper = os.path.join(out, "summary.json")
            with open(keeper, "wb") as handle:
                handle.write(b'{"keep": true}')
            with self.assertRaises(WB.UsageError):
                WB._preflight_matrix(self.plan(["a", "b"], [["a", "b"]]), out)
            self.assertEqual(Path(keeper).read_bytes(), b'{"keep": true}')
            self.assertEqual(os.listdir(out), ["summary.json"])
            shutil.rmtree(out)
            os.makedirs(out)
            with open(os.path.join(out, "b.trace.json"), "wb") as handle:
                handle.write(b"x")
            with self.assertRaises(WB.UsageError):
                WB._preflight_matrix(self.plan(["a", "b"]), out)
            self.assertEqual(os.listdir(out), ["b.trace.json"])

    def test_publish_preserves_neighbour_and_late_output(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "report.json"
            neighbour = Path(tmp) / "report.json.tmp"
            neighbour.write_bytes(b"keep")
            WB._refuse_existing(out)
            out.write_bytes(b"other report")  # 预检后才出现的另一份成品也不能覆盖
            with self.assertRaises(WB.UsageError):
                WB._write_json_atomic(out, {"new": True})
            self.assertEqual(out.read_bytes(), b"other report")
            self.assertEqual(neighbour.read_bytes(), b"keep")
            fresh = Path(tmp) / "new.json"
            WB._write_json_atomic(fresh, {"new": True})
            self.assertEqual(json.loads(fresh.read_text(encoding="utf-8")), {"new": True})
            with self.assertRaises(TypeError):
                WB._write_json_atomic(Path(tmp) / "invalid.json", {"bad": object()})
            self.assertEqual(set(os.listdir(tmp)), {out.name, neighbour.name, fresh.name})

    def test_compare_out_cannot_point_at_input_or_existing_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            a, b, c = (os.path.join(tmp, n) for n in ("a.json", "b.json", "c.json"))
            for path in (a, b):
                with open(path, "wb") as handle:
                    handle.write(b"{}")
            with self.assertRaises(WB.UsageError):
                WB._refuse_existing(a, [a, b])
            with self.assertRaises(WB.UsageError):
                WB._refuse_existing(b, [a, b])
            self.assertEqual(WB._refuse_existing(c, [a, b]), os.path.abspath(c))
            with self.assertRaises(WB.UsageError):
                WB._refuse_existing(os.path.join(tmp, "missing-dir", "c.json"))
            with open(c, "wb") as handle:
                handle.write(b"orig")
            with self.assertRaises(WB.UsageError):
                WB._refuse_existing(c, [a, b])
            self.assertEqual(Path(c).read_bytes(), b"orig")

    def test_matrix_compares_must_name_declared_runs(self):
        with tempfile.TemporaryDirectory() as tmp:
            for compares in ([["a", "zz"]], [["a"]], [{"ref": "zz", "cand": "a"}]):
                with self.assertRaises(WB.UsageError):
                    WB._preflight_matrix(self.plan(["a", "b"], compares), os.path.join(tmp, "out"))
            ids, pairs = WB._preflight_matrix(self.plan(["baseline", "x"], [{"ref": "x", "cand": "baseline", "expect_fault": "differs"}]),
                                              os.path.join(tmp, "out"))
            self.assertEqual((ids, pairs), (["baseline", "x"], [("x", "baseline", {"ref": "x", "cand": "baseline", "expect_fault": "differs"})]))


class TrustGateTests(unittest.TestCase):
    def test_verify_exe_refuses_without_matching_hash(self):
        with tempfile.TemporaryDirectory() as tmp:
            exe = os.path.join(tmp, "renamed_demo_target.exe")
            with open(exe, "wb") as handle:
                handle.write(b"not the demo target")
            for expected in (None, "", "abc", "0" * 64, "g" * 64):
                with self.assertRaises(WT.ExeNotVerified, msg=repr(expected)):
                    WT.verify_exe(exe, expected)
            with self.assertRaises(WT.ExeNotVerified):
                WT.verify_exe(os.path.join(tmp), "0" * 64)  # 目录不是普通文件
            with self.assertRaises(WT.ExeNotVerified):
                WT.verify_exe(os.path.join(tmp, "missing.exe"), "0" * 64)
            self.assertEqual(WT.verify_exe(exe, sha256_file(exe)), sha256_file(exe))

    def test_start_refuses_before_creating_any_process(self):
        with tempfile.TemporaryDirectory() as tmp:
            exe = os.path.join(tmp, "renamed_demo_target.exe")
            with open(exe, "wb") as handle:
                handle.write(b"x")
            target = WT.DemoTarget(exe, None)
            with mock.patch.object(WT.subprocess, "Popen", side_effect=AssertionError("不能创建进程")) as popen:
                with self.assertRaises(WT.ExeNotVerified):
                    target.start()
            popen.assert_not_called()


class CliSafetyTests(unittest.TestCase):
    """CLI 退出码：拒绝都发生在启动任何目标之前（用错误的 SHA 与合成场景证明）。"""

    def test_observe_refuses_missing_or_wrong_sha(self):
        with tempfile.TemporaryDirectory() as tmp:
            for label, extra in (("none", []), ("wrong", ["--exe-sha256", "0" * 64]), ("short", ["--exe-sha256", "abc"])):
                out = os.path.join(tmp, label + ".json")
                rc, text = cli("observe", "--scenario", EXAMPLE / "scenario.example.json", "--out", out,
                               "--exe", sys.executable, *extra)
                self.assertEqual(rc, 2, text)
                self.assertIn("拒绝启动", text)
                self.assertFalse(os.path.exists(out))

    def test_correct_hash_of_another_program_fails_handshake_without_observation(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "o.json")
            rc, text = cli("observe", "--scenario", EXAMPLE / "scenario.example.json", "--out", out,
                           "--exe", sys.executable, "--exe-sha256", sha256_file(sys.executable))
            self.assertEqual(rc, 2, text)
            self.assertIn("基础设施错误", text)
            self.assertFalse(os.path.exists(out))

    def test_compare_out_refused_by_cli(self):
        with tempfile.TemporaryDirectory() as tmp:
            a, b = os.path.join(tmp, "a.json"), os.path.join(tmp, "b.json")
            for path, data in ((a, good_trace()), (b, bad_trace())):
                with open(path, "w", encoding="utf-8") as handle:
                    json.dump(data, handle)
            before = sha256_file(a)
            rc, text = cli("compare", a, b, "--out", a)
            self.assertEqual(rc, 2, text)
            self.assertEqual(sha256_file(a), before)

    def test_compare_same_label_different_target_is_incomparable_via_cli(self):
        ref, cand = RefdiffTests.with_target(good_trace()), RefdiffTests.with_target(good_trace(), "other")
        with tempfile.TemporaryDirectory() as tmp:
            a, b = os.path.join(tmp, "a.json"), os.path.join(tmp, "b.json")
            for path, data in ((a, ref), (b, cand)):
                with open(path, "w", encoding="utf-8") as handle:
                    json.dump(data, handle)
            rc, text = cli("compare", a, b)
            self.assertEqual(rc, 2, text)
            self.assertIn("不可比", text)
            self.assertIn("target", text)

    def test_matrix_unsafe_id_refused_before_launch(self):
        with tempfile.TemporaryDirectory() as tmp:
            plan = {"schema": "windows-flow-plan/1", "scenario": str(EXAMPLE / "scenario.example.json"),
                    "runs": [{"id": "a/b"}], "compares": []}
            plan_path = os.path.join(tmp, "plan.json")
            with open(plan_path, "w", encoding="utf-8") as handle:
                json.dump(plan, handle)
            out = os.path.join(tmp, "out")
            rc, text = cli("matrix", "--plan", plan_path, "--out", out, "--exe", sys.executable,
                           "--exe-sha256", "0" * 64)
            self.assertEqual(rc, 2, text)
            self.assertIn("安全标识", text)
            self.assertNotIn("SHA", text)  # 拒绝来自 id 预检，而不是启动后的哈希核对
            self.assertFalse(os.path.exists(out))

    def test_matrix_existing_artifact_refused_unchanged(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "out")
            os.makedirs(out)
            keeper = os.path.join(out, "summary.json")
            with open(keeper, "wb") as handle:
                handle.write(b'{"keep": true}')
            rc, text = cli("matrix", "--plan", EXAMPLE / "plan.synth.example.json", "--out", out)
            self.assertEqual(rc, 2, text)
            self.assertIn("已存在", text)
            self.assertEqual(Path(keeper).read_bytes(), b'{"keep": true}')
            self.assertEqual(os.listdir(out), ["summary.json"])  # 一个 run 都没跑


@unittest.skipUnless(HAVE_IMAGING, SKIP_IMAGING)
class SynthEndToEndTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.mkdtemp(prefix="wf-synth-")
        cls.out = os.path.join(cls.tmp, "matrix")
        cls.rc, cls.text = cli("matrix", "--plan", EXAMPLE / "plan.synth.example.json", "--out", cls.out)
        with open(os.path.join(cls.out, "summary.json"), encoding="utf-8") as handle:
            cls.summary = json.load(handle)
        cls.traces = {}
        for run in cls.summary["runs"]:
            with open(run["trace"], encoding="utf-8") as handle:
                cls.traces[run["id"]] = json.load(handle)
        cls.reports = {}
        for compare in cls.summary["compares"]:
            with open(compare["report"], encoding="utf-8") as handle:
                cls.reports[(compare["ref"], compare["cand"])] = json.load(handle)

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmp, ignore_errors=True)

    def test_matrix_matches_declarations_with_exit_zero(self):
        self.assertEqual(self.rc, 0, self.text)
        self.assertTrue(self.summary["all_as_declared"])
        self.assertEqual(self.summary["workflow_confirmed"], ["baseline", "fixed", "pause-resume"])
        self.assertEqual(self.summary["workflow_not_confirmed"], ["wrong-point", "stale-move", "effect-late"])
        self.assertIn("不表示工作流成功", self.text)

    def test_bad_candidate_first_diff_and_next_step(self):
        report = self.reports[("baseline", "wrong-point")]
        self.assertEqual(report["verdict"], "differs")
        self.assertEqual((report["first"]["step"], report["first"]["phase"], report["first"]["key"]), ("open", "post", "state"))
        self.assertEqual(report["classification"]["fault"], "delivered-no-effect")
        sent = self.traces["wrong-point"]["steps"][0]["attempts"][0]["input"]
        self.assertEqual((sent["sent"], sent["delivered"]), (True, True))  # 消息到了
        self.assertNotEqual(sent["hit"], "start")  # 但没点中目标控件
        text = RD.render_report(report)  # 合成夹具的真实形态：定位点相同、落点被 click_offset 移走
        self.assertEqual(report["position"], "point-shifted")
        self.assertIn("定位后输入位置变了", text)
        self.assertIn("不满足期待", text)
        self.assertIn("下一步", text)
        self.assertEqual(report["achieved"], {"ref": True, "cand": False})

    def test_fixed_is_same_and_both_achieved(self):
        report = self.reports[("baseline", "fixed")]
        self.assertEqual((report["verdict"], report["achieved"]), ("same", {"ref": True, "cand": True}))
        with tempfile.TemporaryDirectory() as tmp:
            rc, _ = cli("compare", os.path.join(self.out, "baseline.trace.json"), os.path.join(self.out, "fixed.trace.json"),
                        "--out", os.path.join(tmp, "r.json"))
            self.assertEqual(rc, 0)

    def test_identical_bad_observation_is_not_green_via_cli(self):
        bad = os.path.join(self.out, "wrong-point.trace.json")
        with tempfile.TemporaryDirectory() as tmp:
            rc, text = cli("compare", bad, bad, "--out", os.path.join(tmp, "self.json"))
            self.assertEqual(rc, 1, text)
            self.assertIn("未达标", text)
            with open(os.path.join(tmp, "self.json"), encoding="utf-8") as handle:
                self.assertEqual(json.load(handle)["verdict"], "not-achieved")

    def test_missing_picture_evidence_is_unknown_via_cli(self):
        baseline = self.traces["baseline"]
        stripped = copy.deepcopy(baseline)
        stripped["steps"][1]["attempts"][0]["post"]["picture"] = None
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "stripped.json")
            with open(path, "w", encoding="utf-8") as handle:
                json.dump(stripped, handle)
            rc, text = cli("compare", os.path.join(self.out, "baseline.trace.json"), path)
            self.assertEqual(rc, 1, text)
            self.assertIn("未知", text)

    def test_stale_move_refused_without_sending(self):
        trace = self.traces["stale-move"]
        self.assertEqual(trace["outcome"], "refused")
        last = trace["steps"][1]["attempts"][-1]
        self.assertEqual((last["input"]["sent"], last["input"]["reason_code"]), (False, "stale-state"))
        self.assertIsNone(last["input"]["point"])
        self.assertIsNotNone(last["pre"]["picture"])
        self.assertIsNotNone(trace["post_final"]["state"])
        report = self.reports[("baseline", "stale-move")]
        self.assertEqual((report["verdict"], report["classification"]["fault"]), ("differs", "stale-refusal"))

    def test_pause_resume_keeps_boundary_and_recovers(self):
        trace = self.traces["pause-resume"]
        self.assertEqual(trace["outcome"], "confirmed")
        attempts = trace["steps"][1]["attempts"]
        self.assertEqual((attempts[0]["input"]["discarded"], attempts[0]["input"]["sent"]), ("pause", False))
        self.assertEqual(attempts[1]["post"]["confirmed"], True)
        self.assertEqual(attempts[1]["locate"]["source"], "distilled")  # 暂停恢复后改走蒸馏模板定位
        self.assertEqual(RD.boundary_of(trace)["discarded_attempts"], 1)
        report = self.reports[("baseline", "pause-resume")]
        self.assertEqual((report["verdict"], report["classification"]["fault"]), ("differs", "discarded-by-pause"))

    def test_effect_late_is_distinguished_from_no_effect(self):
        trace = self.traces["effect-late"]
        self.assertEqual(trace["outcome"], "unconfirmed")
        report = self.reports[("baseline", "effect-late")]
        self.assertEqual(report["classification"]["fault"], "effect-late")
        self.assertEqual(trace["post_final"]["state"]["state"], "result")  # 放弃后效果才到
        self.assertEqual(self.reports[("wrong-point", "fixed")]["classification"]["fault"], "post-state-differs")

    def test_synth_run_exit_codes_follow_workflow_not_declaration(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "bad.json")
            rc, text = cli("run", "--scenario", EXAMPLE / "scenario.synth.example.json", "--out", out,
                           "--variant", json.dumps({"candidate": {"click_offset": [0, 60]}, "expect_result": "unconfirmed"}))
            self.assertEqual(rc, 1, text)
            self.assertIn("符合声明(unconfirmed): 是", text)
            rc, _ = cli("run", "--scenario", EXAMPLE / "scenario.synth.example.json", "--out", out)
            self.assertEqual(rc, 2)  # 不覆盖已有观察


@unittest.skipUnless(DEMO_EXE and DEMO_SHA and os.name == "nt", SKIP_DEMO)
class DemoTargetTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.mkdtemp(prefix="wf-demo-")
        cls.out = os.path.join(cls.tmp, "matrix")
        cls.rc, cls.text = cli("matrix", "--plan", EXAMPLE / "plan.example.json", "--out", cls.out,
                               "--exe", DEMO_EXE, "--exe-sha256", DEMO_SHA)
        with open(os.path.join(cls.out, "summary.json"), encoding="utf-8") as handle:
            cls.summary = json.load(handle)
        cls.traces, cls.reports = {}, {}
        for run in cls.summary["runs"]:
            with open(run["trace"], encoding="utf-8") as handle:
                cls.traces[run["id"]] = json.load(handle)
        for compare in cls.summary["compares"]:
            with open(compare["report"], encoding="utf-8") as handle:
                cls.reports[(compare["ref"], compare["cand"])] = json.load(handle)

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmp, ignore_errors=True)

    def test_matrix_matches_declarations(self):
        self.assertEqual(self.rc, 0, self.text)
        self.assertEqual(self.summary["workflow_confirmed"], ["baseline", "fixed", "slow-locate", "pause-resume"])
        self.assertEqual(self.summary["workflow_not_confirmed"], ["wrong-point", "stale-move"])

    def test_window_is_offscreen_not_foreground_and_picture_is_real(self):
        trace = self.traces["baseline"]
        binding = trace["start"]["binding"]
        self.assertEqual((binding["visible"], binding["foreground"], binding["placement"]), (True, False, "offscreen"))
        self.assertEqual(binding["client"], [200, 218])  # --pages 3 的客户区（目标 DPI 下）
        for step in trace["steps"]:
            pre, post = step["attempts"][-1]["pre"], step["attempts"][-1]["post"]
            self.assertTrue(all(v is not None for v in pre["picture"].values()), pre["picture"])
            self.assertEqual(RD.compare_value(pre["picture"], post["picture"], RD.TOL_PICTURE), "diff")
        self.assertEqual(trace["unknown"], [])

    def test_wrong_point_vs_fixed_and_fixed_same(self):
        self.assertEqual(self.reports[("baseline", "wrong-point")]["classification"]["fault"], "delivered-no-effect")
        self.assertEqual(self.reports[("baseline", "fixed")]["verdict"], "same")
        self.assertEqual(self.reports[("wrong-point", "fixed")]["classification"]["fault"], "post-state-differs")

    def test_stale_move_and_pause(self):
        last = self.traces["stale-move"]["steps"][1]["attempts"][-1]
        # 窗口位移先被绑定复核（rect 变了）拒绝；合成目标没有窗口几何，由状态/revision 复核拒绝
        self.assertEqual(last["input"]["sent"], False)
        self.assertIn(last["input"]["reason_code"], ("stale-binding", "stale-state"))
        self.assertIsNone(last["input"]["point"])
        self.assertEqual(self.reports[("baseline", "stale-move")]["classification"]["fault"], "stale-refusal")
        attempts = self.traces["pause-resume"]["steps"][1]["attempts"]
        self.assertEqual(attempts[0]["input"]["discarded"], "pause")
        self.assertEqual(attempts[1]["post"]["confirmed"], True)

    def test_slow_locate_attributed(self):
        report = self.reports[("baseline", "slow-locate")]
        self.assertEqual((report["verdict"], report["classification"]["fault"]), ("same", "slow-only"))
        self.assertTrue(all(s["phase"] == "locate" for s in report["slow"]))

    def test_hidden_placement_leaves_picture_unknown(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "hidden.json")
            rc, text = cli("observe", "--scenario", EXAMPLE / "scenario.example.json", "--out", out, "--placement", "hidden",
                           "--exe", DEMO_EXE, "--exe-sha256", DEMO_SHA)
            self.assertNotEqual(rc, 0, text)
            with open(out, encoding="utf-8") as handle:
                hidden = json.load(handle)
            self.assertFalse(hidden["start"]["binding"]["visible"])
            self.assertIsNone(hidden["start"]["picture"])
            rc, text = cli("compare", os.path.join(self.out, "baseline.trace.json"), out)
            self.assertEqual(rc, 1, text)
            self.assertIn("未知", text)


if __name__ == "__main__":
    unittest.main()
