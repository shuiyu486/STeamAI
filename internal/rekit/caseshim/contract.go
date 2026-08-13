package caseshim

import "fmt"

// CurrentCanonicalSHA256 identifies today's canonical publication bytes.
// Accepted hashes are append-only so an already-published pending intent
// remains recoverable after a future canonical template revision.
const CurrentCanonicalSHA256 = "9a690e42ab4ff3692146fcdfad6635aeb47b8225df93e35828063fc3e4b4874e"

var acceptedSHA256 = map[string]struct{}{
	CurrentCanonicalSHA256: {},
	"d44545d7e0d44781b78aabcdfd6ebcdb86dfffbd309011698f83e7d5a26f299b": {},
	"648dd5e4b65a66fc92d5b40b431c21cef55199bc674b3a8ac92909e82a7bee68": {},
}

func ValidateSHA256(sum string) error {
	if _, ok := acceptedSHA256[sum]; !ok {
		return fmt.Errorf("case-local shim does not match an accepted canonical thin-shim generation")
	}
	return nil
}
