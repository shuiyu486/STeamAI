package packmemoryconsumption

import (
	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/releasecheck"
)

const (
	SchemaVersion = 1
	KindDiscovery = "pack-memory-consumption-discovery"
	KindPlan      = "pack-memory-consumption-plan"
	KindReceipt   = "pack-memory-consumption-receipt"
)

type Discovery struct {
	SchemaVersion int                                           `json:"schemaVersion"`
	Kind          string                                        `json:"kind"`
	RepoRoot      string                                        `json:"repoRoot"`
	CaseRoot      string                                        `json:"caseRoot"`
	Pack          string                                        `json:"pack"`
	Available     []ChangeStatus                                `json:"available,omitempty"`
	Consumed      []ChangeStatus                                `json:"consumed,omitempty"`
	Conflicts     []ChangeStatus                                `json:"conflicts,omitempty"`
	Catalog       releasecheck.CompletedPackMemoryChangeCatalog `json:"catalog"`
	NextSteps     []string                                      `json:"nextSteps"`
	Boundary      []string                                      `json:"boundary"`
}

type ChangeStatus struct {
	ChangeID         string   `json:"changeId"`
	ManagedPath      string   `json:"managedPath"`
	SourceSHA256     string   `json:"sourceSha256"`
	TargetSHA256     string   `json:"targetSha256,omitempty"`
	TargetHashAtSync string   `json:"targetHashAtSync,omitempty"`
	State            string   `json:"state"`
	ReceiptPath      string   `json:"receiptPath,omitempty"`
	PreviewCommand   string   `json:"previewCommand,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
}

type Plan struct {
	SchemaVersion      int                                    `json:"schemaVersion"`
	Kind               string                                 `json:"kind"`
	Command            string                                 `json:"command"`
	RepoRoot           string                                 `json:"repoRoot"`
	CaseRoot           string                                 `json:"caseRoot"`
	Pack               string                                 `json:"pack"`
	ChangeID           string                                 `json:"changeId"`
	ManagedPath        string                                 `json:"managedPath"`
	SourcePath         string                                 `json:"sourcePath"`
	SourceSHA256       string                                 `json:"sourceSha256"`
	TargetPath         string                                 `json:"targetPath"`
	TargetSHA256Before string                                 `json:"targetSha256Before,omitempty"`
	StatePath          string                                 `json:"statePath"`
	StateSHA256Before  string                                 `json:"stateSha256Before,omitempty"`
	ReceiptPath        string                                 `json:"receiptPath"`
	BackupPath         string                                 `json:"backupPath,omitempty"`
	Action             string                                 `json:"action"`
	Authority          releasecheck.CompletedPackMemoryChange `json:"authority"`
	ExpectedPlanSHA256 string                                 `json:"expectedPlanSha256"`
	ApplyCommand       string                                 `json:"applyCommand"`
	IsMutation         bool                                   `json:"isMutation"`
	Applied            bool                                   `json:"applied"`
	Replay             bool                                   `json:"replay,omitempty"`
	RequiresReview     bool                                   `json:"requiresReview"`
	NextSteps          []string                               `json:"nextSteps"`
	Boundary           []string                               `json:"boundary"`
}

type CaseRootIdentity struct {
	Scheme       string `json:"scheme"`
	Device       uint64 `json:"device,omitempty"`
	Inode        uint64 `json:"inode,omitempty"`
	VolumeSerial uint32 `json:"volumeSerial,omitempty"`
	FileIndex    uint64 `json:"fileIndex,omitempty"`
}

type Receipt struct {
	SchemaVersion      int                                    `json:"schemaVersion"`
	Kind               string                                 `json:"kind"`
	RepoRoot           string                                 `json:"repoRoot"`
	CaseRoot           string                                 `json:"caseRoot"`
	CaseRootIdentity   CaseRootIdentity                       `json:"caseRootIdentity"`
	Pack               string                                 `json:"pack"`
	ChangeID           string                                 `json:"changeId"`
	ManagedPath        string                                 `json:"managedPath"`
	SourceSHA256       string                                 `json:"sourceSha256"`
	TargetSHA256Before string                                 `json:"targetSha256Before,omitempty"`
	TargetSHA256After  string                                 `json:"targetSha256After"`
	StateSHA256Before  string                                 `json:"stateSha256Before,omitempty"`
	StateSHA256After   string                                 `json:"stateSha256After"`
	BackupPath         string                                 `json:"backupPath,omitempty"`
	PlanSHA256         string                                 `json:"planSha256"`
	Authority          releasecheck.CompletedPackMemoryChange `json:"authority"`
	DoctorRows         int                                    `json:"doctorRows"`
	NoAuthority        bool                                   `json:"noAuthority"`
	NoConfirmed        bool                                   `json:"noConfirmed"`
	NoHeavyTool        bool                                   `json:"noHeavyTool"`
	Boundary           []string                               `json:"boundary"`
}

type Result struct {
	Plan       Plan         `json:"plan"`
	Receipt    Receipt      `json:"receipt"`
	DoctorRows []doctor.Row `json:"doctorRows,omitempty"`
	Discovery  Discovery    `json:"discovery"`
}
