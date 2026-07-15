package workstream

import (
	"os"
	"path/filepath"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func wouldFactKinds(kinds ...string) []StartWrite {
	writes := []StartWrite{}
	for _, kind := range kinds {
		writes = append(writes, StartWrite{Path: mission.FactRelPath(kind), Kind: "fact-jsonl", Action: "would-append"})
	}
	return writes
}

func (ctx continueContext) appendContinueFact(writes *[]StartWrite, kind string, value map[string]any) error {
	rel := mission.FactRelPath(kind)
	path, err := refsf.SafeJoin(ctx.inst.CaseRoot, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := mission.AppendJSONLine(path, value); err != nil {
		return err
	}
	*writes = append(*writes, StartWrite{Path: rel, Kind: "fact-jsonl", Action: "append", TargetPath: path})
	return nil
}
