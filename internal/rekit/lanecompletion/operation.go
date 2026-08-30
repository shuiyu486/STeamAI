package lanecompletion

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	OperationsDir              = "reopen-operations"
	exactPublicationHashMarker = "<reopen-exact-publication-plan-sha256>"
)

var operationName = regexp.MustCompile(`^([0-9]{20})$`)

type PublicationMode string

const (
	PublicationCreateExclusive PublicationMode = "create-exclusive"
	PublicationReplaceExact    PublicationMode = "replace-exact"
)

type OperationPublication struct {
	Path         string          `json:"path"`
	Role         string          `json:"role"`
	Mode         PublicationMode `json:"mode"`
	BeforeExists bool            `json:"beforeExists"`
	BeforeSHA256 string          `json:"beforeSha256,omitempty"`
	AfterSHA256  string          `json:"afterSha256"`
	Bytes        []byte          `json:"bytes"`
}

type OperationTarget struct {
	Lane                         string                 `json:"lane"`
	Sequence                     int                    `json:"sequence"`
	PreviousReceiptSHA           string                 `json:"previousReceiptSha256"`
	SupersededCompletionSequence int                    `json:"supersededCompletionSequence"`
	IntentPath                   string                 `json:"intentPath"`
	ReceiptPath                  string                 `json:"receiptPath"`
	ReceiptSHA256                string                 `json:"receiptSha256,omitempty"`
	Reason                       string                 `json:"reason"`
	Publications                 []OperationPublication `json:"publications"`
}

type OperationIntent struct {
	SchemaVersion          int                    `json:"schemaVersion"`
	Kind                   string                 `json:"kind"`
	OperationID            string                 `json:"operationId"`
	Sequence               int                    `json:"sequence"`
	RequestedLane          string                 `json:"requestedLane"`
	RequestedSelector      string                 `json:"requestedSelector"`
	Actor                  string                 `json:"actor"`
	Reason                 string                 `json:"reason"`
	EvidenceRefs           []string               `json:"evidenceRefs"`
	Evidence               []Evidence             `json:"evidence"`
	Targets                []OperationTarget      `json:"targets"`
	Publications           []OperationPublication `json:"publications"`
	CreatedAt              string                 `json:"createdAt"`
	PreviewSHA256          string                 `json:"previewSha256"`
	ExactPublicationSHA256 string                 `json:"exactPublicationSha256"`
	NoAuthority            bool                   `json:"noAuthority"`
	NoConfirmed            bool                   `json:"noConfirmed"`
	NoHeavyTool            bool                   `json:"noHeavyTool"`
	NoAutoResume           bool                   `json:"noAutoResume"`
}

type OperationCommit struct {
	SchemaVersion int               `json:"schemaVersion"`
	Kind          string            `json:"kind"`
	State         string            `json:"state"`
	OperationID   string            `json:"operationId"`
	Sequence      int               `json:"sequence"`
	RequestedLane string            `json:"requestedLane"`
	Actor         string            `json:"actor"`
	Reason        string            `json:"reason"`
	Targets       []OperationTarget `json:"targets"`
	CommittedAt   string            `json:"committedAt"`
	PreviewSHA256 string            `json:"previewSha256"`
	IntentSHA256  string            `json:"intentSha256"`
	NoAuthority   bool              `json:"noAuthority"`
	NoConfirmed   bool              `json:"noConfirmed"`
	NoHeavyTool   bool              `json:"noHeavyTool"`
	NoAutoResume  bool              `json:"noAutoResume"`
}

type OperationInspection struct {
	Ready             bool
	Pending           bool
	LatestSequence    int
	LatestCommitSHA   string
	LatestIntent      *OperationIntent
	PendingSequence   int
	PendingIntentPath string
	Commits           []OperationCommit
}

func InspectOperations(caseRoot string) (OperationInspection, error) {
	out := OperationInspection{Ready: true}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return out, err
	}
	rootPath := filepath.Join(view.Path, OperationsDir)
	root, err := openMissionNamespaceRoot(caseRoot, view, []string{OperationsDir}, false)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	defer root.Close()
	entries, err := readNamespaceEntries(root)
	if err != nil {
		return out, err
	}
	sequences := []int{}
	names := map[int]string{}
	for _, entry := range entries {
		match := operationName.FindStringSubmatch(entry.Name())
		if match == nil || entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			return out, fmt.Errorf("unexpected reopen operation entry: %s", entry.Name())
		}
		value, _ := strconv.ParseInt(match[1], 10, 64)
		sequence := int(value)
		sequences = append(sequences, sequence)
		names[sequence] = entry.Name()
	}
	sort.Ints(sequences)
	for index, sequence := range sequences {
		if sequence != index+1 {
			return out, fmt.Errorf("reopen operation sequence gap: got=%d want=%d", sequence, index+1)
		}
		name := names[sequence]
		dirPath := filepath.Join(rootPath, name)
		dir, err := openChildRootNoFollow(root, name, dirPath)
		if err != nil {
			return out, err
		}
		children, err := readNamespaceEntries(dir)
		if err != nil {
			_ = dir.Close()
			return out, err
		}
		for _, child := range children {
			if (child.Name() != "intent.json" && child.Name() != "commit.json") || child.Type()&os.ModeSymlink != 0 || !child.Type().IsRegular() {
				_ = dir.Close()
				return out, fmt.Errorf("unexpected reopen operation artifact: sequence=%d name=%s", sequence, child.Name())
			}
		}
		intentPath := filepath.Join(dirPath, "intent.json")
		commitPath := filepath.Join(dirPath, "commit.json")
		intentExists, err := regularExistsRoot(dir, "intent.json", intentPath)
		if err != nil {
			_ = dir.Close()
			return out, err
		}
		commitExists, err := regularExistsRoot(dir, "commit.json", commitPath)
		if err != nil {
			_ = dir.Close()
			return out, err
		}
		if !intentExists {
			_ = dir.Close()
			return out, fmt.Errorf("reopen operation lacks intent: sequence=%d", sequence)
		}
		var intent OperationIntent
		_, err = readStrictRoot(dir, "intent.json", intentPath, &intent)
		if err != nil {
			_ = dir.Close()
			return out, err
		}
		if err := validateOperationIntent(caseRoot, sequence, intent); err != nil {
			_ = dir.Close()
			return out, err
		}
		if !commitExists {
			_ = dir.Close()
			if index != len(sequences)-1 {
				return out, fmt.Errorf("non-latest reopen operation is pending: sequence=%d", sequence)
			}
			out.Ready, out.Pending, out.PendingSequence, out.PendingIntentPath = false, true, sequence, intentPath
			return out, nil
		}
		var commit OperationCommit
		commitBytes, err := readStrictRoot(dir, "commit.json", commitPath, &commit)
		_ = dir.Close()
		if err != nil {
			return out, err
		}
		intentSHA, err := canonicalSHA(intent)
		if err != nil {
			return out, err
		}
		if commit.SchemaVersion != 1 || commit.Kind != "lane-reopen-operation" || commit.State != "committed" || commit.Sequence != sequence || commit.OperationID != intent.OperationID || commit.RequestedLane != intent.RequestedLane || commit.Actor != intent.Actor || commit.Reason != intent.Reason || commit.PreviewSHA256 != intent.PreviewSHA256 || !strings.EqualFold(commit.IntentSHA256, intentSHA) || !commit.NoAuthority || !commit.NoConfirmed || !commit.NoHeavyTool || !commit.NoAutoResume {
			return out, fmt.Errorf("invalid reopen operation commit: sequence=%d intentSha=%s commitIntentSha=%s intentPreview=%s commitPreview=%s", sequence, intentSHA, commit.IntentSHA256, intent.PreviewSHA256, commit.PreviewSHA256)
		}
		if len(commit.Targets) != len(intent.Targets) {
			return out, fmt.Errorf("reopen operation commit target count mismatch: sequence=%d", sequence)
		}
		for targetIndex := range intent.Targets {
			planned, committed := intent.Targets[targetIndex], commit.Targets[targetIndex]
			if planned.Lane != committed.Lane || planned.Sequence != committed.Sequence || planned.SupersededCompletionSequence != committed.SupersededCompletionSequence || !strings.EqualFold(planned.PreviousReceiptSHA, committed.PreviousReceiptSHA) || planned.IntentPath != committed.IntentPath || planned.ReceiptPath != committed.ReceiptPath || planned.Reason != committed.Reason || strings.TrimSpace(committed.ReceiptSHA256) == "" || !operationPublicationsEqual(planned.Publications, committed.Publications) {
				return out, fmt.Errorf("reopen operation commit target mismatch: sequence=%d lane=%s", sequence, planned.Lane)
			}
			receiptPath, err := safeOperationTargetPath(caseRoot, committed.ReceiptPath)
			if err != nil {
				return out, err
			}
			var receipt ReopenReceipt
			receiptBytes, err := readStrictUnder(caseRoot, receiptPath, &receipt)
			if err != nil || receipt.OperationID != intent.OperationID || receipt.Lane != committed.Lane || receipt.Sequence != committed.Sequence || !strings.EqualFold(receipt.PreviousReceiptSHA, committed.PreviousReceiptSHA) || !strings.EqualFold(bytesSHA(receiptBytes), committed.ReceiptSHA256) {
				return out, fmt.Errorf("reopen operation target receipt mismatch: sequence=%d lane=%s", sequence, committed.Lane)
			}
		}
		intentCopy := intent
		out.LatestSequence, out.LatestCommitSHA, out.LatestIntent = sequence, bytesSHA(commitBytes), &intentCopy
		out.Commits = append(out.Commits, commit)
	}
	return out, nil
}

func OperationIntentPathE(caseRoot string, sequence int) (string, error) {
	return projectstate.Join(caseRoot, OperationsDir, fmt.Sprintf("%020d", sequence), "intent.json")
}

func OperationIntentPath(caseRoot string, sequence int) string {
	return mustResolvedPath(OperationIntentPathE(caseRoot, sequence))
}

func OperationCommitPathE(caseRoot string, sequence int) (string, error) {
	return projectstate.Join(caseRoot, OperationsDir, fmt.Sprintf("%020d", sequence), "commit.json")
}

func OperationCommitPath(caseRoot string, sequence int) string {
	return mustResolvedPath(OperationCommitPathE(caseRoot, sequence))
}

func ReadOperationIntent(caseRoot, path string) (OperationIntent, error) {
	var intent OperationIntent
	_, err := readStrictUnder(caseRoot, path, &intent)
	return intent, err
}

func ReadReopenIntent(caseRoot, path string) (ReopenIntent, error) {
	var intent ReopenIntent
	_, err := readStrictUnder(caseRoot, path, &intent)
	return intent, err
}

func ReadReopenReceipt(caseRoot, path string) (ReopenReceipt, []byte, error) {
	var receipt ReopenReceipt
	data, err := readStrictUnder(caseRoot, path, &receipt)
	return receipt, data, err
}

func validateOperationIntent(caseRoot string, sequence int, intent OperationIntent) error {
	if intent.SchemaVersion != 1 || intent.Kind != "lane-reopen-operation-intent" || intent.Sequence != sequence || strings.TrimSpace(intent.OperationID) == "" || strings.TrimSpace(intent.RequestedLane) == "" || strings.TrimSpace(intent.Actor) == "" || strings.TrimSpace(intent.Reason) == "" || len(intent.Targets) == 0 || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool || !intent.NoAutoResume {
		return fmt.Errorf("invalid reopen operation intent: sequence=%d", sequence)
	}
	previous := ""
	for _, target := range intent.Targets {
		if strings.TrimSpace(target.Lane) == "" || target.Lane <= previous || target.Sequence < 2 || target.SupersededCompletionSequence != target.Sequence-1 || strings.TrimSpace(target.PreviousReceiptSHA) == "" || strings.TrimSpace(target.IntentPath) == "" || strings.TrimSpace(target.ReceiptPath) == "" || len(target.Publications) == 0 {
			return fmt.Errorf("invalid or unsorted reopen operation target: sequence=%d lane=%s", sequence, target.Lane)
		}
		previous = target.Lane
	}
	if strings.TrimSpace(intent.RequestedSelector) == "" || strings.TrimSpace(intent.ExactPublicationSHA256) == "" || len(intent.Publications) == 0 {
		return fmt.Errorf("reopen operation intent lacks exact publication plan: sequence=%d", sequence)
	}
	if err := validateOperationPublications(caseRoot, intent); err != nil {
		return err
	}
	recomputed, err := ExactPublicationSHA256(intent)
	if err != nil || !strings.EqualFold(recomputed, intent.ExactPublicationSHA256) || !strings.EqualFold(recomputed, intent.PreviewSHA256) {
		return fmt.Errorf("reopen operation reviewed publication hash mismatch: sequence=%d", sequence)
	}
	return nil
}

func ExactPublicationSHA256(intent OperationIntent) (string, error) {
	planSHA := intent.ExactPublicationSHA256
	raw, err := json.Marshal(intent)
	if err != nil {
		return "", err
	}
	var copyIntent OperationIntent
	if err := json.Unmarshal(raw, &copyIntent); err != nil {
		return "", err
	}
	copyIntent.PreviewSHA256 = ""
	copyIntent.ExactPublicationSHA256 = ""
	for targetIndex := range copyIntent.Targets {
		copyIntent.Targets[targetIndex].ReceiptSHA256 = ""
		for publicationIndex := range copyIntent.Targets[targetIndex].Publications {
			if err := normalizePublicationForHash(&copyIntent.Targets[targetIndex].Publications[publicationIndex], planSHA); err != nil {
				return "", err
			}
		}
	}
	for publicationIndex := range copyIntent.Publications {
		if err := normalizePublicationForHash(&copyIntent.Publications[publicationIndex], planSHA); err != nil {
			return "", err
		}
	}
	data, err := json.Marshal(copyIntent)
	if err != nil {
		return "", err
	}
	return bytesSHA(data), nil
}

func normalizePublicationForHash(publication *OperationPublication, planSHA string) error {
	if planSHA != "" {
		publication.Bytes = []byte(strings.ReplaceAll(string(publication.Bytes), planSHA, exactPublicationHashMarker))
	}
	if publication.Role == "lane-reopen-commit" {
		var receipt ReopenReceipt
		if err := json.Unmarshal(publication.Bytes, &receipt); err != nil {
			return err
		}
		receipt.IntentSHA256 = ""
		data, err := json.MarshalIndent(receipt, "", "  ")
		if err != nil {
			return err
		}
		publication.Bytes = append(data, '\n')
	}
	publication.AfterSHA256 = bytesSHA(publication.Bytes)
	return nil
}

func operationPublicationsEqual(left, right []OperationPublication) bool {
	leftBytes, _ := json.Marshal(left)
	rightBytes, _ := json.Marshal(right)
	return string(leftBytes) == string(rightBytes)
}

func validateTargetPublicationTopology(caseRoot string, target OperationTarget) error {
	base, err := projectstate.Rel(caseRoot, "lanes", target.Lane)
	if err != nil {
		return err
	}
	sequence := fmt.Sprintf("%020d.reopen", target.Sequence)
	expected := map[string]string{
		"lane-reopen-intent": base + "/" + LifecycleDir + "/" + sequence + ".intent.json",
		"lane-event":         base + "/events.jsonl",
		"lane":               base + "/lane.json",
		"lane-resume":        base + "/prompts/RESUME.md",
		"lane-checkpoint":    base + "/checkpoints/latest.json",
		"lane-reopen-commit": base + "/" + LifecycleDir + "/" + sequence + ".json",
	}
	if len(target.Publications) != len(expected) {
		return fmt.Errorf("reopen target publication topology mismatch: %s", target.Lane)
	}
	for _, publication := range target.Publications {
		want, ok := expected[publication.Role]
		if !ok || filepath.ToSlash(publication.Path) != want {
			return fmt.Errorf("reopen target publication path mismatch: lane=%s role=%s", target.Lane, publication.Role)
		}
		delete(expected, publication.Role)
	}
	if filepath.ToSlash(target.IntentPath) != base+"/"+LifecycleDir+"/"+sequence+".intent.json" || filepath.ToSlash(target.ReceiptPath) != base+"/"+LifecycleDir+"/"+sequence+".json" {
		return fmt.Errorf("reopen target lifecycle path mismatch: %s", target.Lane)
	}
	return nil
}

func validateOperationPublications(caseRoot string, intent OperationIntent) error {
	seen := map[string]bool{}
	boardRel, err := projectstate.Rel(caseRoot, "board.json")
	if err != nil {
		return err
	}
	if len(intent.Publications) != 1 || intent.Publications[0].Role != "board" || filepath.ToSlash(intent.Publications[0].Path) != boardRel {
		return fmt.Errorf("reopen operation requires one exact board publication")
	}
	all := append([]OperationPublication{}, intent.Publications...)
	for _, target := range intent.Targets {
		if err := validateTargetPublicationTopology(caseRoot, target); err != nil {
			return err
		}
		all = append(all, target.Publications...)
	}
	for _, publication := range all {
		if publication.Path == "" || seen[publication.Path] || len(publication.Bytes) == 0 || len(publication.Bytes) > maxArtifact || !strings.EqualFold(bytesSHA(publication.Bytes), publication.AfterSHA256) {
			return fmt.Errorf("invalid reopen operation publication: %s", publication.Path)
		}
		if publication.Mode != PublicationCreateExclusive && publication.Mode != PublicationReplaceExact {
			return fmt.Errorf("invalid reopen operation publication mode: %s", publication.Path)
		}
		if publication.BeforeExists != (strings.TrimSpace(publication.BeforeSHA256) != "") {
			return fmt.Errorf("invalid reopen operation predecessor identity: %s", publication.Path)
		}
		seen[publication.Path] = true
	}
	return nil
}

func safeOperationTargetPath(caseRoot, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("reopen operation target path must be case-relative: %s", rel)
	}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return "", err
	}
	path := filepath.Clean(filepath.Join(caseRoot, filepath.FromSlash(rel)))
	base := filepath.Clean(caseRoot)
	relative, err := filepath.Rel(base, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reopen operation target escapes case root: %s", rel)
	}
	stateRelative, err := filepath.Rel(view.Path, path)
	if err != nil || stateRelative == "." || stateRelative == ".." || strings.HasPrefix(stateRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reopen operation target is outside selected mission root %s: %s", view.Path, rel)
	}
	return path, nil
}

func MarshalCanonical(value any) ([]byte, error) { return json.Marshal(value) }
func SHA256Bytes(data []byte) string             { return bytesSHA(data) }
func CanonicalSHA256(value any) (string, error)  { return canonicalSHA(value) }
