package evaluation

import (
	"strings"
	"testing"
)

func TestDecodeRequestIsStrictAndReadonly(t *testing.T) {
	digest := strings.Repeat("a", 64)
	valid := `{"runId":"CAL-001-PAIR-1","purpose":"calibration","scenario":"CAL-001-scenario.md","scenarioSha256":"` + digest + `","rubric":"CAL-001-rubric.md","rubricSha256":"` + digest + `","verifiedLearningContract":"verified-learning.md","verifiedLearningContractSha256":"` + digest + `","baselineSha256":"` + digest + `","candidatePatch":"CAL-001.patch","candidatePatchSha256":"` + digest + `","model":"claude-sonnet-5","slotId":"CAL-001-PAIR-1","expectedClass":"improvement","suiteSpec":"CAL-001-suite.json","suiteSpecSha256":"` + digest + `","maxSeconds":60,"maxBudgetUsd":1.5}`
	if _, err := DecodeRequest(strings.NewReader(valid)); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		strings.Replace(valid, `"purpose":"calibration"`, `"purpose":"other"`, 1),
		strings.Replace(valid, `"scenario":"CAL-001-scenario.md"`, `"scenario":"../escape.md"`, 1),
		strings.Replace(valid, `"scenario":"CAL-001-scenario.md"`, `"scenario":"C:escape.md"`, 1),
		strings.Replace(valid, `"maxSeconds":60`, `"maxSeconds":0`, 1),
		strings.Replace(valid, `"model":"claude-sonnet-5"`, `"model":"--model"`, 1),
		strings.Replace(valid, `"model":"claude-sonnet-5"`, `"model":"model with spaces"`, 1),
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

func TestModelNamesAreProviderNeutral(t *testing.T) {
	for _, model := range []string{"claude-sonnet-5", "gpt-fixture[1m]", "provider/model-v1", "sonnet"} {
		if !modelPattern.MatchString(model) {
			t.Fatalf("模型名称被错误地限制为特定厂商: %s", model)
		}
	}
	for _, model := range []string{"", "--model", "model with spaces", "model\nvalue", "model;command"} {
		if modelPattern.MatchString(model) {
			t.Fatalf("无效模型名称被接受: %q", model)
		}
	}
}

func TestBundleIdentityIgnoresArmOrderingAndBindsPacketAndReveal(t *testing.T) {
	bundle := BundleManifest{
		SchemaVersion: 1, RunID: "CAL-001", Purpose: "calibration", SlotID: "CAL-001-PAIR-1", ExpectedClass: "improvement",
		Arms: []ArmRecord{{Label: "arm-b"}, {Label: "arm-a"}}, ReviewPacket: BoundFile{Path: "blind-review.json", SHA256: strings.Repeat("c", 64)},
		RevealSHA256: strings.Repeat("a", 64),
	}
	first := BundleIdentity(bundle)
	firstBlind := BlindBundleIdentity(bundle)
	bundle.Arms[0], bundle.Arms[1] = bundle.Arms[1], bundle.Arms[0]
	if second := BundleIdentity(bundle); second != first {
		t.Fatalf("bundle identity depends on arm ordering: %s != %s", first, second)
	}
	bundle.ReviewPacket.SHA256 = strings.Repeat("d", 64)
	if BundleIdentity(bundle) == first || BlindBundleIdentity(bundle) == firstBlind {
		t.Fatal("bundle identities 未绑定 review packet SHA")
	}
	bundle.ReviewPacket.SHA256 = strings.Repeat("c", 64)
	bundle.RevealSHA256 = strings.Repeat("b", 64)
	if BundleIdentity(bundle) == first {
		t.Fatal("final bundle identity 未绑定 reveal SHA")
	}
	if BlindBundleIdentity(bundle) != firstBlind {
		t.Fatal("blind bundle identity 错误绑定 reveal SHA")
	}
}
