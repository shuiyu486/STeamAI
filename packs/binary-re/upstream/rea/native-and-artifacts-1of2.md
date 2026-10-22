# Native, managed, and packaged artifacts

## Go build metadata

Use `inspect_go_binary` for an explicit local ELF, PE or thin Mach-O when the
question concerns the embedded compiler version, module/dependency versions,
local replacements or build settings. No `open_binary`, Go installation or
target execution is required. The CLI equivalent is `inspect-go-binary PATH`.
Preserve artifact SHA-256, original file ranges, framed module bytes and every
unparsed module-text line. A missing record is an unknown, not proof of a
non-Go binary; observed metadata does not authenticate its claims. Function,
source and type recovery remain a separate native-analysis question.

## Rust dependency metadata

Use `inspect_rust_binary` for an explicit local ELF, PE, or thin Mach-O when
the question concerns the cargo-auditable crate list embedded in `.dep-v0`
(Mach-O: `__DATA,.dep-v0`). No `open_binary`, Rust installation, or target
execution is required. The CLI equivalent is `inspect-rust-binary PATH`.
Preserve artifact SHA-256, package order, dependency indices, and the section
file range. A missing section is null metadata, not proof the binary has no
dependencies. An omitted package kind stays null. A rustc commit hash is
inferred from a read-only `/rustc/<40-hex>/` path and does not by itself
identify the compiler. See [Rust binaries](https://github.com/morluto/rea/blob/main/docs/rust-binaries.md).

## SQLite snapshots

Use `inspect_sqlite_database` with an explicit local database snapshot for its
schema and optional rows from one exact ordinary table. It requires Node.js
24.x >=24.15 or 26+ with the native `DatabaseSync.limits` API. It also consumes the
existing sibling `-wal`; retain both original digests when citing its result.
Default output is a compact inventory; a selected table expands only its own
schema and samples 20 rows. Select `columns` to avoid irrelevant or large cells,
and request `schema_scope: "database"` only for complete schema details.
Choose `row_limit` when a finite sample is useful and keep truncation and
unspecified row order explicit. Virtual tables, shadow tables and views are
schema facts, not row-read targets. REA inspects private copies and leaves
the source files untouched; obtain a coherent quiescent database/WAL snapshot
before inspection. See [SQLite snapshots](https://github.com/morluto/rea/blob/main/docs/sqlite-databases.md).

## Recorded Linux crashes

Use `inspect_recorded_crash` with an explicit Linux x86-64 ELF core path to
inspect historical thread registers, signals, raw notes and original file
ranges. It is independent of the active disassembler target. Source-bound
register values and note bytes are observations; missing notes, signal-thread
association and current executable/library identity remain unknown. PIDs are
historical metadata and never authorize live attach or control.

The optional `include_debugger_context` facet adds mapping candidates through
caller-supplied GDB/pwndbg in an owned core-only session. Display names do not
establish current file identity; zero reported flags leave permissions unknown.
Requested unavailable context fails with setup guidance. See
[recorded crashes](https://github.com/morluto/rea/blob/main/docs/recorded-crashes.md)
for exact upstream profiles, bounds and verification coverage.

## Offline ELF layout

Use `inspect_binary_layout` with an explicit local ELF path for file-backed
sections, symbols, relocations, linkage names, and static mitigation inferences
without opening a disassembler target. When that complete record is already
retained, call `inspect_analysis_view` for a summary, mitigations or linkage
facet, one section or symbol, or a stable page. Do not re-run the decoder to
read a single object.

## Offline WASM

Use `inspect_wasm_artifact` with caller-supplied WABT for selected local bytes.
For counts, validation and WAT identity, select `detail: "summary"` on the first
MCP call. The complete original Evidence stays retained; use
`normalized_result.parent_evidence_id` for exact exports or text pages. Omitted
or complete detail keeps the existing complete result.
For a completed result, use `inspect_analysis_view` with its reported retained
reference for a summary or `view: {kind: "wasm", facet: "exports", offset: 0, limit: 32}`.
Facets also include sections, imports, candidates, glue, WAT, headers, details
and tool profile. Text offsets count UTF-16 code units; follow `next_offset`.
The complete source remains retained after an oversized MCP reply. No target
execution or network acquisition is implied, and WAT is decoded representation.

## Native targets

After `open_binary`, use focused search, procedure, or function tools directly.
Use `binary_overview` when target metadata or inventory context helps answer the
question. Prefer literal search, names, decompilation, callers, callees, and
cross-references. Addresses and recovered pseudocode are analysis observations,
not original source. Provider unavailability and unsupported metadata remain
unknown rather than false.

For a completed `analyze_function` dossier, use `inspect_analysis_view` with
`view: {kind: "native", facet: "value_flow_operations", offset: 0, limit: 32}`
and the exact retained Evidence ID to read selected facts without reanalysis.
Native facets also include procedure, pseudocode, assembly, basic blocks,
comments, callers/callees, references, unresolved calls, referenced strings and
names, the native API record, and value-flow summaries. Pseudocode offsets and limits count UTF-16
code units; use the returned `next_offset` for continuation. Provider limitations
and unavailable facts remain explicit; the complete parent Evidence is retained.

`inspect_macho` on a large framework returns megabytes of imports and exports.
Select `detail: "summary"` for slice identity, UUID and per-collection counts,
then page rows with `view: {kind: "macho", facet: "exports", offset: 0, limit: 64}`
and `normalized_result.parent_evidence_id`. Facets are entrypoints,
architectures, build_metadata, load_commands, dependencies, imports, exports,
segments and provenance. `source_exhaustive: false` means unreturned symbols
may exist.

To see which functions or Objective-C methods a Mach-O actually calls in one
run, use `observe_native_calls` with explicit breakpoints and a bounded
`duration_ms`/`max_events`. It launches a new process under LLDB, so the target
runs and may change files or use the network. Hardened-runtime targets need the
`get-task-allow` entitlement.

Use `binary_session` with no arguments to check the open target, selected
provider, and alignment. Its default `result.tool_availability` carries a
complete inventory grouped by shared facts. Expand each group's `names` and
inherit the inventory-level client requirements unless a group overrides them;
availability, reasons, remediation, modes, and flags remain inline. The
`effects` and `annotations` arrays name true flags from fixed vocabularies, so an
omitted known flag is false. `tools/list` remains the canonical source for full
tool schemas when target or provider state changes.

Provider capability groups carry provider identity and ordered limitations.
Prepend each set's `shared_limitations` to a group's `limitations` to recover the
operation's full ordered list. Candidate capabilities describe possible
providers. Top-level session capabilities describe the current route, and
`analysis_provider_binding` identifies the selected provider.

## Static memory and original file bytes

For Ghidra, `read_bytes` reads initialized analysis memory, including loader
fixups; it does not read a running process. Check `returned_bytes` and `complete`
before decoding a table. An incomplete read is missing coverage, not a zero-filled
tail. Use `list_segments` to inspect the selected block's initialization and address-space
facts, and retain unavailable facts as unknown.

Use `address_to_file_offset` to anchor one observed address to original file
bytes. A mapping for the first address does not establish a contiguous mapping
for an entire table, and loader fixups can make analysis bytes differ from source
bytes. Keep source ranges and analysis-memory observations separate.

A pointer table can refer to writable slots populated by a constructor rather
than directly to immutable resources. An uninitialized block or an observed zero
does not establish the slot's later runtime value. Follow `xrefs` and the relevant
writer's assembly or pseudocode; distinguish recorded assignments from inferred
execution order. Do not replace unreadable bytes with zeros or conclude that a
resource is absent without the necessary initialization evidence.

## DOS MZ with Ghidra

For an admitted DOS MZ target, use `inspect_native_load_image` to check measured
loaded bytes, file mappings, relocations and the entry against the immutable
snapshot. Keep `mismatch` and `unsupported` explicit. Verified import covers the
reported static image, not runtime DOS or PC-98 hardware behavior.

