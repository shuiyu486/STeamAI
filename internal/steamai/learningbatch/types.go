package learningbatch

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const ConfirmationPrefix = "CONFIRM STEAMAI LEARNING BATCH "

var (
	ErrConfirmationRequired = errors.New("需要与当前预览完全匹配的 learning batch 确认")
	ErrBinding              = errors.New("learning batch 未绑定 current eligible candidates 和 accepted batch review")
	ErrScope                = errors.New("learning batch patch 越出允许范围")
	ErrCanonicalDrift       = errors.New("canonical working tree 在预览后发生变化")
	ErrCaseDrift            = errors.New("case evidence chain 在预览后发生变化")
	ErrPatchDrift           = errors.New("learning batch patch 在预览后发生变化")
	hexSHA                  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	hexGit                  = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	packNamePattern         = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	candidateNamePattern    = regexp.MustCompile(`^L-[A-Za-z0-9][A-Za-z0-9._-]{0,62}\.md$`)
	reviewNamePattern       = regexp.MustCompile(`^R-L-[A-Za-z0-9][A-Za-z0-9._-]{0,62}\.md$`)
	batchReviewNamePattern  = regexp.MustCompile(`^R-LB-[A-Za-z0-9][A-Za-z0-9._-]{0,62}\.md$`)
	patchNamePattern        = regexp.MustCompile(`^LB-[A-Za-z0-9][A-Za-z0-9._-]{0,62}\.patch$`)
)

type Request struct {
	CandidateReviews []CandidateReviewRef `json:"candidateReviews"`
	Patch            string               `json:"patch"`
	BatchReview      string               `json:"batchReview"`
}

type CandidateReviewRef struct {
	Candidate string `json:"candidate"`
	Review    string `json:"review"`
}

type CandidateRecord struct {
	CandidatePath    string `json:"candidatePath"`
	CandidateSHA256  string `json:"candidateSha256"`
	ReviewPath       string `json:"reviewPath"`
	ReviewSHA256     string `json:"reviewSha256"`
	Reviewer         string `json:"reviewer"`
	Destination      string `json:"destination"`
	SourceFinding    string `json:"sourceFinding"`
	SourceFindingSHA string `json:"sourceFindingSha256"`
	SourceReview     string `json:"sourceReview"`
	SourceReviewSHA  string `json:"sourceReviewSha256"`
}

type TargetRecord struct {
	Path       string `json:"path"`
	PreSHA256  string `json:"preSha256"`
	PreBytes   int    `json:"preBytes"`
	PostSHA256 string `json:"postSha256"`
	PostBytes  int    `json:"postBytes"`
}

type Preview struct {
	SchemaVersion     int               `json:"schemaVersion"`
	Pack              string            `json:"pack"`
	CaseRevision      string            `json:"caseRevision"`
	CanonicalHead     string            `json:"canonicalHead"`
	ManifestPath      string            `json:"manifestPath"`
	ManifestSHA256    string            `json:"manifestSha256"`
	ManifestBytes     int               `json:"manifestBytes"`
	SnapshotDigest    string            `json:"snapshotDigest"`
	Candidates        []CandidateRecord `json:"candidates"`
	Targets           []TargetRecord    `json:"targets"`
	PatchPath         string            `json:"patchPath"`
	PatchSHA256       string            `json:"patchSha256"`
	PatchBytes        int               `json:"patchBytes"`
	BatchReviewPath   string            `json:"batchReviewPath"`
	BatchReviewSHA256 string            `json:"batchReviewSha256"`
	BatchReviewer     string            `json:"batchReviewer"`
	Identity          string            `json:"identity"`
	HumanPreview      string            `json:"humanPreview"`
	patchData         []byte
	targetData        map[string][]byte
}

type candidateBinding struct {
	CandidateSHA256  string
	SourceFinding    string
	SourceFindingSHA string
	SourceReview     string
	SourceReviewSHA  string
	Pack             string
	Revision         string
	PackTree         string
	CommonTree       string
	SnapshotDigest   string
	Destination      string
}

type eligibilityBinding struct {
	CandidatePath    string
	CandidateSHA256  string
	Decision         string
	Destination      string
	Reviewer         string
	SourceFinding    string
	SourceFindingSHA string
	SourceReview     string
	SourceReviewSHA  string
	Pack             string
	Revision         string
	PackTree         string
	CommonTree       string
	SnapshotDigest   string
}

type batchBinding struct {
	Reviewer       string
	Decision       string
	Pack           string
	CaseRevision   string
	CanonicalHead  string
	SnapshotDigest string
	PatchPath      string
	PatchSHA256    string
	Candidates     []batchCandidateBinding
	Targets        []batchTargetBinding
}

type batchCandidateBinding struct {
	CandidatePath   string
	CandidateSHA256 string
	ReviewPath      string
	ReviewSHA256    string
	Destination     string
}

type batchTargetBinding struct {
	Path       string
	PreSHA256  string
	PreBytes   int
	PostSHA256 string
	PostBytes  int
}

func DecodeRequest(reader io.Reader) (Request, error) {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		return Request{}, fmt.Errorf("解析 learning batch request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Request{}, errors.New("learning batch request 后存在额外 JSON")
		}
		return Request{}, fmt.Errorf("解析 learning batch request 结尾: %w", err)
	}
	if err := request.Validate(); err != nil {
		return Request{}, err
	}
	return request, nil
}

func (request Request) Validate() error {
	if len(request.CandidateReviews) == 0 {
		return errors.New("learning batch 至少需要一个 candidate")
	}
	if _, err := cleanStateFile(request.Patch, "learnings/patches/", patchNamePattern); err != nil {
		return fmt.Errorf("patch path 无效: %w", err)
	}
	if _, err := cleanStateFile(request.BatchReview, "reviews/", batchReviewNamePattern); err != nil {
		return fmt.Errorf("batch review path 无效: %w", err)
	}
	seenCandidates := map[string]bool{}
	seenReviews := map[string]bool{}
	for _, item := range request.CandidateReviews {
		candidate, err := cleanStateFile(item.Candidate, "learnings/candidates/", candidateNamePattern)
		if err != nil || seenCandidates[candidate] {
			return errors.New("candidate path 无效或重复")
		}
		review, err := cleanStateFile(item.Review, "reviews/", reviewNamePattern)
		if err != nil || seenReviews[review] {
			return errors.New("candidate review path 无效或重复")
		}
		seenCandidates[candidate] = true
		seenReviews[review] = true
	}
	return nil
}

func cleanStateFile(value, prefix string, basePattern *regexp.Regexp) (string, error) {
	if value == "" || strings.Contains(value, "\\") || filepath.IsAbs(filepath.FromSlash(value)) {
		return "", errors.New("必须是 case state 内的相对路径")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	base, ok := strings.CutPrefix(clean, prefix)
	if clean != value || !ok || strings.Contains(base, "/") || !basePattern.MatchString(base) {
		return "", errors.New("路径不匹配固定目录与文件名")
	}
	return clean, nil
}

func fieldMap(data []byte) (map[string]string, error) {
	fields := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(strings.ReplaceAll(string(data), "\r\n", "\n")))
	scanner.Buffer(make([]byte, 1024), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "- ") || !strings.HasSuffix(line, "`") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), "：`")
		if !ok {
			continue
		}
		value = strings.TrimSuffix(value, "`")
		if fields[key] != "" {
			return nil, fmt.Errorf("字段重复: %s", key)
		}
		fields[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return fields, nil
}

func parseCandidate(data []byte) (candidateBinding, error) {
	fields, err := fieldMap(data)
	if err != nil {
		return candidateBinding{}, err
	}
	binding := candidateBinding{
		SourceFinding: fields["Source finding"], SourceFindingSHA: fields["Source finding SHA-256"],
		SourceReview: fields["Source accepted review"], SourceReviewSHA: fields["Source review SHA-256"],
		Pack: fields["Selected pack"], Revision: fields["Source revision"], PackTree: fields["Pack tree"],
		CommonTree: fields["Common tree"], SnapshotDigest: fields["Snapshot digest"], Destination: fields["Proposed destination"],
	}
	if binding.SourceFinding == "" || binding.SourceReview == "" || !hexSHA.MatchString(binding.SourceFindingSHA) ||
		!hexSHA.MatchString(binding.SourceReviewSHA) || !packNamePattern.MatchString(binding.Pack) ||
		!hexGit.MatchString(binding.Revision) || !hexGit.MatchString(binding.PackTree) || !hexGit.MatchString(binding.CommonTree) ||
		!prefixedSHA(binding.SnapshotDigest) || binding.Destination == "" || strings.Contains(binding.Destination, "\\") {
		return candidateBinding{}, ErrBinding
	}
	if fields["Candidate SHA-256"] != "" {
		return candidateBinding{}, errors.New("candidate 不得自引用 exact SHA-256")
	}
	return binding, nil
}

func parseEligibility(data []byte) (eligibilityBinding, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	checkpoint := strings.Index(text, "## Checkpoint A — Eligibility\n")
	if checkpoint < 0 || strings.Contains(text[checkpoint+1:], "## Checkpoint A — Eligibility\n") {
		return eligibilityBinding{}, ErrBinding
	}
	fields, err := fieldMap(data)
	if err != nil {
		return eligibilityBinding{}, err
	}
	beforeFields, err := fieldMap([]byte(text[:checkpoint]))
	if err != nil {
		return eligibilityBinding{}, err
	}
	checkpointFields, err := fieldMap([]byte(text[checkpoint:]))
	if err != nil {
		return eligibilityBinding{}, err
	}
	binding := eligibilityBinding{
		CandidatePath: beforeFields["Candidate"], CandidateSHA256: beforeFields["Candidate SHA-256"],
		Decision: checkpointFields["Decision"], Destination: beforeFields["Proposed destination"], Reviewer: beforeFields["Reviewer 单写者"],
		SourceFinding: beforeFields["Source finding"], SourceFindingSHA: beforeFields["Source finding SHA-256"],
		SourceReview: beforeFields["Source accepted review"], SourceReviewSHA: beforeFields["Source review SHA-256"],
		Pack: beforeFields["Selected pack"], Revision: beforeFields["Source revision"], PackTree: beforeFields["Pack tree"],
		CommonTree: beforeFields["Common tree"], SnapshotDigest: beforeFields["Snapshot digest"],
	}
	if binding.CandidatePath == "" || !hexSHA.MatchString(binding.CandidateSHA256) || binding.Decision != "eligible" ||
		binding.Destination == "" || binding.Reviewer == "" || binding.SourceFinding == "" || !hexSHA.MatchString(binding.SourceFindingSHA) ||
		binding.SourceReview == "" || !hexSHA.MatchString(binding.SourceReviewSHA) || !packNamePattern.MatchString(binding.Pack) ||
		!hexGit.MatchString(binding.Revision) || !hexGit.MatchString(binding.PackTree) || !hexGit.MatchString(binding.CommonTree) ||
		!prefixedSHA(binding.SnapshotDigest) || checkpointFields["Evidence/generalization"] != "pass" || checkpointFields["Applicability/counterexamples"] != "pass" ||
		checkpointFields["Dedup/conflict"] != "pass" || checkpointFields["Redaction/denyPatterns"] != "pass" ||
		checkpointFields["Target allowlist/currentness"] != "pass" || fields["Patch SHA-256"] != "" || fields["Patch decision"] != "" {
		return eligibilityBinding{}, ErrBinding
	}
	return binding, nil
}

func parseBatchReview(data []byte) (batchBinding, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	fields := map[string]string{}
	section := ""
	seenSections := map[string]bool{}
	var candidates []batchCandidateBinding
	var targets []batchTargetBinding
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		var parseErr error
		switch line {
		case "## Candidates":
			if seenSections["candidates"] || seenSections["targets"] || seenSections["final"] {
				return batchBinding{}, ErrBinding
			}
			seenSections["candidates"] = true
			section = "candidates"
			continue
		case "## Targets":
			if !seenSections["candidates"] || seenSections["targets"] || seenSections["final"] {
				return batchBinding{}, ErrBinding
			}
			seenSections["targets"] = true
			section = "targets"
			continue
		case "## Final decision":
			if !seenSections["targets"] || seenSections["final"] {
				return batchBinding{}, ErrBinding
			}
			seenSections["final"] = true
			section = "final"
			continue
		}
		if !strings.HasPrefix(line, "- ") || !strings.HasSuffix(line, "`") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), "：`")
		if !ok {
			continue
		}
		value = strings.TrimSuffix(value, "`")
		switch {
		case section == "final" && key != "Decision":
			return batchBinding{}, ErrBinding
		case section == "candidates" && key == "Candidate":
			candidates = append(candidates, batchCandidateBinding{CandidatePath: value})
		case section == "candidates" && len(candidates) > 0:
			item := &candidates[len(candidates)-1]
			switch key {
			case "Candidate SHA-256":
				if item.CandidateSHA256 != "" {
					return batchBinding{}, ErrBinding
				}
				item.CandidateSHA256 = value
			case "Eligibility review":
				if item.ReviewPath != "" {
					return batchBinding{}, ErrBinding
				}
				item.ReviewPath = value
			case "Eligibility review SHA-256":
				if item.ReviewSHA256 != "" {
					return batchBinding{}, ErrBinding
				}
				item.ReviewSHA256 = value
			case "Destination":
				if item.Destination != "" {
					return batchBinding{}, ErrBinding
				}
				item.Destination = value
			default:
				return batchBinding{}, ErrBinding
			}
		case section == "targets" && key == "Target":
			targets = append(targets, batchTargetBinding{Path: value, PreBytes: -1, PostBytes: -1})
		case section == "targets" && len(targets) > 0:
			item := &targets[len(targets)-1]
			switch key {
			case "Preimage SHA-256":
				if item.PreSHA256 != "" {
					return batchBinding{}, ErrBinding
				}
				item.PreSHA256 = value
			case "Preimage bytes":
				if item.PreBytes != -1 {
					return batchBinding{}, ErrBinding
				}
				item.PreBytes, parseErr = strconv.Atoi(value)
			case "Postimage SHA-256":
				if item.PostSHA256 != "" {
					return batchBinding{}, ErrBinding
				}
				item.PostSHA256 = value
			case "Postimage bytes":
				if item.PostBytes != -1 {
					return batchBinding{}, ErrBinding
				}
				item.PostBytes, parseErr = strconv.Atoi(value)
			default:
				return batchBinding{}, ErrBinding
			}
		default:
			if fields[key] != "" {
				return batchBinding{}, ErrBinding
			}
			fields[key] = value
		}
		if parseErr != nil {
			return batchBinding{}, ErrBinding
		}
	}
	binding := batchBinding{
		Reviewer: fields["Reviewer 单写者"], Decision: fields["Decision"], Pack: fields["Selected pack"],
		CaseRevision: fields["Case revision"], CanonicalHead: fields["Canonical HEAD"], SnapshotDigest: fields["Snapshot digest"],
		PatchPath: fields["Patch"], PatchSHA256: fields["Patch SHA-256"], Candidates: candidates, Targets: targets,
	}
	if !seenSections["candidates"] || !seenSections["targets"] || !seenSections["final"] ||
		binding.Reviewer == "" || binding.Decision != "accepted" || !packNamePattern.MatchString(binding.Pack) || !hexGit.MatchString(binding.CaseRevision) ||
		!hexGit.MatchString(binding.CanonicalHead) || !prefixedSHA(binding.SnapshotDigest) || binding.PatchPath == "" ||
		!hexSHA.MatchString(binding.PatchSHA256) || fields["Added-lines deny result"] != "clear" ||
		fields["`git apply --check` result"] != "pass" || fields["Candidate mapping/theme"] != "pass" ||
		fields["Dedup/conflict/counterexamples"] != "pass" || fields["Redaction"] != "pass" ||
		len(candidates) == 0 || len(targets) == 0 {
		return batchBinding{}, ErrBinding
	}
	for _, item := range candidates {
		if item.CandidatePath == "" || !hexSHA.MatchString(item.CandidateSHA256) || item.ReviewPath == "" ||
			!hexSHA.MatchString(item.ReviewSHA256) || item.Destination == "" {
			return batchBinding{}, ErrBinding
		}
	}
	for _, item := range targets {
		if item.Path == "" || !hexSHA.MatchString(item.PreSHA256) || item.PreBytes < 0 ||
			!hexSHA.MatchString(item.PostSHA256) || item.PostBytes < 0 {
			return batchBinding{}, ErrBinding
		}
	}
	return binding, nil
}

func prefixedSHA(value string) bool {
	digest, ok := strings.CutPrefix(value, "sha256:")
	return ok && hexSHA.MatchString(digest)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func canonicalIdentity(preview Preview) string {
	copy := preview
	copy.Identity = ""
	copy.HumanPreview = ""
	copy.patchData = nil
	copy.targetData = nil
	data, _ := json.Marshal(copy)
	return hashBytes(data)
}

func orderedCandidateRecords(records []CandidateRecord) []CandidateRecord {
	out := append([]CandidateRecord(nil), records...)
	sort.Slice(out, func(i, j int) bool { return out[i].CandidatePath < out[j].CandidatePath })
	return out
}
