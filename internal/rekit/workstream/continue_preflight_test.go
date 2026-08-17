package workstream

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

func TestContinueAuthorityAppendReasonPolicyMatrix(t *testing.T) {
	caseRoot := t.TempDir()
	authorityRel := "captures/vm_opcode_semantics_confirmed.csv"
	handlerRel := "captures/vm_handler_roles_confirmed.csv"
	textRel := "captures/authority.txt"
	missingRel := "captures/missing.csv"
	writeWorkstreamTestFile(t, caseRoot, authorityRel, "opcode,semantics,status\nOP_EXISTING,known,confirmed\n")
	writeWorkstreamTestFile(t, caseRoot, handlerRel, "handler,role,status\nH_EXISTING,known,confirmed\n")
	writeWorkstreamTestFile(t, caseRoot, textRel, "non-csv authority fixture\n")

	baseContext := continueContext{
		inst:     instance.Instance{CaseRoot: caseRoot},
		manifest: &manifest.Manifest{AuthorityFiles: []string{authorityRel, handlerRel, textRel, missingRel}},
		policy:   defaultContinuePolicy(),
	}

	accepted := map[string]any{"verifier": "rule-verifier", "verdict": "accepted", "confidence": 0.95}
	if reason := baseContext.authorityAppendReason(validAuthorityEvent(authorityRel, "OP_OK"), accepted, authorityRel, candidateRows(validAuthorityEvent(authorityRel, "OP_OK"))); reason != "" {
		t.Fatalf("valid authority append reason = %q, want empty", reason)
	}

	cases := []struct {
		name   string
		mutate func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any)
		want   string
	}{
		{
			name: "not allowed authority file",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				event["authorityFile"] = "captures/not_allowed.csv"
				*authorityFile = "captures/not_allowed.csv"
			},
			want: "authority file is not allowed",
		},
		{
			name: "auto append disabled",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				ctx.policy.AuthorityAutoAppend = "never"
			},
			want: "authority auto append disabled",
		},
		{
			name: "low confidence",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				event["confidence"] = "0.65"
			},
			want: "confidence below threshold",
		},
		{
			name: "missing evidence",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				delete(event, "evidence")
			},
			want: "missing evidence",
		},
		{
			name: "missing accepted verifier",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				for key := range verification {
					delete(verification, key)
				}
			},
			want: "missing accepted verifier verdict",
		},
		{
			name: "missing authority file",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				event["authorityFile"] = missingRel
				*authorityFile = missingRel
			},
			want: "missing authority file",
		},
		{
			name: "non csv authority file",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				event["authorityFile"] = textRel
				*authorityFile = textRel
			},
			want: "only csv authority append is automated",
		},
		{
			name: "schema invalid",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				event["row"] = map[string]any{"opcode": "OP_SCHEMA", "semantics": "missing-status"}
				*rows = candidateRows(event)
			},
			want: "candidate row does not match authority csv schema",
		},
		{
			name: "authority conflict",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				event["row"] = map[string]any{"opcode": "OP_EXISTING", "semantics": "conflict", "status": "confirmed"}
				*rows = candidateRows(event)
			},
			want: "authority key conflict",
		},
		{
			name: "no candidate rows",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				ctx.policy.RequireSchemaValid = false
				delete(event, "row")
				*rows = nil
			},
			want: "no candidate rows",
		},
		{
			name: "candidate row contains newline",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				ctx.policy.RequireSchemaValid = false
				ctx.policy.RequireNoConflict = false
				*rows = []any{"OP_NEW,\"bad\nline\",confirmed"}
			},
			want: "candidate row contains newline",
		},
		{
			name: "too many rows",
			mutate: func(ctx *continueContext, event map[string]any, verification map[string]any, authorityFile *string, rows *[]any) {
				many := make([]any, 0, baseContext.policy.MaxAuthorityRowsPerRun+1)
				for i := 0; i <= baseContext.policy.MaxAuthorityRowsPerRun; i++ {
					many = append(many, map[string]any{"opcode": fmt.Sprintf("OP_MANY_%02d", i), "semantics": fmt.Sprintf("many-%02d", i), "status": "confirmed"})
				}
				*rows = many
			},
			want: "too many rows",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := baseContext
			event := validAuthorityEvent(authorityRel, "OP_OK")
			verification := map[string]any{"verifier": "rule-verifier", "verdict": "accepted", "confidence": 0.95}
			authorityFile := authorityRel
			rows := candidateRows(event)
			tc.mutate(&ctx, event, verification, &authorityFile, &rows)
			reason := ctx.authorityAppendReason(event, verification, authorityFile, rows)
			if !strings.Contains(reason, tc.want) {
				t.Fatalf("authority append reason = %q, want containing %q", reason, tc.want)
			}
		})
	}
}

func TestContinueAuthorityPreviewRecordsBackupAndDiffWouldWrites(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	authorityRel := "captures/vm_opcode_semantics_confirmed.csv"
	writeWorkstreamTestFile(t, caseRoot, authorityRel, "opcode,semantics,status\nOP_EXISTING,known,confirmed\n")
	ctx := continueContext{
		inst:     instance.Instance{CaseRoot: caseRoot},
		manifest: &manifest.Manifest{AuthorityFiles: []string{authorityRel}},
		policy:   defaultContinuePolicy(),
	}

	preview, err := ctx.previewEvent(validAuthorityEvent(authorityRel, "OP_OK"))
	if err != nil {
		t.Fatal(err)
	}
	if preview.Decision != "accept" || preview.Reason != "passed authority append policy" || preview.Rows != 1 {
		t.Fatalf("unexpected authority preview: %+v", preview)
	}
	assertWorkstreamPreviewWrite(t, preview.WouldWrites, authorityRel, "authority-csv", "would-append")
	assertWorkstreamPreviewWrite(t, preview.WouldWrites, ".rekit/runs/run-preview/backups/captures/vm_opcode_semantics_confirmed.csv", "run-artifact", "would-create")
	assertWorkstreamPreviewWrite(t, preview.WouldWrites, ".rekit/runs/run-preview/diffs/captures_vm_opcode_semantics_confirmed.csv.diff", "run-artifact", "would-create")
}

func assertWorkstreamPreviewWrite(t *testing.T, writes []StartWrite, path, kind, action string) {
	t.Helper()
	for _, write := range writes {
		if write.Path == path && write.Kind == kind && write.Action == action {
			return
		}
	}
	t.Fatalf("preview write %s/%s/%s not found in %+v", path, kind, action, writes)
}

func writeWorkstreamTestFile(t *testing.T, caseRoot, rel, text string) {
	t.Helper()
	path := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func validAuthorityEvent(authorityRel, opcode string) map[string]any {
	return map[string]any{
		"kind":          "candidate",
		"authorityFile": authorityRel,
		"confidence":    "0.95",
		"evidence":      "evidence-authority-token",
		"row":           map[string]any{"opcode": opcode, "semantics": "semantics-ok", "status": "confirmed"},
	}
}
