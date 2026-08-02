package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runHashBoundHandoffApply(t *testing.T, args []string, out *bytes.Buffer) error {
	t.Helper()
	hasExpected := false
	hasStamp := false
	for _, arg := range args {
		hasExpected = hasExpected || strings.EqualFold(arg, "-ExpectedHandoffPlanSha256") || strings.EqualFold(arg, "-ExpectedHandoffPlanSHA256") || strings.EqualFold(arg, "--expected-handoff-plan-sha256")
		hasStamp = hasStamp || strings.EqualFold(arg, "-HandoffPublicationStamp") || strings.EqualFold(arg, "--handoff-publication-stamp")
	}
	if hasExpected && hasStamp {
		return Run(args, out)
	}
	previewArgs := make([]string, 0, len(args))
	hasApply := false
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		if strings.EqualFold(arg, "-Apply") || strings.EqualFold(arg, "--apply") {
			previewArgs = append(previewArgs, "-WhatIf")
			hasApply = true
			continue
		}
		if strings.EqualFold(arg, "-ExpectedHandoffPlanSha256") || strings.EqualFold(arg, "-ExpectedHandoffPlanSHA256") || strings.EqualFold(arg, "--expected-handoff-plan-sha256") || strings.EqualFold(arg, "-HandoffPublicationStamp") || strings.EqualFold(arg, "--handoff-publication-stamp") {
			idx++
			continue
		}
		previewArgs = append(previewArgs, arg)
	}
	if !hasApply {
		return Run(args, out)
	}
	var previewOut bytes.Buffer
	if err := Run(previewArgs, &previewOut); err != nil {
		return err
	}
	var preview struct {
		PublicationPlanSHA256 string `json:"publicationPlanSha256"`
		PublicationStamp      string `json:"publicationStamp"`
	}
	if err := json.Unmarshal(previewOut.Bytes(), &preview); err != nil {
		t.Fatalf("handoff preview stdout is not JSON: %v\n%s", err, previewOut.String())
	}
	if strings.TrimSpace(preview.PublicationPlanSHA256) == "" {
		return Run(args, out)
	}
	applyArgs := make([]string, 0, len(args)+2)
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		if strings.EqualFold(arg, "-ExpectedHandoffPlanSha256") || strings.EqualFold(arg, "-ExpectedHandoffPlanSHA256") || strings.EqualFold(arg, "--expected-handoff-plan-sha256") || strings.EqualFold(arg, "-HandoffPublicationStamp") || strings.EqualFold(arg, "--handoff-publication-stamp") {
			idx++
			continue
		}
		applyArgs = append(applyArgs, arg)
	}
	applyArgs = append(applyArgs,
		"-ExpectedHandoffPlanSha256", preview.PublicationPlanSHA256,
		"-HandoffPublicationStamp", preview.PublicationStamp,
	)
	return Run(applyArgs, out)
}
