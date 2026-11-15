#!/usr/bin/env python3
"""Offline, selected DM CHM comparison/import; never loads the SDK or runs HTML."""
import argparse
import ctypes
import difflib
import hashlib
from html.parser import HTMLParser
import json
import os
from pathlib import Path
import re
import shutil
import stat
import struct
import subprocess
import sys
import tempfile
import unicodedata


class DocsError(ValueError):
    pass


# This is a bounded text-export surface, not an inventory of all SDK features.
# New ordinary interfaces need an explicit review of this allowlist, not a wildcard.
ORDINARY = {
    **{name: "图色/" + name + ".htm" for name in (
        "Capture", "CmpColor", "FindColor", "FindPic", "FindColorEx", "FindPicEx",
        "GetColor", "GetColorNum")},
    **{name: "窗口/" + name + ".htm" for name in (
        "ClientToScreen", "EnumWindow", "FindWindow", "GetClientSize", "GetWindowClass",
        "GetWindowProcessId", "GetWindowTitle", "ScreenToClient", "FindWindowEx")},
    **{name: "键鼠/" + name + ".htm" for name in (
        "KeyDown", "KeyPress", "KeyUp", "LeftClick", "MoveTo", "RightClick",
        "SetKeypadDelay", "SetMouseDelay", "GetCursorPos")},
    **{name: "文字识别/" + name + ".htm" for name in (
        "EnableShareDict", "FindStr", "FindStrEx", "FindStrFast", "GetNowDict",
        "Ocr", "OcrEx", "SetDict", "UseDict", "FindStrFastEx")},
    **{name: "基本设置/" + name + ".htm" for name in (
        "GetBasePath", "GetDmCount", "GetID", "GetPath", "SetPath", "Ver")},
    **{name: "后台设置/" + name + ".htm" for name in (
        "GetBindWindow", "IsBind", "UnBindWindow")},
}
RESTRICTED = {
    "BindWindow": "后台设置/BindWindow.htm",
    "BindWindowEx": "后台设置/BindWindowEx.htm",
    "GetLastError": "基本设置/GetLastError.htm",
    "Reg": "基本设置/Reg.htm",
    "binding-scope": "常见问题/后台绑定会对哪些接口有影响.htm",
    "input-channel": "常见问题/为什么不能和别的后台插件混用.htm",
    "object-scope": "常见问题/插件接口生效范围.htm",
    "threading": "常见问题/如何在多线程下使用.htm",
}
ALLOWED = {**ORDINARY, **RESTRICTED}
MAX_CHM = 64 * 1024 * 1024
MAX_PAGE = 2 * 1024 * 1024
MAX_EXCERPT = 16 * 1024
RESERVED = re.compile(r"^(?:con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)", re.I)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def require(condition, message):
    if not condition:
        raise DocsError(message)


def relative_name(value):
    require(isinstance(value, str) and value and len(value) <= 240, "invalid relative path")
    require(unicodedata.normalize("NFC", value) == value, "non-canonical Unicode path")
    for part in value.split("/"):
        require(part not in ("", ".", "..") and not RESERVED.match(part), "unsafe path component")
        require(not part.endswith((".", " ")) and not re.search(r'[<>:"\\|?*\x00-\x1f\x7f]', part),
                "unsafe Windows path")
    return value


def plain_path(value, kind):
    path = Path(value)
    require(".." not in path.parts, "parent traversal is not accepted")
    path = path.absolute()
    require(not str(path).startswith(("\\\\", "//")), "network/device paths are not accepted")
    for node in reversed((path, *path.parents)):
        if node != Path(node.anchor):
            relative_name(node.name)
        try:
            info = node.lstat()
        except FileNotFoundError:
            if node == path and kind == "new":
                return path
            raise DocsError("path or parent does not exist") from None
        require(not stat.S_ISLNK(info.st_mode) and not (
            getattr(info, "st_file_attributes", 0) & 0x400), "symlink/reparse path rejected")
        if node != path or kind == "dir":
            require(stat.S_ISDIR(info.st_mode), "expected plain directory")
        elif kind == "file":
            require(stat.S_ISREG(info.st_mode), "expected plain file")
        elif kind == "new":
            raise DocsError("output already exists; choose a new directory")
    return path


def read_bytes(path, limit):
    path = plain_path(path, "file")
    require(path.stat().st_size <= limit, "input exceeds size limit")
    with path.open("rb") as stream:
        data = stream.read(limit + 1)
    require(len(data) <= limit, "input grew beyond size limit")
    return data


def json_bytes(value):
    return (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode("utf-8")


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate JSON key")
        result[key] = value
    return result


def check_hash(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value), "invalid SHA-256")


def load_selection(path):
    try:
        data = json.loads(read_bytes(path, MAX_PAGE).decode("utf-8", "strict"),
                          object_pairs_hook=unique_object)
    except (UnicodeError, json.JSONDecodeError) as exc:
        raise DocsError("selection must be strict UTF-8 JSON") from exc
    require(isinstance(data, dict) and set(data) == {
        "schema", "sdk_label", "chm_sha256", "chm_bytes", "pages"}, "invalid selection fields")
    require(type(data["schema"]) is int and data["schema"] == 1, "unsupported selection schema")
    check_label(data["sdk_label"])
    check_hash(data["chm_sha256"])
    require(type(data["chm_bytes"]) is int and 0 < data["chm_bytes"] <= MAX_CHM, "invalid CHM size")
    require(isinstance(data["pages"], list) and 0 < len(data["pages"]) <= len(ALLOWED), "invalid page count")
    seen = set()
    for page in data["pages"]:
        require(isinstance(page, dict) and set(page) == {
            "output", "source", "source_sha256", "output_sha256", "review"}, "invalid page fields")
        name = page["output"]
        require(isinstance(name, str) and name.endswith(".md") and name[:-3] in ALLOWED,
                "interface not in the reviewed export surface")
        require(page["source"] == ALLOWED[name[:-3]], "interface/source mismatch")
        relative_name(page["source"])
        require(name.casefold() not in seen, "duplicate selected page")
        seen.add(name.casefold())
        require(page["review"] in ("reviewed", "needs-review", "missing"), "invalid review state")
        if page["review"] != "missing":
            check_hash(page["source_sha256"])
        else:
            require(page["source_sha256"] is None, "missing page cannot have current source hash")
        if page["review"] == "reviewed":
            check_hash(page["output_sha256"])
        else:
            require(page["output_sha256"] is None, "unreviewed page cannot reuse an excerpt hash")
    return data


def check_label(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9][A-Za-z0-9._-]{0,31}", value),
            "invalid SDK label (label is not a vendor version attestation)")


def chm_index(data):
    """Validate ITSF v3 directory names BEFORE giving the file to hh.exe.

    Only the uncompressed ITSP/PMGL index is read; HTML Help remains the sole
    decompressor. Reject Windows aliases/traversal and duplicate archive names.
    """
    require(len(data) >= 96 and data[:4] == b"ITSF" and
            struct.unpack_from("<II", data, 4) == (3, 96), "unsupported CHM header")
    offset, length = struct.unpack_from("<QQ", data, 72)
    require(96 <= offset <= len(data) and 84 <= length <= len(data) - offset, "invalid CHM directory range")
    directory = data[offset:offset + length]
    require(directory[:4] == b"ITSP" and struct.unpack_from("<II", directory, 4) == (1, 84),
            "unsupported CHM directory")
    block_size = struct.unpack_from("<I", directory, 16)[0]
    count = struct.unpack_from("<I", directory, 44)[0]
    require(512 <= block_size <= 65536 and 0 < count <= 8192 and
            len(directory) == 84 + block_size * count, "invalid CHM directory blocks")
    names, aliases, entries, total_bytes = set(), {}, 0, 0

    def word(block, pos, end):
        value = 0
        for _ in range(10):
            require(pos < end, "truncated CHM directory entry")
            byte = block[pos]
            pos += 1
            value = (value << 7) | (byte & 127)
            if byte < 128:
                return value, pos
        raise DocsError("oversized CHM directory integer")

    internal_prefix = "::DataSpace/"
    for start in range(84, len(directory), block_size):
        block = directory[start:start + block_size]
        require(block[:4] in (b"PMGL", b"PMGI"), "unsupported CHM directory block")
        if block[:4] == b"PMGI":
            continue
        end = block_size - struct.unpack_from("<I", block, 4)[0]
        require(20 <= end <= block_size - 2, "invalid CHM directory free space")
        pos, block_entries = 20, 0
        while pos < end:
            size, pos = word(block, pos, end)
            require(0 < size <= 1024 and size <= end - pos, "invalid CHM name length")
            try:
                name = block[pos:pos + size].decode("utf-8", "strict")
            except UnicodeError as exc:
                raise DocsError("CHM directory names must be strict UTF-8") from exc
            pos += size
            section, pos = word(block, pos, end)
            _, pos = word(block, pos, end)
            entry_size, pos = word(block, pos, end)
            require(section in (0, 1), "unsupported CHM content section")
            total_bytes += entry_size
            require(total_bytes <= MAX_CHM, "CHM declared expansion exceeds size limit")
            entries += 1
            block_entries += 1
            require(entries <= 16384, "too many CHM entries")
            require(name not in names, "duplicate CHM entry")
            names.add(name)
            if name == "/":
                continue
            if name.startswith(internal_prefix):
                relative = name[len(internal_prefix):].removesuffix("/")
                relative_name(relative)
                continue
            require(name.startswith("/"), "unexpected CHM namespace")
            relative = relative_name(name[1:].removesuffix("/"))
            parts = relative.split("/")
            for i in range(1, len(parts) + 1):
                spelling = "/".join(parts[:i])
                is_dir = i < len(parts) or name.endswith("/")
                key = spelling.casefold()
                previous = aliases.get(key)
                require(previous is None or previous == (spelling, is_dir), "ambiguous CHM path alias")
                aliases[key] = (spelling, is_dir)
        require(block_entries == struct.unpack_from("<H", block, block_size - 2)[0],
                "CHM entry count mismatch")
    require(entries > 0, "empty CHM index")
    return {spelling for spelling, is_dir in aliases.values()
            if not is_dir and spelling.lower().endswith((".htm", ".html"))}


def read_chm(path):
    data = read_bytes(path, MAX_CHM)
    index = chm_index(data)
    return data, index


def extract_selected(data, index, sources):
    require(os.name == "nt", "CHM extraction is supported on Windows only")
    require(all(source in ALLOWED.values() for source in sources), "unselected extraction source")
    buffer = ctypes.create_unicode_buffer(32768)
    size = ctypes.windll.kernel32.GetWindowsDirectoryW(buffer, len(buffer))
    require(0 < size < len(buffer), "cannot locate Windows HTML Help")
    hh = plain_path(Path(buffer.value) / "hh.exe", "file")
    # Private, disposable extraction. No browser invocation, shell, SDK or registration.
    with tempfile.TemporaryDirectory(prefix="steamai-dm-docs-") as temporary:
        work = plain_path(temporary, "dir")
        archive, extracted = work / "input.chm", work / "extracted"
        archive.write_bytes(data)
        extracted.mkdir()
        startup = subprocess.STARTUPINFO()
        startup.dwFlags |= subprocess.STARTF_USESHOWWINDOW
        startup.wShowWindow = 0
        try:
            subprocess.run([str(hh), "-decompile", str(extracted), str(archive)],
                           cwd=work, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL, startupinfo=startup,
                           creationflags=subprocess.CREATE_NO_WINDOW, timeout=60, check=True)
        except (subprocess.SubprocessError, OSError) as exc:
            raise DocsError("HTML Help decompile failed or timed out") from exc
        actual, seen, total = set(), set(), 0
        for parent, dirs, files in os.walk(extracted, followlinks=False):
            for name in dirs + files:
                node = Path(parent) / name
                plain_path(node, "dir" if name in dirs else "file")
                relative = relative_name(node.relative_to(extracted).as_posix())
                require(relative.casefold() not in seen, "duplicate extracted path")
                seen.add(relative.casefold())
                if name in files:
                    total += node.stat().st_size
                    require(total <= MAX_CHM, "extracted CHM exceeds size limit")
                    if relative.lower().endswith((".htm", ".html")):
                        actual.add(relative)
        require(actual == index, "decompiled HTML inventory does not match CHM index")
        return {source: read_bytes(extracted / source, MAX_PAGE)
                for source in sources if source in actual}


class BodyText(HTMLParser):
    BLOCKS = {"p", "div", "li", "tr", "h1", "h2", "h3", "h4", "br"}
    OMIT = {"script", "style", "object", "iframe", "noscript", "template", "svg"}

    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.body = False
        self.bodies = 0
        self.closed = False
        self.omitted = []
        self.parts = []

    def handle_starttag(self, tag, attrs):
        if tag == "body":
            self.bodies += 1
            self.body = True
        if not self.body:
            return
        if tag in self.OMIT:
            self.omitted.append(tag)
        if not self.omitted and tag in self.BLOCKS:
            self.parts.append("\n")

    def handle_endtag(self, tag):
        if self.omitted:
            if tag == self.omitted[-1]:
                self.omitted.pop()
            return
        if tag in self.BLOCKS and self.body:
            self.parts.append("\n")
        if tag == "body":
            self.body = False
            self.closed = True

    def handle_data(self, data):
        if self.body and not self.omitted:
            self.parts.append(re.sub(r"\s+", " ", data))


def body_lines(raw):
    charsets = {match.lower() for match in re.findall(
        rb"charset\s*=\s*[\"']?([A-Za-z0-9_-]+)", raw)}
    encodings = {b"gb2312": "gb18030", b"gbk": "gb18030", b"gb18030": "gb18030",
                 b"utf-8": "utf-8", b"utf8": "utf-8"}
    require(len(charsets) == 1 and next(iter(charsets)) in encodings, "missing/ambiguous/unsupported HTML charset")
    encoding = encodings[next(iter(charsets))]
    try:
        text = raw.decode(encoding, "strict")
    except UnicodeError as exc:
        raise DocsError("HTML strict decoding failed") from exc
    require(not re.search(r"[\x00-\x08\x0b\x0c\x0e-\x1f\ufffd]", text), "invalid HTML text character")
    parser = BodyText()
    parser.feed(text)
    parser.close()
    require(parser.bodies == 1 and parser.closed and not parser.omitted, "incomplete HTML body")
    lines = [re.sub(r"\s+", " ", line).strip() for line in "".join(parser.parts).split("\n")]
    lines = [line for line in lines if line]
    require(lines, "empty HTML body")
    return lines, encoding


def ordinary_text(raw, interface):
    require(interface in ORDINARY, "restricted page cannot be exported as full text")
    lines, _ = body_lines(raw)
    stop = next((i for i, line in enumerate(lines) if re.fullmatch(
        r"(?:示例|示例代码|调用示例)\s*[:：]?", line)), len(lines))
    lines = lines[:stop]
    require(any(re.search(r"\b" + re.escape(interface) + r"\s*\(", line) for line in lines),
            "ordinary page prototype does not match the selected interface")
    # Plain text only: no HTML, executable examples or Markdown rendering.
    return ("\n".join(lines) + "\n").encode("utf-8")


def reviewed_files(selection, directory):
    directory = plain_path(directory, "dir")
    result = {}
    for page in selection["pages"]:
        require(page["review"] == "reviewed", "selection still contains pages requiring review/removal")
        raw = read_bytes(directory / page["output"], MAX_EXCERPT)
        require(raw and b"\r" not in raw, "reviewed Markdown must use UTF-8 LF")
        try:
            text = raw.decode("utf-8", "strict")
        except UnicodeError as exc:
            raise DocsError("reviewed Markdown is not strict UTF-8") from exc
        require("\ufffd" not in text and "\x00" not in text and not text.startswith("\ufeff"),
                "invalid reviewed Markdown text")
        require(sha(raw) == page["output_sha256"], "reviewed excerpt hash mismatch")
        result[page["output"]] = raw
    return result


def verify_selected(selection, data, selected):
    require(len(data) == selection["chm_bytes"] and sha(data) == selection["chm_sha256"],
            "CHM does not match selection")
    for page in selection["pages"]:
        raw = selected.get(page["source"])
        require(raw is not None and sha(raw) == page["source_sha256"], "selected source hash mismatch")
        body_lines(raw)


def check_output(output, reviewed_dir):
    output = plain_path(output, "new")
    reviewed_dir = plain_path(reviewed_dir, "dir")
    require(output != reviewed_dir and reviewed_dir not in output.parents, "output cannot be inside reviewed material")
    for parent in (output.parent, *output.parent.parents):
        require(parent.name != ".steamai-vnext" and not os.path.lexists(parent / ".steamai-vnext"),
                "output cannot be inside an existing case")
    return output


def publish(output, files):
    # On the supported Windows platform rename never replaces an existing target.
    require(os.name == "nt", "publishing is supported on Windows only")
    output = plain_path(output, "new")
    stage = Path(tempfile.mkdtemp(prefix=".dm-docs-", dir=output.parent))
    try:
        for relative, data in sorted(files.items()):
            relative_name(relative)
            destination = stage / relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            with destination.open("xb") as stream:
                stream.write(data)
        plain_path(output, "new")
        os.rename(stage, output)
    finally:
        if stage.exists():
            shutil.rmtree(stage)


def source_markdown(selection, files):
    lines = ["# 大漠资料来源", "", "- SDK 标签：`" + selection["sdk_label"] +
             "`；由调用者提供，不是厂商版本签名，实际 SDK 须另核对 Ver。",
             "- CHM bytes：`" + str(selection["chm_bytes"]) + "`；SHA-256：`" + selection["chm_sha256"] + "`。",
             "- 只保留经审节选；未加载 SDK、未执行 HTML，不证明运行兼容，不自动升级 SDK 或已有 case snapshot。",
             "- 原文著作权归原作者；不附 CHM、SDK、许可证或注册码。",
             "- 原页 unchanged 才能逐字复用已审节选；changed/added 必须另审，不复用旧行号。",
             "- 编码按 HTML charset 严格解码（GB2312/GBK 使用 GB18030）；节选为 UTF-8 LF。", "",
             "| 文件 | CHM 内路径 | 原 HTML SHA-256 | 节选 bytes | 节选 SHA-256 |",
             "|---|---|---|---|---|"]
    for page in selection["pages"]:
        lines.append("| `{output}` | `{source}` | `{source_sha256}` | {size} | `{output_sha256}` |".format(
            **page, size=len(files[page["output"]])))
    return ("\n".join(lines) + "\n").encode("utf-8")


def compare(args):
    selection = load_selection(args.selection)
    excerpts = reviewed_files(selection, args.reviewed_dir)
    output = check_output(args.output, args.reviewed_dir)
    check_label(args.sdk_label)
    additions = args.add_interface or []
    require(len(additions) == len(set(additions)), "duplicate added interface")
    names = {page["output"][:-3] for page in selection["pages"]}
    require(all(name in ORDINARY and name not in names for name in additions), "invalid or already selected addition")
    sources = [page["source"] for page in selection["pages"]] + [ORDINARY[name] for name in additions]
    old_data, old_index = read_chm(args.baseline_chm)
    require(sha(old_data) == selection["chm_sha256"] and len(old_data) == selection["chm_bytes"],
            "baseline CHM does not match selection")
    new_data, new_index = read_chm(args.chm)
    old = extract_selected(old_data, old_index, sources)
    verify_selected(selection, old_data, old)
    new = extract_selected(new_data, new_index, sources)
    pages = selection["pages"] + [{"output": name + ".md", "source": ORDINARY[name],
                                   "source_sha256": None, "output_sha256": None,
                                   "review": "needs-review"} for name in additions]
    result, records, files = [], [], {}
    counts = dict.fromkeys(("unchanged", "changed", "added", "missing"), 0)
    for page in pages:
        name, source = page["output"], page["source"]
        raw = new.get(source)
        current_hash = sha(raw) if raw is not None else None
        status = ("missing" if raw is None else "added" if name[:-3] in additions else
                  "unchanged" if current_hash == page["source_sha256"] else "changed")
        counts[status] += 1
        encoding = body_lines(raw)[1] if raw is not None else None
        next_page = dict(page, source_sha256=current_hash,
                         output_sha256=page["output_sha256"] if status == "unchanged" else None,
                         review="reviewed" if status == "unchanged" else "missing" if raw is None else "needs-review")
        result.append(next_page)
        record = {"output": name, "source": source, "status": status, "encoding": encoding,
                  "old_source_sha256": page["source_sha256"], "source_sha256": current_hash,
                  "output_sha256": next_page["output_sha256"], "review_required": status != "unchanged"}
        if status == "unchanged":
            files["reviewed/" + name] = excerpts[name]
        elif status in ("changed", "added") and name[:-3] in ORDINARY:
            candidate = ordinary_text(raw, name[:-3])
            previous = ordinary_text(old[source], name[:-3]) if source in old else b""
            files["review/" + name[:-3] + ".txt"] = candidate
            delta = "".join(difflib.unified_diff(previous.decode("utf-8").splitlines(True),
                                                candidate.decode("utf-8").splitlines(True),
                                                fromfile="baseline/" + source, tofile="candidate/" + source))
            no_delta = ("Explicit selection addition; exported text matches the baseline. Review required.\n"
                        if status == "added" else
                        "Raw HTML changed; exported text unchanged. Review omitted content/formatting separately.\n")
            files["review/" + name[:-3] + ".diff.txt"] = (delta or no_delta).encode("utf-8")
            record["text_scope"] = "body text before first example; omitted content still requires review"
        elif status == "changed":
            record["text_scope"] = "restricted page: hashes only; manually review the local source, no old line reuse"
        records.append(record)
    next_selection = dict(selection, sdk_label=args.sdk_label, chm_sha256=sha(new_data),
                          chm_bytes=len(new_data), pages=result)
    files["selection.json"] = json_bytes(next_selection)
    report = {"schema": 1, "baseline_sdk_label": selection["sdk_label"], "sdk_label": args.sdk_label,
              "baseline_chm_sha256": sha(old_data), "chm_sha256": sha(new_data),
              "baseline_chm_bytes": len(old_data), "chm_bytes": len(new_data),
              "baseline_html_pages": len(old_index), "html_pages": len(new_index),
              "counts": counts, "scope": "selected pages only; added means explicitly added to selection",
              "pages": records, "outputs": {name: sha(raw) for name, raw in sorted(files.items())}}
    files["report.json"] = json_bytes(report)
    publish(output, files)
    return counts


def import_reviewed(args):
    selection = load_selection(args.selection)
    files = reviewed_files(selection, args.reviewed_dir)
    output = check_output(args.output, args.reviewed_dir)
    data, index = read_chm(args.chm)
    require(sha(data) == selection["chm_sha256"] and len(data) == selection["chm_bytes"],
            "CHM does not match selection")
    selected = extract_selected(data, index, [page["source"] for page in selection["pages"]])
    verify_selected(selection, data, selected)
    files["SOURCE.md"] = source_markdown(selection, files)
    files["selection.json"] = json_bytes(selection)
    publish(output, files)
    return {"imported": len(selection["pages"]), "sdk_label": selection["sdk_label"]}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("compare", "import"):
        command = commands.add_parser(name)
        command.add_argument("--chm", required=True)
        command.add_argument("--selection", required=True)
        command.add_argument("--reviewed-dir", required=True)
        command.add_argument("--output", required=True, help="new directory; parent must already exist")
        if name == "compare":
            command.add_argument("--baseline-chm", required=True)
            command.add_argument("--sdk-label", required=True)
            command.add_argument("--add-interface", action="append", choices=sorted(ORDINARY))
    args = parser.parse_args(argv)
    try:
        summary = compare(args) if args.command == "compare" else import_reviewed(args)
    except (DocsError, OSError) as exc:
        # Avoid putting local absolute paths in a report or copied terminal log.
        print("dm_docs: " + (str(exc) if isinstance(exc, DocsError) else type(exc).__name__), file=sys.stderr)
        return 2
    print(json.dumps(summary, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
