package laneid

import "testing"

func TestResolveAndLabelRoundTrip(t *testing.T) {
	tests := []struct {
		laneType string
		label    string
		id       string
	}{
		{laneType: "feature", label: "analysis", id: "feature-analysis"},
		{laneType: "feature-analysis", label: "analysis-live-check", id: "feature-analysis-live-check"},
		{laneType: "sample-analysis", label: "case", id: "sample-analysis-case"},
	}
	for _, test := range tests {
		t.Run(test.laneType+"/"+test.label, func(t *testing.T) {
			if got := Resolve(test.laneType, test.label); got != test.id {
				t.Fatalf("Resolve() = %q, want %q", got, test.id)
			}
			label, ok := Label(test.laneType, test.id)
			if !ok || label != test.label {
				t.Fatalf("Label() = %q, %t; want %q, true", label, ok, test.label)
			}
		})
	}
}

func TestLabelRejectsNonRoundTripID(t *testing.T) {
	for _, test := range []struct {
		laneType string
		id       string
	}{
		{laneType: "feature-analysis", id: "featureanalysis"},
		{laneType: "sample-analysis", id: "feature-case"},
	} {
		if label, ok := Label(test.laneType, test.id); ok {
			t.Fatalf("Label(%q, %q) = %q, true", test.laneType, test.id, label)
		}
	}
}
