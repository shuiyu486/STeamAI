package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const schemaVersion = 1

type Options struct {
	RepoRoot        string
	Target          string
	Pack            string
	Kind            string
	Force           bool
	WhatIf          bool
	Note            string
	ReviewOutputDir string
	PacketPath      string
	DiffPath        string
}

type State struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Name           string            `json:"name"`
	Kind           string            `json:"kind"`
	Status         string            `json:"status"`
	LifecycleMode  string            `json:"lifecycleMode"`
	CaseRoot       string            `json:"caseRoot"`
	RepoRoot       string            `json:"repoRoot"`
	Pack           string            `json:"pack"`
	Workspace      string            `json:"workspace"`
	CreatedAt      string            `json:"createdAt"`
	UpdatedAt      string            `json:"updatedAt"`
	LastCollectAt  string            `json:"lastCollectAt,omitempty"`
	LastReviewRoot string            `json:"lastReviewRoot,omitempty"`
	Checkpoint     map[string]string `json:"checkpoint,omitempty"`
	Counters       Counters          `json:"counters"`
}

type Counters struct {
	LoweringRequests        int `json:"loweringRequests"`
	UnresolvedRequests      int `json:"unresolvedRequests"`
	VMBlockers              int `json:"vmBlockers"`
	CandidateRows           int `json:"candidateRows"`
	CandidateUnresolvedRows int `json:"candidateUnresolvedRows,omitempty"`
	OutboxMessages          int `json:"outboxMessages"`
	InboxMessages           int `json:"inboxMessages"`
}

type Paths struct {
	CaseRoot    string
	RepoRoot    string
	Pack        string
	Name        string
	SessionRoot string
	Workspace   string
}

type Packet struct {
	SchemaVersion         int            `json:"schemaVersion"`
	PacketKind            string         `json:"packetKind"`
	Command               string         `json:"command"`
	Direction             string         `json:"direction"`
	CreatedAt             string         `json:"createdAt"`
	CaseRoot              string         `json:"caseRoot"`
	RepoRoot              string         `json:"repoRoot"`
	Pack                  string         `json:"pack"`
	ManifestPath          string         `json:"manifestPath,omitempty"`
	ManifestVersion       string         `json:"manifestVersion,omitempty"`
	ReviewRoot            string         `json:"reviewRoot,omitempty"`
	IsMutation            bool           `json:"isMutation"`
	WritesReviewArtifacts bool           `json:"writesReviewArtifacts"`
	ReviewRequired        bool           `json:"reviewRequired"`
	Producer              string         `json:"producer"`
	Session               State          `json:"session"`
	Summary               map[string]any `json:"summary"`
	Artifacts             []Artifact     `json:"artifacts"`
	Recommendations       []string       `json:"recommendations"`
}

type Artifact struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Exists bool   `json:"exists"`
	Bytes  int64  `json:"bytes"`
	Rows   int    `json:"rows,omitempty"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "rekit parallel error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "parallel" {
		args = args[1:]
	}
	opts, pos, err := parseOptions(args)
	if err != nil {
		return err
	}
	if opts.Target == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		opts.Target = cwd
	}
	metadata := readCaseMetadata(opts.Target)
	if opts.RepoRoot == "" {
		if metadata["templateRoot"] != "" {
			opts.RepoRoot = metadata["templateRoot"]
		} else {
			opts.RepoRoot = detectRepoRoot(opts.Target)
		}
	}
	if opts.Pack == "" {
		if metadata["templatePack"] != "" {
			opts.Pack = metadata["templatePack"]
		} else {
			opts.Pack = "vmp-re"
		}
	}
	if opts.Kind == "" {
		manifest := loadPackManifestFromRoots(opts.RepoRoot, opts.Pack)
		opts.Kind = nonEmpty(manifest.ParallelDefaults.DefaultKind, "feature-analysis")
	}

	action, name := parseAction(pos)
	caseRoot, err := filepath.Abs(opts.Target)
	if err != nil {
		return err
	}
	repoRoot, err := filepath.Abs(opts.RepoRoot)
	if err != nil {
		return err
	}
	opts.Target = caseRoot
	opts.RepoRoot = repoRoot

	if action == "overview" {
		return overview(opts)
	}
	if name == "" && action != "doctor" && action != "status" {
		return errors.New("需要 session 名称，例如 /rekit parallel login")
	}
	name = sanitizeName(name)
	if name == "" && action != "doctor" && action != "status" {
		return errors.New("session 名称为空或无效")
	}

	switch action {
	case "auto":
		if !sessionExists(opts, name) {
			return initSession(opts, name)
		}
		return smartSession(opts, name)
	case "init", "start", "new":
		return initSession(opts, name)
	case "status":
		if name == "" {
			return overview(opts)
		}
		return showSession(opts, name)
	case "collect":
		return collectSession(opts, name, "parallel-collect")
	case "review":
		return collectSession(opts, name, "parallel-review")
	case "sync":
		return syncSession(opts, name)
	case "promote":
		return promoteSession(opts, name)
	case "close":
		return closeSession(opts, name)
	case "doctor":
		if name == "" {
			return doctorAll(opts)
		}
		return doctorSession(opts, name)
	case "resume":
		return resumeSession(opts, name)
	case "standalone":
		return setStandalone(opts, name)
	default:
		return fmt.Errorf("未知 parallel action: %s", action)
	}
}

func parseOptions(args []string) (Options, []string, error) {
	opts := Options{}
	pos := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch strings.ToLower(a) {
		case "--repo-root", "-reporoot":
			i++
			if i >= len(args) {
				return opts, pos, fmt.Errorf("%s 缺少值", a)
			}
			opts.RepoRoot = args[i]
		case "--target", "-target":
			i++
			if i >= len(args) {
				return opts, pos, fmt.Errorf("%s 缺少值", a)
			}
			opts.Target = args[i]
		case "--pack", "-pack":
			i++
			if i >= len(args) {
				return opts, pos, fmt.Errorf("%s 缺少值", a)
			}
			opts.Pack = args[i]
		case "--kind", "-kind":
			i++
			if i >= len(args) {
				return opts, pos, fmt.Errorf("%s 缺少值", a)
			}
			opts.Kind = args[i]
		case "--note", "-note":
			i++
			if i >= len(args) {
				return opts, pos, fmt.Errorf("%s 缺少值", a)
			}
			opts.Note = args[i]
		case "--review-output-dir", "-reviewoutputdir":
			i++
			if i >= len(args) {
				return opts, pos, fmt.Errorf("%s 缺少值", a)
			}
			opts.ReviewOutputDir = args[i]
		case "--packet-path", "-packetpath":
			i++
			if i >= len(args) {
				return opts, pos, fmt.Errorf("%s 缺少值", a)
			}
			opts.PacketPath = args[i]
		case "--diff-path", "-diffpath":
			return opts, pos, fmt.Errorf("parallel does not support %s; use packet.json and summary.md outputs instead", a)
		case "--force", "-force":
			opts.Force = true
		case "--what-if", "-whatif":
			opts.WhatIf = true
		default:
			if strings.HasPrefix(a, "-") {
				return opts, pos, fmt.Errorf("未知选项：%s", a)
			}
			pos = append(pos, a)
		}
	}
	return opts, pos, nil
}

func parseAction(pos []string) (string, string) {
	if len(pos) == 0 {
		return "overview", ""
	}
	actions := map[string]bool{
		"init": true, "start": true, "new": true, "status": true, "collect": true, "review": true, "sync": true, "promote": true, "close": true, "doctor": true, "resume": true, "standalone": true,
	}
	first := strings.ToLower(pos[0])
	if actions[first] {
		name := ""
		if len(pos) > 1 {
			name = pos[1]
		}
		return first, name
	}
	name := pos[0]
	if len(pos) > 1 {
		second := strings.ToLower(pos[1])
		if actions[second] {
			return second, name
		}
	}
	return "auto", name
}

func readCaseMetadata(target string) map[string]string {
	if m := readSimpleYAML(filepath.Join(target, ".rekit", "instance.yml")); len(m) > 0 {
		return m
	}
	if m := readSimpleYAML(filepath.Join(target, ".re-template.yml")); len(m) > 0 {
		return m
	}
	return map[string]string{}
}

func detectRepoRoot(target string) string {
	metadata := readCaseMetadata(target)
	if metadata["templateRoot"] != "" {
		return metadata["templateRoot"]
	}
	cwd, _ := os.Getwd()
	return cwd
}

func readSimpleYAML(path string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		out[key] = val
	}
	return out
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "-")
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(strings.ToLower(b.String()), "-_.")
}

func sessionExists(opts Options, name string) bool {
	p, err := makePaths(opts, name)
	if err != nil {
		return false
	}
	return stateExists(p)
}

func stateExists(p Paths) bool {
	_, err := os.Stat(filepath.Join(p.SessionRoot, "state.json"))
	return err == nil
}

func makePaths(opts Options, name string) (Paths, error) {
	caseRoot, err := filepath.Abs(filepath.Clean(opts.Target))
	if err != nil {
		return Paths{}, err
	}
	repoRoot, err := filepath.Abs(filepath.Clean(opts.RepoRoot))
	if err != nil {
		return Paths{}, err
	}
	if !isAttachedCase(caseRoot) {
		return Paths{}, fmt.Errorf("parallel 需要在已 attach/init 的 case 中使用：%s", caseRoot)
	}
	manifest := loadPackManifestFromRoots(repoRoot, opts.Pack)
	sessionRootBase, workspaceRootBase, err := sessionAndWorkspaceRoots(caseRoot, manifest)
	if err != nil {
		return Paths{}, err
	}
	sessionRoot := filepath.Join(sessionRootBase, name)
	workspace := filepath.Join(workspaceRootBase, workspaceName(name, manifest))
	if st, err := loadStateFrom(sessionRoot); err == nil && st.Workspace != "" {
		loadedWorkspace := st.Workspace
		if !filepath.IsAbs(loadedWorkspace) {
			loadedWorkspace = filepath.Join(caseRoot, loadedWorkspace)
		}
		loadedWorkspace, err = filepath.Abs(filepath.Clean(loadedWorkspace))
		if err != nil {
			return Paths{}, err
		}
		if !isChildPath(caseRoot, loadedWorkspace) {
			return Paths{}, fmt.Errorf("parallel workspace escapes case root: %s", loadedWorkspace)
		}
		workspace = loadedWorkspace
	}
	if !isChildPath(caseRoot, sessionRoot) || !isChildPath(caseRoot, workspace) {
		return Paths{}, fmt.Errorf("parallel paths must stay inside case root: %s", caseRoot)
	}
	return Paths{CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: opts.Pack, Name: name, SessionRoot: sessionRoot, Workspace: workspace}, nil
}

func isAttachedCase(caseRoot string) bool {
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "instance.yml")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".re-template.yml")); err == nil {
		return true
	}
	return false
}

func initSession(opts Options, name string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	if stateExists(p) && !opts.Force {
		fmt.Printf("parallel session `%s` 已存在。下面是当前状态：\n\n", name)
		return smartSession(opts, name)
	}
	if opts.WhatIf {
		fmt.Printf("would create parallel session `%s`\n", name)
		fmt.Printf("- session root: %s\n", rel(p.CaseRoot, p.SessionRoot))
		fmt.Printf("- workspace: %s\n", rel(p.CaseRoot, p.Workspace))
		for path := range configuredInitialFiles(p, State{Name: name, Kind: opts.Kind}) {
			fmt.Printf("- file: %s\n", rel(p.CaseRoot, path))
		}
		return nil
	}
	for _, dir := range []string{
		p.SessionRoot, filepath.Join(p.SessionRoot, "prompts"), filepath.Join(p.SessionRoot, "agents"), filepath.Join(p.SessionRoot, "inbox"), filepath.Join(p.SessionRoot, "outbox"), filepath.Join(p.SessionRoot, "checkpoints"), filepath.Join(p.SessionRoot, "reviews"),
		p.Workspace, filepath.Join(p.Workspace, "prompts"), filepath.Join(p.Workspace, "inbox"), filepath.Join(p.Workspace, "outbox"), filepath.Join(p.Workspace, "candidates"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	now := nowText()
	manifest := loadPackManifestForPaths(p)
	status := nonEmpty(manifest.ParallelDefaults.InitialStatus, "open")
	lifecycleMode := nonEmpty(manifest.ParallelDefaults.DefaultLifecycleMode, "attached_to_main")
	st := State{SchemaVersion: schemaVersion, Name: name, Kind: opts.Kind, Status: status, LifecycleMode: lifecycleMode, CaseRoot: p.CaseRoot, RepoRoot: p.RepoRoot, Pack: p.Pack, Workspace: rel(p.CaseRoot, p.Workspace), CreatedAt: now, UpdatedAt: now, Checkpoint: map[string]string{"nextAction": "把 START_HERE.md 发给功能会话，或继续执行 /rekit parallel " + name}}
	if err := writeState(p, st); err != nil {
		return err
	}
	appendTimeline(p, "created", map[string]any{"kind": opts.Kind, "workspace": rel(p.CaseRoot, p.Workspace)})
	if err := writeInitialArtifacts(p, st); err != nil {
		return err
	}
	if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	fmt.Printf("已创建 parallel session `%s`。\n", name)
	fmt.Printf("- 工作区：%s\n", rel(p.CaseRoot, p.Workspace))
	fmt.Printf("- 新功能会话启动提示词：%s\n", rel(p.CaseRoot, filepath.Join(p.Workspace, "START_HERE.md")))
	fmt.Printf("\n下一步：把 START_HERE.md 发给另一个 Claude Code 会话即可开始功能分析。\n")
	return nil
}

func writeInitialArtifacts(p Paths, st State) error {
	files := configuredInitialFiles(p, st)
	if len(files) == 0 {
		files = map[string]string{
			filepath.Join(p.Workspace, "summary.md"):                         templateText(p, "summary.template.md", summaryTemplate(st), st),
			filepath.Join(p.Workspace, "evidence.md"):                        templateText(p, "evidence.template.md", evidenceTemplate(st), st),
			filepath.Join(p.Workspace, "notes.md"):                           notesTemplate(st),
			filepath.Join(p.Workspace, "vm_blockers.csv"):                    templateText(p, "vm_blockers.template.csv", "blocker_id,feature,rva,va,kind,evidence,need,status,owner,notes\n", st),
			filepath.Join(p.Workspace, "lowering_requests.csv"):              templateText(p, "lowering_requests.template.csv", "request_id,feature,rva,handler,reason,evidence,priority,status,main_response,notes\n", st),
			filepath.Join(p.Workspace, "candidates", "handler_roles.csv"):    templateText(p, "handler_roles.template.csv", "feature,handler,role,confidence,evidence,status,notes\n", st),
			filepath.Join(p.Workspace, "candidates", "opcode_semantics.csv"): templateText(p, "opcode_semantics.template.csv", "feature,handler,opcode,semantics,confidence,evidence,status,notes\n", st),
		}
	}
	files[filepath.Join(p.Workspace, "outbox", "to-main.jsonl")] = ""
	files[filepath.Join(p.Workspace, "inbox", "from-main.jsonl")] = ""
	for path, text := range files {
		if err := writeIfMissing(path, text); err != nil {
			return err
		}
	}
	return nil
}

func templateText(p Paths, templateName string, fallback string, st State) string {
	path := filepath.Join(p.RepoRoot, "packs", p.Pack, "templates", "parallel", st.Kind, templateName)
	b, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	text := string(b)
	text = strings.ReplaceAll(text, "<FEATURE_NAME>", st.Name)
	text = strings.ReplaceAll(text, "<CASE_ROOT>", p.CaseRoot)
	text = strings.ReplaceAll(text, "<WORKSPACE>", rel(p.CaseRoot, p.Workspace))
	return text
}

func smartSession(opts Options, name string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	st, err := loadState(p)
	if err != nil {
		return err
	}
	if opts.WhatIf {
		if err := scanSessionFiles(p, &st); err != nil {
			return err
		}
	} else if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	fmt.Printf("parallel session `%s`\n", name)
	printStateSummary(p, st)
	fmt.Println("\n建议：")
	for _, rec := range recommendations(st) {
		fmt.Printf("- %s\n", rec)
	}
	fmt.Println("\n常用下一步：继续输入 `/rekit parallel " + name + "` 可重新评估；需要显式动作时可用 `collect`、`sync`、`resume`、`close`。")
	return nil
}

func overview(opts Options) error {
	caseRoot := filepath.Clean(opts.Target)
	if !isAttachedCase(caseRoot) {
		return fmt.Errorf("parallel 总览需要在已 attach/init 的 case 中使用：%s", caseRoot)
	}
	manifest := loadPackManifestFromRoots(opts.RepoRoot, opts.Pack)
	root, _, err := sessionAndWorkspaceRoots(caseRoot, manifest)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("当前 case 还没有 parallel session。")
			fmt.Println("创建示例：/rekit parallel login")
			return nil
		}
		return err
	}
	fmt.Println("parallel sessions：")
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		p, err := makePaths(opts, name)
		if err != nil {
			continue
		}
		st, err := loadState(p)
		if err != nil {
			continue
		}
		_ = scanSessionFiles(p, &st)
		fmt.Printf("- %s：%s / %s，未处理 request=%d，候选=%d\n", name, st.Status, st.LifecycleMode, st.Counters.UnresolvedRequests, st.Counters.CandidateRows)
		count++
	}
	if count == 0 {
		fmt.Println("- 无。创建示例：/rekit parallel login")
	}
	return nil
}

func showSession(opts Options, name string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	st, err := loadState(p)
	if err != nil {
		return err
	}
	if opts.WhatIf {
		if err := scanSessionFiles(p, &st); err != nil {
			return err
		}
	} else if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	printStateSummary(p, st)
	return nil
}

func collectSession(opts Options, name string, command string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	st, err := loadState(p)
	if err != nil {
		return err
	}
	if opts.WhatIf {
		if err := scanSessionFiles(p, &st); err != nil {
			return err
		}
	} else if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	reviewRoot, err := reviewRootPath(opts, p, command, name)
	if err != nil {
		return err
	}
	st.Status = "needs-review"
	st.LastCollectAt = nowText()
	st.LastReviewRoot = reviewRoot
	st.UpdatedAt = nowText()
	packet, artifacts := buildPacket(p, st, command)
	if opts.WhatIf {
		fmt.Printf("would generate %s package: %s\n", command, rel(p.CaseRoot, reviewRoot))
		return nil
	}
	if err := os.MkdirAll(reviewRoot, 0755); err != nil {
		return err
	}
	packetFile, err := packetPath(opts, reviewRoot)
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(packet, "", "  ")
	if err := os.WriteFile(packetFile, b, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(summaryPath(reviewRoot), []byte(reviewSummary(p, st, command, artifacts)), 0644); err != nil {
		return err
	}
	if err := writeState(p, st); err != nil {
		return err
	}
	appendTimeline(p, command, map[string]any{"reviewRoot": rel(p.CaseRoot, reviewRoot), "unresolvedRequests": st.Counters.UnresolvedRequests})
	if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	fmt.Printf("已生成 %s 包：%s\n", command, rel(p.CaseRoot, reviewRoot))
	fmt.Println("主线会话读取 packet.json / summary.md 后决定如何消费 request/candidate。")
	return nil
}

func syncSession(opts Options, name string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	st, err := loadState(p)
	if err != nil {
		return err
	}
	if opts.WhatIf {
		if err := scanSessionFiles(p, &st); err != nil {
			return err
		}
	} else if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	note := opts.Note
	if note == "" {
		note = fmt.Sprintf("主线同步检查：当前未处理 lowering request=%d，候选行=%d。若主线已补齐语义，请在 summary.md/evidence.md 中继续验证，不要直接写 canonical 文件。", st.Counters.UnresolvedRequests, st.Counters.CandidateRows)
	}
	msg := map[string]any{"time": nowText(), "event": "main-sync", "note": note, "status": st.Status}
	if opts.WhatIf {
		fmt.Printf("would write feature inbox: %s\n", rel(p.CaseRoot, filepath.Join(p.Workspace, "inbox", "from-main.jsonl")))
		return nil
	}
	if err := appendJSONL(filepath.Join(p.Workspace, "inbox", "from-main.jsonl"), msg); err != nil {
		return err
	}
	st.Status = "synced"
	st.UpdatedAt = nowText()
	appendTimeline(p, "sync", msg)
	if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	fmt.Printf("已写入功能会话 inbox：%s\n", rel(p.CaseRoot, filepath.Join(p.Workspace, "inbox", "from-main.jsonl")))
	fmt.Println("功能会话重启时使用 FEATURE_RESUME.md，会看到这轮同步入口。")
	return nil
}

func promoteSession(opts Options, name string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	st, err := loadState(p)
	if err != nil {
		return err
	}
	if opts.WhatIf {
		if err := scanSessionFiles(p, &st); err != nil {
			return err
		}
	} else if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	reviewRoot, err := reviewRootPath(opts, p, "parallel-promote", name)
	if err != nil {
		return err
	}
	st.LastReviewRoot = reviewRoot
	packet, artifacts := buildPacket(p, st, "parallel-promote")
	packet.Recommendations = append(packet.Recommendations,
		"只把通用提示词/流程/工具经验回流 common 或 pack；样本路径、RVA/VA、trace/captures/artifacts 留在 case。",
		"此命令只生成审查包，不直接写 pack。确认后再用 /rekit promote 的 review-first 流程处理。",
	)
	if opts.WhatIf {
		fmt.Printf("would generate parallel promote package: %s\n", rel(p.CaseRoot, reviewRoot))
		return nil
	}
	if err := os.MkdirAll(reviewRoot, 0755); err != nil {
		return err
	}
	packetFile, err := packetPath(opts, reviewRoot)
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(packet, "", "  ")
	if err := os.WriteFile(packetFile, b, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(summaryPath(reviewRoot), []byte(reviewSummary(p, st, "parallel-promote", artifacts)), 0644); err != nil {
		return err
	}
	appendTimeline(p, "promote-review", map[string]any{"reviewRoot": rel(p.CaseRoot, reviewRoot)})
	fmt.Printf("已生成 parallel promote 审查包：%s\n", rel(p.CaseRoot, reviewRoot))
	return nil
}

func closeSession(opts Options, name string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	st, err := loadState(p)
	if err != nil {
		return err
	}
	if opts.WhatIf {
		if err := scanSessionFiles(p, &st); err != nil {
			return err
		}
	} else if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	if (st.Counters.UnresolvedRequests > 0 || st.Counters.CandidateUnresolvedRows > 0 || st.Counters.OutboxMessages > 0) && !opts.Force {
		if opts.WhatIf {
			fmt.Printf("would mark `%s` as needs-authority because pending work exists\n", name)
			return nil
		}
		st.Status = "needs-authority"
		st.UpdatedAt = nowText()
		_ = writeState(p, st)
		_ = refreshSessionFiles(p, &st)
		fmt.Printf("`%s` 还有未处理项，未关闭。\n", name)
		fmt.Printf("- 未处理 lowering request：%d\n", st.Counters.UnresolvedRequests)
		fmt.Printf("- 未处理 candidate 行：%d\n", st.Counters.CandidateUnresolvedRows)
		fmt.Printf("- outbox 消息：%d\n", st.Counters.OutboxMessages)
		fmt.Println("如主线已结束，可继续 standalone；若确认强制关闭，使用 /rekit parallel " + name + " close --force。")
		return nil
	}
	if opts.WhatIf {
		fmt.Printf("would close parallel session `%s`\n", name)
		return nil
	}
	st.Status = "closed"
	st.UpdatedAt = nowText()
	if err := writeState(p, st); err != nil {
		return err
	}
	appendTimeline(p, "closed", map[string]any{"force": opts.Force})
	if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	fmt.Printf("parallel session `%s` 已关闭。\n", name)
	return nil
}

func resumeSession(opts Options, name string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	st, err := loadState(p)
	if err != nil {
		return err
	}
	if opts.WhatIf {
		if err := scanSessionFiles(p, &st); err != nil {
			return err
		}
		fmt.Printf("would refresh resume prompts for `%s`\n", name)
		return nil
	}
	if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	fmt.Printf("已刷新续接提示词：\n- %s\n- %s\n- %s\n", rel(p.CaseRoot, filepath.Join(p.Workspace, "prompts", "FEATURE_RESUME.md")), rel(p.CaseRoot, filepath.Join(p.SessionRoot, "prompts", "MAIN_RESUME.md")), rel(p.CaseRoot, filepath.Join(p.SessionRoot, "prompts", "AUTHORITY_RESUME.md")))
	return nil
}

func setStandalone(opts Options, name string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	st, err := loadState(p)
	if err != nil {
		return err
	}
	if opts.WhatIf {
		fmt.Printf("would switch `%s` to standalone\n", name)
		return nil
	}
	st.LifecycleMode = "standalone"
	st.Status = "standalone"
	st.UpdatedAt = nowText()
	if err := writeState(p, st); err != nil {
		return err
	}
	appendTimeline(p, "standalone", map[string]any{"reason": "main session completed or detached"})
	if err := refreshSessionFiles(p, &st); err != nil {
		return err
	}
	fmt.Printf("`%s` 已切换为 standalone。功能会话可继续分析，但 canonical 文件仍保持只读。\n", name)
	return nil
}

func doctorAll(opts Options) error {
	caseRoot := filepath.Clean(opts.Target)
	manifest := loadPackManifestFromRoots(opts.RepoRoot, opts.Pack)
	root, _, err := sessionAndWorkspaceRoots(caseRoot, manifest)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("parallel doctor：无 session。ok")
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := doctorSession(opts, e.Name()); err != nil {
				return err
			}
		}
	}
	fmt.Println("parallel doctor ok")
	return nil
}

func doctorSession(opts Options, name string) error {
	p, err := makePaths(opts, name)
	if err != nil {
		return err
	}
	st, err := loadState(p)
	if err != nil {
		return err
	}
	if !isChildPath(p.CaseRoot, p.SessionRoot) || !isChildPath(p.CaseRoot, p.Workspace) {
		return fmt.Errorf("session path escapes case root: %s", name)
	}
	required := configuredRequiredFiles(p, st)
	for _, path := range required {
		if info, err := os.Stat(path); err != nil {
			return fmt.Errorf("missing required file: %s", path)
		} else if info.Size() == 0 && strings.HasSuffix(path, ".md") {
			return fmt.Errorf("empty required markdown: %s", path)
		}
	}
	fmt.Printf("%s ok\n", name)
	return nil
}

func loadState(p Paths) (State, error) { return loadStateFrom(p.SessionRoot) }
func loadStateFrom(sessionRoot string) (State, error) {
	b, err := os.ReadFile(filepath.Join(sessionRoot, "state.json"))
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, err
	}
	return st, nil
}

func writeState(p Paths, st State) error {
	st.UpdatedAt = nowText()
	b, _ := json.MarshalIndent(st, "", "  ")
	return os.WriteFile(filepath.Join(p.SessionRoot, "state.json"), b, 0644)
}

func scanSessionFiles(p Paths, st *State) error {
	counts, err := scanCounters(p)
	if err != nil {
		return err
	}
	st.Counters = counts
	return nil
}

func refreshSessionFiles(p Paths, st *State) error {
	if err := scanSessionFiles(p, st); err != nil {
		return err
	}
	st.UpdatedAt = nowText()
	if st.Checkpoint == nil {
		st.Checkpoint = map[string]string{}
	}
	st.Checkpoint["nextAction"] = nextAction(*st)
	if err := writeState(p, *st); err != nil {
		return err
	}
	checkpoint := map[string]any{"schemaVersion": schemaVersion, "name": st.Name, "status": st.Status, "lifecycleMode": st.LifecycleMode, "workspace": rel(p.CaseRoot, p.Workspace), "updatedAt": st.UpdatedAt, "counters": st.Counters, "nextAction": st.Checkpoint["nextAction"], "importantFiles": importantFiles(p)}
	b, _ := json.MarshalIndent(checkpoint, "", "  ")
	if err := os.WriteFile(filepath.Join(p.SessionRoot, "checkpoints", "latest.json"), b, 0644); err != nil {
		return err
	}
	prompts := map[string]string{
		filepath.Join(p.Workspace, "START_HERE.md"):                         startHerePrompt(p, *st),
		filepath.Join(p.Workspace, "prompts", "FEATURE_RESUME.md"):          featureResumePrompt(p, *st),
		filepath.Join(p.SessionRoot, "prompts", "FEATURE_RESUME.md"):        featureResumePrompt(p, *st),
		filepath.Join(p.SessionRoot, "prompts", "MAIN_RESUME.md"):           mainResumePrompt(p, *st),
		filepath.Join(p.SessionRoot, "prompts", "AUTHORITY_RESUME.md"):      authorityResumePrompt(p, *st),
		filepath.Join(p.SessionRoot, "prompts", "STANDALONE_RESUME.md"):     standaloneResumePrompt(p, *st),
		filepath.Join(p.Workspace, "prompts", "STANDALONE_RESUME.md"):       standaloneResumePrompt(p, *st),
		filepath.Join(p.SessionRoot, "agents", "feature-analysis.agent.md"): featureAgentSpec(p, *st),
		filepath.Join(p.SessionRoot, "agents", "merge-review.agent.md"):     mergeReviewAgentSpec(p, *st),
	}
	for path, text := range prompts {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			return err
		}
	}
	return nil
}

func scanCounters(p Paths) (Counters, error) {
	return scanConfiguredCounters(p)
}

func countCSVStatus(path, statusCol string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil || len(rows) <= 1 {
		return 0, 0
	}
	idx := -1
	for i, h := range rows[0] {
		if strings.EqualFold(strings.TrimSpace(h), statusCol) {
			idx = i
			break
		}
	}
	total, unresolved := 0, 0
	for _, row := range rows[1:] {
		blank := true
		for _, c := range row {
			if strings.TrimSpace(c) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		total++
		status := ""
		if idx >= 0 && idx < len(row) {
			status = strings.ToLower(strings.TrimSpace(row[idx]))
		}
		if status == "" || !(status == "resolved" || status == "done" || status == "closed" || status == "accepted" || status == "rejected") {
			unresolved++
		}
	}
	return total, unresolved
}

func countLines(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func buildPacket(p Paths, st State, command string) (Packet, []Artifact) {
	arts := []Artifact{}
	for _, item := range importantFilesForState(p, st) {
		path := filepath.Join(p.CaseRoot, filepath.FromSlash(item))
		info, err := os.Stat(path)
		rows := 0
		if strings.HasSuffix(strings.ToLower(path), ".csv") {
			rows, _ = countCSVStatus(path, "status")
		}
		arts = append(arts, Artifact{Path: item, Kind: artifactKind(item), Exists: err == nil, Bytes: func() int64 {
			if err == nil {
				return info.Size()
			}
			return 0
		}(), Rows: rows})
	}
	manifest := loadPackManifestForPaths(p)
	return Packet{SchemaVersion: schemaVersion, PacketKind: "rekit-review", Command: command, Direction: "feature-to-authority", CreatedAt: nowText(), CaseRoot: p.CaseRoot, RepoRoot: p.RepoRoot, Pack: p.Pack, ManifestPath: manifest.Path, ManifestVersion: manifest.Version, ReviewRoot: rel(p.CaseRoot, st.LastReviewRoot), IsMutation: false, WritesReviewArtifacts: true, ReviewRequired: true, Producer: "rekit-go/parallel", Session: st, Summary: map[string]any{"unresolvedRequests": st.Counters.UnresolvedRequests, "candidateRows": st.Counters.CandidateRows, "candidateUnresolvedRows": st.Counters.CandidateUnresolvedRows, "outboxMessages": st.Counters.OutboxMessages, "reviewRequired": true}, Artifacts: arts, Recommendations: recommendations(st)}, arts
}

func importantFiles(p Paths) []string {
	return importantFilesForState(p, State{Kind: "feature-analysis"})
}

func importantFilesForState(p Paths, st State) []string {
	return configuredReviewFiles(p, st)
}

func artifactKind(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(base, "lowering"):
		return "lowering-requests"
	case strings.Contains(base, "blocker"):
		return "vm-blockers"
	case strings.Contains(path, "candidates"):
		return "candidate"
	case strings.Contains(base, "summary"):
		return "summary"
	case strings.Contains(base, "evidence"):
		return "evidence"
	default:
		return "support"
	}
}

func recommendations(st State) []string {
	recs := []string{}
	if st.Status == "closed" {
		return []string{"该 session 已关闭；如需继续，重新创建或使用 --force init。"}
	}
	if st.Counters.UnresolvedRequests > 0 {
		recs = append(recs, fmt.Sprintf("有 %d 条未处理 lowering request，建议主线运行 collect/review 后优先判断。", st.Counters.UnresolvedRequests))
	}
	if st.Counters.CandidateRows > 0 {
		recs = append(recs, fmt.Sprintf("有 %d 行 candidate，合入前必须 review-first，不要直接写 canonical 文件。", st.Counters.CandidateRows))
	}
	if st.Counters.OutboxMessages > 0 {
		recs = append(recs, "功能会话 outbox 有消息，建议 collect。")
	}
	if st.LifecycleMode == "standalone" {
		recs = append(recs, "当前为 standalone：功能会话可继续，但 canonical 写入需要临时 authority session。")
	}
	if len(recs) == 0 {
		recs = append(recs, "暂无必须同步项；功能会话可继续分析，或用 resume 生成续接提示词。")
	}
	return recs
}

func nextAction(st State) string {
	if st.Counters.UnresolvedRequests > 0 || st.Counters.OutboxMessages > 0 {
		return "主线运行 /rekit parallel " + st.Name + " collect，审查功能会话请求。"
	}
	if st.LifecycleMode == "standalone" {
		return "功能会话继续 native/证据分析；需要底层权威时使用 AUTHORITY_RESUME.md。"
	}
	return "功能会话继续使用 FEATURE_RESUME.md 推进；主线可定期 /rekit parallel " + st.Name + "。"
}

func printStateSummary(p Paths, st State) {
	fmt.Printf("- 状态：%s / %s\n", st.Status, st.LifecycleMode)
	fmt.Printf("- 工作区：%s\n", rel(p.CaseRoot, st.Workspace))
	fmt.Printf("- lowering requests：%d（未处理 %d）\n", st.Counters.LoweringRequests, st.Counters.UnresolvedRequests)
	fmt.Printf("- VM blockers：%d\n", st.Counters.VMBlockers)
	fmt.Printf("- candidate 行：%d\n", st.Counters.CandidateRows)
	fmt.Printf("- inbox/outbox 消息：%d / %d\n", st.Counters.InboxMessages, st.Counters.OutboxMessages)
	fmt.Printf("- 续接提示词：%s\n", rel(p.CaseRoot, filepath.Join(p.Workspace, "prompts", "FEATURE_RESUME.md")))
}

func reviewSummary(p Paths, st State, command string, artifacts []Artifact) string {
	var b strings.Builder
	b.WriteString("# rekit parallel review\n\n")
	b.WriteString("- command: `" + command + "`\n")
	b.WriteString("- session: `" + st.Name + "`\n")
	b.WriteString("- status: `" + st.Status + " / " + st.LifecycleMode + "`\n")
	b.WriteString("- workspace: `" + rel(p.CaseRoot, st.Workspace) + "`\n")
	b.WriteString(fmt.Sprintf("- unresolved lowering requests: `%d`\n", st.Counters.UnresolvedRequests))
	b.WriteString(fmt.Sprintf("- candidate rows: `%d`\n\n", st.Counters.CandidateRows))
	b.WriteString("## Artifacts\n\n")
	for _, a := range artifacts {
		b.WriteString(fmt.Sprintf("- `%s` — %s, exists=%v, rows=%d\n", a.Path, a.Kind, a.Exists, a.Rows))
	}
	b.WriteString("\n## 建议\n\n")
	for _, r := range recommendations(st) {
		b.WriteString("- " + r + "\n")
	}
	b.WriteString("\n主线会话应只把 request/candidate 作为候选输入；confirmed CSV、handoff、IDA 写入仍由 authority/main session 单写者处理。\n")
	return b.String()
}

func appendTimeline(p Paths, event string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["time"] = nowText()
	data["event"] = event
	_ = appendJSONL(filepath.Join(p.SessionRoot, "timeline.jsonl"), data)
}

func appendJSONL(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, _ := json.Marshal(value)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func writeIfMissing(path, text string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0644)
}

func nowText() string { return time.Now().Format(time.RFC3339) }

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

func isChildPath(root, path string) bool {
	rootAbs, _ := filepath.Abs(root)
	pathAbs, _ := filepath.Abs(path)
	rootAbs = filepath.Clean(rootAbs)
	pathAbs = filepath.Clean(pathAbs)
	if strings.EqualFold(rootAbs, pathAbs) {
		return true
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func samePath(a, b string) bool {
	aAbs, _ := filepath.Abs(a)
	bAbs, _ := filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(aAbs), filepath.Clean(bAbs))
}

func summaryTemplate(st State) string {
	return "# 功能分析摘要：" + st.Name + "\n\n## 当前结论\n\n- 待填写。\n\n## 已确认事实\n\n- 待填写；每条结论必须有 evidence.md 或文件/地址证据。\n\n## 未确认假设\n\n- 待填写。\n\n## 下一步\n\n- 按 START_HERE.md 继续。\n"
}
func evidenceTemplate(st State) string {
	return "# 证据记录：" + st.Name + "\n\n| 结论 | 证据 | 来源 | 可信度 |\n|---|---|---|---|\n| 待填写 |  |  |  |\n"
}
func notesTemplate(st State) string {
	return "# Notes\n\n记录废弃路线、待验证点和可能可回流模板的通用经验。不要写入 captures/artifacts 大表内容。\n"
}

func mdCode(value string) string {
	return "`" + value + "`"
}

func startHerePrompt(p Paths, st State) string {
	if text, ok := renderPromptTemplate(p, st, "START_HERE.template.md"); ok {
		return text
	}
	workspace := rel(p.CaseRoot, p.Workspace)
	return fmt.Sprintf("# rekit parallel START_HERE：%s\n\n"+
		"你是当前 case 的功能分析会话，负责分析 **%s**。不要从零脱壳，不要覆盖主线 canonical 文件。\n\n"+
		"## 工作边界\n\n"+
		"- 可以自由做功能级探索、写脚本、生成中间产物。\n"+
		"- 所有产物写入：%s\n"+
		"- 不要修改：%s、%s、%s、共享 IDB 注释/类型。\n"+
		"- 遇到 VMProtected 阻塞点，记录到 %s 或 %s，不要硬猜。\n\n"+
		"## 先读\n\n"+
		"1. 项目 %s\n"+
		"2. %s\n"+
		"3. 本目录 %s、%s、%s\n\n"+
		"## 输出要求\n\n"+
		"- %s：功能流程、关键地址、已确认结论。\n"+
		"- %s：证据和来源。\n"+
		"- %s：需要主线补 VM 语义的请求。\n"+
		"- %s：需要主线关注的简短消息。\n\n"+
		"如果上下文污染或第二天重开，使用 %s 续接。\n",
		st.Name,
		st.Name,
		mdCode(workspace),
		mdCode("captures/vm_opcode_semantics_confirmed.csv"),
		mdCode("captures/vm_handler_roles_confirmed.csv"),
		mdCode("references/vmp-re/task-handoff.md"),
		mdCode("lowering_requests.csv"),
		mdCode("vm_blockers.csv"),
		mdCode("CLAUDE.local.md"),
		mdCode("references/vmp-re/README.md"),
		mdCode("summary.md"),
		mdCode("evidence.md"),
		mdCode("lowering_requests.csv"),
		mdCode("summary.md"),
		mdCode("evidence.md"),
		mdCode("lowering_requests.csv"),
		mdCode("outbox/to-main.jsonl"),
		mdCode("prompts/FEATURE_RESUME.md"),
	)
}

func featureResumePrompt(p Paths, st State) string {
	if text, ok := renderPromptTemplate(p, st, "FEATURE_RESUME.template.md"); ok {
		return text
	}
	return fmt.Sprintf("# FEATURE_RESUME：%s\n\n"+
		"你是功能分析会话的续接。不要从零开始。\n\n"+
		"## 读取顺序\n\n"+
		"1. %s\n"+
		"2. %s\n"+
		"3. %s\n"+
		"4. %s\n"+
		"5. %s\n\n"+
		"## 当前状态\n\n"+
		"- status: %s\n"+
		"- lifecycle: %s\n"+
		"- unresolved lowering requests: %s\n"+
		"- candidate rows: %s\n\n"+
		"## 下一步\n\n%s\n\n"+
		"## 边界\n\n"+
		"继续把产物写入工作区，不要写 canonical 文件。需要主线处理的内容写入 %s 或 %s。\n",
		st.Name,
		mdCode(rel(p.CaseRoot, filepath.Join(p.SessionRoot, "checkpoints", "latest.json"))),
		mdCode(rel(p.CaseRoot, filepath.Join(p.Workspace, "summary.md"))),
		mdCode(rel(p.CaseRoot, filepath.Join(p.Workspace, "evidence.md"))),
		mdCode(rel(p.CaseRoot, filepath.Join(p.Workspace, "lowering_requests.csv"))),
		mdCode(rel(p.CaseRoot, filepath.Join(p.Workspace, "inbox", "from-main.jsonl"))),
		mdCode(st.Status),
		mdCode(st.LifecycleMode),
		mdCode(fmt.Sprintf("%d", st.Counters.UnresolvedRequests)),
		mdCode(fmt.Sprintf("%d", st.Counters.CandidateRows)),
		nextAction(st),
		mdCode("outbox/to-main.jsonl"),
		mdCode("lowering_requests.csv"),
	)
}

func mainResumePrompt(p Paths, st State) string {
	if text, ok := renderPromptTemplate(p, st, "MAIN_RESUME.template.md"); ok {
		return text
	}
	return fmt.Sprintf("# MAIN_RESUME：%s\n\n"+
		"你是主线/authority 会话的续接。目标是消费功能会话的 request/candidate，而不是重做功能分析。\n\n"+
		"## 读取\n\n"+
		"- %s\n"+
		"- %s\n"+
		"- 最近 review packet（如有）：%s\n\n"+
		"## 当前状态\n\n"+
		"- unresolved lowering requests: %s\n"+
		"- candidate rows: %s\n\n"+
		"## 职责\n\n"+
		"- 只把功能会话产物当候选证据。\n"+
		"- confirmed CSV、routine IR、handoff 更新仍由本会话单写者处理。\n"+
		"- 处理完后运行 %s 或写明主线响应。\n",
		st.Name,
		mdCode(rel(p.CaseRoot, filepath.Join(p.SessionRoot, "state.json"))),
		mdCode(rel(p.CaseRoot, filepath.Join(p.Workspace, "lowering_requests.csv"))),
		mdCode(rel(p.CaseRoot, st.LastReviewRoot)),
		mdCode(fmt.Sprintf("%d", st.Counters.UnresolvedRequests)),
		mdCode(fmt.Sprintf("%d", st.Counters.CandidateRows)),
		mdCode("/rekit parallel "+st.Name+" sync"),
	)
}

func authorityResumePrompt(p Paths, st State) string {
	if text, ok := renderPromptTemplate(p, st, "AUTHORITY_RESUME.template.md"); ok {
		return text
	}
	return fmt.Sprintf("# AUTHORITY_RESUME：%s\n\n"+
		"当主线已结束但功能会话遇到 VM 阻塞时，用此提示词开临时 authority 会话。\n\n"+
		"读取：\n\n"+
		"- %s\n"+
		"- %s\n"+
		"- %s\n\n"+
		"任务：只处理功能会话提交的高价值 lowering request。处理后不要直接修改功能结论；通过 %s 回传。\n",
		st.Name,
		mdCode(rel(p.CaseRoot, filepath.Join(p.SessionRoot, "checkpoints", "latest.json"))),
		mdCode(rel(p.CaseRoot, filepath.Join(p.Workspace, "lowering_requests.csv"))),
		mdCode("references/vmp-re/task-handoff.md"),
		mdCode("/rekit parallel "+st.Name+" sync"),
	)
}

func standaloneResumePrompt(p Paths, st State) string {
	if text, ok := renderPromptTemplate(p, st, "STANDALONE_RESUME.template.md"); ok {
		return text
	}
	return fmt.Sprintf("# STANDALONE_RESUME：%s\n\n"+
		"主线可能已经完成或暂时不可用。功能会话可以继续 standalone 分析：native 周边、字符串/import/xref、证据整理、候选 request。\n\n"+
		"限制：不要写 canonical 文件；需要底层 VM 语义时写 %s，未来用 AUTHORITY_RESUME.md 处理。\n\n"+
		"工作区：%s\n",
		st.Name,
		mdCode("lowering_requests.csv"),
		mdCode(rel(p.CaseRoot, p.Workspace)),
	)
}

func featureAgentSpec(p Paths, st State) string {
	return "# feature-analysis agent spec\n\n" + featureResumePrompt(p, st)
}

func mergeReviewAgentSpec(p Paths, st State) string {
	return "# merge-review agent spec\n\n请审查功能会话产物，区分 confirmed/candidate/request/case-only。不要写文件，输出高置信合并建议。\n\n" + mainResumePrompt(p, st)
}
