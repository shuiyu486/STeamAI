package laneowner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

var laneIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Snapshot struct {
	Lane               string `json:"lane"`
	CurrentExecutor    string `json:"currentExecutor"`
	ExecutorGeneration int    `json:"executorGeneration"`
}

type laneFile struct {
	SchemaVersion              int            `json:"schemaVersion"`
	ID                         string         `json:"id"`
	Type                       string         `json:"type"`
	Name                       string         `json:"name"`
	Title                      string         `json:"title"`
	Status                     string         `json:"status"`
	Authority                  bool           `json:"authority"`
	Workspace                  string         `json:"workspace"`
	LaneRoot                   string         `json:"laneRoot"`
	CanWrite                   []string       `json:"canWrite"`
	ReadOnly                   []string       `json:"readOnly"`
	Outputs                    []string       `json:"outputs"`
	Counters                   map[string]int `json:"counters"`
	CurrentExecutor            string         `json:"currentExecutor,omitempty"`
	ExecutorGeneration         int            `json:"executorGeneration,omitempty"`
	LastTakeoverAt             string         `json:"lastTakeoverAt,omitempty"`
	LastTakeoverBy             string         `json:"lastTakeoverBy,omitempty"`
	LastTakeoverReason         string         `json:"lastTakeoverReason,omitempty"`
	LastReconciledIntervention string         `json:"lastReconciledIntervention,omitempty"`
	LastReconcileAt            string         `json:"lastReconcileAt,omitempty"`
	CreatedAt                  string         `json:"createdAt"`
	UpdatedAt                  string         `json:"updatedAt"`
}

func Path(caseRoot, laneID string) (string, string, error) {
	laneID = strings.TrimSpace(laneID)
	if !laneIDPattern.MatchString(laneID) || laneID == "." || laneID == ".." {
		return "", "", fmt.Errorf("invalid lane id path segment: %s", laneID)
	}
	rel, err := projectstate.Rel(caseRoot, "lanes", laneID, "lane.json")
	if err != nil {
		return "", "", err
	}
	path, err := refsf.SafeJoin(caseRoot, rel)
	if err != nil {
		return "", "", err
	}
	return rel, path, nil
}

func Read(caseRoot, laneID string) (Snapshot, error) {
	owner, found, err := ReadOptional(caseRoot, laneID)
	if err != nil {
		return Snapshot{}, err
	}
	if !found {
		return Snapshot{}, fmt.Errorf("lane %s has no current durable executor owner", laneID)
	}
	return owner, nil
}

func ReadOptional(caseRoot, laneID string) (Snapshot, bool, error) {
	_, path, err := Path(caseRoot, laneID)
	if err != nil {
		return Snapshot{}, false, err
	}
	data, err := readStableRegularFile(path, 1<<20)
	if os.IsNotExist(err) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("read durable lane owner %s: %w", laneID, err)
	}
	var lane laneFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&lane); err != nil {
		return Snapshot{}, false, fmt.Errorf("invalid lane json %s: %w", path, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return Snapshot{}, false, fmt.Errorf("invalid lane json %s: trailing data", path)
	}
	if !strings.EqualFold(strings.TrimSpace(lane.ID), strings.TrimSpace(laneID)) {
		return Snapshot{}, false, fmt.Errorf("lane id mismatch for %s: lane.json declares %s", laneID, lane.ID)
	}
	owner := Snapshot{Lane: lane.ID, CurrentExecutor: strings.TrimSpace(lane.CurrentExecutor), ExecutorGeneration: lane.ExecutorGeneration}
	if owner.CurrentExecutor == "" && owner.ExecutorGeneration == 0 {
		return Snapshot{}, false, nil
	}
	if owner.CurrentExecutor == "" || owner.ExecutorGeneration <= 0 {
		return Snapshot{}, false, fmt.Errorf("lane %s has incomplete durable executor owner", laneID)
	}
	return owner, true, nil
}

func Exists(caseRoot, laneID string) (bool, error) {
	_, path, err := Path(caseRoot, laneID)
	if err != nil {
		return false, err
	}
	_, err = os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func readStableRegularFile(path string, limit int64) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
		return nil, fmt.Errorf("path must be a regular non-symlink file: %s", path)
	}
	if st.Size() > limit {
		return nil, fmt.Errorf("file is too large: %s %d > %d", path, st.Size(), limit)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return nil, fmt.Errorf("file changed or is not regular: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file is too large: %s", path)
	}
	post, err := os.Lstat(path)
	if err != nil || post.Mode()&os.ModeSymlink != 0 || !post.Mode().IsRegular() || !os.SameFile(opened, post) {
		return nil, fmt.Errorf("file path changed after open: %s", path)
	}
	return data, nil
}
