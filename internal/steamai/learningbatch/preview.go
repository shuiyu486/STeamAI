package learningbatch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
)

func BuildPreview(git, source, caseRoot string, request Request) (Preview, error) {
	if err := request.Validate(); err != nil {
		return Preview{}, err
	}
	source, caseRoot, err := absoluteRoots(source, caseRoot)
	if err != nil {
		return Preview{}, err
	}
	if err := validateGitRoot(git, source); err != nil {
		return Preview{}, err
	}
	identity, err := casebootstrap.InspectCurrent(caseRoot)
	if err != nil {
		return Preview{}, fmt.Errorf("current case 无效: %w", err)
	}
	manifestRel := "packs/" + identity.Pack + "/manifest.yml"
	manifestPath := filepath.Join(source, filepath.FromSlash(manifestRel))
	if err := requirePlainPath(source, manifestPath, false); err != nil {
		return Preview{}, fmt.Errorf("canonical manifest 无效: %w", err)
	}
	if err := requireTrackedStageZero(git, source, manifestRel); err != nil {
		return Preview{}, errors.New("canonical manifest 必须是 tracked stage-0 regular file")
	}
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return Preview{}, err
	}
	manifestPack, learningTargets, denyPatterns, err := parseLearningManifest(manifest)
	if err != nil || manifestPack != identity.Pack {
		return Preview{}, errors.New("canonical manifest identity 或 learning policy 无效")
	}
	snapshotManifestPath := filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", "packs", identity.Pack, "manifest.yml")
	if err := requirePlainPath(caseRoot, snapshotManifestPath, false); err != nil {
		return Preview{}, ErrBinding
	}
	snapshotManifest, err := os.ReadFile(snapshotManifestPath)
	if err != nil {
		return Preview{}, err
	}
	snapshotPack, snapshotLearningTargets, snapshotDenyPatterns, err := parseLearningManifest(snapshotManifest)
	if err != nil || snapshotPack != identity.Pack {
		return Preview{}, ErrBinding
	}
	head, err := gitOutput(git, source, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return Preview{}, err
	}
	head = strings.TrimSpace(head)
	if !hexGit.MatchString(head) {
		return Preview{}, errors.New("canonical HEAD 无效")
	}
	patchRel, _ := cleanStateFile(request.Patch, "learnings/patches/", patchNamePattern)
	batchReviewRel, _ := cleanStateFile(request.BatchReview, "reviews/", batchReviewNamePattern)
	patchPath := filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(patchRel))
	batchReviewPath := filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(batchReviewRel))
	for _, path := range []string{patchPath, batchReviewPath} {
		if err := requirePlainPath(caseRoot, path, false); err != nil {
			return Preview{}, ErrBinding
		}
	}
	patchData, err := os.ReadFile(patchPath)
	if err != nil {
		return Preview{}, err
	}
	batchReviewData, err := os.ReadFile(batchReviewPath)
	if err != nil {
		return Preview{}, err
	}

	candidateRecords := make([]CandidateRecord, 0, len(request.CandidateReviews))
	for _, ref := range request.CandidateReviews {
		record, err := validateCandidateReview(caseRoot, identity, ref, learningTargets, snapshotLearningTargets, denyPatterns, snapshotDenyPatterns, identity.Pack)
		if err != nil {
			return Preview{}, err
		}
		candidateRecords = append(candidateRecords, record)
	}
	candidateRecords = orderedCandidateRecords(candidateRecords)

	targetPaths, err := validatePatchScope(git, source, patchPath, identity.Pack, learningTargets, denyPatterns)
	if err != nil {
		return Preview{}, err
	}
	if err := sameDestinationSet(candidateRecords, targetPaths); err != nil {
		return Preview{}, err
	}
	targets, targetData, err := capturePatchImages(git, source, patchPath, targetPaths)
	if err != nil {
		return Preview{}, err
	}

	binding, err := parseBatchReview(batchReviewData)
	if err != nil {
		return Preview{}, err
	}
	patchSHA := hashBytes(patchData)
	if err := validateBatchBinding(binding, identity, head, request, candidateRecords, targets, patchSHA); err != nil {
		return Preview{}, err
	}
	preview := Preview{
		SchemaVersion: 1, Pack: identity.Pack, CaseRevision: identity.Revision, CanonicalHead: head,
		ManifestPath: manifestRel, ManifestSHA256: hashBytes(manifest), ManifestBytes: len(manifest),
		SnapshotDigest: identity.PayloadDigest, Candidates: candidateRecords, Targets: targets,
		PatchPath: patchRel, PatchSHA256: patchSHA, PatchBytes: len(patchData),
		BatchReviewPath: batchReviewRel, BatchReviewSHA256: hashBytes(batchReviewData), BatchReviewer: binding.Reviewer,
		patchData: append([]byte(nil), patchData...), targetData: targetData,
	}
	preview.Identity = canonicalIdentity(preview)
	preview.HumanPreview = renderHumanPreview(preview)
	return preview, nil
}

func absoluteRoots(source, caseRoot string) (string, string, error) {
	source, err := filepath.Abs(source)
	if err != nil {
		return "", "", err
	}
	caseRoot, err = filepath.Abs(caseRoot)
	if err != nil {
		return "", "", err
	}
	if err := requirePlainDirectory(source); err != nil {
		return "", "", err
	}
	if err := requirePlainDirectory(caseRoot); err != nil {
		return "", "", err
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return "", "", err
	}
	current := caseRoot
	for {
		info, statErr := os.Stat(current)
		if statErr != nil {
			return "", "", statErr
		}
		if os.SameFile(info, sourceInfo) {
			return "", "", errors.New("current case 不能是 canonical source 或位于其内部")
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return source, caseRoot, nil
}

func validateGitRoot(git, source string) error {
	top, err := gitOutput(git, source, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	topInfo, topErr := os.Stat(strings.TrimSpace(top))
	sourceInfo, sourceErr := os.Stat(source)
	if topErr != nil || sourceErr != nil || !os.SameFile(topInfo, sourceInfo) {
		return errors.New("canonical source 必须是 Git worktree 根目录")
	}
	return nil
}

func trackedStageZeroBlob(git, source, target string) (string, error) {
	out, err := gitOutput(git, source, "ls-files", "--stage", "--", target)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		return "", ErrScope
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 4 || fields[0] != "100644" || fields[2] != "0" || strings.Join(fields[3:], " ") != target || !hexGit.MatchString(fields[1]) {
		return "", ErrScope
	}
	return fields[1], nil
}

func requireTrackedStageZero(git, source, target string) error {
	_, err := trackedStageZeroBlob(git, source, target)
	return err
}

func rosterReviewer(identity casebootstrap.CurrentIdentity, name string) bool {
	for _, member := range identity.Roster {
		if member.Name == name && member.Kind == "reviewer" && member.State == "active" {
			return true
		}
	}
	return false
}

func validateCandidateReview(caseRoot string, identity casebootstrap.CurrentIdentity, ref CandidateReviewRef, canonicalTargets, snapshotTargets, canonicalDeny, snapshotDeny []string, pack string) (CandidateRecord, error) {
	candidateRel, _ := cleanStateFile(ref.Candidate, "learnings/candidates/", candidateNamePattern)
	reviewRel, _ := cleanStateFile(ref.Review, "reviews/", reviewNamePattern)
	candidatePath := filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(candidateRel))
	reviewPath := filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(reviewRel))
	for _, path := range []string{candidatePath, reviewPath} {
		if err := requirePlainPath(caseRoot, path, false); err != nil {
			return CandidateRecord{}, ErrBinding
		}
	}
	candidateData, err := os.ReadFile(candidatePath)
	if err != nil {
		return CandidateRecord{}, err
	}
	reviewData, err := os.ReadFile(reviewPath)
	if err != nil {
		return CandidateRecord{}, err
	}
	candidate, err := parseCandidate(candidateData)
	if err != nil {
		return CandidateRecord{}, err
	}
	candidate.CandidateSHA256 = hashBytes(candidateData)
	eligibility, err := parseEligibility(reviewData)
	if err != nil {
		return CandidateRecord{}, err
	}
	if candidate.Pack != pack || candidate.Revision != identity.Revision || candidate.PackTree != identity.PackTree ||
		candidate.CommonTree != identity.CommonTree || candidate.SnapshotDigest != identity.PayloadDigest ||
		eligibility.CandidatePath != candidateRel || eligibility.CandidateSHA256 != candidate.CandidateSHA256 ||
		eligibility.SourceFinding != candidate.SourceFinding || eligibility.SourceFindingSHA != candidate.SourceFindingSHA ||
		eligibility.SourceReview != candidate.SourceReview || eligibility.SourceReviewSHA != candidate.SourceReviewSHA ||
		eligibility.Pack != candidate.Pack || eligibility.Revision != candidate.Revision || eligibility.PackTree != candidate.PackTree ||
		eligibility.CommonTree != candidate.CommonTree || eligibility.SnapshotDigest != candidate.SnapshotDigest ||
		eligibility.Destination != candidate.Destination || !rosterReviewer(identity, eligibility.Reviewer) ||
		!destinationAllowed(candidate.Destination, pack, canonicalTargets) ||
		!destinationAllowed(candidate.Destination, pack, snapshotTargets) || slicesOverlapCandidate(candidateData, canonicalDeny) ||
		slicesOverlapCandidate(candidateData, snapshotDeny) {
		return CandidateRecord{}, ErrBinding
	}
	findingRel, err := cleanCaseSourceRef(candidate.SourceFinding, "findings/")
	if err != nil {
		return CandidateRecord{}, ErrBinding
	}
	sourceReviewRel, err := cleanCaseSourceRef(candidate.SourceReview, "reviews/")
	if err != nil {
		return CandidateRecord{}, ErrBinding
	}
	if err := validateSourceChain(caseRoot, findingRel, candidate.SourceFindingSHA, sourceReviewRel, candidate.SourceReviewSHA); err != nil {
		return CandidateRecord{}, err
	}
	return CandidateRecord{
		CandidatePath: candidateRel, CandidateSHA256: candidate.CandidateSHA256,
		ReviewPath: reviewRel, ReviewSHA256: hashBytes(reviewData), Reviewer: eligibility.Reviewer,
		Destination: candidate.Destination, SourceFinding: findingRel, SourceFindingSHA: candidate.SourceFindingSHA,
		SourceReview: sourceReviewRel, SourceReviewSHA: candidate.SourceReviewSHA,
	}, nil
}

func cleanCaseSourceRef(value, prefix string) (string, error) {
	if value == "" || strings.Contains(value, "\\") || filepath.IsAbs(filepath.FromSlash(value)) {
		return "", ErrBinding
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	base, ok := strings.CutPrefix(clean, prefix)
	if clean != value || !ok || base == "" || strings.Contains(base, "/") || filepath.Ext(base) != ".md" {
		return "", ErrBinding
	}
	return clean, nil
}

func slicesOverlapCandidate(data []byte, denyPatterns []string) bool {
	text := string(data)
	for _, deny := range denyPatterns {
		if deny != "" && strings.Contains(text, deny) {
			return true
		}
	}
	return false
}

func sameDestinationSet(candidates []CandidateRecord, targets []string) error {
	expected := map[string]bool{}
	for _, candidate := range candidates {
		expected[candidate.Destination] = true
	}
	if len(expected) != len(targets) {
		return ErrScope
	}
	for _, target := range targets {
		if !expected[target] {
			return ErrScope
		}
	}
	return nil
}

func capturePatchImages(git, source, patchPath string, targets []string) ([]TargetRecord, map[string][]byte, error) {
	preimages := map[string][]byte{}
	modes := map[string]os.FileMode{}
	for _, rel := range targets {
		path := filepath.Join(source, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, nil, err
		}
		preimages[rel] = append([]byte(nil), data...)
		modes[rel] = info.Mode().Perm()
	}
	verification, err := os.MkdirTemp(filepath.Dir(source), ".steamai-learning-preview-")
	if err != nil {
		return nil, nil, err
	}
	_ = os.Remove(verification)
	defer os.RemoveAll(verification)
	if _, err := gitOutput(git, filepath.Dir(source), "clone", "--quiet", "--no-hardlinks", source, verification); err != nil {
		return nil, nil, err
	}
	for _, rel := range targets {
		path := filepath.Join(verification, filepath.FromSlash(rel))
		if err := os.WriteFile(path, preimages[rel], modes[rel]); err != nil {
			return nil, nil, err
		}
	}
	if _, err := gitOutput(git, verification, "apply", "--check", patchPath); err != nil {
		return nil, nil, fmt.Errorf("git apply --check 失败: %w", err)
	}
	if _, err := gitOutput(git, verification, "apply", patchPath); err != nil {
		return nil, nil, err
	}
	records := make([]TargetRecord, 0, len(targets))
	for _, rel := range targets {
		post, err := os.ReadFile(filepath.Join(verification, filepath.FromSlash(rel)))
		if err != nil {
			return nil, nil, err
		}
		pre := preimages[rel]
		records = append(records, TargetRecord{Path: rel, PreSHA256: hashBytes(pre), PreBytes: len(pre), PostSHA256: hashBytes(post), PostBytes: len(post)})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Path < records[j].Path })
	return records, preimages, nil
}

func validateBatchBinding(binding batchBinding, identity casebootstrap.CurrentIdentity, head string, request Request, candidates []CandidateRecord, targets []TargetRecord, patchSHA string) error {
	if !rosterReviewer(identity, binding.Reviewer) || binding.Pack != identity.Pack || binding.CaseRevision != identity.Revision ||
		binding.CanonicalHead != head || binding.SnapshotDigest != identity.PayloadDigest ||
		binding.PatchPath != request.Patch || binding.PatchSHA256 != patchSHA ||
		len(binding.Candidates) != len(candidates) || len(binding.Targets) != len(targets) {
		return ErrBinding
	}
	for index, item := range binding.Candidates {
		expected := candidates[index]
		if item.CandidatePath != expected.CandidatePath || item.CandidateSHA256 != expected.CandidateSHA256 ||
			item.ReviewPath != expected.ReviewPath || item.ReviewSHA256 != expected.ReviewSHA256 ||
			expected.Reviewer != binding.Reviewer || item.Destination != expected.Destination {
			return ErrBinding
		}
	}
	for index, item := range binding.Targets {
		expected := targets[index]
		if item.Path != expected.Path || item.PreSHA256 != expected.PreSHA256 || item.PreBytes != expected.PreBytes ||
			item.PostSHA256 != expected.PostSHA256 || item.PostBytes != expected.PostBytes {
			return ErrBinding
		}
	}
	return nil
}

func renderHumanPreview(preview Preview) string {
	var out strings.Builder
	fmt.Fprintln(&out, "STeamAI learning batch exact preview")
	fmt.Fprintf(&out, "- selected-pack: %s\n- case-revision: %s\n- canonical-head: %s\n- snapshot-digest: %s\n", preview.Pack, preview.CaseRevision, preview.CanonicalHead, preview.SnapshotDigest)
	fmt.Fprintf(&out, "- manifest: %s sha256:%s bytes:%d\n", preview.ManifestPath, preview.ManifestSHA256, preview.ManifestBytes)
	fmt.Fprintln(&out, "- candidates:")
	for _, item := range preview.Candidates {
		fmt.Fprintf(&out, "  - %s sha256:%s | eligibility-review:%s sha256:%s reviewer:%s | destination:%s\n", item.CandidatePath, item.CandidateSHA256, item.ReviewPath, item.ReviewSHA256, item.Reviewer, item.Destination)
		fmt.Fprintf(&out, "    source-finding:%s sha256:%s | source-accepted-review:%s sha256:%s\n", item.SourceFinding, item.SourceFindingSHA, item.SourceReview, item.SourceReviewSHA)
	}
	fmt.Fprintln(&out, "- targets:")
	for _, item := range preview.Targets {
		fmt.Fprintf(&out, "  - %s pre:%s/%d post:%s/%d\n", item.Path, item.PreSHA256, item.PreBytes, item.PostSHA256, item.PostBytes)
	}
	fmt.Fprintf(&out, "- batch-review: %s sha256:%s reviewer:%s\n- patch: %s sha256:%s bytes:%d\n\n", preview.BatchReviewPath, preview.BatchReviewSHA256, preview.BatchReviewer, preview.PatchPath, preview.PatchSHA256, preview.PatchBytes)
	fmt.Fprintln(&out, "完整 patch：")
	out.Write(preview.patchData)
	if len(preview.patchData) == 0 || preview.patchData[len(preview.patchData)-1] != '\n' {
		out.WriteByte('\n')
	}
	fmt.Fprintln(&out, "\n当前 canonical pack 仍为零写入。")
	fmt.Fprintln(&out, ConfirmationPrefix+preview.Identity)
	return out.String()
}

func gitOutput(git, dir string, args ...string) (string, error) {
	return gitInput(git, dir, nil, args...)
}

func gitInput(git, dir string, input []byte, args ...string) (string, error) {
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return stdout.String(), nil
}

func parseLearningManifest(data []byte) (string, []string, []string, error) {
	var name string
	lists := map[string][]string{"learningTargets": nil, "denyPatterns": nil}
	section := ""
	for line := range strings.SplitSeq(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "name: "):
			if name != "" {
				return "", nil, nil, ErrScope
			}
			name = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name: ")), "'\"")
		case line == "learningTargets:" || line == "denyPatterns:":
			section = strings.TrimSuffix(line, ":")
		case strings.HasPrefix(line, "  - ") && section != "":
			value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "  - ")), "'\"")
			if value == "" || strings.Contains(value, "\\") {
				return "", nil, nil, ErrScope
			}
			lists[section] = append(lists[section], value)
		case line != "" && !strings.HasPrefix(line, "  "):
			section = ""
		}
	}
	if !packNamePattern.MatchString(name) || len(lists["learningTargets"]) == 0 || len(lists["denyPatterns"]) == 0 {
		return "", nil, nil, ErrScope
	}
	return name, lists["learningTargets"], lists["denyPatterns"], nil
}

func destinationAllowed(destination, pack string, patterns []string) bool {
	prefix := "packs/" + pack + "/"
	if !strings.HasPrefix(destination, prefix) || filepath.Ext(destination) != ".md" || strings.Contains(destination, "\\") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(destination)))
	if clean != destination {
		return false
	}
	rel := strings.TrimPrefix(destination, prefix)
	for _, pattern := range patterns {
		match, err := filepath.Match(filepath.FromSlash(pattern), filepath.FromSlash(rel))
		if err == nil && match {
			return true
		}
	}
	return false
}

func hasExactFullIndexLine(patch, target, oldBlob string) bool {
	header := "diff --git a/" + target + " b/" + target + "\n"
	_, section, found := strings.Cut(patch, header)
	if !found {
		return false
	}
	section, _, _ = strings.Cut(section, "\ndiff --git ")
	count := 0
	for line := range strings.SplitSeq(section, "\n") {
		if !strings.HasPrefix(line, "index ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "100644" {
			return false
		}
		oldNew := strings.Split(fields[1], "..")
		if len(oldNew) != 2 || oldNew[0] != oldBlob || !hexGit.MatchString(oldNew[1]) {
			return false
		}
		count++
	}
	return count == 1
}

func validatePatchScope(git, source, patchPath, pack string, patterns, denyPatterns []string) ([]string, error) {
	patch, err := os.ReadFile(patchPath)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(patch), "\r\n", "\n")
	for _, forbidden := range []string{"GIT binary patch", "Binary files ", "new file mode ", "deleted file mode ", "old mode ", "new mode ", "similarity index ", "rename from ", "rename to ", "copy from ", "copy to ", "/dev/null"} {
		if strings.Contains(text, forbidden) {
			return nil, ErrScope
		}
	}
	out, err := gitOutput(git, source, "apply", "--numstat", "-z", patchPath)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(out, "\x00")
	if len(fields) < 2 || fields[len(fields)-1] != "" {
		return nil, ErrScope
	}
	var targets []string
	seen := map[string]bool{}
	for _, field := range fields[:len(fields)-1] {
		parts := strings.Split(field, "\t")
		if len(parts) != 3 || parts[0] == "-" || parts[1] == "-" {
			return nil, ErrScope
		}
		target := filepath.ToSlash(parts[2])
		if seen[target] || !destinationAllowed(target, pack, patterns) {
			return nil, ErrScope
		}
		seen[target] = true
		targets = append(targets, target)
		path := filepath.Join(source, filepath.FromSlash(target))
		if err := requirePlainPath(source, path, false); err != nil {
			return nil, ErrScope
		}
		indexBlob, err := trackedStageZeroBlob(git, source, target)
		if err != nil {
			return nil, ErrScope
		}
		workingBlob, err := gitOutput(git, source, "hash-object", "--path="+target, "--", target)
		if err != nil {
			return nil, ErrScope
		}
		workingBlob = strings.TrimSpace(workingBlob)
		if !hexGit.MatchString(workingBlob) || indexBlob == "" || !hasExactFullIndexLine(text, target, workingBlob) {
			return nil, ErrScope
		}
		if strings.Count(text, "diff --git a/"+target+" b/"+target+"\n") != 1 ||
			!strings.Contains(text, "--- a/"+target+"\n") || !strings.Contains(text, "+++ b/"+target+"\n") {
			return nil, ErrScope
		}
	}
	if len(targets) == 0 || strings.Count(text, "diff --git ") != len(targets) {
		return nil, ErrScope
	}
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		added := strings.TrimPrefix(line, "+")
		for _, deny := range denyPatterns {
			if deny != "" && strings.Contains(added, deny) {
				return nil, ErrScope
			}
		}
	}
	sort.Strings(targets)
	return targets, nil
}
