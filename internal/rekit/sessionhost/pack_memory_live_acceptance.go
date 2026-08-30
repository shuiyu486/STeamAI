package sessionhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/processguard"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

const (
	packMemoryLiveAcceptanceKind      = "rekit-pack-memory-live-acceptance-receipt"
	packMemoryLiveAcceptanceChildKind = "rekit-pack-memory-live-acceptance-child-result"
)

type PackMemoryLiveAcceptanceOptions struct {
	Goal                           string
	ClaudePath                     string
	Model                          string
	Actor                          string
	Timeout                        time.Duration
	MaxAttempts                    int
	KeepCase                       bool
	ReceiptPath                    string
	InitializationSourceExecutable string
}

type PackMemoryLiveAcceptanceReceipt struct {
	SchemaVersion                int                                 `json:"schemaVersion"`
	Kind                         string                              `json:"kind"`
	Passed                       bool                                `json:"passed"`
	ReceiptPublication           string                              `json:"receiptPublication,omitempty"`
	ReceiptError                 string                              `json:"receiptError,omitempty"`
	SourceRepoRoot               string                              `json:"-"`
	SourceTreeSHA256Before       string                              `json:"sourceTreeSha256Before"`
	SourceTreeSHA256BeforeChild  string                              `json:"sourceTreeSha256BeforeChild"`
	SourceTreeSHA256AfterChild   string                              `json:"sourceTreeSha256AfterChild"`
	SourceTreeSHA256AfterCleanup string                              `json:"sourceTreeSha256AfterCleanup"`
	SourceTreeSHA256After        string                              `json:"sourceTreeSha256After"`
	SourceRepoUnchanged          bool                                `json:"sourceRepoUnchanged"`
	IsolatedKitRoot              string                              `json:"-"`
	IsolationCleanup             string                              `json:"isolationCleanup"`
	ChildSpecSHA256              string                              `json:"childSpecSha256"`
	ChildHostPath                string                              `json:"-"`
	ChildHostSHA256              string                              `json:"childHostSha256"`
	CopyManifestSHA256           string                              `json:"copyManifestSha256"`
	CopyManifestFiles            int                                 `json:"copyManifestFiles"`
	Claude                       LiveAcceptanceClaude                `json:"claude"`
	Child                        PackMemoryLiveAcceptanceChildResult `json:"child"`
	Boundary                     []string                            `json:"boundary"`
}

type PackMemoryLiveAcceptanceChildSpec struct {
	SchemaVersion                  int                              `json:"schemaVersion"`
	Kind                           string                           `json:"kind"`
	IsolatedKitRoot                string                           `json:"isolatedKitRoot"`
	Goal                           string                           `json:"goal"`
	ClaudePath                     string                           `json:"claudePath"`
	ClaudeSHA256                   string                           `json:"claudeSha256"`
	ClaudePublisher                string                           `json:"claudePublisher"`
	Model                          string                           `json:"model,omitempty"`
	Actor                          string                           `json:"actor"`
	TimeoutNanos                   int64                            `json:"timeoutNanos"`
	MaxAttempts                    int                              `json:"maxAttempts"`
	KeepCase                       bool                             `json:"keepCase"`
	ChildHostPath                  string                           `json:"childHostPath"`
	ChildHostSHA256                string                           `json:"childHostSha256"`
	InitializationSourceExecutable string                           `json:"initializationSourceExecutable"`
	CopyManifestSHA256             string                           `json:"copyManifestSha256"`
	CopyManifest                   packMemoryAcceptanceCopyManifest `json:"copyManifest"`
}

type packMemoryAcceptanceCopyManifest struct {
	SchemaVersion int                                    `json:"schemaVersion"`
	Kind          string                                 `json:"kind"`
	Files         []packMemoryAcceptanceCopyManifestFile `json:"files"`
}

type packMemoryAcceptanceCopyManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type PackMemoryLiveAcceptanceProducerProof struct {
	Lane                 string `json:"lane"`
	AttemptID            string `json:"attemptId"`
	TaskContextSHA256    string `json:"taskContextSha256"`
	ManifestSHA256       string `json:"manifestSha256"`
	OutputPath           string `json:"outputPath"`
	OutputSHA256         string `json:"outputSha256"`
	StagingPlanSHA256    string `json:"stagingPlanSha256"`
	StagingReceiptSHA256 string `json:"stagingReceiptSha256"`
	SanitizedSHA256      string `json:"sanitizedSha256"`
	ManagedTargetPath    string `json:"managedTargetPath"`
	CandidateSHA256      string `json:"candidateSha256"`
}

type PackMemoryLiveAcceptanceReviewerProof struct {
	PacketSHA256         string `json:"packetSha256"`
	CandidateSHA256      string `json:"candidateSha256"`
	ReviewerResultSHA256 string `json:"reviewerResultSha256"`
	ReviewerSession      string `json:"reviewerSession"`
	Decision             string `json:"decision"`
	VerificationVerdict  string `json:"verificationVerdict"`
	MainDecision         string `json:"mainDecision"`
	WritebackStatus      string `json:"writebackStatus"`
}

type PackMemoryLiveAcceptancePromotionProof struct {
	DecisionSHA256          string `json:"decisionSha256"`
	DecisionReceiptSHA256   string `json:"decisionReceiptSha256"`
	PromotedSourceSHA256    string `json:"promotedSourceSha256"`
	VerificationProofSHA256 string `json:"verificationProofSha256"`
	RetirementIntentSHA256  string `json:"retirementIntentSha256"`
	RetirementReceiptSHA256 string `json:"retirementReceiptSha256"`
	CleanupProofSHA256      string `json:"cleanupProofSha256"`
	PackDoctorProofSHA256   string `json:"packDoctorProofSha256"`
	FreshProofSHA256        string `json:"freshProofSha256"`
	AttachedProofSHA256     string `json:"attachedProofSha256"`
	ChangeID                string `json:"changeId"`
	AuthoritySHA256         string `json:"authoritySha256"`
}

type PackMemoryLiveAcceptanceConsumerProof struct {
	Lane                     string `json:"lane"`
	AttemptID                string `json:"attemptId"`
	BindingSHA256            string `json:"bindingSha256"`
	ConsumptionPlanSHA256    string `json:"consumptionPlanSha256"`
	ConsumptionReceiptSHA256 string `json:"consumptionReceiptSha256"`
	ManifestSHA256           string `json:"manifestSha256"`
	OutputSHA256             string `json:"outputSha256"`
	QuoteSHA256              string `json:"quoteSha256"`
	AppliedAsSHA256          string `json:"appliedAsSha256"`
	ProofSHA256              string `json:"proofSha256"`
}

type PackMemoryLiveAcceptanceFailure struct {
	Phase             string            `json:"phase"`
	AttemptGeneration int               `json:"attemptGeneration,omitempty"`
	Outcome           string            `json:"outcome"`
	Failure           *FailureDiagnosis `json:"failure,omitempty"`
	Diagnostics       []string          `json:"diagnostics,omitempty"`
}

type PackMemoryLiveAcceptanceChildResult struct {
	SchemaVersion      int                                    `json:"schemaVersion"`
	Kind               string                                 `json:"kind"`
	Passed             bool                                   `json:"passed"`
	Pack               string                                 `json:"pack"`
	ProducerCase       string                                 `json:"-"`
	ConsumerCase       string                                 `json:"-"`
	ProducerLaunches   int                                    `json:"producerLaunches"`
	ReviewerLaunches   int                                    `json:"reviewerLaunches"`
	ConsumerLaunches   int                                    `json:"consumerLaunches"`
	ManualResultWrites int                                    `json:"manualResultWrites"`
	Failures           []PackMemoryLiveAcceptanceFailure      `json:"failures,omitempty"`
	Producer           PackMemoryLiveAcceptanceProducerProof  `json:"producer"`
	Reviewer           PackMemoryLiveAcceptanceReviewerProof  `json:"reviewer"`
	Promotion          PackMemoryLiveAcceptancePromotionProof `json:"promotion"`
	Consumer           PackMemoryLiveAcceptanceConsumerProof  `json:"consumer"`
	Cleanup            string                                 `json:"cleanup"`
	Boundary           []string                               `json:"boundary"`
}

func RunPackMemoryLiveAcceptance(parent context.Context, opt PackMemoryLiveAcceptanceOptions) (receipt PackMemoryLiveAcceptanceReceipt, retErr error) {
	goal := strings.TrimSpace(opt.Goal)
	if goal == "" {
		return receipt, fmt.Errorf("pack-memory live acceptance requires a non-empty natural-language goal")
	}
	if strings.TrimSpace(opt.ClaudePath) != "" {
		return receipt, fmt.Errorf("pack-memory live acceptance refuses a custom Claude executable; omit -claude")
	}
	if opt.Timeout <= 0 {
		opt.Timeout = defaultTimeout
	}
	if opt.MaxAttempts <= 0 {
		opt.MaxAttempts = defaultMaxAttempts
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		actor = "rekit-pack-memory-live-acceptance"
	}
	if strings.ContainsAny(actor, "\r\n") {
		return receipt, fmt.Errorf("pack-memory live acceptance actor must be a single line")
	}
	sourceRepo, err := currentRepoRoot()
	if err != nil {
		return receipt, err
	}
	if path := strings.TrimSpace(opt.ReceiptPath); path != "" {
		full, err := filepath.Abs(path)
		if err != nil {
			return receipt, err
		}
		if liveAcceptancePathWithin(sourceRepo, full) {
			return receipt, fmt.Errorf("pack-memory live acceptance receipt must be outside the source repository: %s", full)
		}
	}
	claude, err := resolveLiveAcceptanceClaude("")
	if err != nil {
		return receipt, err
	}
	before, err := packMemoryAcceptanceTreeSHA256(sourceRepo)
	if err != nil {
		return receipt, err
	}
	isolatedRoot, err := os.MkdirTemp("", "rekit-rh07-pack-memory-kit-")
	if err != nil {
		return receipt, err
	}
	identity := liveAcceptanceCaseIdentity{}
	if err := captureLiveAcceptanceCaseRoot(isolatedRoot, &identity); err != nil {
		_ = os.Remove(isolatedRoot)
		return receipt, err
	}
	defer identity.Close()
	receipt = PackMemoryLiveAcceptanceReceipt{
		SchemaVersion:          1,
		Kind:                   packMemoryLiveAcceptanceKind,
		SourceRepoRoot:         sourceRepo,
		SourceTreeSHA256Before: before,
		IsolatedKitRoot:        isolatedRoot,
		IsolationCleanup:       "pending",
		Claude:                 claude,
		Boundary: []string{
			"this gate is explicit opt-in and is never run by ordinary go test ./...",
			"all producer, ReviewerResult, and consumer-use bytes must come from spawned Claude Code processes",
			"the source repository is read-only to the child and must retain the exact pre-run tree SHA-256",
			"candidate merge, verification, selected sync, and consumer-use proof occur only inside the disposable isolated kit",
			"no authority/confirmed state or heavy-tool execution is permitted",
		},
	}
	defer func() {
		if receipt.SourceTreeSHA256AfterChild == "" {
			afterChild, hashErr := packMemoryAcceptanceTreeSHA256(sourceRepo)
			if hashErr != nil {
				retErr = errors.Join(retErr, hashErr)
			} else {
				receipt.SourceTreeSHA256AfterChild = afterChild
			}
		}
		if opt.KeepCase {
			receipt.IsolationCleanup = "retained-by-request"
		} else if err := removeLiveAcceptanceCase(isolatedRoot, &identity); err != nil {
			receipt.Passed = false
			receipt.IsolationCleanup = "failed"
			retErr = errors.Join(retErr, fmt.Errorf("clean isolated pack-memory kit: %w", err))
		} else {
			receipt.IsolationCleanup = "removed"
		}
		afterCleanup, hashErr := packMemoryAcceptanceTreeSHA256(sourceRepo)
		if hashErr != nil {
			retErr = errors.Join(retErr, hashErr)
		} else {
			receipt.SourceTreeSHA256AfterCleanup = afterCleanup
			receipt.SourceTreeSHA256After = afterCleanup
		}
		receipt.SourceRepoUnchanged = receipt.SourceTreeSHA256BeforeChild != "" &&
			receipt.SourceTreeSHA256AfterChild != "" &&
			receipt.SourceTreeSHA256AfterCleanup != "" &&
			strings.EqualFold(before, receipt.SourceTreeSHA256BeforeChild) &&
			strings.EqualFold(before, receipt.SourceTreeSHA256AfterChild) &&
			strings.EqualFold(before, receipt.SourceTreeSHA256AfterCleanup)
		if !receipt.SourceRepoUnchanged {
			receipt.Passed = false
			retErr = errors.Join(retErr, fmt.Errorf("source repository changed during isolated pack-memory live acceptance"))
		}
	}()
	copyManifest, copyManifestSHA256, err := copyPackMemoryAcceptanceRepo(sourceRepo, isolatedRoot)
	if err != nil {
		return receipt, err
	}
	receipt.CopyManifestSHA256 = copyManifestSHA256
	receipt.CopyManifestFiles = len(copyManifest.Files)
	hostPath, err := os.Executable()
	if err != nil {
		return receipt, err
	}
	hostPath, err = filepath.Abs(hostPath)
	if err != nil {
		return receipt, err
	}
	hostData, err := rekitfs.ReadStableRegularFileAnchored(filepath.Dir(hostPath), hostPath, "pack-memory live acceptance child host", 512<<20)
	if err != nil {
		return receipt, err
	}
	receipt.ChildHostPath = hostPath
	receipt.ChildHostSHA256 = packMemoryAcceptanceBytesSHA256(hostData)
	spec := PackMemoryLiveAcceptanceChildSpec{
		SchemaVersion: 1, Kind: packMemoryLiveAcceptanceChildKind,
		IsolatedKitRoot: isolatedRoot, Goal: goal,
		ClaudePath: claude.Path, ClaudeSHA256: claude.SHA256, ClaudePublisher: claude.Publisher,
		Model: strings.TrimSpace(opt.Model), Actor: actor,
		TimeoutNanos: int64(opt.Timeout), MaxAttempts: opt.MaxAttempts, KeepCase: opt.KeepCase,
		ChildHostPath: hostPath, ChildHostSHA256: receipt.ChildHostSHA256,
		InitializationSourceExecutable: strings.TrimSpace(opt.InitializationSourceExecutable),
		CopyManifestSHA256:             receipt.CopyManifestSHA256, CopyManifest: copyManifest,
	}
	specData, err := json.Marshal(spec)
	if err != nil {
		return receipt, err
	}
	specData = append(specData, '\n')
	specSHA256 := packMemoryAcceptanceBytesSHA256(specData)
	receipt.ChildSpecSHA256 = specSHA256
	specPath := filepath.Join(isolatedRoot, ".rh07-child-spec.json")
	if err := os.WriteFile(specPath, specData, 0o600); err != nil {
		return receipt, err
	}
	if err := bindLiveAcceptanceCaseMarker(&identity, filepath.Base(specPath), specData); err != nil {
		return receipt, err
	}
	beforeChild, err := packMemoryAcceptanceTreeSHA256(sourceRepo)
	if err != nil {
		return receipt, err
	}
	receipt.SourceTreeSHA256BeforeChild = beforeChild
	if !strings.EqualFold(before, beforeChild) {
		return receipt, fmt.Errorf("source repository changed before isolated pack-memory child launch")
	}
	result, err := runPackMemoryAcceptanceChild(parent, isolatedRoot, specPath, specSHA256, receipt.ChildHostPath, receipt.ChildHostSHA256)
	receipt.Child = result
	afterChild, hashErr := packMemoryAcceptanceTreeSHA256(sourceRepo)
	if hashErr != nil {
		return receipt, errors.Join(err, hashErr)
	}
	receipt.SourceTreeSHA256AfterChild = afterChild
	if !strings.EqualFold(before, afterChild) {
		return receipt, errors.Join(err, fmt.Errorf("source repository changed during isolated pack-memory child execution"))
	}
	if err != nil {
		return receipt, err
	}
	if !result.Passed || result.Kind != packMemoryLiveAcceptanceChildKind || result.ManualResultWrites != 0 {
		return receipt, fmt.Errorf("isolated pack-memory child did not satisfy the live acceptance contract")
	}
	receipt.Passed = true
	return receipt, nil
}

func RunPackMemoryLiveAcceptanceChild(parent context.Context, specPath, expectedSHA256 string) (PackMemoryLiveAcceptanceChildResult, error) {
	data, err := rekitfs.ReadStableRegularFileAnchored(filepath.Dir(specPath), specPath, "pack-memory live acceptance child spec", 1<<20)
	if err != nil {
		return PackMemoryLiveAcceptanceChildResult{}, err
	}
	if len(strings.TrimSpace(expectedSHA256)) != 64 || !strings.EqualFold(packMemoryAcceptanceBytesSHA256(data), strings.TrimSpace(expectedSHA256)) {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("pack-memory live acceptance child spec sha256 mismatch")
	}
	var spec PackMemoryLiveAcceptanceChildSpec
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		return PackMemoryLiveAcceptanceChildResult{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("pack-memory child spec must contain exactly one JSON object")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return PackMemoryLiveAcceptanceChildResult{}, err
	}
	root, err := filepath.Abs(spec.IsolatedKitRoot)
	if err != nil || !sameLiveAcceptancePath(cwd, root) || spec.SchemaVersion != 1 || spec.Kind != packMemoryLiveAcceptanceChildKind || strings.TrimSpace(spec.Goal) == "" || strings.TrimSpace(spec.ClaudePath) == "" || len(spec.ClaudeSHA256) != 64 || spec.ClaudePublisher != liveAcceptanceClaudePublisher || strings.TrimSpace(spec.ChildHostPath) == "" || len(spec.ChildHostSHA256) != 64 || len(spec.CopyManifestSHA256) != 64 || spec.CopyManifest.SchemaVersion != 1 || spec.CopyManifest.Kind != "rekit-pack-memory-live-acceptance-copy-manifest" || len(spec.CopyManifest.Files) == 0 {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("pack-memory live acceptance child spec is invalid or cwd-unbound")
	}
	copyManifestData, err := json.Marshal(spec.CopyManifest)
	if err != nil || !strings.EqualFold(packMemoryAcceptanceBytesSHA256(copyManifestData), spec.CopyManifestSHA256) {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("pack-memory live acceptance copy manifest sha256 mismatch: %w", err)
	}
	if err := verifyPackMemoryAcceptanceCopyManifest(root, spec.CopyManifest); err != nil {
		return PackMemoryLiveAcceptanceChildResult{}, err
	}
	hostPath, err := os.Executable()
	if err != nil {
		return PackMemoryLiveAcceptanceChildResult{}, err
	}
	hostData, err := rekitfs.ReadStableRegularFileAnchored(filepath.Dir(hostPath), hostPath, "pack-memory live acceptance running child host", 512<<20)
	if err != nil || !sameLiveAcceptancePath(hostPath, spec.ChildHostPath) || !strings.EqualFold(packMemoryAcceptanceBytesSHA256(hostData), spec.ChildHostSHA256) {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("pack-memory live acceptance running child host is not spec-bound: %w", err)
	}
	runtimeContext, err := rekitruntime.NewWithCwd("", liveAcceptancePack, root)
	if err != nil || !sameLiveAcceptancePath(runtimeContext.RepoRoot, root) {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("pack-memory live acceptance child repo discovery escaped the isolated kit: %w", err)
	}
	return runPackMemoryLiveAcceptanceLifecycle(parent, spec)
}

func runPackMemoryAcceptanceChild(parent context.Context, isolatedRoot, specPath, specSHA256, hostPath, hostSHA256 string) (PackMemoryLiveAcceptanceChildResult, error) {
	binding, err := processguard.LockExecutable(hostPath, 512<<20)
	if err != nil {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("lock pack-memory live acceptance child host: %w", err)
	}
	defer binding.Close()
	if !strings.EqualFold(binding.SHA256(), hostSHA256) {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("pack-memory live acceptance child host sha256 mismatch")
	}
	cmd := exec.CommandContext(parent, binding.Path(), "-internal-pack-memory-live-acceptance", specPath, "-internal-pack-memory-live-acceptance-sha256", specSHA256)
	cmd.Dir = isolatedRoot
	if err := processguard.ConfigureSuspended(cmd, binding); err != nil {
		return PackMemoryLiveAcceptanceChildResult{}, err
	}
	cmd.WaitDelay = time.Second
	var stdout, stderr limitedBuffer
	stdout.limit = maxClaudeStdoutBytes
	stderr.limit = maxDiagnosticBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("start suspended pack-memory live acceptance child: %w", err)
	}
	containment, err := processguard.ValidateContainAndResumeAllowBreakaway(cmd.Process, binding)
	if err != nil {
		_ = cmd.Process.Kill()
		return PackMemoryLiveAcceptanceChildResult{}, errors.Join(fmt.Errorf("validate suspended pack-memory live acceptance child: %w", err), cmd.Wait())
	}
	closeContainment := func() error {
		if containment == nil {
			return nil
		}
		err := containment.Close()
		containment = nil
		return err
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	var runErr error
	select {
	case runErr = <-waitDone:
	case <-parent.Done():
		closeErr := closeContainment()
		runErr = errors.Join(parent.Err(), <-waitDone, closeErr)
	}
	containmentErr := closeContainment()
	runErr = errors.Join(runErr, containmentErr)
	if stdout.exceeded {
		return PackMemoryLiveAcceptanceChildResult{}, fmt.Errorf("pack-memory live acceptance child output exceeded limit")
	}
	result, decodeErr := decodePackMemoryAcceptanceChildResult(stdout.String())
	if decodeErr != nil {
		if runErr != nil {
			return result, errors.Join(fmt.Errorf("pack-memory live acceptance child failed: %w: %s", runErr, strings.TrimSpace(stderr.String())), decodeErr)
		}
		return result, decodeErr
	}
	if runErr != nil {
		return result, fmt.Errorf("pack-memory live acceptance child failed: %w: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	return result, nil
}

func decodePackMemoryAcceptanceChildResult(data string) (PackMemoryLiveAcceptanceChildResult, error) {
	var result PackMemoryLiveAcceptanceChildResult
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return result, fmt.Errorf("decode pack-memory live acceptance child result: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return result, fmt.Errorf("pack-memory live acceptance child returned trailing output")
	}
	return result, nil
}

func copyPackMemoryAcceptanceRepo(source, target string) (packMemoryAcceptanceCopyManifest, string, error) {
	manifest := packMemoryAcceptanceCopyManifest{SchemaVersion: 1, Kind: "rekit-pack-memory-live-acceptance-copy-manifest"}
	source, err := filepath.Abs(source)
	if err != nil {
		return manifest, "", err
	}
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if skipPackMemoryAcceptancePath(relSlash, entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("source repository contains unsupported entry for isolated copy: %s", relSlash)
		}
		if err := rejectPackMemoryAcceptanceReparse(path); err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.Mkdir(dest, info.Mode().Perm())
		}
		data, err := rekitfs.ReadStableRegularFileAnchored(source, path, "isolated kit source", 64<<20)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, info.Mode().Perm()); err != nil {
			return err
		}
		copied, err := rekitfs.ReadStableRegularFileAnchored(target, dest, "isolated kit destination", 64<<20)
		if err != nil || !bytes.Equal(data, copied) {
			return fmt.Errorf("isolated kit destination did not preserve source bytes: %s: %w", relSlash, err)
		}
		manifest.Files = append(manifest.Files, packMemoryAcceptanceCopyManifestFile{
			Path: relSlash, SHA256: packMemoryAcceptanceBytesSHA256(data), Bytes: len(data),
		})
		return nil
	})
	if err != nil {
		return manifest, "", err
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return manifest, "", err
	}
	return manifest, packMemoryAcceptanceBytesSHA256(manifestData), nil
}

func verifyPackMemoryAcceptanceCopyManifest(root string, manifest packMemoryAcceptanceCopyManifest) error {
	seen := map[string]bool{}
	for _, file := range manifest.Files {
		rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(file.Path))))
		if rel == "." || filepath.IsAbs(filepath.FromSlash(rel)) || strings.HasPrefix(rel, "../") || seen[rel] || !packMemoryAcceptanceCopyAllowed(rel) || len(file.SHA256) != 64 || file.Bytes < 0 {
			return fmt.Errorf("pack-memory live acceptance copy manifest contains invalid file: %s", file.Path)
		}
		seen[rel] = true
		path := filepath.Join(root, filepath.FromSlash(rel))
		data, err := rekitfs.ReadStableRegularFileAnchored(root, path, "isolated kit copied file", 64<<20)
		if err != nil || len(data) != file.Bytes || !strings.EqualFold(packMemoryAcceptanceBytesSHA256(data), file.SHA256) {
			return fmt.Errorf("isolated kit copied file drifted from manifest: %s: %w", rel, err)
		}
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := rejectPackMemoryAcceptanceReparse(path); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("isolated kit contains unsupported entry outside copy manifest: %s", rel)
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("isolated kit contains unsupported entry outside copy manifest: %s: %w", rel, err)
		}
		if !seen[rel] && rel != ".rh07-child-spec.json" {
			return fmt.Errorf("isolated kit contains file outside copy manifest: %s", rel)
		}
		return nil
	})
}

func skipPackMemoryAcceptancePath(rel string, isDir bool) bool {
	if !packMemoryAcceptanceCopyAllowed(rel) {
		if isDir && packMemoryAcceptanceCopyPrefix(rel) {
			return false
		}
		return true
	}
	packRoot := "packs/" + liveAcceptancePack + "/"
	if strings.HasPrefix(rel, packRoot+"promote-candidates/") || strings.HasPrefix(rel, packRoot+"tooling/candidates/") {
		return true
	}
	return false
}

func packMemoryAcceptanceCopyAllowed(rel string) bool {
	rel = filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	return rel == "go.mod" || rel == "rekit/tests/catalog.json" || rel == "rekit/templates/case-shim/SKILL.md" ||
		rel == ".claude/skills/rekit/SKILL.md" || strings.HasPrefix(rel, "packs/"+liveAcceptancePack+"/") || strings.HasPrefix(rel, "common/")
}

func packMemoryAcceptanceCopyPrefix(rel string) bool {
	rel = filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	return slices.Contains([]string{"packs", "packs/" + liveAcceptancePack, "common", "rekit", "rekit/tests", "rekit/templates", "rekit/templates/case-shim", ".claude", ".claude/skills", ".claude/skills/rekit"}, rel)
}

func packMemoryAcceptanceTreeSHA256(root string) (string, error) {
	return liveAcceptanceTreeSHA256(root)
}

func packMemoryAcceptanceBytesSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func currentRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for current := cwd; ; current = filepath.Dir(current) {
		if info, err := os.Stat(filepath.Join(current, "go.mod")); err == nil && info.Mode().IsRegular() {
			if info, err := os.Stat(filepath.Join(current, "packs", liveAcceptancePack, "manifest.yml")); err == nil && info.Mode().IsRegular() {
				return current, nil
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("unable to locate source repository root from %s", cwd)
		}
	}
}

func sameLiveAcceptancePath(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func WritePackMemoryLiveAcceptanceReceipt(path string, receipt PackMemoryLiveAcceptanceReceipt) error {
	receipt.Claude.Path = ""
	path, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return err
	}
	anchorPath := filepath.VolumeName(path) + string(filepath.Separator)
	if anchorPath == "" {
		anchorPath = string(filepath.Separator)
	}
	rel, err := filepath.Rel(anchorPath, path)
	if err != nil || filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("pack-memory live acceptance receipt path escapes its volume root: %s", path)
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := rekitfs.WriteNewExclusiveRegularFileAnchored(anchorPath, filepath.ToSlash(rel), "pack-memory live acceptance receipt", data); err != nil {
		return fmt.Errorf("publish pack-memory live acceptance receipt %s: %w", path, err)
	}
	return nil
}
