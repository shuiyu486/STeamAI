package steamai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/STeamAI/internal/steamai/evaluation"
)

func (a *app) evaluationCaseRoot() (string, error) {
	caseRoot, err := a.cwd()
	if err != nil {
		return "", err
	}
	caseRoot, err = filepath.Abs(caseRoot)
	if err != nil {
		return "", err
	}
	state, err := inspectCase(caseRoot)
	if err != nil {
		return "", err
	}
	if state != caseCurrent {
		return "", errors.New("evaluation 只能从完整 current case 运行")
	}
	return caseRoot, nil
}

func (a *app) evaluationSuitePrepare() error {
	caseRoot, err := a.evaluationCaseRoot()
	if err != nil {
		return err
	}
	request, err := evaluation.DecodeSuitePrepareRequest(a.stdin)
	if err != nil {
		return err
	}
	spec, rawSHA, err := evaluation.PrepareSuite(caseRoot, request)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "STeamAI evaluation suite spec 已发布：evaluations/specs/%s sha256:%s identity:%s\n", request.Name, rawSHA, spec.Identity)
	return err
}

func (a *app) evaluationSuiteFinalize() error {
	caseRoot, err := a.evaluationCaseRoot()
	if err != nil {
		return err
	}
	request, err := evaluation.DecodeSuiteFinalizeRequest(a.stdin)
	if err != nil {
		return err
	}
	suite, err := evaluation.FinalizeSuite(caseRoot, request)
	if err != nil {
		return err
	}
	decision := "no-go-or-inconclusive"
	if _, goErr := evaluation.VerifySuite(filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs"), request.Name); goErr == nil {
		decision = "go-eligible"
	}
	_, err = fmt.Fprintf(a.stdout, "STeamAI evaluation suite manifest 已发布：evaluations/runs/%s sha256:%s identity:%s decision:%s\n", request.Name, suite.ManifestSHA256, suite.Manifest.Identity, decision)
	return err
}

func (a *app) evaluationRun() error {
	caseRoot, err := a.evaluationCaseRoot()
	if err != nil {
		return err
	}
	request, err := evaluation.DecodeRequest(a.stdin)
	if err != nil {
		return err
	}
	git, err := a.resolveNativeExecutable("git.exe")
	if err != nil {
		return fmt.Errorf("找不到原生 Git: %w", err)
	}
	claude, err := a.resolveNativeExecutable("claude.exe")
	if err != nil {
		return fmt.Errorf("找不到原生 Claude Code: %w", err)
	}
	version, err := executableVersion(claude)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	bundle, err := evaluation.Run(ctx, git, claude, version, caseRoot, request)
	if err != nil {
		return err
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("编码 evaluation bundle: %w", err)
	}
	if _, err := fmt.Fprintf(a.stdout, "STeamAI evaluation run bundle 已发布：evaluations/runs/%s identity:%s；Reviewer 先审 manifest/arms，Commander 随后读取 reveal.json 解盲\n%s\n", request.RunID, bundle.Identity, data); err != nil {
		return err
	}
	runsRoot := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs")
	results, err := evaluation.BundleResults(runsRoot, bundle)
	if err != nil {
		return err
	}
	outcome := &evaluation.RunOutcomeError{RunID: request.RunID}
	for _, result := range results {
		if result.Status != "completed" || result.SafetyGate != "pass" {
			outcome.Arms = append(outcome.Arms, result)
			fmt.Fprintf(a.stderr, "evaluation arm status=%s exit=%d error=%s；bundle 已发布于 evaluations/runs/%s\n", result.Status, result.ExitCode, result.Error, request.RunID)
		}
	}
	if len(outcome.Arms) > 0 {
		return outcome
	}
	return nil
}

func executableVersion(path string) (string, error) {
	cmd := exec.Command(path, "--version")
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读取 Claude Code 版本: %s: %w", strings.TrimSpace(string(output)), err)
	}
	version := strings.TrimSpace(string(output))
	if version == "" || strings.ContainsAny(version, "\r\n\x00") {
		return "", errors.New("Claude Code 版本输出无效")
	}
	return version, nil
}
