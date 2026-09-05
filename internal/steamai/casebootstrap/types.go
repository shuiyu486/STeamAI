package casebootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const ConfirmationPrefix = "CONFIRM STEAMAI FRESH "

var (
	ErrConfirmationRequired = errors.New("需要与当前预览完全匹配的 Fresh 确认")
	ErrSourceDrift          = errors.New("STeamAI source 在预览后发生变化")
	ErrTargetDrift          = errors.New("case 目标在预览后发生变化")
	ErrPartialCase          = errors.New("当前目录包含不完整或冲突的 STeamAI 状态")
	ErrCollision            = errors.New("Fresh 目标或 source 存在冲突")
	packNamePattern         = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	memberNamePattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	hexIdentityPattern      = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	shaPattern              = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reviewerMarkdownPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}\.md$`)
)

type Facts struct {
	Name          string        `json:"name"`
	Goal          string        `json:"goal"`
	Authorization string        `json:"authorization"`
	Prohibited    string        `json:"prohibited"`
	Stop          string        `json:"stop"`
	Pack          string        `json:"pack"`
	Members       []MemberFacts `json:"members"`
}

type MemberFacts struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Role           string `json:"role"`
	Responsibility string `json:"responsibility"`
	TaskGoal       string `json:"taskGoal"`
	Inputs         string `json:"inputs"`
	AllowedReads   string `json:"allowedReads"`
	AllowedWrites  string `json:"allowedWrites"`
	Deliverables   string `json:"deliverables"`
	StopOrEscalate string `json:"stopOrEscalate"`
	ExitConditions string `json:"exitConditions"`
}

type SourceRecord struct {
	Path        string `json:"path"`
	GitMode     string `json:"gitMode"`
	HeadBlob    string `json:"headBlob"`
	ContentBlob string `json:"contentBlob"`
	SHA256      string `json:"sha256"`
	Bytes       int    `json:"bytes"`
	Changed     bool   `json:"changed"`
	Data        []byte `json:"-"`
}

type PlannedWrite struct {
	SourceKind      string `json:"sourceKind"`
	SourcePath      string `json:"sourcePath"`
	GitMode         string `json:"gitMode"`
	HeadBlob        string `json:"headBlob"`
	ContentBlob     string `json:"contentBlob"`
	TargetPath      string `json:"targetPath"`
	TargetAction    string `json:"targetAction"`
	TargetPreSHA256 string `json:"targetPreSha256,omitempty"`
	TargetPreBytes  int    `json:"targetPreBytes"`
	SHA256          string `json:"sha256"`
	Bytes           int    `json:"bytes"`
	Data            []byte `json:"-"`
}

type Preview struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Revision       string            `json:"revision"`
	PackTree       string            `json:"packTree"`
	CommonTree     string            `json:"commonTree"`
	SourceDigest   string            `json:"sourceDigest"`
	SnapshotDigest string            `json:"snapshotDigest"`
	Facts          Facts             `json:"facts"`
	SourceRecords  []SourceRecord    `json:"sourceRecords"`
	Writes         []PlannedWrite    `json:"writes"`
	SourceDiff     string            `json:"sourceDiff,omitempty"`
	GeneratedFiles map[string]string `json:"generatedFiles"`
	Identity       string            `json:"identity"`
	HumanPreview   string            `json:"humanPreview"`
}

type frozenSource struct {
	Root       string
	Git        string
	Revision   string
	PackTree   string
	CommonTree string
	Digest     string
	Records    []SourceRecord
	ByPath     map[string]SourceRecord
	Diff       string
}

func DecodeFacts(reader io.Reader) (Facts, error) {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	decoder.DisallowUnknownFields()
	var facts Facts
	if err := decoder.Decode(&facts); err != nil {
		return Facts{}, fmt.Errorf("解析 Fresh facts: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Facts{}, errors.New("Fresh facts 后存在额外 JSON")
		}
		return Facts{}, fmt.Errorf("解析 Fresh facts 结尾: %w", err)
	}
	if err := facts.Validate(); err != nil {
		return Facts{}, err
	}
	return facts, nil
}

func (facts Facts) Validate() error {
	for name, value := range map[string]string{
		"name": facts.Name, "goal": facts.Goal, "authorization": facts.Authorization,
		"prohibited": facts.Prohibited, "stop": facts.Stop,
	} {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\x00\r\n") {
			return fmt.Errorf("Fresh facts 字段 %s 为空、包含换行或无效", name)
		}
	}
	if !packNamePattern.MatchString(facts.Pack) || strings.HasPrefix(facts.Pack, "_") {
		return errors.New("selected pack 名称无效")
	}
	if len(facts.Members) > 4 {
		return errors.New("正式成员超过 3 名执行成员加 1 名 Reviewer")
	}
	seen := map[string]bool{}
	executors, reviewers := 0, 0
	for i, member := range facts.Members {
		if !memberNamePattern.MatchString(member.Name) || seen[member.Name] || windowsReservedName(member.Name) {
			return fmt.Errorf("成员 %d 名称无效、重复或为 Windows 保留名", i+1)
		}
		seen[member.Name] = true
		switch member.Kind {
		case "execution":
			executors++
		case "reviewer":
			reviewers++
		default:
			return fmt.Errorf("成员 %s kind 必须是 execution 或 reviewer", member.Name)
		}
		for field, value := range map[string]string{
			"role": member.Role, "responsibility": member.Responsibility, "taskGoal": member.TaskGoal,
			"inputs": member.Inputs, "allowedReads": member.AllowedReads, "allowedWrites": member.AllowedWrites,
			"deliverables": member.Deliverables, "stopOrEscalate": member.StopOrEscalate,
			"exitConditions": member.ExitConditions,
		} {
			if strings.TrimSpace(value) == "" || strings.ContainsRune(value, '\x00') {
				return fmt.Errorf("成员 %s 字段 %s 为空或无效", member.Name, field)
			}
		}
		if member.Kind == "reviewer" && !reviewerWriteScope(member.AllowedWrites) {
			return fmt.Errorf("Reviewer %s 的 allowedWrites 必须只包含 ../../reviews/ 或 ../../evaluations/attestations/ 下的 exact 文件", member.Name)
		}
	}
	if executors > 3 || reviewers > 1 {
		return errors.New("正式成员超过 3 名执行成员加 1 名 Reviewer")
	}
	return nil
}

func reviewerWriteScope(value string) bool {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r':
			return true
		default:
			return false
		}
	})
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		path := strings.TrimSpace(field)
		if path == "" || path != field || strings.ContainsAny(path, " `\t") || strings.Contains(path, "\\") {
			return false
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
		if clean != path {
			return false
		}
		reviewPath, reviewOK := strings.CutPrefix(clean, "../../reviews/")
		attestationPath, attestationOK := strings.CutPrefix(clean, "../../evaluations/attestations/")
		if reviewOK {
			if !reviewerMarkdownName(reviewPath) {
				return false
			}
			continue
		}
		if !attestationOK || !reviewerMarkdownName(attestationPath) {
			return false
		}
	}
	return true
}

func reviewerMarkdownName(name string) bool {
	if !reviewerMarkdownPattern.MatchString(name) || strings.Contains(name, "/") {
		return false
	}
	stem, _, _ := strings.Cut(name, ".")
	return !windowsReservedName(stem)
}

func windowsReservedName(name string) bool {
	switch strings.ToLower(name) {
	case "con", "prn", "aux", "nul",
		"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
		"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9":
		return true
	default:
		return false
	}
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func digestRecords(records []SourceRecord) string {
	ordered := append([]SourceRecord(nil), records...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	var lines []string
	for _, record := range ordered {
		lines = append(lines, strings.Join([]string{
			record.Path, record.GitMode, record.HeadBlob, record.ContentBlob,
			record.SHA256, fmt.Sprint(record.Bytes),
		}, "\x00"))
	}
	return "sha256:" + hashBytes([]byte(strings.Join(lines, "\n")+"\n"))
}
