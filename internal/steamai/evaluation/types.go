package evaluation

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

const maxRequestBytes = 1 << 20

var (
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)
	shaPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	modelPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	ErrInvalid    = errors.New("evaluation request 无效")
	ErrNotAllowed = errors.New("evaluation runner 只允许 synthetic readonly scenario")
)

type Request struct {
	RunID                       string  `json:"runId"`
	Purpose                     string  `json:"purpose"`
	Scenario                    string  `json:"scenario"`
	ScenarioSHA256              string  `json:"scenarioSha256"`
	Rubric                      string  `json:"rubric"`
	RubricSHA256                string  `json:"rubricSha256"`
	VerifiedLearningContract    string  `json:"verifiedLearningContract"`
	VerifiedLearningContractSHA string  `json:"verifiedLearningContractSha256"`
	BaselineSHA256              string  `json:"baselineSha256"`
	CandidatePatch              string  `json:"candidatePatch"`
	PatchSHA256                 string  `json:"candidatePatchSha256"`
	Model                       string  `json:"model"`
	SlotID                      string  `json:"slotId,omitempty"`
	ExpectedClass               string  `json:"expectedClass,omitempty"`
	SuiteSpec                   string  `json:"suiteSpec,omitempty"`
	SuiteSpecSHA256             string  `json:"suiteSpecSha256,omitempty"`
	MaxSeconds                  int     `json:"maxSeconds"`
	MaxBudgetUSD                float64 `json:"maxBudgetUsd"`
}

type RunRecord struct {
	SchemaVersion            int             `json:"schemaVersion"`
	RunID                    string          `json:"runId"`
	Purpose                  string          `json:"purpose"`
	SlotID                   string          `json:"slotId,omitempty"`
	ExpectedClass            string          `json:"expectedClass,omitempty"`
	SuiteSpec                BoundFile       `json:"suiteSpec"`
	Scenario                 BoundFile       `json:"scenario"`
	Rubric                   BoundFile       `json:"rubric"`
	VerifiedLearningContract BoundFile       `json:"verifiedLearningContract"`
	ArmLabel                 string          `json:"armLabel"`
	PackCommitment           string          `json:"packCommitment"`
	Runtime                  RuntimeIdentity `json:"runtime"`
	Budget                   BudgetRecord    `json:"budget"`
	Result                   ResultRecord    `json:"result"`
}

type BundleManifest struct {
	SchemaVersion            int           `json:"schemaVersion"`
	RunID                    string        `json:"runId"`
	Purpose                  string        `json:"purpose"`
	SlotID                   string        `json:"slotId,omitempty"`
	ExpectedClass            string        `json:"expectedClass,omitempty"`
	SuiteSpec                BoundFile     `json:"suiteSpec"`
	Scenario                 BoundFile     `json:"scenario"`
	Rubric                   BoundFile     `json:"rubric"`
	VerifiedLearningContract BoundFile     `json:"verifiedLearningContract"`
	Arms                     []ArmRecord   `json:"arms"`
	RevealSHA256             string        `json:"revealSha256"`
	Identity                 string        `json:"identity"`
	Reveal                   *RevealRecord `json:"-"`
}

type BoundFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type RevealRecord struct {
	SchemaVersion        int    `json:"schemaVersion"`
	RunID                string `json:"runId"`
	BlindIdentity        string `json:"blindIdentity"`
	CommitmentNonce      string `json:"commitmentNonce"`
	BaselineArm          string `json:"baselineArm"`
	BaselinePackSHA256   string `json:"baselinePackSha256"`
	CandidateArm         string `json:"candidateArm"`
	CandidatePackSHA256  string `json:"candidatePackSha256"`
	CandidatePatchSHA256 string `json:"candidatePatchSha256"`
}

type RuntimeIdentity struct {
	Model       string `json:"model"`
	ClaudeCode  string `json:"claudeCode"`
	OS          string `json:"os"`
	ToolProfile string `json:"toolProfile"`
}

type BudgetRecord struct {
	MaxSeconds   int      `json:"maxSeconds"`
	MaxBudgetUSD float64  `json:"maxBudgetUsd"`
	ActualMillis int64    `json:"actualMillis"`
	ActualUSD    *float64 `json:"actualUsd,omitempty"`
}

type ResultRecord struct {
	Status       string `json:"status"`
	ExitCode     int    `json:"exitCode"`
	OutputSHA256 string `json:"outputSha256"`
	OutputBytes  int    `json:"outputBytes"`
	StderrSHA256 string `json:"stderrSha256"`
	StderrBytes  int    `json:"stderrBytes"`
	SafetyGate   string `json:"safetyGate"`
	Error        string `json:"error,omitempty"`
}

type ArmRecord struct {
	Label        string `json:"label"`
	Record       string `json:"record"`
	RecordSHA256 string `json:"recordSha256"`
	Output       string `json:"output"`
	OutputSHA256 string `json:"outputSha256"`
	Stderr       string `json:"stderr"`
	StderrSHA256 string `json:"stderrSha256"`
}

func DecodeRequest(reader io.Reader) (Request, error) {
	decoder := json.NewDecoder(io.LimitReader(reader, maxRequestBytes))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		return Request{}, fmt.Errorf("解析 evaluation request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Request{}, errors.New("evaluation request 后存在额外 JSON")
	}
	if err := request.Validate(); err != nil {
		return Request{}, err
	}
	return request, nil
}

func validRelativeName(value string) bool {
	converted := filepath.FromSlash(value)
	return value != "" && !strings.Contains(value, "\\") && !filepath.IsAbs(converted) && filepath.VolumeName(converted) == ""
}

func (request Request) Validate() error {
	if !idPattern.MatchString(request.RunID) || (request.Purpose != "calibration" && request.Purpose != "candidate") ||
		request.Scenario == "" || request.Rubric == "" || !validRelativeName(request.Scenario) || !validRelativeName(request.Rubric) ||
		request.VerifiedLearningContract != "verified-learning.md" || !shaPattern.MatchString(request.VerifiedLearningContractSHA) ||
		!validRelativeName(request.CandidatePatch) || !shaPattern.MatchString(request.ScenarioSHA256) ||
		!shaPattern.MatchString(request.RubricSHA256) || !shaPattern.MatchString(request.BaselineSHA256) || !modelPattern.MatchString(request.Model) ||
		request.MaxSeconds < 30 || request.MaxSeconds > 3600 ||
		request.MaxBudgetUSD <= 0 || request.MaxBudgetUSD > 100 {
		return ErrInvalid
	}
	if request.CandidatePatch == "" || !shaPattern.MatchString(request.PatchSHA256) {
		return ErrInvalid
	}
	if request.Purpose == "calibration" {
		if !idPattern.MatchString(request.SlotID) || !validControlClass(request.ExpectedClass) ||
			!validRelativeName(request.SuiteSpec) || !shaPattern.MatchString(request.SuiteSpecSHA256) {
			return ErrInvalid
		}
	} else if request.SlotID != "" || request.ExpectedClass != "" || request.SuiteSpec != "" || request.SuiteSpecSHA256 != "" {
		return ErrInvalid
	}
	return nil
}

func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func BlindBundleIdentity(bundle BundleManifest) string {
	copy := bundle
	copy.Identity = ""
	copy.RevealSHA256 = ""
	copy.Reveal = nil
	copy.Arms = append([]ArmRecord(nil), copy.Arms...)
	sort.Slice(copy.Arms, func(i, j int) bool { return copy.Arms[i].Label < copy.Arms[j].Label })
	data, _ := json.Marshal(copy)
	return Hash(append(data, '\n'))
}

func BundleIdentity(bundle BundleManifest) string {
	copy := bundle
	copy.Identity = ""
	copy.Reveal = nil
	copy.Arms = append([]ArmRecord(nil), copy.Arms...)
	sort.Slice(copy.Arms, func(i, j int) bool { return copy.Arms[i].Label < copy.Arms[j].Label })
	data, _ := json.Marshal(copy)
	return Hash(append(data, '\n'))
}

func ToolProfile() string {
	return strings.Join([]string{"Read"}, ",")
}
