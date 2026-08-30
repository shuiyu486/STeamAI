package lanecompletion

import (
	"crypto/sha256"
	"encoding/hex"
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
	IntentFile    = "completion.intent.json"
	CommitFile    = "completion.json"
	LifecycleDir  = "completion-lifecycle"
	maxArtifact   = 1 << 20
	maxOperation  = 16 << 20
	StateNone     = "none"
	StateComplete = "closed-effective"
	StateReopened = "open-reopened"
	StatePending  = "publication-pending"
)

var lifecycleName = regexp.MustCompile(`^(\d{20})\.(complete|reopen)\.(intent\.json|json)$`)

type Evidence struct {
	Ref    string `json:"ref"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type CompletionIntent struct {
	SchemaVersion      int        `json:"schemaVersion"`
	Kind               string     `json:"kind"`
	Sequence           int        `json:"sequence,omitempty"`
	PreviousReceiptSHA string     `json:"previousReceiptSha256,omitempty"`
	Lane               string     `json:"lane"`
	Label              string     `json:"label"`
	Authority          bool       `json:"authority"`
	PreviousStatus     string     `json:"previousStatus"`
	Actor              string     `json:"actor"`
	Reason             string     `json:"reason"`
	EvidenceRefs       []string   `json:"evidenceRefs"`
	Evidence           []Evidence `json:"evidence"`
	CurrentExecutor    string     `json:"currentExecutor,omitempty"`
	ExecutorGeneration int        `json:"executorGeneration,omitempty"`
	CreatedAt          string     `json:"createdAt"`
	EventID            string     `json:"eventId"`
	PreviewSHA256      string     `json:"previewSha256"`
	NoAuthority        bool       `json:"noAuthority"`
	NoConfirmed        bool       `json:"noConfirmed"`
	NoHeavyTool        bool       `json:"noHeavyTool"`
}

type CompletionReceipt struct {
	SchemaVersion      int        `json:"schemaVersion"`
	Kind               string     `json:"kind"`
	State              string     `json:"state"`
	Sequence           int        `json:"sequence,omitempty"`
	PreviousReceiptSHA string     `json:"previousReceiptSha256,omitempty"`
	Lane               string     `json:"lane"`
	Label              string     `json:"label"`
	Authority          bool       `json:"authority"`
	PreviousStatus     string     `json:"previousStatus"`
	Actor              string     `json:"actor"`
	Reason             string     `json:"reason"`
	EvidenceRefs       []string   `json:"evidenceRefs"`
	Evidence           []Evidence `json:"evidence"`
	CurrentExecutor    string     `json:"currentExecutor,omitempty"`
	ExecutorGeneration int        `json:"executorGeneration,omitempty"`
	CompletedAt        string     `json:"completedAt"`
	EventID            string     `json:"eventId"`
	PreviewSHA256      string     `json:"previewSha256"`
	IntentSHA256       string     `json:"intentSha256"`
	LaneSHA256         string     `json:"laneSha256"`
	BoardLaneSHA256    string     `json:"boardLaneSha256"`
	ResumeSHA256       string     `json:"resumeSha256"`
	CheckpointSHA256   string     `json:"checkpointSha256"`
	NoAuthority        bool       `json:"noAuthority"`
	NoConfirmed        bool       `json:"noConfirmed"`
	NoHeavyTool        bool       `json:"noHeavyTool"`
}

type ReopenIntent struct {
	SchemaVersion                int        `json:"schemaVersion"`
	Kind                         string     `json:"kind"`
	OperationID                  string     `json:"operationId"`
	Sequence                     int        `json:"sequence"`
	PreviousReceiptSHA           string     `json:"previousReceiptSha256"`
	SupersededCompletionSequence int        `json:"supersededCompletionSequence"`
	SupersededCompletionSHA256   string     `json:"supersededCompletionSha256"`
	Lane                         string     `json:"lane"`
	Label                        string     `json:"label"`
	Authority                    bool       `json:"authority"`
	PreviousStatus               string     `json:"previousStatus"`
	Actor                        string     `json:"actor"`
	Reason                       string     `json:"reason"`
	EvidenceRefs                 []string   `json:"evidenceRefs"`
	Evidence                     []Evidence `json:"evidence"`
	PreviousExecutor             string     `json:"previousExecutor,omitempty"`
	PreviousExecutorGeneration   int        `json:"previousExecutorGeneration,omitempty"`
	ResultingExecutorGeneration  int        `json:"resultingExecutorGeneration"`
	CreatedAt                    string     `json:"createdAt"`
	EventID                      string     `json:"eventId"`
	PreviewSHA256                string     `json:"previewSha256"`
	NoAuthority                  bool       `json:"noAuthority"`
	NoConfirmed                  bool       `json:"noConfirmed"`
	NoHeavyTool                  bool       `json:"noHeavyTool"`
	NoAutoResume                 bool       `json:"noAutoResume"`
}

type ReopenReceipt struct {
	SchemaVersion                int        `json:"schemaVersion"`
	Kind                         string     `json:"kind"`
	OperationID                  string     `json:"operationId"`
	State                        string     `json:"state"`
	Sequence                     int        `json:"sequence"`
	PreviousReceiptSHA           string     `json:"previousReceiptSha256"`
	SupersededCompletionSequence int        `json:"supersededCompletionSequence"`
	SupersededCompletionSHA256   string     `json:"supersededCompletionSha256"`
	Lane                         string     `json:"lane"`
	Label                        string     `json:"label"`
	Authority                    bool       `json:"authority"`
	PreviousStatus               string     `json:"previousStatus"`
	Actor                        string     `json:"actor"`
	Reason                       string     `json:"reason"`
	EvidenceRefs                 []string   `json:"evidenceRefs"`
	Evidence                     []Evidence `json:"evidence"`
	PreviousExecutor             string     `json:"previousExecutor,omitempty"`
	PreviousExecutorGeneration   int        `json:"previousExecutorGeneration,omitempty"`
	ResultingExecutorGeneration  int        `json:"resultingExecutorGeneration"`
	ReopenedAt                   string     `json:"reopenedAt"`
	EventID                      string     `json:"eventId"`
	PreviewSHA256                string     `json:"previewSha256"`
	IntentSHA256                 string     `json:"intentSha256"`
	LaneSHA256                   string     `json:"laneSha256"`
	BoardLaneSHA256              string     `json:"boardLaneSha256"`
	ResumeSHA256                 string     `json:"resumeSha256"`
	CheckpointSHA256             string     `json:"checkpointSha256"`
	NoAuthority                  bool       `json:"noAuthority"`
	NoConfirmed                  bool       `json:"noConfirmed"`
	NoHeavyTool                  bool       `json:"noHeavyTool"`
	NoAutoResume                 bool       `json:"noAutoResume"`
}

type Transition struct {
	Sequence      int
	Kind          string
	IntentPath    string
	ReceiptPath   string
	IntentSHA256  string
	ReceiptSHA256 string
	Completion    *CompletionReceipt
	Reopen        *ReopenReceipt
}

type Inspection struct {
	State             string
	HeadSequence      int
	HeadKind          string
	HeadReceiptSHA256 string
	PendingSequence   int
	PendingKind       string
	PendingIntentPath string
	Transitions       []Transition
	CurrentCompletion *CompletionReceipt
	CurrentReopen     *ReopenReceipt
}

func Inspect(caseRoot, laneID string) (Inspection, error) {
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil {
		return Inspection{}, err
	}
	laneRootPath := filepath.Join(view.Path, "lanes", laneID)
	out := Inspection{State: StateNone}
	laneRoot, err := openMissionNamespaceRoot(caseRoot, view, []string{"lanes", laneID}, false)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	defer laneRoot.Close()
	intentPath := filepath.Join(laneRootPath, IntentFile)
	receiptPath := filepath.Join(laneRootPath, CommitFile)
	intentExists, err := regularExistsRoot(laneRoot, IntentFile, intentPath)
	if err != nil {
		return out, err
	}
	receiptExists, err := regularExistsRoot(laneRoot, CommitFile, receiptPath)
	if err != nil {
		return out, err
	}
	if intentExists != receiptExists {
		out.State, out.PendingSequence, out.PendingKind, out.PendingIntentPath = StatePending, 1, "complete", intentPath
		if receiptExists {
			return out, fmt.Errorf("lane completion receipt exists without intent: %s", laneID)
		}
		return inspectGenerated(laneRoot, laneRootPath, laneID, out)
	}
	if intentExists {
		var intent CompletionIntent
		intentBytes, err := readStrictRoot(laneRoot, IntentFile, intentPath, &intent)
		if err != nil {
			return out, err
		}
		var receipt CompletionReceipt
		receiptBytes, err := readStrictRoot(laneRoot, CommitFile, receiptPath, &receipt)
		if err != nil {
			return out, err
		}
		if err := validateCompletionPair(laneID, 1, "", intent, receipt); err != nil {
			return out, err
		}
		intentSHA, err := canonicalSHA(intent)
		if err != nil || !strings.EqualFold(intentSHA, receipt.IntentSHA256) {
			return out, fmt.Errorf("lane completion intent hash mismatch: %s", laneID)
		}
		receiptSHA := bytesSHA(receiptBytes)
		out.State, out.HeadSequence, out.HeadKind, out.HeadReceiptSHA256 = StateComplete, 1, "complete", receiptSHA
		out.CurrentCompletion = &receipt
		out.Transitions = append(out.Transitions, Transition{Sequence: 1, Kind: "complete", IntentPath: intentPath, ReceiptPath: receiptPath, IntentSHA256: bytesSHA(intentBytes), ReceiptSHA256: receiptSHA, Completion: &receipt})
	}
	return inspectGenerated(laneRoot, laneRootPath, laneID, out)
}

func inspectGenerated(laneRoot *os.Root, laneRootPath, laneID string, out Inspection) (Inspection, error) {
	lifecyclePath := filepath.Join(laneRootPath, LifecycleDir)
	lifecycleRoot, err := openChildRootNoFollow(laneRoot, LifecycleDir, lifecyclePath)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	defer lifecycleRoot.Close()
	entries, err := readNamespaceEntries(lifecycleRoot)
	if err != nil {
		return out, err
	}
	type pair struct {
		intent, receipt string
		kind            string
	}
	pairs := map[int]*pair{}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return out, fmt.Errorf("lane completion lifecycle entry must be a regular file: %s", entry.Name())
		}
		match := lifecycleName.FindStringSubmatch(entry.Name())
		if match == nil {
			return out, fmt.Errorf("unexpected lane completion lifecycle entry: %s", entry.Name())
		}
		sequence64, _ := strconv.ParseInt(match[1], 10, 64)
		sequence := int(sequence64)
		if sequence < 2 {
			return out, fmt.Errorf("invalid generated lane completion sequence: %d", sequence)
		}
		item := pairs[sequence]
		if item == nil {
			item = &pair{kind: match[2]}
			pairs[sequence] = item
		} else if item.kind != match[2] {
			return out, fmt.Errorf("lane completion sequence has mixed transition kinds: %d", sequence)
		}
		path := filepath.Join(lifecyclePath, entry.Name())
		if match[3] == "intent.json" {
			if item.intent != "" {
				return out, fmt.Errorf("duplicate lane completion intent: %d", sequence)
			}
			item.intent = path
		} else {
			if item.receipt != "" {
				return out, fmt.Errorf("duplicate lane completion receipt: %d", sequence)
			}
			item.receipt = path
		}
	}
	sequences := make([]int, 0, len(pairs))
	for sequence := range pairs {
		sequences = append(sequences, sequence)
	}
	sort.Ints(sequences)
	expected := 2
	previousKind := out.HeadKind
	previousSHA := out.HeadReceiptSHA256
	for index, sequence := range sequences {
		if out.HeadSequence == 0 {
			return out, fmt.Errorf("generated lane completion lifecycle lacks legacy sequence 1: %s", laneID)
		}
		if sequence != expected {
			return out, fmt.Errorf("lane completion lifecycle sequence gap: got %d want %d", sequence, expected)
		}
		item := pairs[sequence]
		wantKind := "complete"
		if previousKind == "complete" {
			wantKind = "reopen"
		}
		if item.kind != wantKind {
			return out, fmt.Errorf("lane completion lifecycle transition does not alternate: sequence=%d got=%s want=%s", sequence, item.kind, wantKind)
		}
		if item.intent == "" {
			return out, fmt.Errorf("lane completion lifecycle receipt lacks intent: sequence=%d", sequence)
		}
		if item.receipt == "" {
			if index != len(sequences)-1 {
				return out, fmt.Errorf("non-latest lane completion lifecycle intent is uncommitted: sequence=%d", sequence)
			}
			out.State, out.PendingSequence, out.PendingKind, out.PendingIntentPath = StatePending, sequence, item.kind, item.intent
			return out, nil
		}
		if item.kind == "complete" {
			var intent CompletionIntent
			intentBytes, err := readStrictRoot(lifecycleRoot, filepath.Base(item.intent), item.intent, &intent)
			if err != nil {
				return out, err
			}
			var receipt CompletionReceipt
			receiptBytes, err := readStrictRoot(lifecycleRoot, filepath.Base(item.receipt), item.receipt, &receipt)
			if err != nil {
				return out, err
			}
			if err := validateCompletionPair(laneID, sequence, previousSHA, intent, receipt); err != nil {
				return out, err
			}
			intentSHA, err := canonicalSHA(intent)
			if err != nil || !strings.EqualFold(intentSHA, receipt.IntentSHA256) {
				return out, fmt.Errorf("generated lane completion intent hash mismatch: %s sequence=%d", laneID, sequence)
			}
			previousSHA = bytesSHA(receiptBytes)
			copyReceipt := receipt
			out.CurrentCompletion, out.CurrentReopen = &copyReceipt, nil
			out.Transitions = append(out.Transitions, Transition{Sequence: sequence, Kind: item.kind, IntentPath: item.intent, ReceiptPath: item.receipt, IntentSHA256: bytesSHA(intentBytes), ReceiptSHA256: previousSHA, Completion: &copyReceipt})
			out.State = StateComplete
		} else {
			var intent ReopenIntent
			intentBytes, err := readStrictRoot(lifecycleRoot, filepath.Base(item.intent), item.intent, &intent)
			if err != nil {
				return out, err
			}
			var receipt ReopenReceipt
			receiptBytes, err := readStrictRoot(lifecycleRoot, filepath.Base(item.receipt), item.receipt, &receipt)
			if err != nil {
				return out, err
			}
			if err := validateReopenPair(laneID, sequence, previousSHA, intent, receipt); err != nil {
				return out, err
			}
			intentSHA, err := canonicalSHA(intent)
			if err != nil || !strings.EqualFold(intentSHA, receipt.IntentSHA256) {
				return out, fmt.Errorf("lane reopen intent hash mismatch: %s sequence=%d", laneID, sequence)
			}
			previousSHA = bytesSHA(receiptBytes)
			copyReceipt := receipt
			out.CurrentReopen, out.CurrentCompletion = &copyReceipt, nil
			out.Transitions = append(out.Transitions, Transition{Sequence: sequence, Kind: item.kind, IntentPath: item.intent, ReceiptPath: item.receipt, IntentSHA256: bytesSHA(intentBytes), ReceiptSHA256: previousSHA, Reopen: &copyReceipt})
			out.State = StateReopened
		}
		out.HeadSequence, out.HeadKind, out.HeadReceiptSHA256 = sequence, item.kind, previousSHA
		previousKind = item.kind
		expected++
	}
	return out, nil
}

func validateCompletionPair(lane string, sequence int, previousSHA string, intent CompletionIntent, receipt CompletionReceipt) error {
	intentSequence := intent.Sequence
	receiptSequence := receipt.Sequence
	if sequence == 1 {
		intentSequence, receiptSequence = 1, 1
	}
	if intent.SchemaVersion != 1 || intent.Kind != "lane-completion-intent" || intentSequence != sequence || intent.Lane != lane || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool ||
		receipt.SchemaVersion != 1 || receipt.Kind != "lane-completion" || receipt.State != "committed" || receiptSequence != sequence || receipt.Lane != lane || !receipt.NoAuthority || !receipt.NoConfirmed || !receipt.NoHeavyTool ||
		!strings.EqualFold(intent.PreviousReceiptSHA, previousSHA) || !strings.EqualFold(receipt.PreviousReceiptSHA, previousSHA) || intent.EventID != receipt.EventID || intent.PreviewSHA256 != receipt.PreviewSHA256 || intent.Actor != receipt.Actor || intent.Reason != receipt.Reason {
		return fmt.Errorf("invalid lane completion lifecycle pair: %s sequence=%d", lane, sequence)
	}
	return nil
}

func validateReopenPair(lane string, sequence int, previousSHA string, intent ReopenIntent, receipt ReopenReceipt) error {
	if intent.SchemaVersion != 1 || intent.Kind != "lane-reopen-intent" || strings.TrimSpace(intent.OperationID) == "" || intent.Sequence != sequence || intent.Lane != lane || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool || !intent.NoAutoResume ||
		receipt.SchemaVersion != 1 || receipt.Kind != "lane-reopen" || receipt.State != "committed" || receipt.OperationID != intent.OperationID || receipt.Sequence != sequence || receipt.Lane != lane || !receipt.NoAuthority || !receipt.NoConfirmed || !receipt.NoHeavyTool || !receipt.NoAutoResume ||
		!strings.EqualFold(intent.PreviousReceiptSHA, previousSHA) || !strings.EqualFold(receipt.PreviousReceiptSHA, previousSHA) || !strings.EqualFold(intent.SupersededCompletionSHA256, previousSHA) || !strings.EqualFold(receipt.SupersededCompletionSHA256, previousSHA) || intent.EventID != receipt.EventID || intent.PreviewSHA256 != receipt.PreviewSHA256 || intent.Actor != receipt.Actor || intent.Reason != receipt.Reason || intent.ResultingExecutorGeneration != receipt.ResultingExecutorGeneration {
		return fmt.Errorf("invalid lane reopen lifecycle pair: %s sequence=%d", lane, sequence)
	}
	return nil
}

func IntentPathE(caseRoot, laneID string, sequence int, kind string) (string, error) {
	if sequence == 1 && kind == "complete" {
		return projectstate.Join(caseRoot, "lanes", laneID, IntentFile)
	}
	return projectstate.Join(caseRoot, "lanes", laneID, LifecycleDir, fmt.Sprintf("%020d.%s.intent.json", sequence, kind))
}

// IntentPath is retained for existing callers. New error-returning flows should
// use IntentPathE so a conflicting state root can be reported explicitly. The
// compatibility API panics rather than returning an empty, potentially unsafe
// path when root resolution fails.
func IntentPath(caseRoot, laneID string, sequence int, kind string) string {
	return mustResolvedPath(IntentPathE(caseRoot, laneID, sequence, kind))
}

func ReceiptPathE(caseRoot, laneID string, sequence int, kind string) (string, error) {
	if sequence == 1 && kind == "complete" {
		return projectstate.Join(caseRoot, "lanes", laneID, CommitFile)
	}
	return projectstate.Join(caseRoot, "lanes", laneID, LifecycleDir, fmt.Sprintf("%020d.%s.json", sequence, kind))
}

// ReceiptPath is retained for existing callers. New error-returning flows
// should use ReceiptPathE.
func ReceiptPath(caseRoot, laneID string, sequence int, kind string) string {
	return mustResolvedPath(ReceiptPathE(caseRoot, laneID, sequence, kind))
}

func mustResolvedPath(path string, err error) string {
	if err != nil {
		panic(err)
	}
	return path
}

func canonicalSHA(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return bytesSHA(data), nil
}

func bytesSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
