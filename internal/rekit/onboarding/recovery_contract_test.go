package onboarding

import (
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
)

func TestCurrentPackControlWritesAreAcceptedRecoveryGenerations(t *testing.T) {
	repo := testRepoRoot(t)
	packs := []struct{ pack, lane string }{
		{"_template", "feature-analysis"}, {defaults.DefaultPack, "binary-analysis-case"}, {"web-security", "feature-analysis"},
		{"malware-analysis", "sample-analysis-case"}, {"vuln-research", "vuln-analysis-case"}, {"ctf", "challenge-analysis-case"},
		{"unpack-pe", "unpack-analysis-case"}, {"ollvm", "obfuscation-analysis-case"}, {"android-native", "native-analysis-case"},
	}
	for _, fixture := range packs {
		t.Run(fixture.pack, func(t *testing.T) {
			opt := testOptions(t.TempDir() + "/case")
			opt.Pack, opt.InitialLane = fixture.pack, fixture.lane
			plan, err := Preview(repo, opt)
			if err != nil {
				t.Fatal(err)
			}
			for _, write := range plan.ExclusivePlan.Writes {
				switch write.Kind {
				case "managed-file", "template-file":
					normalized := append([]byte{}, write.Content...)
					if write.Kind == "template-file" {
						canonical, err := missionintent.CanonicalRecoveryWriteAt(plan.CaseRoot, plan.Identity, missionintent.RecoveryWrite{
							Path: write.Path, Kind: write.Kind, SHA256: write.SHA256, Size: write.Size,
							Content: write.Content, PublicationPhase: write.PublicationPhase,
						})
						if err != nil {
							t.Fatal(err)
						}
						normalized = canonical.Content
					}
					if err := caseshim.ValidatePackRecoveryWrite(fixture.pack, write.Path, write.Kind, missionintent.SHA256(normalized)); err != nil {
						t.Fatal(err)
					}
					if err := caseshim.ValidatePackRecoveryWrite(fixture.pack, write.Path, write.Kind, missionintent.SHA256(append(normalized, 'x'))); err == nil {
						t.Fatalf("mutated trusted write accepted: %s", write.Path)
					}
				case "managed-block":
					if err := caseshim.ValidateManagedBlockSHA256(fixture.pack, write.SHA256); err != nil {
						t.Fatal(err)
					}
				case "support-file":
					if err := caseshim.ValidateSupportSHA256(fixture.pack, write.Path, write.SHA256); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
