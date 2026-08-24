package instructionpacket

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

func TestBuildSeparatesDurableIdentityFromEphemeralContent(t *testing.T) {
	root := t.TempDir()
	writePacketTestFile(t, root, "common/policies/manifest.yml", "policies:\n  - id: baseline\n    path: baseline.md\n")
	writePacketTestFile(t, root, "common/policies/baseline.md", "POLICY-CONTENT\n")
	writePacketTestFile(t, root, "common/prompts/member.md", "PROMPT-CONTENT\n")
	m := &manifest.Manifest{Pack: "fixture-pack", CommonPolicies: []string{"baseline"}, PromptFiles: []string{"common/prompts/member.md"}}
	spec := Spec{Mode: ModePromptAndPolicy, RequiredSources: []string{"common/policies/baseline.md", "common/prompts/member.md"}, ReceiptKind: "fixture-execution-result"}

	packet, err := Build(root, m.Pack, m, spec)
	if err != nil {
		t.Fatal(err)
	}
	identity := packet.Identity()
	if err := ValidateIdentity(identity); err != nil {
		t.Fatal(err)
	}
	if identity.ReceiptKind != spec.ReceiptKind || len(identity.Sources) != 2 || identity.SHA256 == "" {
		t.Fatalf("unexpected durable instruction identity: %+v", identity)
	}
	serializedPacket, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serializedPacket), "CONTENT") || string(serializedPacket) != "{}" {
		t.Fatalf("ephemeral packet exposed source content through JSON: %s", serializedPacket)
	}
	serializedIdentity, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serializedIdentity), "CONTENT") || strings.Contains(string(serializedIdentity), "content") {
		t.Fatalf("durable identity exposed source content: %s", serializedIdentity)
	}
	inline, err := packet.InlineMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"POLICY-CONTENT", "PROMPT-CONTENT", identity.SHA256, spec.ReceiptKind} {
		if !strings.Contains(inline, want) {
			t.Fatalf("inline packet omitted %q", want)
		}
	}
	if _, err := Reload(root, identity); err != nil {
		t.Fatalf("reload rejected current source bytes: %v", err)
	}

	writePacketTestFile(t, root, "common/policies/baseline.md", "POLICY-DRIFT\n")
	if _, err := Reload(root, identity); err == nil || !strings.Contains(err.Error(), "source drifted") {
		t.Fatalf("reload did not reject source drift: %v", err)
	}
}

func TestIdentityHashBindsReceiptKindAndReturnsDeepClone(t *testing.T) {
	root := t.TempDir()
	writePacketTestFile(t, root, "common/policies/manifest.yml", "policies:\n  - id: baseline\n    path: baseline.md\n")
	writePacketTestFile(t, root, "common/policies/baseline.md", "bounded policy\n")
	m := &manifest.Manifest{Pack: "fixture-pack", CommonPolicies: []string{"baseline"}}
	left, err := Build(root, m.Pack, m, Spec{Mode: ModePolicyOnly, RequiredSources: []string{"common/policies/baseline.md"}, ReceiptKind: "left-result"})
	if err != nil {
		t.Fatal(err)
	}
	right, err := Build(root, m.Pack, m, Spec{Mode: ModePolicyOnly, RequiredSources: []string{"common/policies/baseline.md"}, ReceiptKind: "right-result"})
	if err != nil {
		t.Fatal(err)
	}
	if left.Identity().SHA256 == right.Identity().SHA256 {
		t.Fatal("instruction packet hash did not bind receipt kind")
	}
	mutated := left.Identity()
	mutated.Sources[0].Path = "changed.md"
	if left.Identity().Sources[0].Path == "changed.md" {
		t.Fatal("instruction packet exposed mutable identity state")
	}
	forged := left.Identity()
	forged.ReceiptKind = "forged-result"
	if err := ValidateIdentity(forged); err == nil || !strings.Contains(err.Error(), "aggregate identity drifted") {
		t.Fatalf("receipt-kind identity drift was not rejected: %v", err)
	}
}

func TestPolicyOnlyRequiresPoliciesAndRejectsDeclaredPrompts(t *testing.T) {
	root := t.TempDir()
	writePacketTestFile(t, root, "common/policies/manifest.yml", "policies:\n  - id: baseline\n    path: baseline.md\n")
	writePacketTestFile(t, root, "common/policies/baseline.md", "bounded policy\n")
	writePacketTestFile(t, root, "common/prompts/member.md", "member prompt\n")
	spec := Spec{Mode: ModePolicyOnly, RequiredSources: []string{"common/policies/baseline.md"}, ReceiptKind: "fixture-result"}
	withPrompt := &manifest.Manifest{Pack: "fixture-pack", CommonPolicies: []string{"baseline"}, PromptFiles: []string{"common/prompts/member.md"}}
	if _, err := Build(root, withPrompt.Pack, withPrompt, spec); err == nil || !strings.Contains(err.Error(), "cannot silently omit declared prompts") {
		t.Fatalf("policy-only packet accepted a declared prompt it would omit: %v", err)
	}
	withoutPolicy := &manifest.Manifest{Pack: "fixture-pack"}
	if _, err := Build(root, withoutPolicy.Pack, withoutPolicy, spec); err == nil || !strings.Contains(err.Error(), "requires declared policies") {
		t.Fatalf("policy-only packet accepted an empty policy set: %v", err)
	}
	unknownPolicy := &manifest.Manifest{Pack: "fixture-pack", CommonPolicies: []string{"missing"}}
	if _, err := Build(root, unknownPolicy.Pack, unknownPolicy, spec); err == nil || !strings.Contains(err.Error(), "unknown common policy") {
		t.Fatalf("packet accepted an unknown common policy: %v", err)
	}
}

func writePacketTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
