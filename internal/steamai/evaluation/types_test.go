package evaluation

import (
	"strings"
	"testing"
)

func TestDecodeRequestIsStrictAndReadonly(t *testing.T) {
	digest := strings.Repeat("a", 64)
	valid := `{"runId":"CAL-001-PAIR-1","purpose":"calibration","scenario":"CAL-001-scenario.md","scenarioSha256":"` + digest + `","rubric":"CAL-001-rubric.md","rubricSha256":"` + digest + `","verifiedLearningContract":"verified-learning.md","verifiedLearningContractSha256":"` + digest + `","baselineSha256":"` + digest + `","candidatePatch":"CAL-001.patch","candidatePatchSha256":"` + digest + `","model":"sonnet","slotId":"CAL-001-PAIR-1","expectedClass":"improvement","suiteSpec":"CAL-001-suite.json","suiteSpecSha256":"` + digest + `","maxSeconds":60,"maxBudgetUsd":1.5}`
	if _, err := DecodeRequest(strings.NewReader(valid)); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		strings.Replace(valid, `"purpose":"calibration"`, `"purpose":"other"`, 1),
		strings.Replace(valid, `"scenario":"CAL-001-scenario.md"`, `"scenario":"../escape.md"`, 1),
		strings.Replace(valid, `"scenario":"CAL-001-scenario.md"`, `"scenario":"C:escape.md"`, 1),
		strings.Replace(valid, `"maxSeconds":60`, `"maxSeconds":0`, 1),
		strings.Replace(valid, `"baselineSha256":"`+digest+`"`, `"baselineSha256":"bad"`, 1),
		strings.TrimSuffix(valid, "}") + `,"unknown":true}`,
		valid + `{}`,
	} {
		request, err := DecodeRequest(strings.NewReader(invalid))
		if err == nil {
			if _, pathErr := resolveBoundPath(`C:\case\.steamai-vnext\evaluations\specs`, request.Scenario); pathErr == nil {
				t.Fatalf("invalid request accepted: %s", invalid)
			}
		}
	}
}

func TestBundleIdentityIgnoresArmOrderingAndBindsReveal(t *testing.T) {
	bundle := BundleManifest{SchemaVersion: 1, RunID: "CAL-001", Purpose: "calibration", SlotID: "CAL-001-PAIR-1", ExpectedClass: "improvement", Arms: []ArmRecord{{Label: "arm-b"}, {Label: "arm-a"}}, RevealSHA256: strings.Repeat("a", 64)}
	first := BundleIdentity(bundle)
	bundle.Arms[0], bundle.Arms[1] = bundle.Arms[1], bundle.Arms[0]
	if second := BundleIdentity(bundle); second != first {
		t.Fatalf("bundle identity depends on arm ordering: %s != %s", first, second)
	}
	bundle.RevealSHA256 = strings.Repeat("b", 64)
	if BundleIdentity(bundle) == first {
		t.Fatal("final bundle identity 未绑定 reveal SHA")
	}
}
