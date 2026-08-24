package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

// runReviewedContinueRequest keeps CLI product tests on the same review path
// as the public Mission Commander. It executes only an exact returned Apply;
// a blocked preview remains a preview.
func runReviewedContinueRequest(t *testing.T, out *bytes.Buffer, requested []string) error {
	t.Helper()
	requestedFormat := continueRequestedFormat(requested)
	previewArgs := append([]string{}, requested...)
	hadPhase := false
	hadFormat := false
	for index := 0; index < len(previewArgs); index++ {
		switch strings.ToLower(strings.TrimSpace(previewArgs[index])) {
		case "-apply", "--apply":
			previewArgs[index] = "-WhatIf"
			hadPhase = true
		case "-whatif", "--what-if":
			hadPhase = true
		case "-format", "--format":
			if index+1 < len(previewArgs) {
				previewArgs[index+1] = "json"
				index++
				hadFormat = true
			}
		}
	}
	if !hadPhase {
		previewArgs = append(previewArgs, "-WhatIf")
	}
	if !hadFormat {
		previewArgs = append(previewArgs, "-Format", "json")
	}
	out.Reset()
	if err := Run(previewArgs, out); err != nil {
		return err
	}
	var preview struct {
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		ContinuePlanSHA256          string                              `json:"continuePlanSha256"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		return fmt.Errorf("continue preview for exact Apply is not JSON: %w", err)
	}
	request := preview.MissionCommanderActionQueue.CurrentDriverRequest
	if request == nil || !request.CommandExecutable || request.Blocked {
		return writeRequestedContinuePreviewFormat(out, requested, requestedFormat)
	}
	invocation, err := commands.ParsePublicInvocation(request.Command)
	if err != nil {
		return fmt.Errorf("continue preview returned invalid current request: %w", err)
	}
	if invocation.Command != commands.Continue || !invocation.HasFlag("-Apply") || invocation.HasFlag("-WhatIf") {
		return writeRequestedContinuePreviewFormat(out, requested, requestedFormat)
	}
	applyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, request)
	if !ok {
		return fmt.Errorf("continue preview returned a non-executable Apply request: %+v", request)
	}
	applyArgs = appendMissingContinueContext(t, applyArgs, requested)
	out.Reset()
	if err := Run(applyArgs, out); err != nil {
		return err
	}
	if !strings.EqualFold(requestedFormat, "json") {
		parsed, err := Parse(applyArgs)
		if err != nil {
			return err
		}
		ctx, err := runtime.New(parsed.Target, parsed.Pack)
		if err != nil {
			return err
		}
		previewOpt := parsed.Continue
		previewOpt.ExpectedContinuePlanSHA256 = ""
		previewResult, err := workstream.ContinuePreview(ctx.RepoRoot, ctx.Target, ctx.Pack, previewOpt)
		if err != nil {
			return err
		}
		out.Reset()
		return writeContinueText(out, previewResult)
	}
	return nil
}

func continueRequestedFormat(args []string) string {
	for index, arg := range args {
		if (strings.EqualFold(strings.TrimSpace(arg), "-Format") || strings.EqualFold(strings.TrimSpace(arg), "--format")) && index+1 < len(args) {
			return strings.TrimSpace(args[index+1])
		}
	}
	return "json"
}

func writeRequestedContinuePreviewFormat(out *bytes.Buffer, requested []string, format string) error {
	if strings.EqualFold(strings.TrimSpace(format), "json") {
		return nil
	}
	textArgs := append([]string{}, requested...)
	for index := 0; index < len(textArgs); index++ {
		switch strings.ToLower(strings.TrimSpace(textArgs[index])) {
		case "-apply", "--apply":
			textArgs[index] = "-WhatIf"
		case "-format", "--format":
			if index+1 < len(textArgs) {
				textArgs[index+1] = format
				index++
			}
		}
	}
	out.Reset()
	return Run(textArgs, out)
}

func appendMissingContinueContext(t *testing.T, returned, requested []string) []string {
	t.Helper()
	out := append([]string{}, returned...)
	for _, flag := range []string{"-Target", "-Pack"} {
		if hasCLIFlag(out, flag) {
			continue
		}
		if value, ok := cliFlagValue(requested, flag); ok {
			out = append(out, flag, value)
		}
	}
	return out
}

func hasCLIFlag(args []string, wanted string) bool {
	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), wanted) {
			return true
		}
	}
	return false
}

func cliFlagValue(args []string, wanted string) (string, bool) {
	for index, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), wanted) && index+1 < len(args) {
			return args[index+1], true
		}
	}
	return "", false
}
