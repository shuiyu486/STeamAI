package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
)

const (
	PublicFailureSourceExecutable = "executable-validation"
	PublicFailureSourceRuntime    = "public-runtime"
)

type publicUsageError struct {
	err error
}

func (err *publicUsageError) Error() string {
	if err == nil || err.err == nil {
		return ""
	}
	return err.err.Error()
}

func (err *publicUsageError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.err
}

type publicFailureSummary struct {
	Now    string `json:"now"`
	Reason string `json:"reason"`
	Next   string `json:"next"`
}

type publicFailureDiagnostics struct {
	Code             string `json:"code"`
	Stage            string `json:"stage"`
	MutationBoundary string `json:"mutationBoundary"`
	MutationApplied  bool   `json:"mutationApplied"`
	NextAction       string `json:"nextAction"`
	Flag             string `json:"flag,omitempty"`
	Expected         string `json:"expected,omitempty"`
	Actual           string `json:"actual,omitempty"`
	Detail           string `json:"detail"`
}

type publicFailureEnvelope struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Kind          string                   `json:"kind"`
	Command       string                   `json:"command"`
	OK            bool                     `json:"ok"`
	ExitCode      int                      `json:"exitCode"`
	Summary       publicFailureSummary     `json:"summary"`
	Diagnostics   publicFailureDiagnostics `json:"diagnostics"`
}

func wrapPublicUsageError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*publicUsageError](err); ok {
		return err
	}
	return &publicUsageError{err: err}
}

// RenderPublicFailure is the single process-level renderer for the no-mode
// STeamAI public surface. Public status and continue are zero-write routes, so
// failures rendered here have not crossed a mutation boundary.
func RenderPublicFailure(args []string, err error, source string, stdout, stderr io.Writer) int {
	return renderPublicFailure(
		classifyPublicFailure(args, err, source),
		PublicDiagnosticsRequested(args),
		stdout,
		stderr,
	)
}

// RenderRuntimePlanFailure shares the public typed failure envelope with the
// explicit runtime surface without converting ordinary maintenance failures.
func RenderRuntimePlanFailure(args []string, err error, stdout, stderr io.Writer) (int, bool) {
	if _, ok := plancontract.FromError(err); !ok {
		return 0, false
	}
	return renderPublicFailure(
		classifyPublicFailure(args, err, PublicFailureSourceRuntime),
		runtimeJSONRequested(args),
		stdout,
		stderr,
	), true
}

func renderPublicFailure(failure publicFailureEnvelope, diagnostics bool, stdout, stderr io.Writer) int {
	if diagnostics {
		if stdout == nil {
			stdout = io.Discard
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(failure)
		return failure.ExitCode
	}
	if stderr == nil {
		stderr = io.Discard
	}
	_, _ = fmt.Fprintf(
		stderr,
		"现在：%s\n原因：%s\n下一步：%s\n",
		failure.Summary.Now,
		failure.Summary.Reason,
		failure.Summary.Next,
	)
	return failure.ExitCode
}

func PublicDiagnosticsRequested(args []string) bool {
	return diagnosticsRequested(args, 1, false)
}

func runtimeJSONRequested(args []string) bool {
	return diagnosticsRequested(args, 0, true)
}

func diagnosticsRequested(args []string, start int, runtimeFormat bool) bool {
	for index := start; index < len(args); index++ {
		argument := strings.TrimSpace(args[index])
		name, value, assigned := strings.Cut(argument, "=")
		switch strings.ToLower(name) {
		case "--diagnostics":
			return true
		case "--format", "-format":
			if !runtimeFormat && !strings.EqualFold(name, "--format") {
				continue
			}
			if !assigned && index+1 < len(args) {
				value = args[index+1]
			}
			if strings.EqualFold(strings.TrimSpace(value), "json") {
				return true
			}
		}
	}
	return false
}

func classifyPublicFailure(args []string, err error, source string) publicFailureEnvelope {
	failure := publicFailureEnvelope{
		SchemaVersion: 1,
		Kind:          "steamai-public-failure",
		Command:       publicFailureCommand(args),
		OK:            false,
		ExitCode:      1,
		Summary: publicFailureSummary{
			Now:    "本次请求未完成",
			Reason: "运行时未能完成 public command",
			Next:   "刷新状态并按主 Agent 展示的恢复动作处理；需要机器细节时加 --diagnostics",
		},
		Diagnostics: publicFailureDiagnostics{
			Code:             "public-runtime-failed",
			Stage:            PublicFailureSourceRuntime,
			MutationBoundary: "none",
			MutationApplied:  false,
			NextAction:       "Refresh public status and follow only its fresh typed recovery action.",
			Detail:           publicFailureDetail(err),
		},
	}

	if planFailure, ok := plancontract.FromError(err); ok {
		failure.Diagnostics = publicFailureDiagnostics{
			Code:             planFailure.Code,
			Stage:            planFailure.Stage,
			MutationBoundary: planFailure.MutationBoundary,
			MutationApplied:  planFailure.MutationApplied,
			NextAction:       planFailure.NextAction,
			Flag:             planFailure.Flag,
			Expected:         planFailure.Expected,
			Actual:           planFailure.Actual,
			Detail:           planFailure.Detail,
		}
		switch planFailure.Code {
		case plancontract.CodePhaseConflict, plancontract.CodePlanMissing, plancontract.CodePlanInvalid:
			failure.ExitCode = 2
			failure.Summary = publicFailureSummary{
				Now:    "命令未执行",
				Reason: "预览或 Apply 参数未通过安全合同校验",
				Next:   "重新生成 fresh 预览，并只使用它返回的 exact action",
			}
		case plancontract.CodePlanMismatch:
			failure.Summary = publicFailureSummary{
				Now:    "本次没有执行 Apply",
				Reason: "已审核的预览不再是当前版本",
				Next:   "重新生成并审核 fresh 预览",
			}
		}
		return failure
	}

	if usage, ok := errors.AsType[*publicUsageError](err); ok {
		failure.ExitCode = 2
		failure.Summary = publicFailureSummary{
			Now:    "命令未执行",
			Reason: "public command 参数未通过校验",
			Next:   "运行 steamai help，并只使用 public command 支持的参数",
		}
		failure.Diagnostics = publicFailureDiagnostics{
			Code:             "public-usage-invalid",
			Stage:            "public-usage-validation",
			MutationBoundary: "none",
			MutationApplied:  false,
			NextAction:       "Run steamai help and retry with only supported public arguments.",
			Detail:           publicFailureDetail(usage),
		}
		return failure
	}

	if strings.TrimSpace(source) == PublicFailureSourceExecutable {
		failure.Summary = publicFailureSummary{
			Now:    "项目运行入口未通过完整性校验",
			Reason: "当前 executable 不能安全处理该项目请求",
			Next:   "请让 STeamAI 按 fresh 状态恢复或重新发布 verified project-local runtime bundle",
		}
		failure.Diagnostics = publicFailureDiagnostics{
			Code:             "public-executable-invalid",
			Stage:            PublicFailureSourceExecutable,
			MutationBoundary: "none",
			MutationApplied:  false,
			NextAction:       "Recover from fresh project state or republish the verified project-local runtime bundle.",
			Detail:           publicFailureDetail(err),
		}
	}
	return failure
}

func publicFailureCommand(args []string) string {
	if len(args) == 0 {
		return "help"
	}
	switch command := strings.ToLower(strings.TrimSpace(args[0])); command {
	case "-h", "--help":
		return "help"
	case "help", "status", "continue":
		return command
	}
	for index, argument := range args {
		name, value, assigned := strings.Cut(strings.TrimSpace(argument), "=")
		if !strings.EqualFold(name, "-Command") && !strings.EqualFold(name, "--command") {
			continue
		}
		if !assigned && index+1 < len(args) {
			value = args[index+1]
		}
		if command := strings.ToLower(strings.TrimSpace(value)); command != "" {
			return command
		}
	}
	return "unknown"
}

func publicFailureDetail(err error) string {
	if err == nil || strings.TrimSpace(err.Error()) == "" {
		return "public command failed without typed detail"
	}
	return strings.TrimSpace(err.Error())
}
