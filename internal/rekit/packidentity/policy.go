package packidentity

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	// Canonical is the only active binary reverse-engineering pack identity.
	Canonical = "binary-re"
	// RetiredGeneric and RetiredVMP identify old projects without aliasing them.
	RetiredGeneric = "generic-binary-re"
	RetiredVMP     = "vmp-re"
	// MigrationRequiredCode is stable for callers that need machine routing.
	MigrationRequiredCode = "pack-migration-required"
)

var retiredIDs = map[string]struct{}{
	RetiredGeneric: {},
	RetiredVMP:     {},
}

// MigrationRequiredError reports a retired pack identity without silently
// mapping it to the canonical pack or creating an alias.
type MigrationRequiredError struct {
	Requested string
	Canonical string
}

func (e *MigrationRequiredError) Error() string {
	requested := strings.TrimSpace(e.Requested)
	canonical := strings.TrimSpace(e.Canonical)
	if canonical == "" {
		canonical = Canonical
	}
	return fmt.Sprintf(
		"%s: pack %q is retired; use canonical pack %q explicitly; no automatic migration or alias is provided",
		MigrationRequiredCode,
		requested,
		canonical,
	)
}

// Code exposes the stable typed diagnostic code without requiring string
// parsing of Error().
func (e *MigrationRequiredError) Code() string { return MigrationRequiredCode }

// Validate rejects only retired identities. Unknown identities remain ordinary
// missing/unknown-pack errors and are not mislabeled as migrations.
func Validate(pack string) error {
	id := strings.ToLower(strings.TrimSpace(pack))
	if _, retired := retiredIDs[id]; retired {
		return &MigrationRequiredError{Requested: strings.TrimSpace(pack), Canonical: Canonical}
	}
	return nil
}

func IsRetired(pack string) bool {
	id := strings.ToLower(strings.TrimSpace(pack))
	_, ok := retiredIDs[id]
	return ok
}

func IsMigrationRequired(err error) bool {
	var target *MigrationRequiredError
	return errors.As(err, &target)
}

func RetiredIDs() []string {
	ids := make([]string, 0, len(retiredIDs))
	for id := range retiredIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
