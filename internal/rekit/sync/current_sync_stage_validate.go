package sync

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

func currentSyncStagedControlledRoot(
	caseRoot string,
	intent currentSyncIntent,
) (string, error) {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return "", err
	}
	caseFull, err := filepath.Abs(caseRoot)
	if err != nil || !sameCurrentSyncPath(caseFull, intent.Plan.CaseRoot) {
		return "", fmt.Errorf(
			"current sync staged bundle case root is invalid",
		)
	}
	return filepath.Join(
		caseFull,
		projectstate.CurrentDir,
		filepath.FromSlash(intent.TransactionPath),
		"stage",
		"controlled",
	), nil
}

func validateCurrentSyncStagedBundle(
	caseRoot string,
	intent currentSyncIntent,
) error {
	stageRoot, err := currentSyncStagedControlledRoot(caseRoot, intent)
	if err != nil {
		return err
	}
	manifestRel := strings.TrimPrefix(
		intent.Plan.NextManifest.Path,
		projectstate.CurrentDir+"/",
	)
	if manifestRel != runtimebundle.ManifestRel {
		return fmt.Errorf(
			"current sync staged bundle manifest path is invalid: %s",
			manifestRel,
		)
	}
	if _, err := runtimebundle.Validate(
		stageRoot,
		manifestRel,
		intent.Plan.NextManifest.SHA256,
		intent.Plan.Pack,
	); err != nil {
		return fmt.Errorf("current sync staged bundle is invalid: %w", err)
	}
	return nil
}
