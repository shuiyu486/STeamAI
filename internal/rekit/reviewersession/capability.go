package reviewersession

import "github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"

// ReadOnlyCapability returns the only capability binding accepted by Reviewer
// dispatch and completion artifacts. Keeping this constructor here prevents
// production and local fixtures from rebuilding the hash independently.
func ReadOnlyCapability() capabilitycontract.Binding {
	binding, err := capabilitycontract.Bind(capabilitycontract.ReadOnly())
	if err != nil {
		panic(err)
	}
	return binding
}
