package adapterhost

import "errors"

var errOwnedOutputIsolated = errors.New("exact owned output isolated from canonical path")

func isOwnedOutputIsolation(err error) bool {
	return errors.Is(err, errOwnedOutputIsolated)
}
