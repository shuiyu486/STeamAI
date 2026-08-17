package workstream

import "github.com/shuiyu486/re-context-kits/internal/rekit/mission"

func wouldFactKinds(caseRoot string, kinds ...string) ([]StartWrite, error) {
	writes := []StartWrite{}
	for _, kind := range kinds {
		rel, err := mission.FactRelPathFor(caseRoot, kind)
		if err != nil {
			return nil, err
		}
		writes = append(writes, StartWrite{Path: rel, Kind: "fact-jsonl", Action: "would-append"})
	}
	return writes, nil
}

func (ctx continueContext) appendContinueFact(writes *[]StartWrite, kind string, value map[string]any) error {
	rel, path, err := mission.AppendFact(ctx.inst.CaseRoot, kind, value)
	if err != nil {
		return err
	}
	*writes = append(*writes, StartWrite{Path: rel, Kind: "fact-jsonl", Action: "append", TargetPath: path})
	return nil
}
