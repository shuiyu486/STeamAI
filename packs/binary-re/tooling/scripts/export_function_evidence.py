"""显式导出已分析 IDA 数据库中的一个有界 Windows x64 函数。

只读取数据库；不启动分析、调试器或目标。import 本模块不会访问 IDA 或写文件。
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import time


MAX_ENTRIES = 1000
MAX_OUTPUT_BYTES = 256 * 1024
MAX_SECONDS = 30
_ALIAS = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,63}\Z")
_OUTPUT = re.compile(r"[a-z0-9][a-z0-9_-]{0,79}\.json\Z")
_SHA256 = re.compile(r"[0-9a-f]{64}\Z")
_RESERVED = {"con", "prn", "aux", "nul"} | {
    prefix + str(number) for prefix in ("com", "lpt") for number in range(1, 10)
}


class EvidenceError(RuntimeError):
    """未取得完整、有界且可绑定的观察；不得将错误当作成功 artifact。"""


class _IDA:
    def __init__(self):
        try:
            import __main__
            import ida_auto
            import ida_bytes
            import ida_dbg
            import ida_funcs
            import ida_ida
            import ida_idaapi
            import ida_kernwin
            import ida_nalt
            import ida_xref
            import idc
        except ImportError as exc:
            raise EvidenceError("需要现有 IDA/IDAPython 会话，不自动安装或启动") from exc
        self.auto = ida_auto
        self.byte = ida_bytes
        self.dbg = ida_dbg
        self.func = ida_funcs
        self.info = ida_ida
        self.kernel = ida_kernwin
        self.nalt = ida_nalt
        self.xref = ida_xref
        self.idc = idc
        self.badaddr = ida_idaapi.BADADDR
        version = getattr(__main__, "IDAPYTHON_VERSION", None)
        if not isinstance(version, tuple) or len(version) != 3 or any(type(n) is not int for n in version):
            raise EvidenceError("无法核对 IDAPython 版本；不推测兼容接口")
        self.python_version = ".".join(str(n) for n in version)

    def state(self):
        digest = self.nalt.retrieve_input_file_sha256()
        if not isinstance(digest, bytes) or len(digest) != 32:
            raise EvidenceError("数据库缺少有效的 input SHA-256 记录")
        input_path = self.nalt.get_input_file_path()
        if not isinstance(input_path, str) or not input_path:
            raise EvidenceError("数据库输入来源不明")
        return {
            "input_sha256": digest.hex(),
            "input_path": input_path,
            "image_base": self.nalt.get_imagebase(),
            "processor": self.info.inf_get_procname(),
            "is_64bit": self.info.inf_is_64bit(),
            "is_pe": self.info.inf_get_filetype() == self.info.f_PE,
            "analysis_ready": self.auto.auto_is_ok() and self.auto.get_auto_state() == self.auto.AU_NONE,
            "debugger_active": self.dbg.is_debugger_on(),
            "change_count": self.info.inf_get_database_change_count(),
            "ida_version": self.kernel.get_kernel_version(),
            "idapython_version": self.python_version,
        }

    def function(self, ea):
        function = self.func.get_func(ea)
        if function is None:
            return None
        return (function.start_ea, function.end_ea, function.flags)

    def chunks(self, ea):
        function = self.func.get_func(ea)
        if function is None:
            raise EvidenceError("函数在读取期间消失")
        for chunk in self.func.func_tail_iterator_t(function):
            yield (chunk.start_ea, chunk.end_ea)

    def heads(self, start, end):
        ea = start
        if not self.byte.is_head(self.byte.get_full_flags(ea)):
            ea = self.byte.next_head(ea, end)
        while ea != self.badaddr and ea < end:
            yield ea
            next_ea = self.byte.next_head(ea, end)
            if next_ea != self.badaddr and next_ea <= ea:
                raise EvidenceError("IDA head 迭代没有前进")
            ea = next_ea

    def instruction(self, ea, max_end):
        flags = self.byte.get_full_flags(ea)
        if not self.byte.is_code(flags):
            raise EvidenceError("指定范围含非指令 item；本工具不推测其语义")
        end = self.byte.get_item_end(ea)
        if end <= ea or end - ea > 15 or end > max_end:
            raise EvidenceError("指令范围不是有效的 x64 指令或越出许可范围")
        data = self.idc.get_bytes(ea, end - ea, False)
        if not isinstance(data, bytes) or len(data) != end - ea:
            raise EvidenceError("无法完整读取数据库指令字节")
        for offset, value in enumerate(data):
            if self.byte.get_original_byte(ea + offset) != value:
                raise EvidenceError("指定范围存在 patched bytes，需先解释来源")
        text = self.idc.generate_disasm_line(ea, 0)
        if not isinstance(text, str) or not text:
            raise EvidenceError("无法取得指令文本")
        return end, data.hex(), text

    def references(self, ea):
        reference = self.xref.xrefblk_t()
        present = reference.first_from(ea, self.xref.XREF_ALL)
        while present:
            yield {
                "to": reference.to,
                "type": reference.type,
                "is_code": bool(reference.iscode),
                "is_call": reference.type in (self.xref.fl_CN, self.xref.fl_CF),
            }
            present = reference.next_from()


def _integer(value, label, minimum, maximum):
    if type(value) is not int or not minimum <= value <= maximum:
        raise EvidenceError(label + " 无效")
    return value


def _ranges(values):
    if not isinstance(values, (list, tuple)) or not 1 <= len(values) <= 8:
        raise EvidenceError("允许范围必须为 1～8 个 [start, end) 区间")
    ranges = []
    for item in values:
        if not isinstance(item, (list, tuple)) or len(item) != 2:
            raise EvidenceError("允许范围格式无效")
        start = _integer(item[0], "范围起点", 0, (1 << 64) - 1)
        end = _integer(item[1], "范围终点", 1, (1 << 64) - 1)
        if end <= start:
            raise EvidenceError("允许范围必须非空")
        ranges.append((start, end))
    ranges.sort()
    if any(right[0] < left[1] for left, right in zip(ranges, ranges[1:])):
        raise EvidenceError("允许范围重复或重叠")
    return ranges


def _inside(start, end, ranges):
    return any(lo <= start < end <= hi for lo, hi in ranges)


def _plain_directory(path):
    try:
        info = path.lstat()
    except OSError as exc:
        raise EvidenceError("目录不存在或无法读取：" + str(path)) from exc
    if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode) or (
        getattr(info, "st_file_attributes", 0) & getattr(stat, "FILE_ATTRIBUTE_REPARSE_POINT", 0x400)
    ):
        raise EvidenceError("输出路径含非普通目录或 reparse/symlink：" + str(path))
    return info.st_dev, info.st_ino


def _output_path(case_root, output_name):
    if not isinstance(output_name, str) or not _OUTPUT.fullmatch(output_name) or output_name[:-5] in _RESERVED:
        raise EvidenceError("输出必须是非保留名称的小写 JSON basename")
    if not isinstance(case_root, (str, os.PathLike)):
        raise EvidenceError("case_root 必须是已存在的绝对目录")
    raw = os.fspath(case_root)
    if not isinstance(raw, str) or not raw or "\x00" in raw:
        raise EvidenceError("case_root 无效")
    if raw.startswith(("\\\\", "//")):
        raise EvidenceError("不支持 UNC、设备或扩展前缀路径")
    path = Path(raw)
    if not path.is_absolute():
        raise EvidenceError("case_root 必须为绝对路径")
    # 除 Windows drive 外禁止 ADS、父路径和 Windows 名称别名。
    parts = raw.replace("\\", "/").split("/")
    for index, part in enumerate(parts):
        if not part or (index == 0 and re.fullmatch(r"[A-Za-z]:", part)):
            continue
        if part in (".", "..") or ":" in part or part.rstrip(" .") != part or any(c in part for c in '<>"|?*'):
            raise EvidenceError("case_root 含不明确的路径组件")
        if part.split(".")[0].lower() in _RESERVED:
            raise EvidenceError("case_root 含 Windows 保留名称")
    for directory in reversed((path,) + tuple(path.parents)):
        _plain_directory(directory)
    artifact_dir = path / ".steamai-vnext" / "artifacts"
    _plain_directory(artifact_dir.parent)
    identity = _plain_directory(artifact_dir)
    output = artifact_dir / output_name
    if os.path.lexists(output):
        raise EvidenceError("输出已存在，不覆盖：" + str(output))
    return output, identity


class _Budget:
    def __init__(self, max_entries, timeout_seconds, clock):
        self.maximum = max_entries
        self.entries = 0
        self.clock = clock
        self.deadline = clock() + timeout_seconds

    def check(self):
        if self.clock() >= self.deadline:
            raise EvidenceError("达到合作式截止；阻塞的 IDA API 不受硬超时保护")

    def take(self):
        self.check()
        self.entries += 1
        if self.entries > self.maximum:
            raise EvidenceError("函数/chunk/指令/引用总条目超过预算")


def _checked_state(api, digest, image_base):
    state = api.state()
    if state["input_sha256"] != digest or state["image_base"] != image_base:
        raise EvidenceError("已加载数据库的 input SHA-256/image base 与请求不一致")
    if state["processor"] != "metapc" or state["is_64bit"] is not True or state["is_pe"] is not True:
        raise EvidenceError("只支持已分析的 Windows x64 PE 数据库")
    if state["analysis_ready"] is not True or state["debugger_active"] is not False:
        raise EvidenceError("分析未稳定或调试器正在使用；不自动等待、分析或调试")
    _integer(state["change_count"], "数据库 change count", 0, (1 << 64) - 1)
    for key in ("ida_version", "idapython_version", "input_path"):
        if not isinstance(state[key], str) or not state[key]:
            raise EvidenceError("数据库/工具身份缺失：" + key)
    return state


def _collect(api, function_ea, ranges, callsites, budget):
    function = api.function(function_ea)
    if function is None or function[0] != function_ea:
        raise EvidenceError("必须指定已识别函数的精确入口")
    _integer(function[2], "函数 flags", 0, (1 << 64) - 1)
    budget.take()
    chunks = []
    for start, end in api.chunks(function_ea):
        budget.take()
        if not _inside(start, end, ranges):
            raise EvidenceError("函数 chunk 越出允许范围")
        if any(start < old_end and old_start < end for old_start, old_end in chunks):
            raise EvidenceError("函数 chunk 重复或重叠")
        chunks.append((start, end))
    if not chunks or (function[0], function[1]) not in chunks:
        raise EvidenceError("函数入口 chunk 缺失或不一致")
    instructions = []
    references = []
    seen = set()

    def capture(ea, segment, callsite):
        if ea in seen:
            raise EvidenceError("指令范围重复")
        if not _inside(ea, ea + 1, [segment]) or not _inside(ea, ea + 1, ranges):
            raise EvidenceError("指令起点越出许可范围")
        budget.take()
        end, data, text = api.instruction(ea, segment[1])
        if not _inside(ea, end, [segment]) or not _inside(ea, end, ranges):
            raise EvidenceError("指令越出指定 chunk 或允许范围")
        if not re.fullmatch(r"(?:[0-9a-f]{2}){1,15}", data) or len(data) != 2 * (end - ea):
            raise EvidenceError("指令字节与范围不一致")
        if not isinstance(text, str) or not text or len(text.encode("utf-8")) > 4096:
            raise EvidenceError("指令文本缺失或超限")
        seen.add(ea)
        instructions.append({"ea": ea, "end_ea": end, "bytes": data, "text": text, "callsite": callsite})
        calls_function = False
        for reference in api.references(ea):
            budget.take()
            target = _integer(reference["to"], "xref 目标", 0, (1 << 64) - 1)
            kind = _integer(reference["type"], "xref 类型", 0, 255)
            if type(reference["is_code"]) is not bool or type(reference["is_call"]) is not bool:
                raise EvidenceError("xref 属性无效")
            references.append({"from_ea": ea, "to_ea": target, "type": kind,
                               "is_code": reference["is_code"], "is_call": reference["is_call"]})
            calls_function |= reference["is_code"] and reference["is_call"] and target == function_ea
        if callsite and not calls_function:
            raise EvidenceError("指定调用点没有指向目标函数的已识别调用引用")
        return end

    for start, end in chunks:
        previous_end = start
        for ea in api.heads(start, end):
            budget.check()
            if ea != previous_end:
                raise EvidenceError("函数 chunk 存在未覆盖字节/未知 item；不能宣称完整观察")
            previous_end = capture(ea, (start, end), False)
        if previous_end != end:
            raise EvidenceError("函数 chunk 未完整覆盖")
    for ea in callsites:
        if ea in seen:
            raise EvidenceError("额外调用点已在函数内；只接受不同的外部调用点")
        segment = next((pair for pair in ranges if pair[0] <= ea < pair[1]), None)
        if segment is None:
            raise EvidenceError("调用点不在允许范围")
        if api.function(ea) is None:
            raise EvidenceError("调用点不属于已识别函数")
        capture(ea, segment, True)
    return {"entry_ea": function[0], "entry_end_ea": function[1], "flags": function[2],
            "chunks": [{"start_ea": lo, "end_ea": hi} for lo, hi in chunks],
            "instructions": instructions, "references": references}


def export_function_evidence(*, case_root, output_name, input_alias, database_alias,
                             expected_input_sha256, expected_image_base, function_ea,
                             allowed_ranges, callsites=(), max_entries=MAX_ENTRIES,
                             max_output_bytes=MAX_OUTPUT_BYTES, timeout_seconds=MAX_SECONDS,
                             _api=None, _clock=time.monotonic):
    """只在显式调用时访问 IDA；返回 artifact 路径、SHA-256 和 bytes，不判断研究结论。"""
    for value in (input_alias, database_alias):
        if not isinstance(value, str) or not _ALIAS.fullmatch(value):
            raise EvidenceError("输入和数据库必须使用有界 alias，不接受实际路径")
    if not isinstance(expected_input_sha256, str) or not _SHA256.fullmatch(expected_input_sha256):
        raise EvidenceError("expected_input_sha256 必须是小写 SHA-256")
    _integer(expected_image_base, "image base", 0, (1 << 64) - 1)
    _integer(function_ea, "函数入口", 0, (1 << 64) - 1)
    _integer(max_entries, "条目上限", 1, MAX_ENTRIES)
    _integer(max_output_bytes, "输出上限", 1, MAX_OUTPUT_BYTES)
    _integer(timeout_seconds, "截止秒数", 1, MAX_SECONDS)
    ranges = _ranges(allowed_ranges)
    if not isinstance(callsites, (list, tuple)) or len(callsites) > 2:
        raise EvidenceError("最多两个明确调用点")
    for ea in callsites:
        _integer(ea, "调用点", 0, (1 << 64) - 1)
    if len(set(callsites)) != len(callsites) or not _inside(function_ea, function_ea + 1, ranges):
        raise EvidenceError("调用点重复或函数入口越界")
    output, directory_identity = _output_path(case_root, output_name)
    budget = _Budget(max_entries, timeout_seconds, _clock)
    try:
        api = _api if _api is not None else _IDA()
        before = _checked_state(api, expected_input_sha256, expected_image_base)
        budget.check()
        observations = _collect(api, function_ea, ranges, callsites, budget)
        entries = budget.entries
        # 再读同一有界集合检测变化；这是 currentness 检查，不是独立验证。
        budget.entries = 0
        repeated = _collect(api, function_ea, ranges, callsites, budget)
        after = _checked_state(api, expected_input_sha256, expected_image_base)
        budget.check()
        if before != after or observations != repeated:
            raise EvidenceError("数据库状态或选定观察在导出期间变化")
    except (AttributeError, KeyError, TypeError) as exc:
        raise EvidenceError("IDA 接口/状态格式不可验证；不推测兼容") from exc
    packet = {
        "schema_version": 1,
        "kind": "ida-function-evidence",
        "source": {
            "input_alias": input_alias, "database_alias": database_alias,
            "recorded_input_sha256": before["input_sha256"],
            "original_input_rehashed": False, "database_file_hashed": False,
            "image_base": before["image_base"], "processor": before["processor"], "bitness": 64,
            "file_type": "PE", "ida_version": before["ida_version"],
            "idapython_version": before["idapython_version"],
            "database_change_count": before["change_count"], "analysis_ready": True,
            "debugger_active": False, "selected_bytes_match_original": True,
        },
        "selection": {"function_ea": function_ea,
                      "allowed_ranges": [{"start_ea": lo, "end_ea": hi} for lo, hi in ranges],
                      "callsites": list(callsites)},
        "limits": {"max_entries": max_entries, "max_output_bytes": max_output_bytes,
                   "timeout_seconds": timeout_seconds, "entries": entries},
        "observation": observations,
        "limitations": [
            "input SHA-256 仅为当前数据库记录；未重新读取原 binary 或证明其未变。",
            "未 hash 磁盘 IDB；内存数据库不等于磁盘文件，宿主 autosave 不受本工具控制。",
            "仅导出允许范围内的同源静态观察；不证明动态发生、完整程序语义或独立复现。",
            "截止为合作式检查，无法中止阻塞 IDA API；路径检查不提供对抗并发替换的 OS 隔离。",
        ],
    }
    payload = (json.dumps(packet, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
    if len(payload) > max_output_bytes:
        raise EvidenceError("完整 JSON 超过输出预算，不写部分成功 packet")
    budget.check()
    checked_output, checked_identity = _output_path(case_root, output_name)
    if checked_output != output or checked_identity != directory_identity:
        raise EvidenceError("输出目录身份发生变化")
    created = False
    try:
        with output.open("xb") as stream:
            created = True
            stream.write(payload)
            stream.flush()
            os.fsync(stream.fileno())
    except OSError as exc:
        suffix = "；可能有本次不完整文件，未登记为成功 artifact：" if created else "；未覆盖已有文件："
        raise EvidenceError("写入失败" + suffix + str(output)) from exc
    return {"path": str(output), "sha256": hashlib.sha256(payload).hexdigest(), "bytes": len(payload)}


def _address(value):
    try:
        return int(value, 0)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("地址必须是十进制或 0x 前缀整数") from exc


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--case-root", required=True)
    parser.add_argument("--output-name", required=True)
    parser.add_argument("--input-alias", required=True)
    parser.add_argument("--database-alias", required=True)
    parser.add_argument("--expected-input-sha256", required=True)
    parser.add_argument("--expected-image-base", required=True, type=_address)
    parser.add_argument("--function-ea", required=True, type=_address)
    parser.add_argument("--range", dest="allowed_ranges", required=True, action="append", nargs=2, type=_address,
                        metavar=("START", "END"))
    parser.add_argument("--callsite", dest="callsites", action="append", type=_address, default=[])
    parser.add_argument("--max-entries", type=int, default=MAX_ENTRIES)
    parser.add_argument("--max-output-bytes", type=int, default=MAX_OUTPUT_BYTES)
    parser.add_argument("--timeout-seconds", type=int, default=MAX_SECONDS)
    args = vars(parser.parse_args(argv))
    try:
        receipt = export_function_evidence(**args)
    except EvidenceError as exc:
        print(str(exc), file=sys.stderr)
        return 1
    print(json.dumps(receipt, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
