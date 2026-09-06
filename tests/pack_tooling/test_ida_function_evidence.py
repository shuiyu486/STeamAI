"""仅以 fake IDA 边界调用生产导出器；不启动 IDA、目标或网络。"""

import ast
import copy
import hashlib
import json
from pathlib import Path
import runpy
import stat
import sys
import tempfile
import types
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "packs/binary-re/tooling/scripts/export_function_evidence.py"
SCHEMA = ROOT / "packs/binary-re/tooling/schemas/ida-function-evidence-v1.schema.json"
EXPORT = types.SimpleNamespace(**runpy.run_path(str(SCRIPT), run_name="steamai_ida_evidence"))


class FakeIDA:
    def __init__(self):
        self.initial_state = {
            "input_sha256": "a" * 64, "input_path": "fixture-input",
            "image_base": 0x1000, "processor": "metapc", "is_64bit": True, "is_pe": True,
            "analysis_ready": True, "debugger_active": False, "change_count": 7,
            "ida_version": "fixture-ida", "idapython_version": "fixture-idapython",
        }
        self.state_calls = 0
        self.state_changes = {}
        self.function_record = (0x1000, 0x1002, 0)
        self.chunk_records = [(0x1000, 0x1002)]
        self.head_records = {(0x1000, 0x1002): [0x1000, 0x1001]}
        self.instruction_records = {
            0x1000: (0x1001, "90", "fixture-one"),
            0x1001: (0x1002, "c3", "fixture-two"),
            0x2000: (0x2005, "e800000000", "fixture-call"),
        }
        self.xref_records = {0x2000: [{"to": 0x1000, "type": 17, "is_code": True, "is_call": True}]}
        self.instruction_calls = []
        self.instruction_changed = False
        self.xref_yields = 0

    def state(self):
        self.state_calls += 1
        result = copy.deepcopy(self.initial_state)
        if self.state_calls > 1:
            result.update(self.state_changes)
        return result

    def function(self, ea):
        if ea == 0x2000:
            return (0x2000, 0x2005, 0)
        return self.function_record

    def chunks(self, ea):
        yield from self.chunk_records

    def heads(self, start, end):
        yield from self.head_records.get((start, end), [])

    def instruction(self, ea, max_end):
        self.instruction_calls.append(ea)
        result = self.instruction_records[ea]
        if self.instruction_changed and len(self.instruction_calls) > 2:
            return result[0], result[1], "changed"
        if result[0] > max_end:
            raise EXPORT.EvidenceError("指令越界")
        return result

    def references(self, ea):
        for reference in self.xref_records.get(ea, []):
            self.xref_yields += 1
            yield reference


class ExportTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.case = Path(self.temporary.name)
        self.artifacts = self.case / ".steamai-vnext" / "artifacts"
        self.artifacts.mkdir(parents=True)
        self.api = FakeIDA()
        self.request = {
            "case_root": str(self.case), "output_name": "function.json",
            "input_alias": "input-a", "database_alias": "database-a",
            "expected_input_sha256": "a" * 64, "expected_image_base": 0x1000,
            "function_ea": 0x1000, "allowed_ranges": [(0x1000, 0x1002)],
            "_api": self.api,
        }

    def export(self, **changes):
        return EXPORT.export_function_evidence(**(self.request | changes))

    def assert_rejected(self, **changes):
        with self.assertRaises(EXPORT.EvidenceError):
            self.export(**changes)
        self.assertEqual(list(self.artifacts.iterdir()), [])

    def test_success_direct_production_and_receipt(self):
        receipt = self.export()
        data = Path(receipt["path"]).read_bytes()
        result = json.loads(data)
        self.assertEqual(receipt["sha256"], hashlib.sha256(data).hexdigest())
        self.assertEqual(receipt["bytes"], len(data))
        self.assertEqual(self.api.state_calls, 2)
        self.assertEqual(self.api.instruction_calls, [0x1000, 0x1001] * 2)
        self.assertEqual(result["limits"]["entries"], 4)
        self.assertEqual(result["observation"]["instructions"][0]["bytes"], "90")
        self.assertNotIn("input_path", result["source"])
        self.assertFalse(result["source"]["original_input_rehashed"])
        self.assertFalse(result["source"]["database_file_hashed"])

    def test_schema_keys_and_limits_match_producer(self):
        result = json.loads(Path(self.export()["path"]).read_bytes())
        schema = json.loads(SCHEMA.read_text(encoding="utf-8"))
        # 检查本 producer 的 exact shape，不假装实现通用 JSON Schema validator。
        for name, value in [(None, result)] + [(name, result[name]) for name in ("source", "selection", "limits", "observation")]:
            definition = schema if name is None else schema["properties"][name]
            self.assertEqual(set(value), set(definition["required"]))
            self.assertEqual(set(value), set(definition["properties"]))
            self.assertFalse(definition["additionalProperties"])
        for name, expected in (("max_entries", EXPORT.MAX_ENTRIES), ("max_output_bytes", EXPORT.MAX_OUTPUT_BYTES), ("timeout_seconds", EXPORT.MAX_SECONDS)):
            self.assertEqual(schema["properties"]["limits"]["properties"][name]["maximum"], expected)
        for name in ("instructions", "references", "chunks"):
            definition = schema["properties"]["observation"]["properties"][name]["items"]
            if "$ref" in definition:
                definition = schema["$defs"][definition["$ref"].split("/")[-1]]
            for item in result["observation"][name]:
                self.assertEqual(set(item), set(definition["required"]))

    def test_no_overwrite(self):
        output = self.artifacts / "function.json"
        output.write_bytes(b"existing")
        with self.assertRaises(EXPORT.EvidenceError):
            self.export()
        self.assertEqual(output.read_bytes(), b"existing")
        self.assertEqual(self.api.state_calls, 0)

    def test_output_name_and_input_validation_before_ida(self):
        for name in ("../escape.json", "a/b.json", "A.json", "con.json", "x.json:ads", "a\\b.json", "a.json.", "x.txt"):
            with self.subTest(name=name):
                self.assert_rejected(output_name=name)
        for changes in ({"function_ea": True}, {"callsites": [1, 2, 3]}, {"callsites": [1, 1]},
                        {"input_alias": "path/value"}, {"expected_input_sha256": "bad"},
                        {"max_entries": 1001}, {"max_output_bytes": 262145}, {"timeout_seconds": 31},
                        {"allowed_ranges": [(1, 4), (2, 5)]}, {"allowed_ranges": [(5, 4)]},
                        {"allowed_ranges": []}, {"allowed_ranges": [(0, 8)]}):
            with self.subTest(changes=changes):
                self.assert_rejected(**changes)
        self.assertEqual(self.api.state_calls, 0)

    def test_path_rejections(self):
        for root in ("relative", str(self.case / ".." / self.case.name), str(self.case) + ":ads", "//server/share"):
            with self.subTest(root=root):
                self.assert_rejected(case_root=root)
        self.artifacts.rmdir()
        with self.assertRaises(EXPORT.EvidenceError):
            self.export()
        self.assertFalse(self.artifacts.exists())

    def test_reparse_rejected(self):
        original = Path.lstat
        artifact_dir = self.artifacts

        def lookup(path):
            info = original(path)
            if path == artifact_dir:
                return types.SimpleNamespace(st_mode=stat.S_IFDIR, st_file_attributes=0x400, st_dev=info.st_dev, st_ino=info.st_ino)
            return info

        with patch.object(Path, "lstat", lookup):
            self.assert_rejected()

    def test_symlink_directory_rejected(self):
        target = self.case / "target"
        target.mkdir()
        link = self.case / "linked"
        try:
            link.symlink_to(target, target_is_directory=True)
        except OSError:
            self.skipTest("本机无创建 symlink 权限；reparse 替身仍覆盖拒绝分支")
        self.assert_rejected(case_root=str(link))

    def test_supported_database_state(self):
        for key, value in (("input_sha256", "b" * 64), ("image_base", 0), ("processor", "ARM"),
                           ("is_64bit", False), ("is_pe", False), ("analysis_ready", False),
                           ("debugger_active", True), ("change_count", None), ("ida_version", "")):
            with self.subTest(key=key):
                api = FakeIDA()
                api.initial_state[key] = value
                self.assert_rejected(_api=api)

    def test_missing_function_or_interior_address(self):
        self.api.function_record = None
        self.assert_rejected()
        self.api.function_record = (0x0FFF, 0x1002, 0)
        self.assert_rejected()

    def test_chunk_boundary_rejected_before_instruction_read(self):
        self.api.chunk_records.append((0x3000, 0x3001))
        self.assert_rejected()
        self.assertEqual(self.api.instruction_calls, [])

    def test_chunks_must_be_complete_disjoint_and_instruction_covered(self):
        for chunks, heads in (([], {}), ([(0x1000, 0x1002)] * 2, {}),
                              ([(0x1000, 0x1002)], {(0x1000, 0x1002): [0x1000]}),
                              ([(0x1000, 0x1002)], {(0x1000, 0x1002): [0x1001]})):
            with self.subTest(chunks=chunks, heads=heads):
                api = FakeIDA()
                api.chunk_records = chunks
                api.head_records = heads
                self.assert_rejected(_api=api)

    def test_instruction_end_and_bytes_checked(self):
        self.api.instruction_records[0x1001] = (0x1003, "9090", "bad")
        self.assert_rejected()
        self.api.instruction_records[0x1001] = (0x1002, "9090", "bad")
        self.assert_rejected()

    def test_external_callsite_and_xref(self):
        receipt = self.export(allowed_ranges=[(0x1000, 0x1002), (0x2000, 0x2005)], callsites=[0x2000])
        result = json.loads(Path(receipt["path"]).read_bytes())
        self.assertTrue(result["observation"]["instructions"][-1]["callsite"])
        self.assertTrue(result["observation"]["references"][-1]["is_call"])
        self.assertEqual(result["limits"]["entries"], 6)

    def test_invalid_callsite_or_non_call_reference(self):
        self.assert_rejected(callsites=[0x1000])
        self.assert_rejected(callsites=[0x2000])
        self.api.xref_records[0x2000][0]["is_call"] = False
        self.assert_rejected(allowed_ranges=[(0x1000, 0x1002), (0x2000, 0x2005)], callsites=[0x2000])

    def test_entry_cap_stops_generator_not_full_enumeration(self):
        reference = {"to": 0x1001, "type": 21, "is_code": True, "is_call": False}
        self.api.xref_records[0x1000] = (reference for _ in range(1000000))
        self.assert_rejected(max_entries=8)
        self.assertLessEqual(self.api.xref_yields, 6)

    def test_output_budget_no_partial_success(self):
        self.assert_rejected(max_output_bytes=1)

    def test_deadline_stops_before_write(self):
        ticks = iter([0, 0, 0, 31])
        self.assert_rejected(_clock=lambda: next(ticks))

    def test_state_and_selected_observation_drift(self):
        for change in ({"change_count": 8}, {"input_path": "different-fixture"}, {"ida_version": "changed"}):
            self.api = FakeIDA()
            self.api.state_changes = change
            self.assert_rejected(_api=self.api)
        self.api = FakeIDA()
        self.api.instruction_changed = True
        self.assert_rejected(_api=self.api)

    def test_unknown_api_fails_without_output(self):
        self.assert_rejected(_api=object())

    def test_output_directory_drift_before_publication(self):
        actual = EXPORT._output_path
        calls = 0

        def changed(*args):
            nonlocal calls
            calls += 1
            output, identity = actual(*args)
            return output, (identity if calls == 1 else (identity[0], identity[1] + 1))

        with patch.dict(EXPORT.export_function_evidence.__globals__, {"_output_path": changed}):
            self.assert_rejected()

    def test_competing_create_is_never_overwritten(self):
        actual = Path.open
        output = self.artifacts / "function.json"

        def create_first(path, mode="r", *args, **kwargs):
            if path == output and mode == "xb":
                with actual(path, "wb") as stream:
                    stream.write(b"other writer")
            return actual(path, mode, *args, **kwargs)

        with patch.object(Path, "open", create_first):
            with self.assertRaises(EXPORT.EvidenceError):
                self.export()
        self.assertEqual(output.read_bytes(), b"other writer")

    def test_write_failure_reports_exact_residue_and_keeps_file(self):
        with patch.object(EXPORT.os, "fsync", side_effect=OSError("fixture failure")):
            with self.assertRaisesRegex(EXPORT.EvidenceError, "不完整文件") as error:
                self.export()
        self.assertIn(str(self.artifacts / "function.json"), str(error.exception))
        self.assertTrue((self.artifacts / "function.json").exists())

    def test_cli_help_does_not_load_ida_and_invalid_request_is_nonzero(self):
        def forbidden_ida():
            raise AssertionError("加载IDA")

        with patch.dict(EXPORT.export_function_evidence.__globals__, {"_IDA": forbidden_ida}):
            with patch.object(EXPORT.sys, "stdout"):
                with self.assertRaises(SystemExit) as stopped:
                    EXPORT.main(["--help"])
            self.assertEqual(stopped.exception.code, 0)
            with patch.object(EXPORT.sys, "stderr"):
                code = EXPORT.main(["--case-root", str(self.case), "--output-name", "bad.json:ads",
                                    "--input-alias", "input-a", "--database-alias", "database-a",
                                    "--expected-input-sha256", "a" * 64, "--expected-image-base", "0x1000",
                                    "--function-ea", "0x1000", "--range", "0x1000", "0x1002"])
            self.assertEqual(code, 1)
        self.assertEqual(list(self.artifacts.iterdir()), [])

    def test_recipe_loading_and_export_preserve_pinned_source_tree(self):
        recipe = ROOT / "packs/binary-re/tooling/recipes/ida-function-evidence.md"
        snippet = recipe.read_text(encoding="utf-8").split("```python\n", 1)[1].split("```", 1)[0]
        tree = ast.parse(snippet)
        self.assertIsInstance(tree.body[-1], ast.Assign)
        self.assertEqual(tree.body[-1].targets[0].id, "status")
        pinned = self.case / "pinned-scripts"
        pinned.mkdir()
        source = pinned / SCRIPT.name
        source.write_bytes(SCRIPT.read_bytes())
        before = {str(path.relative_to(pinned)): path.read_bytes() for path in pinned.rglob("*") if path.is_file()}
        variables = {"PINNED_SCRIPT": str(source), "CASE_ROOT": str(self.case), "INPUT_SHA256": "a" * 64,
                     "IMAGE_BASE": "0x1000", "FUNCTION_EA": "0x1000",
                     "FUNCTION_RANGE_START": "0x1000", "FUNCTION_RANGE_END": "0x1002"}
        # 强制测试默认可写 bytecode 的环境，不能用全局禁用掩盖 recipe 缺陷。
        with patch.object(sys, "dont_write_bytecode", False):
            exec(compile(ast.Module(body=tree.body[:-1], type_ignores=[]), str(recipe), "exec"), variables)
            exporter = variables["exporter"]
            main = exporter["main"]
            with patch.dict(main.__globals__, {"_IDA": lambda: self.api}), patch.object(EXPORT.sys, "stdout"):
                exec(compile(ast.Module(body=tree.body[-1:], type_ignores=[]), str(recipe), "exec"), variables)
        self.assertEqual(variables["status"], 0)
        self.assertTrue((self.artifacts / "function-check.json").is_file())
        after = {str(path.relative_to(pinned)): path.read_bytes() for path in pinned.rglob("*") if path.is_file()}
        self.assertEqual(set(before), set(after), "文档实际加载路径增加了 pinned source 文件")
        for name, data in before.items():
            self.assertEqual(data, after[name], "文档实际加载路径改写了 pinned source bytes")
        self.assertFalse((pinned / "__pycache__").exists())

    def test_import_has_no_ida_or_filesystem_side_effect(self):
        with patch.object(Path, "open", side_effect=AssertionError("import写文件")), patch.object(EXPORT.os, "lstat", side_effect=AssertionError("import访问文件")):
            runpy.run_path(str(SCRIPT), run_name="steamai_ida_evidence")
        tree = ast.parse(SCRIPT.read_text(encoding="utf-8"))
        forbidden = {"patch_byte", "patch_bytes", "auto_wait", "auto_mark", "save_database", "set_name",
                     "set_cmt", "set_type", "create_insn", "create_data", "start_process", "decompile"}
        attributes = {node.attr for node in ast.walk(tree) if isinstance(node, ast.Attribute)}
        self.assertFalse(forbidden & attributes)


class IDABoundaryTests(unittest.TestCase):
    def test_loaded_state_uses_read_only_identity_queries(self):
        api = EXPORT._IDA.__new__(EXPORT._IDA)
        api.python_version = "fixture-idapython"
        api.nalt = types.SimpleNamespace(retrieve_input_file_sha256=lambda: bytes.fromhex("a" * 64),
                                         get_input_file_path=lambda: "fixture-input", get_imagebase=lambda: 0x1000)
        api.info = types.SimpleNamespace(inf_get_procname=lambda: "metapc", inf_is_64bit=lambda: True,
                                         inf_get_filetype=lambda: 11, f_PE=11,
                                         inf_get_database_change_count=lambda: 7)
        api.auto = types.SimpleNamespace(auto_is_ok=lambda: True, get_auto_state=lambda: 0, AU_NONE=0)
        api.dbg = types.SimpleNamespace(is_debugger_on=lambda: False)
        api.kernel = types.SimpleNamespace(get_kernel_version=lambda: "fixture-ida")
        self.assertEqual(api.state(), FakeIDA().initial_state)
        api.nalt.retrieve_input_file_sha256 = lambda: None
        with self.assertRaises(EXPORT.EvidenceError):
            api.state()

    def test_read_adapter_explicit_database_bytes_and_patch_detection(self):
        api = EXPORT._IDA.__new__(EXPORT._IDA)
        reads = []
        api.byte = types.SimpleNamespace(get_full_flags=lambda ea: 1, is_code=lambda flags: True,
                                         get_item_end=lambda ea: ea + 2, get_original_byte=lambda ea: 0x90)
        api.idc = types.SimpleNamespace(get_bytes=lambda ea, count, use_dbg: reads.append((ea, count, use_dbg)) or b"\x90\x90",
                                       generate_disasm_line=lambda ea, flags: "fixture")
        self.assertEqual(api.instruction(10, 12), (12, "9090", "fixture"))
        self.assertEqual(reads, [(10, 2, False)])
        reads.clear()
        with self.assertRaises(EXPORT.EvidenceError):
            api.instruction(10, 11)
        self.assertEqual(reads, [])
        api.byte.get_original_byte = lambda ea: 0xCC
        with self.assertRaisesRegex(EXPORT.EvidenceError, "patched"):
            api.instruction(10, 12)


if __name__ == "__main__":
    unittest.main()
