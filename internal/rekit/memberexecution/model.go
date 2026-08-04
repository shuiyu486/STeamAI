package memberexecution

const (
	SchemaVersion   = 1
	KindIntent      = "member-lane-execution-intent"
	KindHandoff     = "member-lane-execution-handoff"
	KindCommit      = "member-lane-execution-commit"
	KindManifest    = "member-lane-execution-result-manifest"
	KindObservation = "member-lane-execution-observation"
)

type Owner struct {
	Lane               string `json:"lane"`
	Executor           string `json:"executor"`
	ExecutorGeneration int    `json:"executorGeneration"`
}

type Intent struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	AttemptID     string `json:"attemptId"`
	CaseRoot      string `json:"caseRoot"`
	Pack          string `json:"pack"`
	Owner         Owner  `json:"owner"`
	RequestSHA256 string `json:"requestSha256"`
	CreatedAt     string `json:"createdAt"`
	NoSpawn       bool   `json:"noSpawn"`
	NoPoll        bool   `json:"noPoll"`
	NoStop        bool   `json:"noStop"`
	NoHeavyTool   bool   `json:"noHeavyTool"`
	NoAuthority   bool   `json:"noAuthority"`
	NoConfirmed   bool   `json:"noConfirmed"`
}

type Handoff struct {
	SchemaVersion int      `json:"schemaVersion"`
	Kind          string   `json:"kind"`
	AttemptID     string   `json:"attemptId"`
	Owner         Owner    `json:"owner"`
	IntentSHA256  string   `json:"intentSha256"`
	ManifestPath  string   `json:"manifestPath"`
	OutputsRoot   string   `json:"outputsRoot"`
	NextSteps     []string `json:"nextSteps"`
	Boundary      []string `json:"boundary"`
}

type Commit struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	AttemptID     string `json:"attemptId"`
	IntentSHA256  string `json:"intentSha256"`
	HandoffSHA256 string `json:"handoffSha256"`
}

type Output struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ResultManifest struct {
	SchemaVersion     int      `json:"schemaVersion"`
	Kind              string   `json:"kind"`
	AttemptID         string   `json:"attemptId"`
	Owner             Owner    `json:"owner"`
	Summary           string   `json:"summary"`
	Outputs           []Output `json:"outputs"`
	ReviewerItemsPath string   `json:"reviewerItemsPath,omitempty"`
	NoAuthority       bool     `json:"noAuthority"`
	NoConfirmed       bool     `json:"noConfirmed"`
	NoHeavyTool       bool     `json:"noHeavyTool"`
}

type Observation struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Kind           string `json:"kind"`
	Sequence       int    `json:"sequence"`
	AttemptID      string `json:"attemptId"`
	Owner          Owner  `json:"owner"`
	Outcome        string `json:"outcome"`
	Actor          string `json:"actor"`
	Reason         string `json:"reason,omitempty"`
	ObservedAt     string `json:"observedAt"`
	ManifestSHA256 string `json:"manifestSha256,omitempty"`
	IntentSHA256   string `json:"intentSha256"`
	NoAuthority    bool   `json:"noAuthority"`
	NoConfirmed    bool   `json:"noConfirmed"`
	NoHeavyTool    bool   `json:"noHeavyTool"`
}

type Inspection struct {
	State          string          `json:"state"`
	AttemptID      string          `json:"attemptId,omitempty"`
	Owner          Owner           `json:"owner"`
	Intent         *Intent         `json:"intent,omitempty"`
	Handoff        *Handoff        `json:"handoff,omitempty"`
	Latest         *Observation    `json:"latestObservation,omitempty"`
	Manifest       *ResultManifest `json:"manifest,omitempty"`
	ManifestSHA256 string          `json:"manifestSha256,omitempty"`
	AttemptRoot    string          `json:"attemptRoot,omitempty"`
	ManifestPath   string          `json:"manifestPath,omitempty"`
	OutputsRoot    string          `json:"outputsRoot,omitempty"`
}

type Plan struct {
	SchemaVersion        int        `json:"schemaVersion"`
	Mode                 string     `json:"mode"`
	CaseRoot             string     `json:"caseRoot"`
	Pack                 string     `json:"pack"`
	AttemptID            string     `json:"attemptId"`
	Owner                Owner      `json:"owner"`
	Outcome              string     `json:"outcome,omitempty"`
	Actor                string     `json:"actor,omitempty"`
	Reason               string     `json:"reason,omitempty"`
	ExpectedPlanSHA256   string     `json:"expectedPlanSha256"`
	ExternalHandoff      *Handoff   `json:"externalHandoff,omitempty"`
	Inspection           Inspection `json:"inspection"`
	ReviewRequired       bool       `json:"reviewRequired"`
	RequiresConfirmation bool       `json:"requiresConfirmation"`
	Boundary             []string   `json:"boundary"`
	writes               []plannedWrite
}

type Result struct {
	Plan
	Applied        bool `json:"applied"`
	AlreadyApplied bool `json:"alreadyApplied"`
}

type DispatchOptions struct {
	CaseRoot      string
	Pack          string
	Lane          string
	RequestSHA256 string
	CreatedAt     string
}

type ObservationOptions struct {
	CaseRoot       string
	Pack           string
	Lane           string
	AttemptID      string
	Outcome        string
	Actor          string
	Reason         string
	ObservedAt     string
	ResultSnapshot *ResultSnapshot
}

type ResultSnapshot struct {
	ManifestPath string
	ManifestData []byte
	OutputsRoot  string
	Outputs      map[string][]byte
}

type plannedWrite struct {
	path string
	data []byte
}
