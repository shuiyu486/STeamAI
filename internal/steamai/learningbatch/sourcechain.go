package learningbatch

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var reviewRoundPattern = regexp.MustCompile("(?m)^## Review round `([^`]+)`$")

type artifactBinding struct {
	Alias         string
	Path          string
	SHA256        string
	Bytes         int
	AuthorizedUse string
}

type reviewRound struct {
	Number       string
	Previous     string
	Reviewer     string
	Finding      string
	FindingSHA   string
	Decision     string
	Confidence   string
	Summary      string
	RisksOrGaps  string
	NextAction   string
	EvidenceRefs map[string]string
}

func validateSourceChain(caseRoot, findingRel, findingSHA, reviewRel, reviewSHA string) error {
	findingPath := filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(findingRel))
	reviewPath := filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(reviewRel))
	indexPath := filepath.Join(caseRoot, ".steamai-vnext", "artifacts", "index.md")
	for _, path := range []string{findingPath, reviewPath, indexPath} {
		if err := requirePlainPath(caseRoot, path, false); err != nil {
			return ErrBinding
		}
	}
	findingData, err := os.ReadFile(findingPath)
	if err != nil || hashBytes(findingData) != findingSHA {
		return ErrBinding
	}
	reviewData, err := os.ReadFile(reviewPath)
	if err != nil || hashBytes(reviewData) != reviewSHA {
		return ErrBinding
	}
	rounds, err := parseReviewRounds(reviewData)
	if err != nil || len(rounds) == 0 {
		return ErrBinding
	}
	last := rounds[len(rounds)-1]
	if last.Decision != "accepted" || last.FindingSHA != findingSHA || last.Reviewer == "" || len(last.EvidenceRefs) == 0 {
		return ErrBinding
	}
	for _, round := range rounds {
		if round.Reviewer != last.Reviewer {
			return ErrBinding
		}
	}
	cleanFindingRef := filepath.ToSlash(filepath.Clean(filepath.Join("reviews", filepath.FromSlash(last.Finding))))
	if cleanFindingRef != findingRel {
		return ErrBinding
	}
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	for evidenceRef, evidenceSHA := range last.EvidenceRefs {
		evidenceRel := filepath.ToSlash(filepath.Clean(filepath.Join("reviews", filepath.FromSlash(evidenceRef))))
		if !strings.HasPrefix(evidenceRel, "evidence/") || filepath.Ext(evidenceRel) != ".md" {
			return ErrBinding
		}
		evidencePath := filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(evidenceRel))
		if err := requirePlainPath(caseRoot, evidencePath, false); err != nil {
			return ErrBinding
		}
		evidenceData, err := os.ReadFile(evidencePath)
		if err != nil || hashBytes(evidenceData) != evidenceSHA {
			return ErrBinding
		}
		if !findingReferencesEvidence(findingData, evidenceRel) {
			return ErrBinding
		}
		artifact, err := parseArtifactBinding(evidenceData)
		if err != nil || !artifactIndexContains(indexData, artifact) {
			return ErrBinding
		}
		artifactPath := filepath.Join(caseRoot, filepath.FromSlash(artifact.Path))
		if err := requirePlainPath(caseRoot, artifactPath, false); err != nil {
			return ErrBinding
		}
		artifactData, err := os.ReadFile(artifactPath)
		if err != nil || len(artifactData) != artifact.Bytes || hashBytes(artifactData) != artifact.SHA256 {
			return ErrBinding
		}
	}
	return nil
}

func parseReviewRounds(data []byte) ([]reviewRound, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	locations := reviewRoundPattern.FindAllStringSubmatchIndex(text, -1)
	if len(locations) == 0 {
		return nil, ErrBinding
	}
	var rounds []reviewRound
	for index, location := range locations {
		end := len(text)
		if index+1 < len(locations) {
			end = locations[index+1][0]
		}
		body := text[location[1]:end]
		fields, err := fieldMap([]byte(body))
		if err != nil {
			return nil, err
		}
		round := reviewRound{
			Number: text[location[2]:location[3]], Previous: fields["Previous round"], Reviewer: fields["Reviewer"],
			Finding: fields["Finding"], FindingSHA: fields["Finding SHA-256"], Decision: fields["Decision"],
			Confidence: fields["Confidence"], EvidenceRefs: map[string]string{},
		}
		round.Summary = reviewSection(body, "判断")
		round.RisksOrGaps = reviewSection(body, "风险或缺口")
		round.NextAction = reviewSection(body, "下一步")
		if round.Number != strconv.Itoa(index+1) || (index == 0 && round.Previous != "none") ||
			(index > 0 && round.Previous != strconv.Itoa(index)) || round.Finding == "" || !hexSHA.MatchString(round.FindingSHA) ||
			round.Confidence == "" || round.Summary == "" || round.RisksOrGaps == "" || round.NextAction == "" {
			return nil, ErrBinding
		}
		marker := "### 检查的证据\n"
		start := strings.Index(body, marker)
		if start < 0 {
			return nil, ErrBinding
		}
		section := body[start+len(marker):]
		if stop := strings.Index(section, "\n### "); stop >= 0 {
			section = section[:stop]
		}
		for line := range strings.SplitSeq(section, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "- ") {
				continue
			}
			ref, sha, ok := strings.Cut(strings.TrimPrefix(line, "- "), " — ")
			if !ok || ref == "" || !hexSHA.MatchString(sha) || round.EvidenceRefs[ref] != "" {
				return nil, ErrBinding
			}
			round.EvidenceRefs[ref] = sha
		}
		if len(round.EvidenceRefs) == 0 {
			return nil, ErrBinding
		}
		rounds = append(rounds, round)
	}
	return rounds, nil
}

func reviewSection(body, heading string) string {
	marker := "### " + heading + "\n"
	start := strings.Index(body, marker)
	if start < 0 {
		return ""
	}
	section := body[start+len(marker):]
	if stop := strings.Index(section, "\n### "); stop >= 0 {
		section = section[:stop]
	}
	return strings.TrimSpace(section)
}

func findingReferencesEvidence(finding []byte, evidenceRel string) bool {
	ref := "../" + evidenceRel
	for line := range strings.SplitSeq(strings.ReplaceAll(string(finding), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "- `"+ref+"`" || strings.HasPrefix(trimmed, "- `"+ref+"` ") {
			return true
		}
	}
	return false
}

func parseArtifactBinding(data []byte) (artifactBinding, error) {
	fields, err := fieldMap(data)
	if err != nil {
		return artifactBinding{}, err
	}
	bytesCount, err := strconv.Atoi(fields["Artifact bytes"])
	binding := artifactBinding{
		Alias: fields["Artifact alias"], Path: fields["Artifact path"], SHA256: fields["Artifact SHA-256"],
		Bytes: bytesCount, AuthorizedUse: fields["Authorized use"],
	}
	if err != nil || binding.Alias == "" || binding.Path == "" || !hexSHA.MatchString(binding.SHA256) || binding.Bytes < 0 || binding.AuthorizedUse == "" {
		return artifactBinding{}, ErrBinding
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(binding.Path)))
	if clean != binding.Path || filepath.IsAbs(filepath.FromSlash(binding.Path)) || strings.HasPrefix(clean, ".steamai-vnext/") {
		return artifactBinding{}, ErrBinding
	}
	return binding, nil
}

func artifactIndexContains(index []byte, binding artifactBinding) bool {
	text := strings.ReplaceAll(string(index), "\r\n", "\n")
	marker := "## `" + binding.Alias + "`"
	start := strings.Index(text, marker)
	if start < 0 {
		return false
	}
	entry := text[start:]
	if next := strings.Index(entry[len(marker):], "\n## `"); next >= 0 {
		entry = entry[:len(marker)+next]
	}
	return strings.Contains(entry, "相对路径：`"+binding.Path+"`") &&
		strings.Contains(entry, "SHA-256：`"+binding.SHA256+"`") &&
		strings.Contains(entry, "Bytes：`"+strconv.Itoa(binding.Bytes)+"`") &&
		strings.Contains(entry, "授权范围：`"+binding.AuthorizedUse+"`")
}
