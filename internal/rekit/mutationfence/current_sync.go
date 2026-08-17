package mutationfence

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	currentSyncOwnerRel  = "maintenance/current-sync-v1/owner.json"
	currentSyncIntentRel = "maintenance/current-sync-v1/intent.json"
)

// RefusePendingCurrentSync prevents ordinary mutation owners from writing while
// an interrupted current-sync transaction owns the project. Callers must hold
// the project mutation lock before checking this fence.
func RefusePendingCurrentSync(caseRoot, operation string) error {
	stateRoot, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return err
	}
	if !stateRoot.Existing || stateRoot.Legacy || stateRoot.Dir != projectstate.CurrentDir {
		return nil
	}
	pending := false
	for _, rel := range []string{currentSyncOwnerRel, currentSyncIntentRel} {
		path := filepath.Join(stateRoot.Path, filepath.FromSlash(rel))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("%s refuses project with invalid pending current sync fence: %s", operation, path)
		}
		pending = true
	}
	if !pending {
		return nil
	}
	return fmt.Errorf("%s refuses project while current sync recovery is pending; resume the exact reviewed refresh before any other mutation", operation)
}
