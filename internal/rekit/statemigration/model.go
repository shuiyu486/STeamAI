package statemigration

const (
	SchemaVersion = 1
	Command       = "migrate-state"
	PlanKind      = "state-root-migration-plan"
	ReceiptKind   = "state-root-migration-receipt"
	ResultKind    = "state-root-migration-result"
	ReceiptRel    = ".steamai/migration/state-root-v1.json"
)

type FileBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type InventoryEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256,omitempty"`
	Size   int64  `json:"size,omitempty"`
}

type Inventory struct {
	StateRoot string           `json:"stateRoot"`
	SHA256    string           `json:"sha256"`
	Files     int              `json:"files"`
	Bytes     int64            `json:"bytes"`
	Entries   []InventoryEntry `json:"entries,omitempty"`
}

type Write struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	SHA256     string `json:"sha256,omitempty"`
	Size       int64  `json:"size,omitempty"`
	SourcePath string `json:"sourcePath,omitempty"`
}

type Plan struct {
	SchemaVersion        int          `json:"schemaVersion"`
	Kind                 string       `json:"kind"`
	Command              string       `json:"command"`
	Status               string       `json:"status"`
	CaseRoot             string       `json:"caseRoot"`
	RepoRoot             string       `json:"repoRoot,omitempty"`
	Pack                 string       `json:"pack"`
	ProjectName          string       `json:"projectName,omitempty"`
	SourceStateRoot      string       `json:"sourceStateRoot,omitempty"`
	TargetStateRoot      string       `json:"targetStateRoot"`
	CaseRootIdentity     Identity     `json:"caseRootIdentity,omitempty"`
	SourceRootIdentity   Identity     `json:"sourceRootIdentity,omitempty"`
	LegacyInventory      Inventory    `json:"legacyInventory,omitempty"`
	PlannedInventory     Inventory    `json:"plannedInventory,omitempty"`
	LegacyInstance       FileBinding  `json:"legacyInstance,omitempty"`
	LegacyState          FileBinding  `json:"legacyState,omitempty"`
	LegacyMetadata       *FileBinding `json:"legacyMetadata,omitempty"`
	LegacySkill          FileBinding  `json:"legacySkill,omitempty"`
	CurrentInstance      FileBinding  `json:"currentInstance,omitempty"`
	CurrentState         FileBinding  `json:"currentState,omitempty"`
	CurrentSkill         FileBinding  `json:"currentSkill,omitempty"`
	BundleManifest       FileBinding  `json:"bundleManifest,omitempty"`
	Writes               []Write      `json:"writes,omitempty"`
	ExpectedPlanSHA256   string       `json:"expectedPlanSha256,omitempty"`
	ApplyArgs            []string     `json:"applyArgs,omitempty"`
	IsMutation           bool         `json:"isMutation"`
	Applied              bool         `json:"applied"`
	Replay               bool         `json:"replay"`
	AlreadyCurrent       bool         `json:"alreadyCurrent"`
	RequiresReview       bool         `json:"requiresReview"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	BlockedActions       []string     `json:"blockedActions"`
	NextSteps            []string     `json:"nextSteps"`

	prepared *preparedPlan
}

type ReceiptState struct {
	StateRoot    string   `json:"stateRoot"`
	RootIdentity Identity `json:"rootIdentity"`
	InventorySHA string   `json:"inventorySha256"`
	Files        int      `json:"files"`
	Bytes        int64    `json:"bytes"`
}

type Receipt struct {
	SchemaVersion  int          `json:"schemaVersion"`
	Kind           string       `json:"kind"`
	Command        string       `json:"command"`
	State          string       `json:"state"`
	PlanSHA256     string       `json:"planSha256"`
	Pack           string       `json:"pack"`
	Before         ReceiptState `json:"before"`
	After          ReceiptState `json:"after"`
	Instance       FileBinding  `json:"instance"`
	StateMetadata  FileBinding  `json:"stateMetadata"`
	Skill          FileBinding  `json:"skill"`
	BundleManifest FileBinding  `json:"bundleManifest"`
	LegacyInstance FileBinding  `json:"legacyInstance"`
	LegacyState    FileBinding  `json:"legacyState"`
	LegacyMetadata *FileBinding `json:"legacyMetadata,omitempty"`
	LegacySkill    FileBinding  `json:"legacySkill"`
	NoAuthority    bool         `json:"noAuthority"`
	NoConfirmed    bool         `json:"noConfirmed"`
	NoHeavyTool    bool         `json:"noHeavyTool"`
	NoSyncPromote  bool         `json:"noSyncOrPromote"`
}

type Result struct {
	SchemaVersion  int      `json:"schemaVersion"`
	Kind           string   `json:"kind"`
	Command        string   `json:"command"`
	Status         string   `json:"status"`
	CaseRoot       string   `json:"caseRoot"`
	Pack           string   `json:"pack"`
	IsMutation     bool     `json:"isMutation"`
	Applied        bool     `json:"applied"`
	Replay         bool     `json:"replay"`
	AlreadyCurrent bool     `json:"alreadyCurrent"`
	PlanSHA256     string   `json:"planSha256,omitempty"`
	ReceiptPath    string   `json:"receiptPath,omitempty"`
	Receipt        *Receipt `json:"receipt,omitempty"`
	Writes         []Write  `json:"writes,omitempty"`
	NextSteps      []string `json:"nextSteps"`
}
