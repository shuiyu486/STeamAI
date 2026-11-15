"""Mechanical contracts; real CHM smoke is an explicit, local Windows check."""
import argparse
import copy
import importlib.util
import json
import os
from pathlib import Path
import re
import struct
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "packs/game-automation/tooling/scripts/dm_docs.py"
PINNED = SCRIPT.with_suffix(".selection.json")
UPSTREAM = ROOT / "packs/game-automation/upstream/dm-7.2520"
# 加载生产脚本时禁写字节码，避免在 pack 目录留下 __pycache__（Go contract 拒绝该产物）。
with patch.object(sys, "dont_write_bytecode", True):
    SPEC = importlib.util.spec_from_file_location("dm_docs", SCRIPT)
    DM = importlib.util.module_from_spec(SPEC)
    SPEC.loader.exec_module(DM)


def html(name, text="ordinary text", charset="utf-8"):
    return ('<html><head><meta charset="' + charset + '"></head><body>'
            '<p>功能简介:</p><p>' + text + '</p><p>函数原型:</p><p>long ' + name +
            '(x)</p><p>参数定义:</p><p>x: coordinate</p><p>返回值:</p><p>1</p>'
            '<p>示例:</p><p>DO_NOT_EXPORT_EXAMPLE()</p></body></html>').encode(
                "gb18030" if charset == "gb2312" else charset)


def compressed_word(value):
    parts = [value & 127]
    value >>= 7
    while value:
        parts.append((value & 127) | 128)
        value >>= 7
    return bytes(reversed(parts))


def chm_directory(names):
    """Minimal uncompressed index fixture, never passed to HTML Help."""
    payload = b""
    for name in names:
        raw = name if isinstance(name, bytes) else name.encode("utf-8")
        payload += compressed_word(len(raw)) + raw + b"\0\0\0"
    block_size = 4096
    block = bytearray(block_size)
    block[:4] = b"PMGL"
    struct.pack_into("<I", block, 4, block_size - 20 - len(payload))
    block[20:20 + len(payload)] = payload
    struct.pack_into("<H", block, block_size - 2, len(names))
    directory = bytearray(84)
    directory[:4] = b"ITSP"
    struct.pack_into("<II", directory, 4, 1, 84)
    struct.pack_into("<I", directory, 16, block_size)
    struct.pack_into("<I", directory, 44, 1)
    header = bytearray(96)
    header[:4] = b"ITSF"
    struct.pack_into("<II", header, 4, 3, 96)
    struct.pack_into("<QQ", header, 72, 96, len(directory) + len(block))
    return bytes(header + directory + block)


class SelectionTests(unittest.TestCase):
    def test_pinned_selection_matches_source_and_existing_bytes(self):
        selection = DM.load_selection(PINNED)
        files = DM.reviewed_files(selection, UPSTREAM)
        source = (UPSTREAM / "SOURCE.md").read_text("utf-8")
        rows = re.findall(r"\| `([^`]+\.md)` \| `([^`]+)` \| `([0-9a-f]{64})` \| (\d+) \| `([0-9a-f]{64})` \|", source)
        expected = {name: (path, source_hash, int(size), output_hash)
                    for name, path, source_hash, size, output_hash in rows}
        self.assertTrue(expected)
        self.assertEqual(set(expected), set(files))
        for page in selection["pages"]:
            self.assertEqual(expected[page["output"]],
                             (page["source"], page["source_sha256"], len(files[page["output"]]),
                              page["output_sha256"]))
        self.assertIn(selection["chm_sha256"], source)
        self.assertIn(str(selection["chm_bytes"]), source)

    def test_selection_rejects_duplicate_unknown_and_escaping_entries(self):
        original = DM.load_selection(PINNED)
        bad = []
        value = copy.deepcopy(original)
        value["pages"].append(copy.deepcopy(value["pages"][0]))
        bad.append(value)
        for key, replacement in (("source", "../outside.htm"), ("source", "图色/BindWindow.htm"),
                                 ("output", "../BindWindow.md"), ("output", "DmGuard.md"),
                                 ("source_sha256", "bad"), ("review", "accepted")):
            value = copy.deepcopy(original)
            value["pages"][0][key] = replacement
            bad.append(value)
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "selection.json"
            for value in bad:
                with self.subTest(value=value["pages"][0]):
                    path.write_bytes(DM.json_bytes(value))
                    with self.assertRaises(DM.DocsError):
                        DM.load_selection(path)
            for raw in (b'{"schema":1,"schema":1}', b'\xff', b'\xef\xbb\xbf{}'):
                path.write_bytes(raw)
                with self.assertRaises(DM.DocsError):
                    DM.load_selection(path)

    def test_reviewed_excerpt_is_byte_bound_and_strict_utf8_lf(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            value = DM.load_selection(PINNED)
            value["pages"] = [value["pages"][0]]
            name = value["pages"][0]["output"]
            for raw in (b"wrong hash", b"bad\r\n", b"\xff", b"\xef\xbb\xbftext", b"\0"):
                (root / name).write_bytes(raw)
                if raw != b"wrong hash":
                    value["pages"][0]["output_sha256"] = DM.sha(raw)
                with self.assertRaises(DM.DocsError):
                    DM.reviewed_files(value, root)


class ParserTests(unittest.TestCase):
    def test_plain_text_omits_html_active_content_and_examples(self):
        raw = html("Capture", "保留文本", "gb2312").replace(
            b"<body>", b"<body><script>NO_SCRIPT</script><style>NO_STYLE</style>"
            b"<object>NO_OBJECT</object><iframe>NO_IFRAME</iframe>")
        result = DM.ordinary_text(raw, "Capture").decode("utf-8")
        self.assertIn("保留文本", result)
        self.assertIn("long Capture(x)", result)
        for word in ("NO_SCRIPT", "NO_STYLE", "NO_OBJECT", "NO_IFRAME", "DO_NOT_EXPORT", "<p>"):
            self.assertNotIn(word, result)
        self.assertEqual(DM.body_lines(raw)[1], "gb18030")

    def test_strict_decoding_and_body_shape_fail_closed(self):
        cases = [html("Capture").replace(b"ordinary text", b"\xff"),
                 html("Capture").replace(b'charset="utf-8"', b'charset="bogus"'),
                 html("Capture").replace(b'charset="utf-8"', b'charset="utf-8" charset="gb2312"'),
                 html("Capture").replace(b'charset="utf-8"', b""),
                 html("Capture").replace(b"</body>", b""),
                 html("Capture").replace(b"ordinary text", b"\0"),
                 html("Capture").replace(b"<body>", b"<body><body>"),
                 html("Capture").replace(b"ordinary text", "\ufffd".encode())]
        for raw in cases:
            with self.subTest(raw=raw[:80]), self.assertRaises(DM.DocsError):
                DM.body_lines(raw)
        with self.assertRaises(DM.DocsError):
            DM.ordinary_text(html("BindWindow"), "BindWindow")
        with self.assertRaises(DM.DocsError):
            DM.ordinary_text(html("FindPic"), "Capture")

    def test_index_validates_names_before_decompression(self):
        self.assertEqual(DM.chm_index(chm_directory(["/", "/图色/Capture.htm", "/图色/", "/#SYSTEM",
                                                     "::DataSpace/Storage/MSCompressed/Content"])),
                         {"图色/Capture.htm"})
        cases = [["/../escape.htm"], ["/C:/escape.htm"], ["/\\escape.htm"],
                 ["/dir/file.htm:stream"], ["/dir./file.htm"], ["/NUL.htm"],
                 ["/a.htm", "/a.htm"], ["/a.htm", "/A.htm"],
                 ["/Dir/a.htm", "/dir/b.htm"], ["/dir", "/dir/file.htm"],
                 ["/é.htm", "/e\u0301.htm"], [b"/\xff.htm"], ["relative.htm"]]
        for names in cases:
            with self.subTest(names=names), self.assertRaises(DM.DocsError):
                DM.chm_index(chm_directory(names))
        raw = chm_directory(["/a.htm"])
        for bad in (raw[:90], raw[:150], b"fake", raw[:4] + b"\2" + raw[5:]):
            with self.assertRaises(DM.DocsError):
                DM.chm_index(bad)


@unittest.skipUnless(os.name == "nt", "Windows publish/launcher contract")
class WorkflowTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.reviewed = self.root / "reviewed"
        self.reviewed.mkdir()
        self.old = {DM.ALLOWED[name]: html(name) for name in ("Capture", "BindWindow", "Ver", "GetPath")}
        self.old_data = chm_directory(["/" + source for source in self.old])
        self.selection = {"schema": 1, "sdk_label": "7.0", "chm_sha256": DM.sha(self.old_data),
                          "chm_bytes": len(self.old_data), "pages": []}
        for source, raw in self.old.items():
            name = Path(source).stem + ".md"
            excerpt = ("# " + name + "\n\nREVIEWED_EXCERPT_ONLY\n").encode()
            (self.reviewed / name).write_bytes(excerpt)
            self.selection["pages"].append({"output": name, "source": source, "source_sha256": DM.sha(raw),
                                           "output_sha256": DM.sha(excerpt), "review": "reviewed"})
        self.selection_path = self.root / "selection.json"
        self.selection_path.write_bytes(DM.json_bytes(self.selection))
        self.baseline = self.root / "baseline.chm"
        self.baseline.write_bytes(self.old_data)
        self.new_chm = self.root / "candidate.chm"
        self.args = argparse.Namespace(selection=self.selection_path, reviewed_dir=self.reviewed,
                                       output=self.root / "output", baseline_chm=self.baseline,
                                       chm=self.new_chm, sdk_label="7.1", add_interface=None)

    def run_compare(self, pages):
        # Only OS decompression is substituted. Hash/index/parser/status/write code is production.
        self.new_chm.write_bytes(chm_directory(["/" + source for source in pages]))
        with patch.object(DM, "extract_selected", side_effect=[self.old, pages]):
            return DM.compare(self.args)

    def test_changed_missing_added_never_reuse_old_lines(self):
        new = {source: raw for source, raw in self.old.items() if not source.endswith("GetPath.htm")}
        new[DM.ORDINARY["Capture"]] = html("Capture", "changed ordinary description")
        new[DM.RESTRICTED["BindWindow"]] = html("BindWindow", "RESTRICTED_CONTENT_NOT_EXPORTED")
        new[DM.ORDINARY["GetCursorPos"]] = html("GetCursorPos", "new ordinary description")
        self.args.add_interface = ["GetCursorPos"]
        self.assertEqual(self.run_compare(new), {"unchanged": 1, "changed": 2, "missing": 1, "added": 1})
        output = self.args.output
        report = json.loads((output / "report.json").read_text("utf-8"))
        next_selection = DM.load_selection(output / "selection.json")
        self.assertEqual((output / "reviewed/Ver.md").read_bytes(), (self.reviewed / "Ver.md").read_bytes())
        self.assertFalse((output / "reviewed/Capture.md").exists())
        self.assertFalse((output / "reviewed/BindWindow.md").exists())
        self.assertFalse((output / "review/BindWindow.txt").exists())
        self.assertIn("+changed ordinary description", (output / "review/Capture.diff.txt").read_text("utf-8"))
        self.assertIn("long GetCursorPos", (output / "review/GetCursorPos.txt").read_text("utf-8"))
        for page in next_selection["pages"]:
            if page["output"] != "Ver.md":
                self.assertIsNone(page["output_sha256"])
                self.assertNotEqual(page["review"], "reviewed")
        for name, expected in report["outputs"].items():
            self.assertEqual(DM.sha((output / name).read_bytes()), expected)
        for path in output.rglob("*"):
            if path.is_file():
                raw = path.read_bytes()
                self.assertNotIn(b"RESTRICTED_CONTENT_NOT_EXPORTED", raw)
                self.assertNotIn(str(self.root).encode(), raw)
        with self.assertRaises(DM.DocsError):
            DM.reviewed_files(next_selection, output / "reviewed")

    def test_reproducible_compare_import_and_next_version(self):
        self.assertEqual(self.run_compare(self.old)["unchanged"], len(self.old))
        first = self.args.output
        self.args.output = self.root / "repeat"
        self.run_compare(self.old)
        first_files = {p.relative_to(first): p.read_bytes() for p in first.rglob("*") if p.is_file()}
        second_files = {p.relative_to(self.args.output): p.read_bytes() for p in self.args.output.rglob("*") if p.is_file()}
        self.assertEqual(first_files, second_files)
        target = self.root / "imported"
        args = argparse.Namespace(selection=first / "selection.json", reviewed_dir=first / "reviewed",
                                  chm=self.new_chm, output=target)
        with patch.object(DM, "extract_selected", return_value=self.old):
            self.assertEqual(DM.import_reviewed(args)["imported"], len(self.old))
        self.assertEqual(DM.load_selection(target / "selection.json")["sdk_label"], "7.1")
        for page in self.selection["pages"]:
            self.assertEqual((target / page["output"]).read_bytes(), (self.reviewed / page["output"]).read_bytes())
        source = (target / "SOURCE.md").read_text("utf-8")
        self.assertIn(DM.sha(self.new_chm.read_bytes()), source)
        self.assertNotIn(str(self.root), source)
        with self.assertRaises(DM.DocsError):
            DM.import_reviewed(args)
        self.args.selection = target / "selection.json"
        self.args.reviewed_dir = target
        self.args.baseline_chm = self.new_chm
        self.args.sdk_label = "7.2"
        self.args.output = self.root / "next-version"
        self.assertEqual(self.run_compare(self.old)["unchanged"], len(self.old))

    def test_edited_review_is_required_before_import(self):
        new = dict(self.old)
        new[DM.ORDINARY["Capture"]] = html("Capture", "new description")
        self.run_compare(new)
        output = self.args.output
        args = argparse.Namespace(selection=output / "selection.json", reviewed_dir=output / "reviewed",
                                  chm=self.new_chm, output=self.root / "imported")
        with self.assertRaises(DM.DocsError):
            DM.import_reviewed(args)
        self.assertFalse(args.output.exists())
        selected = DM.load_selection(args.selection)
        page = next(page for page in selected["pages"] if page["output"] == "Capture.md")
        raw = b"# Capture\n\nNew manually reviewed excerpt.\n"
        (args.reviewed_dir / "Capture.md").write_bytes(raw)
        page.update(output_sha256=DM.sha(raw), review="reviewed")
        args.selection.write_bytes(DM.json_bytes(selected))
        with patch.object(DM, "extract_selected", return_value=new):
            DM.import_reviewed(args)
        self.assertEqual((args.output / "Capture.md").read_bytes(), raw)

    def test_wrong_hash_decoding_or_source_aborts_without_output(self):
        for mutate in ("baseline", "raw", "decode"):
            with self.subTest(mutate=mutate):
                old = dict(self.old)
                new = dict(self.old)
                if mutate == "baseline":
                    self.baseline.write_bytes(chm_directory(["/different.htm"]))
                elif mutate == "raw":
                    old[DM.ORDINARY["Capture"]] = html("Capture", "drift")
                else:
                    new[DM.ORDINARY["Capture"]] = html("Capture").replace(b"ordinary text", b"\xff")
                self.new_chm.write_bytes(self.old_data)
                with patch.object(DM, "extract_selected", side_effect=[old, new]), self.assertRaises(DM.DocsError):
                    DM.compare(self.args)
                self.assertFalse(self.args.output.exists())
                self.baseline.write_bytes(self.old_data)

    def test_output_must_be_new_outside_case_and_reviewed_material(self):
        case = self.root / "case"
        case.mkdir()
        (case / ".steamai-vnext").mkdir()
        nested = case / "nested"
        nested.mkdir()
        for output in (self.reviewed, self.reviewed / "new", case / "new", nested / "new",
                       self.root / "absent-parent/new", self.root / ".." / "new", self.root / "file:stream"):
            with self.subTest(output=output), self.assertRaises(DM.DocsError):
                DM.check_output(output, self.reviewed)
        target = self.root / "existing"
        target.mkdir()
        marker = target / "keep.txt"
        marker.write_bytes(b"do not touch")
        with self.assertRaises(DM.DocsError):
            DM.publish(target, {"new.txt": b"bad"})
        self.assertEqual(marker.read_bytes(), b"do not touch")
        self.assertFalse((target / "new.txt").exists())

    def test_junction_input_output_and_symlink_are_rejected(self):
        link = self.root / "junction"
        # A junction does not need SeCreateSymbolicLinkPrivilege on Windows.
        subprocess.run([os.environ.get("COMSPEC", "cmd.exe"), "/d", "/c", "mklink", "/J",
                        str(link), str(self.reviewed)], check=True, capture_output=True,
                       creationflags=subprocess.CREATE_NO_WINDOW)
        try:
            for path, kind in ((link, "dir"), (link / "Ver.md", "file"), (link / "output", "new")):
                with self.subTest(path=path), self.assertRaises(DM.DocsError):
                    DM.plain_path(path, kind)
        finally:
            link.rmdir()
        symlink = self.root / "symlink"
        try:
            symlink.symlink_to(self.reviewed, target_is_directory=True)
        except OSError as exc:
            if getattr(exc, "winerror", None) != 1314:
                raise
            return  # Real junction coverage above is mandatory even without symlink privilege.
        try:
            with self.assertRaises(DM.DocsError):
                DM.plain_path(symlink / "Ver.md", "file")
        finally:
            symlink.unlink()

    def test_publish_race_does_not_replace_a_new_target(self):
        output = self.root / "raced"
        original = DM.os.rename

        def race(source, destination):
            output.mkdir()
            (output / "owner.txt").write_bytes(b"other writer")
            return original(source, destination)

        with patch.object(DM.os, "rename", side_effect=race), self.assertRaises(OSError):
            DM.publish(output, {"file.txt": b"ours"})
        self.assertEqual((output / "owner.txt").read_bytes(), b"other writer")
        self.assertFalse((output / "file.txt").exists())
        self.assertFalse(list(self.root.glob(".dm-docs-*")))

    def test_cli_rejects_missing_input_without_publishing(self):
        output = self.root / "cli-output"
        result = subprocess.run([sys.executable, "-B", str(SCRIPT), "compare", "--baseline-chm", str(self.baseline),
                                 "--chm", str(self.new_chm), "--sdk-label", "7.1",
                                 "--selection", str(self.selection_path), "--reviewed-dir", str(self.reviewed),
                                 "--output", str(output)], capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 2)
        self.assertIn("dm_docs:", result.stderr)
        self.assertFalse(output.exists())
        self.assertNotIn(str(self.root), result.stderr)

    def test_decompile_is_hidden_extract_only_and_temporary(self):
        source = DM.ORDINARY["Capture"]
        raw = html("Capture")
        work = []

        def decompile(command, **kwargs):
            self.assertEqual(Path(command[0]).name, "hh.exe")
            self.assertEqual(command[1], "-decompile")
            self.assertEqual(len(command), 4)
            self.assertNotIn("shell", kwargs)
            self.assertEqual(kwargs["creationflags"], subprocess.CREATE_NO_WINDOW)
            self.assertEqual(kwargs["startupinfo"].wShowWindow, 0)
            self.assertEqual(kwargs["stdin"], subprocess.DEVNULL)
            self.assertEqual(kwargs["timeout"], 60)
            extracted = Path(command[2])
            work.append(extracted.parent)
            self.assertEqual(Path(command[3]).read_bytes(), self.old_data)
            path = extracted / source
            path.parent.mkdir()
            path.write_bytes(raw)

        with patch.object(DM.subprocess, "run", side_effect=decompile):
            self.assertEqual(DM.extract_selected(self.old_data, {source}, [source]), {source: raw})
        self.assertFalse(work[0].exists())
        with patch.object(DM.subprocess, "run", side_effect=subprocess.TimeoutExpired("hh", 60)), self.assertRaises(DM.DocsError):
            DM.extract_selected(self.old_data, {source}, [source])


if __name__ == "__main__":
    unittest.main()
