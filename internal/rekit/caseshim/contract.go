package caseshim

import "fmt"

// CurrentCanonicalSHA256 identifies today's source template. Accepted hashes
// are append-only so an already-published pending intent remains recoverable
// after a future canonical template revision.
const CurrentCanonicalSHA256 = "648dd5e4b65a66fc92d5b40b431c21cef55199bc674b3a8ac92909e82a7bee68"

var acceptedSHA256 = map[string]struct{}{
	CurrentCanonicalSHA256: {},
}

func ValidateSHA256(sum string) error {
	if _, ok := acceptedSHA256[sum]; !ok {
		return fmt.Errorf("case-local shim does not match an accepted canonical thin-shim generation")
	}
	return nil
}
