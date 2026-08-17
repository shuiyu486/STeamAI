package sync

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
)

type currentSyncPendingOwnership struct {
	Exists bool
	Intent currentSyncIntent
}

type currentSyncArchivedTransaction struct {
	Intent           currentSyncIntent
	ProgressExists   bool
	ProgressTerminal bool
}

// inspectCurrentSyncPendingOwnership classifies permanent transaction history
// separately from the one transaction that may own recovery. Archived-only
// ownership is valid only before the first progress generation is published;
// once a journal exists, losing the active intent is ambiguous and fail-closed.
func inspectCurrentSyncPendingOwnership(
	caseRoot,
	stateRoot string,
) (currentSyncPendingOwnership, error) {
	owner, ownerExists, err := readCurrentSyncOwner(caseRoot, stateRoot)
	if err != nil {
		return currentSyncPendingOwnership{}, err
	}
	active, activeExists, err := readCurrentSyncIntent(caseRoot, stateRoot)
	if err != nil {
		return currentSyncPendingOwnership{}, err
	}
	archived, err := listCurrentSyncArchivedTransactions(stateRoot)
	if err != nil {
		return currentSyncPendingOwnership{}, err
	}
	if ownerExists {
		if activeExists && !currentSyncCanonicalEqual(owner.Intent, active) {
			return currentSyncPendingOwnership{}, fmt.Errorf(
				"current sync transaction owner differs from active intent",
			)
		}
		ownerArchiveMatches := 0
		for _, transaction := range archived {
			if currentSyncCanonicalEqual(owner.Intent, transaction.Intent) {
				ownerArchiveMatches++
				continue
			}
			if transaction.ProgressTerminal {
				continue
			}
			if transaction.ProgressExists {
				return currentSyncPendingOwnership{}, fmt.Errorf(
					"current sync transaction owner coexists with an unrelated nonterminal journal: %s",
					transaction.Intent.PlanSHA256,
				)
			}
			return currentSyncPendingOwnership{}, fmt.Errorf(
				"current sync transaction owner coexists with an unrelated archived-only transaction: %s",
				transaction.Intent.PlanSHA256,
			)
		}
		if ownerArchiveMatches > 1 {
			return currentSyncPendingOwnership{}, fmt.Errorf(
				"current sync transaction owner matches multiple archived transactions",
			)
		}
		if ownerArchiveMatches == 0 && activeExists {
			return currentSyncPendingOwnership{}, fmt.Errorf(
				"current sync transaction owner has no archived transaction",
			)
		}
		if ownerArchiveMatches == 0 && !activeExists {
			return currentSyncPendingOwnership{
				Exists: true,
				Intent: owner.Intent,
			}, nil
		}
		return currentSyncPendingOwnership{
			Exists: true,
			Intent: owner.Intent,
		}, nil
	}

	var activeTransaction *currentSyncArchivedTransaction
	archivedOnlyOwners := []currentSyncIntent{}
	for index := range archived {
		transaction := &archived[index]
		if activeExists && currentSyncCanonicalEqual(active, transaction.Intent) {
			if activeTransaction != nil {
				return currentSyncPendingOwnership{}, fmt.Errorf(
					"current sync active intent matches multiple archived transactions",
				)
			}
			activeTransaction = transaction
			continue
		}
		if transaction.ProgressTerminal {
			continue
		}
		if transaction.ProgressExists {
			return currentSyncPendingOwnership{}, fmt.Errorf(
				"current sync nonterminal journal exists without its active intent: %s",
				transaction.Intent.PlanSHA256,
			)
		}
		archivedOnlyOwners = append(archivedOnlyOwners, transaction.Intent)
	}

	if activeExists {
		if activeTransaction == nil {
			if len(archived) == 0 {
				return currentSyncPendingOwnership{Exists: true, Intent: active}, nil
			}
			return currentSyncPendingOwnership{}, fmt.Errorf(
				"current sync active intent has no unique exact archived transaction",
			)
		}
		if len(archivedOnlyOwners) != 0 {
			return currentSyncPendingOwnership{}, fmt.Errorf(
				"current sync has an active owner and an archived-only owner",
			)
		}
		return currentSyncPendingOwnership{Exists: true, Intent: active}, nil
	}
	if len(archivedOnlyOwners) > 1 {
		return currentSyncPendingOwnership{}, fmt.Errorf(
			"current sync has multiple archived-only transaction owners",
		)
	}
	if len(archivedOnlyOwners) == 1 {
		return currentSyncPendingOwnership{
			Exists: true,
			Intent: archivedOnlyOwners[0],
		}, nil
	}
	return currentSyncPendingOwnership{}, nil
}

func listCurrentSyncArchivedTransactions(
	stateRoot string,
) ([]currentSyncArchivedTransaction, error) {
	root, err := rekitfs.OpenAnchoredRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := root.ListNoFollow(currentSyncTransactionsRel, currentSyncMaxFiles+1)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	transactions := make([]currentSyncArchivedTransaction, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf(
				"current sync transaction namespace contains a non-directory transaction: %s",
				entry.Name(),
			)
		}
		if !validCurrentSyncSHA(entry.Name()) || entry.Name() != strings.ToLower(entry.Name()) {
			return nil, fmt.Errorf(
				"current sync transaction name is invalid: %s",
				entry.Name(),
			)
		}
		transactionPath := currentSyncTransactionsRel + "/" + entry.Name()
		intent, err := readCurrentSyncArchivedIntent(stateRoot, transactionPath)
		if err != nil {
			return nil, err
		}
		progress, progressExists, err := readCurrentSyncProgress(stateRoot, intent)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, currentSyncArchivedTransaction{
			Intent:           intent,
			ProgressExists:   progressExists,
			ProgressTerminal: progressExists && currentSyncProgressTerminal(progress, intent),
		})
	}
	return transactions, nil
}
