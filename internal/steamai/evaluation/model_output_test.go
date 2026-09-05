package evaluation

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelOutputRequiresExactModelAndCost(t *testing.T) {
	const model = "gpt-5.6-terra[1m]"
	const normalized = "gpt-5.6-terra"
	for _, test := range []struct {
		name    string
		model   string
		models  []string
		mutate  func(map[string]any)
		wantErr string
	}{
		{name: "claude-exact", model: "claude-sonnet-5", models: []string{"claude-sonnet-5"}},
		{name: "gpt-exact", model: model, models: []string{model}},
		{name: "gpt-1m-normalized", model: model, models: []string{normalized}},
		{name: "gpt-without-suffix", model: normalized, models: []string{normalized}},
		{name: "every-main-message-matches", model: model, models: []string{model, normalized, model}},
		{
			name: "additional-usage-is-not-main-model-drift", model: model, models: []string{normalized},
			mutate: func(result map[string]any) {
				result["modelUsage"].(map[string]any)["claude-haiku-4-5"] = map[string]any{"canonicalModel": "auxiliary-model"}
			},
		},
		{
			name: "canonical-is-optional", model: model, models: []string{normalized},
			mutate: func(result map[string]any) { result["modelUsage"] = map[string]any{model: map[string]any{}} },
		},
		{name: "main-model-drift-with-expected-usage", model: model, models: []string{"claude-sonnet-5"}, wantErr: "主会话消息与冻结选择不一致"},
		{name: "later-main-model-drift", model: model, models: []string{normalized, "other-model"}, wantErr: "主会话消息与冻结选择不一致"},
		{name: "earlier-main-model-drift", model: model, models: []string{"other-model", normalized}, wantErr: "主会话消息与冻结选择不一致"},
		{name: "suffix-is-not-added-to-selection", model: normalized, models: []string{model}, wantErr: "主会话消息与冻结选择不一致"},
		{name: "missing-main-assistant", model: model, wantErr: "冻结选择的用量或执行证据"},
		{name: "missing-message-model", model: model, models: []string{""}, wantErr: "缺少主执行身份"},
		{name: "invalid-message-model", model: model, models: []string{"invalid model"}, wantErr: "缺少主执行身份"},
		{
			name: "missing-usage", model: model, models: []string{normalized}, wantErr: "冻结选择的用量或执行证据",
			mutate: func(result map[string]any) { delete(result, "modelUsage") },
		},
		{
			name: "empty-usage", model: model, models: []string{normalized}, wantErr: "冻结选择的用量或执行证据",
			mutate: func(result map[string]any) { result["modelUsage"] = map[string]any{} },
		},
		{
			name: "normalized-usage-key-is-not-exact-selection", model: model, models: []string{normalized}, wantErr: "冻结选择的用量或执行证据",
			mutate: func(result map[string]any) {
				result["modelUsage"] = map[string]any{normalized: map[string]any{"canonicalModel": model}}
			},
		},
		{
			name: "wrong-usage-key", model: model, models: []string{normalized}, wantErr: "冻结选择的用量或执行证据",
			mutate: func(result map[string]any) {
				result["modelUsage"] = map[string]any{"other-model": map[string]any{"canonicalModel": model}}
			},
		},
		{
			name: "canonical-conflict", model: model, models: []string{normalized}, wantErr: "冻结选择的用量或执行证据",
			mutate: func(result map[string]any) {
				result["modelUsage"] = map[string]any{model: map[string]any{"canonicalModel": "other-model"}}
			},
		},
		{
			name: "canonical-must-keep-exact-selection-suffix", model: model, models: []string{normalized}, wantErr: "冻结选择的用量或执行证据",
			mutate: func(result map[string]any) {
				result["modelUsage"] = map[string]any{model: map[string]any{"canonicalModel": normalized}}
			},
		},
		{
			name: "missing-cost", model: model, models: []string{normalized}, wantErr: "total_cost_usd",
			mutate: func(result map[string]any) { delete(result, "total_cost_usd") },
		},
		{
			name: "null-cost", model: model, models: []string{normalized}, wantErr: "total_cost_usd",
			mutate: func(result map[string]any) { result["total_cost_usd"] = nil },
		},
		{
			name: "negative-cost", model: model, models: []string{normalized}, wantErr: "total_cost_usd",
			mutate: func(result map[string]any) { result["total_cost_usd"] = -0.01 },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := map[string]any{
				"type":              "result",
				"structured_output": modelResult{Summary: "bounded", Evidence: []string{}, Limitations: []string{}, SafetyGate: "pass"},
				"total_cost_usd":    0.01,
				"modelUsage":        map[string]any{test.model: map[string]any{"canonicalModel": test.model}},
			}
			if test.mutate != nil {
				test.mutate(result)
			}
			gate, cost, err := validateModelOutput(marshalModelOutput(t, result, test.models...), test.model)
			if test.wantErr == "" {
				if err != nil || gate != "pass" || cost == nil || *cost != 0.01 {
					t.Fatalf("有效模型输出校验 = %q, %v, %v", gate, cost, err)
				}
			} else {
				if err == nil || gate != "fail" || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("模型输出校验 = %q, %v, %v，预期 %q", gate, cost, err, test.wantErr)
				}
				if test.wantErr == "total_cost_usd" {
					if cost != nil {
						t.Fatalf("无效费用仍被报告: %v", *cost)
					}
				} else if cost == nil || *cost != 0.01 {
					t.Fatal("可解析的身份错误输出未保留已报告费用")
				}
			}
		})
	}
}

func TestModelOutputRequiresVerboseEventArray(t *testing.T) {
	const model = "gpt-5.6-terra[1m]"
	valid := marshalModelOutput(t, modelEnvelope{
		Type:             "result",
		StructuredOutput: modelResult{Summary: "bounded", Evidence: []string{}, Limitations: []string{}, SafetyGate: "pass"},
		TotalCostUSD:     new(0.01),
		ModelUsage:       map[string]modelUsageEntry{model: {CanonicalModel: model}},
	}, model)
	var events []json.RawMessage
	if err := json.Unmarshal(valid, &events); err != nil {
		t.Fatal(err)
	}
	assistant, result := string(events[0]), string(events[1])
	for _, test := range []struct {
		name    string
		output  string
		wantErr string
	}{
		{"array", string(valid), ""},
		{"system-and-user-events", `[{"type":"system"},{"type":"user"},` + assistant + `,` + result + `]`, ""},
		{"omitted-parent-is-main", `[{"type":"assistant","message":{"model":"gpt-5.6-terra"}},` + result + `]`, ""},
		{"not-json", `not json`, "invalid character"},
		{"result-envelope-is-not-verbose-array", result, "cannot unmarshal"},
		{"jsonl-is-not-json-array", assistant + "\n" + result, "cannot unmarshal"},
		{"extra-json-after-array", string(valid) + `{}`, "额外 JSON"},
		{"truncated-array", string(valid[:len(valid)-1]), "unexpected EOF"},
		{"null-array", `null`, "缺少最终 result"},
		{"empty-array", `[]`, "缺少最终 result"},
		{"missing-result", `[` + assistant + `]`, "缺少最终 result"},
		{"missing-result-type", `[` + assistant + `,` + strings.Replace(result, `"type":"result"`, `"type":""`, 1) + `]`, "缺少最终 result"},
		{"result-before-assistant", `[` + result + `,` + assistant + `]`, "唯一末尾事件"},
		{"duplicate-result", `[` + assistant + `,` + result + `,` + result + `]`, "唯一末尾事件"},
		{"event-after-result", `[` + assistant + `,` + result + `,{"type":"system"}]`, "唯一末尾事件"},
		{"subtask-with-expected-model", `[` + assistant + `,{"type":"assistant","parent_tool_use_id":"tool-1","message":{"model":"gpt-5.6-terra"}},` + result + `]`, "未授权子任务"},
		{"empty-subtask-id", `[{"type":"assistant","parent_tool_use_id":"","message":{"model":"gpt-5.6-terra"}},` + result + `]`, "未授权子任务"},
		{"missing-message", `[{"type":"assistant"},` + result + `]`, "缺少主执行身份"},
		{"model-outside-message", `[{"type":"assistant","model":"gpt-5.6-terra"},` + result + `]`, "缺少主执行身份"},
	} {
		t.Run(test.name, func(t *testing.T) {
			gate, cost, err := validateModelOutput([]byte(test.output), model)
			if test.wantErr == "" {
				if err != nil || gate != "pass" || cost == nil || *cost != 0.01 {
					t.Fatalf("有效 event array 校验 = %q, %v, %v", gate, cost, err)
				}
			} else {
				if err == nil || gate != "fail" || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("event array 校验 = %q, %v, %v，预期 %q", gate, cost, err, test.wantErr)
				}
				if (test.wantErr == "未授权子任务" || test.wantErr == "缺少主执行身份") && (cost == nil || *cost != 0.01) {
					t.Fatal("可解析的身份错误事件未保留已报告费用")
				}
			}
		})
	}
}

func TestRunAcceptsUserSelectedGPTModel(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-GPT")
	request.Model = "gpt-5.6-terra[1m]"
	specPath := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "specs", request.SuiteSpec)
	var spec SuiteSpec
	if err := json.Unmarshal(mustReadTestFile(t, specPath), &spec); err != nil {
		t.Fatal(err)
	}
	spec.Model = request.Model
	spec.Identity = SuiteSpecIdentity(spec)
	specData, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	writeExistingTestFile(t, specPath, specData)
	request.SuiteSpecSHA256 = Hash(specData)
	output := marshalModelOutput(t, modelEnvelope{
		Type:             "result",
		StructuredOutput: modelResult{Summary: "bounded", Evidence: []string{}, Limitations: []string{}, SafetyGate: "pass"},
		TotalCostUSD:     new(0.01),
		ModelUsage: map[string]modelUsageEntry{
			request.Model:      {CanonicalModel: request.Model},
			"claude-haiku-4-5": {},
		},
	}, "gpt-5.6-terra", request.Model)
	bundle, err := Run(context.Background(), git, fakeClaudeOutput(t, string(output)), "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	assertPublishedBundle(t, caseRoot, bundle, "completed")
	for _, arm := range bundle.Arms {
		var record RunRecord
		if err := json.Unmarshal(mustReadTestFile(t, filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs", request.RunID, arm.Record)), &record); err != nil {
			t.Fatal(err)
		}
		if record.Runtime.Model != request.Model {
			t.Fatalf("用户冻结的 GPT 选择被改写: %q", record.Runtime.Model)
		}
	}
}

func TestRunPreservesUnverifiedModelOutput(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-DRIFT")
	output := marshalModelOutput(t, modelEnvelope{
		Type:             "result",
		StructuredOutput: modelResult{Summary: "bounded", Evidence: []string{}, Limitations: []string{}, SafetyGate: "pass"},
		TotalCostUSD:     new(0.01),
		ModelUsage: map[string]modelUsageEntry{
			request.Model: {CanonicalModel: request.Model},
			"other-model": {CanonicalModel: "other-model"},
		},
	}, request.Model, "other-model")
	bundle, err := Run(context.Background(), git, fakeClaudeOutput(t, string(output)), "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	assertPublishedBundle(t, caseRoot, bundle, "invalid-output")
	runsRoot := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs")
	for _, arm := range bundle.Arms {
		var record RunRecord
		if err := json.Unmarshal(mustReadTestFile(t, filepath.Join(runsRoot, request.RunID, arm.Record)), &record); err != nil {
			t.Fatal(err)
		}
		if record.Budget.ActualUSD == nil || *record.Budget.ActualUSD != 0.01 {
			t.Fatal("未知模型输出的已报告费用未保留")
		}
		if record.Result.SafetyGate != "fail" || !strings.Contains(record.Result.Error, "主会话消息与冻结选择不一致") {
			t.Fatalf("未知模型输出未命中主模型漂移分支: %+v", record.Result)
		}
		if got := mustReadTestFile(t, filepath.Join(runsRoot, request.RunID, arm.Output)); strings.TrimSpace(string(got)) != string(output) {
			t.Fatal("未知模型输出的原始证据被改写")
		}
	}
	if _, err := VerifyBundle(runsRoot, request.RunID+"/manifest.json", request.Purpose, request.PatchSHA256, true); err == nil {
		t.Fatal("未知模型输出被当成可晋级的 completed bundle")
	}
}

func TestVerifyCompletedBundleRechecksModelCostAndSafety(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	for _, mutation := range []string{"none", "record-model", "missing-cost", "cost-mismatch", "safety-mismatch", "drift-assistant-model"} {
		t.Run(mutation, func(t *testing.T) {
			caseRoot, request := evaluationFixture(t, git, "RUN-OUTPUT")
			bundle, err := Run(context.Background(), git, fakeClaude(t, false), "Claude Code fixture", caseRoot, request)
			if err != nil {
				t.Fatal(err)
			}
			runsRoot := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs")
			runRoot := filepath.Join(runsRoot, request.RunID)
			arm := &bundle.Arms[0]
			var record RunRecord
			if err := json.Unmarshal(mustReadTestFile(t, filepath.Join(runRoot, arm.Record)), &record); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "record-model":
				record.Runtime.Model = "claude-other-model"
			case "missing-cost":
				record.Budget.ActualUSD = nil
			case "cost-mismatch":
				record.Budget.ActualUSD = new(0.5)
			case "safety-mismatch":
				record.Result.SafetyGate = "fail"
			case "drift-assistant-model":
				var events []json.RawMessage
				if err := json.Unmarshal(mustReadTestFile(t, filepath.Join(runRoot, arm.Output)), &events); err != nil {
					t.Fatal(err)
				}
				var assistant modelEvent
				if err := json.Unmarshal(events[0], &assistant); err != nil || assistant.Type != "assistant" {
					t.Fatalf("fixture 缺少主 assistant: %+v, %v", assistant, err)
				}
				assistant.Message.Model = "other-model"
				data, err := json.Marshal(assistant)
				if err != nil {
					t.Fatal(err)
				}
				events[0] = data
				output, err := json.Marshal(events)
				if err != nil {
					t.Fatal(err)
				}
				writeExistingTestFile(t, filepath.Join(runRoot, arm.Output), output)
				arm.OutputSHA256 = Hash(output)
				record.Result.OutputSHA256, record.Result.OutputBytes = Hash(output), len(output)
			}
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			writeExistingTestFile(t, filepath.Join(runRoot, arm.Record), data)
			arm.RecordSHA256 = Hash(data)
			packetSources := make(map[string]reviewPacketSource, len(bundle.Arms))
			for _, currentArm := range bundle.Arms {
				var currentRecord RunRecord
				if err := json.Unmarshal(mustReadTestFile(t, filepath.Join(runRoot, currentArm.Record)), &currentRecord); err != nil {
					t.Fatal(err)
				}
				packetSources[currentArm.Label] = reviewPacketSource{
					record: currentRecord,
					output: mustReadTestFile(t, filepath.Join(runRoot, currentArm.Output)),
				}
			}
			_, packetData, err := encodeBlindReviewPacket(bundle.RunID, bundle.Arms, packetSources)
			if err != nil {
				t.Fatal(err)
			}
			writeExistingTestFile(t, filepath.Join(runRoot, bundle.ReviewPacket.Path), packetData)
			bundle.ReviewPacket.SHA256 = Hash(packetData)
			// 同时重建 packet 与所有 hash/identity，确保拒绝来自 model/cost/safety 语义不一致而非旧摘要。
			bundle.Reveal.BlindIdentity = BlindBundleIdentity(bundle)
			revealData, err := json.Marshal(bundle.Reveal)
			if err != nil {
				t.Fatal(err)
			}
			writeExistingTestFile(t, filepath.Join(runRoot, "reveal.json"), revealData)
			bundle.RevealSHA256 = Hash(revealData)
			bundle.Identity = BundleIdentity(bundle)
			manifestData, err := json.Marshal(bundle)
			if err != nil {
				t.Fatal(err)
			}
			writeExistingTestFile(t, filepath.Join(runRoot, "manifest.json"), manifestData)
			_, err = VerifyBundle(runsRoot, request.RunID+"/manifest.json", request.Purpose, request.PatchSHA256, false)
			if (err == nil) != (mutation == "none") {
				t.Fatalf("已完成 bundle 的输出重验 = %v", err)
			}
		})
	}
}
